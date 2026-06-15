package proceeding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"adjudication/arb/runtime/spec"
)

func TestBuildCouncilReplayInputReconstructsIndependentFirstRound(t *testing.T) {
	outDir := writeCouncilReplayOutputFixture(t)
	input, err := BuildCouncilReplayInput(CouncilReplayBuildOptions{
		Basis:           CouncilReplayBasisReconstructed,
		SourceOutputDir: outDir,
		PromptDir:       councilReplayTestPromptDir(),
		Seat: CouncilSeat{
			MemberID:    "C9",
			Model:       "openrouter://example/model",
			PersonaFile: "personas/test.md",
			PersonaText: "Careful test persona.",
		},
	})
	if err != nil {
		t.Fatalf("BuildCouncilReplayInput returned error: %v", err)
	}
	if input.Basis != CouncilReplayBasisReconstructed {
		t.Fatalf("basis = %q", input.Basis)
	}
	if input.MemberID != "C9" || input.Opportunity.ID != "deliberation:1:C9" {
		t.Fatalf("member/opportunity = %q/%q", input.MemberID, input.Opportunity.ID)
	}
	caseObj := mapAny(input.State["case"])
	if got := mapList(caseObj["council_votes"]); len(got) != 0 {
		t.Fatalf("reconstructed council votes len = %d, want 0", len(got))
	}
	if caseObj["phase"] != "deliberation" || caseObj["resolution"] != "" {
		t.Fatalf("reconstructed case phase/resolution = %#v/%#v", caseObj["phase"], caseObj["resolution"])
	}
	if !strings.Contains(input.Prompt, "Persona:\nCareful test persona.") {
		t.Fatalf("prompt missing supplied persona:\n%s", input.Prompt)
	}
	if !strings.Contains(input.Prompt, "Council API instructions:") {
		t.Fatalf("prompt missing council API instructions:\n%s", input.Prompt)
	}
}

func TestBuildCouncilReplayInputFromSnapshotUsesSnapshotState(t *testing.T) {
	outDir := writeCouncilReplayOutputFixture(t)
	snapshotDir := filepath.Join(outDir, "council-turns", "turn-000010-C2")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapshotState := replayFixtureState()
	caseObj := mapAny(snapshotState["case"])
	caseObj["deliberation_round"] = 2
	caseObj["council_votes"] = []map[string]any{{"round": 1, "member_id": "C1", "vote": "demonstrated", "rationale": "prior"}}
	snapshotState["case"] = caseObj
	snapshot := CouncilTurnSnapshot{
		SchemaVersion:   councilTurnSnapshotSchemaVersion,
		CreatedAt:       utcTimestamp(),
		SourceOutputDir: outDir,
		CaseID:          "case-1",
		RunID:           "run-case-1",
		MemberID:        "C2",
		TurnNumber:      10,
		Opportunity:     Opportunity{ID: "deliberation:2:C2", Role: "council", Phase: "deliberation"},
		Policy:          DefaultPolicy(),
		Runtime:         DefaultRuntimeLimits(),
		Complaint:       spec.Complaint{Proposition: "P"},
		State:           snapshotState,
		Prompt:          "saved original prompt",
	}
	if err := writeJSONFile(filepath.Join(snapshotDir, "input.json"), snapshot); err != nil {
		t.Fatal(err)
	}
	input, err := BuildCouncilReplayInput(CouncilReplayBuildOptions{
		Basis:           CouncilReplayBasisSnapshot,
		SourceOutputDir: outDir,
		SnapshotDir:     snapshotDir,
		PromptDir:       councilReplayTestPromptDir(),
		Seat: CouncilSeat{
			Model:       "openrouter://example/replay-model",
			PersonaFile: "personas/replay.md",
			PersonaText: "Replay persona.",
		},
	})
	if err != nil {
		t.Fatalf("BuildCouncilReplayInput returned error: %v", err)
	}
	if input.Basis != CouncilReplayBasisSnapshot || input.MemberID != "C2" {
		t.Fatalf("basis/member = %q/%q", input.Basis, input.MemberID)
	}
	if input.OriginalPrompt != "saved original prompt" {
		t.Fatalf("original prompt = %q", input.OriginalPrompt)
	}
	if !strings.Contains(input.Prompt, "Prior rounds:\nRound 1 [C1] demonstrated") {
		t.Fatalf("snapshot prompt missing prior vote:\n%s", input.Prompt)
	}
	if !strings.Contains(input.Prompt, "Persona:\nReplay persona.") {
		t.Fatalf("snapshot prompt missing replay persona:\n%s", input.Prompt)
	}
}

func writeCouncilReplayOutputFixture(t *testing.T) string {
	t.Helper()
	outDir := t.TempDir()
	state := replayFixtureState()
	result := Result{
		CaseID:     "case-1",
		RunID:      "run-case-1",
		Status:     "ok",
		Resolution: "demonstrated",
		Complaint:  spec.Complaint{Proposition: "P"},
		FinalState: state,
	}
	if err := writeJSONFile(filepath.Join(outDir, "run.json"), result); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "state.json"), state); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "policy.json"), DefaultPolicy()); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "runtime.json"), DefaultRuntimeLimits()); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "evidence-manifest.json"), map[string]any{
		"schema_version": evidenceManifestSchemaVersion,
		"evidence_count": 0,
		"evidence":       []EvidenceMeta{},
	}); err != nil {
		t.Fatal(err)
	}
	return outDir
}

func replayFixtureState() map[string]any {
	policy := DefaultPolicy()
	return map[string]any{
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
}

func councilReplayTestPromptDir() string {
	return filepath.Join("..", "..", "prompts")
}
