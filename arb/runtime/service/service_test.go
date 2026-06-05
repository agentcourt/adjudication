package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestJoinBaseAndPathUsesSingleCaseAPIBase(t *testing.T) {
	u, err := joinBaseAndPath("http://127.0.0.1:21431", "/lawyerapi/v1/get")
	if err != nil {
		t.Fatalf("join lawyer path: %v", err)
	}
	if got := u.String(); got != "http://127.0.0.1:21431/lawyerapi/v1/get" {
		t.Fatalf("lawyer target = %q", got)
	}
	u, err = joinBaseAndPath("http://127.0.0.1:21431", "/councilapi/v1/wait")
	if err != nil {
		t.Fatalf("join council path: %v", err)
	}
	if got := u.String(); got != "http://127.0.0.1:21431/councilapi/v1/wait" {
		t.Fatalf("council target = %q", got)
	}
}

func TestClerkCreateCompletesAndListsRecord(t *testing.T) {
	root := t.TempDir()
	aarBin := writeFakeAAR(t, `#!/bin/sh
if [ "$1" != "run" ]; then exit 64; fi
shift
case_id=""
run_id=""
out_dir=""
complaint=""
example=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --case-id) case_id="$2"; shift 2 ;;
    --run-id) run_id="$2"; shift 2 ;;
    --out-dir) out_dir="$2"; shift 2 ;;
    --complaint) complaint="$2"; shift 2 ;;
    --*) shift 2 ;;
    *) example="$1"; shift ;;
  esac
done
mkdir -p "$out_dir"
printf '{"case_id":"%s","run_id":"%s","status":"ok","resolution":"demonstrated","example":"%s","complaint":"%s"}\n' "$case_id" "$run_id" "$example" "$complaint"
`)
	s := newClerkTestServer(t, root, aarBin)
	complaint := filepath.Join(t.TempDir(), "complaint.md")
	if err := os.WriteFile(complaint, []byte("# Complaint\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}

	status, got := servicePost(t, s, "/clerk/v1/cases", map[string]any{
		"case_id":        "clerk-1",
		"run_id":         "run-clerk-1",
		"complaint_path": complaint,
		"auto_lawyers":   "defendant",
	})
	if status != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %#v", status, http.StatusAccepted, got)
	}
	rec := waitClerkStatus(t, s, "clerk-1", "completed")
	if rec["run_id"] != "run-clerk-1" {
		t.Fatalf("run_id = %#v", rec["run_id"])
	}
	if _, err := os.Stat(filepath.Join(root, "clerk-1", clerkRecordName)); err != nil {
		t.Fatalf("stat clerk record: %v", err)
	}
	summary, ok := rec["summary"].(map[string]any)
	if !ok || summary["status"] != "ok" || summary["resolution"] != "demonstrated" {
		t.Fatalf("summary = %#v", rec["summary"])
	}

	status, got = servicePost(t, s, "/clerk/v1/cases", map[string]any{
		"case_id":        "clerk-1",
		"complaint_path": complaint,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("duplicate status = %d, want %d", status, http.StatusBadRequest)
	}
}

func TestClerkCreateRejectsMissingComplaintWithoutExample(t *testing.T) {
	root := t.TempDir()
	aarBin := writeFakeAAR(t, "#!/bin/sh\nexit 0\n")
	s := newClerkTestServer(t, root, aarBin)

	status, got := servicePost(t, s, "/clerk/v1/cases", map[string]any{"case_id": "missing-complaint"})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	errObj, ok := got["error"].(map[string]any)
	if !ok || !strings.Contains(mapString(errObj["message"]), "complaint_path is required") {
		t.Fatalf("error = %#v", got["error"])
	}
}

func TestDirectCreateRejectsOutputDirOutsideOutputRoot(t *testing.T) {
	root := t.TempDir()
	aarBin := writeFakeAAR(t, "#!/bin/sh\nexit 0\n")
	s := newClerkTestServer(t, root, aarBin)
	complaint := filepath.Join(t.TempDir(), "complaint.md")
	if err := os.WriteFile(complaint, []byte("# Complaint\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}

	status, got := servicePost(t, s, "/api/v1/cases", map[string]any{
		"case_id":        "direct-outside",
		"complaint_path": complaint,
		"out_dir":        filepath.Join(t.TempDir(), "outside"),
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %#v", status, http.StatusBadRequest, got)
	}
	errObj, ok := got["error"].(map[string]any)
	if !ok || !strings.Contains(mapString(errObj["message"]), "out_dir must be an immediate child") {
		t.Fatalf("error = %#v", got["error"])
	}
}

func TestCreateRejectsPathCaseIDs(t *testing.T) {
	root := t.TempDir()
	aarBin := writeFakeAAR(t, "#!/bin/sh\nexit 0\n")
	s := newClerkTestServer(t, root, aarBin)
	complaint := filepath.Join(t.TempDir(), "complaint.md")
	if err := os.WriteFile(complaint, []byte("# Complaint\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	for _, route := range []string{"/api/v1/cases", "/clerk/v1/cases"} {
		for _, caseID := range []string{".", ".."} {
			status, got := servicePost(t, s, route, map[string]any{
				"case_id":        caseID,
				"complaint_path": complaint,
			})
			if status != http.StatusBadRequest {
				t.Fatalf("%s case_id %q status = %d, body = %#v", route, caseID, status, got)
			}
			errObj, ok := got["error"].(map[string]any)
			if !ok || !strings.Contains(mapString(errObj["message"]), "case_id is invalid") {
				t.Fatalf("%s case_id %q error = %#v", route, caseID, got["error"])
			}
		}
	}
}

func TestListedArtifactNameRequiresExactName(t *testing.T) {
	if !listedArtifactName("digest.md") {
		t.Fatalf("digest.md should be listed")
	}
	for _, name := range []string{"/digest.md", "logs/../digest.md", "digest.md/", " digest.md"} {
		if listedArtifactName(name) {
			t.Fatalf("%q should not be listed", name)
		}
	}
}

func TestClerkKillTerminatesActiveRun(t *testing.T) {
	root := t.TempDir()
	aarBin := writeFakeAAR(t, `#!/bin/sh
if [ "$1" != "run" ]; then exit 64; fi
trap 'exit 0' INT TERM
while :; do sleep 1; done
`)
	s := newClerkTestServer(t, root, aarBin)
	complaint := filepath.Join(t.TempDir(), "complaint.md")
	if err := os.WriteFile(complaint, []byte("# Complaint\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	status, got := servicePost(t, s, "/clerk/v1/cases", map[string]any{
		"case_id":        "clerk-kill",
		"complaint_path": complaint,
	})
	if status != http.StatusAccepted {
		t.Fatalf("create status = %d, want %d: %#v", status, http.StatusAccepted, got)
	}

	status, got = servicePost(t, s, "/clerk/v1/cases/clerk-kill/kill", map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("kill status = %d, want %d: %#v", status, http.StatusOK, got)
	}
	rec := waitClerkStatus(t, s, "clerk-kill", "killed")
	if rec["pid"] != nil {
		t.Fatalf("pid = %#v, want omitted", rec["pid"])
	}
}

func TestClerkListReadsExistingRecordsFromOutputRoot(t *testing.T) {
	root := t.TempDir()
	aarBin := writeFakeAAR(t, "#!/bin/sh\nexit 0\n")
	outDir := filepath.Join(root, "existing")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out dir: %v", err)
	}
	rec := ClerkRecord{
		CaseID:    "existing",
		RunID:     "run-existing",
		Status:    "completed",
		OutDir:    outDir,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal clerk record: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, clerkRecordName), raw, 0o644); err != nil {
		t.Fatalf("write clerk record: %v", err)
	}
	s := newClerkTestServer(t, root, aarBin)

	status, got := serviceGet(t, s, "/clerk/v1/cases")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	cases, ok := got["cases"].([]any)
	if !ok || len(cases) != 1 {
		t.Fatalf("cases = %#v", got["cases"])
	}
	listed, ok := cases[0].(map[string]any)
	if !ok || listed["case_id"] != "existing" || listed["status"] != "completed" {
		t.Fatalf("listed = %#v", cases[0])
	}
}

func TestClerkRoutesReadOutputArtifacts(t *testing.T) {
	root := t.TempDir()
	aarBin := writeFakeAAR(t, "#!/bin/sh\nexit 0\n")
	outDir := filepath.Join(root, "clerk-rich")
	if err := os.MkdirAll(filepath.Join(outDir, "submitted-evidence"), 0o755); err != nil {
		t.Fatalf("mkdir out dir: %v", err)
	}
	rec := ClerkRecord{
		CaseID:    "clerk-rich",
		RunID:     "run-clerk-rich",
		Status:    "completed",
		OutDir:    outDir,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	writeClerkRecord(t, outDir, rec)
	writeJSONFile(t, filepath.Join(outDir, "run.json"), map[string]any{
		"status":       "completed",
		"phase":        "complete",
		"resolution":   "demonstrated",
		"final_reason": "test result",
		"final_state": map[string]any{
			"case": map[string]any{
				"status":             "completed",
				"deliberation_round": 1,
				"council_votes": []map[string]any{
					{"member_id": "C1", "vote": "demonstrated"},
				},
			},
		},
	})
	if err := os.WriteFile(filepath.Join(outDir, "digest.md"), []byte("digest text\n"), 0o644); err != nil {
		t.Fatalf("write digest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(outDir, "logs"), 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "logs", "mcp.stderr"), []byte("secret log\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "openclaw-plaintiff-lawyer-skill.md"), []byte("bearer token\n"), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	writeJSONFile(t, filepath.Join(outDir, "evidence-manifest.json"), []map[string]any{
		{"evidence_id": "EV1", "name": "ev1.txt"},
	})
	if err := os.WriteFile(filepath.Join(outDir, "submitted-evidence", "ev1.txt"), []byte("evidence text\n"), 0o644); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(outDir, "transcript.md")); err != nil {
		t.Fatalf("symlink transcript: %v", err)
	}
	s := newClerkTestServer(t, root, aarBin)

	status, got := serviceGet(t, s, "/clerk/v1/cases/clerk-rich")
	if status != http.StatusOK {
		t.Fatalf("inspect status = %d, body = %#v", status, got)
	}
	caseObj, ok := got["case"].(map[string]any)
	if !ok || caseObj["case_id"] != "clerk-rich" || caseObj["status"] != "completed" {
		t.Fatalf("case = %#v", got["case"])
	}

	status, got = serviceGet(t, s, "/clerk/v1/cases/clerk-rich/result")
	if status != http.StatusOK || got["status"] != "done" {
		t.Fatalf("result status = %d, body = %#v", status, got)
	}
	result, ok := got["result"].(map[string]any)
	if !ok || result["resolution"] != "demonstrated" {
		t.Fatalf("result = %#v", got["result"])
	}

	status, got = serviceGet(t, s, "/clerk/v1/cases/clerk-rich/artifacts")
	if status != http.StatusOK {
		t.Fatalf("artifacts status = %d, body = %#v", status, got)
	}
	if !artifactListContains(got["artifacts"], "run.json") || !artifactListContains(got["artifacts"], "digest.md") {
		t.Fatalf("artifacts = %#v", got["artifacts"])
	}
	if artifactListContains(got["artifacts"], "transcript.md") {
		t.Fatalf("unsafe symlink listed in artifacts = %#v", got["artifacts"])
	}

	rawStatus, body := serviceRawGet(t, s, "/clerk/v1/cases/clerk-rich/artifacts/digest.md")
	if rawStatus != http.StatusOK || string(body) != "digest text\n" {
		t.Fatalf("digest status = %d body = %q", rawStatus, string(body))
	}
	rawStatus, body = serviceRawGet(t, s, "/clerk/v1/cases/clerk-rich/evidence/EV1")
	if rawStatus != http.StatusOK || string(body) != "evidence text\n" {
		t.Fatalf("evidence status = %d body = %q", rawStatus, string(body))
	}
	for _, path := range []string{
		"/clerk/v1/cases/clerk-rich/artifacts/logs/mcp.stderr",
		"/clerk/v1/cases/clerk-rich/artifacts/openclaw-plaintiff-lawyer-skill.md",
	} {
		status, got = serviceGet(t, s, path)
		if status != http.StatusNotFound {
			t.Fatalf("%s status = %d, body = %#v", path, status, got)
		}
	}
	status, got = serviceGet(t, s, "/clerk/v1/cases/clerk-rich/artifacts/transcript.md")
	if status != http.StatusBadRequest {
		t.Fatalf("transcript symlink status = %d, body = %#v", status, got)
	}
}

func TestClerkKillRejectsDiskOnlyActiveRecord(t *testing.T) {
	root := t.TempDir()
	aarBin := writeFakeAAR(t, "#!/bin/sh\nexit 0\n")
	outDir := filepath.Join(root, "active-disk")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out dir: %v", err)
	}
	rec := ClerkRecord{
		CaseID:    "active-disk",
		RunID:     "run-active-disk",
		Status:    "running",
		OutDir:    outDir,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal clerk record: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, clerkRecordName), raw, 0o644); err != nil {
		t.Fatalf("write clerk record: %v", err)
	}
	s := newClerkTestServer(t, root, aarBin)

	status, got := servicePost(t, s, "/clerk/v1/cases/active-disk/kill", map[string]any{})
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %#v", status, http.StatusConflict, got)
	}
	if got["ok"] != false {
		t.Fatalf("ok = %#v, want false", got["ok"])
	}
	errObj, ok := got["error"].(map[string]any)
	if !ok || errObj["code"] != "case_not_attached" {
		t.Fatalf("error = %#v", got["error"])
	}
}

func TestStartupPollMarksRunningFromHealth(t *testing.T) {
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("unexpected health path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer health.Close()
	s, rec := testServerWithStartingCase(t)
	rec.CaseAPIBase = health.URL

	s.pollCaseAPIStartup(rec, time.Second)

	if rec.Status != "running" {
		t.Fatalf("status = %q, want running", rec.Status)
	}
}

func TestStartupPollMarksFailedAfterTimeout(t *testing.T) {
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer health.Close()
	s, rec := testServerWithStartingCase(t)
	rec.CaseAPIBase = health.URL

	s.pollCaseAPIStartup(rec, 20*time.Millisecond)

	if rec.Status != "failed" {
		t.Fatalf("status = %q, want failed", rec.Status)
	}
	if !strings.Contains(rec.Error, "case API did not become healthy") {
		t.Fatalf("error = %q", rec.Error)
	}
}

func TestCaptureStderrDoesNotSetControlState(t *testing.T) {
	s, rec := testServerWithStartingCase(t)
	rec.CaseAPIBase = "http://127.0.0.1:21431"
	logPath := filepath.Join(t.TempDir(), "stderr.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create stderr log: %v", err)
	}

	s.captureStderr(rec, strings.NewReader("lawyerapi listening on http://127.0.0.1:1/lawyerapi/v1\ncouncilapi listening on http://127.0.0.1:2/councilapi/v1\n"), logFile)

	if rec.CaseAPIBase != "http://127.0.0.1:21431" {
		t.Fatalf("caseapi base changed to %q", rec.CaseAPIBase)
	}
	if rec.Status != "starting" {
		t.Fatalf("status = %q, want starting", rec.Status)
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
		cfg: Config{
			RegistryDir: t.TempDir(),
		},
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

func writeFakeAAR(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aar")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake aar: %v", err)
	}
	return path
}

func newClerkTestServer(t *testing.T, outputRoot string, aarBin string) *Server {
	t.Helper()
	s, err := New(Config{
		RegistryDir: filepath.Join(t.TempDir(), "registry"),
		OutputRoot:  outputRoot,
		AARBin:      aarBin,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return s
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

func serviceRawGet(t *testing.T, s *Server, path string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func servicePost(t *testing.T, s *Server, path string, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal post body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, got
}

func writeClerkRecord(t *testing.T, outDir string, rec ClerkRecord) {
	t.Helper()
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatalf("marshal clerk record: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, clerkRecordName), raw, 0o644); err != nil {
		t.Fatalf("write clerk record: %v", err)
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json file: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write json file: %v", err)
	}
}

func artifactListContains(value any, name string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if ok && obj["name"] == name {
			return true
		}
	}
	return false
}

func waitClerkStatus(t *testing.T, s *Server, caseID string, want string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last map[string]any
	for time.Now().Before(deadline) {
		status, got := serviceGet(t, s, "/clerk/v1/cases")
		if status != http.StatusOK {
			t.Fatalf("list status = %d, want %d", status, http.StatusOK)
		}
		cases, ok := got["cases"].([]any)
		if !ok {
			t.Fatalf("cases = %#v", got["cases"])
		}
		for _, item := range cases {
			rec, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("case item = %#v", item)
			}
			if rec["case_id"] == caseID {
				last = rec
				if rec["status"] == want {
					return rec
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("case %s did not reach status %s; last = %#v", caseID, want, last)
	return nil
}
