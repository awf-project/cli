package tui_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/tui"
)

// --- mock implementations of Bridge interfaces ---

// mockWorkflowLister satisfies tui.WorkflowLister.
type mockWorkflowLister struct {
	entries  []workflow.WorkflowEntry
	wfs      map[string]*workflow.Workflow
	listErr  error
	getErr   error
	validErr error
}

func newMockWorkflowLister(names ...string) *mockWorkflowLister {
	m := &mockWorkflowLister{
		entries: make([]workflow.WorkflowEntry, 0, len(names)),
		wfs:     make(map[string]*workflow.Workflow, len(names)),
	}
	for _, name := range names {
		m.entries = append(m.entries, workflow.WorkflowEntry{Name: name, Source: "local"})
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
	if m.getErr != nil {
		return nil, m.getErr
	}
	wf, ok := m.wfs[name]
	if !ok {
		return nil, errors.New("workflow not found: " + name)
	}
	return wf, nil
}

func (m *mockWorkflowLister) ValidateWorkflow(_ context.Context, _ string) error {
	return m.validErr
}

// mockWorkflowRunner satisfies tui.WorkflowRunner.
type mockWorkflowRunner struct {
	execCtx *workflow.ExecutionContext
	runErr  error
}

func newMockWorkflowRunner() *mockWorkflowRunner {
	ctx := workflow.NewExecutionContext("exec-001", "test-workflow")
	return &mockWorkflowRunner{execCtx: ctx}
}

func (m *mockWorkflowRunner) RunWorkflowAsync(
	_ context.Context,
	_ *workflow.Workflow,
	_ map[string]any,
) (*workflow.ExecutionContext, <-chan error, error) {
	if m.runErr != nil {
		return nil, nil, m.runErr
	}
	done := make(chan error, 1)
	done <- nil
	return m.execCtx, done, nil
}

// mockHistoryProvider satisfies tui.HistoryProvider.
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

// --- helpers ---

// newTestBridge builds a Bridge backed by mocks for all three interfaces.
func newTestBridge(t *testing.T) (*tui.Bridge, *mockWorkflowLister, *mockWorkflowRunner, *mockHistoryProvider) {
	t.Helper()
	lister := newMockWorkflowLister("test-workflow")
	runner := newMockWorkflowRunner()
	history := newMockHistoryProvider()
	bridge := tui.NewBridge(lister, runner, history)
	return bridge, lister, runner, history
}

// --- NewBridge ---

func TestBridge_NewBridge_WiresDependencies(t *testing.T) {
	lister := newMockWorkflowLister("wf-1")
	runner := newMockWorkflowRunner()
	history := newMockHistoryProvider()

	bridge := tui.NewBridge(lister, runner, history)

	require.NotNil(t, bridge)
}

// --- LoadWorkflows ---

func TestBridge_LoadWorkflows_ReturnsNonNilCmd(t *testing.T) {
	bridge, _, _, _ := newTestBridge(t)

	cmd := bridge.LoadWorkflows(context.Background())

	assert.NotNil(t, cmd, "LoadWorkflows must return a non-nil tea.Cmd")
}

func TestBridge_LoadWorkflows_EmitsWorkflowsLoadedMsg(t *testing.T) {
	bridge, _, _, _ := newTestBridge(t)

	msg := bridge.LoadWorkflows(context.Background())()

	loaded, ok := msg.(tui.WorkflowsLoadedMsg)
	require.True(t, ok, "expected WorkflowsLoadedMsg, got %T", msg)
	assert.NotEmpty(t, loaded.Workflows, "loaded message should contain available workflows")
	assert.Equal(t, "test-workflow", loaded.Workflows[0].Name)
	assert.NotEmpty(t, loaded.Entries, "loaded message should contain workflow entries")
	assert.Equal(t, "test-workflow", loaded.Entries[0].Name)
}

func TestBridge_LoadWorkflows_RespectsCancellation(t *testing.T) {
	bridge, _, _, _ := newTestBridge(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	msg := bridge.LoadWorkflows(ctx)()

	errMsg, ok := msg.(tui.ErrMsg)
	require.True(t, ok, "expected ErrMsg on cancelled context, got %T", msg)
	assert.Error(t, errMsg.Err)
}

func TestBridge_LoadWorkflows_ListError_EmitsErrMsg(t *testing.T) {
	lister := newMockWorkflowLister()
	lister.listErr = errors.New("repository unavailable")
	bridge := tui.NewBridge(lister, newMockWorkflowRunner(), newMockHistoryProvider())

	msg := bridge.LoadWorkflows(context.Background())()

	errMsg, ok := msg.(tui.ErrMsg)
	require.True(t, ok, "expected ErrMsg on list error, got %T", msg)
	assert.ErrorContains(t, errMsg.Err, "repository unavailable")
}

func TestBridge_LoadWorkflows_GetWorkflowError_SkipsFailedWorkflow(t *testing.T) {
	lister := newMockWorkflowLister("wf-1")
	lister.getErr = errors.New("workflow load failed")
	bridge := tui.NewBridge(lister, newMockWorkflowRunner(), newMockHistoryProvider())

	msg := bridge.LoadWorkflows(context.Background())()

	loaded, ok := msg.(tui.WorkflowsLoadedMsg)
	require.True(t, ok, "expected WorkflowsLoadedMsg even on get error, got %T", msg)
	assert.Empty(t, loaded.Workflows, "failed workflows should be skipped")
	assert.NotEmpty(t, loaded.Entries, "entries should still be populated even when workflow loading fails")
}

func TestBridge_LoadWorkflows_PackWorkflow_NormalizesName(t *testing.T) {
	lister := &mockWorkflowLister{
		entries: []workflow.WorkflowEntry{
			{Name: "my-pack/deploy", Source: "pack", Version: "1.0.0"},
		},
		wfs: map[string]*workflow.Workflow{
			"my-pack/deploy": {
				Name:  "deploy",
				Steps: map[string]*workflow.Step{"s1": {Name: "s1"}},
				Inputs: []workflow.Input{
					{Name: "target", Required: true, Type: "string"},
				},
			},
		},
	}
	bridge := tui.NewBridge(lister, newMockWorkflowRunner(), newMockHistoryProvider())

	msg := bridge.LoadWorkflows(context.Background())()

	loaded, ok := msg.(tui.WorkflowsLoadedMsg)
	require.True(t, ok, "expected WorkflowsLoadedMsg, got %T", msg)
	require.Len(t, loaded.Workflows, 1)
	assert.Equal(t, "my-pack/deploy", loaded.Workflows[0].Name,
		"pack workflow name must match entry name for proper TUI mapping")
	assert.Len(t, loaded.Workflows[0].Inputs, 1,
		"inputs must be preserved after name normalization")
}

func TestBridge_LoadWorkflows_EmptyList_EmitsEmptySlice(t *testing.T) {
	lister := newMockWorkflowLister() // no workflows
	bridge := tui.NewBridge(lister, newMockWorkflowRunner(), newMockHistoryProvider())

	msg := bridge.LoadWorkflows(context.Background())()

	loaded, ok := msg.(tui.WorkflowsLoadedMsg)
	require.True(t, ok, "expected WorkflowsLoadedMsg, got %T", msg)
	assert.Empty(t, loaded.Workflows)
	assert.Empty(t, loaded.Entries)
}

// --- LoadHistory ---

func TestBridge_LoadHistory_ReturnsNonNilCmd(t *testing.T) {
	bridge, _, _, _ := newTestBridge(t)

	cmd := bridge.LoadHistory(context.Background())

	assert.NotNil(t, cmd, "LoadHistory must return a non-nil tea.Cmd")
}

func TestBridge_LoadHistory_EmitsHistoryLoadedMsg(t *testing.T) {
	bridge, _, _, history := newTestBridge(t)

	msg := bridge.LoadHistory(context.Background())()

	loaded, ok := msg.(tui.HistoryLoadedMsg)
	require.True(t, ok, "expected HistoryLoadedMsg, got %T", msg)
	assert.NotEmpty(t, loaded.Records)
	assert.Equal(t, history.records[0].ID, loaded.Records[0].ID)
	assert.NotNil(t, loaded.Stats)
	assert.Equal(t, history.stats.TotalExecutions, loaded.Stats.TotalExecutions)
}

func TestBridge_LoadHistory_RespectsCancellation(t *testing.T) {
	bridge, _, _, _ := newTestBridge(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := bridge.LoadHistory(ctx)()

	errMsg, ok := msg.(tui.ErrMsg)
	require.True(t, ok, "expected ErrMsg on cancelled context, got %T", msg)
	assert.Error(t, errMsg.Err)
}

func TestBridge_LoadHistory_ListError_EmitsErrMsg(t *testing.T) {
	history := newMockHistoryProvider()
	history.listErr = errors.New("history store offline")
	bridge := tui.NewBridge(newMockWorkflowLister("wf-1"), newMockWorkflowRunner(), history)

	msg := bridge.LoadHistory(context.Background())()

	errMsg, ok := msg.(tui.ErrMsg)
	require.True(t, ok, "expected ErrMsg on list error, got %T", msg)
	assert.ErrorContains(t, errMsg.Err, "history store offline")
}

func TestBridge_LoadHistory_StatsError_EmitsErrMsg(t *testing.T) {
	history := newMockHistoryProvider()
	history.statsErr = errors.New("stats query failed")
	bridge := tui.NewBridge(newMockWorkflowLister("wf-1"), newMockWorkflowRunner(), history)

	msg := bridge.LoadHistory(context.Background())()

	errMsg, ok := msg.(tui.ErrMsg)
	require.True(t, ok, "expected ErrMsg on stats error, got %T", msg)
	assert.ErrorContains(t, errMsg.Err, "stats query failed")
}

// --- RunWorkflow ---

func TestBridge_RunWorkflow_ReturnsNonNilCmd(t *testing.T) {
	bridge, _, _, _ := newTestBridge(t)
	wf := &workflow.Workflow{
		Name:  "test-workflow",
		Steps: map[string]*workflow.Step{"step-1": {Name: "step-1"}},
	}

	cmd := bridge.RunWorkflow(context.Background(), wf, nil)

	assert.NotNil(t, cmd, "RunWorkflow must return a non-nil tea.Cmd")
}

func TestBridge_RunWorkflow_EmitsExecutionStartedMsg(t *testing.T) {
	bridge, _, runner, _ := newTestBridge(t)
	wf := &workflow.Workflow{
		Name:  "test-workflow",
		Steps: map[string]*workflow.Step{"step-1": {Name: "step-1"}},
	}

	msg := bridge.RunWorkflow(context.Background(), wf, map[string]any{"key": "value"})()

	started, ok := msg.(tui.ExecutionStartedMsg)
	require.True(t, ok, "expected ExecutionStartedMsg, got %T", msg)
	assert.Equal(t, "test-workflow", started.ExecutionID)
	assert.NotNil(t, started.Done, "Done channel must be set")
	_ = runner // runner executes asynchronously
}

func TestBridge_RunWorkflow_WithEmptySteps_FetchesWorkflowFirst(t *testing.T) {
	bridge, lister, _, _ := newTestBridge(t)
	lister.wfs["test-workflow"] = &workflow.Workflow{
		Name:  "test-workflow",
		Steps: map[string]*workflow.Step{"step-loaded": {Name: "step-loaded"}},
	}

	wf := &workflow.Workflow{Name: "test-workflow"}

	msg := bridge.RunWorkflow(context.Background(), wf, nil)()

	started, ok := msg.(tui.ExecutionStartedMsg)
	require.True(t, ok, "expected ExecutionStartedMsg, got %T", msg)
	assert.Equal(t, "test-workflow", started.ExecutionID)
	assert.NotNil(t, started.Done, "Done channel must be set")
}

func TestBridge_RunWorkflow_RespectsCancellation(t *testing.T) {
	bridge, _, _, _ := newTestBridge(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := bridge.RunWorkflow(ctx, &workflow.Workflow{Name: "test"}, nil)()

	errMsg, ok := msg.(tui.ErrMsg)
	require.True(t, ok, "expected ErrMsg on cancelled context, got %T", msg)
	assert.Error(t, errMsg.Err)
}

func TestBridge_RunWorkflow_RunnerError_ReturnsErrMsgDirectly(t *testing.T) {
	runner := newMockWorkflowRunner()
	runner.runErr = errors.New("execution failed")
	bridge := tui.NewBridge(newMockWorkflowLister("wf-1"), runner, newMockHistoryProvider())
	wf := &workflow.Workflow{
		Name:  "wf-1",
		Steps: map[string]*workflow.Step{"step-1": {Name: "step-1"}},
	}

	msg := bridge.RunWorkflow(context.Background(), wf, nil)()

	errMsg, ok := msg.(tui.ErrMsg)
	require.True(t, ok, "expected ErrMsg when runner fails, got %T", msg)
	assert.ErrorContains(t, errMsg.Err, "execution failed")
}

func TestBridge_RunWorkflow_Success_ExecCtxIsLive(t *testing.T) {
	bridge, _, runner, _ := newTestBridge(t)
	wf := &workflow.Workflow{
		Name:  "test-workflow",
		Steps: map[string]*workflow.Step{"step-1": {Name: "step-1"}},
	}

	msg := bridge.RunWorkflow(context.Background(), wf, nil)()

	started, ok := msg.(tui.ExecutionStartedMsg)
	require.True(t, ok, "expected ExecutionStartedMsg, got %T", msg)
	assert.Equal(t, runner.execCtx, started.ExecCtx)
	assert.NotNil(t, started.Done)
}

// --- ValidateWorkflow ---

func TestBridge_ValidateWorkflow_ReturnsNonNilCmd(t *testing.T) {
	bridge, _, _, _ := newTestBridge(t)

	cmd := bridge.ValidateWorkflow(context.Background(), "test-workflow")

	assert.NotNil(t, cmd, "ValidateWorkflow must return a non-nil tea.Cmd")
}

func TestBridge_ValidateWorkflow_Success_EmitsValidationResultMsg(t *testing.T) {
	bridge, _, _, _ := newTestBridge(t)

	msg := bridge.ValidateWorkflow(context.Background(), "test-workflow")()

	result, ok := msg.(tui.ValidationResultMsg)
	require.True(t, ok, "expected ValidationResultMsg, got %T", msg)
	assert.True(t, result.Success)
	assert.Equal(t, "test-workflow", result.Name)
	assert.Empty(t, result.Error)
}

func TestBridge_ValidateWorkflow_ValidationError_EmitsValidationResultMsg(t *testing.T) {
	lister := newMockWorkflowLister("bad-workflow")
	lister.validErr = errors.New("invalid workflow: missing initial state")
	bridge := tui.NewBridge(lister, newMockWorkflowRunner(), newMockHistoryProvider())

	msg := bridge.ValidateWorkflow(context.Background(), "bad-workflow")()

	result, ok := msg.(tui.ValidationResultMsg)
	require.True(t, ok, "expected ValidationResultMsg on validation failure, got %T", msg)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "invalid workflow")
}

func TestBridge_ValidateWorkflow_RespectsCancellation(t *testing.T) {
	bridge, _, _, _ := newTestBridge(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := bridge.ValidateWorkflow(ctx, "test-workflow")()

	result, ok := msg.(tui.ValidationResultMsg)
	require.True(t, ok, "expected ValidationResultMsg on cancelled context, got %T", msg)
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}
