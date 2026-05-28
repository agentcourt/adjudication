package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"adjudication/arb/runtime/lean"
	"adjudication/arb/runtime/spec"
)

func TestLoadCaseFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("situation.md", "# Proposition\n\nP\n")
	write("complaint.md", "# Proposition\n\nP\n")
	write("instructions.txt", "hello")
	write("samantha_public.pem", "pem")

	files, err := loadCaseFiles(dir)
	if err != nil {
		t.Fatalf("loadCaseFiles returned error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("loadCaseFiles returned %d files, want 2", len(files))
	}
	if files[0].EvidenceID != "instructions.txt" || files[1].EvidenceID != "samantha_public.pem" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestEvidenceRegistryStoresCaseFilesAndReadsBoundedRanges(t *testing.T) {
	dir := t.TempDir()
	casePath := filepath.Join(dir, "source.txt")
	body := []byte("abcdef")
	if err := os.WriteFile(casePath, body, 0o644); err != nil {
		t.Fatalf("write case file: %v", err)
	}
	rc := &runContext{
		cfg: Config{
			OutputDir: dir,
			Policy:    DefaultPolicy(),
		},
		caseFiles: []CaseFile{{
			EvidenceID:   "source.txt",
			Name:         "source.txt",
			Path:         casePath,
			MimeType:     "text/plain",
			TextReadable: true,
			SizeBytes:    len(body),
			Text:         string(body),
		}},
	}
	if err := rc.initializeEvidenceRegistry(); err != nil {
		t.Fatalf("initializeEvidenceRegistry returned error: %v", err)
	}
	if len(rc.evidence) != 1 {
		t.Fatalf("evidence count = %d, want 1", len(rc.evidence))
	}
	evidence := rc.evidence[0]
	if !strings.HasPrefix(evidence.EvidenceID, "ev_") || evidence.SHA256 == "" || evidence.StorageName == "" {
		t.Fatalf("evidence metadata = %#v", evidence)
	}
	if rc.caseFiles[0].EvidenceID != evidence.EvidenceID {
		t.Fatalf("case file evidence id = %q, want %q", rc.caseFiles[0].EvidenceID, evidence.EvidenceID)
	}
	if _, ok := rc.fileByID[evidence.EvidenceID]; !ok {
		t.Fatalf("fileByID missing canonical evidence id %q", evidence.EvidenceID)
	}
	if _, ok := rc.fileByID["source.txt"]; ok {
		t.Fatalf("fileByID retained filename key after canonical evidence registration")
	}
	budget := &evidenceReadBudget{}
	got, err := rc.readEvidenceRange(evidence.EvidenceID, 1, 3, budget)
	if err != nil {
		t.Fatalf("readEvidenceRange returned error: %v", err)
	}
	if got["content_base64"] != "YmNk" || got["length"] != 3 {
		t.Fatalf("read result = %#v", got)
	}
}

func TestPrepareSubmittedEvidencePreservesContentAndBuildsVisibleFile(t *testing.T) {
	dir := t.TempDir()
	rc := &runContext{
		cfg: Config{
			OutputDir: dir,
			Policy:    DefaultPolicy(),
		},
		submittedEvidence: []SubmittedEvidenceMeta{},
	}
	opportunity := Opportunity{Role: "plaintiff", Phase: "arguments"}
	content := "  exact text\n"
	meta, raw, err := rc.prepareSubmittedEvidence(opportunity, map[string]any{
		"title":                  "Source post",
		"source_url":             "https://example.test/post",
		"mime_type":              "text/plain",
		"relevance":              "Shows the disputed announcement.",
		"content":                content,
		"retrieval_timestamp":    "2026-05-14T23:00:00Z",
		"preferred_filename_ext": "txt",
	})
	if err != nil {
		t.Fatalf("prepareSubmittedEvidence returned error: %v", err)
	}
	if string(raw) != content {
		t.Fatalf("raw content = %q, want %q", string(raw), content)
	}
	sum := sha256.Sum256([]byte(content))
	wantSHA := hex.EncodeToString(sum[:])
	if meta.SHA256 != wantSHA {
		t.Fatalf("sha = %s, want %s", meta.SHA256, wantSHA)
	}
	file, err := rc.writeSubmittedEvidenceFile(meta, raw)
	if err != nil {
		t.Fatalf("writeSubmittedEvidenceFile returned error: %v", err)
	}
	if file.EvidenceID != meta.EvidenceID || !file.TextReadable || file.Text != content {
		t.Fatalf("written file metadata = %#v", file)
	}
	written, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatalf("read written evidence: %v", err)
	}
	if string(written) != content {
		t.Fatalf("written content = %q, want %q", string(written), content)
	}
}

func TestChunkedEvidenceUploadCommitsSubmittedEvidenceEvidence(t *testing.T) {
	dir := t.TempDir()
	rc := &runContext{
		cfg: Config{
			OutputDir: dir,
			Policy:    DefaultPolicy(),
		},
		evidenceByID:     map[string]EvidenceMeta{},
		evidenceStoreDir: filepath.Join(dir, "evidence-store"),
		uploadSessions:   map[string]*EvidenceUploadSession{},
	}
	raw := []byte("abcdef")
	sha := sha256.Sum256(raw)
	session, err := rc.beginEvidenceUpload(Opportunity{Role: "plaintiff", Phase: "arguments"}, map[string]any{
		"title":               "Binary source",
		"mime_type":           "application/octet-stream",
		"expected_size_bytes": int64(len(raw)),
		"expected_sha256":     hex.EncodeToString(sha[:]),
		"source_description":  "test source",
		"relevance":           "test relevance",
	})
	if err != nil {
		t.Fatalf("beginEvidenceUpload returned error: %v", err)
	}
	if _, n, err := rc.writeEvidenceChunk(session.UploadID, 0, "YWJj"); err != nil || n != 3 {
		t.Fatalf("write first chunk = session, %d, %v", n, err)
	}
	if _, n, err := rc.writeEvidenceChunk(session.UploadID, 3, "ZGVm"); err != nil || n != 3 {
		t.Fatalf("write second chunk = session, %d, %v", n, err)
	}
	meta, err := rc.prepareEvidenceUploadCommit(session, "bin")
	if err != nil {
		t.Fatalf("prepareEvidenceUploadCommit returned error: %v", err)
	}
	fileMeta := submittedEvidencePayload(meta)
	if fileMeta["evidence_id"] != meta.EvidenceID {
		t.Fatalf("submitted evidence payload missing evidence_id: %#v", fileMeta)
	}
	meta, file, evidence, err := rc.finalizeEvidenceUpload(session, meta)
	if err != nil {
		t.Fatalf("finalizeEvidenceUpload returned error: %v", err)
	}
	if meta.EvidenceID == "" || file.EvidenceID != meta.EvidenceID || evidence.EvidenceID != meta.EvidenceID {
		t.Fatalf("meta=%#v file=%#v evidence=%#v", meta, file, evidence)
	}
	if _, ok := rc.uploadSessions[session.UploadID]; ok {
		t.Fatalf("upload session was not cleared")
	}
	if got, err := os.ReadFile(file.Path); err != nil || string(got) != string(raw) {
		t.Fatalf("uploaded file = %q, %v", string(got), err)
	}
}

func TestSubmittedEvidenceRegistersEvidence(t *testing.T) {
	dir := t.TempDir()
	rc := &runContext{
		cfg: Config{
			OutputDir: dir,
			Policy:    DefaultPolicy(),
		},
		evidenceByID:     map[string]EvidenceMeta{},
		evidenceStoreDir: filepath.Join(dir, "evidence-store"),
	}
	sha := sha256.Sum256([]byte("source"))
	name := "submitted-evidence-01-plaintiff-abcd.txt"
	meta := SubmittedEvidenceMeta{
		Phase:              "arguments",
		Role:               "plaintiff",
		EvidenceID:         evidenceIDForFile(hex.EncodeToString(sha[:]), name),
		Name:               name,
		Title:              "Source",
		SourceURL:          "https://example.test/source",
		MimeType:           "text/plain",
		RetrievalTimestamp: "2026-05-21T12:00:00Z",
		Relevance:          "Shows the fact.",
	}
	file := CaseFile{EvidenceID: meta.EvidenceID, Name: meta.Name, Path: filepath.Join(dir, meta.Name), MimeType: meta.MimeType, TextReadable: true, Text: "source"}
	if err := os.WriteFile(file.Path, []byte(file.Text), 0o644); err != nil {
		t.Fatalf("write evidence file: %v", err)
	}
	evidence, err := rc.registerSubmittedEvidenceEvidence(meta, file)
	if err != nil {
		t.Fatalf("registerSubmittedEvidenceEvidence returned error: %v", err)
	}
	if evidence.AdmissibilityStatus != "submitted_evidence" || evidence.SubmittedByRole != "plaintiff" || evidence.EvidenceID != meta.EvidenceID {
		t.Fatalf("evidence metadata = %#v", evidence)
	}
	if _, err := os.Stat(filepath.Join(dir, "evidence-store", filepath.FromSlash(evidence.StorageName))); err != nil {
		t.Fatalf("stored evidence not found: %v", err)
	}
}

func TestAddEvidenceRejectsSameIDForDifferentBytes(t *testing.T) {
	rc := &runContext{}
	_, err := rc.addEvidence(EvidenceMeta{EvidenceID: "ev_same", SHA256: "aaa", SizeBytes: 3, StorageName: "aa/aaa"})
	if err != nil {
		t.Fatalf("add first evidence returned error: %v", err)
	}
	_, err = rc.addEvidence(EvidenceMeta{EvidenceID: "ev_same", SHA256: "bbb", SizeBytes: 3, StorageName: "bb/bbb"})
	if err == nil || !strings.Contains(err.Error(), "evidence_id collision") {
		t.Fatalf("add conflicting evidence error = %v, want collision", err)
	}
	_, err = rc.addEvidence(EvidenceMeta{EvidenceID: "ev_same", SHA256: "aaa", SizeBytes: 3, StorageName: "aa/aaa", ParentEvidenceID: "parent"})
	if err == nil || !strings.Contains(err.Error(), "metadata differs") {
		t.Fatalf("add same-byte metadata conflict error = %v, want metadata conflict", err)
	}
	if rc.evidenceByID["ev_same"].ParentEvidenceID != "" {
		t.Fatalf("evidence metadata was overwritten: %#v", rc.evidenceByID["ev_same"])
	}
}

func TestAddEvidenceAllowsIdempotentRegistration(t *testing.T) {
	rc := &runContext{}
	meta := EvidenceMeta{
		EvidenceID:          "ev_abc123_source",
		SHA256:              "abc123",
		SizeBytes:           6,
		MimeType:            "text/plain",
		StorageName:         "ab/abc123",
		CreatedAt:           "2026-05-21T20:00:00Z",
		AdmissibilityStatus: "case_packet",
		RecordVisibility:    "juror_visible",
		Title:               "source.txt",
		OriginalName:        "source.txt",
		SubmittedByRole:     "system",
		SubmittedPhase:      "case_packet",
		TextReadable:        true,
	}
	first, err := rc.addEvidence(meta)
	if err != nil {
		t.Fatalf("first addEvidence returned error: %v", err)
	}
	meta.CreatedAt = "2026-05-21T20:01:00Z"
	second, err := rc.addEvidence(meta)
	if err != nil {
		t.Fatalf("second addEvidence returned error: %v", err)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Fatalf("idempotent registration replaced existing metadata: first=%#v second=%#v", first, second)
	}
	if len(rc.evidence) != 1 {
		t.Fatalf("evidence count = %d, want 1", len(rc.evidence))
	}
}

func TestAddEvidenceRejectsMetadataConflict(t *testing.T) {
	rc := &runContext{}
	meta := EvidenceMeta{
		EvidenceID:          "ev_abc123_source",
		SHA256:              "abc123",
		SizeBytes:           6,
		MimeType:            "text/plain",
		StorageName:         "ab/abc123",
		CreatedAt:           "2026-05-21T20:00:00Z",
		AdmissibilityStatus: "case_packet",
		RecordVisibility:    "juror_visible",
		Title:               "source.txt",
		OriginalName:        "source.txt",
		SubmittedByRole:     "system",
		SubmittedPhase:      "case_packet",
		TextReadable:        true,
	}
	if _, err := rc.addEvidence(meta); err != nil {
		t.Fatalf("first addEvidence returned error: %v", err)
	}
	conflicting := meta
	conflicting.Title = "different title"
	if _, err := rc.addEvidence(conflicting); err == nil || !strings.Contains(err.Error(), "metadata differs") {
		t.Fatalf("conflicting addEvidence error = %v, want metadata conflict", err)
	}
}

func TestBeginEvidenceUploadRejectsNonIntegerSize(t *testing.T) {
	rc := &runContext{cfg: Config{OutputDir: t.TempDir(), Policy: DefaultPolicy()}}
	_, err := rc.beginEvidenceUpload(Opportunity{Role: "plaintiff", Phase: "arguments"}, map[string]any{
		"title":               "Bad size",
		"mime_type":           "text/plain",
		"expected_size_bytes": "12",
		"source_description":  "test source",
		"relevance":           "test relevance",
	})
	if err == nil || !strings.Contains(err.Error(), "expected_size_bytes must be an integer") {
		t.Fatalf("beginEvidenceUpload error = %v, want integer error", err)
	}
}

func TestPrepareSubmittedEvidenceHonorsDirectByteLimit(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxDirectSubmittedEvidenceBytes = 4
	policy.MaxSubmittedEvidenceBytes = 8
	rc := &runContext{cfg: Config{OutputDir: t.TempDir(), Policy: policy}}
	_, _, err := rc.prepareSubmittedEvidence(Opportunity{Role: "plaintiff", Phase: "arguments"}, map[string]any{
		"title":              "Too large direct source",
		"source_description": "test source",
		"mime_type":          "text/plain",
		"relevance":          "test relevance",
		"content":            "12345",
	})
	if err == nil || !strings.Contains(err.Error(), "direct submitted evidence exceeds byte limit") {
		t.Fatalf("prepareSubmittedEvidence error = %v, want direct limit error", err)
	}
}

func TestValidatePolicyKeepsUploadLimitWithinRecordEvidenceLimit(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxSubmittedEvidenceBytes = 8
	policy.MaxDirectSubmittedEvidenceBytes = 4
	policy.MaxEvidenceUploadBytes = 9
	policy.MaxEvidenceChunkBytes = 4
	if err := ValidatePolicy(policy); err == nil || !strings.Contains(err.Error(), "max_evidence_upload_bytes") {
		t.Fatalf("ValidatePolicy error = %v, want upload limit error", err)
	}
}

func TestLoadCaseFilesPreservesTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "situation.md"), []byte("# Proposition\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write situation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "complaint.md"), []byte("# Proposition\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	body := "line one\nline two\n"
	if err := os.WriteFile(filepath.Join(dir, "confession.txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("write confession: %v", err)
	}

	files, err := loadCaseFiles(dir)
	if err != nil {
		t.Fatalf("loadCaseFiles returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("loadCaseFiles returned %d files, want 1", len(files))
	}
	if files[0].Text != body {
		t.Fatalf("file text = %q, want %q", files[0].Text, body)
	}
}

func TestLoadCaseFilesAllowsNoUsableFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "situation.md"), []byte("# Proposition\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write situation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "complaint.md"), []byte("# Proposition\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("note\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	files, err := loadCaseFiles(dir)
	if err != nil {
		t.Fatalf("loadCaseFiles returned error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("loadCaseFiles returned %d files, want 0", len(files))
	}
}

func TestLoadCaseFilesFromPaths(t *testing.T) {
	dir := t.TempDir()
	txtPath := filepath.Join(dir, "instructions.txt")
	pemPath := filepath.Join(dir, "samantha_public.pem")
	if err := os.WriteFile(txtPath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write instructions: %v", err)
	}
	if err := os.WriteFile(pemPath, []byte("pem"), 0o644); err != nil {
		t.Fatalf("write pem: %v", err)
	}

	files, err := loadCaseFilesFromPaths([]string{pemPath, txtPath})
	if err != nil {
		t.Fatalf("loadCaseFilesFromPaths returned error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("loadCaseFilesFromPaths returned %d files, want 2", len(files))
	}
	if files[0].EvidenceID != "instructions.txt" || files[1].EvidenceID != "samantha_public.pem" {
		t.Fatalf("unexpected files: %#v", files)
	}
	if files[0].Text != "hello\n" {
		t.Fatalf("instructions text = %q, want hello\\n", files[0].Text)
	}
}

func TestLoadCaseFilesFromPathsRejectsDuplicateBaseNames(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "a")
	right := filepath.Join(dir, "b")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("mkdir left: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("mkdir right: %v", err)
	}
	leftPath := filepath.Join(left, "shared.txt")
	rightPath := filepath.Join(right, "shared.txt")
	if err := os.WriteFile(leftPath, []byte("left"), 0o644); err != nil {
		t.Fatalf("write left: %v", err)
	}
	if err := os.WriteFile(rightPath, []byte("right"), 0o644); err != nil {
		t.Fatalf("write right: %v", err)
	}

	_, err := loadCaseFilesFromPaths([]string{leftPath, rightPath})
	if err == nil || !strings.Contains(err.Error(), "duplicate case file name") {
		t.Fatalf("loadCaseFilesFromPaths error = %v, want duplicate name error", err)
	}
}

func TestValidateAttorneyPayload(t *testing.T) {
	policy := DefaultPolicy()
	fileByID := map[string]CaseFile{
		"instructions.txt": {EvidenceID: "instructions.txt", SizeBytes: 128},
	}
	valid := map[string]any{
		"text": "argument",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "instructions.txt", "label": "PX-1"},
		},
		"technical_reports": []any{
			map[string]any{"title": "Verification", "summary": "Verified OK."},
		},
	}
	if err := validateAttorneyPayload("submit_argument", valid, fileByID, policy); err != nil {
		t.Fatalf("validateAttorneyPayload returned error: %v", err)
	}
	invalid := map[string]any{
		"text": "",
	}
	if err := validateAttorneyPayload("submit_argument", invalid, fileByID, policy); err == nil {
		t.Fatalf("expected validation error for empty text")
	}
	badFile := map[string]any{
		"text": "argument",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "missing.txt"},
		},
	}
	if err := validateAttorneyPayload("submit_argument", badFile, fileByID, policy); err == nil {
		t.Fatalf("expected validation error for missing file")
	}
}

func TestCouncilMemberIDFromOpportunity(t *testing.T) {
	opportunity := Opportunity{ID: "deliberation:2:C4"}
	if got := councilMemberIDFromOpportunity(opportunity); got != "C4" {
		t.Fatalf("councilMemberIDFromOpportunity = %q, want C4", got)
	}
}

func TestValidateAttorneyPayloadAllowsSupplementalMaterialsInRebuttal(t *testing.T) {
	policy := DefaultPolicy()
	fileByID := map[string]CaseFile{
		"instructions.txt": {EvidenceID: "instructions.txt", SizeBytes: 128},
	}
	rebuttal := map[string]any{
		"text": "reply",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "instructions.txt"},
		},
		"technical_reports": []any{
			map[string]any{"title": "Check", "summary": "Done."},
		},
	}
	if err := validateAttorneyPayload("submit_rebuttal", rebuttal, fileByID, policy); err != nil {
		t.Fatalf("expected rebuttal supplemental materials to be accepted: %v", err)
	}
}

func TestValidateAttorneyPayloadRejectsSupplementalMaterialsInSurrebuttal(t *testing.T) {
	policy := DefaultPolicy()
	fileByID := map[string]CaseFile{
		"instructions.txt": {EvidenceID: "instructions.txt", SizeBytes: 128},
	}
	surrebuttal := map[string]any{
		"text": "reply",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "instructions.txt"},
		},
		"technical_reports": []any{
			map[string]any{"title": "Check", "summary": "Done."},
		},
	}
	if err := validateAttorneyPayload("submit_surrebuttal", surrebuttal, fileByID, policy); err == nil {
		t.Fatalf("expected surrebuttal technical_reports to be rejected")
	}
}

func TestValidateAttorneyPayloadRejectsSupplementalMaterialsInClosing(t *testing.T) {
	policy := DefaultPolicy()
	fileByID := map[string]CaseFile{
		"instructions.txt": {EvidenceID: "instructions.txt", SizeBytes: 128},
	}
	closing := map[string]any{
		"text": "closing",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "instructions.txt"},
		},
	}
	if err := validateAttorneyPayload("deliver_closing_statement", closing, fileByID, policy); err == nil {
		t.Fatalf("expected closing offered_evidence to be rejected")
	}
	closing = map[string]any{
		"text": "closing",
		"technical_reports": []any{
			map[string]any{"title": "Late report", "summary": "New analysis."},
		},
	}
	if err := validateAttorneyPayload("deliver_closing_statement", closing, fileByID, policy); err == nil {
		t.Fatalf("expected closing technical_reports to be rejected")
	}
}

func TestValidateAttorneyPayloadRejectsOversizeExhibit(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxExhibitBytes = 16
	fileByID := map[string]CaseFile{
		"instructions.txt": {EvidenceID: "instructions.txt", SizeBytes: 32},
	}
	payload := map[string]any{
		"text": "argument",
		"offered_evidence": []any{
			map[string]any{"evidence_id": "instructions.txt"},
		},
	}
	if err := validateAttorneyPayload("submit_argument", payload, fileByID, policy); err == nil {
		t.Fatalf("expected oversize exhibit to be rejected")
	}
}

func TestValidateAttorneyPayloadRejectsTooManyReports(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxReportsPerFiling = 1
	fileByID := map[string]CaseFile{}
	payload := map[string]any{
		"text": "argument",
		"technical_reports": []any{
			map[string]any{"title": "One", "summary": "A"},
			map[string]any{"title": "Two", "summary": "B"},
		},
	}
	if err := validateAttorneyPayload("submit_argument", payload, fileByID, policy); err == nil {
		t.Fatalf("expected per-filing report limit to be enforced")
	}
}

func TestFormatInvalidAttemptLimitErrorIncludesAttemptReasons(t *testing.T) {
	err := formatInvalidAttemptLimitError("plaintiff", []string{
		"opening statement exceeds character limit of 4000 (got 4687)",
		"payload.text is required",
	})
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	got := err.Error()
	if !strings.Contains(got, "plaintiff exceeded invalid-attempt limit after 2 invalid submissions") {
		t.Fatalf("unexpected invalid-attempt summary: %s", got)
	}
	if !strings.Contains(got, "attempt 1: opening statement exceeds character limit of 4000 (got 4687)") {
		t.Fatalf("missing first attempt reason: %s", got)
	}
	if !strings.Contains(got, "attempt 2: payload.text is required") {
		t.Fatalf("missing second attempt reason: %s", got)
	}
}

func TestFormatInvalidAttemptLimitErrorFallsBackWithoutReasons(t *testing.T) {
	err := formatInvalidAttemptLimitError("plaintiff", []string{"", "  "})
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	if got := err.Error(); got != "plaintiff exceeded invalid-attempt limit" {
		t.Fatalf("unexpected fallback invalid-attempt error: %s", got)
	}
}

func TestFormatAttorneyInvalidDecisionErrorGuidesLengthResubmission(t *testing.T) {
	err := formatAttorneyInvalidDecisionError(
		Opportunity{Role: "plaintiff", Phase: "openings"},
		DefaultPolicy(),
		[]string{"opening statement exceeds character limit of 4000 (got 4687)"},
		3,
	)
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	got := err.Error()
	if !strings.Contains(got, "Opening statement exceeds the character limit: 4687 characters submitted, 4000 allowed.") {
		t.Fatalf("missing length reason: %s", got)
	}
	if !strings.Contains(got, "This is invalid submission 1 of 3 for this opportunity. You have 2 invalid submissions remaining.") {
		t.Fatalf("missing invalid-submission count: %s", got)
	}
	if !strings.Contains(got, "Resubmit at 3000 characters or fewer. Count characters, not tokens.") {
		t.Fatalf("missing resubmission target: %s", got)
	}
	if !strings.Contains(got, "If you exhaust the remaining invalid submissions, this opportunity will fail and the run will end with an error.") {
		t.Fatalf("missing exhaustion warning: %s", got)
	}
}

func TestFormatAttorneyInvalidDecisionErrorGuidesOverflowResubmission(t *testing.T) {
	err := formatAttorneyInvalidDecisionError(
		Opportunity{Role: "plaintiff", Phase: "rebuttals"},
		DefaultPolicy(),
		[]string{"technical_reports for this side exceed limit of 4 (3 already used, 2 attempted, 1 remaining)"},
		3,
	)
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	got := err.Error()
	if !strings.Contains(got, "technical_reports for this side exceed limit of 4 (3 already used, 2 attempted, 1 remaining).") {
		t.Fatalf("missing overflow reason: %s", got)
	}
	if !strings.Contains(got, "Remove the overflow and resubmit within the stated limit.") {
		t.Fatalf("missing overflow guidance: %s", got)
	}
}

func TestFormatAttorneyInvalidDecisionErrorExplainsFinalFailure(t *testing.T) {
	err := formatAttorneyInvalidDecisionError(
		Opportunity{Role: "plaintiff", Phase: "openings"},
		DefaultPolicy(),
		[]string{
			"opening statement exceeds character limit of 4000 (got 4687)",
			"payload.text is required",
			"payload.text is required",
		},
		3,
	)
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	got := err.Error()
	if !strings.Contains(got, "This is invalid submission 3 of 3 for this opportunity. No invalid submissions remain.") {
		t.Fatalf("missing final invalid-submission count: %s", got)
	}
	if !strings.Contains(got, "This opportunity has failed, and the run is ending with an error.") {
		t.Fatalf("missing terminal failure line: %s", got)
	}
	if !strings.Contains(got, "Invalid submission history: attempt 1: Opening statement exceeds the character limit: 4687 characters submitted, 4000 allowed.; attempt 2: payload.text is required.; attempt 3: payload.text is required.") {
		t.Fatalf("missing invalid-submission history: %s", got)
	}
}

func TestValidateAttorneyPayloadAgainstStateRejectsOverlongRebuttal(t *testing.T) {
	policy := DefaultPolicy()
	rc := &runContext{
		cfg: Config{Policy: policy},
		state: map[string]any{
			"case": map[string]any{
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	payload := map[string]any{
		"text": strings.Repeat("a", policy.MaxRebuttalChars+1),
	}
	err := rc.validateAttorneyPayloadAgainstState(Opportunity{
		Role:  "plaintiff",
		Phase: "rebuttals",
	}, "submit_rebuttal", payload)
	if err == nil {
		t.Fatalf("expected rebuttal length error")
	}
	if !strings.Contains(err.Error(), "rebuttal exceeds character limit") || !strings.Contains(err.Error(), fmt.Sprintf("got %d", policy.MaxRebuttalChars+1)) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttorneyPayloadAgainstStateRejectsSideReportOverflow(t *testing.T) {
	policy := DefaultPolicy()
	existing := []map[string]any{
		{"role": "plaintiff", "title": "One", "summary": "A"},
		{"role": "plaintiff", "title": "Two", "summary": "B"},
		{"role": "plaintiff", "title": "Three", "summary": "C"},
	}
	rc := &runContext{
		cfg: Config{Policy: policy},
		state: map[string]any{
			"case": map[string]any{
				"offered_evidence":  []map[string]any{},
				"technical_reports": existing,
			},
		},
	}
	payload := map[string]any{
		"text": "reply",
		"technical_reports": []any{
			map[string]any{"title": "Four", "summary": "D"},
			map[string]any{"title": "Five", "summary": "E"},
		},
	}
	err := rc.validateAttorneyPayloadAgainstState(Opportunity{
		Role:  "plaintiff",
		Phase: "rebuttals",
	}, "submit_rebuttal", payload)
	if err == nil {
		t.Fatalf("expected side report overflow error")
	}
	if !strings.Contains(err.Error(), "technical_reports for this side exceed limit of 4 (3 already used, 2 attempted, 1 remaining)") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePolicyRejectsImpossibleThreshold(t *testing.T) {
	policy := DefaultPolicy()
	policy.RequiredVotesForDecision = 6
	if err := ValidatePolicy(policy); err == nil {
		t.Fatalf("expected policy validation error")
	}
}

func TestValidatePolicyRejectsNonStrictMajorityThreshold(t *testing.T) {
	policy := DefaultPolicy()
	policy.CouncilSize = 4
	policy.RequiredVotesForDecision = 2
	err := ValidatePolicy(policy)
	if err == nil {
		t.Fatalf("expected policy validation error")
	}
	if got := err.Error(); got != "policy.required_votes_for_decision must be a strict majority of council_size" {
		t.Fatalf("unexpected validation error: %s", got)
	}
}

func TestValidateRuntimeLimitsRejectsZeroResponseLimit(t *testing.T) {
	runtime := DefaultRuntimeLimits()
	runtime.MaxResponseBytes = 0
	if err := ValidateRuntimeLimits(runtime); err == nil {
		t.Fatalf("expected runtime validation error")
	}
}

func TestBuildAttorneyPromptStatesCouncilForum(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()
	rc := &runContext{
		cfg: Config{
			Policy: DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Proposition: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"evidence_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":             "openings",
				"openings":          []map[string]any{},
				"arguments":         []map[string]any{},
				"rebuttals":         []map[string]any{},
				"surrebuttals":      []map[string]any{},
				"closings":          []map[string]any{},
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "openings:plaintiff",
		Role:         "plaintiff",
		Phase:        "openings",
		Objective:    "plaintiff opening statement",
		AllowedTools: []string{"record_opening_statement"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "no judge, no clerk, and no voir dire") {
		t.Fatalf("prompt did not state the forum shape:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Address the council, not a judge.") {
		t.Fatalf("prompt did not direct counsel to address the council:\n%s", prompt)
	}
	if !strings.Contains(prompt, "record contains only the proposition and the standard of evidence") {
		t.Fatalf("prompt did not state the opening record limit:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Do not invent facts, sources, quotations, files, analyses, or results.") {
		t.Fatalf("prompt did not forbid fabrication:\n%s", prompt)
	}
	if !strings.Contains(prompt, "When a tool returns an error, treat the error text as authoritative host feedback and correct the stated defect before trying again.") {
		t.Fatalf("prompt did not instruct counsel to respond to tool errors:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Text limit for this submission: 5000 characters.") {
		t.Fatalf("prompt did not state the opening text limit:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Target length for the first submission: 3750 characters or less.") {
		t.Fatalf("prompt did not state the opening target length:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Native web search through the model is available.") {
		t.Fatalf("prompt did not state search availability:\n%s", prompt)
	}
	if strings.Contains(prompt, "Visible case files:") {
		t.Fatalf("opening prompt should not list visible case files:\n%s", prompt)
	}
}

func TestBuildAttorneyPromptStatesWhenSearchIsUnavailable(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()
	rc := &runContext{
		cfg: Config{
			Policy:        DefaultPolicy(),
			AttorneyModel: "openai://gpt-5",
		},
		complaint: spec.Complaint{
			Proposition: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"evidence_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":             "openings",
				"openings":          []map[string]any{},
				"arguments":         []map[string]any{},
				"rebuttals":         []map[string]any{},
				"surrebuttals":      []map[string]any{},
				"closings":          []map[string]any{},
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "openings:plaintiff",
		Role:         "plaintiff",
		Phase:        "openings",
		Objective:    "plaintiff opening statement",
		AllowedTools: []string{"record_opening_statement"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Native web search through the model is not available.") {
		t.Fatalf("prompt did not state search unavailability:\n%s", prompt)
	}
}

func TestBuildAttorneyPromptIncludesWorkProductGuidance(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()
	rc := &runContext{
		cfg: Config{
			Policy:     DefaultPolicy(),
			ACPCommand: "/tmp/acp-podman.sh",
		},
		complaint: spec.Complaint{
			Proposition: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"evidence_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":             "arguments",
				"openings":          []map[string]any{},
				"arguments":         []map[string]any{},
				"rebuttals":         []map[string]any{},
				"surrebuttals":      []map[string]any{},
				"closings":          []map[string]any{},
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "arguments:plaintiff",
		Role:         "plaintiff",
		Phase:        "arguments",
		Objective:    "plaintiff merits argument",
		AllowedTools: []string{"submit_argument"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Private work product: Use `/home/user/work-product/` for internal notes, timelines, source leads, adverse facts, unresolved questions, and draft analyses.") {
		t.Fatalf("prompt did not include work-product guidance:\n%s", prompt)
	}
	if !strings.Contains(prompt, "This directory is not part of the record unless you later turn material from it into an exhibit or technical report.") {
		t.Fatalf("prompt did not distinguish work product from the record:\n%s", prompt)
	}
}

func TestACPToolSpecsArePhaseSpecific(t *testing.T) {
	openingSpecs := acpToolSpecs(Opportunity{Phase: "openings"}, true)
	openingTools := make([]string, 0, len(openingSpecs))
	for _, spec := range openingSpecs {
		openingTools = append(openingTools, mapString(spec["toolName"]))
	}
	if slices.Contains(openingTools, "aar_list_evidence") || slices.Contains(openingTools, "aar_read_evidence_range") || slices.Contains(openingTools, "aar_begin_evidence_upload") {
		t.Fatalf("opening tools exposed evidence access: %#v", openingTools)
	}
	argumentSpecs := acpToolSpecs(Opportunity{Phase: "arguments"}, true)
	argumentTools := make([]string, 0, len(argumentSpecs))
	for _, spec := range argumentSpecs {
		argumentTools = append(argumentTools, mapString(spec["toolName"]))
	}
	if !slices.Contains(argumentTools, "aar_list_evidence") || !slices.Contains(argumentTools, "aar_stat_evidence") || !slices.Contains(argumentTools, "aar_read_evidence_range") || !slices.Contains(argumentTools, "aar_materialize_evidence") || !slices.Contains(argumentTools, "aar_begin_evidence_upload") || !slices.Contains(argumentTools, "aar_write_evidence_chunk") || !slices.Contains(argumentTools, "aar_commit_evidence_upload") || !slices.Contains(argumentTools, "aar_submit_evidence") {
		t.Fatalf("argument tools did not expose evidence access: %#v", argumentTools)
	}
	rebuttalSpecs := acpToolSpecs(Opportunity{Phase: "rebuttals"}, true)
	rebuttalTools := make([]string, 0, len(rebuttalSpecs))
	for _, spec := range rebuttalSpecs {
		rebuttalTools = append(rebuttalTools, mapString(spec["toolName"]))
	}
	if !slices.Contains(rebuttalTools, "aar_list_evidence") || !slices.Contains(rebuttalTools, "aar_stat_evidence") || !slices.Contains(rebuttalTools, "aar_read_evidence_range") || !slices.Contains(rebuttalTools, "aar_materialize_evidence") || !slices.Contains(rebuttalTools, "aar_begin_evidence_upload") || !slices.Contains(rebuttalTools, "aar_write_evidence_chunk") || !slices.Contains(rebuttalTools, "aar_commit_evidence_upload") || !slices.Contains(rebuttalTools, "aar_submit_evidence") {
		t.Fatalf("rebuttal tools did not expose evidence access: %#v", rebuttalTools)
	}
	var submitSpec map[string]any
	for _, spec := range argumentSpecs {
		if mapString(spec["toolName"]) == "aar_submit_decision" {
			submitSpec = spec
			break
		}
	}
	if submitSpec == nil {
		t.Fatalf("missing aar_submit_decision spec")
	}
	properties := mapAny(mapAny(submitSpec["parameters"])["properties"])
	if _, ok := properties["reason"]; ok {
		t.Fatalf("aar_submit_decision should not advertise a reason field: %#v", properties)
	}
	payload := mapAny(properties["payload"])
	if mapString(payload["type"]) != "object" {
		t.Fatalf("payload schema type = %#v, want object", payload["type"])
	}
	payloadProps := mapAny(payload["properties"])
	offeredEvidence := mapAny(payloadProps["offered_evidence"])
	if mapString(offeredEvidence["type"]) != "array" {
		t.Fatalf("offered_evidence schema type = %#v, want array", offeredEvidence["type"])
	}
	offeredItemProps := mapAny(mapAny(offeredEvidence["items"])["properties"])
	if _, ok := offeredItemProps["evidence_id"]; !ok {
		t.Fatalf("offered_evidence items missing evidence_id: %#v", offeredItemProps)
	}
	if _, ok := offeredItemProps["label"]; !ok {
		t.Fatalf("offered_evidence items missing label: %#v", offeredItemProps)
	}
	reports := mapAny(payloadProps["technical_reports"])
	if mapString(reports["type"]) != "array" {
		t.Fatalf("technical_reports schema type = %#v, want array", reports["type"])
	}
	reportItemProps := mapAny(mapAny(reports["items"])["properties"])
	if _, ok := reportItemProps["title"]; !ok {
		t.Fatalf("technical_reports items missing title: %#v", reportItemProps)
	}
	if _, ok := reportItemProps["summary"]; !ok {
		t.Fatalf("technical_reports items missing summary: %#v", reportItemProps)
	}
}

func TestJurorACPToolSpecsAreReadOnly(t *testing.T) {
	tools := make([]string, 0)
	for _, spec := range jurorACPClientToolSpecs(true) {
		tools = append(tools, mapString(spec["toolName"]))
	}
	for _, want := range []string{
		"aar_get_case",
		"aar_list_evidence",
		"aar_stat_evidence",
		"aar_read_evidence_range",
		"aar_materialize_evidence",
		"aar_submit_council_vote",
	} {
		if !slices.Contains(tools, want) {
			t.Fatalf("juror tools missing %s: %#v", want, tools)
		}
	}
	for _, forbidden := range []string{
		"aar_begin_evidence_upload",
		"aar_write_evidence_chunk",
		"aar_commit_evidence_upload",
		"aar_submit_evidence",
		"aar_submit_decision",
	} {
		if slices.Contains(tools, forbidden) {
			t.Fatalf("juror tools exposed forbidden tool %s: %#v", forbidden, tools)
		}
	}
}

func TestBuildCouncilACPPromptConstrainsJurorInvestigation(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()
	rc := &runContext{
		cfg: Config{
			Policy: DefaultPolicy(),
		},
		complaint: spec.Complaint{Proposition: "P"},
		state: map[string]any{
			"policy": map[string]any{"evidence_standard": "preponderance"},
			"case": map[string]any{
				"phase":              "deliberation",
				"deliberation_round": 1,
				"openings":           []map[string]any{},
				"arguments":          []map[string]any{},
				"rebuttals":          []map[string]any{},
				"surrebuttals":       []map[string]any{},
				"closings":           []map[string]any{},
				"offered_evidence":   []map[string]any{},
				"technical_reports":  []map[string]any{},
				"council_votes":      []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildCouncilACPPrompt(CouncilSeat{MemberID: "C1", Model: "openai://gpt-5", PersonaText: "Skeptical."}, Opportunity{ID: "deliberation:1:C1", Role: "council", Phase: "deliberation"})
	if err != nil {
		t.Fatalf("buildCouncilACPPrompt returned error: %v", err)
	}
	for _, want := range []string{
		"You may examine admitted evidence through the read-only AAR tools.",
		"Do not search the web, introduce new facts, create new evidence, upload evidence",
		"Evidence identity is evidence_id plus SHA-256.",
		"call aar_submit_council_vote exactly once",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("council ACP prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestCouncilBackendValidation(t *testing.T) {
	for _, backend := range []string{"", "direct", "pi", "PI"} {
		if err := ValidateCouncilBackend(backend); err != nil {
			t.Fatalf("ValidateCouncilBackend(%q) returned error: %v", backend, err)
		}
	}
	if got := NormalizeCouncilBackend(""); got != "direct" {
		t.Fatalf("NormalizeCouncilBackend empty = %q, want direct", got)
	}
	if err := ValidateCouncilBackend("browser"); err == nil {
		t.Fatalf("ValidateCouncilBackend accepted unknown backend")
	}
}

func TestEnsureCouncilACPSessionRejectsSearchEnabledModel(t *testing.T) {
	rc := &runContext{
		cfg: Config{
			OutputDir:         t.TempDir(),
			CouncilACPCommand: "/tmp/acp",
			Policy:            DefaultPolicy(),
			Runtime:           DefaultRuntimeLimits(),
		},
		acpSessions: map[string]*acpPersistentSession{},
	}
	_, err := rc.ensureCouncilACPSession(context.Background(), CouncilSeat{MemberID: "C1", Model: "openai://gpt-5?tools=search"})
	if err == nil || !strings.Contains(err.Error(), "does not allow web-search-enabled model") {
		t.Fatalf("ensureCouncilACPSession error = %v, want web-search rejection", err)
	}
}

func TestBuildAttorneyPromptConstrainsArgumentExperiments(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()
	rc := &runContext{
		cfg: Config{
			Policy: DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Proposition: "P",
		},
		caseFiles: []CaseFile{{EvidenceID: "instructions.txt", Name: "instructions.txt", MimeType: "text/plain", TextReadable: true}},
		state: map[string]any{
			"policy": map[string]any{
				"evidence_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":             "arguments",
				"openings":          []map[string]any{},
				"arguments":         []map[string]any{},
				"rebuttals":         []map[string]any{},
				"surrebuttals":      []map[string]any{},
				"closings":          []map[string]any{},
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "arguments:plaintiff",
		Role:         "plaintiff",
		Phase:        "arguments",
		Objective:    "plaintiff merits argument",
		AllowedTools: []string{"submit_argument"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Use this phase to file the merits submission for your side.") {
		t.Fatalf("argument prompt did not define the court-owned phase objective:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Exhibits: at most 9 in this filing. This side has used 0 of 12 total, with 12 left.") {
		t.Fatalf("argument prompt did not state exhibit limits:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Technical reports: at most 3 in this filing. This side has used 0 of 4 total, with 4 left.") {
		t.Fatalf("argument prompt did not state report limits:\n%s", prompt)
	}
	if !strings.Contains(prompt, "submit its content and provenance with aar_submit_evidence") {
		t.Fatalf("argument prompt did not require outside source material to enter as submitted evidence:\n%s", prompt)
	}
	if !strings.Contains(prompt, "materialize the needed evidence into the workspace first") {
		t.Fatalf("argument prompt did not instruct counsel to materialize exact file bytes:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Use only visible case evidence_id values in offered_evidence. Submit new source material first with aar_submit_evidence") {
		t.Fatalf("argument prompt did not restrict offered_evidence to visible evidence ids:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Use technical_reports for attorney analysis or synthesized work product") {
		t.Fatalf("argument prompt did not distinguish technical reports from source evidence:\n%s", prompt)
	}
}

func TestBuildAttorneyPromptConstrainsArgumentExperimentsWithoutSearch(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()
	rc := &runContext{
		cfg: Config{
			Policy:        DefaultPolicy(),
			AttorneyModel: "openai://gpt-5",
		},
		complaint: spec.Complaint{
			Proposition: "P",
		},
		caseFiles: []CaseFile{{EvidenceID: "instructions.txt", Name: "instructions.txt", MimeType: "text/plain", TextReadable: true}},
		state: map[string]any{
			"policy": map[string]any{
				"evidence_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":             "arguments",
				"openings":          []map[string]any{},
				"arguments":         []map[string]any{},
				"rebuttals":         []map[string]any{},
				"surrebuttals":      []map[string]any{},
				"closings":          []map[string]any{},
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "arguments:plaintiff",
		Role:         "plaintiff",
		Phase:        "arguments",
		Objective:    "plaintiff merits argument",
		AllowedTools: []string{"submit_argument"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Native web search through the model is not available.") {
		t.Fatalf("argument prompt did not state search unavailability:\n%s", prompt)
	}
}

func TestBuildAttorneyPromptAllowsRebuttalSupplementalMaterials(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()
	rc := &runContext{
		cfg: Config{
			Policy: DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Proposition: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"evidence_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":            "rebuttals",
				"openings":         []map[string]any{},
				"arguments":        []map[string]any{},
				"rebuttals":        []map[string]any{},
				"surrebuttals":     []map[string]any{},
				"closings":         []map[string]any{},
				"offered_evidence": []map[string]any{},
				"technical_reports": []map[string]any{
					{"role": "plaintiff", "title": "One", "summary": "A"},
					{"role": "plaintiff", "title": "Two", "summary": "B"},
					{"role": "plaintiff", "title": "Three", "summary": "C"},
				},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "rebuttals:plaintiff",
		Role:         "plaintiff",
		Phase:        "rebuttals",
		Objective:    "plaintiff rebuttal",
		AllowedTools: []string{"submit_rebuttal", "pass_phase_opportunity"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Offer exhibits, submitted evidence, and technical reports only if they directly answer the opposing argument.") {
		t.Fatalf("rebuttal prompt did not allow targeted supplemental materials:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Text limit for this submission: 4000 characters.") {
		t.Fatalf("rebuttal prompt did not state the rebuttal text limit:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Target length for the first submission: 3000 characters or less.") {
		t.Fatalf("rebuttal prompt did not state the rebuttal target length:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Technical reports: at most 3 in this filing. This side has used 3 of 4 total, with 1 left.") {
		t.Fatalf("rebuttal prompt did not state remaining report capacity:\n%s", prompt)
	}
	if !strings.Contains(prompt, "\"offered_evidence\"") || !strings.Contains(prompt, "\"technical_reports\"") {
		t.Fatalf("rebuttal example payload did not show supplemental materials:\n%s", prompt)
	}
	if !strings.Contains(prompt, "materialize the needed evidence into the workspace first") {
		t.Fatalf("rebuttal prompt did not instruct counsel to materialize exact file bytes:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Use offered_evidence only for visible evidence, by evidence_id.") {
		t.Fatalf("rebuttal prompt did not restrict offered_evidence to visible evidence ids:\n%s", prompt)
	}
	if !strings.Contains(prompt, "submit its content and provenance with aar_submit_evidence") {
		t.Fatalf("rebuttal prompt did not require outside source material to enter as submitted evidence:\n%s", prompt)
	}
}

func TestBuildAttorneyPromptConstrainsRebuttalWithoutSearch(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()
	rc := &runContext{
		cfg: Config{
			Policy:        DefaultPolicy(),
			AttorneyModel: "openai://gpt-5",
		},
		complaint: spec.Complaint{
			Proposition: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"evidence_standard": "preponderance",
			},
			"case": map[string]any{
				"phase":             "rebuttals",
				"openings":          []map[string]any{},
				"arguments":         []map[string]any{},
				"rebuttals":         []map[string]any{},
				"surrebuttals":      []map[string]any{},
				"closings":          []map[string]any{},
				"offered_evidence":  []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}
	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "rebuttals:plaintiff",
		Role:         "plaintiff",
		Phase:        "rebuttals",
		Objective:    "plaintiff rebuttal",
		AllowedTools: []string{"submit_rebuttal", "pass_phase_opportunity"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Native web search through the model is not available.") {
		t.Fatalf("rebuttal prompt did not state search unavailability:\n%s", prompt)
	}
}

func TestBuildCouncilPromptIncludesPersonaAndRecord(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()
	rc := &runContext{
		cfg: Config{
			Policy: DefaultPolicy(),
		},
		complaint: spec.Complaint{
			Proposition: "P",
		},
		state: map[string]any{
			"policy": map[string]any{
				"evidence_standard": "preponderance",
			},
			"case": map[string]any{
				"deliberation_round": 2,
				"openings":           []map[string]any{{"role": "plaintiff", "text": "opening"}},
				"arguments":          []map[string]any{},
				"rebuttals":          []map[string]any{},
				"surrebuttals":       []map[string]any{},
				"closings":           []map[string]any{},
				"offered_evidence":   []map[string]any{},
				"technical_reports":  []map[string]any{},
				"council_votes":      []map[string]any{{"round": 1, "member_id": "C1", "vote": "demonstrated", "rationale": "r"}},
			},
		},
	}
	prompt, err := rc.buildCouncilPrompt(CouncilSeat{
		MemberID:    "C2",
		PersonaText: "Skeptical but concise.",
	}, Opportunity{ID: "deliberation:2:C2", Role: "council", Phase: "deliberation"})
	if err != nil {
		t.Fatalf("buildCouncilPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Persona:\nSkeptical but concise.") {
		t.Fatalf("prompt did not include persona:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Prior rounds:\nRound 1 [C1] demonstrated") {
		t.Fatalf("prompt did not include prior rounds:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Call submit_council_vote with vote=demonstrated or vote=not_demonstrated.") {
		t.Fatalf("prompt did not include council instruction:\n%s", prompt)
	}
}

func TestEnsureACPSessionReusesExistingSession(t *testing.T) {
	t.Parallel()

	existing := &acpPersistentSession{sessionPath: "/tmp/existing"}
	rc := &runContext{
		acpSessions: map[string]*acpPersistentSession{
			"plaintiff": existing,
		},
	}
	session, err := rc.ensureACPSession(context.Background(), "plaintiff")
	if err != nil {
		t.Fatalf("ensureACPSession returned error: %v", err)
	}
	if session != existing {
		t.Fatalf("ensureACPSession returned %p, want existing %p", session, existing)
	}
}

func TestCloseACPSessionsClosesAndClears(t *testing.T) {
	t.Parallel()

	closed := make([]string, 0, 2)
	rc := &runContext{
		acpSessions: map[string]*acpPersistentSession{
			"defendant": {
				cleanup: func() error {
					closed = append(closed, "defendant")
					return nil
				},
			},
			"plaintiff": {
				cleanup: func() error {
					closed = append(closed, "plaintiff")
					return nil
				},
			},
		},
	}
	if err := rc.closeACPSessions(); err != nil {
		t.Fatalf("closeACPSessions returned error: %v", err)
	}
	if len(rc.acpSessions) != 0 {
		t.Fatalf("closeACPSessions left %d sessions", len(rc.acpSessions))
	}
	if got, want := closed, []string{"defendant", "plaintiff"}; !slices.Equal(got, want) {
		t.Fatalf("close order = %#v, want %#v", got, want)
	}
}

func TestIsFunctionArgumentParseError(t *testing.T) {
	t.Parallel()

	if isFunctionArgumentParseError(os.ErrInvalid) {
		t.Fatalf("unexpected parse-error match for os.ErrInvalid")
	}
	if !isFunctionArgumentParseError(fmt.Errorf("parse function arguments for submit_council_vote: unexpected end of JSON input")) {
		t.Fatalf("expected parse function arguments error to match")
	}
}

func TestIsCouncilTimeoutError(t *testing.T) {
	t.Parallel()

	if isCouncilTimeoutError(fmt.Errorf("provider failed")) {
		t.Fatalf("unexpected timeout match for generic error")
	}
	if !isCouncilTimeoutError(context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded to count as timeout")
	}
	if !isCouncilTimeoutError(fmt.Errorf("responses request canceled during backoff: %w", context.DeadlineExceeded)) {
		t.Fatalf("expected wrapped deadline exceeded to count as timeout")
	}
	if !isCouncilTimeoutError(fmt.Errorf("responses request failed: request timed out")) {
		t.Fatalf("expected timed out message to count as timeout")
	}
}

func TestRemoveTimedOutCouncilMemberRecordsEvent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' '{\"ok\":true,\"state\":{\"case\":{\"phase\":\"deliberation\",\"resolution\":\"\",\"council_members\":[{\"member_id\":\"C1\",\"status\":\"timed_out\"}]}}}'\n"
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	rc := &runContext{
		cfg: Config{
			Engine:    lean.Engine{Command: []string{enginePath}},
			OutputDir: dir,
		},
		state: map[string]any{
			"case": map[string]any{
				"phase": "deliberation",
			},
		},
	}
	opportunity := Opportunity{Phase: "deliberation"}
	seat := CouncilSeat{MemberID: "C1", Model: "openrouter://openai/gpt-4o"}
	if err := rc.removeTimedOutCouncilMember(opportunity, seat, context.DeadlineExceeded); err != nil {
		t.Fatalf("removeTimedOutCouncilMember returned error: %v", err)
	}
	caseObj := mapAny(rc.state["case"])
	members := mapList(caseObj["council_members"])
	if len(members) != 1 {
		t.Fatalf("council member count = %d, want 1", len(members))
	}
	if got := mapString(members[0]["status"]); got != "timed_out" {
		t.Fatalf("member status = %q, want timed_out", got)
	}
	if len(rc.events) != 1 {
		t.Fatalf("event count = %d, want 1", len(rc.events))
	}
	event := rc.events[0]
	if event.Type != "council_member_removed" {
		t.Fatalf("event type = %q, want council_member_removed", event.Type)
	}
	if got := mapString(event.Payload["member_id"]); got != "C1" {
		t.Fatalf("member_id = %q, want C1", got)
	}
	if got := mapString(event.Payload["status"]); got != "timed_out" {
		t.Fatalf("status = %q, want timed_out", got)
	}
}
