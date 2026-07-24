package manage

import (
	"reflect"
	"testing"
)

func values(m map[string]string) func(string) string {
	return func(name string) string { return m[name] }
}

func TestBuildPayloadClerk(t *testing.T) {
	payload, problems := BuildPayload("clerk", values(map[string]string{
		"example":       "ex01",
		"council_size":  "7",
		"case_files":    "a.txt\n\n b.txt \n",
		"auto_lawyers":  "both",
		"openclaw_auth": "codex",
	}))
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	want := map[string]any{
		"example":       "ex01",
		"council_size":  7,
		"case_files":    []string{"a.txt", "b.txt"},
		"auto_lawyers":  "both",
		"openclaw_auth": "codex",
	}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("payload %v, want %v", payload, want)
	}
}

func TestBuildPayloadAttested(t *testing.T) {
	payload, problems := BuildPayload("attested", values(map[string]string{
		"example":       "ex03",
		"input_prefix":  "s3://bucket/in",
		"expected_pcr4": "abc",
		"council_size":  "7",
	}))
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	if _, ok := payload["council_size"]; ok {
		t.Error("council_size applies to clerk runs; attested payload must omit it")
	}
	exec, ok := payload["execution"].(map[string]any)
	if !ok || exec["mode"] != "attested" {
		t.Fatalf("execution: %v", payload["execution"])
	}
	att, ok := exec["attestation"].(map[string]any)
	if !ok || att["input_prefix"] != "s3://bucket/in" || att["expected_pcr4"] != "abc" {
		t.Fatalf("attestation: %v", exec["attestation"])
	}
}

func TestBuildPayloadIntProblem(t *testing.T) {
	_, problems := BuildPayload("clerk", values(map[string]string{"council_size": "seven"}))
	if len(problems) != 1 {
		t.Fatalf("problems: %v", problems)
	}
}

func TestBuildPayloadDirectOmitsClerkFields(t *testing.T) {
	payload, _ := BuildPayload("direct", values(map[string]string{
		"complaint_path": "c.md",
		"openclaw_auth":  "codex",
		"council_size":   "5",
	}))
	want := map[string]any{"complaint_path": "c.md"}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("payload %v, want %v", payload, want)
	}
}
