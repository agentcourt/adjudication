package runner

import (
	"errors"
	"strings"
	"testing"

	"adjudication/adc/runtime/spec"
)

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
