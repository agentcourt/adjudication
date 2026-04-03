package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"adjudication/common/openai"
	xproxy "adjudication/common/xproxy"
)

func maybeStartXProxy(required bool) (io.Closer, error) {
	if !required && !envBool("PI_CONTAINER_USE_XPROXY") {
		return nil, nil
	}
	port, err := xproxyPortFromEnv()
	if err != nil {
		return nil, err
	}
	var server io.Closer
	if xproxyHealthy(port) {
		server = nil
	} else {
		configPath, err := resolveXProxyConfigPath()
		if err != nil {
			return nil, err
		}
		server, err = xproxy.StartXProxyServer(xproxy.XProxyOptions{
			ConfigPath: configPath,
			Port:       port,
		})
		if err != nil {
			return nil, err
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if xproxyHealthy(port) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !xproxyHealthy(port) {
			_ = server.Close()
			return nil, fmt.Errorf("xproxy did not become healthy on 127.0.0.1:%d", port)
		}
	}
	return server, nil
}

func newXProxyClient(online bool, timeout time.Duration) (*openai.Client, error) {
	port, err := xproxyPortFromEnv()
	if err != nil {
		return nil, err
	}
	return openai.New("xproxy", fmt.Sprintf("http://127.0.0.1:%d/v1", port), online, timeout)
}

func envBool(name string) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return false
	}
	if raw == "1" {
		return true
	}
	parsed, err := strconv.ParseBool(raw)
	return err == nil && parsed
}

func xproxyPortFromEnv() (int, error) {
	raw := strings.TrimSpace(os.Getenv("PI_CONTAINER_XPROXY_PORT"))
	if raw == "" {
		return xproxy.DefaultPort, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 {
		return 0, fmt.Errorf("PI_CONTAINER_XPROXY_PORT must be a positive integer")
	}
	return port, nil
}

func xproxyHealthy(port int) bool {
	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func resolveXProxyConfigPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("PI_CONTAINER_XPROXY_CONFIG")); path != "" {
		return path, nil
	}
	if path := firstExistingPath(defaultXProxyConfigPath(), "etc/xproxy.json"); path != "" {
		return path, nil
	}
	return "", fmt.Errorf("cannot find xproxy config; looked for %s and etc/xproxy.json, or set PI_CONTAINER_XPROXY_CONFIG", defaultXProxyConfigPath())
}
