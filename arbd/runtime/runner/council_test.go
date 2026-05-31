package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"adjudication/arbd/runtime/lean"
	"adjudication/arbd/runtime/spec"
	openaiapi "adjudication/common/openai"
)

func TestNormalizeCouncilAnswerValueAcceptsIntegerString(t *testing.T) {
	answer, err := normalizeCouncilAnswerValue("60")
	if err != nil {
		t.Fatalf("normalizeCouncilAnswerValue returned error: %v", err)
	}
	if answer != 60 {
		t.Fatalf("answer = %d, want 60", answer)
	}
}

func TestNormalizeCouncilAnswerValueRejectsFraction(t *testing.T) {
	if _, err := normalizeCouncilAnswerValue(60.5); err == nil {
		t.Fatal("normalizeCouncilAnswerValue returned nil error, want failure")
	}
}

func TestPreflightCouncilCandidatesReplacesUnavailableSeat(t *testing.T) {
	candidates := []CouncilSeat{
		{Model: "bad-model", PersonaFile: "bad.md", PersonaText: "bad"},
		{Model: "good-a", PersonaFile: "good-a.md", PersonaText: "good a"},
		{Model: "good-b", PersonaFile: "good-b.md", PersonaText: "good b"},
	}
	checked := []string{}
	seated, replacements, err := preflightCouncilCandidates(context.Background(), candidates, 2, func(_ context.Context, seat CouncilSeat) error {
		checked = append(checked, seat.MemberID+":"+seat.Model)
		if seat.Model == "bad-model" {
			return fmt.Errorf("404 model unavailable")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("preflightCouncilCandidates returned error: %v", err)
	}
	wantChecked := []string{"C1:bad-model", "C1:good-a", "C2:good-b"}
	if !slices.Equal(checked, wantChecked) {
		t.Fatalf("checked = %#v, want %#v", checked, wantChecked)
	}
	if len(seated) != 2 {
		t.Fatalf("seated %d council members, want 2", len(seated))
	}
	if seated[0].MemberID != "C1" || seated[0].Model != "good-a" {
		t.Fatalf("first seated member = %#v, want C1 good-a", seated[0])
	}
	if seated[1].MemberID != "C2" || seated[1].Model != "good-b" {
		t.Fatalf("second seated member = %#v, want C2 good-b", seated[1])
	}
	if len(replacements) != 1 {
		t.Fatalf("replacements = %#v, want one replacement", replacements)
	}
	replacement := replacements[0]
	if replacement.MemberID != "C1" || replacement.UnavailableModel != "bad-model" || replacement.ReplacementModel != "good-a" || !strings.Contains(replacement.Cause, "404") {
		t.Fatalf("replacement = %#v", replacement)
	}
}

func TestPreflightCouncilCandidatesFailsWhenAvailablePoolExhausted(t *testing.T) {
	candidates := []CouncilSeat{
		{Model: "bad-a", PersonaFile: "bad-a.md"},
		{Model: "bad-b", PersonaFile: "bad-b.md"},
	}
	_, _, err := preflightCouncilCandidates(context.Background(), candidates, 1, func(_ context.Context, seat CouncilSeat) error {
		return fmt.Errorf("%s unavailable", seat.Model)
	})
	if err == nil || !strings.Contains(err.Error(), "could not seat C1") {
		t.Fatalf("preflightCouncilCandidates error = %v, want seating failure", err)
	}
}

func TestIsCouncilRequestError(t *testing.T) {
	t.Parallel()

	if isCouncilRequestError(fmt.Errorf("parse function arguments for submit_council_answer: bad json")) {
		t.Fatalf("unexpected request-error match for tool argument parse error")
	}
	if isCouncilRequestError(context.Canceled) {
		t.Fatalf("unexpected request-error match for context cancellation")
	}
	if !isCouncilRequestError(fmt.Errorf("responses request failed: 404 model not found")) {
		t.Fatalf("expected responses request failure to count as request error")
	}
	if !isCouncilRequestError(fmt.Errorf("responses failed after retries: 503 unavailable")) {
		t.Fatalf("expected exhausted responses retries to count as request error")
	}
}

func TestExecuteCouncilOpportunityRetriesAfterOversizeResponse(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()

	rc := newCouncilOpportunityTestContext(t, "")
	client := &fakeCouncilResponseClient{
		responses: []openaiapi.Response{
			{Text: strings.Repeat("x", 4096), ResponseID: "oversize"},
			{ToolCalls: []openaiapi.ToolCall{{Name: "submit_council_answer", Arguments: map[string]any{"answer": "60", "rationale": "record supports a middle score"}}}, ResponseID: "valid"},
		},
	}
	if err := rc.executeCouncilOpportunity(context.Background(), client, Opportunity{ID: "deliberation:1:C1", Role: "council", Phase: "deliberation"}); err != nil {
		t.Fatalf("executeCouncilOpportunity returned error: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("client calls = %d, want 2", client.calls)
	}
	if !strings.Contains(client.inputs[1][len(client.inputs[1])-1]["content"].(string), "response payload") {
		t.Fatalf("second prompt did not include oversize correction: %#v", client.inputs[1])
	}
	caseObj := mapAny(rc.state["case"])
	answers := mapList(caseObj["council_answers"])
	if len(answers) != 1 || numericAnswer(answers[0]["answer"]) != 60 {
		t.Fatalf("answers = %#v, want one 60 answer", answers)
	}
}

func TestExecuteCouncilOpportunityDismissesAfterRepeatedOversizeResponses(t *testing.T) {
	origPromptBaseDir := promptBaseDir
	promptBaseDir = filepath.Join("..", "..", "prompts")
	defer func() { promptBaseDir = origPromptBaseDir }()

	rc := newCouncilOpportunityTestContext(t, "invalid_response")
	rc.cfg.Runtime.InvalidAttemptLimit = 2
	client := &fakeCouncilResponseClient{
		responses: []openaiapi.Response{
			{Text: strings.Repeat("x", 4096), ResponseID: "oversize-1"},
			{Text: strings.Repeat("y", 4096), ResponseID: "oversize-2"},
		},
	}
	if err := rc.executeCouncilOpportunity(context.Background(), client, Opportunity{ID: "deliberation:1:C1", Role: "council", Phase: "deliberation"}); err != nil {
		t.Fatalf("executeCouncilOpportunity returned error: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("client calls = %d, want 2", client.calls)
	}
	assertRemovedCouncilMember(t, rc, "invalid_response")
	if got := mapString(rc.events[0].Payload["cause"]); !strings.Contains(got, "exceeded invalid-attempt limit") || !strings.Contains(got, "byte limit") {
		t.Fatalf("cause = %q, want invalid-attempt byte-limit cause", got)
	}
}

func TestRemoveTimedOutCouncilMemberRecordsEvent(t *testing.T) {
	t.Parallel()

	rc := newCouncilRemovalTestContext(t, "timed_out")
	opportunity := Opportunity{Phase: "deliberation"}
	seat := CouncilSeat{MemberID: "C1", Model: "openrouter://openai/gpt-4o"}
	if err := rc.removeTimedOutCouncilMember(opportunity, seat, context.DeadlineExceeded); err != nil {
		t.Fatalf("removeTimedOutCouncilMember returned error: %v", err)
	}
	assertRemovedCouncilMember(t, rc, "timed_out")
}

func TestRemoveRequestFailedCouncilMemberRecordsEvent(t *testing.T) {
	t.Parallel()

	rc := newCouncilRemovalTestContext(t, "request_failed")
	opportunity := Opportunity{Phase: "deliberation"}
	seat := CouncilSeat{MemberID: "C1", Model: "openrouter://anthropic/claude-3.7-sonnet"}
	if err := rc.removeRequestFailedCouncilMember(opportunity, seat, fmt.Errorf("responses request failed: 404 model not found")); err != nil {
		t.Fatalf("removeRequestFailedCouncilMember returned error: %v", err)
	}
	assertRemovedCouncilMember(t, rc, "request_failed")
	if got := mapString(rc.events[0].Payload["cause"]); !strings.Contains(got, "404") {
		t.Fatalf("cause = %q, want 404 marker", got)
	}
}

type fakeCouncilResponseClient struct {
	responses []openaiapi.Response
	errs      []error
	inputs    [][]map[string]any
	calls     int
}

func (c *fakeCouncilResponseClient) CreateResponseWithMaxOutputTokens(_ context.Context, _ string, inputItems []map[string]any, _ []map[string]any, _ string, _ *float64, _ *int64) (openaiapi.Response, error) {
	c.inputs = append(c.inputs, append([]map[string]any(nil), inputItems...))
	call := c.calls
	c.calls++
	if call < len(c.errs) && c.errs[call] != nil {
		return openaiapi.Response{}, c.errs[call]
	}
	if call < len(c.responses) {
		return c.responses[call], nil
	}
	return openaiapi.Response{}, fmt.Errorf("unexpected fake council client call %d", call+1)
}

func newCouncilOpportunityTestContext(t *testing.T, removalStatus string) *runContext {
	t.Helper()

	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := councilEngineScript(removalStatus)
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	runtimeLimits := DefaultRuntimeLimits()
	runtimeLimits.MaxResponseBytes = 2048
	runtimeLimits.InvalidAttemptLimit = 3
	return &runContext{
		cfg: Config{
			Engine:    lean.Engine{Command: []string{enginePath}},
			OutputDir: dir,
			Policy:    DefaultPolicy(),
			Runtime:   runtimeLimits,
		},
		complaint: spec.Complaint{Question: "How strong is the record?"},
		state: map[string]any{
			"policy": DefaultPolicy().StateMap(),
			"case": map[string]any{
				"phase":              "deliberation",
				"deliberation_round": 1,
				"openings":           []map[string]any{},
				"arguments":          []map[string]any{},
				"rebuttals":          []map[string]any{},
				"surrebuttals":       []map[string]any{},
				"closings":           []map[string]any{},
				"offered_evidence":   []map[string]any{},
				"technical_reports":  []map[string]any{},
				"submitted_evidence": []map[string]any{},
				"council_answers":    []map[string]any{},
				"council_members":    []map[string]any{{"member_id": "C1", "status": "seated"}},
			},
		},
		council: []CouncilSeat{{MemberID: "C1", Model: "openrouter://openai/gpt-4o", PersonaText: "Concise."}},
	}
}

func newCouncilRemovalTestContext(t *testing.T, status string) *runContext {
	t.Helper()

	dir := t.TempDir()
	enginePath := filepath.Join(dir, "engine.sh")
	script := councilEngineScript(status)
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	return &runContext{
		cfg: Config{
			Engine:    lean.Engine{Command: []string{enginePath}},
			OutputDir: dir,
		},
		state: map[string]any{
			"case": map[string]any{
				"phase": "deliberation",
			},
		},
	}
}

func councilEngineScript(removalStatus string) string {
	removalState := fmt.Sprintf(`{"ok":true,"state":{"case":{"phase":"deliberation","council_members":[{"member_id":"C1","status":"%s"}]}}}`, removalStatus)
	answerState := `{"ok":true,"state":{"case":{"phase":"deliberation","council_members":[{"member_id":"C1","status":"seated"}],"council_answers":[{"round":1,"member_id":"C1","answer":60,"rationale":"record supports a middle score"}]}}}`
	return fmt.Sprintf(`#!/bin/sh
request=$(cat)
case "$request" in
  *remove_council_member*) printf '%%s\n' '%s' ;;
  *) printf '%%s\n' '%s' ;;
esac
`, removalState, answerState)
}

func assertRemovedCouncilMember(t *testing.T, rc *runContext, status string) {
	t.Helper()

	caseObj := mapAny(rc.state["case"])
	members := mapList(caseObj["council_members"])
	if len(members) != 1 {
		t.Fatalf("council member count = %d, want 1", len(members))
	}
	if got := mapString(members[0]["status"]); got != status {
		t.Fatalf("member status = %q, want %s", got, status)
	}
	if len(rc.events) != 1 {
		t.Fatalf("event count = %d, want 1", len(rc.events))
	}
	event := rc.events[0]
	if event.Type != "council_member_removed" {
		t.Fatalf("event type = %q, want council_member_removed", event.Type)
	}
	if got := mapString(event.Payload["member_id"]); got != "C1" {
		t.Fatalf("member_id = %q, want C1", got)
	}
	if got := mapString(event.Payload["status"]); got != status {
		t.Fatalf("status = %q, want %s", got, status)
	}
}

func numericAnswer(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
