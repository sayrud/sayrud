// Package openai is a minimal client of the OpenAI-compatible Chat Completions API.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Message is a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client calls the Chat Completions endpoint.
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// NewClient returns a client with the timeout of each request.
func NewClient(baseURL, apiKey, model string, timeout time.Duration) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:  strings.TrimSpace(apiKey),
		Model:   strings.TrimSpace(model),
		HTTP:    &http.Client{Timeout: timeout},
	}
}

// StatusError is a non-2xx response, Temporary reports whether retrying may succeed, e.g. rate limited or server errors.
type StatusError struct {
	StatusCode int
	Message    string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("status %d: %s", e.StatusCode, e.Message)
}

// Temporary reports whether the request may succeed later.
func (e *StatusError) Temporary() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode == http.StatusRequestTimeout || e.StatusCode >= 500
}

// maxResponseSize limits the response body, the completions of the shortcuts are short.
const maxResponseSize = 4 << 20

// Complete returns the content of the first choice, the temperature is 0 so the results are stable.
func (c *Client) Complete(ctx context.Context, messages []Message) (string, error) {
	body, err := json.Marshal(map[string]interface{}{
		"model":       c.Model,
		"messages":    messages,
		"temperature": 0,
		"stream":      false,
	})
	if err != nil {
		return "", errors.Wrap(err, "encode request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", errors.Wrap(err, "new request")
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "send")
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", errors.Wrap(err, "read response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &StatusError{StatusCode: resp.StatusCode, Message: errorMessage(raw)}
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", errors.Wrap(err, "decode response")
	}
	if len(result.Choices) == 0 {
		return "", errors.New("no choice in the response")
	}
	return stripThinking(result.Choices[0].Message.Content), nil
}

// errorMessage returns the message of the OpenAI error body, or the beginning of the raw body.
func errorMessage(raw []byte) string {
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err == nil && body.Error.Message != "" {
		return body.Error.Message
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// stripThinking removes the leading `<think>...</think>` block emitted by some reasoning models.
func stripThinking(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "<think>") {
		if end := strings.Index(trimmed, "</think>"); end >= 0 {
			trimmed = trimmed[end+len("</think>"):]
		}
	}
	return strings.TrimSpace(trimmed)
}
