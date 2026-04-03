package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	xproxy "adjudication/common/xproxy"
)

func RunXProxy(args []string, stdout io.Writer, stderr io.Writer) error {
	_ = stdout
	var fs *flag.FlagSet
	fs = newFlagSet("xproxy", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc xproxy [options]\n\n")
		fs.PrintDefaults()
	})
	configPathFlag := fs.String("config", "", "Path to xproxy config JSON")
	portFlag := fs.Int("port", 0, "Port to listen on")
	maxOutputTokensFlag := fs.Int("max-output-tokens", 0, "Override max output tokens for upstream requests")
	logStreamEvents := fs.Bool("log-stream-events", false, "Log synthetic stream events")
	logPayloadSummary := fs.Bool("log-payload-summary", false, "Log request payload summaries")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("xproxy accepts no positional arguments")
	}
	configPath, err := resolveStandaloneXProxyConfigPath(strings.TrimSpace(*configPathFlag))
	if err != nil {
		return err
	}
	port, err := resolveStandaloneXProxyPort(*portFlag)
	if err != nil {
		return err
	}
	maxOutputTokens, err := resolveStandaloneXProxyMaxOutputTokens(*maxOutputTokensFlag)
	if err != nil {
		return err
	}
	server, err := startStandaloneXProxy(configPath, port, maxOutputTokens, *logStreamEvents, *logPayloadSummary)
	if err != nil {
		return err
	}
	return waitForXProxySignal(server)
}

func resolveStandaloneXProxyConfigPath(configPath string) (string, error) {
	if configPath != "" {
		return configPath, nil
	}
	return resolveXProxyConfigPath()
}

func resolveStandaloneXProxyPort(port int) (int, error) {
	if port < 0 {
		return 0, fmt.Errorf("--port must be >= 0")
	}
	if port > 0 {
		return port, nil
	}
	return xproxyPortFromEnv()
}

func resolveStandaloneXProxyMaxOutputTokens(value int) (*int, error) {
	if value < 0 {
		return nil, fmt.Errorf("--max-output-tokens must be >= 0")
	}
	if value == 0 {
		return nil, nil
	}
	resolved := value
	return &resolved, nil
}

func startStandaloneXProxy(configPath string, port int, maxOutputTokens *int, logStreamEvents bool, logPayloadSummary bool) (io.Closer, error) {
	if xproxyHealthy(port) {
		return nil, fmt.Errorf("xproxy already healthy on 127.0.0.1:%d", port)
	}
	return xproxy.StartXProxyServer(xproxy.XProxyOptions{
		ConfigPath:        configPath,
		Port:              port,
		MaxOutputTokens:   maxOutputTokens,
		LogStreamEvents:   logStreamEvents,
		LogPayloadSummary: logPayloadSummary,
	})
}

func waitForXProxySignal(server io.Closer) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	<-signals
	return server.Close()
}
