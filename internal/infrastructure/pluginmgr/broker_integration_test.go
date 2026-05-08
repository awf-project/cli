package pluginmgr

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingStreamSender captures all sent stream messages for verification.
type recordingStreamSender struct {
	mu       sync.Mutex
	messages []*pluginv1.EventStreamMessage
}

func (r *recordingStreamSender) Send(msg *pluginv1.EventStreamMessage) error {
	r.mu.Lock()
	r.messages = append(r.messages, msg)
	r.mu.Unlock()
	return nil
}

func (r *recordingStreamSender) getMessages() []*pluginv1.EventStreamMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*pluginv1.EventStreamMessage, len(r.messages))
	copy(out, r.messages)
	return out
}

func TestBrokerIntegration_EmitAndReceive(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	received := make(chan *pluginmodel.DomainEvent, 1)
	receiverDeliverer := &mockEventDeliverer{
		deliverFunc: func(_ context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			received <- event
			return nil, nil
		},
	}
	// custom.*.*  matches 3-segment types like custom.analysis.complete
	bus.Subscribe("receiver-plugin", []string{"custom.*.*"}, receiverDeliverer)

	service := newHostEventService(bus, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "custom.analysis.complete",
		SourcePlugin: "authorized-plugin",
		Payload:      []byte("analysis result"),
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	require.True(t, resp.Success)

	select {
	case event := <-received:
		assert.Equal(t, "custom.analysis.complete", event.Type)
		assert.Equal(t, "authorized-plugin", event.Source)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("receiver did not receive event within timeout")
	}
}

func TestBrokerIntegration_EmitDenied_DoesNotRoute(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	receiverDeliverer := &mockEventDeliverer{}
	bus.Subscribe("receiver-plugin", []string{"*"}, receiverDeliverer)

	service := newHostEventService(bus, manifestLookup, logger)

	req := &pluginv1.EmitRequest{
		EventType:    "undeclared.event.type",
		SourcePlugin: "authorized-plugin",
	}

	resp, err := service.Emit(context.Background(), req)

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "not authorized")

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, receiverDeliverer.getCallCount())
}

func TestBrokerIntegration_StreamDelivery_100Events(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	sm := NewStreamManager(logger)
	recorder := &recordingStreamSender{}
	sm.RegisterStream("stream-plugin", recorder)

	fallback := &mockEventDeliverer{}
	deliverer := sm.GetDeliverer("stream-plugin", fallback)
	bus.Subscribe("stream-plugin", []string{"*"}, deliverer)

	ctx := context.Background()
	for i := range 100 {
		event := &pluginmodel.DomainEvent{
			ID:        fmt.Sprintf("evt-%d", i),
			Type:      "test.event",
			Timestamp: time.Now(),
		}
		require.NoError(t, bus.Publish(ctx, event))
	}

	time.Sleep(200 * time.Millisecond)

	msgs := recorder.getMessages()
	require.Len(t, msgs, 100)
	for i, msg := range msgs {
		assert.Equal(t, uint64(i+1), msg.SequenceNumber, "event index %d", i)
	}
}

func TestBrokerIntegration_StreamFallbackToUnary_Transparent(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	sm := NewStreamManager(logger)
	// No stream registered — GetDeliverer returns unaryFallback directly

	received := make(chan *pluginmodel.DomainEvent, 1)
	unaryDeliverer := &mockEventDeliverer{
		deliverFunc: func(_ context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			received <- event
			return nil, nil
		},
	}
	deliverer := sm.GetDeliverer("unary-plugin", unaryDeliverer)
	bus.Subscribe("unary-plugin", []string{"*"}, deliverer)

	event := &pluginmodel.DomainEvent{
		ID:        "evt-fallback",
		Type:      "test.event",
		Timestamp: time.Now(),
		Source:    "emitter-plugin",
	}

	require.NoError(t, bus.Publish(context.Background(), event))

	select {
	case got := <-received:
		assert.Equal(t, "evt-fallback", got.ID)
		assert.Equal(t, "emitter-plugin", got.Source)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("unary deliverer did not receive event")
	}
	assert.Equal(t, 1, unaryDeliverer.getCallCount())
}

func TestBrokerIntegration_GoroutineLeaks_50Cycles(t *testing.T) {
	baseline := runtime.NumGoroutine()

	for i := range 50 {
		logger := &noopLogger{}
		bus := NewEventBus(logger)
		sm := NewStreamManager(logger)

		pluginName := fmt.Sprintf("gc-plugin-%d", i)
		sm.RegisterStream(pluginName, &mockStreamEventsClient{})
		deliverer := sm.GetDeliverer(pluginName, &mockEventDeliverer{})
		bus.Subscribe(pluginName, []string{"test.*"}, deliverer)

		bus.Unsubscribe(pluginName)
		sm.UnregisterStream(pluginName)
		sm.Close()
		bus.Close() //nolint:errcheck // error from Close is irrelevant in GC leak test cleanup
	}

	time.Sleep(150 * time.Millisecond)

	after := runtime.NumGoroutine()
	assert.InDelta(t, baseline, after, 3)
}

func TestBrokerIntegration_BackwardCompatibility_F090Events(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	client := &mockEventClient{
		response: &pluginv1.HandleEventResponse{},
	}
	adapter := newGRPCEventAdapter(client, "legacy-plugin")
	bus.Subscribe("legacy-plugin", []string{"workflow.*"}, adapter)

	event := &pluginmodel.DomainEvent{
		ID:        "evt-legacy",
		Type:      "workflow.started",
		Timestamp: time.Now(),
		Source:    "workflow-service",
	}

	require.NoError(t, bus.Publish(context.Background(), event))
	time.Sleep(150 * time.Millisecond)

	req := client.getLastRequest()
	require.NotNil(t, req, "legacy-plugin should have received the event via HandleEvent")
	assert.Equal(t, "evt-legacy", req.GetId())
	assert.Equal(t, "workflow.started", req.GetType())
	assert.Equal(t, "workflow-service", req.GetSource())
}

func BenchmarkEventDelivery_Unary(b *testing.B) {
	client := &mockEventClient{
		response: &pluginv1.HandleEventResponse{},
	}
	adapter := newGRPCEventAdapter(client, "bench-plugin")

	event := &pluginmodel.DomainEvent{
		ID:        "bench-evt",
		Type:      "bench.event",
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for range b.N {
		_, _ = adapter.DeliverEvent(context.Background(), event)
	}
}

func BenchmarkEventDelivery_Stream(b *testing.B) {
	sm := NewStreamManager(&noopLogger{})
	client := &mockStreamEventsClient{}
	sm.RegisterStream("bench-plugin", client)
	defer sm.Close()

	fallback := &mockEventDeliverer{}
	deliverer := sm.GetDeliverer("bench-plugin", fallback)

	event := &pluginmodel.DomainEvent{
		ID:        "bench-evt",
		Type:      "bench.event",
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for range b.N {
		_, _ = deliverer.DeliverEvent(context.Background(), event)
	}
}
