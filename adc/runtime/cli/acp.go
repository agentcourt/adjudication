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

	"adjudication/common/acp"
)

type stringListFlag []string
type stringMapFlag map[string]string

func (s *stringListFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringListFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func (m *stringMapFlag) String() string {
	if m == nil || len(*m) == 0 {
		return ""
	}
	parts := make([]string, 0, len(*m))
	for k, v := range *m {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}

func (m *stringMapFlag) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("expected METHOD=TEXT")
	}
	method := strings.TrimSpace(parts[0])
	if method == "" {
		return fmt.Errorf("method is required")
	}
	if *m == nil {
		*m = map[string]string{}
	}
	(*m)[method] = parts[1]
	return nil
}

func RunACP(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("acp", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc acp --prompt <text> [options]\n\n")
		fs.PrintDefaults()
	})
	command := fs.String("command", "", "ACP server command")
	cwd := fs.String("cwd", "", "Working directory for session/new")
	prompt := fs.String("prompt", "", "Prompt text")
	promptFile := fs.String("prompt-file", "", "Path to prompt text file")
	timeoutSeconds := fs.Int("timeout-seconds", 60, "Request timeout in seconds")
	var argList stringListFlag
	var envList stringListFlag
	var extText stringMapFlag
	fs.Var(&argList, "arg", "ACP server argument; repeat as needed")
	fs.Var(&envList, "env", "Environment override KEY=VALUE; repeat as needed")
	fs.Var(&extText, "ext-text", "Register ACP custom method METHOD=TEXT; repeat as needed")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	repoCwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	commandValue := strings.TrimSpace(*command)
	argValues := []string(argList)
	defaultACPServer := filepath.Join(repoCwd, defaultACPServerPath())
	if commandValue == "" {
		if !fileExists(defaultACPServer) {
			return fmt.Errorf("cannot find %s; run from the repository root or pass --command explicitly", defaultACPServer)
		}
		commandValue = defaultACPServer
	}
	envValues := []string(envList)
	xproxyServer, err := maybeStartXProxy(true)
	if err != nil {
		return err
	}
	if xproxyServer != nil {
		defer xproxyServer.Close()
	}
	sessionCwd := strings.TrimSpace(*cwd)
	if sessionCwd == "" {
		var err error
		sessionCwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}
	}
	sessionCwd, err = filepath.Abs(sessionCwd)
	if err != nil {
		return fmt.Errorf("resolve cwd: %w", err)
	}
	promptText, err := loadPromptText(strings.TrimSpace(*prompt), strings.TrimSpace(*promptFile))
	if err != nil {
		return err
	}
	if strings.TrimSpace(promptText) == "" {
		return fmt.Errorf("prompt text is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSeconds)*time.Second)
	defer cancel()

	client, err := acp.NewClient(acp.Config{
		Command: commandValue,
		Args:    argValues,
		Cwd:     sessionCwd,
		Env:     envValues,
	})
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	for method, text := range extText {
		responseText := text
		client.HandleMethod(method, func(_ context.Context, params map[string]any) (map[string]any, error) {
			encoded, err := json.Marshal(params)
			if err != nil {
				return nil, fmt.Errorf("marshal ext method params: %w", err)
			}
			fmt.Fprintf(stderr, "acp ext_method=%s params=%s\n", method, string(encoded))
			return map[string]any{"text": responseText}, nil
		})
	}

	unsub := client.OnNotification(func(note acp.Notification) {
		if note.Method != "session/update" {
			return
		}
		update, _ := note.Params["update"].(map[string]any)
		if update == nil {
			return
		}
		switch stringValue(update["sessionUpdate"]) {
		case "agent_message_chunk", "agent_thought_chunk":
			content, _ := update["content"].(map[string]any)
			text := stringValue(content["text"])
			if text != "" {
				_, _ = io.WriteString(stdout, text)
			}
		case "tool_call":
			fmt.Fprintf(
				stderr,
				"tool_call id=%s title=%s status=%s\n",
				stringValue(update["toolCallId"]),
				stringValue(update["title"]),
				stringValue(update["status"]),
			)
		case "tool_call_update":
			status := stringValue(update["status"])
			if status != "" {
				fmt.Fprintf(stderr, "tool_call_update id=%s status=%s\n", stringValue(update["toolCallId"]), status)
			}
			if contentItems, ok := update["content"].([]any); ok {
				for _, item := range contentItems {
					itemMap, _ := item.(map[string]any)
					content, _ := itemMap["content"].(map[string]any)
					text := stringValue(content["text"])
					if text != "" {
						fmt.Fprintf(stderr, "tool_call_output id=%s text=%s\n", stringValue(update["toolCallId"]), text)
					}
				}
			}
		}
	})
	defer unsub()

	initResp, err := client.Initialize(ctx, 1)
	if err != nil {
		return err
	}
	fmt.Fprintf(
		stderr,
		"acp agent name=%s title=%s version=%s\n",
		initResp.AgentInfo.Name,
		initResp.AgentInfo.Title,
		initResp.AgentInfo.Version,
	)

	sessionResp, err := client.NewSession(ctx, sessionCwd)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "acp session id=%s cwd=%s\n", sessionResp.SessionID, sessionCwd)

	promptResp, err := client.Prompt(ctx, acp.PromptRequest{
		SessionID: sessionResp.SessionID,
		Prompt:    []acp.TextBlock{{Type: "text", Text: promptText}},
	})
	if err != nil {
		return err
	}
	_, _ = io.WriteString(stdout, "\n")
	fmt.Fprintf(stderr, "acp stop_reason=%s\n", promptResp.StopReason)
	return nil
}

func hasEnvKey(env []string, key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}

func loadPromptText(prompt string, promptFile string) (string, error) {
	if prompt != "" && promptFile != "" {
		return "", fmt.Errorf("use either --prompt or --prompt-file")
	}
	if prompt != "" {
		return prompt, nil
	}
	if promptFile == "" {
		return "", nil
	}
	raw, err := os.ReadFile(promptFile)
	if err != nil {
		return "", fmt.Errorf("read prompt file: %w", err)
	}
	return string(raw), nil
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}
