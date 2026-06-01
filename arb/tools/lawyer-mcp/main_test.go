package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMCPListsDynamicToolsForBoundCaseRole(t *testing.T) {
	fake := newFakeLawyerAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/lawyerapi/v1")

	sessionID := initializeMCP(t, srv, "arb-1", "plaintiff")
	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	})
	tools := resultTools(t, got)
	for _, want := range []string{"get_current_opportunity", "wait_for_opportunity", "get_case", "submit_decision"} {
		if !hasTool(tools, want) {
			t.Fatalf("tools/list missing %s: %#v", want, tools)
		}
	}
	if hasTool(tools, "list_evidence") {
		t.Fatalf("opening tools exposed list_evidence: %#v", tools)
	}

	fake.setPhase("arguments", []map[string]any{
		lawyerTool("get_case"),
		lawyerTool("list_evidence"),
		lawyerTool("submit_decision"),
	})
	got = callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/list",
	})
	tools = resultTools(t, got)
	for _, want := range []string{"get_current_opportunity", "wait_for_opportunity", "get_case", "list_evidence", "submit_decision"} {
		if !hasTool(tools, want) {
			t.Fatalf("argument tools/list missing %s: %#v", want, tools)
		}
	}
}

func TestMCPToolCallInjectsCaseRoleAndOpportunityID(t *testing.T) {
	fake := newFakeLawyerAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/lawyerapi/v1")
	sessionID := initializeMCP(t, srv, "arb-1", "plaintiff")

	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "submit_decision",
			"arguments": map[string]any{
				"kind":      "tool",
				"tool_name": "record_opening_statement",
				"payload": map[string]any{
					"text": "Opening.",
				},
			},
		},
	})
	if errObj := got["error"]; errObj != nil {
		t.Fatalf("tools/call returned RPC error: %#v", errObj)
	}
	result := rpcResult(t, got)
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("tools/call isError = true: %#v", result)
	}
	body := fake.lastDoBody(t)
	for _, field := range []struct {
		name string
		want string
	}{
		{name: "case_id", want: "arb-1"},
		{name: "role_id", want: "plaintiff"},
		{name: "opportunity_id", want: "openings:plaintiff"},
		{name: "tool", want: "submit_decision"},
	} {
		if got := mapString(body[field.name]); got != field.want {
			t.Fatalf("%s = %q, want %q in POST body %#v", field.name, got, field.want, body)
		}
	}
}

func TestMCPWaitingSessionExposesOnlyStatusTools(t *testing.T) {
	fake := newFakeLawyerAPI(t)
	defer fake.server.Close()
	fake.setWaiting()
	srv := newTestMCPServer(fake.server.URL + "/lawyerapi/v1")
	sessionID := initializeMCP(t, srv, "arb-1", "plaintiff")

	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/list",
	})
	tools := resultTools(t, got)
	if !hasTool(tools, "get_current_opportunity") {
		t.Fatalf("waiting tools missing get_current_opportunity: %#v", tools)
	}
	if !hasTool(tools, "wait_for_opportunity") {
		t.Fatalf("waiting tools missing wait_for_opportunity: %#v", tools)
	}
	if len(tools) != 2 {
		t.Fatalf("waiting tools = %#v, want get_current_opportunity and wait_for_opportunity", tools)
	}

	got = callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "submit_decision",
			"arguments": map[string]any{},
		},
	})
	result := rpcResult(t, got)
	if isError, _ := result["isError"].(bool); !isError {
		t.Fatalf("unavailable tool isError = false: %#v", result)
	}
	content := result["structuredContent"].(map[string]any)
	errObj := content["error"].(map[string]any)
	if code := mapString(errObj["code"]); code != "tool_unavailable" {
		t.Fatalf("error code = %q, want tool_unavailable", code)
	}
}

func TestMCPWaitForOpportunityReturnsReady(t *testing.T) {
	fake := newFakeLawyerAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/lawyerapi/v1")
	sessionID := initializeMCP(t, srv, "arb-1", "plaintiff")

	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]any{
			"name": waitToolName,
			"arguments": map[string]any{
				"after_opportunity_id": "previous:plaintiff",
				"after_version":        1,
				"timeout_ms":           200,
			},
		},
	})
	result := rpcResult(t, got)
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("wait_for_opportunity isError = true: %#v", result)
	}
	content := result["structuredContent"].(map[string]any)
	if state := mapString(content["state"]); state != "ready" {
		t.Fatalf("state = %q, want ready in %#v", state, content)
	}
	if afterVersion := mapNumberString(content["after_version"]); afterVersion != "2" {
		t.Fatalf("after_version = %q, want 2 in %#v", afterVersion, content)
	}
	query := fake.lastWaitQuery(t)
	if got := query.Get("after"); got != "previous:plaintiff" {
		t.Fatalf("wait after query = %q, want previous:plaintiff", got)
	}
	if got := query.Get("after_version"); got != "1" {
		t.Fatalf("wait after_version query = %q, want 1", got)
	}
	if got := query.Get("timeout_ms"); got != "200" {
		t.Fatalf("wait timeout_ms query = %q, want 200", got)
	}
}

func TestMCPWaitForOpportunityReturnsWaiting(t *testing.T) {
	fake := newFakeLawyerAPI(t)
	defer fake.server.Close()
	fake.setWaiting()
	srv := newTestMCPServer(fake.server.URL + "/lawyerapi/v1")
	sessionID := initializeMCP(t, srv, "arb-1", "plaintiff")

	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      11,
		"method":  "tools/call",
		"params": map[string]any{
			"name": waitToolName,
			"arguments": map[string]any{
				"after_version": 2,
				"timeout_ms":    90000,
			},
		},
	})
	result := rpcResult(t, got)
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("wait_for_opportunity waiting returned isError = true: %#v", result)
	}
	content := result["structuredContent"].(map[string]any)
	if state := mapString(content["state"]); state != "waiting" {
		t.Fatalf("state = %q, want waiting in %#v", state, content)
	}
	if afterVersion := mapNumberString(content["after_version"]); afterVersion != "3" {
		t.Fatalf("after_version = %q, want 3 in %#v", afterVersion, content)
	}
	query := fake.lastWaitQuery(t)
	if got := query.Get("timeout_ms"); got != "30000" {
		t.Fatalf("wait timeout_ms query = %q, want capped 30000", got)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "state: waiting") || !strings.Contains(text, "after_version: 3") {
		t.Fatalf("wait text missing loop fields:\n%s", text)
	}
}

func TestMCPWaitForOpportunityReturnsDone(t *testing.T) {
	fake := newFakeLawyerAPI(t)
	defer fake.server.Close()
	fake.setDone()
	srv := newTestMCPServer(fake.server.URL + "/lawyerapi/v1")
	sessionID := initializeMCP(t, srv, "arb-1", "plaintiff")

	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      12,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      waitToolName,
			"arguments": map[string]any{},
		},
	})
	result := rpcResult(t, got)
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("wait_for_opportunity done returned isError = true: %#v", result)
	}
	content := result["structuredContent"].(map[string]any)
	if state := mapString(content["state"]); state != "done" {
		t.Fatalf("state = %q, want done in %#v", state, content)
	}
}

func TestMCPReconnectCreatesNewSessionForSameCaseRole(t *testing.T) {
	fake := newFakeLawyerAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/lawyerapi/v1")

	first := initializeMCP(t, srv, "arb-1", "plaintiff")
	second := initializeMCP(t, srv, "arb-1", "plaintiff")
	if first == second {
		t.Fatalf("reconnect reused session id %q", first)
	}
	got := callMCP(t, srv, second, map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/list",
	})
	if !hasTool(resultTools(t, got), "submit_decision") {
		t.Fatalf("new session did not expose current tools: %#v", got)
	}
}

func TestMCPSessionExpiryDeletesIdleSession(t *testing.T) {
	fake := newFakeLawyerAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/lawyerapi/v1")
	srv.sessionTTL = time.Minute
	sessionID := initializeMCP(t, srv, "arb-1", "plaintiff")
	now := time.Now()
	srv.mu.Lock()
	srv.sessions[sessionID].LastSeen = now.Add(-2 * time.Minute)
	srv.mu.Unlock()

	if expired := srv.expireIdleSessions(now); expired != 1 {
		t.Fatalf("expired sessions = %d, want 1", expired)
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/list",
	})))
	req.Header.Set("Mcp-Session-Id", sessionID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expired session status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	rejoined := initializeMCP(t, srv, "arb-1", "plaintiff")
	if rejoined == sessionID {
		t.Fatalf("rejoined session reused expired session id %q", sessionID)
	}
	got := callMCP(t, srv, rejoined, map[string]any{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/list",
	})
	if !hasTool(resultTools(t, got), "submit_decision") {
		t.Fatalf("rejoined session did not expose current tools: %#v", got)
	}
}

func TestMCPSessionAccessRefreshesLastSeen(t *testing.T) {
	fake := newFakeLawyerAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/lawyerapi/v1")
	srv.sessionTTL = time.Hour
	sessionID := initializeMCP(t, srv, "arb-1", "plaintiff")
	oldSeen := time.Now().Add(-10 * time.Minute)
	srv.mu.Lock()
	srv.sessions[sessionID].LastSeen = oldSeen
	srv.mu.Unlock()

	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/list",
	})
	if !hasTool(resultTools(t, got), "submit_decision") {
		t.Fatalf("tools/list missing submit_decision after refresh: %#v", got)
	}
	srv.mu.Lock()
	refreshed := srv.sessions[sessionID].LastSeen
	srv.mu.Unlock()
	if !refreshed.After(oldSeen) {
		t.Fatalf("LastSeen was not refreshed: got %s, old %s", refreshed, oldSeen)
	}
}

func TestMCPObserverToolCallOmitsOpportunityID(t *testing.T) {
	fake := newFakeLawyerAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/lawyerapi/v1")
	sessionID := initializeMCP(t, srv, "arb-1", "observer")

	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/list",
	})
	tools := resultTools(t, got)
	if !hasTool(tools, "get_turn") {
		t.Fatalf("observer tools/list missing get_turn: %#v", tools)
	}

	got = callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      9,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_turn",
			"arguments": map[string]any{},
		},
	})
	if errObj := got["error"]; errObj != nil {
		t.Fatalf("observer tools/call returned RPC error: %#v", errObj)
	}
	body := fake.lastDoBody(t)
	if _, ok := body["opportunity_id"]; ok {
		t.Fatalf("observer POST body included opportunity_id: %#v", body)
	}
	for _, field := range []struct {
		name string
		want string
	}{
		{name: "case_id", want: "arb-1"},
		{name: "role_id", want: "observer"},
		{name: "tool", want: "get_turn"},
	} {
		if got := mapString(body[field.name]); got != field.want {
			t.Fatalf("%s = %q, want %q in POST body %#v", field.name, got, field.want, body)
		}
	}
}

func TestToolResultTextSummarizesWithoutDuplicatingStructuredContent(t *testing.T) {
	text := toolResultText(map[string]any{
		"ok":                   true,
		"case_id":              "arb-1",
		"role_id":              "plaintiff",
		"status":               "ready",
		"state":                "ready",
		"after_version":        json.Number("2"),
		"after_opportunity_id": "openings:plaintiff",
		"message":              "An opportunity is ready.",
		"prompt":               "Plaintiff prompt.",
		"wait": map[string]any{
			"reason":  "ready",
			"version": json.Number("2"),
		},
		"tools": []any{
			map[string]any{"name": "submit_decision"},
			map[string]any{"name": "get_case"},
		},
		"turn": map[string]any{
			"phase":              "openings",
			"opportunity_id":     "openings:plaintiff",
			"remaining_ms":       json.Number("120000"),
			"attempts_remaining": json.Number("3"),
		},
		"result": map[string]any{
			"content_base64": strings.Repeat("a", 8192),
		},
	})
	for _, want := range []string{
		"ok: true",
		"status: ready",
		"state: ready",
		"message: An opportunity is ready.",
		"after_version: 2",
		"after_opportunity_id: openings:plaintiff",
		"wait_reason: ready",
		"wait_version: 2",
		"role_id: plaintiff",
		"phase: openings",
		"opportunity_id: openings:plaintiff",
		"remaining_ms: 120000",
		"attempts_remaining: 3",
		"tools: get_case, submit_decision",
		"prompt:\nPlaintiff prompt.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("toolResultText missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "content_base64") || strings.Contains(text, strings.Repeat("a", 1024)) {
		t.Fatalf("toolResultText duplicated structured content:\n%s", text)
	}
}

func newTestMCPServer(base string) *mcpServer {
	return &mcpServer{
		defaultLawyerAPI: strings.TrimRight(base, "/"),
		caseLawyerAPI:    map[string]string{},
		allowedOrigins:   map[string]struct{}{},
		client:           &http.Client{Timeout: 5 * time.Second},
		sessions:         map[string]*mcpSession{},
	}
}

func initializeMCP(t *testing.T, srv *mcpServer, caseID string, roleID string) string {
	t.Helper()
	raw := mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "0"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp?case_id="+caseID+"&role_id="+roleID, bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("initialize status = %d, body %s", rec.Code, rec.Body.String())
	}
	sessionID := rec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize did not return Mcp-Session-Id")
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode initialize response: %v", err)
	}
	if got["error"] != nil {
		t.Fatalf("initialize error: %#v", got["error"])
	}
	return sessionID
}

func callMCP(t *testing.T, srv *mcpServer, sessionID string, body map[string]any) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(mustJSON(t, body)))
	req.Header.Set("Mcp-Session-Id", sessionID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("MCP status = %d, body %s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	return got
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return raw
}

func rpcResult(t *testing.T, got map[string]any) map[string]any {
	t.Helper()
	result, ok := got["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want object in response %#v", got["result"], got)
	}
	return result
}

func resultTools(t *testing.T, got map[string]any) []any {
	t.Helper()
	result := rpcResult(t, got)
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("result.tools = %#v, want array", result["tools"])
	}
	return tools
}

func hasTool(tools []any, name string) bool {
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		if mapString(tool["name"]) == name {
			return true
		}
	}
	return false
}

type fakeLawyerAPI struct {
	t      *testing.T
	server *httptest.Server

	mu        sync.Mutex
	phase     string
	status    string
	tools     []map[string]any
	version   int
	lastDoRaw []byte
	lastWait  url.Values
}

func newFakeLawyerAPI(t *testing.T) *fakeLawyerAPI {
	f := &fakeLawyerAPI{
		t:       t,
		phase:   "openings",
		status:  "ready",
		tools:   []map[string]any{lawyerTool("get_case"), lawyerTool("submit_decision")},
		version: 2,
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	return f
}

func (f *fakeLawyerAPI) setPhase(phase string, tools []map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.phase = phase
	f.status = "ready"
	f.tools = tools
}

func (f *fakeLawyerAPI) setWaiting() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = "waiting"
	f.tools = nil
	f.version = 3
}

func (f *fakeLawyerAPI) setDone() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = "done"
	f.tools = nil
	f.version = 4
}

func (f *fakeLawyerAPI) lastDoBody(t *testing.T) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	if err := json.Unmarshal(f.lastDoRaw, &body); err != nil {
		t.Fatalf("decode last do body: %v", err)
	}
	return body
}

func (f *fakeLawyerAPI) lastWaitQuery(t *testing.T) url.Values {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastWait
}

func (f *fakeLawyerAPI) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/lawyerapi/v1/get":
		roleID := r.URL.Query().Get("role_id")
		if r.URL.Query().Get("case_id") != "arb-1" || (roleID != "plaintiff" && roleID != "observer") {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": map[string]any{"code": "bad_role"}})
			return
		}
		if roleID == "observer" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":      true,
				"case_id": "arb-1",
				"role_id": "observer",
				"status":  "ready",
				"prompt":  "",
				"tools": []map[string]any{
					lawyerTool("get_turn"),
				},
				"turn": map[string]any{
					"role_id":            "plaintiff",
					"phase":              f.phase,
					"opportunity_id":     f.phase + ":plaintiff",
					"remaining_ms":       120000,
					"attempts_remaining": 3,
				},
			})
			return
		}
		resp := map[string]any{
			"ok":      true,
			"case_id": "arb-1",
			"role_id": "plaintiff",
			"status":  f.status,
			"prompt":  "Plaintiff prompt.",
			"tools":   f.tools,
			"turn": map[string]any{
				"role_id":            "plaintiff",
				"phase":              f.phase,
				"opportunity_id":     f.phase + ":plaintiff",
				"remaining_ms":       120000,
				"attempts_remaining": 3,
			},
		}
		if f.status == "waiting" {
			resp["prompt"] = ""
			resp["tools"] = []map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(resp)
	case r.Method == http.MethodGet && r.URL.Path == "/lawyerapi/v1/wait":
		roleID := r.URL.Query().Get("role_id")
		if r.URL.Query().Get("case_id") != "arb-1" || roleID != "plaintiff" {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": map[string]any{"code": "bad_role"}})
			return
		}
		f.lastWait = r.URL.Query()
		reason := "ready"
		if f.status == "waiting" {
			reason = "timeout"
		}
		resp := map[string]any{
			"ok":      true,
			"case_id": "arb-1",
			"role_id": "plaintiff",
			"status":  f.status,
			"prompt":  "Plaintiff prompt.",
			"tools":   f.tools,
			"turn": map[string]any{
				"role_id":            "plaintiff",
				"phase":              f.phase,
				"opportunity_id":     f.phase + ":plaintiff",
				"remaining_ms":       120000,
				"attempts_remaining": 3,
			},
			"wait": map[string]any{
				"reason":  reason,
				"version": f.version,
			},
		}
		if f.status == "waiting" {
			resp["prompt"] = ""
			resp["tools"] = []map[string]any{}
		}
		if f.status == "done" {
			resp["prompt"] = ""
			resp["tools"] = []map[string]any{}
			resp["turn"] = nil
			resp["wait"].(map[string]any)["reason"] = "done"
		}
		_ = json.NewEncoder(w).Encode(resp)
	case r.Method == http.MethodPost && r.URL.Path == "/lawyerapi/v1/do":
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			f.t.Fatalf("read POST body: %v", err)
		}
		f.lastDoRaw = raw
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"case_id": "arb-1",
			"role_id": mapString(bodyField(raw, "role_id")),
			"turn": map[string]any{
				"role_id":            "plaintiff",
				"phase":              f.phase,
				"opportunity_id":     f.phase + ":plaintiff",
				"remaining_ms":       119000,
				"attempts_remaining": 3,
			},
			"result": map[string]any{"text": "Decision accepted."},
		})
	default:
		http.NotFound(w, r)
	}
}

func bodyField(raw []byte, name string) any {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil
	}
	return body[name]
}

func lawyerTool(name string) map[string]any {
	return map[string]any{
		"name":        name,
		"description": "tool " + name,
		"input_schema": map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": true,
		},
		"read_only": name != "submit_decision" && name != "submit_evidence",
	}
}
