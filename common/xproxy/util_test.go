package xproxy

import (
	"testing"
	"time"
)

func TestLLMCSVLine(t *testing.T) {
	t.Parallel()

	got := llmCSVLineAt(time.Date(2026, 3, 18, 21, 4, 5, 678000000, time.UTC), "openai://gpt-5", 123, 456, 789)
	want := "llm_csv,2026-03-18 21:04:05.678,openai://gpt-5,123,456,789"
	if got != want {
		t.Fatalf("llmCSVLine = %q, want %q", got, want)
	}
}

func TestLLMCSVModel(t *testing.T) {
	t.Parallel()

	got := llmCSVModel(ModelSpec{Endpoint: "openai", ModelIn: "gpt-5"})
	if got != "openai://gpt-5" {
		t.Fatalf("llmCSVModel = %q", got)
	}

	got = llmCSVModel(ModelSpec{Endpoint: "openai", ModelIn: "gpt-5", ForceSearch: true})
	if got != "openai://gpt-5?tools=search" {
		t.Fatalf("llmCSVModel with search = %q", got)
	}
}
