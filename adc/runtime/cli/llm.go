package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"adjudication/adc/runtime/runner"
	"adjudication/common/openai"
	"adjudication/common/persona"
	"adjudication/common/xproxy"
)

const llmToolCheckName = "submit_juror_vote"

func RunLLM(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("llm", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc llm --prompt <text> [options]\n\n")
		fs.PrintDefaults()
	})
	prompt := fs.String("prompt", "", "Prompt text")
	promptFile := fs.String("prompt-file", "", "Path to prompt text file")
	model := fs.String("model", "openrouter://openai/gpt-5", "Model name in xproxy PROVIDER://MODEL form")
	personaRecord := fs.String("persona", "", `Persona record in PROVIDER://MODEL,path/to/persona.txt form, or "random" to sample from the shared personas file`)
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM HTTP timeout in seconds")
	toolCheck := fs.Bool("tool-check", false, "Require one submit_juror_vote tool call and print its arguments")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	promptText, err := loadPromptText(strings.TrimSpace(*prompt), strings.TrimSpace(*promptFile))
	if err != nil {
		return err
	}
	if strings.TrimSpace(promptText) == "" {
		return fmt.Errorf("--prompt or --prompt-file is required")
	}
	modelName := strings.TrimSpace(*model)
	systemPrompt := ""
	if strings.TrimSpace(*personaRecord) != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}
		spec, sampled, err := resolveLLMPersonaSpec(strings.TrimSpace(*personaRecord), cwd)
		if err != nil {
			return fmt.Errorf("parse --persona: %w", err)
		}
		modelName = spec.Model
		systemPrompt = persona.JurorPrompt("", spec.Text)
		if sampled {
			if _, err := fmt.Fprintln(stdout, spec.File); err != nil {
				return err
			}
		}
	}
	if modelName == "" {
		return fmt.Errorf("--model is required")
	}
	if _, err := xproxy.ParseXProxyModel(modelName); err != nil {
		return fmt.Errorf("parse --model: %w", err)
	}
	xproxyServer, err := maybeStartXProxy(true)
	if err != nil {
		return err
	}
	if xproxyServer != nil {
		defer xproxyServer.Close()
	}
	client, err := newXProxyClient(false, time.Duration(*timeoutSeconds)*time.Second)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSeconds)*time.Second)
	defer cancel()
	input := make([]map[string]any, 0, 2)
	if systemPrompt != "" {
		input = append(input, map[string]any{"role": "system", "content": systemPrompt})
	}
	if *toolCheck {
		input = append(input, map[string]any{
			"role":    "system",
			"content": "When the prompt asks for a juror act, call submit_juror_vote exactly once.  Do not reply with plain text instead of the required tool call.",
		})
	}
	input = append(input, map[string]any{"role": "user", "content": promptText})
	tools := []map[string]any(nil)
	if *toolCheck {
		tools, err = llmToolCheckTools()
		if err != nil {
			return err
		}
	}
	resp, err := client.CreateResponse(ctx, modelName, input, tools, "", nil)
	if err != nil {
		return err
	}
	output := strings.TrimSpace(resp.Text)
	if *toolCheck {
		output, err = extractToolCheckArguments(resp)
		if err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(stdout, output); err != nil {
		return err
	}
	return nil
}

func llmToolCheckTools() ([]map[string]any, error) {
	return runner.BuildTools([]string{llmToolCheckName})
}

func extractToolCheckArguments(resp openai.Response) (string, error) {
	if len(resp.ToolCalls) != 1 {
		return "", fmt.Errorf("model did not call required tool %s", llmToolCheckName)
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != llmToolCheckName {
		return "", fmt.Errorf("model called %s, want %s", strings.TrimSpace(call.Name), llmToolCheckName)
	}
	if strings.TrimSpace(call.ArgumentsError) != "" {
		return "", fmt.Errorf("required tool %s returned malformed arguments: %s", llmToolCheckName, call.ArgumentsError)
	}
	if strings.TrimSpace(stringArg(call.Arguments, "juror_id")) == "" {
		return "", fmt.Errorf("required tool %s missing juror_id", llmToolCheckName)
	}
	if strings.TrimSpace(stringArg(call.Arguments, "vote")) == "" {
		return "", fmt.Errorf("required tool %s missing vote", llmToolCheckName)
	}
	if strings.TrimSpace(stringArg(call.Arguments, "confidence")) == "" {
		return "", fmt.Errorf("required tool %s missing confidence", llmToolCheckName)
	}
	if strings.TrimSpace(stringArg(call.Arguments, "explanation")) == "" {
		return "", fmt.Errorf("required tool %s missing explanation", llmToolCheckName)
	}
	if _, ok := call.Arguments["damages"]; !ok {
		return "", fmt.Errorf("required tool %s missing damages", llmToolCheckName)
	}
	encoded, err := json.Marshal(call.Arguments)
	if err != nil {
		return "", fmt.Errorf("marshal tool arguments: %w", err)
	}
	return string(encoded), nil
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func resolveLLMPersonaSpec(record string, cwd string) (persona.Spec, bool, error) {
	record = strings.TrimSpace(record)
	if record == "" {
		return persona.Spec{}, false, nil
	}
	if record == "random" {
		spec, err := persona.SampleRecordFile(defaultPersonaRecordsPathFor(cwd), cwd)
		if err != nil {
			return persona.Spec{}, false, err
		}
		return spec, true, nil
	}
	spec, err := persona.ParseRecord(record, cwd)
	if err == nil {
		return spec, false, nil
	}
	spec, fallbackErr := persona.ParseRecord(record, filepath.Join(cwd, "etc"))
	if fallbackErr == nil {
		return spec, false, nil
	}
	return persona.Spec{}, false, err
}
