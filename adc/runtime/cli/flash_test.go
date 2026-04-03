package cli

import (
	"strings"
	"testing"

	"adjudication/adc/runtime/spec"
)

func TestParseFlashModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		direct  string
		xproxy  string
		wantErr bool
	}{
		{name: "empty", input: "", direct: "", xproxy: ""},
		{name: "bare", input: "gpt-5-mini", direct: "gpt-5-mini", xproxy: "openai://gpt-5-mini"},
		{name: "xproxy", input: "openai://gpt-5-mini", direct: "gpt-5-mini", xproxy: "openai://gpt-5-mini"},
		{name: "invalid", input: "openai/gpt-5-mini", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseFlashModel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseFlashModel(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFlashModel(%q) error = %v", tt.input, err)
			}
			if got.Direct != tt.direct || got.XProxy != tt.xproxy {
				t.Fatalf("parseFlashModel(%q) = %+v", tt.input, got)
			}
		})
	}
}

func TestOverridePISettingsDefaultModel(t *testing.T) {
	t.Parallel()

	raw := []byte("{\n  \"defaultProvider\": \"xproxy\",\n  \"defaultModel\": \"openai://gpt-5\"\n}\n")
	updated, err := overridePISettingsDefaultModel(raw, "openai://gpt-5-mini")
	if err != nil {
		t.Fatalf("overridePISettingsDefaultModel error = %v", err)
	}
	if !strings.Contains(string(updated), "\"defaultModel\": \"openai://gpt-5-mini\"") {
		t.Fatalf("updated settings missing flash model: %s", string(updated))
	}
}

func TestEnsurePIModelCatalog(t *testing.T) {
	t.Parallel()

	raw := []byte("{\n  \"providers\": {\n    \"xproxy\": {\n      \"models\": [\n        {\n          \"id\": \"openai://gpt-5\",\n          \"name\": \"OpenAI GPT-5\"\n        }\n      ]\n    }\n  }\n}\n")
	updated, err := ensurePIModelCatalog(raw, "openai://gpt-5-mini")
	if err != nil {
		t.Fatalf("ensurePIModelCatalog error = %v", err)
	}
	if !strings.Contains(string(updated), "\"id\": \"openai://gpt-5-mini\"") {
		t.Fatalf("updated catalog missing flash model: %s", string(updated))
	}
}

func TestNormalizeXProxyModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "plain", input: "gpt-5", want: "openai://gpt-5"},
		{name: "existing", input: "openrouter://openai/gpt-5", want: "openrouter://openai/gpt-5"},
		{name: "invalid", input: "openai://gpt-5?bad=1", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeXProxyModel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeXProxyModel(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeXProxyModel(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeXProxyModel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeScenarioModelsForXProxy(t *testing.T) {
	t.Parallel()

	scenario := spec.FormalScenario{
		Name:      "demo",
		CourtName: "United States District Court",
		Model:     "gpt-4.1-mini",
		Roles: []spec.RoleSpec{
			{Name: "judge", Model: "gpt-5.4"},
			{Name: "plaintiff", Model: "openrouter://openai/gpt-5"},
			{Name: "clerk"},
		},
	}

	got, err := normalizeScenarioModelsForXProxy(scenario)
	if err != nil {
		t.Fatalf("normalizeScenarioModelsForXProxy error = %v", err)
	}
	if got.Model != "openai://gpt-4.1-mini" {
		t.Fatalf("scenario model = %q", got.Model)
	}
	if got.Roles[0].Model != "openai://gpt-5.4" {
		t.Fatalf("judge model = %q", got.Roles[0].Model)
	}
	if got.Roles[1].Model != "openrouter://openai/gpt-5" {
		t.Fatalf("plaintiff model = %q", got.Roles[1].Model)
	}
	if got.Roles[2].Model != "" {
		t.Fatalf("clerk model = %q, want empty", got.Roles[2].Model)
	}
}
