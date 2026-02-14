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

func TestNtfyBackend_ImplementsInterface(t *testing.T) {
	var _ Backend = (*ntfyBackend)(nil)
}

// --- Constructor tests ---

func TestNewNtfyBackend_CreatesValidInstance(t *testing.T) {
	backend, err := newNtfyBackend("https://ntfy.sh")

	require.NoError(t, err, "newNtfyBackend() should succeed with valid URL")
	require.NotNil(t, backend, "newNtfyBackend() should not return nil")
	assert.Equal(t, "https://ntfy.sh", backend.baseURL, "baseURL should be set correctly")
	assert.NotNil(t, backend.client, "client should be initialized")
}

func TestNewNtfyBackend_MultipleInstances(t *testing.T) {
	backend1, err1 := newNtfyBackend("https://ntfy.sh")
	backend2, err2 := newNtfyBackend("https://ntfy.sh")

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NotNil(t, backend1)
	require.NotNil(t, backend2)
	assert.NotEqual(t, backend1, backend2, "should create separate instances")
	assert.NotEqual(t, backend1.id, backend2.id, "instance IDs should be unique")
}

func TestNewNtfyBackend_WithCustomServer(t *testing.T) {
	customURL := "https://ntfy.example.com"
	backend, err := newNtfyBackend(customURL)

	require.NoError(t, err)
	require.NotNil(t, backend)
	assert.Equal(t, customURL, backend.baseURL, "should support custom ntfy servers")
}

// --- Constructor tests - Error Handling ---

func TestNewNtfyBackend_EmptyURL(t *testing.T) {
	backend, err := newNtfyBackend("")

	// Given: empty baseURL (missing configuration)
	// When: newNtfyBackend is called
	// Then: should return error indicating missing configuration
	assert.Error(t, err, "should fail with empty URL")
	assert.Nil(t, backend, "backend should be nil on error")
	assert.Contains(t, err.Error(), "ntfy_url", "error should mention missing ntfy_url")
}

func TestNewNtfyBackend_WhitespaceURL(t *testing.T) {
	backend, err := newNtfyBackend("   ")

	assert.Error(t, err, "should fail with whitespace-only URL")
	assert.Nil(t, backend)
	assert.Contains(t, err.Error(), "ntfy_url", "error should mention missing ntfy_url")
}

// --- Send method tests - Happy Path ---

func TestNtfyBackend_Send_MinimalPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method, "should use POST method")
		assert.Equal(t, "/my-builds", r.URL.Path, "should POST to topic path")
		assert.Equal(t, "AWF Workflow", r.Header.Get("Title"), "should use default title")
		assert.Equal(t, "default", r.Header.Get("Priority"), "should use default priority")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"abc123"}`))
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Workflow completed",
		Metadata: map[string]string{
			"topic": "my-builds",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: minimal payload (message + topic)
	// When: Send is called
	// Then: should succeed and return result with HTTP status
	require.NoError(t, err, "Send should succeed with minimal payload")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "ntfy", result.Backend, "backend name should be 'ntfy'")
	assert.Equal(t, http.StatusOK, result.StatusCode, "status code should be 200")
}

func TestNtfyBackend_Send_FullPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method, "should use POST method")
		assert.Equal(t, "/my-builds", r.URL.Path, "should POST to topic path")
		assert.Equal(t, "AWF Workflow", r.Header.Get("Title"), "should set custom title")
		assert.Equal(t, "high", r.Header.Get("Priority"), "should set high priority")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"xyz789"}`))
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Title:    "AWF Workflow",
		Message:  "Workflow 'build-project' completed successfully",
		Priority: "high",
		Metadata: map[string]string{
			"topic":    "my-builds",
			"workflow": "build-project",
			"status":   "success",
			"duration": "3m45s",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: full payload with all fields populated
	// When: Send is called
	// Then: should succeed and POST to ntfy server
	require.NoError(t, err, "Send should succeed with full payload")
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
	assert.Equal(t, http.StatusOK, result.StatusCode, "should return 200 status code")
}

func TestNtfyBackend_Send_DefaultTitleWhenEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "AWF Workflow", r.Header.Get("Title"), "should use default title")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Title:   "", // Empty title, should use default
		Message: "Build finished",
		Metadata: map[string]string{
			"topic": "builds",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: empty title field
	// When: Send is called
	// Then: should use default title "AWF Workflow"
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
}

func TestNtfyBackend_Send_DifferentPriorities(t *testing.T) {
	tests := []struct {
		name             string
		priority         string
		expectedPriority string
	}{
		{"low_priority", "low", "low"},
		{"default_priority", "default", "default"},
		{"high_priority", "high", "high"},
		{"empty_priority", "", "default"}, // Should default to "default"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.expectedPriority, r.Header.Get("Priority"), "should set correct priority")
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			backend, err := newNtfyBackend(server.URL)
			require.NoError(t, err)

			ctx := context.Background()
			payload := NotificationPayload{
				Title:    "Test Notification",
				Message:  "Testing priority levels",
				Priority: tt.priority,
				Metadata: map[string]string{
					"topic": "test",
				},
			}

			result, err := backend.Send(ctx, payload)

			// Given: different priority values
			// When: Send is called
			// Then: should send notification with correct priority header
			require.NoError(t, err, "should handle priority: %s", tt.priority)
			require.NotNil(t, result)
			assert.Equal(t, "ntfy", result.Backend)
		})
	}
}

func TestNtfyBackend_Send_CustomServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/alerts", r.URL.Path, "should POST to alerts topic")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "alerts",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: custom ntfy server URL
	// When: Send is called
	// Then: should POST to custom server
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
}

// --- Send method tests - Edge Cases ---

func TestNtfyBackend_Send_EmptyMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "", // Empty message
		Metadata: map[string]string{
			"topic": "test",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: empty message field
	// When: Send is called
	// Then: should send notification with empty body (ntfy allows this)
	require.NoError(t, err, "ntfy allows empty messages")
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
}

func TestNtfyBackend_Send_VeryLongMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	// Create a message with 5000 characters (ntfy has limits)
	longMessage := make([]byte, 5000)
	for i := range longMessage {
		longMessage[i] = 'A'
	}

	payload := NotificationPayload{
		Message: string(longMessage),
		Metadata: map[string]string{
			"topic": "test",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: very long message
	// When: Send is called
	// Then: should handle long messages (ntfy will truncate if needed)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
}

func TestNtfyBackend_Send_SpecialCharactersInMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Special chars: \n\t<>&\"'{}[]",
		Metadata: map[string]string{
			"topic": "test",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: message with special characters
	// When: Send is called
	// Then: should handle special characters correctly
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
}

func TestNtfyBackend_Send_UnicodeInMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Unicode: 你好世界 🎉 Привет мир",
		Metadata: map[string]string{
			"topic": "test",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: message with unicode characters
	// When: Send is called
	// Then: should handle UTF-8 correctly
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
}

func TestNtfyBackend_Send_TopicWithSpecialChars(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/my-builds_2024", r.URL.Path, "should handle topic with special chars")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "my-builds_2024",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: topic with underscores and numbers
	// When: Send is called
	// Then: should construct valid URL with topic
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
}

// --- Send method tests - Error Handling ---

func TestNtfyBackend_Send_MissingTopic(t *testing.T) {
	backend, err := newNtfyBackend("https://ntfy.sh")
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message:  "Workflow completed",
		Metadata: map[string]string{}, // No topic
	}

	result, err := backend.Send(ctx, payload)

	// Given: payload without topic in metadata
	// When: Send is called
	// Then: should return error indicating missing topic
	require.Error(t, err, "should fail without topic")
	assert.Nil(t, result, "result should be nil on error")
	if err != nil {
		assert.Contains(t, err.Error(), "topic", "error should mention missing topic")
	}
}

func TestNtfyBackend_Send_EmptyTopic(t *testing.T) {
	backend, err := newNtfyBackend("https://ntfy.sh")
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Workflow completed",
		Metadata: map[string]string{
			"topic": "", // Empty topic
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: empty topic value
	// When: Send is called
	// Then: should return error for empty topic
	require.Error(t, err, "should fail with empty topic")
	assert.Nil(t, result)
	if err != nil {
		assert.Contains(t, err.Error(), "topic", "error should mention topic")
	}
}

func TestNtfyBackend_Send_WhitespaceTopic(t *testing.T) {
	backend, err := newNtfyBackend("https://ntfy.sh")
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "   ", // Whitespace-only topic
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: whitespace-only topic
	// When: Send is called
	// Then: should return error for invalid topic
	require.Error(t, err, "should fail with whitespace topic")
	assert.Nil(t, result)
}

func TestNtfyBackend_Send_ContextCancellation(t *testing.T) {
	backend, err := newNtfyBackend("https://ntfy.sh")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	payload := NotificationPayload{
		Message: "Workflow completed",
		Metadata: map[string]string{
			"topic": "test",
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

func TestNtfyBackend_Send_ContextTimeout(t *testing.T) {
	backend, err := newNtfyBackend("https://ntfy.sh")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // Ensure timeout occurs

	payload := NotificationPayload{
		Message: "Workflow completed",
		Metadata: map[string]string{
			"topic": "test",
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

func TestNtfyBackend_Send_InvalidServerURL(t *testing.T) {
	// Create backend with invalid URL format
	backend, err := newNtfyBackend("not-a-valid-url")
	require.NoError(t, err) // Constructor doesn't validate URL format

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "test",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: backend with malformed URL
	// When: Send is called
	// Then: should fail during HTTP request
	assert.Error(t, err, "should fail with invalid URL")
	assert.Nil(t, result)
}

func TestNtfyBackend_Send_HTTPTimeoutEnforced(t *testing.T) {
	// Test that the 10-second timeout from NFR-001 is enforced
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second) // Exceed 10s timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "test",
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

func TestNtfyBackend_Send_UnreachableServer(t *testing.T) {
	// Use unreachable server (RFC 5737 TEST-NET-1)
	backend, err := newNtfyBackend("https://192.0.2.1")
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "test",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: unreachable ntfy server
	// When: Send is called
	// Then: should return connection error
	assert.Error(t, err, "should fail with unreachable server")
	assert.Nil(t, result)
}

// --- Send method tests - HTTP Response Codes ---

func TestNtfyBackend_Send_SuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test123"}`))
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "test",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: valid request to ntfy server
	// When: Send is called
	// Then: should return 2xx status code
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
	assert.GreaterOrEqual(t, result.StatusCode, 200, "status should be 2xx on success")
	assert.Less(t, result.StatusCode, 300, "status should be 2xx on success")
}

func TestNtfyBackend_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "test",
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

func TestNtfyBackend_Send_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "test",
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

// --- Send method tests - Metadata Handling ---

func TestNtfyBackend_Send_NilMetadata(t *testing.T) {
	backend, err := newNtfyBackend("https://ntfy.sh")
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message:  "Test message",
		Metadata: nil, // Nil metadata
	}

	result, err := backend.Send(ctx, payload)

	// Given: nil metadata (no topic)
	// When: Send is called
	// Then: should return error for missing topic
	require.Error(t, err, "should fail with nil metadata")
	assert.Nil(t, result)
	if err != nil {
		assert.Contains(t, err.Error(), "topic", "error should mention missing topic")
	}
}

func TestNtfyBackend_Send_AdditionalMetadataIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test", r.URL.Path, "should only use topic from metadata")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic":    "test",
			"workflow": "build-project", // Additional metadata should be ignored
			"status":   "success",
			"duration": "3m45s",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: metadata with extra fields beyond topic
	// When: Send is called
	// Then: should succeed and ignore extra metadata
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
}

// --- Send method tests - URL Construction ---

func TestNtfyBackend_Send_URLConstructionWithoutTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/builds", r.URL.Path, "should construct correct path")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "builds",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: baseURL without trailing slash
	// When: Send is called
	// Then: should construct URL as https://ntfy.sh/builds
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
}

func TestNtfyBackend_Send_URLConstructionWithTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/builds", r.URL.Path, "should avoid double slashes")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL + "/")
	require.NoError(t, err)

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "Test message",
		Metadata: map[string]string{
			"topic": "builds",
		},
	}

	result, err := backend.Send(ctx, payload)

	// Given: baseURL with trailing slash
	// When: Send is called
	// Then: should handle trailing slash correctly (no double slashes)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ntfy", result.Backend)
}

// --- Concurrent Send tests ---

func TestNtfyBackend_Send_ConcurrentCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"concurrent"}`))
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			ctx := context.Background()
			payload := NotificationPayload{
				Message: "Concurrent test",
				Metadata: map[string]string{
					"topic": "test",
				},
			}

			result, err := backend.Send(ctx, payload)

			// Given: concurrent Send calls
			// When: multiple goroutines call Send simultaneously
			// Then: should handle concurrent requests safely
			assert.NoError(t, err, "goroutine %d failed", id)
			assert.NotNil(t, result, "goroutine %d got nil result", id)
			assert.Equal(t, "ntfy", result.Backend)

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

// --- Rate Limiting tests (if applicable) ---

func TestNtfyBackend_Send_RateLimitHandling(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount >= 3 {
			// Simulate rate limiting on 3rd call
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"ok"}`))
		}
	}))
	defer server.Close()

	backend, err := newNtfyBackend(server.URL)
	require.NoError(t, err)

	ctx := context.Background()

	// Send multiple notifications rapidly
	for range 3 {
		payload := NotificationPayload{
			Message: "Rate limit test",
			Metadata: map[string]string{
				"topic": "test-rate-limit",
			},
		}

		result, err := backend.Send(ctx, payload)

		// Given: rapid consecutive sends
		// When: Send is called multiple times
		// Then: should handle rate limiting gracefully (if ntfy enforces it)
		if err != nil {
			// Rate limiting may occur
			assert.NotNil(t, result, "should have result even on rate limit")
			assert.Equal(t, 429, result.StatusCode, "rate limit should return 429")
		} else {
			require.NotNil(t, result)
			assert.Equal(t, "ntfy", result.Backend)
		}
	}
}
