package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	adceval "adjudication/adc/runtime/eval"
	"adjudication/adc/runtime/lean"
)

func RunEval(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printEvalUsage(stderr)
		return fmt.Errorf("eval subcommand is required")
	}
	switch args[0] {
	case "judge-voir-dire":
		return RunEvalJudgeVoirDire(args[1:], stdout, stderr)
	case "judge-for-cause":
		return RunEvalJudgeForCause(args[1:], stdout, stderr)
	case "judge-rule56":
		return RunEvalJudgeRule56(args[1:], stdout, stderr)
	case "judge-rule12":
		return RunEvalJudgeRule12(args[1:], stdout, stderr)
	case "judge-rule51":
		return RunEvalJudgeRule51(args[1:], stdout, stderr)
	case "judge-rule37":
		return RunEvalJudgeRule37(args[1:], stdout, stderr)
	case "judge-rule11":
		return RunEvalJudgeRule11(args[1:], stdout, stderr)
	case "judge-rule52":
		return RunEvalJudgeRule52(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		if len(args) == 1 {
			printEvalUsage(stdout)
			return nil
		}
		switch args[1] {
		case "judge-voir-dire":
			return RunEvalJudgeVoirDire([]string{"-h"}, stdout, stderr)
		case "judge-for-cause":
			return RunEvalJudgeForCause([]string{"-h"}, stdout, stderr)
		case "judge-rule56":
			return RunEvalJudgeRule56([]string{"-h"}, stdout, stderr)
		case "judge-rule12":
			return RunEvalJudgeRule12([]string{"-h"}, stdout, stderr)
		case "judge-rule51":
			return RunEvalJudgeRule51([]string{"-h"}, stdout, stderr)
		case "judge-rule37":
			return RunEvalJudgeRule37([]string{"-h"}, stdout, stderr)
		case "judge-rule11":
			return RunEvalJudgeRule11([]string{"-h"}, stdout, stderr)
		case "judge-rule52":
			return RunEvalJudgeRule52([]string{"-h"}, stdout, stderr)
		default:
			printEvalUsage(stderr)
			return fmt.Errorf("unknown eval help topic %q", args[1])
		}
	default:
		printEvalUsage(stderr)
		return fmt.Errorf("unknown eval subcommand %q", args[0])
	}
}

func RunEvalJudgeVoirDire(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-voir-dire", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc eval judge-voir-dire [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultADCPath("evals", "judge", "rules", "rule47", "voir_dire_questions.jsonl"), "Judge voir dire fixture JSONL file")
	outDir := fs.String("out-dir", defaultADCPath("evals", "judge", "out", "latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected rulings as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeVoirDire(adceval.JudgeVoirDireRescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	summary, err := adceval.RunJudgeVoirDire(context.Background(), adceval.JudgeVoirDireOptions{
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeForCause(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-for-cause", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc eval judge-for-cause [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultADCPath("evals", "judge", "rules", "rule47", "for_cause_challenges.jsonl"), "Judge for-cause fixture JSONL file")
	outDir := fs.String("out-dir", defaultADCPath("evals", "judge", "out", "for-cause-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected for-cause rulings as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeForCause(adceval.JudgeForCauseRescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	summary, err := adceval.RunJudgeForCause(context.Background(), adceval.JudgeForCauseOptions{
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule56(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule56", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc eval judge-rule56 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultADCPath("evals", "judge", "rules", "rule56", "fixtures.jsonl"), "Judge Rule 56 fixture JSONL file")
	outDir := fs.String("out-dir", defaultADCPath("evals", "judge", "out", "rule56-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	dryRun := fs.Bool("dry-run", false, "Use expected dispositions as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeRule56(context.Background(), adceval.JudgeRule56Options{
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule12(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule12", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc eval judge-rule12 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultADCPath("evals", "judge", "rules", "rule12", "fixtures.jsonl"), "Judge Rule 12 fixture JSONL file")
	outDir := fs.String("out-dir", defaultADCPath("evals", "judge", "out", "rule12-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected dispositions as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule12(adceval.JudgeRule12RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	summary, err := adceval.RunJudgeRule12(context.Background(), adceval.JudgeRule12Options{
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule51(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule51", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc eval judge-rule51 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultADCPath("evals", "judge", "rules", "rule51", "fixtures.jsonl"), "Judge Rule 51 fixture JSONL file")
	outDir := fs.String("out-dir", defaultADCPath("evals", "judge", "out", "rule51-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected instruction summary as a synthetic model response")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule51(adceval.JudgeRule51RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	summary, err := adceval.RunJudgeRule51(context.Background(), adceval.JudgeRule51Options{
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule37(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule37", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc eval judge-rule37 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultADCPath("evals", "judge", "rules", "rule37", "fixtures.jsonl"), "Judge Rule 37 fixture JSONL file")
	outDir := fs.String("out-dir", defaultADCPath("evals", "judge", "out", "rule37-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected Rule 37 decisions as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule37(adceval.JudgeRule37RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	summary, err := adceval.RunJudgeRule37(context.Background(), adceval.JudgeRule37Options{
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule11(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule11", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc eval judge-rule11 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultADCPath("evals", "judge", "rules", "rule11", "fixtures.jsonl"), "Judge Rule 11 fixture JSONL file")
	outDir := fs.String("out-dir", defaultADCPath("evals", "judge", "out", "rule11-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected Rule 11 decisions as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule11(adceval.JudgeRule11RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	summary, err := adceval.RunJudgeRule11(context.Background(), adceval.JudgeRule11Options{
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule52(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule52", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc eval judge-rule52 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultADCPath("evals", "judge", "rules", "rule52", "fixtures.jsonl"), "Judge Rule 52 fixture JSONL file")
	outDir := fs.String("out-dir", defaultADCPath("evals", "judge", "out", "rule52-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected Rule 52 bench opinions as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule52(adceval.JudgeRule52RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	summary, err := adceval.RunJudgeRule52(context.Background(), adceval.JudgeRule52Options{
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func printEvalUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: adc eval <eval> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Evals:")
	fmt.Fprintln(w, "  judge-for-cause  Evaluate judge rulings on for-cause juror challenges")
	fmt.Fprintln(w, "  judge-voir-dire  Evaluate judge rulings on proposed voir dire questions")
	fmt.Fprintln(w, "  judge-rule11     Evaluate judge dispositions of Rule 11 motions")
	fmt.Fprintln(w, "  judge-rule12     Evaluate judge dispositions of Rule 12 motions")
	fmt.Fprintln(w, "  judge-rule37     Evaluate judge dispositions of Rule 37 motions")
	fmt.Fprintln(w, "  judge-rule51     Evaluate judge settlement of jury instructions")
	fmt.Fprintln(w, "  judge-rule52     Evaluate judge Rule 52 bench opinions")
	fmt.Fprintln(w, "  judge-rule56     Evaluate judge dispositions of Rule 56 motions")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Use 'adc eval help <eval>' for eval flags.")
}
