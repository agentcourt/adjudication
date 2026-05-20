package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"adjudication/arbd/runtime/runner"
)

func TestBuildCaseSuccessSummaryUsesAnswers(t *testing.T) {
	summary := buildCaseSuccessSummary(runner.Result{
		RunID:   "run-1",
		Answers: map[string]int{"C1": 72},
	}, "out/demo")
	if summary.Status != "ok" {
		t.Fatalf("summary status = %q, want ok", summary.Status)
	}
	if summary.Answers["C1"] != 72 {
		t.Fatalf("summary answers = %#v", summary.Answers)
	}
}

func TestWriteCaseSummaryWritesJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCaseSummary(&buf, caseRunSummary{
		Status:  "ok",
		Answers: map[string]int{"C1": 72},
	}); err != nil {
		t.Fatalf("writeCaseSummary returned error: %v", err)
	}
	var decoded caseRunSummary
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("summary is not valid JSON: %v", err)
	}
	if decoded.Answers["C1"] != 72 {
		t.Fatalf("decoded summary = %#v", decoded)
	}
}

func TestValidateExplicitCaseFilePathRejectsDotGitignore(t *testing.T) {
	if err := validateExplicitCaseFilePath(".gitignore"); err == nil {
		t.Fatal("validateExplicitCaseFilePath returned nil error, want failure")
	}
}

func TestRunCaseRejectsPlaintiffModelWithACPEndpoint(t *testing.T) {
	dir := t.TempDir()
	complaintPath := filepath.Join(dir, "complaint.md")
	if err := os.WriteFile(complaintPath, []byte("# Question\n\nWhat percentage is reused?\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunCase([]string{
		"--complaint", complaintPath,
		"--out-dir", filepath.Join(dir, "out"),
		"--plaintiff-attorney-model", "openai://gpt-5?tools=search",
		"--plaintiff-acp-endpoint", "tcp://127.0.0.1:7000",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("RunCase returned nil error, want failure")
	}
	if !IsReportedError(err) {
		t.Fatalf("RunCase error = %T, want reported error", err)
	}
	var summary caseRunSummary
	if decodeErr := json.Unmarshal(stdout.Bytes(), &summary); decodeErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", decodeErr, stdout.String())
	}
	if summary.Status != "error" {
		t.Fatalf("summary status = %q, want error", summary.Status)
	}
	if !strings.Contains(summary.Error, "remote ACP attorney owns model selection") {
		t.Fatalf("summary error = %q, want endpoint model-selection message", summary.Error)
	}
}
