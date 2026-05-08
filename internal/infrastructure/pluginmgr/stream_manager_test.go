package pluginmgr

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// blockingMockStreamEventsClient is a mock whose Send blocks until done is closed.
type blockingMockStreamEventsClient struct {
	done chan struct{}
}

func (m *blockingMockStreamEventsClient) Send(_ *pluginv1.EventStreamMessage) error {
	<-m.done
	return nil
}

// mockStreamEventsClient implements EventStreamSender for unit testing.
type mockStreamEventsClient struct {
	mu              sync.Mutex
	messages        []*pluginv1.EventStreamMessage
	sendErr         error
	callCount       int
	shouldReturnNil bool
}

func (m *mockStreamEventsClient) Send(msg *pluginv1.EventStreamMessage) error {
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.callCount++
	m.mu.Unlock()

	if m.sendErr != nil {
		return m.sendErr
	}
	return nil
}

// getLastMessage pops the most recently sent message. This dequeue semantics ensures
// that concurrent goroutines each retrieve a distinct message when verifying unique sequence numbers.
func (m *mockStreamEventsClient) getLastMessage() *pluginv1.EventStreamMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) == 0 {
		return nil
	}
	msg := m.messages[len(m.messages)-1]
	m.messages = m.messages[:len(m.messages)-1]
	return msg
}

func (m *mockStreamEventsClient) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// streamTestDeliverer implements EventDeliverer for testing fallback behavior.
type streamTestDeliverer struct {
	mu        sync.Mutex
	callCount int
	lastEvent *pluginmodel.DomainEvent
	response  []*pluginmodel.DomainEvent
	err       error
}

func (m *streamTestDeliverer) DeliverEvent(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	m.lastEvent = event
	return m.response, m.err
}

func (m *streamTestDeliverer) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *streamTestDeliverer) getLastEvent() *pluginmodel.DomainEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastEvent
}

type noopLogger struct{}

func (n *noopLogger) Debug(msg string, fields ...any)             {}
func (n *noopLogger) Info(msg string, fields ...any)              {}
func (n *noopLogger) Warn(msg string, fields ...any)              {}
func (n *noopLogger) Error(msg string, fields ...any)             {}
func (n *noopLogger) WithContext(ctx map[string]any) ports.Logger { return n }

var _ ports.Logger = (*noopLogger)(nil)

func TestNewStreamManager(t *testing.T) {
	logger := &noopLogger{}
	sm := NewStreamManager(logger)
	assert.NotNil(t, sm)
}

func TestRegisterStream(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	client := &mockStreamEventsClient{}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)

	assert.True(t, sm.HasStream(pluginName))
}

func TestUnregisterStream(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	client := &mockStreamEventsClient{}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)
	assert.True(t, sm.HasStream(pluginName))

	sm.UnregisterStream(pluginName)
	assert.False(t, sm.HasStream(pluginName))
}

func TestHasStream(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	pluginName := "test-plugin"

	assert.False(t, sm.HasStream(pluginName))

	sm.RegisterStream(pluginName, &mockStreamEventsClient{})
	assert.True(t, sm.HasStream(pluginName))

	sm.UnregisterStream(pluginName)
	assert.False(t, sm.HasStream(pluginName))
}

func TestGetDeliverer_NoStream(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	fallback := &streamTestDeliverer{}

	deliverer := sm.GetDeliverer("nonexistent", fallback)

	assert.Equal(t, fallback, deliverer)
}

func TestGetDeliverer_WithStream(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	client := &mockStreamEventsClient{}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)
	fallback := &streamTestDeliverer{}

	deliverer := sm.GetDeliverer(pluginName, fallback)

	assert.NotNil(t, deliverer)
	assert.NotEqual(t, fallback, deliverer)
}

func TestStreamDeliverer_DeliverEvent_Success(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	client := &mockStreamEventsClient{}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)
	fallback := &streamTestDeliverer{}
	deliverer := sm.GetDeliverer(pluginName, fallback)

	ctx := context.Background()
	event := &pluginmodel.DomainEvent{
		ID:               "evt-123",
		Type:             "test.event",
		Timestamp:        time.Unix(1700000000, 123456789),
		Source:           "test-source",
		Metadata:         map[string]string{"key": "value"},
		Payload:          []byte("test-payload"),
		PropagationDepth: 1,
	}

	result, err := deliverer.DeliverEvent(ctx, event)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, client.getCallCount())
}

func TestStreamDeliverer_DeliverEvent_FieldMapping(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	client := &mockStreamEventsClient{}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)
	fallback := &streamTestDeliverer{}
	deliverer := sm.GetDeliverer(pluginName, fallback)

	ctx := context.Background()
	ts := time.Unix(1700000000, 123456789)
	event := &pluginmodel.DomainEvent{
		ID:               "evt-123",
		Type:             "test.event",
		Timestamp:        ts,
		Source:           "test-source",
		Metadata:         map[string]string{"key": "value"},
		Payload:          []byte("test-payload"),
		PropagationDepth: 1,
	}

	_, _ = deliverer.DeliverEvent(ctx, event)

	msg := client.getLastMessage()
	assert.NotNil(t, msg)
	assert.Equal(t, "evt-123", msg.Id)
	assert.Equal(t, "test.event", msg.Type)
	assert.Equal(t, ts.UnixNano(), msg.TimestampUnixNanos)
	assert.Equal(t, "test-source", msg.Source)
	assert.Equal(t, map[string]string{"key": "value"}, msg.Metadata)
	assert.Equal(t, []byte("test-payload"), msg.Payload)
	assert.Equal(t, int32(1), msg.PropagationDepth)
	assert.Equal(t, uint64(1), msg.SequenceNumber)
}

func TestStreamDeliverer_DeliverEvent_SequenceNumbers(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	client := &mockStreamEventsClient{}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)
	fallback := &streamTestDeliverer{}
	deliverer := sm.GetDeliverer(pluginName, fallback)

	ctx := context.Background()
	event := &pluginmodel.DomainEvent{
		ID:        "evt-seq",
		Type:      "test.event",
		Timestamp: time.Now(),
	}

	// Send three events and verify sequence numbers increment atomically
	_, _ = deliverer.DeliverEvent(ctx, event)
	msg1 := client.getLastMessage()

	_, _ = deliverer.DeliverEvent(ctx, event)
	msg2 := client.getLastMessage()

	_, _ = deliverer.DeliverEvent(ctx, event)
	msg3 := client.getLastMessage()

	assert.Equal(t, uint64(1), msg1.SequenceNumber)
	assert.Equal(t, uint64(2), msg2.SequenceNumber)
	assert.Equal(t, uint64(3), msg3.SequenceNumber)
}

func TestStreamDeliverer_DeliverEvent_FallbackOnSendError(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	client := &mockStreamEventsClient{
		sendErr: errors.New("send failed"),
	}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)
	fallback := &streamTestDeliverer{}
	deliverer := sm.GetDeliverer(pluginName, fallback)

	ctx := context.Background()
	event := &pluginmodel.DomainEvent{
		ID:        "evt-123",
		Type:      "test.event",
		Timestamp: time.Now(),
	}

	_, err := deliverer.DeliverEvent(ctx, event)

	assert.NoError(t, err)
	assert.Equal(t, 1, fallback.getCallCount())
	assert.False(t, sm.HasStream(pluginName))
}

func TestStreamDeliverer_DeliverEvent_FallbackOnUnimplemented(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	client := &mockStreamEventsClient{
		sendErr: status.Error(codes.Unimplemented, "method not implemented"),
	}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)
	fallback := &streamTestDeliverer{}
	deliverer := sm.GetDeliverer(pluginName, fallback)

	ctx := context.Background()
	event := &pluginmodel.DomainEvent{
		ID:        "evt-123",
		Type:      "test.event",
		Timestamp: time.Now(),
	}

	_, err := deliverer.DeliverEvent(ctx, event)

	assert.NoError(t, err)
	assert.Equal(t, 1, fallback.getCallCount())
	assert.False(t, sm.HasStream(pluginName))
}

func TestClose(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})

	client1 := &mockStreamEventsClient{}
	client2 := &mockStreamEventsClient{}

	sm.RegisterStream("plugin-1", client1)
	sm.RegisterStream("plugin-2", client2)

	assert.True(t, sm.HasStream("plugin-1"))
	assert.True(t, sm.HasStream("plugin-2"))

	sm.Close()

	assert.False(t, sm.HasStream("plugin-1"))
	assert.False(t, sm.HasStream("plugin-2"))
}

func TestClose_GoroutineCleanup(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})

	for i := 0; i < 10; i++ {
		client := &mockStreamEventsClient{}
		sm.RegisterStream(("plugin-" + string(rune(i))), client)
	}

	baselineGoroutines := runtime.NumGoroutine()

	sm.Close()

	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	delta := finalGoroutines - baselineGoroutines

	assert.True(t, delta <= 2, "goroutine delta %d exceeds tolerance of 2", delta)
}

func TestStreamDeliverer_DeliverEvent_FallbackOnSendTimeout(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	done := make(chan struct{})
	defer close(done)
	client := &blockingMockStreamEventsClient{done: done}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)
	fallback := &streamTestDeliverer{}
	deliverer := sm.GetDeliverer(pluginName, fallback)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	event := &pluginmodel.DomainEvent{
		ID:        "evt-timeout",
		Type:      "test.event",
		Timestamp: time.Now(),
	}

	_, err := deliverer.DeliverEvent(ctx, event)

	assert.NoError(t, err)
	assert.Equal(t, 1, fallback.getCallCount())
	assert.False(t, sm.HasStream(pluginName))
}

func TestStreamDeliverer_ConcurrentSends(t *testing.T) {
	sm := NewStreamManager(&noopLogger{})
	client := &mockStreamEventsClient{}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)
	fallback := &streamTestDeliverer{}
	deliverer := sm.GetDeliverer(pluginName, fallback)

	ctx := context.Background()
	event := &pluginmodel.DomainEvent{
		ID:        "evt-concurrent",
		Type:      "test.event",
		Timestamp: time.Now(),
	}

	var wg sync.WaitGroup
	var seqNums []uint64
	var seqMu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = deliverer.DeliverEvent(ctx, event)

			msg := client.getLastMessage()
			if msg != nil {
				seqMu.Lock()
				seqNums = append(seqNums, msg.SequenceNumber)
				seqMu.Unlock()
			}
		}()
	}

	wg.Wait()

	seqMu.Lock()
	defer seqMu.Unlock()

	assert.Len(t, seqNums, 10)

	seen := make(map[uint64]bool)
	for _, seq := range seqNums {
		assert.False(t, seen[seq], "duplicate sequence number: %d", seq)
		seen[seq] = true
	}
}
