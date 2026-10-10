package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lingcell/internal/config"
)

func TestHTTPToolProtocol(t *testing.T) {
	t.Setenv("TEST_LINGCELL_KEY", "mock-secret-never-a-real-key")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer mock-secret-never-a-real-key" {
			t.Error("request endpoint or auth mismatch")
		}
		var body struct {
			Model    string    `json:"model"`
			Messages []Message `json:"messages"`
			Tools    []Tool    `json:"tools"`
			Parallel bool      `json:"parallel_tool_calls"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "mock-model" || len(body.Tools) != 1 || body.Parallel || body.Messages[1].ToolCallID != "1" {
			t.Errorf("invalid request: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"tool_calls":[{"id":"2","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"README.md\"}"}}]}}]}`))
	}))
	defer srv.Close()
	c := New(config.Config{BaseURL: srv.URL + "/v1", Model: "mock-model", APIKeyEnv: "TEST_LINGCELL_KEY", TimeoutSeconds: 1})
	m, err := c.Complete(context.Background(), []Message{{Role: "assistant", ToolCalls: []ToolCall{{ID: "1", Type: "function", Function: FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`}}}}, {Role: "tool", ToolCallID: "1", Content: "untrusted"}}, []Tool{{Type: "function", Function: ToolDefinition{Name: "read_file", Parameters: map[string]any{"type": "object"}}}})
	if err != nil || len(m.ToolCalls) != 1 || m.ToolCalls[0].ID != "2" {
		t.Fatalf("tool response: %+v %v", m, err)
	}
}
func TestClassificationAndFailures(t *testing.T) {
	for _, test := range []struct {
		name, response string
		status         int
		want           string
		bad            bool
	}{
		{"classify", `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"kind\":\"shell\"}"}}]}`, 200, "shell", false},
		{"bad class", `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"shell"}}]}`, 200, "", true},
		{"truncated", `{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":"{\"kind\":\"shell\"}"}}]}`, 200, "", true},
		{"invalid JSON", `{`, 200, "", true}, {"remote secret", `mock-secret-never-a-real-key`, 401, "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				w.Write([]byte(test.response))
			}))
			defer srv.Close()
			c := New(config.Config{BaseURL: srv.URL, Model: "mock", TimeoutSeconds: 1})
			kind, err := Classify(context.Background(), c, "lss -la")
			if (err != nil) != test.bad || kind != test.want {
				t.Fatalf("classification: %s %v", kind, err)
			}
			if err != nil && strings.Contains(err.Error(), "mock-secret") {
				t.Fatal("secret echoed from remote error")
			}
		})
	}
}
func TestCancellationAndNoRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(250 * time.Millisecond):
		}
	}))
	defer srv.Close()
	c := New(config.Config{BaseURL: srv.URL, Model: "mock", TimeoutSeconds: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.Complete(ctx, []Message{{Role: "user", Content: "test"}}, nil); err == nil {
		t.Fatal("timeout was ignored")
	}
	if time.Since(start) > time.Second {
		t.Fatal("request was not canceled")
	}
	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	c = New(config.Config{BaseURL: redirect.URL, Model: "mock", TimeoutSeconds: 1})
	if _, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "test"}}, nil); err == nil || redirected {
		t.Fatal("unexpected redirect forwarding")
	}
}
