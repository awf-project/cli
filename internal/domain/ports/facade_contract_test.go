package ports_test

import (
	"context"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func (f *fakeFacade) Resume(_ context.Context, _ string) (ports.RunSession, error) {
	return nil, nil
}

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
