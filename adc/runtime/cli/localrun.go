package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"adjudication/adc/runtime/casegen"
	"adjudication/adc/runtime/courts"
	"adjudication/adc/runtime/localrun"
	"adjudication/adc/runtime/runner"
	"adjudication/common/openai"
)

func RunLocal(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("run", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc run (--complaint <markdown> | --scenario <json>) [options]\n\n")
		fs.PrintDefaults()
	})
	complaintPath := fs.String("complaint", "", "Complaint markdown path")
	scenarioPath := fs.String("scenario", "", "Scenario JSON path")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	outDir := fs.String("out-dir", "", "Output directory")
	model := fs.String("model", "", "Runtime model. Default: scenario default for --scenario, ADC default for --complaint")
	nonJurorModel := fs.String("non-juror-model", casegen.DefaultNonJurorModel(), "Runtime model for judge, lawyers, and clerk when preparing a complaint")
	plaintiffModel := fs.String("plaintiff-model", "", "Runtime model for plaintiff counsel when preparing a complaint. Default: --non-juror-model")
	defendantModel := fs.String("defendant-model", "", "Runtime model for defense counsel when preparing a complaint. Default: --non-juror-model")
	judgeModel := fs.String("judge-model", "", "Runtime model for the judge when preparing a complaint. Default: --non-juror-model")
	clerkModel := fs.String("clerk-model", "", "Runtime model for the clerk when preparing a complaint. Default: --non-juror-model")
	plannerModel := fs.String("planner-model", casegen.DefaultPlannerModel(), "Model for neutral intake and strategy planning when preparing a complaint")
	reportModel := fs.String("report-model", casegen.DefaultRuntimeModel(), "Model for digest generation")
	temperature := fs.String("temperature", "", "Override runtime temperature")
	nonJurorTemperature := fs.String("non-juror-temperature", "", "Override runtime temperature for judge, lawyers, and clerk when preparing a complaint")
	jurorTemperature := fs.String("juror-temperature", "", "Override runtime temperature for direct jurors if used")
	jurorPersonas := fs.String("juror-personas", defaultPersonaRecordsPath(), "Juror JSONL request-spec pool")
	trialMode := fs.String("trial-mode", "auto", "Trial mode override for complaint setup: auto, jury, or bench")
	skipVoirDire := fs.Bool("skip-voir-dire", false, "Skip questionnaires and voir dire during complaint setup, then empanel randomly from the candidate panel")
	jurorCount := fs.Int("juror-count", 0, "Jury size for jury trials, 6 through 12. Omit to use the scenario or court default")
	minimumConcurring := fs.Int("minimum-concurring", 0, "Minimum concurring jurors needed for a verdict. Omit to use the scenario or court default")
	unanimousRequired := fs.String("unanimous-required", "", "Whether the jury verdict must be unanimous: true or false. Omit to use the scenario or court default")
	online := fs.Bool("online", false, "Enable web search for internal direct model calls")
	offline := fs.Bool("offline", false, "Disable internal LLM calls")
	caseAPIAddr := fs.String("caseapi-addr", runner.DefaultCaseAPIAddr, "Private case API listen address")
	mcpListenAddr := fs.String("mcp-listen", "0.0.0.0:0", "MCP listen address")
	mcpBearerToken := fs.String("mcp-bearer-token", "", "MCP bearer token. Default: generated")
	jurorTimeoutSeconds := fs.Int("juror-timeout-seconds", localrun.DefaultRunJurorTimeoutSeconds, "Juror opportunity timeout seconds")
	lawyerTimeoutSeconds := fs.Int("lawyer-timeout-seconds", localrun.DefaultRunLawyerTimeoutSeconds, "Lawyer opportunity timeout seconds")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "Internal LLM HTTP timeout seconds")
	maxResponseBytes := fs.Int("max-response-bytes", runner.DefaultMaxResponseBytes, "Maximum bytes allowed in one direct-runtime model response")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", runner.DefaultInvalidAttemptLimit, "Maximum invalid submissions before an opportunity fails")
	enginePath := fs.String("engine", defaultEngineCommand(), "Lean engine command string")
	runID := fs.String("run-id", "", "Run ID override")
	caseID := fs.String("case-id", "", "Case ID")
	lawyerInstructions := fs.String("lawyer-instructions", localrun.DefaultLawyerInstructionsPath(), "OpenClaw lawyer instruction template")
	remoteLawyerSkill := fs.String("remote-lawyer-skill", localrun.DefaultRemoteLawyerSkillPath(), "OpenClaw remote lawyer skill template")
	jurorInstructions := fs.String("juror-instructions", localrun.DefaultJurorInstructionsPath(), "Pi juror instruction template")
	autoLawyers := fs.String("auto-lawyers", localrun.DefaultAutoLawyers, "OpenClaw lawyers started by adc run: both, plaintiff, or defendant")
	mcpPublicBaseURL := fs.String("mcp-public-base-url", "", "Public MCP base URL for remote lawyers")
	dockerCommand := fs.String("docker", localrun.DefaultDockerCommand, "Docker command")
	podmanCommand := fs.String("podman", localrun.DefaultPodmanCommand, "Podman command")
	openClawImage := fs.String("openclaw-image", "", "OpenClaw container image")
	openClawModel := fs.String("openclaw-model", "", "OpenClaw model")
	openClawThinking := fs.String("openclaw-thinking", "", "OpenClaw thinking setting")
	openClawTimeoutSeconds := fs.Int("openclaw-timeout-seconds", 0, "OpenClaw agent timeout seconds")
	openClawAuth := fs.String("openclaw-auth", "", "OpenClaw auth mode: auto, codex, or api-key")
	openClawCodexAuth := fs.String("openclaw-codex-auth", "", "Codex auth.json path for OpenClaw")
	openClawStartDelaySeconds := fs.Int("openclaw-lawyer-start-delay-seconds", -1, "Delay between plaintiff and defendant OpenClaw startup; 0 disables")
	piImage := fs.String("pi-image", "", "Pi container image")
	piMCPAdapter := fs.String("pi-mcp-adapter", "", "Pi MCP adapter package")
	jurorOutputLimitBytes := fs.Int64("juror-output-limit-bytes", localrun.DefaultJurorOutputLimitBytes, "Total stdout plus stderr byte limit per Pi juror agent")
	dockerMCPHost := fs.String("docker-mcp-host", "", "Host name used by Docker containers to reach MCP")
	podmanMCPHost := fs.String("podman-mcp-host", "", "Host name used by Podman containers to reach MCP")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc run accepts no positional arguments")
	}
	hasComplaint := strings.TrimSpace(*complaintPath) != ""
	hasScenario := strings.TrimSpace(*scenarioPath) != ""
	if hasComplaint == hasScenario {
		return fmt.Errorf("exactly one of --complaint or --scenario is required")
	}
	now := time.Now().UTC().Format("20060102150405")
	if strings.TrimSpace(*caseID) == "" {
		*caseID = "adc-" + now
	}
	if strings.TrimSpace(*runID) == "" {
		*runID = "run-" + strings.TrimSpace(*caseID)
	}
	if strings.TrimSpace(*outDir) == "" {
		*outDir = filepath.Join("out", strings.TrimSpace(*caseID))
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	nonJurorTempPtr, err := parseOptionalFloat(*nonJurorTemperature)
	if err != nil {
		return fmt.Errorf("parse --non-juror-temperature: %w", err)
	}
	jurorTempPtr, err := parseOptionalFloat(*jurorTemperature)
	if err != nil {
		return fmt.Errorf("parse --juror-temperature: %w", err)
	}
	unanimousRequiredPtr, err := parseOptionalBool(*unanimousRequired)
	if err != nil {
		return fmt.Errorf("parse --unanimous-required: %w", err)
	}
	policyOverrides, err := juryPolicyOverrides(*jurorCount, *minimumConcurring, unanimousRequiredPtr)
	if err != nil {
		return err
	}

	runScenarioPath := strings.TrimSpace(*scenarioPath)
	runModel := strings.TrimSpace(*model)
	if hasComplaint {
		if *offline {
			return fmt.Errorf("--offline cannot prepare a complaint-based run")
		}
		client, err := openai.NewFromEnv(*online, time.Duration(*timeoutSeconds)*time.Second)
		if err != nil {
			return err
		}
		setup, err := prepareComplaintScenario(context.Background(), client, complaintSetupOptions{
			ComplaintPath:       *complaintPath,
			CourtRef:            *courtRef,
			OutDir:              *outDir,
			RuntimeModel:        *model,
			PlannerModel:        *plannerModel,
			NonJurorModel:       *nonJurorModel,
			PlaintiffModel:      *plaintiffModel,
			DefendantModel:      *defendantModel,
			JudgeModel:          *judgeModel,
			ClerkModel:          *clerkModel,
			Temperature:         tempPtr,
			NonJurorTemperature: nonJurorTempPtr,
			TrialModeOverride:   *trialMode,
			SkipVoirDire:        *skipVoirDire,
			JurorCount:          *jurorCount,
			MinimumConcurring:   *minimumConcurring,
			UnanimousRequired:   unanimousRequiredPtr,
		})
		if err != nil {
			return err
		}
		runScenarioPath = setup.ScenarioPath
		runModel = setup.RuntimeModel
	}

	result, err := localrun.Run(context.Background(), localrun.Options{
		ScenarioPath:              runScenarioPath,
		OutputDir:                 *outDir,
		Model:                     runModel,
		DigestModel:               resolveDefault(*reportModel, casegen.DefaultRuntimeModel()),
		Temperature:               tempPtr,
		JurorTemperature:          jurorTempPtr,
		JurorPersonasPath:         *jurorPersonas,
		Online:                    *online,
		Offline:                   *offline,
		CaseAPIAddr:               *caseAPIAddr,
		MCPListenAddr:             *mcpListenAddr,
		MCPBearerToken:            *mcpBearerToken,
		JurorTimeoutSeconds:       *jurorTimeoutSeconds,
		LawyerTimeoutSeconds:      *lawyerTimeoutSeconds,
		TimeoutSeconds:            *timeoutSeconds,
		MaxResponseBytes:          *maxResponseBytes,
		InvalidAttemptLimit:       *invalidAttemptLimit,
		EnginePath:                *enginePath,
		RunID:                     *runID,
		CaseID:                    *caseID,
		LawyerInstructionsPath:    *lawyerInstructions,
		RemoteLawyerSkillPath:     *remoteLawyerSkill,
		JurorInstructionsPath:     *jurorInstructions,
		AutoLawyers:               *autoLawyers,
		MCPPublicBaseURL:          *mcpPublicBaseURL,
		DockerCommand:             *dockerCommand,
		PodmanCommand:             *podmanCommand,
		OpenClawImage:             *openClawImage,
		OpenClawModel:             *openClawModel,
		OpenClawThinking:          *openClawThinking,
		OpenClawTimeoutSeconds:    *openClawTimeoutSeconds,
		OpenClawAuth:              *openClawAuth,
		OpenClawCodexAuthPath:     *openClawCodexAuth,
		OpenClawStartDelaySeconds: *openClawStartDelaySeconds,
		PiImage:                   *piImage,
		PiMCPAdapter:              *piMCPAdapter,
		JurorOutputLimitBytes:     *jurorOutputLimitBytes,
		DockerMCPHost:             *dockerMCPHost,
		PodmanMCPHost:             *podmanMCPHost,
		PolicyOverrides:           policyOverrides,
		Log:                       stderr,
	})
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
