package application

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
)

func TestInputBridge_ReadInputSynthesizesEventInputRequired(t *testing.T) {
	session := newRunSession("test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := ports.InputRequest{Prompt: "Enter value"}

	go func() {
		_, _ = bridge.ReadInput(ctx, req)
	}()

	time.Sleep(50 * time.Millisecond)

	events := session.replayFromSeq(0)
	require.NotEmpty(t, events, "session should have events")

	found := false
	for _, ev := range events {
		if ev.Kind == ports.EventInputRequired {
			found = true
			break
		}
	}

	assert.True(t, found, "EventInputRequired should be synthesized on session events")
}

func TestInputBridge_RespondUnblocksReadInput(t *testing.T) {
	session := newRunSession("test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	ctx := context.Background()
	req := ports.InputRequest{Prompt: "Enter value"}

	resultCh := make(chan ports.InputResponse)
	errCh := make(chan error)

	go func() {
		resp, err := bridge.ReadInput(ctx, req)
		resultCh <- resp
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)

	sentResp := ports.InputResponse{Value: "user input"}
	err := bridge.Respond(sentResp)
	require.NoError(t, err)

	receivedResp := <-resultCh
	receivedErr := <-errCh

	assert.NoError(t, receivedErr)
	assert.Equal(t, sentResp.Value, receivedResp.Value)
}

func TestInputBridge_RespondAfterCloseReturnsErrSessionClosed(t *testing.T) {
	session := newRunSession("test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	bridge.Close()

	resp := ports.InputResponse{Value: "test"}
	err := bridge.Respond(resp)

	assert.ErrorIs(t, err, ports.ErrSessionClosed)
}

func TestInputBridge_DuplicateRespondRejected(t *testing.T) {
	session := newRunSession("test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := ports.InputRequest{Prompt: "Enter value"}

	resultCh := make(chan ports.InputResponse)
	errCh := make(chan error)

	go func() {
		resp, err := bridge.ReadInput(ctx, req)
		resultCh <- resp
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)

	resp1 := ports.InputResponse{Value: "first"}
	err1 := bridge.Respond(resp1)
	require.NoError(t, err1, "first Respond should succeed")

	resp2 := ports.InputResponse{Value: "second"}
	err2 := bridge.Respond(resp2)

	assert.ErrorIs(t, err2, ports.ErrDuplicateResponse, "second Respond should be rejected")

	_ = <-resultCh
	_ = <-errCh
}

func TestInputBridge_ReadInputUnblocksOnContextCancel(t *testing.T) {
	session := newRunSession("test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := ports.InputRequest{Prompt: "Enter value"}

	errCh := make(chan error)

	go func() {
		_, err := bridge.ReadInput(ctx, req)
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errCh

	assert.ErrorIs(t, err, context.Canceled, "ReadInput should unblock on context cancellation")
}

func TestInputBridge_CloseBeforeRespondUnblocks(t *testing.T) {
	session := newRunSession("test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	ctx := context.Background()
	req := ports.InputRequest{Prompt: "Enter value"}

	errCh := make(chan error)

	go func() {
		_, err := bridge.ReadInput(ctx, req)
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)

	bridge.Close()

	sessionErr := <-errCh

	assert.ErrorIs(t, sessionErr, context.Canceled, "ReadInput should unblock when session context is cancelled")
}

func TestInputBridge_CloseBeforeRespondEmitsTerminalWorkflowFailed(t *testing.T) {
	session := newRunSession("test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	ctx := context.Background()
	req := ports.InputRequest{Prompt: "Enter value"}

	errCh := make(chan error)
	go func() {
		_, err := bridge.ReadInput(ctx, req)
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)

	bridge.Close()

	_ = <-errCh

	events := session.replayFromSeq(0)
	require.NotEmpty(t, events, "session should have events after close")

	lastEvent := events[len(events)-1]
	assert.Equal(t, ports.EventWorkflowFailed, lastEvent.Kind, "last event should be EventWorkflowFailed")

	code, ok := lastEvent.Payload.(domainerrors.ErrorCode)
	require.True(t, ok, "EventWorkflowFailed payload should carry an ErrorCode")
	assert.Equal(t, domainerrors.ErrorCodeUserFacadeSessionClosed, code, "ErrorCode should map context.Canceled")
}

func TestInputBridge_NoGoroutineLeakUnderFaultInjection(t *testing.T) {
	t.Run("close_before_respond", func(t *testing.T) {
		initialGoroutines := runtime.NumGoroutine()

		session := newRunSession("test-id", context.Background(), 256)
		bridge := NewInputBridge(session)

		ctx := context.Background()
		req := ports.InputRequest{Prompt: "Enter value"}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = bridge.ReadInput(ctx, req)
		}()

		time.Sleep(50 * time.Millisecond)

		bridge.Close()

		wg.Wait()
		time.Sleep(100 * time.Millisecond)
		runtime.GC()

		finalGoroutines := runtime.NumGoroutine()
		assert.LessOrEqual(t, finalGoroutines, initialGoroutines+1, "should not leak goroutines on close before respond")
	})

	t.Run("ctx_cancel_mid_park", func(t *testing.T) {
		initialGoroutines := runtime.NumGoroutine()

		session := newRunSession("test-id", context.Background(), 256)
		bridge := NewInputBridge(session)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req := ports.InputRequest{Prompt: "Enter value"}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = bridge.ReadInput(ctx, req)
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()

		wg.Wait()
		time.Sleep(100 * time.Millisecond)
		runtime.GC()

		finalGoroutines := runtime.NumGoroutine()
		assert.LessOrEqual(t, finalGoroutines, initialGoroutines+1, "should not leak goroutines on context cancel")
	})

	t.Run("duplicate_respond", func(t *testing.T) {
		initialGoroutines := runtime.NumGoroutine()

		session := newRunSession("test-id", context.Background(), 256)
		bridge := NewInputBridge(session)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		req := ports.InputRequest{Prompt: "Enter value"}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = bridge.ReadInput(ctx, req)
		}()

		time.Sleep(50 * time.Millisecond)

		_ = bridge.Respond(ports.InputResponse{Value: "first"})
		_ = bridge.Respond(ports.InputResponse{Value: "second"})

		wg.Wait()
		time.Sleep(100 * time.Millisecond)
		runtime.GC()

		finalGoroutines := runtime.NumGoroutine()
		assert.LessOrEqual(t, finalGoroutines, initialGoroutines+1, "should not leak goroutines on duplicate respond")
	})
}

func TestInputBridge_CloseConcurrencyIsSafe(t *testing.T) {
	session := newRunSession("test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bridge.Close()
		}()
	}
	wg.Wait()
}

func TestInputBridge_RespondNonBlockingWithSelect(t *testing.T) {
	session := newRunSession("test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	resp1 := ports.InputResponse{Value: "first"}
	err1 := bridge.Respond(resp1)
	require.NoError(t, err1)

	resp2 := ports.InputResponse{Value: "second"}
	err2 := bridge.Respond(resp2)

	assert.ErrorIs(t, err2, ports.ErrDuplicateResponse, "second Respond should fail non-blockingly")
}

func TestInputBridge_NewInputBridgeInitializesChannelWithCapacity(t *testing.T) {
	session := newRunSession("test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	resp := ports.InputResponse{Value: "test"}
	err := bridge.Respond(resp)

	require.NoError(t, err, "first Respond should succeed without a parked reader")
}

// TestInputBridge_StaleResponseNotDeliveredToNextCall verifies that a response
// deposited into the channel during call #1 (which exits via ctx cancellation
// before consuming it) is drained at ReadInput entry so call #2 never sees it.
//
// The test engineers a deterministic stale state:
//  1. Pre-fill responseCh via the internal channel (bypassing the dispatched
//     flag) so the value sits there before call #1 even starts.
//  2. Cancel ctx1 before starting call #1 so it exits immediately via ctx.Done()
//     without touching responseCh, leaving the value buffered.
//  3. Call #2 must drain that stale value at entry and then wait for its own
//     fresh Respond.
func TestInputBridge_StaleResponseNotDeliveredToNextCall(t *testing.T) {
	session := newRunSession("stale-test-id", context.Background(), 256)
	bridge := NewInputBridge(session)

	// Pre-fill the buffered channel with a stale value directly.
	// We bypass Respond() here intentionally: we want the value in the channel
	// WITHOUT setting dispatched=true so that call #1 can't even tell it's there
	// (simulating the case where Respond raced ctx.Done and ctx.Done "won").
	bridge.responseCh <- ports.InputResponse{Value: "stale-value"}

	// --- Call #1: ctx is already cancelled; exits via ctx.Done() immediately ---
	ctx1, cancel1 := context.WithCancel(context.Background())
	cancel1() // cancelled before ReadInput is called

	req1 := ports.InputRequest{Prompt: "call one"}

	var wg1 sync.WaitGroup
	wg1.Add(1)
	go func() {
		defer wg1.Done()
		_, _ = bridge.ReadInput(ctx1, req1)
	}()
	wg1.Wait()

	// At this point the stale value is still in responseCh because ctx.Done()
	// fired immediately. Without the fix, call #2 would pick it up.

	// --- Call #2: must receive its OWN response, never the stale one ---
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	req2 := ports.InputRequest{Prompt: "call two"}
	freshResp := ports.InputResponse{Value: "fresh-value"}

	resultCh2 := make(chan ports.InputResponse, 1)
	errCh2 := make(chan error, 1)

	go func() {
		resp, err := bridge.ReadInput(ctx2, req2)
		resultCh2 <- resp
		errCh2 <- err
	}()

	// Give ReadInput #2 time to enter (and drain stale value under the fix).
	time.Sleep(50 * time.Millisecond)

	// Deliver the fresh response for call #2.
	err2 := bridge.Respond(freshResp)
	require.NoError(t, err2, "Respond for call #2 should succeed (stale value must have been drained)")

	received2 := <-resultCh2
	receivedErr2 := <-errCh2

	require.NoError(t, receivedErr2)
	assert.Equal(t, freshResp.Value, received2.Value,
		"call #2 must receive the fresh response, not a stale one from call #1")
}
