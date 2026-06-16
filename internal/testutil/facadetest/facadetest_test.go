package facadetest_test

import (
	"context"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/facadetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeFacade_ScriptedEventsArriveInOrder(t *testing.T) {
	t.Parallel()
	script := []ports.Event{
		{Kind: ports.EventRunStarted, Seq: 1},
		{Kind: ports.EventStepStarted, Seq: 2},
		{Kind: ports.EventStepCompleted, Seq: 3},
		{Kind: ports.EventWorkflowCompleted, Seq: 4},
	}
	f := facadetest.New().Script(script...)
	sess, err := f.Run(context.Background(), ports.RunRequest{Identifier: "pack/wf"})
	require.NoError(t, err)

	for _, want := range script {
		select {
		case got, ok := <-sess.Events():
			require.True(t, ok, "channel closed before all scripted events arrived")
			assert.Equal(t, want.Kind, got.Kind)
			assert.Equal(t, want.Seq, got.Seq)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for event %v", want.Kind)
		}
	}

	select {
	case _, open := <-sess.Events():
		assert.False(t, open, "events channel must be closed after the terminal event")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for events channel to close after terminal event")
	}
}

func TestFakeFacade_TerminalEventClosesChannel(t *testing.T) {
	t.Parallel()
	f := facadetest.New().WithTerminalCompleted()
	sess, err := f.Run(context.Background(), ports.RunRequest{Identifier: "pack/wf"})
	require.NoError(t, err)

	select {
	case ev, ok := <-sess.Events():
		require.True(t, ok)
		assert.Equal(t, ports.EventWorkflowCompleted, ev.Kind)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for EventWorkflowCompleted")
	}

	select {
	case _, open := <-sess.Events():
		assert.False(t, open, "events channel must be closed after terminal EventWorkflowCompleted")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for events channel to close after terminal event")
	}
}

func TestFakeFacade_InputRequiredBlocksUntilRespond(t *testing.T) {
	t.Parallel()
	req := ports.InputRequest{PromptID: "p1", Prompt: "Enter value"}
	f := facadetest.New().
		WithInputRequired(req).
		WithTerminalCompleted()
	sess, err := f.Run(context.Background(), ports.RunRequest{Identifier: "pack/wf"})
	require.NoError(t, err)

	// Step 1: input event arrives.
	select {
	case ev, ok := <-sess.Events():
		require.True(t, ok)
		require.Equal(t, ports.EventInputRequired, ev.Kind)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for EventInputRequired")
	}

	// Step 2: next event is not yet available — pump is blocked waiting for Respond.
	select {
	case ev, ok := <-sess.Events():
		t.Errorf("expected no event before Respond, got kind=%v ok=%v", ev.Kind, ok)
	default:
	}

	// Step 3: Respond unblocks the pump.
	require.NoError(t, sess.Respond(ports.InputResponse{PromptID: "p1", Value: "answer"}))

	// Step 4: next event arrives after Respond.
	select {
	case ev, ok := <-sess.Events():
		require.True(t, ok)
		assert.Equal(t, ports.EventWorkflowCompleted, ev.Kind)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for EventWorkflowCompleted after Respond")
	}
}

func TestFakeFacade_SatisfiesPortContract(t *testing.T) {
	t.Parallel()

	t.Run("close_idempotent", func(t *testing.T) {
		t.Parallel()
		f := facadetest.New().WithTerminalCompleted()
		sess, err := f.Run(context.Background(), ports.RunRequest{Identifier: "pack/wf"})
		require.NoError(t, err)
		require.NoError(t, sess.Close())
		assert.NoError(t, sess.Close())
	})

	t.Run("events_channel_closed_after_close", func(t *testing.T) {
		t.Parallel()
		// Empty script so the channel has no buffered events when Close is called.
		f := facadetest.New()
		sess, err := f.Run(context.Background(), ports.RunRequest{Identifier: "pack/wf"})
		require.NoError(t, err)
		require.NoError(t, sess.Close())
		_, open := <-sess.Events()
		assert.False(t, open, "events channel must be closed after Close")
	})

	t.Run("run_zero_request_returns_err_invalid_request", func(t *testing.T) {
		t.Parallel()
		f := facadetest.New()
		_, err := f.Run(context.Background(), ports.RunRequest{})
		assert.ErrorIs(t, err, ports.ErrInvalidRequest)
	})

	t.Run("run_ctx_canceled_still_emits_terminal", func(t *testing.T) {
		t.Parallel()
		// The fake emits its scripted sequence unconditionally; a pre-cancelled context
		// does not abort Run (see Fake.Run / pump docs). This mirrors the spec-mandated
		// TestFacadeE2E_CtxCancelProducesWorkflowFailed conformance scenario, where a
		// cancelled run must still surface a terminal WorkflowFailed event rather than
		// failing at the Run call.
		f := facadetest.New().WithTerminalFailed(context.Canceled)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		sess, err := f.Run(ctx, ports.RunRequest{Identifier: "pack/wf"})
		require.NoError(t, err)
		t.Cleanup(func() { _ = sess.Close() })

		var last ports.Event
		var got bool
		for ev := range sess.Events() {
			last = ev
			got = true
		}
		require.True(t, got, "Run must emit at least one event even on cancel")
		assert.Equal(t, ports.EventWorkflowFailed, last.Kind)
	})

	t.Run("respond_after_close_returns_err_session_closed", func(t *testing.T) {
		t.Parallel()
		f := facadetest.New().WithTerminalCompleted()
		sess, err := f.Run(context.Background(), ports.RunRequest{Identifier: "pack/wf"})
		require.NoError(t, err)
		require.NoError(t, sess.Close())
		err = sess.Respond(ports.InputResponse{})
		assert.ErrorIs(t, err, ports.ErrSessionClosed)
	})
}
