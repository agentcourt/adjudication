package console

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListCasesForwardsBearerAndRendersRows(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/clerk/v1/cases" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer service-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		writeTestJSON(w, map[string]any{
			"ok": true,
			"cases": []map[string]any{{
				"case_id":    "case-1",
				"run_id":     "run-case-1",
				"status":     "running",
				"created_at": "2026-07-10T00:00:00Z",
			}},
		})
	}))
	defer api.Close()
	app := testApp(t, api.URL, "service-token")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/arb/clerk/cases", nil)
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "case-1") || !strings.Contains(body, "running") {
		t.Fatalf("body missing case row: %s", body)
	}
}

func TestCreateCasePostsRawJSONAndRedirects(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/clerk/v1/cases" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("bad payload: %v", err)
		}
		if payload["case_id"] != "case-2" {
			t.Fatalf("payload = %#v", payload)
		}
		writeTestJSON(w, map[string]any{"ok": true, "case": map[string]any{"case_id": "case-2", "status": "starting"}})
	}))
	defer api.Close()
	app := testApp(t, api.URL, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/arb/clerk/cases", strings.NewReader("payload=%7B%22case_id%22%3A%22case-2%22%7D"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/system/arb/clerk/cases/case-2" {
		t.Fatalf("location = %q", got)
	}
}

func TestArtifactProxyUsesServiceAPI(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/clerk/v1/cases/case-3/artifacts/digest.md" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte("# Digest\n"))
	}))
	defer api.Close()
	app := testApp(t, api.URL, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/arb/clerk/cases/case-3/artifacts/digest.md", nil)
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "# Digest\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestArtifactProxyPreservesNestedArtifactName(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/cases/case-3/artifacts/service-logs/aar.stderr" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("stderr\n"))
	}))
	defer api.Close()
	app := testApp(t, api.URL, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/arb/direct/cases/case-3/artifacts/service-logs%2Faar.stderr", nil)
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "stderr\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestCaseDetailLinksStructuredLogFields(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clerk/v1/cases/case-log":
			writeTestJSON(w, map[string]any{
				"ok": true,
				"case": map[string]any{
					"case_id":    "case-log",
					"status":     "failed",
					"stdout_log": "/tmp/run/case-log/clerk.stdout",
					"stderr_log": "/tmp/run/case-log/clerk.stderr",
				},
			})
		case "/clerk/v1/cases/case-log/result":
			writeTestJSON(w, map[string]any{"ok": true, "case_id": "case-log", "status": "failed"})
		case "/clerk/v1/cases/case-log/artifacts":
			writeTestJSON(w, map[string]any{"ok": true, "case_id": "case-log", "artifacts": []map[string]any{{"name": "clerk.stderr", "size_bytes": 11}}})
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer api.Close()
	app := testApp(t, api.URL, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/arb/clerk/cases/case-log", nil)
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`href="/system/arb/clerk/cases/case-log/artifacts/clerk.stdout"`,
		`href="/system/arb/clerk/cases/case-log/artifacts/clerk.stderr"`,
		"/tmp/run/case-log/clerk.stderr",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestCaseDetailCompactsStructuredFieldsAndRefreshesRunningCase(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clerk/v1/cases/case-running":
			writeTestJSON(w, map[string]any{
				"ok": true,
				"case": map[string]any{
					"case_id": "case-running",
					"status":  "running",
					"summary": map[string]any{
						"answers": map[string]any{"C5": 73},
						"events":  []any{"one", "two"},
					},
				},
			})
		case "/clerk/v1/cases/case-running/result":
			writeTestJSON(w, map[string]any{"ok": true, "case_id": "case-running", "status": "running"})
		case "/clerk/v1/cases/case-running/artifacts":
			writeTestJSON(w, map[string]any{"ok": true, "case_id": "case-running", "artifacts": []map[string]any{}})
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer api.Close()
	app := testApp(t, api.URL, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/arb/clerk/cases/case-running", nil)
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<meta http-equiv="refresh" content="10">`,
		"object (2 keys)",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "map[answers") {
		t.Fatalf("body contains fmt map rendering: %s", body)
	}
}

func TestCaseDetailSummarizesFailureEvents(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clerk/v1/cases/case-failure":
			writeTestJSON(w, map[string]any{"ok": true, "case": map[string]any{"case_id": "case-failure", "status": "completed"}})
		case "/clerk/v1/cases/case-failure/result":
			writeTestJSON(w, map[string]any{"ok": true, "case_id": "case-failure", "status": "done"})
		case "/clerk/v1/cases/case-failure/artifacts":
			writeTestJSON(w, map[string]any{"ok": true, "case_id": "case-failure", "artifacts": []map[string]any{{"name": "events.ndjson", "size_bytes": 512}}})
		case "/clerk/v1/cases/case-failure/artifacts/events.ndjson":
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = w.Write([]byte(`{"timestamp":"2026-07-10T15:27:32Z","phase":"deliberation","type":"opportunity_failed","payload":{"member_id":"C2","process_name":"pi-C2","reason":"agent_exited","message":"Council member C2 failed: provider rejected function.arguments","agent_error_log":"/tmp/run/logs/pi-C2.stdout"}}` + "\n"))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer api.Close()
	app := testApp(t, api.URL, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/arb/clerk/cases/case-failure", nil)
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Failure Events",
		"opportunity_failed",
		"pi-C2",
		"provider rejected function.arguments",
		"/tmp/run/logs/pi-C2.stdout",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestEvidencePageRendersNonJSONEvidence(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clerk/v1/cases/case-4/artifacts/evidence-manifest.json":
			writeTestJSON(w, map[string]any{"ok": true, "evidence": []map[string]any{{
				"evidence_id":          "E1",
				"title":                "First exhibit",
				"mime_type":            "text/plain",
				"size_bytes":           15,
				"admissibility_status": "case_packet",
				"record_visibility":    "juror_visible",
			}}})
			return
		case "/clerk/v1/cases/case-4/evidence/E1":
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("evidence bytes\n"))
	}))
	defer api.Close()
	app := testApp(t, api.URL, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/arb/clerk/cases/case-4/evidence?id=E1", nil)
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "evidence bytes") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestEvidencePageListsManifestEntries(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/clerk/v1/cases/case-5/artifacts/evidence-manifest.json" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		writeTestJSON(w, map[string]any{"ok": true, "evidence": []map[string]any{{
			"evidence_id":          "ev_123",
			"title":                "Deadline thread",
			"mime_type":            "text/plain",
			"size_bytes":           820,
			"admissibility_status": "case_packet",
			"record_visibility":    "juror_visible",
		}}})
	}))
	defer api.Close()
	app := testApp(t, api.URL, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/arb/clerk/cases/case-5/evidence", nil)
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Evidence Manifest",
		`href="/system/arb/clerk/cases/case-5/evidence?id=ev_123"`,
		"Deadline thread",
		"case_packet",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestResponseTextOmitsLargeResponses(t *testing.T) {
	got := responseText(&Response{JSON: map[string]any{"text": strings.Repeat("x", 13000)}})
	if !strings.Contains(got, "response body not rendered") {
		t.Fatalf("response text = %s", got)
	}
	if strings.Contains(got, strings.Repeat("x", 1000)) {
		t.Fatalf("large response was embedded: %s", got)
	}
}

func TestRawRequestRejectsAbsoluteURL(t *testing.T) {
	app := testApp(t, "http://127.0.0.1:1", "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/arb/request", strings.NewReader("method=GET&path=https%3A%2F%2Fexample.com%2F"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "service path must start with /") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func testApp(t *testing.T, arbURL string, token string) *App {
	t.Helper()
	cfg := DefaultConfig()
	for id, sys := range cfg.Systems {
		sys.BaseURL = ""
		if id == "arb" {
			sys.BaseURL = arbURL
			sys.BearerToken = token
		}
		cfg.Systems[id] = sys
	}
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func writeTestJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	raw, _ := json.Marshal(value)
	_, _ = w.Write(raw)
}
