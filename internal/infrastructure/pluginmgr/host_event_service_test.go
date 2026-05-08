package pluginmgr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmitEventPublisher captures published events and can simulate errors
type mockEmitEventPublisher struct {
	publishedEvents []*pluginmodel.DomainEvent
	publishError    error
}

func (m *mockEmitEventPublisher) Publish(ctx context.Context, event *pluginmodel.DomainEvent) error {
	if m.publishError != nil {
		return m.publishError
	}
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

func (m *mockEmitEventPublisher) Close() error {
	return nil
}

// mockEmitLogger discards log messages
type mockEmitLogger struct{}

func (m *mockEmitLogger) Debug(msg string, fields ...any)             {}
func (m *mockEmitLogger) Info(msg string, fields ...any)              {}
func (m *mockEmitLogger) Warn(msg string, fields ...any)              {}
func (m *mockEmitLogger) Error(msg string, fields ...any)             {}
func (m *mockEmitLogger) WithContext(ctx map[string]any) ports.Logger { return m }

// manifestLookup returns PluginInfo for test plugins
func manifestLookup(name string) (*pluginmodel.PluginInfo, bool) {
	switch name {
	case "authorized-plugin":
		return &pluginmodel.PluginInfo{
			Manifest: &pluginmodel.Manifest{
				Name: "authorized-plugin",
				Events: pluginmodel.ManifestEvents{
					Emit: []string{"custom.analysis.*", "custom.export.complete"},
				},
				Capabilities: []string{pluginmodel.CapabilityEvents},
			},
		}, true

	case "no-emit-plugin":
		return &pluginmodel.PluginInfo{
			Manifest: &pluginmodel.Manifest{
				Name: "no-emit-plugin",
				Events: pluginmodel.ManifestEvents{
					Emit: []string{},
				},
				Capabilities: []string{pluginmodel.CapabilityEvents},
			},
		}, true

	case "no-events-capability":
		return &pluginmodel.PluginInfo{
			Manifest: &pluginmodel.Manifest{
				Name:         "no-events-capability",
				Capabilities: []string{pluginmodel.CapabilityOperations},
				Events: pluginmodel.ManifestEvents{
					Emit: []string{"custom.event"},
				},
			},
		}, true

	default:
		return nil, false
	}
}

func TestNewHostEventService_Constructor(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}

	service := newHostEventService(publisher, manifestLookup, logger)

	assert.NotNil(t, service)
}

func TestHostEventService_Emit_ValidPattern(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "custom.analysis.complete",
		SourcePlugin: "authorized-plugin",
		Payload:      []byte("test payload"),
		Metadata:     map[string]string{"key": "value"},
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.EventId)
	assert.Empty(t, resp.ErrorMessage)
	assert.Len(t, publisher.publishedEvents, 1)

	publishedEvent := publisher.publishedEvents[0]
	assert.Equal(t, "custom.analysis.complete", publishedEvent.Type)
	assert.Equal(t, "authorized-plugin", publishedEvent.Source)
	assert.Equal(t, []byte("test payload"), publishedEvent.Payload)
	assert.Equal(t, map[string]string{"key": "value"}, publishedEvent.Metadata)
}

func TestHostEventService_Emit_UndeclaredEventType(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "undeclared.event.type",
		SourcePlugin: "authorized-plugin",
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "not authorized")
	assert.Empty(t, resp.EventId)
	assert.Len(t, publisher.publishedEvents, 0)
}

func TestHostEventService_Emit_UnknownPlugin(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "custom.event",
		SourcePlugin: "unknown-plugin",
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "not authorized")
	assert.Empty(t, resp.EventId)
	assert.Len(t, publisher.publishedEvents, 0)
}

func TestHostEventService_Emit_NoEventsCapability(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "custom.event",
		SourcePlugin: "no-events-capability",
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "not authorized")
	assert.Empty(t, resp.EventId)
	assert.Len(t, publisher.publishedEvents, 0)
}

func TestHostEventService_Emit_NoEmitPermission(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "custom.event",
		SourcePlugin: "no-emit-plugin",
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "not authorized")
	assert.Empty(t, resp.EventId)
	assert.Len(t, publisher.publishedEvents, 0)
}

func TestHostEventService_Emit_PublisherError(t *testing.T) {
	publisher := &mockEmitEventPublisher{
		publishError: errors.New("publisher unavailable"),
	}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "custom.analysis.complete",
		SourcePlugin: "authorized-plugin",
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Empty(t, resp.EventId)
}

func TestHostEventService_Emit_GRPCErrorAlwaysNil(t *testing.T) {
	tests := []struct {
		name           string
		eventType      string
		sourcePlugin   string
		publisherError error
		shouldSucceed  bool
	}{
		{
			name:          "success case",
			eventType:     "custom.analysis.complete",
			sourcePlugin:  "authorized-plugin",
			shouldSucceed: true,
		},
		{
			name:          "permission denied",
			eventType:     "undeclared.event",
			sourcePlugin:  "authorized-plugin",
			shouldSucceed: false,
		},
		{
			name:          "unknown plugin",
			eventType:     "custom.event",
			sourcePlugin:  "unknown-plugin",
			shouldSucceed: false,
		},
		{
			name:           "publisher error",
			eventType:      "custom.analysis.complete",
			sourcePlugin:   "authorized-plugin",
			publisherError: errors.New("network error"),
			shouldSucceed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &mockEmitEventPublisher{publishError: tt.publisherError}
			logger := &mockEmitLogger{}
			service := newHostEventService(publisher, manifestLookup, logger)

			req := &pluginv1.EmitRequest{
				EventType:    tt.eventType,
				SourcePlugin: tt.sourcePlugin,
			}

			resp, err := service.Emit(context.Background(), req)

			assert.NoError(t, err, "Emit should always return nil gRPC error")
			assert.NotNil(t, resp)
			assert.Equal(t, tt.shouldSucceed, resp.Success)
		})
	}
}

func TestHostEventService_EmitRequestToDomainEvent_AllFields(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	metadata := map[string]string{"trace_id": "123", "user": "alice"}
	payload := []byte("test payload content")
	timestamp := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	req := &pluginv1.EmitRequest{
		EventType:          "custom.analysis.complete",
		Payload:            payload,
		SourcePlugin:       "authorized-plugin",
		PropagationDepth:   2,
		TimestampUnixNanos: timestamp.UnixNano(),
		Metadata:           metadata,
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Success)
	require.Len(t, publisher.publishedEvents, 1)

	event := publisher.publishedEvents[0]
	assert.Equal(t, "custom.analysis.complete", event.Type)
	assert.Equal(t, payload, event.Payload)
	assert.Equal(t, "authorized-plugin", event.Source)
	assert.Equal(t, 2, event.PropagationDepth)
	assert.Equal(t, timestamp.Unix(), event.Timestamp.Unix())
	assert.Equal(t, metadata, event.Metadata)
}

func TestHostEventService_EmitRequestToDomainEvent_UseCurrentTimeWhenZero(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	beforeTime := time.Now()

	req := &pluginv1.EmitRequest{
		EventType:          "custom.analysis.complete",
		SourcePlugin:       "authorized-plugin",
		TimestampUnixNanos: 0, // Zero timestamp
	}

	resp, err := service.Emit(context.Background(), req)

	afterTime := time.Now()

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Success)
	require.Len(t, publisher.publishedEvents, 1)

	event := publisher.publishedEvents[0]
	assert.True(
		t,
		event.Timestamp.After(beforeTime.Add(-time.Second)) && event.Timestamp.Before(afterTime.Add(time.Second)),
		"Event timestamp should be close to current time",
	)
}

func TestHostEventService_EmitRequestToDomainEvent_EmptyMetadata(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "custom.analysis.complete",
		SourcePlugin: "authorized-plugin",
		Payload:      []byte("test"),
		Metadata:     map[string]string{},
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Len(t, publisher.publishedEvents, 1)

	event := publisher.publishedEvents[0]
	assert.NotNil(t, event.Metadata)
	assert.Len(t, event.Metadata, 0)
}

func TestHostEventService_EmitRequestToDomainEvent_NilMetadata(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "custom.analysis.complete",
		SourcePlugin: "authorized-plugin",
		Payload:      []byte("test"),
		Metadata:     nil,
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Len(t, publisher.publishedEvents, 1)

	event := publisher.publishedEvents[0]
	assert.Nil(t, event.Metadata)
}

func TestHostEventService_Emit_GlobPatternMatching(t *testing.T) {
	tests := []struct {
		name        string
		eventType   string
		shouldMatch bool
	}{
		{
			name:        "exact match",
			eventType:   "custom.analysis.complete",
			shouldMatch: true,
		},
		{
			name:        "wildcard match middle segment",
			eventType:   "custom.analysis.start",
			shouldMatch: true,
		},
		{
			name:        "wildcard match different suffix",
			eventType:   "custom.analysis.error",
			shouldMatch: true,
		},
		{
			name:        "explicit pattern match",
			eventType:   "custom.export.complete",
			shouldMatch: true,
		},
		{
			name:        "pattern mismatch - wrong prefix",
			eventType:   "internal.analysis.complete",
			shouldMatch: false,
		},
		{
			name:        "pattern mismatch - wrong segment count",
			eventType:   "custom.analysis",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &mockEmitEventPublisher{}
			logger := &mockEmitLogger{}
			service := newHostEventService(publisher, manifestLookup, logger)

			req := &pluginv1.EmitRequest{
				EventType:    tt.eventType,
				SourcePlugin: "authorized-plugin",
			}

			resp, err := service.Emit(context.Background(), req)

			require.NoError(t, err)
			assert.Equal(t, tt.shouldMatch, resp.Success)
			if tt.shouldMatch {
				assert.Len(t, publisher.publishedEvents, 1)
			} else {
				assert.Len(t, publisher.publishedEvents, 0)
			}
		})
	}
}

func TestHostEventService_EmitResponse_UUIDNotEmpty(t *testing.T) {
	publisher := &mockEmitEventPublisher{}
	logger := &mockEmitLogger{}
	service := newHostEventService(publisher, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "custom.analysis.complete",
		SourcePlugin: "authorized-plugin",
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.True(t, resp.Success)
	assert.NotEmpty(t, resp.EventId)
	assert.Len(t, resp.EventId, 36) // UUID format: 8-4-4-4-12 = 36 chars with hyphens
}
