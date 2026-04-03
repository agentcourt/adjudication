package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"adjudication/adc/runtime/runner"
	"adjudication/adc/runtime/spec"
	"adjudication/common/xproxy"
)

const (
	defaultACPTimeoutSeconds = runner.DefaultACPTimeoutSeconds
	defaultLLMTimeoutSeconds = runner.DefaultLLMTimeoutSeconds
)

type flashModelOverride struct {
	Direct string
	XProxy string
}

func newFlagSet(name string, stderr io.Writer, usage func()) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = usage
	return fs
}

func writeJSONFile(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func parseOptionalFloat(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var parsed float64
	if _, err := fmt.Sscanf(raw, "%f", &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(strings.TrimSpace(path))
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", path, err)
	}
	return nil
}

func defaultEngineCommand() string {
	return ".bin/adcengine"
}

func defaultCommonRoot() string {
	cwd, err := os.Getwd()
	if err == nil {
		return locateCommonRootFrom(cwd)
	}
	return filepath.FromSlash("../common")
}

func defaultCommonPath(parts ...string) string {
	return filepath.Join(append([]string{defaultCommonRoot()}, parts...)...)
}

func defaultCommonPathFrom(baseDir string, parts ...string) string {
	return filepath.Join(append([]string{locateCommonRootFrom(baseDir)}, parts...)...)
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if fileExists(path) {
			return path
		}
	}
	return ""
}

func defaultPersonaRecordsPathFor(baseDir string) string {
	return resolveDefault(
		firstExistingPath(defaultCommonPathFrom(baseDir, "etc", "personas.csv"), filepath.Join(baseDir, "etc", "personas.csv")),
		defaultCommonPathFrom(baseDir, "etc", "personas.csv"),
	)
}

func defaultPersonaRecordsPath() string {
	cwd, err := os.Getwd()
	if err == nil {
		return defaultPersonaRecordsPathFor(cwd)
	}
	return defaultCommonPath("etc", "personas.csv")
}

func defaultXProxyConfigPath() string {
	return resolveDefault(
		firstExistingPath(defaultCommonPath("etc", "xproxy.json"), "etc/xproxy.json"),
		defaultCommonPath("etc", "xproxy.json"),
	)
}

func defaultACPServerPath() string {
	return resolveDefault(
		firstExistingPath(defaultCommonPath("pi-container", "acp-podman.sh"), "pi-container/acp-podman.sh"),
		defaultCommonPath("pi-container", "acp-podman.sh"),
	)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func locateCommonRootFrom(start string) string {
	base := filepath.Clean(strings.TrimSpace(start))
	if base == "" {
		return filepath.FromSlash("../common")
	}
	if !filepath.IsAbs(base) {
		if absBase, err := filepath.Abs(base); err == nil {
			base = absBase
		}
	}
	for {
		candidate := filepath.Join(base, "common")
		if fileExists(filepath.Join(candidate, "etc", "xproxy.json")) || fileExists(filepath.Join(candidate, "etc", "personas.csv")) {
			return candidate
		}
		if filepath.Base(base) == "common" && (fileExists(filepath.Join(base, "etc", "xproxy.json")) || fileExists(filepath.Join(base, "etc", "personas.csv"))) {
			return base
		}
		next := filepath.Dir(base)
		if next == base {
			break
		}
		base = next
	}
	return filepath.Clean(filepath.Join(start, filepath.FromSlash("../common")))
}

func resolveDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func parseFlashModel(raw string) (flashModelOverride, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return flashModelOverride{}, nil
	}
	switch value {
	case "gpt-5-mini", "openai://gpt-5-mini":
		return flashModelOverride{
			Direct: "gpt-5-mini",
			XProxy: "openai://gpt-5-mini",
		}, nil
	default:
		return flashModelOverride{}, fmt.Errorf(`--flash must be "gpt-5-mini" or "openai://gpt-5-mini"`)
	}
}

func normalizeXProxyModel(model string) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", nil
	}
	if strings.Contains(model, "://") {
		if _, err := xproxy.ParseXProxyModel(model); err != nil {
			return "", err
		}
		return model, nil
	}
	normalized := "openai://" + model
	if _, err := xproxy.ParseXProxyModel(normalized); err != nil {
		return "", err
	}
	return normalized, nil
}

func normalizeScenarioModelsForXProxy(scenario spec.FormalScenario) (spec.FormalScenario, error) {
	var err error
	scenario.Model, err = normalizeXProxyModel(scenario.Model)
	if err != nil {
		return spec.FormalScenario{}, fmt.Errorf("normalize scenario model: %w", err)
	}
	for i := range scenario.Roles {
		if strings.TrimSpace(scenario.Roles[i].Model) == "" {
			continue
		}
		scenario.Roles[i].Model, err = normalizeXProxyModel(scenario.Roles[i].Model)
		if err != nil {
			return spec.FormalScenario{}, fmt.Errorf("normalize role %s model: %w", scenario.Roles[i].Name, err)
		}
	}
	return scenario, nil
}
