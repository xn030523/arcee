package yydsmail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultBaseURL = "https://maliapi.215.im/v1"

type Client struct {
	baseURL     string
	httpClient  *http.Client
	apiKey      string
	bearerToken string
}

type Option func(*Client)

func WithBaseURL(rawURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(rawURL, "/")
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func WithAPIKey(apiKey string) Option {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

func WithBearerToken(token string) Option {
	return func(c *Client) {
		c.bearerToken = token
	}
}

func NewClient(opts ...Option) *Client {
	client := &Client{
		baseURL:    defaultBaseURL,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

type apiResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Message string `json:"message"`
	Error   string `json:"error"`
	Code    string `json:"code"`
}

type APIError struct {
	StatusCode int
	Message    string
	Code       string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("yydsmail api error: status=%d code=%s message=%s", e.StatusCode, e.Code, e.Message)
	}
	if e.Message != "" {
		return fmt.Sprintf("yydsmail api error: status=%d message=%s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("yydsmail api error: status=%d", e.StatusCode)
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body any) (*http.Request, error) {
	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	} else if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}

	return req, nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if len(rawBody) == 0 {
		if resp.StatusCode >= http.StatusBadRequest {
			return &APIError{StatusCode: resp.StatusCode}
		}
		return nil
	}

	if out == nil {
		if resp.StatusCode >= http.StatusBadRequest {
			return parseAPIError(resp.StatusCode, rawBody)
		}
		return nil
	}

	if err := json.Unmarshal(rawBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return parseAPIError(resp.StatusCode, rawBody)
	}

	return nil
}

func parseAPIError(statusCode int, rawBody []byte) error {
	var payload apiResponse[json.RawMessage]
	if err := json.Unmarshal(rawBody, &payload); err == nil {
		message := firstNonEmpty(payload.Message, payload.Error)
		if message == "" {
			message = string(rawBody)
		}
		return &APIError{
			StatusCode: statusCode,
			Message:    message,
			Code:       payload.Code,
		}
	}

	return &APIError{
		StatusCode: statusCode,
		Message:    string(rawBody),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
