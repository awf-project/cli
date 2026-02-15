package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper functions ---

// newMockBackend creates a mockBackend with fixed result and error.
func newMockBackend(backend string, statusCode int, response string, err error) *mockBackend {
	var result *BackendResult
	if err == nil {
		result = &BackendResult{
			Backend:    backend,
			StatusCode: statusCode,
			Response:   response,
		}
	}
	return &mockBackend{
		result: result,
		err:    err,
	}
}

// --- RegisterBackend tests ---

func TestRegisterBackend_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		backendName string
		backend     Backend
	}{
		{
			name:        "register_desktop",
			backendName: "desktop",
			backend:     &mockBackend{},
		},
		{
			name:        "register_webhook",
			backendName: "webhook",
			backend:     &mockBackend{},
		},
		{
			name:        "register_custom_backend",
			backendName: "custom",
			backend:     &mockBackend{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewNotifyOperationProvider(&mockLogger{})

			err := provider.RegisterBackend(tt.backendName, tt.backend)

			assert.NoError(t, err, "RegisterBackend should succeed for valid input")
			assert.Contains(t, provider.backends, tt.backendName, "backend should be registered in map")
			assert.Equal(t, tt.backend, provider.backends[tt.backendName], "registered backend should match input")
		})
	}
}

func TestRegisterBackend_MultipleBackends(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	desktop := &mockBackend{}
	webhook := &mockBackend{}

	err1 := provider.RegisterBackend("desktop", desktop)
	err2 := provider.RegisterBackend("webhook", webhook)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Len(t, provider.backends, 2, "should have 2 registered backends")
	assert.Equal(t, desktop, provider.backends["desktop"])
	assert.Equal(t, webhook, provider.backends["webhook"])
}

func TestRegisterBackend_DuplicateRegistration(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	backend1 := &mockBackend{}
	backend2 := &mockBackend{}

	err1 := provider.RegisterBackend("desktop", backend1)
	require.NoError(t, err1, "first registration should succeed")

	err2 := provider.RegisterBackend("desktop", backend2)

	assert.Error(t, err2, "duplicate registration should return error")
	assert.Same(t, backend1, provider.backends["desktop"], "original backend should remain registered")
	assert.NotSame(t, backend2, provider.backends["desktop"], "second backend should not replace first")
}

func TestRegisterBackend_NilBackend(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	err := provider.RegisterBackend("desktop", nil)

	assert.Error(t, err, "registering nil backend should return error")
	assert.NotContains(t, provider.backends, "desktop", "nil backend should not be registered")
}

func TestRegisterBackend_EmptyName(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	backend := &mockBackend{}

	err := provider.RegisterBackend("", backend)

	assert.Error(t, err, "registering with empty name should return error")
	assert.NotContains(t, provider.backends, "", "empty name should not be registered")
}

func TestRegisterBackend_WhitespaceName(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	backend := &mockBackend{}

	tests := []struct {
		name    string
		backend string
		wantErr bool
	}{
		{"spaces_only", "   ", true},
		{"tabs_only", "\t\t", true},
		{"newlines", "\n\n", true},
		{"mixed_whitespace", " \t\n ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.RegisterBackend(tt.backend, backend)

			if tt.wantErr {
				assert.Error(t, err, "registering with whitespace-only name should return error")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- SetDefaultBackend tests ---

func TestSetDefaultBackend_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		defaultBackend string
	}{
		{"set_desktop", "desktop"},
		{"set_webhook", "webhook"},
		{"set_custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewNotifyOperationProvider(&mockLogger{})

			provider.SetDefaultBackend(tt.defaultBackend)

			assert.Equal(t, tt.defaultBackend, provider.defaultBackend, "default backend should be set")
		})
	}
}

func TestSetDefaultBackend_OverwritePrevious(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	provider.SetDefaultBackend("desktop")
	assert.Equal(t, "desktop", provider.defaultBackend)

	provider.SetDefaultBackend("webhook")
	assert.Equal(t, "webhook", provider.defaultBackend, "second call should overwrite first")

	provider.SetDefaultBackend("custom")
	assert.Equal(t, "custom", provider.defaultBackend, "third call should overwrite second")
}

func TestSetDefaultBackend_EmptyString(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	provider.SetDefaultBackend("desktop")
	assert.Equal(t, "desktop", provider.defaultBackend)

	provider.SetDefaultBackend("")
	assert.Equal(t, "", provider.defaultBackend, "should allow clearing default backend")
}

func TestSetDefaultBackend_DoesNotValidateBackendExists(t *testing.T) {
	// SetDefaultBackend should NOT validate that the backend is registered.
	// Validation happens at execution time in Execute().
	provider := NewNotifyOperationProvider(&mockLogger{})

	provider.SetDefaultBackend("nonexistent")

	assert.Equal(t, "nonexistent", provider.defaultBackend, "should accept unregistered backend name")
	assert.Empty(t, provider.backends, "should not register backend automatically")
}

// --- Constructor tests ---

func TestNewNotifyOperationProvider_AcceptsOnlyLogger(t *testing.T) {
	logger := &mockLogger{}

	provider := NewNotifyOperationProvider(logger)

	require.NotNil(t, provider, "constructor should return non-nil provider")
	assert.Equal(t, logger, provider.logger, "logger should be set")
	assert.NotNil(t, provider.backends, "backends map should be initialized")
	assert.Empty(t, provider.backends, "backends map should start empty")
	assert.Equal(t, "", provider.defaultBackend, "defaultBackend should start empty")
	assert.NotNil(t, provider.operations, "operations registry should be initialized")
}

func TestNewNotifyOperationProvider_NilLogger(t *testing.T) {
	provider := NewNotifyOperationProvider(nil)

	require.NotNil(t, provider, "constructor should return non-nil provider even with nil logger")
	assert.Nil(t, provider.logger, "nil logger should be accepted")
	assert.NotNil(t, provider.backends, "backends map should still be initialized")
	assert.NotNil(t, provider.operations, "operations registry should still be initialized")
}

func TestNewNotifyOperationProvider_BackendsNotHardcoded(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	assert.Empty(t, provider.backends, "backends should NOT be hardcoded in constructor")
	assert.NotContains(t, provider.backends, "desktop", "desktop backend should not be pre-registered")
	assert.NotContains(t, provider.backends, "webhook", "webhook backend should not be pre-registered")
}

// --- Execute integration with RegisterBackend ---

func TestExecute_UsesRegisteredBackend(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	// Register a mock backend
	backend := newMockBackend("test", 200, "sent", nil)

	err := provider.RegisterBackend("test", backend)
	require.NoError(t, err)

	// Execute notify.send with registered backend
	result, err := provider.Execute(context.Background(), "notify.send", map[string]any{
		"backend": "test",
		"title":   "Test Title",
		"message": "Test Message",
	})

	assert.NoError(t, err, "Execute should succeed with registered backend")
	assert.True(t, result.Success, "result should indicate success")
	assert.Equal(t, "test", result.Outputs["backend"])
	assert.Equal(t, 200, result.Outputs["status"])
}

func TestExecute_UnregisteredBackend(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	// Do NOT register any backend
	result, err := provider.Execute(context.Background(), "notify.send", map[string]any{
		"backend": "nonexistent",
		"message": "Test",
	})

	assert.Error(t, err, "Execute should fail with unregistered backend")
	assert.False(t, result.Success, "result should indicate failure")
	assert.Contains(t, err.Error(), "not available", "error should mention backend unavailability")
	assert.Contains(t, err.Error(), "nonexistent", "error should mention the requested backend")
}

func TestExecute_BackendFailure(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	// Register backend that always fails
	backend := newMockBackend("failing", 0, "", errors.New("backend send failed"))

	err := provider.RegisterBackend("failing", backend)
	require.NoError(t, err)

	result, err := provider.Execute(context.Background(), "notify.send", map[string]any{
		"backend": "failing",
		"message": "Test",
	})

	assert.Error(t, err, "Execute should propagate backend error")
	assert.False(t, result.Success, "result should indicate failure")
	assert.Contains(t, err.Error(), "backend \"failing\" failed", "error should identify failing backend")
}

// --- Execute with default backend ---

func TestExecute_UsesDefaultBackend_WhenNoBackendInput(t *testing.T) {
	// NOTE: This test WILL FAIL against the current stub because Execute
	// currently requires explicit 'backend' input. The real implementation
	// should fall back to defaultBackend when 'backend' input is missing.

	provider := NewNotifyOperationProvider(&mockLogger{})

	backend := newMockBackend("default-backend", 200, "sent via default", nil)

	err := provider.RegisterBackend("default-backend", backend)
	require.NoError(t, err)

	provider.SetDefaultBackend("default-backend")

	// Execute WITHOUT explicit 'backend' input — should use default
	result, err := provider.Execute(context.Background(), "notify.send", map[string]any{
		"message": "Test Message",
	})

	assert.NoError(t, err, "Execute should succeed using default backend")
	assert.True(t, result.Success, "result should indicate success")
	assert.Equal(t, "default-backend", result.Outputs["backend"])
}

func TestExecute_ExplicitBackendOverridesDefault(t *testing.T) {
	provider := NewNotifyOperationProvider(&mockLogger{})

	defaultBackend := newMockBackend("default-backend", 200, "sent", nil)
	explicitBackend := newMockBackend("explicit-backend", 200, "sent", nil)

	err1 := provider.RegisterBackend("default-backend", defaultBackend)
	err2 := provider.RegisterBackend("explicit-backend", explicitBackend)
	require.NoError(t, err1)
	require.NoError(t, err2)

	provider.SetDefaultBackend("default-backend")

	// Execute WITH explicit 'backend' input — should override default
	result, err := provider.Execute(context.Background(), "notify.send", map[string]any{
		"backend": "explicit-backend",
		"message": "Test",
	})

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "explicit-backend", result.Outputs["backend"], "should use explicit backend, not default")
}

func TestExecute_NoDefaultBackend_NoExplicitBackend_Fails(t *testing.T) {
	// NOTE: This test WILL FAIL against the current stub because Execute
	// treats 'backend' as required. The real implementation should return
	// an error when both defaultBackend is empty AND no 'backend' input.

	provider := NewNotifyOperationProvider(&mockLogger{})
	// Do not set default backend
	// Do not provide 'backend' in inputs

	result, err := provider.Execute(context.Background(), "notify.send", map[string]any{
		"message": "Test",
	})

	assert.Error(t, err, "Execute should fail when no backend specified and no default set")
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "backend", "error should mention missing backend")
}
