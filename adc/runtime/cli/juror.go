package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"adjudication/common/modelrequest"
	"adjudication/common/openai"
	"adjudication/common/persona"
)

type jurorTranscriptEntry struct {
	Time    string `json:"time"`
	Member  string `json:"member"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

func RunJuror(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("juror", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc juror --member SELECTOR --prompt <text> [options]\n")
		fmt.Fprintf(stderr, "       adc juror --list [options]\n\n")
		fs.PrintDefaults()
	})
	poolPath := fs.String("pool", defaultPersonaRecordsPath(), "Juror pool JSONL file")
	list := fs.Bool("list", false, "List pool members and exit")
	member := fs.String("member", "", "Pool member: 1-based index or unique substring of a --list line")
	prompt := fs.String("prompt", "", "Prompt text")
	promptFile := fs.String("prompt-file", "", "Path to prompt text file")
	repeat := fs.Int("repeat", 1, "Number of independent samples")
	vote := fs.Bool("vote", false, "Require one submit_juror_vote tool call and print its arguments")
	transcript := fs.String("transcript", "", "Conversation transcript NDJSON file: prior turns are replayed, this turn is appended")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM HTTP timeout in seconds")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	specs, err := persona.LoadRecordsFile(*poolPath, cwd)
	if err != nil {
		return err
	}
	if *list {
		for i, spec := range specs {
			if _, err := fmt.Fprintf(stdout, "%d %s\n", i+1, jurorMemberLabel(spec)); err != nil {
				return err
			}
		}
		return nil
	}
	spec, err := selectJurorMember(specs, *member)
	if err != nil {
		return err
	}
	promptText, err := loadPromptText(strings.TrimSpace(*prompt), strings.TrimSpace(*promptFile))
	if err != nil {
		return err
	}
	if strings.TrimSpace(promptText) == "" {
		return fmt.Errorf("--prompt or --prompt-file is required")
	}
	if *repeat < 1 {
		return fmt.Errorf("--repeat must be >= 1")
	}
	if *transcript != "" && *repeat != 1 {
		return fmt.Errorf("--transcript requires --repeat 1")
	}
	label := jurorMemberLabel(spec)
	var history []jurorTranscriptEntry
	if *transcript != "" {
		history, err = readJurorTranscript(*transcript, label)
		if err != nil {
			return err
		}
	}

	input := make([]map[string]any, 0, len(history)+3)
	input = append(input, map[string]any{"role": "system", "content": persona.JurorPrompt("", spec.Text)})
	if *vote {
		input = append(input, map[string]any{
			"role":    "system",
			"content": "When the prompt asks for a juror act, call submit_juror_vote exactly once.  Do not reply with plain text instead of the required tool call.",
		})
	}
	for _, entry := range history {
		input = append(input, map[string]any{"role": entry.Role, "content": entry.Content})
	}
	input = append(input, map[string]any{"role": "user", "content": promptText})

	var tools []map[string]any
	if *vote {
		tools, err = llmToolCheckTools()
		if err != nil {
			return err
		}
	}
	endpoint := ""
	plainModel := ""
	if spec.RequestSpec != nil {
		endpoint = spec.RequestSpec.Endpoint
	} else {
		ref, err := modelrequest.ParseModelRef(spec.Model)
		if err != nil {
			return fmt.Errorf("parse pool member model %q: %w", spec.Model, err)
		}
		endpoint = ref.Endpoint
		plainModel = ref.Model
	}
	client, err := openai.NewForEndpoint(endpoint, false, time.Duration(*timeoutSeconds)*time.Second)
	if err != nil {
		return err
	}

	for sample := 0; sample < *repeat; sample++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSeconds)*time.Second)
		var resp openai.Response
		if spec.RequestSpec != nil {
			resp, err = client.CreateResponseWithRequestSpec(ctx, *spec.RequestSpec, input, tools, "")
		} else {
			resp, err = client.CreateResponse(ctx, plainModel, input, tools, "", nil)
		}
		cancel()
		if err != nil {
			return err
		}
		output := strings.TrimSpace(resp.Text)
		if *vote {
			output, err = extractToolCheckArguments(resp)
			if err != nil {
				return err
			}
		}
		if *repeat > 1 && !*vote {
			encoded, err := json.Marshal(map[string]any{"sample": sample + 1, "answer": output})
			if err != nil {
				return err
			}
			output = string(encoded)
		}
		if _, err := fmt.Fprintln(stdout, output); err != nil {
			return err
		}
		if *transcript != "" {
			now := time.Now().UTC().Format(time.RFC3339)
			entries := []jurorTranscriptEntry{
				{Time: now, Member: label, Role: "user", Content: promptText},
				{Time: now, Member: label, Role: "assistant", Content: output},
			}
			if err := appendJurorTranscript(*transcript, entries); err != nil {
				return err
			}
		}
	}
	return nil
}

func jurorMemberLabel(spec persona.Spec) string {
	parts := []string{spec.Model}
	if spec.RequestSpec != nil && spec.RequestSpec.Provider != nil {
		provider := spec.RequestSpec.Provider
		if len(provider.Only) > 0 {
			parts = append(parts, "provider="+strings.Join(provider.Only, "/"))
		}
		if len(provider.Quantizations) > 0 {
			parts = append(parts, "quant="+strings.Join(provider.Quantizations, "/"))
		}
	}
	parts = append(parts, "persona="+spec.File)
	return strings.Join(parts, " ")
}

func selectJurorMember(specs []persona.Spec, selector string) (persona.Spec, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return persona.Spec{}, fmt.Errorf("--member is required unless --list is set")
	}
	if index, err := strconv.Atoi(selector); err == nil {
		if index < 1 || index > len(specs) {
			return persona.Spec{}, fmt.Errorf("--member index %d out of range 1..%d", index, len(specs))
		}
		return specs[index-1], nil
	}
	needle := strings.ToLower(selector)
	matches := make([]int, 0, 2)
	for i, spec := range specs {
		if strings.Contains(strings.ToLower(jurorMemberLabel(spec)), needle) {
			matches = append(matches, i)
		}
	}
	switch len(matches) {
	case 1:
		return specs[matches[0]], nil
	case 0:
		return persona.Spec{}, fmt.Errorf("--member %q matches no pool member; use --list", selector)
	default:
		lines := make([]string, 0, len(matches))
		for _, i := range matches {
			lines = append(lines, fmt.Sprintf("%d %s", i+1, jurorMemberLabel(specs[i])))
		}
		return persona.Spec{}, fmt.Errorf("--member %q matches %d pool members:\n%s", selector, len(matches), strings.Join(lines, "\n"))
	}
}

func readJurorTranscript(path string, memberLabel string) ([]jurorTranscriptEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open transcript %s: %w", path, err)
	}
	defer f.Close()
	entries := make([]jurorTranscriptEntry, 0)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry jurorTranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse transcript %s line %d: %w", path, lineNo, err)
		}
		if entry.Role != "user" && entry.Role != "assistant" {
			return nil, fmt.Errorf("transcript %s line %d: role must be user or assistant, got %q", path, lineNo, entry.Role)
		}
		if entry.Member != memberLabel {
			return nil, fmt.Errorf("transcript %s line %d is for member %q, not %q", path, lineNo, entry.Member, memberLabel)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read transcript %s: %w", path, err)
	}
	return entries, nil
}

func appendJurorTranscript(path string, entries []jurorTranscriptEntry) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open transcript %s: %w", path, err)
	}
	defer f.Close()
	for _, entry := range entries {
		encoded, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("encode transcript entry: %w", err)
		}
		if _, err := fmt.Fprintln(f, string(encoded)); err != nil {
			return fmt.Errorf("write transcript %s: %w", path, err)
		}
	}
	return nil
}
