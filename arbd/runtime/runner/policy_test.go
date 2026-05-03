package runner

import "testing"

func TestValidatePolicyRejectsBlankJudgmentStandard(t *testing.T) {
	policy := DefaultPolicy()
	policy.JudgmentStandard = ""
	if err := ValidatePolicy(policy); err == nil {
		t.Fatal("ValidatePolicy returned nil error, want failure")
	}
}

func TestCurrentAnswersBuildsMemberMap(t *testing.T) {
	state := map[string]any{
		"case": map[string]any{
			"council_answers": []any{
				map[string]any{"member_id": "C1", "answer": 72},
				map[string]any{"member_id": "C2", "answer": 45},
			},
		},
	}
	answers := currentAnswers(state)
	if len(answers) != 2 || answers["C1"] != 72 || answers["C2"] != 45 {
		t.Fatalf("currentAnswers = %#v", answers)
	}
}

func TestFinalCouncilBuildsSeatStatusesFromState(t *testing.T) {
	state := map[string]any{
		"case": map[string]any{
			"council_members": []any{
				map[string]any{
					"member_id":        "C1",
					"model":            "m1",
					"persona_filename": "p1",
					"status":           "seated",
				},
				map[string]any{
					"member_id":        "C2",
					"model":            "m2",
					"persona_filename": "p2",
					"status":           "timed_out",
				},
			},
		},
	}
	council := finalCouncil(state)
	if len(council) != 2 {
		t.Fatalf("len(finalCouncil) = %d, want 2", len(council))
	}
	if council[0].Status != "seated" || council[1].Status != "timed_out" {
		t.Fatalf("finalCouncil statuses = %#v", council)
	}
}
