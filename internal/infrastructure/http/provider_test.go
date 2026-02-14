package http

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
	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/pkg/httputil"
)

// --- Mock Logger ---

type mockLogger struct {
	messages []string
}

func (m *mockLogger) Debug(msg string, fields ...any) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogger) Info(msg string, fields ...any) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogger) Warn(msg string, fields ...any) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogger) Error(msg string, fields ...any) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

// --- Mock HTTPDoer ---

type mockHTTPDoer struct {
	response *http.Response
	err      error
	requests []*http.Request
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)
	return m.response, m.err
}

// --- Interface compliance tests ---

func TestHTTPOperationProvider_ImplementsInterface(t *testing.T) {
	var _ ports.OperationProvider = (*HTTPOperationProvider)(nil)
}

// --- Constructor tests ---

func TestNewHTTPOperationProvider(t *testing.T) {
	client := httputil.NewClient()
	logger := &mockLogger{}

	provider := NewHTTPOperationProvider(client, logger)

	require.NotNil(t, provider, "NewHTTPOperationProvider() should not return nil")
	assert.Len(t, provider.operations, len(AllOperations()), "operations map should be initialized with all operations")
}

func TestNewHTTPOperationProvider_RegistersAllOperations(t *testing.T) {
	client := httputil.NewClient()
	logger := &mockLogger{}

	provider := NewHTTPOperationProvider(client, logger)

	expectedOps := AllOperations()
	assert.Len(t, provider.operations, len(expectedOps), "should register all operations")

	for _, expectedOp := range expectedOps {
		registeredOp, found := provider.operations[expectedOp.Name]
		require.True(t, found, "operation %s should be registered", expectedOp.Name)
		assert.Equal(t, expectedOp.Name, registeredOp.Name, "operation name should match")
	}
}

// --- GetOperation tests ---

func TestHTTPOperationProvider_GetOperation_HTTPRequest(t *testing.T) {
	provider := newTestProvider()

	op, found := provider.GetOperation("http.request")

	require.True(t, found, "http.request should be found")
	require.NotNil(t, op, "operation should not be nil")
	assert.Equal(t, "http.request", op.Name)
	assert.Contains(t, op.Description, "HTTP request")
}

func TestHTTPOperationProvider_GetOperation_NonExistent(t *testing.T) {
	provider := newTestProvider()

	tests := []struct {
		name          string
		operationName string
	}{
		{"unknown_operation", "http.unknown"},
		{"empty_name", ""},
		{"wrong_namespace", "github.create_pr"},
		{"partial_match", "http"},
		{"case_mismatch", "HTTP.Request"},
		{"typo", "http.requestt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, found := provider.GetOperation(tt.operationName)

			assert.False(t, found, "operation %s should not be found", tt.operationName)
			assert.Nil(t, op, "operation should be nil when not found")
		})
	}
}

func TestHTTPOperationProvider_GetOperation_ValidatesSchema(t *testing.T) {
	provider := newTestProvider()

	op, found := provider.GetOperation("http.request")
	require.True(t, found)

	// Verify required inputs
	urlInput := op.Inputs["url"]
	assert.True(t, urlInput.Required, "url should be required")
	assert.Equal(t, plugin.InputTypeString, urlInput.Type)

	methodInput := op.Inputs["method"]
	assert.True(t, methodInput.Required, "method should be required")
	assert.Equal(t, plugin.InputTypeString, methodInput.Type)

	// Verify optional inputs
	headersInput := op.Inputs["headers"]
	assert.False(t, headersInput.Required, "headers should be optional")
	assert.Equal(t, plugin.InputTypeObject, headersInput.Type)

	bodyInput := op.Inputs["body"]
	assert.False(t, bodyInput.Required, "body should be optional")
	assert.Equal(t, plugin.InputTypeString, bodyInput.Type)

	timeoutInput := op.Inputs["timeout"]
	assert.False(t, timeoutInput.Required, "timeout should be optional")
	assert.Equal(t, plugin.InputTypeInteger, timeoutInput.Type)
	assert.Equal(t, 30, timeoutInput.Default, "default timeout should be 30 seconds")

	retryableInput := op.Inputs["retryable_status_codes"]
	assert.False(t, retryableInput.Required, "retryable_status_codes should be optional")
	assert.Equal(t, plugin.InputTypeArray, retryableInput.Type)

	// Verify outputs
	expectedOutputs := []string{"status_code", "body", "headers", "body_truncated"}
	assert.Equal(t, expectedOutputs, op.Outputs, "outputs should match schema")
}

// --- ListOperations tests ---

func TestHTTPOperationProvider_ListOperations(t *testing.T) {
	provider := newTestProvider()

	ops := provider.ListOperations()

	require.Len(t, ops, 1, "should return 1 operation")
	assert.Equal(t, "http.request", ops[0].Name)
}

// --- Execute tests - Happy path ---

func TestHTTPOperationProvider_Execute_GET_Success(t *testing.T) {
	// Setup httptest server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "test-123")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"success"}`))
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL + "/test",
		"method": "GET",
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	// Assertions - these will FAIL against the stub (which returns Success=false, Error="not implemented")
	require.NoError(t, err, "Execute should not return error")
	assert.True(t, result.Success, "result should be successful")
	assert.Empty(t, result.Error, "error should be empty on success")

	// Verify outputs
	assert.Equal(t, 200, result.Outputs["status_code"], "status_code should be 200")
	assert.Equal(t, `{"message":"success"}`, result.Outputs["body"], "body should match response")

	headers, ok := result.Outputs["headers"].(map[string]string)
	require.True(t, ok, "headers should be map[string]string")
	assert.Equal(t, "application/json", headers["Content-Type"], "Content-Type header should be captured")
	assert.Equal(t, "test-123", headers["X-Request-Id"], "custom header should be captured")

	assert.False(t, result.Outputs["body_truncated"].(bool), "body should not be truncated")
}

func TestHTTPOperationProvider_Execute_POST_WithBodyAndHeaders(t *testing.T) {
	// Setup httptest server
	var receivedBody string
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		receivedHeaders = r.Header
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL + "/users",
		"method": "POST",
		"headers": map[string]any{
			"Content-Type":  "application/json",
			"Authorization": "Bearer secret-token",
		},
		"body": `{"name":"John Doe"}`,
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify server received correct request
	assert.Equal(t, `{"name":"John Doe"}`, receivedBody, "server should receive request body")
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
	assert.Equal(t, "Bearer secret-token", receivedHeaders.Get("Authorization"))

	// Verify response
	assert.Equal(t, 201, result.Outputs["status_code"])
	assert.Equal(t, `{"id":42}`, result.Outputs["body"])
}

func TestHTTPOperationProvider_Execute_PUT_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL + "/resource/1",
		"method": "PUT",
		"body":   `{"status":"updated"}`,
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 204, result.Outputs["status_code"])
	assert.Equal(t, "", result.Outputs["body"], "204 response should have empty body")
}

func TestHTTPOperationProvider_Execute_DELETE_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"deleted":true}`))
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL + "/resource/1",
		"method": "DELETE",
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 200, result.Outputs["status_code"])
	assert.Equal(t, `{"deleted":true}`, result.Outputs["body"])
}

// --- Execute tests - Method variations ---

func TestHTTPOperationProvider_Execute_MethodCaseInsensitive(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"lowercase_get", "get"},
		{"uppercase_GET", "GET"},
		{"mixedcase_GeT", "GeT"},
		{"lowercase_post", "post"},
		{"uppercase_POST", "POST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Method should be normalized to uppercase
				assert.Contains(t, []string{"GET", "POST"}, r.Method)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			provider := newTestProvider()
			inputs := map[string]any{
				"url":    server.URL,
				"method": tt.method,
			}

			result, err := provider.Execute(context.Background(), "http.request", inputs)

			require.NoError(t, err)
			assert.True(t, result.Success)
		})
	}
}

// --- Execute tests - Edge cases ---

func TestHTTPOperationProvider_Execute_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		assert.Empty(t, bodyBytes, "body should be empty")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL,
		"method": "POST",
		"body":   "",
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestHTTPOperationProvider_Execute_NoHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL,
		"method": "GET",
		// No headers specified
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.NotNil(t, result.Outputs["headers"], "headers output should exist even if empty")
}

func TestHTTPOperationProvider_Execute_MultiValueHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send multi-value header
		w.Header().Add("Set-Cookie", "session=abc123")
		w.Header().Add("Set-Cookie", "tracking=xyz789")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL,
		"method": "GET",
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)

	headers := result.Outputs["headers"].(map[string]string)
	// Multi-value headers should be joined with ", " per HTTP spec
	assert.Contains(t, headers["Set-Cookie"], "session=abc123")
	assert.Contains(t, headers["Set-Cookie"], "tracking=xyz789")
	assert.Contains(t, headers["Set-Cookie"], ", ", "multi-value headers should be joined with comma-space")
}

func TestHTTPOperationProvider_Execute_HeaderCanonicalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/plain")
		w.Header().Set("x-custom-header", "value")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL,
		"method": "GET",
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	headers := result.Outputs["headers"].(map[string]string)

	// Headers should be canonicalized (Content-Type not content-type)
	_, hasContentType := headers["Content-Type"]
	assert.True(t, hasContentType, "headers should be canonicalized to Content-Type")
}

func TestHTTPOperationProvider_Execute_LargeResponseBody(t *testing.T) {
	// Create response larger than 1MB
	largeBody := strings.Repeat("x", 1024*1024+1000) // 1MB + 1000 bytes

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largeBody))
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL,
		"method": "GET",
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)

	// Body should be truncated to 1MB
	bodyStr := result.Outputs["body"].(string)
	assert.Equal(t, 1024*1024, len(bodyStr), "body should be truncated to 1MB")
	assert.True(t, result.Outputs["body_truncated"].(bool), "body_truncated should be true")
}

// --- Execute tests - Error handling ---

func TestHTTPOperationProvider_Execute_UnknownOperation(t *testing.T) {
	provider := newTestProvider()
	inputs := map[string]any{
		"url":    "http://example.com",
		"method": "GET",
	}

	_, err := provider.Execute(context.Background(), "http.unknown", inputs)

	// Should return error for unknown operation
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operation")
}

func TestHTTPOperationProvider_Execute_MissingURL(t *testing.T) {
	provider := newTestProvider()
	inputs := map[string]any{
		"method": "GET",
		// url missing
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	// Should fail - either error or Success=false
	if err == nil {
		assert.False(t, result.Success, "should fail when url is missing")
		assert.NotEmpty(t, result.Error, "error message should explain missing url")
	}
}

func TestHTTPOperationProvider_Execute_InvalidMethod(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"unsupported_PATCH", "PATCH"},
		{"unsupported_HEAD", "HEAD"},
		{"invalid_INVALID", "INVALID"},
		{"empty_method", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newTestProvider()
			inputs := map[string]any{
				"url":    "http://example.com",
				"method": tt.method,
			}

			result, err := provider.Execute(context.Background(), "http.request", inputs)

			// Should fail - either error or Success=false with error message
			if err == nil {
				assert.False(t, result.Success, "should fail for invalid method %s", tt.method)
				assert.Contains(t, result.Error, "method", "error should mention invalid method")
			} else {
				assert.Contains(t, err.Error(), "method", "error should mention invalid method")
			}
		})
	}
}

func TestHTTPOperationProvider_Execute_InvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"no_scheme", "example.com"},
		{"ftp_scheme", "ftp://example.com"},
		{"empty_url", ""},
		{"malformed", "ht!tp://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newTestProvider()
			inputs := map[string]any{
				"url":    tt.url,
				"method": "GET",
			}

			result, err := provider.Execute(context.Background(), "http.request", inputs)

			// Should fail - either error or Success=false
			if err == nil {
				assert.False(t, result.Success, "should fail for invalid URL %s", tt.url)
				assert.Contains(t, result.Error, "url", "error should mention invalid URL")
			} else {
				assert.Contains(t, err.Error(), "url", "error should mention invalid URL")
			}
		})
	}
}

func TestHTTPOperationProvider_Execute_ConnectionError(t *testing.T) {
	// Use a mock HTTPDoer that returns connection error
	mockDoer := &mockHTTPDoer{
		err: errors.New("connection refused"),
	}

	client := httputil.NewClient(httputil.WithDoer(mockDoer))
	provider := NewHTTPOperationProvider(client, &mockLogger{})

	inputs := map[string]any{
		"url":    "http://unreachable.invalid:12345",
		"method": "GET",
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	// Should fail with descriptive error
	if err == nil {
		assert.False(t, result.Success, "should fail on connection error")
		assert.NotEmpty(t, result.Error, "should have error message")
	} else {
		assert.Contains(t, err.Error(), "connection", "error should mention connection failure")
	}
}

func TestHTTPOperationProvider_Execute_TimeoutError(t *testing.T) {
	// GIVEN: Server delays 2s, operation timeout is 1s
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":     server.URL,
		"method":  "GET",
		"timeout": 1, // 1 second — shorter than server's 2s delay
	}

	// WHEN: Execute with timeout shorter than server delay
	result, err := provider.Execute(context.Background(), "http.request", inputs)

	// THEN: Fails with timeout error
	if err == nil {
		assert.False(t, result.Success, "should fail on timeout")
		assert.Contains(t, result.Error, "timeout", "error should mention timeout")
	} else {
		assert.Contains(t, err.Error(), "timeout", "error should mention timeout")
	}
}

func TestHTTPOperationProvider_Execute_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL,
		"method": "GET",
	}

	// Create context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "http.request", inputs)

	// Should fail due to cancelled context
	if err == nil {
		assert.False(t, result.Success, "should fail when context is cancelled")
	} else {
		assert.True(t, errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled"))
	}
}

// --- Execute tests - Retryable status codes ---

func TestHTTPOperationProvider_Execute_RetryableStatusCode_503(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // 503
		_, _ = w.Write([]byte("Service temporarily unavailable"))
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":                    server.URL,
		"method":                 "GET",
		"retryable_status_codes": []any{429, 502, 503},
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	// Should return Success=false with error indicating retryable failure
	require.NoError(t, err, "should not return Go error, but operation result with Success=false")
	assert.False(t, result.Success, "should be marked as failure for retryable status")
	assert.Contains(t, result.Error, "503", "error should mention status code")
	assert.Contains(t, result.Error, "retryable", "error should indicate retryable failure")

	// Outputs should still capture the response
	assert.Equal(t, 503, result.Outputs["status_code"])
	assert.NotEmpty(t, result.Outputs["body"])
}

func TestHTTPOperationProvider_Execute_RetryableStatusCode_429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests) // 429
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":                    server.URL,
		"method":                 "POST",
		"retryable_status_codes": []any{429},
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	assert.False(t, result.Success, "429 should be treated as retryable failure")
	assert.Contains(t, result.Error, "429")
	assert.Equal(t, 429, result.Outputs["status_code"])
}

func TestHTTPOperationProvider_Execute_NonRetryableStatusCode_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // 404
		_, _ = w.Write([]byte("Not found"))
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":                    server.URL,
		"method":                 "GET",
		"retryable_status_codes": []any{429, 503}, // 404 not in list
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	// 404 is not retryable, but it's a successful HTTP transaction
	// Depending on implementation, this might be Success=true with status_code=404
	// or Success=false for 4xx errors
	require.NoError(t, err)
	assert.Equal(t, 404, result.Outputs["status_code"])
	assert.Equal(t, "Not found", result.Outputs["body"])

	// If Success=false, error should NOT mention "retryable"
	if !result.Success {
		assert.NotContains(t, result.Error, "retryable", "404 should not be marked as retryable")
	}
}

func TestHTTPOperationProvider_Execute_NoRetryableCodesSpecified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // 503
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL,
		"method": "GET",
		// No retryable_status_codes specified
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	// Without retryable_status_codes, 503 should be treated normally (not as retryable)
	require.NoError(t, err)
	assert.Equal(t, 503, result.Outputs["status_code"])

	// If marked as failure, should NOT mention "retryable"
	if !result.Success {
		assert.NotContains(t, result.Error, "retryable")
	}
}

// --- Execute tests - Timeout configuration ---

func TestHTTPOperationProvider_Execute_CustomTimeout(t *testing.T) {
	// This test verifies timeout input is respected
	// We test that a longer timeout allows a delayed server response to succeed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay less than the configured timeout
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":     server.URL,
		"method":  "GET",
		"timeout": 5, // 5 seconds - long enough for the 100ms delay
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success, "should succeed with sufficient timeout")
}

func TestHTTPOperationProvider_Execute_DefaultTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newTestProvider()
	inputs := map[string]any{
		"url":    server.URL,
		"method": "GET",
		// No timeout specified - should use default 30s
	}

	result, err := provider.Execute(context.Background(), "http.request", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)
	// Default timeout behavior is tested in httputil package
}

// --- Helper functions ---

func newTestProvider() *HTTPOperationProvider {
	client := httputil.NewClient()
	logger := &mockLogger{}
	return NewHTTPOperationProvider(client, logger)
}
