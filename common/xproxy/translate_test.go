package xproxy

import (
	"strings"
	"testing"
)

func TestAppendWebSearchTools(t *testing.T) {
	t.Parallel()

	openAI := appendWebSearchTool([]any{map[string]any{"type": "function", "name": "lookup"}})
	if len(openAI) != 2 {
		t.Fatalf("appendWebSearchTool len = %d, want 2", len(openAI))
	}
	openAI = appendWebSearchTool(openAI)
	if len(openAI) != 2 {
		t.Fatalf("appendWebSearchTool duplicate len = %d, want 2", len(openAI))
	}

	anthropic := appendAnthropicWebSearchTool([]any{map[string]any{"type": "function", "name": "lookup"}})
	if len(anthropic) != 2 {
		t.Fatalf("appendAnthropicWebSearchTool len = %d, want 2", len(anthropic))
	}
	anthropic = appendAnthropicWebSearchTool(anthropic)
	if len(anthropic) != 2 {
		t.Fatalf("appendAnthropicWebSearchTool duplicate len = %d, want 2", len(anthropic))
	}
}

func TestSanitizeOpenAIID(t *testing.T) {
	t.Parallel()

	got := sanitizeOpenAIID("bad id/with spaces and punctuation!" + strings.Repeat("x", 80))
	if strings.ContainsAny(got, " /!") {
		t.Fatalf("sanitizeOpenAIID = %q", got)
	}
	if len(got) > 64 {
		t.Fatalf("sanitizeOpenAIID len = %d, want <= 64", len(got))
	}
}

func TestOpenAIInputToAnthropic(t *testing.T) {
	t.Parallel()

	system, messages, err := openAIInputToAnthropic([]any{
		map[string]any{"role": "system", "content": "Federal rules apply."},
		map[string]any{"type": "function_call", "call_id": "call_1", "name": "get_case", "arguments": `{"case_id":"case-1"}`},
		map[string]any{"type": "function_call_output", "call_id": "call_1", "output": map[string]any{"caption": "Peter v. Samantha"}},
		map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "What happened?"}}},
	})
	if err != nil {
		t.Fatalf("openAIInputToAnthropic error = %v", err)
	}
	if len(system) != 1 || system[0]["text"] != "Federal rules apply." {
		t.Fatalf("system = %+v", system)
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}
	if messages[0]["role"] != "assistant" {
		t.Fatalf("messages[0] = %+v", messages[0])
	}
	content, _ := messages[1]["content"].([]map[string]any)
	if len(content) != 2 {
		t.Fatalf("messages[1] content = %+v", messages[1]["content"])
	}

	if _, _, err := openAIInputToAnthropic(map[string]any{}); err == nil {
		t.Fatalf("openAIInputToAnthropic invalid input error = nil")
	}
}

func TestOpenAIInputToGemini(t *testing.T) {
	t.Parallel()

	systemInstruction, contents, err := openAIInputToGemini([]any{
		map[string]any{"role": "developer", "content": "Court rules"},
		map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "Prior answer"}}},
		map[string]any{"type": "function_call", "call_id": "call_2", "name": "get_case", "arguments": `{"case_id":"case-2"}`},
		map[string]any{"type": "function_call_output", "call_id": "call_2", "output": map[string]any{"caption": "Peter v. Samantha"}},
		map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "Summarize the dispute"}}},
	})
	if err != nil {
		t.Fatalf("openAIInputToGemini error = %v", err)
	}
	if systemInstruction == nil {
		t.Fatalf("systemInstruction = nil")
	}
	if len(contents) < 2 {
		t.Fatalf("contents = %+v", contents)
	}
	last := contents[len(contents)-1]
	if last["role"] != "user" {
		t.Fatalf("last content role = %+v", last)
	}
}

func TestProviderResponsesTranslation(t *testing.T) {
	t.Parallel()

	anthropicResp := map[string]any{
		"id":          "msg_1",
		"stop_reason": "end_turn",
		"content": []any{
			map[string]any{"type": "text", "text": "Answer text"},
			map[string]any{"type": "tool_use", "id": "call_1", "name": "get_case", "input": map[string]any{"case_id": "case-1"}},
		},
		"usage": map[string]any{"input_tokens": 10, "output_tokens": 4},
	}
	responseObj, items, err := anthropicToResponsesResponse(anthropicResp, "anthropic://claude")
	if err != nil {
		t.Fatalf("anthropicToResponsesResponse error = %v", err)
	}
	if responseObj["status"] != "completed" || responseObj["output_text"] != "Answer text" {
		t.Fatalf("responseObj = %+v", responseObj)
	}
	if len(items) != 2 || items[1]["type"] != "function_call" || items[1]["call_id"] != "call_1" {
		t.Fatalf("items = %+v", items)
	}

	geminiResp := map[string]any{
		"responseId": "resp_1",
		"candidates": []any{
			map[string]any{
				"finishReason": "STOP",
				"content": map[string]any{
					"parts": []any{
						map[string]any{"text": "Gemini answer"},
						map[string]any{"functionCall": map[string]any{"name": "get_case", "args": map[string]any{"case_id": "case-2"}, "id": "call_2"}},
					},
				},
			},
		},
		"usageMetadata": map[string]any{"promptTokenCount": 6, "candidatesTokenCount": 3, "totalTokenCount": 9},
	}
	responseObj, items, err = geminiToResponsesResponse(geminiResp, "gemini://gemini-2.5-pro")
	if err != nil {
		t.Fatalf("geminiToResponsesResponse error = %v", err)
	}
	if responseObj["status"] != "completed" || responseObj["output_text"] != "Gemini answer" {
		t.Fatalf("gemini responseObj = %+v", responseObj)
	}
	if len(items) != 2 || items[1]["call_id"] != "call_2" {
		t.Fatalf("gemini items = %+v", items)
	}
}

func TestPayloadSummary(t *testing.T) {
	t.Parallel()

	summary := payloadSummary(map[string]any{
		"stream":      "true",
		"tool_choice": "required",
		"input": []any{
			map[string]any{"role": "system", "content": "Rules"},
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "What happened?"}}},
		},
	})
	if summary["stream"] != true || summary["tool_choice"] != "required" {
		t.Fatalf("summary = %+v", summary)
	}
	if summary["input_items"] != 2 || summary["input_chars"] != len("Rules")+len("What happened?") {
		t.Fatalf("summary = %+v", summary)
	}
	roles, _ := summary["roles"].(map[string]int)
	if roles["system"] != 1 || roles["user"] != 1 {
		t.Fatalf("roles = %+v", roles)
	}
	if maxOutputTokens(map[string]any{"max_output_tokens": 200}, 50) != 200 {
		t.Fatalf("maxOutputTokens explicit failed")
	}
	if maxOutputTokens(map[string]any{}, 50) != 50 {
		t.Fatalf("maxOutputTokens fallback failed")
	}
}
