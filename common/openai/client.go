package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)

type ToolCall struct {
	CallID         string
	Name           string
	Arguments      map[string]any
	RawArguments   string
	ArgumentsError string
}

type Response struct {
	Text       string
	ToolCalls  []ToolCall
	ResponseID string
}

type Client struct {
	client             openai.Client
	online             bool
	defaultTemperature *float64
	retryDelays        []time.Duration
}

func New(apiKey string, baseURL string, online bool, timeout time.Duration) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	var defaultTemperature *float64
	if raw := strings.TrimSpace(os.Getenv("OPENAI_TEMPERATURE")); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse OPENAI_TEMPERATURE: %w", err)
		}
		defaultTemperature = &v
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
		option.WithRequestTimeout(timeout),
		option.WithMaxRetries(0),
	}
	if strings.Contains(baseURL, "openrouter.ai") {
		if site := strings.TrimSpace(os.Getenv("OPENROUTER_SITE_URL")); site != "" {
			opts = append(opts, option.WithHeader("HTTP-Referer", site))
		}
		if title := strings.TrimSpace(os.Getenv("OPENROUTER_APP_NAME")); title != "" {
			opts = append(opts, option.WithHeader("X-Title", title))
		}
	}
	return &Client{
		client:             openai.NewClient(opts...),
		online:             online,
		defaultTemperature: defaultTemperature,
		retryDelays:        []time.Duration{0, 5 * time.Second, 30 * time.Second},
	}, nil
}

func NewFromEnv(online bool, timeout time.Duration) (*Client, error) {
	openAIKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	openRouterKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	apiKey := openAIKey
	if apiKey == "" {
		apiKey = openRouterKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY or OPENROUTER_API_KEY is required")
	}
	baseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	if baseURL == "" {
		if openAIKey == "" && openRouterKey != "" {
			baseURL = "https://openrouter.ai/api/v1"
		} else {
			baseURL = "https://api.openai.com/v1"
		}
	}
	return New(apiKey, baseURL, online, timeout)
}

func (c *Client) CreateResponse(
	ctx context.Context,
	model string,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
	temperature *float64,
) (Response, error) {
	return c.CreateResponseWithMaxOutputTokens(ctx, model, inputItems, tools, previousResponseID, temperature, nil)
}

func (c *Client) CreateResponseWithMaxOutputTokens(
	ctx context.Context,
	model string,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
	temperature *float64,
	maxOutputTokens *int64,
) (Response, error) {
	convertedInput, err := convertInputItems(inputItems)
	if err != nil {
		return Response{}, err
	}
	convertedTools, err := convertTools(tools, c.online)
	if err != nil {
		return Response{}, err
	}
	params := responseParams(model, convertedInput, convertedTools, previousResponseID, temperature, c.defaultTemperature, maxOutputTokens)

	maxAttempts := 1 + len(c.retryDelays)
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		res, err := c.client.Responses.New(ctx, params)
		if err == nil {
			return parseResponse(res)
		}
		lastErr = err
		if c.shouldRetry(err, attempt, maxAttempts) {
			delay := c.retryDelay(attempt)
			fmt.Fprintf(
				os.Stderr,
				"openai request retryable error attempt=%d/%d code=%s cause=%s retry_in=%s\n",
				attempt+1,
				maxAttempts,
				retryStatusCode(err),
				retryCause(err),
				delay.String(),
			)
			if err := c.sleepBeforeRetry(ctx, attempt); err != nil {
				return Response{}, fmt.Errorf("responses request canceled during backoff: %w", err)
			}
			continue
		}
		return Response{}, fmt.Errorf("responses request failed: %w", err)
	}
	if lastErr != nil {
		return Response{}, fmt.Errorf("responses failed after retries: %w", lastErr)
	}
	return Response{}, fmt.Errorf("responses failed after retries")
}

func responseParams(
	model string,
	input responses.ResponseInputParam,
	tools []responses.ToolUnionParam,
	previousResponseID string,
	temperature *float64,
	defaultTemperature *float64,
	maxOutputTokens *int64,
) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(model),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: input,
		},
		Tools: tools,
	}
	effectiveTemperature := temperature
	if effectiveTemperature == nil {
		effectiveTemperature = defaultTemperature
	}
	if effectiveTemperature != nil {
		params.Temperature = openai.Float(*effectiveTemperature)
	}
	if previousResponseID != "" {
		params.PreviousResponseID = openai.String(previousResponseID)
	}
	if maxOutputTokens != nil && *maxOutputTokens > 0 {
		params.MaxOutputTokens = openai.Int(*maxOutputTokens)
	}
	return params
}

func parseResponse(res *responses.Response) (Response, error) {
	if res == nil {
		return Response{}, fmt.Errorf("responses: nil response")
	}
	out := Response{
		ResponseID: res.ID,
		Text:       res.OutputText(),
	}
	calls := make([]ToolCall, 0)
	for _, item := range res.Output {
		if item.Type != "function_call" {
			continue
		}
		args := map[string]any{}
		rawArguments := strings.TrimSpace(item.Arguments)
		argumentsError := ""
		if rawArguments != "" {
			if err := json.Unmarshal([]byte(rawArguments), &args); err != nil {
				argumentsError = err.Error()
				args = nil
			}
		}
		calls = append(calls, ToolCall{
			CallID:         item.CallID,
			Name:           item.Name,
			Arguments:      args,
			RawArguments:   rawArguments,
			ArgumentsError: argumentsError,
		})
	}
	out.ToolCalls = calls
	return out, nil
}

func convertInputItems(items []map[string]any) (responses.ResponseInputParam, error) {
	out := make(responses.ResponseInputParam, 0, len(items))
	for _, item := range items {
		typ, _ := item["type"].(string)
		switch typ {
		case "function_call_output":
			callID, _ := item["call_id"].(string)
			output, _ := item["output"].(string)
			out = append(out, responses.ResponseInputItemParamOfFunctionCallOutput(callID, output))
			continue
		}
		roleRaw, hasRole := item["role"]
		contentRaw, hasContent := item["content"]
		contentItemsRaw, hasContentItems := item["content_items"]
		if !hasRole || (!hasContent && !hasContentItems) {
			return nil, fmt.Errorf("unsupported input item shape: %v", item)
		}
		role, ok := roleRaw.(string)
		if !ok {
			return nil, fmt.Errorf("input item role must be string")
		}
		msgRole, err := toMessageRole(role)
		if err != nil {
			return nil, err
		}
		if hasContentItems {
			contentItems, err := convertContentItems(contentItemsRaw)
			if err != nil {
				return nil, err
			}
			out = append(out, responses.ResponseInputItemParamOfMessage(contentItems, msgRole))
			continue
		}
		content, ok := contentRaw.(string)
		if !ok {
			return nil, fmt.Errorf("input item content must be string")
		}
		out = append(out, responses.ResponseInputItemParamOfMessage(content, msgRole))
	}
	return out, nil
}

func convertContentItems(raw any) (responses.ResponseInputMessageContentListParam, error) {
	itemsRaw, ok := raw.([]map[string]any)
	if !ok {
		itemsAny, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("content_items must be an array")
		}
		itemsRaw = make([]map[string]any, 0, len(itemsAny))
		for _, entry := range itemsAny {
			item, ok := entry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("content_items entry must be an object")
			}
			itemsRaw = append(itemsRaw, item)
		}
	}
	content := make(responses.ResponseInputMessageContentListParam, 0, len(itemsRaw))
	for _, item := range itemsRaw {
		itemType, _ := item["type"].(string)
		switch itemType {
		case "input_text":
			text, _ := item["text"].(string)
			content = append(content, responses.ResponseInputContentParamOfInputText(text))
		case "input_image":
			var image responses.ResponseInputImageParam
			if detail, _ := item["detail"].(string); strings.TrimSpace(detail) != "" {
				image.Detail = responses.ResponseInputImageDetail(detail)
			} else {
				image.Detail = responses.ResponseInputImageDetailAuto
			}
			if imageURL, _ := item["image_url"].(string); strings.TrimSpace(imageURL) != "" {
				image.ImageURL = openai.String(imageURL)
			}
			if fileID, _ := item["file_id"].(string); strings.TrimSpace(fileID) != "" {
				image.FileID = openai.String(fileID)
			}
			content = append(content, responses.ResponseInputContentUnionParam{OfInputImage: &image})
		case "input_file":
			var file responses.ResponseInputFileParam
			if fileData, _ := item["file_data"].(string); strings.TrimSpace(fileData) != "" {
				file.FileData = openai.String(fileData)
			}
			if fileURL, _ := item["file_url"].(string); strings.TrimSpace(fileURL) != "" {
				file.FileURL = openai.String(fileURL)
			}
			if fileID, _ := item["file_id"].(string); strings.TrimSpace(fileID) != "" {
				file.FileID = openai.String(fileID)
			}
			if filename, _ := item["filename"].(string); strings.TrimSpace(filename) != "" {
				file.Filename = openai.String(filename)
			}
			content = append(content, responses.ResponseInputContentUnionParam{OfInputFile: &file})
		default:
			return nil, fmt.Errorf("unsupported content item type: %s", itemType)
		}
	}
	return content, nil
}

func convertTools(tools []map[string]any, online bool) ([]responses.ToolUnionParam, error) {
	out := make([]responses.ToolUnionParam, 0, len(tools)+1)
	hasWebSearch := false
	for _, t := range tools {
		typ, _ := t["type"].(string)
		switch typ {
		case "function":
			name, _ := t["name"].(string)
			if name == "" {
				return nil, fmt.Errorf("function tool missing name")
			}
			parameters := map[string]any{}
			if raw, ok := t["parameters"].(map[string]any); ok {
				parameters = raw
			}
			out = append(out, responses.ToolParamOfFunction(name, parameters, false))
		case "web_search":
			hasWebSearch = true
			out = append(out, responses.ToolParamOfWebSearchPreview(responses.WebSearchToolTypeWebSearchPreview))
		default:
			return nil, fmt.Errorf("unsupported tool type: %s", typ)
		}
	}
	if online && !hasWebSearch {
		out = append(out, responses.ToolParamOfWebSearchPreview(responses.WebSearchToolTypeWebSearchPreview))
	}
	return out, nil
}

func toMessageRole(role string) (responses.EasyInputMessageRole, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user":
		return responses.EasyInputMessageRoleUser, nil
	case "assistant":
		return responses.EasyInputMessageRoleAssistant, nil
	case "system":
		return responses.EasyInputMessageRoleSystem, nil
	case "developer":
		return responses.EasyInputMessageRoleDeveloper, nil
	default:
		return "", fmt.Errorf("unsupported message role: %s", role)
	}
}

func (c *Client) shouldRetry(err error, attempt int, maxAttempts int) bool {
	if attempt >= maxAttempts-1 {
		return false
	}
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		code := apiErr.StatusCode
		if code == 408 || code == 409 || code == 429 {
			return true
		}
		return code >= 500 && code <= 599
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "timeout") {
		return true
	}
	if strings.Contains(msg, "temporary failure") && strings.Contains(msg, "name resolution") {
		return true
	}
	return false
}

func (c *Client) retryDelay(attempt int) time.Duration {
	if attempt >= len(c.retryDelays) {
		return 0
	}
	return c.retryDelays[attempt]
}

func (c *Client) sleepBeforeRetry(ctx context.Context, attempt int) error {
	delay := c.retryDelay(attempt)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryStatusCode(err error) string {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode > 0 {
		return strconv.Itoa(apiErr.StatusCode)
	}
	return "n/a"
}

func retryCause(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Error()
	}
	return err.Error()
}
