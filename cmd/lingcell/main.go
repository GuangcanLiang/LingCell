//go:build linux

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
	"lingcell/internal/shell"
)

func main() { os.Exit(run()) }
func run() int {
	path := flag.String("config", config.DefaultPath(), "JSON configuration path")
	noHistory := flag.Bool("no-history", false, "do not persist input history")
	version := flag.Bool("version", false, "show version")
	flag.Parse()
	if *version {
		fmt.Println("lingcell 0.1.0")
		return 0
	}
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: lingcell [-config path] [-no-history]")
		return 2
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
	sess, err := shell.Start(context.Background(), shell.Options{Dir: dir, Output: os.Stdout, Input: os.Stdin})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer sess.Close()
	var mu sync.Mutex
	var taskCancel context.CancelFunc
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	finished := make(chan struct{})
	defer close(finished)
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
	a := app.App{Shell: sess, Reader: reader, Model: model.New(cfg), Config: cfg, Output: os.Stdout}
	a.WithCancel = func(cancel context.CancelFunc) func() {
		mu.Lock()
		previous := taskCancel
		taskCancel = cancel
		mu.Unlock()
		return func() {
			mu.Lock()
			taskCancel = previous
			mu.Unlock()
		}
	}
	if err := a.Init(*noHistory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := a.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
