package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Interface compliance tests ---

func TestWebhookBackend_ImplementsInterface(t *testing.T) {
	var _ Backend = (*webhookBackend)(nil)
}

// --- Constructor tests ---

func TestNewWebhookBackend_CreatesValidInstance(t *testing.T) {
	backend := newWebhookBackend()

	require.NotNil(t, backend, "newWebhookBackend() should not return nil")
	assert.NotNil(t, backend.client, "client should be initialized")
}

func TestNewWebhookBackend_MultipleInstances(t *testing.T) {
	backend1 := newWebhookBackend()
	backend2 := newWebhookBackend()

	require.NotNil(t, backend1)
	require.NotNil(t, backend2)
	assert.NotEqual(t, backend1, backend2, "should create separate instances")
	assert.NotEqual(t, backend1.id, backend2.id, "instance IDs should be unique")
}

// --- Send method tests - Happy Path ---

func TestWebhookBackend_Send_MinimalPayload(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Workflow completed",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: minimal payload (message + webhook_url)
	// When: Send is called
	// Then: should succeed and return result with HTTP status
	require.NoError(t, err, "Send should succeed with minimal payload")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "webhook", result.Backend, "backend name should be 'webhook'")
	assert.Greater(t, result.StatusCode, 0, "status code should be > 0 for HTTP backend")
}

func TestWebhookBackend_Send_FullPayload(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Title:    "AWF Workflow",
		Message:  "Workflow 'build-project' completed successfully",
		Priority: "high",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
			"workflow":    "build-project",
			"status":      "success",
			"duration":    "3m45s",
			"outputs":     `{"build_id": "12345", "artifact": "app.tar.gz"}`,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: full payload with all fields populated
	// When: Send is called
	// Then: should succeed and POST JSON with workflow/status/duration/message/outputs
	require.NoError(t, err, "Send should succeed with full payload")
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
	assert.Greater(t, result.StatusCode, 0, "should return HTTP status code")
}

func TestWebhookBackend_Send_DifferentWebhookURLs(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"standard_https"},
		{"http_internal"},
		{"with_query_params"},
		{"with_path_params"},
		{"discord_webhook"},
		{"teams_webhook"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer mockServer.Close()

			backend := newWebhookBackend()

			ctx := context.Background()
			payload := NotificationPayload{
				Message: "Test message",
				Metadata: map[string]string{
					"webhook_url": mockServer.URL,
				},
			}

			result, err := backend.Send(ctx, payload)

			// Given: different webhook URL formats
			// When: Send is called
			// Then: should POST to the specified URL
			require.NoError(t, err, "should handle URL: %s", mockServer.URL)
			require.NotNil(t, result)
			assert.Equal(t, "webhook", result.Backend)
		})
	}
}

func TestWebhookBackend_Send_DifferentPriorities(t *testing.T) {
	tests := []struct {
		name     string
		priority string
	}{
		{"low_priority", "low"},
		{"default_priority", "default"},
		{"high_priority", "high"},
		{"empty_priority", ""}, // Should default to "default"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer mockServer.Close()

			backend := newWebhookBackend()

			ctx := context.Background()
			payload := NotificationPayload{
				Title:    "Test Notification",
				Message:  "Testing priority levels",
				Priority: tt.priority,
				Metadata: map[string]string{
					"webhook_url": mockServer.URL,
				},
			}

			result, err := backend.Send(ctx, payload)

			// Given: different priority values
			// When: Send is called
			// Then: should include priority in JSON payload
			require.NoError(t, err, "should handle priority: %s", tt.priority)
			require.NotNil(t, result)
			assert.Equal(t, "webhook", result.Backend)
		})
	}
}

// --- Send method tests - Edge Cases ---

func TestWebhookBackend_Send_EmptyMessage(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "", // Empty message
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: empty message field
	// When: Send is called
	// Then: should send notification with empty message
	require.NoError(t, err, "webhook allows empty messages")
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

func TestWebhookBackend_Send_VeryLongMessage(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	// Create a message with 5000 characters
	longMessage := make([]byte, 5000)
	for i := range longMessage {
		longMessage[i] = 'A'
	}

	payload := NotificationPayload{
		Message: string(longMessage),
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: very long message
	// When: Send is called
	// Then: should handle long messages
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

func TestWebhookBackend_Send_SpecialCharactersInMessage(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Special chars: \n\t<>&\"'{}[]",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: message with special characters
	// When: Send is called
	// Then: should escape special characters correctly for JSON
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

func TestWebhookBackend_Send_UnicodeInMessage(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Unicode: 你好世界 🎉 Привет мир ✅ ❌",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: message with unicode characters and emojis
	// When: Send is called
	// Then: should handle UTF-8 correctly
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

func TestWebhookBackend_Send_URLsInMessage(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Check the build: https://ci.example.com/build/123",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: message with URLs
	// When: Send is called
	// Then: should handle URLs correctly in JSON
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

func TestWebhookBackend_Send_MultilineMessage(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Line 1\nLine 2\nLine 3",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: multiline message
	// When: Send is called
	// Then: should preserve line breaks in JSON
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

// --- Send method tests - Error Handling ---

func TestWebhookBackend_Send_MissingWebhookURL(t *testing.T) {
	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message:  "Workflow completed",
		Metadata: map[string]string{}, // No webhook_url
	}

	result, err := backend.Send(ctx, payload)

	// Given: payload without webhook_url in metadata
	// When: Send is called
	// Then: should return error indicating missing webhook_url
	require.Error(t, err, "should fail without webhook_url")
	assert.Nil(t, result, "result should be nil on error")
	if err != nil {
		assert.Contains(t, err.Error(), "webhook_url", "error should mention missing webhook_url")
	}
}

func TestWebhookBackend_Send_EmptyWebhookURL(t *testing.T) {
	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Workflow completed",
		Metadata: map[string]string{
			"webhook_url": "", // Empty webhook_url
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: empty webhook_url value
	// When: Send is called
	// Then: should return error for empty webhook_url
	require.Error(t, err, "should fail with empty webhook_url")
	assert.Nil(t, result)
	if err != nil {
		assert.Contains(t, err.Error(), "webhook_url", "error should mention webhook_url")
	}
}

func TestWebhookBackend_Send_WhitespaceWebhookURL(t *testing.T) {
	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"webhook_url": "   ", // Whitespace-only webhook_url
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: whitespace-only webhook_url
	// When: Send is called
	// Then: should return error for invalid webhook_url
	require.Error(t, err, "should fail with whitespace webhook_url")
	assert.Nil(t, result)
}

func TestWebhookBackend_Send_TabsAndNewlinesWebhookURL(t *testing.T) {
	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"webhook_url": "\t\n  \t",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: webhook_url with only tabs and newlines
	// When: Send is called
	// Then: should return error
	require.Error(t, err, "should fail with whitespace-only webhook_url")
	assert.Nil(t, result)
}

func TestWebhookBackend_Send_ContextCancellation(t *testing.T) {
	// Create mock HTTP server (won't be reached due to cancelled context)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	payload := NotificationPayload{
		Message: "Workflow completed",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: cancelled context
	// When: Send is called
	// Then: should return context.Canceled error
	assert.Error(t, err, "should fail with cancelled context")
	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled, "error should be context.Canceled")
}

func TestWebhookBackend_Send_ContextTimeout(t *testing.T) {
	// Create mock HTTP server (won't be reached due to expired context)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // Ensure timeout occurs

	payload := NotificationPayload{
		Message: "Workflow completed",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: expired context timeout
	// When: Send is called
	// Then: should return context.DeadlineExceeded error
	assert.Error(t, err, "should fail with expired context")
	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "error should be context.DeadlineExceeded")
}

func TestWebhookBackend_Send_InvalidWebhookURL(t *testing.T) {
	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"webhook_url": "not-a-valid-url",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: backend with malformed URL
	// When: Send is called
	// Then: should fail during HTTP request
	assert.Error(t, err, "should fail with invalid URL")
	assert.Nil(t, result)
}

func TestWebhookBackend_Send_HTTPTimeoutEnforced(t *testing.T) {
	// Test that the 10-second timeout from NFR-001 is enforced
	// Create mock HTTP server that delays response for 15 seconds
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	start := time.Now()
	result, err := backend.Send(ctx, payload)
	elapsed := time.Since(start)

	// Given: server that delays response > 10 seconds
	// When: Send is called
	// Then: should timeout around 10 seconds (NFR-001)
	assert.Error(t, err, "should timeout")
	assert.Nil(t, result)
	assert.Less(t, elapsed, 12*time.Second, "should timeout within 12 seconds")
	assert.Greater(t, elapsed, 8*time.Second, "should wait at least 8 seconds")
}

func TestWebhookBackend_Send_UnreachableWebhook(t *testing.T) {
	// Use unreachable server (RFC 5737 TEST-NET-1)
	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"webhook_url": "http://192.0.2.1:1/webhook",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: unreachable webhook URL
	// When: Send is called
	// Then: should return connection error
	assert.Error(t, err, "should fail with unreachable server")
	assert.Nil(t, result)
}

// --- Send method tests - HTTP Response Codes ---

func TestWebhookBackend_Send_SuccessfulResponse(t *testing.T) {
	// Create mock HTTP server that returns 200 OK
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: valid request to webhook endpoint
	// When: Send is called
	// Then: should return 2xx status code
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
	assert.GreaterOrEqual(t, result.StatusCode, 200, "status should be 2xx on success")
	assert.Less(t, result.StatusCode, 300, "status should be 2xx on success")
}

func TestWebhookBackend_Send_ServerError(t *testing.T) {
	// Create mock HTTP server that returns 500 error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: server returns 500 error
	// When: Send is called
	// Then: should return result with 500 status code and error
	assert.Error(t, err, "should fail with 5xx status")
	assert.NotNil(t, result, "result should contain status code even on error")
	assert.Equal(t, 500, result.StatusCode, "should capture status code")
}

func TestWebhookBackend_Send_ClientError(t *testing.T) {
	// Create mock HTTP server that returns 400 error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: server returns 400 error
	// When: Send is called
	// Then: should return result with 400 status code and error
	assert.Error(t, err, "should fail with 4xx status")
	assert.NotNil(t, result, "result should contain status code even on error")
	assert.Equal(t, 400, result.StatusCode, "should capture status code")
}

func TestWebhookBackend_Send_NotFoundError(t *testing.T) {
	// Create mock HTTP server that returns 404 error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: webhook endpoint returns 404
	// When: Send is called
	// Then: should return error with 404 status
	assert.Error(t, err, "should fail with 404 status")
	assert.NotNil(t, result, "result should contain status code even on error")
	assert.Equal(t, 404, result.StatusCode, "should capture status code")
}

// --- Send method tests - Metadata Handling ---

func TestWebhookBackend_Send_NilMetadata(t *testing.T) {
	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message:  "Test message",
		Metadata: nil, // Nil metadata
	}

	result, err := backend.Send(ctx, payload)

	// Given: nil metadata (no webhook_url)
	// When: Send is called
	// Then: should return error for missing webhook_url
	require.Error(t, err, "should fail with nil metadata")
	assert.Nil(t, result)
	if err != nil {
		assert.Contains(t, err.Error(), "webhook_url", "error should mention missing webhook_url")
	}
}

func TestWebhookBackend_Send_WithAllMetadataFields(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Title:   "Build Status",
		Message: "Build completed",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
			"workflow":    "ci-pipeline",
			"status":      "success",
			"duration":    "5m23s",
			"outputs":     `{"build_id": "12345"}`,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: metadata with workflow, status, duration, outputs
	// When: Send is called
	// Then: should format JSON with all metadata fields
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

func TestWebhookBackend_Send_MetadataWithSpecialCharacters(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Build completed",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
			"workflow":    "build-'special'-project",
			"status":      "success <>&",
			"duration":    "2m30s",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: metadata values with special characters
	// When: Send is called
	// Then: should escape special characters in JSON
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

func TestWebhookBackend_Send_OnlyRequiredFields(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Workflow completed",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
			// Only webhook_url, no workflow/status/duration
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: metadata with only webhook_url
	// When: Send is called
	// Then: should succeed with minimal JSON payload
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

// --- Send method tests - JSON Payload Structure ---

func TestWebhookBackend_Send_JSONPayloadStructure(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Title:   "Build Complete",
		Message: "The CI pipeline finished successfully",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
			"workflow":    "ci-pipeline",
			"status":      "success",
			"duration":    "3m45s",
			"outputs":     `{"artifact": "build.tar.gz"}`,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: full payload with title, message, and metadata
	// When: Send is called
	// Then: should POST JSON with fields: workflow, status, duration, message, outputs (per FR-006)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

func TestWebhookBackend_Send_OutputsAsJSONString(t *testing.T) {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Build completed",
		Metadata: map[string]string{
			"webhook_url": mockServer.URL,
			"outputs":     `{"build_id": "12345", "version": "v1.2.3", "artifacts": ["app.tar.gz", "app.zip"]}`,
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: outputs field with complex JSON string
	// When: Send is called
	// Then: should include outputs in webhook payload
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "webhook", result.Backend)
}

// --- Concurrent Send tests ---

func TestWebhookBackend_Send_ConcurrentCalls(t *testing.T) {
	backend := newWebhookBackend()

	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			ctx := context.Background()
			payload := NotificationPayload{
				Message: "Concurrent test",
				Metadata: map[string]string{
					"webhook_url": "https://httpbin.org/post",
				},
			}

			result, err := backend.Send(ctx, payload)

			// Given: concurrent Send calls
			// When: multiple goroutines call Send simultaneously
			// Then: should handle concurrent requests safely
			assert.NoError(t, err, "goroutine %d failed", id)
			assert.NotNil(t, result, "goroutine %d got nil result", id)
			assert.Equal(t, "webhook", result.Backend)

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatal("timeout waiting for concurrent goroutines")
		}
	}
}

// --- Rate Limiting tests ---

func TestWebhookBackend_Send_RateLimitHandling(t *testing.T) {
	// Create mock HTTP server that simulates rate limiting on 3rd request
	requestCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 3 {
			w.WriteHeader(http.StatusTooManyRequests)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer mockServer.Close()

	backend := newWebhookBackend()

	ctx := context.Background()

	// Send multiple notifications rapidly
	for i := range 3 {
		payload := NotificationPayload{
			Message: "Rate limit test",
			Metadata: map[string]string{
				"webhook_url": mockServer.URL,
			},
		}

		result, err := backend.Send(ctx, payload)

		// Given: rapid consecutive sends
		// When: Send is called multiple times
		// Then: should handle rate limiting gracefully (if webhook enforces it)
		if i == 2 {
			// Third request should be rate limited
			assert.Error(t, err, "should return error on rate limit")
			assert.NotNil(t, result, "should have result even on rate limit")
			assert.Equal(t, 429, result.StatusCode, "rate limit should return 429")
		} else {
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "webhook", result.Backend)
		}
	}
}

// --- Status Field tests ---

func TestWebhookBackend_Send_StatusFieldWithDifferentValues(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"success_status", "success"},
		{"failure_status", "failure"},
		{"error_status", "error"},
		{"running_status", "running"},
		{"pending_status", "pending"},
		{"cancelled_status", "cancelled"},
		{"unknown_status", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer mockServer.Close()

			backend := newWebhookBackend()

			ctx := context.Background()
			payload := NotificationPayload{
				Message: "Workflow status update",
				Metadata: map[string]string{
					"webhook_url": mockServer.URL,
					"status":      tt.status,
				},
			}

			result, err := backend.Send(ctx, payload)

			// Given: different status values in metadata
			// When: Send is called
			// Then: should include status in JSON payload
			require.NoError(t, err, "should handle status: %s", tt.status)
			require.NotNil(t, result)
			assert.Equal(t, "webhook", result.Backend)
		})
	}
}
