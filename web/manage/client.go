package manage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls the ARB service API and decodes its JSON envelopes.
type Client struct {
	Base  string
	Token string
	HTTP  *http.Client
}

func NewClient(base, token string) *Client {
	return &Client{Base: base, Token: token, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Call performs one service request.  It returns the HTTP status and
// the decoded JSON body.  A transport failure or an undecodable body
// returns an error; a service-level error returns normally with the
// envelope, so callers can show the service's own error text.
func (c *Client) Call(method, path string, body []byte) (int, map[string]any, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.Base+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return resp.StatusCode, nil, fmt.Errorf("service returned HTTP %d with a non-JSON body: %.200s", resp.StatusCode, data)
	}
	return resp.StatusCode, payload, nil
}

// EnvelopeError extracts the service error text from a response
// envelope, or "" when the envelope reports ok.
func EnvelopeError(status int, payload map[string]any) string {
	if ok, _ := payload["ok"].(bool); ok {
		return ""
	}
	if e, ok := payload["error"].(map[string]any); ok {
		code, _ := e["code"].(string)
		msg, _ := e["message"].(string)
		if code != "" && msg != "" {
			return fmt.Sprintf("HTTP %d %s: %s", status, code, msg)
		}
		if code != "" {
			return fmt.Sprintf("HTTP %d %s", status, code)
		}
	}
	return fmt.Sprintf("HTTP %d", status)
}
