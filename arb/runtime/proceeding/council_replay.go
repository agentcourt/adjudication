package proceeding

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"adjudication/arb/runtime/spec"
)

const (
	CouncilReplayBasisReconstructed  = "reconstructed_first_round"
	CouncilReplayBasisSnapshot       = "snapshot"
	councilReplayInputSchemaVersion  = "aar.council-replay-input.v0"
	councilTurnSnapshotSchemaVersion = "aar.council-turn-snapshot.v0"
)

type CouncilReplayBuildOptions struct {
	Basis           string
	SourceOutputDir string
	SnapshotDir     string
	PromptDir       string
	Seat            CouncilSeat
}

type CouncilReplayInput struct {
	SchemaVersion    string            `json:"schema_version"`
	Basis            string            `json:"basis"`
	CreatedAt        string            `json:"created_at"`
	SourceOutputDir  string            `json:"source_output_dir"`
	SnapshotDir      string            `json:"snapshot_dir,omitempty"`
	CaseID           string            `json:"case_id"`
	RunID            string            `json:"run_id,omitempty"`
	MemberID         string            `json:"member_id"`
	TurnNumber       int               `json:"turn_number"`
	Opportunity      Opportunity       `json:"opportunity"`
	Seat             CouncilReplaySeat `json:"seat"`
	Policy           Policy            `json:"policy"`
	Runtime          RuntimeLimits     `json:"runtime"`
	Complaint        spec.Complaint    `json:"complaint"`
	State            map[string]any    `json:"state"`
	Prompt           string            `json:"prompt"`
	OriginalPrompt   string            `json:"original_prompt,omitempty"`
	Tools            []map[string]any  `json:"tools"`
	Limits           map[string]any    `json:"limits"`
	CaseView         map[string]any    `json:"case_view"`
	Evidence         []EvidenceMeta    `json:"evidence"`
	EvidenceManifest map[string]any    `json:"evidence_manifest"`
}

type CouncilReplaySeat struct {
	MemberID    string `json:"member_id"`
	Model       string `json:"model"`
	PersonaFile string `json:"persona_file"`
	PersonaText string `json:"persona_text,omitempty"`
	RequestSpec any    `json:"request_spec,omitempty"`
}

type CouncilTurnSnapshot struct {
	SchemaVersion    string            `json:"schema_version"`
	CreatedAt        string            `json:"created_at"`
	SourceOutputDir  string            `json:"source_output_dir,omitempty"`
	CaseID           string            `json:"case_id"`
	RunID            string            `json:"run_id,omitempty"`
	MemberID         string            `json:"member_id"`
	TurnNumber       int               `json:"turn_number"`
	Opportunity      Opportunity       `json:"opportunity"`
	Seat             CouncilReplaySeat `json:"seat"`
	Policy           Policy            `json:"policy"`
	Runtime          RuntimeLimits     `json:"runtime"`
	Complaint        spec.Complaint    `json:"complaint"`
	State            map[string]any    `json:"state"`
	Prompt           string            `json:"prompt"`
	Tools            []map[string]any  `json:"tools"`
	Limits           map[string]any    `json:"limits"`
	CaseView         map[string]any    `json:"case_view"`
	Evidence         []EvidenceMeta    `json:"evidence"`
	EvidenceManifest map[string]any    `json:"evidence_manifest"`
}

type replayOutputBundle struct {
	dir              string
	result           Result
	policy           Policy
	runtime          RuntimeLimits
	council          []CouncilSeat
	evidence         []EvidenceMeta
	evidenceManifest map[string]any
}

func BuildCouncilReplayInput(opts CouncilReplayBuildOptions) (CouncilReplayInput, error) {
	basis := strings.TrimSpace(opts.Basis)
	switch basis {
	case CouncilReplayBasisReconstructed:
		return buildReconstructedCouncilReplayInput(opts)
	case CouncilReplayBasisSnapshot:
		return buildSnapshotCouncilReplayInput(opts)
	default:
		return CouncilReplayInput{}, fmt.Errorf("council replay basis must be %s or %s", CouncilReplayBasisReconstructed, CouncilReplayBasisSnapshot)
	}
}

func ResolveAAROutputDir(pathValue string) (string, error) {
	path := strings.TrimSpace(pathValue)
	if path == "" {
		return "", fmt.Errorf("source output dir is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve source output dir %s: %w", path, err)
	}
	candidates := []string{
		abs,
		filepath.Join(abs, "aar-output"),
		filepath.Join(abs, "aar-partial"),
	}
	for _, candidate := range candidates {
		if fileExists(filepath.Join(candidate, "run.json")) || fileExists(filepath.Join(candidate, "state.json")) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("source output dir %s does not contain run.json or state.json", path)
}

func buildReconstructedCouncilReplayInput(opts CouncilReplayBuildOptions) (CouncilReplayInput, error) {
	bundle, err := loadReplayOutputBundle(opts.SourceOutputDir)
	if err != nil {
		return CouncilReplayInput{}, err
	}
	seat := opts.Seat
	if strings.TrimSpace(seat.MemberID) == "" {
		seat.MemberID = "C1"
	}
	state, err := cloneMapJSON(bundle.result.FinalState)
	if err != nil {
		return CouncilReplayInput{}, err
	}
	caseObj := mapAny(state["case"])
	if len(caseObj) == 0 {
		return CouncilReplayInput{}, fmt.Errorf("state has no case object")
	}
	caseObj["status"] = "open"
	caseObj["phase"] = "deliberation"
	caseObj["resolution"] = ""
	caseObj["deliberation_round"] = 1
	caseObj["council_votes"] = []map[string]any{}
	caseObj["council_members"] = councilSeatMaps([]CouncilSeat{seat})
	state["case"] = caseObj
	opportunity := Opportunity{
		ID:           "deliberation:1:" + seat.MemberID,
		Role:         "council",
		Phase:        "deliberation",
		Objective:    "Decide whether the proposition has been demonstrated.",
		AllowedTools: []string{"submit_council_vote"},
	}
	return buildCouncilReplayInputFromState(CouncilReplayBasisReconstructed, bundle, "", opts.PromptDir, state, opportunity, 1, seat, "")
}

func buildSnapshotCouncilReplayInput(opts CouncilReplayBuildOptions) (CouncilReplayInput, error) {
	bundle, err := loadReplayOutputBundle(opts.SourceOutputDir)
	if err != nil {
		return CouncilReplayInput{}, err
	}
	snapshot, snapshotDir, err := loadCouncilTurnSnapshot(opts.SnapshotDir)
	if err != nil {
		return CouncilReplayInput{}, err
	}
	seat := opts.Seat
	if strings.TrimSpace(seat.MemberID) == "" {
		seat.MemberID = strings.TrimSpace(snapshot.MemberID)
	}
	if strings.TrimSpace(seat.MemberID) == "" {
		return CouncilReplayInput{}, fmt.Errorf("snapshot has no member_id")
	}
	if strings.TrimSpace(snapshot.Opportunity.ID) == "" {
		return CouncilReplayInput{}, fmt.Errorf("snapshot has no opportunity id")
	}
	state, err := cloneMapJSON(snapshot.State)
	if err != nil {
		return CouncilReplayInput{}, err
	}
	if len(snapshot.Evidence) > 0 {
		bundle.evidence = snapshot.Evidence
	}
	if len(snapshot.EvidenceManifest) > 0 {
		bundle.evidenceManifest = snapshot.EvidenceManifest
	}
	if strings.TrimSpace(snapshot.CaseID) != "" {
		bundle.result.CaseID = snapshot.CaseID
	}
	if strings.TrimSpace(snapshot.RunID) != "" {
		bundle.result.RunID = snapshot.RunID
	}
	if strings.TrimSpace(snapshot.Complaint.Proposition) != "" {
		bundle.result.Complaint = snapshot.Complaint
	}
	if snapshot.Policy.CouncilSize > 0 {
		bundle.policy = snapshot.Policy
	}
	if snapshot.Runtime.CouncilLLMTimeoutSeconds > 0 {
		bundle.runtime = snapshot.Runtime
	}
	return buildCouncilReplayInputFromState(CouncilReplayBasisSnapshot, bundle, snapshotDir, opts.PromptDir, state, snapshot.Opportunity, snapshot.TurnNumber, seat, snapshot.Prompt)
}

func buildCouncilReplayInputFromState(
	basis string,
	bundle replayOutputBundle,
	snapshotDir string,
	promptDir string,
	state map[string]any,
	opportunity Opportunity,
	turnNumber int,
	seat CouncilSeat,
	originalPrompt string,
) (CouncilReplayInput, error) {
	fileByID, err := fileByIDFromEvidence(bundle.dir, bundle.evidence)
	if err != nil {
		return CouncilReplayInput{}, err
	}
	evidenceByID := make(map[string]EvidenceMeta, len(bundle.evidence))
	for _, meta := range bundle.evidence {
		evidenceByID[meta.EvidenceID] = meta
	}
	rc := &runContext{
		cfg: Config{
			CaseID:    bundle.result.CaseID,
			RunID:     bundle.result.RunID,
			OutputDir: bundle.dir,
			PromptDir: strings.TrimSpace(promptDir),
			Policy:    bundle.policy,
			Runtime:   bundle.runtime,
		},
		complaint:        bundle.result.Complaint,
		state:            state,
		evidence:         append([]EvidenceMeta(nil), bundle.evidence...),
		evidenceByID:     evidenceByID,
		evidenceStoreDir: filepath.Join(bundle.dir, "evidence-store"),
		fileByID:         fileByID,
		council:          []CouncilSeat{seat},
	}
	prompt, err := rc.buildCouncilAPIPrompt(seat, opportunity)
	if err != nil {
		return CouncilReplayInput{}, err
	}
	turn := &councilTurn{
		opportunity:       opportunity,
		seat:              seat,
		turnNumber:        turnNumber,
		deadline:          time.Now().Add(bundle.runtime.CouncilTimeout()),
		attemptsMax:       bundle.runtime.InvalidAttemptLimit,
		attemptsRemaining: bundle.runtime.InvalidAttemptLimit,
		evidenceBudget:    &evidenceReadBudget{},
	}
	return CouncilReplayInput{
		SchemaVersion:    councilReplayInputSchemaVersion,
		Basis:            basis,
		CreatedAt:        utcTimestamp(),
		SourceOutputDir:  bundle.dir,
		SnapshotDir:      snapshotDir,
		CaseID:           normalizeCaseID(bundle.result.CaseID),
		RunID:            bundle.result.RunID,
		MemberID:         seat.MemberID,
		TurnNumber:       turnNumber,
		Opportunity:      opportunity,
		Seat:             replaySeat(seat),
		Policy:           bundle.policy,
		Runtime:          bundle.runtime,
		Complaint:        bundle.result.Complaint,
		State:            state,
		Prompt:           prompt,
		OriginalPrompt:   originalPrompt,
		Tools:            councilToolSpecs(),
		Limits:           councilLimits(bundle.policy, bundle.runtime, turn),
		CaseView:         rc.councilView(seat, opportunity),
		Evidence:         rc.listVisibleEvidence(),
		EvidenceManifest: bundle.evidenceManifest,
	}, nil
}

func loadReplayOutputBundle(sourceOutputDir string) (replayOutputBundle, error) {
	dir, err := ResolveAAROutputDir(sourceOutputDir)
	if err != nil {
		return replayOutputBundle{}, err
	}
	var result Result
	if err := readJSON(filepath.Join(dir, "run.json"), &result); err != nil {
		return replayOutputBundle{}, err
	}
	if len(result.FinalState) == 0 {
		if err := readJSON(filepath.Join(dir, "state.json"), &result.FinalState); err != nil {
			return replayOutputBundle{}, err
		}
	}
	if strings.TrimSpace(result.Complaint.Proposition) == "" {
		raw, err := os.ReadFile(filepath.Join(dir, "complaint.md"))
		if err != nil {
			return replayOutputBundle{}, fmt.Errorf("read complaint copy: %w", err)
		}
		complaint, err := spec.ParseComplaintMarkdown(string(raw))
		if err != nil {
			return replayOutputBundle{}, err
		}
		result.Complaint = complaint
	}
	var policy Policy
	if err := readJSON(filepath.Join(dir, "policy.json"), &policy); err != nil {
		return replayOutputBundle{}, err
	}
	var runtime RuntimeLimits
	if err := readJSON(filepath.Join(dir, "runtime.json"), &runtime); err != nil {
		return replayOutputBundle{}, err
	}
	var council []CouncilSeat
	if fileExists(filepath.Join(dir, "council.json")) {
		if err := readJSON(filepath.Join(dir, "council.json"), &council); err != nil {
			return replayOutputBundle{}, err
		}
	}
	evidence, manifest, err := loadEvidenceManifest(filepath.Join(dir, "evidence-manifest.json"))
	if err != nil {
		return replayOutputBundle{}, err
	}
	return replayOutputBundle{
		dir:              dir,
		result:           result,
		policy:           policy,
		runtime:          runtime,
		council:          council,
		evidence:         evidence,
		evidenceManifest: manifest,
	}, nil
}

func loadEvidenceManifest(path string) ([]EvidenceMeta, map[string]any, error) {
	var manifest map[string]any
	if err := readJSON(path, &manifest); err != nil {
		return nil, nil, err
	}
	rawEvidence, ok := manifest["evidence"]
	if !ok {
		return nil, nil, fmt.Errorf("evidence manifest has no evidence list")
	}
	raw, err := json.Marshal(rawEvidence)
	if err != nil {
		return nil, nil, err
	}
	var evidence []EvidenceMeta
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return nil, nil, fmt.Errorf("decode evidence manifest list: %w", err)
	}
	return evidence, manifest, nil
}

func loadCouncilTurnSnapshot(pathValue string) (CouncilTurnSnapshot, string, error) {
	path := strings.TrimSpace(pathValue)
	if path == "" {
		return CouncilTurnSnapshot{}, "", fmt.Errorf("snapshot dir is required for snapshot replay")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return CouncilTurnSnapshot{}, "", fmt.Errorf("resolve snapshot dir %s: %w", path, err)
	}
	inputPath := abs
	info, err := os.Stat(inputPath)
	if err != nil {
		return CouncilTurnSnapshot{}, "", fmt.Errorf("stat snapshot %s: %w", abs, err)
	}
	if info.IsDir() {
		inputPath = filepath.Join(abs, "input.json")
	}
	var snapshot CouncilTurnSnapshot
	if err := readJSON(inputPath, &snapshot); err != nil {
		return CouncilTurnSnapshot{}, "", err
	}
	if snapshot.SchemaVersion != councilTurnSnapshotSchemaVersion {
		return CouncilTurnSnapshot{}, "", fmt.Errorf("unsupported council turn snapshot schema %q", snapshot.SchemaVersion)
	}
	return snapshot, filepath.Dir(inputPath), nil
}

func (rc *runContext) writeCouncilTurnSnapshot(turn *councilTurn, prompt string) error {
	if turn == nil {
		return fmt.Errorf("council turn is required")
	}
	state, err := cloneMapJSON(rc.state)
	if err != nil {
		return err
	}
	snapshot := CouncilTurnSnapshot{
		SchemaVersion:    councilTurnSnapshotSchemaVersion,
		CreatedAt:        utcTimestamp(),
		SourceOutputDir:  rc.cfg.OutputDir,
		CaseID:           normalizeCaseID(rc.cfg.CaseID),
		RunID:            rc.cfg.RunID,
		MemberID:         turn.seat.MemberID,
		TurnNumber:       turn.turnNumber,
		Opportunity:      turn.opportunity,
		Seat:             replaySeat(turn.seat),
		Policy:           rc.cfg.Policy,
		Runtime:          rc.cfg.Runtime,
		Complaint:        rc.complaint,
		State:            state,
		Prompt:           prompt,
		Tools:            councilToolSpecs(),
		Limits:           councilLimits(rc.cfg.Policy, rc.cfg.Runtime, turn),
		CaseView:         rc.councilView(turn.seat, turn.opportunity),
		Evidence:         rc.listVisibleEvidence(),
		EvidenceManifest: rc.evidenceManifest(),
	}
	dir := filepath.Join(rc.cfg.OutputDir, "council-turns", fmt.Sprintf("turn-%06d-%s", turn.turnNumber, safePathComponent(turn.seat.MemberID)))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create council snapshot dir: %w", err)
	}
	if err := writeJSONFile(filepath.Join(dir, "input.json"), snapshot); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "prompt.txt"), []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("write council prompt snapshot: %w", err)
	}
	return nil
}

func councilLimits(policy Policy, runtime RuntimeLimits, turn *councilTurn) map[string]any {
	remainingReads := policy.MaxEvidenceReadsPerOpportunity
	remainingBytes := policy.MaxEvidenceReadBytesPerOpportunity
	if turn != nil && turn.evidenceBudget != nil {
		remainingReads = remainingCapacity(policy.MaxEvidenceReadsPerOpportunity, turn.evidenceBudget.reads)
		remainingBytes = remainingCapacity(policy.MaxEvidenceReadBytesPerOpportunity, turn.evidenceBudget.bytes)
	}
	attemptsMax := runtime.InvalidAttemptLimit
	attemptsRemaining := runtime.InvalidAttemptLimit
	if turn != nil {
		attemptsMax = turn.attemptsMax
		attemptsRemaining = turn.attemptsRemaining
	}
	return map[string]any{
		"max_response_bytes":                            runtime.MaxResponseBytes,
		"attempts_max":                                  attemptsMax,
		"attempts_remaining":                            attemptsRemaining,
		"max_evidence_read_bytes":                       policy.MaxEvidenceReadBytes,
		"max_evidence_reads_per_opportunity":            policy.MaxEvidenceReadsPerOpportunity,
		"max_evidence_read_bytes_per_opportunity":       policy.MaxEvidenceReadBytesPerOpportunity,
		"remaining_evidence_reads_for_opportunity":      remainingReads,
		"remaining_evidence_read_bytes_for_opportunity": remainingBytes,
	}
}

func replaySeat(seat CouncilSeat) CouncilReplaySeat {
	return CouncilReplaySeat{
		MemberID:    seat.MemberID,
		Model:       seat.Model,
		PersonaFile: seat.PersonaFile,
		PersonaText: seat.PersonaText,
		RequestSpec: seat.RequestSpec,
	}
}

func fileByIDFromEvidence(outputDir string, evidence []EvidenceMeta) (map[string]CaseFile, error) {
	out := make(map[string]CaseFile, len(evidence))
	for _, meta := range evidence {
		if strings.TrimSpace(meta.EvidenceID) == "" {
			continue
		}
		path := filepath.Join(outputDir, "evidence-store", filepath.FromSlash(meta.StorageName))
		file := CaseFile{
			EvidenceID:   meta.EvidenceID,
			Name:         meta.OriginalName,
			Path:         path,
			MimeType:     meta.MimeType,
			TextReadable: meta.TextReadable,
			SizeBytes:    meta.SizeBytes,
		}
		if strings.TrimSpace(file.Name) == "" {
			file.Name = meta.EvidenceID
		}
		if file.TextReadable {
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read evidence text %s: %w", meta.EvidenceID, err)
			}
			file.Text = string(raw)
		}
		out[meta.EvidenceID] = file
	}
	return out, nil
}

func cloneMapJSON(in map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func readJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func safePathComponent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "member"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return "member"
	}
	return out
}
