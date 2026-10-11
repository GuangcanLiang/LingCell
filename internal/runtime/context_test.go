package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"lingcell/internal/model"
)

type contextModel struct {
	calls    int
	requests [][]model.Message
	reply    model.Message
	err      error
}

func (m *contextModel) Complete(_ context.Context, messages []model.Message, _ []model.Tool) (model.Message, error) {
	m.calls++
	m.requests = append(m.requests, append([]model.Message(nil), messages...))
	return m.reply, m.err
}
func testRuntime(t *testing.T, s *Store) *Runtime {
	t.Helper()
	r, err := New(s, "/fixed/project", "", 128*1024, "mock", 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}
func recordTurn(t *testing.T, r *Runtime, messages []model.Message) {
	t.Helper()
	if err := r.Begin(nil); err != nil {
		t.Fatal(err)
	}
	for _, m := range messages {
		if err := r.Append(m); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Finish(true); err != nil {
		t.Fatal(err)
	}
}
func TestContinuityRestoreIsolationAndInterrupted(t *testing.T) {
	s := testStore(t)
	r := testRuntime(t, s)
	recordTurn(t, r, []model.Message{{Role: "user", Content: `{"request":"Keep user constraint"}`}, {Role: "assistant", Content: "first answer"}})
	m := &contextModel{}
	out, err := r.Context(context.Background(), m, model.Message{Role: "system", Content: "current rules"}, []model.Message{{Role: "user", Content: "follow up"}}, nil)
	if err != nil || len(out) != 4 || out[2].Content != "first answer" {
		t.Fatalf("continuity %v %v", out, err)
	}
	if err = r.Begin([]model.Message{{Role: "user", Content: "command request"}}); err != nil {
		t.Fatal(err)
	}
	c := model.ToolCall{ID: "call", Type: "function", Function: model.FunctionCall{Name: "run_command", Arguments: `{"command":"touch must-not-repeat","purpose":"test"}`}}
	if err = r.Append(model.Message{Role: "assistant", ToolCalls: []model.ToolCall{c}}); err != nil {
		t.Fatal(err)
	}
	if err = r.ToolState("call", "started", true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = New(s, r.Session.Root, r.Session.ID, 0, "mock", 0); err == nil {
		t.Fatal("concurrent restore accepted")
	}
	if _, err = New(s, "/wrong", r.Session.ID, 0, "mock", 0); err == nil {
		t.Fatal("cross-project restore")
	}
	id := r.Session.ID
	r.Close()
	restored, err := New(s, "/fixed/project", id, 0, "mock", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if !strings.Contains(restored.Interrupted(), "结果未知") || restored.Turns[1].Status != "interrupted" {
		t.Fatal("unknown execution not surfaced")
	}
	out, err = restored.Context(context.Background(), m, model.Message{Role: "system"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	last := out[len(out)-1]
	if last.Role != "tool" || !strings.Contains(last.Content, "unknown") {
		t.Fatalf("pending call not closed: %+v", last)
	}
	fresh := testRuntime(t, s)
	out, err = fresh.Context(context.Background(), m, model.Message{Role: "system"}, nil, nil)
	if err != nil || len(out) != 1 {
		t.Fatalf("fresh history leaked %v %v", out, err)
	}
}
func TestExactBudgetSummaryAndFailureRetention(t *testing.T) {
	r := testRuntime(t, testStore(t))
	r.MaxBytes = 25 * 1024
	for i := 0; i < 5; i++ {
		recordTurn(t, r, []model.Message{{Role: "user", Content: "constraint never delete"}, {Role: "assistant", Content: strings.Repeat("中", 1800)}})
	}
	m := &contextModel{reply: model.Message{Role: "assistant", Content: "Preserve constraint never delete; first tasks completed"}}
	out, err := r.Context(context.Background(), m, model.Message{Role: "system", Content: "rules"}, []model.Message{{Role: "user", Content: "followup"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := model.EncodeRequest(r.ModelName, out, nil)
	if err != nil || len(raw) > r.MaxBytes || len(raw) != r.LastBytes || m.calls == 0 || r.Through > 3 {
		t.Fatalf("budget %d %d summary %d %d %v", len(raw), r.LastBytes, m.calls, r.Through, err)
	}
	for _, req := range m.requests {
		b, _ := model.EncodeRequest(r.ModelName, req, nil)
		if len(b) > r.MaxBytes {
			t.Fatal("summary request over budget")
		}
	}
	old := r.Through
	recordTurn(t, r, []model.Message{{Role: "user", Content: "next"}, {Role: "assistant", Content: strings.Repeat("x", 15*1024)}})
	failing := &contextModel{err: errors.New("summary offline")}
	_, err = r.Context(context.Background(), failing, model.Message{Role: "system"}, nil, nil)
	if err == nil || r.Through != old {
		t.Fatal("summary failure silently discarded history")
	}
	saved, err := r.Store.turns(r.Session.ID)
	if err != nil || len(saved) != 6 || !strings.Contains(saved[0].Messages[0].Content, "never delete") {
		t.Fatal("archive lost after summary")
	}
}
func TestContextLargeCurrentToolAndHistoryBound(t *testing.T) {
	r := testRuntime(t, nil)
	r.MaxBytes = 16 * 1024
	call := model.Message{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "big", Type: "function", Function: model.FunctionCall{Name: "read_file", Arguments: `{"path":"a"}`}}}}
	data := model.Message{Role: "tool", ToolCallID: "big", Content: strings.Repeat("x", 64*1024)}
	current := []model.Message{{Role: "user", Content: "keep this constraint"}, call, data}
	out, err := r.Context(context.Background(), &contextModel{}, model.Message{Role: "system"}, current, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out[1].Content != "keep this constraint" || out[2].ToolCalls[0].ID != out[3].ToolCallID || !strings.Contains(out[3].Content, "truncated") {
		t.Fatal("tool pairing or constraints lost")
	}
	if len(current[2].Content) != 64*1024 {
		t.Fatal("context changed original")
	}
	messages := []model.Message{}
	for i := 0; i < 64; i++ {
		messages = append(messages, model.Message{Role: "assistant", Content: strings.Repeat("z", 1024)})
	}
	recordTurn(t, r, messages)
	page, err := r.History(1, 20)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(page)
	if len(raw) > 32*1024 || !strings.Contains(string(raw), `"truncated":true`) {
		t.Fatal("unbounded history", len(raw))
	}
	_, err = r.Context(context.Background(), &contextModel{}, model.Message{Role: "system"}, []model.Message{{Role: "user", Content: strings.Repeat("x", 64*1024)}}, nil)
	if err == nil {
		t.Fatal("large current constraint silently discarded")
	}
}
func TestWriteFaultStopsSubsequentTurns(t *testing.T) {
	r := testRuntime(t, testStore(t))
	if err := r.Begin([]model.Message{{Role: "user", Content: "first"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Store.db.Exec("PRAGMA query_only=ON"); err != nil {
		t.Fatal(err)
	}
	if err := r.Append(model.Message{Role: "assistant", Content: "unsaved response"}); err == nil {
		t.Fatal("write fault ignored")
	}
	_ = r.Finish(false)
	if err := r.Begin(nil); err == nil {
		t.Fatal("continued after write failure")
	}
}
