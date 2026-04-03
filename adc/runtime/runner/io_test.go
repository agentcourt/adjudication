package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"adjudication/adc/runtime/store"
)

func TestPersistAgentEventWritesNDJSONAndSQLite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, "events.ndjson")
	dbPath := filepath.Join(tmpDir, "run.db")

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()
	if err := st.CreateRun("run-1", "scenario"); err != nil {
		t.Fatalf("create run: %v", err)
	}

	r := &Runner{
		store: st,
		cfg: Config{
			RunID:      "run-1",
			EventsPath: eventsPath,
		},
	}
	payload := map[string]any{
		"tool_call_id": "call-1",
		"title":        "bash",
		"raw_input":    map[string]any{"cmd": "openssl dgst -sha256 -verify key.pem -signature sig msg.txt"},
	}
	if err := r.persistAgentEvent(7, 2, "plaintiff", "agent_tool_call", payload); err != nil {
		t.Fatalf("persistAgentEvent returned error: %v", err)
	}

	raw, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	text := string(raw)
	for _, needle := range []string{
		"\"timestamp\":",
		"\"agent_event\":\"agent_tool_call\"",
		"\"step\":-2",
		"openssl dgst -sha256 -verify key.pem -signature sig msg.txt",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("events log missing %q\n%s", needle, text)
		}
	}
}

func TestPersistAgentCompletionResultWritesNDJSONAndSQLite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	eventsPath := filepath.Join(tmpDir, "events.ndjson")
	dbPath := filepath.Join(tmpDir, "run.db")

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()
	if err := st.CreateRun("run-1", "scenario"); err != nil {
		t.Fatalf("create run: %v", err)
	}

	r := &Runner{
		store: st,
		cfg: Config{
			RunID:      "run-1",
			EventsPath: eventsPath,
		},
	}
	payload := map[string]any{
		"model":            "openrouter://google/gemini-2.5-flash",
		"opportunity_id":   "opp-24",
		"status":           "rejected",
		"invalid_attempt":  2,
		"response_text":    "I think I can be fair.",
		"rejection_reason": "Choose one allowed action or use a reference tool now.",
	}
	if err := r.persistAgentCompletionResult(24, 3, "juror", payload); err != nil {
		t.Fatalf("persistAgentCompletionResult returned error: %v", err)
	}

	raw, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	text := string(raw)
	for _, needle := range []string{
		"\"timestamp\":",
		"\"agent_event\":\"agent_completion_result\"",
		"\"turn\":24",
		"\"role\":\"juror\"",
		"\"response_text\":\"I think I can be fair.\"",
		"Choose one allowed action or use a reference tool now.",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("events log missing %q\n%s", needle, text)
		}
	}
}
