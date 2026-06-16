package application

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// blockingWorkflowRepo is a WorkflowRepository that blocks Load calls until released.
// Used to exercise the semaphore ctx-cancellation path in HandleSessionNew (issue #2).
type blockingWorkflowRepo struct {
	infos   []ports.WorkflowInfo
	block   chan struct{} // closed to release all blocked Load calls
	started chan struct{} // receives one send per Load that started
}

func newBlockingWorkflowRepo(n int) *blockingWorkflowRepo {
	infos := make([]ports.WorkflowInfo, n)
	for i := range infos {
		infos[i] = ports.WorkflowInfo{
			Name:   fmt.Sprintf("wf-%d", i),
			Source: ports.SourceLocal,
		}
	}
	return &blockingWorkflowRepo{
		infos:   infos,
		block:   make(chan struct{}),
		started: make(chan struct{}, n),
	}
}

func (r *blockingWorkflowRepo) ListWithSource(_ context.Context) ([]ports.WorkflowInfo, error) {
	return r.infos, nil
}

func (r *blockingWorkflowRepo) Load(ctx context.Context, _ string) (*workflow.Workflow, error) {
	r.started <- struct{}{}
	select {
	case <-r.block:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *blockingWorkflowRepo) List(_ context.Context) ([]string, error) {
	names := make([]string, len(r.infos))
	for i, info := range r.infos {
		names[i] = info.Name
	}
	return names, nil
}

func (r *blockingWorkflowRepo) Exists(_ context.Context, name string) (bool, error) {
	for _, info := range r.infos {
		if info.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// TestACPSessionService_Issue2_SemaphoreUnblocksOnCtxCancel verifies that cancelling the
// context while HandleSessionNew is running its bounded parallel workflow loads causes all
// goroutines to exit and wg.Wait() to return — not deadlock.
//
// Pre-fix: sem <- struct{}{} was a plain channel send that blocked forever when all 8 slots
// were occupied and ctx was cancelled. With 12 workflows, 4 goroutines would queue and never
// unblock if the context was cancelled before a slot freed up.
//
// Post-fix: the select { case sem <- struct{}{}: case <-ctx.Done(): return } unblocks queued
// goroutines immediately when ctx is cancelled, allowing wg.Wait() and the handler to return.
func TestACPSessionService_Issue2_SemaphoreUnblocksOnCtxCancel(t *testing.T) {
	// 12 workflows: more than maxParallelLoads (8), so 4 goroutines will queue on the semaphore.
	const numWorkflows = 12
	repo := newBlockingWorkflowRepo(numWorkflows)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	svc := &ACPSessionService{workflowRepo: repo, logger: ports.NopLogger{}}

	handlerDone := make(chan *ACPHandlerError, 1)
	go func() {
		_, err := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
		handlerDone <- err
	}()

	// Wait until at least 8 Load calls have started (semaphore is now full).
	// The remaining 4 goroutines are blocked waiting for a slot.
	for i := range 8 {
		select {
		case <-repo.started:
		case <-time.After(2 * time.Second):
			t.Fatalf("issue #2: only %d Load calls started before timeout (expected 8)", i)
		}
	}

	// Cancel the context. The 4 queued goroutines must unblock via ctx.Done() and return.
	// The 8 running goroutines will also return via ctx.Done() in their Load implementation.
	cancelCtx()

	select {
	case <-handlerDone:
		// Handler returned — wg.Wait() unblocked correctly. Test passes.
	case <-time.After(3 * time.Second):
		t.Fatal("issue #2: HandleSessionNew deadlocked after ctx cancellation — semaphore not ctx-aware")
	}
}

// TestACPSessionService_Issue8_ShutdownRejectsNewSessions verifies that once Shutdown has
// started, HandleSessionNew returns an ACPErrInternal immediately rather than creating a
// session whose resources would be leaked (created between the two Range passes in Shutdown).
func TestACPSessionService_Issue8_ShutdownRejectsNewSessions(t *testing.T) {
	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{}, nil)

	svc := &ACPSessionService{workflowRepo: mockRepo, logger: ports.NopLogger{}}

	// Confirm that before Shutdown, HandleSessionNew succeeds.
	_, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr, "session/new must succeed before Shutdown is called")

	// Trigger shutdown.
	svc.Shutdown()

	// After Shutdown, HandleSessionNew must be rejected immediately.
	_, acpErr = svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.NotNil(t, acpErr, "issue #8: session/new must be rejected after Shutdown")
	assert.Equal(t, ACPErrInternal, acpErr.Kind,
		"issue #8: post-shutdown session/new must return ACPErrInternal")
	assert.Contains(t, acpErr.Message, "shutting down",
		"issue #8: error message must indicate the server is shutting down")
}
