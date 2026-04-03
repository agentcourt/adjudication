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

func TestPrepareEphemeralPIHome(t *testing.T) {
	repoRoot := t.TempDir()
	etcDir := filepath.Join(repoRoot, "etc")
	if err := os.MkdirAll(etcDir, 0o755); err != nil {
		t.Fatalf("mkdir etc: %v", err)
	}
	settings := []byte("{\"defaultModel\":\"openai://gpt-5\"}\n")
	models := []byte("{\"providers\":{\"xproxy\":{\"models\":[{\"id\":\"openai://gpt-5\",\"name\":\"openai://gpt-5\"}]}}}\n")
	if err := os.WriteFile(filepath.Join(etcDir, "pi-settings.xproxy.json"), settings, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(etcDir, "pi-models.xproxy.json"), models, 0o644); err != nil {
		t.Fatalf("write models: %v", err)
	}

	t.Setenv("ADC_FLASH_XPROXY_MODEL", "openai://gpt-5-mini")
	homeDir, cleanup, err := prepareEphemeralPIHome(repoRoot)
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
	if got := stringOrDefault(settingsObj["defaultModel"], ""); got != "openai://gpt-5-mini" {
		t.Fatalf("defaultModel = %q, want openai://gpt-5-mini", got)
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
	xproxy, _ := providers["xproxy"].(map[string]any)
	modelList, _ := xproxy["models"].([]any)
	found := false
	for _, raw := range modelList {
		model, _ := raw.(map[string]any)
		if stringOrDefault(model["id"], "") == "openai://gpt-5-mini" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("staged model catalog missing flash model: %#v", modelList)
	}

	authRaw, err := os.ReadFile(filepath.Join(homeDir, ".pi", "agent", "auth.json"))
	if err != nil {
		t.Fatalf("read staged auth: %v", err)
	}
	if string(authRaw) != "{}\n" {
		t.Fatalf("auth.json = %q, want {}\n", string(authRaw))
	}

	if err := cleanup(); err != nil && !os.IsNotExist(err) {
		t.Fatalf("cleanup PI home dir: %v", err)
	}
	if _, err := os.Stat(homeDir); !os.IsNotExist(err) {
		t.Fatalf("expected cleanup to remove %s, stat err=%v", homeDir, err)
	}
}
