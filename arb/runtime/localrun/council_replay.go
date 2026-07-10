package localrun

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"adjudication/arb/runtime/mcp"
	"adjudication/arb/runtime/proceeding"
	"adjudication/common/modelrequest"
	"adjudication/common/persona"
)

type CouncilReplayOptions struct {
	Basis                   string
	SourceOutputDir         string
	SnapshotDir             string
	PromptDir               string
	ModelConfigPath         string
	PersonaPath             string
	OutputDir               string
	MemberID                string
	CaseAPIAddr             string
	MCPListenAddr           string
	MCPBearerToken          string
	CouncilTimeoutSeconds   int
	CouncilInstructionsPath string
	PodmanCommand           string
	PiImage                 string
	PiMCPAdapter            string
	PodmanMCPHost           string
	CouncilOutputLimitBytes int64
	Log                     io.Writer
}

type CouncilReplayResult struct {
	Status          string                        `json:"status"`
	Basis           string                        `json:"basis"`
	CaseID          string                        `json:"case_id"`
	MemberID        string                        `json:"member_id"`
	Model           string                        `json:"model"`
	OutputDir       string                        `json:"out_dir"`
	SourceOutputDir string                        `json:"source_output_dir"`
	SnapshotDir     string                        `json:"snapshot_dir,omitempty"`
	Vote            string                        `json:"vote,omitempty"`
	Rationale       string                        `json:"rationale,omitempty"`
	Error           string                        `json:"error,omitempty"`
	ErrorDetails    map[string]any                `json:"error_details,omitempty"`
	ToolCalls       []CouncilReplayToolCall       `json:"tool_calls,omitempty"`
	Input           proceeding.CouncilReplayInput `json:"input"`
}

type CouncilReplayToolCall struct {
	Timestamp string         `json:"timestamp"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    map[string]any `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
}

type councilReplayVote struct {
	Vote      string `json:"vote"`
	Rationale string `json:"rationale"`
}

type councilReplayFailure struct {
	Reason  string
	Message string
	Details map[string]any
}

type replayDoRequest struct {
	CaseID        string         `json:"case_id"`
	MemberID      string         `json:"member_id"`
	OpportunityID string         `json:"opportunity_id"`
	Tool          string         `json:"tool"`
	Arguments     map[string]any `json:"arguments"`
}

func ReplayCouncil(ctx context.Context, opts CouncilReplayOptions) (result CouncilReplayResult, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = applyCouncilReplayDefaults(opts)
	if err := validateCouncilReplayOptions(opts); err != nil {
		return CouncilReplayResult{}, err
	}
	entry, seat, err := loadCouncilReplayModelConfig(opts.ModelConfigPath, opts.MemberID, opts.PersonaPath)
	if err != nil {
		return CouncilReplayResult{}, err
	}
	input, err := proceeding.BuildCouncilReplayInput(proceeding.CouncilReplayBuildOptions{
		Basis:           opts.Basis,
		SourceOutputDir: opts.SourceOutputDir,
		SnapshotDir:     opts.SnapshotDir,
		PromptDir:       opts.PromptDir,
		Seat:            seat,
	})
	if err != nil {
		return CouncilReplayResult{}, err
	}
	entry.MemberID = input.MemberID
	if err := os.MkdirAll(filepath.Join(opts.OutputDir, "logs"), 0o755); err != nil {
		return CouncilReplayResult{}, fmt.Errorf("create replay logs: %w", err)
	}
	if err := writeJSONFile(filepath.Join(opts.OutputDir, "input.json"), input); err != nil {
		return CouncilReplayResult{}, err
	}
	if err := os.WriteFile(filepath.Join(opts.OutputDir, "prompt.txt"), []byte(input.Prompt), 0o644); err != nil {
		return CouncilReplayResult{}, fmt.Errorf("write replay prompt: %w", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	state := &runState{
		opts: Options{
			OutputDir:               opts.OutputDir,
			CaseID:                  input.CaseID,
			MCPBearerToken:          opts.MCPBearerToken,
			PodmanCommand:           opts.PodmanCommand,
			PiImage:                 opts.PiImage,
			PiMCPAdapter:            opts.PiMCPAdapter,
			PodmanMCPHost:           opts.PodmanMCPHost,
			CouncilInstructionsPath: opts.CouncilInstructionsPath,
			CouncilOutputLimitBytes: opts.CouncilOutputLimitBytes,
			Log:                     opts.Log,
		},
		logDir:        filepath.Join(opts.OutputDir, "logs"),
		token:         strings.TrimSpace(opts.MCPBearerToken),
		councilStarts: map[string]bool{},
		agentErrs:     make(chan error, 8),
	}
	defer func() {
		if cleanupErr := state.cleanupSecrets(); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()
	if state.token == "" {
		token, err := randomToken()
		if err != nil {
			return CouncilReplayResult{}, err
		}
		state.token = token
	}
	caseAddr, err := resolveListenAddr(opts.CaseAPIAddr, "127.0.0.1")
	if err != nil {
		return CouncilReplayResult{}, fmt.Errorf("resolve replay case API address: %w", err)
	}
	caseServer, caseBase, err := startFrozenCouncilReplayServer(runCtx, input, caseAddr, opts.OutputDir)
	if err != nil {
		return CouncilReplayResult{}, err
	}
	defer caseServer.Close()
	state.caseBase = caseBase
	if err := waitForHealth(runCtx, state.caseBase+"/health", defaultCaseAPIStartupWait); err != nil {
		return CouncilReplayResult{}, err
	}
	mcpListenAddr, err := resolveListenAddr(opts.MCPListenAddr, "127.0.0.1")
	if err != nil {
		return CouncilReplayResult{}, fmt.Errorf("resolve replay MCP address: %w", err)
	}
	_, mcpPort, err := net.SplitHostPort(mcpListenAddr)
	if err != nil {
		return CouncilReplayResult{}, fmt.Errorf("parse replay MCP address %q: %w", mcpListenAddr, err)
	}
	state.mcpBase = "http://" + net.JoinHostPort("127.0.0.1", mcpPort)
	mcpDone := make(chan error, 1)
	go func() {
		logFile, err := os.Create(filepath.Join(state.logDir, "mcp.stderr"))
		if err != nil {
			mcpDone <- fmt.Errorf("create replay MCP log: %w", err)
			return
		}
		defer logFile.Close()
		mcpDone <- mcp.Run(runCtx, mcp.Options{
			ListenAddr:           mcpListenAddr,
			CaseAPIBase:          state.caseBase,
			BearerToken:          state.token,
			DisableSessionExpiry: true,
			Log:                  logFile,
		})
	}()
	if err := waitForHealth(runCtx, state.mcpBase+"/health", defaultMCPStartupWait); err != nil {
		cancel()
		return CouncilReplayResult{}, err
	}
	if err := state.startPiCouncil(runCtx, entry, mcpPort, input.Opportunity.ID); err != nil {
		cancel()
		return CouncilReplayResult{}, err
	}
	result, err = waitCouncilReplayResult(runCtx, opts, input, state, caseServer)
	cancel()
	stopErr := state.stopAgents()
	mcpErr := <-mcpDone
	if mcpErr != nil && !errors.Is(mcpErr, context.Canceled) && err == nil {
		err = mcpErr
	}
	if stopErr != nil && err == nil {
		err = stopErr
	}
	result.ToolCalls = caseServer.ToolCalls()
	result.Input = input
	if writeErr := writeCouncilReplayResult(opts.OutputDir, result); writeErr != nil && err == nil {
		err = writeErr
	}
	return result, err
}

func applyCouncilReplayDefaults(opts CouncilReplayOptions) CouncilReplayOptions {
	opts.Basis = strings.TrimSpace(opts.Basis)
	if strings.TrimSpace(opts.PodmanCommand) == "" {
		opts.PodmanCommand = DefaultPodmanCommand
	}
	if strings.TrimSpace(opts.PiImage) == "" {
		if image := strings.TrimSpace(os.Getenv("PI_CONTAINER_IMAGE")); image != "" {
			opts.PiImage = image
		} else {
			opts.PiImage = defaultPiImage
		}
	}
	if strings.TrimSpace(opts.PiMCPAdapter) == "" {
		opts.PiMCPAdapter = defaultPiMCPAdapter
	}
	if strings.TrimSpace(opts.PodmanMCPHost) == "" {
		opts.PodmanMCPHost = "127.0.0.1"
	}
	if strings.TrimSpace(opts.CouncilInstructionsPath) == "" {
		opts.CouncilInstructionsPath = DefaultCouncilInstructionsPath()
	}
	if opts.CouncilOutputLimitBytes == 0 {
		opts.CouncilOutputLimitBytes = DefaultCouncilOutputLimitBytes
	}
	if opts.CouncilTimeoutSeconds <= 0 {
		opts.CouncilTimeoutSeconds = DefaultRunCouncilTimeoutSeconds
	}
	return opts
}

func validateCouncilReplayOptions(opts CouncilReplayOptions) error {
	if opts.Basis != proceeding.CouncilReplayBasisReconstructed && opts.Basis != proceeding.CouncilReplayBasisSnapshot {
		return fmt.Errorf("--basis must be %s or %s", proceeding.CouncilReplayBasisReconstructed, proceeding.CouncilReplayBasisSnapshot)
	}
	if strings.TrimSpace(opts.SourceOutputDir) == "" {
		return fmt.Errorf("--source-output is required")
	}
	if opts.Basis == proceeding.CouncilReplayBasisSnapshot && strings.TrimSpace(opts.SnapshotDir) == "" {
		return fmt.Errorf("--snapshot is required with --basis %s", proceeding.CouncilReplayBasisSnapshot)
	}
	if opts.Basis == proceeding.CouncilReplayBasisSnapshot && strings.TrimSpace(opts.MemberID) != "" {
		return fmt.Errorf("--member-id cannot override a snapshot replay member")
	}
	if strings.TrimSpace(opts.ModelConfigPath) == "" {
		return fmt.Errorf("--config is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return fmt.Errorf("--out-dir is required")
	}
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		return fmt.Errorf("OPENROUTER_API_KEY is required for Pi council replay")
	}
	if opts.CouncilOutputLimitBytes < 0 {
		return fmt.Errorf("council output limit bytes must be non-negative")
	}
	if _, err := os.Stat(opts.CouncilInstructionsPath); err != nil {
		return fmt.Errorf("stat council instruction template %s: %w", opts.CouncilInstructionsPath, err)
	}
	return nil
}

func loadCouncilReplayModelConfig(path string, memberID string, personaPath string) (councilRosterEntry, proceeding.CouncilSeat, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return councilRosterEntry{}, proceeding.CouncilSeat{}, fmt.Errorf("read council config %s: %w", path, err)
	}
	if strings.TrimSpace(personaPath) != "" {
		requestSpec, err := modelrequest.ParseJSON(raw)
		if err != nil {
			return councilRosterEntry{}, proceeding.CouncilSeat{}, fmt.Errorf("parse council model config %s: %w", path, err)
		}
		file, text, err := readReplayPersonaOverride(personaPath)
		if err != nil {
			return councilRosterEntry{}, proceeding.CouncilSeat{}, err
		}
		requestSpec.Persona = file
		return councilReplayConfigFromSpec(persona.Spec{
			Model:       requestSpec.RuntimeModel(),
			File:        file,
			FilePath:    file,
			Text:        text,
			RequestSpec: &requestSpec,
		}, memberID)
	}
	spec, err := persona.ParseRecord(string(raw), filepath.Dir(path))
	if err != nil {
		return councilRosterEntry{}, proceeding.CouncilSeat{}, err
	}
	if spec.RequestSpec == nil {
		return councilRosterEntry{}, proceeding.CouncilSeat{}, fmt.Errorf("council config must be a JSON request-spec record")
	}
	return councilReplayConfigFromSpec(spec, memberID)
}

func readReplayPersonaOverride(path string) (string, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "", fmt.Errorf("--persona is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve persona path %s: %w", path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", fmt.Errorf("stat persona %s: %w", abs, err)
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("persona path is a directory: %s", abs)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return "", "", fmt.Errorf("read persona text %s: %w", abs, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "", "", fmt.Errorf("empty persona text: %s", abs)
	}
	return abs, text, nil
}

func councilReplayConfigFromSpec(spec persona.Spec, memberID string) (councilRosterEntry, proceeding.CouncilSeat, error) {
	if spec.RequestSpec == nil {
		return councilRosterEntry{}, proceeding.CouncilSeat{}, fmt.Errorf("council config must be a JSON request-spec record")
	}
	memberID = strings.TrimSpace(memberID)
	if memberID == "" {
		memberID = "C1"
	}
	entry := councilRosterEntry{
		MemberID:    memberID,
		Model:       spec.Model,
		RequestSpec: spec.RequestSpec,
	}
	seat := proceeding.CouncilSeat{
		MemberID:    memberID,
		Model:       spec.Model,
		PersonaFile: spec.File,
		RequestSpec: spec.RequestSpec,
		PersonaText: spec.Text,
	}
	return entry, seat, nil
}

func waitCouncilReplayResult(
	ctx context.Context,
	opts CouncilReplayOptions,
	input proceeding.CouncilReplayInput,
	state *runState,
	server *frozenCouncilReplayServer,
) (CouncilReplayResult, error) {
	timeout := time.Duration(opts.CouncilTimeoutSeconds) * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	base := CouncilReplayResult{
		Status:          "error",
		Basis:           input.Basis,
		CaseID:          input.CaseID,
		MemberID:        input.MemberID,
		Model:           input.Seat.Model,
		OutputDir:       opts.OutputDir,
		SourceOutputDir: input.SourceOutputDir,
		SnapshotDir:     input.SnapshotDir,
	}
	select {
	case vote := <-server.VoteDone():
		base.Status = "ok"
		base.Vote = vote.Vote
		base.Rationale = vote.Rationale
		return base, nil
	case failure := <-server.FailureDone():
		message := strings.TrimSpace(failure.Message)
		if message == "" {
			message = "council replay agent failed"
		}
		err := errors.New(message)
		base.Error = message
		base.ErrorDetails = cloneReplayMap(failure.Details)
		return base, err
	case err := <-state.agentErrs:
		base.Error = err.Error()
		return base, err
	case <-timer.C:
		err := fmt.Errorf("council replay timed out after %s", timeout)
		base.Error = err.Error()
		return base, err
	case <-ctx.Done():
		base.Error = ctx.Err().Error()
		return base, ctx.Err()
	}
}

func writeCouncilReplayResult(outputDir string, result CouncilReplayResult) error {
	if err := writeJSONFile(filepath.Join(outputDir, "result.json"), result); err != nil {
		return err
	}
	toolPath := filepath.Join(outputDir, "tool-calls.ndjson")
	if err := os.Remove(toolPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reset %s: %w", toolPath, err)
	}
	for _, call := range result.ToolCalls {
		raw, err := json.Marshal(call)
		if err != nil {
			return err
		}
		if err := appendLine(toolPath, raw); err != nil {
			return err
		}
	}
	return nil
}

func appendLine(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := f.Write(append(raw, '\n')); err != nil {
		closeErr := f.Close()
		return errors.Join(fmt.Errorf("write %s: %w", path, err), closeErr)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

type frozenCouncilReplayServer struct {
	input          proceeding.CouncilReplayInput
	outputDir      string
	server         *http.Server
	voteDone       chan councilReplayVote
	failureDone    chan councilReplayFailure
	version        uint64
	deadline       time.Time
	evidenceBudget replayEvidenceBudget
	mu             sync.Mutex
	vote           *councilReplayVote
	failure        string
	toolCalls      []CouncilReplayToolCall
}

type replayEvidenceBudget struct {
	bytes int
	reads int
}

func startFrozenCouncilReplayServer(ctx context.Context, input proceeding.CouncilReplayInput, addr string, outputDir string) (*frozenCouncilReplayServer, string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, "", err
	}
	server := &frozenCouncilReplayServer{
		input:       input,
		outputDir:   outputDir,
		voteDone:    make(chan councilReplayVote, 1),
		failureDone: make(chan councilReplayFailure, 1),
		version:     1,
		deadline:    time.Now().Add(input.Runtime.CouncilTimeout()),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/councilapi/v1/get", server.handleGet)
	mux.HandleFunc("/councilapi/v1/wait", server.handleWait)
	mux.HandleFunc("/councilapi/v1/do", server.handleDo)
	mux.HandleFunc("/councilapi/v1/fail", server.handleFail)
	server.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.server.Shutdown(shutdownCtx)
	}()
	go func() {
		if err := server.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			server.mu.Lock()
			server.failure = err.Error()
			server.version++
			server.mu.Unlock()
		}
	}()
	return server, "http://" + ln.Addr().String(), nil
}

func (s *frozenCouncilReplayServer) Close() error {
	if s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *frozenCouncilReplayServer) VoteDone() <-chan councilReplayVote {
	return s.voteDone
}

func (s *frozenCouncilReplayServer) FailureDone() <-chan councilReplayFailure {
	return s.failureDone
}

func (s *frozenCouncilReplayServer) ToolCalls() []CouncilReplayToolCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]CouncilReplayToolCall(nil), s.toolCalls...)
}

func (s *frozenCouncilReplayServer) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeReplayJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": replayAPIError("method_not_allowed", "use GET")})
		return
	}
	caseID, memberID, ok := s.parseIdentity(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	response := s.statusResponseLocked(caseID, memberID)
	s.mu.Unlock()
	writeReplayJSON(w, http.StatusOK, response)
}

func (s *frozenCouncilReplayServer) handleWait(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeReplayJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": replayAPIError("method_not_allowed", "use GET")})
		return
	}
	caseID, memberID, ok := s.parseIdentity(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	response := s.statusResponseLocked(caseID, memberID)
	reason := "ready"
	if response["status"] == "done" {
		reason = "done"
	} else if response["status"] == "failed" {
		reason = "failed"
	}
	response["wait"] = map[string]any{
		"reason":        reason,
		"version":       s.version,
		"state_version": s.input.State["state_version"],
	}
	s.mu.Unlock()
	writeReplayJSON(w, http.StatusOK, response)
}

func (s *frozenCouncilReplayServer) handleDo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeReplayJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": replayAPIError("method_not_allowed", "use POST")})
		return
	}
	var req replayDoRequest
	if err := decodeReplayJSONBody(w, r, &req, int64(s.input.Runtime.MaxResponseBytes)); err != nil {
		writeReplayJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": replayAPIError("bad_json", err.Error())})
		return
	}
	req.CaseID = strings.TrimSpace(req.CaseID)
	req.MemberID = strings.TrimSpace(req.MemberID)
	req.OpportunityID = strings.TrimSpace(req.OpportunityID)
	req.Tool = strings.TrimSpace(req.Tool)
	if req.Arguments == nil {
		req.Arguments = map[string]any{}
	}
	s.mu.Lock()
	result, err := s.callToolLocked(req)
	response := s.responseBaseLocked(req.CaseID, req.MemberID)
	call := CouncilReplayToolCall{
		Timestamp: utcNowForReplay(),
		Tool:      req.Tool,
		Arguments: cloneReplayMap(req.Arguments),
	}
	if err != nil {
		response["ok"] = false
		response["error"] = replayAPIError("tool_failed", err.Error())
		call.Error = err.Error()
	} else {
		response["ok"] = true
		response["result"] = result
		call.Result = cloneReplayMap(result)
	}
	s.toolCalls = append(s.toolCalls, call)
	s.mu.Unlock()
	writeReplayJSON(w, http.StatusOK, response)
}

func (s *frozenCouncilReplayServer) handleFail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeReplayJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": replayAPIError("method_not_allowed", "use POST")})
		return
	}
	var req struct {
		CaseID        string         `json:"case_id"`
		MemberID      string         `json:"member_id"`
		OpportunityID string         `json:"opportunity_id"`
		Reason        string         `json:"reason"`
		Message       string         `json:"message"`
		Details       map[string]any `json:"details"`
	}
	if err := decodeReplayJSONBody(w, r, &req, int64(s.input.Runtime.MaxResponseBytes)); err != nil {
		writeReplayJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": replayAPIError("bad_json", err.Error())})
		return
	}
	failure := councilReplayFailure{
		Reason:  strings.TrimSpace(req.Reason),
		Message: strings.TrimSpace(req.Message),
		Details: cloneReplayMap(req.Details),
	}
	if failure.Message == "" {
		failure.Message = "council replay agent failed"
	}
	s.mu.Lock()
	shouldSignal := s.failure == ""
	s.failure = failure.Message
	s.version++
	s.mu.Unlock()
	if shouldSignal {
		select {
		case s.failureDone <- failure:
		default:
		}
	}
	writeReplayJSON(w, http.StatusOK, map[string]any{"ok": true, "result": map[string]any{"text": "Council member failure recorded."}})
}

func (s *frozenCouncilReplayServer) parseIdentity(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	caseID := strings.TrimSpace(r.URL.Query().Get("case_id"))
	memberID := strings.TrimSpace(r.URL.Query().Get("member_id"))
	if caseID == "" {
		writeReplayJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": replayAPIError("missing_case_id", "case_id is required")})
		return "", "", false
	}
	if caseID != s.input.CaseID {
		writeReplayJSON(w, http.StatusNotFound, map[string]any{"ok": false, "case_id": caseID, "error": replayAPIError("unknown_case", "case_id does not match this replay")})
		return "", "", false
	}
	if memberID == "" {
		writeReplayJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "case_id": caseID, "error": replayAPIError("missing_member_id", "member_id is required")})
		return "", "", false
	}
	return caseID, memberID, true
}

func (s *frozenCouncilReplayServer) statusResponseLocked(caseID string, memberID string) map[string]any {
	response := s.responseBaseLocked(caseID, memberID)
	if s.failure != "" {
		response["status"] = "failed"
		response["prompt"] = ""
		response["tools"] = []map[string]any{}
		response["error"] = s.failure
		return response
	}
	if s.vote != nil {
		response["status"] = "done"
		response["prompt"] = ""
		response["tools"] = []map[string]any{}
		response["final_reason"] = "replay vote accepted"
		return response
	}
	if memberID != s.input.MemberID {
		response["status"] = "waiting"
		response["prompt"] = ""
		response["tools"] = []map[string]any{}
		return response
	}
	response["status"] = "ready"
	response["prompt"] = s.input.Prompt
	response["tools"] = s.input.Tools
	response["limits"] = s.replayLimitsLocked()
	return response
}

func (s *frozenCouncilReplayServer) responseBaseLocked(caseID string, memberID string) map[string]any {
	return map[string]any{
		"ok":        true,
		"case_id":   strings.TrimSpace(caseID),
		"member_id": strings.TrimSpace(memberID),
		"turn":      s.turnPayloadLocked(),
	}
}

func (s *frozenCouncilReplayServer) turnPayloadLocked() map[string]any {
	remaining := time.Until(s.deadline).Milliseconds()
	if remaining < 0 {
		remaining = 0
	}
	deliberationRound := any(1)
	if caseObj, ok := s.input.State["case"].(map[string]any); ok {
		if value, ok := caseObj["deliberation_round"]; ok {
			deliberationRound = value
		}
	}
	completed := s.vote != nil || s.failure != ""
	return map[string]any{
		"role_id":            "council",
		"member_id":          s.input.MemberID,
		"phase":              s.input.Opportunity.Phase,
		"opportunity_id":     s.input.Opportunity.ID,
		"turn_number":        s.input.TurnNumber,
		"deliberation_round": deliberationRound,
		"deadline":           s.deadline.UTC().Format(time.RFC3339Nano),
		"remaining_ms":       remaining,
		"attempts_max":       s.input.Runtime.InvalidAttemptLimit,
		"attempts_remaining": s.input.Runtime.InvalidAttemptLimit,
		"completed":          completed,
	}
}

func (s *frozenCouncilReplayServer) replayLimitsLocked() map[string]any {
	limits := cloneReplayMap(s.input.Limits)
	limits["remaining_evidence_reads_for_opportunity"] = remainingReplayCapacity(s.input.Policy.MaxEvidenceReadsPerOpportunity, s.evidenceBudget.reads)
	limits["remaining_evidence_read_bytes_for_opportunity"] = remainingReplayCapacity(s.input.Policy.MaxEvidenceReadBytesPerOpportunity, s.evidenceBudget.bytes)
	return limits
}

func (s *frozenCouncilReplayServer) callToolLocked(req replayDoRequest) (map[string]any, error) {
	if req.CaseID != s.input.CaseID {
		return nil, fmt.Errorf("case_id does not match this replay")
	}
	if req.MemberID != s.input.MemberID {
		return nil, fmt.Errorf("current council turn belongs to %s", s.input.MemberID)
	}
	if s.vote != nil {
		return nil, fmt.Errorf("council vote already submitted for this replay")
	}
	if req.OpportunityID != s.input.Opportunity.ID {
		return nil, fmt.Errorf("request opportunity_id %q does not match active opportunity_id %q", req.OpportunityID, s.input.Opportunity.ID)
	}
	switch req.Tool {
	case "get_case":
		return map[string]any{"case": s.input.CaseView}, nil
	case "list_evidence":
		return map[string]any{"evidence": s.input.Evidence}, nil
	case "stat_evidence":
		meta, err := s.statEvidenceLocked(mapStringLocal(req.Arguments["evidence_id"]))
		if err != nil {
			return nil, err
		}
		return map[string]any{"evidence": meta, "limits": s.evidenceReadLimitsLocked()}, nil
	case "read_evidence_range":
		offset, err := replayIntParam(req.Arguments, "offset")
		if err != nil {
			return nil, err
		}
		length, err := replayIntParam(req.Arguments, "length")
		if err != nil {
			return nil, err
		}
		return s.readEvidenceRangeLocked(mapStringLocal(req.Arguments["evidence_id"]), int64(offset), length)
	case "submit_council_vote":
		vote := strings.TrimSpace(mapStringLocal(req.Arguments["vote"]))
		rationale := strings.TrimSpace(mapStringLocal(req.Arguments["rationale"]))
		if vote != "demonstrated" && vote != "not_demonstrated" {
			return nil, fmt.Errorf("vote must be demonstrated or not_demonstrated")
		}
		if rationale == "" {
			return nil, fmt.Errorf("rationale is required")
		}
		accepted := councilReplayVote{Vote: vote, Rationale: rationale}
		s.vote = &accepted
		s.version++
		select {
		case s.voteDone <- accepted:
		default:
		}
		return map[string]any{"text": "Council vote accepted."}, nil
	default:
		return nil, fmt.Errorf("unknown tool %q", req.Tool)
	}
}

func (s *frozenCouncilReplayServer) evidenceReadLimitsLocked() map[string]any {
	return map[string]any{
		"max_read_bytes":                       s.input.Policy.MaxEvidenceReadBytes,
		"max_reads_per_opportunity":            s.input.Policy.MaxEvidenceReadsPerOpportunity,
		"max_read_bytes_per_opportunity":       s.input.Policy.MaxEvidenceReadBytesPerOpportunity,
		"remaining_read_bytes_for_opportunity": remainingReplayCapacity(s.input.Policy.MaxEvidenceReadBytesPerOpportunity, s.evidenceBudget.bytes),
		"remaining_reads_for_opportunity":      remainingReplayCapacity(s.input.Policy.MaxEvidenceReadsPerOpportunity, s.evidenceBudget.reads),
	}
}

func (s *frozenCouncilReplayServer) statEvidenceLocked(evidenceID string) (proceeding.EvidenceMeta, error) {
	evidenceID = strings.TrimSpace(evidenceID)
	if evidenceID == "" {
		return proceeding.EvidenceMeta{}, fmt.Errorf("evidence_id is required")
	}
	for _, meta := range s.input.Evidence {
		if meta.EvidenceID != evidenceID || meta.RecordVisibility == "system_private" {
			continue
		}
		path := filepath.Join(s.input.SourceOutputDir, "evidence-store", filepath.FromSlash(meta.StorageName))
		sha, size, err := replayFileHashAndSize(path)
		if err != nil {
			return proceeding.EvidenceMeta{}, err
		}
		if sha != meta.SHA256 {
			return proceeding.EvidenceMeta{}, fmt.Errorf("evidence %s sha256 mismatch", evidenceID)
		}
		if size != meta.SizeBytes {
			return proceeding.EvidenceMeta{}, fmt.Errorf("evidence %s size mismatch", evidenceID)
		}
		return meta, nil
	}
	return proceeding.EvidenceMeta{}, fmt.Errorf("unknown evidence %q", evidenceID)
}

func (s *frozenCouncilReplayServer) readEvidenceRangeLocked(evidenceID string, offset int64, length int) (map[string]any, error) {
	if offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative")
	}
	if length <= 0 {
		return nil, fmt.Errorf("length must be positive")
	}
	if length > s.input.Policy.MaxEvidenceReadBytes {
		return nil, fmt.Errorf("evidence_read_limit_exceeded: requested %d, max %d", length, s.input.Policy.MaxEvidenceReadBytes)
	}
	if s.evidenceBudget.reads >= s.input.Policy.MaxEvidenceReadsPerOpportunity {
		return nil, fmt.Errorf("evidence_read_limit_exceeded: read count limit %d", s.input.Policy.MaxEvidenceReadsPerOpportunity)
	}
	if s.evidenceBudget.bytes+length > s.input.Policy.MaxEvidenceReadBytesPerOpportunity {
		return nil, fmt.Errorf("evidence_read_limit_exceeded: opportunity byte budget %d", s.input.Policy.MaxEvidenceReadBytesPerOpportunity)
	}
	meta, err := s.statEvidenceLocked(evidenceID)
	if err != nil {
		return nil, err
	}
	if offset > int64(meta.SizeBytes) {
		return nil, fmt.Errorf("invalid_evidence_range: offset %d exceeds size %d", offset, meta.SizeBytes)
	}
	path := filepath.Join(s.input.SourceOutputDir, "evidence-store", filepath.FromSlash(meta.StorageName))
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read evidence %s: %w", evidenceID, err)
	}
	end := offset + int64(length)
	if end > int64(len(raw)) {
		end = int64(len(raw))
	}
	out := raw[offset:end]
	s.evidenceBudget.reads++
	s.evidenceBudget.bytes += len(out)
	return map[string]any{
		"evidence_id":                          meta.EvidenceID,
		"offset":                               offset,
		"length":                               len(out),
		"total_size_bytes":                     meta.SizeBytes,
		"sha256":                               meta.SHA256,
		"mime_type":                            meta.MimeType,
		"content_base64":                       base64.StdEncoding.EncodeToString(out),
		"remaining_read_bytes_for_opportunity": remainingReplayCapacity(s.input.Policy.MaxEvidenceReadBytesPerOpportunity, s.evidenceBudget.bytes),
		"remaining_reads_for_opportunity":      remainingReplayCapacity(s.input.Policy.MaxEvidenceReadsPerOpportunity, s.evidenceBudget.reads),
	}, nil
}

func replayFileHashAndSize(path string) (string, int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("read evidence store file %s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), len(raw), nil
}

func remainingReplayCapacity(limit int, used int) int {
	remaining := limit - used
	if remaining < 0 {
		return 0
	}
	return remaining
}

func replayIntParam(args map[string]any, key string) (int, error) {
	value, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if float64(int(v)) == v {
			return int(v), nil
		}
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return int(n), nil
		}
	}
	return 0, fmt.Errorf("%s must be an integer", key)
}

func decodeReplayJSONBody(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	body := http.MaxBytesReader(w, r.Body, limit)
	defer r.Body.Close()
	dec := json.NewDecoder(body)
	dec.UseNumber()
	if err := dec.Decode(target); err != nil {
		return err
	}
	return nil
}

func writeReplayJSON(w http.ResponseWriter, status int, value map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func replayAPIError(code string, message string) map[string]any {
	return map[string]any{"code": code, "message": message}
}

func cloneReplayMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mapStringLocal(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func utcNowForReplay() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
}
