package api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
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
		return nil, errors.New("workflow not found: " + name)
	}
	return wf, nil
}

func (m *mockWorkflowLister) ValidateWorkflow(_ context.Context, name string) error {
	m.lastValidateName = name
	return m.validErr
}

type mockWorkflowRunner struct {
	execCtx *workflow.ExecutionContext
	runErr  error
	done    <-chan error
}

func newMockWorkflowRunner() *mockWorkflowRunner {
	ctx := workflow.NewExecutionContext("exec-001", "test-workflow")
	return &mockWorkflowRunner{execCtx: ctx}
}

func newMockWorkflowRunnerWithDone(done <-chan error) *mockWorkflowRunner {
	ctx := workflow.NewExecutionContext("exec-001", "test-workflow")
	return &mockWorkflowRunner{execCtx: ctx, done: done}
}

func (m *mockWorkflowRunner) RunWorkflowAsync(
	_ context.Context,
	wf *workflow.Workflow,
	_ map[string]any,
) (*workflow.ExecutionContext, <-chan error, error) {
	if m.runErr != nil {
		return nil, nil, m.runErr
	}
	// If a custom done channel was set, use it; otherwise create a buffered one
	done := m.done
	if done == nil {
		buffered := make(chan error, 1)
		buffered <- nil
		done = buffered
	}
	// Create a fresh execution context for each run with workflow name
	execCtx := workflow.NewExecutionContext(wf.Name+"-ctx", wf.Name)
	return execCtx, done, nil
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

// --- tests ---

func TestBridge_NewBridge_WiresDependencies(t *testing.T) {
	lister := newMockWorkflowLister("wf-1")
	runner := newMockWorkflowRunner()
	history := newMockHistoryProvider()

	bridge := NewBridge(lister, runner, history)

	require.NotNil(t, bridge)
	assert.NotNil(t, bridge.workflows, "workflows dep must be wired")
	assert.NotNil(t, bridge.runner, "runner dep must be wired")
	assert.NotNil(t, bridge.history, "history dep must be wired")
}

func TestBridge_StartExecution_StoresInMap_AndReturnsID(t *testing.T) {
	// Use a blocking channel so the cleanup goroutine does not remove the entry
	// before GetExecution is called below.
	block := make(chan error)
	t.Cleanup(func() { close(block) })

	runner := newMockWorkflowRunnerWithDone(block)
	bridge := NewBridge(newMockWorkflowLister("wf-1"), runner, newMockHistoryProvider())
	wf := &workflow.Workflow{Name: "wf-1", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}

	id, exec, err := bridge.StartExecution(context.Background(), wf, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, id, "must return a non-empty execution ID")
	require.NotNil(t, exec)
	assert.Equal(t, id, exec.ExecutionID)
	assert.Equal(t, "wf-1", exec.WorkflowName)
	assert.NotNil(t, exec.Cancel)
	assert.NotNil(t, exec.Done)
	assert.NotNil(t, exec.ExecutionContext)

	stored, ok := bridge.GetExecution(id)
	assert.True(t, ok, "execution must be stored in sync.Map")
	assert.Equal(t, exec, stored)
}

func TestBridge_StartExecution_RunnerError_ReturnsError(t *testing.T) {
	runner := newMockWorkflowRunner()
	runner.runErr = errors.New("runner failed")
	bridge := NewBridge(newMockWorkflowLister("wf-1"), runner, newMockHistoryProvider())
	wf := &workflow.Workflow{Name: "wf-1", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}

	id, exec, err := bridge.StartExecution(context.Background(), wf, nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "runner failed")
	assert.Empty(t, id)
	assert.Nil(t, exec)
}

func TestBridge_CancelExecution_CallsCancelFunc(t *testing.T) {
	// Blocking channel keeps the entry in sync.Map until the test completes,
	// so both CancelExecution calls find it and the first does not return "not found".
	block := make(chan error)
	t.Cleanup(func() { close(block) })

	runner := newMockWorkflowRunnerWithDone(block)
	bridge := NewBridge(newMockWorkflowLister("wf-1"), runner, newMockHistoryProvider())
	wf := &workflow.Workflow{Name: "wf-1", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	id, exec, err := bridge.StartExecution(context.Background(), wf, nil)
	require.NoError(t, err)

	cancelErr := bridge.CancelExecution(id)

	assert.NoError(t, cancelErr)
	// context must be cancelled after CancelExecution
	assert.Error(t, exec.Ctx.Err(), "ctx must be done after cancel")

	// idempotent: second call must not panic
	assert.NotPanics(t, func() { _ = bridge.CancelExecution(id) })
}

func TestBridge_CancelExecution_UnknownID_ReturnsError(t *testing.T) {
	bridge := NewBridge(newMockWorkflowLister(), newMockWorkflowRunner(), newMockHistoryProvider())

	err := bridge.CancelExecution("does-not-exist")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
}

func TestBridge_GetExecution_LiveSnapshot(t *testing.T) {
	// Blocking channel prevents the cleanup goroutine from removing the entry
	// before GetExecution is called.
	block := make(chan error)
	t.Cleanup(func() { close(block) })

	runner := newMockWorkflowRunnerWithDone(block)
	bridge := NewBridge(newMockWorkflowLister("wf-1"), runner, newMockHistoryProvider())
	wf := &workflow.Workflow{Name: "wf-1", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	id, _, err := bridge.StartExecution(context.Background(), wf, nil)
	require.NoError(t, err)

	exec, ok := bridge.GetExecution(id)

	assert.True(t, ok)
	require.NotNil(t, exec)
	assert.Equal(t, id, exec.ExecutionID)
	assert.Equal(t, "wf-1", exec.WorkflowName)
}

func TestBridge_TrackResumedExecution_PersistsEntryForSubsequentQueries(t *testing.T) {
	// Resumed executions are intentionally persisted so the /resume handler can
	// return an ID that subsequent GET /api/executions/{id} (and the SSE/DELETE
	// endpoints) can serve. Eviction/TTL of completed entries is out of scope
	// here; this test guards against accidental immediate-cleanup regressions.
	bridge := NewBridge(newMockWorkflowLister(), newMockWorkflowRunner(), newMockHistoryProvider())
	execCtx := workflow.NewExecutionContext("resumed-001", "my-workflow")

	id := bridge.TrackResumedExecution(execCtx)
	require.NotEmpty(t, id, "must return a non-empty execution ID")

	stored, ok := bridge.GetExecution(id)
	require.True(t, ok, "entry must be queryable immediately after TrackResumedExecution")
	require.NotNil(t, stored)
	assert.Equal(t, id, stored.ExecutionID)
	assert.Equal(t, "my-workflow", stored.WorkflowName)
	assert.Same(t, execCtx, stored.ExecutionContext)
}

func TestBridge_ListExecutions_ReturnsActiveAndCompleted(t *testing.T) {
	// Use blocking channels to prevent cleanup goroutine from removing entries
	blockA := make(chan error)
	blockB := make(chan error)
	t.Cleanup(func() { close(blockA); close(blockB) })

	runner := &mockWorkflowRunner{
		execCtx: workflow.NewExecutionContext("exec-a", "wf-a"),
	}
	bridge := NewBridge(newMockWorkflowLister("wf-a", "wf-b"), runner, newMockHistoryProvider())

	wfA := &workflow.Workflow{Name: "wf-a", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	runner.done = blockA
	_, _, err := bridge.StartExecution(context.Background(), wfA, nil)
	require.NoError(t, err)

	wfB := &workflow.Workflow{Name: "wf-b", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	runner.done = blockB
	_, _, err = bridge.StartExecution(context.Background(), wfB, nil)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	list := bridge.ListExecutions()

	assert.Len(t, list, 2)
	workflowNames := make([]string, len(list))
	for i, e := range list {
		workflowNames[i] = e.WorkflowName
	}
	assert.ElementsMatch(t, []string{"wf-a", "wf-b"}, workflowNames)
}
