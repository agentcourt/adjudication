package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"adjudication/arb/runtime/localrun"
	"adjudication/arb/runtime/proceeding"
)

func runLocal(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var caseFiles explicitFileList
	complaintPath := fs.String("complaint", "", "Complaint markdown file")
	fs.Var(&caseFiles, "file", "Explicit case file path or glob. May be repeated")
	outDir := fs.String("out-dir", "", "Output directory")
	policyPath := fs.String("policy", "", "Policy JSON file")
	councilSize := fs.Int("council-size", 0, "Override policy council_size")
	evidenceStandard := fs.String("evidence-standard", "", "Override policy evidence_standard")
	attorneyInstructionsPath := fs.String("attorney-instructions", "", "Attorney instructions markdown file")
	promptDir := fs.String("prompt-dir", "", "Prompt directory override")
	attorneyCommonPrompt := fs.String("attorney-common-prompt", "", "Attorney common prompt file override")
	attorneyArgumentPrompt := fs.String("attorney-arguments-prompt", "", "Attorney arguments prompt file override")
	attorneyRebuttalPrompt := fs.String("attorney-rebuttals-prompt", "", "Attorney rebuttals prompt file override")
	commonRoot := fs.String("common-root", proceeding.DefaultCommonRoot(), "Path to sibling shared common directory")
	councilPool := fs.String("council-pool", "", "Council model/persona pool file")
	caseAPIAddr := fs.String("caseapi-addr", "127.0.0.1:0", "Private case API listen address")
	mcpListenAddr := fs.String("mcp-listen", "0.0.0.0:0", "MCP listen address")
	mcpBearerToken := fs.String("mcp-bearer-token", "", "MCP bearer token. Default: generated")
	councilTimeoutSeconds := fs.Int("council-timeout-seconds", 900, "Council turn timeout seconds")
	lawyerTimeoutSeconds := fs.Int("lawyer-timeout-seconds", 900, "Lawyer turn timeout seconds")
	maxResponseBytes := fs.Int("max-response-bytes", 0, "Override runtime max parsed response bytes")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", 0, "Override runtime invalid-attempt limit")
	enginePath := fs.String("engine", proceeding.DefaultEnginePath(), "Lean engine binary")
	runID := fs.String("run-id", "", "Run ID override")
	caseID := fs.String("case-id", "", "Case ID")
	lawyerInstructions := fs.String("lawyer-instructions", filepath.Join("agent-instructions", "openclaw-lawyer.md.tmpl"), "OpenClaw lawyer instruction template")
	councilInstructions := fs.String("council-instructions", filepath.Join("agent-instructions", "pi-council.md.tmpl"), "Pi council instruction template")
	dockerCommand := fs.String("docker", "docker", "Docker command")
	podmanCommand := fs.String("podman", "podman", "Podman command")
	openClawImage := fs.String("openclaw-image", "", "OpenClaw container image")
	openClawModel := fs.String("openclaw-model", "", "OpenClaw model")
	openClawThinking := fs.String("openclaw-thinking", "", "OpenClaw thinking setting")
	openClawTimeoutSeconds := fs.Int("openclaw-timeout-seconds", 0, "OpenClaw agent timeout seconds")
	openClawAuth := fs.String("openclaw-auth", "", "OpenClaw auth mode: auto, codex, or api-key")
	openClawCodexAuth := fs.String("openclaw-codex-auth", "", "Codex auth.json path for OpenClaw")
	piImage := fs.String("pi-image", "", "Pi container image")
	piMCPAdapter := fs.String("pi-mcp-adapter", "", "Pi MCP adapter package")
	dockerMCPHost := fs.String("docker-mcp-host", "", "Host name used by Docker containers to reach MCP")
	podmanMCPHost := fs.String("podman-mcp-host", "", "Host name used by Podman containers to reach MCP")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: aar run [EXAMPLE] [options]\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	example := ""
	if fs.NArg() > 1 {
		return fmt.Errorf("aar run accepts at most one example name")
	}
	if fs.NArg() == 1 {
		example = strings.TrimSpace(fs.Arg(0))
		if example == "" || strings.Contains(example, "/") || strings.HasPrefix(example, ".") || strings.Contains(example, "..") {
			return fmt.Errorf("invalid example name: %s", example)
		}
	}
	now := time.Now().UTC().Format("20060102150405")
	if example != "" {
		if strings.TrimSpace(*complaintPath) == "" {
			*complaintPath = filepath.Join("examples", example, "complaint.md")
		}
		if strings.TrimSpace(*caseID) == "" {
			*caseID = "arb-" + example + "-" + now
		}
		if strings.TrimSpace(*outDir) == "" {
			*outDir = filepath.Join("out", example+"-openclaw-pi-"+now)
		}
	} else if strings.TrimSpace(*caseID) == "" {
		*caseID = "arb-" + now
	}
	if strings.TrimSpace(*runID) == "" {
		*runID = "run-" + strings.TrimSpace(*caseID)
	}
	if strings.TrimSpace(*outDir) == "" {
		*outDir = filepath.Join("out", strings.TrimSpace(*caseID))
	}
	opts := localrun.Options{
		ComplaintPath:              *complaintPath,
		CaseFiles:                  caseFiles.values,
		OutputDir:                  *outDir,
		PolicyPath:                 *policyPath,
		CouncilSize:                *councilSize,
		EvidenceStandard:           *evidenceStandard,
		AttorneyInstructionsPath:   *attorneyInstructionsPath,
		PromptDir:                  *promptDir,
		AttorneyCommonPromptPath:   *attorneyCommonPrompt,
		AttorneyArgumentPromptPath: *attorneyArgumentPrompt,
		AttorneyRebuttalPromptPath: *attorneyRebuttalPrompt,
		CommonRoot:                 *commonRoot,
		CouncilPoolPath:            *councilPool,
		CaseAPIAddr:                *caseAPIAddr,
		MCPListenAddr:              *mcpListenAddr,
		MCPBearerToken:             *mcpBearerToken,
		CouncilTimeoutSeconds:      *councilTimeoutSeconds,
		LawyerTimeoutSeconds:       *lawyerTimeoutSeconds,
		MaxResponseBytes:           *maxResponseBytes,
		InvalidAttemptLimit:        *invalidAttemptLimit,
		EnginePath:                 *enginePath,
		RunID:                      *runID,
		CaseID:                     *caseID,
		LawyerInstructionsPath:     *lawyerInstructions,
		CouncilInstructionsPath:    *councilInstructions,
		DockerCommand:              *dockerCommand,
		PodmanCommand:              *podmanCommand,
		OpenClawImage:              *openClawImage,
		OpenClawModel:              *openClawModel,
		OpenClawThinking:           *openClawThinking,
		OpenClawTimeoutSeconds:     *openClawTimeoutSeconds,
		OpenClawAuth:               *openClawAuth,
		OpenClawCodexAuthPath:      *openClawCodexAuth,
		PiImage:                    *piImage,
		PiMCPAdapter:               *piMCPAdapter,
		DockerMCPHost:              *dockerMCPHost,
		PodmanMCPHost:              *podmanMCPHost,
		Log:                        stderr,
	}
	result, err := localrun.Run(ctx, opts)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal run result: %w", err)
	}
	fmt.Fprintln(stdout, string(raw))
	return nil
}
