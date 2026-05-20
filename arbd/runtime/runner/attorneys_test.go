package runner

import (
	"path/filepath"
	"strings"
	"testing"
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
