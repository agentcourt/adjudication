package xproxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJSONFile(t *testing.T, payload any) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	return path
}

func TestParseXProxyModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		model         string
		wantEndpoint  string
		wantModelOut  string
		wantSearch    bool
		wantForce     bool
		wantErrSubstr string
	}{
		{
			name:         "basic",
			model:        "openrouter://openai/gpt-5",
			wantEndpoint: "openrouter",
			wantModelOut: "openai/gpt-5",
		},
		{
			name:         "force search",
			model:        "openrouter://openai/gpt-5?tools=search",
			wantEndpoint: "openrouter",
			wantModelOut: "openai/gpt-5",
			wantSearch:   true,
			wantForce:    true,
		},
		{
			name:         "openai online suffix",
			model:        "openai://gpt-5:online",
			wantEndpoint: "openai",
			wantModelOut: "gpt-5",
			wantSearch:   true,
		},
		{
			name:         "openrouter online suffix preserved",
			model:        "openrouter://openai/gpt-5:online",
			wantEndpoint: "openrouter",
			wantModelOut: "openai/gpt-5:online",
			wantSearch:   true,
		},
		{name: "missing separator", model: "openai/gpt-5", wantErrSubstr: "ENDPOINT://MODEL"},
		{name: "duplicate arg", model: "openai://gpt-5?tools=search&tools=search", wantErrSubstr: "duplicate query arg"},
		{name: "bad arg", model: "openai://gpt-5?mode=fast", wantErrSubstr: "unsupported query arg"},
		{name: "bad tools value", model: "openai://gpt-5?tools=web", wantErrSubstr: `tools must be "search"`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseXProxyModel(tt.model)
			if tt.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("ParseXProxyModel(%q) error = nil, want %q", tt.model, tt.wantErrSubstr)
				}
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("ParseXProxyModel(%q) error = %v, want substring %q", tt.model, err, tt.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseXProxyModel(%q) error = %v", tt.model, err)
			}
			if got.Endpoint != tt.wantEndpoint || got.ModelOut != tt.wantModelOut || got.SearchRequested != tt.wantSearch || got.ForceSearch != tt.wantForce {
				t.Fatalf("ParseXProxyModel(%q) = %+v", tt.model, got)
			}
		})
	}
}

func TestLoadXProxyConfig(t *testing.T) {
	t.Parallel()

	path := writeJSONFile(t, map[string]any{
		"endpoints": map[string]any{
			"openai": map[string]any{
				"url":       "https://api.openai.com/v1/responses",
				"api":       "openai-responses",
				"apiKeyEnv": "OPENAI_API_KEY",
			},
			"anthropic": map[string]any{
				"url":       "https://api.anthropic.com/v1/messages",
				"api":       "anthropic-messages",
				"apiKeyEnv": "ANTHROPIC_API_KEY",
			},
		},
	})

	cfg, err := LoadXProxyConfig(path)
	if err != nil {
		t.Fatalf("LoadXProxyConfig error = %v", err)
	}
	if cfg.Endpoints["anthropic"].AnthropicVersion != "2023-06-01" {
		t.Fatalf("AnthropicVersion = %q", cfg.Endpoints["anthropic"].AnthropicVersion)
	}

	tests := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{
			name:    "missing endpoints",
			payload: map[string]any{},
			want:    "requires endpoints object",
		},
		{
			name: "non-https url",
			payload: map[string]any{"endpoints": map[string]any{
				"openai": map[string]any{"url": "http://example.com", "api": "openai-responses", "apiKeyEnv": "OPENAI_API_KEY"},
			}},
			want: "requires https url",
		},
		{
			name: "unsupported api",
			payload: map[string]any{"endpoints": map[string]any{
				"openai": map[string]any{"url": "https://example.com", "api": "chat-completions", "apiKeyEnv": "OPENAI_API_KEY"},
			}},
			want: "api not supported",
		},
		{
			name: "missing api key env",
			payload: map[string]any{"endpoints": map[string]any{
				"openai": map[string]any{"url": "https://example.com", "api": "openai-responses"},
			}},
			want: "requires apiKeyEnv",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := LoadXProxyConfig(writeJSONFile(t, tt.payload))
			if err == nil {
				t.Fatalf("LoadXProxyConfig error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadXProxyConfig error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestLoadSubagentConfig(t *testing.T) {
	t.Setenv("PI_SUBAGENT_DEFAULT_TIMEOUT_SECONDS", "")
	t.Setenv("PI_SUBAGENT_MAX_TIMEOUT_SECONDS", "")

	path := writeJSONFile(t, map[string]any{
		"subagent": map[string]any{
			"default_timeout_sec": 30,
			"max_timeout_sec":     90,
			"max_spawns":          4,
		},
	})

	cfg, err := LoadSubagentConfig(path)
	if err != nil {
		t.Fatalf("LoadSubagentConfig error = %v", err)
	}
	if cfg.DefaultTimeoutSec != 30 || cfg.MaxTimeoutSec != 90 || cfg.MaxSpawns != 4 {
		t.Fatalf("cfg = %+v", cfg)
	}

	t.Setenv("PI_SUBAGENT_DEFAULT_TIMEOUT_SECONDS", "45")
	t.Setenv("PI_SUBAGENT_MAX_TIMEOUT_SECONDS", "120")
	cfg, err = LoadSubagentConfig(path)
	if err != nil {
		t.Fatalf("LoadSubagentConfig override error = %v", err)
	}
	if cfg.DefaultTimeoutSec != 45 || cfg.MaxTimeoutSec != 120 {
		t.Fatalf("override cfg = %+v", cfg)
	}

	t.Setenv("PI_SUBAGENT_DEFAULT_TIMEOUT_SECONDS", "abc")
	if _, err := LoadSubagentConfig(path); err == nil {
		t.Fatalf("LoadSubagentConfig invalid env error = nil")
	}
}
