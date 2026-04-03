package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"adjudication/adc/runtime/lean"
	"adjudication/adc/runtime/report"
	"adjudication/adc/runtime/runner"
	"adjudication/adc/runtime/spec"
	"adjudication/adc/runtime/store"
	"adjudication/common/openai"
)

func RunScenario(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("run", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc run --scenario <json> [options]\n\n")
		fs.PrintDefaults()
	})
	scenarioPath := fs.String("scenario", "", "Path to scenario JSON")
	outputPath := fs.String("output", "out/adc-run.json", "Run artifact output path")
	runtimePath := fs.String("runtime", "out/adc-runtime.json", "Runtime limits artifact output path")
	eventsPath := fs.String("events", "out/adc-actions.ndjson", "Event log output path")
	dbPath := fs.String("db", "out/adc-run.db", "SQLite path")
	model := fs.String("model", "", "Override the scenario default model for roles without their own model")
	temperature := fs.String("temperature", "", "Override the scenario default temperature for roles without their own temperature")
	jurorTemperature := fs.String("juror-temperature", "", "Override runtime temperature for jurors only")
	jurorPersonas := fs.String("juror-personas", defaultPersonaRecordsPath(), "Path to juror model/persona pairs file")
	online := fs.Bool("online", false, "Enable web search tool")
	offline := fs.Bool("offline", false, "Disable LLM calls; only deterministic turns")
	allThroughXProxy := fs.Bool("all-through-xproxy", false, "Send direct runtime inference and digest summarization through xproxy. Plain model names are treated as OpenAI xproxy models")
	var acpRoles stringListFlag
	acpCommand := fs.String("acp-command", "", "ACP server command shared by delegated roles")
	acpTimeoutSeconds := fs.Int("acp-timeout-seconds", defaultACPTimeoutSeconds, "Timeout in seconds for each delegated ACP opportunity turn")
	maxResponseBytes := fs.Int("max-response-bytes", runner.DefaultMaxResponseBytes, "Maximum bytes allowed in one direct-runtime model response")
	runID := fs.String("run-id", "", "Run ID override")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM HTTP timeout")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", runner.DefaultInvalidAttemptLimit, "Maximum invalid model responses before a turn fails")
	jsonSummary := fs.Bool("json-summary", true, "Emit JSON summary to stdout")
	transcriptPath := fs.String("transcript", "", "Optional transcript markdown output path")
	digestPath := fs.String("digest", "", "Optional digest/report markdown output path")
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
	if strings.TrimSpace(*scenarioPath) == "" {
		return fmt.Errorf("--scenario is required")
	}
	if len(acpRoles) > 0 && strings.TrimSpace(*acpCommand) == "" {
		return fmt.Errorf("--acp-command is required when --acp-role is set")
	}
	useJurorXProxy := strings.TrimSpace(*jurorPersonas) != ""
	if *allThroughXProxy || len(acpRoles) > 0 || (!*offline && useJurorXProxy) {
		xproxyServer, err := maybeStartXProxy(true)
		if err != nil {
			return err
		}
		if xproxyServer != nil {
			defer xproxyServer.Close()
		}
	}
	for _, path := range []string{*outputPath, *runtimePath, *eventsPath, *dbPath, *transcriptPath, *digestPath} {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := ensureParentDir(path); err != nil {
			return err
		}
	}
	effectiveRunID := strings.TrimSpace(*runID)
	if effectiveRunID == "" {
		effectiveRunID = fmt.Sprintf("run-%d", time.Now().UTC().UnixNano())
	}
	activeScenarioPath := *scenarioPath
	if *allThroughXProxy {
		scenario, err := spec.Load(*scenarioPath)
		if err != nil {
			return err
		}
		scenario, err = normalizeScenarioModelsForXProxy(scenario)
		if err != nil {
			return fmt.Errorf("normalize scenario models for xproxy: %w", err)
		}
		tmp, err := os.CreateTemp("", "adc-scenario-xproxy-*.json")
		if err != nil {
			return fmt.Errorf("create temporary xproxy scenario: %w", err)
		}
		tmpPath := tmp.Name()
		if err := tmp.Close(); err != nil {
			return fmt.Errorf("close temporary xproxy scenario: %w", err)
		}
		defer os.Remove(tmpPath)
		if err := writeJSONFile(tmpPath, scenario); err != nil {
			return err
		}
		activeScenarioPath = tmpPath
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := st.Close(); closeErr != nil {
			log.Printf("error closing sqlite: %v", closeErr)
		}
	}()

	engine := lean.New(strings.Fields(strings.TrimSpace(*engineCommand)))
	var client *openai.Client
	var jurorClient *openai.Client
	resolvedModel := strings.TrimSpace(*model)
	if *allThroughXProxy {
		resolvedModel, err = normalizeXProxyModel(resolvedModel)
		if err != nil {
			return fmt.Errorf("normalize --model for xproxy: %w", err)
		}
	}
	if !*offline {
		if *allThroughXProxy {
			client, err = newXProxyClient(*online, time.Duration(*timeoutSeconds)*time.Second)
			if err != nil {
				return err
			}
		} else {
			client, err = openai.NewFromEnv(*online, time.Duration(*timeoutSeconds)*time.Second)
			if err != nil {
				return err
			}
		}
		if useJurorXProxy {
			jurorClient, err = newXProxyClient(*online, time.Duration(*timeoutSeconds)*time.Second)
			if err != nil {
				return err
			}
		}
	}

	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	jurorTempPtr, err := parseOptionalFloat(*jurorTemperature)
	if err != nil {
		return fmt.Errorf("parse --juror-temperature: %w", err)
	}

	acpCfg, err := runner.NewACPConfig([]string(acpRoles), *acpCommand, []string(acpArgList), []string(acpEnvList), time.Duration(*acpTimeoutSeconds)*time.Second)
	if err != nil {
		return err
	}
	runtimeLimits := runner.RuntimeLimits{
		LLMTimeoutSeconds:   *timeoutSeconds,
		ACPTimeoutSeconds:   *acpTimeoutSeconds,
		MaxResponseBytes:    *maxResponseBytes,
		InvalidAttemptLimit: *invalidAttemptLimit,
	}.Normalized()
	if err := writeJSONFile(*runtimePath, runtimeLimits); err != nil {
		return err
	}

	r, err := runner.New(st, engine, client, jurorClient, runner.Config{
		ScenarioPath:      activeScenarioPath,
		OutputPath:        *outputPath,
		EventsPath:        *eventsPath,
		RunID:             effectiveRunID,
		Model:             resolvedModel,
		Temperature:       tempPtr,
		JurorTemperature:  jurorTempPtr,
		JurorPersonasPath: strings.TrimSpace(*jurorPersonas),
		Runtime:           runtimeLimits,
		Offline:           *offline,
		ACP:               acpCfg,
	})
	if err != nil {
		return err
	}
	if *offline && r.RequiresLLMTurns() {
		log.Printf("warning: --offline is set, but scenario includes non-deterministic turns that require an LLM")
	}
	result, err := r.Run(context.Background())
	if err != nil {
		return err
	}
	failed := 0
	for _, a := range result.Assertions {
		if passed, _ := a["passed"].(bool); !passed {
			failed++
		}
	}
	summary := map[string]any{
		"scenario":           result.Scenario,
		"assertion_failures": failed,
		"output":             *outputPath,
		"runtime":            *runtimePath,
		"events":             *eventsPath,
		"db":                 *dbPath,
		"run_id":             effectiveRunID,
	}
	if *jsonSummary {
		wire, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(stdout, string(wire)); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(
			stdout,
			"scenario=%s run_id=%s assertion_failures=%d output=%s runtime=%s events=%s db=%s\n",
			result.Scenario,
			effectiveRunID,
			failed,
			*outputPath,
			*runtimePath,
			*eventsPath,
			*dbPath,
		); err != nil {
			return err
		}
	}
	if err := report.WriteTranscript(strings.TrimSpace(*transcriptPath), result); err != nil {
		return err
	}
	if err := report.WriteDigestWithClient(strings.TrimSpace(*digestPath), result, "", client, *allThroughXProxy); err != nil {
		return err
	}
	if failed > 0 {
		return fmt.Errorf("assertions failed: %d", failed)
	}
	return nil
}
