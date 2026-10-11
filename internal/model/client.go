// Package model implements the tool-calling Chat Completions wire protocol.
package model

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"lingcell/internal/config"
)

// ToolCall describes an assistant's requested function invocation.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall contains a tool name and its JSON-encoded arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message is a Chat Completions conversation message or tool result.
// A tool result uses ToolCallID to match an assistant call. File and command
// contents belong in tool messages rather than user or system instructions.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool advertises an available function to the model service.
type Tool struct {
	Type     string         `json:"type"`
	Function ToolDefinition `json:"function"`
}

// ToolDefinition describes a tool and its JSON Schema parameters.
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// Client returns a complete assistant decision or an error. Implementations
// must respect context cancellation; they do not execute tools.
type Client interface {
	// Complete returns one assistant decision without invoking any tools.
	// The caller owns the conversation and must not mutate it during the call.
	Complete(context.Context, []Message, []Tool) (Message, error)
}

// HTTPClient talks to a configured service without following redirects.
// It can be shared by callers; credentials are read from the configured env var.
type HTTPClient struct {
	config config.Config
	http   *http.Client
}

// New constructs a client using the configured request timeout.
// The caller supplies a validated config, normally obtained from config.Load.
func New(cfg config.Config) *HTTPClient {
	return &HTTPClient{
		config: cfg,
		http: &http.Client{
			Timeout:       time.Duration(cfg.TimeoutSeconds) * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Complete sends one bounded, non-streaming Chat Completions request.
// It makes no application-level retry and follows no redirects. Successful
// responses are capped at 1 MiB and must contain exactly one finished assistant
// message. An empty tools slice requests an ordinary text response.
func (c *HTTPClient) Complete(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	if !c.config.Ready() {
		return Message{}, &Error{Kind: Configuration}
	}
	body := struct {
		Model      string    `json:"model"`
		Messages   []Message `json:"messages"`
		Tools      []Tool    `json:"tools,omitempty"`
		ToolChoice string    `json:"tool_choice,omitempty"`
		Parallel   bool      `json:"parallel_tool_calls"`
	}{Model: c.config.Model, Messages: messages, Tools: tools}
	if len(tools) > 0 {
		body.ToolChoice = "auto"
	}
	// The zero Parallel field is deliberately encoded as false: execution
	// authority is granted one command at a time by the Agent.
	data, err := json.Marshal(body)
	if err != nil {
		return Message{}, &Error{Kind: Protocol}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.config.BaseURL, "/")+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return Message{}, &Error{Kind: Protocol}
	}
	req.Header.Set("Content-Type", "application/json")
	if key := os.Getenv(c.config.APIKeyEnv); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Message{}, transportError(err)
	}
	defer resp.Body.Close()
	// Never print remote error bodies: they may echo credentials or request data.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Message{}, statusError(resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return Message{}, transportError(err)
	}
	if len(b) > 1024*1024 {
		return Message{}, &Error{Kind: Protocol}
	}
	var result struct {
		Choices []struct {
			Message      Message `json:"message"`
			FinishReason string  `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return Message{}, &Error{Kind: Protocol}
	}
	if len(result.Choices) != 1 {
		return Message{}, &Error{Kind: Protocol}
	}
	choice := result.Choices[0]
	if choice.FinishReason != "stop" && choice.FinishReason != "tool_calls" {
		return Message{}, &Error{Kind: Protocol}
	}
	m := choice.Message
	if m.Role != "assistant" || (m.Content == "" && len(m.ToolCalls) == 0) {
		return Message{}, &Error{Kind: Protocol}
	}
	return m, nil
}

// Classify asks the model to return shell or agent without rewriting input.
// Tool calls, unknown fields, extra JSON values and all other classifications
// are rejected; classification alone never authorizes command execution.
func Classify(ctx context.Context, c Client, input string) (string, error) {
	m, err := c.Complete(ctx, []Message{{Role: "system", Content: `Classify the user's input as a shell command (including misspelled commands) or a natural-language task. Ambiguous requests such as "find large files" are tasks. Return only {"kind":"shell"} or {"kind":"agent"}. Do not rewrite input or call tools.`}, {Role: "user", Content: input}}, nil)
	if err != nil {
		return "", err
	}
	if len(m.ToolCalls) > 0 {
		return "", &Error{Kind: Protocol}
	}
	var result struct {
		Kind string `json:"kind"`
	}
	dec := json.NewDecoder(strings.NewReader(m.Content))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return "", &Error{Kind: Protocol}
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || (result.Kind != "shell" && result.Kind != "agent") {
		return "", &Error{Kind: Protocol}
	}
	return result.Kind, nil
}
