package ports_test

import (
	"context"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface guards ensure fakeFacade and fakeSession stay in sync
// with the port definitions. A missing method on either type is a build error here
// rather than a silent runtime failure elsewhere.
var (
	_ ports.WorkflowFacade = (*fakeFacade)(nil)
	_ ports.RunSession     = (*fakeSession)(nil)
)

// fakeSession is a minimal in-memory RunSession for contract verification.
type fakeSession struct {
	id     string
	events chan ports.Event
	closed bool
	mu     sync.Mutex
	once   sync.Once
}

func newFakeSession(id string) *fakeSession {
	return &fakeSession{
		id:     id,
		events: make(chan ports.Event, 16),
	}
}

func (s *fakeSession) ID() string { return s.id }

func (s *fakeSession) Events() <-chan ports.Event { return s.events }

func (s *fakeSession) Respond(_ ports.InputResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ports.ErrSessionClosed
	}
	return nil
}

func (s *fakeSession) Err() error { return nil }

func (s *fakeSession) Close() error {
	s.once.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.closed = true
		close(s.events)
	})
	return nil
}

// fakeFacade is a minimal in-memory WorkflowFacade for contract verification.
// It stores sessions by run identifier so Resume can look them up by RunID.
type fakeFacade struct {
	mu       sync.Mutex
	sessions map[string]*fakeSession
}

func (f *fakeFacade) List(_ context.Context) ([]ports.WorkflowSummary, error) {
	return nil, nil
}

func (f *fakeFacade) Validate(_ context.Context, _ ports.RunRequest) (ports.ValidationReport, error) {
	return ports.ValidationReport{}, nil
}

func (f *fakeFacade) Status(_ context.Context, _ string) (ports.RunStatus, error) {
	return ports.RunStatus{}, nil
}

func (f *fakeFacade) History(_ context.Context, _ ports.HistoryFilter) ([]ports.RunRecord, error) {
	return nil, nil
}

func (f *fakeFacade) Run(ctx context.Context, req ports.RunRequest) (ports.RunSession, error) {
	if req.Identifier == "" {
		return nil, ports.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessions == nil {
		f.sessions = make(map[string]*fakeSession)
	}
	s := newFakeSession(req.Identifier)
	f.sessions[req.Identifier] = s
	return s, nil
}

// Resume returns ErrRunNotFound when the RunID is unknown, satisfying the port
// contract that callers must be able to detect a missing run with errors.Is.
// When the run is known it returns the existing session (as if reconnecting).
func (f *fakeFacade) Resume(_ context.Context, req ports.ResumeRequest) (ports.RunSession, error) {
	if req.RunID == "" {
		return nil, ports.ErrInvalidRequest
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[req.RunID]
	if !ok {
		return nil, ports.ErrRunNotFound
	}
	return s, nil
}

// fakeSingleStepRunner is a minimal SingleStepRunner for contract verification.
// It is separate from fakeFacade because RunStep is not part of WorkflowFacade.
type fakeSingleStepRunner struct{}

var _ ports.SingleStepRunner = (*fakeSingleStepRunner)(nil)

func (r *fakeSingleStepRunner) RunStep(_ context.Context, _ ports.RunStepRequest) (ports.StepResult, error) {
	return ports.StepResult{}, nil
}

// ---------------------------------------------------------------------------
// RunSession contract tests
// ---------------------------------------------------------------------------

func TestWorkflowFacadeContract_RunSessionCloseIdempotent(t *testing.T) {
	facade := &fakeFacade{}
	sess, err := facade.Run(context.Background(), ports.RunRequest{Identifier: "pack/workflow"})
	require.NoError(t, err)
	require.NoError(t, sess.Close())
	assert.NoError(t, sess.Close())
}

func TestWorkflowFacadeContract_EventsChannelClosedAfterClose(t *testing.T) {
	facade := &fakeFacade{}
	sess, err := facade.Run(context.Background(), ports.RunRequest{Identifier: "pack/workflow"})
	require.NoError(t, err)
	require.NoError(t, sess.Close())
	_, open := <-sess.Events()
	assert.False(t, open, "events channel must be closed after Close()")
}

func TestWorkflowFacadeContract_RunZeroRequestReturnsErrInvalidRequest(t *testing.T) {
	facade := &fakeFacade{}
	_, err := facade.Run(context.Background(), ports.RunRequest{})
	assert.ErrorIs(t, err, ports.ErrInvalidRequest)
}

func TestWorkflowFacadeContract_RunCtxCanceledPropagates(t *testing.T) {
	facade := &fakeFacade{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := facade.Run(ctx, ports.RunRequest{Identifier: "pack/workflow"})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestWorkflowFacadeContract_RespondAfterCloseReturnsErrSessionClosed(t *testing.T) {
	facade := &fakeFacade{}
	sess, err := facade.Run(context.Background(), ports.RunRequest{Identifier: "pack/workflow"})
	require.NoError(t, err)
	require.NoError(t, sess.Close())
	err = sess.Respond(ports.InputResponse{})
	assert.ErrorIs(t, err, ports.ErrSessionClosed)
}

// ---------------------------------------------------------------------------
// Resume contract tests
// ---------------------------------------------------------------------------

func TestWorkflowFacadeContract_ResumeUnknownRunIDReturnsErrRunNotFound(t *testing.T) {
	facade := &fakeFacade{}
	_, err := facade.Resume(context.Background(), ports.ResumeRequest{RunID: "no-such-run"})
	assert.ErrorIs(t, err, ports.ErrRunNotFound)
}

func TestWorkflowFacadeContract_ResumeEmptyRunIDReturnsErrInvalidRequest(t *testing.T) {
	facade := &fakeFacade{}
	_, err := facade.Resume(context.Background(), ports.ResumeRequest{})
	assert.ErrorIs(t, err, ports.ErrInvalidRequest)
}

func TestWorkflowFacadeContract_ResumeKnownRunIDReturnsUsableSession(t *testing.T) {
	facade := &fakeFacade{}
	const runID = "pack/workflow"

	// Seed a known session via Run.
	_, err := facade.Run(context.Background(), ports.RunRequest{Identifier: runID})
	require.NoError(t, err)

	// Resume must return a non-nil session for a known run.
	sess, err := facade.Resume(context.Background(), ports.ResumeRequest{RunID: runID})
	require.NoError(t, err)
	require.NotNil(t, sess, "Resume must return a usable session for a known run ID")
	assert.Equal(t, runID, sess.ID())
}

// ---------------------------------------------------------------------------
// EventKind.String() coverage
// ---------------------------------------------------------------------------

func TestEventKind_String(t *testing.T) {
	tests := []struct {
		kind ports.EventKind
		want string
	}{
		{ports.EventKindUnknown, "unknown"},
		{ports.EventRunStarted, "run.started"},
		{ports.EventRunCompleted, "run.completed"},
		{ports.EventStepStarted, "step.started"},
		{ports.EventStepCompleted, "step.completed"},
		{ports.EventStepCallWorkflowStarted, "step.call_workflow.started"},
		{ports.EventStepCallWorkflowCompleted, "step.call_workflow.completed"},
		{ports.EventMessageUser, "message.user"},
		{ports.EventMessageAssistant, "message.assistant"},
		{ports.EventToolCall, "tool.call"},
		{ports.EventToolResult, "tool.result"},
		{ports.EventInputRequired, "input.required"},
		{ports.EventWorkflowCompleted, "workflow.completed"},
		{ports.EventWorkflowFailed, "workflow.failed"},
		// Out-of-range value must fall through to the "unknown" default.
		{ports.EventKind(255), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}
