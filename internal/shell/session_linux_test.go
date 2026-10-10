package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func session(t *testing.T, rc string) *Session {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")
	if err := os.WriteFile(path, []byte(rc), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := Start(ctx, Options{Dir: dir, RCFile: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}
func run(t *testing.T, s *Session, command string) Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.Run(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestStateAndBashSemantics(t *testing.T) {
	s := session(t, `alias personal='printf personal'; personal_fn(){ printf fn; }`)
	for _, command := range []string{"mkdir child", "cd child", "export LC_TEST=persisted", "alias hi='printf alias'", "greet(){ printf function; }"} {
		if r := run(t, s, command); r.ExitCode != 0 {
			t.Fatalf("%s: %+v", command, r)
		}
	}
	r := run(t, s, `printf '%s\n' "$LC_TEST"; hi; greet; personal; personal_fn`)
	if !strings.Contains(r.Output, "persisted\r\naliasfunctionpersonalfn") || !strings.HasSuffix(r.Cwd, "/child") {
		t.Fatalf("state not preserved: %+v", r)
	}
	if !s.Knows("hi") || !s.Knows("greet") || !s.Knows("personal") {
		t.Fatal("command names were not refreshed")
	}
	r = run(t, s, "printf '%s\\n' 'a b' | tr a-z A-Z > sample\ncat sample")
	if r.ExitCode != 0 || !strings.Contains(r.Output, "A B") {
		t.Fatalf("pipeline/multiline: %+v", r)
	}
	r = run(t, s, "cat <<'END'\none\ntwo\nEND")
	if !strings.Contains(r.Output, "one\r\ntwo") {
		t.Fatalf("heredoc: %+v", r)
	}
	r = run(t, s, "printf '%s' "+Quote(strings.Repeat("x", 9000)))
	if len(r.Output) != 9000 {
		t.Fatalf("long command truncated at PTY boundary: %d", len(r.Output))
	}
}
func TestPromptHooksAndStatus(t *testing.T) {
	s := session(t, `printf 'startup\n'; PROMPT_COMMAND=('printf "hook-one\n"' 'printf "hook-two\n"'); PS1='CUSTOM>'`)
	r := run(t, s, "false")
	if r.ExitCode != 1 || !strings.Contains(r.Output, "hook-one") || !strings.Contains(r.Output, "hook-two") {
		t.Fatalf("hook or status lost: %+v", r)
	}
	if strings.Contains(r.Output, "CUSTOM>") || strings.Contains(r.Output, "__lc_") || strings.ContainsRune(r.Output, 0) {
		t.Fatalf("control data leaked: %q", r.Output)
	}
	s2 := session(t, `PROMPT_COMMAND='printf "prior-status=%s\n" "$?"'`)
	r = run(t, s2, "(exit 7)")
	if r.ExitCode != 7 || !strings.Contains(r.Output, "prior-status=7") {
		t.Fatalf("old hook status: %+v", r)
	}
}
func TestFailureCancelAndTruncation(t *testing.T) {
	s := session(t, "")
	if r := run(t, s, "printf )"); r.ExitCode != 2 || !strings.Contains(r.Output, "syntax error") {
		t.Fatalf("syntax failure: %+v", r)
	}
	r := run(t, s, "head -c 80000 /dev/zero | tr '\\0' x")
	if !r.Truncated || len(r.Output) != OutputLimit {
		t.Fatalf("output limit: %d %v", len(r.Output), r.Truncated)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	r, err := s.Run(ctx, "sleep 30")
	if err != nil {
		t.Fatal(err)
	}
	if r.ExitCode != 130 {
		t.Fatalf("cancel result: %+v", r)
	}
	if r = run(t, s, "echo alive"); r.ExitCode != 0 || !strings.Contains(r.Output, "alive") {
		t.Fatalf("session lost: %+v", r)
	}
}
func TestJobControl(t *testing.T) {
	s := session(t, "")
	// STOP via the PTY's foreground process group produces the same Bash path
	// as Ctrl-Z, without needing to steal the test runner's terminal input.
	done := make(chan Result, 1)
	fail := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r, e := s.Run(ctx, "sleep 30")
		if e != nil {
			fail <- e
		} else {
			done <- r
		}
	}()
	time.Sleep(100 * time.Millisecond)
	if _, err := s.terminal.Write([]byte{26}); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		if r.ExitCode != 148 {
			t.Fatalf("stop status %d", r.ExitCode)
		}
	case e := <-fail:
		t.Fatal(e)
	case <-time.After(6 * time.Second):
		t.Fatal("stop hung")
	}
	r := run(t, s, "jobs")
	if !strings.Contains(r.Output, "Stopped") {
		t.Fatalf("jobs missing: %+v", r)
	}
	r = run(t, s, "bg")
	if r.ExitCode != 0 {
		t.Fatalf("bg: %+v", r)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	r, err := s.Run(ctx, "fg")
	if err != nil || r.ExitCode != 130 {
		t.Fatalf("fg/cancel: %+v %v", r, err)
	}
}
func TestHookLossAndExit(t *testing.T) {
	for _, command := range []string{"unset PROMPT_COMMAND", "unset -f __lc_report", "exec 3>&-", "exit"} {
		t.Run(command, func(t *testing.T) {
			s := session(t, "")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := s.Run(ctx, command)
			if err == nil {
				t.Fatal("expected explicit session error")
			}
			if ctx.Err() != nil {
				t.Fatal("hook failure was not detected promptly")
			}
		})
	}
}
