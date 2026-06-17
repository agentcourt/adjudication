package localrun

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"adjudication/adc/runtime/runner"
	"adjudication/common/modelrequest"
)

func TestMCPURLIncludesCaseRoleAndPrincipal(t *testing.T) {
	t.Parallel()

	raw := mcpURL("http://127.0.0.1:8001/", "case 1", map[string]string{
		"role_id":      "juror",
		"principal_id": "J1",
	})
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse MCP URL: %v", err)
	}
	if parsed.Scheme != "http" || parsed.Host != "127.0.0.1:8001" || parsed.Path != "/mcp" {
		t.Fatalf("URL = %q", raw)
	}
	values := parsed.Query()
	if values.Get("case_id") != "case 1" {
		t.Fatalf("case_id = %q", values.Get("case_id"))
	}
	if values.Get("role_id") != "juror" {
		t.Fatalf("role_id = %q", values.Get("role_id"))
	}
	if values.Get("principal_id") != "J1" {
		t.Fatalf("principal_id = %q", values.Get("principal_id"))
	}
}

func TestAutoLawyerRoles(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"both":      {"plaintiff", "defendant"},
		"plaintiff": {"plaintiff"},
		"defendant": {"defendant"},
	}
	for mode, want := range cases {
		got, err := autoLawyerRoles(mode)
		if err != nil {
			t.Fatalf("autoLawyerRoles(%q): %v", mode, err)
		}
		if len(got) != len(want) {
			t.Fatalf("autoLawyerRoles(%q) len = %d, want %d", mode, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("autoLawyerRoles(%q)[%d] = %q, want %q", mode, i, got[i], want[i])
			}
		}
	}
	if _, err := autoLawyerRoles("none"); err == nil {
		t.Fatalf("autoLawyerRoles accepted invalid mode")
	}
}

func TestOpenClawCodexAuthCommandImportsAccessToken(t *testing.T) {
	t.Parallel()

	cmd := openClawCodexAuthCommand()
	for _, want := range []string{
		"unset OPENAI_API_KEY",
		"CODEX_HOME",
		"auth.json",
		"tokens.access_token",
		"openclaw models auth paste-token --provider openai --profile-id openai:codex",
		"unset codex_token",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("auth command missing %q:\n%s", want, cmd)
		}
	}
}

func TestStageOpenClawCodexAuthUsesContainerReadableModes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	authPath := filepath.Join(root, "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"tokens":{"access_token":"token-1"}}`), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	state := &runState{
		opts: Options{
			OutputDir: root,
		},
		openClawAuth: openClawAuthConfig{
			Mode:          "codex",
			CodexAuthPath: authPath,
		},
	}
	home, err := state.stageOpenClawCodexAuth("plaintiff")
	if err != nil {
		t.Fatalf("stage auth: %v", err)
	}
	homeInfo, err := os.Stat(home)
	if err != nil {
		t.Fatalf("stat home: %v", err)
	}
	if got := homeInfo.Mode().Perm(); got != 0o777 {
		t.Fatalf("home mode = %o, want 777", got)
	}
	authInfo, err := os.Stat(filepath.Join(home, "auth.json"))
	if err != nil {
		t.Fatalf("stat staged auth: %v", err)
	}
	if got := authInfo.Mode().Perm(); got != 0o666 {
		t.Fatalf("auth mode = %o, want 666", got)
	}
}

func TestIsConnectionRefused(t *testing.T) {
	t.Parallel()

	err := &url.Error{
		Op:  "Get",
		URL: "http://127.0.0.1:1/roleapi/v1/status",
		Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED},
	}
	if !isConnectionRefused(err) {
		t.Fatalf("isConnectionRefused returned false for ECONNREFUSED")
	}
	if isConnectionRefused(os.ErrNotExist) {
		t.Fatalf("isConnectionRefused returned true for unrelated error")
	}
}

func TestWritePiConfigUsesFullOpenRouterSpec(t *testing.T) {
	t.Parallel()

	spec, err := modelrequest.ParseJSON([]byte(`{
		"endpoint":"openrouter",
		"model":"anthropic/claude-3.5-sonnet",
		"provider":{"only":["deepinfra"],"allow_fallbacks":false,"require_parameters":true,"quantizations":["bf16"]},
		"persona":"personas/j1.txt"
	}`))
	if err != nil {
		t.Fatalf("parse request spec: %v", err)
	}
	home := t.TempDir()
	model, err := writePiConfig(home, activeJurorOpportunity{
		principalID: "J1",
		requestSpec: &spec,
	}, "adc", "http://host/mcp?case_id=c&role_id=juror&principal_id=J1", "token-1")
	if err != nil {
		t.Fatalf("writePiConfig: %v", err)
	}
	if model != "anthropic/claude-3.5-sonnet" {
		t.Fatalf("model = %q", model)
	}

	models := readJSONMap(t, filepath.Join(home, ".pi", "agent", "models.json"))
	openrouter := models["providers"].(map[string]any)["openrouter"].(map[string]any)
	entries := openrouter["models"].([]any)
	entry := entries[0].(map[string]any)
	if entry["maxTokens"].(float64) != float64(runner.DefaultJurorMaxOutputTokens) {
		t.Fatalf("maxTokens = %#v", entry["maxTokens"])
	}
	routing := entry["compat"].(map[string]any)["openRouterRouting"].(map[string]any)
	if routing["allow_fallbacks"] != false {
		t.Fatalf("allow_fallbacks = %#v", routing["allow_fallbacks"])
	}
	if routing["require_parameters"] != true {
		t.Fatalf("require_parameters = %#v", routing["require_parameters"])
	}
	if routing["only"].([]any)[0].(string) != "deepinfra" {
		t.Fatalf("provider.only = %#v", routing["only"])
	}
	if routing["quantizations"].([]any)[0].(string) != "bf16" {
		t.Fatalf("provider.quantizations = %#v", routing["quantizations"])
	}

	mcp := readJSONMap(t, filepath.Join(home, ".mcp.json"))
	server := mcp["mcpServers"].(map[string]any)["adc"].(map[string]any)
	if server["transport"].(string) != "streamable-http" {
		t.Fatalf("MCP transport = %#v", server["transport"])
	}
	if server["headers"].(map[string]any)["Authorization"].(string) != "Bearer token-1" {
		t.Fatalf("MCP auth header = %#v", server["headers"])
	}
}

func TestJurorInstructionsStopAfterActiveOpportunity(t *testing.T) {
	t.Parallel()

	text, err := renderInstructions(DefaultJurorInstructionsPath(), instructionData{
		CaseID:           "case-1",
		PrincipalID:      "J2",
		OpportunityID:    "opp-1",
		OpportunityPhase: "deliberation",
		MCPServer:        "adc",
	})
	if err != nil {
		t.Fatalf("renderInstructions: %v", err)
	}
	for _, want := range []string{
		"opportunity opp-1 in phase deliberation",
		"After `adc_submit_decision` returns `ok: true`, stop.",
		"Do not wait for another juror opportunity.",
		"ADC will start a new Pi process if juror J2 later receives another opportunity.",
		"the prompt includes the trial transcript from openings through closings",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("juror instructions missing %q:\n%s", want, text)
		}
	}
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return out
}
