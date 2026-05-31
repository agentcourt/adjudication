package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"adjudication/arbd/runtime/spec"
)

func TestACPToolSpecsExposeEvidenceOnlyInArgumentPhases(t *testing.T) {
	openingTools := toolNames(acpToolSpecs(Opportunity{Phase: "openings"}, true))
	for _, forbidden := range []string{"aar_list_evidence", "aar_read_evidence_range", "aar_begin_evidence_upload", "aar_submit_evidence"} {
		if slices.Contains(openingTools, forbidden) {
			t.Fatalf("opening tools exposed %s: %#v", forbidden, openingTools)
		}
	}

	for _, phase := range []string{"arguments", "rebuttals"} {
		tools := toolNames(acpToolSpecs(Opportunity{Phase: phase}, true))
		for _, want := range []string{
			"aar_get_case",
			"aar_list_evidence",
			"aar_stat_evidence",
			"aar_read_evidence_range",
			"aar_materialize_evidence",
			"aar_begin_evidence_upload",
			"aar_write_evidence_chunk",
			"aar_commit_evidence_upload",
			"aar_submit_evidence",
			"aar_submit_decision",
		} {
			if !slices.Contains(tools, want) {
				t.Fatalf("%s tools missing %s: %#v", phase, want, tools)
			}
		}
	}
}

func TestAttorneyPromptMetaUsesOpportunityToolSpecs(t *testing.T) {
	openingSpecs := listOfMaps(mapAny(attorneyPromptMeta(Opportunity{Phase: "openings", AllowedTools: []string{"record_opening_statement"}}, true)["agentcourt"])["clientTools"])
	openingTools := toolNames(openingSpecs)
	for _, forbidden := range []string{"aar_list_evidence", "aar_read_evidence_range", "aar_submit_evidence"} {
		if slices.Contains(openingTools, forbidden) {
			t.Fatalf("opening metadata exposed %s: %#v", forbidden, openingTools)
		}
	}
	if !slices.Contains(openingTools, "aar_submit_decision") {
		t.Fatalf("opening metadata missing aar_submit_decision: %#v", openingTools)
	}
	openingEnum := submitDecisionEnum(openingSpecs)
	if len(openingEnum) != 1 || openingEnum[0] != "record_opening_statement" {
		t.Fatalf("opening submit_decision enum = %#v, want record_opening_statement only", openingEnum)
	}

	argumentSpecs := listOfMaps(mapAny(attorneyPromptMeta(Opportunity{Phase: "arguments", AllowedTools: []string{"submit_argument"}}, true)["agentcourt"])["clientTools"])
	argumentTools := toolNames(argumentSpecs)
	for _, want := range []string{"aar_get_case", "aar_read_evidence_range", "aar_submit_evidence", "aar_submit_decision"} {
		if !slices.Contains(argumentTools, want) {
			t.Fatalf("argument metadata missing %s: %#v", want, argumentTools)
		}
	}
	argumentEnum := submitDecisionEnum(argumentSpecs)
	if len(argumentEnum) != 1 || argumentEnum[0] != "submit_argument" {
		t.Fatalf("argument submit_decision enum = %#v, want submit_argument only", argumentEnum)
	}
}

func TestAttorneyPromptRequiresSubmittedEvidenceForNewSources(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()

	rc := &runContext{
		cfg:       Config{Policy: DefaultPolicy()},
		complaint: spec.Complaint{Question: "What percentage is reused?"},
		state: map[string]any{
			"policy": map[string]any{"judgment_standard": "Answer with one integer."},
			"case": map[string]any{
				"phase":              "arguments",
				"openings":           []map[string]any{},
				"arguments":          []map[string]any{},
				"rebuttals":          []map[string]any{},
				"surrebuttals":       []map[string]any{},
				"closings":           []map[string]any{},
				"offered_evidence":   []map[string]any{},
				"technical_reports":  []map[string]any{},
				"submitted_evidence": []map[string]any{},
			},
		},
	}

	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "arguments:plaintiff",
		Role:         "plaintiff",
		Phase:        "arguments",
		Objective:    "plaintiff merits argument",
		AllowedTools: []string{"submit_evidence", "submit_argument"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	for _, want := range []string{
		"Question: What percentage is reused?",
		"Judgment standard: Answer with one integer.",
		"submit its content and provenance with aar_submit_evidence",
		"materialize the needed evidence into the workspace first",
		"Use only visible case evidence_id values in offered_evidence. Submit new source material first with aar_submit_evidence",
		"Use technical_reports for attorney analysis or synthesized work product",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestChunkedEvidenceUploadPersistsParentMetadata(t *testing.T) {
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
	parentPath := filepath.Join(dir, "parent.txt")
	if err := os.WriteFile(parentPath, []byte("parent source"), 0o644); err != nil {
		t.Fatalf("write parent evidence: %v", err)
	}
	parent, err := rc.registerCaseFileEvidence(CaseFile{EvidenceID: "parent-source.txt", Name: "parent.txt", Path: parentPath, MimeType: "text/plain", TextReadable: true, SizeBytes: len("parent source")})
	if err != nil {
		t.Fatalf("register parent evidence: %v", err)
	}

	raw := []byte("derived source")
	sha := sha256.Sum256(raw)
	session, err := rc.beginEvidenceUpload(Opportunity{Role: "plaintiff", Phase: "arguments"}, map[string]any{
		"title":               "Derived source",
		"mime_type":           "text/plain",
		"expected_size_bytes": len(raw),
		"expected_sha256":     hex.EncodeToString(sha[:]),
		"source_description":  "derived from parent",
		"relevance":           "tests parent metadata",
		"parent_evidence_id":  parent.EvidenceID,
		"derivation_method":   "manual excerpt",
	})
	if err != nil {
		t.Fatalf("beginEvidenceUpload returned error: %v", err)
	}
	if _, n, err := rc.writeEvidenceChunk(session.UploadID, 0, "ZGVyaXZlZCBzb3VyY2U="); err != nil || n != len(raw) {
		t.Fatalf("write chunk = %d, %v", n, err)
	}
	meta, err := rc.prepareEvidenceUploadCommit(session, "txt")
	if err != nil {
		t.Fatalf("prepareEvidenceUploadCommit returned error: %v", err)
	}
	_, _, evidence, err := rc.finalizeEvidenceUpload(session, meta)
	if err != nil {
		t.Fatalf("finalizeEvidenceUpload returned error: %v", err)
	}
	if evidence.ParentEvidenceID != parent.EvidenceID || evidence.ParentSHA256 != parent.SHA256 || evidence.DerivationMethod != "manual excerpt" {
		t.Fatalf("derived evidence metadata = %#v, parent = %#v", evidence, parent)
	}
	stored := rc.evidenceByID[evidence.EvidenceID]
	if stored.ParentEvidenceID != parent.EvidenceID || stored.ParentSHA256 != parent.SHA256 || stored.DerivationMethod != "manual excerpt" {
		t.Fatalf("stored evidence metadata = %#v, parent = %#v", stored, parent)
	}
	manifest := rc.evidenceManifest()
	items, ok := manifest["evidence"].([]EvidenceMeta)
	if !ok {
		t.Fatalf("manifest evidence has type %T", manifest["evidence"])
	}
	found := false
	for _, item := range items {
		if item.EvidenceID == evidence.EvidenceID {
			found = true
			if item.ParentEvidenceID != parent.EvidenceID || item.ParentSHA256 != parent.SHA256 || item.DerivationMethod != "manual excerpt" {
				t.Fatalf("manifest derived metadata = %#v, parent = %#v", item, parent)
			}
		}
	}
	if !found {
		t.Fatalf("derived evidence %s missing from manifest %#v", evidence.EvidenceID, manifest)
	}
}

func TestChunkedEvidenceUploadRejectsUnknownParent(t *testing.T) {
	rc := &runContext{
		cfg:          Config{Policy: DefaultPolicy()},
		evidenceByID: map[string]EvidenceMeta{},
	}
	_, err := rc.beginEvidenceUpload(Opportunity{Role: "plaintiff", Phase: "arguments"}, map[string]any{
		"title":               "Derived source",
		"mime_type":           "text/plain",
		"expected_size_bytes": 3,
		"source_description":  "derived from missing parent",
		"relevance":           "tests parent validation",
		"parent_evidence_id":  "ev_missing_parent",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid parent_evidence_id") {
		t.Fatalf("beginEvidenceUpload error = %v, want invalid parent", err)
	}
}

func TestDefaultPolicyAllowsUploadedEvidenceToBeOffered(t *testing.T) {
	policy := DefaultPolicy()
	if policy.MaxExhibitBytes != 64*1024*1024 {
		t.Fatalf("MaxExhibitBytes = %d, want 64 MiB", policy.MaxExhibitBytes)
	}
	if policy.MaxExhibitBytes < policy.MaxEvidenceUploadBytes {
		t.Fatalf("MaxExhibitBytes = %d, MaxEvidenceUploadBytes = %d", policy.MaxExhibitBytes, policy.MaxEvidenceUploadBytes)
	}
	loaded, err := LoadPolicyFile(filepath.Join("..", "..", "etc", "policy.json"))
	if err != nil {
		t.Fatalf("LoadPolicyFile returned error: %v", err)
	}
	if loaded.MaxExhibitBytes != policy.MaxExhibitBytes || loaded.MaxExhibitBytes < loaded.MaxEvidenceUploadBytes {
		t.Fatalf("loaded policy max_exhibit_bytes = %d, max_evidence_upload_bytes = %d", loaded.MaxExhibitBytes, loaded.MaxEvidenceUploadBytes)
	}
}

func toolNames(specs []map[string]any) []string {
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, mapString(spec["toolName"]))
	}
	return out
}

func submitDecisionEnum(specs []map[string]any) []string {
	for _, spec := range specs {
		if mapString(spec["toolName"]) != "aar_submit_decision" {
			continue
		}
		enum, _ := mapAny(mapAny(mapAny(spec["parameters"])["properties"])["tool_name"])["enum"].([]string)
		return enum
	}
	return nil
}
