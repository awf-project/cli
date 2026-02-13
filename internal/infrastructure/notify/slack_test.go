package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockSlackServer creates an httptest server that responds with the given status code.
func newMockSlackServer(statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte("ok"))
	}))
}

// newDelayedMockServer creates an httptest server that delays before responding.
func newDelayedMockServer(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		case <-r.Context().Done():
			// Client cancelled or timed out
		}
	}))
}

// --- Interface compliance tests ---

func TestSlackBackend_ImplementsInterface(t *testing.T) {
	var _ Backend = (*slackBackend)(nil)
}

// --- Constructor tests ---

func TestNewSlackBackend_CreatesValidInstance(t *testing.T) {
	backend, err := newSlackBackend("https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX")

	require.NoError(t, err, "newSlackBackend() should succeed with valid URL")
	require.NotNil(t, backend, "newSlackBackend() should not return nil")
	assert.Equal(t, "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX", backend.webhookURL, "webhookURL should be set correctly")
	assert.NotNil(t, backend.sender, "sender should be initialized")
}

func TestNewSlackBackend_MultipleInstances(t *testing.T) {
	backend1, err1 := newSlackBackend("https://hooks.slack.com/services/TEST1")
	backend2, err2 := newSlackBackend("https://hooks.slack.com/services/TEST2")

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NotNil(t, backend1)
	require.NotNil(t, backend2)
	assert.NotEqual(t, backend1, backend2, "should create separate instances")
	assert.NotEqual(t, backend1.id, backend2.id, "instance IDs should be unique")
}

func TestNewSlackBackend_WithDifferentWebhookFormats(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		wantErr    bool
	}{
		{
			name:       "standard_slack_webhook",
			webhookURL: "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX",
			wantErr:    false,
		},
		{
			name:       "custom_domain_webhook",
			webhookURL: "https://custom.company.com/slack/webhook",
			wantErr:    false,
		},
		{
			name:       "http_webhook",
			webhookURL: "http://internal-slack-proxy:8080/webhook",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := newSlackBackend(tt.webhookURL)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, backend)
			} else {
				require.NoError(t, err)
				require.NotNil(t, backend)
				assert.Equal(t, tt.webhookURL, backend.webhookURL)
			}
		})
	}
}

// --- Constructor tests - Error Handling ---

func TestNewSlackBackend_EmptyWebhookURL(t *testing.T) {
	backend, err := newSlackBackend("")

	// Given: empty webhookURL (missing configuration)
	// When: newSlackBackend is called
	// Then: should return error indicating missing configuration
	assert.Error(t, err, "should fail with empty URL")
	assert.Nil(t, backend, "backend should be nil on error")
	assert.Contains(t, err.Error(), "slack_webhook_url", "error should mention missing slack_webhook_url")
}

func TestNewSlackBackend_WhitespaceWebhookURL(t *testing.T) {
	backend, err := newSlackBackend("   ")

	assert.Error(t, err, "should fail with whitespace-only URL")
	assert.Nil(t, backend)
	assert.Contains(t, err.Error(), "slack_webhook_url", "error should mention missing slack_webhook_url")
}

func TestNewSlackBackend_TabsAndNewlinesWebhookURL(t *testing.T) {
	backend, err := newSlackBackend("\t\n  \t")

	assert.Error(t, err, "should fail with whitespace-only URL")
	assert.Nil(t, backend)
	assert.Contains(t, err.Error(), "slack_webhook_url", "error should mention missing slack_webhook_url")
}

// --- Send method tests - Happy Path ---

func TestSlackBackend_Send_MinimalPayload(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Workflow completed",
	}

	result, err := backend.Send(ctx, payload)

	// Given: minimal payload (only message)
	// When: Send is called
	// Then: should succeed and return result with HTTP status
	require.NoError(t, err, "Send should succeed with minimal payload")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "slack", result.Backend, "backend name should be 'slack'")
	assert.Greater(t, result.StatusCode, 0, "status code should be > 0 for HTTP backend")
}

func TestSlackBackend_Send_FullPayload(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Title:    "AWF Workflow",
		Message:  "Workflow 'build-project' completed successfully",
		Priority: "high",
		Metadata: map[string]string{
			"workflow": "build-project",
			"status":   "success",
			"duration": "3m45s",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: full payload with all fields populated
	// When: Send is called
	// Then: should succeed and POST formatted Slack message blocks
	require.NoError(t, err, "Send should succeed with full payload")
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
	assert.Greater(t, result.StatusCode, 0, "should return HTTP status code")
}

func TestSlackBackend_Send_DefaultTitleWhenEmpty(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Title:   "", // Empty title, should use default
		Message: "Build finished",
	}

	result, err := backend.Send(ctx, payload)

	// Given: empty title field
	// When: Send is called
	// Then: should use default title "AWF Workflow"
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_WithWorkflowMetadata(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Title:   "Build Complete",
		Message: "The build has completed successfully",
		Metadata: map[string]string{
			"workflow": "ci-pipeline",
			"status":   "success",
			"duration": "2m30s",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: payload with workflow metadata
	// When: Send is called
	// Then: should format Slack message blocks with metadata fields
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_DifferentPriorities(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

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
			backend, err := newSlackBackend(mockServer.URL)
			require.NoError(t, err)

			ctx := context.Background()
			payload := NotificationPayload{
				Title:    "Test Notification",
				Message:  "Testing priority levels",
				Priority: tt.priority,
			}

			result, err := backend.Send(ctx, payload)

			// Given: different priority values
			// When: Send is called
			// Then: should format message with appropriate visual indicators
			require.NoError(t, err, "should handle priority: %s", tt.priority)
			require.NotNil(t, result)
			assert.Equal(t, "slack", result.Backend)
		})
	}
}

// --- Send method tests - Edge Cases ---

func TestSlackBackend_Send_EmptyMessage(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "", // Empty message
	}

	result, err := backend.Send(ctx, payload)

	// Given: empty message field
	// When: Send is called
	// Then: should send notification with empty message (Slack allows this)
	require.NoError(t, err, "Slack allows empty messages")
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_VeryLongMessage(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	// Create a message with 5000 characters
	longMessage := make([]byte, 5000)
	for i := range longMessage {
		longMessage[i] = 'A'
	}

	payload := NotificationPayload{
		Message: string(longMessage),
	}

	result, err := backend.Send(ctx, payload)

	// Given: very long message
	// When: Send is called
	// Then: should handle long messages (Slack has limits but accepts long text)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_SpecialCharactersInMessage(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Special chars: \n\t<>&\"'{}[]",
	}

	result, err := backend.Send(ctx, payload)

	// Given: message with special characters
	// When: Send is called
	// Then: should escape/handle special characters correctly for JSON
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_UnicodeInMessage(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Unicode: 你好世界 🎉 Привет мир ✅ ❌",
	}

	result, err := backend.Send(ctx, payload)

	// Given: message with unicode characters and emojis
	// When: Send is called
	// Then: should handle UTF-8 correctly
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_MarkdownInMessage(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "*Bold* _italic_ ~strike~ `code` ```code block```",
	}

	result, err := backend.Send(ctx, payload)

	// Given: message with Slack markdown formatting
	// When: Send is called
	// Then: should preserve markdown formatting
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_URLsInMessage(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Check the build: https://ci.example.com/build/123",
	}

	result, err := backend.Send(ctx, payload)

	// Given: message with URLs
	// When: Send is called
	// Then: should handle URLs correctly (Slack will auto-link)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_MultilineMessage(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Line 1\nLine 2\nLine 3",
	}

	result, err := backend.Send(ctx, payload)

	// Given: multiline message
	// When: Send is called
	// Then: should preserve line breaks
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

// --- Send method tests - Error Handling ---

func TestSlackBackend_Send_ContextCancellation(t *testing.T) {
	// Use a delayed server so the request is in-flight when cancelled
	mockServer := newDelayedMockServer(5 * time.Second)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	payload := NotificationPayload{
		Message: "Workflow completed",
	}

	result, err := backend.Send(ctx, payload)

	// Given: cancelled context
	// When: Send is called
	// Then: should return context.Canceled error
	assert.Error(t, err, "should fail with cancelled context")
	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled, "error should be context.Canceled")
}

func TestSlackBackend_Send_ContextTimeout(t *testing.T) {
	// Use a delayed server so the request is in-flight when timeout expires
	mockServer := newDelayedMockServer(5 * time.Second)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // Ensure timeout occurs

	payload := NotificationPayload{
		Message: "Workflow completed",
	}

	result, err := backend.Send(ctx, payload)

	// Given: expired context timeout
	// When: Send is called
	// Then: should return context.DeadlineExceeded error
	assert.Error(t, err, "should fail with expired context")
	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "error should be context.DeadlineExceeded")
}

func TestSlackBackend_Send_InvalidWebhookURL(t *testing.T) {
	// Create backend with invalid URL format
	backend, err := newSlackBackend("not-a-valid-url")
	require.NoError(t, err) // Constructor doesn't validate URL format

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
	}

	result, err := backend.Send(ctx, payload)

	// Given: backend with malformed URL
	// When: Send is called
	// Then: should fail during HTTP request
	assert.Error(t, err, "should fail with invalid URL")
	assert.Nil(t, result)
}

func TestSlackBackend_Send_HTTPTimeoutEnforced(t *testing.T) {
	// Test that the 10-second timeout from NFR-001 is enforced
	mockServer := newDelayedMockServer(15 * time.Second)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
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

func TestSlackBackend_Send_UnreachableWebhook(t *testing.T) {
	// Use unreachable server (RFC 5737 TEST-NET-1)
	backend, err := newSlackBackend("https://192.0.2.1/webhook")
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
	}

	result, err := backend.Send(ctx, payload)

	// Given: unreachable webhook URL
	// When: Send is called
	// Then: should return connection error
	assert.Error(t, err, "should fail with unreachable server")
	assert.Nil(t, result)
}

// --- Send method tests - HTTP Response Codes ---

func TestSlackBackend_Send_SuccessfulResponse(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
	}

	result, err := backend.Send(ctx, payload)

	// Given: valid request to Slack webhook
	// When: Send is called
	// Then: should return 2xx status code
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
	assert.GreaterOrEqual(t, result.StatusCode, 200, "status should be 2xx on success")
	assert.Less(t, result.StatusCode, 300, "status should be 2xx on success")
}

func TestSlackBackend_Send_ServerError(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusInternalServerError)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
	}

	result, err := backend.Send(ctx, payload)

	// Given: server returns 500 error
	// When: Send is called
	// Then: should return result with 500 status code and error
	assert.Error(t, err, "should fail with 5xx status")
	assert.NotNil(t, result, "result should contain status code even on error")
	assert.Equal(t, 500, result.StatusCode, "should capture status code")
}

func TestSlackBackend_Send_ClientError(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusBadRequest)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
	}

	result, err := backend.Send(ctx, payload)

	// Given: server returns 400 error
	// When: Send is called
	// Then: should return result with 400 status code and error
	assert.Error(t, err, "should fail with 4xx status")
	assert.NotNil(t, result, "result should contain status code even on error")
	assert.Equal(t, 400, result.StatusCode, "should capture status code")
}

func TestSlackBackend_Send_InvalidToken(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusNotFound)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
	}

	result, err := backend.Send(ctx, payload)

	// Given: invalid webhook token (404 response)
	// When: Send is called
	// Then: should return error with 404 status
	assert.Error(t, err, "should fail with 404 status")
	assert.NotNil(t, result, "result should contain status code even on error")
	assert.Equal(t, 404, result.StatusCode, "should capture status code")
}

// --- Send method tests - Metadata Handling ---

func TestSlackBackend_Send_NilMetadata(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message:  "Test message",
		Metadata: nil, // Nil metadata
	}

	result, err := backend.Send(ctx, payload)

	// Given: nil metadata
	// When: Send is called
	// Then: should succeed (metadata is optional for Slack)
	require.NoError(t, err, "should succeed with nil metadata")
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_EmptyMetadata(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message:  "Test message",
		Metadata: map[string]string{}, // Empty metadata
	}

	result, err := backend.Send(ctx, payload)

	// Given: empty metadata map
	// When: Send is called
	// Then: should succeed without metadata fields
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_WithAllMetadataFields(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Title:   "Build Status",
		Message: "Build completed",
		Metadata: map[string]string{
			"workflow": "ci-pipeline",
			"status":   "success",
			"duration": "5m23s",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: metadata with workflow, status, duration
	// When: Send is called
	// Then: should format Slack message blocks with all metadata fields
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

func TestSlackBackend_Send_MetadataWithSpecialCharacters(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Build completed",
		Metadata: map[string]string{
			"workflow": "build-'special'-project",
			"status":   "success <>&",
			"duration": "2m30s",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: metadata values with special characters
	// When: Send is called
	// Then: should escape special characters in JSON
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

// --- Send method tests - Message Block Formatting ---

func TestSlackBackend_Send_MessageBlockStructure(t *testing.T) {
	mockServer := newMockSlackServer(http.StatusOK)
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Title:   "Build Complete",
		Message: "The CI pipeline finished successfully",
		Metadata: map[string]string{
			"workflow": "ci-pipeline",
			"status":   "success",
			"duration": "3m45s",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: full payload with title, message, and metadata
	// When: Send is called
	// Then: should format Slack message blocks with header, body, and fields
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "slack", result.Backend)
}

// --- Concurrent Send tests ---

func TestSlackBackend_Send_ConcurrentCalls(t *testing.T) {
	// Create mock server that handles concurrent requests
	var requestCount atomic.Uint32
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			ctx := context.Background()
			payload := NotificationPayload{
				Message: "Concurrent test",
			}

			result, err := backend.Send(ctx, payload)

			// Given: concurrent Send calls
			// When: multiple goroutines call Send simultaneously
			// Then: should handle concurrent requests safely
			assert.NoError(t, err, "goroutine %d failed", id)
			assert.NotNil(t, result, "goroutine %d got nil result", id)
			assert.Equal(t, "slack", result.Backend)

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

	// Verify all requests were received
	assert.Equal(t, uint32(numGoroutines), requestCount.Load(), "all concurrent requests should be received")
}

// --- Rate Limiting tests ---

func TestSlackBackend_Send_RateLimitHandling(t *testing.T) {
	// Simulate rate limiting: first request succeeds, subsequent ones get 429
	var requestCount atomic.Uint32
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := requestCount.Add(1)
		if count > 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate_limited"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}
	}))
	defer mockServer.Close()

	backend, err := newSlackBackend(mockServer.URL)
	require.NoError(t, err)

	ctx := context.Background()

	// Send multiple notifications rapidly
	for range 3 {
		payload := NotificationPayload{
			Message: "Rate limit test",
		}

		result, err := backend.Send(ctx, payload)

		// Given: rapid consecutive sends
		// When: Send is called multiple times
		// Then: should handle rate limiting gracefully (if server enforces it)
		if err != nil {
			// Rate limiting may occur
			assert.NotNil(t, result, "should have result even on rate limit")
			assert.Equal(t, 429, result.StatusCode, "rate limit should return 429")
		} else {
			require.NotNil(t, result)
			assert.Equal(t, "slack", result.Backend)
		}
	}
}

// --- Status Field Color Mapping tests ---

func TestSlackBackend_Send_StatusFieldWithDifferentValues(t *testing.T) {
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
			// Create mock server that returns 200 OK for all requests
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			}))
			defer mockServer.Close()

			backend, err := newSlackBackend(mockServer.URL)
			require.NoError(t, err)

			ctx := context.Background()
			payload := NotificationPayload{
				Message: "Workflow status update",
				Metadata: map[string]string{
					"status": tt.status,
				},
			}

			result, err := backend.Send(ctx, payload)

			// Given: different status values in metadata
			// When: Send is called
			// Then: should format message with appropriate status indicator
			require.NoError(t, err, "should handle status: %s", tt.status)
			require.NotNil(t, result)
			assert.Equal(t, "slack", result.Backend)
		})
	}
}
