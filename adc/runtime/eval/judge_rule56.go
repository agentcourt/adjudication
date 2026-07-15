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

const JudgeRule56Tool = "decide_rule56_motion"

type JudgeRule56Fixture struct {
	ID                  string   `json:"id"`
	Tier                int      `json:"tier"`
	IssueFamily         string   `json:"issue_family"`
	CaseTheme           string   `json:"case_theme"`
	MovingParty         string   `json:"moving_party"`
	MotionScope         string   `json:"motion_scope,omitempty"`
	RequestText         string   `json:"request_text"`
	StatementOfFacts    string   `json:"statement_of_undisputed_facts"`
	EvidenceRefs        []string `json:"evidence_refs,omitempty"`
	OppositionText      string   `json:"opposition_text"`
	ReplyText           string   `json:"reply_text,omitempty"`
	ExpectedDisposition string   `json:"expected_disposition"`
	ExpectedSurviving   []string `json:"expected_surviving_issues,omitempty"`
	ExpectedReasonTags  []string `json:"expected_reason_tags"`
	Severity            float64  `json:"severity"`
	ContextNotes        string   `json:"context_notes,omitempty"`
}

type JudgeRule56Options struct {
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

type JudgeRule56Summary struct {
	Evaluation        string                      `json:"evaluation"`
	Model             string                      `json:"model"`
	DryRun            bool                        `json:"dry_run"`
	PromptSource      string                      `json:"prompt_source"`
	PromptName        string                      `json:"prompt_name"`
	PromptPath        string                      `json:"prompt_path,omitempty"`
	PromptCopyPath    string                      `json:"prompt_copy_path,omitempty"`
	FixturesPath      string                      `json:"fixtures_path"`
	OutputDir         string                      `json:"output_dir"`
	ResultsPath       string                      `json:"results_path"`
	SummaryPath       string                      `json:"summary_path"`
	Total             int                         `json:"total"`
	Correct           int                         `json:"correct"`
	ReasonCorrect     int                         `json:"reason_correct"`
	Invalid           int                         `json:"invalid"`
	FalseGrants       int                         `json:"false_grants"`
	FalseDenials      int                         `json:"false_denials"`
	PartialMismatches int                         `json:"partial_mismatches"`
	Accuracy          float64                     `json:"accuracy"`
	WeightedAccuracy  float64                     `json:"weighted_accuracy"`
	FalseGrantRate    float64                     `json:"false_grant_rate"`
	FalseDenialRate   float64                     `json:"false_denial_rate"`
	InvalidRate       float64                     `json:"invalid_rate"`
	ByReasonTag       map[string]JudgeRule56Slice `json:"by_reason_tag"`
	ByIssueFamily     map[string]JudgeRule56Slice `json:"by_issue_family"`
	ByTier            map[string]JudgeRule56Slice `json:"by_tier"`
	ByMovingParty     map[string]JudgeRule56Slice `json:"by_moving_party"`
	GeneratedAt       string                      `json:"generated_at"`
}

type JudgeRule56Slice struct {
	Total             int     `json:"total"`
	Correct           int     `json:"correct"`
	FalseGrants       int     `json:"false_grants"`
	FalseDenials      int     `json:"false_denials"`
	PartialMismatches int     `json:"partial_mismatches"`
	Invalid           int     `json:"invalid"`
	Weight            float64 `json:"weight"`
	CorrectWeight     float64 `json:"correct_weight"`
	Accuracy          float64 `json:"accuracy"`
	WeightedAccuracy  float64 `json:"weighted_accuracy"`
}

type JudgeRule56Result struct {
	ID                  string           `json:"id"`
	Tier                int              `json:"tier"`
	IssueFamily         string           `json:"issue_family"`
	CaseTheme           string           `json:"case_theme"`
	MovingParty         string           `json:"moving_party"`
	MotionScope         string           `json:"motion_scope,omitempty"`
	RequestText         string           `json:"request_text"`
	StatementOfFacts    string           `json:"statement_of_undisputed_facts"`
	EvidenceRefs        []string         `json:"evidence_refs,omitempty"`
	OppositionText      string           `json:"opposition_text"`
	ReplyText           string           `json:"reply_text,omitempty"`
	ExpectedDisposition string           `json:"expected_disposition"`
	ExpectedSurviving   []string         `json:"expected_surviving_issues,omitempty"`
	ExpectedReasonTags  []string         `json:"expected_reason_tags"`
	Severity            float64          `json:"severity"`
	ContextNotes        string           `json:"context_notes,omitempty"`
	Model               string           `json:"model"`
	DryRun              bool             `json:"dry_run"`
	PromptSource        string           `json:"prompt_source"`
	PromptName          string           `json:"prompt_name"`
	PromptPath          string           `json:"prompt_path,omitempty"`
	State               map[string]any   `json:"state"`
	View                map[string]any   `json:"view"`
	Opportunity         map[string]any   `json:"opportunity"`
	Input               []map[string]any `json:"input"`
	RawResponse         map[string]any   `json:"raw_response"`
	ToolPayload         map[string]any   `json:"tool_payload,omitempty"`
	Disposition         string           `json:"disposition,omitempty"`
	SurvivingIssues     []string         `json:"surviving_issues,omitempty"`
	Reasoning           string           `json:"reasoning,omitempty"`
	MatchedReasonTags   []string         `json:"matched_reason_tags"`
	OutcomeCorrect      bool             `json:"outcome_correct"`
	ReasonCorrect       bool             `json:"reason_correct"`
	InvalidReason       string           `json:"invalid_reason,omitempty"`
	LeanAccepted        bool             `json:"lean_accepted"`
	LeanError           string           `json:"lean_error,omitempty"`
}

type judgeRule56PromptVariant struct {
	Source   string
	Name     string
	Path     string
	CopyPath string
	Text     string
}

func RunJudgeRule56(ctx context.Context, opts JudgeRule56Options) (JudgeRule56Summary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeRule56Summary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeRule56Summary{}, fmt.Errorf("output directory is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	fixtures, err := LoadJudgeRule56Fixtures(opts.FixturesPath)
	if err != nil {
		return JudgeRule56Summary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeRule56Summary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef := modelrequest.ModelRef{}
	var client *openai.Client
	if !opts.DryRun {
		modelRef, err = modelrequest.ParseModelRef(opts.Model)
		if err != nil {
			return JudgeRule56Summary{}, fmt.Errorf("parse --model: %w", err)
		}
		client, err = openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
		if err != nil {
			return JudgeRule56Summary{}, err
		}
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeRule56Summary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeRule56PromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName, opts.OutputDir)
	if err != nil {
		return JudgeRule56Summary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeRule56Summary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer resultsFile.Close()

	summary := JudgeRule56Summary{
		Evaluation:     "judge_rule56",
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
		ByReasonTag:    map[string]JudgeRule56Slice{},
		ByIssueFamily:  map[string]JudgeRule56Slice{},
		ByTier:         map[string]JudgeRule56Slice{},
		ByMovingParty:  map[string]JudgeRule56Slice{},
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeRule56Fixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeRule56Summary{}, err
		}
		if err := encoder.Encode(result); err != nil {
			return JudgeRule56Summary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applyRule56SummaryResult(&summary, result, weight)
	}
	finalizeRule56Summary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeRule56Summary{}, err
	}
	return summary, nil
}

func runJudgeRule56Fixture(
	ctx context.Context,
	opts JudgeRule56Options,
	promptVariant judgeRule56PromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeRule56Fixture,
) (JudgeRule56Result, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeRule56Result{}, err
	}
	state := BuildJudgeRule56State(fixture)
	roles := judgeRule56Roles()
	viewResp, err := opts.Engine.View(state, "judge")
	if err != nil {
		return JudgeRule56Result{}, fmt.Errorf("fixture %s view: %w", fixture.ID, err)
	}
	if ok, _ := viewResp["ok"].(bool); !ok {
		return JudgeRule56Result{}, fmt.Errorf("fixture %s view rejected: %s", fixture.ID, stringField(viewResp, "error"))
	}
	view, _ := viewResp["view"].(map[string]any)
	opportunityResp, err := opts.Engine.NextOpportunity(state, roles, 3)
	if err != nil {
		return JudgeRule56Result{}, fmt.Errorf("fixture %s next opportunity: %w", fixture.ID, err)
	}
	if ok, _ := opportunityResp["ok"].(bool); !ok {
		return JudgeRule56Result{}, fmt.Errorf("fixture %s next opportunity rejected: %s", fixture.ID, stringField(opportunityResp, "error"))
	}
	opportunity, _ := opportunityResp["opportunity"].(map[string]any)
	if len(opportunity) == 0 {
		return JudgeRule56Result{}, fmt.Errorf("fixture %s returned no opportunity", fixture.ID)
	}
	if stringField(opportunity, "role") != "judge" {
		return JudgeRule56Result{}, fmt.Errorf("fixture %s opportunity role = %q, want judge", fixture.ID, stringField(opportunity, "role"))
	}
	if !stringSliceContains(stringSliceField(opportunity, "allowed_tools"), JudgeRule56Tool) {
		return JudgeRule56Result{}, fmt.Errorf("fixture %s opportunity lacks %s", fixture.ID, JudgeRule56Tool)
	}
	input, err := buildJudgeRule56Input(view, opportunity, fixture, promptVariant)
	if err != nil {
		return JudgeRule56Result{}, fmt.Errorf("fixture %s build prompt: %w", fixture.ID, err)
	}
	tools, err := runner.BuildTools([]string{JudgeRule56Tool})
	if err != nil {
		return JudgeRule56Result{}, err
	}
	var resp openai.Response
	if opts.DryRun {
		resp = dryRunJudgeRule56Response(fixture)
	} else {
		callCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		resp, err = client.CreateResponse(callCtx, modelRef.Model, input, tools, "", opts.Temperature)
		cancel()
		if err != nil {
			return JudgeRule56Result{}, fmt.Errorf("fixture %s model call: %w", fixture.ID, err)
		}
	}
	result := scoreJudgeRule56Response(fixture, opts.Model, opts.DryRun, state, view, opportunity, input, resp)
	result.PromptSource = promptVariant.Source
	result.PromptName = promptVariant.Name
	result.PromptPath = promptVariant.Path
	if result.InvalidReason == "" {
		decision := map[string]any{
			"kind":      "tool",
			"tool_name": JudgeRule56Tool,
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

func LoadJudgeRule56Fixtures(path string) ([]JudgeRule56Fixture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeRule56Fixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeRule56Fixture
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

func loadJudgeRule56PromptVariant(path string, name string, outputDir string) (judgeRule56PromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeRule56PromptVariant{
			Source: "production",
			Name:   name,
		}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeRule56PromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeRule56PromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	copyPath := filepath.Join(outputDir, "opportunity_prompt.md")
	if err := os.WriteFile(copyPath, raw, 0o644); err != nil {
		return judgeRule56PromptVariant{}, fmt.Errorf("copy opportunity prompt to %s: %w", copyPath, err)
	}
	return judgeRule56PromptVariant{
		Source:   "file:" + path,
		Name:     name,
		Path:     path,
		CopyPath: copyPath,
		Text:     text,
	}, nil
}

func (f JudgeRule56Fixture) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("fixture missing id")
	}
	if f.Tier < 1 {
		return fmt.Errorf("fixture %s tier must be positive", f.ID)
	}
	if strings.TrimSpace(f.IssueFamily) == "" {
		return fmt.Errorf("fixture %s missing issue_family", f.ID)
	}
	if normalizeParty(f.MovingParty) == "" {
		return fmt.Errorf("fixture %s moving_party must be plaintiff or defendant", f.ID)
	}
	if strings.TrimSpace(f.RequestText) == "" {
		return fmt.Errorf("fixture %s missing request_text", f.ID)
	}
	if strings.TrimSpace(f.StatementOfFacts) == "" {
		return fmt.Errorf("fixture %s missing statement_of_undisputed_facts", f.ID)
	}
	if strings.TrimSpace(f.OppositionText) == "" {
		return fmt.Errorf("fixture %s missing opposition_text", f.ID)
	}
	if !validRule56Disposition(f.ExpectedDisposition) {
		return fmt.Errorf("fixture %s invalid expected_disposition %q", f.ID, f.ExpectedDisposition)
	}
	if len(f.ExpectedReasonTags) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeRule56State(f JudgeRule56Fixture) map[string]any {
	moving := normalizeParty(f.MovingParty)
	opposing := opposingParty(moving)
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           "Judge Eval Court",
		"court_profile":        nil,
		"policy":               defaultJudgeEvalPolicy(),
		"state_version":        0,
		"passed_opportunities": []any{},
		"case": map[string]any{
			"case_id":                       "judge-rule56-" + strings.TrimSpace(f.ID),
			"caption":                       strings.TrimSpace(f.CaseTheme),
			"judge":                         "Judge Eval",
			"filed_on":                      "2026-07-14",
			"auto_rule11":                   false,
			"status":                        "pretrial",
			"trial_mode":                    "jury",
			"phase":                         "pretrial",
			"last_pleading_served_on":       "2026-07-01",
			"jury_demanded_on":              "2026-07-01",
			"jury_configuration":            map[string]any{"juror_count": 6, "unanimous_required": true, "minimum_concurring": 6},
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
			"docket":                        judgeRule56Docket(f, moving, opposing),
			"decision_traces":               []any{},
		},
	}
}

func judgeRule56Docket(f JudgeRule56Fixture, moving string, opposing string) []any {
	entries := []any{
		map[string]any{
			"title":       "Complaint",
			"description": "plaintiff: " + strings.TrimSpace(f.CaseTheme),
		},
		map[string]any{
			"title":       "Answer",
			"description": "defendant: denies liability and demands proof of every element.",
		},
		map[string]any{
			"title":       "Rule 56 Motion",
			"description": moving + ": " + joinNonEmpty([]string{fieldSentence("scope", f.MotionScope), strings.TrimSpace(f.RequestText), fieldSentence("statement", f.StatementOfFacts), fieldSentence("evidence", strings.Join(f.EvidenceRefs, ", "))}),
		},
		map[string]any{
			"title":       "Rule 56 Opposition",
			"description": opposing + ": " + strings.TrimSpace(f.OppositionText),
		},
	}
	if strings.TrimSpace(f.ReplyText) != "" {
		entries = append(entries, map[string]any{
			"title":       "Rule 56 Reply",
			"description": moving + ": " + strings.TrimSpace(f.ReplyText),
		})
	}
	return entries
}

func fieldSentence(label string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return label + ": " + value
}

func joinNonEmpty(values []string) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, " ")
}

func opposingParty(party string) string {
	switch normalizeParty(party) {
	case "plaintiff":
		return "defendant"
	case "defendant":
		return "plaintiff"
	default:
		return ""
	}
}

func buildJudgeRule56Input(
	view map[string]any,
	opportunity map[string]any,
	fixture JudgeRule56Fixture,
	promptVariant judgeRule56PromptVariant,
) ([]map[string]any, error) {
	role := judgeRule56Role()
	systemPrompt, err := buildJudgeRule56SystemPrompt(role, view)
	if err != nil {
		return nil, err
	}
	userPrompt, err := buildJudgeRule56OpportunityPrompt(opportunity, fixture, promptVariant)
	if err != nil {
		return nil, err
	}
	return []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}, nil
}

func scoreJudgeRule56Response(
	fixture JudgeRule56Fixture,
	model string,
	dryRun bool,
	state map[string]any,
	view map[string]any,
	opportunity map[string]any,
	input []map[string]any,
	resp openai.Response,
) JudgeRule56Result {
	result := JudgeRule56Result{
		ID:                  fixture.ID,
		Tier:                fixture.Tier,
		IssueFamily:         fixture.IssueFamily,
		CaseTheme:           fixture.CaseTheme,
		MovingParty:         normalizeParty(fixture.MovingParty),
		MotionScope:         strings.TrimSpace(fixture.MotionScope),
		RequestText:         strings.TrimSpace(fixture.RequestText),
		StatementOfFacts:    strings.TrimSpace(fixture.StatementOfFacts),
		EvidenceRefs:        append([]string{}, fixture.EvidenceRefs...),
		OppositionText:      strings.TrimSpace(fixture.OppositionText),
		ReplyText:           strings.TrimSpace(fixture.ReplyText),
		ExpectedDisposition: normalizeRule56Disposition(fixture.ExpectedDisposition),
		ExpectedSurviving:   append([]string{}, fixture.ExpectedSurviving...),
		ExpectedReasonTags:  append([]string{}, fixture.ExpectedReasonTags...),
		Severity:            normalizedSeverity(fixture.Severity),
		ContextNotes:        strings.TrimSpace(fixture.ContextNotes),
		Model:               model,
		DryRun:              dryRun,
		State:               state,
		View:                view,
		Opportunity:         opportunity,
		Input:               input,
		RawResponse:         responseJSON(resp),
	}
	payload, invalid := extractJudgeRule56Payload(resp)
	if invalid != "" {
		result.InvalidReason = invalid
		return result
	}
	result.ToolPayload = payload
	if intField(payload, "motion_index") != 0 {
		result.InvalidReason = "wrong_motion_index"
		return result
	}
	disposition := normalizeRule56Disposition(stringField(payload, "disposition"))
	if !validRule56Disposition(disposition) {
		result.InvalidReason = "invalid_disposition"
		return result
	}
	result.Disposition = disposition
	result.SurvivingIssues = stringsFromAnySlice(payload["surviving_issues"])
	result.Reasoning = strings.TrimSpace(stringField(payload, "reasoning"))
	if result.Reasoning == "" {
		result.InvalidReason = "empty_reasoning"
		return result
	}
	result.OutcomeCorrect = disposition == result.ExpectedDisposition
	result.MatchedReasonTags = matchedRule56ReasonTags(result.Reasoning, fixture.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
	return result
}

func extractJudgeRule56Payload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeRule56Tool {
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

func dryRunJudgeRule56Response(f JudgeRule56Fixture) openai.Response {
	return openai.Response{
		Text:       "",
		ResponseID: "dry-run-" + strings.TrimSpace(f.ID),
		ToolCalls: []openai.ToolCall{{
			CallID: "dry-run-call-" + strings.TrimSpace(f.ID),
			Name:   JudgeRule56Tool,
			Arguments: map[string]any{
				"motion_index":     0,
				"disposition":      normalizeRule56Disposition(f.ExpectedDisposition),
				"surviving_issues": append([]string{}, f.ExpectedSurviving...),
				"reasoning":        "gold tags: " + strings.Join(f.ExpectedReasonTags, ", "),
			},
		}},
	}
}

func buildJudgeRule56SystemPrompt(role spec.RoleSpec, view map[string]any) (string, error) {
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

func buildJudgeRule56OpportunityPrompt(
	opportunity map[string]any,
	fixture JudgeRule56Fixture,
	promptVariant judgeRule56PromptVariant,
) (string, error) {
	tools, err := runner.BuildTools([]string{JudgeRule56Tool})
	if err != nil {
		return "", err
	}
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		objective = renderJudgeRule56PromptTemplate(promptVariant.Text, fixture, opportunity)
	}
	lines := []string{
		"Current opportunity:",
		stringField(opportunity, "actor_message"),
		formatRule56Objective(objective),
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

func formatRule56Objective(objective string) string {
	objective = strings.TrimSpace(objective)
	if strings.Contains(objective, "\n") {
		return "Objective:\n" + objective
	}
	return "Objective: " + objective
}

func renderJudgeRule56PromptTemplate(template string, fixture JudgeRule56Fixture, opportunity map[string]any) string {
	replacer := strings.NewReplacer(
		"{{production_objective}}", stringField(opportunity, "objective"),
		"{{actor_message}}", stringField(opportunity, "actor_message"),
		"{{phase}}", stringField(opportunity, "phase"),
		"{{allowed_tools}}", strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
		"{{fixture_id}}", strings.TrimSpace(fixture.ID),
		"{{tier}}", strconv.Itoa(fixture.Tier),
		"{{issue_family}}", strings.TrimSpace(fixture.IssueFamily),
		"{{case_theme}}", strings.TrimSpace(fixture.CaseTheme),
		"{{moving_party}}", normalizeParty(fixture.MovingParty),
		"{{opposing_party}}", opposingParty(fixture.MovingParty),
		"{{motion_scope}}", strings.TrimSpace(fixture.MotionScope),
		"{{request_text}}", strings.TrimSpace(fixture.RequestText),
		"{{statement_of_undisputed_facts}}", strings.TrimSpace(fixture.StatementOfFacts),
		"{{evidence_refs}}", strings.Join(fixture.EvidenceRefs, ", "),
		"{{opposition_text}}", strings.TrimSpace(fixture.OppositionText),
		"{{reply_text}}", strings.TrimSpace(fixture.ReplyText),
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
	return strings.TrimSpace(replacer.Replace(template))
}

func judgeRule56Role() spec.RoleSpec {
	return spec.RoleSpec{
		Name:           "judge",
		Instructions:   "Judge for procedural rulings, trial control, and judgment entry.",
		PromptPreamble: casegen.JudgeRuntimeBrief(),
		AllowedTools:   []string{JudgeRule56Tool},
	}
}

func judgeRule56Roles() []map[string]any {
	return []map[string]any{{
		"role":          "judge",
		"allowed_tools": []string{JudgeRule56Tool},
	}}
}

func applyRule56SummaryResult(summary *JudgeRule56Summary, result JudgeRule56Result, weight float64) {
	summary.Total++
	if result.InvalidReason != "" {
		summary.Invalid++
	} else if result.OutcomeCorrect {
		summary.Correct++
	} else {
		classifyRule56Error(result, &summary.FalseGrants, &summary.FalseDenials, &summary.PartialMismatches)
	}
	if result.ReasonCorrect {
		summary.ReasonCorrect++
	}
	for _, tag := range result.ExpectedReasonTags {
		updateRule56Slice(summary.ByReasonTag, tag, result, weight)
	}
	updateRule56Slice(summary.ByIssueFamily, result.IssueFamily, result, weight)
	updateRule56Slice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
	updateRule56Slice(summary.ByMovingParty, result.MovingParty, result, weight)
}

func updateRule56Slice(m map[string]JudgeRule56Slice, key string, result JudgeRule56Result, weight float64) {
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
		classifyRule56Error(result, &s.FalseGrants, &s.FalseDenials, &s.PartialMismatches)
	}
	m[key] = s
}

func classifyRule56Error(result JudgeRule56Result, falseGrants *int, falseDenials *int, partialMismatches *int) {
	got := normalizeRule56Disposition(result.Disposition)
	want := normalizeRule56Disposition(result.ExpectedDisposition)
	if (got == "granted" || got == "partial") && want == "denied" {
		(*falseGrants)++
		return
	}
	if got == "granted" && want == "partial" {
		(*falseGrants)++
		return
	}
	if got == "denied" && (want == "granted" || want == "partial") {
		(*falseDenials)++
		return
	}
	(*partialMismatches)++
}

func finalizeRule56Summary(summary *JudgeRule56Summary, totalWeight float64, correctWeight float64) {
	if summary.Total > 0 {
		summary.Accuracy = float64(summary.Correct) / float64(summary.Total)
		summary.FalseGrantRate = float64(summary.FalseGrants) / float64(summary.Total)
		summary.FalseDenialRate = float64(summary.FalseDenials) / float64(summary.Total)
		summary.InvalidRate = float64(summary.Invalid) / float64(summary.Total)
	}
	if totalWeight > 0 {
		summary.WeightedAccuracy = correctWeight / totalWeight
	}
	finalizeRule56Slices(summary.ByReasonTag)
	finalizeRule56Slices(summary.ByIssueFamily)
	finalizeRule56Slices(summary.ByTier)
	finalizeRule56Slices(summary.ByMovingParty)
}

func finalizeRule56Slices(m map[string]JudgeRule56Slice) {
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

func matchedRule56ReasonTags(reason string, expected []string) []string {
	reason = normalizeReasonText(reason)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if rule56ReasonMatchesTag(reason, tag) {
			matches = append(matches, tag)
		}
	}
	sort.Strings(matches)
	return matches
}

func rule56ReasonMatchesTag(reason string, tag string) bool {
	normalizedTag := normalizeReasonText(tag)
	if strings.Contains(reason, normalizedTag) {
		return true
	}
	for _, keyword := range rule56ReasonTagKeywords()[tag] {
		if strings.Contains(reason, keyword) {
			return true
		}
	}
	return false
}

func rule56ReasonTagKeywords() map[string][]string {
	return map[string][]string{
		"no_genuine_dispute":     {"no genuine dispute", "undisputed", "entitled to judgment", "matter of law"},
		"missing_element":        {"missing element", "essential element", "no evidence", "failed to show", "cannot prove", "causation", "breach"},
		"credibility_dispute":    {"credibility", "credible", "weigh", "testimony", "believe", "jury could credit"},
		"competing_inference":    {"competing inference", "inference", "reasonable jury", "factfinder", "view the evidence"},
		"unsupported_damages":    {"damages", "damage amount", "unsupported amount", "proof of damages", "lost profit"},
		"authentication_dispute": {"authentication", "authenticity", "authenticated", "foundation", "genuine dispute over authenticity"},
		"movant_burden_not_met":  {"movant", "burden", "failed to carry", "not met", "failed to show"},
		"legal_bar":              {"statute", "limitations", "release", "barred", "waiver", "time barred", "time-barred"},
	}
}

func validRule56Disposition(s string) bool {
	switch normalizeRule56Disposition(s) {
	case "granted", "denied", "partial":
		return true
	default:
		return false
	}
}

func normalizeRule56Disposition(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func stringsFromAnySlice(value any) []string {
	switch typed := value.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if strings.TrimSpace(item) != "" {
				out = append(out, strings.TrimSpace(item))
			}
		}
		return out
	default:
		return nil
	}
}
