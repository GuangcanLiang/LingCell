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
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	APIKeyEnv      string `json:"api_key_env"`
	MaxDecisions   int    `json:"max_decisions"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	HistoryFile    string `json:"history_file"`
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
func Load(path string) (Config, error) {
	cfg := Config{MaxDecisions: 8, TimeoutSeconds: 60, APIKeyEnv: "LINGCELL_API_KEY"}
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
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return cfg, errors.New("config must contain one JSON object")
	}
	if cfg.MaxDecisions < 1 || cfg.MaxDecisions > 64 {
		return cfg, errors.New("max_decisions must be between 1 and 64")
	}
	if cfg.TimeoutSeconds < 1 || cfg.TimeoutSeconds > 600 {
		return cfg, errors.New("timeout_seconds must be between 1 and 600")
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
func (c Config) Ready() bool {
	return strings.TrimSpace(c.BaseURL) != "" && strings.TrimSpace(c.Model) != ""
}
