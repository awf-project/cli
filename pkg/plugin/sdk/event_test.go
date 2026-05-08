package sdk

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// nonSubscriberPlugin implements Plugin but NOT EventSubscriber.
// Used to test the fallback path when the type assertion s.impl.(EventSubscriber) fails.
type nonSubscriberPlugin struct{}

func (p *nonSubscriberPlugin) Name() string                                   { return "non-subscriber" }
func (p *nonSubscriberPlugin) Version() string                                { return "1.0.0" }
func (p *nonSubscriberPlugin) Init(_ context.Context, _ map[string]any) error { return nil }
func (p *nonSubscriberPlugin) Shutdown(_ context.Context) error               { return nil }

// capturingSubscriberPlugin embeds BasePlugin and overrides EventSubscriber methods
// to capture what the server passes in, enabling assertion of dispatch behavior.
type capturingSubscriberPlugin struct {
	BasePlugin
	patterns      []string
	emittedEvents []Event
	handleErr     error
	lastEvent     Event
	handleCalled  bool
}

func (p *capturingSubscriberPlugin) Patterns() []string { return p.patterns }

func (p *capturingSubscriberPlugin) HandleEvent(_ context.Context, event Event) ([]Event, error) { //nolint:gocritic // hugeParam: satisfies EventSubscriber interface
	p.handleCalled = true
	p.lastEvent = event
	return p.emittedEvents, p.handleErr
}

func TestEvent_StructHasRequiredFields(t *testing.T) {
	now := time.Now().Truncate(time.Nanosecond)
	e := Event{
		ID:               "evt-001",
		Type:             "workflow.started",
		Timestamp:        now,
		Source:           "awf-core",
		Metadata:         map[string]string{"run_id": "xyz"},
		Payload:          []byte(`{"step":"init"}`),
		PropagationDepth: 2,
	}

	assert.Equal(t, "evt-001", e.ID)
	assert.Equal(t, "workflow.started", e.Type)
	assert.Equal(t, now, e.Timestamp)
	assert.Equal(t, "awf-core", e.Source)
	assert.Equal(t, map[string]string{"run_id": "xyz"}, e.Metadata)
	assert.Equal(t, []byte(`{"step":"init"}`), e.Payload)
	assert.Equal(t, 2, e.PropagationDepth)
}

func TestEventSubscriber_InterfaceCompliance(t *testing.T) {
	var _ EventSubscriber = (*BasePlugin)(nil)
	var _ EventSubscriber = (*capturingSubscriberPlugin)(nil)
}

func TestBasePlugin_Patterns_ReturnsNil(t *testing.T) {
	p := &BasePlugin{}

	patterns := p.Patterns()

	assert.Nil(t, patterns)
}

func TestBasePlugin_HandleEvent_ReturnsNilNil(t *testing.T) {
	p := &BasePlugin{}

	emitted, err := p.HandleEvent(context.Background(), Event{ID: "ignored"})

	assert.NoError(t, err)
	assert.Nil(t, emitted)
}

func TestCustomPlugin_CanOverridePatterns(t *testing.T) {
	p := &capturingSubscriberPlugin{
		patterns: []string{"workflow.*", "step.completed"},
	}

	assert.Equal(t, []string{"workflow.*", "step.completed"}, p.Patterns())
}

func TestCustomPlugin_CanOverrideHandleEvent(t *testing.T) {
	want := []Event{{ID: "emitted-1", Type: "downstream.event"}}
	p := &capturingSubscriberPlugin{emittedEvents: want}

	got, err := p.HandleEvent(context.Background(), Event{ID: "input"})

	require.NoError(t, err)
	assert.True(t, p.handleCalled)
	assert.Equal(t, want, got)
}

func TestEventServiceServer_HandleEvent_DispatchesToSubscriber(t *testing.T) {
	p := &capturingSubscriberPlugin{patterns: []string{"workflow.*"}}
	server := &eventServiceServer{impl: p}

	req := &pluginv1.HandleEventRequest{
		Id:   "evt-dispatch",
		Type: "workflow.started",
	}

	resp, err := server.HandleEvent(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, p.handleCalled, "plugin HandleEvent must be called when EventSubscriber is implemented")
}

func TestEventServiceServer_HandleEvent_ReturnsEmptyWhenNotSubscriber(t *testing.T) {
	p := &nonSubscriberPlugin{}
	server := &eventServiceServer{impl: p}

	req := &pluginv1.HandleEventRequest{
		Id:   "evt-no-sub",
		Type: "step.completed",
	}

	resp, err := server.HandleEvent(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.EmittedEvents)
}

func TestEventServiceServer_HandleEvent_ConvertsProtoFieldsToEvent(t *testing.T) {
	nanos := int64(1_700_000_000_000_000_000)
	p := &capturingSubscriberPlugin{}
	server := &eventServiceServer{impl: p}

	req := &pluginv1.HandleEventRequest{
		Id:                 "evt-789",
		Type:               "step.completed",
		TimestampUnixNanos: nanos,
		Source:             "plugin-x",
		Metadata:           map[string]string{"run_id": "abc123"},
		Payload:            []byte(`{"status":"ok"}`),
		PropagationDepth:   3,
	}

	_, err := server.HandleEvent(context.Background(), req)

	require.NoError(t, err)
	require.True(t, p.handleCalled)
	assert.Equal(t, "evt-789", p.lastEvent.ID)
	assert.Equal(t, "step.completed", p.lastEvent.Type)
	assert.Equal(t, time.Unix(0, nanos), p.lastEvent.Timestamp)
	assert.Equal(t, "plugin-x", p.lastEvent.Source)
	assert.Equal(t, map[string]string{"run_id": "abc123"}, p.lastEvent.Metadata)
	assert.Equal(t, []byte(`{"status":"ok"}`), p.lastEvent.Payload)
	assert.Equal(t, 3, p.lastEvent.PropagationDepth)
}

func TestEventServiceServer_HandleEvent_ConvertsEmittedEventsToProto(t *testing.T) {
	nanos := time.Now().UnixNano()
	emitted := []Event{
		{
			ID:               "emitted-1",
			Type:             "downstream.triggered",
			Timestamp:        time.Unix(0, nanos),
			Source:           "plugin-x",
			Metadata:         map[string]string{"key": "val"},
			Payload:          []byte(`{}`),
			PropagationDepth: 1,
		},
	}
	p := &capturingSubscriberPlugin{emittedEvents: emitted}
	server := &eventServiceServer{impl: p}

	resp, err := server.HandleEvent(context.Background(), &pluginv1.HandleEventRequest{Id: "trigger"})

	require.NoError(t, err)
	require.Len(t, resp.EmittedEvents, 1)
	got := resp.EmittedEvents[0]
	assert.Equal(t, "emitted-1", got.Id)
	assert.Equal(t, "downstream.triggered", got.Type)
	assert.Equal(t, nanos, got.TimestampUnixNanos)
	assert.Equal(t, "plugin-x", got.Source)
	assert.Equal(t, map[string]string{"key": "val"}, got.Metadata)
	assert.Equal(t, []byte(`{}`), got.Payload)
	assert.Equal(t, int32(1), got.PropagationDepth)
}

// mockStreamEventsServer implements EventService_StreamEventsServer for testing.
type mockStreamEventsServer struct {
	messages []*pluginv1.EventStreamMessage
	pos      int
	recvErr  error
	closed   bool
	closeErr error
	ctx      context.Context
}

func (m *mockStreamEventsServer) Recv() (*pluginv1.EventStreamMessage, error) {
	if m.pos < len(m.messages) {
		msg := m.messages[m.pos]
		m.pos++
		return msg, nil
	}
	if m.recvErr != nil {
		return nil, m.recvErr
	}
	return nil, io.EOF
}

func (m *mockStreamEventsServer) SendAndClose(_ *pluginv1.StreamEventsResponse) error {
	m.closed = true
	return m.closeErr
}

func (m *mockStreamEventsServer) SetHeader(metadata.MD) error  { return nil }
func (m *mockStreamEventsServer) SendHeader(metadata.MD) error { return nil }
func (m *mockStreamEventsServer) SetTrailer(metadata.MD)       {}
func (m *mockStreamEventsServer) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}
func (m *mockStreamEventsServer) SendMsg(any) error { return nil }
func (m *mockStreamEventsServer) RecvMsg(any) error { return nil }

func TestStreamEvents_ReturnsUnimplementedWhenNotSubscriber(t *testing.T) {
	p := &nonSubscriberPlugin{}
	server := &eventServiceServer{impl: p}
	stream := &mockStreamEventsServer{}

	err := server.StreamEvents(stream)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unimplemented, st.Code())
}

func TestStreamEvents_SendsAndClosesOnEOF(t *testing.T) {
	p := &capturingSubscriberPlugin{}
	server := &eventServiceServer{impl: p}
	stream := &mockStreamEventsServer{messages: []*pluginv1.EventStreamMessage{}}

	err := server.StreamEvents(stream)

	require.NoError(t, err)
	assert.True(t, stream.closed, "SendAndClose must be called on EOF")
}

func TestStreamEvents_DispatchesEachEventToSubscriber(t *testing.T) {
	p := &capturingSubscriberPlugin{}
	server := &eventServiceServer{impl: p}
	stream := &mockStreamEventsServer{
		messages: []*pluginv1.EventStreamMessage{
			{Id: "evt-1", Type: "workflow.started"},
			{Id: "evt-2", Type: "step.completed"},
		},
	}

	err := server.StreamEvents(stream)

	require.NoError(t, err)
	assert.True(t, p.handleCalled, "HandleEvent must be called for each received message")
	assert.Equal(t, "evt-2", p.lastEvent.ID, "last dispatched event must be the second message")
}

func TestStreamEvents_ConvertsMessageFieldsToEvent(t *testing.T) {
	nanos := int64(1_700_000_000_000_000_000)
	p := &capturingSubscriberPlugin{}
	server := &eventServiceServer{impl: p}
	stream := &mockStreamEventsServer{
		messages: []*pluginv1.EventStreamMessage{
			{
				Id:                 "evt-xyz",
				Type:               "step.completed",
				TimestampUnixNanos: nanos,
				Source:             "plugin-a",
				Metadata:           map[string]string{"run_id": "abc"},
				Payload:            []byte(`{"ok":true}`),
				PropagationDepth:   2,
			},
		},
	}

	err := server.StreamEvents(stream)

	require.NoError(t, err)
	require.True(t, p.handleCalled)
	assert.Equal(t, "evt-xyz", p.lastEvent.ID)
	assert.Equal(t, "step.completed", p.lastEvent.Type)
	assert.Equal(t, time.Unix(0, nanos), p.lastEvent.Timestamp)
	assert.Equal(t, "plugin-a", p.lastEvent.Source)
	assert.Equal(t, map[string]string{"run_id": "abc"}, p.lastEvent.Metadata)
	assert.Equal(t, []byte(`{"ok":true}`), p.lastEvent.Payload)
	assert.Equal(t, 2, p.lastEvent.PropagationDepth)
}

func TestStreamEvents_PropagatesNonEOFRecvError(t *testing.T) {
	p := &capturingSubscriberPlugin{}
	server := &eventServiceServer{impl: p}
	recvErr := errors.New("transport error")
	stream := &mockStreamEventsServer{recvErr: recvErr}

	err := server.StreamEvents(stream)

	assert.ErrorIs(t, err, recvErr)
	assert.False(t, stream.closed, "SendAndClose must not be called on transport error")
}

func TestGRPCServer_RegistersEventService(t *testing.T) {
	p := &testPlugin{BasePlugin{PluginName: "test", PluginVersion: "1.0.0"}}
	bridge := &GRPCPluginBridge{impl: p}

	server := grpc.NewServer()
	defer server.Stop()

	err := bridge.GRPCServer(nil, server)
	require.NoError(t, err)

	info := server.GetServiceInfo()
	_, hasEventService := info["plugin.v1.EventService"]
	assert.True(t, hasEventService, "GRPCServer must register plugin.v1.EventService")
}
