package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"adjudication/adc/runtime/casegen"
	"adjudication/adc/runtime/lean"
	"adjudication/adc/runtime/runner"
	"adjudication/adc/runtime/spec"
	"adjudication/common/modelrequest"
	"adjudication/common/openai"
)

const JudgeForCauseTool = "decide_juror_for_cause_challenge"

type JudgeForCauseFixture struct {
	ID                 string   `json:"id"`
	Tier               int      `json:"tier"`
	IssueFamily        string   `json:"issue_family"`
	CaseTheme          string   `json:"case_theme"`
	ChallengedBy       string   `json:"challenged_by"`
	JurorID            string   `json:"juror_id"`
	VoirDireRecord     string   `json:"voir_dire_record"`
	ChallengeGrounds   string   `json:"challenge_grounds"`
	ExpectedGranted    bool     `json:"expected_granted"`
	ExpectedReasonTags []string `json:"expected_reason_tags"`
	Severity           float64  `json:"severity"`
	ContextNotes       string   `json:"context_notes,omitempty"`
}

type JudgeForCauseOptions struct {
	FixturesPath          string
	OutputDir             string
	OpportunityPromptPath string
	OpportunityPromptName string
	Engine                lean.Engine
	Model                 string
	Online                bool
	DryRun                bool
	Limit                 int
	Timeout               time.Duration
	Temperature           *float64
}

type JudgeForCauseRescoreOptions struct {
	ResultsPath string
	OutputDir   string
}

type JudgeForCauseSummary struct {
	Evaluation       string                        `json:"evaluation"`
	Model            string                        `json:"model"`
	DryRun           bool                          `json:"dry_run"`
	PromptSource     string                        `json:"prompt_source"`
	PromptName       string                        `json:"prompt_name"`
	PromptPath       string                        `json:"prompt_path,omitempty"`
	PromptCopyPath   string                        `json:"prompt_copy_path,omitempty"`
	FixturesPath     string                        `json:"fixtures_path"`
	OutputDir        string                        `json:"output_dir"`
	ResultsPath      string                        `json:"results_path"`
	SummaryPath      string                        `json:"summary_path"`
	Total            int                           `json:"total"`
	Correct          int                           `json:"correct"`
	ReasonCorrect    int                           `json:"reason_correct"`
	Invalid          int                           `json:"invalid"`
	FalseGrants      int                           `json:"false_grants"`
	FalseDenials     int                           `json:"false_denials"`
	Accuracy         float64                       `json:"accuracy"`
	WeightedAccuracy float64                       `json:"weighted_accuracy"`
	FalseGrantRate   float64                       `json:"false_grant_rate"`
	FalseDenialRate  float64                       `json:"false_denial_rate"`
	InvalidRate      float64                       `json:"invalid_rate"`
	ByReasonTag      map[string]JudgeForCauseSlice `json:"by_reason_tag"`
	ByIssueFamily    map[string]JudgeForCauseSlice `json:"by_issue_family"`
	ByTier           map[string]JudgeForCauseSlice `json:"by_tier"`
	ByChallengedBy   map[string]JudgeForCauseSlice `json:"by_challenged_by"`
	GeneratedAt      string                        `json:"generated_at"`
}

type JudgeForCauseSlice struct {
	Total            int     `json:"total"`
	Correct          int     `json:"correct"`
	FalseGrants      int     `json:"false_grants"`
	FalseDenials     int     `json:"false_denials"`
	Invalid          int     `json:"invalid"`
	Weight           float64 `json:"weight"`
	CorrectWeight    float64 `json:"correct_weight"`
	Accuracy         float64 `json:"accuracy"`
	WeightedAccuracy float64 `json:"weighted_accuracy"`
}

type JudgeForCauseResult struct {
	ID                 string           `json:"id"`
	Tier               int              `json:"tier"`
	IssueFamily        string           `json:"issue_family"`
	CaseTheme          string           `json:"case_theme"`
	ChallengedBy       string           `json:"challenged_by"`
	JurorID            string           `json:"juror_id"`
	VoirDireRecord     string           `json:"voir_dire_record"`
	ChallengeGrounds   string           `json:"challenge_grounds"`
	ExpectedGranted    bool             `json:"expected_granted"`
	ExpectedReasonTags []string         `json:"expected_reason_tags"`
	Severity           float64          `json:"severity"`
	ContextNotes       string           `json:"context_notes,omitempty"`
	Model              string           `json:"model"`
	DryRun             bool             `json:"dry_run"`
	PromptSource       string           `json:"prompt_source"`
	PromptName         string           `json:"prompt_name"`
	PromptPath         string           `json:"prompt_path,omitempty"`
	State              map[string]any   `json:"state"`
	View               map[string]any   `json:"view"`
	Opportunity        map[string]any   `json:"opportunity"`
	Input              []map[string]any `json:"input"`
	RawResponse        map[string]any   `json:"raw_response"`
	ToolPayload        map[string]any   `json:"tool_payload,omitempty"`
	ChallengeID        string           `json:"challenge_id,omitempty"`
	Granted            *bool            `json:"granted,omitempty"`
	RulingReason       string           `json:"ruling_reason,omitempty"`
	MatchedReasonTags  []string         `json:"matched_reason_tags"`
	OutcomeCorrect     bool             `json:"outcome_correct"`
	ReasonCorrect      bool             `json:"reason_correct"`
	InvalidReason      string           `json:"invalid_reason,omitempty"`
	LeanAccepted       bool             `json:"lean_accepted"`
	LeanError          string           `json:"lean_error,omitempty"`
}

type judgeForCausePromptVariant struct {
	Source   string
	Name     string
	Path     string
	CopyPath string
	Text     string
}

func RunJudgeForCause(ctx context.Context, opts JudgeForCauseOptions) (JudgeForCauseSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeForCauseSummary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeForCauseSummary{}, fmt.Errorf("output directory is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	fixtures, err := LoadJudgeForCauseFixtures(opts.FixturesPath)
	if err != nil {
		return JudgeForCauseSummary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeForCauseSummary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef := modelrequest.ModelRef{}
	var client *openai.Client
	if !opts.DryRun {
		modelRef, err = modelrequest.ParseModelRef(opts.Model)
		if err != nil {
			return JudgeForCauseSummary{}, fmt.Errorf("parse --model: %w", err)
		}
		client, err = openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
		if err != nil {
			return JudgeForCauseSummary{}, err
		}
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeForCauseSummary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeForCausePromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName, opts.OutputDir)
	if err != nil {
		return JudgeForCauseSummary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeForCauseSummary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer resultsFile.Close()

	summary := newJudgeForCauseSummary(opts, promptVariant, resultsPath, summaryPath)
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeForCauseFixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeForCauseSummary{}, err
		}
		if err := encoder.Encode(result); err != nil {
			return JudgeForCauseSummary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeForCauseSummaryResult(&summary, result, weight)
	}
	finalizeJudgeForCauseSummary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeForCauseSummary{}, err
	}
	return summary, nil
}

func RescoreJudgeForCause(opts JudgeForCauseRescoreOptions) (JudgeForCauseSummary, error) {
	if strings.TrimSpace(opts.ResultsPath) == "" {
		return JudgeForCauseSummary{}, fmt.Errorf("results path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeForCauseSummary{}, fmt.Errorf("output directory is required")
	}
	results, err := readJudgeForCauseResults(opts.ResultsPath)
	if err != nil {
		return JudgeForCauseSummary{}, err
	}
	if len(results) == 0 {
		return JudgeForCauseSummary{}, fmt.Errorf("no results loaded from %s", opts.ResultsPath)
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeForCauseSummary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeForCauseSummary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer resultsFile.Close()

	summary := JudgeForCauseSummary{
		Evaluation:     "judge_for_cause",
		Model:          results[0].Model,
		DryRun:         results[0].DryRun,
		PromptSource:   resultJudgeForCausePromptSource(results[0]),
		PromptName:     resultJudgeForCausePromptName(results[0]),
		PromptPath:     results[0].PromptPath,
		FixturesPath:   "rescored from " + opts.ResultsPath,
		OutputDir:      opts.OutputDir,
		ResultsPath:    resultsPath,
		SummaryPath:    summaryPath,
		ByReasonTag:    map[string]JudgeForCauseSlice{},
		ByIssueFamily:  map[string]JudgeForCauseSlice{},
		ByTier:         map[string]JudgeForCauseSlice{},
		ByChallengedBy: map[string]JudgeForCauseSlice{},
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, result := range results {
		rescoreJudgeForCauseResult(&result)
		if err := encoder.Encode(result); err != nil {
			return JudgeForCauseSummary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeForCauseSummaryResult(&summary, result, weight)
	}
	finalizeJudgeForCauseSummary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeForCauseSummary{}, err
	}
	return summary, nil
}

func newJudgeForCauseSummary(opts JudgeForCauseOptions, promptVariant judgeForCausePromptVariant, resultsPath string, summaryPath string) JudgeForCauseSummary {
	return JudgeForCauseSummary{
		Evaluation:     "judge_for_cause",
		Model:          opts.Model,
		DryRun:         opts.DryRun,
		PromptSource:   promptVariant.Source,
		PromptName:     promptVariant.Name,
		PromptPath:     promptVariant.Path,
		PromptCopyPath: promptVariant.CopyPath,
		FixturesPath:   opts.FixturesPath,
		OutputDir:      opts.OutputDir,
		ResultsPath:    resultsPath,
		SummaryPath:    summaryPath,
		ByReasonTag:    map[string]JudgeForCauseSlice{},
		ByIssueFamily:  map[string]JudgeForCauseSlice{},
		ByTier:         map[string]JudgeForCauseSlice{},
		ByChallengedBy: map[string]JudgeForCauseSlice{},
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}
}

func runJudgeForCauseFixture(
	ctx context.Context,
	opts JudgeForCauseOptions,
	promptVariant judgeForCausePromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeForCauseFixture,
) (JudgeForCauseResult, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeForCauseResult{}, err
	}
	state := BuildJudgeForCauseState(fixture)
	roles := judgeForCauseRoles()
	viewResp, err := opts.Engine.View(state, "judge")
	if err != nil {
		return JudgeForCauseResult{}, fmt.Errorf("fixture %s view: %w", fixture.ID, err)
	}
	if ok, _ := viewResp["ok"].(bool); !ok {
		return JudgeForCauseResult{}, fmt.Errorf("fixture %s view rejected: %s", fixture.ID, stringField(viewResp, "error"))
	}
	view, _ := viewResp["view"].(map[string]any)
	opportunityResp, err := opts.Engine.NextOpportunity(state, roles, 3)
	if err != nil {
		return JudgeForCauseResult{}, fmt.Errorf("fixture %s next opportunity: %w", fixture.ID, err)
	}
	if ok, _ := opportunityResp["ok"].(bool); !ok {
		return JudgeForCauseResult{}, fmt.Errorf("fixture %s next opportunity rejected: %s", fixture.ID, stringField(opportunityResp, "error"))
	}
	opportunity, _ := opportunityResp["opportunity"].(map[string]any)
	if len(opportunity) == 0 {
		return JudgeForCauseResult{}, fmt.Errorf("fixture %s returned no opportunity", fixture.ID)
	}
	if stringField(opportunity, "role") != "judge" {
		return JudgeForCauseResult{}, fmt.Errorf("fixture %s opportunity role = %q, want judge", fixture.ID, stringField(opportunity, "role"))
	}
	if !stringSliceContains(stringSliceField(opportunity, "allowed_tools"), JudgeForCauseTool) {
		return JudgeForCauseResult{}, fmt.Errorf("fixture %s opportunity lacks %s", fixture.ID, JudgeForCauseTool)
	}
	input, err := buildJudgeForCauseInput(view, opportunity, fixture, promptVariant)
	if err != nil {
		return JudgeForCauseResult{}, fmt.Errorf("fixture %s build prompt: %w", fixture.ID, err)
	}
	tools, err := runner.BuildTools([]string{JudgeForCauseTool})
	if err != nil {
		return JudgeForCauseResult{}, err
	}
	var resp openai.Response
	if opts.DryRun {
		resp = dryRunJudgeForCauseResponse(fixture)
	} else {
		callCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		resp, err = client.CreateResponse(callCtx, modelRef.Model, input, tools, "", opts.Temperature)
		cancel()
		if err != nil {
			return JudgeForCauseResult{}, fmt.Errorf("fixture %s model call: %w", fixture.ID, err)
		}
	}
	result := scoreJudgeForCauseResponse(fixture, opts.Model, opts.DryRun, state, view, opportunity, input, resp)
	result.PromptSource = promptVariant.Source
	result.PromptName = promptVariant.Name
	result.PromptPath = promptVariant.Path
	if result.InvalidReason == "" {
		decision := map[string]any{
			"kind":      "tool",
			"tool_name": JudgeForCauseTool,
			"payload":   result.ToolPayload,
		}
		applyResp, err := opts.Engine.ApplyDecision(state, intField(state, "state_version"), stringField(opportunity, "opportunity_id"), "judge", decision, roles, 3)
		if err != nil {
			result.LeanError = err.Error()
		} else if ok, _ := applyResp["ok"].(bool); ok {
			result.LeanAccepted = true
		} else {
			result.LeanError = stringField(applyResp, "error")
		}
	}
	return result, nil
}

func LoadJudgeForCauseFixtures(path string) ([]JudgeForCauseFixture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeForCauseFixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeForCauseFixture
		if err := json.Unmarshal([]byte(line), &fixture); err != nil {
			return nil, fmt.Errorf("parse fixtures %s line %d: %w", path, lineNo, err)
		}
		if err := fixture.Validate(); err != nil {
			return nil, fmt.Errorf("fixtures %s line %d: %w", path, lineNo, err)
		}
		out = append(out, fixture)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan fixtures %s: %w", path, err)
	}
	return out, nil
}

func readJudgeForCauseResults(path string) ([]JudgeForCauseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open results %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 32*1024*1024)
	out := make([]JudgeForCauseResult, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var result JudgeForCauseResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, fmt.Errorf("parse results %s line %d: %w", path, lineNo, err)
		}
		out = append(out, result)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan results %s: %w", path, err)
	}
	return out, nil
}

func loadJudgeForCausePromptVariant(path string, name string, outputDir string) (judgeForCausePromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeForCausePromptVariant{Source: "production", Name: name}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeForCausePromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeForCausePromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	copyPath := filepath.Join(outputDir, "opportunity_prompt.md")
	if err := os.WriteFile(copyPath, raw, 0o644); err != nil {
		return judgeForCausePromptVariant{}, fmt.Errorf("copy opportunity prompt to %s: %w", copyPath, err)
	}
	return judgeForCausePromptVariant{Source: "file:" + path, Name: name, Path: path, CopyPath: copyPath, Text: text}, nil
}

func (f JudgeForCauseFixture) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("fixture missing id")
	}
	if f.Tier < 1 {
		return fmt.Errorf("fixture %s tier must be positive", f.ID)
	}
	if strings.TrimSpace(f.IssueFamily) == "" {
		return fmt.Errorf("fixture %s missing issue_family", f.ID)
	}
	if normalizeParty(f.ChallengedBy) == "" {
		return fmt.Errorf("fixture %s invalid challenged_by %q", f.ID, f.ChallengedBy)
	}
	if strings.TrimSpace(f.JurorID) == "" {
		return fmt.Errorf("fixture %s missing juror_id", f.ID)
	}
	if strings.TrimSpace(f.VoirDireRecord) == "" {
		return fmt.Errorf("fixture %s missing voir_dire_record", f.ID)
	}
	if strings.TrimSpace(f.ChallengeGrounds) == "" {
		return fmt.Errorf("fixture %s missing challenge_grounds", f.ID)
	}
	if len(f.ExpectedReasonTags) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeForCauseState(f JudgeForCauseFixture) map[string]any {
	jurorID := strings.TrimSpace(f.JurorID)
	byParty := normalizeParty(f.ChallengedBy)
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           "Judge Eval Court",
		"court_profile":        nil,
		"policy":               defaultJudgeVoirDirePolicy(),
		"state_version":        0,
		"passed_opportunities": []any{},
		"case": map[string]any{
			"case_id":                       "judge-for-cause-" + strings.TrimSpace(f.ID),
			"caption":                       strings.TrimSpace(f.CaseTheme),
			"judge":                         "Judge Eval",
			"filed_on":                      "2026-07-14",
			"auto_rule11":                   false,
			"status":                        "trial",
			"trial_mode":                    "jury",
			"phase":                         "voir_dire",
			"last_pleading_served_on":       "2026-07-01",
			"jury_demanded_on":              "2026-07-01",
			"jury_configuration":            map[string]any{"juror_count": 6, "unanimous_required": true, "minimum_concurring": 6},
			"single_claim":                  defaultJudgeVoirDireClaim(),
			"jurisdictional_allegations":    nil,
			"jurors":                        []any{map[string]any{"juror_id": jurorID, "name": "Juror " + jurorID, "status": "candidate", "note": "", "model": "eval-model", "persona_filename": "eval-persona"}},
			"juror_questionnaire":           []any{map[string]any{"question_id": "q1", "question": "Can you follow the court's instructions and decide the case from the record?"}},
			"juror_questionnaire_responses": []any{map[string]any{"juror_id": jurorID, "submitted_at": "2026-07-14", "answers": []any{map[string]any{"question_id": "q1", "answer": strings.TrimSpace(f.VoirDireRecord)}}}},
			"voir_dire_exchanges": []any{map[string]any{
				"exchange_id":   "vx-1",
				"juror_id":      jurorID,
				"asked_by":      byParty,
				"question":      "Counsel explored whether this candidate can follow the law and decide from the record.",
				"judge_allowed": true,
				"ruling_reason": "Permitted bias and qualification inquiry.",
				"response":      strings.TrimSpace(f.VoirDireRecord),
				"asked_at":      "2026-07-14",
				"ruled_at":      "2026-07-14",
				"answered_at":   "2026-07-14",
			}},
			"for_cause_challenges": []any{map[string]any{
				"challenge_id":  "fc-1",
				"juror_id":      jurorID,
				"by_party":      byParty,
				"grounds":       strings.TrimSpace(f.ChallengeGrounds),
				"requested_at":  "2026-07-14",
				"granted":       nil,
				"decided_at":    nil,
				"ruling_reason": "",
			}},
			"deliberation_round":       1,
			"juror_votes":              []any{},
			"jury_verdict":             nil,
			"hung_jury":                nil,
			"contempt_counts":          []any{},
			"protective_orders":        []any{},
			"bench_findings":           []any{},
			"bench_conclusions":        []any{},
			"juror_explanations":       []any{},
			"local_rule_overrides":     []any{},
			"limit_usage":              []any{},
			"rule56_window_closed_for": []any{},
			"case_files":               []any{},
			"file_events":              []any{},
			"rule68_offers":            []any{},
			"technical_reports":        []any{},
			"monetary_judgment":        0.0,
			"docket": []any{
				map[string]any{"title": "Voir dire answer", "description": jurorID + ": " + strings.TrimSpace(f.VoirDireRecord)},
				map[string]any{"title": "For-cause challenge requested", "description": byParty + " requested excusal of " + jurorID + ": " + strings.TrimSpace(f.ChallengeGrounds)},
			},
			"decision_traces": []any{
				map[string]any{"action": "answer_voir_dire_question", "outcome": jurorID, "citations": []any{"FRCP 47(a)"}},
				map[string]any{"action": "challenge_juror_for_cause", "outcome": byParty + ":requested:" + jurorID, "citations": []any{"FRCP 47(a)"}},
			},
		},
	}
}

func buildJudgeForCauseInput(
	view map[string]any,
	opportunity map[string]any,
	fixture JudgeForCauseFixture,
	promptVariant judgeForCausePromptVariant,
) ([]map[string]any, error) {
	role := judgeForCauseRole()
	systemPrompt, err := buildJudgeForCauseSystemPrompt(role, view)
	if err != nil {
		return nil, err
	}
	userPrompt, err := buildJudgeForCauseOpportunityPrompt(opportunity, fixture, promptVariant)
	if err != nil {
		return nil, err
	}
	return []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}, nil
}

func scoreJudgeForCauseResponse(
	fixture JudgeForCauseFixture,
	model string,
	dryRun bool,
	state map[string]any,
	view map[string]any,
	opportunity map[string]any,
	input []map[string]any,
	resp openai.Response,
) JudgeForCauseResult {
	result := JudgeForCauseResult{
		ID:                 fixture.ID,
		Tier:               fixture.Tier,
		IssueFamily:        fixture.IssueFamily,
		CaseTheme:          strings.TrimSpace(fixture.CaseTheme),
		ChallengedBy:       normalizeParty(fixture.ChallengedBy),
		JurorID:            strings.TrimSpace(fixture.JurorID),
		VoirDireRecord:     strings.TrimSpace(fixture.VoirDireRecord),
		ChallengeGrounds:   strings.TrimSpace(fixture.ChallengeGrounds),
		ExpectedGranted:    fixture.ExpectedGranted,
		ExpectedReasonTags: append([]string{}, fixture.ExpectedReasonTags...),
		Severity:           normalizedSeverity(fixture.Severity),
		ContextNotes:       strings.TrimSpace(fixture.ContextNotes),
		Model:              model,
		DryRun:             dryRun,
		State:              state,
		View:               view,
		Opportunity:        opportunity,
		Input:              input,
		RawResponse:        responseJSON(resp),
	}
	payload, invalid := extractJudgeForCausePayload(resp)
	if invalid != "" {
		result.InvalidReason = invalid
		return result
	}
	result.ToolPayload = payload
	if got := strings.TrimSpace(stringField(payload, "challenge_id")); got != "fc-1" {
		result.InvalidReason = "wrong_challenge_id"
		return result
	}
	if got := strings.TrimSpace(stringField(payload, "juror_id")); got != result.JurorID {
		result.InvalidReason = "wrong_juror_id"
		return result
	}
	if got := normalizeParty(stringField(payload, "by_party")); got != result.ChallengedBy {
		result.InvalidReason = "wrong_by_party"
		return result
	}
	granted, ok := payload["granted"].(bool)
	if !ok {
		result.InvalidReason = "malformed_granted"
		return result
	}
	result.Granted = &granted
	result.RulingReason = strings.TrimSpace(stringField(payload, "ruling_reason"))
	if result.RulingReason == "" {
		result.InvalidReason = "empty_ruling_reason"
		return result
	}
	rescoreJudgeForCauseResult(&result)
	return result
}

func rescoreJudgeForCauseResult(result *JudgeForCauseResult) {
	if result == nil || result.InvalidReason != "" || result.Granted == nil {
		return
	}
	result.OutcomeCorrect = *result.Granted == result.ExpectedGranted
	result.MatchedReasonTags = matchedJudgeForCauseReasonTags(result.RulingReason, result.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
}

func extractJudgeForCausePayload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeForCauseTool {
		return nil, "wrong_tool"
	}
	if strings.TrimSpace(call.ArgumentsError) != "" {
		return nil, "malformed_arguments"
	}
	if call.Arguments == nil {
		return nil, "missing_arguments"
	}
	return call.Arguments, ""
}

func dryRunJudgeForCauseResponse(f JudgeForCauseFixture) openai.Response {
	return openai.Response{
		ResponseID: "dry-run-" + strings.TrimSpace(f.ID),
		ToolCalls: []openai.ToolCall{{
			CallID: "dry-run-call-" + strings.TrimSpace(f.ID),
			Name:   JudgeForCauseTool,
			Arguments: map[string]any{
				"challenge_id":  "fc-1",
				"juror_id":      strings.TrimSpace(f.JurorID),
				"by_party":      normalizeParty(f.ChallengedBy),
				"granted":       f.ExpectedGranted,
				"ruling_reason": "gold tags: " + strings.Join(f.ExpectedReasonTags, ", "),
			},
		}},
	}
}

func buildJudgeForCauseSystemPrompt(role spec.RoleSpec, view map[string]any) (string, error) {
	payload, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal judge view: %w", err)
	}
	preamble := ""
	if strings.TrimSpace(role.PromptPreamble) != "" {
		preamble = "\nRole prompt preamble: " + role.PromptPreamble
	}
	return "Role: " + role.Name +
		preamble +
		"\nInstructions: " + role.Instructions +
		"\nAllowed actions: " + strings.Join(role.EffectiveAllowedActions(), ", ") +
		"\nUse only listed tools with precise payloads." +
		"\nWhen you decide to act, call exactly one tool rather than replying with prose." +
		"\nCurrent view:\n" + string(payload), nil
}

func buildJudgeForCauseOpportunityPrompt(
	opportunity map[string]any,
	fixture JudgeForCauseFixture,
	promptVariant judgeForCausePromptVariant,
) (string, error) {
	tools, err := runner.BuildTools([]string{JudgeForCauseTool})
	if err != nil {
		return "", err
	}
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		objective = renderJudgeForCausePromptTemplate(promptVariant.Text, fixture, opportunity)
	}
	lines := []string{
		"Current opportunity:",
		stringField(opportunity, "actor_message"),
		formatJudgeForCauseObjective(objective),
		"Phase: " + stringField(opportunity, "phase"),
		"Allowed actions: " + strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
	}
	if constraints, ok := opportunity["constraints"].(map[string]any); ok && len(constraints) > 0 {
		raw, err := json.Marshal(constraints)
		if err != nil {
			return "", fmt.Errorf("marshal opportunity constraints: %w", err)
		}
		lines = append(lines, "Opportunity constraints: "+string(raw))
	}
	if mayPass, _ := opportunity["may_pass"].(bool); mayPass {
		lines = append(lines, "You may decline this opportunity by calling pass_turn.")
	} else {
		lines = append(lines, "You must choose one allowed action now.")
	}
	lines = append(lines, "", "Tool payloads:")
	for _, tool := range tools {
		raw, err := json.Marshal(tool["parameters"])
		if err != nil {
			return "", fmt.Errorf("marshal tool payload schema: %w", err)
		}
		lines = append(lines, fmt.Sprintf("Tool `%s` payload: %s", stringField(tool, "name"), string(raw)))
	}
	return strings.Join(lines, "\n"), nil
}

func formatJudgeForCauseObjective(objective string) string {
	objective = strings.TrimSpace(objective)
	if strings.Contains(objective, "\n") {
		return "Objective:\n" + objective
	}
	return "Objective: " + objective
}

func renderJudgeForCausePromptTemplate(template string, fixture JudgeForCauseFixture, opportunity map[string]any) string {
	replacer := strings.NewReplacer(
		"{{production_objective}}", stringField(opportunity, "objective"),
		"{{actor_message}}", stringField(opportunity, "actor_message"),
		"{{phase}}", stringField(opportunity, "phase"),
		"{{allowed_tools}}", strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
		"{{fixture_id}}", strings.TrimSpace(fixture.ID),
		"{{tier}}", strconv.Itoa(fixture.Tier),
		"{{issue_family}}", strings.TrimSpace(fixture.IssueFamily),
		"{{case_theme}}", strings.TrimSpace(fixture.CaseTheme),
		"{{challenged_by}}", normalizeParty(fixture.ChallengedBy),
		"{{juror_id}}", strings.TrimSpace(fixture.JurorID),
		"{{voir_dire_record}}", strings.TrimSpace(fixture.VoirDireRecord),
		"{{challenge_grounds}}", strings.TrimSpace(fixture.ChallengeGrounds),
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
	return strings.TrimSpace(replacer.Replace(template))
}

func judgeForCauseRole() spec.RoleSpec {
	return spec.RoleSpec{
		Name:           "judge",
		Instructions:   "Judge for procedural rulings, trial control, and judgment entry.",
		PromptPreamble: casegen.JudgeRuntimeBrief(),
		AllowedTools:   []string{JudgeForCauseTool},
	}
}

func judgeForCauseRoles() []map[string]any {
	return []map[string]any{{"role": "judge", "allowed_tools": []string{JudgeForCauseTool}}}
}

func applyJudgeForCauseSummaryResult(summary *JudgeForCauseSummary, result JudgeForCauseResult, weight float64) {
	summary.Total++
	if result.InvalidReason != "" {
		summary.Invalid++
	} else if result.OutcomeCorrect {
		summary.Correct++
	} else {
		classifyJudgeForCauseError(result, &summary.FalseGrants, &summary.FalseDenials)
	}
	if result.ReasonCorrect {
		summary.ReasonCorrect++
	}
	for _, tag := range result.ExpectedReasonTags {
		updateJudgeForCauseSlice(summary.ByReasonTag, tag, result, weight)
	}
	updateJudgeForCauseSlice(summary.ByIssueFamily, result.IssueFamily, result, weight)
	updateJudgeForCauseSlice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
	updateJudgeForCauseSlice(summary.ByChallengedBy, result.ChallengedBy, result, weight)
}

func updateJudgeForCauseSlice(m map[string]JudgeForCauseSlice, key string, result JudgeForCauseResult, weight float64) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unspecified"
	}
	s := m[key]
	s.Total++
	s.Weight += weight
	if result.InvalidReason != "" {
		s.Invalid++
	} else if result.OutcomeCorrect {
		s.Correct++
		s.CorrectWeight += weight
	} else {
		classifyJudgeForCauseError(result, &s.FalseGrants, &s.FalseDenials)
	}
	m[key] = s
}

func classifyJudgeForCauseError(result JudgeForCauseResult, falseGrants *int, falseDenials *int) {
	if result.Granted == nil {
		return
	}
	if *result.Granted && !result.ExpectedGranted {
		(*falseGrants)++
	}
	if !*result.Granted && result.ExpectedGranted {
		(*falseDenials)++
	}
}

func finalizeJudgeForCauseSummary(summary *JudgeForCauseSummary, totalWeight float64, correctWeight float64) {
	if summary.Total > 0 {
		summary.Accuracy = float64(summary.Correct) / float64(summary.Total)
		summary.FalseGrantRate = float64(summary.FalseGrants) / float64(summary.Total)
		summary.FalseDenialRate = float64(summary.FalseDenials) / float64(summary.Total)
		summary.InvalidRate = float64(summary.Invalid) / float64(summary.Total)
	}
	if totalWeight > 0 {
		summary.WeightedAccuracy = correctWeight / totalWeight
	}
	finalizeJudgeForCauseSlices(summary.ByReasonTag)
	finalizeJudgeForCauseSlices(summary.ByIssueFamily)
	finalizeJudgeForCauseSlices(summary.ByTier)
	finalizeJudgeForCauseSlices(summary.ByChallengedBy)
}

func finalizeJudgeForCauseSlices(m map[string]JudgeForCauseSlice) {
	for key, s := range m {
		if s.Total > 0 {
			s.Accuracy = float64(s.Correct) / float64(s.Total)
		}
		if s.Weight > 0 {
			s.WeightedAccuracy = s.CorrectWeight / s.Weight
		}
		m[key] = s
	}
}

func matchedJudgeForCauseReasonTags(reason string, expected []string) []string {
	reason = normalizeReasonText(reason)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if judgeForCauseReasonMatchesTag(reason, tag) {
			matches = append(matches, tag)
		}
	}
	sort.Strings(matches)
	return matches
}

func judgeForCauseReasonMatchesTag(reason string, tag string) bool {
	normalizedTag := normalizeReasonText(tag)
	if strings.Contains(reason, normalizedTag) {
		return true
	}
	for _, keyword := range judgeForCauseReasonTagKeywords()[tag] {
		if strings.Contains(reason, keyword) {
			return true
		}
	}
	return false
}

func judgeForCauseReasonTagKeywords() map[string][]string {
	return map[string][]string{
		"fixed_bias":            {"cannot be impartial", "fixed opinion", "already decided", "bias"},
		"follow_law":            {"follow the law", "follow instructions", "follow the instructions", "court instructions", "burden of proof", "preponderance"},
		"damages_precommitment": {"damages", "minimum", "floor", "cap"},
		"digital_evidence":      {"digital", "electronic", "documentary", "records"},
		"hardship":              {"hardship", "unable to serve", "attention"},
		"relationship_interest": {"relationship", "employed", "financial interest", "stock"},
		"rehabilitated":         {"rehabilitated", "rehabilitation", "can be fair", "can serve", "credible assurance", "assurance"},
		"lawful_attitude":       {"general concern", "lawful attitude", "skepticism", "general preference", "not disqualifying", "can follow"},
		"sympathy_bias":         {"sympathy", "favor", "small business"},
		"language_attention":    {"language", "understand", "attention"},
	}
}

func resultJudgeForCausePromptSource(result JudgeForCauseResult) string {
	if strings.TrimSpace(result.PromptSource) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptSource)
}

func resultJudgeForCausePromptName(result JudgeForCauseResult) string {
	if strings.TrimSpace(result.PromptName) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptName)
}
