package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInitializeRejectsMixedPrincipals(t *testing.T) {
	server := testMCPServer()
	req := httptest.NewRequest(http.MethodPost, "/mcp?case_id=case-1&role_id=plaintiff&member_id=C1", bytes.NewReader(initializeRequest(t)))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got rpcResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Error == nil {
		t.Fatalf("expected initialize error")
	}
}

func TestUnifiedToolNamesForLawyerAndCouncil(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		want  string
	}{
		{name: "lawyer", query: "case_id=case-1&role_id=plaintiff", want: "submit_decision"},
		{name: "council", query: "case_id=case-1&member_id=C1", want: "submit_council_vote"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := testMCPServer()
			sessionID := initializeSession(t, server, tc.query)
			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(rpcRequest(t, "tools/list", map[string]any{})))
			req.Header.Set("Mcp-Session-Id", sessionID)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			var got struct {
				Result map[string][]map[string]any `json:"result"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if !hasTool(got.Result["tools"], "wait_for_opportunity") {
				t.Fatalf("missing wait_for_opportunity: %#v", got.Result["tools"])
			}
			if !hasTool(got.Result["tools"], "get_current_opportunity") {
				t.Fatalf("missing get_current_opportunity: %#v", got.Result["tools"])
			}
			if !hasTool(got.Result["tools"], tc.want) {
				t.Fatalf("missing %s: %#v", tc.want, got.Result["tools"])
			}
		})
	}
}

func testMCPServer() *mcpServer {
	return &mcpServer{
		lawyerAPIBase:  "http://127.0.0.1:1/lawyerapi/v1",
		councilAPIBase: "http://127.0.0.1:1/councilapi/v1",
		sessions:       map[string]*mcpSession{},
	}
}

func initializeSession(t *testing.T, server *mcpServer, query string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp?"+query, bytes.NewReader(initializeRequest(t)))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	sessionID := rec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("missing session id")
	}
	return sessionID
}

func initializeRequest(t *testing.T) []byte {
	t.Helper()
	return rpcRequest(t, "initialize", map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0"},
	})
}

func rpcRequest(t *testing.T, method string, params map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return raw
}

func hasTool(tools []map[string]any, name string) bool {
	for _, tool := range tools {
		if tool["name"] == name {
			return true
		}
	}
	return false
}
