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
	"sync/atomic"
	"syscall"
	"text/template"
	"time"

	"adjudication/adc/runtime/lean"
	"adjudication/adc/runtime/mcp"
	"adjudication/adc/runtime/report"
	"adjudication/adc/runtime/runner"
	"adjudication/adc/runtime/store"
	"adjudication/common/modelrequest"
	"adjudication/common/openai"
)

const (
	defaultOpenClawImage          = "ghcr.io/openclaw/openclaw:latest"
	defaultOpenClawModel          = "gpt-5.5"
	defaultOpenClawThinking       = "low"
	defaultOpenClawTimeoutSeconds = 3600
	defaultOpenClawAuth           = "auto"
	defaultOpenClawStartDelay     = 15
	defaultPiImage                = "agentcourt-pi-sandbox"
	defaultPiMCPAdapter           = "npm:pi-mcp-adapter"
	defaultPiMCPServer            = "adc"
	defaultCaseAPIStartupWait     = 10 * time.Minute
	defaultMCPStartupWait         = 30 * time.Second
	defaultJurorOutputCheck       = 5 * time.Second
	defaultPiMaxOutputTokens      = 4096
	jurorFailureAgentExited       = "agent_exited"
	jurorFailureOutputLimit       = "agent_output_limit_exceeded"
	openClawCodexContainerHome    = "/adc-codex"
)

const (
	DefaultRunJurorTimeoutSeconds  = 15 * 60
	DefaultRunLawyerTimeoutSeconds = DefaultRunJurorTimeoutSeconds
	DefaultJurorOutputLimitBytes   = 128 * 1024 * 1024
	DefaultAutoLawyers             = "both"
	DefaultDockerCommand           = "docker"
	DefaultPodmanCommand           = "podman"
)

func DefaultLawyerInstructionsPath() string {
	return resolveADCDefaultPath("agent-instructions", "openclaw-lawyer.md.tmpl")
}

func DefaultRemoteLawyerSkillPath() string {
	return resolveADCDefaultPath("agent-instructions", "openclaw-remote-lawyer-skill.md.tmpl")
}

func DefaultJurorInstructionsPath() string {
	return resolveADCDefaultPath("agent-instructions", "pi-juror.md.tmpl")
}

func resolveADCDefaultPath(parts ...string) string {
	rel := filepath.Join(parts...)
	cwd, err := os.Getwd()
	if err != nil {
		return rel
	}
	for {
		for _, candidate := range []string{
			filepath.Join(cwd, rel),
			filepath.Join(cwd, "adc", rel),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return rel
		}
		cwd = parent
	}
}

type Options struct {
	ScenarioPath              string
	OutputDir                 string
	Model                     string
	DigestModel               string
	Temperature               *float64
	JurorTemperature          *float64
	JurorPersonasPath         string
	Online                    bool
	Offline                   bool
	CaseAPIAddr               string
	MCPListenAddr             string
	MCPBearerToken            string
	JurorTimeoutSeconds       int
	LawyerTimeoutSeconds      int
	TimeoutSeconds            int
	MaxResponseBytes          int
	InvalidAttemptLimit       int
	EnginePath                string
	RunID                     string
	CaseID                    string
	LawyerInstructionsPath    string
	RemoteLawyerSkillPath     string
	JurorInstructionsPath     string
	AutoLawyers               string
	MCPPublicBaseURL          string
	DockerCommand             string
	PodmanCommand             string
	OpenClawImage             string
	OpenClawModel             string
	OpenClawThinking          string
	OpenClawTimeoutSeconds    int
	OpenClawAuth              string
	OpenClawCodexAuthPath     string
	OpenClawStartDelaySeconds int
	PiImage                   string
	PiMCPAdapter              string
	JurorOutputLimitBytes     int64
	DockerMCPHost             string
	PodmanMCPHost             string
	Log                       io.Writer
}

type instructionData struct {
	CaseID      string
	RoleID      string
	PrincipalID string
	MCPServer   string
	MCPURL      string
	MCPJSON     string
}

type processRecord struct {
	name     string
	kind     string
	command  *exec.Cmd
	done     chan error
	stopName string

	stdoutPath string
	stderrPath string
	finished   chan struct{}

	stdoutCounter *processOutputCounter

	mu            sync.Mutex
	exited        bool
	forcedReason  string
	forcedMessage string
	forcedDetails map[string]any
}

type jurorProcessTarget struct {
	principalID   string
	opportunityID string
}

type activeJurorOpportunity struct {
	principalID   string
	opportunityID string
	requestSpec   *modelrequest.Spec
}

type processOutputSize struct {
	Stdout int64
	Stderr int64
	Total  int64
}

type processOutputCounter struct {
	dst   io.Writer
	count atomic.Int64
}

func newProcessOutputCounter(dst io.Writer) *processOutputCounter {
	return &processOutputCounter{dst: dst}
}

func (w *processOutputCounter) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	if n > 0 {
		w.count.Add(int64(n))
	}
	return n, err
}

func (w *processOutputCounter) Size() int64 {
	return w.count.Load()
}

type runState struct {
	opts             Options
	logDir           string
	caseBase         string
	mcpBase          string
	mcpPublicBase    string
	token            string
	openClawAuth     openClawAuthConfig
	processes        []*processRecord
	secretDirs       []string
	jurorProcesses   map[string]*processRecord
	failedJurorTurns map[string]bool
	agentErrs        chan error

	mu sync.Mutex
}

type openClawAuthConfig struct {
	Mode          string
	CodexAuthPath string
}

type caseOutcome struct {
	result runner.Result
	err    error
}

func Run(ctx context.Context, opts Options) (result runner.Result, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = applyDefaults(opts)
	if err := validateOptions(opts); err != nil {
		return runner.Result{}, err
	}
	openClawAuth, err := resolveOpenClawAuth(opts)
	if err != nil {
		return runner.Result{}, err
	}
	if err := os.MkdirAll(filepath.Join(opts.OutputDir, "logs"), 0o755); err != nil {
		return runner.Result{}, fmt.Errorf("create output logs: %w", err)
	}
	state := &runState{
		opts:             opts,
		logDir:           filepath.Join(opts.OutputDir, "logs"),
		token:            strings.TrimSpace(opts.MCPBearerToken),
		openClawAuth:     openClawAuth,
		jurorProcesses:   map[string]*processRecord{},
		failedJurorTurns: map[string]bool{},
		agentErrs:        make(chan error, 32),
	}
	if state.token == "" {
		token, err := randomToken()
		if err != nil {
			return runner.Result{}, err
		}
		state.token = token
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		err = errors.Join(err, state.stopAgents(), state.cleanupSecrets())
	}()

	caseAPIAddr, err := resolveListenAddr(opts.CaseAPIAddr, "127.0.0.1")
	if err != nil {
		return runner.Result{}, fmt.Errorf("resolve case API address: %w", err)
	}
	state.caseBase = "http://" + caseAPIAddr
	mcpListenAddr, err := resolveListenAddr(opts.MCPListenAddr, "0.0.0.0")
	if err != nil {
		return runner.Result{}, fmt.Errorf("resolve MCP listen address: %w", err)
	}
	state.mcpPublicBase, err = publicMCPBase(opts.MCPPublicBaseURL, mcpListenAddr)
	if err != nil {
		return runner.Result{}, err
	}
	if len(manualLawyerRoles(opts.AutoLawyers)) > 0 {
		if err := validateManualLawyerAddress(opts.MCPPublicBaseURL, mcpListenAddr); err != nil {
			return runner.Result{}, err
		}
	}
	_, mcpPort, err := net.SplitHostPort(mcpListenAddr)
	if err != nil {
		return runner.Result{}, fmt.Errorf("parse MCP listen address %q: %w", mcpListenAddr, err)
	}
	state.mcpBase = "http://" + net.JoinHostPort("127.0.0.1", mcpPort)

	caseDone := make(chan caseOutcome, 1)
	go func() {
		result, err := runScenarioCase(runCtx, opts, caseAPIAddr)
		caseDone <- caseOutcome{result: result, err: err}
	}()
	if err := state.waitForCaseAPI(runCtx, caseDone); err != nil {
		cancel()
		return runner.Result{}, err
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
			DisableSessionExpiry: true,
			Log:                  logFile,
		})
	}()
	if err := state.waitForMCP(runCtx, caseDone, mcpDone); err != nil {
		cancel()
		return runner.Result{}, err
	}

	for _, role := range manualLawyerRoles(opts.AutoLawyers) {
		if err := state.writeRemoteLawyerSkill(role); err != nil {
			cancel()
			return runner.Result{}, err
		}
	}
	startedPlaintiff := false
	if autoLawyerEnabled(opts.AutoLawyers, "plaintiff") {
		if err := state.startOpenClawLawyer(runCtx, "plaintiff", mcpPort); err != nil {
			cancel()
			return runner.Result{}, err
		}
		startedPlaintiff = true
	}
	if autoLawyerEnabled(opts.AutoLawyers, "defendant") {
		if startedPlaintiff {
			if err := state.waitOpenClawStartDelay(runCtx); err != nil {
				cancel()
				return runner.Result{}, err
			}
		}
		if err := state.startOpenClawLawyer(runCtx, "defendant", mcpPort); err != nil {
			cancel()
			return runner.Result{}, err
		}
	}

	jurorTicker := time.NewTicker(time.Second)
	defer jurorTicker.Stop()
	if err := state.startActiveJuror(runCtx, mcpPort); err != nil {
		cancel()
		return runner.Result{}, err
	}
	finishCase := func(outcome caseOutcome) (runner.Result, error) {
		cancel()
		if err := <-mcpDone; err != nil && !errors.Is(err, context.Canceled) {
			return outcome.result, err
		}
		if writeErr := writeRunSummary(opts.OutputDir, outcome.result, opts); writeErr != nil {
			return outcome.result, writeErr
		}
		return outcome.result, outcome.err
	}
	for {
		select {
		case outcome := <-caseDone:
			return finishCase(outcome)
		default:
		}
		select {
		case outcome := <-caseDone:
			return finishCase(outcome)
		case err := <-mcpDone:
			cancel()
			if err == nil {
				return runner.Result{}, fmt.Errorf("MCP server exited before case completion")
			}
			return runner.Result{}, fmt.Errorf("MCP server failed: %w", err)
		case exit := <-state.agentErrs:
			cancel()
			return runner.Result{}, exit
		case <-jurorTicker.C:
			if err := state.startActiveJuror(runCtx, mcpPort); err != nil {
				if isConnectionRefused(err) {
					select {
					case outcome := <-caseDone:
						return finishCase(outcome)
					case <-ctx.Done():
						cancel()
						return runner.Result{}, ctx.Err()
					}
				}
				select {
				case outcome := <-caseDone:
					return finishCase(outcome)
				default:
				}
				cancel()
				return runner.Result{}, err
			}
		case <-ctx.Done():
			cancel()
			return runner.Result{}, ctx.Err()
		}
	}
}

func runScenarioCase(ctx context.Context, opts Options, caseAPIAddr string) (runner.Result, error) {
	runtimeLimits := runner.RuntimeLimits{
		LLMTimeoutSeconds:     opts.TimeoutSeconds,
		RoleAPITimeoutSeconds: opts.LawyerTimeoutSeconds,
		MaxResponseBytes:      opts.MaxResponseBytes,
		InvalidAttemptLimit:   opts.InvalidAttemptLimit,
	}.Normalized()
	if opts.JurorTimeoutSeconds > runtimeLimits.RoleAPITimeoutSeconds {
		runtimeLimits.RoleAPITimeoutSeconds = opts.JurorTimeoutSeconds
	}
	if err := writeJSONFile(filepath.Join(opts.OutputDir, "runtime.json"), runtimeLimits); err != nil {
		return runner.Result{}, err
	}

	st, err := store.Open(filepath.Join(opts.OutputDir, "run.db"))
	if err != nil {
		return runner.Result{}, err
	}
	defer st.Close()

	var client *openai.Client
	var jurorClient *openai.Client
	if !opts.Offline {
		client, err = openai.NewFromEnv(opts.Online, time.Duration(runtimeLimits.LLMTimeoutSeconds)*time.Second)
		if err != nil {
			return runner.Result{}, err
		}
		if strings.TrimSpace(opts.JurorPersonasPath) != "" {
			jurorClient, err = openai.NewFromEnv(opts.Online, time.Duration(runtimeLimits.LLMTimeoutSeconds)*time.Second)
			if err != nil {
				return runner.Result{}, err
			}
		}
	}

	r, err := runner.New(st, lean.New(strings.Fields(strings.TrimSpace(opts.EnginePath))), client, jurorClient, runner.Config{
		ScenarioPath:      opts.ScenarioPath,
		OutputPath:        filepath.Join(opts.OutputDir, "run.json"),
		EventsPath:        filepath.Join(opts.OutputDir, "events.ndjson"),
		RunID:             opts.RunID,
		CaseID:            opts.CaseID,
		CaseAPIAddr:       caseAPIAddr,
		ExternalRoles:     []string{"plaintiff", "defendant", "juror"},
		Model:             strings.TrimSpace(opts.Model),
		Temperature:       opts.Temperature,
		JurorTemperature:  opts.JurorTemperature,
		JurorPersonasPath: strings.TrimSpace(opts.JurorPersonasPath),
		Offline:           opts.Offline,
		Runtime:           runtimeLimits,
	})
	if err != nil {
		return runner.Result{}, err
	}
	result, err := r.Run(ctx)
	if err != nil {
		return result, err
	}
	if err := report.WriteTranscript(filepath.Join(opts.OutputDir, "transcript.md"), result); err != nil {
		return result, err
	}
	if err := report.WriteDigestWithClient(filepath.Join(opts.OutputDir, "digest.md"), result, strings.TrimSpace(opts.DigestModel), client); err != nil {
		return result, err
	}
	return result, nil
}

func applyDefaults(opts Options) Options {
	if strings.TrimSpace(opts.DockerCommand) == "" {
		opts.DockerCommand = DefaultDockerCommand
	}
	if strings.TrimSpace(opts.PodmanCommand) == "" {
		opts.PodmanCommand = DefaultPodmanCommand
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
	if strings.TrimSpace(opts.OpenClawAuth) == "" {
		opts.OpenClawAuth = defaultOpenClawAuth
	}
	if opts.OpenClawStartDelaySeconds < 0 {
		opts.OpenClawStartDelaySeconds = defaultOpenClawStartDelay
	}
	if strings.TrimSpace(opts.OpenClawCodexAuthPath) == "" {
		opts.OpenClawCodexAuthPath = defaultCodexAuthPath()
	}
	if opts.JurorTimeoutSeconds <= 0 {
		opts.JurorTimeoutSeconds = DefaultRunJurorTimeoutSeconds
	}
	if opts.LawyerTimeoutSeconds <= 0 {
		opts.LawyerTimeoutSeconds = DefaultRunLawyerTimeoutSeconds
	}
	if opts.TimeoutSeconds <= 0 {
		opts.TimeoutSeconds = runner.DefaultLLMTimeoutSeconds
	}
	if opts.MaxResponseBytes <= 0 {
		opts.MaxResponseBytes = runner.DefaultMaxResponseBytes
	}
	if opts.InvalidAttemptLimit <= 0 {
		opts.InvalidAttemptLimit = runner.DefaultInvalidAttemptLimit
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
	if opts.JurorOutputLimitBytes == 0 {
		opts.JurorOutputLimitBytes = DefaultJurorOutputLimitBytes
	}
	if strings.TrimSpace(opts.DockerMCPHost) == "" {
		opts.DockerMCPHost = "host.docker.internal"
	}
	if strings.TrimSpace(opts.PodmanMCPHost) == "" {
		opts.PodmanMCPHost = "127.0.0.1"
	}
	if strings.TrimSpace(opts.LawyerInstructionsPath) == "" {
		opts.LawyerInstructionsPath = DefaultLawyerInstructionsPath()
	}
	if strings.TrimSpace(opts.RemoteLawyerSkillPath) == "" {
		opts.RemoteLawyerSkillPath = DefaultRemoteLawyerSkillPath()
	}
	if strings.TrimSpace(opts.JurorInstructionsPath) == "" {
		opts.JurorInstructionsPath = DefaultJurorInstructionsPath()
	}
	if strings.TrimSpace(opts.AutoLawyers) == "" {
		opts.AutoLawyers = DefaultAutoLawyers
	}
	return opts
}

func validateOptions(opts Options) error {
	if strings.TrimSpace(opts.ScenarioPath) == "" {
		return fmt.Errorf("scenario path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return fmt.Errorf("output dir is required")
	}
	if strings.TrimSpace(opts.CaseID) == "" {
		return fmt.Errorf("case id is required")
	}
	if strings.TrimSpace(opts.JurorPersonasPath) == "" {
		return fmt.Errorf("juror personas path is required for Pi jurors")
	}
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		return fmt.Errorf("OPENROUTER_API_KEY is required for Pi jurors")
	}
	if _, err := autoLawyerRoles(opts.AutoLawyers); err != nil {
		return err
	}
	if opts.JurorOutputLimitBytes < 0 {
		return fmt.Errorf("juror output limit bytes must be non-negative")
	}
	for _, path := range []string{opts.ScenarioPath, opts.JurorPersonasPath, opts.LawyerInstructionsPath, opts.RemoteLawyerSkillPath, opts.JurorInstructionsPath} {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
	}
	return nil
}

func autoLawyerRoles(mode string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "both":
		return []string{"plaintiff", "defendant"}, nil
	case "plaintiff":
		return []string{"plaintiff"}, nil
	case "defendant":
		return []string{"defendant"}, nil
	default:
		return nil, fmt.Errorf("invalid auto lawyer mode %q; expected both, plaintiff, or defendant", mode)
	}
}

func autoLawyerEnabled(mode string, role string) bool {
	roles, err := autoLawyerRoles(mode)
	if err != nil {
		return false
	}
	for _, current := range roles {
		if current == role {
			return true
		}
	}
	return false
}

func manualLawyerRoles(mode string) []string {
	manual := []string{}
	for _, role := range []string{"plaintiff", "defendant"} {
		if !autoLawyerEnabled(mode, role) {
			manual = append(manual, role)
		}
	}
	return manual
}

func defaultCodexAuthPath() string {
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		return filepath.Join(codexHome, "auth.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".codex", "auth.json")
	}
	return filepath.Join(home, ".codex", "auth.json")
}

func resolveOpenClawAuth(opts Options) (openClawAuthConfig, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.OpenClawAuth))
	if mode == "" {
		mode = defaultOpenClawAuth
	}
	switch mode {
	case "auto":
		path, err := validateCodexAuthPath(opts.OpenClawCodexAuthPath)
		if err == nil {
			return openClawAuthConfig{Mode: "codex", CodexAuthPath: path}, nil
		}
		if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" {
			return openClawAuthConfig{Mode: "api-key"}, nil
		}
		return openClawAuthConfig{}, fmt.Errorf("OpenClaw auth requires a readable Codex auth file at %s or OPENAI_API_KEY", opts.OpenClawCodexAuthPath)
	case "codex":
		path, err := validateCodexAuthPath(opts.OpenClawCodexAuthPath)
		if err != nil {
			return openClawAuthConfig{}, err
		}
		return openClawAuthConfig{Mode: "codex", CodexAuthPath: path}, nil
	case "api-key":
		if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
			return openClawAuthConfig{}, fmt.Errorf("OPENAI_API_KEY is required when --openclaw-auth=api-key")
		}
		return openClawAuthConfig{Mode: "api-key"}, nil
	default:
		return openClawAuthConfig{}, fmt.Errorf("invalid OpenClaw auth mode %q; expected auto, codex, or api-key", mode)
	}
}

func validateCodexAuthPath(path string) (string, error) {
	path = expandUserPath(strings.TrimSpace(path))
	if path == "" {
		return "", fmt.Errorf("Codex auth path is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read Codex auth file %s: %w", path, err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("decode Codex auth file %s: %w", path, err)
	}
	if len(decoded) == 0 {
		return "", fmt.Errorf("Codex auth file %s is empty", path)
	}
	return path, nil
}

func expandUserPath(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(home) != "" {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(home) != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
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

func publicMCPBase(value string, listenAddr string) (string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return "http://" + listenAddr, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse MCP public base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("MCP public base URL must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("MCP public base URL requires a host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("MCP public base URL must not contain a query or fragment")
	}
	return value, nil
}

func validateManualLawyerAddress(publicBase string, listenAddr string) error {
	if strings.TrimSpace(publicBase) != "" {
		return nil
	}
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return fmt.Errorf("parse MCP listen address %q: %w", listenAddr, err)
	}
	switch host {
	case "", "0.0.0.0", "::":
		return fmt.Errorf("manual lawyer mode requires --mcp-public-base-url when --mcp-listen uses a wildcard host")
	default:
		return nil
	}
}

func mcpURL(baseURL string, caseID string, query map[string]string) string {
	values := url.Values{"case_id": []string{caseID}}
	for key, value := range query {
		values.Set(key, value)
	}
	return strings.TrimRight(baseURL, "/") + "/mcp?" + values.Encode()
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
		healthDone <- waitForHealth(ctx, s.caseBase+"/health", defaultCaseAPIStartupWait)
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
		healthDone <- waitForHealth(ctx, s.mcpBase+"/health", defaultMCPStartupWait)
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
	server := "adc-" + s.opts.CaseID + "-" + role
	url := mcpURL("http://"+net.JoinHostPort(s.opts.DockerMCPHost, mcpPort), s.opts.CaseID, map[string]string{"role_id": role})
	instructions, err := renderInstructions(s.opts.LawyerInstructionsPath, instructionData{
		CaseID:    s.opts.CaseID,
		RoleID:    role,
		MCPServer: server,
		MCPURL:    url,
	})
	if err != nil {
		return err
	}
	mcpJSON, err := json.Marshal(map[string]any{
		"url":       url,
		"transport": "streamable-http",
		"headers":   map[string]string{"Authorization": "Bearer " + s.token},
	})
	if err != nil {
		return err
	}
	name := containerName("adc-" + s.opts.CaseID + "-" + role)
	authArgs, commandPrefix, err := s.openClawAuthArgs(role)
	if err != nil {
		return err
	}
	configPrefix, err := openClawConfigPatchCommand(effectiveLawyerTurnTimeoutSeconds(s.opts))
	if err != nil {
		return err
	}
	args := []string{
		"run", "--rm",
		"--name", name,
		"--add-host=host.docker.internal:host-gateway",
	}
	args = append(args, authArgs...)
	args = append(args,
		"-e", "ADC_MCP_NAME="+server,
		"-e", "ADC_MCP_JSON="+string(mcpJSON),
		"-e", "ADC_SESSION_KEY=agent:adc:"+s.opts.CaseID+":"+role,
		"-e", "ADC_ASSIGNMENT="+instructions,
		s.opts.OpenClawImage,
		"sh", "-lc",
		fmt.Sprintf("set -eu\n%s%sopenclaw mcp set \"$ADC_MCP_NAME\" \"$ADC_MCP_JSON\"\nexec openclaw agent --local --model %q --thinking %q --timeout %d --session-key \"$ADC_SESSION_KEY\" --message \"$ADC_ASSIGNMENT\" --json", commandPrefix, configPrefix, s.opts.OpenClawModel, s.opts.OpenClawThinking, s.opts.OpenClawTimeoutSeconds),
	)
	proc, err := s.startProcess(ctx, "openclaw-"+role, "docker", s.opts.DockerCommand, args, name, nil)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.processes = append(s.processes, proc)
	s.mu.Unlock()
	return nil
}

func (s *runState) writeRemoteLawyerSkill(role string) error {
	server := "adc-" + s.opts.CaseID + "-" + role
	url := mcpURL(s.mcpPublicBase, s.opts.CaseID, map[string]string{"role_id": role})
	mcpJSON, err := json.Marshal(map[string]any{
		"url":       url,
		"transport": "streamable-http",
		"headers":   map[string]string{"Authorization": "Bearer " + s.token},
	})
	if err != nil {
		return err
	}
	instructions, err := renderInstructions(s.opts.RemoteLawyerSkillPath, instructionData{
		CaseID:    s.opts.CaseID,
		RoleID:    role,
		MCPServer: server,
		MCPURL:    url,
		MCPJSON:   string(mcpJSON),
	})
	if err != nil {
		return err
	}
	name := "openclaw-" + role + "-lawyer-skill.md"
	path := filepath.Join(s.opts.OutputDir, name)
	if err := os.WriteFile(path, []byte(instructions), 0o600); err != nil {
		return fmt.Errorf("write remote lawyer skill %s: %w", path, err)
	}
	if s.opts.Log != nil {
		fmt.Fprintf(s.opts.Log, "remote %s lawyer skill written to %s\n", role, path)
	}
	return nil
}

func effectiveLawyerTurnTimeoutSeconds(opts Options) int {
	if opts.LawyerTimeoutSeconds > 0 {
		return opts.LawyerTimeoutSeconds
	}
	return DefaultRunLawyerTimeoutSeconds
}

func openClawConfigPatchCommand(lawyerTimeoutSeconds int) (string, error) {
	if lawyerTimeoutSeconds <= 0 {
		return "", fmt.Errorf("lawyer timeout must be positive")
	}
	timeoutMS := lawyerTimeoutSeconds * 1000
	patch := map[string]any{
		"plugins": map[string]any{
			"entries": map[string]any{
				"codex": map[string]any{
					"enabled": true,
					"config": map[string]any{
						"appServer": map[string]any{
							"turnCompletionIdleTimeoutMs":                 timeoutMS,
							"postToolRawAssistantCompletionIdleTimeoutMs": timeoutMS,
						},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(patch)
	if err != nil {
		return "", fmt.Errorf("marshal OpenClaw config patch: %w", err)
	}
	return fmt.Sprintf("cat > /tmp/adc-openclaw-config.json <<'JSON'\n%s\nJSON\nopenclaw config patch --file /tmp/adc-openclaw-config.json\n", raw), nil
}

func (s *runState) waitOpenClawStartDelay(ctx context.Context) error {
	delay := time.Duration(s.opts.OpenClawStartDelaySeconds) * time.Second
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case err := <-s.agentErrs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *runState) openClawAuthArgs(role string) ([]string, string, error) {
	switch s.openClawAuth.Mode {
	case "api-key":
		return []string{"-e", "OPENAI_API_KEY"}, "", nil
	case "codex":
		home, err := s.stageOpenClawCodexAuth(role)
		if err != nil {
			return nil, "", err
		}
		return []string{
			"-v", home + ":" + openClawCodexContainerHome + ":rw",
			"-e", "CODEX_HOME=" + openClawCodexContainerHome,
		}, openClawCodexAuthCommand(), nil
	default:
		return nil, "", fmt.Errorf("unsupported OpenClaw auth mode %q", s.openClawAuth.Mode)
	}
}

func openClawCodexAuthCommand() string {
	return `unset OPENAI_API_KEY
codex_token="$(node -e 'const fs=require("fs"); const home=process.env.CODEX_HOME; if (!home) process.exit(2); const d=JSON.parse(fs.readFileSync(home + "/auth.json", "utf8")); const t=d.tokens && d.tokens.access_token; if (!t) process.exit(3); process.stdout.write(t);')"
printf '%s\n' "$codex_token" | openclaw models auth paste-token --provider openai --profile-id openai:codex >/dev/null
unset codex_token
`
}

func (s *runState) stageOpenClawCodexAuth(role string) (string, error) {
	home, err := outputSubdir(s.opts.OutputDir, "openclaw-"+role+"-codex")
	if err != nil {
		return "", fmt.Errorf("resolve OpenClaw Codex home path: %w", err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", fmt.Errorf("create OpenClaw Codex home: %w", err)
	}
	raw, err := os.ReadFile(s.openClawAuth.CodexAuthPath)
	if err != nil {
		return "", fmt.Errorf("read Codex auth file %s: %w", s.openClawAuth.CodexAuthPath, err)
	}
	target := filepath.Join(home, "auth.json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return "", fmt.Errorf("write staged Codex auth file: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", errors.Join(fmt.Errorf("install staged Codex auth file: %w", err), os.Remove(tmp))
	}
	if err := os.Chmod(target, 0o600); err != nil {
		return "", fmt.Errorf("chmod staged Codex auth file: %w", err)
	}
	s.mu.Lock()
	s.secretDirs = append(s.secretDirs, home)
	s.mu.Unlock()
	return home, nil
}

func (s *runState) startActiveJuror(ctx context.Context, mcpPort string) error {
	active, err := s.activeJurorOpportunity(ctx)
	if err != nil || active == nil {
		return err
	}
	s.mu.Lock()
	proc := s.jurorProcesses[active.principalID]
	failed := s.failedJurorTurns[active.opportunityID]
	s.mu.Unlock()
	if proc != nil {
		if proc.isExited() && !failed {
			return s.reportJurorFailure(ctx, active.principalID, active.opportunityID, jurorFailureAgentExited, fmt.Sprintf("Juror %s agent process exited before completing opportunity %s.", active.principalID, active.opportunityID), map[string]any{"process_name": proc.name})
		}
		return nil
	}
	if err := s.startPiJuror(ctx, *active, mcpPort); err != nil {
		return err
	}
	return nil
}

func (s *runState) activeJurorOpportunity(ctx context.Context) (*activeJurorOpportunity, error) {
	statusURL := s.caseBase + "/roleapi/v1/status?case_id=" + url.QueryEscape(s.opts.CaseID) + "&role_id=observer"
	status, err := getJSON(ctx, statusURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mapString(status["status"])) != "active" {
		return nil, nil
	}
	currentTurn, _ := status["current_turn"].(map[string]any)
	if strings.TrimSpace(mapString(currentTurn["role_id"])) != "juror" {
		return nil, nil
	}
	principalID := strings.TrimSpace(mapString(currentTurn["principal_id"]))
	opportunityID := strings.TrimSpace(mapString(currentTurn["opportunity_id"]))
	if principalID == "" || opportunityID == "" {
		return nil, fmt.Errorf("active juror turn missing principal_id or opportunity_id")
	}
	getURL := s.caseBase + "/roleapi/v1/get?case_id=" + url.QueryEscape(s.opts.CaseID) + "&role_id=juror&principal_id=" + url.QueryEscape(principalID)
	detail, err := getJSON(ctx, getURL)
	if err != nil {
		return nil, err
	}
	opportunity, _ := detail["opportunity"].(map[string]any)
	agent, _ := opportunity["agent"].(map[string]any)
	rawSpec, _ := agent["request_spec"].(map[string]any)
	if rawSpec == nil {
		return nil, fmt.Errorf("active juror %s has no request_spec in role API response", principalID)
	}
	spec, err := modelrequest.ParseMap(rawSpec)
	if err != nil {
		return nil, fmt.Errorf("parse request_spec for juror %s: %w", principalID, err)
	}
	return &activeJurorOpportunity{principalID: principalID, opportunityID: opportunityID, requestSpec: &spec}, nil
}

func getJSON(ctx context.Context, rawURL string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned HTTP %d: %s", rawURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func isConnectionRefused(err error) bool {
	var syscallErr *os.SyscallError
	return errors.As(err, &syscallErr) && errors.Is(syscallErr.Err, syscall.ECONNREFUSED)
}

func (s *runState) startPiJuror(ctx context.Context, active activeJurorOpportunity, mcpPort string) error {
	server := defaultPiMCPServer
	url := mcpURL("http://"+net.JoinHostPort(s.opts.PodmanMCPHost, mcpPort), s.opts.CaseID, map[string]string{"role_id": "juror", "principal_id": active.principalID})
	instructions, err := renderInstructions(s.opts.JurorInstructionsPath, instructionData{
		CaseID:      s.opts.CaseID,
		PrincipalID: active.principalID,
		MCPServer:   server,
		MCPURL:      url,
	})
	if err != nil {
		return err
	}
	home, err := outputSubdir(s.opts.OutputDir, "pi-"+active.principalID)
	if err != nil {
		return fmt.Errorf("resolve Pi home path: %w", err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return fmt.Errorf("create Pi home: %w", err)
	}
	model, err := writePiConfig(home, active, server, url, s.token)
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
	proc, err := s.startProcess(ctx, "pi-"+active.principalID, "podman", s.opts.PodmanCommand, args, "", &jurorProcessTarget{
		principalID:   active.principalID,
		opportunityID: active.opportunityID,
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.processes = append(s.processes, proc)
	s.jurorProcesses[active.principalID] = proc
	s.mu.Unlock()
	return nil
}

func outputSubdir(outputDir string, name string) (string, error) {
	return filepath.Abs(filepath.Join(outputDir, name))
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

func writePiConfig(home string, active activeJurorOpportunity, server string, mcpURL string, token string) (string, error) {
	if active.requestSpec == nil {
		return "", fmt.Errorf("juror %s has no request_spec; JSONL juror pool records are required", active.principalID)
	}
	spec := *active.requestSpec
	if spec.Endpoint != "openrouter" {
		return "", fmt.Errorf("Pi juror requires openrouter endpoint for %s; got %s", active.principalID, spec.Endpoint)
	}
	unsupported := []string{}
	if spec.Request.Temperature != nil {
		unsupported = append(unsupported, "temperature")
	}
	if spec.Request.TopP != nil {
		unsupported = append(unsupported, "top_p")
	}
	if len(unsupported) > 0 {
		return "", fmt.Errorf("Pi juror cannot enforce request fields for %s: %s", active.principalID, strings.Join(unsupported, " "))
	}
	model := spec.UpstreamModel()
	settingsDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return "", fmt.Errorf("create Pi settings dir: %w", err)
	}
	if err := writeJSONFile(filepath.Join(settingsDir, "settings.json"), map[string]any{
		"defaultProvider": "openrouter",
		"defaultModel":    model,
		"quietStartup":    true,
	}); err != nil {
		return "", err
	}
	modelEntry := map[string]any{
		"id":   model,
		"name": "ADC " + active.principalID + " " + model,
	}
	spec = spec.WithFallbackMaxOutputTokens(defaultPiMaxOutputTokens)
	if maxTokens := spec.MaxOutputTokens(); maxTokens != nil {
		modelEntry["maxTokens"] = *maxTokens
	}
	if routing := spec.ProviderBody(); len(routing) > 0 {
		modelEntry["compat"] = map[string]any{"openRouterRouting": routing}
	}
	if err := writeJSONFile(filepath.Join(settingsDir, "models.json"), map[string]any{
		"providers": map[string]any{
			"openrouter": map[string]any{
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKey":  "$OPENROUTER_API_KEY",
				"api":     "openai-completions",
				"models":  []map[string]any{modelEntry},
			},
		},
	}); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(home, ".mcp.json"), map[string]any{
		"mcpServers": map[string]any{
			server: map[string]any{
				"url":       mcpURL,
				"transport": "streamable-http",
				"lifecycle": "keep-alive",
				"headers":   map[string]string{"Authorization": "Bearer " + token},
			},
		},
	}); err != nil {
		return "", err
	}
	return model, nil
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

func (s *runState) startProcess(ctx context.Context, name string, kind string, command string, args []string, stopName string, jurorTarget *jurorProcessTarget) (*processRecord, error) {
	stdoutPath := filepath.Join(s.logDir, name+".stdout")
	stderrPath := filepath.Join(s.logDir, name+".stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("create %s stdout log: %w", name, err)
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("create %s stderr log: %w", name, err)
	}
	stdoutWriter := io.Writer(stdout)
	var stdoutFilter *piTailLogWriter
	if jurorTarget != nil && strings.HasPrefix(name, "pi-") {
		stdoutFilter = newPiTailLogWriter(stdout)
		stdoutWriter = stdoutFilter
	}
	stdoutCounter := newProcessOutputCounter(stdoutWriter)
	closeStdout := func() error {
		if stdoutFilter == nil {
			return stdout.Close()
		}
		return errors.Join(stdoutFilter.Flush(), stdout.Close())
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = stdoutCounter
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(fmt.Errorf("start %s: %w", name, err), closeStdout(), stderr.Close())
	}
	if err := os.WriteFile(filepath.Join(s.opts.OutputDir, name+".pid"), []byte(fmt.Sprintf("%d\n", cmd.Process.Pid)), 0o644); err != nil {
		return nil, errors.Join(err, cmd.Process.Kill(), cmd.Wait(), closeStdout(), stderr.Close())
	}
	record := &processRecord{
		name:          name,
		kind:          kind,
		command:       cmd,
		done:          make(chan error, 1),
		stopName:      stopName,
		stdoutPath:    stdoutPath,
		stderrPath:    stderrPath,
		finished:      make(chan struct{}),
		stdoutCounter: stdoutCounter,
	}
	go func() {
		err := cmd.Wait()
		closeOut := closeStdout()
		closeErr := stderr.Close()
		waitErr := errors.Join(err, closeOut, closeErr)
		record.markExited()
		record.done <- waitErr
		if ctx.Err() != nil {
			return
		}
		if jurorTarget != nil {
			if err := s.handleJurorProcessExit(ctx, record, *jurorTarget, waitErr); err != nil {
				s.agentErrs <- err
			}
			return
		}
		if waitErr != nil {
			s.agentErrs <- fmt.Errorf("%s process %s failed: %w", kind, name, waitErr)
			return
		}
		s.agentErrs <- fmt.Errorf("%s process %s exited before case completion", kind, name)
	}()
	if jurorTarget != nil {
		go s.monitorJurorOutput(ctx, record, *jurorTarget, defaultJurorOutputCheck)
	}
	return record, nil
}

func (p *processRecord) markExited() {
	p.mu.Lock()
	p.exited = true
	close(p.finished)
	p.mu.Unlock()
}

func (p *processRecord) isExited() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exited
}

func (p *processRecord) setForcedFailure(reason string, message string, details map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.forcedReason != "" {
		return
	}
	p.forcedReason = strings.TrimSpace(reason)
	p.forcedMessage = strings.TrimSpace(message)
	p.forcedDetails = cloneLocalMap(details)
}

func (p *processRecord) forcedFailure() (string, string, map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.forcedReason, p.forcedMessage, cloneLocalMap(p.forcedDetails)
}

func (s *runState) monitorJurorOutput(ctx context.Context, proc *processRecord, target jurorProcessTarget, interval time.Duration) {
	if s.opts.JurorOutputLimitBytes <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if proc.isExited() {
				return
			}
			size, err := jurorProcessOutputSize(proc)
			if err != nil {
				s.agentErrs <- fmt.Errorf("check juror output for %s: %w", proc.name, err)
				return
			}
			if size.Total <= s.opts.JurorOutputLimitBytes {
				continue
			}
			message, details := jurorOutputLimitFailure(proc.name, target, size, s.opts.JurorOutputLimitBytes)
			proc.setForcedFailure(jurorFailureOutputLimit, message, details)
			if err := proc.command.Process.Kill(); err != nil {
				if proc.isExited() {
					return
				}
				s.agentErrs <- fmt.Errorf("kill %s after juror output limit exceeded: %w", proc.name, err)
			}
			return
		case <-proc.finished:
			return
		case <-ctx.Done():
			return
		}
	}
}

func jurorProcessOutputSize(proc *processRecord) (processOutputSize, error) {
	stdoutInfo, err := os.Stat(proc.stdoutPath)
	if err != nil {
		return processOutputSize{}, fmt.Errorf("stat stdout log %s: %w", proc.stdoutPath, err)
	}
	stderrInfo, err := os.Stat(proc.stderrPath)
	if err != nil {
		return processOutputSize{}, fmt.Errorf("stat stderr log %s: %w", proc.stderrPath, err)
	}
	stdoutBytes := stdoutInfo.Size()
	stderrBytes := stderrInfo.Size()
	if proc.stdoutCounter != nil {
		stdoutBytes = proc.stdoutCounter.Size()
	}
	if stdoutBytes > int64(^uint64(0)>>1)-stderrBytes {
		return processOutputSize{}, fmt.Errorf("process output size overflow for %s", proc.name)
	}
	return processOutputSize{Stdout: stdoutBytes, Stderr: stderrBytes, Total: stdoutBytes + stderrBytes}, nil
}

func jurorOutputLimitFailure(procName string, target jurorProcessTarget, size processOutputSize, limit int64) (string, map[string]any) {
	message := fmt.Sprintf(
		"Juror %s agent process exceeded the output limit before completing opportunity %s: %d bytes written, limit %d bytes.",
		target.principalID,
		target.opportunityID,
		size.Total,
		limit,
	)
	return message, map[string]any{
		"process_name":       procName,
		"output_bytes":       size.Total,
		"stdout_bytes":       size.Stdout,
		"stderr_bytes":       size.Stderr,
		"output_limit_bytes": limit,
	}
}

func cloneLocalMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		if strings.TrimSpace(key) != "" && value != nil {
			out[key] = value
		}
	}
	return out
}

func (s *runState) handleJurorProcessExit(ctx context.Context, proc *processRecord, target jurorProcessTarget, waitErr error) error {
	active, err := s.activeJurorOpportunity(ctx)
	if err != nil {
		return fmt.Errorf("check juror status after %s exit: %w", proc.name, err)
	}
	if active == nil || active.principalID != target.principalID || active.opportunityID != target.opportunityID {
		return nil
	}
	reason, forcedMessage, forcedDetails := proc.forcedFailure()
	if reason == "" {
		reason = jurorFailureAgentExited
	}
	message := fmt.Sprintf("Juror %s agent process exited before completing opportunity %s.", target.principalID, target.opportunityID)
	details := map[string]any{"process_name": proc.name}
	for key, value := range forcedDetails {
		details[key] = value
	}
	if forcedMessage != "" {
		message = forcedMessage
	}
	if waitErr != nil {
		if forcedMessage == "" {
			message = fmt.Sprintf("Juror %s agent process failed before completing opportunity %s: %s.", target.principalID, target.opportunityID, waitErr.Error())
		}
		details["process_error"] = waitErr.Error()
	}
	return s.reportJurorFailure(ctx, target.principalID, target.opportunityID, reason, message, details)
}

func (s *runState) reportJurorFailure(ctx context.Context, principalID string, opportunityID string, reason string, message string, details map[string]any) error {
	s.mu.Lock()
	if s.failedJurorTurns[opportunityID] {
		s.mu.Unlock()
		return nil
	}
	s.failedJurorTurns[opportunityID] = true
	s.mu.Unlock()

	payload := map[string]any{
		"case_id":        s.opts.CaseID,
		"role_id":        "juror",
		"principal_id":   principalID,
		"opportunity_id": opportunityID,
		"reason":         reason,
		"message":        message,
		"details":        details,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	failURL := s.caseBase + "/roleapi/v1/fail"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, failURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("report juror failure for %s: %w", principalID, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("report juror failure for %s returned HTTP %d: %s", principalID, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var response map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&response); err != nil {
		return fmt.Errorf("decode juror failure response for %s: %w", principalID, err)
	}
	if ok, _ := response["ok"].(bool); !ok {
		message := ""
		if errObj, ok := response["error"].(map[string]any); ok {
			message = fmt.Sprint(errObj["message"])
		}
		if strings.TrimSpace(message) == "" {
			message = strings.TrimSpace(string(body))
		}
		return fmt.Errorf("report juror failure for %s was rejected: %s", principalID, message)
	}
	return nil
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

func (s *runState) cleanupSecrets() error {
	s.mu.Lock()
	dirs := append([]string{}, s.secretDirs...)
	s.mu.Unlock()
	var errs []error
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			errs = append(errs, fmt.Errorf("remove staged secret directory %s: %w", dir, err))
		}
	}
	return errors.Join(errs...)
}

func writeRunSummary(outDir string, result runner.Result, opts Options) error {
	caseObj, _ := result.FinalState["case"].(map[string]any)
	return writeJSONFile(filepath.Join(outDir, "local-run.json"), map[string]any{
		"case_id":                             opts.CaseID,
		"run_id":                              opts.RunID,
		"scenario":                            result.Scenario,
		"case_status":                         mapString(caseObj["status"]),
		"auto_lawyers":                        opts.AutoLawyers,
		"mcp_public_base_url":                 opts.MCPPublicBaseURL,
		"openclaw_lawyer_start_delay_seconds": opts.OpenClawStartDelaySeconds,
		"juror_output_limit_bytes":            opts.JurorOutputLimitBytes,
		"juror_default_max_output_tokens":     defaultPiMaxOutputTokens,
		"assertion_count":                     len(result.Assertions),
		"turn_count":                          len(result.TurnLogs),
	})
}

func randomToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "adc-" + hex.EncodeToString(buf[:]), nil
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
		return "adc"
	}
	return out
}

func mapString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}
