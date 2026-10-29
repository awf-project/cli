package pluginmgr

import (
	"context"
	"errors"
	"fmt"
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
	lastCtx   context.Context
	response  []*pluginmodel.DomainEvent
	err       error
}

func (m *streamTestDeliverer) DeliverEvent(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	m.lastEvent = event
	m.lastCtx = ctx
	return m.response, m.err
}

func (m *streamTestDeliverer) getLastContext() context.Context {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastCtx
}

func (m *streamTestDeliverer) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

type noopLogger struct{}

func (n *noopLogger) Debug(msg string, fields ...any)             {}
func (n *noopLogger) Info(msg string, fields ...any)              {}
func (n *noopLogger) Warn(msg string, fields ...any)              {}
func (n *noopLogger) Error(msg string, fields ...any)             {}
func (n *noopLogger) WithContext(ctx map[string]any) ports.Logger { return n }

var _ ports.Logger = (*noopLogger)(nil)

type recordingLogger struct {
	mu      sync.Mutex
	entries []string
}

func (l *recordingLogger) append(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprint(append([]any{msg}, fields...)...))
}

func (l *recordingLogger) Debug(msg string, fields ...any) { l.append(msg, fields...) }
func (l *recordingLogger) Info(msg string, fields ...any)  { l.append(msg, fields...) }
func (l *recordingLogger) Warn(msg string, fields ...any)  { l.append(msg, fields...) }
func (l *recordingLogger) Error(msg string, fields ...any) { l.append(msg, fields...) }
func (l *recordingLogger) WithContext(map[string]any) ports.Logger {
	return l
}

func (l *recordingLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return fmt.Sprint(l.entries)
}

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

	for i := range 10 {
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
	logger := &recordingLogger{}
	sm := NewStreamManager(logger)
	sm.timeout = 25 * time.Millisecond
	done := make(chan struct{})
	defer close(done)
	client := &blockingMockStreamEventsClient{done: done}
	pluginName := "test-plugin"

	sm.RegisterStream(pluginName, client)
	fallback := &streamTestDeliverer{}
	deliverer := sm.GetDeliverer(pluginName, fallback)

	ctx := context.Background()
	const confidentialPayloadMarker = "event-payload-must-not-leak"
	event := &pluginmodel.DomainEvent{
		ID:        "evt-timeout",
		Type:      "test.event",
		Timestamp: time.Now(),
		Payload:   []byte(confidentialPayloadMarker),
	}

	started := time.Now()
	result := make(chan error, 1)
	go func() {
		_, err := deliverer.DeliverEvent(ctx, event)
		result <- err
	}()

	var err error
	select {
	case err = <-result:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("DeliverEvent did not fall back after the stream send timeout")
	}

	require.NoError(t, err)
	assert.Less(t, time.Since(started), 500*time.Millisecond)
	assert.Equal(t, 1, fallback.getCallCount())
	assert.False(t, sm.HasStream(pluginName))
	assert.Equal(t, ctx, fallback.getLastContext())
	assert.NoError(t, ctx.Err())
	assert.Contains(t, logger.String(), pluginName)
	assert.NotContains(t, logger.String(), confidentialPayloadMarker)
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

	for range 10 {
		wg.Go(func() {
			_, _ = deliverer.DeliverEvent(ctx, event)

			msg := client.getLastMessage()
			if msg != nil {
				seqMu.Lock()
				seqNums = append(seqNums, msg.SequenceNumber)
				seqMu.Unlock()
			}
		})
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
