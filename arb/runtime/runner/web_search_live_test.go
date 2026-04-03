package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"adjudication/common/acp"
)

func TestACPAgentCanUseWebSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live ACP web-search test in short mode")
	}
	if strings.TrimSpace(os.Getenv("LIVE_TESTS")) == "" {
		t.Skip("set LIVE_TESTS=1 to run live integration tests")
	}

	commonRoot, err := filepath.Abs(filepath.Join("..", "..", "..", "common"))
	if err != nil {
		t.Fatalf("resolve common root: %v", err)
	}
	server, err := maybeStartXProxy(filepath.Join(commonRoot, "etc", "xproxy.json"), 18459)
	if err != nil {
		t.Fatalf("start xproxy: %v", err)
	}
	if server != nil {
		defer server.Close()
	}
	homeDir, cleanup, err := prepareEphemeralPIHome(commonRoot, DefaultAttorneyModel)
	if err != nil {
		t.Fatalf("prepare PI home: %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup PI home: %v", err)
		}
	}()

	client, err := acp.NewClient(acp.Config{
		Command: filepath.Join(commonRoot, "pi-container", "acp-podman.sh"),
		Cwd:     t.TempDir(),
		Env: []string{
			"PI_CONTAINER_HOME_DIR=" + homeDir,
			"PI_ACP_CLIENT_TOOLS=[]",
		},
	})
	if err != nil {
		t.Fatalf("new ACP client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ACP client: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if _, err := client.Initialize(ctx, 1); err != nil {
		t.Fatalf("initialize ACP client: %v\nstderr:\n%s", err, client.Stderr())
	}
	session, err := client.NewSession(ctx, "/home/user")
	if err != nil {
		t.Fatalf("create ACP session: %v\nstderr:\n%s", err, client.Stderr())
	}

	var mu sync.Mutex
	var sawSearch bool
	var sawURL bool
	var transcript strings.Builder
	var updates []string
	unsub := client.OnNotification(func(note acp.Notification) {
		if note.Method != "session/update" {
			return
		}
		update := mapAny(note.Params["update"])
		sessionUpdate := mapString(update["sessionUpdate"])
		switch sessionUpdate {
		case "agent_message_chunk", "agent_thought_chunk":
			text := mapString(mapAny(update["content"])["text"])
			mu.Lock()
			transcript.WriteString(text)
			if strings.Contains(text, "https://") || strings.Contains(text, "http://") {
				sawURL = true
			}
			mu.Unlock()
		case "tool_call", "tool_call_update":
			title := strings.ToLower(mapString(update["title"]))
			rawInput := strings.ToLower(marshalInline(update["rawInput"]))
			rawOutput := strings.ToLower(marshalInline(update["rawOutput"]))
			line := sessionUpdate + " title=" + title + " raw_input=" + rawInput + " raw_output=" + rawOutput
			mu.Lock()
			updates = append(updates, line)
			if strings.Contains(title, "search") ||
				strings.Contains(rawInput, "search") ||
				strings.Contains(rawOutput, "search") ||
				strings.Contains(title, "web") ||
				strings.Contains(rawInput, "web_search") ||
				strings.Contains(rawOutput, "web_search") {
				sawSearch = true
			}
			if strings.Contains(rawInput, "https://") || strings.Contains(rawInput, "http://") ||
				strings.Contains(rawOutput, "https://") || strings.Contains(rawOutput, "http://") {
				sawURL = true
			}
			mu.Unlock()
		}
	})
	defer unsub()

	_, err = client.Prompt(ctx, acp.PromptRequest{
		SessionID: session.SessionID,
		Prompt: []acp.TextBlock{{
			Type: "text",
			Text: "Use web search before answering. Find the current title of the front page for OpenAI. Return one sentence with the title and the exact URL you used.",
		}},
	})
	if err != nil {
		t.Fatalf("prompt ACP session: %v\nstderr:\n%s", err, client.Stderr())
	}

	mu.Lock()
	defer mu.Unlock()
	if !sawSearch {
		t.Fatalf("did not observe web-search activity in ACP session updates\nupdates:\n%s\ntranscript:\n%s\nstderr:\n%s", strings.Join(updates, "\n"), transcript.String(), client.Stderr())
	}
	if !sawURL {
		t.Fatalf("did not observe a URL in ACP session output\nupdates:\n%s\ntranscript:\n%s\nstderr:\n%s", strings.Join(updates, "\n"), transcript.String(), client.Stderr())
	}
}
