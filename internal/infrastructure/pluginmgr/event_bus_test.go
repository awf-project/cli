package pluginmgr

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEventDeliverer implements EventDeliverer with configurable behavior.
type mockEventDeliverer struct {
	deliverFunc func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error)
	mu          sync.Mutex
	callCount   int
	lastEvent   *pluginmodel.DomainEvent
}

func (m *mockEventDeliverer) DeliverEvent(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
	m.mu.Lock()
	m.callCount++
	m.lastEvent = event
	m.mu.Unlock()

	if m.deliverFunc != nil {
		return m.deliverFunc(ctx, event)
	}
	return nil, nil
}

func (m *mockEventDeliverer) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *mockEventDeliverer) getLastEvent() *pluginmodel.DomainEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastEvent
}

// testLogger implements ports.Logger with warning recording.
type testLogger struct {
	mu       sync.Mutex
	warnings []string
	infos    []string
}

func (l *testLogger) Debug(msg string, fields ...any) {}

func (l *testLogger) Info(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, msg)
}

func (l *testLogger) Warn(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warnings = append(l.warnings, msg)
}

func (l *testLogger) Error(msg string, fields ...any) {}

func (l *testLogger) WithContext(ctx map[string]any) ports.Logger {
	return l
}

func (l *testLogger) hasWarning(substring string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, w := range l.warnings {
		if strings.Contains(w, substring) {
			return true
		}
	}
	return false
}

func TestMatchEventPatternValidPatterns(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		eventType string
		want      bool
	}{
		{
			name:      "single wildcard matches one segment",
			pattern:   "workflow.*",
			eventType: "workflow.started",
			want:      true,
		},
		{
			name:      "wildcard does not cross dots",
			pattern:   "workflow.*",
			eventType: "workflow.step.started",
			want:      false,
		},
		{
			name:      "wildcard matches single segment with any content",
			pattern:   "*",
			eventType: "workflow",
			want:      true,
		},
		{
			name:      "single segment wildcard cannot match multi-segment",
			pattern:   "*",
			eventType: "workflow.started",
			want:      false,
		},
		{
			name:      "multiple wildcards match multiple segments",
			pattern:   "workflow.*.*",
			eventType: "workflow.step.started",
			want:      true,
		},
		{
			name:      "multiple wildcards match multi-segment pattern",
			pattern:   "*.*",
			eventType: "workflow.started",
			want:      true,
		},
		{
			name:      "pattern prefix mismatch",
			pattern:   "step.*",
			eventType: "workflow.started",
			want:      false,
		},
		{
			name:      "empty pattern",
			pattern:   "",
			eventType: "workflow.started",
			want:      false,
		},
		{
			name:      "empty event type",
			pattern:   "workflow.*",
			eventType: "",
			want:      false,
		},
		{
			name:      "exact match without wildcards",
			pattern:   "plugin.event",
			eventType: "plugin.event",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchEventPattern(tt.pattern, tt.eventType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEventBusImplementsEventPublisher(t *testing.T) {
	var _ ports.EventPublisher = (*EventBus)(nil)
	assert.True(t, true)
}

func TestSubscribeRegistersAndStartsGoroutine(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	baseGoroutines := runtime.NumGoroutine()

	deliverer := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			return nil, nil
		},
	}

	bus.Subscribe("plugin-1", []string{"test.*"}, deliverer)

	time.Sleep(50 * time.Millisecond)
	afterGoroutines := runtime.NumGoroutine()

	assert.Greater(t, afterGoroutines, baseGoroutines, "goroutine should be started for delivery")
}

func TestPublishDeliversToMatchingPatterns(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	deliverer := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			return nil, nil
		},
	}

	bus.Subscribe("plugin-1", []string{"workflow.*"}, deliverer)

	event := &pluginmodel.DomainEvent{
		ID:   "evt-1",
		Type: "workflow.started",
	}

	err := bus.Publish(context.Background(), event)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 1, deliverer.getCallCount(), "deliverer should be called once")
	assert.Equal(t, "evt-1", deliverer.getLastEvent().ID)
}

func TestPublishDoesNotDeliverToNonMatchingPatterns(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	deliverer := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			return nil, nil
		},
	}

	bus.Subscribe("plugin-1", []string{"step.*"}, deliverer)

	event := &pluginmodel.DomainEvent{
		ID:   "evt-1",
		Type: "workflow.started",
	}

	err := bus.Publish(context.Background(), event)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 0, deliverer.getCallCount(), "deliverer should not be called for non-matching pattern")
}

func TestPublishBufferFullDropsAndLogsWarning(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBusWithBufferSize(logger, 2)
	defer bus.Close()

	blockedDelivery := make(chan struct{})
	deliverer := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			<-blockedDelivery
			return nil, nil
		},
	}

	bus.Subscribe("plugin-1", []string{"test.*"}, deliverer)

	for i := 0; i < 5; i++ {
		event := &pluginmodel.DomainEvent{
			ID:   fmt.Sprintf("evt-%d", i),
			Type: "test.event",
		}
		err := bus.Publish(context.Background(), event)
		require.NoError(t, err)
	}

	close(blockedDelivery)
	time.Sleep(200 * time.Millisecond)

	assert.True(t, logger.hasWarning("event buffer full"), "should log buffer full warning")
}

func TestPublishPropagationDepthExceededLogsWarning(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	deliverer := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			if event.PropagationDepth < 3 {
				return []*pluginmodel.DomainEvent{
					{
						ID:               "evt-emitted",
						Type:             "test.emitted",
						PropagationDepth: event.PropagationDepth + 1,
					},
				}, nil
			}
			return nil, nil
		},
	}

	bus.Subscribe("plugin-1", []string{"*"}, deliverer)

	event := &pluginmodel.DomainEvent{
		ID:               "evt-0",
		Type:             "test.event",
		PropagationDepth: 0,
	}

	err := bus.Publish(context.Background(), event)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	assert.True(t, logger.hasWarning("propagation depth exceeded"), "should log propagation depth warning")
}

func TestUnsubscribeStopsGoroutineAndRemovesSubscription(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	baseGoroutines := runtime.NumGoroutine()

	deliverer := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			return nil, nil
		},
	}

	bus.Subscribe("plugin-1", []string{"test.*"}, deliverer)
	time.Sleep(50 * time.Millisecond)

	afterSubscribeGoroutines := runtime.NumGoroutine()
	assert.Greater(t, afterSubscribeGoroutines, baseGoroutines)

	bus.Unsubscribe("plugin-1")
	time.Sleep(50 * time.Millisecond)

	afterUnsubscribeGoroutines := runtime.NumGoroutine()

	assert.Equal(t, baseGoroutines, afterUnsubscribeGoroutines, "goroutine should be stopped after unsubscribe")
}

func TestGoroutineCountReturnsToBaselineAfterUnsubscribeAll(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	baseGoroutines := runtime.NumGoroutine()

	deliverer1 := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			return nil, nil
		},
	}

	deliverer2 := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			return nil, nil
		},
	}

	bus.Subscribe("plugin-1", []string{"test.*"}, deliverer1)
	bus.Subscribe("plugin-2", []string{"test.*"}, deliverer2)
	time.Sleep(100 * time.Millisecond)

	bus.Unsubscribe("plugin-1")
	bus.Unsubscribe("plugin-2")
	time.Sleep(100 * time.Millisecond)

	afterGoroutines := runtime.NumGoroutine()

	assert.InDelta(t, baseGoroutines, afterGoroutines, 2, "goroutine count should return to baseline within delta of 2")
}

func TestCloseUnsubscribesAllPlugins(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)

	baseGoroutines := runtime.NumGoroutine()

	deliverer1 := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			return nil, nil
		},
	}

	deliverer2 := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			return nil, nil
		},
	}

	bus.Subscribe("plugin-1", []string{"test.*"}, deliverer1)
	bus.Subscribe("plugin-2", []string{"test.*"}, deliverer2)
	time.Sleep(100 * time.Millisecond)

	err := bus.Close()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	afterGoroutines := runtime.NumGoroutine()

	assert.InDelta(t, baseGoroutines, afterGoroutines, 2, "all goroutines should be cleaned up after Close")
}

func TestTwoSubscribersWithOverlappingPatternsReceiveIndependentCopies(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	deliveredToPlugin1 := make([]*pluginmodel.DomainEvent, 0)
	deliveredToPlugin2 := make([]*pluginmodel.DomainEvent, 0)
	mu := sync.Mutex{}

	deliverer1 := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			mu.Lock()
			deliveredToPlugin1 = append(deliveredToPlugin1, event)
			mu.Unlock()
			return nil, nil
		},
	}

	deliverer2 := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			mu.Lock()
			deliveredToPlugin2 = append(deliveredToPlugin2, event)
			mu.Unlock()
			return nil, nil
		},
	}

	bus.Subscribe("plugin-1", []string{"*"}, deliverer1)
	bus.Subscribe("plugin-2", []string{"workflow.*"}, deliverer2)

	event := &pluginmodel.DomainEvent{
		ID:   "evt-1",
		Type: "workflow.started",
	}

	err := bus.Publish(context.Background(), event)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	assert.Len(t, deliveredToPlugin1, 1, "plugin-1 should receive event")
	assert.Len(t, deliveredToPlugin2, 1, "plugin-2 should receive event")
	assert.Equal(t, "evt-1", deliveredToPlugin1[0].ID)
	assert.Equal(t, "evt-1", deliveredToPlugin2[0].ID)
	mu.Unlock()
}

func TestPublishWithContextCancellation(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	ctxCancelled := atomic.Bool{}
	deliverer := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			<-ctx.Done()
			ctxCancelled.Store(true)
			return nil, ctx.Err()
		},
	}

	bus.Subscribe("plugin-1", []string{"test.*"}, deliverer)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := &pluginmodel.DomainEvent{
		ID:   "evt-1",
		Type: "test.event",
	}

	bus.Publish(ctx, event)
	time.Sleep(100 * time.Millisecond)

	assert.True(t, ctxCancelled.Load(), "context should be cancelled in deliverer")
}

func TestPublishMultipleEventsSequentially(t *testing.T) {
	logger := &testLogger{}
	bus := NewEventBus(logger)
	defer bus.Close()

	deliveredEvents := make([]*pluginmodel.DomainEvent, 0)
	mu := sync.Mutex{}

	deliverer := &mockEventDeliverer{
		deliverFunc: func(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
			mu.Lock()
			deliveredEvents = append(deliveredEvents, event)
			mu.Unlock()
			return nil, nil
		},
	}

	bus.Subscribe("plugin-1", []string{"*"}, deliverer)

	for i := 0; i < 5; i++ {
		event := &pluginmodel.DomainEvent{
			ID:   fmt.Sprintf("evt-%d", i),
			Type: "test.event",
		}
		err := bus.Publish(context.Background(), event)
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	assert.Len(t, deliveredEvents, 5, "all events should be delivered")
	for i := 0; i < 5; i++ {
		assert.Equal(t, fmt.Sprintf("evt-%d", i), deliveredEvents[i].ID)
	}
	mu.Unlock()
}
