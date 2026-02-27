package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPDoer abstracts HTTP request execution for testing.
// *http.Client satisfies this interface natively.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client wraps HTTP operations with configurable timeout and convenience methods.
type Client struct {
	doer    HTTPDoer
	timeout time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithDoer sets a custom HTTPDoer (for testing).
func WithDoer(doer HTTPDoer) Option {
	return func(c *Client) {
		c.doer = doer
	}
}

// WithTimeout sets the default timeout for requests.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// NewClient creates a new HTTP client with options.
// Default: *http.Client with 30s timeout.
func NewClient(opts ...Option) *Client {
	c := &Client{
		doer:    &http.Client{Timeout: 30 * time.Second},
		timeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Do executes an HTTP request with the given parameters.
// Creates a request with context, applies timeout when no deadline exists,
// applies headers, executes via doer, reads response via ReadBody.
// Returns Response or error.
func (c *Client) Do(ctx context.Context, method, url string, headers map[string]string, body string, maxBodyBytes int64) (*Response, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	httpResp, err := c.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	bodyStr, truncated, err := ReadBody(httpResp.Body, maxBodyBytes)
	if err != nil {
		return nil, err
	}

	respHeaders := make(map[string]string)
	for key, values := range httpResp.Header {
		respHeaders[key] = strings.Join(values, ", ")
	}

	resp := &Response{
		StatusCode: httpResp.StatusCode,
		Body:       bodyStr,
		Headers:    respHeaders,
		Truncated:  truncated,
	}

	return resp, nil
}

// Get is a convenience method for GET requests.
func (c *Client) Get(ctx context.Context, url string, headers map[string]string, maxBodyBytes int64) (*Response, error) {
	return c.Do(ctx, http.MethodGet, url, headers, "", maxBodyBytes)
}

// Post is a convenience method for POST requests.
func (c *Client) Post(ctx context.Context, url string, headers map[string]string, body string, maxBodyBytes int64) (*Response, error) {
	return c.Do(ctx, http.MethodPost, url, headers, body, maxBodyBytes)
}

// Put is a convenience method for PUT requests.
func (c *Client) Put(ctx context.Context, url string, headers map[string]string, body string, maxBodyBytes int64) (*Response, error) {
	return c.Do(ctx, http.MethodPut, url, headers, body, maxBodyBytes)
}

// Delete is a convenience method for DELETE requests.
func (c *Client) Delete(ctx context.Context, url string, headers map[string]string, maxBodyBytes int64) (*Response, error) {
	return c.Do(ctx, http.MethodDelete, url, headers, "", maxBodyBytes)
}
