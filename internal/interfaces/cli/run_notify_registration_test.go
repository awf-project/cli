package cli

// Component T028 Integration Tests: Backend Registration End-to-End
// Purpose: Verify that registerNotifyBackends correctly registers backends based on configuration
// Scope: Config-driven registration, default backend handling, error scenarios, execution modes
//
// Test Strategy:
// - Happy Path: All backends registered and callable when fully configured
// - Default Backend: Fallback behavior and explicit override semantics
// - Error Handling: Invalid URLs, missing backend scenarios with descriptive errors
// - Execution Modes: Dry-run, resume, config changes between runs
// - Thread Safety: Concurrent registration attempts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/config"
	"github.com/awf-project/cli/internal/infrastructure/notify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyBackendRegistration_FullConfiguration(t *testing.T) {
	// GIVEN: Full notify configuration
	cfg := &config.ProjectConfig{}
	cfg.Notify.DefaultBackend = "desktop"

	logger := &mockLogger{}
	provider := notify.NewNotifyOperationProvider(logger)

	// WHEN: Registering backends
	err := registerNotifyBackends(provider, cfg, logger)

	// THEN: Both backends should be registered
	require.NoError(t, err, "registration should succeed")

	// Verify desktop backend is registered and callable
	desktopOp, ok := provider.GetOperation("notify.send")
	require.True(t, ok, "notify.send operation should be registered")
	require.NotNil(t, desktopOp, "notify.send operation should exist")

	// Verify all backends by attempting execution with test mode
	os.Setenv("AWF_TEST_MODE", "1")
	defer os.Unsetenv("AWF_TEST_MODE")

	ctx := context.Background()

	// Test desktop backend
	result, err := provider.Execute(ctx, "notify.send", map[string]interface{}{
		"backend": "desktop",
		"message": "test",
	})
	assert.NoError(t, err, "desktop backend should execute")
	assert.True(t, result.Success, "desktop backend should succeed")

	// Test webhook backend
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	result, err = provider.Execute(ctx, "notify.send", map[string]interface{}{
		"backend":     "webhook",
		"message":     "test",
		"webhook_url": webhookServer.URL,
	})
	assert.NoError(t, err, "webhook backend should execute")
	assert.True(t, result.Success, "webhook backend should succeed")
}

func TestNotifyBackendRegistration_PartialConfiguration(t *testing.T) {
	// GIVEN: Config with only desktop and webhook (no ntfy/slack URLs)
	cfg := &config.ProjectConfig{}
	cfg.Notify.DefaultBackend = "desktop"

	logger := &mockLogger{}
	provider := notify.NewNotifyOperationProvider(logger)

	// WHEN: Registering backends
	err := registerNotifyBackends(provider, cfg, logger)

	// THEN: Desktop and webhook should be registered, ntfy and slack should not
	require.NoError(t, err, "registration should succeed with partial config")

	os.Setenv("AWF_TEST_MODE", "1")
	defer os.Unsetenv("AWF_TEST_MODE")

	ctx := context.Background()

	// Desktop should work
	result, err := provider.Execute(ctx, "notify.send", map[string]interface{}{
		"backend": "desktop",
		"message": "test",
	})
	assert.NoError(t, err, "desktop backend should be available")
	assert.True(t, result.Success)

	// Webhook should work
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	result, err = provider.Execute(ctx, "notify.send", map[string]interface{}{
		"backend":     "webhook",
		"message":     "test",
		"webhook_url": webhookServer.URL,
	})
	assert.NoError(t, err, "webhook backend should be available")
	assert.True(t, result.Success)
}

func TestNotifyBackendRegistration_DefaultBackendFallback(t *testing.T) {
	// GIVEN: Config with default_backend set to "desktop"
	cfg := &config.ProjectConfig{}
	cfg.Notify.DefaultBackend = "desktop"

	logger := &mockLogger{}
	provider := notify.NewNotifyOperationProvider(logger)

	// WHEN: Registering backends with default
	err := registerNotifyBackends(provider, cfg, logger)
	require.NoError(t, err)

	os.Setenv("AWF_TEST_MODE", "1")
	defer os.Unsetenv("AWF_TEST_MODE")

	ctx := context.Background()

	// THEN: Execution without explicit backend should use default
	// NOTE: This test assumes provider supports default backend fallback
	// If not implemented yet, this will fail in RED phase
	result, err := provider.Execute(ctx, "notify.send", map[string]interface{}{
		"message": "test without backend",
	})
	assert.NoError(t, err, "should use default backend when backend not specified")
	assert.True(t, result.Success, "default backend should succeed")
}

func TestNotifyBackendRegistration_ExplicitBackendOverridesDefault(t *testing.T) {
	// GIVEN: Config with default_backend="desktop" but explicit backend="webhook"
	cfg := &config.ProjectConfig{}
	cfg.Notify.DefaultBackend = "desktop"

	logger := &mockLogger{}
	provider := notify.NewNotifyOperationProvider(logger)

	err := registerNotifyBackends(provider, cfg, logger)
	require.NoError(t, err)

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	ctx := context.Background()

	// WHEN: Explicit backend provided in inputs
	result, err := provider.Execute(ctx, "notify.send", map[string]interface{}{
		"backend":     "webhook",
		"message":     "test",
		"webhook_url": webhookServer.URL,
	})

	// THEN: Explicit backend should be used, not default
	assert.NoError(t, err, "explicit backend should override default")
	assert.True(t, result.Success)
}

func TestNotifyBackendRegistration_DefaultBackendNotRegistered(t *testing.T) {
	// GIVEN: Config with default_backend set to unknown backend
	cfg := &config.ProjectConfig{}
	cfg.Notify.DefaultBackend = "unknown" // Set default to backend that doesn't exist

	logger := &mockLogger{}
	provider := notify.NewNotifyOperationProvider(logger)

	// WHEN: Registering backends
	err := registerNotifyBackends(provider, cfg, logger)

	// THEN: Registration should succeed (validation deferred to execution time)
	require.NoError(t, err, "registration succeeds even if default backend won't be available")

	ctx := context.Background()

	// But execution without explicit backend should fail
	result, err := provider.Execute(ctx, "notify.send", map[string]interface{}{
		"message": "test",
	})
	assert.Error(t, err, "should fail when default backend not registered")
	assert.Contains(t, err.Error(), "not available", "should indicate backend unavailable")
	assert.False(t, result.Success)
}

func TestNotifyBackendRegistration_DryRunMode(t *testing.T) {
	// GIVEN: Config with backends configured
	cfg := &config.ProjectConfig{}

	logger := &mockLogger{}
	provider := notify.NewNotifyOperationProvider(logger)

	err := registerNotifyBackends(provider, cfg, logger)
	require.NoError(t, err)

	// WHEN: Executing with dry-run flag
	// NOTE: This test verifies that backends are registered for validation
	// The actual dry-run execution logic may be handled by ExecutionService
	// Here we verify the backends are callable without panicking

	ctx := context.Background()

	// THEN: Backend validation should work without actual execution
	// NOTE: If dry-run is not yet implemented, this test documents expected behavior
	os.Setenv("AWF_TEST_MODE", "1")
	defer os.Unsetenv("AWF_TEST_MODE")

	result, err := provider.Execute(ctx, "notify.send", map[string]interface{}{
		"backend": "desktop",
		"message": "dry-run test",
	})
	// In test mode, execution should succeed without side effects
	assert.NoError(t, err, "backends should be executable in test mode")
	assert.True(t, result.Success)
}

func TestNotifyBackendRegistration_ResumeCommand(t *testing.T) {
	// GIVEN: Backends registered during initial run
	cfg := &config.ProjectConfig{}

	logger1 := &mockLogger{}
	provider1 := notify.NewNotifyOperationProvider(logger1)

	err := registerNotifyBackends(provider1, cfg, logger1)
	require.NoError(t, err, "initial registration should succeed")

	// WHEN: Resume command creates new provider and re-registers
	logger2 := &mockLogger{}
	provider2 := notify.NewNotifyOperationProvider(logger2)
	err = registerNotifyBackends(provider2, cfg, logger2)

	// THEN: Re-registration should succeed without conflicts
	require.NoError(t, err, "re-registration should succeed for resume")

	os.Setenv("AWF_TEST_MODE", "1")
	defer os.Unsetenv("AWF_TEST_MODE")

	ctx := context.Background()

	// Verify backends are available in new provider
	result, err := provider2.Execute(ctx, "notify.send", map[string]interface{}{
		"backend": "desktop",
		"message": "resume test",
	})
	assert.NoError(t, err, "backends should be available after re-registration")
	assert.True(t, result.Success)
}

func TestNotifyBackendRegistration_NilProvider(t *testing.T) {
	// GIVEN: Nil provider
	cfg := &config.ProjectConfig{}
	logger := &mockLogger{}

	// WHEN: Attempting to register with nil provider
	err := registerNotifyBackends(nil, cfg, logger)

	// THEN: Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider cannot be nil")
}

func TestNotifyBackendRegistration_NilConfig(t *testing.T) {
	// GIVEN: Nil config
	logger := &mockLogger{}
	provider := notify.NewNotifyOperationProvider(logger)

	// WHEN: Attempting to register with nil config
	err := registerNotifyBackends(provider, nil, logger)

	// THEN: Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config cannot be nil")
}

func TestNotifyBackendRegistration_EmptyConfig(t *testing.T) {
	// GIVEN: Empty config (no URLs, no default backend)
	cfg := &config.ProjectConfig{}
	// Notify struct has zero values

	logger := &mockLogger{}
	provider := notify.NewNotifyOperationProvider(logger)

	// WHEN: Registering with empty config
	err := registerNotifyBackends(provider, cfg, logger)

	// THEN: Should succeed (desktop and webhook are always enabled)
	require.NoError(t, err, "empty config should succeed")

	os.Setenv("AWF_TEST_MODE", "1")
	defer os.Unsetenv("AWF_TEST_MODE")

	ctx := context.Background()

	// Verify desktop backend is available
	result, err := provider.Execute(ctx, "notify.send", map[string]interface{}{
		"backend": "desktop",
		"message": "test",
	})
	assert.NoError(t, err, "desktop should be available with empty config")
	assert.True(t, result.Success)
}

func TestNotifyBackendRegistration_ConcurrentAccess(t *testing.T) {
	// GIVEN: Multiple goroutines attempting registration
	cfg := &config.ProjectConfig{}
	cfg.Notify.DefaultBackend = "desktop"

	var wg sync.WaitGroup
	errors := make(chan error, 5)

	// WHEN: Concurrent registration attempts
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger := &mockLogger{}
			provider := notify.NewNotifyOperationProvider(logger)
			err := registerNotifyBackends(provider, cfg, logger)
			errors <- err
		}()
	}

	wg.Wait()
	close(errors)

	// THEN: All registrations should succeed
	for err := range errors {
		assert.NoError(t, err, "concurrent registration should succeed")
	}
}
