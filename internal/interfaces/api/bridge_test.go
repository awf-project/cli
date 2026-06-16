package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/facadetest"
)

// --- mock implementations of Bridge interfaces ---

type mockWorkflowLister struct {
	entries          []workflow.WorkflowEntry
	wfs              map[string]*workflow.Workflow
	listErr          error
	getErr           error
	validErr         error
	lastGetName      string
	lastValidateName string
}

func newMockWorkflowLister(names ...string) *mockWorkflowLister {
	m := &mockWorkflowLister{
		entries: make([]workflow.WorkflowEntry, 0, len(names)),
		wfs:     make(map[string]*workflow.Workflow, len(names)),
	}
	for _, name := range names {
		scope, wfName, _ := strings.Cut(name, "/")
		if !strings.Contains(name, "/") {
			scope = "local"
			wfName = name
		}
		m.entries = append(m.entries, workflow.WorkflowEntry{
			Name:     name,
			Source:   "local",
			Scope:    scope,
			Workflow: wfName,
		})
		m.wfs[name] = &workflow.Workflow{
			Name:  name,
			Steps: map[string]*workflow.Step{"step-1": {Name: "step-1"}},
		}
	}
	return m
}

func (m *mockWorkflowLister) ListAllWorkflows(_ context.Context) ([]workflow.WorkflowEntry, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.entries, nil
}

func (m *mockWorkflowLister) GetWorkflow(_ context.Context, name string) (*workflow.Workflow, error) {
	m.lastGetName = name
	if m.getErr != nil {
		return nil, m.getErr
	}
	wf, ok := m.wfs[name]
	if !ok {
		// Mirror infrastructure: unknown workflow → StructuredError with
		// ErrorCodeUserInputMissingFile so handler tests observe the real contract.
		return nil, domainerrors.NewUserError(
			domainerrors.ErrorCodeUserInputMissingFile,
			"workflow not found: "+name,
			nil, nil,
		)
	}
	return wf, nil
}

func (m *mockWorkflowLister) ValidateWorkflow(_ context.Context, name string) error {
	m.lastValidateName = name
	return m.validErr
}

type mockHistoryProvider struct {
	records  []*workflow.ExecutionRecord
	stats    *workflow.HistoryStats
	listErr  error
	statsErr error
}

func newMockHistoryProvider() *mockHistoryProvider {
	return &mockHistoryProvider{
		records: []*workflow.ExecutionRecord{
			{ID: "rec-1", WorkflowName: "wf-a", Status: "success"},
		},
		stats: &workflow.HistoryStats{
			TotalExecutions: 1,
			SuccessCount:    1,
		},
	}
}

func (m *mockHistoryProvider) List(_ context.Context, _ *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.records, nil
}

func (m *mockHistoryProvider) GetStats(_ context.Context, _ *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	return m.stats, nil
}

// --- seedExecution: white-box test helper ---

// seedExecution inserts a synthetic ActiveExecution into b.activeExecutions.
// It derives a real cancellable context from context.Background() so that
// callers can verify CancelExecution / Shutdown propagate cancellation.
// The Done channel is pre-closed (the entry is already complete from the bridge's
// perspective) — this mirrors the contract of TrackFacadeSession.
// Returns the execution ID stored as the map key.
func seedExecution(b *Bridge, workflowName string) (string, *ActiveExecution) {
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is stored in ActiveExecution.Cancel and invoked by CancelExecution/Shutdown under test
	execCtx := workflow.NewExecutionContext(workflowName+"-ctx", workflowName)
	id := workflowName + "-seed-" + "001"
	ae := &ActiveExecution{
		ExecutionID:      id,
		WorkflowName:     workflowName,
		Ctx:              ctx,
		Cancel:           cancel,
		ExecutionContext: execCtx,
	}
	b.activeExecutions.Store(id, ae)
	return id, ae
}

// seedExecutionWithID inserts a synthetic ActiveExecution with a caller-chosen ID.
// Useful when the test needs two entries with distinct workflow names and stable IDs.
func seedExecutionWithID(b *Bridge, id, workflowName string) *ActiveExecution {
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is stored in ActiveExecution.Cancel and invoked by CancelExecution/Shutdown under test
	execCtx := workflow.NewExecutionContext(workflowName+"-ctx", workflowName)
	ae := &ActiveExecution{
		ExecutionID:      id,
		WorkflowName:     workflowName,
		Ctx:              ctx,
		Cancel:           cancel,
		ExecutionContext: execCtx,
	}
	b.activeExecutions.Store(id, ae)
	return ae
}

// --- tests ---

func TestBridge_CancelExecution_CallsCancelFunc(t *testing.T) {
	bridge := NewBridge()
	id, exec := seedExecution(bridge, "wf-1")

	cancelErr := bridge.CancelExecution(id)

	assert.NoError(t, cancelErr)
	// context must be cancelled after CancelExecution
	assert.Error(t, exec.Ctx.Err(), "ctx must be done after cancel")

	// idempotent: second call must not panic
	assert.NotPanics(t, func() { _ = bridge.CancelExecution(id) })
}

func TestBridge_CancelExecution_UnknownID_ReturnsError(t *testing.T) {
	bridge := NewBridge()

	err := bridge.CancelExecution("does-not-exist")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
}

func TestBridge_GetExecution_LiveSnapshot(t *testing.T) {
	bridge := NewBridge()
	id, _ := seedExecution(bridge, "wf-1")

	exec, ok := bridge.GetExecution(id)

	assert.True(t, ok)
	require.NotNil(t, exec)
	assert.Equal(t, id, exec.ExecutionID)
	assert.Equal(t, "wf-1", exec.WorkflowName)
}

func TestBridge_Shutdown_CancelsAllActiveExecutions(t *testing.T) {
	bridge := NewBridge()

	execA := seedExecutionWithID(bridge, "exec-a", "wf-a")
	execB := seedExecutionWithID(bridge, "exec-b", "wf-b")

	// Both contexts must be live before Shutdown.
	require.NoError(t, execA.Ctx.Err(), "execA context must not be cancelled before Shutdown")
	require.NoError(t, execB.Ctx.Err(), "execB context must not be cancelled before Shutdown")

	bridge.Shutdown()

	assert.Error(t, execA.Ctx.Err(), "execA context must be cancelled after Shutdown")
	assert.ErrorIs(t, execA.Ctx.Err(), context.Canceled)
	assert.Error(t, execB.Ctx.Err(), "execB context must be cancelled after Shutdown")
	assert.ErrorIs(t, execB.Ctx.Err(), context.Canceled)
}

func TestBridge_Shutdown_EmptyMap_DoesNotPanic(t *testing.T) {
	bridge := NewBridge()
	// Must not panic when no executions are tracked.
	assert.NotPanics(t, func() { bridge.Shutdown() })
}

func TestBridge_Shutdown_Idempotent(t *testing.T) {
	bridge := NewBridge()
	seedExecution(bridge, "wf-1")

	// Second Shutdown must not panic — context.CancelFunc is idempotent.
	assert.NotPanics(t, func() {
		bridge.Shutdown()
		bridge.Shutdown()
	})
}

// --- T073: TrackFacadeSession ---

// TestBridge_TrackFacadeSession_StoresSessionIDAndWorkflowName verifies that
// TrackFacadeSession stores the facade RunSession in activeExecutions so that
// subsequent List/Get/Cancel calls resolve by the session's own ID.
func TestBridge_TrackFacadeSession_StoresSessionIDAndWorkflowName(t *testing.T) {
	bridge := NewBridge()

	fake := facadetest.New().WithTerminalCompleted()
	sess, err := fake.Run(context.Background(), ports.RunRequest{Identifier: "deploy-prod"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	ae := bridge.TrackFacadeSession(sess, "deploy-prod")

	require.NotNil(t, ae, "TrackFacadeSession must return a non-nil ActiveExecution")
	assert.Equal(t, sess.ID(), ae.ExecutionID, "ActiveExecution.ExecutionID must equal session.ID()")
	assert.Equal(t, "deploy-prod", ae.WorkflowName)

	stored, ok := bridge.GetExecution(sess.ID())
	assert.True(t, ok, "entry must be stored in activeExecutions under session.ID()")
	assert.NotNil(t, stored)
}

// TestBridge_TrackFacadeSession_GetExecutionReturnsEntry verifies that an entry
// created via TrackFacadeSession is returned by GetExecution so that the HTTP
// GET /api/executions/{id} handler resolves it correctly.
func TestBridge_TrackFacadeSession_GetExecutionReturnsEntry(t *testing.T) {
	bridge := NewBridge()

	fake := facadetest.New().WithTerminalCompleted()
	sess, err := fake.Run(context.Background(), ports.RunRequest{Identifier: "wf-x"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	bridge.TrackFacadeSession(sess, "wf-x")

	stored, ok := bridge.GetExecution(sess.ID())
	require.True(t, ok)
	require.NotNil(t, stored)
	assert.Equal(t, sess.ID(), stored.ExecutionID)
	assert.Equal(t, "wf-x", stored.WorkflowName)
}

func TestBridge_ListExecutions_ReturnsActiveAndCompleted(t *testing.T) {
	bridge := NewBridge()

	seedExecutionWithID(bridge, "id-list-a", "wf-a")
	seedExecutionWithID(bridge, "id-list-b", "wf-b")

	time.Sleep(10 * time.Millisecond)

	list := bridge.ListExecutions()

	assert.Len(t, list, 2)
	workflowNames := make([]string, len(list))
	for i, e := range list {
		workflowNames[i] = e.WorkflowName
	}
	assert.ElementsMatch(t, []string{"wf-a", "wf-b"}, workflowNames)
}
