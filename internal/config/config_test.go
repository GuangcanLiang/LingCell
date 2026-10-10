package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfiguration checks missing-file defaults, model readiness and strict
// rejection of unknown fields, invalid limits, unsafe URLs and trailing JSON.
func TestConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c, err := Load(path)
	if err != nil || c.Ready() || c.MaxDecisions != 8 || c.TimeoutSeconds != 60 {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	if err := os.WriteFile(path, []byte(`{"base_url":"https://example.test/v1","model":"selected-model","api_key_env":"CUSTOM_KEY"}`), 0600); err != nil {
		t.Fatal(err)
	}
	c, err = Load(path)
	if err != nil || !c.Ready() || c.APIKeyEnv != "CUSTOM_KEY" {
		t.Fatalf("configured: %+v %v", c, err)
	}
	for _, data := range []string{`{"unknown":true}`, `{"timeout_seconds":0}`, `{"max_decisions":0}`, `{"base_url":"file:///tmp/a"}`, `{"base_url":"https://user:secret@example.test/v1"}`, `{} {}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Errorf("invalid config accepted: %s", data)
		}
	}
}

// TestRejectOversizedAndNullConfig checks that a valid prefix cannot hide an
// oversized document and that null cannot masquerade as a configuration object.
func TestRejectOversizedAndNullConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, data := range []string{"null", "{}" + strings.Repeat(" ", maxConfigSize) + "{}"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("accepted a truncated or non-object config")
		}
	}
}
