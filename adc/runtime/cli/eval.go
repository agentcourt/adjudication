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
	case "help", "-h", "--help":
		if len(args) == 1 {
			printEvalUsage(stdout)
			return nil
		}
		switch args[1] {
		case "judge-voir-dire":
			return RunEvalJudgeVoirDire([]string{"-h"}, stdout, stderr)
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
	fixtures := fs.String("fixtures", defaultADCPath("evals", "judge", "voir_dire_questions.jsonl"), "Judge voir dire fixture JSONL file")
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

func printEvalUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: adc eval <eval> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Evals:")
	fmt.Fprintln(w, "  judge-voir-dire  Evaluate judge rulings on proposed voir dire questions")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Use 'adc eval help <eval>' for eval flags.")
}
