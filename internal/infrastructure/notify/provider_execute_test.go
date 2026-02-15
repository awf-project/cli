package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock Backend with Function ---

type mockBackendFunc struct {
	sendFunc func(ctx context.Context, payload NotificationPayload) (*BackendResult, error)
}

func (m *mockBackendFunc) Send(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, payload)
	}
	return &BackendResult{
		Backend:    "mock",
		StatusCode: 200,
		Response:   "OK",
	}, nil
}

// --- Happy Path Tests ---

func TestNotifyOperationProvider_Execute_Desktop_HappyPath(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	// Register desktop backend with mock
	mockDesktop := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			assert.Equal(t, "Test Title", payload.Title)
			assert.Equal(t, "Test Message", payload.Message)
			assert.Equal(t, "default", payload.Priority)
			return &BackendResult{
				Backend:    "desktop",
				StatusCode: 0,
				Response:   "notification sent",
			}, nil
		},
	}
	err := provider.RegisterBackend("desktop", mockDesktop)
	require.NoError(t, err, "backend registration should succeed")

	ctx := context.Background()
	inputs := map[string]any{
		"backend": "desktop",
		"title":   "Test Title",
		"message": "Test Message",
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.NoError(t, err, "desktop backend should succeed")
	require.NotNil(t, result)
	assert.True(t, result.Success, "result should indicate success")
	assert.Equal(t, "desktop", result.Outputs["backend"])
	assert.Equal(t, 0, result.Outputs["status"])
	assert.Equal(t, "notification sent", result.Outputs["response"])
}

func TestNotifyOperationProvider_Execute_Webhook_HappyPath(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	// Replace webhook backend with mock
	mockWebhook := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			assert.Equal(t, "Deployment", payload.Title)
			assert.Equal(t, "Production deployed", payload.Message)
			assert.Equal(t, "https://hooks.example.com/deploy", payload.Metadata["webhook_url"])
			return &BackendResult{
				Backend:    "webhook",
				StatusCode: 201,
				Response:   `{"received": true}`,
			}, nil
		},
	}
	err := provider.RegisterBackend("webhook", mockWebhook)
	require.NoError(t, err, "backend registration should succeed")

	ctx := context.Background()
	inputs := map[string]any{
		"backend":     "webhook",
		"title":       "Deployment",
		"message":     "Production deployed",
		"webhook_url": "https://hooks.example.com/deploy",
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.NoError(t, err, "webhook backend should succeed")
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "webhook", result.Outputs["backend"])
	assert.Equal(t, 201, result.Outputs["status"])
}

// --- Input Validation Tests ---

func TestNotifyOperationProvider_Execute_MissingBackend(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})
	ctx := context.Background()

	inputs := map[string]any{
		"message": "test message",
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.Error(t, err, "should fail when backend is missing and no default configured")
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "no backend specified and no default backend configured")
}

func TestNotifyOperationProvider_Execute_MissingMessage(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})
	ctx := context.Background()

	inputs := map[string]any{
		"backend": "desktop",
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.Error(t, err, "should fail when message is missing")
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "required input \"message\"")
}

func TestNotifyOperationProvider_Execute_InvalidBackendType(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})
	ctx := context.Background()

	tests := []struct {
		name    string
		backend any
	}{
		{"backend_as_int", 123},
		{"backend_as_bool", true},
		{"backend_as_array", []string{"desktop"}},
		{"backend_as_map", map[string]string{"type": "desktop"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := map[string]any{
				"backend": tt.backend,
				"message": "test",
			}

			result, err := provider.Execute(ctx, "notify.send", inputs)

			require.Error(t, err, "should fail when backend has wrong type")
			require.NotNil(t, result)
			assert.False(t, result.Success)
			assert.Contains(t, err.Error(), "must be a string")
		})
	}
}

func TestNotifyOperationProvider_Execute_InvalidMessageType(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})
	ctx := context.Background()

	tests := []struct {
		name    string
		message any
	}{
		{"message_as_int", 42},
		{"message_as_bool", false},
		{"message_as_array", []string{"test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := map[string]any{
				"backend": "desktop",
				"message": tt.message,
			}

			result, err := provider.Execute(ctx, "notify.send", inputs)

			require.Error(t, err, "should fail when message has wrong type")
			require.NotNil(t, result)
			assert.False(t, result.Success)
			assert.Contains(t, err.Error(), "must be a string")
		})
	}
}

func TestNotifyOperationProvider_Execute_InvalidPriority(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})
	ctx := context.Background()

	// Register mock backend for successful validation test case
	mockDesktop := &mockBackendFunc{}
	err := provider.RegisterBackend("desktop", mockDesktop)
	require.NoError(t, err, "backend registration should succeed")

	tests := []struct {
		name     string
		priority string
	}{
		{"invalid_priority", "urgent"},
		{"numeric_priority", "1"},
		{"empty_priority_allowed", ""}, // Should use default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := map[string]any{
				"backend":  "desktop",
				"message":  "test",
				"priority": tt.priority,
			}

			result, err := provider.Execute(ctx, "notify.send", inputs)

			if tt.name == "empty_priority_allowed" {
				// Empty priority should default to "default"
				require.NoError(t, err)
			} else {
				require.Error(t, err, "should fail for invalid priority")
				require.NotNil(t, result)
				assert.False(t, result.Success)
				assert.Contains(t, err.Error(), "invalid priority")
			}
		})
	}
}

func TestNotifyOperationProvider_Execute_UnknownBackend(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})
	ctx := context.Background()

	inputs := map[string]any{
		"backend": "email", // Not a valid backend
		"message": "test",
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.Error(t, err, "should fail for unknown backend")
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "backend \"email\" not available")
}

// --- Backend-Specific Validation Tests ---

func TestNotifyOperationProvider_Execute_Webhook_MissingURL(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})
	ctx := context.Background()

	// Register mock webhook backend to test input validation
	mockWebhook := &mockBackendFunc{}
	err := provider.RegisterBackend("webhook", mockWebhook)
	require.NoError(t, err, "backend registration should succeed")

	inputs := map[string]any{
		"backend": "webhook",
		"message": "test message",
		// Missing webhook_url
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.Error(t, err, "webhook backend should require webhook_url")
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "requires 'webhook_url'")
}

// --- Backend Unavailable Tests ---

func TestNotifyOperationProvider_Execute_UnknownBackendNotConfigured(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})
	ctx := context.Background()

	inputs := map[string]any{
		"backend": "unknown",
		"message": "test",
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.Error(t, err, "unknown backend should not be available when not configured")
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "backend \"unknown\" not available")
}

// --- Backend Failure Tests ---

func TestNotifyOperationProvider_Execute_BackendReturnsError(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	// Mock backend that always fails
	mockFailing := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			return nil, errors.New("network timeout")
		},
	}
	err := provider.RegisterBackend("desktop", mockFailing)
	require.NoError(t, err, "backend registration should succeed")

	ctx := context.Background()
	inputs := map[string]any{
		"backend": "desktop",
		"message": "test",
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.Error(t, err, "should propagate backend error")
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "backend \"desktop\" failed")
	assert.Contains(t, err.Error(), "network timeout")
}

// --- Default Values Tests ---

func TestNotifyOperationProvider_Execute_DefaultTitle(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	mockDesktop := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			assert.Equal(t, "AWF Workflow", payload.Title, "should use default title")
			return &BackendResult{Backend: "desktop", StatusCode: 0, Response: "OK"}, nil
		},
	}
	err := provider.RegisterBackend("desktop", mockDesktop)
	require.NoError(t, err, "backend registration should succeed")

	ctx := context.Background()
	inputs := map[string]any{
		"backend": "desktop",
		"message": "test",
		// No title provided
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestNotifyOperationProvider_Execute_DefaultPriority(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	mockDesktop := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			assert.Equal(t, "default", payload.Priority, "should use default priority")
			return &BackendResult{Backend: "desktop", StatusCode: 0, Response: "OK"}, nil
		},
	}
	err := provider.RegisterBackend("desktop", mockDesktop)
	require.NoError(t, err, "backend registration should succeed")

	ctx := context.Background()
	inputs := map[string]any{
		"backend": "desktop",
		"message": "test",
		// No priority provided
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)
}

// --- Priority Validation Tests ---

func TestNotifyOperationProvider_Execute_ValidPriorities(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	mockDesktop := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			return &BackendResult{Backend: "desktop", StatusCode: 0, Response: "OK"}, nil
		},
	}
	err := provider.RegisterBackend("desktop", mockDesktop)
	require.NoError(t, err, "backend registration should succeed")

	ctx := context.Background()
	validPriorities := []string{"low", "default", "high"}

	for _, priority := range validPriorities {
		t.Run("priority_"+priority, func(t *testing.T) {
			inputs := map[string]any{
				"backend":  "desktop",
				"message":  "test",
				"priority": priority,
			}

			result, err := provider.Execute(ctx, "notify.send", inputs)

			require.NoError(t, err, "priority %q should be valid", priority)
			assert.True(t, result.Success)
		})
	}
}

// --- Operation Not Found Tests ---

func TestNotifyOperationProvider_Execute_UnknownOperation(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})
	ctx := context.Background()

	tests := []struct {
		name          string
		operationName string
	}{
		{"unknown_operation", "notify.unknown"},
		{"empty_operation", ""},
		{"wrong_namespace", "github.create_pr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := map[string]any{
				"backend": "desktop",
				"message": "test",
			}

			result, err := provider.Execute(ctx, tt.operationName, inputs)

			require.Error(t, err, "should fail for unknown operation")
			require.NotNil(t, result)
			assert.False(t, result.Success)
			assert.Contains(t, err.Error(), "not found")
		})
	}
}

// --- Edge Cases ---

func TestNotifyOperationProvider_Execute_EmptyMessage(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	mockDesktop := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			assert.Equal(t, "", payload.Message, "empty message should be passed through")
			return &BackendResult{Backend: "desktop", StatusCode: 0, Response: "OK"}, nil
		},
	}
	err := provider.RegisterBackend("desktop", mockDesktop)
	require.NoError(t, err, "backend registration should succeed")

	ctx := context.Background()
	inputs := map[string]any{
		"backend": "desktop",
		"message": "", // Empty but provided
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.NoError(t, err, "empty message should be allowed")
	assert.True(t, result.Success)
}

func TestNotifyOperationProvider_Execute_WhitespaceOnlyInputs(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	mockDesktop := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			// TrimSpace should remove whitespace
			assert.Equal(t, "", payload.Message, "message whitespace should be trimmed")
			// Title defaults to "AWF Workflow" when empty after trimming
			assert.Equal(t, "AWF Workflow", payload.Title, "empty title should use default")
			return &BackendResult{Backend: "desktop", StatusCode: 0, Response: "OK"}, nil
		},
	}
	err := provider.RegisterBackend("desktop", mockDesktop)
	require.NoError(t, err, "backend registration should succeed")

	ctx := context.Background()
	inputs := map[string]any{
		"backend": "desktop",
		"message": "   ", // Whitespace only
		"title":   "  \t\n  ",
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.NoError(t, err, "whitespace should be trimmed")
	assert.True(t, result.Success)
}

func TestNotifyOperationProvider_Execute_SpecialCharacters(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	specialMessage := "Test with <html> & \"quotes\" and\nnewlines ✓"

	mockDesktop := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			assert.Equal(t, specialMessage, payload.Message, "special chars should be preserved")
			return &BackendResult{Backend: "desktop", StatusCode: 0, Response: "OK"}, nil
		},
	}
	err := provider.RegisterBackend("desktop", mockDesktop)
	require.NoError(t, err, "backend registration should succeed")

	ctx := context.Background()
	inputs := map[string]any{
		"backend": "desktop",
		"message": specialMessage,
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.NoError(t, err, "special characters should be handled")
	assert.True(t, result.Success)
}

// --- Context Tests ---

func TestNotifyOperationProvider_Execute_ContextCancellation(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	mockDesktop := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &BackendResult{Backend: "desktop", StatusCode: 0, Response: "OK"}, nil
			}
		},
	}
	err := provider.RegisterBackend("desktop", mockDesktop)
	require.NoError(t, err, "backend registration should succeed")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	inputs := map[string]any{
		"backend": "desktop",
		"message": "test",
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.Error(t, err, "should fail when context is cancelled")
	require.NotNil(t, result)
	assert.False(t, result.Success)
}

// --- Metadata Propagation Tests ---

func TestNotifyOperationProvider_Execute_MetadataPassthrough(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	mockWebhook := &mockBackendFunc{
		sendFunc: func(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
			assert.Equal(t, "https://example.com/hook", payload.Metadata["webhook_url"], "webhook_url should be in metadata")
			return &BackendResult{Backend: "webhook", StatusCode: 200, Response: "OK"}, nil
		},
	}
	err := provider.RegisterBackend("webhook", mockWebhook)
	require.NoError(t, err, "backend registration should succeed")

	ctx := context.Background()
	inputs := map[string]any{
		"backend":     "webhook",
		"message":     "test",
		"webhook_url": "https://example.com/hook",
	}

	result, err := provider.Execute(ctx, "notify.send", inputs)

	require.NoError(t, err)
	assert.True(t, result.Success)
}
