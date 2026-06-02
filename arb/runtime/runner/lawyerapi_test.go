package runner

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func TestLawyerSendWorkNotesWritesOffRecordLog(t *testing.T) {
	api, turn := testLawyerAPIWithTurn()
	api.rc.cfg.OutputDir = t.TempDir()
	notes := strings.Repeat("plan source-chain OCR adverse checks\n", 128)

	status, got := callLawyerAPIDo(t, api, map[string]any{
		"case_id":        "arb-1",
		"role_id":        "plaintiff",
		"opportunity_id": "openings:plaintiff",
		"tool":           "send_work_notes",
		"call_id":        "notes-1",
		"arguments": map[string]any{
			"notes": notes,
		},
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %#v, want true", got["ok"])
	}
	if turn.completed {
		t.Fatalf("send_work_notes completed the turn")
	}
	if turn.attemptsRemaining != turn.attemptsMax {
		t.Fatalf("attemptsRemaining = %d, want %d", turn.attemptsRemaining, turn.attemptsMax)
	}
	if len(api.rc.events) != 0 {
		t.Fatalf("send_work_notes recorded case events: %#v", api.rc.events)
	}
	raw, err := os.ReadFile(api.rc.cfg.OutputDir + "/work-notes.ndjson")
	if err != nil {
		t.Fatalf("read work-notes.ndjson: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("work note lines = %d, want 1: %s", len(lines), raw)
	}
	var note WorkNote
	if err := json.Unmarshal([]byte(lines[0]), &note); err != nil {
		t.Fatalf("decode work note: %v", err)
	}
	if note.Notes != notes {
		t.Fatalf("notes were not preserved exactly")
	}
	if note.Role != "plaintiff" || note.Phase != "openings" || note.OpportunityID != "openings:plaintiff" || note.CallID != "notes-1" {
		t.Fatalf("unexpected note metadata: %#v", note)
	}
}

func TestLawyerToolSpecsIncludeWorkNotesEveryPhase(t *testing.T) {
	for _, phase := range []string{"openings", "arguments", "rebuttals", "surrebuttals", "closings"} {
		tools := lawyerToolSpecs(Opportunity{Phase: phase})
		if !hasLawyerTool(tools, "send_work_notes") {
			t.Fatalf("%s tools missing send_work_notes: %#v", phase, tools)
		}
	}
}

func TestLawyerWaitReturnsReadyOpportunity(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()

	status, got := callLawyerAPIWait(t, api, "case_id=arb-1&role_id=plaintiff&timeout_ms=100")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "ready" {
		t.Fatalf("status field = %#v, want ready", got["status"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok {
		t.Fatalf("wait = %#v, want object", got["wait"])
	}
	if wait["reason"] != "ready" {
		t.Fatalf("wait.reason = %#v, want ready", wait["reason"])
	}
	turn, ok := got["turn"].(map[string]any)
	if !ok {
		t.Fatalf("turn = %#v, want object", got["turn"])
	}
	if turn["opportunity_id"] != "openings:plaintiff" {
		t.Fatalf("opportunity_id = %#v, want openings:plaintiff", turn["opportunity_id"])
	}
}

func TestLawyerWaitBlocksUntilLaterOpportunity(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()
	done := make(chan map[string]any, 1)
	go func() {
		_, got := callLawyerAPIWait(t, api, "case_id=arb-1&role_id=plaintiff&after=openings:plaintiff&timeout_ms=1000")
		done <- got
	}()
	select {
	case got := <-done:
		t.Fatalf("wait returned before a later opportunity: %#v", got)
	case <-time.After(20 * time.Millisecond):
	}
	api.mu.Lock()
	api.active = testLawyerTurn("arguments:plaintiff", "plaintiff", "arguments")
	api.signalChangedLocked()
	api.mu.Unlock()
	select {
	case got := <-done:
		if got["status"] != "ready" {
			t.Fatalf("status field = %#v, want ready", got["status"])
		}
		wait, ok := got["wait"].(map[string]any)
		if !ok {
			t.Fatalf("wait = %#v, want object", got["wait"])
		}
		if wait["reason"] != "ready" {
			t.Fatalf("wait.reason = %#v, want ready", wait["reason"])
		}
		turn, ok := got["turn"].(map[string]any)
		if !ok {
			t.Fatalf("turn = %#v, want object", got["turn"])
		}
		if turn["opportunity_id"] != "arguments:plaintiff" {
			t.Fatalf("opportunity_id = %#v, want arguments:plaintiff", turn["opportunity_id"])
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not return after later opportunity")
	}
}

func TestLawyerWaitReturnsOnCaseChange(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()
	done := make(chan map[string]any, 1)
	go func() {
		_, got := callLawyerAPIWait(t, api, "case_id=arb-1&role_id=plaintiff&after=openings:plaintiff&timeout_ms=1000")
		done <- got
	}()
	select {
	case got := <-done:
		t.Fatalf("wait returned before case change: %#v", got)
	case <-time.After(20 * time.Millisecond):
	}
	api.signalChanged()
	select {
	case got := <-done:
		if got["status"] != "ready" {
			t.Fatalf("status field = %#v, want ready", got["status"])
		}
		wait, ok := got["wait"].(map[string]any)
		if !ok {
			t.Fatalf("wait = %#v, want object", got["wait"])
		}
		if wait["reason"] != "changed" {
			t.Fatalf("wait.reason = %#v, want changed", wait["reason"])
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not return after case change")
	}
}

func TestLawyerWaitReturnsForOlderVersion(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()

	status, got := callLawyerAPIWait(t, api, "case_id=arb-1&role_id=plaintiff&after=openings:plaintiff&after_version=0&timeout_ms=1000")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok {
		t.Fatalf("wait = %#v, want object", got["wait"])
	}
	if wait["reason"] != "changed" {
		t.Fatalf("wait.reason = %#v, want changed", wait["reason"])
	}
}

func TestLawyerWaitReturnsDoneOnTerminalCase(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()
	api.setTerminal("demonstrated")

	status, got := callLawyerAPIWait(t, api, "case_id=arb-1&role_id=plaintiff&after=openings:plaintiff&timeout_ms=1000")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "done" {
		t.Fatalf("status field = %#v, want done", got["status"])
	}
	if got["final_reason"] != "demonstrated" {
		t.Fatalf("final_reason = %#v, want demonstrated", got["final_reason"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok {
		t.Fatalf("wait = %#v, want object", got["wait"])
	}
	if wait["reason"] != "done" {
		t.Fatalf("wait.reason = %#v, want done", wait["reason"])
	}
}

func TestLawyerWaitTimeout(t *testing.T) {
	api, _ := testLawyerAPIWithTurn()

	status, got := callLawyerAPIWait(t, api, "case_id=arb-1&role_id=plaintiff&after=openings:plaintiff&timeout_ms=10")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok {
		t.Fatalf("wait = %#v, want object", got["wait"])
	}
	if wait["reason"] != "timeout" {
		t.Fatalf("wait.reason = %#v, want timeout", wait["reason"])
	}
}

func testLawyerAPIWithTurn() (*lawyerAPIServer, *lawyerTurn) {
	policy := DefaultPolicy()
	turn := testLawyerTurn("openings:plaintiff", "plaintiff", "openings")
	rc := &runContext{
		cfg: Config{
			Policy:  policy,
			Runtime: DefaultRuntimeLimits(),
		},
		state:          initialState(policy),
		fileByID:       map[string]CaseFile{},
		uploadSessions: map[string]*EvidenceUploadSession{},
	}
	return &lawyerAPIServer{rc: rc, active: turn, version: 1}, turn
}

func testLawyerTurn(id string, role string, phase string) *lawyerTurn {
	return &lawyerTurn{
		opportunity: Opportunity{
			ID:           id,
			Role:         role,
			Phase:        phase,
			AllowedTools: []string{"record_opening_statement"},
		},
		turnNumber:        1,
		deadline:          time.Now().Add(time.Minute),
		attemptsMax:       3,
		attemptsRemaining: 3,
		evidenceBudget:    &evidenceReadBudget{},
		done:              make(chan error, 1),
	}
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

func callLawyerAPIWait(t *testing.T, api *lawyerAPIServer, query string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, lawyerAPIBasePath+"/wait?"+query, nil)
	rec := httptest.NewRecorder()
	api.handleWait(rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}

func hasLawyerTool(tools []map[string]any, name string) bool {
	for _, tool := range tools {
		if mapString(tool["name"]) == name {
			return true
		}
	}
	return false
}

func lawyerAPIErrorCode(t *testing.T, got map[string]any) string {
	t.Helper()
	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %#v, want object", got["error"])
	}
	return mapString(errObj["code"])
}
