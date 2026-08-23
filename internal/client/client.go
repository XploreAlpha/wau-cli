// Package client provides HTTP client for WAU-core-kernel API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a HTTP client for WAU-core-kernel.
type Client struct {
	baseURL    string
	httpClient *http.Client
	role       string
	auth       AuthProvider // optional; nil = 不发 Authorization header

	// retry 配置(per 第二刀 P1.1)
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

// Options for creating a new Client.
type Options struct {
	BaseURL        string
	Role           string
	Timeout        time.Duration
	Auth           AuthProvider // optional; nil = 无 auth
	MaxRetries     int           // 0 = DefaultMaxRetries(3)
	InitialBackoff time.Duration // 0 = DefaultInitialBackoff(500ms)
	MaxBackoff     time.Duration // 0 = DefaultMaxBackoff(8s)
}

// NewClient creates a new WAU kernel client.
func NewClient(opts Options) *Client {
	if opts.BaseURL == "" {
		opts.BaseURL = "http://localhost:18400"
	}
	if opts.Role == "" {
		opts.Role = "external_agent"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = DefaultMaxRetries
	}
	if opts.InitialBackoff == 0 {
		opts.InitialBackoff = DefaultInitialBackoff
	}
	if opts.MaxBackoff == 0 {
		opts.MaxBackoff = DefaultMaxBackoff
	}

	return &Client{
		baseURL:        opts.BaseURL,
		role:           opts.Role,
		auth:           opts.Auth,
		maxRetries:     opts.MaxRetries,
		initialBackoff: opts.InitialBackoff,
		maxBackoff:     opts.MaxBackoff,
		httpClient: &http.Client{
			Timeout: opts.Timeout,
		},
	}
}

// doRequest performs an HTTP request and returns the response body.
//
// 现在裸 doRequest 不带 retry — 外部调用 doRequestWithRetry 才会重试。
// (保留 doRequest 公开语义,便于测试直接调。)
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, v interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set standard headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Agent-Role", c.role) // 保留向后兼容(老 server 不读 Bearer)

	// Add bearer auth(per 第二刀 P1.2)
	if c.auth != nil {
		if token, terr := c.auth.Token(ctx); terr == nil && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		// auth 失败不 fatal — 让 server 用 401 回应,client 再 refresh
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode >= 400 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	// Decode response
	if v != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// APIError represents an error from the API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Body)
}

// Get performs a GET request (with retry).
func (c *Client) Get(ctx context.Context, path string, v interface{}) error {
	return c.doRequestWithRetry(ctx, http.MethodGet, path, nil, v, RequestOpts{})
}

// Post performs a POST request (with retry).
func (c *Client) Post(ctx context.Context, path string, body, v interface{}) error {
	return c.doRequestWithRetry(ctx, http.MethodPost, path, body, v, RequestOpts{})
}

// Put performs a PUT request (with retry).
func (c *Client) Put(ctx context.Context, path string, body, v interface{}) error {
	return c.doRequestWithRetry(ctx, http.MethodPut, path, body, v, RequestOpts{})
}

// Delete performs a DELETE request (with retry).
func (c *Client) Delete(ctx context.Context, path string, v interface{}) error {
	return c.doRequestWithRetry(ctx, http.MethodDelete, path, nil, v, RequestOpts{})
}

// GetWithOpts / PostWithOpts 等 — 给需要覆盖 retry 行为的 caller 用。
func (c *Client) PostWithOpts(ctx context.Context, path string, body, v interface{}, opts RequestOpts) error {
	return c.doRequestWithRetry(ctx, http.MethodPost, path, body, v, opts)
}

func (c *Client) GetWithOpts(ctx context.Context, path string, v interface{}, opts RequestOpts) error {
	return c.doRequestWithRetry(ctx, http.MethodGet, path, nil, v, opts)
}
