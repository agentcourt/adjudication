package proceeding

import (
	"context"
	"strings"
	"sync"
	"time"

	"adjudication/common/modelrequest"
	openaiapi "adjudication/common/openai"
)

type directCouncilClient struct {
	timeout time.Duration
	mu      sync.Mutex
	clients map[string]*openaiapi.Client
}

func newDirectCouncilClient(timeout time.Duration) *directCouncilClient {
	return &directCouncilClient{
		timeout: timeout,
		clients: map[string]*openaiapi.Client{},
	}
}

func (c *directCouncilClient) CreateResponseWithMaxOutputTokens(
	ctx context.Context,
	model string,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
	temperature *float64,
	maxOutputTokens *int64,
) (openaiapi.Response, error) {
	spec, err := modelrequest.ParseLegacy(model)
	if err != nil {
		return openaiapi.Response{}, err
	}
	spec.Request.Temperature = temperature
	if maxOutputTokens != nil {
		spec.Request.MaxOutputTokens = maxOutputTokens
	}
	return c.CreateResponseWithRequestSpec(ctx, spec, inputItems, tools, previousResponseID)
}

func (c *directCouncilClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	spec modelrequest.Spec,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (openaiapi.Response, error) {
	client, err := c.clientForEndpoint(spec.Endpoint)
	if err != nil {
		return openaiapi.Response{}, err
	}
	return client.CreateResponseWithRequestSpec(ctx, spec, inputItems, tools, previousResponseID)
}

func (c *directCouncilClient) clientForEndpoint(endpoint string) (*openaiapi.Client, error) {
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	c.mu.Lock()
	defer c.mu.Unlock()
	if client, ok := c.clients[endpoint]; ok {
		return client, nil
	}
	client, err := openaiapi.NewForEndpoint(endpoint, false, c.timeout)
	if err != nil {
		return nil, err
	}
	c.clients[endpoint] = client
	return client, nil
}
