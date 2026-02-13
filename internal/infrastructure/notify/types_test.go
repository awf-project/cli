package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NotificationPayload tests ---

func TestNotificationPayload_Construction(t *testing.T) {
	tests := []struct {
		name     string
		payload  NotificationPayload
		validate func(t *testing.T, p NotificationPayload)
	}{
		{
			name: "minimal_payload_with_required_fields",
			payload: NotificationPayload{
				Message: "Workflow completed",
			},
			validate: func(t *testing.T, p NotificationPayload) {
				assert.Equal(t, "Workflow completed", p.Message)
				assert.Empty(t, p.Title, "title should be empty when not provided")
				assert.Empty(t, p.Priority, "priority should be empty when not provided")
				assert.Nil(t, p.Metadata, "metadata should be nil when not provided")
			},
		},
		{
			name: "full_payload_with_all_fields",
			payload: NotificationPayload{
				Title:    "Build Status",
				Message:  "Build succeeded in 2m30s",
				Priority: "high",
				Metadata: map[string]string{
					"workflow": "ci-pipeline",
					"status":   "success",
					"duration": "2m30s",
				},
			},
			validate: func(t *testing.T, p NotificationPayload) {
				assert.Equal(t, "Build Status", p.Title)
				assert.Equal(t, "Build succeeded in 2m30s", p.Message)
				assert.Equal(t, "high", p.Priority)
				assert.Len(t, p.Metadata, 3)
				assert.Equal(t, "ci-pipeline", p.Metadata["workflow"])
				assert.Equal(t, "success", p.Metadata["status"])
				assert.Equal(t, "2m30s", p.Metadata["duration"])
			},
		},
		{
			name: "payload_with_empty_metadata",
			payload: NotificationPayload{
				Message:  "Test notification",
				Metadata: map[string]string{},
			},
			validate: func(t *testing.T, p NotificationPayload) {
				assert.Equal(t, "Test notification", p.Message)
				assert.NotNil(t, p.Metadata, "metadata should be initialized but empty")
				assert.Len(t, p.Metadata, 0)
			},
		},
		{
			name: "payload_with_priority_variations",
			payload: NotificationPayload{
				Message:  "Priority test",
				Priority: "low",
			},
			validate: func(t *testing.T, p NotificationPayload) {
				assert.Equal(t, "low", p.Priority)
			},
		},
		{
			name: "payload_with_unicode_characters",
			payload: NotificationPayload{
				Title:   "🎉 Success",
				Message: "Build completed successfully! ✓",
			},
			validate: func(t *testing.T, p NotificationPayload) {
				assert.Equal(t, "🎉 Success", p.Title)
				assert.Contains(t, p.Message, "✓")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, tt.payload)
		})
	}
}

func TestNotificationPayload_EmptyMessage(t *testing.T) {
	// Edge case: empty message (validation should happen in provider, not type)
	payload := NotificationPayload{
		Title:   "Some Title",
		Message: "",
	}

	assert.Empty(t, payload.Message, "empty message should be allowed at type level")
}

func TestNotificationPayload_LongMessage(t *testing.T) {
	// Edge case: very long message
	longMessage := make([]byte, 10000)
	for i := range longMessage {
		longMessage[i] = 'a'
	}

	payload := NotificationPayload{
		Message: string(longMessage),
	}

	assert.Len(t, payload.Message, 10000, "should handle long messages")
}

func TestNotificationPayload_MetadataModification(t *testing.T) {
	// Test that metadata is modifiable (not immutable at type level)
	payload := NotificationPayload{
		Message: "Test",
		Metadata: map[string]string{
			"key1": "value1",
		},
	}

	payload.Metadata["key2"] = "value2"

	assert.Len(t, payload.Metadata, 2, "metadata should be modifiable")
	assert.Equal(t, "value2", payload.Metadata["key2"])
}

func TestNotificationPayload_NilMetadata(t *testing.T) {
	// Edge case: nil metadata
	payload := NotificationPayload{
		Message:  "Test",
		Metadata: nil,
	}

	assert.Nil(t, payload.Metadata)
	// Should not panic when accessed
	_, exists := payload.Metadata["key"]
	assert.False(t, exists)
}

// --- BackendResult tests ---

func TestBackendResult_Construction(t *testing.T) {
	tests := []struct {
		name     string
		result   BackendResult
		validate func(t *testing.T, r BackendResult)
	}{
		{
			name: "desktop_backend_result_with_zero_status_code",
			result: BackendResult{
				Backend:    "desktop",
				StatusCode: 0,
				Response:   "Notification sent via notify-send",
			},
			validate: func(t *testing.T, r BackendResult) {
				assert.Equal(t, "desktop", r.Backend)
				assert.Equal(t, 0, r.StatusCode, "desktop backend should use 0 status code")
				assert.Contains(t, r.Response, "notify-send")
			},
		},
		{
			name: "http_backend_result_with_success_status",
			result: BackendResult{
				Backend:    "ntfy",
				StatusCode: 200,
				Response:   `{"id":"abc123","time":1234567890}`,
			},
			validate: func(t *testing.T, r BackendResult) {
				assert.Equal(t, "ntfy", r.Backend)
				assert.Equal(t, 200, r.StatusCode)
				assert.Contains(t, r.Response, "abc123")
			},
		},
		{
			name: "http_backend_result_with_error_status",
			result: BackendResult{
				Backend:    "webhook",
				StatusCode: 500,
				Response:   "Internal Server Error",
			},
			validate: func(t *testing.T, r BackendResult) {
				assert.Equal(t, "webhook", r.Backend)
				assert.Equal(t, 500, r.StatusCode)
				assert.Equal(t, "Internal Server Error", r.Response)
			},
		},
		{
			name: "slack_backend_result_with_ok_response",
			result: BackendResult{
				Backend:    "slack",
				StatusCode: 200,
				Response:   "ok",
			},
			validate: func(t *testing.T, r BackendResult) {
				assert.Equal(t, "slack", r.Backend)
				assert.Equal(t, 200, r.StatusCode)
				assert.Equal(t, "ok", r.Response)
			},
		},
		{
			name: "backend_result_with_empty_response",
			result: BackendResult{
				Backend:    "ntfy",
				StatusCode: 204,
				Response:   "",
			},
			validate: func(t *testing.T, r BackendResult) {
				assert.Equal(t, "ntfy", r.Backend)
				assert.Equal(t, 204, r.StatusCode)
				assert.Empty(t, r.Response)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, tt.result)
		})
	}
}

func TestBackendResult_StatusCodeRanges(t *testing.T) {
	// Edge case: various HTTP status codes
	tests := []struct {
		name       string
		statusCode int
	}{
		{"success_200", 200},
		{"created_201", 201},
		{"accepted_202", 202},
		{"no_content_204", 204},
		{"bad_request_400", 400},
		{"unauthorized_401", 401},
		{"forbidden_403", 403},
		{"not_found_404", 404},
		{"internal_error_500", 500},
		{"bad_gateway_502", 502},
		{"service_unavailable_503", 503},
		{"gateway_timeout_504", 504},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BackendResult{
				Backend:    "test",
				StatusCode: tt.statusCode,
				Response:   "test response",
			}

			assert.Equal(t, tt.statusCode, result.StatusCode)
		})
	}
}

func TestBackendResult_LongResponse(t *testing.T) {
	// Edge case: very long response body
	longResponse := make([]byte, 100000)
	for i := range longResponse {
		longResponse[i] = 'x'
	}

	result := BackendResult{
		Backend:    "webhook",
		StatusCode: 200,
		Response:   string(longResponse),
	}

	assert.Len(t, result.Response, 100000, "should handle long responses")
}

func TestBackendResult_SpecialCharactersInResponse(t *testing.T) {
	// Edge case: special characters in response
	result := BackendResult{
		Backend:    "webhook",
		StatusCode: 200,
		Response:   `{"message":"Test with \"quotes\" and\nnewlines"}`,
	}

	assert.Contains(t, result.Response, `\"quotes\"`)
	assert.Contains(t, result.Response, `\n`)
}

// --- Backend interface tests ---

func TestBackend_InterfaceContract(t *testing.T) {
	// Ensure mockBackend implements Backend interface
	var _ Backend = (*mockBackend)(nil)
}

func TestBackend_SendMethodSignature(t *testing.T) {
	// Test that Send method accepts context and payload
	backend := &mockBackend{
		result: &BackendResult{
			Backend:    "mock",
			StatusCode: 200,
			Response:   "success",
		},
		err: nil,
	}

	ctx := context.Background()
	payload := NotificationPayload{
		Message: "test",
	}

	result, err := backend.Send(ctx, payload)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "mock", result.Backend)
}

func TestBackend_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		backend     Backend
		ctx         context.Context
		payload     NotificationPayload
		expectedErr bool
		errCheck    func(t *testing.T, err error)
	}{
		{
			name: "backend_returns_error",
			backend: &mockBackend{
				result: nil,
				err:    errors.New("connection timeout"),
			},
			ctx: context.Background(),
			payload: NotificationPayload{
				Message: "test",
			},
			expectedErr: true,
			errCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "connection timeout")
			},
		},
		{
			name: "backend_returns_nil_result_with_error",
			backend: &mockBackend{
				result: nil,
				err:    errors.New("backend unavailable"),
			},
			ctx: context.Background(),
			payload: NotificationPayload{
				Message: "test",
			},
			expectedErr: true,
			errCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "backend unavailable")
			},
		},
		{
			name: "context_canceled_before_send",
			backend: &mockBackend{
				result: nil,
				err:    context.Canceled,
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			payload: NotificationPayload{
				Message: "test",
			},
			expectedErr: true,
			errCheck: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, context.Canceled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.backend.Send(tt.ctx, tt.payload)

			if tt.expectedErr {
				require.Error(t, err)
				assert.Nil(t, result)
				if tt.errCheck != nil {
					tt.errCheck(t, err)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

// --- NotifyConfig tests ---

func TestNotifyConfig_Construction(t *testing.T) {
	tests := []struct {
		name     string
		config   NotifyConfig
		validate func(t *testing.T, c NotifyConfig)
	}{
		{
			name:   "empty_config",
			config: NotifyConfig{},
			validate: func(t *testing.T, c NotifyConfig) {
				assert.Empty(t, c.NtfyURL)
				assert.Empty(t, c.SlackWebhookURL)
				assert.Empty(t, c.DefaultBackend)
			},
		},
		{
			name: "ntfy_config_only",
			config: NotifyConfig{
				NtfyURL: "https://ntfy.sh",
			},
			validate: func(t *testing.T, c NotifyConfig) {
				assert.Equal(t, "https://ntfy.sh", c.NtfyURL)
				assert.Empty(t, c.SlackWebhookURL)
				assert.Empty(t, c.DefaultBackend)
			},
		},
		{
			name: "slack_config_only",
			config: NotifyConfig{
				SlackWebhookURL: "https://hooks.slack.com/services/XXX/YYY/ZZZ",
			},
			validate: func(t *testing.T, c NotifyConfig) {
				assert.Empty(t, c.NtfyURL)
				assert.Equal(t, "https://hooks.slack.com/services/XXX/YYY/ZZZ", c.SlackWebhookURL)
				assert.Empty(t, c.DefaultBackend)
			},
		},
		{
			name: "default_backend_only",
			config: NotifyConfig{
				DefaultBackend: "desktop",
			},
			validate: func(t *testing.T, c NotifyConfig) {
				assert.Empty(t, c.NtfyURL)
				assert.Empty(t, c.SlackWebhookURL)
				assert.Equal(t, "desktop", c.DefaultBackend)
			},
		},
		{
			name: "full_config",
			config: NotifyConfig{
				NtfyURL:         "https://ntfy.example.com",
				SlackWebhookURL: "https://hooks.slack.com/services/ABC/DEF/GHI",
				DefaultBackend:  "ntfy",
			},
			validate: func(t *testing.T, c NotifyConfig) {
				assert.Equal(t, "https://ntfy.example.com", c.NtfyURL)
				assert.Equal(t, "https://hooks.slack.com/services/ABC/DEF/GHI", c.SlackWebhookURL)
				assert.Equal(t, "ntfy", c.DefaultBackend)
			},
		},
		{
			name: "self_hosted_ntfy_url",
			config: NotifyConfig{
				NtfyURL:        "http://localhost:8080",
				DefaultBackend: "ntfy",
			},
			validate: func(t *testing.T, c NotifyConfig) {
				assert.Equal(t, "http://localhost:8080", c.NtfyURL)
				assert.Equal(t, "ntfy", c.DefaultBackend)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, tt.config)
		})
	}
}

func TestNotifyConfig_DefaultBackendValues(t *testing.T) {
	// Edge case: test all valid backend values
	backends := []string{"desktop", "ntfy", "slack", "webhook"}

	for _, backend := range backends {
		t.Run(backend, func(t *testing.T) {
			config := NotifyConfig{
				DefaultBackend: backend,
			}

			assert.Equal(t, backend, config.DefaultBackend)
		})
	}
}

func TestNotifyConfig_InvalidBackendValue(t *testing.T) {
	// Edge case: invalid backend value (validation happens in provider, not type)
	config := NotifyConfig{
		DefaultBackend: "invalid-backend",
	}

	assert.Equal(t, "invalid-backend", config.DefaultBackend, "type should accept any string")
}

func TestNotifyConfig_URLFormats(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"http_url", "http://example.com"},
		{"https_url", "https://example.com"},
		{"url_with_port", "https://example.com:8080"},
		{"url_with_path", "https://example.com/path/to/resource"},
		{"url_with_query", "https://example.com?key=value"},
		{"localhost_url", "http://localhost:3000"},
		{"ip_address_url", "http://192.168.1.1:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NotifyConfig{
				NtfyURL: tt.url,
			}

			assert.Equal(t, tt.url, config.NtfyURL)
		})
	}
}

// --- Mock implementations for testing ---

// mockBackend is a test double implementing the Backend interface
type mockBackend struct {
	result *BackendResult
	err    error
}

func (m *mockBackend) Send(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
	return m.result, m.err
}
