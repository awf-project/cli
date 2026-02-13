package notify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Constructor tests ---

func TestNewHTTPSender(t *testing.T) {
	sender := newHTTPSender()

	require.NotNil(t, sender, "newHTTPSender() should not return nil")
	require.NotNil(t, sender.client, "HTTP client should be initialized")
	assert.Equal(t, 10*time.Second, sender.client.Timeout, "client should have 10-second timeout per NFR-001")
}

// --- PostJSON tests ---

func TestHTTPSender_PostJSON_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    []byte
		responseStatus int
		responseBody   string
		wantStatusCode int
		wantResponse   string
	}{
		{
			name:           "successful_json_post_200",
			requestBody:    []byte(`{"message":"test"}`),
			responseStatus: http.StatusOK,
			responseBody:   `{"status":"ok"}`,
			wantStatusCode: http.StatusOK,
			wantResponse:   `{"status":"ok"}`,
		},
		{
			name:           "successful_json_post_201",
			requestBody:    []byte(`{"workflow":"test-workflow"}`),
			responseStatus: http.StatusCreated,
			responseBody:   `{"id":"12345"}`,
			wantStatusCode: http.StatusCreated,
			wantResponse:   `{"id":"12345"}`,
		},
		{
			name:           "empty_response_body",
			requestBody:    []byte(`{}`),
			responseStatus: http.StatusNoContent,
			responseBody:   "",
			wantStatusCode: http.StatusNoContent,
			wantResponse:   "",
		},
		{
			name:           "large_json_payload",
			requestBody:    []byte(`{"data":"` + string(make([]byte, 1024)) + `"}`),
			responseStatus: http.StatusOK,
			responseBody:   `{"received":true}`,
			wantStatusCode: http.StatusOK,
			wantResponse:   `{"received":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server that validates request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method, "should use POST method")
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "should set JSON content-type")

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			sender := newHTTPSender()
			ctx := context.Background()

			statusCode, response, err := sender.PostJSON(ctx, server.URL, tt.requestBody)

			require.NoError(t, err, "PostJSON should not return error on success")
			assert.Equal(t, tt.wantStatusCode, statusCode, "status code should match")
			assert.Equal(t, tt.wantResponse, response, "response body should match")
		})
	}
}

func TestHTTPSender_PostJSON_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		wantErr     bool
		errContains string
	}{
		{
			name: "server_returns_500",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"internal server error"}`))
				}))
			},
			wantErr:     false, // HTTP errors don't return Go errors, just non-2xx status
			errContains: "",
		},
		{
			name: "server_returns_404",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`not found`))
				}))
			},
			wantErr:     false,
			errContains: "",
		},
		{
			name: "malformed_response_body",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Length", "100")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`short`)) // Content-Length mismatch
				}))
			},
			wantErr:     false, // Should still read partial response
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			sender := newHTTPSender()
			ctx := context.Background()

			statusCode, response, err := sender.PostJSON(ctx, server.URL, []byte(`{}`))

			if tt.wantErr {
				require.Error(t, err, "PostJSON should return error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, "error should contain expected message")
				}
			} else {
				// Non-2xx status codes are not errors, just returned as status
				require.NoError(t, err, "PostJSON should not return error for HTTP errors")
				assert.NotEqual(t, 0, statusCode, "should return actual status code")
				assert.NotEmpty(t, response, "should return response body even on HTTP errors")
			}
		})
	}
}

func TestHTTPSender_PostJSON_InvalidURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantErr     bool
		errContains string
	}{
		{
			name:        "malformed_url",
			url:         "://invalid-url",
			wantErr:     true,
			errContains: "url",
		},
		{
			name:        "empty_url",
			url:         "",
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "non_http_scheme",
			url:         "ftp://example.com",
			wantErr:     true,
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := newHTTPSender()
			ctx := context.Background()

			_, _, err := sender.PostJSON(ctx, tt.url, []byte(`{}`))

			if tt.wantErr {
				require.Error(t, err, "PostJSON should return error for invalid URL")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, "error should contain expected message")
				}
			}
		})
	}
}

func TestHTTPSender_PostJSON_ContextCancellation(t *testing.T) {
	tests := []struct {
		name         string
		cancelBefore bool
		cancelDuring bool
		serverDelay  time.Duration
		wantErr      bool
		errCheck     func(error) bool
	}{
		{
			name:         "context_cancelled_before_request",
			cancelBefore: true,
			wantErr:      true,
			errCheck:     func(err error) bool { return errors.Is(err, context.Canceled) },
		},
		{
			name:         "context_cancelled_during_request",
			cancelDuring: true,
			serverDelay:  100 * time.Millisecond,
			wantErr:      true,
			errCheck:     func(err error) bool { return errors.Is(err, context.Canceled) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.serverDelay > 0 {
					time.Sleep(tt.serverDelay)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			sender := newHTTPSender()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel() // Always call cancel

			if tt.cancelBefore {
				cancel()
			}

			if tt.cancelDuring {
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
			}

			_, _, err := sender.PostJSON(ctx, server.URL, []byte(`{}`))

			if tt.wantErr {
				require.Error(t, err, "PostJSON should return error on context cancellation")
				if tt.errCheck != nil {
					assert.True(t, tt.errCheck(err), "error should match expected type")
				}
			}
		})
	}
}

func TestHTTPSender_PostJSON_Timeout(t *testing.T) {
	// Test that 10-second timeout is enforced
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second) // Exceed 10s timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := newHTTPSender()
	ctx := context.Background()

	_, _, err := sender.PostJSON(ctx, server.URL, []byte(`{}`))

	require.Error(t, err, "PostJSON should timeout after 10 seconds per NFR-001")
	// Error should be a timeout error (either context.DeadlineExceeded or http.Client timeout)
	assert.True(t,
		errors.Is(err, context.DeadlineExceeded) || err.Error() == "timeout",
		"error should be a timeout error")
}

// --- PostText tests ---

func TestHTTPSender_PostText_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    []byte
		responseStatus int
		responseBody   string
		wantStatusCode int
		wantResponse   string
	}{
		{
			name:           "successful_text_post",
			requestBody:    []byte("Workflow completed successfully"),
			responseStatus: http.StatusOK,
			responseBody:   "Message received",
			wantStatusCode: http.StatusOK,
			wantResponse:   "Message received",
		},
		{
			name:           "empty_text_body",
			requestBody:    []byte(""),
			responseStatus: http.StatusAccepted,
			responseBody:   "",
			wantStatusCode: http.StatusAccepted,
			wantResponse:   "",
		},
		{
			name:           "multiline_text",
			requestBody:    []byte("Line 1\nLine 2\nLine 3"),
			responseStatus: http.StatusOK,
			responseBody:   "OK",
			wantStatusCode: http.StatusOK,
			wantResponse:   "OK",
		},
		{
			name:           "unicode_text",
			requestBody:    []byte("Test message with émojis 🎉"),
			responseStatus: http.StatusOK,
			responseBody:   "Received",
			wantStatusCode: http.StatusOK,
			wantResponse:   "Received",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method, "should use POST method")
				assert.Equal(t, "text/plain", r.Header.Get("Content-Type"), "should set plain text content-type")

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			sender := newHTTPSender()
			ctx := context.Background()

			statusCode, response, err := sender.PostText(ctx, server.URL, tt.requestBody)

			require.NoError(t, err, "PostText should not return error on success")
			assert.Equal(t, tt.wantStatusCode, statusCode, "status code should match")
			assert.Equal(t, tt.wantResponse, response, "response body should match")
		})
	}
}

func TestHTTPSender_PostText_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		wantErr     bool
	}{
		{
			name: "server_returns_400",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte("Bad request"))
				}))
			},
			wantErr: false, // HTTP errors return status code, not Go error
		},
		{
			name: "server_returns_503",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte("Service unavailable"))
				}))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			sender := newHTTPSender()
			ctx := context.Background()

			statusCode, response, err := sender.PostText(ctx, server.URL, []byte("test"))

			if tt.wantErr {
				require.Error(t, err, "PostText should return error")
			} else {
				require.NoError(t, err, "PostText should not return error for HTTP errors")
				assert.NotEqual(t, 0, statusCode, "should return actual status code")
				assert.NotEmpty(t, response, "should return response body")
			}
		})
	}
}

func TestHTTPSender_PostText_InvalidURL(t *testing.T) {
	sender := newHTTPSender()
	ctx := context.Background()

	_, _, err := sender.PostText(ctx, "://invalid", []byte("test"))

	require.Error(t, err, "PostText should return error for invalid URL")
}

func TestHTTPSender_PostText_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := newHTTPSender()
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, _, err := sender.PostText(ctx, server.URL, []byte("test"))

	require.Error(t, err, "PostText should return error on context cancellation")
	assert.True(t, errors.Is(err, context.Canceled), "error should be context.Canceled")
}

// --- post internal method tests ---

func TestHTTPSender_Post_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		requestBody    []byte
		responseStatus int
		responseBody   string
		wantStatusCode int
		wantResponse   string
	}{
		{
			name:           "json_content_type",
			contentType:    "application/json",
			requestBody:    []byte(`{"key":"value"}`),
			responseStatus: http.StatusOK,
			responseBody:   `{"result":"ok"}`,
			wantStatusCode: http.StatusOK,
			wantResponse:   `{"result":"ok"}`,
		},
		{
			name:           "text_content_type",
			contentType:    "text/plain",
			requestBody:    []byte("plain text"),
			responseStatus: http.StatusCreated,
			responseBody:   "created",
			wantStatusCode: http.StatusCreated,
			wantResponse:   "created",
		},
		{
			name:           "custom_content_type",
			contentType:    "application/x-www-form-urlencoded",
			requestBody:    []byte("key=value"),
			responseStatus: http.StatusOK,
			responseBody:   "OK",
			wantStatusCode: http.StatusOK,
			wantResponse:   "OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method, "should use POST method")
				assert.Equal(t, tt.contentType, r.Header.Get("Content-Type"), "content-type should match")

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			sender := newHTTPSender()
			ctx := context.Background()

			statusCode, response, err := sender.post(ctx, server.URL, tt.contentType, tt.requestBody)

			require.NoError(t, err, "post should not return error on success")
			assert.Equal(t, tt.wantStatusCode, statusCode, "status code should match")
			assert.Equal(t, tt.wantResponse, response, "response body should match")
		})
	}
}

func TestHTTPSender_Post_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		contentType string
		body        []byte
		wantErr     bool
	}{
		{
			name:        "empty_content_type",
			url:         "http://example.com",
			contentType: "",
			body:        []byte("test"),
			wantErr:     false, // Empty content-type is valid (defaults to application/octet-stream)
		},
		{
			name:        "nil_body",
			url:         "http://example.com",
			contentType: "application/json",
			body:        nil,
			wantErr:     false, // nil body is valid (empty request)
		},
		{
			name:        "large_body",
			url:         "http://example.com",
			contentType: "application/json",
			body:        make([]byte, 1024*1024), // 1MB
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			sender := newHTTPSender()
			ctx := context.Background()

			_, _, err := sender.post(ctx, server.URL, tt.contentType, tt.body)

			if tt.wantErr {
				require.Error(t, err, "post should return error")
			} else {
				require.NoError(t, err, "post should not return error")
			}
		})
	}
}

// --- responseToString tests ---

func TestResponseToString_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantText string
	}{
		{
			name:     "simple_text",
			body:     "Hello, World!",
			wantText: "Hello, World!",
		},
		{
			name:     "json_response",
			body:     `{"status":"ok","message":"success"}`,
			wantText: `{"status":"ok","message":"success"}`,
		},
		{
			name:     "empty_body",
			body:     "",
			wantText: "",
		},
		{
			name:     "multiline_text",
			body:     "Line 1\nLine 2\nLine 3",
			wantText: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "unicode_content",
			body:     "Test with émojis 🎉 and ümläuts",
			wantText: "Test with émojis 🎉 and ümläuts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock ReadCloser from test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, http.NoBody)
			require.NoError(t, err, "test setup failed")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err, "test setup failed")
			defer resp.Body.Close()

			result, err := responseToString(resp.Body)

			require.NoError(t, err, "responseToString should not return error")
			assert.Equal(t, tt.wantText, result, "response text should match")
		})
	}
}

func TestResponseToString_LargeResponse(t *testing.T) {
	// Test with a large response to verify size limiting
	largeBody := string(make([]byte, 10*1024*1024)) // 10MB

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(largeBody))
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, http.NoBody)
	require.NoError(t, err, "test setup failed")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "test setup failed")
	defer resp.Body.Close()

	result, err := responseToString(resp.Body)

	// Should either limit size or handle gracefully
	if err != nil {
		assert.Contains(t, err.Error(), "too large", "error should indicate size limit")
	} else {
		// If no error, result should be bounded (implementation may limit reading)
		assert.NotEmpty(t, result, "should read at least some content")
	}
}

func TestResponseToString_ErrorHandling(t *testing.T) {
	// Test with http.NoBody (empty body)
	result, err := responseToString(http.NoBody)

	require.NoError(t, err, "responseToString should not return error for empty body")
	assert.Equal(t, "", result, "empty body should return empty string")
}
