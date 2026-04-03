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

	"adjudication/adc/runtime/lean"
	"adjudication/adc/runtime/runner"
	"adjudication/adc/runtime/store"
)

func RunACPRole(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("acp-role", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc acp-role --scenario <json> --role <name> --command <server> [options]\n\n")
		fs.PrintDefaults()
	})
	scenarioPath := fs.String("scenario", "", "Path to scenario JSON")
	roleName := fs.String("role", "", "Role to delegate to the external ACP agent")
	command := fs.String("command", "", "ACP server command")
	outputPath := fs.String("output", "out/adc-role-run.json", "Run artifact output path")
	eventsPath := fs.String("events", "out/adc-role-actions.ndjson", "Event log output path")
	dbPath := fs.String("db", "out/adc-role.db", "SQLite path")
	runID := fs.String("run-id", "", "Run ID override")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	timeoutSeconds := fs.Int("timeout-seconds", 120, "ACP prompt timeout in seconds")
	jsonSummary := fs.Bool("json-summary", true, "Emit JSON summary to stdout")
	var argList stringListFlag
	var envList stringListFlag
	fs.Var(&argList, "arg", "ACP server argument; repeat as needed")
	fs.Var(&envList, "env", "Environment override KEY=VALUE; repeat as needed")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*scenarioPath) == "" {
		return fmt.Errorf("--scenario is required")
	}
	if strings.TrimSpace(*roleName) == "" {
		return fmt.Errorf("--role is required")
	}
	if strings.TrimSpace(*command) == "" {
		return fmt.Errorf("--command is required")
	}
	xproxyServer, err := maybeStartXProxy(false)
	if err != nil {
		return err
	}
	if xproxyServer != nil {
		defer xproxyServer.Close()
	}
	for _, path := range []string{*outputPath, *eventsPath, *dbPath} {
		if err := ensureParentDir(path); err != nil {
			return err
		}
	}
	effectiveRunID := strings.TrimSpace(*runID)
	if effectiveRunID == "" {
		effectiveRunID = fmt.Sprintf("acp-role-%d", time.Now().UTC().UnixNano())
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	engine := lean.New(strings.Fields(strings.TrimSpace(*engineCommand)))
	r, err := runner.New(st, engine, nil, nil, runner.Config{
		ScenarioPath: *scenarioPath,
		OutputPath:   *outputPath,
		EventsPath:   *eventsPath,
		RunID:        effectiveRunID,
		Offline:      true,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSeconds)*time.Second)
	defer cancel()
	result, err := r.RunACPRoleExperiment(ctx, runner.ACPRoleConfig{
		Role:    strings.TrimSpace(*roleName),
		Command: strings.TrimSpace(*command),
		Args:    []string(argList),
		Env:     []string(envList),
	})
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
		"role":               strings.TrimSpace(*roleName),
		"assertion_failures": failed,
		"output":             filepath.Clean(*outputPath),
		"events":             filepath.Clean(*eventsPath),
		"db":                 filepath.Clean(*dbPath),
		"run_id":             effectiveRunID,
	}
	if *jsonSummary {
		wire, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(wire))
		return err
	}
	_, err = fmt.Fprintf(
		stdout,
		"scenario=%s role=%s run_id=%s assertion_failures=%d output=%s events=%s db=%s\n",
		result.Scenario,
		strings.TrimSpace(*roleName),
		effectiveRunID,
		failed,
		filepath.Clean(*outputPath),
		filepath.Clean(*eventsPath),
		filepath.Clean(*dbPath),
	)
	return err
}
