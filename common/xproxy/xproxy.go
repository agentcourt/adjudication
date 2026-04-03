package xproxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	DefaultPort              = 18459
	xproxyUpstreamTimeoutEnv = "PI_XPROXY_UPSTREAM_TIMEOUT_SECONDS"
	xproxyMaxRequestBytes    = 5 * 1024 * 1024
)

type XProxyOptions struct {
	ConfigPath        string
	Port              int
	UnixSocketPath    string
	MaxOutputTokens   *int
	LogStreamEvents   bool
	LogPayloadSummary bool
}

type XProxyServer struct {
	server   *http.Server
	listener net.Listener
}

func StartXProxyServer(opts XProxyOptions) (*XProxyServer, error) {
	cfgPath := strings.TrimSpace(opts.ConfigPath)
	if cfgPath == "" {
		return nil, errors.New("xproxy config path is required")
	}
	cfg, err := LoadXProxyConfig(cfgPath)
	if err != nil {
		return nil, err
	}
	timeout := 600.0
	if value, ok, err := positiveNumberEnv(xproxyUpstreamTimeoutEnv); err != nil {
		return nil, err
	} else if ok {
		timeout = value
	}
	logf(
		"xproxy config path=%s endpoints=%v max_output_tokens=%v upstream_timeout_sec=%v log_stream_events=%t log_payload_summary=%t",
		cfgPath,
		sortedKeys(cfg.Endpoints),
		formatOptionalInt(opts.MaxOutputTokens),
		timeout,
		opts.LogStreamEvents,
		opts.LogPayloadSummary,
	)

	handler := &xproxyHandler{
		config:            cfg,
		client:            &http.Client{Timeout: time.Duration(timeout * float64(time.Second))},
		maxOutputTokens:   opts.MaxOutputTokens,
		logStreamEvents:   opts.LogStreamEvents,
		logPayloadSummary: opts.LogPayloadSummary,
	}
	server := &http.Server{Handler: handler}
	var listener net.Listener
	if strings.TrimSpace(opts.UnixSocketPath) != "" {
		if err := ensureDir(filepathDir(opts.UnixSocketPath)); err != nil {
			return nil, err
		}
		_ = os.Remove(opts.UnixSocketPath)
		listener, err = net.Listen("unix", opts.UnixSocketPath)
		if err != nil {
			return nil, err
		}
		if err := os.Chmod(opts.UnixSocketPath, 0o666); err != nil {
			listener.Close()
			return nil, err
		}
		logf("xproxy started socket=%s log_stream_events=%t log_payload_summary=%t", opts.UnixSocketPath, opts.LogStreamEvents, opts.LogPayloadSummary)
	} else {
		port := opts.Port
		if port == 0 {
			port = DefaultPort
		}
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			return nil, err
		}
		logf("xproxy started port=%d log_stream_events=%t log_payload_summary=%t", port, opts.LogStreamEvents, opts.LogPayloadSummary)
	}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logf("xproxy serve error: %v", err)
		}
	}()
	return &XProxyServer{server: server, listener: listener}, nil
}

func (s *XProxyServer) Close() error {
	err1 := s.server.Close()
	err2 := s.listener.Close()
	if err1 != nil && err1 != http.ErrServerClosed {
		return err1
	}
	if err2 != nil && !errors.Is(err2, net.ErrClosed) {
		return err2
	}
	if s.listener.Addr().Network() == "unix" {
		_ = os.Remove(s.listener.Addr().String())
	}
	return nil
}

type xproxyHandler struct {
	config            XProxyConfig
	client            *http.Client
	maxOutputTokens   *int
	logStreamEvents   bool
	logPayloadSummary bool
}

func (h *xproxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	case r.Method == http.MethodPost && r.URL.Path == "/v1/responses":
		h.handleResponses(w, r)
		return
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
		return
	}
}

func (h *xproxyHandler) handleResponses(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	bodyReader := http.MaxBytesReader(w, r.Body, xproxyMaxRequestBytes)
	defer bodyReader.Close()
	raw, err := io.ReadAll(bodyReader)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("invalid_body: %v", err)})
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		logf("xproxy invalid_json error=%v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("invalid_json: %v", err)})
		return
	}
	spec, err := ParseXProxyModel(stringValue(payload["model"]))
	if err != nil {
		logf("xproxy invalid_model error=%v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("invalid_model: %v", err)})
		return
	}
	endpoint, ok := h.config.Endpoints[spec.Endpoint]
	if !ok {
		logf("xproxy unknown_endpoint endpoint=%s", spec.Endpoint)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("unknown_endpoint: %s", spec.Endpoint)})
		return
	}
	apiKey := os.Getenv(endpoint.APIKeyEnv)
	if strings.TrimSpace(apiKey) == "" {
		logf("xproxy missing_api_key_env env=%s", endpoint.APIKeyEnv)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("missing_api_key_env: %s", endpoint.APIKeyEnv)})
		return
	}
	payload["model"] = spec.ModelOut
	if spec.SearchRequested && (spec.Endpoint == "openai" || spec.Endpoint == "xai") {
		payload["tools"] = appendWebSearchTool(payload["tools"])
	}
	if spec.SearchRequested && endpoint.API == "anthropic-messages" {
		payload["tools"] = appendAnthropicWebSearchTool(payload["tools"])
	}
	if spec.SearchRequested && endpoint.API == "gemini-generate-content" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "gemini tools=search is not implemented"})
		return
	}
	if h.maxOutputTokens != nil {
		payload["max_output_tokens"] = *h.maxOutputTokens
	}
	reqID := randomHex(12)
	toolCount := 0
	if tools, ok := payload["tools"].([]any); ok {
		toolCount = len(tools)
	}
	logf(
		"xproxy request req_id=%s endpoint=%s model_in=%s model_out=%s search_requested=%t force_search=%t tool_count=%d stream=%t",
		reqID, spec.Endpoint, spec.ModelIn, spec.ModelOut, spec.SearchRequested, spec.ForceSearch, toolCount, boolValue(payload["stream"]),
	)
	if h.logPayloadSummary {
		summary, _ := json.Marshal(payloadSummary(payload))
		logf("xproxy payload_summary req_id=%s summary=%s", reqID, summary)
	}
	switch endpoint.API {
	case "openai-responses":
		h.forwardOpenAIResponses(w, r, endpoint, spec, payload, reqID, started, apiKey)
	case "anthropic-messages":
		h.forwardAnthropicMessages(w, endpoint, spec, payload, reqID, started, apiKey)
	case "gemini-generate-content":
		h.forwardGeminiGenerateContent(w, endpoint, spec, payload, reqID, started, apiKey)
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("unsupported_endpoint_api: %s", endpoint.API)})
	}
}

func appendWebSearchTool(value any) []any {
	tools, _ := value.([]any)
	for _, item := range tools {
		obj, ok := item.(map[string]any)
		if ok && stringValue(obj["type"]) == "web_search" {
			return tools
		}
	}
	return append(tools, map[string]any{"type": "web_search"})
}

func appendAnthropicWebSearchTool(value any) []any {
	tools, _ := value.([]any)
	for _, item := range tools {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch stringValue(obj["type"]) {
		case "web_search", "web_search_20250305":
			return tools
		}
	}
	return append(tools, map[string]any{"type": "web_search_20250305", "name": "web_search"})
}

func (h *xproxyHandler) forwardOpenAIResponses(w http.ResponseWriter, r *http.Request, endpoint XProxyEndpoint, spec ModelSpec, payload map[string]any, reqID string, started time.Time, apiKey string) {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("invalid_payload: %v", err)})
		return
	}
	upReq, err := http.NewRequest(http.MethodPost, endpoint.URL, bytes.NewReader(requestBody))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("upstream_request_error: %v", err)})
		return
	}
	upReq.Header.Set("content-type", "application/json")
	upReq.Header.Set("authorization", "Bearer "+apiKey)
	if spec.Endpoint == "xai" {
		upReq.Header.Set("user-agent", "pi-xproxy/1.0")
	}
	if value := strings.TrimSpace(r.Header.Get("http-referer")); value != "" {
		upReq.Header.Set("http-referer", value)
	}
	if value := strings.TrimSpace(r.Header.Get("x-title")); value != "" {
		upReq.Header.Set("x-title", value)
	}
	resp, err := h.client.Do(upReq)
	if err != nil {
		logf("xproxy upstream_connect_error req_id=%s error=%v", reqID, err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("upstream_connect_error: %v", err)})
		return
	}
	defer resp.Body.Close()
	for key, values := range resp.Header {
		low := strings.ToLower(key)
		if low == "connection" || low == "transfer-encoding" || low == "content-length" {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	bytesOut, copyErr := io.Copy(w, resp.Body)
	durationMs := time.Since(started).Milliseconds()
	if copyErr != nil {
		logf("xproxy stream_error req_id=%s status=%d bytes=%d duration_ms=%d error=%v", reqID, resp.StatusCode, bytesOut, durationMs, copyErr)
		return
	}
	logf("xproxy response req_id=%s status=%d bytes=%d duration_ms=%d", reqID, resp.StatusCode, bytesOut, durationMs)
	logLLMCSV(llmCSVModel(spec), len(requestBody), int(bytesOut), durationMs)
}

func (h *xproxyHandler) forwardAnthropicMessages(w http.ResponseWriter, endpoint XProxyEndpoint, spec ModelSpec, payload map[string]any, reqID string, started time.Time, apiKey string) {
	systemBlocks, anthropicMessages, err := openAIInputToAnthropic(payload["input"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("invalid_input: %v", err)})
		return
	}
	anthropicReq := map[string]any{
		"model":      stringValue(payload["model"]),
		"messages":   anthropicMessages,
		"max_tokens": maxOutputTokens(payload, 4096),
		"stream":     false,
	}
	if len(systemBlocks) > 0 {
		anthropicReq["system"] = systemBlocks
	}
	if anthropicTools := convertOpenAIToolsToAnthropic(payload["tools"]); len(anthropicTools) > 0 {
		anthropicReq["tools"] = anthropicTools
	}
	switch stringValue(payload["tool_choice"]) {
	case "required":
		anthropicReq["tool_choice"] = map[string]any{"type": "any"}
	case "auto":
		anthropicReq["tool_choice"] = map[string]any{"type": "auto"}
	}
	requestBody, err := json.Marshal(anthropicReq)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("invalid_payload: %v", err)})
		return
	}
	upReq, err := http.NewRequest(http.MethodPost, endpoint.URL, bytes.NewReader(requestBody))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("upstream_request_error: %v", err)})
		return
	}
	upReq.Header.Set("content-type", "application/json")
	upReq.Header.Set("x-api-key", apiKey)
	upReq.Header.Set("anthropic-version", endpoint.AnthropicVersion)
	if endpoint.AnthropicBeta != "" {
		upReq.Header.Set("anthropic-beta", endpoint.AnthropicBeta)
	}
	resp, err := h.client.Do(upReq)
	if err != nil {
		logf("xproxy upstream_connect_error req_id=%s error=%v", reqID, err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("upstream_connect_error: %v", err)})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		logf("xproxy upstream_http_error req_id=%s status=%d body=%s", reqID, resp.StatusCode, truncateASCII(string(body), 400))
		writeJSON(w, resp.StatusCode, map[string]any{"error": "anthropic_http_error: " + string(body)})
		return
	}
	rawRespBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("anthropic_read_error: %v", err)})
		return
	}
	var anthropicResp map[string]any
	if err := json.Unmarshal(rawRespBody, &anthropicResp); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("anthropic_decode_error: %v", err)})
		return
	}
	responseObj, responseItems, err := anthropicToResponsesResponse(anthropicResp, stringValue(payload["model"]))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("anthropic_translate_error: %v", err)})
		return
	}
	h.writeResponsesOutput(w, reqID, started, resp.StatusCode, llmCSVModel(spec), len(requestBody), len(rawRespBody), payload, responseObj, responseItems)
}

func (h *xproxyHandler) forwardGeminiGenerateContent(w http.ResponseWriter, endpoint XProxyEndpoint, spec ModelSpec, payload map[string]any, reqID string, started time.Time, apiKey string) {
	systemInstruction, geminiContents, err := openAIInputToGemini(payload["input"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("invalid_input: %v", err)})
		return
	}
	geminiReq := map[string]any{
		"contents": geminiContents,
		"generationConfig": map[string]any{
			"maxOutputTokens": maxOutputTokens(payload, 4096),
		},
	}
	if systemInstruction != nil {
		geminiReq["systemInstruction"] = systemInstruction
	}
	geminiTools := convertOpenAIToolsToGemini(payload["tools"])
	if len(geminiTools) > 0 {
		geminiReq["tools"] = geminiTools
		switch stringValue(payload["tool_choice"]) {
		case "required":
			geminiReq["toolConfig"] = map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY"}}
		case "auto":
			geminiReq["toolConfig"] = map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}
		}
	}
	requestBody, err := json.Marshal(geminiReq)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("invalid_payload: %v", err)})
		return
	}
	geminiURL := strings.TrimRight(endpoint.URL, "/") + "/" + stringValue(payload["model"]) + ":generateContent"
	upReq, err := http.NewRequest(http.MethodPost, geminiURL, bytes.NewReader(requestBody))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("upstream_request_error: %v", err)})
		return
	}
	upReq.Header.Set("content-type", "application/json")
	upReq.Header.Set("x-goog-api-key", apiKey)
	resp, err := h.client.Do(upReq)
	if err != nil {
		logf("xproxy upstream_connect_error req_id=%s error=%v", reqID, err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("upstream_connect_error: %v", err)})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		logf("xproxy upstream_http_error req_id=%s status=%d body=%s", reqID, resp.StatusCode, truncateASCII(string(body), 400))
		writeJSON(w, resp.StatusCode, map[string]any{"error": "gemini_http_error: " + string(body)})
		return
	}
	rawRespBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("gemini_read_error: %v", err)})
		return
	}
	var geminiResp map[string]any
	if err := json.Unmarshal(rawRespBody, &geminiResp); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("gemini_decode_error: %v", err)})
		return
	}
	responseObj, responseItems, err := geminiToResponsesResponse(geminiResp, stringValue(payload["model"]))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("gemini_translate_error: %v", err)})
		return
	}
	h.writeResponsesOutput(w, reqID, started, resp.StatusCode, llmCSVModel(spec), len(requestBody), len(rawRespBody), payload, responseObj, responseItems)
}

func (h *xproxyHandler) writeResponsesOutput(w http.ResponseWriter, reqID string, started time.Time, upstreamStatus int, modelIn string, bytesIn int, upstreamBytesOut int, payload map[string]any, responseObj map[string]any, responseItems []map[string]any) {
	if boolValue(payload["stream"]) {
		h.writeSyntheticResponsesStream(w, reqID, started, upstreamStatus, modelIn, bytesIn, upstreamBytesOut, responseObj, responseItems)
		return
	}
	responseBody, err := json.Marshal(responseObj)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("response_encode_error: %v", err)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(responseBody)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBody)
	durationMs := time.Since(started).Milliseconds()
	logf("xproxy response req_id=%s status=200 upstream_status=%d bytes=%d duration_ms=%d", reqID, upstreamStatus, len(responseBody), durationMs)
	logLLMCSV(modelIn, bytesIn, upstreamBytesOut, durationMs)
}

func (h *xproxyHandler) writeSyntheticResponsesStream(w http.ResponseWriter, reqID string, started time.Time, upstreamStatus int, modelIn string, bytesIn int, upstreamBytesOut int, responseObj map[string]any, responseItems []map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "close")
	w.WriteHeader(http.StatusOK)
	bytesOut := 0
	sendSSE := func(eventObj map[string]any) error {
		data, err := json.Marshal(eventObj)
		if err != nil {
			return err
		}
		packet := append([]byte("data: "), data...)
		packet = append(packet, []byte("\n\n")...)
		n, err := w.Write(packet)
		bytesOut += n
		if err != nil {
			return err
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		if h.logStreamEvents {
			logf("xproxy stream_event req_id=%s event=%s", reqID, stringValue(eventObj["type"]))
		}
		return nil
	}

	if err := sendSSE(map[string]any{"type": "response.created", "response": responseObj}); err != nil {
		h.logSyntheticStreamError(reqID, started, bytesOut, err)
		return
	}
	if err := sendSSE(map[string]any{
		"type": "response.in_progress",
		"response": map[string]any{
			"id":          responseObj["id"],
			"object":      "response",
			"model":       responseObj["model"],
			"status":      "in_progress",
			"output":      []any{},
			"output_text": "",
		},
	}); err != nil {
		h.logSyntheticStreamError(reqID, started, bytesOut, err)
		return
	}

	hasFunctionCall := false
	for _, item := range responseItems {
		if stringValue(item["type"]) == "function_call" {
			hasFunctionCall = true
			break
		}
	}
	streamIndexOffset := 0
	if hasFunctionCall {
		reasoningID := "rs_" + randomHex(16)
		if err := sendSSE(map[string]any{
			"type":         "response.output_item.added",
			"output_index": 0,
			"item": map[string]any{
				"id":      reasoningID,
				"type":    "reasoning",
				"summary": []any{},
				"status":  "in_progress",
			},
		}); err != nil {
			h.logSyntheticStreamError(reqID, started, bytesOut, err)
			return
		}
		if err := sendSSE(map[string]any{
			"type":         "response.output_item.done",
			"output_index": 0,
			"item": map[string]any{
				"id":      reasoningID,
				"type":    "reasoning",
				"summary": []any{},
				"status":  "completed",
			},
		}); err != nil {
			h.logSyntheticStreamError(reqID, started, bytesOut, err)
			return
		}
		streamIndexOffset = 1
	}

	for _, item := range responseItems {
		outputIndex := intValue(item["_output_index"]) + streamIndexOffset
		switch stringValue(item["type"]) {
		case "message":
			itemID := stringValue(item["id"])
			textValue := ""
			if content, ok := item["content"].([]map[string]any); ok && len(content) > 0 {
				textValue = stringValue(content[0]["text"])
			} else if content, ok := item["content"].([]any); ok && len(content) > 0 {
				if first, ok := content[0].(map[string]any); ok {
					textValue = stringValue(first["text"])
				}
			}
			for _, eventObj := range []map[string]any{
				{
					"type":         "response.output_item.added",
					"output_index": outputIndex,
					"item": map[string]any{
						"id":      itemID,
						"type":    "message",
						"status":  "in_progress",
						"role":    "assistant",
						"content": []any{},
					},
				},
				{
					"type":          "response.content_part.added",
					"output_index":  outputIndex,
					"item_id":       itemID,
					"content_index": 0,
					"part":          map[string]any{"type": "output_text", "text": "", "annotations": []any{}},
				},
			} {
				if err := sendSSE(eventObj); err != nil {
					h.logSyntheticStreamError(reqID, started, bytesOut, err)
					return
				}
			}
			if textValue != "" {
				if err := sendSSE(map[string]any{
					"type":          "response.output_text.delta",
					"output_index":  outputIndex,
					"item_id":       itemID,
					"content_index": 0,
					"delta":         textValue,
				}); err != nil {
					h.logSyntheticStreamError(reqID, started, bytesOut, err)
					return
				}
			}
			for _, eventObj := range []map[string]any{
				{
					"type":          "response.output_text.done",
					"output_index":  outputIndex,
					"item_id":       itemID,
					"content_index": 0,
					"text":          textValue,
				},
				{
					"type":          "response.content_part.done",
					"output_index":  outputIndex,
					"item_id":       itemID,
					"content_index": 0,
					"part":          map[string]any{"type": "output_text", "text": textValue, "annotations": []any{}},
				},
				{
					"type":         "response.output_item.done",
					"output_index": outputIndex,
					"item": map[string]any{
						"id":      itemID,
						"type":    "message",
						"status":  "completed",
						"role":    "assistant",
						"content": item["content"],
					},
				},
			} {
				if err := sendSSE(eventObj); err != nil {
					h.logSyntheticStreamError(reqID, started, bytesOut, err)
					return
				}
			}
		case "function_call":
			itemID := stringValue(item["id"])
			callID := stringValue(item["call_id"])
			name := stringValue(item["name"])
			arguments := stringValue(item["arguments"])
			for _, eventObj := range []map[string]any{
				{
					"type":         "response.output_item.added",
					"output_index": outputIndex,
					"item": map[string]any{
						"id":        itemID,
						"type":      "function_call",
						"call_id":   callID,
						"name":      name,
						"arguments": "",
						"status":    "in_progress",
					},
				},
			} {
				if err := sendSSE(eventObj); err != nil {
					h.logSyntheticStreamError(reqID, started, bytesOut, err)
					return
				}
			}
			if arguments != "" {
				if err := sendSSE(map[string]any{
					"type":         "response.function_call_arguments.delta",
					"output_index": outputIndex,
					"item_id":      itemID,
					"delta":        arguments,
				}); err != nil {
					h.logSyntheticStreamError(reqID, started, bytesOut, err)
					return
				}
			}
			for _, eventObj := range []map[string]any{
				{
					"type":         "response.function_call_arguments.done",
					"output_index": outputIndex,
					"item_id":      itemID,
					"arguments":    arguments,
				},
				{
					"type":         "response.output_item.done",
					"output_index": outputIndex,
					"item": map[string]any{
						"id":        itemID,
						"type":      "function_call",
						"call_id":   callID,
						"name":      name,
						"arguments": arguments,
						"status":    "completed",
					},
				},
			} {
				if err := sendSSE(eventObj); err != nil {
					h.logSyntheticStreamError(reqID, started, bytesOut, err)
					return
				}
			}
		}
	}

	if err := sendSSE(map[string]any{"type": "response.completed", "response": responseObj}); err != nil {
		h.logSyntheticStreamError(reqID, started, bytesOut, err)
		return
	}
	donePacket := []byte("data: [DONE]\n\n")
	n, err := w.Write(donePacket)
	bytesOut += n
	if err != nil {
		h.logSyntheticStreamError(reqID, started, bytesOut, err)
		return
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	if h.logStreamEvents {
		logf("xproxy stream_event req_id=%s event=[DONE]", reqID)
	}
	durationMs := time.Since(started).Milliseconds()
	logf("xproxy response req_id=%s status=200 upstream_status=%d bytes=%d duration_ms=%d", reqID, upstreamStatus, bytesOut, durationMs)
	logLLMCSV(modelIn, bytesIn, upstreamBytesOut, durationMs)
}

func (h *xproxyHandler) logSyntheticStreamError(reqID string, started time.Time, bytesOut int, err error) {
	durationMs := time.Since(started).Milliseconds()
	logf("xproxy stream_error req_id=%s status=200 bytes=%d duration_ms=%d error=%v", reqID, bytesOut, durationMs, err)
}

func maxOutputTokens(payload map[string]any, fallback int) int {
	if value := intValue(payload["max_output_tokens"]); value > 0 {
		return value
	}
	return fallback
}

func payloadSummary(payload map[string]any) map[string]any {
	out := map[string]any{
		"stream": boolValue(payload["stream"]),
	}
	if toolChoice, ok := payload["tool_choice"]; ok {
		out["tool_choice"] = toolChoice
	}
	input := payload["input"]
	out["input_type"] = typeName(input)
	switch v := input.(type) {
	case string:
		out["input_items"] = 1
		out["input_chars"] = len(v)
	case []any:
		out["input_items"] = len(v)
		inputChars := 0
		roleCounts := map[string]int{}
		for _, item := range v {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			role := stringValue(obj["role"])
			if role != "" {
				roleCounts[role]++
			}
			switch content := obj["content"].(type) {
			case string:
				inputChars += len(content)
			case []any:
				for _, block := range content {
					blockObj, ok := block.(map[string]any)
					if ok {
						inputChars += len(stringValue(blockObj["text"]))
					}
				}
			}
		}
		out["input_chars"] = inputChars
		if len(roleCounts) > 0 {
			out["roles"] = roleCounts
		}
	}
	if tools, ok := payload["tools"].([]any); ok {
		out["tool_count"] = len(tools)
		kinds := make([]string, 0, len(tools))
		for _, item := range tools {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			typ := stringValue(obj["type"])
			if typ == "function" {
				name := stringValue(obj["name"])
				if name == "" {
					if fn, ok := obj["function"].(map[string]any); ok {
						name = stringValue(fn["name"])
					}
				}
				if name != "" {
					kinds = append(kinds, "function:"+name)
				} else {
					kinds = append(kinds, "function")
				}
			} else if typ != "" {
				kinds = append(kinds, typ)
			}
		}
		if len(kinds) > 20 {
			kinds = kinds[:20]
		}
		if len(kinds) > 0 {
			out["tool_kinds"] = kinds
		}
	}
	return out
}
