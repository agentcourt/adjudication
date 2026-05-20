package runner

import (
	"errors"
	"strings"
	"testing"
	"time"

	"adjudication/adc/runtime/spec"
)

func TestNewACPConfigAcceptsEndpoint(t *testing.T) {
	t.Parallel()

	cfg, err := NewACPConfig([]string{"plaintiff"}, "", "tcp://127.0.0.1:19701", nil, nil, time.Second)
	if err != nil {
		t.Fatalf("NewACPConfig returned error: %v", err)
	}
	if cfg.Command != "" || cfg.Endpoint != "tcp://127.0.0.1:19701" {
		t.Fatalf("cfg = %#v", cfg)
	}
	if _, ok := cfg.Roles["plaintiff"]; !ok {
		t.Fatalf("cfg roles missing plaintiff: %#v", cfg.Roles)
	}
}

func TestNewACPConfigRejectsCommandAndEndpoint(t *testing.T) {
	t.Parallel()

	_, err := NewACPConfig([]string{"plaintiff"}, "pi-acp", "tcp://127.0.0.1:19701", nil, nil, time.Second)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("NewACPConfig error = %v, want mutual exclusion error", err)
	}
}

func TestNewACPConfigRejectsEndpointEnv(t *testing.T) {
	t.Parallel()

	_, err := NewACPConfig([]string{"plaintiff"}, "", "tcp://127.0.0.1:19701", nil, []string{"A=B"}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "command mode") {
		t.Fatalf("NewACPConfig error = %v, want endpoint env error", err)
	}
}

func TestFormatInvalidAttemptLimitErrorIncludesHistory(t *testing.T) {
	t.Parallel()

	err := formatInvalidAttemptLimitError("agent turn=4 role=plaintiff", []string{
		"tool not allowed",
		"payload.text is required",
	})
	text := err.Error()
	for _, want := range []string{
		"agent turn=4 role=plaintiff exceeded invalid-attempt limit after 2 invalid submissions",
		"attempt 1: tool not allowed",
		"attempt 2: payload.text is required",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("error missing %q: %s", want, text)
		}
	}
}

func TestACPRoleToolSpecsIncludesJurorContext(t *testing.T) {
	t.Parallel()

	role := spec.RoleSpec{
		Name:           "plaintiff",
		AllowedActions: []string{"get_case", "get_juror_context", "record_voir_dire_question"},
	}
	specs := acpRoleToolSpecs(role)
	found := false
	for _, spec := range specs {
		if stringOrDefault(spec["toolName"], "") == "adc_get_juror_context" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("acpRoleToolSpecs missing adc_get_juror_context")
	}
}

func TestBuildACPRolePromptDirectsVoirDireToJurorContext(t *testing.T) {
	t.Parallel()

	role := spec.RoleSpec{
		Name:           "plaintiff",
		AllowedActions: []string{"get_case", "get_juror_context", "record_voir_dire_question"},
		Instructions:   "Question jurors.",
	}
	opportunity := leanOpportunity{
		Phase:        "voir_dire",
		AllowedTools: []string{"record_voir_dire_question"},
	}
	view := map[string]any{
		"case": map[string]any{
			"case_files": []any{},
		},
	}
	r := &Runner{}
	prompt := r.buildACPRolePrompt(role, view, opportunity)
	if !strings.Contains(prompt, "Do not call adc_get_case to reread the same view.") {
		t.Fatalf("prompt missing get_case guidance\n%s", prompt)
	}
	if !strings.Contains(prompt, "use adc_get_juror_context with that juror_id instead of adc_get_case") {
		t.Fatalf("prompt missing voir dire juror-context guidance\n%s", prompt)
	}
}

func TestBuildACPRolePromptRemovesWorkingTreeGuidance(t *testing.T) {
	t.Parallel()

	role := spec.RoleSpec{
		Name:           "plaintiff",
		AllowedActions: []string{"list_case_files", "read_case_text_file", "import_case_file"},
		Instructions:   "Handle evidence.",
	}
	opportunity := leanOpportunity{
		Phase:        "pretrial",
		AllowedTools: []string{"import_case_file"},
	}
	view := map[string]any{
		"case": map[string]any{
			"case_files": []any{},
		},
	}
	r := &Runner{}
	prompt := r.buildACPRolePrompt(role, view, opportunity)
	if strings.Contains(prompt, "case working directory for this run") {
		t.Fatalf("prompt still mentions case working directory\n%s", prompt)
	}
	if !strings.Contains(prompt, "You do not have direct filesystem access to case materials.") {
		t.Fatalf("prompt missing filesystem restriction guidance\n%s", prompt)
	}
	if !strings.Contains(prompt, "Before you submit a technical report, trial theory, exhibit offer, motion, opening, or closing, analyze the visible case files that bear on the disputed points.") {
		t.Fatalf("prompt missing file-analysis guidance\n%s", prompt)
	}
	if !strings.Contains(prompt, "You may use local tools in your runtime environment to analyze materials you obtain through the ADC tools.") {
		t.Fatalf("prompt missing local-analysis guidance\n%s", prompt)
	}
	if !strings.Contains(prompt, "If a needed local tool is missing, you may install it in that runtime environment for the current task.") {
		t.Fatalf("prompt missing install-tools guidance\n%s", prompt)
	}
	if !strings.Contains(prompt, "Do the analysis before you draft the filing, not as a plan for later.") {
		t.Fatalf("prompt missing execute-now guidance\n%s", prompt)
	}
	if !strings.Contains(prompt, "Do not submit a technical report, motion, opening, closing, or trial theory that only proposes a later verification or calculation when you can do it now from the visible case files.") {
		t.Fatalf("prompt missing verification guidance\n%s", prompt)
	}
	if !strings.Contains(prompt, "decode the signature locally, verify it locally, and report the result instead of saying verification could be done later") {
		t.Fatalf("prompt missing concrete verification example\n%s", prompt)
	}
	if !strings.Contains(prompt, "original_name and content_base64") {
		t.Fatalf("prompt missing import guidance\n%s", prompt)
	}
}

func TestBuildACPRolePromptIncludesCapabilityAndLimits(t *testing.T) {
	t.Parallel()

	role := spec.RoleSpec{
		Name:           "plaintiff",
		AllowedActions: []string{"list_case_files", "offer_case_file_as_exhibit", "submit_technical_report"},
		Instructions:   "Handle evidence.",
	}
	opportunity := leanOpportunity{
		Phase:        "plaintiff_evidence",
		AllowedTools: []string{"offer_case_file_as_exhibit"},
		StepBudget:   2,
	}
	r := &Runner{
		state: map[string]any{
			"policy": map[string]any{
				"max_support_tool_calls_per_opportunity": 7,
				"max_exhibits_per_side":                  4,
				"max_technical_reports_per_side":         3,
			},
			"case": map[string]any{
				"case_files": []any{map[string]any{"file_id": "F1"}},
				"file_events": []any{
					map[string]any{"action": "offer_case_file_as_exhibit", "actor": "plaintiff", "file_id": "F1"},
				},
				"technical_reports": []any{
					map[string]any{"party": "plaintiff", "report_id": "R1"},
				},
			},
		},
		cfg: Config{Runtime: RuntimeLimits{InvalidAttemptLimit: 5}},
	}
	prompt := r.buildACPRolePrompt(role, map[string]any{"case": map[string]any{}}, opportunity)
	for _, want := range []string{
		"ACP capability and limits:",
		"Support host-method calls remaining for this opportunity: 7.",
		"Decision submissions allowed by this opportunity: 2.",
		"Invalid submissions allowed before the opportunity fails: 5.",
		"Visible case files in the current case state: 1.",
		"plaintiff exhibit offers: 1 used, 3 remaining, 4 maximum.",
		"plaintiff technical reports: 1 used, 2 remaining, 3 maximum.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q\n%s", want, prompt)
		}
	}
}

func TestCloseACPSessionsClearsSessionCache(t *testing.T) {
	t.Parallel()

	cleanupCalls := 0
	r := &Runner{
		acpSessions: map[string]*acpPersistentSession{
			"plaintiff": {
				cleanup: func() error {
					cleanupCalls++
					return nil
				},
			},
			"defendant": {
				cleanup: func() error {
					cleanupCalls++
					return nil
				},
			},
		},
	}
	if err := r.closeACPSessions(); err != nil {
		t.Fatalf("closeACPSessions returned error: %v", err)
	}
	if cleanupCalls != 2 {
		t.Fatalf("cleanupCalls = %d, want 2", cleanupCalls)
	}
	if len(r.acpSessions) != 0 {
		t.Fatalf("expected session cache to be empty, got %d entries", len(r.acpSessions))
	}
}

func TestCloseACPSessionsReturnsCleanupErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("cleanup failed")
	r := &Runner{
		acpSessions: map[string]*acpPersistentSession{
			"plaintiff": {
				cleanup: func() error {
					return want
				},
			},
		},
	}
	err := r.closeACPSessions()
	if err == nil {
		t.Fatal("closeACPSessions returned nil error")
	}
	if !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("closeACPSessions error = %v, want cleanup failure", err)
	}
	if len(r.acpSessions) != 0 {
		t.Fatalf("expected session cache to be empty after cleanup error, got %d entries", len(r.acpSessions))
	}
}
