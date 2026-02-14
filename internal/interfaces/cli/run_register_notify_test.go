package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/infrastructure/config"
	"github.com/vanoix/awf/internal/infrastructure/notify"
)

// TestRegisterNotifyBackends_AlwaysEnabledBackends tests that desktop and webhook
// backends are always registered regardless of configuration.
func TestRegisterNotifyBackends_AlwaysEnabledBackends(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.ProjectConfig
		wantDesktop    bool
		wantWebhook    bool
		wantNtfy       bool
		wantSlack      bool
		wantDefaultSet bool
	}{
		{
			name:           "empty config registers desktop and webhook",
			cfg:            &config.ProjectConfig{},
			wantDesktop:    true,
			wantWebhook:    true,
			wantNtfy:       false,
			wantSlack:      false,
			wantDefaultSet: false,
		},
		{
			name:           "nil notify config still registers desktop and webhook",
			cfg:            &config.ProjectConfig{},
			wantDesktop:    true,
			wantWebhook:    true,
			wantNtfy:       false,
			wantSlack:      false,
			wantDefaultSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Enable test mode to prevent actual desktop notifications
			originalTestMode := os.Getenv("AWF_TEST_MODE")
			os.Setenv("AWF_TEST_MODE", "1")
			defer func() {
				if originalTestMode != "" {
					os.Setenv("AWF_TEST_MODE", originalTestMode)
				} else {
					os.Unsetenv("AWF_TEST_MODE")
				}
			}()

			// Setup mock HTTP server for webhook tests
			webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"ok": true}`))
			}))
			defer webhookServer.Close()

			provider := notify.NewNotifyOperationProvider(&mockLogger{})
			logger := &mockLogger{}

			err := registerNotifyBackends(provider, tt.cfg, logger)
			require.NoError(t, err, "registerNotifyBackends should not fail with valid config")

			// Verify notify.send operation exists
			desktopOp, ok := provider.GetOperation("notify.send")
			require.True(t, ok, "notify.send operation should exist")
			assert.NotNil(t, desktopOp)

			// Test desktop backend execution succeeds (backend is registered)
			result, err := provider.Execute(context.TODO(), "notify.send", map[string]any{
				"backend": "desktop",
				"message": "test",
			})
			if tt.wantDesktop {
				// Should succeed - backend registered
				assert.NoError(t, err, "desktop backend should be registered and executable")
				assert.NotNil(t, result)
			} else {
				// Should fail - backend not registered
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "backend \"desktop\" not available")
			}

			// Test webhook backend execution succeeds (backend is registered)
			result, err = provider.Execute(context.TODO(), "notify.send", map[string]any{
				"backend":     "webhook",
				"message":     "test",
				"webhook_url": webhookServer.URL,
			})
			if tt.wantWebhook {
				// Should succeed - backend registered
				assert.NoError(t, err, "webhook backend should be registered and executable")
				assert.NotNil(t, result)
			} else {
				// Should fail - backend not registered
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "backend \"webhook\" not available")
			}

			// Test ntfy backend - should fail if not configured
			_, err = provider.Execute(context.TODO(), "notify.send", map[string]any{
				"backend": "ntfy",
				"message": "test",
				"topic":   "test-topic",
			})
			if tt.wantNtfy {
				assert.NoError(t, err, "ntfy backend should be registered when ntfy_url is configured")
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "backend \"ntfy\" not available")
			}

			// Test slack backend - should fail if not configured
			_, err = provider.Execute(context.TODO(), "notify.send", map[string]any{
				"backend": "slack",
				"message": "test",
			})
			if tt.wantSlack {
				assert.NoError(t, err, "slack backend should be registered when slack_webhook_url is configured")
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "backend \"slack\" not available")
			}
		})
	}
}

// TestRegisterNotifyBackends_NtfyBackendConditional tests that ntfy backend
// is only registered when ntfy_url is configured.
func TestRegisterNotifyBackends_NtfyBackendConditional(t *testing.T) {
	tests := []struct {
		name        string
		ntfyURL     string
		wantNtfy    bool
		wantErrNtfy bool
	}{
		{
			name:        "ntfy backend registered with valid URL",
			ntfyURL:     "mock", // Will be replaced with httptest server URL
			wantNtfy:    true,
			wantErrNtfy: false,
		},
		{
			name:        "ntfy backend registered with custom URL",
			ntfyURL:     "mock", // Will be replaced with httptest server URL
			wantNtfy:    true,
			wantErrNtfy: false,
		},
		{
			name:        "ntfy backend NOT registered with empty URL",
			ntfyURL:     "",
			wantNtfy:    false,
			wantErrNtfy: true,
		},
		{
			name:        "ntfy backend NOT registered with whitespace-only URL",
			ntfyURL:     "   ",
			wantNtfy:    false,
			wantErrNtfy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ProjectConfig{}

			// Setup mock HTTP server for tests that need a valid URL
			var mockServer *httptest.Server
			if tt.ntfyURL == "mock" {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"id":"test","time":1234567890}`))
				}))
				defer mockServer.Close()
				cfg.Notify.NtfyURL = mockServer.URL
			} else {
				cfg.Notify.NtfyURL = tt.ntfyURL
			}

			provider := notify.NewNotifyOperationProvider(&mockLogger{})
			logger := &mockLogger{}

			err := registerNotifyBackends(provider, cfg, logger)
			require.NoError(t, err, "registerNotifyBackends should not return error")

			// Try to execute ntfy.send operation
			_, err = provider.Execute(context.TODO(), "notify.send", map[string]any{
				"backend": "ntfy",
				"message": "test message",
				"topic":   "test-topic",
			})

			if tt.wantNtfy {
				assert.NoError(t, err, "ntfy backend should be registered and executable")
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "backend \"ntfy\" not available", "should fail with backend not registered error")
			}
		})
	}
}

// TestRegisterNotifyBackends_SlackBackendConditional tests that slack backend
// is only registered when slack_webhook_url is configured.
func TestRegisterNotifyBackends_SlackBackendConditional(t *testing.T) {
	tests := []struct {
		name            string
		slackWebhookURL string
		wantSlack       bool
		wantErrSlack    bool
	}{
		{
			name:            "slack backend registered with valid webhook URL",
			slackWebhookURL: "mock", // Will be replaced with httptest server URL
			wantSlack:       true,
			wantErrSlack:    false,
		},
		{
			name:            "slack backend NOT registered with empty webhook URL",
			slackWebhookURL: "",
			wantSlack:       false,
			wantErrSlack:    true,
		},
		{
			name:            "slack backend NOT registered with whitespace-only webhook URL",
			slackWebhookURL: "   ",
			wantSlack:       false,
			wantErrSlack:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ProjectConfig{}

			// Setup mock HTTP server for tests that need a valid URL
			var mockServer *httptest.Server
			if tt.slackWebhookURL == "mock" {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("ok"))
				}))
				defer mockServer.Close()
				cfg.Notify.SlackWebhookURL = mockServer.URL
			} else {
				cfg.Notify.SlackWebhookURL = tt.slackWebhookURL
			}

			provider := notify.NewNotifyOperationProvider(&mockLogger{})
			logger := &mockLogger{}

			err := registerNotifyBackends(provider, cfg, logger)
			require.NoError(t, err, "registerNotifyBackends should not return error")

			// Try to execute slack backend
			_, err = provider.Execute(context.TODO(), "notify.send", map[string]any{
				"backend": "slack",
				"message": "test message",
			})

			if tt.wantSlack {
				assert.NoError(t, err, "slack backend should be registered and executable")
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "backend \"slack\" not available", "should fail with backend not registered error")
			}
		})
	}
}

// TestRegisterNotifyBackends_DefaultBackendConfiguration tests that the default
// backend is set correctly based on configuration.
func TestRegisterNotifyBackends_DefaultBackendConfiguration(t *testing.T) {
	// Enable test mode to avoid real desktop/network calls in CI
	originalTestMode := os.Getenv("AWF_TEST_MODE")
	os.Setenv("AWF_TEST_MODE", "1")
	defer func() {
		if originalTestMode != "" {
			os.Setenv("AWF_TEST_MODE", originalTestMode)
		} else {
			os.Unsetenv("AWF_TEST_MODE")
		}
	}()

	tests := []struct {
		name           string
		defaultBackend string
		wantSet        bool
		executeBackend string // backend to use in Execute call (empty = no backend input)
		wantExecuteOK  bool   // should Execute succeed?
	}{
		{
			name:           "default backend set to desktop",
			defaultBackend: "desktop",
			wantSet:        true,
			executeBackend: "", // omit backend input
			wantExecuteOK:  true,
		},
		{
			name:           "default backend set to webhook",
			defaultBackend: "webhook",
			wantSet:        true,
			executeBackend: "",    // omit backend input
			wantExecuteOK:  false, // webhook requires webhook_url input
		},
		{
			name:           "no default backend configured",
			defaultBackend: "",
			wantSet:        false,
			executeBackend: "",    // omit backend input
			wantExecuteOK:  false, // no backend selected
		},
		{
			name:           "default backend overridden by explicit input",
			defaultBackend: "desktop",
			wantSet:        true,
			executeBackend: "webhook", // explicit backend
			wantExecuteOK:  false,     // webhook requires webhook_url
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ProjectConfig{}
			cfg.Notify.DefaultBackend = tt.defaultBackend

			provider := notify.NewNotifyOperationProvider(&mockLogger{})
			logger := &mockLogger{}

			err := registerNotifyBackends(provider, cfg, logger)
			require.NoError(t, err, "registerNotifyBackends should not return error")

			// Build execution inputs
			inputs := map[string]any{
				"message": "test message",
			}
			if tt.executeBackend != "" {
				inputs["backend"] = tt.executeBackend
			}

			// Note: webhook_url deliberately omitted for webhook cases
			// to test input validation ("webhook requires webhook_url").

			// Execute operation
			_, err = provider.Execute(context.TODO(), "notify.send", inputs)

			if tt.wantExecuteOK {
				assert.NoError(t, err, "Execute should succeed when default backend is properly configured")
			} else {
				assert.Error(t, err, "Execute should fail when backend requirements not met")
			}
		})
	}
}

// TestRegisterNotifyBackends_AllBackendsRegistered tests that when all backends
// are configured, all four are registered and executable.
func TestRegisterNotifyBackends_AllBackendsRegistered(t *testing.T) {
	// Enable test mode to prevent actual desktop notifications
	originalTestMode := os.Getenv("AWF_TEST_MODE")
	os.Setenv("AWF_TEST_MODE", "1")
	defer func() {
		if originalTestMode != "" {
			os.Setenv("AWF_TEST_MODE", originalTestMode)
		} else {
			os.Unsetenv("AWF_TEST_MODE")
		}
	}()

	// Setup mock HTTP servers for ntfy and slack
	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"test","time":1234567890}`))
	}))
	defer ntfyServer.Close()

	slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer slackServer.Close()

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer webhookServer.Close()

	cfg := &config.ProjectConfig{}
	cfg.Notify.NtfyURL = ntfyServer.URL
	cfg.Notify.SlackWebhookURL = slackServer.URL
	cfg.Notify.DefaultBackend = "desktop"

	provider := notify.NewNotifyOperationProvider(&mockLogger{})
	logger := &mockLogger{}

	err := registerNotifyBackends(provider, cfg, logger)
	require.NoError(t, err, "registerNotifyBackends should succeed with all configs set")

	// Test desktop backend
	_, err = provider.Execute(context.TODO(), "notify.send", map[string]any{
		"backend": "desktop",
		"message": "desktop test",
	})
	assert.NoError(t, err, "desktop backend should be registered")

	// Test ntfy backend
	_, err = provider.Execute(context.TODO(), "notify.send", map[string]any{
		"backend": "ntfy",
		"message": "ntfy test",
		"topic":   "test-topic",
	})
	assert.NoError(t, err, "ntfy backend should be registered")

	// Test slack backend
	_, err = provider.Execute(context.TODO(), "notify.send", map[string]any{
		"backend": "slack",
		"message": "slack test",
	})
	assert.NoError(t, err, "slack backend should be registered")

	// Test webhook backend
	_, err = provider.Execute(context.TODO(), "notify.send", map[string]any{
		"backend":     "webhook",
		"message":     "webhook test",
		"webhook_url": webhookServer.URL,
	})
	assert.NoError(t, err, "webhook backend should be registered")
}

// TestRegisterNotifyBackends_NilProvider tests that the function handles nil provider gracefully.
func TestRegisterNotifyBackends_NilProvider(t *testing.T) {
	cfg := &config.ProjectConfig{}
	logger := &mockLogger{}

	// This should return error or panic - nil provider is invalid
	// The implementation should validate provider != nil
	err := registerNotifyBackends(nil, cfg, logger)
	// Either panic or error is acceptable - just need to handle the nil case
	// For now, expect error
	assert.Error(t, err, "should return error with nil provider")
}

// TestRegisterNotifyBackends_NilConfig tests that the function handles nil config gracefully.
func TestRegisterNotifyBackends_NilConfig(t *testing.T) {
	provider := notify.NewNotifyOperationProvider(&mockLogger{})
	logger := &mockLogger{}

	// Should return error when config is nil
	err := registerNotifyBackends(provider, nil, logger)
	assert.Error(t, err, "should return error with nil config")
}

// mockLogger is defined in plugins_internal_test.go and reused here.
