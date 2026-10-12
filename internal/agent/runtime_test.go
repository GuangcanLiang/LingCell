package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"lingcell/internal/model"
	"lingcell/internal/runtime"
	"lingcell/internal/shell"
)

func newRuntime(t *testing.T) *runtime.Runtime {
	t.Helper()
	s, err := runtime.Open(filepath.Join(t.TempDir(), "state", "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	r, err := runtime.New(s, "/project", "", 128*1024, "mock", 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}
func TestRuntimeMultiTurnAndRefusalNoReplay(t *testing.T) {
	r := newRuntime(t)
	m := &mockModel{replies: []model.Message{{Role: "assistant", Content: "Remember request-specific constraint"}, call("same-id", "run_command", `{"command":"touch unapproved","purpose":"test"}`), {Role: "assistant", Content: "followup"}}}
	s := &mockShell{}
	a := Agent{Runtime: r, Model: m, Shell: s, Reader: &mockReader{}, Confirm: func(string, string, string) (bool, error) { return false, nil }}
	if err := a.Run(context.Background(), "keep constraint", "/project"); err != nil {
		t.Fatal(err)
	}
	if err := a.Run(context.Background(), "act", "/project"); !errors.Is(err, ErrDeclined) {
		t.Fatal(err)
	}
	if err := a.Run(context.Background(), "follow up", "/project"); err != nil {
		t.Fatal(err)
	}
	if len(s.commands) != 0 || len(r.Turns) != 3 || r.Turns[1].Steps[0].Status != "declined" {
		t.Fatal("refusal not preserved")
	}
	request := m.requests[2]
	all := ""
	for _, msg := range request {
		all += msg.Content
	}
	if !strings.Contains(all, "constraint") || !strings.Contains(all, "declined") {
		t.Fatal("history lost")
	}
	for i, msg := range request {
		if len(msg.ToolCalls) > 0 {
			if i+1 >= len(request) || request[i+1].Role != "tool" || request[i+1].ToolCallID != msg.ToolCalls[0].ID {
				t.Fatal("historical tool not paired")
			}
		}
	}
}
func TestRuntimeWriteBarrierPreventsApprovedCommand(t *testing.T) {
	r := newRuntime(t)
	m := &mockModel{replies: []model.Message{call("exec", "run_command", `{"command":"touch marker","purpose":"test"}`)}}
	s := &mockShell{}
	a := Agent{Runtime: r, Model: m, Shell: s, Reader: &mockReader{}, Confirm: func(string, string, string) (bool, error) { r.Store.Close(); return true, nil }}
	if err := a.Run(context.Background(), "do it", "/project"); err == nil {
		t.Fatal("store failure ignored")
	}
	if len(s.commands) != 0 {
		t.Fatal("executed before durable start record")
	}
}

// uncertainShell models transport loss after execution starts.
type uncertainShell struct{ runs int }

func (s *uncertainShell) Cwd() string { return "/project" }
func (s *uncertainShell) Run(context.Context, string) (shell.Result, error) {
	s.runs++
	return shell.Result{}, errors.New("transport lost")
}
func TestRuntimeUnknownExecutionAndFreshApprovals(t *testing.T) {
	r := newRuntime(t)
	s := &uncertainShell{}
	m := &mockModel{replies: []model.Message{call("exec", "run_command", `{"command":"touch marker","purpose":"test"}`)}}
	a := Agent{Runtime: r, Model: m, Shell: s, Reader: &mockReader{}, Confirm: func(string, string, string) (bool, error) { return true, nil }}
	if err := a.Run(context.Background(), "do it", "/project"); err == nil || s.runs != 1 {
		t.Fatal("uncertain execution retried")
	}
	if r.Turns[0].Steps[0].Status != "unknown" || !strings.Contains(r.Interrupted(), "结果未知") {
		t.Fatal("unknown state missing")
	}
	a.Model = &mockModel{replies: []model.Message{call("exec", "run_command", `{"command":"touch marker","purpose":"test again"}`)}}
	a.Confirm = func(string, string, string) (bool, error) { return false, nil }
	if err := a.Run(context.Background(), "again", "/project"); !errors.Is(err, ErrDeclined) || s.runs != 1 {
		t.Fatal("old permission reused", err)
	}
}
