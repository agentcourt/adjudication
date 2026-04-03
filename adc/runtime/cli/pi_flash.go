package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

func overridePISettingsDefaultModel(raw []byte, flashModel string) ([]byte, error) {
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

func ensurePIModelCatalog(raw []byte, flashModel string) ([]byte, error) {
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
		if strings.TrimSpace(stringValue(model["id"])) == flashModel {
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
