package httpx

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Connection setup follows Go's standard transport bounds. These limits end
// when TCP and TLS setup complete; they never budget response generation or reading.
const (
	defaultDialTimeout         = 30 * time.Second
	defaultTLSHandshakeTimeout = 10 * time.Second
)

// HTTPDoer abstracts HTTP request execution for testing.
// *http.Client satisfies this interface natively.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client wraps HTTP operations with an optional caller-selected timeout and
// convenience methods.
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

// WithDoerDecorator wraps the constructed transport without replacing its timeout policy.
// The decorator is useful for request instrumentation and must preserve cancellation.
func WithDoerDecorator(decorate func(HTTPDoer) HTTPDoer) Option {
	return func(c *Client) {
		if decorate != nil {
			c.doer = decorate(c.doer)
		}
	}
}

// WithTimeout sets a request timeout when timeout is positive. A shorter parent
// context deadline remains authoritative through normal context composition.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// NewClient creates a new HTTP client with options.
// By default, total request lifetime is controlled by the caller's context.
// TCP connection establishment is bounded to 30 seconds and TLS negotiation to
// 10 seconds. These setup limits do not bound response headers or body reading.
func NewClient(opts ...Option) *Client {
	c := &Client{
		doer: &http.Client{Transport: newExecutionTransport()},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func newConnectionDialer() *net.Dialer {
	return &net.Dialer{Timeout: defaultDialTimeout, KeepAlive: 30 * time.Second}
}

func newExecutionTransport() *http.Transport {
	dialer := newConnectionDialer()

	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: time.Second,
	}
}

// Do executes an HTTP request with the given parameters.
// Creates a request with context, applies any explicit client timeout,
// applies headers, executes via doer, reads response via ReadBody.
// Returns Response or error.
func (c *Client) Do(ctx context.Context, method, url string, headers map[string]string, body string, maxBodyBytes int64) (*Response, error) {
	if c.timeout > 0 {
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
		return nil, fmt.Errorf("failed to create HTTP request: %w", RedactError(err))
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	httpResp, err := c.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", RedactError(err))
	}
	defer httpResp.Body.Close()

	bodyStr, truncated, err := ReadBody(httpResp.Body, maxBodyBytes)
	if err != nil {
		return nil, RedactError(err)
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
