package httputil

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPDoerInterface verifies *http.Client satisfies HTTPDoer.
// This is a compile-time check - if HTTPDoer changes incompatibly, this will fail to compile.
func TestHTTPDoerInterface(t *testing.T) {
	var _ HTTPDoer = (*http.Client)(nil)
}

// TestClientDefaults verifies default Client construction.
// Default: *http.Client with 30s timeout.
func TestClientDefaults(t *testing.T) {
	// Act
	client := NewClient()

	// Assert
	assert.NotNil(t, client)
	assert.NotNil(t, client.doer)
	assert.Equal(t, 30*time.Second, client.timeout, "default timeout should be 30s")
}

// TestClientWithTimeout verifies timeout option.
func TestClientWithTimeout(t *testing.T) {
	// Act
	client := NewClient(WithTimeout(5 * time.Second))

	// Assert
	assert.Equal(t, 5*time.Second, client.timeout, "custom timeout should be applied")
}

// TestClientWithDoer verifies custom HTTPDoer injection.
func TestClientWithDoer(t *testing.T) {
	// Arrange
	mockDoer := &mockHTTPDoer{}

	// Act
	client := NewClient(WithDoer(mockDoer))

	// Assert
	assert.Equal(t, mockDoer, client.doer, "custom doer should be injected")
}

// TestClientWithMultipleOptions verifies option composition.
func TestClientWithMultipleOptions(t *testing.T) {
	// Arrange
	mockDoer := &mockHTTPDoer{}

	// Act
	client := NewClient(
		WithTimeout(10*time.Second),
		WithDoer(mockDoer),
	)

	// Assert
	assert.Equal(t, 10*time.Second, client.timeout)
	assert.Equal(t, mockDoer, client.doer)
}

// TestClientGet verifies GET request execution.
// This test uses httptest.NewServer to verify real HTTP behavior.
func TestClientGet(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		w.Header().Set("X-Test-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("GET response"))
	}))
	defer server.Close()

	client := NewClient()

	// Act
	resp, err := client.Get(context.Background(), server.URL, nil, 1024)

	// Assert
	require.NoError(t, err, "GET request should succeed")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "GET response", resp.Body)
	assert.Equal(t, "test-value", resp.Headers["X-Test-Header"])
	assert.False(t, resp.Truncated)
}

// TestClientPost verifies POST request with body.
func TestClientPost(t *testing.T) {
	// Arrange
	expectedBody := `{"name":"test"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

		// Verify body was sent
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, expectedBody, string(body), "request body should match")

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	}))
	defer server.Close()

	client := NewClient()

	// Act
	resp, err := client.Post(context.Background(), server.URL, nil, expectedBody, 1024)

	// Assert
	require.NoError(t, err, "POST request should succeed")
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "Created", resp.Body)
}

// TestClientPut verifies PUT request.
func TestClientPut(t *testing.T) {
	// Arrange
	expectedBody := `{"updated":"value"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")

		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, expectedBody, string(body))

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient()

	// Act
	resp, err := client.Put(context.Background(), server.URL, nil, expectedBody, 1024)

	// Assert
	require.NoError(t, err, "PUT request should succeed")
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Empty(t, resp.Body, "204 No Content should have empty body")
}

// TestClientDelete verifies DELETE request.
func TestClientDelete(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Deleted"))
	}))
	defer server.Close()

	client := NewClient()

	// Act
	resp, err := client.Delete(context.Background(), server.URL, nil, 1024)

	// Assert
	require.NoError(t, err, "DELETE request should succeed")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Deleted", resp.Body)
}

// TestClientCustomHeaders verifies header forwarding.
func TestClientCustomHeaders(t *testing.T) {
	// Arrange
	expectedHeaders := map[string]string{
		"Authorization":   "Bearer token123",
		"Content-Type":    "application/json",
		"X-Custom-Header": "custom-value",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify all headers were forwarded
		for key, expectedValue := range expectedHeaders {
			assert.Equal(t, expectedValue, r.Header.Get(key), "header %s should be forwarded", key)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient()

	// Act
	resp, err := client.Get(context.Background(), server.URL, expectedHeaders, 1024)

	// Assert
	require.NoError(t, err, "request with headers should succeed")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestClientResponseHeaders verifies response header capture.
// Multi-value headers should be joined with ", " per HTTP spec.
func TestClientResponseHeaders(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "abc123")
		// Multi-value header
		w.Header().Add("Set-Cookie", "session=xyz")
		w.Header().Add("Set-Cookie", "token=abc")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient()

	// Act
	resp, err := client.Get(context.Background(), server.URL, nil, 1024)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "application/json", resp.Headers["Content-Type"])
	assert.Equal(t, "abc123", resp.Headers["X-Request-Id"])
	// Multi-value headers joined with ", "
	assert.Equal(t, "session=xyz, token=abc", resp.Headers["Set-Cookie"])
}

// TestClientContextCancellation verifies context cancellation handling.
func TestClientContextCancellation(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay to allow cancellation
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act
	_, err := client.Get(ctx, server.URL, nil, 1024)

	// Assert
	assert.Error(t, err, "cancelled context should cause error")
	assert.True(t, errors.Is(err, context.Canceled), "error should be context.Canceled")
}

// TestClientTimeout verifies timeout is applied when context has no deadline.
func TestClientTimeout(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay longer than client timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(WithTimeout(50 * time.Millisecond))

	// Act
	start := time.Now()
	_, err := client.Get(context.Background(), server.URL, nil, 1024)
	elapsed := time.Since(start)

	// Assert
	assert.Error(t, err, "timeout should trigger error")
	assert.Less(t, elapsed, 150*time.Millisecond, "should timeout before server responds")
}

// TestClientTimeoutRespectedWithDeadline verifies client timeout is only applied when context has no deadline.
func TestClientTimeoutRespectedWithDeadline(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Client with long timeout (5s), but context with short deadline (50ms)
	client := NewClient(WithTimeout(5 * time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Act
	start := time.Now()
	_, err := client.Get(ctx, server.URL, nil, 1024)
	elapsed := time.Since(start)

	// Assert
	assert.Error(t, err, "context deadline should trigger error")
	assert.Less(t, elapsed, 150*time.Millisecond, "should timeout via context deadline, not client timeout")
}

// TestClientBodyTruncation verifies response body size limiting.
// This covers the 1MB limit requirement for F058 http.request operation.
func TestClientBodyTruncation(t *testing.T) {
	tests := []struct {
		name         string
		responseSize int
		maxBodyBytes int64
		wantTrunc    bool
		wantBodyLen  int
	}{
		{
			name:         "body within limit",
			responseSize: 100,
			maxBodyBytes: 1024,
			wantTrunc:    false,
			wantBodyLen:  100,
		},
		{
			name:         "body exceeds limit",
			responseSize: 2048,
			maxBodyBytes: 1024,
			wantTrunc:    true,
			wantBodyLen:  1024,
		},
		{
			name:         "unlimited (maxBodyBytes=0)",
			responseSize: 10000,
			maxBodyBytes: 0,
			wantTrunc:    false,
			wantBodyLen:  10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			responseBody := strings.Repeat("A", tt.responseSize)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(responseBody))
			}))
			defer server.Close()

			client := NewClient()

			// Act
			resp, err := client.Get(context.Background(), server.URL, nil, tt.maxBodyBytes)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, tt.wantTrunc, resp.Truncated, "truncation flag mismatch")
			assert.Len(t, resp.Body, tt.wantBodyLen, "body length mismatch")
		})
	}
}

// TestClientDo_AllMethods verifies Do method with all HTTP methods.
func TestClientDo_AllMethods(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		body     string
		wantBody bool
	}{
		{
			name:     "GET without body",
			method:   http.MethodGet,
			body:     "",
			wantBody: false,
		},
		{
			name:     "POST with body",
			method:   http.MethodPost,
			body:     `{"data":"value"}`,
			wantBody: true,
		},
		{
			name:     "PUT with body",
			method:   http.MethodPut,
			body:     `{"update":"value"}`,
			wantBody: true,
		},
		{
			name:     "DELETE without body",
			method:   http.MethodDelete,
			body:     "",
			wantBody: false,
		},
		{
			name:     "PATCH with body",
			method:   http.MethodPatch,
			body:     `{"patch":"data"}`,
			wantBody: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.method, r.Method, "method mismatch")

				if tt.wantBody {
					body, _ := io.ReadAll(r.Body)
					assert.Equal(t, tt.body, string(body), "body mismatch")
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}))
			defer server.Close()

			client := NewClient()

			// Act
			resp, err := client.Do(context.Background(), tt.method, server.URL, nil, tt.body, 1024)

			// Assert
			require.NoError(t, err, "request should succeed")
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "OK", resp.Body)
		})
	}
}

// TestClientDo_ErrorCases verifies error handling.
func TestClientDo_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "invalid URL scheme",
			url:     "ftp://invalid.com",
			wantErr: true,
		},
		{
			name:    "non-existent host",
			url:     "http://nonexistent-host-12345.invalid",
			wantErr: true,
		},
		{
			name:    "malformed URL",
			url:     "ht!tp://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			client := NewClient(WithTimeout(1 * time.Second))

			// Act
			_, err := client.Get(context.Background(), tt.url, nil, 1024)

			// Assert
			if tt.wantErr {
				assert.Error(t, err, "should return error for invalid URL")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestClientWithMockDoer verifies custom HTTPDoer injection for testing.
func TestClientWithMockDoer(t *testing.T) {
	// Arrange
	mockDoer := &mockHTTPDoer{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"X-Mock": []string{"true"}},
			Body:       io.NopCloser(strings.NewReader("mock response")),
		},
	}

	client := NewClient(WithDoer(mockDoer))

	// Act
	resp, err := client.Get(context.Background(), "http://example.com", nil, 1024)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "mock response", resp.Body)
	assert.Equal(t, "true", resp.Headers["X-Mock"])
	assert.True(t, mockDoer.called, "mock doer should have been called")
}

// TestClientWithMockDoer_Error verifies error propagation from HTTPDoer.
func TestClientWithMockDoer_Error(t *testing.T) {
	// Arrange
	expectedErr := errors.New("mock error")
	mockDoer := &mockHTTPDoer{
		err: expectedErr,
	}

	client := NewClient(WithDoer(mockDoer))

	// Act
	_, err := client.Get(context.Background(), "http://example.com", nil, 1024)

	// Assert
	assert.Error(t, err)
	assert.True(t, mockDoer.called)
}

// TestClientConvenienceMethods verifies Get/Post/Put/Delete delegate to Do.
func TestClientConvenienceMethods(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.Method))
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	tests := []struct {
		name           string
		call           func() (*Response, error)
		expectedMethod string
	}{
		{
			name: "Get",
			call: func() (*Response, error) {
				return client.Get(ctx, server.URL, nil, 1024)
			},
			expectedMethod: http.MethodGet,
		},
		{
			name: "Post",
			call: func() (*Response, error) {
				return client.Post(ctx, server.URL, nil, "body", 1024)
			},
			expectedMethod: http.MethodPost,
		},
		{
			name: "Put",
			call: func() (*Response, error) {
				return client.Put(ctx, server.URL, nil, "body", 1024)
			},
			expectedMethod: http.MethodPut,
		},
		{
			name: "Delete",
			call: func() (*Response, error) {
				return client.Delete(ctx, server.URL, nil, 1024)
			},
			expectedMethod: http.MethodDelete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			resp, err := tt.call()

			// Assert
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, tt.expectedMethod, resp.Body, "convenience method should use correct HTTP method")
		})
	}
}

// TestClientEmptyBody verifies handling of empty request and response bodies.
func TestClientEmptyBody(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify empty body for GET
		if r.Method == http.MethodGet {
			body, _ := io.ReadAll(r.Body)
			assert.Empty(t, body, "GET should have empty body")
		}

		w.WriteHeader(http.StatusOK)
		// Empty response body
	}))
	defer server.Close()

	client := NewClient()

	// Act
	resp, err := client.Get(context.Background(), server.URL, nil, 1024)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, resp.Body, "empty response body should return empty string")
	assert.False(t, resp.Truncated)
}

// TestClientStatusCodes verifies handling of various HTTP status codes.
func TestClientStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"204 No Content", http.StatusNoContent},
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"404 Not Found", http.StatusNotFound},
		{"429 Too Many Requests", http.StatusTooManyRequests},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"502 Bad Gateway", http.StatusBadGateway},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("response body"))
			}))
			defer server.Close()

			client := NewClient()

			// Act
			resp, err := client.Get(context.Background(), server.URL, nil, 1024)

			// Assert
			require.NoError(t, err, "client should not error on non-2xx status codes")
			assert.Equal(t, tt.statusCode, resp.StatusCode, "status code should be captured")
			// 204 No Content MUST NOT include a message body per HTTP spec (Go enforces this)
			if tt.statusCode == http.StatusNoContent {
				assert.Empty(t, resp.Body, "204 No Content should have empty body")
			} else {
				assert.Equal(t, "response body", resp.Body)
			}
		})
	}
}

// mockHTTPDoer is a test double for HTTPDoer interface.
type mockHTTPDoer struct {
	response *http.Response
	err      error
	called   bool
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	m.called = true
	return m.response, m.err
}
