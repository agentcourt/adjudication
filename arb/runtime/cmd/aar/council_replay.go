package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"adjudication/arb/runtime/localrun"
)

type councilReplaySummary struct {
	Status          string `json:"status"`
	Basis           string `json:"basis,omitempty"`
	CaseID          string `json:"case_id,omitempty"`
	MemberID        string `json:"member_id,omitempty"`
	Model           string `json:"model,omitempty"`
	Vote            string `json:"vote,omitempty"`
	Rationale       string `json:"rationale,omitempty"`
	OutputDir       string `json:"out_dir,omitempty"`
	SourceOutputDir string `json:"source_output_dir,omitempty"`
	SnapshotDir     string `json:"snapshot_dir,omitempty"`
	Error           string `json:"error,omitempty"`
}

func runCouncilReplay(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("council-replay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	basis := fs.String("basis", "", "Replay basis: reconstructed_first_round or snapshot")
	sourceOutput := fs.String("source-output", "", "Extracted AAR output directory, or a run directory containing aar-output")
	snapshot := fs.String("snapshot", "", "Council snapshot directory or input.json, required for --basis snapshot")
	promptDir := fs.String("prompt-dir", "", "Prompt directory override")
	config := fs.String("config", "", "Single council JSON request-spec record")
	outDir := fs.String("out-dir", "", "Replay output directory")
	memberID := fs.String("member-id", "", "Council member id for reconstructed_first_round. Default: C1")
	caseAPIAddr := fs.String("caseapi-addr", "127.0.0.1:0", "Private replay case API listen address")
	mcpListenAddr := fs.String("mcp-listen", "127.0.0.1:0", "Replay MCP listen address")
	mcpBearerToken := fs.String("mcp-bearer-token", "", "MCP bearer token. Default: generated")
	timeoutSeconds := fs.Int("timeout-seconds", localrun.DefaultRunCouncilTimeoutSeconds, "Replay council timeout seconds")
	councilInstructions := fs.String("council-instructions", localrun.DefaultCouncilInstructionsPath(), "Pi council instruction template")
	podmanCommand := fs.String("podman", localrun.DefaultPodmanCommand, "Podman command")
	piImage := fs.String("pi-image", "", "Pi container image")
	piMCPAdapter := fs.String("pi-mcp-adapter", "", "Pi MCP adapter package")
	podmanMCPHost := fs.String("podman-mcp-host", "", "Host name used by Podman containers to reach MCP")
	councilOutputLimitBytes := fs.Int64("council-output-limit-bytes", localrun.DefaultCouncilOutputLimitBytes, "Total stdout plus stderr byte limit for the Pi council agent")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: aar council-replay --basis BASIS --source-output DIR --config FILE --out-dir DIR\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return reportCouncilReplayError(stdout, localrun.CouncilReplayResult{}, err)
	}
	result, err := localrun.ReplayCouncil(ctx, localrun.CouncilReplayOptions{
		Basis:                   *basis,
		SourceOutputDir:         *sourceOutput,
		SnapshotDir:             *snapshot,
		PromptDir:               *promptDir,
		ModelConfigPath:         *config,
		OutputDir:               *outDir,
		MemberID:                *memberID,
		CaseAPIAddr:             *caseAPIAddr,
		MCPListenAddr:           *mcpListenAddr,
		MCPBearerToken:          *mcpBearerToken,
		CouncilTimeoutSeconds:   *timeoutSeconds,
		CouncilInstructionsPath: *councilInstructions,
		PodmanCommand:           *podmanCommand,
		PiImage:                 *piImage,
		PiMCPAdapter:            *piMCPAdapter,
		PodmanMCPHost:           *podmanMCPHost,
		CouncilOutputLimitBytes: *councilOutputLimitBytes,
		Log:                     stderr,
	})
	if err != nil {
		return reportCouncilReplayError(stdout, result, err)
	}
	return writeCouncilReplaySummary(stdout, buildCouncilReplaySummary(result))
}

func buildCouncilReplaySummary(result localrun.CouncilReplayResult) councilReplaySummary {
	return councilReplaySummary{
		Status:          strings.TrimSpace(result.Status),
		Basis:           strings.TrimSpace(result.Basis),
		CaseID:          strings.TrimSpace(result.CaseID),
		MemberID:        strings.TrimSpace(result.MemberID),
		Model:           strings.TrimSpace(result.Model),
		Vote:            strings.TrimSpace(result.Vote),
		Rationale:       strings.TrimSpace(result.Rationale),
		OutputDir:       strings.TrimSpace(result.OutputDir),
		SourceOutputDir: strings.TrimSpace(result.SourceOutputDir),
		SnapshotDir:     strings.TrimSpace(result.SnapshotDir),
		Error:           strings.TrimSpace(result.Error),
	}
}

func reportCouncilReplayError(stdout io.Writer, result localrun.CouncilReplayResult, err error) error {
	summary := buildCouncilReplaySummary(result)
	summary.Status = "error"
	if strings.TrimSpace(summary.Error) == "" {
		summary.Error = strings.TrimSpace(err.Error())
	}
	if writeErr := writeCouncilReplaySummary(stdout, summary); writeErr != nil {
		return writeErr
	}
	return &reportedError{err: err}
}

func writeCouncilReplaySummary(w io.Writer, summary councilReplaySummary) error {
	raw, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal council replay summary: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(raw)); err != nil {
		return fmt.Errorf("write council replay summary: %w", err)
	}
	return nil
}
