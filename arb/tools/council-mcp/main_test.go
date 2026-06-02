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

func TestMCPListsCouncilToolsForBoundCaseMember(t *testing.T) {
	fake := newFakeCouncilAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/councilapi/v1")

	sessionID := initializeMCP(t, srv, "arb-1", "C1")
	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	})
	tools := resultTools(t, got)
	for _, want := range []string{"get_current_council_opportunity", "wait_for_council_opportunity", "get_case", "submit_council_vote"} {
		if !hasTool(tools, want) {
			t.Fatalf("tools/list missing %s: %#v", want, tools)
		}
	}

	fake.setWaiting()
	got = callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/list",
	})
	tools = resultTools(t, got)
	for _, want := range []string{"get_current_council_opportunity", "wait_for_council_opportunity"} {
		if !hasTool(tools, want) {
			t.Fatalf("waiting tools/list missing %s: %#v", want, tools)
		}
	}
	if hasTool(tools, "submit_council_vote") {
		t.Fatalf("waiting tools/list exposed submit_council_vote: %#v", tools)
	}
}

func TestMCPToolCallInjectsCaseMemberAndOpportunityID(t *testing.T) {
	fake := newFakeCouncilAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/councilapi/v1")
	sessionID := initializeMCP(t, srv, "arb-1", "C1")

	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "submit_council_vote",
			"arguments": map[string]any{
				"vote":      "demonstrated",
				"rationale": "The record supports the proposition.",
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
		{name: "member_id", want: "C1"},
		{name: "opportunity_id", want: "deliberation:1:C1"},
		{name: "tool", want: "submit_council_vote"},
	} {
		if got := mapString(body[field.name]); got != field.want {
			t.Fatalf("%s = %q, want %q in POST body %#v", field.name, got, field.want, body)
		}
	}
}

func TestMCPWaitForCouncilOpportunityForwardsCursor(t *testing.T) {
	fake := newFakeCouncilAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/councilapi/v1")
	sessionID := initializeMCP(t, srv, "arb-1", "C1")

	got := callMCP(t, srv, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]any{
			"name": waitToolName,
			"arguments": map[string]any{
				"after_opportunity_id": "deliberation:0:C1",
				"after_version":        7,
				"timeout_ms":           200,
			},
		},
	})
	result := rpcResult(t, got)
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("wait_for_council_opportunity returned isError = true: %#v", result)
	}
	content := result["structuredContent"].(map[string]any)
	if state := mapString(content["state"]); state != "ready" {
		t.Fatalf("state = %q, want ready in %#v", state, content)
	}
	query := fake.lastWaitQuery(t)
	for _, field := range []struct {
		name string
		want string
	}{
		{name: "case_id", want: "arb-1"},
		{name: "member_id", want: "C1"},
		{name: "after", want: "deliberation:0:C1"},
		{name: "after_version", want: "7"},
		{name: "timeout_ms", want: "200"},
	} {
		if got := query.Get(field.name); got != field.want {
			t.Fatalf("wait %s query = %q, want %q", field.name, got, field.want)
		}
	}
}

func TestMCPWaitForCouncilOpportunityReturnsWaiting(t *testing.T) {
	fake := newFakeCouncilAPI(t)
	defer fake.server.Close()
	fake.setWaiting()
	srv := newTestMCPServer(fake.server.URL + "/councilapi/v1")
	sessionID := initializeMCP(t, srv, "arb-1", "C1")

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
		t.Fatalf("wait_for_council_opportunity waiting returned isError = true: %#v", result)
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

func TestMCPWaitForCouncilOpportunityReturnsDone(t *testing.T) {
	fake := newFakeCouncilAPI(t)
	defer fake.server.Close()
	fake.setDone()
	srv := newTestMCPServer(fake.server.URL + "/councilapi/v1")
	sessionID := initializeMCP(t, srv, "arb-1", "C1")

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
		t.Fatalf("wait_for_council_opportunity done returned isError = true: %#v", result)
	}
	content := result["structuredContent"].(map[string]any)
	if state := mapString(content["state"]); state != "done" {
		t.Fatalf("state = %q, want done in %#v", state, content)
	}
}

func TestMCPReconnectCreatesNewSessionForSameCaseMember(t *testing.T) {
	fake := newFakeCouncilAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/councilapi/v1")

	first := initializeMCP(t, srv, "arb-1", "C1")
	second := initializeMCP(t, srv, "arb-1", "C1")
	if first == second {
		t.Fatalf("reconnect reused session id %q", first)
	}
	got := callMCP(t, srv, second, map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/list",
	})
	if !hasTool(resultTools(t, got), "submit_council_vote") {
		t.Fatalf("new session did not expose current tools: %#v", got)
	}
}

func TestMCPSessionExpiryDeletesIdleSession(t *testing.T) {
	fake := newFakeCouncilAPI(t)
	defer fake.server.Close()
	srv := newTestMCPServer(fake.server.URL + "/councilapi/v1")
	srv.sessionTTL = time.Minute
	sessionID := initializeMCP(t, srv, "arb-1", "C1")
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

	rejoined := initializeMCP(t, srv, "arb-1", "C1")
	if rejoined == sessionID {
		t.Fatalf("rejoined session reused expired session id %q", sessionID)
	}
}

func TestToolResultTextSummarizesCouncilContent(t *testing.T) {
	text := toolResultText(map[string]any{
		"ok":                   true,
		"case_id":              "arb-1",
		"member_id":            "C1",
		"status":               "ready",
		"state":                "ready",
		"after_version":        json.Number("2"),
		"after_opportunity_id": "deliberation:1:C1",
		"message":              "An opportunity is ready.",
		"prompt":               "Council prompt.",
		"wait": map[string]any{
			"reason":  "ready",
			"version": json.Number("2"),
		},
		"tools": []any{
			map[string]any{"name": "submit_council_vote"},
			map[string]any{"name": "get_case"},
		},
		"turn": map[string]any{
			"phase":              "deliberation",
			"opportunity_id":     "deliberation:1:C1",
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
		"after_version: 2",
		"after_opportunity_id: deliberation:1:C1",
		"wait_reason: ready",
		"wait_version: 2",
		"member_id: C1",
		"phase: deliberation",
		"opportunity_id: deliberation:1:C1",
		"attempts_remaining: 3",
		"tools: get_case, submit_council_vote",
		"prompt:\nCouncil prompt.",
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
		defaultCouncilAPI: strings.TrimRight(base, "/"),
		caseCouncilAPI:    map[string]string{},
		allowedOrigins:    map[string]struct{}{},
		client:            &http.Client{Timeout: 5 * time.Second},
		sessions:          map[string]*mcpSession{},
	}
}

func initializeMCP(t *testing.T, srv *mcpServer, caseID string, memberID string) string {
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
	req := httptest.NewRequest(http.MethodPost, "/mcp?case_id="+caseID+"&member_id="+memberID, bytes.NewReader(raw))
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

type fakeCouncilAPI struct {
	t      *testing.T
	server *httptest.Server

	mu        sync.Mutex
	status    string
	tools     []map[string]any
	version   int
	lastDoRaw []byte
	lastWait  url.Values
}

func newFakeCouncilAPI(t *testing.T) *fakeCouncilAPI {
	f := &fakeCouncilAPI{
		t:       t,
		status:  "ready",
		tools:   []map[string]any{councilTool("get_case"), councilTool("submit_council_vote")},
		version: 2,
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	return f
}

func (f *fakeCouncilAPI) setWaiting() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = "waiting"
	f.tools = nil
	f.version = 3
}

func (f *fakeCouncilAPI) setDone() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = "done"
	f.tools = nil
	f.version = 4
}

func (f *fakeCouncilAPI) lastDoBody(t *testing.T) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	if err := json.Unmarshal(f.lastDoRaw, &body); err != nil {
		t.Fatalf("decode last do body: %v", err)
	}
	return body
}

func (f *fakeCouncilAPI) lastWaitQuery(t *testing.T) url.Values {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastWait
}

func (f *fakeCouncilAPI) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/councilapi/v1/get":
		memberID := r.URL.Query().Get("member_id")
		if r.URL.Query().Get("case_id") != "arb-1" || memberID != "C1" {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": map[string]any{"code": "bad_member"}})
			return
		}
		resp := f.statusResponse(memberID, "get")
		_ = json.NewEncoder(w).Encode(resp)
	case r.Method == http.MethodGet && r.URL.Path == "/councilapi/v1/wait":
		memberID := r.URL.Query().Get("member_id")
		if r.URL.Query().Get("case_id") != "arb-1" || memberID != "C1" {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": map[string]any{"code": "bad_member"}})
			return
		}
		f.lastWait = r.URL.Query()
		resp := f.statusResponse(memberID, "wait")
		_ = json.NewEncoder(w).Encode(resp)
	case r.Method == http.MethodPost && r.URL.Path == "/councilapi/v1/do":
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			f.t.Fatalf("read POST body: %v", err)
		}
		f.lastDoRaw = raw
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"case_id":   "arb-1",
			"member_id": mapString(bodyField(raw, "member_id")),
			"turn":      councilTurnPayload(mapString(bodyField(raw, "member_id"))),
			"result":    map[string]any{"text": "Council vote accepted."},
		})
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeCouncilAPI) statusResponse(memberID string, source string) map[string]any {
	resp := map[string]any{
		"ok":        true,
		"case_id":   "arb-1",
		"member_id": memberID,
		"status":    f.status,
		"prompt":    "Council prompt.",
		"tools":     f.tools,
		"turn":      councilTurnPayload(memberID),
	}
	if source == "wait" {
		reason := "ready"
		if f.status == "waiting" {
			reason = "timeout"
		}
		resp["wait"] = map[string]any{
			"reason":  reason,
			"version": f.version,
		}
	}
	if f.status == "waiting" {
		resp["prompt"] = ""
		resp["tools"] = []map[string]any{}
	}
	if f.status == "done" {
		resp["prompt"] = ""
		resp["tools"] = []map[string]any{}
		resp["turn"] = nil
		if wait, ok := resp["wait"].(map[string]any); ok {
			wait["reason"] = "done"
		}
	}
	return resp
}

func councilTurnPayload(memberID string) map[string]any {
	return map[string]any{
		"role_id":            "council",
		"member_id":          memberID,
		"phase":              "deliberation",
		"opportunity_id":     "deliberation:1:" + memberID,
		"remaining_ms":       120000,
		"attempts_remaining": 3,
	}
}

func bodyField(raw []byte, name string) any {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil
	}
	return body[name]
}

func councilTool(name string) map[string]any {
	return map[string]any{
		"name":        name,
		"description": "tool " + name,
		"input_schema": map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": true,
		},
		"read_only": name != "submit_council_vote",
	}
}
