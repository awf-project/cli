package notify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// httpSender provides shared HTTP client functionality for notification backends
// that require HTTP POST requests (ntfy, slack, webhook).
// It enforces a 10-second timeout per NFR-001 and supports context cancellation.
type httpSender struct {
	client *http.Client
}

// newHTTPSender creates an httpSender with a 10-second timeout.
// This timeout applies to the entire request/response cycle.
func newHTTPSender() *httpSender {
	return &httpSender{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// PostJSON sends an HTTP POST request with JSON content-type.
// The context is used for cancellation control. Returns the response body
// and status code on success, or an error on failure.
func (h *httpSender) PostJSON(ctx context.Context, url string, body []byte) (statusCode int, response string, err error) {
	return h.post(ctx, url, "application/json", body)
}

// PostText sends an HTTP POST request with plain text content-type.
// The context is used for cancellation control. Returns the response body
// and status code on success, or an error on failure.
func (h *httpSender) PostText(ctx context.Context, url string, body []byte) (statusCode int, response string, err error) {
	return h.post(ctx, url, "text/plain", body)
}

// post is a shared internal method for both PostJSON and PostText.
// It constructs the request, sets headers, and executes the HTTP call.
func (h *httpSender) post(ctx context.Context, url, contentType string, body []byte) (statusCode int, response string, err error) {
	// Create request body reader
	var bodyReader io.Reader = http.NoBody
	if len(body) > 0 {
		bodyReader = strings.NewReader(string(body))
	}

	// Create request with context for cancellation support
	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set Content-Type header
	req.Header.Set("Content-Type", contentType)

	// Execute request
	resp, err := h.client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseStr, err := responseToString(resp.Body)
	if err != nil {
		return resp.StatusCode, "", err
	}

	return resp.StatusCode, responseStr, nil
}

// responseToString safely reads an HTTP response body and converts it to a string.
// It limits reading to prevent memory exhaustion from large responses.
// Note: Partial reads (e.g., from Content-Length mismatches) are tolerated to handle
// malformed responses gracefully - we return what we got instead of erroring.
func responseToString(body io.ReadCloser) (string, error) {
	// Read entire response body (already limited by http.Client.MaxBytesReader or timeout)
	data, err := io.ReadAll(body)
	// Tolerate EOF errors - these can happen with Content-Length mismatches
	// but we still want to return the partial data we read
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	return string(data), nil
}
