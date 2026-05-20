package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"adjudication/common/xproxy"
)

const stagedACPRoleInstructionsPath = "/home/user/.pi/agent/adc-role-instructions.md"

func UsesPIContainerWrapper(command string) bool {
	base := strings.TrimSpace(filepath.Base(command))
	return base == "acp-podman.sh" || base == "pi-podman.sh"
}

func PrepareEphemeralPIHome(commonRoot string, model string, instructions string) (string, func() error, error) {
	commonRoot = strings.TrimSpace(commonRoot)
	if commonRoot == "" {
		return "", nil, fmt.Errorf("common root is required")
	}
	homeDir, err := os.MkdirTemp("", "agentcourt-pi-home-")
	if err != nil {
		return "", nil, fmt.Errorf("create PI home dir: %w", err)
	}
	cleanup := func() error {
		return os.RemoveAll(homeDir)
	}
	fail := func(err error) (string, func() error, error) {
		cleanupErr := cleanup()
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove PI home dir: %w", cleanupErr))
		}
		return "", nil, err
	}
	agentDir := filepath.Join(homeDir, ".pi", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return fail(fmt.Errorf("create PI agent dir: %w", err))
	}
	piModel, err := effectiveACPPIModel(model)
	if err != nil {
		return fail(err)
	}
	for _, file := range []struct {
		src string
		dst string
	}{
		{
			src: filepath.Join(commonRoot, "etc", "pi-settings.xproxy.json"),
			dst: filepath.Join(agentDir, "settings.json"),
		},
		{
			src: filepath.Join(commonRoot, "etc", "pi-models.xproxy.json"),
			dst: filepath.Join(agentDir, "models.json"),
		},
	} {
		raw, err := os.ReadFile(file.src)
		if err != nil {
			return fail(fmt.Errorf("read %s: %w", file.src, err))
		}
		switch filepath.Base(file.dst) {
		case "settings.json":
			raw, err = overridePISettingsDefaultModelForRunner(raw, piModel)
			if err != nil {
				return fail(err)
			}
		case "models.json":
			raw, err = ensurePIModelCatalogForRunner(raw, piModel)
			if err != nil {
				return fail(err)
			}
		}
		if err := os.WriteFile(file.dst, raw, 0o644); err != nil {
			return fail(fmt.Errorf("write %s: %w", file.dst, err))
		}
	}
	authPath := filepath.Join(agentDir, "auth.json")
	if err := os.WriteFile(authPath, []byte("{}\n"), 0o644); err != nil {
		return fail(fmt.Errorf("write %s: %w", authPath, err))
	}
	if strings.TrimSpace(instructions) != "" {
		targetPath := filepath.Join(agentDir, "adc-role-instructions.md")
		if err := os.WriteFile(targetPath, []byte(strings.TrimSpace(instructions)+"\n"), 0o644); err != nil {
			return fail(fmt.Errorf("write %s: %w", targetPath, err))
		}
	}
	return homeDir, cleanup, nil
}

func usesPIContainerWrapper(command string) bool {
	return UsesPIContainerWrapper(command)
}

func prepareEphemeralPIHome(commonRoot string, model string, instructions string) (string, func() error, error) {
	return PrepareEphemeralPIHome(commonRoot, model, instructions)
}

func effectiveACPPIModel(model string) (string, error) {
	if envModel := strings.TrimSpace(os.Getenv("ADC_FLASH_XPROXY_MODEL")); envModel != "" {
		if _, err := xproxy.ParseXProxyModel(envModel); err != nil {
			return "", fmt.Errorf("parse ADC_FLASH_XPROXY_MODEL: %w", err)
		}
		return envModel, nil
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return "", nil
	}
	if !strings.Contains(model, "://") {
		model = "openai://" + model
	}
	if _, err := xproxy.ParseXProxyModel(model); err != nil {
		return "", fmt.Errorf("parse ACP role model %q: %w", model, err)
	}
	return model, nil
}

func overridePISettingsDefaultModelForRunner(raw []byte, flashModel string) ([]byte, error) {
	flashModel = strings.TrimSpace(flashModel)
	if flashModel == "" {
		return raw, nil
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("parse pi settings: %w", err)
	}
	settings["defaultModel"] = flashModel
	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal pi settings: %w", err)
	}
	return append(updated, '\n'), nil
}

func ensurePIModelCatalogForRunner(raw []byte, flashModel string) ([]byte, error) {
	flashModel = strings.TrimSpace(flashModel)
	if flashModel == "" {
		return raw, nil
	}
	var catalog map[string]any
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return nil, fmt.Errorf("parse pi model catalog: %w", err)
	}
	providers, _ := catalog["providers"].(map[string]any)
	if providers == nil {
		return nil, fmt.Errorf("pi model catalog missing providers")
	}
	xproxyProvider, _ := providers["xproxy"].(map[string]any)
	if xproxyProvider == nil {
		return nil, fmt.Errorf("pi model catalog missing xproxy provider")
	}
	models, _ := xproxyProvider["models"].([]any)
	for _, rawModel := range models {
		model, _ := rawModel.(map[string]any)
		if strings.TrimSpace(stringOrDefault(model["id"], "")) == flashModel {
			updated, err := json.MarshalIndent(catalog, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshal pi model catalog: %w", err)
			}
			return append(updated, '\n'), nil
		}
	}
	models = append(models, map[string]any{
		"id":   flashModel,
		"name": flashModel,
	})
	xproxyProvider["models"] = models
	providers["xproxy"] = xproxyProvider
	catalog["providers"] = providers
	updated, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal pi model catalog: %w", err)
	}
	return append(updated, '\n'), nil
}
