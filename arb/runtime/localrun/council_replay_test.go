package localrun

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"adjudication/arb/runtime/proceeding"
	"adjudication/arb/runtime/spec"
)

func TestLoadCouncilReplayModelConfigAppliesPersonaOverride(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "model.json")
	personaPath := filepath.Join(dir, "persona.txt")
	if err := os.WriteFile(configPath, []byte(`{"endpoint":"openrouter","model":"example/model","persona":"missing.md"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(personaPath, []byte("Override persona.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry, seat, err := loadCouncilReplayModelConfig(configPath, "C9", personaPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	absPersonaPath, err := filepath.Abs(personaPath)
	if err != nil {
		t.Fatal(err)
	}
	if entry.MemberID != "C9" || seat.MemberID != "C9" {
		t.Fatalf("member ids = %q/%q", entry.MemberID, seat.MemberID)
	}
	if entry.Model != "openrouter://example/model" || seat.Model != "openrouter://example/model" {
		t.Fatalf("models = %q/%q", entry.Model, seat.Model)
	}
	if seat.PersonaFile != absPersonaPath || seat.PersonaText != "Override persona." {
		t.Fatalf("persona = %q/%q", seat.PersonaFile, seat.PersonaText)
	}
	if seat.RequestSpec == nil || seat.RequestSpec.Persona != absPersonaPath {
		t.Fatalf("request spec persona = %#v", seat.RequestSpec)
	}
	if entry.RequestSpec == nil || entry.RequestSpec.Persona != absPersonaPath {
		t.Fatalf("entry request spec persona = %#v", entry.RequestSpec)
	}
}

func TestLoadCouncilReplayModelConfigRejectsMissingPersonaOverride(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "model.json")
	if err := os.WriteFile(configPath, []byte(`{"endpoint":"openrouter","model":"example/model"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := loadCouncilReplayModelConfig(configPath, "C1", filepath.Join(dir, "missing.txt"))
	if err == nil || !strings.Contains(err.Error(), "stat persona") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadCouncilReplayModelConfigRejectsEmptyPersonaOverride(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "model.json")
	personaPath := filepath.Join(dir, "persona.txt")
	if err := os.WriteFile(configPath, []byte(`{"endpoint":"openrouter","model":"example/model"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(personaPath, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := loadCouncilReplayModelConfig(configPath, "C1", personaPath)
	if err == nil || !strings.Contains(err.Error(), "empty persona text") {
		t.Fatalf("error = %v", err)
	}
}

func TestWaitCouncilReplayResultReturnsPostedFailure(t *testing.T) {
	input := proceeding.CouncilReplayInput{
		Basis:           proceeding.CouncilReplayBasisSnapshot,
		CaseID:          "case-1",
		MemberID:        "C1",
		SourceOutputDir: "source-out",
		Runtime:         proceeding.DefaultRuntimeLimits(),
		Seat: proceeding.CouncilReplaySeat{
			MemberID: "C1",
			Model:    "openrouter://example/model",
		},
	}
	server := &frozenCouncilReplayServer{
		input:       input,
		voteDone:    make(chan councilReplayVote, 1),
		failureDone: make(chan councilReplayFailure, 1),
	}
	const message = "Council member C1 agent process failed before completing opportunity deliberation:1:C1: provider error."
	req := httptest.NewRequest(http.MethodPost, "/councilapi/v1/fail", bytes.NewBufferString(`{
		"case_id":"case-1",
		"member_id":"C1",
		"opportunity_id":"deliberation:1:C1",
		"reason":"agent_exited",
		"message":"`+message+`",
		"details":{"agent_error":"provider error"}
	}`))
	rec := httptest.NewRecorder()
	server.handleFail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("handleFail status = %d body=%s", rec.Code, rec.Body.String())
	}
	result, err := waitCouncilReplayResult(context.Background(), CouncilReplayOptions{
		CouncilTimeoutSeconds: 60,
		OutputDir:             "out",
	}, input, &runState{agentErrs: make(chan error, 1)}, server)
	if err == nil || err.Error() != message {
		t.Fatalf("wait error = %v", err)
	}
	if result.Error != message || result.Status != "error" {
		t.Fatalf("result = %#v", result)
	}
	if result.ErrorDetails["agent_error"] != "provider error" {
		t.Fatalf("error details = %#v", result.ErrorDetails)
	}
}

func TestReplayCouncilCleansSecretsWhenPiStartFails(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	dir := t.TempDir()
	sourceDir := writeReplayCouncilSourceFixture(t)
	configPath := filepath.Join(dir, "model.json")
	personaPath := filepath.Join(dir, "persona.txt")
	outDir := filepath.Join(dir, "replay-out")
	if err := os.WriteFile(configPath, []byte(`{"endpoint":"openrouter","model":"example/model"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(personaPath, []byte("Cleanup persona."), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReplayCouncil(context.Background(), CouncilReplayOptions{
		Basis:                   proceeding.CouncilReplayBasisReconstructed,
		SourceOutputDir:         sourceDir,
		PromptDir:               filepath.Join("..", "..", "prompts"),
		ModelConfigPath:         configPath,
		PersonaPath:             personaPath,
		OutputDir:               outDir,
		MemberID:                "C1",
		CouncilInstructionsPath: filepath.Join("..", "..", "agent-instructions", "pi-council.md.tmpl"),
		PodmanCommand:           filepath.Join(dir, "missing-container-command"),
		PiImage:                 "agentcourt-pi-sandbox:test",
	})
	if err == nil || !strings.Contains(err.Error(), "start pi-C1") {
		t.Fatalf("ReplayCouncil error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(outDir, "pi-C1", ".mcp.json"),
		filepath.Join(outDir, "pi-C1", ".pi", "agent", "auth.json"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("secret file still exists %s: %v", path, err)
		}
	}
}

func writeReplayCouncilSourceFixture(t *testing.T) string {
	t.Helper()
	outDir := t.TempDir()
	policy := proceeding.DefaultPolicy()
	complaint := spec.Complaint{Proposition: "P"}
	state := map[string]any{
		"state_version": 18,
		"policy":        policy.StateMap(),
		"case": map[string]any{
			"case_id":            "case-1",
			"status":             "closed",
			"phase":              "closed",
			"resolution":         "demonstrated",
			"deliberation_round": 1,
			"openings":           []map[string]any{{"role": "plaintiff", "text": "opening"}},
			"arguments":          []map[string]any{},
			"rebuttals":          []map[string]any{},
			"surrebuttals":       []map[string]any{},
			"closings":           []map[string]any{},
			"offered_evidence":   []map[string]any{},
			"submitted_evidence": []map[string]any{},
			"technical_reports":  []map[string]any{},
			"council_votes":      []map[string]any{{"round": 1, "member_id": "C1", "vote": "demonstrated", "rationale": "old"}},
			"council_members":    []map[string]any{},
		},
	}
	if err := writeJSONFile(filepath.Join(outDir, "run.json"), proceeding.Result{
		CaseID:     "case-1",
		RunID:      "run-case-1",
		Status:     "ok",
		Resolution: "demonstrated",
		Complaint:  complaint,
		FinalState: state,
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "state.json"), state); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "policy.json"), policy); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "runtime.json"), proceeding.DefaultRuntimeLimits()); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "evidence-manifest.json"), map[string]any{
		"schema_version": "aar.evidence-manifest.v0",
		"evidence_count": 0,
		"evidence":       []proceeding.EvidenceMeta{},
	}); err != nil {
		t.Fatal(err)
	}
	return outDir
}
