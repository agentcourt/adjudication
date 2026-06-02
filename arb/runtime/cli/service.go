package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"adjudication/arb/runtime/service"
)

func RunService(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("service", flag.ContinueOnError)
	fs.SetOutput(stderr)
	listen := fs.String("listen", "127.0.0.1:19770", "Service listen address")
	registryDir := fs.String("registry-dir", "", "Case registry directory")
	outputRoot := fs.String("out-root", "", "Case output root")
	aarBin := fs.String("aar-bin", "", "Path to aar binary used for child cases")
	commonRoot := fs.String("common-root", defaultCommonRoot(), "Path to sibling shared common directory")
	enginePath := fs.String("engine", defaultEnginePath(), "Lean engine binary")
	bearerToken := fs.String("bearer-token", "", "Optional bearer token required for service requests")
	startupWait := fs.Duration("case-startup-timeout", 30*time.Second, "Maximum time to wait for a child case API health response")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: aar service [options]\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	outRoot := strings.TrimSpace(*outputRoot)
	if outRoot == "" {
		outRoot = filepath.Join("out", "service")
	}
	regDir := strings.TrimSpace(*registryDir)
	if regDir == "" {
		regDir = filepath.Join(outRoot, "registry")
	}
	bin := strings.TrimSpace(*aarBin)
	if bin == "" {
		self, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve current executable: %w", err)
		}
		bin = self
	}
	commonRootResolved, err := filepath.Abs(strings.TrimSpace(*commonRoot))
	if err != nil {
		return fmt.Errorf("resolve --common-root: %w", err)
	}
	cfg := service.Config{
		ListenAddr:  strings.TrimSpace(*listen),
		RegistryDir: regDir,
		OutputRoot:  outRoot,
		AARBin:      bin,
		CommonRoot:  commonRootResolved,
		EnginePath:  strings.TrimSpace(*enginePath),
		BearerToken: strings.TrimSpace(*bearerToken),
		StartupWait: *startupWait,
	}
	server, err := service.New(cfg)
	if err != nil {
		return err
	}
	ln, err := server.Listen()
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "aar service listening on http://%s\n", ln.Addr().String())
	return server.Serve(context.Background(), ln)
}
