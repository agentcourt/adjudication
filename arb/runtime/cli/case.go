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

	"adjudication/arb/runtime/lean"
	"adjudication/arb/runtime/runner"
	"adjudication/arb/runtime/spec"
)

func RunCase(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = flag.NewFlagSet("case", flag.ContinueOnError)
	fs.SetOutput(stderr)
	complaintPath := fs.String("complaint", "", "Complaint markdown file")
	outDir := fs.String("out-dir", "", "Output directory")
	policyPath := fs.String("policy", "", "Policy JSON file. Default: ./etc/policy.json when present")
	councilSize := fs.Int("council-size", 0, "Override policy council_size")
	evidenceStandard := fs.String("evidence-standard", "", "Override policy evidence_standard")
	commonRoot := fs.String("common-root", defaultCommonRoot(), "Path to the sibling shared common directory")
	legacyCommonRoot := fs.String("agentcourt-root", "", "Deprecated alias for --common-root")
	councilPool := fs.String("council-pool", "", "Council model/persona pool file. Default: <common-root>/etc/personas.csv")
	acpCommand := fs.String("acp-command", "", "ACP command. Default: <common-root>/pi-container/acp-podman.sh")
	xproxyConfig := fs.String("xproxy-config", "", "xproxy config path. Default: <common-root>/etc/xproxy.json")
	xproxyPort := fs.Int("xproxy-port", 18459, "xproxy port")
	timeoutSeconds := fs.Int("timeout-seconds", 0, "Override runtime council LLM timeout in seconds")
	acpTimeoutSeconds := fs.Int("acp-timeout-seconds", 0, "Override runtime attorney ACP timeout in seconds")
	maxResponseBytes := fs.Int("max-response-bytes", 0, "Override runtime max parsed response bytes")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", 0, "Override runtime invalid-attempt limit")
	enginePath := fs.String("engine", defaultEnginePath(), "Lean engine binary")
	runID := fs.String("run-id", "", "Run ID override")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: aar case --complaint FILE --out-dir DIR\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *complaintPath == "" || *outDir == "" {
		return fmt.Errorf("--complaint and --out-dir are required")
	}
	raw, err := os.ReadFile(*complaintPath)
	if err != nil {
		return fmt.Errorf("read complaint: %w", err)
	}
	complaint, err := spec.ParseComplaintMarkdown(string(raw))
	if err != nil {
		return err
	}
	commonRootValue := strings.TrimSpace(*commonRoot)
	if strings.TrimSpace(*legacyCommonRoot) != "" {
		commonRootValue = strings.TrimSpace(*legacyCommonRoot)
	}
	commonRootResolved, err := filepath.Abs(commonRootValue)
	if err != nil {
		return fmt.Errorf("resolve --common-root: %w", err)
	}
	policy, err := loadCasePolicy(*policyPath)
	if err != nil {
		return err
	}
	if *councilSize > 0 {
		policy.CouncilSize = *councilSize
	}
	if strings.TrimSpace(*evidenceStandard) != "" {
		policy.EvidenceStandard = strings.TrimSpace(*evidenceStandard)
	}
	if err := runner.ValidatePolicy(policy); err != nil {
		return err
	}
	runtimeLimits := runner.DefaultRuntimeLimits()
	if *timeoutSeconds > 0 {
		runtimeLimits.CouncilLLMTimeoutSeconds = *timeoutSeconds
	}
	if *acpTimeoutSeconds > 0 {
		runtimeLimits.AttorneyACPTimeoutSeconds = *acpTimeoutSeconds
	}
	if *maxResponseBytes > 0 {
		runtimeLimits.MaxResponseBytes = *maxResponseBytes
	}
	if *invalidAttemptLimit > 0 {
		runtimeLimits.InvalidAttemptLimit = *invalidAttemptLimit
	}
	if err := runner.ValidateRuntimeLimits(runtimeLimits); err != nil {
		return err
	}
	councilPoolPath := strings.TrimSpace(*councilPool)
	if councilPoolPath == "" {
		councilPoolPath = filepath.Join(commonRootResolved, "etc", "personas.csv")
	}
	xproxyConfigPath := strings.TrimSpace(*xproxyConfig)
	if xproxyConfigPath == "" {
		xproxyConfigPath = filepath.Join(commonRootResolved, "etc", "xproxy.json")
	}
	acpCommandPath := strings.TrimSpace(*acpCommand)
	if acpCommandPath == "" {
		acpCommandPath = filepath.Join(commonRootResolved, "pi-container", "acp-podman.sh")
	}
	effectiveRunID := strings.TrimSpace(*runID)
	if effectiveRunID == "" {
		effectiveRunID = fmt.Sprintf("run-%d", time.Now().UTC().UnixNano())
	}
	cfg := runner.Config{
		RunID:            effectiveRunID,
		ComplaintPath:    *complaintPath,
		OutputDir:        *outDir,
		CommonRoot:       commonRootResolved,
		CouncilPoolPath:  councilPoolPath,
		Policy:           policy,
		Runtime:          runtimeLimits,
		XProxyConfigPath: xproxyConfigPath,
		XProxyPort:       *xproxyPort,
		ACPCommand:       acpCommandPath,
		Engine:           lean.New([]string{*enginePath}),
	}
	if _, err := runner.Run(context.Background(), cfg, complaint); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, *outDir)
	return err
}

func defaultEnginePath() string {
	exe, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(exe), "aarengine")
	}
	return filepath.FromSlash(".bin/aarengine")
}

func loadCasePolicy(flagValue string) (runner.Policy, error) {
	path := strings.TrimSpace(flagValue)
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return runner.Policy{}, fmt.Errorf("resolve current working directory: %w", err)
		}
		path = filepath.Join(cwd, "etc", "policy.json")
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return runner.DefaultPolicy(), nil
			}
			return runner.Policy{}, fmt.Errorf("stat default policy: %w", err)
		}
	}
	policy, err := runner.LoadPolicyFile(path)
	if err != nil {
		return runner.Policy{}, fmt.Errorf("load policy %s: %w", path, err)
	}
	return policy, nil
}

func defaultCommonRoot() string {
	cwd, err := os.Getwd()
	if err == nil {
		return locateCommonRootFrom(cwd)
	}
	return filepath.FromSlash("../common")
}

func locateCommonRootFrom(start string) string {
	base := filepath.Clean(strings.TrimSpace(start))
	if base == "" {
		return filepath.FromSlash("../common")
	}
	if !filepath.IsAbs(base) {
		if absBase, err := filepath.Abs(base); err == nil {
			base = absBase
		}
	}
	for {
		candidate := filepath.Join(base, "common")
		if fileExists(filepath.Join(candidate, "etc", "xproxy.json")) || fileExists(filepath.Join(candidate, "etc", "personas.csv")) {
			return candidate
		}
		if filepath.Base(base) == "common" && (fileExists(filepath.Join(base, "etc", "xproxy.json")) || fileExists(filepath.Join(base, "etc", "personas.csv"))) {
			return base
		}
		next := filepath.Dir(base)
		if next == base {
			break
		}
		base = next
	}
	return filepath.Clean(filepath.Join(start, filepath.FromSlash("../common")))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
