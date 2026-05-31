package openclawattorney

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const defaultCommandTimeout = 600 * time.Second

type Config struct {
	Command             string
	FixedDecision       string
	CommandTimeout      time.Duration
	IncludeCaseView     bool
	IncludeTextFiles    bool
	UseOpenClawAgent    bool
	OpenClawCLI         string
	OpenClawAgentID     string
	OpenClawSessionID   string
	OpenClawThinking    string
	OpenClawLocal       bool
	OpenClawExtraPrompt string
}

type Server struct {
	rw      io.ReadWriter
	stderr  io.Writer
	cfg     Config
	nextID  atomic.Int64
	mu      sync.Mutex
	pending map[int64]chan rpcResponse
	closed  bool
	errc    chan error
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type lawyerJob struct {
	SessionID        string                       `json:"session_id"`
	Prompt           string                       `json:"prompt"`
	Case             map[string]any               `json:"case,omitempty"`
	TextEvidence     []textEvidence               `json:"text_evidence,omitempty"`
	RejectedFilings  []string                     `json:"rejected_filings,omitempty"`
	AcceptedEvidence []acceptedEvidenceSubmission `json:"accepted_evidence,omitempty"`
}

type acceptedEvidenceSubmission struct {
	EvidenceID   string `json:"evidence_id"`
	Title        string `json:"title,omitempty"`
	OfferLabel   string `json:"offer_label,omitempty"`
	SubmittedNow bool   `json:"submitted_now,omitempty"`
}

type textEvidence struct {
	EvidenceID string `json:"evidence_id"`
	Title      string `json:"title"`
	Text       string `json:"text"`
}

func ConfigFromEnv() Config {
	cfg := Config{
		Command:             strings.TrimSpace(os.Getenv("AAR_OPENCLAW_ATTORNEY_COMMAND")),
		FixedDecision:       strings.TrimSpace(os.Getenv("AAR_OPENCLAW_ATTORNEY_DECISION_JSON")),
		CommandTimeout:      defaultCommandTimeout,
		IncludeCaseView:     true,
		IncludeTextFiles:    true,
		UseOpenClawAgent:    envBool("AAR_OPENCLAW_AGENT", false),
		OpenClawCLI:         strings.TrimSpace(os.Getenv("AAR_OPENCLAW_CLI")),
		OpenClawAgentID:     strings.TrimSpace(os.Getenv("AAR_OPENCLAW_AGENT_ID")),
		OpenClawSessionID:   strings.TrimSpace(os.Getenv("AAR_OPENCLAW_AGENT_SESSION_ID")),
		OpenClawThinking:    strings.TrimSpace(os.Getenv("AAR_OPENCLAW_AGENT_THINKING")),
		OpenClawLocal:       envBool("AAR_OPENCLAW_AGENT_LOCAL", false),
		OpenClawExtraPrompt: strings.TrimSpace(os.Getenv("AAR_OPENCLAW_AGENT_EXTRA_PROMPT")),
	}
	if cfg.OpenClawCLI == "" {
		cfg.OpenClawCLI = "openclaw"
	}
	if raw := strings.TrimSpace(os.Getenv("AAR_OPENCLAW_ATTORNEY_TIMEOUT_SECONDS")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			cfg.CommandTimeout = time.Duration(seconds) * time.Second
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AAR_OPENCLAW_ATTORNEY_INCLUDE_CASE")); raw != "" {
		cfg.IncludeCaseView = raw != "0" && !strings.EqualFold(raw, "false") && !strings.EqualFold(raw, "no")
	}
	if raw := strings.TrimSpace(os.Getenv("AAR_OPENCLAW_ATTORNEY_INCLUDE_TEXT_FILES")); raw != "" {
		cfg.IncludeTextFiles = raw != "0" && !strings.EqualFold(raw, "false") && !strings.EqualFold(raw, "no")
	}
	return cfg
}

func Run(ctx context.Context, rw io.ReadWriter, stderr io.Writer, cfg Config) error {
	if cfg.CommandTimeout == 0 {
		cfg.CommandTimeout = defaultCommandTimeout
	}
	if stderr == nil {
		stderr = io.Discard
	}
	s := &Server{
		rw:      rw,
		stderr:  stderr,
		cfg:     cfg,
		pending: map[int64]chan rpcResponse{},
		errc:    make(chan error, 1),
	}
	return s.serve(ctx)
}

func (s *Server) serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.rw)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	scanErr := make(chan error, 1)
	go func() {
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				scanErr <- ctx.Err()
				return
			default:
			}
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			if err := s.handleLine(ctx, append([]byte(nil), line...)); err != nil {
				scanErr <- err
				return
			}
		}
		scanErr <- scanner.Err()
	}()
	var err error
	select {
	case err = <-scanErr:
	case err = <-s.errc:
	case <-ctx.Done():
		err = ctx.Err()
	}
	if closer, ok := s.rw.(io.Closer); ok {
		err = errors.Join(err, closer.Close())
	}
	pendingErr := err
	if pendingErr == nil {
		pendingErr = io.EOF
	}
	s.closePending(pendingErr)
	return err
}

func (s *Server) handleLine(ctx context.Context, line []byte) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(line, &envelope); err != nil {
		return fmt.Errorf("decode acp line: %w", err)
	}
	if methodRaw, ok := envelope["method"]; ok {
		var method string
		if err := json.Unmarshal(methodRaw, &method); err != nil {
			return fmt.Errorf("decode acp method: %w", err)
		}
		var id int64
		if idRaw, ok := envelope["id"]; ok && len(idRaw) > 0 {
			if err := json.Unmarshal(idRaw, &id); err != nil {
				return fmt.Errorf("decode acp request id: %w", err)
			}
		}
		params := map[string]any{}
		if paramsRaw, ok := envelope["params"]; ok && len(paramsRaw) > 0 {
			if err := json.Unmarshal(paramsRaw, &params); err != nil {
				return fmt.Errorf("decode acp params: %w", err)
			}
		}
		if id == 0 {
			return nil
		}
		go s.handleRequest(ctx, id, method, params)
		return nil
	}
	var resp rpcResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("decode acp response: %w", err)
	}
	s.mu.Lock()
	ch := s.pending[resp.ID]
	if ch != nil {
		delete(s.pending, resp.ID)
	}
	s.mu.Unlock()
	if ch != nil {
		ch <- resp
	}
	return nil
}

func (s *Server) handleRequest(ctx context.Context, id int64, method string, params map[string]any) {
	result, err := s.dispatchRequest(ctx, method, params)
	if err != nil {
		if err := s.writeJSON(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]any{
				"code":    -32000,
				"message": err.Error(),
			},
		}); err != nil {
			s.fail(fmt.Errorf("write acp error response: %w", err))
		}
		return
	}
	if err := s.writeJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}); err != nil {
		s.fail(fmt.Errorf("write acp response: %w", err))
	}
}

func (s *Server) dispatchRequest(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	switch method {
	case "initialize":
		return map[string]any{
			"protocolVersion": 1,
			"agentInfo": map[string]any{
				"name":    "aar-openclaw-attorney",
				"title":   "AAR OpenClaw Attorney Adapter",
				"version": "0.1.0",
			},
		}, nil
	case "session/new":
		return map[string]any{"sessionId": fmt.Sprintf("aar-openclaw-attorney-%d", time.Now().UTC().UnixNano())}, nil
	case "session/prompt":
		if err := s.handlePrompt(ctx, params); err != nil {
			return nil, err
		}
		return map[string]any{"stopReason": "end_turn"}, nil
	default:
		return nil, fmt.Errorf("method not handled: %s", method)
	}
}

func (s *Server) handlePrompt(ctx context.Context, params map[string]any) error {
	sessionID := stringValue(params["sessionId"])
	prompt := promptText(params["prompt"])
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("session/prompt missing text prompt")
	}
	caseView := map[string]any{}
	if s.cfg.IncludeCaseView {
		result, err := s.clientRequest(ctx, "_aar/get_case", map[string]any{})
		if err != nil {
			return fmt.Errorf("get AAR case: %w", err)
		}
		caseView = result
	}
	textEvidence, err := s.loadTextEvidence(ctx)
	if err != nil {
		return err
	}
	job := lawyerJob{SessionID: sessionID, Prompt: prompt, Case: caseView, TextEvidence: textEvidence}
	var submitErr error
	for attempt := 1; attempt <= 3; attempt++ {
		response, err := s.obtainDecision(ctx, job)
		if err != nil {
			return err
		}
		decision, accepted, err := s.prepareDecision(ctx, response)
		if len(accepted) > 0 {
			job.AcceptedEvidence = append(job.AcceptedEvidence, accepted...)
		}
		if err != nil {
			submitErr = err
			job.RejectedFilings = append(job.RejectedFilings, submitErr.Error())
			continue
		}
		if _, err := s.clientRequest(ctx, "_aar/submit_decision", decision); err != nil {
			submitErr = fmt.Errorf("submit AAR decision: %w", err)
			job.RejectedFilings = append(job.RejectedFilings, submitErr.Error())
			continue
		}
		return nil
	}
	return submitErr
}

func (s *Server) loadTextEvidence(ctx context.Context) ([]textEvidence, error) {
	if !s.cfg.IncludeTextFiles {
		return nil, nil
	}
	result, err := s.clientRequest(ctx, "_aar/list_evidence", map[string]any{})
	if err != nil {
		if strings.Contains(err.Error(), "evidence access is not allowed") {
			return nil, nil
		}
		return nil, fmt.Errorf("list AAR evidence: %w", err)
	}
	metas, _ := result["evidence"].([]any)
	out := make([]textEvidence, 0, len(metas))
	for _, raw := range metas {
		meta, _ := raw.(map[string]any)
		if meta == nil || !boolValue(meta["text_readable"]) {
			continue
		}
		evidenceID := strings.TrimSpace(stringValue(meta["evidence_id"]))
		if evidenceID == "" {
			continue
		}
		sizeBytes := intValue(meta["size_bytes"])
		if sizeBytes <= 0 {
			continue
		}
		text, err := s.readTextEvidence(ctx, evidenceID, sizeBytes)
		if err != nil {
			return nil, fmt.Errorf("read AAR evidence %s: %w", evidenceID, err)
		}
		out = append(out, textEvidence{
			EvidenceID: evidenceID,
			Title:      stringValue(meta["title"]),
			Text:       text,
		})
	}
	return out, nil
}

func (s *Server) readTextEvidence(ctx context.Context, evidenceID string, sizeBytes int) (string, error) {
	stat, err := s.clientRequest(ctx, "_aar/stat_evidence", map[string]any{"evidence_id": evidenceID})
	if err != nil {
		return "", err
	}
	limits := mapValue(stat["limits"])
	maxReadBytes := intValue(limits["max_read_bytes"])
	if maxReadBytes <= 0 {
		maxReadBytes = sizeBytes
	}
	remainingBytes := sizeBytes
	if raw, ok := limits["remaining_read_bytes_for_opportunity"]; ok {
		remainingBytes = intValue(raw)
	}
	remainingReads := (sizeBytes + maxReadBytes - 1) / maxReadBytes
	if raw, ok := limits["remaining_reads_for_opportunity"]; ok {
		remainingReads = intValue(raw)
	}

	var b strings.Builder
	offset := 0
	for offset < sizeBytes && remainingReads > 0 && remainingBytes > 0 {
		length := sizeBytes - offset
		if length > maxReadBytes {
			length = maxReadBytes
		}
		if length > remainingBytes {
			length = remainingBytes
		}
		if length <= 0 {
			break
		}
		readResult, err := s.clientRequest(ctx, "_aar/read_evidence_range", map[string]any{"evidence_id": evidenceID, "offset": offset, "length": length})
		if err != nil {
			return "", err
		}
		rawText, err := base64.StdEncoding.DecodeString(stringValue(readResult["content_base64"]))
		if err != nil {
			return "", fmt.Errorf("decode evidence range: %w", err)
		}
		n := intValue(readResult["length"])
		if n <= 0 {
			n = len(rawText)
		}
		if n <= 0 {
			break
		}
		if n > len(rawText) {
			n = len(rawText)
		}
		b.Write(rawText[:n])
		offset += n
		remainingBytes -= n
		remainingReads--
		if raw, ok := readResult["remaining_read_bytes_for_opportunity"]; ok {
			remainingBytes = intValue(raw)
		}
		if raw, ok := readResult["remaining_reads_for_opportunity"]; ok {
			remainingReads = intValue(raw)
		}
	}
	if offset < sizeBytes {
		b.WriteString(fmt.Sprintf("\n\n[evidence text truncated after %d of %d bytes by AAR read limits]", offset, sizeBytes))
	}
	return b.String(), nil
}

func (s *Server) obtainDecision(ctx context.Context, job lawyerJob) (map[string]any, error) {
	if strings.TrimSpace(s.cfg.FixedDecision) != "" {
		return parseDecisionJSON([]byte(s.cfg.FixedDecision))
	}
	if strings.TrimSpace(s.cfg.Command) == "" {
		if s.cfg.UseOpenClawAgent {
			return s.obtainDecisionFromOpenClawAgent(ctx, job)
		}
		return nil, errors.New("AAR_OPENCLAW_ATTORNEY_COMMAND, AAR_OPENCLAW_ATTORNEY_DECISION_JSON, or AAR_OPENCLAW_AGENT=1 is required")
	}
	body, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal lawyer job: %w", err)
	}
	timeout := s.cfg.CommandTimeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "/bin/sh", "-c", s.cfg.Command)
	cmd.Stdin = bytes.NewReader(body)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(),
		"AAR_OPENCLAW_ATTORNEY_SESSION_ID="+job.SessionID,
	)
	if err := cmd.Run(); err != nil {
		if cmdCtx.Err() != nil {
			return nil, fmt.Errorf("lawyer command timed out after %s", timeout)
		}
		return nil, fmt.Errorf("lawyer command failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if text := strings.TrimSpace(stderr.String()); text != "" {
		fmt.Fprintln(s.stderr, text)
	}
	decision, err := parseDecisionJSON(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("parse lawyer decision: %w", err)
	}
	return decision, nil
}

func (s *Server) obtainDecisionFromOpenClawAgent(ctx context.Context, job lawyerJob) (map[string]any, error) {
	if strings.TrimSpace(s.cfg.OpenClawAgentID) == "" {
		return nil, errors.New("AAR_OPENCLAW_AGENT_ID is required when AAR_OPENCLAW_AGENT=1; use a dedicated lawyer agent, not the default personal agent")
	}
	prompt, err := buildOpenClawAgentPrompt(job, s.cfg.OpenClawExtraPrompt)
	if err != nil {
		return nil, err
	}
	timeout := s.cfg.CommandTimeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"agent", "--message", prompt, "--json", "--timeout", strconv.Itoa(int(timeout.Seconds()))}
	if s.cfg.OpenClawAgentID != "" {
		args = append(args, "--agent", s.cfg.OpenClawAgentID)
	}
	sessionID := s.cfg.OpenClawSessionID
	if sessionID == "" {
		sessionID = "aar-openclaw-attorney-" + safeSessionID(job.SessionID)
	}
	args = append(args, "--session-id", sessionID)
	if s.cfg.OpenClawThinking != "" {
		args = append(args, "--thinking", s.cfg.OpenClawThinking)
	}
	if s.cfg.OpenClawLocal {
		args = append(args, "--local")
	}
	cmd := exec.CommandContext(cmdCtx, s.cfg.OpenClawCLI, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if cmdCtx.Err() != nil {
			return nil, fmt.Errorf("OpenClaw agent command timed out after %s", timeout)
		}
		return nil, fmt.Errorf("OpenClaw agent command failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if text := strings.TrimSpace(stderr.String()); text != "" {
		fmt.Fprintln(s.stderr, text)
	}
	text := extractOpenClawText(stdout.Bytes())
	if strings.TrimSpace(text) == "" {
		text = stdout.String()
	}
	decision, err := parseDecisionJSON([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("parse OpenClaw lawyer decision: %w", err)
	}
	return decision, nil
}

func buildOpenClawAgentPrompt(job lawyerJob, extra string) (string, error) {
	caseJSON, err := json.MarshalIndent(job.Case, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal case view: %w", err)
	}
	var b strings.Builder
	b.WriteString("You are counsel in an AgentCourt AAR arbitration. Analyze the record and file the legal act for the current opportunity.\n\n")
	if strings.TrimSpace(extra) != "" {
		b.WriteString(strings.TrimSpace(extra))
		b.WriteString("\n\n")
	}
	b.WriteString("Return exactly one JSON object. Do not include prose, markdown, or a code fence.\n")
	b.WriteString("You may return either an ordinary aar_submit_decision object, or a structured bundle with evidence_submissions and decision.\n")
	b.WriteString("Use the structured bundle when you found source material outside the record that should become evidence. Each evidence_submissions item may include title, source_url, source_description, retrieval_timestamp, mime_type, relevance, content or content_base64, preferred_filename_ext, offer_label, and offer_as_exhibit. The adapter submits those items with aar_submit_evidence before filing the decision. If offer_as_exhibit is omitted, accepted evidence is cited in offered_evidence for arguments and rebuttals. Do not include evidence_submissions in closings.\n")
	b.WriteString("Ordinary decision form: {\"kind\":\"tool\",\"tool_name\":\"submit_argument\",\"payload\":{...}}. Structured bundle form: {\"evidence_submissions\":[{...}],\"decision\":{\"kind\":\"tool\",\"tool_name\":\"submit_argument\",\"payload\":{...}}}.\n\n")
	if len(job.AcceptedEvidence) > 0 {
		b.WriteString("Evidence already accepted during this opportunity. Do not resubmit these items; cite the evidence_id values in offered_evidence if needed:\n")
		for i, item := range job.AcceptedEvidence {
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(". ")
			b.WriteString(item.EvidenceID)
			if item.Title != "" {
				b.WriteString(" — ")
				b.WriteString(item.Title)
			}
			b.WriteByte('\n')
		}
		b.WriteString("\n")
	}
	if len(job.RejectedFilings) > 0 {
		b.WriteString("Your previous filing for this same opportunity was rejected by AAR. Correct the filing and return a new JSON object. Rejections:\n")
		for i, rejection := range job.RejectedFilings {
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(". ")
			b.WriteString(rejection)
			b.WriteByte('\n')
		}
		b.WriteString("\n")
	}
	b.WriteString("AAR attorney prompt:\n")
	b.WriteString(job.Prompt)
	b.WriteString("\n\nVisible arbitration record from aar_get_case:\n")
	b.Write(caseJSON)
	if len(job.TextEvidence) > 0 {
		b.WriteString("\n\nVisible text evidence:\n")
		for _, file := range job.TextEvidence {
			b.WriteString("\n--- evidence_id: ")
			b.WriteString(file.EvidenceID)
			if file.Title != "" {
				b.WriteString(" title: ")
				b.WriteString(file.Title)
			}
			b.WriteString(" ---\n")
			b.WriteString(file.Text)
			if !strings.HasSuffix(file.Text, "\n") {
				b.WriteByte('\n')
			}
		}
	}
	return b.String(), nil
}

func extractOpenClawText(raw []byte) string {
	var root map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &root); err != nil {
		return ""
	}
	for _, candidate := range []any{root["result"], root} {
		m, _ := candidate.(map[string]any)
		if m == nil {
			continue
		}
		payloads, _ := m["payloads"].([]any)
		parts := make([]string, 0, len(payloads))
		for _, rawPayload := range payloads {
			payload, _ := rawPayload.(map[string]any)
			if payload == nil {
				continue
			}
			if text := strings.TrimSpace(stringValue(payload["text"])); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	return ""
}

func safeSessionID(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	}
	return b.String()
}

func (s *Server) clientRequest(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	id := s.nextID.Add(1)
	ch := make(chan rpcResponse, 1)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("server is closed")
	}
	s.pending[id] = ch
	s.mu.Unlock()
	if err := s.writeJSON(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		s.removePending(id)
		return nil, err
	}
	select {
	case <-ctx.Done():
		s.removePending(id)
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("client method %s failed: %s", method, resp.Error.Message)
		}
		out := map[string]any{}
		if len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, &out); err != nil {
				return nil, fmt.Errorf("decode client method result: %w", err)
			}
		}
		return out, nil
	}
}

func (s *Server) removePending(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, id)
}

func (s *Server) fail(err error) {
	if err == nil {
		return
	}
	s.closePending(err)
	select {
	case s.errc <- err:
	default:
	}
}

func (s *Server) closePending(err error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	pending := s.pending
	s.pending = map[int64]chan rpcResponse{}
	s.mu.Unlock()
	if err == nil {
		return
	}
	resp := rpcResponse{Error: &rpcError{Code: -32000, Message: err.Error()}}
	for _, ch := range pending {
		ch <- resp
	}
}

func (s *Server) writeJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.rw.Write(append(payload, '\n'))
	return err
}

func (s *Server) prepareDecision(ctx context.Context, response map[string]any) (map[string]any, []acceptedEvidenceSubmission, error) {
	decision := response
	if nested := mapValue(response["decision"]); nested != nil {
		decision = cloneMap(nested)
	}
	evidenceEntries := listOfMaps(response["evidence_submissions"])
	if len(evidenceEntries) == 0 {
		return decision, nil, nil
	}
	toolName := strings.TrimSpace(stringValue(decision["tool_name"]))
	if toolName != "submit_argument" && toolName != "submit_rebuttal" {
		return nil, nil, fmt.Errorf("evidence_submissions are allowed only with submit_argument or submit_rebuttal decisions")
	}
	accepted := make([]acceptedEvidenceSubmission, 0, len(evidenceEntries))
	for i, entry := range evidenceEntries {
		params := cloneMap(entry)
		offerLabel := strings.TrimSpace(stringValue(params["offer_label"]))
		offerAsExhibit := boolDefault(params["offer_as_exhibit"], true)
		delete(params, "offer_label")
		delete(params, "offer_as_exhibit")
		result, err := s.clientRequest(ctx, "_aar/submit_evidence", params)
		if err != nil {
			return nil, accepted, fmt.Errorf("submit AAR evidence %d: %w", i+1, err)
		}
		evidenceID := strings.TrimSpace(stringValue(result["evidence_id"]))
		if evidenceID == "" {
			if evidence := mapValue(result["evidence"]); evidence != nil {
				evidenceID = strings.TrimSpace(stringValue(evidence["evidence_id"]))
			}
		}
		if evidenceID == "" {
			return nil, accepted, fmt.Errorf("submit AAR evidence %d returned no evidence_id", i+1)
		}
		if offerLabel == "" {
			offerLabel = strings.TrimSpace(stringValue(params["title"]))
		}
		if offerLabel == "" {
			offerLabel = evidenceID
		}
		accepted = append(accepted, acceptedEvidenceSubmission{
			EvidenceID:   evidenceID,
			Title:        strings.TrimSpace(stringValue(params["title"])),
			OfferLabel:   offerLabel,
			SubmittedNow: true,
		})
		if offerAsExhibit {
			appendOfferedEvidence(decision, evidenceID, offerLabel)
		}
	}
	return decision, accepted, nil
}

func appendOfferedEvidence(decision map[string]any, evidenceID string, label string) {
	payload := mapValue(decision["payload"])
	if payload == nil {
		payload = map[string]any{}
	}
	offered := listOfMaps(payload["offered_evidence"])
	for _, item := range offered {
		if strings.TrimSpace(stringValue(item["evidence_id"])) == evidenceID {
			decision["payload"] = payload
			return
		}
	}
	offered = append(offered, map[string]any{"evidence_id": evidenceID, "label": label})
	payload["offered_evidence"] = offered
	decision["payload"] = payload
}

func mapValue(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func listOfMaps(value any) []map[string]any {
	switch v := value.(type) {
	case []map[string]any:
		out := make([]map[string]any, len(v))
		copy(out, v)
		return out
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, raw := range v {
			if entry, ok := raw.(map[string]any); ok && entry != nil {
				out = append(out, cloneMap(entry))
			}
		}
		return out
	default:
		return nil
	}
}

func boolDefault(value any, fallback bool) bool {
	if value == nil {
		return fallback
	}
	b, ok := value.(bool)
	if !ok {
		return fallback
	}
	return b
}

func parseDecisionJSON(raw []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty decision JSON")
	}
	candidates := [][]byte{trimmed}
	if extracted, ok := extractFirstJSONObject(trimmed); ok {
		candidates = append(candidates, extracted)
	}
	var lastErr error
	for _, candidate := range candidates {
		var out map[string]any
		dec := json.NewDecoder(bytes.NewReader(candidate))
		if err := dec.Decode(&out); err != nil {
			lastErr = err
			continue
		}
		if strings.TrimSpace(stringValue(out["kind"])) != "" {
			return out, nil
		}
		if decision := mapValue(out["decision"]); decision != nil {
			if strings.TrimSpace(stringValue(decision["kind"])) == "" {
				lastErr = fmt.Errorf("structured decision JSON missing decision.kind")
				continue
			}
			return out, nil
		}
		lastErr = fmt.Errorf("decision JSON missing kind or decision")
	}
	return nil, lastErr
}

func extractFirstJSONObject(raw []byte) ([]byte, bool) {
	start := bytes.IndexByte(raw, '{')
	if start < 0 {
		return nil, false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		b := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1], true
			}
		}
	}
	return nil, false
}

func promptText(raw any) string {
	items, ok := raw.([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if stringValue(m["type"]) == "text" {
			if text := stringValue(m["text"]); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func boolValue(v any) bool {
	b, _ := v.(bool)
	return b
}

func intValue(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func envBool(name string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	return raw != "0" && !strings.EqualFold(raw, "false") && !strings.EqualFold(raw, "no")
}
