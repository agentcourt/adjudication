package manage

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// fakeService imitates the aar service envelope shapes.
type fakeService struct {
	lastCreate map[string]any
	killed     []string
	canceled   []string
}

func (f *fakeService) handler() http.Handler {
	mux := http.NewServeMux()
	clerkRecord := map[string]any{
		"case_id":    "arb-1",
		"run_id":     "run-arb-1",
		"status":     "running",
		"out_dir":    "/srv/out/arb-1",
		"created_at": "2026-07-24T10:00:00Z",
	}
	directRecord := map[string]any{
		"case_id":    "api-1",
		"status":     "completed",
		"out_dir":    "/srv/out/api-1",
		"created_at": "2026-07-24T09:00:00Z",
	}
	mux.HandleFunc("GET /clerk/v1/cases", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "cases": []any{clerkRecord}})
	})
	mux.HandleFunc("POST /clerk/v1/cases", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &f.lastCreate)
		writeJSON(w, 202, map[string]any{"ok": true, "case": map[string]any{"case_id": "arb-new"}})
	})
	mux.HandleFunc("GET /clerk/v1/cases/arb-1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "case": clerkRecord})
	})
	mux.HandleFunc("GET /clerk/v1/cases/arb-k", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "case": map[string]any{
			"case_id": "arb-k", "status": "killing", "out_dir": "/srv/out/arb-k",
		}})
	})
	mux.HandleFunc("POST /clerk/v1/cases/arb-1/kill", func(w http.ResponseWriter, r *http.Request) {
		f.killed = append(f.killed, "arb-1")
		writeJSON(w, 200, map[string]any{"ok": true, "case": clerkRecord})
	})
	mux.HandleFunc("GET /api/v1/cases", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "cases": []any{directRecord}})
	})
	mux.HandleFunc("GET /api/v1/cases/api-1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "case": directRecord})
	})
	mux.HandleFunc("GET /api/v1/cases/api-1/result", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "resolution": "demonstrated"})
	})
	mux.HandleFunc("POST /api/v1/cases/api-1/cancel", func(w http.ResponseWriter, r *http.Request) {
		f.canceled = append(f.canceled, "api-1")
		writeJSON(w, 200, map[string]any{"ok": true, "case": directRecord})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func newTestServer(t *testing.T) (*Server, *fakeService) {
	t.Helper()
	fake := &fakeService{}
	backend := httptest.NewServer(fake.handler())
	t.Cleanup(backend.Close)
	cfg := Config{
		ARBURL:      backend.URL,
		ReportURL:   "http://report.example",
		ReportRoots: []ReportRoot{{Name: "svc", Path: "/srv/out"}},
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return srv, fake
}

func get(t *testing.T, srv *Server, target string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("GET", target, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func postForm(t *testing.T, srv *Server, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestOverview(t *testing.T) {
	srv, _ := newTestServer(t)
	code, body := get(t, srv, "/")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	for _, want := range []string{
		"arb-1", "api-1", "running 1", "completed 1",
		`href="http://report.example/run/svc/arb-1"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("overview missing %q", want)
		}
	}
}

func TestCasePageAndStop(t *testing.T) {
	srv, fake := newTestServer(t)
	code, body := get(t, srv, "/clerk/arb-1")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	for _, want := range []string{"kill case", `action="/clerk/arb-1/kill"`, "run-arb-1", `href="http://report.example/run/svc/arb-1"`} {
		if !strings.Contains(body, want) {
			t.Errorf("case page missing %q", want)
		}
	}
	rec := postForm(t, srv, "/clerk/arb-1/kill", url.Values{})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("kill status %d", rec.Code)
	}
	if len(fake.killed) != 1 {
		t.Fatalf("killed: %v", fake.killed)
	}
}

func TestDirectCaseShowsResultWithoutStop(t *testing.T) {
	srv, _ := newTestServer(t)
	_, body := get(t, srv, "/direct/api-1")
	if strings.Contains(body, "cancel case") {
		t.Error("completed case offers cancel")
	}
	if !strings.Contains(body, "demonstrated") {
		t.Error("case page missing result")
	}
}

func TestStartSubmitCreatesAndRedirects(t *testing.T) {
	srv, fake := newTestServer(t)
	rec := postForm(t, srv, "/start", url.Values{
		"kind":         {"clerk"},
		"example":      {"ex01"},
		"council_size": {"7"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/clerk/arb-new" {
		t.Fatalf("location %q", loc)
	}
	if fake.lastCreate["example"] != "ex01" || fake.lastCreate["council_size"] != float64(7) {
		t.Fatalf("create payload: %v", fake.lastCreate)
	}
}

func TestStartSubmitFieldProblemRerenders(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := postForm(t, srv, "/start", url.Values{
		"kind":         {"clerk"},
		"council_size": {"seven"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "not an integer") || !strings.Contains(body, `value="seven"`) {
		t.Errorf("form not re-rendered with problem and value:\n%.500s", body)
	}
}

func TestRawSubmitPassesBodyUnchanged(t *testing.T) {
	srv, fake := newTestServer(t)
	rec := postForm(t, srv, "/raw", url.Values{
		"target":  {"clerk"},
		"payload": {`{"example":"ex01","extra_field":true}`},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if fake.lastCreate["extra_field"] != true {
		t.Fatalf("raw payload not passed through: %v", fake.lastCreate)
	}
	if !strings.Contains(rec.Body.String(), "HTTP 202") {
		t.Error("raw page missing response status")
	}
}

func TestKillingCaseRefreshesWithoutStopButton(t *testing.T) {
	srv, _ := newTestServer(t)
	code, body := get(t, srv, "/clerk/arb-k")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if !strings.Contains(body, `http-equiv="refresh"`) {
		t.Error("killing case page does not refresh")
	}
	if strings.Contains(body, "kill case") {
		t.Error("killing case page offers kill")
	}
	if !strings.Contains(body, `class="state-active">killing`) {
		t.Error("killing status has no active color")
	}
}

func TestCrossOriginPostRejected(t *testing.T) {
	srv, fake := newTestServer(t)
	cases := []struct {
		name    string
		headers map[string]string
		allowed bool
	}{
		{"no browser headers", nil, true},
		{"same-origin fetch metadata", map[string]string{"Sec-Fetch-Site": "same-origin"}, true},
		{"direct navigation", map[string]string{"Sec-Fetch-Site": "none"}, true},
		{"cross-site fetch metadata", map[string]string{"Sec-Fetch-Site": "cross-site"}, false},
		{"matching origin", map[string]string{"Origin": "http://example.com"}, true},
		{"foreign origin", map[string]string{"Origin": "http://evil.example"}, false},
		{"null origin", map[string]string{"Origin": "null"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := len(fake.killed)
			req := httptest.NewRequest("POST", "http://example.com/clerk/arb-1/kill", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if tc.allowed && rec.Code != http.StatusSeeOther {
				t.Fatalf("status %d, want redirect", rec.Code)
			}
			if !tc.allowed {
				if rec.Code != http.StatusForbidden {
					t.Fatalf("status %d, want 403", rec.Code)
				}
				if len(fake.killed) != before {
					t.Fatal("service received a rejected kill")
				}
			}
		})
	}
}

func TestServiceDownShowsError(t *testing.T) {
	cfg := Config{ARBURL: "http://127.0.0.1:1"}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	code, body := get(t, srv, "/")
	if code != http.StatusOK || !strings.Contains(body, "connection refused") {
		t.Fatalf("status %d body %.300s", code, body)
	}
}
