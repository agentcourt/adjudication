package runner

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"adjudication/adc/runtime/spec"
)

func TestObserverCannotActOnActiveTurn(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)

	status := api.statusResponseLocked(roleAPIRequest{CaseID: "case-1", RoleID: "observer"})
	if status["status"] != "active" {
		t.Fatalf("observer status = %#v, want active read access", status)
	}

	response, statusCode := api.doLocked(roleAPIRequest{
		CaseID:        "case-1",
		RoleID:        "observer",
		OpportunityID: "opp-1",
		Tool:          "send_work_notes",
		Arguments:     map[string]any{"notes": "observer should not be able to write notes for plaintiff"},
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("observer do status = %d, response = %#v", statusCode, response)
	}
	if response["ok"] != false {
		t.Fatalf("observer do response = %#v", response)
	}
	if turn.completed {
		t.Fatalf("observer do completed the active turn")
	}

	response, statusCode = api.doLocked(roleAPIRequest{
		CaseID:        "case-1",
		RoleID:        "observer",
		OpportunityID: "opp-1",
		Tool:          "submit_decision",
		Arguments:     map[string]any{"kind": "pass", "reason": "observer should not be able to pass for plaintiff"},
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("observer submit status = %d, response = %#v", statusCode, response)
	}
	if response["ok"] != false {
		t.Fatalf("observer submit response = %#v", response)
	}
	if turn.completed {
		t.Fatalf("observer submit completed the active turn")
	}
}

func TestObserverCannotReportFailureForActiveTurn(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)

	response, statusCode := api.failLocked(roleAPIRequest{
		CaseID:  "case-1",
		RoleID:  "observer",
		Message: "observer should not be able to fail plaintiff",
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("observer fail status = %d, response = %#v", statusCode, response)
	}
	if response["ok"] != false {
		t.Fatalf("observer fail response = %#v", response)
	}
	if turn.completed {
		t.Fatalf("observer fail completed the active turn")
	}
	select {
	case result := <-turn.done:
		t.Fatalf("observer fail sent turn result: %#v", result)
	default:
	}
}

func TestWorkNotesStayOutOfTurnTranscript(t *testing.T) {
	api, turn := testRoleAPIWithActiveTurn(t)

	response, statusCode := api.doLocked(roleAPIRequest{
		CaseID:        "case-1",
		RoleID:        "plaintiff",
		OpportunityID: "opp-1",
		Tool:          "send_work_notes",
		Arguments:     map[string]any{"notes": "plan, work log, and evidence analysis"},
	})
	if statusCode != http.StatusOK {
		t.Fatalf("send notes status = %d, response = %#v", statusCode, response)
	}
	if response["ok"] != true {
		t.Fatalf("send notes response = %#v", response)
	}
	if len(turn.transcript) != 0 {
		t.Fatalf("transcript = %#v, want no work-note entry", turn.transcript)
	}
	raw, err := os.ReadFile(api.r.workNotesPath())
	if err != nil {
		t.Fatalf("read work notes: %v", err)
	}
	if !strings.Contains(string(raw), "plan, work log, and evidence analysis") {
		t.Fatalf("work notes = %s", string(raw))
	}
	if turn.completed {
		t.Fatalf("send_work_notes completed the active turn")
	}
}

func testRoleAPIWithActiveTurn(t *testing.T) (*roleAPIServer, *externalOpportunityTurn) {
	t.Helper()

	r := &Runner{
		cfg: Config{CaseID: "case-1", ScenarioBaseDir: t.TempDir()},
		state: map[string]any{
			"case": map[string]any{"status": "active", "phase": "trial"},
		},
	}
	api := newRoleAPIServer(r)
	r.roleAPI = api
	turn := &externalOpportunityTurn{
		turnIndex: 1,
		role:      spec.RoleSpec{Name: "plaintiff"},
		opportunity: leanOpportunity{
			OpportunityID: "opp-1",
			Phase:         "trial",
			Kind:          "plaintiff_opening",
			MayPass:       true,
		},
		deadline:          time.Now().Add(time.Minute),
		attemptsMax:       3,
		attemptsRemaining: 3,
		supportBudget:     30,
		view:              map[string]any{"role": "plaintiff"},
		done:              make(chan externalOpportunityResult, 1),
	}
	api.active = turn
	return api, turn
}
