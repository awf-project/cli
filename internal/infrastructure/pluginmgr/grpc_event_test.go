package pluginmgr

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// mockEventClient is a minimal manual mock of pluginv1.EventServiceClient for unit testing.
type mockEventClient struct {
	mu          sync.Mutex
	lastRequest *pluginv1.HandleEventRequest
	response    *pluginv1.HandleEventResponse
	err         error
}

func (m *mockEventClient) HandleEvent(_ context.Context, in *pluginv1.HandleEventRequest, _ ...grpc.CallOption) (*pluginv1.HandleEventResponse, error) {
	m.mu.Lock()
	m.lastRequest = in
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil {
		return m.response, nil
	}
	return &pluginv1.HandleEventResponse{}, nil
}

func (m *mockEventClient) StreamEvents(_ context.Context, _ ...grpc.CallOption) (grpc.ClientStreamingClient[pluginv1.EventStreamMessage, pluginv1.StreamEventsResponse], error) {
	return nil, nil
}

func (m *mockEventClient) getLastRequest() *pluginv1.HandleEventRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastRequest
}

func TestDomainEventToProto_AllFields(t *testing.T) {
	ts := time.Unix(1700000000, 123456789)
	event := &pluginmodel.DomainEvent{
		ID:               "event-abc",
		Type:             "workflow.started",
		Timestamp:        ts,
		Source:           "workflow-service",
		Metadata:         map[string]string{"env": "prod"},
		Payload:          []byte(`{"step":"init"}`),
		PropagationDepth: 2,
	}

	req := domainEventToProto(event)

	require.NotNil(t, req)
	assert.Equal(t, "event-abc", req.GetId())
	assert.Equal(t, "workflow.started", req.GetType())
	assert.Equal(t, ts.UnixNano(), req.GetTimestampUnixNanos())
	assert.Equal(t, "workflow-service", req.GetSource())
	assert.Equal(t, map[string]string{"env": "prod"}, req.GetMetadata())
	assert.Equal(t, []byte(`{"step":"init"}`), req.GetPayload())
	assert.Equal(t, int32(2), req.GetPropagationDepth())
}

func TestProtoToDomainEvent_AllFields(t *testing.T) {
	ts := time.Unix(1700000000, 123456789)
	req := &pluginv1.HandleEventRequest{
		Id:                 "event-abc",
		Type:               "workflow.started",
		TimestampUnixNanos: ts.UnixNano(),
		Source:             "workflow-service",
		Metadata:           map[string]string{"env": "prod"},
		Payload:            []byte(`{"step":"init"}`),
		PropagationDepth:   2,
	}

	event := protoToDomainEvent(req)

	require.NotNil(t, event)
	assert.Equal(t, "event-abc", event.ID)
	assert.Equal(t, "workflow.started", event.Type)
	assert.Equal(t, ts.UnixNano(), event.Timestamp.UnixNano())
	assert.Equal(t, "workflow-service", event.Source)
	assert.Equal(t, map[string]string{"env": "prod"}, event.Metadata)
	assert.Equal(t, []byte(`{"step":"init"}`), event.Payload)
	assert.Equal(t, 2, event.PropagationDepth)
}

func TestGRPCEventAdapter_DeliverEvent_CallsHandleEventAndReturnsEmittedEvents(t *testing.T) {
	client := &mockEventClient{
		response: &pluginv1.HandleEventResponse{
			EmittedEvents: []*pluginv1.HandleEventRequest{
				{Id: "emitted-1", Type: "step.completed"},
				{Id: "emitted-2", Type: "step.completed"},
			},
		},
	}
	adapter := newGRPCEventAdapter(client, "test-plugin")
	inEvent := &pluginmodel.DomainEvent{
		ID:     "source-event",
		Type:   "workflow.started",
		Source: "test",
	}

	results, err := adapter.DeliverEvent(context.Background(), inEvent)

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "emitted-1", results[0].ID)
	assert.Equal(t, "step.completed", results[0].Type)
	lastReq := client.getLastRequest()
	require.NotNil(t, lastReq)
	assert.Equal(t, "source-event", lastReq.GetId())
}

func TestGRPCEventAdapter_DeliverEvent_PropagatesClientError(t *testing.T) {
	transportErr := errors.New("grpc transport failure")
	client := &mockEventClient{err: transportErr}
	adapter := newGRPCEventAdapter(client, "test-plugin")

	_, err := adapter.DeliverEvent(context.Background(), &pluginmodel.DomainEvent{
		ID:   "event-id",
		Type: "test.event",
	})

	require.Error(t, err)
}

func TestDomainEventToStreamMessage_AllFields(t *testing.T) {
	ts := time.Unix(1700000000, 123456789)
	event := &pluginmodel.DomainEvent{
		ID:               "stream-event-1",
		Type:             "workflow.started",
		Timestamp:        ts,
		Source:           "workflow-service",
		Metadata:         map[string]string{"env": "prod"},
		Payload:          []byte(`{"step":"init"}`),
		PropagationDepth: 3,
	}

	msg := domainEventToStreamMessage(event, 42)

	require.NotNil(t, msg)
	assert.Equal(t, "stream-event-1", msg.GetId())
	assert.Equal(t, "workflow.started", msg.GetType())
	assert.Equal(t, ts.UnixNano(), msg.GetTimestampUnixNanos())
	assert.Equal(t, "workflow-service", msg.GetSource())
	assert.Equal(t, map[string]string{"env": "prod"}, msg.GetMetadata())
	assert.Equal(t, []byte(`{"step":"init"}`), msg.GetPayload())
	assert.Equal(t, int32(3), msg.GetPropagationDepth())
	assert.Equal(t, uint64(42), msg.GetSequenceNumber())
}

func TestDomainEventToStreamMessage_ZeroSequenceNumber(t *testing.T) {
	event := &pluginmodel.DomainEvent{
		ID:   "event-zero",
		Type: "test.event",
	}

	msg := domainEventToStreamMessage(event, 0)

	require.NotNil(t, msg)
	assert.Equal(t, uint64(0), msg.GetSequenceNumber())
	assert.Equal(t, "event-zero", msg.GetId())
}

func TestWireEventSubscriptions_RegistersSubscriptionForEventsCapability(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	mgr := NewRPCPluginManager(NewFileSystemLoader(nil))
	mgr.SetEventBus(bus)

	conn := &pluginConnection{
		event: &mockEventClient{},
	}
	info := &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{
			Name:         "test-plugin",
			Capabilities: []string{pluginmodel.CapabilityEvents},
			Events:       pluginmodel.ManifestEvents{Subscribe: []string{"workflow.*"}},
		},
	}

	mgr.wireEventSubscriptions("test-plugin", conn, info)

	bus.mu.RLock()
	_, registered := bus.subscriptions["test-plugin"]
	bus.mu.RUnlock()
	assert.True(t, registered, "subscription should be registered in EventBus for events capability")
}
