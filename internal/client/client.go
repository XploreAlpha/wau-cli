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
}

// Options for creating a new Client.
type Options struct {
	BaseURL string
	Role    string
	Timeout time.Duration
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

	return &Client{
		baseURL: opts.BaseURL,
		role:    opts.Role,
		httpClient: &http.Client{
			Timeout: opts.Timeout,
		},
	}
}

// doRequest performs an HTTP request and returns the response body.
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

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Agent-Role", c.role)

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

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, path string, v interface{}) error {
	return c.doRequest(ctx, http.MethodGet, path, nil, v)
}

// Post performs a POST request.
func (c *Client) Post(ctx context.Context, path string, body, v interface{}) error {
	return c.doRequest(ctx, http.MethodPost, path, body, v)
}

// Put performs a PUT request.
func (c *Client) Put(ctx context.Context, path string, body, v interface{}) error {
	return c.doRequest(ctx, http.MethodPut, path, body, v)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, v interface{}) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, v)
}
