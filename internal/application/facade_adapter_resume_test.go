package application_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openRecorder is a minimal ports.Recorder whose Subscribe returns an OPEN channel that
// blocks until cancelled (the Adapter's terminal event is emitted directly, not via the
// recorder). cancel closes the channel so the Adapter's drainTranscript goroutine exits
// cleanly. Record is a no-op: the resumed ExecutionService emits to its own recorder.
type openRecorder struct {
	ch   chan transcript.ExchangeEvent
	once sync.Once
}

func newOpenRecorder() *openRecorder {
	return &openRecorder{ch: make(chan transcript.ExchangeEvent)}
}

func (r *openRecorder) Record(context.Context, transcript.ExchangeEvent) error { return nil }

func (r *openRecorder) Subscribe() (<-chan transcript.ExchangeEvent, func()) {
	return r.ch, func() { r.once.Do(func() { close(r.ch) }) }
}

func (r *openRecorder) Close() error {
	r.once.Do(func() { close(r.ch) })
	return nil
}

// TestFacadeAdapter_ResumeStreamsPersistedRun verifies that Adapter.Resume re-drives a
// persisted, non-completed run by its runID through the real ExecutionService.Resume path
// and streams it through a RunSession: the session is registered under the ORIGINAL runID
// (so Status/SSE lookups resolve it) and a terminal EventWorkflowCompleted is emitted once
// the resumed execution finishes. This locks the core Resume wiring (previously a stub that
// ignored the runID and never resumed anything).
func TestFacadeAdapter_ResumeStreamsPersistedRun(t *testing.T) {
	const runID = "resume-run-1"

	wf := &workflow.Workflow{
		Name:    "resume-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2", OnSuccess: "step3"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Persisted state: step1 done, paused at step2 — a resumable (non-completed) run.
	interruptedState := &workflow.ExecutionContext{
		WorkflowID:   runID,
		WorkflowName: "resume-test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2",
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", Status: workflow.StatusCompleted, Output: "output1\n", StartedAt: time.Now().Add(-time.Minute)},
		},
		Env:       make(map[string]string),
		StartedAt: time.Now().Add(-time.Minute),
		UpdatedAt: time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("resume-test", wf).
		WithCommandResult("echo 2", &ports.CommandResult{Stdout: "output2\n", ExitCode: 0}).
		WithCommandResult("echo 3", &ports.CommandResult{Stdout: "output3\n", ExitCode: 0}).
		Build()
	require.NoError(t, mocks.StateStore.Save(context.Background(), interruptedState))

	registry := application.NewSessionRegistry()
	adapter := application.NewAdapter(nil, execSvc, nil, nil, newOpenRecorder(), registry)

	session, err := adapter.Resume(context.Background(), ports.ResumeRequest{RunID: runID})
	require.NoError(t, err)
	require.NotNil(t, session)
	defer session.Close() //nolint:errcheck // Close always returns nil

	// Registered under the ORIGINAL runID (not a freshly minted uuid) so Status/SSE resolve it.
	assert.Equal(t, runID, session.ID(), "resumed session must keep the original runID")
	_, ok := registry.Get(runID)
	assert.True(t, ok, "resumed session must be registered under its runID")

	// Drain events until a terminal event; the resumed run completes successfully.
	gotTerminal := false
	var terminalKind ports.EventKind
	timeout := time.After(5 * time.Second)
	for !gotTerminal {
		select {
		case ev, open := <-session.Events():
			if !open {
				t.Fatal("events channel closed before a terminal event was observed")
			}
			if ev.Kind == ports.EventWorkflowCompleted || ev.Kind == ports.EventWorkflowFailed {
				gotTerminal = true
				terminalKind = ev.Kind
			}
		case <-timeout:
			t.Fatal("timed out waiting for terminal event from resumed run")
		}
	}
	assert.Equal(t, ports.EventWorkflowCompleted, terminalKind, "resumed run should complete successfully")
}
