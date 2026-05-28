package xproxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIResponsesForwardsOpenRouterMetadataHeaderAndProvider(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")

	var gotHeader string
	var gotPayload map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-OpenRouter-Experimental-Metadata")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response","created_at":0,"model":"deepseek/deepseek-v4-flash","output":[],"openrouter_metadata":{"selected_provider":"DeepInfra"}}`))
	}))
	defer upstream.Close()

	handler := &xproxyHandler{
		config: XProxyConfig{Endpoints: map[string]XProxyEndpoint{
			"openrouter": {URL: upstream.URL, API: "openai-responses", APIKeyEnv: "OPENROUTER_API_KEY"},
		}},
		client: upstream.Client(),
	}
	body := []byte(`{"model":"openrouter://deepseek/deepseek-v4-flash","input":"hello","provider":{"only":["deepinfra/fp4"],"allow_fallbacks":false,"require_parameters":true,"quantizations":["fp4"]}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenRouter-Experimental-Metadata", "enabled")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if gotHeader != "enabled" {
		t.Fatalf("metadata header = %q, want enabled", gotHeader)
	}
	if gotPayload["model"] != "deepseek/deepseek-v4-flash" {
		t.Fatalf("upstream model = %#v", gotPayload["model"])
	}
	provider, ok := gotPayload["provider"].(map[string]any)
	if !ok {
		t.Fatalf("provider missing from upstream payload: %#v", gotPayload)
	}
	only := provider["only"].([]any)
	if only[0] != "deepinfra/fp4" {
		t.Fatalf("provider.only = %#v", provider["only"])
	}
	if provider["allow_fallbacks"] != false || provider["require_parameters"] != true {
		t.Fatalf("provider flags = %#v", provider)
	}
}
