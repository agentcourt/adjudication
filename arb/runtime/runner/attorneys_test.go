package runner

import (
	"path/filepath"
	"strings"
	"testing"

	"adjudication/arb/runtime/spec"
)

func TestResolveAttorneyUsesEndpointWithoutModelSelection(t *testing.T) {
	t.Parallel()

	complaintPath := filepath.Join(t.TempDir(), "complaint.md")
	cfg := Config{
		AttorneyModel: "openai://gpt-5",
		ACPCommand:    "/tmp/acp-podman.sh",
		PlaintiffAttorney: AttorneyRoleConfig{
			ACPEndpoint: "tcp://127.0.0.1:7000",
		},
	}

	plaintiff, err := resolveAttorney("plaintiff", cfg, complaintPath)
	if err != nil {
		t.Fatalf("resolveAttorney(plaintiff) returned error: %v", err)
	}
	if plaintiff.Model != "" {
		t.Fatalf("plaintiff model = %q, want empty for remote ACP endpoint", plaintiff.Model)
	}
	if plaintiff.SearchEnabled != nil {
		t.Fatal("plaintiff search capability should be unknown for remote ACP endpoint")
	}
	if plaintiff.ACPTransport != "tcp" {
		t.Fatalf("plaintiff transport = %q, want tcp", plaintiff.ACPTransport)
	}
	if plaintiff.ACPEndpoint != "tcp://127.0.0.1:7000" {
		t.Fatalf("plaintiff endpoint = %q", plaintiff.ACPEndpoint)
	}
	if plaintiff.ACPCommand != "" {
		t.Fatalf("plaintiff command = %q, want empty", plaintiff.ACPCommand)
	}
	if plaintiff.SessionCwd != defaultRemoteSessionCwd {
		t.Fatalf("plaintiff session cwd = %q, want %q", plaintiff.SessionCwd, defaultRemoteSessionCwd)
	}

	defendant, err := resolveAttorney("defendant", cfg, complaintPath)
	if err != nil {
		t.Fatalf("resolveAttorney(defendant) returned error: %v", err)
	}
	if defendant.Model != "openai://gpt-5" {
		t.Fatalf("defendant model = %q", defendant.Model)
	}
	if defendant.SearchEnabled == nil || *defendant.SearchEnabled {
		t.Fatal("defendant search should be disabled")
	}
	if defendant.ACPTransport != "stdio" {
		t.Fatalf("defendant transport = %q, want stdio", defendant.ACPTransport)
	}
	if defendant.ACPCommand != "/tmp/acp-podman.sh" {
		t.Fatalf("defendant command = %q", defendant.ACPCommand)
	}
	wantDefendantCwd, _ := filepath.Abs(filepath.Dir(complaintPath))
	if defendant.SessionCwd != wantDefendantCwd {
		t.Fatalf("defendant session cwd = %q, want %q", defendant.SessionCwd, wantDefendantCwd)
	}
}

func TestResolveAttorneyRejectsRoleModelWithEndpoint(t *testing.T) {
	t.Parallel()

	complaintPath := filepath.Join(t.TempDir(), "complaint.md")
	cfg := Config{
		AttorneyModel: "openai://gpt-5",
		ACPCommand:    "/tmp/acp-podman.sh",
		PlaintiffAttorney: AttorneyRoleConfig{
			Model:       "openai://gpt-5?tools=search",
			ACPEndpoint: "tcp://127.0.0.1:7000",
		},
	}
	_, err := resolveAttorney("plaintiff", cfg, complaintPath)
	if err == nil || !strings.Contains(err.Error(), "remote ACP attorney owns model selection") {
		t.Fatalf("resolveAttorney error = %v, want remote ACP model-selection error", err)
	}
}

func TestBuildAttorneyPromptUsesRoleSpecificCapability(t *testing.T) {
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
		attorneys: map[string]AttorneyRunInfo{
			"plaintiff": {
				Role:          "plaintiff",
				Model:         "openai://gpt-5?tools=search",
				SearchEnabled: boolPtr(true),
				ACPTransport:  "stdio",
				ACPCommand:    "/tmp/acp-podman.sh",
			},
			"defendant": {
				Role:          "defendant",
				Model:         "openai://gpt-5",
				SearchEnabled: boolPtr(false),
				ACPTransport:  "stdio",
				ACPCommand:    "/tmp/acp-podman.sh",
			},
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
				"offered_artifacts": []map[string]any{},
				"technical_reports": []map[string]any{},
			},
		},
	}

	prompt, err := rc.buildAttorneyPrompt(Opportunity{
		ID:           "openings:defendant",
		Role:         "defendant",
		Phase:        "openings",
		Objective:    "defendant opening statement",
		AllowedTools: []string{"record_opening_statement"},
	})
	if err != nil {
		t.Fatalf("buildAttorneyPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Native web search through the model is not available.") {
		t.Fatalf("prompt did not use defendant capability:\n%s", prompt)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
