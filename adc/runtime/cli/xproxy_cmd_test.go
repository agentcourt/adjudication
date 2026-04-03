package cli

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRootUsageIncludesXProxy(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(help) error = %v", err)
	}
	if !strings.Contains(stdout.String(), "xproxy") {
		t.Fatalf("root help missing xproxy: %q", stdout.String())
	}
}

func TestHelpXProxy(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Run([]string{"help", "xproxy"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(help xproxy) error = %v", err)
	}
	if !strings.Contains(stderr.String(), "Usage: adc xproxy") {
		t.Fatalf("xproxy help missing usage: %q", stderr.String())
	}
}

func TestStartStandaloneXProxyServesHealthz(t *testing.T) {
	configPath := writeXProxyTestConfig(t)
	port := freeTCPPort(t)
	server, err := startStandaloneXProxy(configPath, port, nil, false, false)
	if err != nil {
		t.Fatalf("startStandaloneXProxy error = %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if err := server.Close(); err != nil {
					t.Fatalf("server.Close() error = %v", err)
				}
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = server.Close()
	t.Fatalf("xproxy did not become healthy at %s", url)
}

func writeXProxyTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "xproxy.json")
	raw := []byte("{\n  \"endpoints\": {\n    \"openai\": {\n      \"url\": \"https://example.com/v1/responses\",\n      \"api\": \"openai-responses\",\n      \"apiKeyEnv\": \"OPENAI_API_KEY\"\n    }\n  }\n}\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
