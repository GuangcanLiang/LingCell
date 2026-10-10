package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
