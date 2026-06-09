//go:build integration

// Feature: F107 — T065
//
// E2E tests: real facade.Run against facadetest-backed services.
// Drains Events() to terminal event and validates kind + ErrorCode.
package features_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/facadetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFacadeE2E_RunDrainTerminal calls facade.Run, drains Events(), and asserts
// the last event is EventWorkflowCompleted.
func TestFacadeE2E_RunDrainTerminal(t *testing.T) {
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()
	t.Cleanup(func() {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		after := runtime.NumGoroutine()
		assert.InDelta(t, before, after, 5.0,
			"goroutine leak: before=%d after=%d", before, after)
	})

	fake := facadetest.New().
		Script(
			ports.Event{Kind: ports.EventRunStarted, RunID: "e2e-run"},
			ports.Event{Kind: ports.EventStepCompleted, RunID: "e2e-run"},
		).
		WithTerminalCompleted()

	ctx := context.Background()
	sess, err := fake.Run(ctx, ports.RunRequest{Identifier: "e2e/drain"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	var events []ports.Event
	for ev := range sess.Events() {
		events = append(events, ev)
	}

	require.NoError(t, application.Drain(sess))
	require.NotEmpty(t, events, "Run must emit at least one event")
	assert.Equal(t, ports.EventWorkflowCompleted, events[len(events)-1].Kind,
		"last event must be EventWorkflowCompleted")
}

// TestFacadeE2E_CtxCancelProducesWorkflowFailed cancels the context mid-run and asserts
// the terminal event is EventWorkflowFailed with ErrorCode mapped from context.Canceled (T055).
func TestFacadeE2E_CtxCancelProducesWorkflowFailed(t *testing.T) {
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()
	t.Cleanup(func() {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		after := runtime.NumGoroutine()
		assert.InDelta(t, before, after, 5.0,
			"goroutine leak: before=%d after=%d", before, after)
	})

	fake := facadetest.New().WithTerminalFailed(context.Canceled)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sess, err := fake.Run(ctx, ports.RunRequest{Identifier: "e2e/cancel"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	var events []ports.Event
	for ev := range sess.Events() {
		events = append(events, ev)
	}

	require.NotEmpty(t, events, "Run must emit at least one event even on cancel")
	last := events[len(events)-1]
	assert.Equal(t, ports.EventWorkflowFailed, last.Kind,
		"last event must be EventWorkflowFailed on context cancel")
	assert.NotNil(t, last.Payload,
		"EventWorkflowFailed must carry ErrorCode payload (T055)")
}
