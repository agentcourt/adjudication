package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"adjudication/adc/runtime/casegen"
	"adjudication/adc/runtime/courts"
	"adjudication/adc/runtime/lean"
	"adjudication/adc/runtime/report"
	"adjudication/adc/runtime/runner"
	"adjudication/adc/runtime/store"
	"adjudication/common/openai"
)

func RunCase(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("case", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc case --complaint <markdown> [options]\n\n")
		fs.PrintDefaults()
	})
	complaintPath := fs.String("complaint", "", "Path to complaint markdown")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	outDir := fs.String("out-dir", "out/case", "Output directory for staged inputs and run artifacts")
	model := fs.String("model", casegen.DefaultRuntimeModel(), "Runtime model for litigation agents")
	nonJurorModel := fs.String("non-juror-model", casegen.DefaultNonJurorModel(), "Runtime model for judge, lawyers, and clerk")
	plaintiffModel := fs.String("plaintiff-model", "", "Runtime model for plaintiff counsel. Default: --non-juror-model")
	defendantModel := fs.String("defendant-model", "", "Runtime model for defense counsel. Default: --non-juror-model")
	judgeModel := fs.String("judge-model", "", "Runtime model for the judge. Default: --non-juror-model")
	clerkModel := fs.String("clerk-model", "", "Runtime model for the clerk. Default: --non-juror-model")
	flashModel := fs.String("flash", "", `Temporary live-role override. Accepts "gpt-5-mini" or "openai://gpt-5-mini". Leaves planner and report unchanged.`)
	plannerModel := fs.String("planner-model", casegen.DefaultPlannerModel(), "Model for neutral intake and strategy planning")
	reportModel := fs.String("report-model", casegen.DefaultRuntimeModel(), "Model for digest generation")
	temperature := fs.String("temperature", "", "Override runtime temperature")
	nonJurorTemperature := fs.String("non-juror-temperature", "", "Override runtime temperature for judge, lawyers, and clerk")
	jurorTemperature := fs.String("juror-temperature", "", "Override runtime temperature for jurors only")
	jurorPersonas := fs.String("juror-personas", defaultPersonaRecordsPath(), "Path to juror model/persona pairs file")
	trialMode := fs.String("trial-mode", "auto", "Trial mode override: auto, jury, or bench")
	skipVoirDire := fs.Bool("skip-voir-dire", false, "Skip questionnaires and voir dire, then empanel randomly from the candidate panel")
	online := fs.Bool("online", false, "Enable web search tool for planning and litigation agents")
	allThroughXProxy := fs.Bool("all-through-xproxy", false, "Send complaint planning, live litigation, and digest summarization through xproxy. Plain model names are treated as OpenAI xproxy models")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM HTTP timeout in seconds")
	maxResponseBytes := fs.Int("max-response-bytes", runner.DefaultMaxResponseBytes, "Maximum bytes allowed in one direct-runtime model response")
	var acpRoles stringListFlag
	acpCommand := fs.String("acp-command", "", "ACP server command shared by delegated roles")
	acpTimeoutSeconds := fs.Int("acp-timeout-seconds", defaultACPTimeoutSeconds, "Timeout in seconds for each delegated ACP opportunity turn")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", runner.DefaultInvalidAttemptLimit, "Maximum invalid model responses before a turn fails")
	runID := fs.String("run-id", "", "Run ID override")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	jsonSummary := fs.Bool("json-summary", true, "Emit JSON summary to stdout")
	var acpArgList stringListFlag
	var acpEnvList stringListFlag
	fs.Var(&acpRoles, "acp-role", "Role to delegate through ACP during opportunity turns; repeat as needed")
	fs.Var(&acpArgList, "acp-arg", "ACP server argument; repeat as needed")
	fs.Var(&acpEnvList, "acp-env", "ACP environment override KEY=VALUE; repeat as needed")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*complaintPath) == "" {
		return fmt.Errorf("--complaint is required")
	}
	if strings.TrimSpace(*outDir) == "" {
		return fmt.Errorf("--out-dir is required")
	}
	if len(acpRoles) > 0 && strings.TrimSpace(*acpCommand) == "" {
		return fmt.Errorf("--acp-command is required when --acp-role is set")
	}
	flashOverride, err := parseFlashModel(*flashModel)
	if err != nil {
		return err
	}
	if flashOverride.XProxy != "" {
		prevFlashModel, hadPrevFlashModel := os.LookupEnv("ADC_FLASH_XPROXY_MODEL")
		if err := os.Setenv("ADC_FLASH_XPROXY_MODEL", flashOverride.XProxy); err != nil {
			return fmt.Errorf("set ADC_FLASH_XPROXY_MODEL: %w", err)
		}
		defer func() {
			if hadPrevFlashModel {
				_ = os.Setenv("ADC_FLASH_XPROXY_MODEL", prevFlashModel)
				return
			}
			_ = os.Unsetenv("ADC_FLASH_XPROXY_MODEL")
		}()
	}
	useJurorXProxy := strings.TrimSpace(*jurorPersonas) != ""
	if *allThroughXProxy || len(acpRoles) > 0 || useJurorXProxy {
		xproxyServer, err := maybeStartXProxy(true)
		if err != nil {
			return err
		}
		if xproxyServer != nil {
			defer xproxyServer.Close()
		}
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}

	resolvedRuntimeModel := resolveDefault(*model, casegen.DefaultRuntimeModel())
	resolvedPlannerModel := resolveDefault(*plannerModel, casegen.DefaultPlannerModel())
	resolvedReportModel := resolveDefault(*reportModel, casegen.DefaultRuntimeModel())
	resolvedNonJurorModel := resolveDefault(*nonJurorModel, casegen.DefaultNonJurorModel())
	resolvedPlaintiffModel := resolveDefault(*plaintiffModel, resolvedNonJurorModel)
	resolvedDefendantModel := resolveDefault(*defendantModel, resolvedNonJurorModel)
	resolvedJudgeModel := resolveDefault(*judgeModel, resolvedNonJurorModel)
	resolvedClerkModel := resolveDefault(*clerkModel, resolvedNonJurorModel)
	if flashOverride.Direct != "" {
		resolvedRuntimeModel = flashOverride.Direct
		resolvedNonJurorModel = flashOverride.Direct
		resolvedPlaintiffModel = flashOverride.Direct
		resolvedDefendantModel = flashOverride.Direct
		resolvedJudgeModel = flashOverride.Direct
		resolvedClerkModel = flashOverride.Direct
	}
	if *allThroughXProxy {
		for label, target := range map[string]*string{
			"--model":           &resolvedRuntimeModel,
			"--planner-model":   &resolvedPlannerModel,
			"--report-model":    &resolvedReportModel,
			"--non-juror-model": &resolvedNonJurorModel,
			"--plaintiff-model": &resolvedPlaintiffModel,
			"--defendant-model": &resolvedDefendantModel,
			"--judge-model":     &resolvedJudgeModel,
			"--clerk-model":     &resolvedClerkModel,
		} {
			normalized, err := normalizeXProxyModel(*target)
			if err != nil {
				return fmt.Errorf("normalize %s for xproxy: %w", label, err)
			}
			*target = normalized
		}
	}

	timeout := time.Duration(*timeoutSeconds) * time.Second
	var client *openai.Client
	var jurorClient *openai.Client
	if *allThroughXProxy {
		client, err = newXProxyClient(*online, timeout)
		if err != nil {
			return err
		}
	} else {
		client, err = openai.NewFromEnv(*online, timeout)
		if err != nil {
			return err
		}
	}
	if useJurorXProxy {
		jurorClient, err = newXProxyClient(*online, timeout)
		if err != nil {
			return err
		}
	}
	complaint, err := casegen.LoadComplaint(*complaintPath)
	if err != nil {
		return err
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return err
	}
	complaint, err = casegen.StageComplaintAssets(*outDir, complaint)
	if err != nil {
		return err
	}

	ctx := context.Background()
	plan, err := casegen.CreatePlan(ctx, client, resolvedPlannerModel, complaint, court)
	if err != nil {
		return err
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

	scenario, err := casegen.BuildScenario(plan, complaint, casegen.ScenarioOptions{
		RuntimeModel:        resolvedRuntimeModel,
		Temperature:         tempPtr,
		NonJurorTemperature: nonJurorTempPtr,
		PlaintiffModel:      resolvedPlaintiffModel,
		DefendantModel:      resolvedDefendantModel,
		JudgeModel:          resolvedJudgeModel,
		ClerkModel:          resolvedClerkModel,
		Court:               court,
		TrialModeOverride:   strings.TrimSpace(*trialMode),
		SkipVoirDire:        *skipVoirDire,
	})
	if err != nil {
		return err
	}

	normalizedCasePath := filepath.Join(*outDir, "normalized-case.json")
	plaintiffStrategyPath := filepath.Join(*outDir, "plaintiff-strategy.md")
	defenseStrategyPath := filepath.Join(*outDir, "defense-strategy.md")
	scenarioPath := filepath.Join(*outDir, "generated-scenario.json")
	outputPath := filepath.Join(*outDir, "run.json")
	runtimePath := filepath.Join(*outDir, "runtime.json")
	eventsPath := filepath.Join(*outDir, "events.ndjson")
	dbPath := filepath.Join(*outDir, "run.db")
	transcriptPath := filepath.Join(*outDir, "transcript.md")
	digestPath := filepath.Join(*outDir, "digest.md")

	if err := writeJSONFile(normalizedCasePath, plan.Packet); err != nil {
		return err
	}
	if err := os.WriteFile(plaintiffStrategyPath, []byte(strings.TrimSpace(plan.PlaintiffStrategy)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write plaintiff strategy: %w", err)
	}
	if err := os.WriteFile(defenseStrategyPath, []byte(strings.TrimSpace(plan.DefenseStrategy)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write defense strategy: %w", err)
	}
	if err := writeJSONFile(scenarioPath, scenario); err != nil {
		return err
	}
	runtimeLimits := runner.RuntimeLimits{
		LLMTimeoutSeconds:   *timeoutSeconds,
		ACPTimeoutSeconds:   *acpTimeoutSeconds,
		MaxResponseBytes:    *maxResponseBytes,
		InvalidAttemptLimit: *invalidAttemptLimit,
	}.Normalized()
	if err := writeJSONFile(runtimePath, runtimeLimits); err != nil {
		return err
	}

	effectiveRunID := strings.TrimSpace(*runID)
	if effectiveRunID == "" {
		effectiveRunID = fmt.Sprintf("run-%d", time.Now().UTC().UnixNano())
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := st.Close(); closeErr != nil {
			log.Printf("error closing sqlite: %v", closeErr)
		}
	}()

	engine := lean.New(strings.Fields(strings.TrimSpace(*engineCommand)))
	acpCfg, err := runner.NewACPConfig([]string(acpRoles), *acpCommand, []string(acpArgList), []string(acpEnvList), time.Duration(*acpTimeoutSeconds)*time.Second)
	if err != nil {
		return err
	}

	r, err := runner.New(st, engine, client, jurorClient, runner.Config{
		ScenarioPath:      scenarioPath,
		OutputPath:        outputPath,
		EventsPath:        eventsPath,
		RunID:             effectiveRunID,
		Model:             resolvedRuntimeModel,
		Temperature:       tempPtr,
		JurorTemperature:  jurorTempPtr,
		JurorPersonasPath: strings.TrimSpace(*jurorPersonas),
		Runtime:           runtimeLimits,
		FlashJurorModel:   flashOverride.XProxy,
		ACP:               acpCfg,
	})
	if err != nil {
		return err
	}
	result, err := r.Run(ctx)
	if err != nil {
		return err
	}
	if err := report.WriteTranscript(transcriptPath, result); err != nil {
		return err
	}
	if err := report.WriteDigestWithClient(digestPath, result, resolvedReportModel, client, *allThroughXProxy); err != nil {
		return err
	}

	summary := map[string]any{
		"run_id":             effectiveRunID,
		"complaint":          complaint.StagedRelPath,
		"normalized_case":    normalizedCasePath,
		"plaintiff_strategy": plaintiffStrategyPath,
		"defense_strategy":   defenseStrategyPath,
		"generated_scenario": scenarioPath,
		"output":             outputPath,
		"runtime":            runtimePath,
		"events":             eventsPath,
		"db":                 dbPath,
		"transcript":         transcriptPath,
		"digest":             digestPath,
	}
	if *jsonSummary {
		wire, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(wire))
		return err
	}
	_, err = fmt.Fprintf(stdout, "run_id=%s out_dir=%s scenario=%s output=%s runtime=%s digest=%s transcript=%s\n", effectiveRunID, *outDir, scenarioPath, outputPath, runtimePath, digestPath, transcriptPath)
	return err
}
