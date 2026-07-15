package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"adjudication/adc/runtime/lean"
	"adjudication/common/openai"
)

func TestLoadJudgeRule56Fixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	line := `{"id":"r56-test","tier":1,"issue_family":"missing_element","case_theme":"theme","moving_party":"defendant","request_text":"Defendant seeks summary judgment.","statement_of_undisputed_facts":"No record evidence supports causation.","opposition_text":"Plaintiff identifies no causation evidence.","expected_disposition":"granted","expected_reason_tags":["missing_element"],"severity":1}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	fixtures, err := LoadJudgeRule56Fixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeRule56Fixtures error = %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].ID != "r56-test" {
		t.Fatalf("fixtures = %+v", fixtures)
	}
}

func TestBuildJudgeRule56StateCreatesMotionDocket(t *testing.T) {
	t.Parallel()

	fixture := testRule56Fixture("denied", "credibility_dispute")
	state := BuildJudgeRule56State(fixture)
	caseObj, _ := state["case"].(map[string]any)
	if caseObj["phase"] != "pretrial" || caseObj["status"] != "pretrial" {
		t.Fatalf("case phase/status = %v/%v", caseObj["phase"], caseObj["status"])
	}
	docket, _ := caseObj["docket"].([]any)
	var hasMotion bool
	var hasOpposition bool
	for _, item := range docket {
		entry, _ := item.(map[string]any)
		if entry["title"] == "Rule 56 Motion" {
			hasMotion = true
			if !strings.Contains(entry["description"].(string), fixture.RequestText) {
				t.Fatalf("motion description = %q", entry["description"])
			}
		}
		if entry["title"] == "Rule 56 Opposition" {
			hasOpposition = true
		}
	}
	if !hasMotion || !hasOpposition {
		t.Fatalf("docket missing motion or opposition: %+v", docket)
	}
}

func TestScoreJudgeRule56ResponseDetectsFalseGrant(t *testing.T) {
	t.Parallel()

	fixture := testRule56Fixture("denied", "credibility_dispute")
	resp := openai.Response{
		ToolCalls: []openai.ToolCall{{
			Name: JudgeRule56Tool,
			Arguments: map[string]any{
				"motion_index": 0,
				"disposition":  "granted",
				"reasoning":    "The movant says the testimony is not credible.",
			},
		}},
	}
	result := scoreJudgeRule56Response(fixture, "test-model", false, nil, nil, nil, nil, resp)
	if result.InvalidReason != "" {
		t.Fatalf("InvalidReason = %q", result.InvalidReason)
	}
	if result.OutcomeCorrect {
		t.Fatalf("OutcomeCorrect = true, want false")
	}
	summary := JudgeRule56Summary{
		ByReasonTag:   map[string]JudgeRule56Slice{},
		ByIssueFamily: map[string]JudgeRule56Slice{},
		ByTier:        map[string]JudgeRule56Slice{},
		ByMovingParty: map[string]JudgeRule56Slice{},
	}
	applyRule56SummaryResult(&summary, result, 1)
	if summary.FalseGrants != 1 {
		t.Fatalf("FalseGrants = %d, want 1", summary.FalseGrants)
	}
}

func TestRule56ReasonTagsMatchLiveWording(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag    string
		reason string
	}{
		{
			tag:    "credibility_dispute",
			reason: "Denied because the motion asks the court to weigh witness credibility.",
		},
		{
			tag:    "competing_inference",
			reason: "Denied because a reasonable jury could draw competing inferences from the record.",
		},
		{
			tag:    "no_genuine_dispute",
			reason: "Granted because the undisputed record leaves no genuine dispute and the movant is entitled to judgment as a matter of law.",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()
			matches := matchedRule56ReasonTags(tt.reason, []string{tt.tag})
			if len(matches) != 1 || matches[0] != tt.tag {
				t.Fatalf("matchedRule56ReasonTags(%q, %q) = %v", tt.reason, tt.tag, matches)
			}
		})
	}
}

func TestRunJudgeRule56DryRunWritesReports(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixtures.jsonl")
	fixtureLine := `{"id":"r56-dry","tier":1,"issue_family":"missing_element","case_theme":"theme","moving_party":"defendant","request_text":"Defendant seeks summary judgment on causation.","statement_of_undisputed_facts":"Plaintiff has no causation evidence.","opposition_text":"Plaintiff concedes the record has no causation witness or document.","expected_disposition":"granted","expected_reason_tags":["missing_element"],"severity":1}`
	if err := os.WriteFile(fixturePath, []byte(fixtureLine+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile fixture error = %v", err)
	}
	engineScript := writeFakeJudgeRule56Engine(t)
	outDir := filepath.Join(t.TempDir(), "out")
	summary, err := RunJudgeRule56(nil, JudgeRule56Options{
		FixturesPath: fixturePath,
		OutputDir:    outDir,
		Engine:       lean.New([]string{engineScript}),
		Model:        "dry-model",
		DryRun:       true,
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("RunJudgeRule56 error = %v", err)
	}
	if summary.Total != 1 || summary.Correct != 1 || summary.Invalid != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	rawSummary, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("ReadFile summary error = %v", err)
	}
	var parsed JudgeRule56Summary
	if err := json.Unmarshal(rawSummary, &parsed); err != nil {
		t.Fatalf("Unmarshal summary error = %v", err)
	}
	if parsed.Total != 1 || parsed.WeightedAccuracy != 1 {
		t.Fatalf("parsed summary = %+v", parsed)
	}
	rawResults, err := os.ReadFile(filepath.Join(outDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile results error = %v", err)
	}
	if !strings.Contains(string(rawResults), `"lean_accepted":true`) {
		t.Fatalf("results missing accepted Lean decision: %s", rawResults)
	}
}

func testRule56Fixture(expectedDisposition string, tag string) JudgeRule56Fixture {
	return JudgeRule56Fixture{
		ID:                  "r56-test",
		Tier:                1,
		IssueFamily:         tag,
		CaseTheme:           "summary judgment test",
		MovingParty:         "defendant",
		RequestText:         "Defendant seeks summary judgment.",
		StatementOfFacts:    "Defendant says the record is undisputed.",
		OppositionText:      "Plaintiff identifies contrary sworn testimony.",
		ExpectedDisposition: expectedDisposition,
		ExpectedReasonTags:  []string{tag},
		Severity:            1,
	}
}

func writeFakeJudgeRule56Engine(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "engine.sh")
	body := `#!/bin/sh
req=$(cat)
case "$req" in
*'"request_type":"role_view"'*)
  printf '%s' '{"ok":true,"view":{"role":"judge","state":{"case":"visible"},"redactions":[],"role_private":{}}}'
  ;;
*'"request_type":"next_opportunity"'*)
  printf '%s' '{"ok":true,"state_version":0,"opportunity":{"opportunity_id":"opp-1","role":"judge","phase":"pretrial","kind":"turn","may_pass":false,"actor_message":"Current pretrial opportunity for judge: act on this objective now.","objective":"For case 0, decide Rule 56 motion_index 0 with disposition granted, denied, or partial, and explain the decisive record-based reason.","allowed_tools":["decide_rule56_motion"],"step_budget":3,"priority":100,"constraints":{}}}'
  ;;
*'"request_type":"apply_decision"'*)
  printf '%s' '{"ok":true,"result_kind":"execute_tool","state":{"accepted":true},"action":{"action_type":"decide_rule56_motion"}}'
  ;;
*)
  printf '%s' '{"ok":false,"error":"unexpected request"}'
  ;;
esac
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("WriteFile engine error = %v", err)
	}
	return path
}
