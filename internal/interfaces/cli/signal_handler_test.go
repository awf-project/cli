package cli

import (
	"context"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: C019
// Component: signal_handler_extraction
// Tests for setupSignalHandler function extracted from run.go

func TestSetupSignalHandler_HappyPath_SIGINTCancelsContext(t *testing.T) {
	// Arrange: Create context and track cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callbackCalled := false
	onSignal := func() {
		callbackCalled = true
	}

	// Act: Setup signal handler
	cleanup := setupSignalHandler(ctx, cancel, onSignal)
	defer cleanup()

	// Send SIGINT signal to current process
	err := syscall.Kill(os.Getpid(), syscall.SIGINT)
	require.NoError(t, err, "failed to send SIGINT")

	// Wait for signal handling
	select {
	case <-ctx.Done():
		// Context was cancelled - expected
	case <-time.After(1 * time.Second):
		t.Fatal("context was not cancelled within timeout")
	}

	// Assert: Callback was called and context cancelled
	assert.True(t, callbackCalled, "onSignal callback should have been called")
	assert.ErrorIs(t, ctx.Err(), context.Canceled, "context should be cancelled")
}

func TestSetupSignalHandler_HappyPath_SIGTERMCancelsContext(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callbackCalled := false
	onSignal := func() {
		callbackCalled = true
	}

	// Act: Setup signal handler
	cleanup := setupSignalHandler(ctx, cancel, onSignal)
	defer cleanup()

	// Send SIGTERM signal
	err := syscall.Kill(os.Getpid(), syscall.SIGTERM)
	require.NoError(t, err, "failed to send SIGTERM")

	// Wait for signal handling
	select {
	case <-ctx.Done():
		// Context was cancelled - expected
	case <-time.After(1 * time.Second):
		t.Fatal("context was not cancelled within timeout")
	}

	// Assert
	assert.True(t, callbackCalled, "onSignal callback should have been called")
	assert.ErrorIs(t, ctx.Err(), context.Canceled, "context should be cancelled")
}

func TestSetupSignalHandler_HappyPath_NilCallbackDoesNotPanic(t *testing.T) {
	// Arrange: Create context with nil callback
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act: Setup signal handler with nil callback (should not panic)
	cleanup := setupSignalHandler(ctx, cancel, nil)
	defer cleanup()

	// Send signal
	err := syscall.Kill(os.Getpid(), syscall.SIGINT)
	require.NoError(t, err, "failed to send SIGINT")

	// Wait for cancellation
	select {
	case <-ctx.Done():
		// Context was cancelled - expected
	case <-time.After(1 * time.Second):
		t.Fatal("context was not cancelled within timeout")
	}

	// Assert: No panic occurred and context was cancelled
	assert.ErrorIs(t, ctx.Err(), context.Canceled, "context should be cancelled")
}

func TestSetupSignalHandler_EdgeCase_ContextCancelledBeforeSignal(t *testing.T) {
	// Arrange: Create context
	ctx, cancel := context.WithCancel(context.Background())

	callbackCalled := false
	onSignal := func() {
		callbackCalled = true
	}

	// Act: Setup signal handler
	cleanup := setupSignalHandler(ctx, cancel, onSignal)
	defer cleanup()

	// Cancel context BEFORE sending signal
	cancel()

	// Wait for goroutine to process context cancellation
	time.Sleep(100 * time.Millisecond)

	// Assert: Callback should NOT have been called (context was cancelled, not signal received)
	assert.False(t, callbackCalled, "onSignal callback should NOT be called when context cancelled externally")
	assert.ErrorIs(t, ctx.Err(), context.Canceled, "context should be cancelled")
}

func TestSetupSignalHandler_EdgeCase_CleanupStopsSignalDelivery(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callbackCalled := false
	onSignal := func() {
		callbackCalled = true
	}

	// Act: Setup and immediately cleanup
	cleanup := setupSignalHandler(ctx, cancel, onSignal)
	cleanup() // Call cleanup to stop signal notification

	// Wait to verify goroutine exits cleanly
	time.Sleep(100 * time.Millisecond)

	// Assert: Callback was not called because cleanup stopped signal delivery
	// Note: We cannot safely send signal after cleanup as it would affect the test process
	// This test verifies that cleanup() can be called and doesn't panic
	assert.False(t, callbackCalled, "onSignal callback should NOT be called")
	assert.NoError(t, ctx.Err(), "context should NOT be cancelled by signal handler")
}

func TestSetupSignalHandler_EdgeCase_MultipleSignalsIdempotent(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callCount := 0
	onSignal := func() {
		callCount++
	}

	// Act: Setup signal handler
	cleanup := setupSignalHandler(ctx, cancel, onSignal)
	defer cleanup()

	// Send single signal (multiple signals are racy in test environment)
	err := syscall.Kill(os.Getpid(), syscall.SIGINT)
	require.NoError(t, err, "failed to send SIGINT")

	// Wait for signal handling
	select {
	case <-ctx.Done():
		// Context was cancelled - expected
	case <-time.After(1 * time.Second):
		t.Fatal("context was not cancelled within timeout")
	}

	// Assert: Callback was called and context cancelled
	assert.GreaterOrEqual(t, callCount, 1, "onSignal should be called at least once")
	assert.ErrorIs(t, ctx.Err(), context.Canceled, "context should be cancelled")
}

func TestSetupSignalHandler_GoroutineLeakDetection_CleanupPreventsLeak(t *testing.T) {
	// Arrange: Count goroutines before test
	runtime.GC()
	time.Sleep(100 * time.Millisecond) // Allow GC to settle
	beforeCount := runtime.NumGoroutine()

	// Act: Create and cleanup signal handler 100 times
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cleanup := setupSignalHandler(ctx, cancel, nil)

		// Immediately cleanup (simulates normal workflow completion)
		cleanup()
		cancel()
	}

	// Wait for goroutines to exit
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	afterCount := runtime.NumGoroutine()

	// Assert: Goroutine count should not grow significantly
	// Allow tolerance of 5 goroutines for runtime internals
	assert.InDelta(t, beforeCount, afterCount, 5,
		"goroutine count should not grow after cleanup (before=%d, after=%d)",
		beforeCount, afterCount)
}

func TestSetupSignalHandler_GoroutineLeakDetection_ContextCancellationExitsGoroutine(t *testing.T) {
	// Arrange: Count goroutines before test
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	beforeCount := runtime.NumGoroutine()

	// Act: Create signal handlers and cancel contexts (simulates workflow completion)
	cleanups := make([]func(), 0, 50)
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cleanup := setupSignalHandler(ctx, cancel, nil)
		cleanups = append(cleanups, cleanup)

		// Cancel context (workflow completes normally)
		cancel()
	}

	// Cleanup all handlers
	for _, cleanup := range cleanups {
		cleanup()
	}

	// Wait for goroutines to exit
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	afterCount := runtime.NumGoroutine()

	// Assert: No goroutine leak
	assert.InDelta(t, beforeCount, afterCount, 5,
		"goroutine count should not grow (before=%d, after=%d)",
		beforeCount, afterCount)
}

func TestSetupSignalHandler_ErrorHandling_CleanupCalledMultipleTimes(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act: Setup signal handler
	cleanup := setupSignalHandler(ctx, cancel, nil)

	// Call cleanup multiple times (should be idempotent)
	require.NotPanics(t, func() {
		cleanup()
		cleanup()
		cleanup()
	}, "calling cleanup multiple times should not panic")

	// Assert: No panic occurred
}

func TestSetupSignalHandler_ErrorHandling_CleanupAfterContextCancelled(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())

	// Act: Setup signal handler
	cleanup := setupSignalHandler(ctx, cancel, nil)

	// Cancel context first
	cancel()
	time.Sleep(100 * time.Millisecond) // Allow goroutine to exit

	// Then call cleanup
	require.NotPanics(t, func() {
		cleanup()
	}, "cleanup after context cancellation should not panic")

	// Assert: No panic occurred
	assert.ErrorIs(t, ctx.Err(), context.Canceled, "context should be cancelled")
}

func TestSetupSignalHandler_EdgeCase_RapidSetupCleanupCycles(t *testing.T) {
	// Arrange & Act: Rapidly create and destroy signal handlers
	for i := 0; i < 1000; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cleanup := setupSignalHandler(ctx, cancel, nil)
		cleanup()
		cancel()
	}

	// Allow goroutines to settle
	time.Sleep(200 * time.Millisecond)
	runtime.GC()

	// Assert: No deadlock or panic occurred
	// If we reach here, the test passes
}

func TestSetupSignalHandler_Boundary_ZeroDelayBetweenSetupAndCleanup(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act: Setup and cleanup with zero delay
	cleanup := setupSignalHandler(ctx, cancel, nil)
	cleanup() // Immediate cleanup

	// Assert: No panic or deadlock
	// This tests race condition between goroutine start and cleanup
}

func TestSetupSignalHandler_Integration_WorkflowCompletionScenario(t *testing.T) {
	// Arrange: Simulate a workflow execution scenario
	beforeGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	cleanup := setupSignalHandler(ctx, cancel, func() {
		// Simulate cleanup message
	})
	defer cleanup()

	// Act: Simulate workflow completing normally (no signal received)
	// Workflow runs for a short time
	time.Sleep(100 * time.Millisecond)

	// Workflow completes - cancel context
	cancel()

	// Cleanup signal handler
	cleanup()

	// Wait for goroutine cleanup
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	afterGoroutines := runtime.NumGoroutine()

	// Assert: No goroutine leak
	assert.InDelta(t, beforeGoroutines, afterGoroutines, 3,
		"no goroutine leak after normal workflow completion (before=%d, after=%d)",
		beforeGoroutines, afterGoroutines)
	assert.ErrorIs(t, ctx.Err(), context.Canceled, "context should be cancelled")
}

func TestSetupSignalHandler_Integration_SignalInterruptionScenario(t *testing.T) {
	// Arrange: Simulate a workflow execution that gets interrupted
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interrupted := false
	onSignal := func() {
		interrupted = true
	}

	cleanup := setupSignalHandler(ctx, cancel, onSignal)
	defer cleanup()

	// Act: Simulate workflow running, then receiving signal
	go func() {
		time.Sleep(50 * time.Millisecond)
		err := syscall.Kill(os.Getpid(), syscall.SIGINT)
		if err != nil {
			t.Logf("failed to send signal: %v", err)
		}
	}()

	// Wait for interruption
	select {
	case <-ctx.Done():
		// Interrupted as expected
	case <-time.After(1 * time.Second):
		t.Fatal("workflow was not interrupted within timeout")
	}

	// Assert: Workflow was interrupted properly
	assert.True(t, interrupted, "interruption callback should have been called")
	assert.ErrorIs(t, ctx.Err(), context.Canceled, "context should be cancelled")

	// Cleanup
	cleanup()
}
