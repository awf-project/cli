package application

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
)

func TestRunSession_CloseIdempotent(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 256)

	var closeErrs [3]error
	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		go func(idx int) {
			defer wg.Done()
			closeErrs[idx] = s.Close()
		}(i)
	}
	wg.Wait()

	assert.NoError(t, closeErrs[0])
	assert.Equal(t, closeErrs[0], closeErrs[1])
	assert.Equal(t, closeErrs[1], closeErrs[2])
}

func TestRunSession_RespondAfterCloseReturnsErrSessionClosed(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 256)
	err := s.Close()
	require.NoError(t, err)

	resp := ports.InputResponse{Value: "test"}
	err = s.Respond(resp)

	assert.ErrorIs(t, err, ports.ErrSessionClosed)
}

func TestRunSession_ReplayFromSeqMonotonic(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 256)

	for i := 0; i < 10; i++ {
		ev := ports.Event{
			Seq:  uint64(i),
			Kind: ports.EventRunStarted,
		}
		s.appendEvent(ev)
	}

	result := s.replayFromSeq(5)

	require.Len(t, result, 5)
	assert.Equal(t, uint64(5), result[0].Seq)
	assert.Equal(t, uint64(6), result[1].Seq)
	assert.Equal(t, uint64(7), result[2].Seq)
	assert.Equal(t, uint64(8), result[3].Seq)
	assert.Equal(t, uint64(9), result[4].Seq)

	for i := 0; i < len(result)-1; i++ {
		assert.Less(t, result[i].Seq, result[i+1].Seq)
	}
}

func TestRunSession_ReplayBufferOverflowDropsOldest(t *testing.T) {
	bufferSize := 10
	s := newRunSession("test-id", context.Background(), bufferSize)

	for i := 0; i < 20; i++ {
		ev := ports.Event{
			Seq:  uint64(i),
			Kind: ports.EventRunStarted,
		}
		s.appendEvent(ev)
	}

	result := s.replayFromSeq(0)

	require.Len(t, result, bufferSize)
	assert.Equal(t, uint64(10), result[0].Seq)
	assert.Equal(t, uint64(19), result[len(result)-1].Seq)
}

func TestRunSession_ErrReflectsTerminalCause(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := newRunSession("test-id", ctx, 256)

	cancel()
	s.setErr(context.Canceled)

	err := s.Err()
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunSession_NoGoroutineLeakOnClose(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newRunSession("test-id", ctx, 256)

	ev := ports.Event{
		Seq:  1,
		Kind: ports.EventRunStarted,
	}
	s.appendEvent(ev)

	err := s.Close()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	assert.LessOrEqual(t, finalGoroutines, initialGoroutines+1)
}

func TestRunSession_ID(t *testing.T) {
	s := newRunSession("unique-id", context.Background(), 256)
	assert.Equal(t, "unique-id", s.ID())
}

func TestRunSession_RespondBeforeCloseSucceeds(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 256)

	resp := ports.InputResponse{Value: "test-response"}
	err := s.Respond(resp)

	assert.NoError(t, err)

	received := <-s.respondCh
	assert.Equal(t, resp.Value, received.Value)
}

func TestRunSession_RespondDuplicateReturnsErrDuplicateResponse(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 256)

	resp1 := ports.InputResponse{Value: "first"}
	err1 := s.Respond(resp1)
	require.NoError(t, err1)

	resp2 := ports.InputResponse{Value: "second"}
	err2 := s.Respond(resp2)

	assert.ErrorIs(t, err2, ports.ErrDuplicateResponse)
}

func TestRunSession_EventsChannelClosedAfterClose(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 256)

	err := s.Close()
	require.NoError(t, err)

	_, ok := <-s.Events()
	assert.False(t, ok, "channel should be closed after Close()")
}

func TestRunSession_ReplayFromSeqMissing(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 256)

	for i := 0; i < 5; i++ {
		ev := ports.Event{Seq: uint64(i), Kind: ports.EventRunStarted}
		s.appendEvent(ev)
	}

	result := s.replayFromSeq(100)
	assert.Len(t, result, 0)
}

func TestRunSession_AppendEventIntoReplayBuffer(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 5)

	for i := 0; i < 3; i++ {
		ev := ports.Event{Seq: uint64(i), Kind: ports.EventRunStarted}
		s.appendEvent(ev)
	}

	result := s.replayFromSeq(0)
	require.Len(t, result, 3)

	ev := ports.Event{Seq: uint64(10), Kind: ports.EventRunCompleted}
	s.appendEvent(ev)

	result = s.replayFromSeq(10)
	assert.Len(t, result, 1)
	assert.Equal(t, uint64(10), result[0].Seq)
}

func TestRunSession_ErrorNilByDefault(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 256)
	assert.Nil(t, s.Err())
}

func TestRunSession_ErrPersistsAfterSet(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 256)
	expectedErr := errors.New("test error")
	s.setErr(expectedErr)

	assert.Equal(t, expectedErr, s.Err())
}

func TestRunSession_DefaultBufferSize(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 0)

	for i := 0; i < 300; i++ {
		ev := ports.Event{Seq: uint64(i), Kind: ports.EventRunStarted}
		s.appendEvent(ev)
	}

	result := s.replayFromSeq(0)
	require.Len(t, result, 256)
	assert.Equal(t, uint64(44), result[0].Seq)
}

// TestRunSession_AppendEventNoPanicAfterClose verifies that concurrent appendEvent
// calls racing against Close() do not panic with "send on closed channel".
// Bug #1: select+default does NOT guard a closed channel — the fix adds a s.closed bool
// checked under s.mu before any send.
func TestRunSession_AppendEventNoPanicAfterClose(t *testing.T) {
	s := newRunSession("panic-test", context.Background(), 256)

	const numWorkers = 20
	const eventsPerWorker = 100

	var wg sync.WaitGroup
	// Drain the events channel so it never fills up and blocks workers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range s.Events() { //nolint:revive // intentionally consuming all events
		}
	}()

	// Launch workers that append events concurrently.
	appenders := make(chan struct{})
	var appendWG sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		appendWG.Add(1)
		go func(workerID int) {
			defer appendWG.Done()
			<-appenders // wait for the start signal
			for j := 0; j < eventsPerWorker; j++ {
				s.appendEvent(ports.Event{
					Seq:  uint64(workerID*eventsPerWorker + j), //nolint:gosec // G115: controlled test bounds
					Kind: ports.EventRunStarted,
				})
			}
		}(i)
	}

	// Close races with the appenders.
	close(appenders) // start all workers simultaneously
	_ = s.Close()

	appendWG.Wait()
	wg.Wait() // wait for the drain goroutine to finish (Events() is closed)
}

// TestRunSession_ClosePreservesBufferedEvents verifies that Close() does NOT drain the
// buffered events channel before closing it. After Close(), a range over Events() must
// yield all previously-appended events.
// Bug #8: the old drain loop `for len(s.events) > 0 { <-s.events }` discarded events
// that concurrent readers using `range session.Events()` expected to receive.
func TestRunSession_ClosePreservesBufferedEvents(t *testing.T) {
	const numEvents = 10
	s := newRunSession("preserve-test", context.Background(), numEvents+4)

	for i := 0; i < numEvents; i++ {
		s.appendEvent(ports.Event{
			Seq:  uint64(i), //nolint:gosec // G115: controlled test bounds
			Kind: ports.EventRunStarted,
		})
	}

	require.NoError(t, s.Close())

	var received []ports.Event
	for ev := range s.Events() {
		received = append(received, ev)
	}

	assert.Len(t, received, numEvents, "all buffered events must survive Close()")
}

func TestRunSession_ConcurrentAppendAndReplay(t *testing.T) {
	s := newRunSession("test-id", context.Background(), 256)

	var wg sync.WaitGroup
	done := make(chan struct{})

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				ev := ports.Event{
					Seq:  uint64(idx*100 + j), //nolint:gosec // G115: controlled test bounds; idx ∈ [0,4], j ∈ [0,49]
					Kind: ports.EventRunStarted,
				}
				s.appendEvent(ev)
			}
		}(i)
	}

	go func() {
		for i := 0; i < 100; i++ {
			result := s.replayFromSeq(0)
			_ = result
			time.Sleep(1 * time.Millisecond)
		}
		close(done)
	}()

	wg.Wait()
	<-done
}
