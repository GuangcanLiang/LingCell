// Package config loads and validates LingCell's local JSON configuration.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const maxConfigSize = 64 * 1024

// Config selects the model service, request limits and input-history location.
// APIKeyEnv names an environment variable; it never stores a credential value.
type Config struct {
	// SkillDirs adds explicit skill libraries; default project/user roots remain.
	SkillDirs []string `json:"skill_dirs,omitempty"`
	// BaseURL is the API root; the client appends /chat/completions.
	BaseURL string `json:"base_url"`
	// Model names a model that supports Chat Completions tool calls.
	Model string `json:"model"`
	// APIKeyEnv names the credential environment variable, not its value.
	APIKeyEnv string `json:"api_key_env"`
	// MaxDecisions bounds model decisions per task; Load defaults it to 8.
	MaxDecisions int `json:"max_decisions"`
	// TimeoutSeconds bounds each model request; Load defaults it to 60.
	TimeoutSeconds int `json:"timeout_seconds"`
	// HistoryFile stores top-level input. An empty value disables persistence.
	HistoryFile string `json:"history_file"`
	// ContextMaxBytes bounds the serialized model request, not provider tokens.
	ContextMaxBytes int `json:"context_max_bytes"`
}

// DefaultPath returns the configuration path in the user's config directory.
func DefaultPath() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return "lingcell.json"
	}
	return filepath.Join(d, "lingcell", "config.json")
}

// Load reads one JSON object, applying defaults to omitted fields.
// A missing file returns defaults so Bash can run without a model service.
// Files over 64 KiB, unknown fields, trailing JSON and invalid limits or URLs
// are rejected. Load does not read the credential environment variable.
func Load(path string) (Config, error) {
	cfg := Config{ContextMaxBytes: 128 * 1024, MaxDecisions: 8, TimeoutSeconds: 60, APIKeyEnv: "LINGCELL_API_KEY"}
	d, err := os.UserCacheDir()
	if err == nil {
		cfg.HistoryFile = filepath.Join(d, "lingcell", "history")
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	// Read an extra byte to distinguish an oversized file from valid JSON
	// followed by whitespace; a truncated reader could otherwise accept it.
	data, err := io.ReadAll(io.LimitReader(f, maxConfigSize+1))
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if len(data) > maxConfigSize {
		return cfg, errors.New("config exceeds 64 KiB")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return cfg, errors.New("config must contain one JSON object")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("config: %w", err)
	}
	var limits map[string]json.RawMessage
	if err := json.Unmarshal(data, &limits); err != nil {
		return cfg, fmt.Errorf("config: %w", err)
	}
	for _, name := range []string{"context_max_bytes", "max_decisions", "timeout_seconds"} {
		if raw, ok := limits[name]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return cfg, fmt.Errorf("%s must be an integer", name)
		}
	}
	var extra any
	// One successfully decoded object is insufficient if more input follows.
	if err := dec.Decode(&extra); err != io.EOF {
		return cfg, errors.New("config must contain one JSON object")
	}
	if cfg.ContextMaxBytes < 16*1024 || cfg.ContextMaxBytes > 2*1024*1024 {
		return cfg, errors.New("context_max_bytes must be between 16384 and 2097152")
	}
	if cfg.MaxDecisions < 1 || cfg.MaxDecisions > 64 {
		return cfg, errors.New("max_decisions must be between 1 and 64")
	}
	if cfg.TimeoutSeconds < 1 || cfg.TimeoutSeconds > 600 {
		return cfg, errors.New("timeout_seconds must be between 1 and 600")
	}
	if len(cfg.SkillDirs) > 16 {
		return cfg, errors.New("skill_dirs accepts at most 16 directories")
	}
	for _, dir := range cfg.SkillDirs {
		if !filepath.IsAbs(dir) || strings.ContainsRune(dir, 0) {
			return cfg, errors.New("skill_dirs requires absolute paths without NUL")
		}
	}
	if cfg.BaseURL != "" {
		u, err := url.Parse(cfg.BaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return cfg, errors.New("base_url must be an http(s) URL without credentials, query or fragment")
		}
	}
	return cfg, nil
}

// Ready reports whether the model endpoint and model name are configured.
// It does not verify credentials, connectivity or provider tool-call support.
func (c Config) Ready() bool {
	return strings.TrimSpace(c.BaseURL) != "" && strings.TrimSpace(c.Model) != ""
}
