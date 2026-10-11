//go:build linux

// Lingcell runs an interactive Bash with confirmed, model-assisted tasks.
// It requires a terminal; an absent model configuration leaves Bash usable.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/term"

	"lingcell/internal/app"
	"lingcell/internal/config"
	"lingcell/internal/model"
	"lingcell/internal/project"
	"lingcell/internal/runtime"
	"lingcell/internal/shell"
	"time"
)

func main() { os.Exit(run()) }

// run owns startup and cleanup and returns the process exit code.
// Keeping cleanup here lets deferred terminal restoration run before os.Exit.
func run() int {
	path := flag.String("config", config.DefaultPath(), "JSON configuration path")
	noHistory := flag.Bool("no-history", false, "do not persist input history")
	resume := flag.String("resume", "", "explicitly restore a conversation by ID")
	ephemeral := flag.Bool("ephemeral", false, "disable session, memory and input-history persistence")
	doctor := flag.Bool("doctor", false, "check local dependencies and configuration without network access")
	version := flag.Bool("version", false, "show version")
	flag.Parse()
	if *version {
		fmt.Println("lingcell 0.1.0")
		return 0
	}
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: lingcell [--config path] [--no-history] [--resume id | --ephemeral] [--doctor]")
		return 2
	}
	if *ephemeral && *resume != "" {
		fmt.Fprintln(os.Stderr, "--ephemeral cannot resume persisted sessions")
		return 2
	}
	if *doctor {
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL working directory")
			return 1
		}
		if err := app.Doctor(os.Stdout, *path, dir); err != nil {
			return 1
		}
		return 0
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		fmt.Fprintln(os.Stderr, "LingCell requires an interactive terminal")
		return 1
	}
	state, err := term.GetState(fd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer term.Restore(fd, state)
	cfg, err := config.Load(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	reader, err := project.New(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer reader.Close()
	var store *runtime.Store
	var runtimeErr error
	if !*ephemeral {
		store, runtimeErr = runtime.Open(runtime.DefaultPath(), os.Getenv(cfg.APIKeyEnv))
		if store != nil {
			defer store.Close()
		}
	}
	var conversation *runtime.Runtime
	if runtimeErr == nil {
		conversation, runtimeErr = runtime.New(store, reader.Root, *resume, cfg.ContextMaxBytes, cfg.Model, time.Duration(cfg.TimeoutSeconds)*time.Second)
	}
	if runtimeErr != nil && *resume != "" {
		fmt.Fprintln(os.Stderr, runtimeErr)
		return 1
	}
	defer func() {
		if conversation != nil {
			_ = conversation.Close()
		}
	}()
	// A deferred closure below closes the currently selected session after any switch.

	sess, err := shell.Start(context.Background(), shell.Options{Dir: dir, Output: os.Stdout, Input: os.Stdin})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer sess.Close()
	// The active cancel function is shared by the signal goroutine and interaction
	// loop. Copy it under the lock, then call it outside the critical section.
	var mu sync.Mutex
	var taskCancel context.CancelFunc
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	finished := make(chan struct{})
	defer close(finished)
	// Normal return closes finished. Termination signals exit the process after
	// explicit cleanup because os.Exit does not execute deferred functions.
	go func() {
		for {
			select {
			case sig := <-signals:
				mu.Lock()
				cancel := taskCancel
				mu.Unlock()
				if sig == os.Interrupt && cancel != nil {
					cancel()
					continue
				}
				sess.Close()
				_ = term.Restore(fd, state)
				os.Exit(128 + int(sig.(syscall.Signal)))
			case <-finished:
				return
			}
		}
	}()
	a := app.App{Runtime: conversation, RuntimeError: runtimeErr, Shell: sess, Reader: reader, Model: model.New(cfg), Config: cfg, Output: os.Stdout}
	a.WithCancel = func(cancel context.CancelFunc) func() {
		mu.Lock()
		// Classification can enter a nested command or Agent task. Restoring the
		// previous handler keeps Ctrl-C bound to the enclosing active task.
		previous := taskCancel
		taskCancel = cancel
		mu.Unlock()
		return func() {
			mu.Lock()
			taskCancel = previous
			mu.Unlock()
		}
	}
	defer func() {
		if a.Runtime != nil {
			_ = a.Runtime.Close()
		}
	}()
	if err := a.Init(*noHistory || *ephemeral); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := a.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
