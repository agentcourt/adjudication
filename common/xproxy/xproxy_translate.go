package xproxy

import (
	"encoding/json"
	"fmt"
	"strings"
)

func sanitizeOpenAIID(text string) string {
	var b strings.Builder
	for _, ch := range text {
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteRune(ch)
		case ch >= 'A' && ch <= 'Z':
			b.WriteRune(ch)
		case ch >= '0' && ch <= '9':
			b.WriteRune(ch)
		case ch == '_' || ch == '-':
			b.WriteRune(ch)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 64 {
			break
		}
	}
	return b.String()
}

func textBlocksFromOpenAIContent(content any) []map[string]any {
	blocks := []map[string]any{}
	switch v := content.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": v})
		}
	case []any:
		for _, item := range v {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch stringValue(obj["type"]) {
			case "text", "input_text", "output_text":
				text := stringValue(obj["text"])
				if strings.TrimSpace(text) != "" {
					blocks = append(blocks, map[string]any{"type": "text", "text": text})
				}
			case "refusal":
				text := stringValue(obj["refusal"])
				if strings.TrimSpace(text) != "" {
					blocks = append(blocks, map[string]any{"type": "text", "text": text})
				}
			}
		}
	}
	return blocks
}

func openAIInputToAnthropic(payloadInput any) ([]map[string]any, []map[string]any, error) {
	systemBlocks := []map[string]any{}
	messages := []map[string]any{}
	appendMessage := func(role string, blocks []map[string]any) {
		if len(blocks) == 0 {
			return
		}
		if len(messages) > 0 && stringValue(messages[len(messages)-1]["role"]) == role {
			current, _ := messages[len(messages)-1]["content"].([]map[string]any)
			messages[len(messages)-1]["content"] = append(current, blocks...)
			return
		}
		messages = append(messages, map[string]any{"role": role, "content": blocks})
	}

	switch v := payloadInput.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			messages = append(messages, map[string]any{
				"role":    "user",
				"content": []map[string]any{{"type": "text", "text": v}},
			})
		}
	case []any:
		for _, raw := range v {
			msg, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			role := stringValue(msg["role"])
			msgType := stringValue(msg["type"])
			switch {
			case role == "system" || role == "developer":
				systemBlocks = append(systemBlocks, textBlocksFromOpenAIContent(msg["content"])...)
			case role == "user" || role == "assistant":
				appendMessage(role, textBlocksFromOpenAIContent(msg["content"]))
			case msgType == "function_call_output":
				callID := stringValue(msg["call_id"])
				if callID == "" {
					continue
				}
				output := msg["output"]
				outputText := stringValue(output)
				if outputText == "" {
					if marshaled, err := json.Marshal(output); err == nil {
						outputText = string(marshaled)
					}
				}
				if strings.TrimSpace(outputText) == "" {
					outputText = "(empty)"
				}
				appendMessage("user", []map[string]any{{
					"type":        "tool_result",
					"tool_use_id": callID,
					"content":     outputText,
				}})
			case msgType == "function_call":
				callID := stringValue(msg["call_id"])
				name := stringValue(msg["name"])
				if callID == "" || name == "" {
					continue
				}
				args := parseJSONArgs(msg["arguments"])
				appendMessage("assistant", []map[string]any{{
					"type":  "tool_use",
					"id":    callID,
					"name":  name,
					"input": args,
				}})
			}
		}
	default:
		return nil, nil, fmt.Errorf("payload input must be string or array")
	}
	if len(messages) == 0 {
		return nil, nil, fmt.Errorf("no usable messages in payload input")
	}
	return systemBlocks, messages, nil
}

func convertOpenAIToolsToAnthropic(tools any) []map[string]any {
	out := []map[string]any{}
	list, _ := tools.([]any)
	for _, item := range list {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch stringValue(tool["type"]) {
		case "function":
			name := stringValue(tool["name"])
			if name == "" {
				continue
			}
			params, _ := tool["parameters"].(map[string]any)
			if params == nil {
				params = map[string]any{"type": "object", "properties": map[string]any{}, "required": []any{}}
			}
			out = append(out, map[string]any{
				"name":         name,
				"description":  stringValue(tool["description"]),
				"input_schema": params,
			})
		case "web_search":
			out = append(out, map[string]any{
				"type": "web_search_20250305",
				"name": "web_search",
			})
		}
	}
	return out
}

func anthropicStopToResponsesStatus(stopReason string) string {
	switch stopReason {
	case "max_tokens":
		return "incomplete"
	case "refusal", "sensitive":
		return "failed"
	default:
		return "completed"
	}
}

func anthropicToResponsesResponse(anthropicResp map[string]any, modelID string) (map[string]any, []map[string]any, error) {
	content, _ := anthropicResp["content"].([]any)
	outputItems := []map[string]any{}
	outputTextChunks := []string{}
	outputIndex := 0
	for _, raw := range content {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch stringValue(block["type"]) {
		case "text":
			txt := stringValue(block["text"])
			outputTextChunks = append(outputTextChunks, txt)
			outputItems = append(outputItems, map[string]any{
				"id":            "msg_" + sanitizeOpenAIID(randomHex(12)),
				"type":          "message",
				"status":        "completed",
				"role":          "assistant",
				"content":       []map[string]any{{"type": "output_text", "text": txt, "annotations": []any{}}},
				"_output_index": outputIndex,
			})
			outputIndex++
		case "tool_use":
			callID := stringValue(block["id"])
			if callID == "" {
				callID = "call_" + randomHex(8)
			}
			name := stringValue(block["name"])
			if name == "" {
				name = "tool"
			}
			args, _ := json.Marshal(block["input"])
			outputItems = append(outputItems, map[string]any{
				"id":            "fc_" + sanitizeOpenAIID(randomHex(16)),
				"type":          "function_call",
				"call_id":       callID,
				"name":          name,
				"arguments":     string(args),
				"status":        "completed",
				"_output_index": outputIndex,
			})
			outputIndex++
		}
	}
	usage, _ := anthropicResp["usage"].(map[string]any)
	inputTokens := intValue(usage["input_tokens"])
	outputTokens := intValue(usage["output_tokens"])
	responseObj := map[string]any{
		"id":          valueOr(anthropicResp["id"], "resp_"+randomHex(16)),
		"object":      "response",
		"model":       modelID,
		"status":      anthropicStopToResponsesStatus(stringValue(anthropicResp["stop_reason"])),
		"output":      stripOutputIndexes(outputItems),
		"output_text": strings.Join(outputTextChunks, ""),
		"usage": map[string]any{
			"input_tokens":         inputTokens,
			"input_tokens_details": map[string]any{"cached_tokens": 0},
			"output_tokens":        outputTokens,
			"output_tokens_details": map[string]any{
				"reasoning_tokens": 0,
			},
			"total_tokens": inputTokens + outputTokens,
		},
	}
	return responseObj, outputItems, nil
}

func openAIInputToGemini(payloadInput any) (map[string]any, []map[string]any, error) {
	systemTexts := []string{}
	contents := []map[string]any{}
	callNameByID := map[string]string{}
	appendPart := func(role string, part map[string]any) {
		if len(contents) > 0 && stringValue(contents[len(contents)-1]["role"]) == role {
			parts, _ := contents[len(contents)-1]["parts"].([]map[string]any)
			contents[len(contents)-1]["parts"] = append(parts, part)
			return
		}
		contents = append(contents, map[string]any{"role": role, "parts": []map[string]any{part}})
	}

	switch v := payloadInput.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			contents = append(contents, map[string]any{"role": "user", "parts": []map[string]any{{"text": v}}})
		}
	case []any:
		for _, raw := range v {
			msg, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			role := stringValue(msg["role"])
			msgType := stringValue(msg["type"])
			switch {
			case role == "system" || role == "developer":
				for _, block := range textBlocksFromOpenAIContent(msg["content"]) {
					text := stringValue(block["text"])
					if strings.TrimSpace(text) != "" {
						systemTexts = append(systemTexts, text)
					}
				}
			case role == "user":
				for _, block := range textBlocksFromOpenAIContent(msg["content"]) {
					text := stringValue(block["text"])
					if strings.TrimSpace(text) != "" {
						appendPart("user", map[string]any{"text": text})
					}
				}
			case role == "assistant":
				for _, block := range textBlocksFromOpenAIContent(msg["content"]) {
					text := stringValue(block["text"])
					if strings.TrimSpace(text) != "" {
						appendPart("model", map[string]any{"text": text})
					}
				}
			case msgType == "function_call":
				name := stringValue(msg["name"])
				if name == "" {
					continue
				}
				callID := stringValue(msg["call_id"])
				args := parseJSONArgs(msg["arguments"])
				appendPart("model", map[string]any{"functionCall": map[string]any{"name": name, "args": args}})
				if callID != "" {
					callNameByID[callID] = name
				}
			case msgType == "function_call_output":
				callID := stringValue(msg["call_id"])
				output := parseJSONValue(msg["output"])
				name := callNameByID[callID]
				if name == "" {
					name = "tool"
				}
				appendPart("user", map[string]any{"functionResponse": map[string]any{"name": name, "response": output}})
			}
		}
	default:
		return nil, nil, fmt.Errorf("payload input must be string or array")
	}
	if len(contents) == 0 {
		return nil, nil, fmt.Errorf("no usable messages in payload input")
	}
	var systemInstruction map[string]any
	if len(systemTexts) > 0 {
		systemInstruction = map[string]any{
			"parts": []map[string]any{{"text": strings.Join(systemTexts, "\n\n")}},
		}
	}
	return systemInstruction, contents, nil
}

func convertOpenAIToolsToGemini(tools any) []map[string]any {
	functionDeclarations := []map[string]any{}
	list, _ := tools.([]any)
	for _, item := range list {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if stringValue(tool["type"]) != "function" {
			continue
		}
		name := stringValue(tool["name"])
		if name == "" {
			continue
		}
		params, _ := tool["parameters"].(map[string]any)
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}, "required": []any{}}
		}
		functionDeclarations = append(functionDeclarations, map[string]any{
			"name":        name,
			"description": stringValue(tool["description"]),
			"parameters":  params,
		})
	}
	if len(functionDeclarations) == 0 {
		return nil
	}
	return []map[string]any{{"functionDeclarations": functionDeclarations}}
}

func geminiFinishToResponsesStatus(finishReason string) string {
	switch finishReason {
	case "MAX_TOKENS":
		return "incomplete"
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
		return "failed"
	default:
		return "completed"
	}
}

func geminiToResponsesResponse(geminiResp map[string]any, modelID string) (map[string]any, []map[string]any, error) {
	candidates, _ := geminiResp["candidates"].([]any)
	var candidate map[string]any
	if len(candidates) > 0 {
		candidate, _ = candidates[0].(map[string]any)
	}
	content, _ := candidate["content"].(map[string]any)
	parts, _ := content["parts"].([]any)

	outputItems := []map[string]any{}
	outputTextChunks := []string{}
	outputIndex := 0
	for _, raw := range parts {
		part, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if text := stringValue(part["text"]); text != "" {
			outputTextChunks = append(outputTextChunks, text)
			outputItems = append(outputItems, map[string]any{
				"id":            "msg_" + sanitizeOpenAIID(randomHex(12)),
				"type":          "message",
				"status":        "completed",
				"role":          "assistant",
				"content":       []map[string]any{{"type": "output_text", "text": text, "annotations": []any{}}},
				"_output_index": outputIndex,
			})
			outputIndex++
			continue
		}
		functionCall, _ := part["functionCall"].(map[string]any)
		if functionCall != nil {
			name := stringValue(functionCall["name"])
			if name == "" {
				name = "tool"
			}
			callID := stringValue(functionCall["id"])
			if callID == "" {
				callID = "call_" + randomHex(8)
			}
			args, _ := json.Marshal(functionCall["args"])
			outputItems = append(outputItems, map[string]any{
				"id":            "fc_" + sanitizeOpenAIID(randomHex(16)),
				"type":          "function_call",
				"call_id":       callID,
				"name":          name,
				"arguments":     string(args),
				"status":        "completed",
				"_output_index": outputIndex,
			})
			outputIndex++
		}
	}

	usage, _ := geminiResp["usageMetadata"].(map[string]any)
	inputTokens := intValue(usage["promptTokenCount"])
	outputTokens := intValue(usage["candidatesTokenCount"])
	totalTokens := intValue(usage["totalTokenCount"])
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}
	responseObj := map[string]any{
		"id":          valueOr(geminiResp["responseId"], "resp_"+randomHex(16)),
		"object":      "response",
		"model":       modelID,
		"status":      geminiFinishToResponsesStatus(stringValue(candidate["finishReason"])),
		"output":      stripOutputIndexes(outputItems),
		"output_text": strings.Join(outputTextChunks, ""),
		"usage": map[string]any{
			"input_tokens":         inputTokens,
			"input_tokens_details": map[string]any{"cached_tokens": 0},
			"output_tokens":        outputTokens,
			"output_tokens_details": map[string]any{
				"reasoning_tokens": 0,
			},
			"total_tokens": totalTokens,
		},
	}
	return responseObj, outputItems, nil
}

func stripOutputIndexes(items []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		copyItem := map[string]any{}
		for key, value := range item {
			if strings.HasPrefix(key, "_") {
				continue
			}
			copyItem[key] = value
		}
		out = append(out, copyItem)
	}
	return out
}

func parseJSONArgs(value any) map[string]any {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return map[string]any{}
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(v), &out); err == nil && out != nil {
			return out
		}
	case map[string]any:
		return v
	}
	return map[string]any{}
}

func parseJSONValue(value any) any {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return map[string]any{"output": ""}
		}
		var out any
		if err := json.Unmarshal([]byte(v), &out); err == nil {
			return out
		}
		return map[string]any{"output": v}
	case map[string]any, []any:
		return v
	default:
		return map[string]any{"output": v}
	}
}

func intValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func valueOr(value any, fallback string) any {
	if str := stringValue(value); str != "" {
		return str
	}
	return fallback
}
