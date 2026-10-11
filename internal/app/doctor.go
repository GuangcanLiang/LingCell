package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"lingcell/internal/config"
	"lingcell/internal/project"
)

// Doctor checks local readiness without a terminal, network or writes.
// Output contains fixed status labels only, never config values or file data.
// A named but absent credential is a warning: local services can be unauthenticated.
func Doctor(output io.Writer, path, dir string) error {
	failed := false
	report := func(status, check string) error {
		if status == "FAIL" {
			failed = true
		}
		text := fmt.Sprintf("%s %s\n", status, check)
		n, err := io.WriteString(output, text)
		if err == nil && n != len(text) {
			return io.ErrShortWrite
		}
		return err
	}
	for _, name := range []string{"bash", "stty"} {
		status := "PASS"
		if _, err := exec.LookPath(name); err != nil {
			status = "FAIL"
		}
		if err := report(status, "dependency "+name); err != nil {
			return err
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		if err := report("FAIL", "configuration invalid or unreadable; check the local config file"); err != nil {
			return err
		}
	} else {
		label := "configuration valid"
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			label = "configuration absent; defaults loaded"
		}
		if err := report("PASS", label); err != nil {
			return err
		}
		status := "PASS"
		if !cfg.Ready() {
			status = "FAIL"
		}
		if err := report(status, "model endpoint and name configured"); err != nil {
			return err
		}
		status = "SKIP"
		if cfg.APIKeyEnv != "" {
			status = "PASS"
			if os.Getenv(cfg.APIKeyEnv) == "" {
				status = "WARN"
			}
		}
		if err := report(status, "credential environment variable present (absence allowed for unauthenticated services)"); err != nil {
			return err
		}
	}
	status := "PASS"
	reader, err := project.New(dir)
	if err != nil {
		status = "FAIL"
	} else {
		if _, err := reader.NewTask().ListFiles("."); err != nil {
			status = "FAIL"
		}
		if err := reader.Close(); err != nil {
			status = "FAIL"
		}
	}
	if err := report(status, "project reading and kernel path constraints"); err != nil {
		return err
	}
	if failed {
		return errors.New("local readiness checks failed")
	}
	return nil
}
