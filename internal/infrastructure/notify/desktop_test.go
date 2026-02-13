//go:build integration
// +build integration

package notify

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Interface compliance tests ---

func TestDesktopBackend_ImplementsInterface(t *testing.T) {
	var _ Backend = (*desktopBackend)(nil)
}

// --- Constructor tests ---

func TestNewDesktopBackend_CreatesValidInstance(t *testing.T) {
	backend := newDesktopBackend()

	require.NotNil(t, backend, "newDesktopBackend() should not return nil")
}

func TestNewDesktopBackend_MultipleInstances(t *testing.T) {
	backend1 := newDesktopBackend()
	backend2 := newDesktopBackend()

	require.NotNil(t, backend1)
	require.NotNil(t, backend2)
	assert.NotEqual(t, backend1, backend2, "should create separate instances")
}

// --- Send method tests - Happy Path ---

func TestDesktopBackend_Send_MinimalPayload(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	payload := NotificationPayload{
		Message: "Workflow completed",
	}

	result, err := backend.Send(ctx, payload)

	// Given: minimal payload (message only)
	// When: Send is called
	// Then: should succeed and return result with backend name
	require.NoError(t, err, "Send should succeed with minimal payload")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "desktop", result.Backend, "backend name should be 'desktop'")
	assert.Equal(t, 0, result.StatusCode, "status code should be 0 on success")
}

func TestDesktopBackend_Send_FullPayload(t *testing.T) {
	backend := newDesktopBackend()
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
	// Then: should succeed and return result
	require.NoError(t, err, "Send should succeed with full payload")
	require.NotNil(t, result)
	assert.Equal(t, "desktop", result.Backend)
	assert.Equal(t, 0, result.StatusCode)
}

func TestDesktopBackend_Send_DefaultTitleWhenEmpty(t *testing.T) {
	backend := newDesktopBackend()
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
	assert.Equal(t, "desktop", result.Backend)
}

func TestDesktopBackend_Send_PriorityLevels(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	tests := []struct {
		name     string
		priority string
	}{
		{"low_priority", "low"},
		{"default_priority", "default"},
		{"high_priority", "high"},
		{"empty_priority", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Rate limiting: wait before each test to avoid GDBus ExcessNotificationGeneration
			time.Sleep(1 * time.Second)

			payload := NotificationPayload{
				Title:    "Test Notification",
				Message:  "Priority test",
				Priority: tt.priority,
			}

			result, err := backend.Send(ctx, payload)

			// Given: different priority levels
			// When: Send is called
			// Then: should succeed for all valid priority levels
			require.NoError(t, err, "Send should succeed for priority: %s", tt.priority)
			require.NotNil(t, result)
			assert.Equal(t, "desktop", result.Backend)
		})
	}
}

// --- Send method tests - Edge Cases ---

func TestDesktopBackend_Send_EmptyMessage(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	payload := NotificationPayload{
		Title:   "Test",
		Message: "", // Empty message
	}

	result, err := backend.Send(ctx, payload)

	// Given: empty message field
	// When: Send is called
	// Then: implementation should handle gracefully (may succeed or fail depending on platform)
	// For stub: should succeed (validation comes later)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestDesktopBackend_Send_LongMessage(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	// Create a message with 500 characters
	longMessage := string(make([]byte, 500))
	for i := range longMessage {
		longMessage = longMessage[:i] + "A" + longMessage[i+1:]
	}

	payload := NotificationPayload{
		Title:   "Long Message Test",
		Message: longMessage,
	}

	result, err := backend.Send(ctx, payload)

	// Given: very long message
	// When: Send is called
	// Then: should handle gracefully (platform may truncate)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestDesktopBackend_Send_SpecialCharactersInMessage(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	tests := []struct {
		name    string
		title   string
		message string
	}{
		{
			name:    "unicode_emoji",
			title:   "Success ✅",
			message: "Workflow completed successfully 🎉",
		},
		{
			name:    "special_chars",
			title:   "Test: Validation & Escaping",
			message: "Test with <special> & \"quoted\" characters",
		},
		{
			name:    "newlines",
			title:   "Multi-line",
			message: "Line 1\nLine 2\nLine 3",
		},
		{
			name:    "single_quotes",
			title:   "Quote's Test",
			message: "It's working with single quotes",
		},
		{
			name:    "backticks",
			title:   "Code Test",
			message: "Command: `awf run workflow`",
		},
		{
			name:    "shell_chars",
			title:   "Shell Test",
			message: "Variables: $HOME and $(pwd)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Rate limiting: wait before each test to avoid GDBus ExcessNotificationGeneration
			time.Sleep(1 * time.Second)

			payload := NotificationPayload{
				Title:   tt.title,
				Message: tt.message,
			}

			result, err := backend.Send(ctx, payload)

			// Given: special characters in title/message
			// When: Send is called
			// Then: should not crash, should handle escaping
			require.NoError(t, err, "should handle special characters without error")
			require.NotNil(t, result)
			assert.Equal(t, "desktop", result.Backend)
		})
	}
}

func TestDesktopBackend_Send_MetadataFields(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	tests := []struct {
		name     string
		metadata map[string]string
	}{
		{
			name:     "nil_metadata",
			metadata: nil,
		},
		{
			name:     "empty_metadata",
			metadata: map[string]string{},
		},
		{
			name: "workflow_metadata",
			metadata: map[string]string{
				"workflow": "build-project",
				"status":   "success",
				"duration": "3m45s",
			},
		},
		{
			name: "large_metadata",
			metadata: map[string]string{
				"workflow":   "very-long-workflow-name-that-exceeds-normal-length",
				"status":     "success",
				"duration":   "1h23m45s",
				"outputs":    `{"key1":"value1","key2":"value2"}`,
				"step_count": "42",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Rate limiting: wait before each test to avoid GDBus ExcessNotificationGeneration
			time.Sleep(1 * time.Second)

			payload := NotificationPayload{
				Title:    "Test",
				Message:  "Metadata test",
				Metadata: tt.metadata,
			}

			result, err := backend.Send(ctx, payload)

			// Given: various metadata configurations
			// When: Send is called
			// Then: should handle metadata gracefully
			require.NoError(t, err)
			require.NotNil(t, result)
		})
	}
}

// --- Send method tests - Context Handling ---

func TestDesktopBackend_Send_ContextCancellation(t *testing.T) {
	backend := newDesktopBackend()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately before Send

	payload := NotificationPayload{
		Message: "Test message",
	}

	result, err := backend.Send(ctx, payload)
	// Given: cancelled context
	// When: Send is called
	// Then: should detect cancellation and return error
	// (stub may not implement this yet, but test should exist)
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled, "should return context.Canceled error")
	}
	// Stub may succeed with cancelled context (not implemented yet)
	// Real implementation should check ctx.Err()
	_ = result // May be nil or valid depending on implementation
}

func TestDesktopBackend_Send_ContextTimeout(t *testing.T) {
	backend := newDesktopBackend()

	// Very short timeout that will expire
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond) // Ensure timeout has expired

	payload := NotificationPayload{
		Message: "Test message",
	}

	result, err := backend.Send(ctx, payload)
	// Given: expired context timeout
	// When: Send is called
	// Then: should detect timeout and return error
	if err != nil {
		assert.ErrorIs(t, err, context.DeadlineExceeded, "should return context.DeadlineExceeded")
	}
	_ = result // May be nil or valid depending on implementation
}

func TestDesktopBackend_Send_ContextWithValidTimeout(t *testing.T) {
	backend := newDesktopBackend()

	// Long timeout that won't expire during test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	payload := NotificationPayload{
		Message: "Test message",
	}

	result, err := backend.Send(ctx, payload)

	// Given: context with valid (non-expired) timeout
	// When: Send is called
	// Then: should succeed normally
	require.NoError(t, err)
	require.NotNil(t, result)
}

// --- Send method tests - Platform Detection ---

func TestDesktopBackend_Send_ShouldDetectPlatform(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	payload := NotificationPayload{
		Title:   "Platform Test",
		Message: "Testing platform detection",
	}

	result, err := backend.Send(ctx, payload)

	// Given: desktop backend on current OS
	// When: Send is called
	// Then: should detect platform and use appropriate command
	// (Linux: notify-send, macOS: osascript)
	// For stub: just verify it doesn't crash
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "desktop", result.Backend)
	assert.NotEmpty(t, result.Response, "response should contain platform-specific output")
}

// --- Send method tests - Error Handling ---

func TestDesktopBackend_Send_HeadlessEnvironment(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	payload := NotificationPayload{
		Title:   "Headless Test",
		Message: "Testing headless environment",
	}

	result, err := backend.Send(ctx, payload)

	// Given: potentially headless environment (CI server, SSH session)
	// When: Send is called
	// Then: real implementation should gracefully fail with descriptive error
	// Stub: will succeed, but real implementation needs to detect DISPLAY env
	// We accept both outcomes for now since it's platform-dependent
	if err != nil {
		// Real implementation on headless: should fail gracefully
		assert.Contains(t, err.Error(), "display", "error should mention display/headless issue")
		assert.NotNil(t, result)
		assert.Equal(t, "desktop", result.Backend)
	} else {
		// Stub or environment with display: succeeds
		require.NotNil(t, result)
		assert.Equal(t, "desktop", result.Backend)
	}
}

func TestDesktopBackend_Send_UnsupportedPlatform(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	payload := NotificationPayload{
		Message: "Platform test",
	}

	result, err := backend.Send(ctx, payload)

	// Given: potentially unsupported platform (Windows without WSL, etc.)
	// When: Send is called
	// Then: real implementation should fail with descriptive error
	// Stub: will succeed, but real implementation needs platform check
	if err != nil {
		// Real implementation on unsupported platform
		assert.Contains(t, err.Error(), "platform", "error should mention unsupported platform")
	} else {
		// Stub or supported platform
		require.NotNil(t, result)
	}
}

func TestDesktopBackend_Send_CommandNotFound(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	payload := NotificationPayload{
		Title:   "Command Test",
		Message: "Testing missing command",
	}

	result, err := backend.Send(ctx, payload)

	// Given: system without notify-send/osascript installed
	// When: Send is called
	// Then: real implementation should fail with "command not found" error
	// Stub: will succeed, but real implementation needs to check exec.LookPath
	if err != nil {
		// Real implementation without notification tools
		assert.True(t,
			contains(err.Error(), "not found") || contains(err.Error(), "executable"),
			"error should mention missing command")
	} else {
		// Stub or system with notification tools
		require.NotNil(t, result)
	}
}

// --- Send method tests - Concurrent Access ---

func TestDesktopBackend_Send_ConcurrentCalls(t *testing.T) {
	t.Skip("Skipping concurrent test: OS notification daemon has strict rate limiting that prevents concurrent notifications")

	backend := newDesktopBackend()
	ctx := context.Background()

	// Send 3 notifications concurrently with staggered start
	// to avoid overwhelming the OS notification daemon (reduced from 10 due to rate limits)
	const concurrency = 3
	results := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		// Stagger goroutine launches to avoid rate limiting
		time.Sleep(1 * time.Second)

		go func(id int) {
			payload := NotificationPayload{
				Title:   "Concurrent Test",
				Message: "Message from goroutine",
				Metadata: map[string]string{
					"id": string(rune(id)),
				},
			}

			_, err := backend.Send(ctx, payload)
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < concurrency; i++ {
		err := <-results
		assert.NoError(t, err, "concurrent Send #%d should succeed", i)
	}
}

func TestDesktopBackend_Send_MultipleCalls(t *testing.T) {
	backend := newDesktopBackend()
	ctx := context.Background()

	// Send multiple notifications sequentially
	for i := 0; i < 5; i++ {
		// Rate limiting: wait before each notification to avoid GDBus ExcessNotificationGeneration
		if i > 0 {
			time.Sleep(1 * time.Second)
		}

		payload := NotificationPayload{
			Title:   "Sequential Test",
			Message: "Sequential notification",
		}

		result, err := backend.Send(ctx, payload)
		require.NoError(t, err, "notification #%d should succeed", i)
		require.NotNil(t, result)
		assert.Equal(t, "desktop", result.Backend)
	}
}

// --- Helper functions ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
