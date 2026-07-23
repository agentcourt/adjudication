package report

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testRunJSON = `{
  "case_id": "arb-test",
  "run_id": "run-arb-test",
  "started_at": "2026-07-16T19:26:58Z",
  "finished_at": "2026-07-16T19:44:43Z",
  "status": "ok",
  "phase": "closed",
  "resolution": "not_demonstrated",
  "evidence_standard": "Preponderance of the evidence.",
  "complaint": {"Proposition": "The claim holds."},
  "council": [{"member_id": "C1", "model": "openrouter://m", "persona_file": "personas/generic.md"}],
  "attorneys": [{"role": "plaintiff", "interface": "lawyerapi"}]
}`

const testStateJSON = `{
  "schema_version": "aar.state.v0",
  "case": {
    "case_id": "arb-test",
    "phase": "closed",
    "status": "ok",
    "resolution": "not_demonstrated",
    "proposition": "The claim holds.",
    "council_votes": [
      {"member_id": "C1", "round": 1, "vote": "not_demonstrated", "rationale": "Insufficient evidence."},
      {"member_id": "C2", "round": 1, "vote": "demonstrated", "rationale": "PX-1 suffices."}
    ],
    "failure": null
  }
}`

const testEvents = `{"timestamp":"2026-07-16T19:27:02Z","turn":0,"role":"system","phase":"openings","type":"run_initialized","payload":{"k":"v"}}
{"timestamp":"2026-07-16T19:30:00Z","turn":1,"role":"plaintiff","phase":"openings","type":"opening_filed","payload":{"text":"hello"}}
`

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	run := filepath.Join(dir, "out", "arb-test")
	writeFile(t, filepath.Join(run, "run.json"), testRunJSON)
	writeFile(t, filepath.Join(run, "state.json"), testStateJSON)
	writeFile(t, filepath.Join(run, "events.ndjson"), testEvents)
	writeFile(t, filepath.Join(run, "digest.md"), "# Digest\n\nResolution: `not_demonstrated`\n")
	writeFile(t, filepath.Join(run, "logs", "aar.log"), "log line\n")
	writeFile(t, filepath.Join(run, "blob.bin"), "\xff\xfe\x00\x01")

	cfg, err := LoadConfig("", []Root{{Name: "t", Path: dir}}, "")
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return srv, dir
}

func get(t *testing.T, srv *Server, url string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("GET", url, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func TestIndex(t *testing.T) {
	srv, _ := newTestServer(t)
	code, body := get(t, srv, "/")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	for _, want := range []string{"arb-test", "demonstrated 1, not_demonstrated 1", `href="/run/t/out/arb-test"`} {
		if !strings.Contains(body, want) {
			t.Errorf("index missing %q", want)
		}
	}
}

func TestIndexUnknownRootFilter(t *testing.T) {
	srv, _ := newTestServer(t)
	code, _ := get(t, srv, "/?root=nope")
	if code != http.StatusNotFound {
		t.Fatalf("status %d", code)
	}
}

func TestRunPage(t *testing.T) {
	srv, _ := newTestServer(t)
	code, body := get(t, srv, "/run/t/out/arb-test")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	for _, want := range []string{
		"not_demonstrated",
		"Insufficient evidence.",
		"run_initialized",
		"digest.md",
		`href="/view/t/out/arb-test/logs/aar.log"`,
		`href="/raw/t/out/arb-test/run.json"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("run page missing %q", want)
		}
	}
}

func TestRunPageEventParseLimit(t *testing.T) {
	srv, dir := newTestServer(t)
	big := make([]byte, viewMaxBytes+1)
	for i := range big {
		big[i] = '\n'
	}
	if err := os.WriteFile(filepath.Join(dir, "out", "arb-test", "events.ndjson"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	code, body := get(t, srv, "/run/t/out/arb-test")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if !strings.Contains(body, "parse limit") {
		t.Error("run page missing parse-limit notice")
	}
	if strings.Contains(body, "run_initialized") {
		t.Error("run page parsed events despite the size limit")
	}
}

func TestViewMarkdown(t *testing.T) {
	srv, _ := newTestServer(t)
	code, body := get(t, srv, "/view/t/out/arb-test/digest.md")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	for _, want := range []string{"<h1>Digest</h1>", "<code>not_demonstrated</code>", `href="/run/t/out/arb-test"`, "?as=text"} {
		if !strings.Contains(body, want) {
			t.Errorf("markdown view missing %q", want)
		}
	}
	_, text := get(t, srv, "/view/t/out/arb-test/digest.md?as=text")
	if !strings.Contains(text, "# Digest") || strings.Contains(text, "<h1>Digest</h1>") {
		t.Errorf("text view wrong:\n%s", text)
	}
}

func TestViewJSONAndNDJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	_, body := get(t, srv, "/view/t/out/arb-test/run.json")
	if !strings.Contains(body, "&#34;case_id&#34;: &#34;arb-test&#34;") {
		t.Errorf("json view not pretty-printed:\n%s", body)
	}
	_, body = get(t, srv, "/view/t/out/arb-test/events.ndjson")
	if !strings.Contains(body, "<details>") || !strings.Contains(body, "run_initialized") {
		t.Errorf("ndjson view wrong:\n%s", body)
	}
}

func TestViewBinaryAndDir(t *testing.T) {
	srv, _ := newTestServer(t)
	_, body := get(t, srv, "/view/t/out/arb-test/blob.bin")
	if !strings.Contains(body, "Binary file") {
		t.Errorf("binary view wrong:\n%s", body)
	}
	_, body = get(t, srv, "/view/t/out/arb-test/logs")
	if !strings.Contains(body, "aar.log") {
		t.Errorf("dir view wrong:\n%s", body)
	}
}

func TestRaw(t *testing.T) {
	srv, _ := newTestServer(t)
	code, body := get(t, srv, "/raw/t/out/arb-test/digest.md")
	if code != http.StatusOK || body != "# Digest\n\nResolution: `not_demonstrated`\n" {
		t.Fatalf("raw mismatch: %d %q", code, body)
	}
}

func TestPathConfinement(t *testing.T) {
	srv, dir := newTestServer(t)
	outside := filepath.Join(filepath.Dir(dir), "secret.txt")
	if err := os.WriteFile(outside, []byte("CONFINEMENT-CANARY"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The mux redirects literal dot-dot paths before matching, so those
	// requests may answer 301; no request may reach the file content.
	for _, url := range []string{
		"/raw/t/../secret.txt",
		"/raw/t/%2e%2e/secret.txt",
		"/view/t/../secret.txt",
		"/raw/nope/x",
	} {
		code, body := get(t, srv, url)
		if code == http.StatusOK || strings.Contains(body, "CONFINEMENT-CANARY") {
			t.Errorf("%s: status %d body %q", url, code, body)
		}
	}
}

func TestSymlinkEscapeBlocked(t *testing.T) {
	srv, dir := newTestServer(t)
	outside := filepath.Join(filepath.Dir(dir), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "out", "arb-test", "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	code, body := get(t, srv, "/raw/t/out/arb-test/escape")
	if code != http.StatusNotFound || strings.Contains(body, "outside") {
		t.Fatalf("symlink escape served: %d %q", code, body)
	}
}
