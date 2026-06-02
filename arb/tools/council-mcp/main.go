package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	mcpProtocolVersion = "2025-06-18"
	serverName         = "aar-council-mcp"
	serverVersion      = "0.1.0"

	waitToolName       = "wait_for_council_opportunity"
	waitToolDefault    = 30 * time.Second
	waitToolMax        = 30 * time.Second
	waitToolHTTPMargin = 2 * time.Second

	defaultSessionTTL             = 30 * time.Minute
	defaultSessionCleanupInterval = time.Minute
)

type caseMapFlag map[string]string

func (m caseMapFlag) String() string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+m[key])
	}
	return strings.Join(parts, ",")
}

func (m caseMapFlag) Set(value string) error {
	left, right, ok := strings.Cut(strings.TrimSpace(value), "=")
	if !ok {
		return fmt.Errorf("case mapping must be case_id=councilapi_base")
	}
	caseID := strings.TrimSpace(left)
	baseURL := strings.TrimRight(strings.TrimSpace(right), "/")
	if caseID == "" {
		return fmt.Errorf("case mapping has empty case_id")
	}
	if baseURL == "" {
		return fmt.Errorf("case mapping for %s has empty councilapi_base", caseID)
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return fmt.Errorf("case mapping for %s has invalid councilapi_base: %w", caseID, err)
	}
	m[caseID] = baseURL
	return nil
}

type originList map[string]struct{}

func (l originList) String() string {
	if len(l) == 0 {
		return ""
	}
	values := make([]string, 0, len(l))
	for value := range l {
		values = append(values, value)
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

func (l originList) Set(value string) error {
	origin := strings.TrimSpace(value)
	if origin == "" {
		return fmt.Errorf("origin must not be empty")
	}
	l[origin] = struct{}{}
	return nil
}

type mcpServer struct {
	defaultCouncilAPI string
	caseCouncilAPI    map[string]string
	bearerToken       string
	allowedOrigins    map[string]struct{}
	client            *http.Client
	log               io.Writer
	sessionTTL        time.Duration

	mu       sync.Mutex
	sessions map[string]*mcpSession
}

type mcpSession struct {
	ID             string
	CaseID         string
	MemberID       string
	CouncilAPIBase string
	CreatedAt      time.Time
	LastSeen       time.Time
}

type rpcMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("aar-council-mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var caseMap = caseMapFlag{}
	var origins = originList{}
	listen := fs.String("listen", "127.0.0.1:19780", "listen address")
	defaultCouncilAPI := fs.String("councilapi-base", "", "default Council API base URL, for example http://127.0.0.1:19771/councilapi/v1")
	bearerToken := fs.String("bearer-token", "", "optional bearer token required for MCP requests")
	sessionTTL := fs.Duration("session-ttl", defaultSessionTTL, "idle MCP session TTL; 0 disables expiry")
	sessionCleanupInterval := fs.Duration("session-cleanup-interval", defaultSessionCleanupInterval, "interval for deleting expired MCP sessions")
	fs.Var(caseMap, "case", "case mapping case_id=councilapi_base; repeat for multiple cases")
	fs.Var(origins, "allow-origin", "allowed HTTP Origin; repeat when browser clients need non-localhost origins")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*defaultCouncilAPI) == "" && len(caseMap) == 0 {
		fmt.Fprintln(stderr, "councilapi-base or at least one -case mapping is required")
		fs.Usage()
		return 2
	}
	defaultBase := strings.TrimRight(strings.TrimSpace(*defaultCouncilAPI), "/")
	if defaultBase != "" {
		if _, err := url.ParseRequestURI(defaultBase); err != nil {
			fmt.Fprintf(stderr, "invalid councilapi-base: %v\n", err)
			return 2
		}
	}
	if *sessionTTL < 0 {
		fmt.Fprintln(stderr, "session-ttl must be non-negative")
		return 2
	}
	if *sessionTTL > 0 && *sessionCleanupInterval <= 0 {
		fmt.Fprintln(stderr, "session-cleanup-interval must be positive when session expiry is enabled")
		return 2
	}
	handler := &mcpServer{
		defaultCouncilAPI: defaultBase,
		caseCouncilAPI:    caseMap,
		bearerToken:       strings.TrimSpace(*bearerToken),
		allowedOrigins:    origins,
		client:            &http.Client{Timeout: waitToolMax + waitToolHTTPMargin},
		log:               stderr,
		sessionTTL:        *sessionTTL,
		sessions:          map[string]*mcpSession{},
	}
	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	defer cancelCleanup()
	if *sessionTTL > 0 {
		go handler.expireSessionsLoop(cleanupCtx, *sessionCleanupInterval)
	}
	srv := &http.Server{
		Addr:              strings.TrimSpace(*listen),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Fprintf(stderr, "aar-council-mcp listening on http://%s/mcp\n", listenerDisplayAddr(srv.Addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "mcp server failed: %v\n", err)
		return 1
	}
	return 0
}

func listenerDisplayAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func (s *mcpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.Path != "/mcp" {
		http.NotFound(w, r)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.originAllowed(r.Header.Get("Origin")) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodPost:
		s.handlePost(w, r)
	case http.MethodGet:
		w.Header().Set("Allow", "POST, GET, DELETE")
		http.Error(w, "server-sent event stream is not supported", http.StatusMethodNotAllowed)
	case http.MethodDelete:
		s.handleDelete(w, r)
	default:
		w.Header().Set("Allow", "POST, GET, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *mcpServer) authorized(r *http.Request) bool {
	if s.bearerToken == "" {
		return true
	}
	return strings.TrimSpace(r.Header.Get("Authorization")) == "Bearer "+s.bearerToken
}

func (s *mcpServer) originAllowed(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return true
	}
	if _, ok := s.allowedOrigins[origin]; ok {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (s *mcpServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.Header.Get("Mcp-Session-Id"))
	if sessionID == "" {
		http.Error(w, "Mcp-Session-Id is required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	_, existed := s.sessions[sessionID]
	if existed {
		delete(s.sessions, sessionID)
	}
	s.mu.Unlock()
	if existed {
		s.logf("mcp_session_deleted session_id=%s reason=delete", sessionID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *mcpServer) handlePost(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, 4*1024*1024)
	var msg rpcMessage
	dec := json.NewDecoder(body)
	dec.UseNumber()
	if err := dec.Decode(&msg); err != nil {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
		return
	}
	if msg.JSONRPC != "2.0" || msg.Method == "" {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32600, Message: "invalid JSON-RPC request"}})
		return
	}
	if msg.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	switch msg.Method {
	case "initialize":
		s.handleInitialize(w, r, msg)
	case "ping":
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: msg.ID, Result: map[string]any{}})
	case "tools/list":
		session, ok := s.requireSession(w, r, msg)
		if !ok {
			return
		}
		result, err := s.listTools(r.Context(), session)
		if err != nil {
			writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32603, Message: err.Error()}})
			return
		}
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: msg.ID, Result: result})
	case "tools/call":
		session, ok := s.requireSession(w, r, msg)
		if !ok {
			return
		}
		result, err := s.callTool(r.Context(), session, msg.Params)
		if err != nil {
			writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
			return
		}
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: msg.ID, Result: result})
	default:
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32601, Message: "method not found"}})
	}
}

func (s *mcpServer) handleInitialize(w http.ResponseWriter, r *http.Request, msg rpcMessage) {
	session, err := s.newSession(r)
	if err != nil {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
		return
	}
	w.Header().Set("Mcp-Session-Id", session.ID)
	writeRPC(w, rpcResponse{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]any{
			"protocolVersion": mcpProtocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{
					"listChanged": false,
				},
			},
			"serverInfo": map[string]any{
				"name":    serverName,
				"version": serverVersion,
			},
			"instructions": sessionInstructions(session),
		},
	})
}

func sessionInstructions(session *mcpSession) string {
	return fmt.Sprintf(
		"This MCP session is bound to case_id %s and member_id %s. Call wait_for_council_opportunity first. If it returns state waiting, call wait_for_council_opportunity again with the returned after_version. If it returns state ready, use the returned prompt, turn, limits, and available tools to complete exactly that opportunity. After a successful submit_council_vote, call wait_for_council_opportunity again. If it returns state done, stop. If it returns state error, report the error and stop. Tools are forwarded to the AAR Council API.",
		session.CaseID,
		session.MemberID,
	)
}

func (s *mcpServer) newSession(r *http.Request) (*mcpSession, error) {
	query := r.URL.Query()
	caseID := strings.TrimSpace(query.Get("case_id"))
	memberID := strings.TrimSpace(query.Get("member_id"))
	if caseID == "" {
		return nil, fmt.Errorf("case_id query parameter is required")
	}
	if memberID == "" {
		return nil, fmt.Errorf("member_id query parameter is required")
	}
	if !validMemberID(memberID) {
		return nil, fmt.Errorf("member_id must be a non-empty identifier without whitespace")
	}
	base, err := s.councilAPIBase(caseID)
	if err != nil {
		return nil, err
	}
	sessionID, err := randomSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	session := &mcpSession{
		ID:             sessionID,
		CaseID:         caseID,
		MemberID:       memberID,
		CouncilAPIBase: base,
		CreatedAt:      now,
		LastSeen:       now,
	}
	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()
	s.logf("mcp_session_created session_id=%s case_id=%s member_id=%s councilapi_base=%s", session.ID, session.CaseID, session.MemberID, session.CouncilAPIBase)
	return session, nil
}

func validMemberID(memberID string) bool {
	return memberID != "" && !strings.ContainsAny(memberID, " \t\r\n")
}

func (s *mcpServer) requireSession(w http.ResponseWriter, r *http.Request, msg rpcMessage) (*mcpSession, bool) {
	sessionID := strings.TrimSpace(r.Header.Get("Mcp-Session-Id"))
	if sessionID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32600, Message: "Mcp-Session-Id is required after initialize"}})
		return nil, false
	}
	now := time.Now()
	s.mu.Lock()
	session := s.sessions[sessionID]
	expired := false
	if session != nil {
		if s.sessionTTL > 0 && !session.LastSeen.Add(s.sessionTTL).After(now) {
			delete(s.sessions, sessionID)
			session = nil
			expired = true
		} else {
			session.LastSeen = now
		}
	}
	s.mu.Unlock()
	if session == nil {
		if expired {
			s.logf("mcp_session_deleted session_id=%s reason=expired", sessionID)
			http.Error(w, "expired MCP session", http.StatusNotFound)
		} else {
			http.Error(w, "unknown MCP session", http.StatusNotFound)
		}
		return nil, false
	}
	return session, true
}

func (s *mcpServer) expireSessionsLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.expireIdleSessions(now)
		}
	}
}

func (s *mcpServer) expireIdleSessions(now time.Time) int {
	if s.sessionTTL <= 0 {
		return 0
	}
	expired := []string{}
	s.mu.Lock()
	for id, session := range s.sessions {
		if !session.LastSeen.Add(s.sessionTTL).After(now) {
			delete(s.sessions, id)
			expired = append(expired, id)
		}
	}
	s.mu.Unlock()
	sort.Strings(expired)
	for _, id := range expired {
		s.logf("mcp_session_deleted session_id=%s reason=expired", id)
	}
	return len(expired)
}

func (s *mcpServer) councilAPIBase(caseID string) (string, error) {
	if base := strings.TrimSpace(s.caseCouncilAPI[caseID]); base != "" {
		return strings.TrimRight(base, "/"), nil
	}
	if s.defaultCouncilAPI != "" {
		return s.defaultCouncilAPI, nil
	}
	return "", fmt.Errorf("no Council API base configured for case_id %q", caseID)
}

func randomSessionID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

func (s *mcpServer) listTools(ctx context.Context, session *mcpSession) (map[string]any, error) {
	status, err := s.getCurrent(ctx, session)
	if err != nil {
		return nil, err
	}
	tools := []map[string]any{currentOpportunityToolSpec(), waitForOpportunityToolSpec()}
	for _, tool := range councilToolsFromStatus(status) {
		tools = append(tools, mcpToolSpecFromCouncil(tool))
	}
	return map[string]any{"tools": tools}, nil
}

func currentOpportunityToolSpec() map[string]any {
	return map[string]any{
		"name":        "get_current_council_opportunity",
		"description": "Return the current Council API status, prompt, turn, tools, limits, remaining time, and attempts for this bound case-member.",
		"inputSchema": map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		"annotations": map[string]any{"readOnlyHint": true},
	}
}

func waitForOpportunityToolSpec() map[string]any {
	return map[string]any{
		"name":        waitToolName,
		"description": "Wait up to 30 seconds for this bound case-member to have a ready opportunity or a case-status change. If the result state is waiting, call this tool again with the returned after_version.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"after_opportunity_id": map[string]any{
					"type":        "string",
					"description": "Opportunity id already seen by the council member. A different live opportunity returns ready.",
				},
				"after_version": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"description": "Wait version returned by an earlier wait_for_council_opportunity call.",
				},
				"timeout_ms": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     waitToolMax.Milliseconds(),
					"description": "Requested wait duration. The adapter caps the value at 30000.",
				},
			},
			"additionalProperties": false,
		},
		"annotations": map[string]any{"readOnlyHint": true},
	}
}

func mcpToolSpecFromCouncil(tool map[string]any) map[string]any {
	spec := map[string]any{
		"name":        mapString(tool["name"]),
		"description": mapString(tool["description"]),
	}
	if schema, ok := tool["input_schema"].(map[string]any); ok {
		spec["inputSchema"] = schema
	} else {
		spec["inputSchema"] = map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
	}
	if readOnly, _ := tool["read_only"].(bool); readOnly {
		spec["annotations"] = map[string]any{"readOnlyHint": true}
	}
	return spec
}

func councilToolsFromStatus(status map[string]any) []map[string]any {
	raw, ok := status["tools"].([]any)
	if !ok {
		return nil
	}
	tools := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if tool, ok := item.(map[string]any); ok && mapString(tool["name"]) != "" {
			tools = append(tools, tool)
		}
	}
	return tools
}

func (s *mcpServer) callTool(ctx context.Context, session *mcpSession, params json.RawMessage) (map[string]any, error) {
	var call toolCallParams
	if len(params) > 0 {
		dec := json.NewDecoder(bytes.NewReader(params))
		dec.UseNumber()
		if err := dec.Decode(&call); err != nil {
			return nil, fmt.Errorf("decode tool call params: %w", err)
		}
	}
	call.Name = strings.TrimSpace(call.Name)
	if call.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}
	if call.Name == "get_current_council_opportunity" {
		status, err := s.getCurrent(ctx, session)
		if err != nil {
			return toolResult(map[string]any{"ok": false, "error": err.Error()}, true), nil
		}
		return toolResult(status, false), nil
	}
	if call.Name == waitToolName {
		result, err := s.waitForOpportunity(ctx, session, call.Arguments)
		if err != nil {
			return toolResult(waitErrorResult(session, err), true), nil
		}
		return toolResult(result, mapString(result["state"]) == "error"), nil
	}
	status, err := s.getCurrent(ctx, session)
	if err != nil {
		return toolResult(map[string]any{"ok": false, "error": err.Error()}, true), nil
	}
	if !toolAvailable(status, call.Name) {
		return toolResult(map[string]any{
			"ok":        false,
			"case_id":   session.CaseID,
			"member_id": session.MemberID,
			"turn":      status["turn"],
			"error": map[string]any{
				"code":    "tool_unavailable",
				"message": fmt.Sprintf("tool %q is not available for this member and turn", call.Name),
			},
		}, true), nil
	}
	result, err := s.postTool(ctx, session, status, call.Name, call.Arguments)
	if err != nil {
		return toolResult(map[string]any{"ok": false, "error": err.Error()}, true), nil
	}
	ok, _ := result["ok"].(bool)
	return toolResult(result, !ok), nil
}

func toolAvailable(status map[string]any, name string) bool {
	for _, tool := range councilToolsFromStatus(status) {
		if mapString(tool["name"]) == name {
			return true
		}
	}
	return false
}

func (s *mcpServer) getCurrent(ctx context.Context, session *mcpSession) (map[string]any, error) {
	u, err := url.Parse(session.CouncilAPIBase + "/get")
	if err != nil {
		return nil, fmt.Errorf("build Council API GET URL: %w", err)
	}
	query := u.Query()
	query.Set("case_id", session.CaseID)
	query.Set("member_id", session.MemberID)
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Council API GET: %w", err)
	}
	defer resp.Body.Close()
	value, err := decodeJSONObject(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decode Council API GET response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Council API GET returned HTTP %d: %s", resp.StatusCode, compactJSON(value))
	}
	return value, nil
}

func (s *mcpServer) waitForOpportunity(ctx context.Context, session *mcpSession, args map[string]any) (map[string]any, error) {
	timeout, err := waitToolTimeout(args["timeout_ms"])
	if err != nil {
		return nil, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout+waitToolHTTPMargin)
	defer cancel()
	u, err := url.Parse(session.CouncilAPIBase + "/wait")
	if err != nil {
		return nil, fmt.Errorf("build Council API wait URL: %w", err)
	}
	query := u.Query()
	query.Set("case_id", session.CaseID)
	query.Set("member_id", session.MemberID)
	query.Set("timeout_ms", fmt.Sprintf("%d", timeout.Milliseconds()))
	if after := mapString(args["after_opportunity_id"]); after != "" {
		query.Set("after", after)
	}
	if afterVersion := mapNumberString(args["after_version"]); afterVersion != "" {
		query.Set("after_version", afterVersion)
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(waitCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Council API wait: %w", err)
	}
	defer resp.Body.Close()
	value, err := decodeJSONObject(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decode Council API wait response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		value["ok"] = false
		value["state"] = "error"
		value["http_status"] = resp.StatusCode
		value["message"] = fmt.Sprintf("Council API wait returned HTTP %d.", resp.StatusCode)
		return value, nil
	}
	state := waitToolState(value)
	value["state"] = state
	if version := waitVersion(value); version != nil {
		value["after_version"] = version
	}
	if opportunityID := currentOpportunityID(value); opportunityID != "" {
		value["after_opportunity_id"] = opportunityID
	}
	value["message"] = waitToolMessage(state)
	s.logf(
		"councilapi_wait case_id=%s member_id=%s state=%s wait_reason=%s opportunity_id=%s",
		session.CaseID,
		session.MemberID,
		state,
		waitReason(value),
		currentOpportunityID(value),
	)
	return value, nil
}

func waitToolTimeout(value any) (time.Duration, error) {
	if value == nil {
		return waitToolDefault, nil
	}
	text := mapNumberString(value)
	if text == "" {
		return 0, fmt.Errorf("timeout_ms must be an integer")
	}
	ms, err := parsePositiveInt64(text)
	if err != nil {
		return 0, fmt.Errorf("timeout_ms must be a positive integer")
	}
	timeout := time.Duration(ms) * time.Millisecond
	if timeout > waitToolMax {
		return waitToolMax, nil
	}
	return timeout, nil
}

func parsePositiveInt64(value string) (int64, error) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("not positive")
	}
	return n, nil
}

func waitToolState(value map[string]any) string {
	if ok, hasOK := value["ok"].(bool); hasOK && !ok {
		return "error"
	}
	switch strings.ToLower(mapString(value["status"])) {
	case "ready":
		return "ready"
	case "done", "terminal", "complete", "completed":
		return "done"
	}
	return "waiting"
}

func waitVersion(value map[string]any) any {
	wait, _ := value["wait"].(map[string]any)
	return wait["version"]
}

func waitReason(value map[string]any) string {
	wait, _ := value["wait"].(map[string]any)
	return mapString(wait["reason"])
}

func waitToolMessage(state string) string {
	switch state {
	case "ready":
		return "An opportunity is ready for this member. Use the returned prompt, turn, limits, and tools to act."
	case "done":
		return "The case is done. Stop acting on this assignment."
	case "error":
		return "The council member cannot continue without operator attention."
	default:
		return "No opportunity is ready for this member. Call wait_for_council_opportunity again with after_version."
	}
}

func waitErrorResult(session *mcpSession, err error) map[string]any {
	return map[string]any{
		"ok":        false,
		"state":     "error",
		"case_id":   session.CaseID,
		"member_id": session.MemberID,
		"error": map[string]any{
			"code":    "wait_failed",
			"message": err.Error(),
		},
		"message": waitToolMessage("error"),
	}
}

func (s *mcpServer) postTool(ctx context.Context, session *mcpSession, status map[string]any, tool string, arguments map[string]any) (map[string]any, error) {
	body := map[string]any{
		"case_id":   session.CaseID,
		"member_id": session.MemberID,
		"tool":      tool,
		"arguments": arguments,
	}
	opportunityID := currentOpportunityID(status)
	if opportunityID == "" {
		return nil, fmt.Errorf("current turn has no opportunity_id")
	}
	body["opportunity_id"] = opportunityID
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, session.CouncilAPIBase+"/do", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Council API POST: %w", err)
	}
	defer resp.Body.Close()
	value, err := decodeJSONObject(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decode Council API POST response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		value["ok"] = false
		value["http_status"] = resp.StatusCode
	}
	s.logf(
		"councilapi_do case_id=%s member_id=%s opportunity_id=%s tool=%s http_status=%d ok=%v",
		session.CaseID,
		session.MemberID,
		mapString(body["opportunity_id"]),
		tool,
		resp.StatusCode,
		value["ok"],
	)
	return value, nil
}

func (s *mcpServer) logf(format string, args ...any) {
	if s.log == nil {
		return
	}
	fmt.Fprintf(s.log, format+"\n", args...)
}

func currentOpportunityID(status map[string]any) string {
	turn, _ := status["turn"].(map[string]any)
	return mapString(turn["opportunity_id"])
}

func toolResult(value map[string]any, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": toolResultText(value),
		}},
		"structuredContent": value,
		"isError":           isError,
	}
}

func toolResultText(value map[string]any) string {
	var b strings.Builder
	if ok, hasOK := value["ok"].(bool); hasOK {
		if ok {
			b.WriteString("ok: true\n")
		} else {
			b.WriteString("ok: false\n")
		}
	}
	if status := mapString(value["status"]); status != "" {
		b.WriteString("status: ")
		b.WriteString(status)
		b.WriteByte('\n')
	}
	if state := mapString(value["state"]); state != "" {
		b.WriteString("state: ")
		b.WriteString(state)
		b.WriteByte('\n')
	}
	if msg := mapString(value["message"]); msg != "" {
		b.WriteString("message: ")
		b.WriteString(msg)
		b.WriteByte('\n')
	}
	if afterVersion := mapNumberString(value["after_version"]); afterVersion != "" {
		b.WriteString("after_version: ")
		b.WriteString(afterVersion)
		b.WriteByte('\n')
	}
	if afterOpportunity := mapString(value["after_opportunity_id"]); afterOpportunity != "" {
		b.WriteString("after_opportunity_id: ")
		b.WriteString(afterOpportunity)
		b.WriteByte('\n')
	}
	if wait, ok := value["wait"].(map[string]any); ok {
		if reason := mapString(wait["reason"]); reason != "" {
			b.WriteString("wait_reason: ")
			b.WriteString(reason)
			b.WriteByte('\n')
		}
		if version := mapNumberString(wait["version"]); version != "" {
			b.WriteString("wait_version: ")
			b.WriteString(version)
			b.WriteByte('\n')
		}
	}
	if role := mapString(value["member_id"]); role != "" {
		b.WriteString("member_id: ")
		b.WriteString(role)
		b.WriteByte('\n')
	}
	if turn, ok := value["turn"].(map[string]any); ok {
		if phase := mapString(turn["phase"]); phase != "" {
			b.WriteString("phase: ")
			b.WriteString(phase)
			b.WriteByte('\n')
		}
		if opportunity := mapString(turn["opportunity_id"]); opportunity != "" {
			b.WriteString("opportunity_id: ")
			b.WriteString(opportunity)
			b.WriteByte('\n')
		}
		if remaining := mapNumberString(turn["remaining_ms"]); remaining != "" {
			b.WriteString("remaining_ms: ")
			b.WriteString(remaining)
			b.WriteByte('\n')
		}
		if attempts := mapNumberString(turn["attempts_remaining"]); attempts != "" {
			b.WriteString("attempts_remaining: ")
			b.WriteString(attempts)
			b.WriteByte('\n')
		}
	}
	if tools := toolNamesFromValue(value); len(tools) > 0 {
		b.WriteString("tools: ")
		b.WriteString(strings.Join(tools, ", "))
		b.WriteByte('\n')
	}
	if errObj, ok := value["error"].(map[string]any); ok {
		if code := mapString(errObj["code"]); code != "" {
			b.WriteString("error_code: ")
			b.WriteString(code)
			b.WriteByte('\n')
		}
		if msg := mapString(errObj["message"]); msg != "" {
			b.WriteString("error_message: ")
			b.WriteString(msg)
			b.WriteByte('\n')
		}
	} else if errText := mapString(value["error"]); errText != "" {
		b.WriteString("error: ")
		b.WriteString(errText)
		b.WriteByte('\n')
	}
	if prompt := mapString(value["prompt"]); prompt != "" {
		b.WriteString("\nprompt:\n")
		b.WriteString(prompt)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func toolNamesFromValue(value map[string]any) []string {
	tools := councilToolsFromStatus(value)
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if name := mapString(tool["name"]); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func writeRPC(w http.ResponseWriter, response rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	if response.JSONRPC == "" {
		response.JSONRPC = "2.0"
	}
	_ = json.NewEncoder(w).Encode(response)
}

func decodeJSONObject(r io.Reader) (map[string]any, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var value map[string]any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	if value == nil {
		value = map[string]any{}
	}
	return value, nil
}

func compactJSON(value map[string]any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(raw)
}

func mapString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}

func mapNumberString(value any) string {
	switch v := value.(type) {
	case json.Number:
		return v.String()
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	default:
		return ""
	}
}
