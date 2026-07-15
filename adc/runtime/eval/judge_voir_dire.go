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

const JudgeVoirDireTool = "decide_voir_dire_question"

type JudgeVoirDireFixture struct {
	ID                 string   `json:"id"`
	Tier               int      `json:"tier"`
	QuestionFamily     string   `json:"question_family"`
	CaseTheme          string   `json:"case_theme"`
	AskedBy            string   `json:"asked_by"`
	JurorID            string   `json:"juror_id"`
	Question           string   `json:"question"`
	ExpectedAllowed    bool     `json:"expected_allowed"`
	ExpectedReasonTags []string `json:"expected_reason_tags"`
	Severity           float64  `json:"severity"`
	ContextNotes       string   `json:"context_notes,omitempty"`
}

type JudgeVoirDireOptions struct {
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

type JudgeVoirDireRescoreOptions struct {
	ResultsPath string
	OutputDir   string
}

type JudgeVoirDireSummary struct {
	Evaluation        string                        `json:"evaluation"`
	Model             string                        `json:"model"`
	DryRun            bool                          `json:"dry_run"`
	PromptSource      string                        `json:"prompt_source"`
	PromptName        string                        `json:"prompt_name"`
	PromptPath        string                        `json:"prompt_path,omitempty"`
	PromptCopyPath    string                        `json:"prompt_copy_path,omitempty"`
	FixturesPath      string                        `json:"fixtures_path"`
	OutputDir         string                        `json:"output_dir"`
	ResultsPath       string                        `json:"results_path"`
	SummaryPath       string                        `json:"summary_path"`
	Total             int                           `json:"total"`
	Correct           int                           `json:"correct"`
	ReasonCorrect     int                           `json:"reason_correct"`
	Invalid           int                           `json:"invalid"`
	FalseAllows       int                           `json:"false_allows"`
	FalseDisallows    int                           `json:"false_disallows"`
	Accuracy          float64                       `json:"accuracy"`
	WeightedAccuracy  float64                       `json:"weighted_accuracy"`
	FalseAllowRate    float64                       `json:"false_allow_rate"`
	FalseDisallowRate float64                       `json:"false_disallow_rate"`
	InvalidRate       float64                       `json:"invalid_rate"`
	ByReasonTag       map[string]JudgeVoirDireSlice `json:"by_reason_tag"`
	ByQuestionFamily  map[string]JudgeVoirDireSlice `json:"by_question_family"`
	ByTier            map[string]JudgeVoirDireSlice `json:"by_tier"`
	ByAskedBy         map[string]JudgeVoirDireSlice `json:"by_asked_by"`
	GeneratedAt       string                        `json:"generated_at"`
}

type JudgeVoirDireSlice struct {
	Total            int     `json:"total"`
	Correct          int     `json:"correct"`
	FalseAllows      int     `json:"false_allows"`
	FalseDisallows   int     `json:"false_disallows"`
	Invalid          int     `json:"invalid"`
	Weight           float64 `json:"weight"`
	CorrectWeight    float64 `json:"correct_weight"`
	Accuracy         float64 `json:"accuracy"`
	WeightedAccuracy float64 `json:"weighted_accuracy"`
}

type JudgeVoirDireResult struct {
	ID                 string           `json:"id"`
	Tier               int              `json:"tier"`
	QuestionFamily     string           `json:"question_family"`
	CaseTheme          string           `json:"case_theme"`
	AskedBy            string           `json:"asked_by"`
	JurorID            string           `json:"juror_id"`
	Question           string           `json:"question"`
	ExpectedAllowed    bool             `json:"expected_allowed"`
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
	Allowed            *bool            `json:"allowed,omitempty"`
	RulingReason       string           `json:"ruling_reason,omitempty"`
	MatchedReasonTags  []string         `json:"matched_reason_tags"`
	OutcomeCorrect     bool             `json:"outcome_correct"`
	ReasonCorrect      bool             `json:"reason_correct"`
	InvalidReason      string           `json:"invalid_reason,omitempty"`
	LeanAccepted       bool             `json:"lean_accepted"`
	LeanError          string           `json:"lean_error,omitempty"`
}

type judgeVoirDirePromptVariant struct {
	Source   string
	Name     string
	Path     string
	CopyPath string
	Text     string
}

func RunJudgeVoirDire(ctx context.Context, opts JudgeVoirDireOptions) (JudgeVoirDireSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.FixturesPath) == "" {
		return JudgeVoirDireSummary{}, fmt.Errorf("fixtures path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeVoirDireSummary{}, fmt.Errorf("output directory is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	fixtures, err := LoadJudgeVoirDireFixtures(opts.FixturesPath)
	if err != nil {
		return JudgeVoirDireSummary{}, err
	}
	if opts.Limit > 0 && opts.Limit < len(fixtures) {
		fixtures = fixtures[:opts.Limit]
	}
	if len(fixtures) == 0 {
		return JudgeVoirDireSummary{}, fmt.Errorf("no fixtures loaded from %s", opts.FixturesPath)
	}
	if len(opts.Engine.Command) == 0 {
		opts.Engine = lean.New(nil)
	}
	modelRef := modelrequest.ModelRef{}
	var client *openai.Client
	if !opts.DryRun {
		modelRef, err = modelrequest.ParseModelRef(opts.Model)
		if err != nil {
			return JudgeVoirDireSummary{}, fmt.Errorf("parse --model: %w", err)
		}
		client, err = openai.NewForEndpoint(modelRef.Endpoint, opts.Online, opts.Timeout)
		if err != nil {
			return JudgeVoirDireSummary{}, err
		}
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeVoirDireSummary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	promptVariant, err := loadJudgeVoirDirePromptVariant(opts.OpportunityPromptPath, opts.OpportunityPromptName, opts.OutputDir)
	if err != nil {
		return JudgeVoirDireSummary{}, err
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeVoirDireSummary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer resultsFile.Close()

	summary := JudgeVoirDireSummary{
		Evaluation:       "judge_voir_dire",
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
		ByReasonTag:      map[string]JudgeVoirDireSlice{},
		ByQuestionFamily: map[string]JudgeVoirDireSlice{},
		ByTier:           map[string]JudgeVoirDireSlice{},
		ByAskedBy:        map[string]JudgeVoirDireSlice{},
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, fixture := range fixtures {
		result, err := runJudgeVoirDireFixture(ctx, opts, promptVariant, modelRef, client, fixture)
		if err != nil {
			return JudgeVoirDireSummary{}, err
		}
		if err := encoder.Encode(result); err != nil {
			return JudgeVoirDireSummary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applySummaryResult(&summary, result, weight)
	}
	finalizeSummary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeVoirDireSummary{}, err
	}
	return summary, nil
}

func RescoreJudgeVoirDire(opts JudgeVoirDireRescoreOptions) (JudgeVoirDireSummary, error) {
	if strings.TrimSpace(opts.ResultsPath) == "" {
		return JudgeVoirDireSummary{}, fmt.Errorf("results path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return JudgeVoirDireSummary{}, fmt.Errorf("output directory is required")
	}
	results, err := readJudgeVoirDireResults(opts.ResultsPath)
	if err != nil {
		return JudgeVoirDireSummary{}, err
	}
	if len(results) == 0 {
		return JudgeVoirDireSummary{}, fmt.Errorf("no results loaded from %s", opts.ResultsPath)
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return JudgeVoirDireSummary{}, fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
	}
	resultsPath := filepath.Join(opts.OutputDir, "results.jsonl")
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return JudgeVoirDireSummary{}, fmt.Errorf("create %s: %w", resultsPath, err)
	}
	defer resultsFile.Close()
	summary := JudgeVoirDireSummary{
		Evaluation:       "judge_voir_dire",
		Model:            results[0].Model,
		DryRun:           results[0].DryRun,
		PromptSource:     resultPromptSource(results[0]),
		PromptName:       resultPromptName(results[0]),
		PromptPath:       results[0].PromptPath,
		FixturesPath:     "rescored from " + opts.ResultsPath,
		OutputDir:        opts.OutputDir,
		ResultsPath:      resultsPath,
		SummaryPath:      summaryPath,
		ByReasonTag:      map[string]JudgeVoirDireSlice{},
		ByQuestionFamily: map[string]JudgeVoirDireSlice{},
		ByTier:           map[string]JudgeVoirDireSlice{},
		ByAskedBy:        map[string]JudgeVoirDireSlice{},
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	var totalWeight float64
	var correctWeight float64
	encoder := json.NewEncoder(resultsFile)
	for _, result := range results {
		rescoreJudgeVoirDireResult(&result)
		if err := encoder.Encode(result); err != nil {
			return JudgeVoirDireSummary{}, fmt.Errorf("write %s: %w", resultsPath, err)
		}
		weight := normalizedSeverity(result.Severity)
		totalWeight += weight
		if result.OutcomeCorrect && result.InvalidReason == "" {
			correctWeight += weight
		}
		applySummaryResult(&summary, result, weight)
	}
	finalizeSummary(&summary, totalWeight, correctWeight)
	if err := writeJSON(summaryPath, summary); err != nil {
		return JudgeVoirDireSummary{}, err
	}
	return summary, nil
}

func runJudgeVoirDireFixture(
	ctx context.Context,
	opts JudgeVoirDireOptions,
	promptVariant judgeVoirDirePromptVariant,
	modelRef modelrequest.ModelRef,
	client *openai.Client,
	fixture JudgeVoirDireFixture,
) (JudgeVoirDireResult, error) {
	if err := fixture.Validate(); err != nil {
		return JudgeVoirDireResult{}, err
	}
	state := BuildJudgeVoirDireState(fixture)
	roles := judgeVoirDireRoles()
	viewResp, err := opts.Engine.View(state, "judge")
	if err != nil {
		return JudgeVoirDireResult{}, fmt.Errorf("fixture %s view: %w", fixture.ID, err)
	}
	if ok, _ := viewResp["ok"].(bool); !ok {
		return JudgeVoirDireResult{}, fmt.Errorf("fixture %s view rejected: %s", fixture.ID, stringField(viewResp, "error"))
	}
	view, _ := viewResp["view"].(map[string]any)
	opportunityResp, err := opts.Engine.NextOpportunity(state, roles, 3)
	if err != nil {
		return JudgeVoirDireResult{}, fmt.Errorf("fixture %s next opportunity: %w", fixture.ID, err)
	}
	if ok, _ := opportunityResp["ok"].(bool); !ok {
		return JudgeVoirDireResult{}, fmt.Errorf("fixture %s next opportunity rejected: %s", fixture.ID, stringField(opportunityResp, "error"))
	}
	opportunity, _ := opportunityResp["opportunity"].(map[string]any)
	if len(opportunity) == 0 {
		return JudgeVoirDireResult{}, fmt.Errorf("fixture %s returned no opportunity", fixture.ID)
	}
	if stringField(opportunity, "role") != "judge" {
		return JudgeVoirDireResult{}, fmt.Errorf("fixture %s opportunity role = %q, want judge", fixture.ID, stringField(opportunity, "role"))
	}
	if !stringSliceContains(stringSliceField(opportunity, "allowed_tools"), JudgeVoirDireTool) {
		return JudgeVoirDireResult{}, fmt.Errorf("fixture %s opportunity lacks %s", fixture.ID, JudgeVoirDireTool)
	}
	input, err := buildJudgeVoirDireInput(view, opportunity, fixture, promptVariant)
	if err != nil {
		return JudgeVoirDireResult{}, fmt.Errorf("fixture %s build prompt: %w", fixture.ID, err)
	}
	tools, err := runner.BuildTools([]string{JudgeVoirDireTool})
	if err != nil {
		return JudgeVoirDireResult{}, err
	}
	var resp openai.Response
	if opts.DryRun {
		resp = dryRunJudgeVoirDireResponse(fixture)
	} else {
		callCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		resp, err = client.CreateResponse(callCtx, modelRef.Model, input, tools, "", opts.Temperature)
		cancel()
		if err != nil {
			return JudgeVoirDireResult{}, fmt.Errorf("fixture %s model call: %w", fixture.ID, err)
		}
	}
	result := scoreJudgeVoirDireResponse(fixture, opts.Model, opts.DryRun, state, view, opportunity, input, resp)
	result.PromptSource = promptVariant.Source
	result.PromptName = promptVariant.Name
	result.PromptPath = promptVariant.Path
	if result.InvalidReason == "" {
		decision := map[string]any{
			"kind":      "tool",
			"tool_name": JudgeVoirDireTool,
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

func LoadJudgeVoirDireFixtures(path string) ([]JudgeVoirDireFixture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open fixtures %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := make([]JudgeVoirDireFixture, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fixture JudgeVoirDireFixture
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

func readJudgeVoirDireResults(path string) ([]JudgeVoirDireResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open results %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 32*1024*1024)
	out := make([]JudgeVoirDireResult, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var result JudgeVoirDireResult
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

func loadJudgeVoirDirePromptVariant(path string, name string, outputDir string) (judgeVoirDirePromptVariant, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if path == "" {
		if name == "" {
			name = "production"
		}
		return judgeVoirDirePromptVariant{
			Source: "production",
			Name:   name,
		}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return judgeVoirDirePromptVariant{}, fmt.Errorf("read opportunity prompt file %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return judgeVoirDirePromptVariant{}, fmt.Errorf("opportunity prompt file %s is empty", path)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if name == "" || name == "." {
		name = "file"
	}
	copyPath := filepath.Join(outputDir, "opportunity_prompt.md")
	if err := os.WriteFile(copyPath, raw, 0o644); err != nil {
		return judgeVoirDirePromptVariant{}, fmt.Errorf("copy opportunity prompt to %s: %w", copyPath, err)
	}
	return judgeVoirDirePromptVariant{
		Source:   "file:" + path,
		Name:     name,
		Path:     path,
		CopyPath: copyPath,
		Text:     text,
	}, nil
}

func (f JudgeVoirDireFixture) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("fixture missing id")
	}
	if f.Tier < 1 {
		return fmt.Errorf("fixture %s tier must be positive", f.ID)
	}
	if strings.TrimSpace(f.QuestionFamily) == "" {
		return fmt.Errorf("fixture %s missing question_family", f.ID)
	}
	if normalizeParty(f.AskedBy) == "" {
		return fmt.Errorf("fixture %s asked_by must be plaintiff or defendant", f.ID)
	}
	if strings.TrimSpace(f.JurorID) == "" {
		return fmt.Errorf("fixture %s missing juror_id", f.ID)
	}
	if strings.TrimSpace(f.Question) == "" {
		return fmt.Errorf("fixture %s missing question", f.ID)
	}
	if len(f.ExpectedReasonTags) == 0 {
		return fmt.Errorf("fixture %s missing expected_reason_tags", f.ID)
	}
	return nil
}

func BuildJudgeVoirDireState(f JudgeVoirDireFixture) map[string]any {
	askedBy := normalizeParty(f.AskedBy)
	jurorID := strings.TrimSpace(f.JurorID)
	return map[string]any{
		"schema_version":       "v1",
		"court_name":           "Judge Eval Court",
		"court_profile":        nil,
		"policy":               defaultJudgeVoirDirePolicy(),
		"state_version":        0,
		"passed_opportunities": []any{},
		"case": map[string]any{
			"case_id":                       "judge-voir-dire-" + strings.TrimSpace(f.ID),
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
			"juror_questionnaire_responses": []any{map[string]any{"juror_id": jurorID, "submitted_at": "2026-07-14", "answers": []any{map[string]any{"question_id": "q1", "answer": "Yes."}}}},
			"voir_dire_exchanges": []any{map[string]any{
				"exchange_id":   "vx-1",
				"juror_id":      jurorID,
				"asked_by":      askedBy,
				"question":      strings.TrimSpace(f.Question),
				"judge_allowed": nil,
				"ruling_reason": "",
				"response":      "",
				"asked_at":      "2026-07-14",
				"ruled_at":      nil,
				"answered_at":   nil,
			}},
			"for_cause_challenges":     []any{},
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
			"docket":                   []any{},
			"decision_traces":          []any{},
		},
	}
}

func BuildJudgeVoirDireInput(view map[string]any, opportunity map[string]any) ([]map[string]any, error) {
	return buildJudgeVoirDireInput(view, opportunity, JudgeVoirDireFixture{}, judgeVoirDirePromptVariant{
		Source: "production",
		Name:   "production",
	})
}

func buildJudgeVoirDireInput(
	view map[string]any,
	opportunity map[string]any,
	fixture JudgeVoirDireFixture,
	promptVariant judgeVoirDirePromptVariant,
) ([]map[string]any, error) {
	role := judgeVoirDireRole()
	systemPrompt, err := buildJudgeVoirDireSystemPrompt(role, view)
	if err != nil {
		return nil, err
	}
	userPrompt, err := buildJudgeVoirDireOpportunityPrompt(opportunity, fixture, promptVariant)
	if err != nil {
		return nil, err
	}
	return []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}, nil
}

func scoreJudgeVoirDireResponse(
	fixture JudgeVoirDireFixture,
	model string,
	dryRun bool,
	state map[string]any,
	view map[string]any,
	opportunity map[string]any,
	input []map[string]any,
	resp openai.Response,
) JudgeVoirDireResult {
	result := JudgeVoirDireResult{
		ID:                 fixture.ID,
		Tier:               fixture.Tier,
		QuestionFamily:     fixture.QuestionFamily,
		CaseTheme:          fixture.CaseTheme,
		AskedBy:            normalizeParty(fixture.AskedBy),
		JurorID:            strings.TrimSpace(fixture.JurorID),
		Question:           strings.TrimSpace(fixture.Question),
		ExpectedAllowed:    fixture.ExpectedAllowed,
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
	payload, invalid := extractJudgeVoirDirePayload(resp)
	if invalid != "" {
		result.InvalidReason = invalid
		return result
	}
	result.ToolPayload = payload
	if got := strings.TrimSpace(stringField(payload, "exchange_id")); got != "vx-1" {
		result.InvalidReason = "wrong_exchange_id"
		return result
	}
	if got := strings.TrimSpace(stringField(payload, "juror_id")); got != strings.TrimSpace(fixture.JurorID) {
		result.InvalidReason = "wrong_juror_id"
		return result
	}
	allowed, ok := payload["allowed"].(bool)
	if !ok {
		result.InvalidReason = "malformed_allowed"
		return result
	}
	result.Allowed = &allowed
	result.RulingReason = strings.TrimSpace(stringField(payload, "ruling_reason"))
	if result.RulingReason == "" {
		result.InvalidReason = "empty_ruling_reason"
		return result
	}
	result.OutcomeCorrect = allowed == fixture.ExpectedAllowed
	result.MatchedReasonTags = matchedReasonTags(result.RulingReason, fixture.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
	return result
}

func rescoreJudgeVoirDireResult(result *JudgeVoirDireResult) {
	if result == nil || result.InvalidReason != "" || result.Allowed == nil {
		return
	}
	result.OutcomeCorrect = *result.Allowed == result.ExpectedAllowed
	result.MatchedReasonTags = matchedReasonTags(result.RulingReason, result.ExpectedReasonTags)
	result.ReasonCorrect = len(result.MatchedReasonTags) > 0
}

func extractJudgeVoirDirePayload(resp openai.Response) (map[string]any, string) {
	if len(resp.ToolCalls) == 0 {
		return nil, "missing_tool_call"
	}
	if len(resp.ToolCalls) != 1 {
		return nil, "multiple_tool_calls"
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != JudgeVoirDireTool {
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

func dryRunJudgeVoirDireResponse(f JudgeVoirDireFixture) openai.Response {
	return openai.Response{
		Text:       "",
		ResponseID: "dry-run-" + strings.TrimSpace(f.ID),
		ToolCalls: []openai.ToolCall{{
			CallID: "dry-run-call-" + strings.TrimSpace(f.ID),
			Name:   JudgeVoirDireTool,
			Arguments: map[string]any{
				"exchange_id":   "vx-1",
				"juror_id":      strings.TrimSpace(f.JurorID),
				"allowed":       f.ExpectedAllowed,
				"ruling_reason": "gold tags: " + strings.Join(f.ExpectedReasonTags, ", "),
			},
		}},
	}
}

func buildJudgeVoirDireSystemPrompt(role spec.RoleSpec, view map[string]any) (string, error) {
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

func buildJudgeVoirDireOpportunityPrompt(
	opportunity map[string]any,
	fixture JudgeVoirDireFixture,
	promptVariant judgeVoirDirePromptVariant,
) (string, error) {
	tools, err := runner.BuildTools([]string{JudgeVoirDireTool})
	if err != nil {
		return "", err
	}
	objective := stringField(opportunity, "objective")
	if strings.TrimSpace(promptVariant.Text) != "" {
		objective = renderJudgeVoirDirePromptTemplate(promptVariant.Text, fixture, opportunity)
	}
	lines := []string{
		"Current opportunity:",
		stringField(opportunity, "actor_message"),
		formatJudgeVoirDireObjective(objective),
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

func formatJudgeVoirDireObjective(objective string) string {
	objective = strings.TrimSpace(objective)
	if strings.Contains(objective, "\n") {
		return "Objective:\n" + objective
	}
	return "Objective: " + objective
}

func renderJudgeVoirDirePromptTemplate(template string, fixture JudgeVoirDireFixture, opportunity map[string]any) string {
	askedBy := normalizeParty(fixture.AskedBy)
	if askedBy == "" {
		askedBy = stringField(opportunityRequiredPayload(opportunity), "asked_by")
	}
	jurorID := strings.TrimSpace(fixture.JurorID)
	if jurorID == "" {
		jurorID = stringField(opportunityRequiredPayload(opportunity), "juror_id")
	}
	exchangeID := stringField(opportunityRequiredPayload(opportunity), "exchange_id")
	replacer := strings.NewReplacer(
		"{{production_objective}}", stringField(opportunity, "objective"),
		"{{actor_message}}", stringField(opportunity, "actor_message"),
		"{{phase}}", stringField(opportunity, "phase"),
		"{{allowed_tools}}", strings.Join(stringSliceField(opportunity, "allowed_tools"), ", "),
		"{{fixture_id}}", strings.TrimSpace(fixture.ID),
		"{{tier}}", strconv.Itoa(fixture.Tier),
		"{{question_family}}", strings.TrimSpace(fixture.QuestionFamily),
		"{{case_theme}}", strings.TrimSpace(fixture.CaseTheme),
		"{{asked_by}}", askedBy,
		"{{juror_id}}", jurorID,
		"{{question}}", strings.TrimSpace(fixture.Question),
		"{{exchange_id}}", exchangeID,
		"{{context_notes}}", strings.TrimSpace(fixture.ContextNotes),
	)
	return strings.TrimSpace(replacer.Replace(template))
}

func opportunityRequiredPayload(opportunity map[string]any) map[string]any {
	constraints, _ := opportunity["constraints"].(map[string]any)
	required, _ := constraints["required_payload"].(map[string]any)
	if required == nil {
		return map[string]any{}
	}
	return required
}

func judgeVoirDireRole() spec.RoleSpec {
	return spec.RoleSpec{
		Name:           "judge",
		Instructions:   "Judge for procedural rulings, trial control, and judgment entry.",
		PromptPreamble: casegen.JudgeRuntimeBrief(),
		AllowedTools:   []string{JudgeVoirDireTool},
	}
}

func judgeVoirDireRoles() []map[string]any {
	return []map[string]any{{
		"role":          "judge",
		"allowed_tools": []string{JudgeVoirDireTool},
	}}
}

func defaultJudgeVoirDirePolicy() map[string]any {
	return defaultJudgeEvalPolicy()
}

func defaultJudgeEvalPolicy() map[string]any {
	return map[string]any{
		"max_opening_chars":                           6000,
		"max_trial_theory_chars":                      4000,
		"max_closing_chars":                           8000,
		"max_exhibits_per_side":                       20,
		"max_support_tool_calls_per_opportunity":      30,
		"max_jury_note_chars":                         3000,
		"skip_voir_dire":                              0,
		"jury_juror_count":                            6,
		"jury_unanimous_required":                     1,
		"jury_minimum_concurring":                     6,
		"voir_dire_candidate_count":                   1,
		"max_voir_dire_questions_per_side_per_juror":  1,
		"max_disallowed_voir_dire_questions_per_side": 3,
		"max_for_cause_challenges_per_side":           1,
		"max_peremptory_challenges_per_side":          1,
		"max_deliberation_rounds":                     3,
		"max_dispositive_motions_per_side_pretrial":   2,
		"max_interrogatories_per_set":                 5,
		"max_interrogatory_sets_per_side":             2,
		"max_rfp_requests_per_set":                    40,
		"max_rfp_sets_per_side":                       2,
		"max_rfa_requests_per_set":                    40,
		"max_rfa_sets_per_side":                       2,
		"max_discovery_response_deadline_days":        30,
		"max_rule12_summary_chars":                    5000,
		"max_rule56_summary_chars":                    10000,
		"max_rule56_reply_chars":                      4000,
		"max_technical_reports_per_side":              3,
		"max_technical_report_summary_chars":          5000,
	}
}

func defaultJudgeVoirDireClaim() map[string]any {
	return defaultJudgeEvalClaim()
}

func defaultJudgeEvalClaim() map[string]any {
	return map[string]any{
		"claim_id":          "claim-1",
		"label":             "Digital contract dispute",
		"legal_theory":      "breach_of_contract",
		"standard_of_proof": "preponderance_of_the_evidence",
		"burden_holder":     "plaintiff",
		"elements":          []any{"contract", "breach", "causation", "damages"},
		"defenses":          []any{"no breach", "damages not proven"},
		"damages_question":  "What damages, if any, did plaintiff prove?",
	}
}

func applySummaryResult(summary *JudgeVoirDireSummary, result JudgeVoirDireResult, weight float64) {
	summary.Total++
	if result.InvalidReason != "" {
		summary.Invalid++
	} else if result.OutcomeCorrect {
		summary.Correct++
	} else if result.Allowed != nil && *result.Allowed && !result.ExpectedAllowed {
		summary.FalseAllows++
	} else if result.Allowed != nil && !*result.Allowed && result.ExpectedAllowed {
		summary.FalseDisallows++
	}
	if result.ReasonCorrect {
		summary.ReasonCorrect++
	}
	for _, tag := range result.ExpectedReasonTags {
		updateSlice(summary.ByReasonTag, tag, result, weight)
	}
	updateSlice(summary.ByQuestionFamily, result.QuestionFamily, result, weight)
	updateSlice(summary.ByTier, fmt.Sprintf("tier_%d", result.Tier), result, weight)
	updateSlice(summary.ByAskedBy, result.AskedBy, result, weight)
}

func updateSlice(m map[string]JudgeVoirDireSlice, key string, result JudgeVoirDireResult, weight float64) {
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
	} else if result.Allowed != nil && *result.Allowed && !result.ExpectedAllowed {
		s.FalseAllows++
	} else if result.Allowed != nil && !*result.Allowed && result.ExpectedAllowed {
		s.FalseDisallows++
	}
	m[key] = s
}

func finalizeSummary(summary *JudgeVoirDireSummary, totalWeight float64, correctWeight float64) {
	if summary.Total > 0 {
		summary.Accuracy = float64(summary.Correct) / float64(summary.Total)
		summary.FalseAllowRate = float64(summary.FalseAllows) / float64(summary.Total)
		summary.FalseDisallowRate = float64(summary.FalseDisallows) / float64(summary.Total)
		summary.InvalidRate = float64(summary.Invalid) / float64(summary.Total)
	}
	if totalWeight > 0 {
		summary.WeightedAccuracy = correctWeight / totalWeight
	}
	finalizeSlices(summary.ByReasonTag)
	finalizeSlices(summary.ByQuestionFamily)
	finalizeSlices(summary.ByTier)
	finalizeSlices(summary.ByAskedBy)
}

func finalizeSlices(m map[string]JudgeVoirDireSlice) {
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

func matchedReasonTags(reason string, expected []string) []string {
	reason = normalizeReasonText(reason)
	matches := make([]string, 0, len(expected))
	for _, tag := range expected {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if reasonMatchesTag(reason, tag) {
			matches = append(matches, tag)
		}
	}
	sort.Strings(matches)
	return matches
}

func reasonMatchesTag(reason string, tag string) bool {
	normalizedTag := normalizeReasonText(tag)
	if strings.Contains(reason, normalizedTag) {
		return true
	}
	for _, keyword := range reasonTagKeywords()[tag] {
		if strings.Contains(reason, keyword) {
			return true
		}
	}
	return false
}

func reasonTagKeywords() map[string][]string {
	return map[string][]string{
		"proper_bias_probe":                []string{"bias", "biased", "impartial", "favor", "disfavor", "prejudice"},
		"proper_burden_probe":              []string{"burden", "preponderance", "proof standard", "standard of proof"},
		"proper_digital_evidence_probe":    []string{"digital", "documentary", "record", "authenticated", "electronic"},
		"proper_damages_skepticism_probe":  []string{"damages", "skepticism", "money", "award"},
		"proper_attention_probe":           []string{"attention", "attend", "focus", "fairly consider", "evaluate documentary", "evaluate documentary and testimonial", "follow testimony"},
		"proper_follow_instructions_probe": []string{"follow instructions", "follow the court", "court instructions", "judge instructions", "limiting instruction", "limiting instructions", "apply the law", "personal views"},
		"precommitment_liability":          []string{"precommit", "commit", "forecast", "vote", "verdict", "liability", "liable"},
		"precommitment_damages":            []string{"precommit", "commit", "damages", "amount", "range", "award"},
		"specific_evidence_sufficiency":    []string{"specific proof", "specific evidence", "enough", "sufficient", "sufficiency"},
		"assumed_disputed_fact":            []string{"assume", "assumes", "disputed fact", "not established", "unproven"},
		"merits_argument":                  []string{"argues", "argument", "merits", "advocacy"},
		"inadmissible_material":            []string{"inadmissible", "excluded", "unadmitted", "outside the record"},
		"compound_precommitment":           []string{"compound", "precommit", "vote", "verdict"},
	}
}

func normalizeReasonText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer("_", " ", "-", " ", ".", " ", ",", " ", ";", " ", ":", " ")
	s = replacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func responseJSON(resp openai.Response) map[string]any {
	out := map[string]any{
		"text":        resp.Text,
		"response_id": resp.ResponseID,
	}
	if strings.TrimSpace(resp.RawJSON) != "" {
		out["raw_json"] = resp.RawJSON
	}
	if len(resp.ToolCalls) > 0 {
		calls := make([]map[string]any, 0, len(resp.ToolCalls))
		for _, call := range resp.ToolCalls {
			calls = append(calls, map[string]any{
				"call_id":         call.CallID,
				"name":            call.Name,
				"arguments":       call.Arguments,
				"raw_arguments":   call.RawArguments,
				"arguments_error": call.ArgumentsError,
			})
		}
		out["tool_calls"] = calls
	}
	return out
}

func resultPromptSource(result JudgeVoirDireResult) string {
	if strings.TrimSpace(result.PromptSource) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptSource)
}

func resultPromptName(result JudgeVoirDireResult) string {
	if strings.TrimSpace(result.PromptName) == "" {
		return "production"
	}
	return strings.TrimSpace(result.PromptName)
}

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func normalizeParty(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "plaintiff":
		return "plaintiff"
	case "defendant":
		return "defendant"
	default:
		return ""
	}
}

func normalizedSeverity(v float64) float64 {
	if v <= 0 {
		return 1
	}
	return v
}

func stringField(m map[string]any, key string) string {
	value, _ := m[key].(string)
	return strings.TrimSpace(value)
}

func intField(m map[string]any, key string) int {
	switch value := m[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	default:
		return 0
	}
}

func stringSliceField(m map[string]any, key string) []string {
	switch raw := m[key].(type) {
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if s, ok := item.(string); ok {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			out = append(out, strings.TrimSpace(item))
		}
		return out
	default:
		return nil
	}
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == needle {
			return true
		}
	}
	return false
}
