//go:build integration

// Feature: F107 — T065
//
// Resume test: persists workflow state, kills session, calls facade.Resume(runID),
// asserts state is restored (US4).
package features_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/facadetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFacadeResume_RestoresState persists workflow state via a facade Run, closes the session,
// then calls facade.Resume(runID) and asserts that a live RunSession is returned.
//
// RED: facadetest.Fake.Resume() returns nil — this test drives the GREEN implementation
// of a real Resume path that restores state from persistence (US4).
func TestFacadeResume_RestoresState(t *testing.T) {
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

	fake := facadetest.New().WithTerminalCompleted()

	ctx := context.Background()

	// Run a workflow to completion and capture its ID.
	sess, err := fake.Run(ctx, ports.RunRequest{Identifier: "resume/workflow"})
	require.NoError(t, err)

	runID := sess.ID()
	require.NotEmpty(t, runID, "session ID must be non-empty")

	for range sess.Events() {
	}
	require.NoError(t, sess.Close())

	// Resume via facade — RED: fake returns nil session until real state persistence is wired.
	resumed, err := fake.Resume(ctx, runID)
	require.NoError(t, err, "Resume must not return an error")
	require.NotNil(t, resumed,
		"Resume must return a live RunSession (RED: implement real state restore in GREEN phase)")

	t.Cleanup(func() {
		if resumed != nil {
			_ = resumed.Close()
		}
	})

	assert.Equal(t, runID, resumed.ID(),
		"resumed session ID must match original run ID")
}
