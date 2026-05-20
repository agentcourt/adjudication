package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"slices"
	"testing"
)

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
		"relevance":              "Shows the disputed comparison.",
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
	if file.FileID != meta.FileID || !file.TextReadable || file.Text != content {
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

func TestACPToolSpecsExposeEvidenceOnlyInRecordBuildingPhases(t *testing.T) {
	argumentTools := toolNames(acpToolSpecs(Opportunity{Phase: "arguments"}, true))
	if !slices.Contains(argumentTools, "aar_submit_evidence") {
		t.Fatalf("argument tools = %#v, want aar_submit_evidence", argumentTools)
	}
	if !slices.Contains(argumentTools, "aar_write_case_file") {
		t.Fatalf("argument tools = %#v, want workspace writer", argumentTools)
	}

	closingTools := toolNames(acpToolSpecs(Opportunity{Phase: "closings"}, true))
	if slices.Contains(closingTools, "aar_submit_evidence") {
		t.Fatalf("closing tools = %#v, do not want aar_submit_evidence", closingTools)
	}
	if slices.Contains(closingTools, "aar_write_case_file") {
		t.Fatalf("closing tools = %#v, do not want workspace writer", closingTools)
	}
}

func toolNames(specs []map[string]any) []string {
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, mapString(spec["toolName"]))
	}
	return out
}
