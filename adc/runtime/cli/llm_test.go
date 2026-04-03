package cli

import (
	"os"
	"path/filepath"
	"testing"

	"adjudication/common/openai"
)

func TestResolveLLMPersonaSpecRandom(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "etc", "personas", "persons"), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	personaPath := filepath.Join(cwd, "etc", "personas", "persons", "a.txt")
	if err := os.WriteFile(personaPath, []byte("skeptical of unsigned documents"), 0o644); err != nil {
		t.Fatalf("WriteFile persona error = %v", err)
	}
	recordsPath := filepath.Join(cwd, "etc", "personas.csv")
	record := "openrouter://openai/gpt-5,personas/persons/a.txt\n"
	if err := os.WriteFile(recordsPath, []byte(record), 0o644); err != nil {
		t.Fatalf("WriteFile records error = %v", err)
	}

	spec, sampled, err := resolveLLMPersonaSpec("random", cwd)
	if err != nil {
		t.Fatalf("resolveLLMPersonaSpec random error = %v", err)
	}
	if !sampled {
		t.Fatalf("resolveLLMPersonaSpec random sampled = false, want true")
	}
	if spec.File != "personas/persons/a.txt" {
		t.Fatalf("resolveLLMPersonaSpec random file = %q", spec.File)
	}
}

func TestResolveLLMPersonaSpecFallsBackToEtcBase(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "etc", "personas", "persons"), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	personaPath := filepath.Join(cwd, "etc", "personas", "persons", "b.txt")
	if err := os.WriteFile(personaPath, []byte("requires corroboration"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	spec, sampled, err := resolveLLMPersonaSpec("openrouter://openai/gpt-5-mini,personas/persons/b.txt", cwd)
	if err != nil {
		t.Fatalf("resolveLLMPersonaSpec fallback error = %v", err)
	}
	if sampled {
		t.Fatalf("resolveLLMPersonaSpec fallback sampled = true, want false")
	}
	if spec.File != "personas/persons/b.txt" {
		t.Fatalf("resolveLLMPersonaSpec fallback file = %q", spec.File)
	}
}

func TestExtractToolCheckAnswer(t *testing.T) {
	t.Parallel()

	resp := openai.Response{
		ToolCalls: []openai.ToolCall{
			{
				Name:      llmToolCheckName,
				Arguments: map[string]any{"answer": "requires corroboration"},
			},
		},
	}
	got, err := extractToolCheckAnswer(resp)
	if err != nil {
		t.Fatalf("extractToolCheckAnswer error = %v", err)
	}
	if got != "requires corroboration" {
		t.Fatalf("extractToolCheckAnswer = %q", got)
	}
}

func TestExtractToolCheckAnswerRejectsMissingRequiredTool(t *testing.T) {
	t.Parallel()

	_, err := extractToolCheckAnswer(openai.Response{Text: "plain text"})
	if err == nil {
		t.Fatalf("extractToolCheckAnswer error = nil")
	}
}
