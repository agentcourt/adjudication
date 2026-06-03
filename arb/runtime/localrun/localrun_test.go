package localrun

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"adjudication/arb/runtime/proceeding"
	"adjudication/common/modelrequest"
)

func TestRenderInstructionsUsesTemplateData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lawyer.md.tmpl")
	if err := os.WriteFile(path, []byte("case={{.CaseID}} role={{.RoleID}} server={{.MCPServer}} url={{.MCPURL}}\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	got, err := renderInstructions(path, instructionData{
		CaseID:    "case-1",
		RoleID:    "plaintiff",
		MCPServer: "aar-case-1-plaintiff",
		MCPURL:    "http://example/mcp",
	})
	if err != nil {
		t.Fatalf("render instructions: %v", err)
	}
	for _, want := range []string{"case=case-1", "role=plaintiff", "server=aar-case-1-plaintiff", "url=http://example/mcp"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered instructions missing %q: %s", want, got)
		}
	}
}

func TestRenderInstructionsRejectsMissingTemplateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.md.tmpl")
	if err := os.WriteFile(path, []byte("{{.Missing}}\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	_, err := renderInstructions(path, instructionData{CaseID: "case-1"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestPiCouncilInstructionsUseProxyToolNames(t *testing.T) {
	path := filepath.Join("..", "..", "agent-instructions", "pi-council.md.tmpl")
	got, err := renderInstructions(path, instructionData{
		CaseID:    "case-1",
		MemberID:  "C1",
		MCPServer: defaultPiMCPServer,
		MCPURL:    "http://127.0.0.1:19780/mcp?case_id=case-1&member_id=C1",
	})
	if err != nil {
		t.Fatalf("render instructions: %v", err)
	}
	for _, want := range []string{"aar_wait_for_opportunity", "aar_get_case", "aar_list_evidence", "aar_stat_evidence", "aar_read_evidence_range", "aar_submit_council_vote"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered instructions missing %q:\n%s", want, got)
		}
	}
}

func TestRemoteLawyerSkillIncludesConnectionAndWorkLoop(t *testing.T) {
	path := filepath.Join("..", "..", "agent-instructions", "openclaw-remote-lawyer-skill.md.tmpl")
	mcpJSON := `{"url":"http://aar.example:8001/mcp?case_id=case-1&role_id=plaintiff","transport":"streamable-http","headers":{"Authorization":"Bearer token-1"}}`
	got, err := renderInstructions(path, instructionData{
		CaseID:    "case-1",
		RoleID:    "plaintiff",
		MCPServer: "aar-case-1-plaintiff",
		MCPURL:    "http://aar.example:8001/mcp?case_id=case-1&role_id=plaintiff",
		MCPJSON:   mcpJSON,
	})
	if err != nil {
		t.Fatalf("render instructions: %v", err)
	}
	for _, want := range []string{"openclaw mcp set", "aar-case-1-plaintiff", "Bearer token-1", "wait_for_opportunity", "send_work_notes", "submit_evidence", "submit_decision"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered skill missing %q:\n%s", want, got)
		}
	}
}

func TestWritePiConfigFromRosterEntry(t *testing.T) {
	maxTokens := int64(1234)
	allowFallbacks := false
	home := t.TempDir()
	model, err := writePiConfig(home, councilRosterEntry{
		MemberID: "C1",
		RequestSpec: &modelrequest.Spec{
			Endpoint: "openrouter",
			Model:    "anthropic/claude-sonnet-4",
			Provider: &modelrequest.ProviderConstraints{
				Only:           []string{"anthropic"},
				AllowFallbacks: &allowFallbacks,
				Quantizations:  []string{"bf16"},
			},
			Request: modelrequest.RequestParameters{MaxOutputTokens: &maxTokens},
		},
	}, "aar-case-C1", "http://127.0.0.1:19780/mcp?case_id=case-1&member_id=C1", "token-1")
	if err != nil {
		t.Fatalf("write Pi config: %v", err)
	}
	if model != "anthropic/claude-sonnet-4" {
		t.Fatalf("model = %q", model)
	}
	settings := readJSONMap(t, filepath.Join(home, ".pi", "agent", "settings.json"))
	if settings["defaultProvider"] != "openrouter" || settings["defaultModel"] != "anthropic/claude-sonnet-4" {
		t.Fatalf("settings = %#v", settings)
	}
	models := readJSONMap(t, filepath.Join(home, ".pi", "agent", "models.json"))
	providers := models["providers"].(map[string]any)
	openrouter := providers["openrouter"].(map[string]any)
	modelList := openrouter["models"].([]any)
	modelEntry := modelList[0].(map[string]any)
	if modelEntry["maxTokens"] != float64(maxTokens) {
		t.Fatalf("model entry maxTokens = %#v", modelEntry["maxTokens"])
	}
	compat := modelEntry["compat"].(map[string]any)
	routing := compat["openRouterRouting"].(map[string]any)
	if routing["allow_fallbacks"] != false {
		t.Fatalf("routing = %#v", routing)
	}
	quantizations := routing["quantizations"].([]any)
	if len(quantizations) != 1 || quantizations[0] != "bf16" {
		t.Fatalf("routing quantizations = %#v", routing["quantizations"])
	}
	mcpConfig := readJSONMap(t, filepath.Join(home, ".mcp.json"))
	servers := mcpConfig["mcpServers"].(map[string]any)
	server := servers["aar-case-C1"].(map[string]any)
	if server["url"] != "http://127.0.0.1:19780/mcp?case_id=case-1&member_id=C1" {
		t.Fatalf("mcp server = %#v", server)
	}
	headers := server["headers"].(map[string]any)
	if headers["Authorization"] != "Bearer token-1" {
		t.Fatalf("headers = %#v", headers)
	}
}

func TestWritePiConfigRejectsMissingRequestSpec(t *testing.T) {
	_, err := writePiConfig(t.TempDir(), councilRosterEntry{
		MemberID: "C1",
		Model:    "openrouter://anthropic/claude-sonnet-4",
	}, "server", "http://example/mcp", "token")
	if err == nil || !strings.Contains(err.Error(), "request_spec") {
		t.Fatalf("error = %v", err)
	}
}

func TestWritePiConfigRejectsUnsupportedRequestFields(t *testing.T) {
	temperature := 0.2
	_, err := writePiConfig(t.TempDir(), councilRosterEntry{
		MemberID: "C1",
		RequestSpec: &modelrequest.Spec{
			Endpoint: "openrouter",
			Model:    "model",
			Request:  modelrequest.RequestParameters{Temperature: &temperature},
		},
	}, "server", "http://example/mcp", "token")
	if err == nil || !strings.Contains(err.Error(), "temperature") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveOpenClawAuthAutoPrefersCodexAuth(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "api-key")
	path := writeCodexAuth(t, t.TempDir())
	auth, err := resolveOpenClawAuth(Options{
		OpenClawAuth:          "auto",
		OpenClawCodexAuthPath: path,
	})
	if err != nil {
		t.Fatalf("resolve OpenClaw auth: %v", err)
	}
	if auth.Mode != "codex" || auth.CodexAuthPath != path {
		t.Fatalf("auth = %#v", auth)
	}
}

func TestResolveOpenClawAuthAutoFallsBackToAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "api-key")
	auth, err := resolveOpenClawAuth(Options{
		OpenClawAuth:          "auto",
		OpenClawCodexAuthPath: filepath.Join(t.TempDir(), "missing-auth.json"),
	})
	if err != nil {
		t.Fatalf("resolve OpenClaw auth: %v", err)
	}
	if auth.Mode != "api-key" {
		t.Fatalf("auth = %#v", auth)
	}
}

func TestResolveOpenClawAuthRequiresSelectedCredential(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := resolveOpenClawAuth(Options{
		OpenClawAuth:          "auto",
		OpenClawCodexAuthPath: filepath.Join(t.TempDir(), "missing-auth.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "OpenClaw auth requires") {
		t.Fatalf("error = %v", err)
	}
	_, err = resolveOpenClawAuth(Options{OpenClawAuth: "api-key"})
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("api-key error = %v", err)
	}
}

func TestOpenClawAuthArgsForAPIKey(t *testing.T) {
	state := &runState{openClawAuth: openClawAuthConfig{Mode: "api-key"}}
	args, prefix, err := state.openClawAuthArgs("plaintiff")
	if err != nil {
		t.Fatalf("auth args: %v", err)
	}
	if prefix != "" {
		t.Fatalf("prefix = %q", prefix)
	}
	if strings.Join(args, "\x00") != "-e\x00OPENAI_API_KEY" {
		t.Fatalf("args = %#v", args)
	}
}

func TestOpenClawAuthArgsForCodex(t *testing.T) {
	out := t.TempDir()
	source := writeCodexAuth(t, t.TempDir())
	state := &runState{
		opts:         Options{OutputDir: out},
		openClawAuth: openClawAuthConfig{Mode: "codex", CodexAuthPath: source},
	}
	args, prefix, err := state.openClawAuthArgs("plaintiff")
	if err != nil {
		t.Fatalf("auth args: %v", err)
	}
	if prefix != "unset OPENAI_API_KEY\n" {
		t.Fatalf("prefix = %q", prefix)
	}
	joined := strings.Join(args, "\n")
	if strings.Contains(joined, "OPENAI_API_KEY") {
		t.Fatalf("args contain OPENAI_API_KEY: %#v", args)
	}
	if !strings.Contains(joined, "CODEX_HOME=/aar-codex") {
		t.Fatalf("args missing CODEX_HOME: %#v", args)
	}
	staged := filepath.Join(out, "openclaw-plaintiff-codex", "auth.json")
	raw, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("read staged auth: %v", err)
	}
	if !strings.Contains(string(raw), `"auth_mode"`) {
		t.Fatalf("staged auth = %s", string(raw))
	}
	info, err := os.Stat(staged)
	if err != nil {
		t.Fatalf("stat staged auth: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("staged auth mode = %o", info.Mode().Perm())
	}
	if len(state.secretDirs) != 1 {
		t.Fatalf("secret dirs = %#v", state.secretDirs)
	}
	if err := state.cleanupSecrets(); err != nil {
		t.Fatalf("cleanup secrets: %v", err)
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatalf("staged auth still exists: %v", err)
	}
}

func TestOpenClawAuthArgsForCodexUsesAbsoluteMountPath(t *testing.T) {
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldCwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	source := writeCodexAuth(t, t.TempDir())
	state := &runState{
		opts:         Options{OutputDir: "relative-out"},
		openClawAuth: openClawAuthConfig{Mode: "codex", CodexAuthPath: source},
	}
	args, _, err := state.openClawAuthArgs("plaintiff")
	if err != nil {
		t.Fatalf("auth args: %v", err)
	}
	var mount string
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-v" {
			mount = args[i+1]
			break
		}
	}
	hostPath := strings.SplitN(mount, ":", 2)[0]
	if !filepath.IsAbs(hostPath) {
		t.Fatalf("mount host path is not absolute: %q", mount)
	}
	if err := state.cleanupSecrets(); err != nil {
		t.Fatalf("cleanup secrets: %v", err)
	}
}

func TestEffectiveLawyerTurnTimeoutSeconds(t *testing.T) {
	if got := effectiveLawyerTurnTimeoutSeconds(Options{}); got != proceeding.DefaultRuntimeLimits().LawyerTurnTimeoutSeconds {
		t.Fatalf("default timeout = %d", got)
	}
	if got := effectiveLawyerTurnTimeoutSeconds(Options{LawyerTimeoutSeconds: 123}); got != 123 {
		t.Fatalf("override timeout = %d", got)
	}
}

func TestOpenClawConfigPatchCommandUsesLawyerTimeout(t *testing.T) {
	cmd, err := openClawConfigPatchCommand(900)
	if err != nil {
		t.Fatalf("config patch command: %v", err)
	}
	start := strings.Index(cmd, "\n")
	end := strings.Index(cmd, "\nJSON\n")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("command does not contain JSON heredoc: %q", cmd)
	}
	raw := cmd[start+1 : end]
	var patch map[string]any
	if err := json.Unmarshal([]byte(raw), &patch); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	codex := patch["plugins"].(map[string]any)["entries"].(map[string]any)["codex"].(map[string]any)
	if codex["enabled"] != true {
		t.Fatalf("codex enabled = %#v", codex["enabled"])
	}
	appServer := codex["config"].(map[string]any)["appServer"].(map[string]any)
	for _, name := range []string{"turnCompletionIdleTimeoutMs", "postToolRawAssistantCompletionIdleTimeoutMs"} {
		if appServer[name] != float64(900000) {
			t.Fatalf("%s = %#v", name, appServer[name])
		}
	}
	if !strings.Contains(cmd, "openclaw config patch --file /tmp/aar-openclaw-config.json") {
		t.Fatalf("command = %q", cmd)
	}
}

func TestApplyDefaultsOpenClawStartDelay(t *testing.T) {
	if got := applyDefaults(Options{OpenClawStartDelaySeconds: -1}).OpenClawStartDelaySeconds; got != defaultOpenClawStartDelay {
		t.Fatalf("default start delay = %d", got)
	}
	if got := applyDefaults(Options{OpenClawStartDelaySeconds: 0}).OpenClawStartDelaySeconds; got != 0 {
		t.Fatalf("zero start delay = %d", got)
	}
	if got := applyDefaults(Options{OpenClawStartDelaySeconds: 27}).OpenClawStartDelaySeconds; got != 27 {
		t.Fatalf("override start delay = %d", got)
	}
}

func TestAutoLawyerRoles(t *testing.T) {
	tests := []struct {
		mode string
		want string
	}{
		{mode: "both", want: "plaintiff,defendant"},
		{mode: "plaintiff", want: "plaintiff"},
		{mode: "defendant", want: "defendant"},
	}
	for _, tc := range tests {
		got, err := autoLawyerRoles(tc.mode)
		if err != nil {
			t.Fatalf("%s: %v", tc.mode, err)
		}
		if strings.Join(got, ",") != tc.want {
			t.Fatalf("%s roles = %#v", tc.mode, got)
		}
	}
	if _, err := autoLawyerRoles("none"); err == nil {
		t.Fatalf("expected invalid mode error")
	}
	if got := strings.Join(manualLawyerRoles("defendant"), ","); got != "plaintiff" {
		t.Fatalf("manual roles = %q", got)
	}
}

func TestPublicMCPBaseAndManualAddressValidation(t *testing.T) {
	base, err := publicMCPBase("http://aar.example:8001/", "0.0.0.0:1234")
	if err != nil {
		t.Fatalf("public base: %v", err)
	}
	if base != "http://aar.example:8001" {
		t.Fatalf("base = %q", base)
	}
	if err := validateManualLawyerAddress("", "0.0.0.0:1234"); err == nil {
		t.Fatalf("expected wildcard listen error")
	}
	if err := validateManualLawyerAddress("", "192.0.2.10:1234"); err != nil {
		t.Fatalf("non-wildcard listen: %v", err)
	}
}

func TestWriteRemoteLawyerSkill(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "remote.md.tmpl")
	if err := os.WriteFile(templatePath, []byte("case={{.CaseID}} role={{.RoleID}} server={{.MCPServer}} url={{.MCPURL}} json={{.MCPJSON}}\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	state := &runState{
		opts: Options{
			CaseID:                "case-1",
			OutputDir:             dir,
			RemoteLawyerSkillPath: templatePath,
		},
		mcpPublicBase: "http://aar.example:8001",
		token:         "token-1",
	}
	if err := state.writeRemoteLawyerSkill("plaintiff"); err != nil {
		t.Fatalf("write remote skill: %v", err)
	}
	path := filepath.Join(dir, "openclaw-plaintiff-lawyer-skill.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	text := string(raw)
	for _, want := range []string{"case=case-1", "role=plaintiff", "http://aar.example:8001/mcp?case_id=case-1&role_id=plaintiff", "Bearer token-1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("skill missing %q: %s", want, text)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat skill: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("skill mode = %o", info.Mode().Perm())
	}
}

func TestWriteRunSummaryIncludesOpenClawStartDelay(t *testing.T) {
	dir := t.TempDir()
	if err := writeRunSummary(dir, proceeding.Result{
		CaseID: "case-1",
		RunID:  "run-1",
		Status: "ok",
	}, Options{OpenClawStartDelaySeconds: 15, AutoLawyers: "defendant", MCPPublicBaseURL: "http://aar.example:8001"}); err != nil {
		t.Fatalf("write run summary: %v", err)
	}
	summary := readJSONMap(t, filepath.Join(dir, "local-run.json"))
	if summary["openclaw_lawyer_start_delay_seconds"] != float64(15) {
		t.Fatalf("summary = %#v", summary)
	}
	if summary["auto_lawyers"] != "defendant" || summary["mcp_public_base_url"] != "http://aar.example:8001" {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestOutputSubdirReturnsAbsolutePath(t *testing.T) {
	got, err := outputSubdir("relative-out", "pi-C1")
	if err != nil {
		t.Fatalf("output subdir: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("path is not absolute: %q", got)
	}
	if !strings.HasSuffix(got, filepath.Join("relative-out", "pi-C1")) {
		t.Fatalf("path = %q", got)
	}
}

func TestResolveListenAddrAllocatesPort(t *testing.T) {
	addr, err := resolveListenAddr("0.0.0.0:0", "127.0.0.1")
	if err != nil {
		t.Fatalf("resolve listen addr: %v", err)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split listen addr: %v", err)
	}
	if host != "0.0.0.0" || port == "0" || port == "" {
		t.Fatalf("addr = %q", addr)
	}
}

func TestContainerNameSanitizesAndBounds(t *testing.T) {
	got := containerName("AAR/case with spaces and @ symbols " + strings.Repeat("x", 100))
	if len(got) > 63 {
		t.Fatalf("container name length = %d", len(got))
	}
	if strings.ContainsAny(got, "/ @") {
		t.Fatalf("container name = %q", got)
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

func writeCodexAuth(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "auth.json")
	raw := []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"test","refresh_token":"test"}}` + "\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write Codex auth: %v", err)
	}
	return path
}
