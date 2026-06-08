package transcript_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	infra "github.com/awf-project/cli/internal/infrastructure/transcript"
)

// fakeLogger captures calls for assertion.
type fakeLogger struct {
	mu       sync.Mutex
	warns    []string
	debugs   []string
	infos    []string
	errors   []string
	contexts []map[string]any
}

func (fl *fakeLogger) Debug(msg string, fields ...any) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.debugs = append(fl.debugs, msg)
}

func (fl *fakeLogger) Info(msg string, fields ...any) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.infos = append(fl.infos, msg)
}

func (fl *fakeLogger) Warn(msg string, fields ...any) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.warns = append(fl.warns, msg)
}

func (fl *fakeLogger) Error(msg string, fields ...any) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.errors = append(fl.errors, msg)
}

func (fl *fakeLogger) WithContext(ctx map[string]any) ports.Logger {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.contexts = append(fl.contexts, ctx)
	return fl
}

func (fl *fakeLogger) warnCount() int {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	return len(fl.warns)
}

func TestFanOut_NewFanOutWithDefaults(t *testing.T) {
	fo := infra.NewFanOut()
	require.NotNil(t, fo)
	stats := fo.Stats()
	assert.Equal(t, 0, stats.Subscribers)
	assert.Equal(t, uint64(0), stats.Drops)
}

func TestFanOut_WithBufferSize(t *testing.T) {
	fo := infra.NewFanOut(infra.WithBufferSize(512))
	require.NotNil(t, fo)

	ch, cancel := fo.Subscribe()
	defer cancel()

	require.NotNil(t, ch)
	stats := fo.Stats()
	assert.Equal(t, 1, stats.Subscribers)
}

func TestFanOut_WithLogger(t *testing.T) {
	fakeLog := &fakeLogger{}
	fo := infra.NewFanOut(infra.WithLogger(fakeLog))
	require.NotNil(t, fo)

	ch, cancel := fo.Subscribe()
	defer cancel()

	event := transcript.ExchangeEvent{
		Type:  transcript.EventTypeRunStarted,
		RunID: "run-123",
	}
	fo.Publish(event)

	select {
	case received := <-ch:
		assert.Equal(t, transcript.EventTypeRunStarted, received.Type)
		assert.Equal(t, "run-123", received.RunID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestFanOut_SubscribeReturnsChannelAndCancel(t *testing.T) {
	fo := infra.NewFanOut()
	ch, cancel := fo.Subscribe()

	require.NotNil(t, ch)
	require.NotNil(t, cancel)

	event := transcript.ExchangeEvent{
		Type:  transcript.EventTypeRunStarted,
		RunID: "run-123",
	}
	fo.Publish(event)

	received := <-ch
	assert.Equal(t, transcript.EventTypeRunStarted, received.Type)

	cancel()

	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after cancel")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestFanOut_PublishToMultipleSubscribers(t *testing.T) {
	fo := infra.NewFanOut()

	ch1, cancel1 := fo.Subscribe()
	defer cancel1()

	ch2, cancel2 := fo.Subscribe()
	defer cancel2()

	require.Equal(t, 2, fo.Stats().Subscribers)

	event := transcript.ExchangeEvent{
		Type:  transcript.EventTypeMessageUser,
		RunID: "run-456",
	}
	fo.Publish(event)

	received1 := <-ch1
	received2 := <-ch2

	assert.Equal(t, event.Type, received1.Type)
	assert.Equal(t, event.Type, received2.Type)
	assert.Equal(t, event.RunID, received1.RunID)
	assert.Equal(t, event.RunID, received2.RunID)
}

func TestFanOut_BoundedBufferDropsNewest(t *testing.T) {
	fo := infra.NewFanOut(infra.WithBufferSize(4))
	ch, cancel := fo.Subscribe()
	defer cancel()

	// Fill buffer with 4 events
	for i := range 4 {
		event := transcript.ExchangeEvent{
			Type:  transcript.EventTypeRunStarted,
			RunID: "run-123",
			Seq:   uint64(i + 1),
		}
		fo.Publish(event)
	}

	// Publish 5th event; should be dropped (drop-newest policy)
	fo.Publish(transcript.ExchangeEvent{
		Type:  transcript.EventTypeRunStarted,
		RunID: "run-123",
		Seq:   5,
	})

	// Verify first 4 events are in channel (drop-newest = newest dropped, oldest preserved)
	received1 := <-ch
	assert.Equal(t, uint64(1), received1.Seq)

	received2 := <-ch
	assert.Equal(t, uint64(2), received2.Seq)

	received3 := <-ch
	assert.Equal(t, uint64(3), received3.Seq)

	received4 := <-ch
	assert.Equal(t, uint64(4), received4.Seq)

	// Stats should show 1 drop
	stats := fo.Stats()
	assert.Greater(t, stats.Drops, uint64(0))
}

func TestFanOut_SlowSubscriberDoesNotBlockProducer(t *testing.T) {
	fo := infra.NewFanOut()
	ch, cancel := fo.Subscribe()
	defer cancel()

	eventCount := atomic.Int32{}
	start := time.Now()

	// Subscriber sleeps (blocks)
	go func() {
		for range ch {
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Producer publishes non-blocking
	for i := range 100 {
		fo.Publish(transcript.ExchangeEvent{
			Type:  transcript.EventTypeRunStarted,
			RunID: "run-slow",
			Seq:   uint64(i),
		})
		eventCount.Add(1)
	}

	elapsed := time.Since(start)

	// Producer should finish quickly (100ms ~ 1-2x buffer drain time)
	// NOT 1000ms (100 events * 10ms sleep)
	assert.Less(t, elapsed, 500*time.Millisecond,
		"producer should not block on slow subscriber (expected <500ms, got %v)", elapsed)

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestFanOut_StatsExposeDropCount(t *testing.T) {
	fo := infra.NewFanOut(infra.WithBufferSize(2))
	ch, cancel := fo.Subscribe()

	// Publish 10 events with only 2-size buffer
	for i := range 10 {
		fo.Publish(transcript.ExchangeEvent{
			Type:  transcript.EventTypeRunStarted,
			RunID: "run-drop-test",
			Seq:   uint64(i),
		})
	}

	// Drain channel to make room
	_ = <-ch
	_ = <-ch

	cancel()

	// Stats should reflect drops
	stats := fo.Stats()
	assert.Greater(t, stats.Drops, uint64(0),
		"drops should be greater than 0 after buffer overflow")
}

func TestFanOut_IdempotentClose(t *testing.T) {
	fo := infra.NewFanOut()
	ch, cancel := fo.Subscribe()
	defer cancel()

	err1 := fo.Close()
	require.NoError(t, err1)

	// Drain any pending events
	select {
	case <-ch:
	default:
	}

	err2 := fo.Close()
	assert.NoError(t, err2, "second Close should also return nil")

	err3 := fo.Close()
	assert.NoError(t, err3, "third Close should also return nil")

	_ = ch
}

func TestFanOut_IdempotentSubscriberCancel(t *testing.T) {
	fo := infra.NewFanOut()
	ch, cancel := fo.Subscribe()

	// First cancel
	cancel()

	// Verify channel is closed
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for channel close")
	}

	// Subsequent cancels should not panic
	assert.NotPanics(t, func() {
		cancel()
	})

	assert.NotPanics(t, func() {
		cancel()
	})
}

func TestFanOut_ZeroEventWarnsAndReturns(t *testing.T) {
	fakeLog := &fakeLogger{}
	fo := infra.NewFanOut(infra.WithLogger(fakeLog))
	_, cancel := fo.Subscribe()
	defer cancel()

	// Publish zero-value event
	assert.NotPanics(t, func() {
		fo.Publish(transcript.ExchangeEvent{})
	})

	// Should emit WARN via logger
	assert.Greater(t, fakeLog.warnCount(), 0,
		"publishing zero-value event should emit WARN")
}

func TestFanOut_MultipleSubscribersIndependentDrops(t *testing.T) {
	fo := infra.NewFanOut(infra.WithBufferSize(3))

	ch1, cancel1 := fo.Subscribe()
	ch2, cancel2 := fo.Subscribe()
	defer cancel1()
	defer cancel2()

	// Publish 5 events
	for i := range 5 {
		fo.Publish(transcript.ExchangeEvent{
			Type:  transcript.EventTypeRunStarted,
			RunID: "multi-sub",
			Seq:   uint64(i),
		})
	}

	// ch1 subscriber processes events
	_ = <-ch1
	_ = <-ch1

	// ch2 subscriber is slow; events will drop for it
	_ = <-ch2

	stats := fo.Stats()
	assert.Greater(t, stats.Drops, uint64(0),
		"drops should reflect slow subscriber buffer overflow")
}

func TestFanOut_RateLimitedWarnPerSubscriber(t *testing.T) {
	fakeLog := &fakeLogger{}
	fo := infra.NewFanOut(
		infra.WithBufferSize(1),
		infra.WithLogger(fakeLog),
	)

	_, cancel := fo.Subscribe()
	defer cancel()

	// Trigger drops within 1s window
	fo.Publish(transcript.ExchangeEvent{Type: transcript.EventTypeRunStarted})
	fo.Publish(transcript.ExchangeEvent{Type: transcript.EventTypeRunStarted}) // Drop 1
	fo.Publish(transcript.ExchangeEvent{Type: transcript.EventTypeRunStarted}) // Drop 2

	initialWarnCount := fakeLog.warnCount()

	// Publish again immediately (should NOT warn due to 1s rate limit)
	fo.Publish(transcript.ExchangeEvent{Type: transcript.EventTypeRunStarted})
	fo.Publish(transcript.ExchangeEvent{Type: transcript.EventTypeRunStarted})

	afterWarnCount := fakeLog.warnCount()

	// Should have warned for initial drops, but rate-limited subsequent warns
	assert.Greater(t, initialWarnCount, 0, "should warn on drop")
	// After rate limit, no new warns in same 1s window
	assert.Equal(t, initialWarnCount, afterWarnCount,
		"warns should be rate-limited to 1 per 1s per subscriber")
}

func TestFanOut_CloseClosesAllSubscribers(t *testing.T) {
	fo := infra.NewFanOut()

	ch1, cancel1 := fo.Subscribe()
	ch2, cancel2 := fo.Subscribe()
	defer cancel1()
	defer cancel2()

	err := fo.Close()
	require.NoError(t, err)

	// Both channels should be closed
	time.Sleep(50 * time.Millisecond)

	select {
	case _, ok := <-ch1:
		assert.False(t, ok, "ch1 should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ch1 close")
	}

	select {
	case _, ok := <-ch2:
		assert.False(t, ok, "ch2 should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ch2 close")
	}
}

func TestFanOut_PublishAfterCloseDoesNotCrash(t *testing.T) {
	fo := infra.NewFanOut()
	ch, cancel := fo.Subscribe()
	defer cancel()
	fo.Close()
	_ = ch

	assert.NotPanics(t, func() {
		fo.Publish(transcript.ExchangeEvent{
			Type: transcript.EventTypeRunStarted,
		})
	})
}

func TestFanOut_SubscribeAfterCloseReturnsNilChannel(t *testing.T) {
	fo := infra.NewFanOut()
	fo.Close()

	ch, cancel := fo.Subscribe()
	defer cancel()

	// After close, Subscribe should either return nil or a closed channel
	// Verify it doesn't panic when we try to use it
	assert.NotPanics(t, func() {
		select {
		case _, ok := <-ch:
			if ch != nil {
				assert.False(t, ok, "channel should be closed")
			}
		case <-time.After(50 * time.Millisecond):
		}
	})
}

func TestFanOut_StatsSubscriberCount(t *testing.T) {
	fo := infra.NewFanOut()

	assert.Equal(t, 0, fo.Stats().Subscribers)

	_, cancel1 := fo.Subscribe()
	assert.Equal(t, 1, fo.Stats().Subscribers)

	_, cancel2 := fo.Subscribe()
	assert.Equal(t, 2, fo.Stats().Subscribers)

	cancel1()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, fo.Stats().Subscribers)

	cancel2()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, fo.Stats().Subscribers)
}
