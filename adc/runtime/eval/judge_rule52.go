package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

const JudgeRule52Tool = "file_bench_opinion"

type JudgeRule52Fixture struct {
	ID                 string   `json:"id"`
	Tier               int      `json:"tier"`
	IssueFamily        string   `json:"issue_family"`
	CaseTheme          string   `json:"case_theme"`
	ComplaintText      string   `json:"complaint_text"`
	AnswerText         string   `json:"answer_text"`
	PlaintiffTheory    string   `json:"plaintiff_theory"`
	DefendantTheory    string   `json:"defendant_theory"`
	AdmittedEvidence   []string `json:"admitted_evidence"`
	ExcludedEvidence   []string `json:"excluded_evidence,omitempty"`
	PlaintiffClosing   string   `json:"plaintiff_closing"`
	DefendantClosing   string   `json:"defendant_closing"`
	ExpectedWinner     string   `json:"expected_winner"`
	ExpectedAmountText string   `json:"expected_amount_text,omitempty"`
	RequiredConcepts   []string `json:"required_concepts"`
	ProhibitedConcepts []string `json:"prohibited_concepts,omitempty"`
	ExpectedReasonTags []string `json:"expected_reason_tags"`
	Severity           float64  `json:"severity"`
	ContextNotes       string   `json:"context_notes,omitempty"`
}

type JudgeRule52Options struct {
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

type JudgeRule52RescoreOptions struct {
	ResultsPath string
	OutputDir   string
}

type JudgeRule52Summary struct {
	Evaluation             string                      `json:"evaluation"`
	Model                  string                      `json:"model"`
	DryRun                 bool                        `json:"dry_run"`
	PromptSource           string                      `json:"prompt_source"`
	PromptName             string                      `json:"prompt_name"`
	PromptPath             string                      `json:"prompt_path,omitempty"`
	PromptCopyPath         string                      `json:"prompt_copy_path,omitempty"`
	FixturesPath           string                      `json:"fixtures_path"`
	OutputDir              string                      `json:"output_dir"`
	ResultsPath            string                      `json:"results_path"`
	SummaryPath            string                      `json:"summary_path"`
	Total                  int                         `json:"total"`
	Correct                int                         `json:"correct"`
	WinnerCorrect          int                         `json:"winner_correct"`
	AmountCorrect          int                         `json:"amount_correct"`
	RequiredCorrect        int                         `json:"required_correct"`
	ProhibitedCorrect      int                         `json:"prohibited_correct"`
	SeparationCorrect      int                         `json:"separation_correct"`
	ReasonCorrect          int                         `json:"reason_correct"`
	Invalid                int                         `json:"invalid"`
	FalsePlaintiffJudgment int                         `json:"false_plaintiff_judgment"`
	FalseDefenseJudgment   int                         `json:"false_defense_judgment"`
	Accuracy               float64                     `json:"accuracy"`
	WinnerAccuracy         float64                     `json:"winner_accuracy"`
	WeightedAccuracy       float64                     `json:"weighted_accuracy"`
	InvalidRate            float64                     `json:"invalid_rate"`
	ByReasonTag            map[string]JudgeRule52Slice `json:"by_reason_tag"`
	ByIssueFamily          map[string]JudgeRule52Slice `json:"by_issue_family"`
	ByTier                 map[string]JudgeRule52Slice `json:"by_tier"`
	ByExpectedWinner       map[string]JudgeRule52Slice `json:"by_expected_winner"`
	GeneratedAt            string                      `json:"generated_at"`
}

type JudgeRule52Slice struct {
	Total                  int     `json:"total"`
	Correct                int     `json:"correct"`
	WinnerCorrect          int     `json:"winner_correct"`
	Invalid                int     `json:"invalid"`
	FalsePlaintiffJudgment int     `json:"false_plaintiff_judgment"`
	FalseDefenseJudgment   int     `json:"false_defense_judgment"`
	Weight                 float64 `json:"weight"`
	CorrectWeight          float64 `json:"correct_weight"`
	Accuracy               float64 `json:"accuracy"`
	WinnerAccuracy         float64 `json:"winner_accuracy"`
	WeightedAccuracy       float64 `json:"weighted_accuracy"`
}

type JudgeRule52Result struct {
	ID                        string           `json:"id"`
	Tier                      int              `json:"tier"`
	IssueFamily               string           `json:"issue_family"`
	CaseTheme                 string           `json:"case_theme"`
	ComplaintText             string           `json:"complaint_text"`
	AnswerText                string           `json:"answer_text"`
	PlaintiffTheory           string           `json:"plaintiff_theory"`
	DefendantTheory           string           `json:"defendant_theory"`
	AdmittedEvidence          []string         `json:"admitted_evidence"`
	ExcludedEvidence          []string         `json:"excluded_evidence,omitempty"`
	PlaintiffClosing          string           `json:"plaintiff_closing"`
	DefendantClosing          string           `json:"defendant_closing"`
	ExpectedWinner            string           `json:"expected_winner"`
	ExpectedAmountText        string           `json:"expected_amount_text,omitempty"`
	RequiredConcepts          []string         `json:"required_concepts"`
	ProhibitedConcepts        []string         `json:"prohibited_concepts,omitempty"`
	ExpectedReasonTags        []string         `json:"expected_reason_tags"`
	Severity                  float64          `json:"severity"`
	ContextNotes              string           `json:"context_notes,omitempty"`
	Model                     string           `json:"model"`
	DryRun                    bool             `json:"dry_run"`
	PromptSource              string           `json:"prompt_source"`
	PromptName                string           `json:"prompt_name"`
	PromptPath                string           `json:"prompt_path,omitempty"`
	State                     map[string]any   `json:"state"`
	View                      map[string]any   `json:"view"`
	Opportunity               map[string]any   `json:"opportunity"`
	Input                     []map[string]any `json:"input"`
	RawResponse               map[string]any   `json:"raw_response"`
	ToolPayload               map[string]any   `json:"tool_payload,omitempty"`
	OpinionText               string           `json:"opinion_text,omitempty"`
	DetectedWinner            string           `json:"detected_winner,omitempty"`
	MissingRequiredConcepts   []string         `json:"missing_required_concepts,omitempty"`
	PresentProhibitedConcepts []string         `json:"present_prohibited_concepts,omitempty"`
	MatchedReasonTags         []string         `json:"matched_reason_tags,omitempty"`
	WinnerCorrect             bool             `json:"winner_correct"`
	AmountCorrect             bool             `json:"amount_correct"`
	RequiredCorrect           bool             `json:"required_correct"`
	ProhibitedCorrect         bool             `json:"prohibited_correct"`
	SeparationCorrect         bool             `json:"separation_correct"`
	OutcomeCorrect            bool             `json:"outcome_correct"`
	ReasonCorrect             bool             `json:"reason_correct"`
	InvalidReason             string           `json:"invalid_reason,omitempty"`
	LeanAccepted              bool             `json:"lean_accepted"`
	LeanError                 string           `json:"lean_error,omitempty"`
}

type judgeRule52PromptVariant struct {
	Source   string
	Name     string
	Path     string
	CopyPath string
	Text     string
}

func RunJudgeRule52(ctx context.Context, opts JudgeRule52Options) (JudgeRule52Summary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeRule52Summary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule52Summary{}, fmt.Errorf("output directory is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	fixtures, err := LoadJudgeRule52Fixtures(opts.FixturesPath)
	if err != nil {
		return JudgeRule52Summary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeRule52Summary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef := modelrequest.ModelRef{}
	var client *openai.Client
	if !opts.DryRun {
		modelRef, err = modelrequest.ParseModelRef(opts.Model)
		if err != nil {
			return JudgeRule52Summary{}, fmt.Errorf("parse --model: %w", err)
		}
		client, err = openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
		if err != nil {
			return JudgeRule52Summary{}, err
		}
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule52Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeRule52PromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName, opts.OutputDir)
	if err != nil {
		return JudgeRule52Summary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule52Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer resultsFile.Close()

	summary := newJudgeRule52Summary(opts, promptVariant, resultsPath, summaryPath)
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeRule52Fixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeRule52Summary{}, err
		}
		if err := encoder.Encode(result); err != nil {
			return JudgeRule52Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeRule52SummaryResult(&summary, result, weight)
	}
	finalizeJudgeRule52Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule52Summary{}, err
	}
	return summary, nil
}

func RescoreJudgeRule52(opts JudgeRule52RescoreOptions) (JudgeRule52Summary, error) {
	if strings.TrimSpace(opts.ResultsPath) == "" {
		return JudgeRule52Summary{}, fmt.Errorf("results path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule52Summary{}, fmt.Errorf("output directory is required")
	}
	results, err := readJudgeRule52Results(opts.ResultsPath)
	if err != nil {
		return JudgeRule52Summary{}, err
	}
	if len(results) == 0 {
		return JudgeRule52Summary{}, fmt.Errorf("no results loaded from %s", opts.ResultsPath)
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule52Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule52Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer resultsFile.Close()

	summary := JudgeRule52Summary{
		Evaluation:       "judge_rule52",
		Model:            results[0].Model,
		DryRun:           results[0].DryRun,
		PromptSource:     resultJudgeRule52PromptSource(results[0]),
		PromptName:       resultJudgeRule52PromptName(results[0]),
		PromptPath:       results[0].PromptPath,
		FixturesPath:     "rescored from " + opts.ResultsPath,
		OutputDir:        opts.OutputDir,
		ResultsPath:      resultsPath,
		SummaryPath:      summaryPath,
		ByReasonTag:      map[string]JudgeRule52Slice{},
		ByIssueFamily:    map[string]JudgeRule52Slice{},
		ByTier:           map[string]JudgeRule52Slice{},
		ByExpectedWinner: map[string]JudgeRule52Slice{},
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, result := range results {
		rescoreJudgeRule52Result(&result)
		if err := encoder.Encode(result); err != nil {
			return JudgeRule52Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyJudgeRule52SummaryResult(&summary, result, weight)
	}
	finalizeJudgeRule52Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule52Summary{}, err
	}
	return summary, nil
}

func newJudgeRule52Summary(opts JudgeRule52Options, promptVariant judgeRule52PromptVariant, resultsPath string, summaryPath string) JudgeRule52Summary {
	return JudgeRule52Summary{
		Evaluation:       "judge_rule52",
		Model:            opts.Model,
		DryRun:           opts.DryRun,
		PromptSource:     promptVariant.Source,
		PromptName:       promptVariant.Name,
		PromptPath:       promptVariant.Path,
		PromptCopyPath:   promptVariant.CopyPath,
		FixturesPath:     opts.FixturesPath,
		OutputDir:        opts.OutputDir,
		ResultsPath:      resultsPath,
		SummaryPath:      summaryPath,
		ByReasonTag:      map[string]JudgeRule52Slice{},
		ByIssueFamily:    map[string]JudgeRule52Slice{},
		ByTier:           map[string]JudgeRule52Slice{},
		ByExpectedWinner: map[string]JudgeRule52Slice{},
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
	}
}

func runJudgeRule52Fixture(
	ctx context.Context,
	opts JudgeRule52Options,
	promptVariant judgeRule52PromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeRule52Fixture,
) (JudgeRule52Result, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeRule52Result{}, err
	}
	state := BuildJudgeRule52State(fixture)
	roles := judgeRule52Roles()
	viewResp, err := opts.Engine.View(state, "judge")
	if err != nil {
		return JudgeRule52Result{}, fmt.Errorf("fixture %s view: %w", fixture.ID, err)
	}
	if ok, _ := viewResp["ok"].(bool); !ok {
		return JudgeRule52Result{}, fmt.Errorf("fixture %s view rejected: %s", fixture.ID, stringField(viewResp, "error"))
	}
	view, _ := viewResp["view"].(map[string]any)
	opportunityResp, err := opts.Engine.NextOpportunity(state, roles, 3)
	if err != nil {
		return JudgeRule52Result{}, fmt.Errorf("fixture %s next opportunity: %w", fixture.ID, err)
	}
	if ok, _ := opportunityResp["ok"].(bool); !ok {
		return JudgeRule52Result{}, fmt.Errorf("fixture %s next opportunity rejected: %s", fixture.ID, stringField(opportunityResp, "error"))
	}
	opportunity, _ := opportunityResp["opportunity"].(map[string]any)
	if len(opportunity) == 0 {
		return JudgeRule52Result{}, fmt.Errorf("fixture %s returned no opportunity", fixture.ID)
	}
	if stringField(opportunity, "role") != "judge" {
		return JudgeRule52Result{}, fmt.Errorf("fixture %s opportunity role = %q, want judge", fixture.ID, stringField(opportunity, "role"))
	}
	if !stringSliceContains(stringSliceField(opportunity, "allowed_tools"), JudgeRule52Tool) {
		return JudgeRule52Result{}, fmt.Errorf("fixture %s opportunity lacks %s", fixture.ID, JudgeRule52Tool)
	}
	input, err := buildJudgeRule52Input(view, opportunity, fixture, promptVariant)
	if err != nil {
		return JudgeRule52Result{}, fmt.Errorf("fixture %s build prompt: %w", fixture.ID, err)
	}
	tools, err := runner.BuildTools([]string{JudgeRule52Tool})
	if err != nil {
		return JudgeRule52Result{}, err
	}
	var resp openai.Response
	if opts.DryRun {
		resp = dryRunJudgeRule52Response(fixture)
	} else {
		callCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		resp, err = client.CreateResponse(callCtx, modelRef.Model, input, tools, "", opts.Temperature)
		cancel()
		if err != nil {
			return JudgeRule52Result{}, fmt.Errorf("fixture %s model call: %w", fixture.ID, err)
		}
	}
	result := scoreJudgeRule52Response(fixture, opts.Model, opts.DryRun, state, view, opportunity, input, resp)
	result.PromptSource = promptVariant.Source
	result.PromptName = promptVariant.Name
	result.PromptPath = promptVariant.Path
	if result.InvalidReason == "" {
		decision := map[string]any{
			"kind":      "tool",
			"tool_name": JudgeRule52Tool,
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

func LoadJudgeRule52Fixtures(path string) ([]JudgeRule52Fixture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeRule52Fixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeRule52Fixture
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

func readJudgeRule52Results(path string) ([]JudgeRule52Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open results %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 32*1024*1024)
	out := make([]JudgeRule52Result, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var result JudgeRule52Result
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

func loadJudgeRule52PromptVariant(path string, name string, outputDir string) (judgeRule52PromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeRule52PromptVariant{Source: "production", Name: name}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeRule52PromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeRule52PromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	copyPath := filepath.Join(outputDir, "opportunity_prompt.md")
	if err := os.WriteFile(copyPath, raw, 0o644); err != nil {
		return judgeRule52PromptVariant{}, fmt.Errorf("copy opportunity prompt to %s: %w", copyPath, err)
	}
	return judgeRule52PromptVariant{Source: "file:" + path, Name: name, Path: path, CopyPath: copyPath, Text: text}, nil
}

func (f JudgeRule52Fixture) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("fixture missing id")
	}
	if f.Tier < 1 {
		return fmt.Errorf("fixture %s tier must be positive", f.ID)
	}
	if strings.TrimSpace(f.IssueFamily) == "" {
		return fmt.Errorf("fixture %s missing issue_family", f.ID)
	}
	if strings.TrimSpace(f.CaseTheme) == "" {
		return fmt.Errorf("fixture %s missing case_theme", f.ID)
	}
	if strings.TrimSpace(f.ComplaintText) == "" {
		return fmt.Errorf("fixture %s missing complaint_text", f.ID)
	}
	if strings.TrimSpace(f.AnswerText) == "" {
		return fmt.Errorf("fixture %s missing answer_text", f.ID)
	}
	if strings.TrimSpace(f.PlaintiffTheory) == "" {
		return fmt.Errorf("fixture %s missing plaintiff_theory", f.ID)
	}
	if strings.TrimSpace(f.DefendantTheory) == "" {
		return fmt.Errorf("fixture %s missing defendant_theory", f.ID)
	}
	if len(nonemptyRule52Strings(f.AdmittedEvidence)) == 0 {
		return fmt.Errorf("fixture %s missing admitted_evidence", f.ID)
	}
	if strings.TrimSpace(f.PlaintiffClosing) == "" {
		return fmt.Errorf("fixture %s missing plaintiff_closing", f.ID)
	}
	if strings.TrimSpace(f.DefendantClosing) == "" {
		return fmt.Errorf("fixture %s missing defendant_closing", f.ID)
	}
	if f.ExpectedWinner != "plaintiff" && f.ExpectedWinner != "defendant" {
		return fmt.Errorf("fixture %s invalid expected_winner %q", f.ID, f.ExpectedWinner)
	}
	if len(nonemptyRule52Strings(f.RequiredConcepts)) == 0 {
		return fmt.Errorf("fixture %s missing required_concepts", f.ID)
	}
	if len(nonemptyRule52Strings(f.ExpectedReasonTags)) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeRule52State(f JudgeRule52Fixture) map[string]any {
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           "Judge Eval Court",
		"court_profile":        nil,
		"policy":               defaultJudgeEvalPolicy(),
		"state_version":        0,
		"passed_opportunities": []any{},
		"case": map[string]any{
			"case_id":                       "judge-rule52-" + strings.TrimSpace(f.ID),
			"caption":                       strings.TrimSpace(f.CaseTheme),
			"judge":                         "Judge Eval",
			"filed_on":                      "2026-07-14",
			"auto_rule11":                   false,
			"status":                        "trial",
			"trial_mode":                    "bench",
			"phase":                         "verdict_return",
			"last_pleading_served_on":       "2026-07-01",
			"jury_demanded_on":              "",
			"jury_configuration":            nil,
			"single_claim":                  defaultJudgeEvalClaim(),
			"jurisdictional_allegations":    nil,
			"jurors":                        []any{},
			"juror_questionnaire":           []any{},
			"juror_questionnaire_responses": []any{},
			"voir_dire_exchanges":           []any{},
			"for_cause_challenges":          []any{},
			"deliberation_round":            1,
			"juror_votes":                   []any{},
			"jury_verdict":                  nil,
			"hung_jury":                     nil,
			"contempt_counts":               []any{},
			"protective_orders":             []any{},
			"bench_findings":                []any{},
			"bench_conclusions":             []any{},
			"juror_explanations":            []any{},
			"local_rule_overrides":          []any{},
			"limit_usage":                   []any{},
			"rule56_window_closed_for":      []any{},
			"case_files":                    []any{},
			"file_events":                   []any{},
			"rule68_offers":                 []any{},
			"technical_reports":             []any{},
			"monetary_judgment":             0.0,
			"docket":                        judgeRule52Docket(f),
			"decision_traces":               judgeRule52DecisionTraces(f),
		},
	}
}

func judgeRule52Docket(f JudgeRule52Fixture) []any {
	entries := []any{
		map[string]any{"title": "Complaint filed", "description": "plaintiff: " + strings.TrimSpace(f.ComplaintText)},
		map[string]any{"title": "Answer filed", "description": "defendant: " + strings.TrimSpace(f.AnswerText)},
		map[string]any{"title": "Trial mode resolved", "description": "bench trial"},
		map[string]any{"title": "Opening statement - plaintiff", "description": strings.TrimSpace(f.PlaintiffTheory)},
		map[string]any{"title": "Opening statement - defendant", "description": strings.TrimSpace(f.DefendantTheory)},
		map[string]any{"title": "Trial theory - plaintiff", "description": strings.TrimSpace(f.PlaintiffTheory)},
		map[string]any{"title": "Trial theory - defendant", "description": strings.TrimSpace(f.DefendantTheory)},
	}
	for i, evidence := range nonemptyRule52Strings(f.AdmittedEvidence) {
		entries = append(entries, map[string]any{"title": fmt.Sprintf("Admitted bench evidence %d", i+1), "description": evidence})
	}
	for i, evidence := range nonemptyRule52Strings(f.ExcludedEvidence) {
		entries = append(entries, map[string]any{"title": fmt.Sprintf("Excluded bench evidence %d", i+1), "description": evidence})
	}
	entries = append(entries,
		map[string]any{"title": "Plaintiff case rested", "description": "plaintiff rested"},
		map[string]any{"title": "Defense case rested", "description": "defendant rested"},
		map[string]any{"title": "Closing argument - plaintiff", "description": strings.TrimSpace(f.PlaintiffClosing)},
		map[string]any{"title": "Closing argument - defendant", "description": strings.TrimSpace(f.DefendantClosing)},
	)
	return entries
}

func judgeRule52DecisionTraces(f JudgeRule52Fixture) []any {
	traces := []any{
		map[string]any{"action": "file_complaint", "outcome": "filed", "citations": []any{"FRCP 3", "FRCP 8(a)"}},
		map[string]any{"action": "file_answer", "outcome": "filed", "citations": []any{"FRCP 8(b)"}},
		map[string]any{"action": "resolve_trial_mode", "outcome": "bench", "citations": []any{"FRCP 39"}},
		map[string]any{"action": "transition_case", "outcome": "trial", "citations": []any{}},
	}
	for i := range nonemptyRule52Strings(f.AdmittedEvidence) {
		traces = append(traces, map[string]any{"action": "offer_exhibit", "outcome": fmt.Sprintf("admitted-%d", i+1), "citations": []any{"FRE 401", "FRE 402"}})
	}
	for i := range nonemptyRule52Strings(f.ExcludedEvidence) {
		traces = append(traces, map[string]any{"action": "offer_exhibit", "outcome": fmt.Sprintf("excluded-%d", i+1), "citations": []any{"FRE 401", "FRE 403"}})
	}
	traces = append(traces,
		map[string]any{"action": "rest_case", "outcome": "plaintiff", "citations": []any{}},
		map[string]any{"action": "rest_case", "outcome": "defendant", "citations": []any{}},
		map[string]any{"action": "deliver_closing_argument", "outcome": "plaintiff", "citations": []any{}},
		map[string]any{"action": "deliver_closing_argument", "outcome": "defendant", "citations": []any{}},
		map[string]any{"action": "advance_trial_phase", "outcome": "verdict_return", "citations": []any{}},
	)
	return traces
}

func buildJudgeRule52Input(
	view map[string]any,
	opportunity map[string]any,
	fixture JudgeRule52Fixture,
	promptVariant judgeRule52PromptVariant,
) ([]map[string]any, error) {
	role := judgeRule52Role()
	systemPrompt, err := buildJudgeRule52SystemPrompt(role, view)
	if err != nil {
		return nil, err
	}
	userPrompt, err := buildJudgeRule52OpportunityPrompt(opportunity, fixture, promptVariant)
	if err != nil {
		return nil, err
	}
	return []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}, nil
}

func scoreJudgeRule52Response(
	fixture JudgeRule52Fixture,
	model string,
	dryRun bool,
	state map[string]any,
	view map[string]any,
	opportunity map[string]any,
	input []map[string]any,
	resp openai.Response,
) JudgeRule52Result {
	result := JudgeRule52Result{
		ID:                 fixture.ID,
		Tier:               fixture.Tier,
		IssueFamily:        strings.TrimSpace(fixture.IssueFamily),
		CaseTheme:          strings.TrimSpace(fixture.CaseTheme),
		ComplaintText:      strings.TrimSpace(fixture.ComplaintText),
		AnswerText:         strings.TrimSpace(fixture.AnswerText),
		PlaintiffTheory:    strings.TrimSpace(fixture.PlaintiffTheory),
		DefendantTheory:    strings.TrimSpace(fixture.DefendantTheory),
		AdmittedEvidence:   nonemptyRule52Strings(fixture.AdmittedEvidence),
		ExcludedEvidence:   nonemptyRule52Strings(fixture.ExcludedEvidence),
		PlaintiffClosing:   strings.TrimSpace(fixture.PlaintiffClosing),
		DefendantClosing:   strings.TrimSpace(fixture.DefendantClosing),
		ExpectedWinner:     strings.TrimSpace(fixture.ExpectedWinner),
		ExpectedAmountText: strings.TrimSpace(fixture.ExpectedAmountText),
		RequiredConcepts:   nonemptyRule52Strings(fixture.RequiredConcepts),
		ProhibitedConcepts: nonemptyRule52Strings(fixture.ProhibitedConcepts),
		ExpectedReasonTags: nonemptyRule52Strings(fixture.ExpectedReasonTags),
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
	payload, invalid := extractJudgeRule52Payload(resp)
	if invalid != "" {
		result.InvalidReason = invalid
		return result
	}
	result.ToolPayload = payload
	text, ok := payload["text"].(string)
	if !ok {
		result.InvalidReason = "malformed_text"
		return result
	}
	result.OpinionText = strings.TrimSpace(text)
	if result.OpinionText == "" {
		result.InvalidReason = "empty_text"
		return result
	}
	rescoreJudgeRule52Result(&result)
	return result
}

func rescoreJudgeRule52Result(result *JudgeRule52Result) {
	if result == nil || result.InvalidReason != "" {
		return
	}
	result.DetectedWinner = detectJudgeRule52Winner(result.OpinionText)
	result.WinnerCorrect = result.DetectedWinner == result.ExpectedWinner
	result.AmountCorrect = judgeRule52AmountCorrect(result.OpinionText, result.ExpectedAmountText)
	result.MissingRequiredConcepts = missingJudgeRule52Concepts(result.OpinionText, result.RequiredConcepts)
	result.PresentProhibitedConcepts = presentJudgeRule52Concepts(result.OpinionText, result.ProhibitedConcepts)
	result.RequiredCorrect = len(result.MissingRequiredConcepts) == 0
	result.ProhibitedCorrect = len(result.PresentProhibitedConcepts) == 0
	result.SeparationCorrect = judgeRule52SeparationCorrect(result.OpinionText)
	result.MatchedReasonTags = matchedJudgeRule52ReasonTags(result.OpinionText, result.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
	result.OutcomeCorrect = result.WinnerCorrect &&
		result.AmountCorrect &&
		result.RequiredCorrect &&
		result.ProhibitedCorrect &&
		result.SeparationCorrect
}

func extractJudgeRule52Payload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeRule52Tool {
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

func dryRunJudgeRule52Response(f JudgeRule52Fixture) openai.Response {
	opinion := dryRunJudgeRule52Opinion(f)
	return openai.Response{
		ResponseID: "dry-run-" + strings.TrimSpace(f.ID),
		ToolCalls: []openai.ToolCall{{
			CallID: "dry-run-call-" + strings.TrimSpace(f.ID),
			Name:   JudgeRule52Tool,
			Arguments: map[string]any{
				"text": opinion,
			},
		}},
	}
}

func dryRunJudgeRule52Opinion(f JudgeRule52Fixture) string {
	judgment := "Judgment for " + strings.TrimSpace(f.ExpectedWinner)
	if strings.TrimSpace(f.ExpectedAmountText) != "" {
		judgment += " in the amount of $" + strings.TrimSpace(f.ExpectedAmountText)
	}
	return "Findings of Fact: " + strings.Join(nonemptyRule52Strings(f.RequiredConcepts), "; ") +
		". Conclusions of Law: " + strings.Join(nonemptyRule52Strings(f.ExpectedReasonTags), "; ") +
		". Judgment: " + judgment + "."
}

func buildJudgeRule52SystemPrompt(role spec.RoleSpec, view map[string]any) (string, error) {
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

func buildJudgeRule52OpportunityPrompt(
	opportunity map[string]any,
	fixture JudgeRule52Fixture,
	promptVariant judgeRule52PromptVariant,
) (string, error) {
	tools, err := runner.BuildTools([]string{JudgeRule52Tool})
	if err != nil {
		return "", err
	}
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		objective = renderJudgeRule52PromptTemplate(promptVariant.Text, fixture, opportunity)
	}
	lines := []string{
		"Current opportunity:",
		stringField(opportunity, "actor_message"),
		formatJudgeRule52Objective(objective),
		"Phase: " + stringField(opportunity, "phase"),
		"Allowed actions: " + strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
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

func formatJudgeRule52Objective(objective string) string {
	objective = strings.TrimSpace(objective)
	if strings.Contains(objective, "\n") {
		return "Objective:\n" + objective
	}
	return "Objective: " + objective
}

func renderJudgeRule52PromptTemplate(template string, fixture JudgeRule52Fixture, opportunity map[string]any) string {
	replacer := strings.NewReplacer(
		"{{production_objective}}", stringField(opportunity, "objective"),
		"{{actor_message}}", stringField(opportunity, "actor_message"),
		"{{phase}}", stringField(opportunity, "phase"),
		"{{allowed_tools}}", strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
		"{{fixture_id}}", strings.TrimSpace(fixture.ID),
		"{{tier}}", strconv.Itoa(fixture.Tier),
		"{{issue_family}}", strings.TrimSpace(fixture.IssueFamily),
		"{{case_theme}}", strings.TrimSpace(fixture.CaseTheme),
		"{{complaint_text}}", strings.TrimSpace(fixture.ComplaintText),
		"{{answer_text}}", strings.TrimSpace(fixture.AnswerText),
		"{{plaintiff_theory}}", strings.TrimSpace(fixture.PlaintiffTheory),
		"{{defendant_theory}}", strings.TrimSpace(fixture.DefendantTheory),
		"{{admitted_evidence}}", strings.Join(nonemptyRule52Strings(fixture.AdmittedEvidence), " | "),
		"{{excluded_evidence}}", strings.Join(nonemptyRule52Strings(fixture.ExcludedEvidence), " | "),
		"{{plaintiff_closing}}", strings.TrimSpace(fixture.PlaintiffClosing),
		"{{defendant_closing}}", strings.TrimSpace(fixture.DefendantClosing),
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
	return strings.TrimSpace(replacer.Replace(template))
}

func judgeRule52Role() spec.RoleSpec {
	return spec.RoleSpec{
		Name:           "judge",
		Instructions:   "Judge for procedural rulings, trial control, and judgment entry.",
		PromptPreamble: casegen.JudgeRuntimeBrief(),
		AllowedTools:   []string{JudgeRule52Tool},
	}
}

func judgeRule52Roles() []map[string]any {
	return []map[string]any{{"role": "judge", "allowed_tools": []string{JudgeRule52Tool}}}
}

func applyJudgeRule52SummaryResult(summary *JudgeRule52Summary, result JudgeRule52Result, weight float64) {
	summary.Total++
	if result.InvalidReason != "" {
		summary.Invalid++
	} else {
		if result.WinnerCorrect {
			summary.WinnerCorrect++
		}
		if result.AmountCorrect {
			summary.AmountCorrect++
		}
		if result.RequiredCorrect {
			summary.RequiredCorrect++
		}
		if result.ProhibitedCorrect {
			summary.ProhibitedCorrect++
		}
		if result.SeparationCorrect {
			summary.SeparationCorrect++
		}
		if result.OutcomeCorrect {
			summary.Correct++
		} else {
			classifyJudgeRule52WinnerError(result, &summary.FalsePlaintiffJudgment, &summary.FalseDefenseJudgment)
		}
	}
	if result.ReasonCorrect {
		summary.ReasonCorrect++
	}
	for _, tag := range result.ExpectedReasonTags {
		updateJudgeRule52Slice(summary.ByReasonTag, tag, result, weight)
	}
	updateJudgeRule52Slice(summary.ByIssueFamily, result.IssueFamily, result, weight)
	updateJudgeRule52Slice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
	updateJudgeRule52Slice(summary.ByExpectedWinner, result.ExpectedWinner, result, weight)
}

func updateJudgeRule52Slice(m map[string]JudgeRule52Slice, key string, result JudgeRule52Result, weight float64) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unspecified"
	}
	s := m[key]
	s.Total++
	s.Weight += weight
	if result.InvalidReason != "" {
		s.Invalid++
	} else {
		if result.WinnerCorrect {
			s.WinnerCorrect++
		}
		if result.OutcomeCorrect {
			s.Correct++
			s.CorrectWeight += weight
		} else {
			classifyJudgeRule52WinnerError(result, &s.FalsePlaintiffJudgment, &s.FalseDefenseJudgment)
		}
	}
	m[key] = s
}

func classifyJudgeRule52WinnerError(result JudgeRule52Result, falsePlaintiff *int, falseDefense *int) {
	if result.DetectedWinner == "plaintiff" && result.ExpectedWinner == "defendant" {
		(*falsePlaintiff)++
	}
	if result.DetectedWinner == "defendant" && result.ExpectedWinner == "plaintiff" {
		(*falseDefense)++
	}
}

func finalizeJudgeRule52Summary(summary *JudgeRule52Summary, totalWeight float64, correctWeight float64) {
	if summary.Total > 0 {
		summary.Accuracy = float64(summary.Correct) / float64(summary.Total)
		summary.WinnerAccuracy = float64(summary.WinnerCorrect) / float64(summary.Total)
		summary.InvalidRate = float64(summary.Invalid) / float64(summary.Total)
	}
	if totalWeight > 0 {
		summary.WeightedAccuracy = correctWeight / totalWeight
	}
	finalizeJudgeRule52Slices(summary.ByReasonTag)
	finalizeJudgeRule52Slices(summary.ByIssueFamily)
	finalizeJudgeRule52Slices(summary.ByTier)
	finalizeJudgeRule52Slices(summary.ByExpectedWinner)
}

func finalizeJudgeRule52Slices(m map[string]JudgeRule52Slice) {
	for key, s := range m {
		if s.Total > 0 {
			s.Accuracy = float64(s.Correct) / float64(s.Total)
			s.WinnerAccuracy = float64(s.WinnerCorrect) / float64(s.Total)
		}
		if s.Weight > 0 {
			s.WeightedAccuracy = s.CorrectWeight / s.Weight
		}
		m[key] = s
	}
}

func detectJudgeRule52Winner(text string) string {
	normalized := normalizeReasonText(text)
	plaintiff := judgeRule52ContainsAny(normalized, []string{
		"judgment for plaintiff",
		"judgment for the plaintiff",
		"judgment will be entered for plaintiff",
		"judgment will be entered for the plaintiff",
		"judgment is entered for plaintiff",
		"judgment is entered for the plaintiff",
		"judgment entered for plaintiff",
		"judgment entered for the plaintiff",
		"liability is found for plaintiff",
		"liability is found for the plaintiff",
		"plaintiff prevails",
		"find for plaintiff",
		"finds for plaintiff",
		"plaintiff has proved",
	})
	defendant := judgeRule52ContainsAny(normalized, []string{
		"judgment for defendant",
		"judgment for the defendant",
		"judgment will be entered for defendant",
		"judgment will be entered for the defendant",
		"judgment is entered for defendant",
		"judgment is entered for the defendant",
		"judgment entered for defendant",
		"judgment entered for the defendant",
		"defendant prevails",
		"find for defendant",
		"finds for defendant",
		"plaintiff failed to prove",
		"plaintiff has not proved",
	})
	if plaintiff && !defendant {
		return "plaintiff"
	}
	if defendant && !plaintiff {
		return "defendant"
	}
	if plaintiff && defendant {
		return "mixed"
	}
	return ""
}

func judgeRule52AmountCorrect(text string, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return true
	}
	expectedDigits := digitsOnly(expected)
	if expectedDigits == "" {
		return true
	}
	return strings.Contains(digitsOnly(text), expectedDigits)
}

func missingJudgeRule52Concepts(text string, concepts []string) []string {
	normalized := normalizeReasonText(text)
	missing := make([]string, 0)
	for _, concept := range nonemptyRule52Strings(concepts) {
		if !judgeRule52ConceptPresent(normalized, text, concept) {
			missing = append(missing, concept)
		}
	}
	sort.Strings(missing)
	return missing
}

func presentJudgeRule52Concepts(text string, concepts []string) []string {
	normalized := normalizeReasonText(text)
	present := make([]string, 0)
	for _, concept := range nonemptyRule52Strings(concepts) {
		if judgeRule52ProhibitedConceptPresent(normalized, concept) {
			present = append(present, concept)
		}
	}
	sort.Strings(present)
	return present
}

func judgeRule52SeparationCorrect(text string) bool {
	normalized := normalizeReasonText(text)
	return strings.Contains(normalized, "finding") &&
		strings.Contains(normalized, "conclusion") &&
		strings.Contains(normalized, "judgment")
}

func matchedJudgeRule52ReasonTags(text string, expected []string) []string {
	normalized := normalizeReasonText(text)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if judgeRule52ReasonMatchesTag(normalized, tag) {
			matches = append(matches, tag)
		}
	}
	sort.Strings(matches)
	return matches
}

func judgeRule52ReasonMatchesTag(normalizedText string, tag string) bool {
	normalizedTag := normalizeReasonText(tag)
	if strings.Contains(normalizedText, normalizedTag) {
		return true
	}
	for _, keyword := range judgeRule52ReasonTagKeywords()[tag] {
		if strings.Contains(normalizedText, keyword) {
			return true
		}
	}
	return false
}

func judgeRule52ReasonTagKeywords() map[string][]string {
	return map[string][]string{
		"breach_proved":       {"breach proved", "breached", "failed to pay", "repudiated"},
		"no_contract":         {"no contract", "no acceptance", "unsigned draft"},
		"credibility":         {"credible", "credits", "inconsistent"},
		"causation_gap":       {"causation", "caused", "lost sale"},
		"excluded_evidence":   {"excluded", "not admitted", "does not consider"},
		"damages_proved":      {"damages", "amount", "proved"},
		"damages_gap":         {"speculative", "not proved", "damages not proved"},
		"damages_limited":     {"limited", "direct damages", "consequential"},
		"authentication":      {"authenticated", "authentication", "not admitted"},
		"agency":              {"agency", "authority", "agent"},
		"notice":              {"notice", "timely"},
		"fact_law_separation": {"findings", "conclusions", "judgment"},
	}
}

func judgeRule52ContainsAny(text string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(text, normalizeReasonText(phrase)) {
			return true
		}
	}
	return false
}

func judgeRule52ConceptPresent(normalizedText string, rawText string, concept string) bool {
	normalizedConcept := normalizeReasonText(concept)
	if strings.Contains(normalizedText, normalizedConcept) {
		return true
	}
	conceptDigits := digitsOnly(concept)
	if conceptIsOnlyNumeric(concept) && conceptDigits != "" && strings.Contains(digitsOnly(rawText), conceptDigits) {
		return true
	}
	for _, alias := range judgeRule52ConceptAliases()[normalizedConcept] {
		if strings.Contains(normalizedText, alias) {
			return true
		}
	}
	return false
}

func judgeRule52ProhibitedConceptPresent(normalizedText string, concept string) bool {
	normalizedConcept := normalizeReasonText(concept)
	if strings.Contains(normalizedText, normalizedConcept) {
		return true
	}
	for _, alias := range judgeRule52ConceptAliases()[normalizedConcept] {
		if strings.Contains(normalizedText, alias) {
			return true
		}
	}
	return false
}

func judgeRule52ConceptAliases() map[string][]string {
	return map[string][]string{
		"no acceptance":                    {"not accepted", "never accepted", "no accepted"},
		"unsigned draft":                   {"draft order", "no completed signature block", "contains no completed signature block"},
		"no contract":                      {"without a contract", "contract formation with defendant", "failed to prove the existence of a contract"},
		"credible":                         {"credit this", "credits", "credited", "court credits"},
		"lost resale not proved":           {"failed to prove causation", "did not prove causation", "causation and damages are not proven", "cancellation occurred for reasons independent", "resale failed because", "does not establish causation", "plaintiff has not proven causation", "not proven causation"},
		"excluded screenshot":              {"screenshot px 9", "px 9"},
		"not considered":                   {"does not consider", "disregarded", "cannot rely", "not evidence"},
		"extra freight":                    {"additional freight", "freight charges"},
		"consequential damages not proved": {"has not proved consequential damages", "did not prove lost profits", "lost profits are therefore not proven", "lost profit claim lacks", "lost profit model lacks", "not proven with reasonable certainty", "not recoverable", "not carried its burden on consequential damages"},
		"damages not proved":               {"failed to prove damages", "did not prove any damages amount", "did not carry its burden to prove the amount of damages", "plaintiff failed to prove damages", "not proven the amount of damages"},
		"no source data":                   {"does not identify source data", "without identifying or substantiating the underlying data", "underlying source data", "not supported by identified source records", "no admitted source data"},
		"signed service agreement":         {"executed service agreement", "parties executed service agreement"},
		"authenticated audit log":          {"audit log was authenticated", "audit log al 3 was authenticated", "audit log (al 3) was authenticated", "system audit log (al 3) was authenticated", "system audit log al 3 was authenticated"},
		"text messages not admitted":       {"text message exhibit", "text-message exhibit", "px 4", "not part of the evidentiary record"},
		"no change order":                  {"no admitted change order", "no other admitted writing"},
		"no authority":                     {"no admitted evidence shows that defendant communicated", "not proven that the contractor had actual or apparent authority", "actual authority was not proven", "apparent authority was not proven", "did not prove apparent authority", "negate any express or implied actual authority", "failed to prove that the contractor had actual or apparent authority"},
		"prior orders":                     {"prior lee orders", "prior lee initiated orders", "previously accepted two prior", "accepted two prior"},
		"no prior purchase":                {"no prior course of dealing", "no prior acceptance", "no course of dealing"},
		"notice condition failed":          {"condition precedent failed", "did not satisfy the condition precedent", "did not meet the contractual written notice requirement", "did not satisfy the contract s written notice deadline", "condition failed", "did not prove compliance with the contractual notice requirement", "noncompliance with the contract s ten day written notice requirement"},
		"on time delivery":                 {"timely delivery", "on time delivery"},
		"on-time delivery":                 {"timely delivery", "on time delivery"},
		"credible contemporaneous":         {"contemporaneous service log and receiving message were more reliable", "contemporaneous service log and receiving message more reliable", "contemporaneous documentary evidence", "contemporaneous records", "reliable admissible evidence"},
	}
}

var rule52Digits = regexp.MustCompile(`\D+`)

func digitsOnly(value string) string {
	return rule52Digits.ReplaceAllString(value, "")
}

func conceptIsOnlyNumeric(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '$', ',', '.', ' ':
			continue
		default:
			return false
		}
	}
	return true
}

func nonemptyRule52Strings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func resultJudgeRule52PromptSource(result JudgeRule52Result) string {
	if strings.TrimSpace(result.PromptSource) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptSource)
}

func resultJudgeRule52PromptName(result JudgeRule52Result) string {
	if strings.TrimSpace(result.PromptName) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptName)
}
