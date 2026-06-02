package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCompletedLawyerReadsReturnDoneFromArtifact(t *testing.T) {
	s, rec := testServerWithCompletedCase(t)

	status, got := serviceGet(t, s, "/lawyerapi/v1/wait?case_id=case-1&role_id=plaintiff&timeout_ms=1")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "done" {
		t.Fatalf("status field = %#v, want done", got["status"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok || wait["reason"] != "done" {
		t.Fatalf("wait = %#v, want reason done", got["wait"])
	}

	status, got = serviceGet(t, s, "/lawyerapi/v1/result?case_id=case-1&role_id=observer")
	if status != http.StatusOK {
		t.Fatalf("result status = %d, want %d", status, http.StatusOK)
	}
	result, ok := got["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want object", got["result"])
	}
	if result["resolution"] != "demonstrated" {
		t.Fatalf("resolution = %#v", result["resolution"])
	}
	tally, ok := result["vote_tally"].(map[string]any)
	if !ok || intNumber(tally["demonstrated"]) != 1 || intNumber(tally["not_demonstrated"]) != 1 {
		t.Fatalf("vote_tally = %#v", result["vote_tally"])
	}

	if rec.Status != "completed" {
		t.Fatalf("record status = %q", rec.Status)
	}
}

func TestCompletedCouncilWaitReturnsDoneFromArtifact(t *testing.T) {
	s, _ := testServerWithCompletedCase(t)

	status, got := serviceGet(t, s, "/councilapi/v1/wait?case_id=case-1&member_id=C1&timeout_ms=1")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "done" {
		t.Fatalf("status field = %#v, want done", got["status"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok || wait["reason"] != "done" {
		t.Fatalf("wait = %#v, want reason done", got["wait"])
	}
}

func TestCompletedLawyerReadsReturnFailedFromArtifact(t *testing.T) {
	s, _ := testServerWithFailedCase(t)

	status, got := serviceGet(t, s, "/lawyerapi/v1/wait?case_id=case-1&role_id=plaintiff&timeout_ms=1")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "failed" {
		t.Fatalf("status field = %#v, want failed", got["status"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok || wait["reason"] != "failed" {
		t.Fatalf("wait = %#v, want reason failed", got["wait"])
	}

	status, got = serviceGet(t, s, "/api/v1/cases/case-1/result")
	if status != http.StatusOK {
		t.Fatalf("result status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "failed" {
		t.Fatalf("result status field = %#v, want failed", got["status"])
	}
	failure, ok := got["failure"].(map[string]any)
	if !ok || failure["type"] != "opportunity_failed" || failure["reason"] != "deadline_expired" {
		t.Fatalf("failure = %#v", got["failure"])
	}
}

func TestStartingLawyerWaitReturnsWaiting(t *testing.T) {
	s, _ := testServerWithStartingCase(t)

	status, got := serviceGet(t, s, "/lawyerapi/v1/wait?case_id=case-1&role_id=plaintiff&timeout_ms=1")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "waiting" {
		t.Fatalf("status field = %#v, want waiting", got["status"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok || wait["reason"] != "starting" {
		t.Fatalf("wait = %#v, want reason starting", got["wait"])
	}
}

func TestStartingCouncilWaitReturnsWaiting(t *testing.T) {
	s, _ := testServerWithStartingCase(t)

	status, got := serviceGet(t, s, "/councilapi/v1/wait?case_id=case-1&member_id=C1&timeout_ms=1")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got["status"] != "waiting" {
		t.Fatalf("status field = %#v, want waiting", got["status"])
	}
	wait, ok := got["wait"].(map[string]any)
	if !ok || wait["reason"] != "starting" {
		t.Fatalf("wait = %#v, want reason starting", got["wait"])
	}
}

func testServerWithCompletedCase(t *testing.T) (*Server, *CaseRecord) {
	t.Helper()
	root := t.TempDir()
	outDir := filepath.Join(root, "case-1")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}
	run := map[string]any{
		"case_id":      "case-1",
		"run_id":       "run-case-1",
		"status":       "ok",
		"phase":        "closed",
		"resolution":   "demonstrated",
		"final_reason": "threshold_met",
		"final_state": map[string]any{
			"state_version": 12,
			"case": map[string]any{
				"status":             "closed",
				"deliberation_round": 1,
				"council_votes": []map[string]any{
					{"round": 1, "member_id": "C1", "vote": "demonstrated", "rationale": "record proves it"},
					{"round": 1, "member_id": "C2", "vote": "not_demonstrated", "rationale": "gap remains"},
				},
			},
		},
	}
	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "run.json"), raw, 0o644); err != nil {
		t.Fatalf("write run.json: %v", err)
	}
	s := &Server{
		cases:  map[string]*CaseRecord{},
		client: &http.Client{},
	}
	s.cond = syncCond(&s.mu)
	rec := &CaseRecord{
		CaseID:         "case-1",
		RunID:          "run-case-1",
		Status:         "completed",
		OutputDir:      outDir,
		CouncilBackend: "councilapi",
	}
	s.cases[rec.CaseID] = rec
	return s, rec
}

func testServerWithFailedCase(t *testing.T) (*Server, *CaseRecord) {
	t.Helper()
	root := t.TempDir()
	outDir := filepath.Join(root, "case-1")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}
	failure := map[string]any{
		"type":           "opportunity_failed",
		"role":           "plaintiff",
		"phase":          "arguments",
		"opportunity_id": "arguments:plaintiff",
		"reason":         "deadline_expired",
		"message":        "Plaintiff lawyer opportunity arguments:plaintiff failed because the deadline expired.",
	}
	run := map[string]any{
		"case_id":      "case-1",
		"run_id":       "run-case-1",
		"status":       "failed",
		"phase":        "arguments",
		"error":        failure["message"],
		"failure":      failure,
		"final_reason": "deadline_expired",
		"final_state": map[string]any{
			"state_version": 4,
			"case": map[string]any{
				"status":             "failed",
				"phase":              "arguments",
				"deliberation_round": 0,
				"council_votes":      []map[string]any{},
				"failure":            failure,
			},
		},
	}
	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "run.json"), raw, 0o644); err != nil {
		t.Fatalf("write run.json: %v", err)
	}
	s := &Server{
		cases:  map[string]*CaseRecord{},
		client: &http.Client{},
	}
	s.cond = syncCond(&s.mu)
	rec := &CaseRecord{
		CaseID:         "case-1",
		RunID:          "run-case-1",
		Status:         "failed",
		OutputDir:      outDir,
		CouncilBackend: "councilapi",
		Error:          mapString(failure["message"]),
	}
	s.cases[rec.CaseID] = rec
	return s, rec
}

func testServerWithStartingCase(t *testing.T) (*Server, *CaseRecord) {
	t.Helper()
	s := &Server{
		cases:  map[string]*CaseRecord{},
		client: &http.Client{},
	}
	s.cond = syncCond(&s.mu)
	rec := &CaseRecord{
		CaseID:         "case-1",
		RunID:          "run-case-1",
		Status:         "starting",
		CouncilBackend: "councilapi",
	}
	s.cases[rec.CaseID] = rec
	return s, rec
}

func syncCond(mu *sync.Mutex) *sync.Cond {
	return sync.NewCond(mu)
}

func serviceGet(t *testing.T, s *Server, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}
