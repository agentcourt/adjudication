package xproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type XProxyConfig struct {
	Endpoints map[string]XProxyEndpoint `json:"endpoints"`
}

type XProxyEndpoint struct {
	URL              string `json:"url"`
	API              string `json:"api"`
	APIKeyEnv        string `json:"apiKeyEnv"`
	AnthropicVersion string `json:"anthropicVersion"`
	AnthropicBeta    string `json:"anthropicBeta"`
}

type ModelSpec struct {
	Endpoint        string
	ModelIn         string
	ModelOut        string
	SearchRequested bool
	ForceSearch     bool
}

type subagentConfigFile struct {
	Subagent SubagentConfig `json:"subagent"`
}

type SubagentConfig struct {
	DefaultTimeoutSec int `json:"default_timeout_sec"`
	MaxTimeoutSec     int `json:"max_timeout_sec"`
	MaxSpawns         int `json:"max_spawns"`
}

func LoadXProxyConfig(path string) (XProxyConfig, error) {
	var cfg XProxyConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if len(cfg.Endpoints) == 0 {
		return cfg, errors.New("xproxy config requires endpoints object")
	}
	for name, endpoint := range cfg.Endpoints {
		if strings.TrimSpace(name) == "" {
			return cfg, errors.New("xproxy endpoint name must be non-empty string")
		}
		if !strings.HasPrefix(endpoint.URL, "https://") {
			return cfg, fmt.Errorf("xproxy endpoint %s requires https url", name)
		}
		switch endpoint.API {
		case "openai-responses", "anthropic-messages", "gemini-generate-content":
		default:
			return cfg, fmt.Errorf("xproxy endpoint %s api not supported: %s", name, endpoint.API)
		}
		if strings.TrimSpace(endpoint.APIKeyEnv) == "" {
			return cfg, fmt.Errorf("xproxy endpoint %s requires apiKeyEnv", name)
		}
		if endpoint.API == "anthropic-messages" {
			if strings.TrimSpace(endpoint.AnthropicVersion) == "" {
				endpoint.AnthropicVersion = "2023-06-01"
			}
			cfg.Endpoints[name] = endpoint
		}
	}
	return cfg, nil
}

func ParseXProxyModel(model string) (ModelSpec, error) {
	if strings.TrimSpace(model) == "" {
		return ModelSpec{}, errors.New("model must be non-empty string")
	}
	parts := strings.SplitN(model, "://", 2)
	if len(parts) != 2 {
		return ModelSpec{}, errors.New("model must match ENDPOINT://MODEL[?ARGS]")
	}
	endpointName := parts[0]
	rest := parts[1]
	if endpointName == "" {
		return ModelSpec{}, errors.New("missing endpoint name")
	}
	modelName := rest
	rawQuery := ""
	if idx := strings.IndexByte(rest, '?'); idx >= 0 {
		modelName = rest[:idx]
		rawQuery = rest[idx+1:]
	}
	if modelName == "" {
		return ModelSpec{}, errors.New("missing model name")
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return ModelSpec{}, err
	}
	for key, vals := range values {
		if len(vals) > 1 {
			return ModelSpec{}, fmt.Errorf("duplicate query arg: %s", key)
		}
		if key != "tools" {
			return ModelSpec{}, fmt.Errorf("unsupported query arg: %s", key)
		}
		if key == "tools" && vals[0] != "search" {
			return ModelSpec{}, errors.New(`tools must be "search"`)
		}
	}
	searchRequested := values.Get("tools") == "search" || strings.HasSuffix(modelName, ":online")
	modelOut := modelName
	if strings.HasSuffix(modelName, ":online") && (endpointName == "openai" || endpointName == "anthropic" || endpointName == "xai") {
		modelOut = strings.TrimSuffix(modelName, ":online")
	}
	return ModelSpec{
		Endpoint:        endpointName,
		ModelIn:         modelName,
		ModelOut:        modelOut,
		SearchRequested: searchRequested,
		ForceSearch:     values.Get("tools") == "search",
	}, nil
}

func LoadSubagentConfig(path string) (SubagentConfig, error) {
	var cfgFile subagentConfigFile
	data, err := os.ReadFile(path)
	if err != nil {
		return SubagentConfig{}, err
	}
	if err := json.Unmarshal(data, &cfgFile); err != nil {
		return SubagentConfig{}, err
	}
	cfg := cfgFile.Subagent
	if cfg.DefaultTimeoutSec <= 0 {
		return SubagentConfig{}, errors.New("default_timeout_sec must be > 0")
	}
	if cfg.MaxTimeoutSec <= 0 {
		return SubagentConfig{}, errors.New("max_timeout_sec must be > 0")
	}
	if cfg.MaxSpawns <= 0 {
		return SubagentConfig{}, errors.New("max_spawns must be > 0")
	}
	if cfg.DefaultTimeoutSec > cfg.MaxTimeoutSec {
		return SubagentConfig{}, errors.New("default_timeout_sec must be <= max_timeout_sec")
	}
	if envValue, ok, err := positiveIntEnv("PI_SUBAGENT_DEFAULT_TIMEOUT_SECONDS"); err != nil {
		return SubagentConfig{}, err
	} else if ok {
		cfg.DefaultTimeoutSec = envValue
	}
	if envValue, ok, err := positiveIntEnv("PI_SUBAGENT_MAX_TIMEOUT_SECONDS"); err != nil {
		return SubagentConfig{}, err
	} else if ok {
		cfg.MaxTimeoutSec = envValue
	}
	if cfg.DefaultTimeoutSec > cfg.MaxTimeoutSec {
		return SubagentConfig{}, errors.New("default_timeout_sec must be <= max_timeout_sec")
	}
	return cfg, nil
}

func positiveNumberEnv(name string) (float64, bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, false, nil
	}
	var value float64
	_, err := fmt.Sscanf(raw, "%f", &value)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be a number", name)
	}
	if value <= 0 {
		return 0, false, fmt.Errorf("%s must be > 0", name)
	}
	return value, true, nil
}

func positiveIntEnv(name string) (int, bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, false, nil
	}
	var value int
	_, err := fmt.Sscanf(raw, "%d", &value)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be an integer", name)
	}
	if value <= 0 {
		return 0, false, fmt.Errorf("%s must be > 0", name)
	}
	return value, true, nil
}

func ensureDir(path string) error {
	if path == "" {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

func diagPath(root string, parts ...string) string {
	all := append([]string{root, "_diag"}, parts...)
	return filepath.Join(all...)
}
