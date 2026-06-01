package acpserver_test

// goroutine_leak_test.go — Verifies that Serve drains ALL goroutines it starts
// (including the internal scanLoop) when the context is cancelled before stdin
// reaches EOF.
//
// Without the cancellable-reader fix (M2), scanLoop stays alive after Serve
// returns because ReadSlice is a blocking syscall that ignores context
// cancellation.  goleak detects the leaked goroutine and the test fails.
//
// TDD note: this test is written BEFORE the fix and must FAIL until M2 is
// applied.  Once the fix is in, scanLoop unblocks through the cancellable
// pipe and goleak sees no residual goroutines.
//
// # Goroutine ownership contract (post-M2)
//
// Serve owns three goroutines internally:
//   - closer: tracked in wg; calls pipeWriter.CloseWithError on ctx cancel.
//   - copier: NOT tracked in wg; forwards bytes from the real stdin into the
//     pipe.  It terminates when the real stdin is closed by the caller — the
//     caller is responsible for closing in after Serve returns (same as before
//     the fix, since open-stdin is a caller concern).
//   - scanLoop: NOT tracked in wg; terminates when pipeReader is closed, which
//     happens as soon as closer or copier closes pipeWriter.
//
// The test therefore closes the stdin pipes BEFORE the goleak assertion so the
// copier and scanLoop can drain, then asserts no goroutines remain.

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/acpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// TestServe_NoGoroutineLeakOnContextCancel asserts that all goroutines started
// by Serve have terminated once (a) the context is cancelled and (b) the caller
// closes stdin — matching the documented caller contract.
//
// The key assertion compared to the pre-M2 state: before the fix, scanLoop
// blocked in ReadSlice even AFTER stdin was closed (because it was reading from
// the original blocking stdin, not the pipe).  After the fix, closing either
// the context or the stdin is sufficient for all goroutines to drain.
func TestServe_NoGoroutineLeakOnContextCancel(t *testing.T) {
	srv := acpserver.New(discardLogger())

	// net.Pipe produces a synchronous, blocking in-process connection.
	// The server reads from stdinConn; stdinClient is the remote end.
	stdinConn, stdinClient := net.Pipe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveComplete := make(chan error, 1)
	go func() {
		serveComplete <- srv.Serve(ctx, stdinConn, io.Discard)
	}()

	// Wait for Serve to reach its running state before cancelling.
	ctxReady, cancelReady := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelReady()
	require.NoError(t, srv.Notify(ctxReady, "probe/ready", nil),
		"server must be ready before cancel")

	// Cancel the context — triggers Serve to return while stdin is still open.
	cancel()

	select {
	case err := <-serveComplete:
		if err != nil {
			t.Errorf("Serve returned unexpected error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Serve did not return within 500ms of context cancellation")
	}

	// Close stdin (caller responsibility per contract): this lets the copier
	// goroutine exit its io.Copy(pipeWriter, in) call.
	stdinClient.Close()
	stdinConn.Close()

	// Poll deterministically instead of sleeping a fixed interval: goroutines
	// may wind down at different rates under -race or on slow CI hosts, so a
	// single time.Sleep(50ms) can both spuriously fail (too short) and waste
	// wall-clock time (too long). We poll until goleak.Find returns nil or the
	// 500ms budget is exhausted, logging the last leak for diagnostics.
	var lastLeak error
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		lastLeak = goleak.Find()
		if lastLeak == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if lastLeak != nil {
		t.Errorf("goroutine leak after Serve + stdin close (M2): %v", lastLeak)
	}
}

// TestServe_ScanLoopTerminatesBeforeServeReturns is a stricter variant that
// asserts scanLoop specifically is NOT alive after Serve returns on context
// cancel (before stdin is closed).  The closer goroutine's CloseWithError must
// unblock scanLoop, not just the copier.
func TestServe_ScanLoopTerminatesBeforeServeReturns(t *testing.T) {
	srv := acpserver.New(discardLogger())

	stdinConn, stdinClient := net.Pipe()
	defer stdinClient.Close()
	defer stdinConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveComplete := make(chan error, 1)
	go func() {
		serveComplete <- srv.Serve(ctx, stdinConn, io.Discard)
	}()

	ctxReady, cancelReady := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelReady()
	require.NoError(t, srv.Notify(ctxReady, "probe/ready", nil))

	cancel()

	select {
	case <-serveComplete:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Serve did not return within 500ms")
	}

	// At this point stdinClient is still open. Verify that scanLoop
	// specifically is no longer running. We look for the known stack frame.
	//
	// Poll deterministically: scanLoop should exit promptly once the closer
	// goroutine calls pipeWriter.CloseWithError, but under -race the scheduler
	// may not run it immediately. assert.Eventually avoids a fixed sleep by
	// retrying the check until the goroutine is gone or 500ms elapses.
	assert.Eventually(
		t,
		func() bool {
			leaks := goleak.Find()
			if leaks == nil {
				return true
			}
			// scanLoop must be gone; copier may still be alive (blocked on
			// stdinConn read) — that is acceptable per the caller contract.
			return !strings.Contains(leaks.Error(), "scanLoop")
		},
		500*time.Millisecond,
		5*time.Millisecond,
		"scanLoop goroutine leaked after Serve returned on ctx cancel",
	)
}
