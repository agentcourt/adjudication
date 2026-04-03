package runner

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"adjudication/adc/runtime/courts"
	"adjudication/adc/runtime/lean"
	"adjudication/adc/runtime/spec"
	"adjudication/adc/runtime/store"
)

type stubTimeoutError struct{}

func (stubTimeoutError) Error() string   { return "timed out" }
func (stubTimeoutError) Timeout() bool   { return true }
func (stubTimeoutError) Temporary() bool { return true }

func TestIsTimeoutError(t *testing.T) {
	t.Parallel()

	var netErr net.Error = stubTimeoutError{}
	if !isTimeoutError(netErr) {
		t.Fatalf("isTimeoutError(net timeout) = false, want true")
	}
	if !isTimeoutError(context.DeadlineExceeded) {
		t.Fatalf("isTimeoutError(context deadline exceeded) = false, want true")
	}
	if isTimeoutError(nil) {
		t.Fatalf("isTimeoutError(nil) = true, want false")
	}
	if isTimeoutError(assertionError("different failure")) {
		t.Fatalf("isTimeoutError(non-timeout) = true, want false")
	}
}

func TestJurorTimeoutOpportunityKinds(t *testing.T) {
	t.Parallel()

	if !isCandidateJurorOpportunity(leanOpportunity{
		Phase:        "voir_dire",
		AllowedTools: []string{"answer_juror_questionnaire"},
	}) {
		t.Fatalf("candidate voir dire opportunity not recognized")
	}
	if !isCandidateJurorOpportunity(leanOpportunity{
		Phase:        "voir_dire",
		AllowedTools: []string{"answer_voir_dire_question"},
	}) {
		t.Fatalf("candidate voir dire answer opportunity not recognized")
	}
	if isCandidateJurorOpportunity(leanOpportunity{
		Phase:        "deliberation",
		AllowedTools: []string{"submit_juror_vote"},
	}) {
		t.Fatalf("deliberation opportunity misclassified as candidate timeout")
	}
	if !isDeliberationJurorOpportunity(leanOpportunity{
		Phase:        "deliberation",
		AllowedTools: []string{"submit_juror_vote"},
	}) {
		t.Fatalf("deliberation opportunity not recognized")
	}
	if isDeliberationJurorOpportunity(leanOpportunity{
		Phase:        "voir_dire",
		AllowedTools: []string{"answer_juror_questionnaire"},
	}) {
		t.Fatalf("voir dire opportunity misclassified as deliberation timeout")
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }

func newTimeoutTestRunner(t *testing.T) *Runner {
	t.Helper()

	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "run.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})
	if err := st.CreateRun("run-1", "timeout-test"); err != nil {
		t.Fatalf("create run: %v", err)
	}
	engineDir, err := filepath.Abs(filepath.Join("..", "..", "engine"))
	if err != nil {
		t.Fatalf("resolve engine dir: %v", err)
	}
	return &Runner{
		lean:  lean.New([]string{"bash", "-lc", fmt.Sprintf("cd %s && lake exe adcengine", engineDir)}),
		store: st,
		cfg: Config{
			RunID: "run-1",
		},
		state: buildInitialState(spec.FormalScenario{CourtName: "Test Court"}, courts.Profile{}),
	}
}

func TestHandleCandidateJurorTimeoutReplacesJuror(t *testing.T) {
	r := newTimeoutTestRunner(t)
	caseObj := r.state["case"].(map[string]any)
	caseObj["status"] = "trial"
	caseObj["trial_mode"] = "jury"
	caseObj["phase"] = "voir_dire"
	caseObj["jury_configuration"] = map[string]any{
		"juror_count":        6,
		"unanimous_required": true,
		"minimum_concurring": 6,
	}
	caseObj["jurors"] = []any{
		map[string]any{"juror_id": "J1", "name": "Juror 1", "status": "candidate", "note": "", "model": "", "persona_filename": ""},
	}
	opportunity := leanOpportunity{
		OpportunityID: "opp-1",
		Phase:         "voir_dire",
		Objective:     "Answer the juror questionnaire.",
		AllowedTools:  []string{"answer_juror_questionnaire"},
		Constraints: map[string]any{
			"required_payload": map[string]any{"juror_id": "J1"},
		},
	}
	log, err := r.handleCandidateJurorTimeout(24, opportunity, "openrouter://model", "J1", context.DeadlineExceeded)
	if err != nil {
		t.Fatalf("handleCandidateJurorTimeout error = %v", err)
	}
	if log.Steps != 2 {
		t.Fatalf("Steps = %d, want 2", log.Steps)
	}
	caseObj = r.state["case"].(map[string]any)
	jurors := caseObj["jurors"].([]any)
	if len(jurors) != 2 {
		t.Fatalf("juror count = %d, want 2", len(jurors))
	}
	first := jurors[0].(map[string]any)
	second := jurors[1].(map[string]any)
	if first["juror_id"] != "J1" || first["status"] != "timed_out" {
		t.Fatalf("first juror = %#v", first)
	}
	if second["juror_id"] != "J2" || second["status"] != "candidate" {
		t.Fatalf("replacement juror = %#v", second)
	}
}

func TestHandleDeliberatingJurorTimeoutDeclaresHungJuryWhenThresholdImpossible(t *testing.T) {
	r := newTimeoutTestRunner(t)
	caseObj := r.state["case"].(map[string]any)
	caseObj["status"] = "trial"
	caseObj["trial_mode"] = "jury"
	caseObj["phase"] = "deliberation"
	caseObj["jury_configuration"] = map[string]any{
		"juror_count":        6,
		"unanimous_required": true,
		"minimum_concurring": 6,
	}
	caseObj["jurors"] = []any{
		map[string]any{"juror_id": "J1", "name": "Juror 1", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
		map[string]any{"juror_id": "J2", "name": "Juror 2", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
		map[string]any{"juror_id": "J3", "name": "Juror 3", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
		map[string]any{"juror_id": "J4", "name": "Juror 4", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
		map[string]any{"juror_id": "J5", "name": "Juror 5", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
		map[string]any{"juror_id": "J6", "name": "Juror 6", "status": "sworn", "note": "", "model": "", "persona_filename": ""},
	}
	opportunity := leanOpportunity{
		OpportunityID: "opp-2",
		Phase:         "deliberation",
		Objective:     "Submit the juror vote.",
		AllowedTools:  []string{"submit_juror_vote"},
		Constraints: map[string]any{
			"required_payload": map[string]any{"juror_id": "J1"},
		},
	}
	log, err := r.handleDeliberatingJurorTimeout(69, opportunity, "openrouter://model", "J1", context.DeadlineExceeded)
	if err != nil {
		t.Fatalf("handleDeliberatingJurorTimeout error = %v", err)
	}
	if log.Steps != 1 {
		t.Fatalf("Steps = %d, want 1", log.Steps)
	}
	caseObj = r.state["case"].(map[string]any)
	jurors := caseObj["jurors"].([]any)
	first := jurors[0].(map[string]any)
	if first["status"] != "timed_out" {
		t.Fatalf("timed out juror = %#v", first)
	}
	hung, _ := caseObj["hung_jury"].(map[string]any)
	if hung == nil {
		t.Fatalf("hung_jury = nil, want record")
	}
	note, _ := hung["note"].(string)
	if !strings.Contains(note, "fewer than 6 sworn jurors remained eligible to deliberate") {
		t.Fatalf("hung jury note = %q", note)
	}
}
