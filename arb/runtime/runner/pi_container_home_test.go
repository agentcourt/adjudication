package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUsesPIContainerWrapper(t *testing.T) {
	t.Parallel()

	if !usesPIContainerWrapper("/tmp/acp-podman.sh") {
		t.Fatalf("expected acp-podman.sh to be recognized")
	}
	if !usesPIContainerWrapper("pi-podman.sh") {
		t.Fatalf("expected pi-podman.sh to be recognized")
	}
	if usesPIContainerWrapper("/tmp/other-command") {
		t.Fatalf("did not expect unrelated command to be recognized")
	}
}

func TestPrepareEphemeralPIHomeStagesSearchModel(t *testing.T) {
	commonRoot := t.TempDir()
	etcDir := filepath.Join(commonRoot, "etc")
	if err := os.MkdirAll(etcDir, 0o755); err != nil {
		t.Fatalf("mkdir etc: %v", err)
	}
	settings := []byte("{\"defaultProvider\":\"xproxy\",\"defaultModel\":\"openai://gpt-5\"}\n")
	models := []byte("{\"providers\":{\"xproxy\":{\"models\":[{\"id\":\"openai://gpt-5\",\"name\":\"OpenAI GPT-5\"}]}}}\n")
	if err := os.WriteFile(filepath.Join(etcDir, "pi-settings.xproxy.json"), settings, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(etcDir, "pi-models.xproxy.json"), models, 0o644); err != nil {
		t.Fatalf("write models: %v", err)
	}
	instructionsPath := filepath.Join(t.TempDir(), "default.md")
	instructionsText := "Attorney instructions.\nFile the narrowest truthful submission.\n"
	if err := os.WriteFile(instructionsPath, []byte(instructionsText), 0o644); err != nil {
		t.Fatalf("write instructions: %v", err)
	}

	homeDir, cleanup, err := prepareEphemeralPIHome(commonRoot, DefaultAttorneyModel, instructionsPath)
	if err != nil {
		t.Fatalf("prepareEphemeralPIHome returned error: %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil && !os.IsNotExist(err) {
			t.Fatalf("cleanup PI home dir: %v", err)
		}
	}()

	settingsRaw, err := os.ReadFile(filepath.Join(homeDir, ".pi", "agent", "settings.json"))
	if err != nil {
		t.Fatalf("read staged settings: %v", err)
	}
	var settingsObj map[string]any
	if err := json.Unmarshal(settingsRaw, &settingsObj); err != nil {
		t.Fatalf("parse staged settings: %v", err)
	}
	if got := stringValueOrDefault(settingsObj["defaultModel"], ""); got != DefaultAttorneyModel {
		t.Fatalf("defaultModel = %q, want %q", got, DefaultAttorneyModel)
	}

	modelsRaw, err := os.ReadFile(filepath.Join(homeDir, ".pi", "agent", "models.json"))
	if err != nil {
		t.Fatalf("read staged models: %v", err)
	}
	var modelsObj map[string]any
	if err := json.Unmarshal(modelsRaw, &modelsObj); err != nil {
		t.Fatalf("parse staged models: %v", err)
	}
	providers, _ := modelsObj["providers"].(map[string]any)
	xproxyProvider, _ := providers["xproxy"].(map[string]any)
	modelList, _ := xproxyProvider["models"].([]any)
	found := false
	for _, raw := range modelList {
		entry, _ := raw.(map[string]any)
		if stringValueOrDefault(entry["id"], "") == DefaultAttorneyModel {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("staged model catalog missing search model: %#v", modelList)
	}

	authRaw, err := os.ReadFile(filepath.Join(homeDir, ".pi", "agent", "auth.json"))
	if err != nil {
		t.Fatalf("read staged auth: %v", err)
	}
	if string(authRaw) != "{}\n" {
		t.Fatalf("auth.json = %q, want {}\n", string(authRaw))
	}
	instructionsRaw, err := os.ReadFile(filepath.Join(homeDir, ".pi", "agent", "attorney-instructions.md"))
	if err != nil {
		t.Fatalf("read staged instructions: %v", err)
	}
	if string(instructionsRaw) != instructionsText {
		t.Fatalf("attorney instructions = %q, want %q", string(instructionsRaw), instructionsText)
	}
}

func TestPrepareEphemeralPIHomeStagesExplicitNonSearchModel(t *testing.T) {
	commonRoot := t.TempDir()
	etcDir := filepath.Join(commonRoot, "etc")
	if err := os.MkdirAll(etcDir, 0o755); err != nil {
		t.Fatalf("mkdir etc: %v", err)
	}
	settings := []byte("{\"defaultProvider\":\"xproxy\",\"defaultModel\":\"openai://gpt-5?tools=search\"}\n")
	models := []byte("{\"providers\":{\"xproxy\":{\"models\":[{\"id\":\"openai://gpt-5?tools=search\",\"name\":\"OpenAI GPT-5 Search\"}]}}}\n")
	if err := os.WriteFile(filepath.Join(etcDir, "pi-settings.xproxy.json"), settings, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(etcDir, "pi-models.xproxy.json"), models, 0o644); err != nil {
		t.Fatalf("write models: %v", err)
	}

	homeDir, cleanup, err := prepareEphemeralPIHome(commonRoot, "openai://gpt-5", "")
	if err != nil {
		t.Fatalf("prepareEphemeralPIHome returned error: %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil && !os.IsNotExist(err) {
			t.Fatalf("cleanup PI home dir: %v", err)
		}
	}()

	settingsRaw, err := os.ReadFile(filepath.Join(homeDir, ".pi", "agent", "settings.json"))
	if err != nil {
		t.Fatalf("read staged settings: %v", err)
	}
	var settingsObj map[string]any
	if err := json.Unmarshal(settingsRaw, &settingsObj); err != nil {
		t.Fatalf("parse staged settings: %v", err)
	}
	if got := stringValueOrDefault(settingsObj["defaultModel"], ""); got != "openai://gpt-5" {
		t.Fatalf("defaultModel = %q, want openai://gpt-5", got)
	}
}

func TestPrepareEphemeralPIHomeAppliesExplicitXProxyBaseURL(t *testing.T) {
	commonRoot := t.TempDir()
	etcDir := filepath.Join(commonRoot, "etc")
	if err := os.MkdirAll(etcDir, 0o755); err != nil {
		t.Fatalf("mkdir etc: %v", err)
	}
	settings := []byte("{\"defaultProvider\":\"xproxy\",\"defaultModel\":\"openai://gpt-5\"}\n")
	models := []byte("{\"providers\":{\"xproxy\":{\"baseUrl\":\"http://127.0.0.1:18459/v1\",\"models\":[{\"id\":\"openai://gpt-5\",\"name\":\"OpenAI GPT-5\"}]}}}\n")
	if err := os.WriteFile(filepath.Join(etcDir, "pi-settings.xproxy.json"), settings, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(etcDir, "pi-models.xproxy.json"), models, 0o644); err != nil {
		t.Fatalf("write models: %v", err)
	}

	const wantBaseURL = "http://host.containers.internal:18459/v1"
	t.Setenv(piXProxyBaseURLEnvVar, wantBaseURL)

	homeDir, cleanup, err := prepareEphemeralPIHome(commonRoot, "openai://gpt-5", "")
	if err != nil {
		t.Fatalf("prepareEphemeralPIHome returned error: %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil && !os.IsNotExist(err) {
			t.Fatalf("cleanup PI home dir: %v", err)
		}
	}()

	modelsRaw, err := os.ReadFile(filepath.Join(homeDir, ".pi", "agent", "models.json"))
	if err != nil {
		t.Fatalf("read staged models: %v", err)
	}
	var modelsObj map[string]any
	if err := json.Unmarshal(modelsRaw, &modelsObj); err != nil {
		t.Fatalf("parse staged models: %v", err)
	}
	providers, _ := modelsObj["providers"].(map[string]any)
	xproxyProvider, _ := providers["xproxy"].(map[string]any)
	if got := stringValueOrDefault(xproxyProvider["baseUrl"], ""); got != wantBaseURL {
		t.Fatalf("baseUrl = %q, want %q", got, wantBaseURL)
	}
}
