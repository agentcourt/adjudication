package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"adjudication/arb/runtime/localrun"
	"adjudication/arb/runtime/proceeding"
)

const jurorReplayMetadataSchemaVersion = "aar.juror-replay.v0"

type jurorReplaySelection struct {
	Basis       string
	SnapshotDir string
	MemberID    string
}

type jurorReplaySummary struct {
	Status          string         `json:"status"`
	Basis           string         `json:"basis,omitempty"`
	CaseID          string         `json:"case_id,omitempty"`
	MemberID        string         `json:"member_id,omitempty"`
	Model           string         `json:"model,omitempty"`
	Vote            string         `json:"vote,omitempty"`
	Rationale       string         `json:"rationale,omitempty"`
	OutputDir       string         `json:"out_dir,omitempty"`
	SourceOutputDir string         `json:"source_output_dir,omitempty"`
	SnapshotDir     string         `json:"snapshot_dir,omitempty"`
	ModelConfigPath string         `json:"model_config_path,omitempty"`
	PersonaPath     string         `json:"persona_path,omitempty"`
	ToolCallCount   int            `json:"tool_call_count,omitempty"`
	Error           string         `json:"error,omitempty"`
	ErrorDetails    map[string]any `json:"error_details,omitempty"`
}

type jurorReplayMetadata struct {
	SchemaVersion   string         `json:"schema_version"`
	CreatedAt       string         `json:"created_at"`
	Status          string         `json:"status"`
	Basis           string         `json:"basis,omitempty"`
	CaseID          string         `json:"case_id,omitempty"`
	MemberID        string         `json:"member_id,omitempty"`
	Model           string         `json:"model,omitempty"`
	Vote            string         `json:"vote,omitempty"`
	Rationale       string         `json:"rationale,omitempty"`
	OutputDir       string         `json:"out_dir,omitempty"`
	SourceOutputDir string         `json:"source_output_dir,omitempty"`
	SnapshotDir     string         `json:"snapshot_dir,omitempty"`
	ModelConfigPath string         `json:"model_config_path,omitempty"`
	PersonaPath     string         `json:"persona_path,omitempty"`
	PersonaSHA256   string         `json:"persona_sha256,omitempty"`
	ToolCallCount   int            `json:"tool_call_count"`
	Error           string         `json:"error,omitempty"`
	ErrorDetails    map[string]any `json:"error_details,omitempty"`
}

func runJurorReplay(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("juror-replay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	basis := fs.String("basis", "", "Replay basis: reconstructed_first_round or snapshot. Default: use a matching snapshot when available")
	sourceOutput := fs.String("source-output", "", "Extracted AAR output directory, or a run directory containing aar-output")
	snapshot := fs.String("snapshot", "", "Council snapshot directory or input.json")
	promptDir := fs.String("prompt-dir", "", "Prompt directory override")
	modelConfig := fs.String("model-config", "", "Single JSON request-spec record")
	personaPath := fs.String("persona", "", "Persona file for the replay juror")
	outDir := fs.String("out-dir", "", "Replay output directory")
	memberID := fs.String("member-id", "", "Council member id used to find a snapshot or reconstruct a first-round replay")
	caseAPIAddr := fs.String("caseapi-addr", "127.0.0.1:0", "Private replay case API listen address")
	mcpListenAddr := fs.String("mcp-listen", "127.0.0.1:0", "Replay MCP listen address")
	mcpBearerToken := fs.String("mcp-bearer-token", "", "MCP bearer token. Default: generated")
	timeoutSeconds := fs.Int("timeout-seconds", localrun.DefaultRunCouncilTimeoutSeconds, "Replay council timeout seconds")
	councilInstructions := fs.String("council-instructions", localrun.DefaultCouncilInstructionsPath(), "Pi council instruction template")
	podmanCommand := fs.String("podman", localrun.DefaultPodmanCommand, "Podman command")
	piImage := fs.String("pi-image", "", "Pi container image")
	piMCPAdapter := fs.String("pi-mcp-adapter", "", "Pi MCP adapter path or package source")
	podmanMCPHost := fs.String("podman-mcp-host", "", "Host name used by Podman containers to reach MCP")
	councilOutputLimitBytes := fs.Int64("council-output-limit-bytes", localrun.DefaultCouncilOutputLimitBytes, "Total stdout plus stderr byte limit for the Pi council agent")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: aar juror-replay --source-output DIR --model-config FILE --persona FILE --out-dir DIR\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return reportJurorReplayError(stdout, localrun.CouncilReplayResult{}, *modelConfig, *personaPath, err)
	}
	if err := validateJurorReplayFlags(*sourceOutput, *modelConfig, *personaPath, *outDir); err != nil {
		return reportJurorReplayError(stdout, localrun.CouncilReplayResult{}, *modelConfig, *personaPath, err)
	}
	selection, err := resolveJurorReplaySelection(*sourceOutput, *basis, *snapshot, *memberID)
	if err != nil {
		return reportJurorReplayError(stdout, localrun.CouncilReplayResult{}, *modelConfig, *personaPath, err)
	}
	result, err := localrun.ReplayCouncil(ctx, localrun.CouncilReplayOptions{
		Basis:                   selection.Basis,
		SourceOutputDir:         *sourceOutput,
		SnapshotDir:             selection.SnapshotDir,
		PromptDir:               *promptDir,
		ModelConfigPath:         *modelConfig,
		PersonaPath:             *personaPath,
		OutputDir:               *outDir,
		MemberID:                selection.MemberID,
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
	metadataErr := writeJurorReplayMetadata(*outDir, result, *modelConfig, *personaPath, err)
	if err != nil {
		if metadataErr != nil {
			err = errors.Join(err, metadataErr)
		}
		return reportJurorReplayError(stdout, result, *modelConfig, *personaPath, err)
	}
	if metadataErr != nil {
		return reportJurorReplayError(stdout, result, *modelConfig, *personaPath, metadataErr)
	}
	return writeJurorReplaySummary(stdout, buildJurorReplaySummary(result, *modelConfig, *personaPath, ""))
}

func validateJurorReplayFlags(sourceOutput string, modelConfig string, personaPath string, outDir string) error {
	if strings.TrimSpace(sourceOutput) == "" {
		return fmt.Errorf("--source-output is required")
	}
	if strings.TrimSpace(modelConfig) == "" {
		return fmt.Errorf("--model-config is required")
	}
	if strings.TrimSpace(personaPath) == "" {
		return fmt.Errorf("--persona is required")
	}
	if strings.TrimSpace(outDir) == "" {
		return fmt.Errorf("--out-dir is required")
	}
	return nil
}

func resolveJurorReplaySelection(sourceOutput string, basis string, snapshot string, memberID string) (jurorReplaySelection, error) {
	basis = strings.TrimSpace(basis)
	snapshot = strings.TrimSpace(snapshot)
	memberID = strings.TrimSpace(memberID)
	switch basis {
	case "":
	case proceeding.CouncilReplayBasisReconstructed:
		if snapshot != "" {
			return jurorReplaySelection{}, fmt.Errorf("--snapshot cannot be used with --basis %s", proceeding.CouncilReplayBasisReconstructed)
		}
		if memberID == "" {
			memberID = "C1"
		}
		return jurorReplaySelection{Basis: proceeding.CouncilReplayBasisReconstructed, MemberID: memberID}, nil
	case proceeding.CouncilReplayBasisSnapshot:
		if snapshot != "" {
			if memberID != "" {
				return jurorReplaySelection{}, fmt.Errorf("--member-id cannot be used with an explicit --snapshot")
			}
			return jurorReplaySelection{Basis: proceeding.CouncilReplayBasisSnapshot, SnapshotDir: snapshot}, nil
		}
		if memberID == "" {
			return jurorReplaySelection{}, fmt.Errorf("--basis %s requires --snapshot or --member-id", proceeding.CouncilReplayBasisSnapshot)
		}
		found, exists, err := discoverJurorReplaySnapshot(sourceOutput, memberID)
		if err != nil {
			return jurorReplaySelection{}, err
		}
		if !exists {
			return jurorReplaySelection{}, fmt.Errorf("source output has no council-turn snapshots")
		}
		if found == "" {
			return jurorReplaySelection{}, fmt.Errorf("source output has no council-turn snapshot for member %s", memberID)
		}
		return jurorReplaySelection{Basis: proceeding.CouncilReplayBasisSnapshot, SnapshotDir: found}, nil
	default:
		return jurorReplaySelection{}, fmt.Errorf("--basis must be %s or %s", proceeding.CouncilReplayBasisReconstructed, proceeding.CouncilReplayBasisSnapshot)
	}
	if snapshot != "" {
		if memberID != "" {
			return jurorReplaySelection{}, fmt.Errorf("--member-id cannot be used with an explicit --snapshot")
		}
		return jurorReplaySelection{Basis: proceeding.CouncilReplayBasisSnapshot, SnapshotDir: snapshot}, nil
	}
	found, exists, err := discoverJurorReplaySnapshot(sourceOutput, memberID)
	if err != nil {
		return jurorReplaySelection{}, err
	}
	if found != "" {
		return jurorReplaySelection{Basis: proceeding.CouncilReplayBasisSnapshot, SnapshotDir: found}, nil
	}
	if exists {
		if memberID == "" {
			return jurorReplaySelection{}, fmt.Errorf("source output has multiple council-turn snapshots; pass --snapshot or --member-id")
		}
		return jurorReplaySelection{}, fmt.Errorf("source output has no council-turn snapshot for member %s", memberID)
	}
	if memberID == "" {
		memberID = "C1"
	}
	return jurorReplaySelection{Basis: proceeding.CouncilReplayBasisReconstructed, MemberID: memberID}, nil
}

func discoverJurorReplaySnapshot(sourceOutput string, memberID string) (string, bool, error) {
	dir, err := proceeding.ResolveAAROutputDir(sourceOutput)
	if err != nil {
		return "", false, err
	}
	root := filepath.Join(dir, "council-turns")
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("stat council-turns directory %s: %w", root, err)
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("council-turns path is not a directory: %s", root)
	}
	paths, err := filepath.Glob(filepath.Join(root, "turn-*", "input.json"))
	if err != nil {
		return "", true, fmt.Errorf("scan council-turn snapshots: %w", err)
	}
	if len(paths) == 0 {
		return "", true, fmt.Errorf("council-turns directory contains no snapshots: %s", root)
	}
	sort.Strings(paths)
	type snapshotHeader struct {
		MemberID string `json:"member_id"`
	}
	var matches []string
	memberID = strings.TrimSpace(memberID)
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", true, fmt.Errorf("read snapshot %s: %w", path, err)
		}
		var header snapshotHeader
		if err := json.Unmarshal(raw, &header); err != nil {
			return "", true, fmt.Errorf("parse snapshot %s: %w", path, err)
		}
		if memberID == "" || strings.TrimSpace(header.MemberID) == memberID {
			matches = append(matches, filepath.Dir(path))
		}
	}
	if len(matches) == 1 {
		return matches[0], true, nil
	}
	if len(matches) == 0 {
		return "", true, nil
	}
	if memberID == "" {
		return "", true, fmt.Errorf("source output has %d council-turn snapshots; pass --snapshot or --member-id", len(matches))
	}
	return "", true, fmt.Errorf("member %s has %d council-turn snapshots; pass --snapshot", memberID, len(matches))
}

func buildJurorReplaySummary(result localrun.CouncilReplayResult, modelConfigPath string, personaPath string, errorText string) jurorReplaySummary {
	status := strings.TrimSpace(result.Status)
	if status == "" && errorText != "" {
		status = "error"
	}
	personaPath = replayPersonaPath(result, personaPath)
	return jurorReplaySummary{
		Status:          status,
		Basis:           strings.TrimSpace(result.Basis),
		CaseID:          strings.TrimSpace(result.CaseID),
		MemberID:        strings.TrimSpace(result.MemberID),
		Model:           strings.TrimSpace(result.Model),
		Vote:            strings.TrimSpace(result.Vote),
		Rationale:       strings.TrimSpace(result.Rationale),
		OutputDir:       strings.TrimSpace(result.OutputDir),
		SourceOutputDir: strings.TrimSpace(result.SourceOutputDir),
		SnapshotDir:     strings.TrimSpace(result.SnapshotDir),
		ModelConfigPath: strings.TrimSpace(modelConfigPath),
		PersonaPath:     strings.TrimSpace(personaPath),
		ToolCallCount:   len(result.ToolCalls),
		Error:           strings.TrimSpace(errorText),
		ErrorDetails:    cloneSummaryMap(result.ErrorDetails),
	}
}

func reportJurorReplayError(stdout io.Writer, result localrun.CouncilReplayResult, modelConfigPath string, personaPath string, err error) error {
	summary := buildJurorReplaySummary(result, modelConfigPath, personaPath, strings.TrimSpace(err.Error()))
	if writeErr := writeJurorReplaySummary(stdout, summary); writeErr != nil {
		return writeErr
	}
	return &reportedError{err: err}
}

func writeJurorReplaySummary(w io.Writer, summary jurorReplaySummary) error {
	raw, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal juror replay summary: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(raw)); err != nil {
		return fmt.Errorf("write juror replay summary: %w", err)
	}
	return nil
}

func writeJurorReplayMetadata(outDir string, result localrun.CouncilReplayResult, modelConfigPath string, personaPath string, replayErr error) error {
	outDir = strings.TrimSpace(outDir)
	if outDir == "" {
		return fmt.Errorf("--out-dir is required")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create juror replay output dir: %w", err)
	}
	errorText := ""
	if replayErr != nil {
		errorText = strings.TrimSpace(replayErr.Error())
	}
	summary := buildJurorReplaySummary(result, modelConfigPath, personaPath, errorText)
	hashPath := summary.PersonaPath
	if strings.TrimSpace(result.Input.Seat.PersonaFile) == "" && replayErr != nil {
		hashPath = ""
	}
	hash := ""
	if hashPath != "" {
		var err error
		hash, err = fileSHA256Hex(hashPath)
		if err != nil {
			return err
		}
	}
	metadata := jurorReplayMetadata{
		SchemaVersion:   jurorReplayMetadataSchemaVersion,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Status:          summary.Status,
		Basis:           summary.Basis,
		CaseID:          summary.CaseID,
		MemberID:        summary.MemberID,
		Model:           summary.Model,
		Vote:            summary.Vote,
		Rationale:       summary.Rationale,
		OutputDir:       summary.OutputDir,
		SourceOutputDir: summary.SourceOutputDir,
		SnapshotDir:     summary.SnapshotDir,
		ModelConfigPath: summary.ModelConfigPath,
		PersonaPath:     summary.PersonaPath,
		PersonaSHA256:   hash,
		ToolCallCount:   summary.ToolCallCount,
		Error:           summary.Error,
		ErrorDetails:    cloneSummaryMap(summary.ErrorDetails),
	}
	raw, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal juror replay metadata: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(outDir, "juror-replay.json"), raw, 0o644); err != nil {
		return fmt.Errorf("write juror replay metadata: %w", err)
	}
	return nil
}

func cloneSummaryMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if strings.TrimSpace(key) != "" && value != nil {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func replayPersonaPath(result localrun.CouncilReplayResult, fallback string) string {
	if strings.TrimSpace(result.Input.Seat.PersonaFile) != "" {
		return result.Input.Seat.PersonaFile
	}
	return strings.TrimSpace(fallback)
}

func fileSHA256Hex(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read persona for hash %s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
