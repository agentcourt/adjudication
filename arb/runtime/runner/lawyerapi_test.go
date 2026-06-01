package runner

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLawyerDoRequiresActiveOpportunityID(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()

	for _, tc := range []struct {
		name          string
		opportunityID string
		wantCode      string
	}{
		{name: "missing", wantCode: "missing_opportunity_id"},
		{name: "stale", opportunityID: "openings:defendant", wantCode: "stale_opportunity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{
				"case_id":   "arb-1",
				"role_id":   "plaintiff",
				"tool":      "get_case",
				"arguments": map[string]any{},
			}
			if tc.opportunityID != "" {
				body["opportunity_id"] = tc.opportunityID
			}
			status, got := callLawyerAPIDo(t, api, body)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			if got["ok"] != false {
				t.Fatalf("ok = %#v, want false", got["ok"])
			}
			if code := lawyerAPIErrorCode(t, got); code != tc.wantCode {
				t.Fatalf("error code = %q, want %q", code, tc.wantCode)
			}
			if turn.attemptsRemaining != turn.attemptsMax {
				t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
			}
		})
	}

	status, got := callLawyerAPIDo(t, api, map[string]any{
		"case_id":        "arb-1",
		"role_id":        "plaintiff",
		"opportunity_id": "openings:plaintiff",
		"tool":           "get_case",
		"arguments":      map[string]any{},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %#v, want true", got["ok"])
	}
	if turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
	}
}

func TestObserverDoDoesNotRequireOpportunityID(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()

	status, got := callLawyerAPIDo(t, api, map[string]any{
		"case_id":   "arb-1",
		"role_id":   "observer",
		"tool":      "get_turn",
		"arguments": map[string]any{},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %#v, want true", got["ok"])
	}
	result, ok := got["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want object", got["result"])
	}
	turn, ok := result["turn"].(map[string]any)
	if !ok {
		t.Fatalf("result.turn = %#v, want object", result["turn"])
	}
	if turn["opportunity_id"] != "openings:plaintiff" {
		t.Fatalf("opportunity_id = %#v, want openings:plaintiff", turn["opportunity_id"])
	}
}

func testLawyerAPIWithTurn() (*lawyerAPIServer, *lawyerTurn) {
	policy := DefaultPolicy()
	turn := &lawyerTurn{
		opportunity: Opportunity{
			ID:           "openings:plaintiff",
			Role:         "plaintiff",
			Phase:        "openings",
			AllowedTools: []string{"record_opening_statement"},
		},
		turnNumber:        1,
		deadline:          time.Now().Add(time.Minute),
		attemptsMax:       3,
		attemptsRemaining: 3,
		evidenceBudget:    &evidenceReadBudget{},
		done:              make(chan error, 1),
	}
	rc := &runContext{
		cfg: Config{
			Policy:  policy,
			Runtime: DefaultRuntimeLimits(),
		},
		state:          initialState(policy),
		fileByID:       map[string]CaseFile{},
		uploadSessions: map[string]*EvidenceUploadSession{},
	}
	return &lawyerAPIServer{rc: rc, active: turn}, turn
}

func callLawyerAPIDo(t *testing.T, api *lawyerAPIServer, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, lawyerAPIBasePath+"/do", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.handleDo(rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}

func lawyerAPIErrorCode(t *testing.T, got map[string]any) string {
	t.Helper()
	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %#v, want object", got["error"])
	}
	return mapString(errObj["code"])
}
