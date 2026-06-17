package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestJoinBaseAndPathUsesRoleAPIPath(t *testing.T) {
	u, err := joinBaseAndPath("http://127.0.0.1:21431", "/roleapi/v1/get")
	if err != nil {
		t.Fatalf("join path: %v", err)
	}
	if got := u.String(); got != "http://127.0.0.1:21431/roleapi/v1/get" {
		t.Fatalf("target = %q", got)
	}

	u, err = joinBaseAndPath("http://127.0.0.1:21431/private", "/roleapi/v1/wait_for_opportunity")
	if err != nil {
		t.Fatalf("join private path: %v", err)
	}
	if got := u.String(); got != "http://127.0.0.1:21431/private/wait_for_opportunity" {
		t.Fatalf("private target = %q", got)
	}
}

func TestLoadCaseRecordsMarksDetachedActiveCaseFailed(t *testing.T) {
	root := t.TempDir()
	outDir := filepath.Join(root, "case-1")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}
	writeServiceRecord(t, outDir, CaseRecord{
		CaseID:    "case-1",
		RunID:     "run-case-1",
		Status:    "running",
		OutputDir: outDir,
	})

	s, err := New(Config{OutputRoot: root, ADCBin: "/bin/false"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	rec, ok := s.getCase("case-1")
	if !ok {
		t.Fatalf("case record missing")
	}
	if rec.Status != "failed" {
		t.Fatalf("status = %q, want failed", rec.Status)
	}
	if rec.Error != "service restarted and child process is not attached" {
		t.Fatalf("error = %q", rec.Error)
	}
}

func TestRoleAPIProxyForwardsGetAndPost(t *testing.T) {
	var sawGet bool
	var sawPostBody map[string]any
	private := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /roleapi/v1/status":
			sawGet = true
			if r.URL.Query().Get("case_id") != "case-1" {
				t.Fatalf("GET query = %s", r.URL.RawQuery)
			}
			writeTestJSON(w, map[string]any{"ok": true, "case_id": "case-1", "status": "waiting"})
		case "POST /roleapi/v1/do":
			if err := json.NewDecoder(r.Body).Decode(&sawPostBody); err != nil {
				t.Fatalf("decode post body: %v", err)
			}
			writeTestJSON(w, map[string]any{"ok": true, "case_id": "case-1", "status": "active"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer private.Close()

	s := testServiceWithCase(t, CaseRecord{
		CaseID:      "case-1",
		RunID:       "run-case-1",
		Status:      "running",
		OutputDir:   t.TempDir(),
		CaseAPIBase: private.URL,
	})

	status, got := serviceGet(t, s, "/roleapi/v1/status?case_id=case-1&role_id=observer")
	if status != http.StatusOK || got["status"] != "waiting" || !sawGet {
		t.Fatalf("GET proxy status=%d body=%#v sawGet=%v", status, got, sawGet)
	}

	status, got = servicePost(t, s, "/roleapi/v1/do", map[string]any{"case_id": "case-1", "role_id": "plaintiff", "tool": "case_status"})
	if status != http.StatusOK || got["status"] != "active" {
		t.Fatalf("POST proxy status=%d body=%#v", status, got)
	}
	if sawPostBody["case_id"] != "case-1" || sawPostBody["tool"] != "case_status" {
		t.Fatalf("forwarded body = %#v", sawPostBody)
	}
}

func TestCaseProcessArgsDefaultsToRun(t *testing.T) {
	s := &Server{cfg: Config{EnginePath: "lake exe adc-engine"}}
	startDelay := 15
	unanimousRequired := false
	args := s.caseProcessArgs("", CaseCreateRequest{
		Model:                     "model-1",
		JurorPersonas:             "pool.jsonl",
		JurorCount:                8,
		MinimumConcurring:         6,
		UnanimousRequired:         &unanimousRequired,
		RoleAPITimeoutSeconds:     900,
		MCPListenAddr:             "127.0.0.1:8001",
		AutoLawyers:               "defendant",
		OpenClawAuth:              "codex",
		OpenClawCodexAuthPath:     "auth.json",
		OpenClawStartDelaySeconds: &startDelay,
		PiImage:                   "pi-image",
		JurorOutputLimitBytes:     4096,
		ExternalRoles:             []string{"plaintiff"},
		Offline:                   true,
	}, "case-1", "run-case-1", "/tmp/out", "127.0.0.1:9001", "", "/tmp/scenario.json")
	if args[0] != "run" {
		t.Fatalf("command = %#v", args)
	}
	for _, want := range []string{"--scenario", "/tmp/scenario.json", "--caseapi-addr", "127.0.0.1:9001", "--model", "model-1", "--juror-personas", "pool.jsonl", "--juror-count", "8", "--minimum-concurring", "6", "--unanimous-required", "false", "--offline", "--mcp-listen", "127.0.0.1:8001", "--lawyer-timeout-seconds", "900", "--juror-timeout-seconds", "900", "--auto-lawyers", "defendant", "--openclaw-auth", "codex", "--openclaw-codex-auth", "auth.json", "--openclaw-lawyer-start-delay-seconds", "15", "--pi-image", "pi-image", "--juror-output-limit-bytes", "4096"} {
		if !containsArg(args, want) {
			t.Fatalf("missing %q in %#v", want, args)
		}
	}
	for _, forbidden := range []string{"--external-role", "plaintiff"} {
		if containsArg(args, forbidden) {
			t.Fatalf("run args contain direct-mode value %q: %#v", forbidden, args)
		}
	}
}

func TestCaseProcessArgsKeepsExplicitZeroOpenClawStartDelay(t *testing.T) {
	s := &Server{cfg: Config{EnginePath: "lake exe adc-engine"}}
	startDelay := 0
	args := s.caseProcessArgs("", CaseCreateRequest{
		ScenarioPath:              "/tmp/scenario.json",
		OpenClawStartDelaySeconds: &startDelay,
	}, "case-1", "run-case-1", "/tmp/out", "127.0.0.1:9001", "", "/tmp/scenario.json")

	for i, arg := range args {
		if arg == "--openclaw-lawyer-start-delay-seconds" {
			if i+1 >= len(args) || args[i+1] != "0" {
				t.Fatalf("delay args = %#v", args)
			}
			return
		}
	}
	t.Fatalf("missing explicit zero delay in %#v", args)
}

func TestStartCaseRejectsOfflineComplaintSetup(t *testing.T) {
	root := t.TempDir()
	complaint := filepath.Join(root, "complaint.md")
	if err := os.WriteFile(complaint, []byte("# Complaint\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	s, err := New(Config{OutputRoot: root, ADCBin: "/bin/false"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	_, err = s.startCase(context.Background(), CaseCreateRequest{
		ComplaintPath: complaint,
		Offline:       true,
	})
	if err == nil || !strings.Contains(err.Error(), "offline mode cannot prepare a complaint-based case") {
		t.Fatalf("error = %v", err)
	}
}

func TestStartCaseRejectsOutputDirOutsideOutputRoot(t *testing.T) {
	root := t.TempDir()
	scenario := filepath.Join(root, "scenario.json")
	if err := os.WriteFile(scenario, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write scenario: %v", err)
	}
	outputRoot := filepath.Join(root, "service")
	outside := filepath.Join(root, "outside", "case-1")
	s, err := New(Config{OutputRoot: outputRoot, ADCBin: "/bin/false"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	_, err = s.startCase(context.Background(), CaseCreateRequest{
		CaseID:       "case-1",
		ScenarioPath: scenario,
		OutputDir:    outside,
	})
	if err == nil || !strings.Contains(err.Error(), "out_dir must be an immediate child") {
		t.Fatalf("error = %v", err)
	}
}

func TestStartCaseRejectsPathCaseIDs(t *testing.T) {
	root := t.TempDir()
	scenario := filepath.Join(root, "scenario.json")
	if err := os.WriteFile(scenario, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write scenario: %v", err)
	}
	s, err := New(Config{OutputRoot: filepath.Join(root, "service"), ADCBin: "/bin/false"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	for _, caseID := range []string{".", ".."} {
		_, err := s.startCase(context.Background(), CaseCreateRequest{
			CaseID:       caseID,
			ScenarioPath: scenario,
		})
		if err == nil || !strings.Contains(err.Error(), "case_id is invalid") {
			t.Fatalf("case_id %q error = %v", caseID, err)
		}
	}
}

func TestValidateServiceOutputDirAcceptsImmediateChild(t *testing.T) {
	root := t.TempDir()
	if err := validateServiceOutputDir(root, filepath.Join(root, "case-1")); err != nil {
		t.Fatalf("validate immediate child: %v", err)
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

func TestArtifactRouteServesOnlyListedArtifacts(t *testing.T) {
	outDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outDir, "logs"), 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "digest.md"), []byte("digest text\n"), 0o644); err != nil {
		t.Fatalf("write digest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "logs", "mcp.stderr"), []byte("secret log\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "openclaw-plaintiff-lawyer-skill.md"), []byte("bearer token\n"), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(outDir, "transcript.md")); err != nil {
		t.Fatalf("symlink transcript: %v", err)
	}
	s := testServiceWithCase(t, CaseRecord{
		CaseID:    "case-1",
		RunID:     "run-case-1",
		Status:    "completed",
		OutputDir: outDir,
	})

	status, body := serviceRawGet(t, s, "/api/v1/cases/case-1/artifacts/digest.md")
	if status != http.StatusOK || string(body) != "digest text\n" {
		t.Fatalf("digest status = %d body = %q", status, string(body))
	}
	status, got := serviceGet(t, s, "/api/v1/cases/case-1/artifacts")
	if status != http.StatusOK {
		t.Fatalf("artifacts status = %d body = %#v", status, got)
	}
	if !artifactListContains(got["artifacts"], "digest.md") || artifactListContains(got["artifacts"], "transcript.md") {
		t.Fatalf("artifacts = %#v", got["artifacts"])
	}
	for _, path := range []string{
		"/api/v1/cases/case-1/artifacts/logs/mcp.stderr",
		"/api/v1/cases/case-1/artifacts/openclaw-plaintiff-lawyer-skill.md",
	} {
		status, got := serviceGet(t, s, path)
		if status != http.StatusNotFound {
			t.Fatalf("%s status = %d body = %#v", path, status, got)
		}
	}
	status, got = serviceGet(t, s, "/api/v1/cases/case-1/artifacts/transcript.md")
	if status != http.StatusBadRequest {
		t.Fatalf("transcript symlink status = %d body = %#v", status, got)
	}
}

func TestCreateAttestedComplaintCompletesAfterVerification(t *testing.T) {
	root := t.TempDir()
	complaint := filepath.Join(root, "complaint.md")
	if err := os.WriteFile(complaint, []byte("# Complaint\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	outputRoot := filepath.Join(root, "service")
	s, err := New(Config{
		OutputRoot: outputRoot,
		ADCBin:     "/bin/false",
		Attested: AttestedClerkConfig{
			DriverPath:   writeFakeAttestedADCDriver(t, 0),
			InputPrefix:  "s3://bucket/adc-input",
			OutputRoot:   "s3://bucket/adc-output",
			ExecAMI:      "ami-123",
			ExpectedPCR4: strings.Repeat("a", 96),
			ExpectedPCR7: strings.Repeat("b", 96),
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	status, got := servicePost(t, s, "/clerk/v1/cases", map[string]any{
		"case_id":        "attested-1",
		"complaint_path": complaint,
		"execution": map[string]any{
			"mode":        "attested",
			"attestation": map[string]any{"verify": true},
		},
	})
	if status != http.StatusAccepted {
		t.Fatalf("status = %d body=%#v", status, got)
	}
	rec := waitCaseStatus(t, s, "attested-1", "completed")
	if rec.Execution == nil || rec.Execution.Attestation == nil || rec.Execution.Attestation.Status != attestationStatusVerified {
		t.Fatalf("execution = %#v", rec.Execution)
	}
	if rec.Execution.Resolved.OutputPrefix != "s3://bucket/adc-output/run-attested-1" {
		t.Fatalf("resolved output prefix = %q", rec.Execution.Resolved.OutputPrefix)
	}
	if rec.Summary["complaint"] != complaint || rec.Summary["case_id"] != "attested-1" {
		t.Fatalf("summary = %#v", rec.Summary)
	}
	runEnv, err := os.ReadFile(filepath.Join(outputRoot, "attested-1", "run.env"))
	if err != nil {
		t.Fatalf("read run.env: %v", err)
	}
	if !strings.Contains(string(runEnv), "ADC_INPUT_MODE=case-packet\n") || strings.Contains(string(runEnv), "ADC_EXAMPLE=") {
		t.Fatalf("run.env = %s", string(runEnv))
	}
	rawStatus, body := serviceRawGet(t, s, "/clerk/v1/cases/attested-1/artifacts/digest.md")
	if rawStatus != http.StatusOK || string(body) != "digest text\n" {
		t.Fatalf("digest status=%d body=%q", rawStatus, string(body))
	}
	rawStatus, body = serviceRawGet(t, s, "/clerk/v1/cases/attested-1/evidence/EV1")
	if rawStatus != http.StatusOK || string(body) != "evidence text\n" {
		t.Fatalf("evidence status=%d body=%q", rawStatus, string(body))
	}
	rawStatus, body = serviceRawGet(t, s, "/clerk/v1/cases/attested-1/attestation/events")
	if rawStatus != http.StatusOK || string(body) != "{\"event\":\"completed\",\"case_id\":\"attested-1\"}\n" {
		t.Fatalf("events status=%d body=%q", rawStatus, string(body))
	}
}

func TestCreateAttestedRejectsScenario(t *testing.T) {
	root := t.TempDir()
	scenario := filepath.Join(root, "scenario.json")
	if err := os.WriteFile(scenario, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write scenario: %v", err)
	}
	s, err := New(Config{
		OutputRoot: filepath.Join(root, "service"),
		ADCBin:     "/bin/false",
		Attested: AttestedClerkConfig{
			DriverPath:   writeFakeAttestedADCDriver(t, 0),
			InputPrefix:  "s3://bucket/adc-input",
			OutputRoot:   "s3://bucket/adc-output",
			ExecAMI:      "ami-123",
			ExpectedPCR4: strings.Repeat("a", 96),
			ExpectedPCR7: strings.Repeat("b", 96),
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	status, got := servicePost(t, s, "/clerk/v1/cases", map[string]any{
		"case_id":       "attested-scenario",
		"scenario_path": scenario,
		"execution": map[string]any{
			"mode":        "attested",
			"attestation": map[string]any{"verify": true},
		},
	})
	if status != http.StatusBadRequest || !strings.Contains(errorMessage(got), "complaint_path only") {
		t.Fatalf("status=%d body=%#v", status, got)
	}
}

func TestCreateAttestedRejectsLocalRunFields(t *testing.T) {
	root := t.TempDir()
	complaint := filepath.Join(root, "complaint.md")
	if err := os.WriteFile(complaint, []byte("# Complaint\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	s, err := New(Config{
		OutputRoot: filepath.Join(root, "service"),
		ADCBin:     "/bin/false",
		Attested: AttestedClerkConfig{
			DriverPath:   writeFakeAttestedADCDriver(t, 0),
			InputPrefix:  "s3://bucket/adc-input",
			OutputRoot:   "s3://bucket/adc-output",
			ExecAMI:      "ami-123",
			ExpectedPCR4: strings.Repeat("a", 96),
			ExpectedPCR7: strings.Repeat("b", 96),
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	status, got := servicePost(t, s, "/clerk/v1/cases", map[string]any{
		"case_id":        "attested-reject",
		"complaint_path": complaint,
		"model":          "model-1",
		"execution": map[string]any{
			"mode":        "attested",
			"attestation": map[string]any{"verify": true},
		},
	})
	if status != http.StatusBadRequest || !strings.Contains(errorMessage(got), "model") {
		t.Fatalf("status=%d body=%#v", status, got)
	}
}

func TestCaseProcessArgsForDirectExistingScenario(t *testing.T) {
	s := &Server{cfg: Config{EnginePath: "lake exe adc-engine"}}
	args := s.caseProcessArgs("direct", CaseCreateRequest{
		Model:                 "model-1",
		NonJurorModel:         "case-only-model",
		JurorPersonas:         "pool.jsonl",
		ExternalRoles:         []string{"plaintiff", "juror"},
		RoleAPITimeoutSeconds: 900,
	}, "case-1", "run-case-1", "/tmp/out", "127.0.0.1:9001", "", "/tmp/scenario.json")
	if args[0] != "scenario" {
		t.Fatalf("command = %#v", args)
	}
	for _, want := range []string{"--scenario", "/tmp/scenario.json", "--output", "/tmp/out/run.json", "--caseapi-addr", "127.0.0.1:9001", "--model", "model-1", "--juror-personas", "pool.jsonl", "--external-role", "plaintiff", "--external-role", "juror"} {
		if !containsArg(args, want) {
			t.Fatalf("missing %q in %#v", want, args)
		}
	}
	for _, forbidden := range []string{"--non-juror-model", "case-only-model", "--planner-model", "--skip-voir-dire"} {
		if containsArg(args, forbidden) {
			t.Fatalf("scenario args contain case-only value %q: %#v", forbidden, args)
		}
	}
}

func waitCaseStatus(t *testing.T, s *Server, caseID string, want string) CaseRecord {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		rec, ok := s.getCase(caseID)
		if ok && rec.Status == want {
			return rec
		}
		time.Sleep(20 * time.Millisecond)
	}
	rec, ok := s.getCase(caseID)
	if !ok {
		t.Fatalf("case %s missing", caseID)
	}
	t.Fatalf("case %s status = %q, want %q; error=%s", caseID, rec.Status, want, rec.Error)
	return CaseRecord{}
}

func writeFakeAttestedADCDriver(t *testing.T, exitCode int) string {
	t.Helper()
	code := "0"
	if exitCode != 0 {
		code = "7"
	}
	script := strings.ReplaceAll(`#!/bin/sh
out_dir=""
case_id=""
run_id=""
complaint=""
input_prefix=""
output_prefix=""
exec_ami=""
verify=0
allow_nonempty=0
expected4=""
expected7=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --case-id) case_id="$2"; shift 2 ;;
    --complaint) complaint="$2"; shift 2 ;;
    --example) exit 66 ;;
    --input-prefix) input_prefix="$2"; shift 2 ;;
    --output-prefix) output_prefix="$2"; shift 2 ;;
    --exec-ami) exec_ami="$2"; shift 2 ;;
    --run-id) run_id="$2"; shift 2 ;;
    --out-dir) out_dir="$2"; shift 2 ;;
    --verify) verify=1; shift ;;
    --allow-nonempty-out-dir) allow_nonempty=1; shift ;;
    --expected-pcr4) expected4="$2"; shift 2 ;;
    --expected-pcr7) expected7="$2"; shift 2 ;;
    --*) shift 2 ;;
    *) shift ;;
  esac
done
if [ -z "$out_dir" ] || [ -z "$run_id" ] || [ -z "$input_prefix" ] || [ -z "$exec_ami" ] || [ -z "$complaint" ]; then
  exit 64
fi
if [ "$verify" != "1" ] || [ "$allow_nonempty" != "1" ] || [ -z "$expected4" ] || [ -z "$expected7" ]; then
  exit 65
fi
mkdir -p "$out_dir"
printf 'ADC_INPUT_MODE=case-packet\nINPUT_PREFIX=%s\nOUTPUT_PREFIX=%s\nEXEC_AMI=%s\nADC_COMPLAINT=%s\nADC_CASE_ID=%s\n' "$input_prefix" "$output_prefix" "$exec_ami" "$complaint" "$case_id" > "$out_dir/run.env"
printf 'moving\n' > "$out_dir/progress.log"
printf 'launch\n' > "$out_dir/launcher.log"
printf '{"files":[]}\n' > "$out_dir/manifest.json"
printf 'sha384 test\n' > "$out_dir/manifest.sha384"
printf 'attestation text\n' > "$out_dir/attestation.txt"
printf 'verified\n' > "$out_dir/verification.log"
printf '{"event":"live","case_id":"%s"}\n' "$case_id" > "$out_dir/events.ndjson"
if [ "__EXIT_CODE__" != "0" ]; then
  exit __EXIT_CODE__
fi
mkdir -p "$out_dir/adc-output/submitted-evidence"
printf '{"case_id":"%s","run_id":"%s","status":"completed","phase":"complete","complaint":"%s"}\n' "$case_id" "$run_id" "$complaint" > "$out_dir/adc-output/run.json"
printf 'digest text\n' > "$out_dir/adc-output/digest.md"
printf '{"event":"completed","case_id":"%s"}\n' "$case_id" > "$out_dir/adc-output/events.ndjson"
printf '[{"evidence_id":"EV1","name":"ev1.txt"}]\n' > "$out_dir/adc-output/evidence-manifest.json"
printf 'evidence text\n' > "$out_dir/adc-output/submitted-evidence/ev1.txt"
exit 0
`, "__EXIT_CODE__", code)
	path := filepath.Join(t.TempDir(), "run-adc-attested")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake attested driver: %v", err)
	}
	return path
}

func errorMessage(got map[string]any) string {
	errObj, ok := got["error"].(map[string]any)
	if !ok {
		return ""
	}
	msg, _ := errObj["message"].(string)
	return msg
}

func TestKillRouteMarksCaseKilling(t *testing.T) {
	outDir := t.TempDir()
	cmd, cleanup := startedTestProcess(t)
	defer cleanup()
	s := testServiceWithCase(t, CaseRecord{
		CaseID:    "case-1",
		RunID:     "run-case-1",
		Status:    "running",
		OutputDir: outDir,
		cmd:       cmd,
	})

	status, got := servicePost(t, s, "/api/v1/cases/case-1/kill", map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("kill status = %d body=%#v", status, got)
	}
	caseObj, ok := got["case"].(map[string]any)
	if !ok || caseObj["status"] != "killing" {
		t.Fatalf("case = %#v", got["case"])
	}
	if _, err := os.Stat(filepath.Join(outDir, "service-case.json")); err != nil {
		t.Fatalf("service record missing: %v", err)
	}
}

func TestKillRouteRejectsTerminalCase(t *testing.T) {
	s := testServiceWithCase(t, CaseRecord{
		CaseID:    "case-1",
		RunID:     "run-case-1",
		Status:    "completed",
		OutputDir: t.TempDir(),
	})

	status, got := servicePost(t, s, "/api/v1/cases/case-1/kill", map[string]any{})
	if status != http.StatusConflict {
		t.Fatalf("kill status = %d body=%#v", status, got)
	}
	if got["status"] != "completed" {
		t.Fatalf("status = %#v", got["status"])
	}
	rec, ok := s.getCase("case-1")
	if !ok {
		t.Fatalf("case missing")
	}
	if rec.Status != "completed" {
		t.Fatalf("stored status = %q", rec.Status)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func testServiceWithCase(t *testing.T, rec CaseRecord) *Server {
	t.Helper()
	s := &Server{
		cfg:    Config{OutputRoot: t.TempDir(), ADCBin: "/bin/false"},
		cases:  map[string]*CaseRecord{},
		client: http.DefaultClient,
	}
	s.cond = sync.NewCond(&s.mu)
	copy := rec
	s.cases[copy.CaseID] = &copy
	return s
}

func startedTestProcess(t *testing.T) (*exec.Cmd, func()) {
	t.Helper()
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start test process: %v", err)
	}
	cleanup := func() {
		if cmd.ProcessState == nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}
	return cmd, cleanup
}

func serviceGet(t *testing.T, s *Server, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return decodeServiceResponse(t, rec)
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
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return decodeServiceResponse(t, rec)
}

func decodeServiceResponse(t *testing.T, rec *httptest.ResponseRecorder) (int, map[string]any) {
	t.Helper()
	var got map[string]any
	dec := json.NewDecoder(rec.Body)
	dec.UseNumber()
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("decode response status=%d body=%q: %v", rec.Code, rec.Body.String(), err)
	}
	return rec.Code, got
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

func writeServiceRecord(t *testing.T, outDir string, rec CaseRecord) {
	t.Helper()
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatalf("marshal service record: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "service-case.json"), raw, 0o644); err != nil {
		t.Fatalf("write service record: %v", err)
	}
}

func writeTestJSON(w http.ResponseWriter, value map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
