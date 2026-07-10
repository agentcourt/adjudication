package console

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	maxBytes   int64
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	JSON       map[string]any
}

func NewClient(timeout time.Duration, maxBytes int64) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		maxBytes:   maxBytes,
	}
}

func (c *Client) JSON(ctx context.Context, sys SystemConfig, method string, requestPath string, body []byte) (Response, error) {
	resp, err := c.do(ctx, sys, method, requestPath, body, "application/json")
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes+1))
	if err != nil {
		return Response{}, err
	}
	if int64(len(raw)) > c.maxBytes {
		return Response{}, fmt.Errorf("service response exceeded %d bytes", c.maxBytes)
	}
	out := Response{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: raw}
	if len(bytes.TrimSpace(raw)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		_ = dec.Decode(&out.JSON)
	}
	return out, nil
}

func (c *Client) Proxy(ctx context.Context, sys SystemConfig, method string, requestPath string, body []byte, contentType string) (*http.Response, error) {
	return c.ProxyWithHeaders(ctx, sys, method, requestPath, body, contentType, nil)
}

func (c *Client) ProxyWithHeaders(ctx context.Context, sys SystemConfig, method string, requestPath string, body []byte, contentType string, headers http.Header) (*http.Response, error) {
	return c.doWithHeaders(ctx, sys, method, requestPath, body, contentType, headers)
}

func (c *Client) do(ctx context.Context, sys SystemConfig, method string, requestPath string, body []byte, contentType string) (*http.Response, error) {
	return c.doWithHeaders(ctx, sys, method, requestPath, body, contentType, nil)
}

func (c *Client) doWithHeaders(ctx context.Context, sys SystemConfig, method string, requestPath string, body []byte, contentType string, headers http.Header) (*http.Response, error) {
	base := strings.TrimSpace(sys.BaseURL)
	if base == "" {
		return nil, fmt.Errorf("%s service URL is not configured", sys.Label)
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse %s service URL: %w", sys.Label, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%s service URL must use http or https", sys.Label)
	}
	if !strings.HasPrefix(requestPath, "/") {
		return nil, fmt.Errorf("service path must start with /")
	}
	rel, err := url.Parse(requestPath)
	if err != nil {
		return nil, err
	}
	if rel.IsAbs() || rel.Host != "" {
		return nil, fmt.Errorf("service path must be relative to configured service base")
	}
	u.Path = joinURLPath(u.Path, rel.Path)
	u.RawQuery = rel.RawQuery
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if strings.TrimSpace(sys.BearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(sys.BearerToken))
	}
	return c.httpClient.Do(req)
}

func joinURLPath(basePath string, relPath string) string {
	if strings.TrimSpace(basePath) == "" || basePath == "/" {
		return relPath
	}
	return path.Join("/", basePath, relPath)
}
