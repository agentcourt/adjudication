package persona

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRecordLoadsRelativeFile(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "personas"), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	personaPath := filepath.Join(baseDir, "personas", "j1.txt")
	if err := os.WriteFile(personaPath, []byte(" skeptical about screenshots \n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	spec, err := ParseRecord("openrouter://openai/gpt-5, personas/j1.txt", baseDir)
	if err != nil {
		t.Fatalf("ParseRecord error = %v", err)
	}
	if spec.Model != "openrouter://openai/gpt-5" {
		t.Fatalf("Model = %q", spec.Model)
	}
	if spec.File != "personas/j1.txt" {
		t.Fatalf("File = %q", spec.File)
	}
	if spec.Text != "skeptical about screenshots" {
		t.Fatalf("Text = %q", spec.Text)
	}
	if spec.FilePath != filepath.Clean(personaPath) {
		t.Fatalf("FilePath = %q, want %q", spec.FilePath, personaPath)
	}
}

func TestParseRecordLoadsAbsoluteFile(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	personaPath := filepath.Join(baseDir, "j2.txt")
	if err := os.WriteFile(personaPath, []byte("doubts digital signatures"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	spec, err := ParseRecord("openai://gpt-5-mini,"+personaPath, baseDir)
	if err != nil {
		t.Fatalf("ParseRecord error = %v", err)
	}
	if spec.FilePath != personaPath {
		t.Fatalf("FilePath = %q, want %q", spec.FilePath, personaPath)
	}
}

func TestLoadRecordsFile(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "personas"), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "personas", "a.txt"), []byte("first persona"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "personas", "b.txt"), []byte("second persona"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	recordsPath := filepath.Join(baseDir, "personas.csv")
	content := strings.Join([]string{
		"# comment",
		"",
		"openrouter://openai/gpt-5,personas/a.txt",
		`{"openrouter_model_id":"deepseek/deepseek-v4-flash","endpoint_tag":"deepinfra/fp4","quantization":"fp4","persona":"personas/b.txt"}`,
		"",
	}, "\n")
	if err := os.WriteFile(recordsPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	specs, err := LoadRecordsFile("personas.csv", baseDir)
	if err != nil {
		t.Fatalf("LoadRecordsFile error = %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("LoadRecordsFile count = %d, want 2", len(specs))
	}
	if specs[0].File != "personas/a.txt" || specs[1].File != "personas/b.txt" {
		t.Fatalf("LoadRecordsFile files = %q, %q", specs[0].File, specs[1].File)
	}
	if specs[0].RequestSpec != nil {
		t.Fatalf("legacy CSV RequestSpec = %#v, want nil", specs[0].RequestSpec)
	}
	if specs[1].RequestSpec == nil {
		t.Fatalf("JSON record RequestSpec = nil")
	}
	if specs[1].Model != "openrouter://deepseek/deepseek-v4-flash" {
		t.Fatalf("JSON record model = %q", specs[1].Model)
	}
	provider := specs[1].RequestSpec.ProviderBody()
	if provider["only"].([]string)[0] != "deepinfra/fp4" {
		t.Fatalf("JSON record provider = %#v", provider)
	}
}

func TestLoadRecordsFileResolvesSharedDataPoolPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dataDir := filepath.Join(root, "common", "data", "personas")
	etcDir := filepath.Join(root, "common", "etc", "personas")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll data error = %v", err)
	}
	if err := os.MkdirAll(etcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll etc error = %v", err)
	}
	personaPath := filepath.Join(etcDir, "a.txt")
	if err := os.WriteFile(personaPath, []byte("shared persona"), 0o644); err != nil {
		t.Fatalf("WriteFile persona error = %v", err)
	}
	recordsPath := filepath.Join(dataDir, "pool.csv")
	if err := os.WriteFile(recordsPath, []byte("openrouter://openai/gpt-5,personas/a.txt\n"), 0o644); err != nil {
		t.Fatalf("WriteFile records error = %v", err)
	}

	specs, err := LoadRecordsFile(recordsPath, root)
	if err != nil {
		t.Fatalf("LoadRecordsFile error = %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("LoadRecordsFile count = %d, want 1", len(specs))
	}
	if specs[0].FilePath != filepath.Clean(personaPath) {
		t.Fatalf("FilePath = %q, want %q", specs[0].FilePath, personaPath)
	}
}

func TestSampleRecordFileSingle(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "personas"), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "personas", "only.txt"), []byte("only persona"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "personas.csv"), []byte("openrouter://openai/gpt-5,personas/only.txt\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	spec, err := SampleRecordFile("personas.csv", baseDir)
	if err != nil {
		t.Fatalf("SampleRecordFile error = %v", err)
	}
	if spec.File != "personas/only.txt" {
		t.Fatalf("SampleRecordFile file = %q", spec.File)
	}
}

func TestParseRecordRejectsBadInput(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	emptyPath := filepath.Join(baseDir, "empty.txt")
	if err := os.WriteFile(emptyPath, []byte(" \n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	tests := []struct {
		name   string
		record string
		want   string
	}{
		{name: "missing comma", record: "openai://gpt-5-mini", want: "invalid persona record"},
		{name: "blank model", record: ",persona.txt", want: "invalid persona record"},
		{name: "invalid model", record: "openai/gpt-5-mini,persona.txt", want: "invalid persona model"},
		{name: "missing file", record: "openai://gpt-5-mini,missing.txt", want: "read persona text"},
		{name: "empty file", record: "openai://gpt-5-mini," + emptyPath, want: "empty persona text"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseRecord(tt.record, baseDir)
			if err == nil {
				t.Fatalf("ParseRecord(%q) error = nil, want %q", tt.record, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseRecord(%q) error = %v, want substring %q", tt.record, err, tt.want)
			}
		})
	}
}

func TestJurorPrompt(t *testing.T) {
	t.Parallel()

	if got := JurorPrompt("", " \n "); got != "" {
		t.Fatalf("JurorPrompt empty = %q, want empty string", got)
	}

	got := JurorPrompt("J4", " skeptical of unsigned drafts ")
	if !strings.Contains(got, "You are J4.") {
		t.Fatalf("JurorPrompt missing juror id: %q", got)
	}
	if !strings.Contains(got, "skeptical of unsigned drafts") {
		t.Fatalf("JurorPrompt missing persona text: %q", got)
	}
}

func TestVoteExplanationPromptMatchesJurorPrompt(t *testing.T) {
	t.Parallel()

	jurorPrompt := JurorPrompt("J8", "insists on corroboration")
	if got := VoteExplanationPrompt("J8", "insists on corroboration"); got != jurorPrompt {
		t.Fatalf("VoteExplanationPrompt = %q, want %q", got, jurorPrompt)
	}
}
