package openclawattorney

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseDecisionJSONExtractsObject(t *testing.T) {
	decision, err := parseDecisionJSON([]byte("notes\n```json\n{\"kind\":\"tool\",\"tool_name\":\"record_opening_statement\",\"payload\":{\"text\":\"ok\"}}\n```"))
	if err != nil {
		t.Fatalf("parseDecisionJSON returned error: %v", err)
	}
	if decision["kind"] != "tool" {
		t.Fatalf("kind = %v, want tool", decision["kind"])
	}
}

func TestExtractOpenClawText(t *testing.T) {
	raw := []byte(`{"result":{"payloads":[{"text":"{\"kind\":\"pass\"}"}]}}`)
	if got := extractOpenClawText(raw); got != `{"kind":"pass"}` {
		t.Fatalf("extractOpenClawText() = %q", got)
	}
}

func TestObtainDecisionFromCommand(t *testing.T) {
	s := &Server{cfg: Config{Command: `cat >/dev/null; printf '{"kind":"pass"}'`, CommandTimeout: time.Second}}
	decision, err := s.obtainDecision(context.Background(), lawyerJob{SessionID: "s1", Prompt: "prompt"})
	if err != nil {
		t.Fatalf("obtainDecision returned error: %v", err)
	}
	if decision["kind"] != "pass" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestOpenClawAgentRequiresExplicitAgentID(t *testing.T) {
	s := &Server{cfg: Config{UseOpenClawAgent: true, CommandTimeout: time.Second}}
	_, err := s.obtainDecision(context.Background(), lawyerJob{SessionID: "s1", Prompt: "prompt"})
	if err == nil || !strings.Contains(err.Error(), "AAR_OPENCLAW_AGENT_ID") {
		t.Fatalf("error = %v, want explicit agent id error", err)
	}
}

func TestServerFilesDecisionThroughAARClientMethod(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fixed := `{"kind":"tool","tool_name":"record_opening_statement","payload":{"text":"Opening statement."}}`
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, serverConn, nil, Config{FixedDecision: fixed, CommandTimeout: time.Second, IncludeCaseView: true, IncludeTextFiles: true})
	}()

	reader := bufio.NewReader(clientConn)
	writeRequest := func(id int64, method string, params map[string]any) {
		t.Helper()
		wire, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		if _, err := clientConn.Write(append(wire, '\n')); err != nil {
			t.Fatalf("write request: %v", err)
		}
	}
	readEnvelope := func() map[string]any {
		t.Helper()
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatalf("read envelope: %v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(line, &out); err != nil {
			t.Fatalf("decode envelope %s: %v", string(line), err)
		}
		return out
	}
	writeResponse := func(id any, result map[string]any) {
		t.Helper()
		wire, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		if _, err := clientConn.Write(append(wire, '\n')); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}

	writeRequest(1, "initialize", map[string]any{"protocolVersion": 1})
	if got := readEnvelope(); got["id"].(float64) != 1 {
		t.Fatalf("initialize response id = %#v", got)
	}

	writeRequest(2, "session/new", map[string]any{"cwd": "/tmp", "mcpServers": []any{}})
	if got := readEnvelope(); got["id"].(float64) != 2 {
		t.Fatalf("new session response id = %#v", got)
	}

	writeRequest(3, "session/prompt", map[string]any{
		"sessionId": "s1",
		"prompt":    []any{map[string]any{"type": "text", "text": "File an opening."}},
	})

	content := []byte("Evidence text.")
	var submitted map[string]any
	var readCalls int
	promptDone := false
	for submitted == nil || !promptDone {
		env := readEnvelope()
		if method, _ := env["method"].(string); method != "" {
			switch method {
			case "_aar/get_case":
				writeResponse(env["id"], map[string]any{"case": map[string]any{"proposition": "P"}})
			case "_aar/list_artifacts":
				writeResponse(env["id"], map[string]any{"artifacts": []any{map[string]any{"artifact_id": "f1", "title": "evidence.txt", "text_readable": true, "size_bytes": len(content)}}})
			case "_aar/stat_artifact":
				writeResponse(env["id"], map[string]any{"artifact": map[string]any{"artifact_id": "f1"}, "limits": map[string]any{"max_read_bytes": 5, "remaining_read_bytes_for_opportunity": 20, "remaining_reads_for_opportunity": 4}})
			case "_aar/read_artifact_range":
				params, _ := env["params"].(map[string]any)
				offset := intValue(params["offset"])
				length := intValue(params["length"])
				if params["artifact_id"] != "f1" || length > 5 || offset < 0 || offset >= len(content) {
					t.Fatalf("read artifact params = %#v", params)
				}
				end := offset + length
				if end > len(content) {
					end = len(content)
				}
				readCalls++
				writeResponse(env["id"], map[string]any{"artifact_id": "f1", "offset": offset, "length": end - offset, "content_base64": base64.StdEncoding.EncodeToString(content[offset:end]), "remaining_read_bytes_for_opportunity": 20 - end})
			case "_aar/submit_decision":
				params, _ := env["params"].(map[string]any)
				submitted = params
				writeResponse(env["id"], map[string]any{"text": "Decision accepted."})
			default:
				t.Fatalf("unexpected client method %q", method)
			}
			continue
		}
		if env["id"].(float64) == 3 {
			if result, _ := env["result"].(map[string]any); result["stopReason"] != "end_turn" {
				t.Fatalf("prompt result = %#v", result)
			}
			promptDone = true
		}
	}
	if submitted["kind"] != "tool" || submitted["tool_name"] != "record_opening_statement" {
		t.Fatalf("submitted decision = %#v", submitted)
	}
	if readCalls != 3 {
		t.Fatalf("read calls = %d, want 3", readCalls)
	}

	_ = clientConn.Close()
	select {
	case err := <-errCh:
		if err != nil && !strings.Contains(err.Error(), "closed") {
			t.Fatalf("server returned error: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("server did not exit")
	}
}

func TestServerRetriesRejectedDecision(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "lawyer.sh")
	counter := filepath.Join(dir, "counter")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ncat >/dev/null\nn=$(cat '"+counter+"' 2>/dev/null || echo 0)\nn=$((n+1))\necho $n > '"+counter+"'\nif [ \"$n\" = 1 ]; then\n  printf '{\"kind\":\"tool\",\"tool_name\":\"record_opening_statement\",\"payload\":{\"text\":\"too long\"}}'\nelse\n  printf '{\"kind\":\"pass\"}'\nfi\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, serverConn, nil, Config{Command: script, CommandTimeout: time.Second, IncludeCaseView: true, IncludeTextFiles: false})
	}()

	reader := bufio.NewReader(clientConn)
	writeRequest := func(id int64, method string, params map[string]any) {
		t.Helper()
		wire, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		if _, err := clientConn.Write(append(wire, '\n')); err != nil {
			t.Fatalf("write request: %v", err)
		}
	}
	readEnvelope := func() map[string]any {
		t.Helper()
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatalf("read envelope: %v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(line, &out); err != nil {
			t.Fatalf("decode envelope %s: %v", string(line), err)
		}
		return out
	}
	writeResponse := func(id any, result map[string]any) {
		t.Helper()
		wire, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		if _, err := clientConn.Write(append(wire, '\n')); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}
	writeError := func(id any, message string) {
		t.Helper()
		wire, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32000, "message": message}})
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		if _, err := clientConn.Write(append(wire, '\n')); err != nil {
			t.Fatalf("write error: %v", err)
		}
	}

	writeRequest(1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readEnvelope()
	writeRequest(2, "session/new", map[string]any{"cwd": "/tmp", "mcpServers": []any{}})
	_ = readEnvelope()
	writeRequest(3, "session/prompt", map[string]any{"sessionId": "s1", "prompt": []any{map[string]any{"type": "text", "text": "File."}}})

	submitCount := 0
	promptDone := false
	for !promptDone {
		env := readEnvelope()
		if method, _ := env["method"].(string); method != "" {
			switch method {
			case "_aar/get_case":
				writeResponse(env["id"], map[string]any{"case": map[string]any{"proposition": "P"}})
			case "_aar/submit_decision":
				submitCount++
				if submitCount == 1 {
					writeError(env["id"], "Argument exceeds the character limit: 10 characters submitted, 5 allowed")
				} else {
					writeResponse(env["id"], map[string]any{"text": "Decision accepted."})
				}
			default:
				t.Fatalf("unexpected method %q", method)
			}
			continue
		}
		if env["id"].(float64) == 3 {
			if _, ok := env["error"]; ok {
				t.Fatalf("prompt failed: %#v", env)
			}
			promptDone = true
		}
	}
	if submitCount != 2 {
		t.Fatalf("submitCount = %d, want 2", submitCount)
	}
	_ = clientConn.Close()
	select {
	case err := <-errCh:
		if err != nil && !strings.Contains(err.Error(), "closed") {
			t.Fatalf("server returned error: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("server did not exit")
	}
}

func TestParseDecisionJSONAcceptsStructuredEvidenceBundle(t *testing.T) {
	response, err := parseDecisionJSON([]byte(`{"evidence_submissions":[{"title":"Source","mime_type":"text/plain","relevance":"R","content":"body"}],"decision":{"kind":"tool","tool_name":"submit_argument","payload":{"text":"Argument."}}}`))
	if err != nil {
		t.Fatalf("parseDecisionJSON returned error: %v", err)
	}
	if decision, _ := response["decision"].(map[string]any); decision == nil || decision["kind"] != "tool" {
		t.Fatalf("structured decision = %#v", response["decision"])
	}
}

func TestServerSubmitsEvidenceBundleBeforeDecision(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fixed := `{"evidence_submissions":[{"title":"Primary source","source_url":"https://example.test/source","mime_type":"text/plain","relevance":"Shows the key fact.","content":"exact source text","offer_label":"PX-new"}],"decision":{"kind":"tool","tool_name":"submit_argument","payload":{"text":"Argument relying on PX-new.","technical_reports":[]}}}`
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, serverConn, nil, Config{FixedDecision: fixed, CommandTimeout: time.Second, IncludeCaseView: true, IncludeTextFiles: false})
	}()

	reader := bufio.NewReader(clientConn)
	writeRequest := func(id int64, method string, params map[string]any) {
		t.Helper()
		wire, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		if _, err := clientConn.Write(append(wire, '\n')); err != nil {
			t.Fatalf("write request: %v", err)
		}
	}
	readEnvelope := func() map[string]any {
		t.Helper()
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatalf("read envelope: %v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(line, &out); err != nil {
			t.Fatalf("decode envelope %s: %v", string(line), err)
		}
		return out
	}
	writeResponse := func(id any, result map[string]any) {
		t.Helper()
		wire, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		if _, err := clientConn.Write(append(wire, '\n')); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}

	writeRequest(1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readEnvelope()
	writeRequest(2, "session/new", map[string]any{"cwd": "/tmp", "mcpServers": []any{}})
	_ = readEnvelope()
	writeRequest(3, "session/prompt", map[string]any{"sessionId": "s1", "prompt": []any{map[string]any{"type": "text", "text": "File an argument."}}})

	var submittedArtifact map[string]any
	var submittedDecision map[string]any
	var calls []string
	promptDone := false
	for submittedDecision == nil || !promptDone {
		env := readEnvelope()
		if method, _ := env["method"].(string); method != "" {
			switch method {
			case "_aar/get_case":
				writeResponse(env["id"], map[string]any{"case": map[string]any{"proposition": "P"}})
			case "_aar/submit_artifact":
				calls = append(calls, "submit_artifact")
				params, _ := env["params"].(map[string]any)
				submittedArtifact = params
				if _, exists := params["offer_label"]; exists {
					t.Fatalf("submit_artifact params leaked offer_label: %#v", params)
				}
				writeResponse(env["id"], map[string]any{"artifact_id": "submitted-evidence-01-plaintiff-abc.txt"})
			case "_aar/submit_decision":
				calls = append(calls, "submit_decision")
				params, _ := env["params"].(map[string]any)
				submittedDecision = params
				writeResponse(env["id"], map[string]any{"text": "Decision accepted."})
			default:
				t.Fatalf("unexpected client method %q", method)
			}
			continue
		}
		if env["id"].(float64) == 3 {
			if result, _ := env["result"].(map[string]any); result["stopReason"] != "end_turn" {
				t.Fatalf("prompt result = %#v", result)
			}
			promptDone = true
		}
	}
	if submittedArtifact == nil {
		t.Fatal("no evidence submitted")
	}
	if !reflect.DeepEqual(calls, []string{"submit_artifact", "submit_decision"}) {
		t.Fatalf("calls = %#v", calls)
	}
	if submittedArtifact["content"] != "exact source text" {
		t.Fatalf("submitted evidence = %#v", submittedArtifact)
	}
	payload, _ := submittedDecision["payload"].(map[string]any)
	offered, _ := payload["offered_artifacts"].([]any)
	if len(offered) != 1 {
		t.Fatalf("offered_artifacts = %#v", payload["offered_artifacts"])
	}
	first, _ := offered[0].(map[string]any)
	if first["artifact_id"] != "submitted-evidence-01-plaintiff-abc.txt" || first["label"] != "PX-new" {
		t.Fatalf("offered file = %#v", first)
	}

	_ = clientConn.Close()
	select {
	case err := <-errCh:
		if err != nil && !strings.Contains(err.Error(), "closed") {
			t.Fatalf("server returned error: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("server did not exit")
	}
}
