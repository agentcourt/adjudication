package localrun

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"adjudication/arb/runtime/mcp"
	"adjudication/arb/runtime/proceeding"
	"adjudication/common/modelrequest"
)

const (
	defaultOpenClawImage          = "ghcr.io/openclaw/openclaw:latest"
	defaultOpenClawModel          = "gpt-5.5"
	defaultOpenClawThinking       = "low"
	defaultOpenClawTimeoutSeconds = 3600
	defaultPiImage                = "agentcourt-pi-sandbox"
	defaultPiMCPAdapter           = "npm:pi-mcp-adapter"
	defaultCaseStartupWait        = 30 * time.Second
	defaultCouncilRosterWait      = 2 * time.Minute
)

type Options struct {
	ComplaintPath              string
	CaseFiles                  []string
	OutputDir                  string
	PolicyPath                 string
	CouncilSize                int
	EvidenceStandard           string
	AttorneyInstructionsPath   string
	PromptDir                  string
	AttorneyCommonPromptPath   string
	AttorneyArgumentPromptPath string
	AttorneyRebuttalPromptPath string
	CommonRoot                 string
	CouncilPoolPath            string
	CaseAPIAddr                string
	MCPListenAddr              string
	MCPBearerToken             string
	CouncilTimeoutSeconds      int
	LawyerTimeoutSeconds       int
	MaxResponseBytes           int
	InvalidAttemptLimit        int
	EnginePath                 string
	RunID                      string
	CaseID                     string
	LawyerInstructionsPath     string
	CouncilInstructionsPath    string
	DockerCommand              string
	PodmanCommand              string
	OpenClawImage              string
	OpenClawModel              string
	OpenClawThinking           string
	OpenClawTimeoutSeconds     int
	PiImage                    string
	PiMCPAdapter               string
	DockerMCPHost              string
	PodmanMCPHost              string
	Log                        io.Writer
}

type instructionData struct {
	CaseID    string
	RoleID    string
	MemberID  string
	MCPServer string
	MCPURL    string
}

type processRecord struct {
	name     string
	kind     string
	command  *exec.Cmd
	done     chan error
	stopName string

	mu     sync.Mutex
	exited bool
}

type councilRosterResponse struct {
	CouncilRoster []councilRosterEntry `json:"council_roster"`
}

type councilRosterEntry struct {
	MemberID    string             `json:"member_id"`
	Model       string             `json:"model"`
	RequestSpec *modelrequest.Spec `json:"request_spec"`
}

type runState struct {
	opts      Options
	logDir    string
	caseBase  string
	mcpBase   string
	token     string
	processes []*processRecord
	agentErrs chan error

	mu sync.Mutex
}

func Run(ctx context.Context, opts Options) (result proceeding.Result, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = applyDefaults(opts)
	if err := validateOptions(opts); err != nil {
		return proceeding.Result{}, err
	}
	if err := os.MkdirAll(filepath.Join(opts.OutputDir, "logs"), 0o755); err != nil {
		return proceeding.Result{}, fmt.Errorf("create output logs: %w", err)
	}
	state := &runState{
		opts:      opts,
		logDir:    filepath.Join(opts.OutputDir, "logs"),
		token:     strings.TrimSpace(opts.MCPBearerToken),
		agentErrs: make(chan error, 32),
	}
	if state.token == "" {
		token, err := randomToken()
		if err != nil {
			return proceeding.Result{}, err
		}
		state.token = token
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		err = errors.Join(err, state.stopAgents())
	}()

	caseAPIAddr, err := resolveListenAddr(opts.CaseAPIAddr, "127.0.0.1")
	if err != nil {
		return proceeding.Result{}, fmt.Errorf("resolve case API address: %w", err)
	}
	state.caseBase = "http://" + caseAPIAddr
	mcpListenAddr, err := resolveListenAddr(opts.MCPListenAddr, "0.0.0.0")
	if err != nil {
		return proceeding.Result{}, fmt.Errorf("resolve MCP listen address: %w", err)
	}
	_, mcpPort, err := net.SplitHostPort(mcpListenAddr)
	if err != nil {
		return proceeding.Result{}, fmt.Errorf("parse MCP listen address %q: %w", mcpListenAddr, err)
	}
	state.mcpBase = "http://" + net.JoinHostPort("127.0.0.1", mcpPort)

	caseDone := make(chan caseOutcome, 1)
	go func() {
		result, err := proceeding.Run(runCtx, proceeding.Options{
			ComplaintPath:              opts.ComplaintPath,
			CaseFiles:                  opts.CaseFiles,
			OutputDir:                  opts.OutputDir,
			PolicyPath:                 opts.PolicyPath,
			CouncilSize:                opts.CouncilSize,
			EvidenceStandard:           opts.EvidenceStandard,
			AttorneyInstructionsPath:   opts.AttorneyInstructionsPath,
			PromptDir:                  opts.PromptDir,
			AttorneyCommonPromptPath:   opts.AttorneyCommonPromptPath,
			AttorneyArgumentPromptPath: opts.AttorneyArgumentPromptPath,
			AttorneyRebuttalPromptPath: opts.AttorneyRebuttalPromptPath,
			CommonRoot:                 opts.CommonRoot,
			CouncilPoolPath:            opts.CouncilPoolPath,
			CaseAPIAddr:                caseAPIAddr,
			CouncilBackend:             "councilapi",
			CouncilTimeoutSeconds:      opts.CouncilTimeoutSeconds,
			LawyerTimeoutSeconds:       opts.LawyerTimeoutSeconds,
			MaxResponseBytes:           opts.MaxResponseBytes,
			InvalidAttemptLimit:        opts.InvalidAttemptLimit,
			EnginePath:                 opts.EnginePath,
			RunID:                      opts.RunID,
			CaseID:                     opts.CaseID,
		})
		caseDone <- caseOutcome{result: result, err: err}
	}()
	if err := state.waitForCaseAPI(runCtx, caseDone); err != nil {
		cancel()
		return proceeding.Result{}, err
	}

	mcpDone := make(chan error, 1)
	go func() {
		logFile, err := os.Create(filepath.Join(state.logDir, "mcp.stderr"))
		if err != nil {
			mcpDone <- fmt.Errorf("create MCP log: %w", err)
			return
		}
		defer logFile.Close()
		mcpDone <- mcp.Run(runCtx, mcp.Options{
			ListenAddr:           mcpListenAddr,
			CaseAPIBase:          state.caseBase,
			BearerToken:          state.token,
			APIBearerToken:       "",
			DisableSessionExpiry: true,
			Log:                  logFile,
		})
	}()
	if err := state.waitForMCP(runCtx, caseDone, mcpDone); err != nil {
		cancel()
		return proceeding.Result{}, err
	}

	if err := state.startOpenClawLawyer(runCtx, "plaintiff", mcpPort); err != nil {
		cancel()
		return proceeding.Result{}, err
	}
	if err := state.startOpenClawLawyer(runCtx, "defendant", mcpPort); err != nil {
		cancel()
		return proceeding.Result{}, err
	}
	roster, err := state.waitForCouncilRoster(runCtx, caseDone, mcpDone)
	if err != nil {
		cancel()
		return proceeding.Result{}, err
	}
	for _, entry := range roster {
		if err := state.startPiCouncil(runCtx, entry, mcpPort); err != nil {
			cancel()
			return proceeding.Result{}, err
		}
	}

	select {
	case outcome := <-caseDone:
		cancel()
		if err := <-mcpDone; err != nil && !errors.Is(err, context.Canceled) {
			return outcome.result, err
		}
		if writeErr := writeRunSummary(opts.OutputDir, outcome.result); writeErr != nil {
			return outcome.result, writeErr
		}
		return outcome.result, outcome.err
	case err := <-mcpDone:
		cancel()
		if err == nil {
			return proceeding.Result{}, fmt.Errorf("MCP server exited before case completion")
		}
		return proceeding.Result{}, fmt.Errorf("MCP server failed: %w", err)
	case exit := <-state.agentErrs:
		cancel()
		return proceeding.Result{}, exit
	case <-ctx.Done():
		cancel()
		return proceeding.Result{}, ctx.Err()
	}
}

type caseOutcome struct {
	result proceeding.Result
	err    error
}

type rosterOutcome struct {
	roster []councilRosterEntry
	err    error
}

func applyDefaults(opts Options) Options {
	if strings.TrimSpace(opts.DockerCommand) == "" {
		opts.DockerCommand = "docker"
	}
	if strings.TrimSpace(opts.PodmanCommand) == "" {
		opts.PodmanCommand = "podman"
	}
	if strings.TrimSpace(opts.OpenClawImage) == "" {
		opts.OpenClawImage = defaultOpenClawImage
	}
	if strings.TrimSpace(opts.OpenClawModel) == "" {
		opts.OpenClawModel = defaultOpenClawModel
	}
	if strings.TrimSpace(opts.OpenClawThinking) == "" {
		opts.OpenClawThinking = defaultOpenClawThinking
	}
	if opts.OpenClawTimeoutSeconds <= 0 {
		opts.OpenClawTimeoutSeconds = defaultOpenClawTimeoutSeconds
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
	if strings.TrimSpace(opts.DockerMCPHost) == "" {
		opts.DockerMCPHost = "host.docker.internal"
	}
	if strings.TrimSpace(opts.PodmanMCPHost) == "" {
		opts.PodmanMCPHost = "127.0.0.1"
	}
	if strings.TrimSpace(opts.LawyerInstructionsPath) == "" {
		opts.LawyerInstructionsPath = filepath.Join("agent-instructions", "openclaw-lawyer.md.tmpl")
	}
	if strings.TrimSpace(opts.CouncilInstructionsPath) == "" {
		opts.CouncilInstructionsPath = filepath.Join("agent-instructions", "pi-council.md.tmpl")
	}
	return opts
}

func validateOptions(opts Options) error {
	if strings.TrimSpace(opts.ComplaintPath) == "" {
		return fmt.Errorf("complaint path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return fmt.Errorf("output dir is required")
	}
	if strings.TrimSpace(opts.CaseID) == "" {
		return fmt.Errorf("case id is required")
	}
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		return fmt.Errorf("OPENAI_API_KEY is required for OpenClaw lawyers")
	}
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		return fmt.Errorf("OPENROUTER_API_KEY is required for Pi council")
	}
	for _, path := range []string{opts.LawyerInstructionsPath, opts.CouncilInstructionsPath} {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("stat instruction template %s: %w", path, err)
		}
	}
	return nil
}

func resolveListenAddr(value string, defaultHost string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" && !strings.HasSuffix(value, ":0") {
		return value, nil
	}
	host := defaultHost
	if value != "" {
		parsedHost, _, err := net.SplitHostPort(value)
		if err == nil && parsedHost != "" {
			host = parsedHost
		}
	}
	probeHost := host
	if probeHost == "0.0.0.0" || probeHost == "::" || probeHost == "" {
		probeHost = "127.0.0.1"
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(probeHost, "0"))
	if err != nil {
		return "", err
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	closeErr := ln.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return net.JoinHostPort(host, port), nil
}

func waitForHealth(ctx context.Context, rawURL string, timeout time.Duration) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			closeErr := resp.Body.Close()
			if closeErr != nil {
				return closeErr
			}
			if resp.StatusCode == http.StatusNoContent {
				return nil
			}
		}
		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("%s did not become healthy within %s", rawURL, timeout)
		case <-ticker.C:
		}
	}
}

func (s *runState) waitForCaseAPI(ctx context.Context, caseDone <-chan caseOutcome) error {
	healthDone := make(chan error, 1)
	go func() {
		healthDone <- waitForHealth(ctx, s.caseBase+"/health", defaultCaseStartupWait)
	}()
	select {
	case err := <-healthDone:
		return err
	case outcome := <-caseDone:
		if outcome.err != nil {
			return outcome.err
		}
		return fmt.Errorf("case finished before case API became healthy")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *runState) waitForMCP(ctx context.Context, caseDone <-chan caseOutcome, mcpDone <-chan error) error {
	healthDone := make(chan error, 1)
	go func() {
		healthDone <- waitForHealth(ctx, s.mcpBase+"/health", defaultCaseStartupWait)
	}()
	select {
	case err := <-healthDone:
		return err
	case outcome := <-caseDone:
		if outcome.err != nil {
			return outcome.err
		}
		return fmt.Errorf("case finished before MCP became healthy")
	case err := <-mcpDone:
		if err == nil {
			return fmt.Errorf("MCP server exited before health check")
		}
		return fmt.Errorf("MCP server failed before health check: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *runState) startOpenClawLawyer(ctx context.Context, role string, mcpPort string) error {
	server := "aar-" + s.opts.CaseID + "-" + role
	mcpURL := "http://" + net.JoinHostPort(s.opts.DockerMCPHost, mcpPort) + "/mcp?case_id=" + url.QueryEscape(s.opts.CaseID) + "&role_id=" + url.QueryEscape(role)
	instructions, err := renderInstructions(s.opts.LawyerInstructionsPath, instructionData{
		CaseID:    s.opts.CaseID,
		RoleID:    role,
		MCPServer: server,
		MCPURL:    mcpURL,
	})
	if err != nil {
		return err
	}
	mcpJSON, err := json.Marshal(map[string]any{
		"url":       mcpURL,
		"transport": "streamable-http",
		"headers":   map[string]string{"Authorization": "Bearer " + s.token},
	})
	if err != nil {
		return err
	}
	name := containerName("aar-" + s.opts.CaseID + "-" + role)
	args := []string{
		"run", "--rm",
		"--name", name,
		"--add-host=host.docker.internal:host-gateway",
		"-e", "OPENAI_API_KEY",
		"-e", "AAR_MCP_NAME=" + server,
		"-e", "AAR_MCP_JSON=" + string(mcpJSON),
		"-e", "AAR_SESSION_KEY=agent:aar:" + s.opts.CaseID + ":" + role,
		"-e", "AAR_ASSIGNMENT=" + instructions,
		"-e", "AAR_PRINCIPAL=" + role,
		s.opts.OpenClawImage,
		"sh", "-lc",
		fmt.Sprintf("set -u\nopenclaw mcp set \"$AAR_MCP_NAME\" \"$AAR_MCP_JSON\"\nexec openclaw agent --local --model %q --thinking %q --timeout %d --session-key \"$AAR_SESSION_KEY\" --message \"$AAR_ASSIGNMENT\" --json", s.opts.OpenClawModel, s.opts.OpenClawThinking, s.opts.OpenClawTimeoutSeconds),
	}
	proc, err := s.startProcess(ctx, "openclaw-"+role, "docker", s.opts.DockerCommand, args, name)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.processes = append(s.processes, proc)
	s.mu.Unlock()
	return nil
}

func (s *runState) waitForCouncilRoster(ctx context.Context, caseDone <-chan caseOutcome, mcpDone <-chan error) ([]councilRosterEntry, error) {
	rosterDone := make(chan rosterOutcome, 1)
	go func() {
		roster, err := s.pollCouncilRoster(ctx)
		rosterDone <- rosterOutcome{roster: roster, err: err}
	}()
	select {
	case outcome := <-rosterDone:
		return outcome.roster, outcome.err
	case outcome := <-caseDone:
		if outcome.err != nil {
			return nil, outcome.err
		}
		return nil, fmt.Errorf("case finished before council roster became available")
	case err := <-mcpDone:
		if err == nil {
			return nil, fmt.Errorf("MCP server exited before council roster became available")
		}
		return nil, fmt.Errorf("MCP server failed before council roster became available: %w", err)
	case err := <-s.agentErrs:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *runState) pollCouncilRoster(ctx context.Context) ([]councilRosterEntry, error) {
	deadlineCtx, cancel := context.WithTimeout(ctx, defaultCouncilRosterWait)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	statusURL := s.caseBase + "/lawyerapi/v1/status?case_id=" + url.QueryEscape(s.opts.CaseID) + "&role_id=observer"
	for {
		req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, statusURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var status councilRosterResponse
				dec := json.NewDecoder(bytes.NewReader(body))
				dec.UseNumber()
				if err := dec.Decode(&status); err != nil {
					return nil, err
				}
				if len(status.CouncilRoster) > 0 {
					if err := os.WriteFile(filepath.Join(s.logDir, "observer-status.json"), body, 0o644); err != nil {
						return nil, err
					}
					return status.CouncilRoster, nil
				}
			}
		}
		select {
		case <-deadlineCtx.Done():
			return nil, fmt.Errorf("council roster did not become available within %s", defaultCouncilRosterWait)
		case <-ticker.C:
		}
	}
}

func (s *runState) startPiCouncil(ctx context.Context, entry councilRosterEntry, mcpPort string) error {
	if strings.TrimSpace(entry.MemberID) == "" {
		return fmt.Errorf("council roster entry has empty member_id")
	}
	server := "aar-" + s.opts.CaseID + "-" + entry.MemberID
	mcpURL := "http://" + net.JoinHostPort(s.opts.PodmanMCPHost, mcpPort) + "/mcp?case_id=" + url.QueryEscape(s.opts.CaseID) + "&member_id=" + url.QueryEscape(entry.MemberID)
	instructions, err := renderInstructions(s.opts.CouncilInstructionsPath, instructionData{
		CaseID:    s.opts.CaseID,
		MemberID:  entry.MemberID,
		MCPServer: server,
		MCPURL:    mcpURL,
	})
	if err != nil {
		return err
	}
	home := filepath.Join(s.opts.OutputDir, "pi-"+entry.MemberID)
	if err := os.MkdirAll(home, 0o755); err != nil {
		return fmt.Errorf("create Pi home: %w", err)
	}
	model, err := writePiConfig(home, entry, server, mcpURL, s.token)
	if err != nil {
		return err
	}
	args := []string{
		"run", "--rm",
		"--network", "host",
		"--user", "0:0",
		"-e", "HOME=/home/user",
		"-e", "TMPDIR=/home/user",
		"-e", "PI_CODING_AGENT_DIR=/home/user/.pi/agent",
		"-e", "OPENROUTER_API_KEY",
		"-e", "NODE_OPTIONS",
		"-v", home + ":/home/user",
		"-w", "/home/user",
		s.opts.PiImage,
		"--provider", "openrouter",
		"--model", model,
		"-e", s.opts.PiMCPAdapter,
		"--mode", "json",
		"-p", instructions,
	}
	proc, err := s.startProcess(ctx, "pi-"+entry.MemberID, "podman", s.opts.PodmanCommand, args, "")
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.processes = append(s.processes, proc)
	s.mu.Unlock()
	return nil
}

func renderInstructions(path string, data instructionData) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read instruction template %s: %w", path, err)
	}
	tmpl, err := template.New(filepath.Base(path)).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse instruction template %s: %w", path, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render instruction template %s: %w", path, err)
	}
	return out.String(), nil
}

func writePiConfig(home string, entry councilRosterEntry, server string, mcpURL string, token string) (string, error) {
	spec, err := piRequestSpec(entry)
	if err != nil {
		return "", err
	}
	if spec.Endpoint != "openrouter" {
		return "", fmt.Errorf("Pi council requires openrouter endpoint for %s; got %s", entry.MemberID, spec.Endpoint)
	}
	unsupported := []string{}
	if spec.Request.Temperature != nil {
		unsupported = append(unsupported, "temperature")
	}
	if spec.Request.TopP != nil {
		unsupported = append(unsupported, "top_p")
	}
	if len(unsupported) > 0 {
		return "", fmt.Errorf("Pi council cannot enforce request fields for %s: %s", entry.MemberID, strings.Join(unsupported, " "))
	}
	model := spec.UpstreamModel()
	settingsDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return "", fmt.Errorf("create Pi settings dir: %w", err)
	}
	settings := map[string]any{
		"defaultProvider": "openrouter",
		"defaultModel":    model,
		"quietStartup":    true,
	}
	if err := writeJSONFile(filepath.Join(settingsDir, "settings.json"), settings); err != nil {
		return "", err
	}
	modelEntry := map[string]any{
		"id":   model,
		"name": "AAR " + entry.MemberID + " " + model,
	}
	if maxTokens := spec.MaxOutputTokens(); maxTokens != nil {
		modelEntry["maxTokens"] = *maxTokens
	}
	if routing := spec.ProviderBody(); len(routing) > 0 {
		modelEntry["compat"] = map[string]any{"openRouterRouting": routing}
	}
	models := map[string]any{
		"providers": map[string]any{
			"openrouter": map[string]any{
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKey":  "$OPENROUTER_API_KEY",
				"api":     "openai-completions",
				"models":  []map[string]any{modelEntry},
			},
		},
	}
	if err := writeJSONFile(filepath.Join(settingsDir, "models.json"), models); err != nil {
		return "", err
	}
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			server: map[string]any{
				"url":       mcpURL,
				"transport": "streamable-http",
				"lifecycle": "keep-alive",
				"headers":   map[string]string{"Authorization": "Bearer " + token},
			},
		},
	}
	if err := writeJSONFile(filepath.Join(home, ".mcp.json"), mcpConfig); err != nil {
		return "", err
	}
	return model, nil
}

func piRequestSpec(entry councilRosterEntry) (modelrequest.Spec, error) {
	if entry.RequestSpec != nil {
		return *entry.RequestSpec, nil
	}
	return modelrequest.ParseLegacy(entry.Model)
}

func writeJSONFile(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (s *runState) startProcess(ctx context.Context, name string, kind string, command string, args []string, stopName string) (*processRecord, error) {
	stdout, err := os.Create(filepath.Join(s.logDir, name+".stdout"))
	if err != nil {
		return nil, fmt.Errorf("create %s stdout log: %w", name, err)
	}
	stderr, err := os.Create(filepath.Join(s.logDir, name+".stderr"))
	if err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("create %s stderr log: %w", name, err)
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(fmt.Errorf("start %s: %w", name, err), stdout.Close(), stderr.Close())
	}
	if err := os.WriteFile(filepath.Join(s.opts.OutputDir, name+".pid"), []byte(fmt.Sprintf("%d\n", cmd.Process.Pid)), 0o644); err != nil {
		return nil, errors.Join(err, cmd.Process.Kill(), cmd.Wait(), stdout.Close(), stderr.Close())
	}
	record := &processRecord{name: name, kind: kind, command: cmd, done: make(chan error, 1), stopName: stopName}
	go func() {
		err := cmd.Wait()
		closeOut := stdout.Close()
		closeErr := stderr.Close()
		waitErr := errors.Join(err, closeOut, closeErr)
		record.mu.Lock()
		record.exited = true
		record.mu.Unlock()
		record.done <- waitErr
		if waitErr != nil {
			s.agentErrs <- fmt.Errorf("%s process %s failed: %w", kind, name, waitErr)
			return
		}
		s.agentErrs <- fmt.Errorf("%s process %s exited before case completion", kind, name)
	}()
	return record, nil
}

func (s *runState) stopAgents() error {
	s.mu.Lock()
	processes := append([]*processRecord{}, s.processes...)
	s.mu.Unlock()
	var errs []error
	for _, proc := range processes {
		proc.mu.Lock()
		exited := proc.exited
		proc.mu.Unlock()
		if exited || proc.command.Process == nil {
			continue
		}
		if proc.kind == "docker" && strings.TrimSpace(proc.stopName) != "" {
			if err := exec.Command(s.opts.DockerCommand, "stop", proc.stopName).Run(); err != nil {
				errs = append(errs, fmt.Errorf("docker stop %s: %w", proc.stopName, err))
			}
			continue
		}
		if err := proc.command.Process.Kill(); err != nil {
			errs = append(errs, fmt.Errorf("kill %s: %w", proc.name, err))
		}
	}
	return errors.Join(errs...)
}

func writeRunSummary(outDir string, result proceeding.Result) error {
	return writeJSONFile(filepath.Join(outDir, "local-run.json"), map[string]any{
		"case_id":    result.CaseID,
		"run_id":     result.RunID,
		"status":     result.Status,
		"resolution": result.Resolution,
		"error":      result.Error,
		"failure":    result.Failure,
	})
}

func randomToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "aar-" + hex.EncodeToString(buf[:]), nil
}

func containerName(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_.")
	if len(out) > 63 {
		out = out[:63]
		out = strings.Trim(out, "-_.")
	}
	if out == "" {
		return "aar"
	}
	return out
}
