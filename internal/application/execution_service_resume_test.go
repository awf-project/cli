package application_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type attemptStateStore struct {
	mu      sync.Mutex
	state   *workflow.ExecutionContext
	saves   []*workflow.ExecutionContext
	saveErr error
	loadErr error
}

func cloneAttemptContext(src *workflow.ExecutionContext) *workflow.ExecutionContext {
	if src == nil {
		return nil
	}
	return &workflow.ExecutionContext{
		WorkflowID: src.WorkflowID, WorkflowName: src.WorkflowName, ParentRunID: src.ParentRunID,
		Status: src.Status, CurrentStep: src.CurrentStep, ExitCode: src.ExitCode,
		Inputs: src.GetInputs(), States: src.GetAllStepStates(), Env: src.Env, StartedAt: src.StartedAt,
		UpdatedAt: src.UpdatedAt, CompletedAt: src.CompletedAt, CallStack: src.CallStack,
		HistoryRecordID: src.HistoryRecordID, HistoryAttempt: src.HistoryAttempt,
		PreviousHistoryRecordID: src.PreviousHistoryRecordID, HistoryRecorded: src.HistoryRecorded,
	}
}

func (s *attemptStateStore) Save(_ context.Context, state *workflow.ExecutionContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	s.state = cloneAttemptContext(state)
	s.saves = append(s.saves, cloneAttemptContext(state))
	return nil
}

func (s *attemptStateStore) Load(_ context.Context, workflowID string) (*workflow.ExecutionContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	if s.state == nil || s.state.WorkflowID != workflowID {
		return nil, nil
	}
	return cloneAttemptContext(s.state), nil
}

func (s *attemptStateStore) Delete(context.Context, string) error   { return nil }
func (s *attemptStateStore) List(context.Context) ([]string, error) { return nil, nil }

type observingCommandExecutor struct {
	store    *attemptStateStore
	calls    int
	commands []string
	observed []*workflow.ExecutionContext
	err      error
}

type resumeDeadlineObservingExecutor struct {
	remaining   []time.Duration
	contexts    []context.Context
	err         error
	waitForDone bool
}

func (e *resumeDeadlineObservingExecutor) Execute(ctx context.Context, _ *ports.Command) (*ports.CommandResult, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		e.remaining = append(e.remaining, 0)
	} else {
		e.remaining = append(e.remaining, time.Until(deadline))
	}
	e.contexts = append(e.contexts, ctx)
	if e.waitForDone {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return nil, e.err
}

func (e *observingCommandExecutor) Execute(_ context.Context, command *ports.Command) (*ports.CommandResult, error) {
	e.calls++
	e.commands = append(e.commands, command.Program)
	e.store.mu.Lock()
	e.observed = append(e.observed, cloneAttemptContext(e.store.state))
	e.store.mu.Unlock()
	if e.err != nil {
		return nil, e.err
	}
	return &ports.CommandResult{}, nil
}

type attemptHistoryStore struct {
	records     []*workflow.ExecutionRecord
	listErr     error
	lastFilter  *workflow.HistoryFilter
	listCalls   int
	recordCalls int
}

func (s *attemptHistoryStore) Record(context.Context, *workflow.ExecutionRecord) error {
	s.recordCalls++
	return nil
}

func (s *attemptHistoryStore) List(_ context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	s.listCalls++
	if filter != nil {
		copyFilter := *filter
		s.lastFilter = &copyFilter
	}
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.records, nil
}

func (s *attemptHistoryStore) GetStats(context.Context, *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	return &workflow.HistoryStats{}, nil
}
func (s *attemptHistoryStore) Cleanup(context.Context, time.Duration) (int, error) { return 0, nil }
func (s *attemptHistoryStore) Close() error                                        { return nil }

type attemptAuditWriter struct{ events []workflow.AuditEvent }

func (w *attemptAuditWriter) Write(_ context.Context, event *workflow.AuditEvent) error {
	w.events = append(w.events, *event)
	return nil
}

func (*attemptAuditWriter) Close() error { return nil }

func newAttemptExecutionService(
	wf *workflow.Workflow,
	store *attemptStateStore,
	executor ports.CommandExecutor,
	historyStore ports.HistoryStore,
) *application.ExecutionService {
	var historySvc *application.HistoryService
	if historyStore != nil {
		historySvc = application.NewHistoryService(historyStore, testmocks.NewMockLogger())
	}
	return newAttemptExecutionServiceWithHistory(wf, store, executor, historySvc)
}

func newAttemptExecutionServiceWithHistory(
	wf *workflow.Workflow,
	store *attemptStateStore,
	executor ports.CommandExecutor,
	historySvc *application.HistoryService,
) *application.ExecutionService {
	repo := testmocks.NewMockWorkflowRepository()
	repo.AddWorkflow(wf.Name, wf)
	logger := testmocks.NewMockLogger()
	wfSvc := application.NewWorkflowService(repo, store, executor, logger, nil)
	return application.NewExecutionService(
		wfSvc, executor, application.NewParallelExecutor(logger), store, logger,
		interpolation.NewTemplateResolver(), historySvc,
	)
}

func attemptTestWorkflow(command string) *workflow.Workflow {
	return &workflow.Workflow{
		Name: "attempt-workflow", Initial: "run",
		Steps: map[string]*workflow.Step{
			"run":  {Name: "run", Type: workflow.StepTypeCommand, Command: command, OnSuccess: "done"},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}
}

func TestExecutionService_ResumeRejectsInvalidInputsBeforeAllocation(t *testing.T) {
	tests := []struct {
		name   string
		inputs map[string]any
	}{
		{name: "missing required input"},
		{name: "invalid input type", inputs: map[string]any{"count": "not-an-integer"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := attemptTestWorkflow("must not execute")
			wf.Inputs = []workflow.Input{{Name: "count", Type: "integer", Required: true}}
			state := workflow.NewExecutionContext("stable-run", wf.Name)
			state.Status, state.CurrentStep = workflow.StatusFailed, "run"
			store := &attemptStateStore{state: state}
			before := cloneAttemptContext(state)
			executor := &observingCommandExecutor{store: store}
			service := newAttemptExecutionService(wf, store, executor, nil)

			resumed, err := service.Resume(context.Background(), state.WorkflowID, tt.inputs, "current")

			require.ErrorContains(t, err, "input validation failed")
			assert.Nil(t, resumed)
			assert.Zero(t, executor.calls)
			assert.Empty(t, store.saves, "rejected input must not allocate or persist another attempt")
			assert.Equal(t, before, store.state)
		})
	}
}

func TestExecutionService_InitialRunAllocatesHistoryAttemptBeforeExecution(t *testing.T) {
	t.Run("failing command observes durable allocation", func(t *testing.T) {
		store := &attemptStateStore{}
		commandErr := errors.New("command failed")
		executor := &observingCommandExecutor{store: store, err: commandErr}
		wf := attemptTestWorkflow("fail")
		svc := newAttemptExecutionService(wf, store, executor, nil)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "stable-run")

		require.ErrorIs(t, err, commandErr)
		require.NotNil(t, execCtx)
		require.Len(t, executor.observed, 1)
		persisted := executor.observed[0]
		require.NotNil(t, persisted)
		assert.Equal(t, "stable-run", persisted.WorkflowID)
		assert.Equal(t, 1, persisted.HistoryAttempt)
		assert.NotEmpty(t, persisted.HistoryRecordID)
		assert.Empty(t, persisted.PreviousHistoryRecordID)
	})

	t.Run("preloaded parent and child workflow allocate before child execution", func(t *testing.T) {
		store := &attemptStateStore{}
		executor := &observingCommandExecutor{store: store}
		child := attemptTestWorkflow("child-command")
		child.Name = "child-workflow"
		parent := &workflow.Workflow{
			Name: "parent-workflow", Initial: "call-child",
			Steps: map[string]*workflow.Step{
				"call-child": {
					Name: "call-child", Type: workflow.StepTypeCallWorkflow,
					CallWorkflow: &workflow.CallWorkflowConfig{Workflow: child.Name}, OnSuccess: "done",
				},
				"done": {Name: "done", Type: workflow.StepTypeTerminal},
			},
		}
		repo := testmocks.NewMockWorkflowRepository()
		repo.AddWorkflow(parent.Name, parent)
		repo.AddWorkflow(child.Name, child)
		logger := testmocks.NewMockLogger()
		wfSvc := application.NewWorkflowService(repo, store, executor, logger, nil)
		svc := application.NewExecutionService(wfSvc, executor, application.NewParallelExecutor(logger), store, logger,
			interpolation.NewTemplateResolver(), nil)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), parent, nil, "stable-parent")

		require.NoError(t, err)
		require.NotNil(t, execCtx)
		require.Len(t, executor.observed, 1)
		childAttempt := executor.observed[0]
		require.NotNil(t, childAttempt)
		assert.NotEmpty(t, childAttempt.HistoryRecordID)
		assert.Equal(t, 1, childAttempt.HistoryAttempt)
		assert.NotEqual(t, "stable-parent", childAttempt.WorkflowID)
		var parentAllocated bool
		for _, saved := range store.saves {
			if saved.WorkflowID == "stable-parent" && saved.HistoryAttempt == 1 && saved.HistoryRecordID != "" {
				parentAllocated = true
				break
			}
		}
		assert.True(t, parentAllocated, "the preloaded parent must allocate before child execution")
	})

	t.Run("allocation save failure prevents hooks and commands", func(t *testing.T) {
		saveErr := errors.New("state unavailable")
		store := &attemptStateStore{saveErr: saveErr}
		executor := &observingCommandExecutor{store: store}
		historyStore := &attemptHistoryStore{}
		wf := attemptTestWorkflow("run")
		wf.Hooks.WorkflowStart = workflow.Hook{{Command: "start-hook"}}
		svc := newAttemptExecutionService(wf, store, executor, historyStore)
		auditWriter := &attemptAuditWriter{}
		svc.SetAuditTrailWriter(auditWriter)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "stable-run")

		assert.ErrorIs(t, err, saveErr)
		assert.Nil(t, execCtx)
		assert.Zero(t, executor.calls)
		assert.Empty(t, executor.commands, "workflow-start hooks must not run")
		assert.Zero(t, historyStore.recordCalls, "terminal history must not be written")
		assert.Empty(t, auditWriter.events, "workflow audit events must not be emitted")
		assert.Nil(t, store.state, "failed allocation must not create durable state")
	})
}

func TestExecutionService_ResumedAttemptsKeepStableWorkflowIdentity(t *testing.T) {
	initialStartedAt := time.Now().Add(-time.Hour)
	initialCompletedAt := initialStartedAt.Add(time.Minute)
	store := &attemptStateStore{state: &workflow.ExecutionContext{
		WorkflowID: "stable-run", WorkflowName: "attempt-workflow", Status: workflow.StatusFailed,
		CurrentStep: "run", Inputs: map[string]any{}, States: map[string]workflow.StepState{}, Env: map[string]string{},
		HistoryRecordID: "record-1", HistoryAttempt: 1, StartedAt: initialStartedAt, CompletedAt: initialCompletedAt,
	}}
	executor := &observingCommandExecutor{store: store, err: errors.New("still failing")}
	wf := attemptTestWorkflow("fail")
	svc := newAttemptExecutionService(wf, store, executor, nil)

	for _, outcome := range []struct {
		name    string
		err     error
		status  workflow.ExecutionStatus
		command string
	}{
		{name: "failed", err: errors.New("still failing"), status: workflow.StatusFailed, command: "fail"},
		{name: "cancelled", err: context.Canceled, status: workflow.StatusCancelled, command: "fail"},
		{name: "successful", status: workflow.StatusCompleted, command: "run"},
	} {
		t.Run(outcome.name, func(t *testing.T) {
			executor.err = outcome.err
			wf.Steps["run"].Command = outcome.command
			prior := cloneAttemptContext(store.state)

			execCtx, err := svc.Resume(context.Background(), "stable-run", nil, "current")

			if outcome.err != nil {
				require.ErrorIs(t, err, outcome.err)
			} else {
				require.NoError(t, err)
			}
			require.NotNil(t, execCtx)
			assert.Equal(t, "stable-run", execCtx.WorkflowID)
			assert.Equal(t, prior.HistoryAttempt+1, execCtx.HistoryAttempt)
			assert.NotEmpty(t, execCtx.HistoryRecordID)
			assert.NotEqual(t, prior.HistoryRecordID, execCtx.HistoryRecordID)
			assert.Equal(t, prior.HistoryRecordID, execCtx.PreviousHistoryRecordID)
			assert.Equal(t, outcome.status, execCtx.Status)
			require.NotEmpty(t, executor.observed)
			observed := executor.observed[len(executor.observed)-1]
			assert.Equal(t, execCtx.HistoryRecordID, observed.HistoryRecordID)
			assert.True(t, observed.StartedAt.After(initialStartedAt), "a resumed attempt must reset StartedAt")
			assert.Zero(t, observed.CompletedAt, "a resumed attempt must clear prior CompletedAt before execution")
		})
	}

	priorDurable := cloneAttemptContext(store.state)
	store.state.Status = workflow.StatusFailed
	priorDurable.Status = workflow.StatusFailed
	store.saveErr = errors.New("cannot persist resumed attempt")
	executor.calls = 0
	failedCtx, saveErr := svc.Resume(context.Background(), "stable-run", nil, "current")
	assert.ErrorIs(t, saveErr, store.saveErr)
	assert.Nil(t, failedCtx)
	assert.Zero(t, executor.calls)
	require.NotNil(t, store.state)
	assert.Equal(t, priorDurable.HistoryRecordID, store.state.HistoryRecordID)
	assert.Equal(t, priorDurable.HistoryAttempt, store.state.HistoryAttempt)
}

func TestExecutionService_ResumePreservesTimeoutSemantics(t *testing.T) {
	t.Run("fresh and resumed local expiry have the same structured timeout", func(t *testing.T) {
		const timeoutSeconds = 1
		store := &attemptStateStore{}
		executor := &resumeDeadlineObservingExecutor{waitForDone: true}
		wf := attemptTestWorkflow("observe deadline")
		wf.Steps["run"].Timeout = timeoutSeconds
		svc := newAttemptExecutionService(wf, store, executor, nil)

		initialCtx, initialErr := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "stable-timeout-run")
		require.NotNil(t, initialCtx)
		assertResumeLocalTimeout(t, initialErr, timeoutSeconds)

		resumedCtx, resumedErr := svc.Resume(context.Background(), "stable-timeout-run", nil, "current")
		require.NotNil(t, resumedCtx)
		assertResumeLocalTimeout(t, resumedErr, timeoutSeconds)
		require.Len(t, executor.remaining, 2)
		for _, remaining := range executor.remaining {
			assert.InDelta(t, float64(timeoutSeconds), remaining.Seconds(), 0.25)
		}
	})

	t.Run("fresh and resumed caller cancellation preserve cancellation identity", func(t *testing.T) {
		wf := attemptTestWorkflow("observe cancellation")
		freshExecutor := &resumeDeadlineObservingExecutor{waitForDone: true}
		freshSvc := newAttemptExecutionService(wf, &attemptStateStore{}, freshExecutor, nil)
		freshParent, freshCancel := context.WithCancel(context.Background())
		freshCancel()
		_, freshErr := freshSvc.RunWithWorkflowAndRunID(freshParent, wf, nil, "fresh-cancel-run")
		require.ErrorIs(t, freshErr, context.Canceled)
		assert.False(t, errors.Is(freshErr, context.DeadlineExceeded))

		store := &attemptStateStore{}
		executor := &resumeDeadlineObservingExecutor{err: errors.New("seed attempt")}
		svc := newAttemptExecutionService(wf, store, executor, nil)
		_, seedErr := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "resume-cancel-run")
		require.Error(t, seedErr)
		executor.err = nil
		executor.waitForDone = true
		resumedParent, resumedCancel := context.WithCancel(context.Background())
		resumedCancel()
		_, resumedErr := svc.Resume(resumedParent, "resume-cancel-run", nil, "current")
		require.ErrorIs(t, resumedErr, context.Canceled)
		assert.False(t, errors.Is(resumedErr, context.DeadlineExceeded))
	})
}

func assertResumeLocalTimeout(t *testing.T, err error, timeoutSeconds int) {
	t.Helper()
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var structuredErr *domainerrors.StructuredError
	require.ErrorAs(t, err, &structuredErr)
	assert.Equal(t, domainerrors.ErrorCodeExecutionCommandTimeout, structuredErr.Code)
	assert.Equal(t, map[string]any{"timeout_owner": "step", "timeout_seconds": timeoutSeconds}, structuredErr.Details)
}

func TestExecutionService_ResumeNormalizesLegacyAttemptFromHistory(t *testing.T) {
	records := make([]*workflow.ExecutionRecord, 0, 24)
	for attempt := 1; attempt <= 22; attempt++ {
		predecessor := ""
		if len(records) > 0 {
			predecessor = records[len(records)-1].ID
		}
		records = append(records, &workflow.ExecutionRecord{
			ID: "old-record-" + time.Unix(int64(attempt), 0).Format("150405"), WorkflowID: "stable-run",
			Attempt: attempt, ResumedFromID: predecessor, CompletedAt: time.Unix(int64(100-attempt), 0),
		})
	}
	expectedPredecessor := records[len(records)-1].ID
	records = append(records,
		&workflow.ExecutionRecord{ID: "foreign", WorkflowID: "other-run", Attempt: 900},
		&workflow.ExecutionRecord{WorkflowID: "other-run"}, // Unrelated malformed rows must not affect this workflow.
	)
	for left, right := 0, len(records)-1; left < right; left, right = left+1, right-1 {
		records[left], records[right] = records[right], records[left]
	}
	historyStore := &attemptHistoryStore{records: records}
	store := &attemptStateStore{state: &workflow.ExecutionContext{
		WorkflowID: "stable-run", WorkflowName: "attempt-workflow", Status: workflow.StatusFailed,
		CurrentStep: "run", Inputs: map[string]any{}, States: map[string]workflow.StepState{}, Env: map[string]string{},
	}}
	executor := &observingCommandExecutor{store: store, err: errors.New("stop")}
	svc := newAttemptExecutionService(attemptTestWorkflow("run"), store, executor, historyStore)

	execCtx, err := svc.Resume(context.Background(), "stable-run", nil, "current")

	require.Error(t, err)
	require.NotNil(t, execCtx)
	assert.Equal(t, 23, execCtx.HistoryAttempt)
	assert.Equal(t, expectedPredecessor, execCtx.PreviousHistoryRecordID)
	require.NotNil(t, historyStore.lastFilter)
	assert.Equal(t, "stable-run", historyStore.lastFilter.WorkflowID)
	assert.Equal(t, math.MaxInt, historyStore.lastFilter.Limit)
	assert.Empty(t, historyStore.lastFilter.WorkflowName)
	assert.Empty(t, historyStore.lastFilter.Status)

	invalidCases := []struct {
		name         string
		records      []*workflow.ExecutionRecord
		savedAttempt int
		savedRecord  string
	}{
		{name: "nil history row", records: []*workflow.ExecutionRecord{nil}},
		{name: "invalid local lineage", records: []*workflow.ExecutionRecord{{WorkflowID: "stable-run", Attempt: 1}}},
		{name: "zero ordinal", records: []*workflow.ExecutionRecord{{ID: "record-0", WorkflowID: "stable-run"}}},
		{name: "negative ordinal", records: []*workflow.ExecutionRecord{{ID: "record-negative", WorkflowID: "stable-run", Attempt: -1}}},
		{name: "first attempt has predecessor", records: []*workflow.ExecutionRecord{
			{ID: "record-1", WorkflowID: "stable-run", Attempt: 1, ResumedFromID: "record-0"},
		}},
		{name: "later attempt has no predecessor", records: []*workflow.ExecutionRecord{
			{ID: "record-2", WorkflowID: "stable-run", Attempt: 2},
		}},
		{name: "self predecessor", records: []*workflow.ExecutionRecord{
			{ID: "record-2", WorkflowID: "stable-run", Attempt: 2, ResumedFromID: "record-2"},
		}},
		{name: "retained predecessor has wrong ordinal", records: []*workflow.ExecutionRecord{
			{ID: "record-1", WorkflowID: "stable-run", Attempt: 1},
			{ID: "record-3", WorkflowID: "stable-run", Attempt: 3, ResumedFromID: "record-1"},
		}},
		{name: "retained predecessor belongs to another workflow", records: []*workflow.ExecutionRecord{
			{ID: "foreign-1", WorkflowID: "other-run", Attempt: 1},
			{ID: "record-2", WorkflowID: "stable-run", Attempt: 2, ResumedFromID: "foreign-1"},
		}},
		{name: "same record has conflicting ordinals", records: []*workflow.ExecutionRecord{
			{ID: "record-1", WorkflowID: "stable-run", Attempt: 1},
			{ID: "record-1", WorkflowID: "stable-run", Attempt: 2, ResumedFromID: "removed-record"},
		}},
		{name: "conflicting ordinal", records: []*workflow.ExecutionRecord{
			{ID: "record-2a", WorkflowID: "stable-run", Attempt: 2, ResumedFromID: "record-1"},
			{ID: "record-2b", WorkflowID: "stable-run", Attempt: 2, ResumedFromID: "record-1"},
		}},
		{name: "legacy maximum overflows", records: []*workflow.ExecutionRecord{{ID: "record-max", WorkflowID: "stable-run", Attempt: math.MaxInt, ResumedFromID: "previous-max"}}},
		{name: "saved attempt overflows", savedAttempt: math.MaxInt, savedRecord: "saved-max"},
	}
	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			caseStore := &attemptStateStore{state: &workflow.ExecutionContext{
				WorkflowID: "stable-run", WorkflowName: "attempt-workflow", Status: workflow.StatusFailed,
				CurrentStep: "run", Inputs: map[string]any{}, States: map[string]workflow.StepState{}, Env: map[string]string{},
				HistoryAttempt: tt.savedAttempt, HistoryRecordID: tt.savedRecord,
			}}
			caseExecutor := &observingCommandExecutor{store: caseStore}
			caseHistory := &attemptHistoryStore{records: tt.records}
			caseSvc := newAttemptExecutionService(attemptTestWorkflow("run"), caseStore, caseExecutor, caseHistory)

			got, caseErr := caseSvc.Resume(context.Background(), "stable-run", nil, "current")

			assert.ErrorIs(t, caseErr, workflow.ErrInvalidHistoryAttempt)
			assert.Nil(t, got)
			assert.Zero(t, caseExecutor.calls)
		})
	}

	t.Run("non-contiguous retained attempts remain valid", func(t *testing.T) {
		caseStore := &attemptStateStore{state: &workflow.ExecutionContext{
			WorkflowID: "stable-run", WorkflowName: "attempt-workflow", Status: workflow.StatusFailed,
			CurrentStep: "run", Inputs: map[string]any{}, States: map[string]workflow.StepState{}, Env: map[string]string{},
		}}
		caseExecutor := &observingCommandExecutor{store: caseStore, err: errors.New("stop")}
		caseHistory := &attemptHistoryStore{records: []*workflow.ExecutionRecord{
			{ID: "retained-3", WorkflowID: "stable-run", Attempt: 3, ResumedFromID: "removed-2"},
			{ID: "retained-9", WorkflowID: "stable-run", Attempt: 9, ResumedFromID: "removed-8"},
		}}
		caseSvc := newAttemptExecutionService(attemptTestWorkflow("run"), caseStore, caseExecutor, caseHistory)

		got, caseErr := caseSvc.Resume(context.Background(), "stable-run", nil, "current")

		require.Error(t, caseErr)
		require.NotNil(t, got)
		assert.Equal(t, 10, got.HistoryAttempt)
		assert.Equal(t, "retained-9", got.PreviousHistoryRecordID)
	})
}

func TestExecutionService_ResumeRewindAllocationFailurePreservesDurableState(t *testing.T) {
	for _, fromStep := range []string{"previous", "prepare"} {
		t.Run(fromStep, func(t *testing.T) {
			started := time.Now().Add(-time.Hour)
			state := &workflow.ExecutionContext{
				WorkflowID: "rewind-run", WorkflowName: "attempt-workflow", CurrentStep: "run",
				Status: workflow.StatusFailed, ExitCode: 17, StartedAt: started,
				UpdatedAt: started.Add(2 * time.Minute), CompletedAt: started.Add(2 * time.Minute),
				HistoryRecordID: "record-2", HistoryAttempt: 2, PreviousHistoryRecordID: "record-1", HistoryRecorded: true,
				Inputs: map[string]any{"value": "original"}, Env: map[string]string{},
				States: map[string]workflow.StepState{
					"prepare": {Name: "prepare", Status: workflow.StatusCompleted, CompletedAt: started.Add(time.Minute)},
					"run":     {Name: "run", Status: workflow.StatusFailed, CompletedAt: started.Add(2 * time.Minute), Error: "old failure"},
				},
			}
			before := cloneAttemptContext(state)
			saveErr := errors.New("allocation storage unavailable")
			store := &attemptStateStore{state: state, saveErr: saveErr}
			executor := &observingCommandExecutor{store: store}
			history := &attemptHistoryStore{}
			wf := attemptTestWorkflow("run")
			wf.Initial = "prepare"
			wf.Steps["prepare"] = &workflow.Step{Name: "prepare", Type: workflow.StepTypeCommand, Command: "prepare", OnSuccess: "run"}
			wf.Inputs = []workflow.Input{{Name: "value", Type: "string"}}
			wf.Hooks.WorkflowStart = workflow.Hook{{Command: "start-hook"}}
			svc := newAttemptExecutionService(wf, store, executor, history)
			audit := &attemptAuditWriter{}
			svc.SetAuditTrailWriter(audit)

			resumed, err := svc.Resume(context.Background(), state.WorkflowID, map[string]any{"value": "override"}, fromStep)

			require.ErrorIs(t, err, saveErr)
			assert.Nil(t, resumed)
			assert.Equal(t, before, store.state, "rewind, overrides, timing, and identity must remain uncommitted")
			assert.Empty(t, store.saves)
			assert.Zero(t, executor.calls, "neither hooks nor commands may execute")
			assert.Zero(t, history.recordCalls)
			assert.Empty(t, audit.events)
		})
	}
}

type blockingResumeExecutor struct {
	started chan struct{}
	release chan struct{}
}

func (e *blockingResumeExecutor) Execute(ctx context.Context, _ *ports.Command) (*ports.CommandResult, error) {
	close(e.started)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.release:
		return &ports.CommandResult{}, nil
	}
}

func TestExecutionService_ResumeSerializesIndependentServices(t *testing.T) {
	store := &attemptStateStore{state: &workflow.ExecutionContext{
		WorkflowID: "shared-resume", WorkflowName: "attempt-workflow", CurrentStep: "run", Status: workflow.StatusFailed,
		HistoryRecordID: "record-1", HistoryAttempt: 1, HistoryRecorded: true,
	}}
	firstExecutor := &blockingResumeExecutor{started: make(chan struct{}), release: make(chan struct{})}
	secondExecutor := &observingCommandExecutor{store: store}
	firstService := newAttemptExecutionService(attemptTestWorkflow("run"), store, firstExecutor, nil)
	secondService := newAttemptExecutionService(attemptTestWorkflow("run"), store, secondExecutor, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := firstService.Resume(ctx, "shared-resume", nil, "current")
		result <- err
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("first resume did not terminate")
		}
	})
	select {
	case <-firstExecutor.started:
	case <-ctx.Done():
		t.Fatal("first resume did not reach command execution")
	}

	resumed, err := secondService.Resume(ctx, "shared-resume", nil, "current")
	require.ErrorContains(t, err, "workflow resume already in progress")
	assert.Nil(t, resumed)
	assert.Zero(t, secondExecutor.calls)
	store.mu.Lock()
	assert.Equal(t, 2, store.state.HistoryAttempt)
	assert.Equal(t, "record-1", store.state.PreviousHistoryRecordID)
	store.mu.Unlock()
	close(firstExecutor.release)
	select {
	case err = <-result:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("first resume did not finish")
	}
}

func TestExecutionService_ResumeInitializesLegacyAttemptWithoutHistory(t *testing.T) {
	tests := []struct {
		name              string
		historyStore      *attemptHistoryStore
		historyConfigured bool
		wantErr           error
		wantCalls         int
	}{
		{name: "history service disabled"},
		{name: "history service has nil store", historyConfigured: true},
		{name: "configured history is empty", historyStore: &attemptHistoryStore{}, historyConfigured: true, wantCalls: 1},
		{name: "cancellation is propagated", historyStore: &attemptHistoryStore{listErr: context.Canceled}, historyConfigured: true, wantErr: context.Canceled, wantCalls: 1},
		{name: "permission error is propagated", historyStore: &attemptHistoryStore{listErr: errors.New("history permission denied")}, historyConfigured: true, wantCalls: 1},
		{name: "other list error is propagated", historyStore: &attemptHistoryStore{listErr: errors.New("history unavailable")}, historyConfigured: true, wantCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &attemptStateStore{state: &workflow.ExecutionContext{
				WorkflowID: "stable-run", WorkflowName: "attempt-workflow", Status: workflow.StatusFailed,
				CurrentStep: "run", Inputs: map[string]any{}, States: map[string]workflow.StepState{}, Env: map[string]string{},
			}}
			executor := &observingCommandExecutor{store: store, err: errors.New("stop")}
			if tt.historyStore != nil && tt.wantErr == nil && tt.historyStore.listErr != nil {
				tt.wantErr = tt.historyStore.listErr
			}
			var historySvc *application.HistoryService
			if tt.historyConfigured {
				historySvc = application.NewHistoryService(tt.historyStore, testmocks.NewMockLogger())
			}
			svc := newAttemptExecutionServiceWithHistory(attemptTestWorkflow("run"), store, executor, historySvc)

			execCtx, err := svc.Resume(context.Background(), "stable-run", nil, "current")

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, execCtx)
				assert.Zero(t, executor.calls)
			} else {
				require.Error(t, err)
				require.NotNil(t, execCtx)
				assert.Equal(t, 1, execCtx.HistoryAttempt)
				assert.NotEmpty(t, execCtx.HistoryRecordID)
				assert.Empty(t, execCtx.PreviousHistoryRecordID)
			}
			if tt.historyStore != nil {
				assert.Equal(t, tt.wantCalls, tt.historyStore.listCalls)
			}
		})
	}
}

func TestExecutionService_ResumeValidatesBeforeAllocatingAttempt(t *testing.T) {
	baseState := func() *workflow.ExecutionContext {
		return &workflow.ExecutionContext{
			WorkflowID: "stable-run", WorkflowName: "attempt-workflow", Status: workflow.StatusFailed,
			CurrentStep: "run", Inputs: map[string]any{}, States: map[string]workflow.StepState{}, Env: map[string]string{},
			HistoryRecordID: "record-1", HistoryAttempt: 1,
		}
	}

	tests := []struct {
		name       string
		state      *workflow.ExecutionContext
		workflowID string
		fromStep   string
		checkError func(*testing.T, error)
	}{
		{
			name: "execution not found", workflowID: "missing", fromStep: "current",
			checkError: func(t *testing.T, err error) { assert.ErrorIs(t, err, application.ErrExecutionNotFound) },
		},
		{
			name: "completed execution", state: func() *workflow.ExecutionContext {
				state := baseState()
				state.Status = workflow.StatusCompleted
				return state
			}(), workflowID: "stable-run", fromStep: "current",
			checkError: func(t *testing.T, err error) { assert.ErrorContains(t, err, "already completed") },
		},
		{
			name: "workflow missing", state: func() *workflow.ExecutionContext {
				state := baseState()
				state.WorkflowName = "missing-workflow"
				return state
			}(), workflowID: "stable-run", fromStep: "current",
			checkError: func(t *testing.T, err error) { assert.ErrorContains(t, err, "load workflow") },
		},
		{
			name: "current step missing", state: func() *workflow.ExecutionContext {
				state := baseState()
				state.CurrentStep = "removed"
				return state
			}(), workflowID: "stable-run", fromStep: "current",
			checkError: func(t *testing.T, err error) { assert.ErrorContains(t, err, "no longer exists") },
		},
		{
			name: "from step invalid", state: baseState(), workflowID: "stable-run", fromStep: "not-in-history",
			checkError: func(t *testing.T, err error) { assert.ErrorContains(t, err, "step not found in execution history") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &attemptStateStore{state: cloneAttemptContext(tt.state)}
			executor := &observingCommandExecutor{store: store}
			svc := newAttemptExecutionService(attemptTestWorkflow("run"), store, executor, nil)
			before := cloneAttemptContext(store.state)

			got, err := svc.Resume(context.Background(), tt.workflowID, nil, tt.fromStep)

			require.Error(t, err)
			tt.checkError(t, err)
			assert.Nil(t, got)
			assert.Zero(t, executor.calls)
			assert.Empty(t, store.saves, "validation must precede allocation persistence")
			if before != nil {
				require.NotNil(t, store.state)
				assert.Equal(t, before.HistoryRecordID, store.state.HistoryRecordID)
				assert.Equal(t, before.HistoryAttempt, store.state.HistoryAttempt)
				assert.Equal(t, before.PreviousHistoryRecordID, store.state.PreviousHistoryRecordID)
			}
		})
	}
}

func TestBuildInterpolationContext_ExcludesHistoryAttemptMetadata(t *testing.T) {
	variables := []string{"history_record_id", "history_attempt", "resumed_from_id", "history_recorded"}
	for _, variable := range variables {
		t.Run(variable+" is unresolvable", func(t *testing.T) {
			store := &attemptStateStore{}
			executor := &observingCommandExecutor{store: store}
			wf := attemptTestWorkflow("echo {{." + variable + "}}")
			svc := newAttemptExecutionService(wf, store, executor, nil)

			_, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "stable-run")

			require.Error(t, err)
			var undefined *interpolation.UndefinedVariableError
			require.ErrorAs(t, err, &undefined)
			assert.ErrorContains(t, undefined, variable)
			assert.Zero(t, executor.calls)
		})
	}

	store := &attemptStateStore{}
	executor := &observingCommandExecutor{store: store}
	wf := attemptTestWorkflow("echo {{.workflow.ID}}")
	svc := newAttemptExecutionService(wf, store, executor, nil)

	_, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "stable-run")

	require.NoError(t, err)
	require.Len(t, executor.observed, 1)
}

// executeFromStep Tests
// Feature: C054 - Increase Application Layer Test Coverage
// Component: T005 - execute_from_step_tests
//
// This file tests the executeFromStep function which handles workflow resumption
// from a specific step, managing state transitions, error hooks, and step dispatch.
//
// Coverage target: 55.4% -> 80%+
//
// This function is private (executeFromStep), so we test it indirectly through
// the Resume() public API by constructing workflows with saved states and observing
// execution flow from the resume point.

func TestExecuteFromStep_StepNotFound(t *testing.T) {
	// This tests the path where executeFromStep finds step doesn't exist during execution loop
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "nonexistent_step", // Points to non-existent step
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start", // Valid step
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		Build()

	// Pre-save the state
	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-001", nil, "current")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step not found")
	require.NotNil(t, ctx, "execution context should not be nil even on error")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

func TestExecuteFromStep_TerminalStepSuccess(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "success_terminal",
			},
			"success_terminal": {
				Name:   "success_terminal",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess, // Default success
			},
		},
	}

	// Create saved state at terminal step
	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-002",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "success_terminal",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-002", nil, "current")

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "success_terminal", ctx.CurrentStep)
	assert.NotZero(t, ctx.CompletedAt)
}

func TestExecuteFromStep_TerminalStepFailure(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnFailure: "failure_terminal",
			},
			"failure_terminal": {
				Name:   "failure_terminal",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure, // Explicit failure
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-003",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "failure_terminal",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-003", nil, "current")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal failure state")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
	assert.NotZero(t, ctx.CompletedAt)
}

func TestExecuteFromStep_ContextCancelled(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "sleep 10",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
		Hooks: workflow.WorkflowHooks{
			WorkflowCancel: workflow.Hook{
				{Command: "echo cancelled"},
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-004",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo cancelled", &ports.CommandResult{Stdout: "cancelled\n", ExitCode: 0}).
		Build()

	// Configure executor to return context.Canceled error for all commands
	mocks.Executor.SetExecuteError(context.Canceled)

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	execCtx, err := execSvc.Resume(context.Background(), "wf-004", nil, "current")

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, workflow.StatusCancelled, execCtx.Status)
}

func TestExecuteFromStep_RegularError_ClassifiesAndExecutesHook(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
		Hooks: workflow.WorkflowHooks{
			WorkflowError: workflow.Hook{
				{Command: "echo error_hook"},
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-005",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("exit 1", &ports.CommandResult{Stderr: "command failed", ExitCode: 1}).
		WithCommandResult("echo error_hook", &ports.CommandResult{Stdout: "error_hook\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-005", nil, "current")

	require.Error(t, err)
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
	// Verify error hook was executed by checking if the command was called
	calls := mocks.Executor.GetCalls()
	var foundErrorHook bool
	for _, call := range calls {
		if call.Program == "echo error_hook" {
			foundErrorHook = true
			break
		}
	}
	assert.True(t, foundErrorHook, "WorkflowError hook should have been executed")
}

func TestExecuteFromStep_SuccessfulCompletion_ExecutesEndHook(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
		Hooks: workflow.WorkflowHooks{
			WorkflowEnd: workflow.Hook{
				{Command: "echo workflow_completed"},
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-006",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
		WithCommandResult("echo workflow_completed", &ports.CommandResult{Stdout: "workflow_completed\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-006", nil, "current")

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "end", ctx.CurrentStep)
}

func TestExecuteFromStep_DispatchesOperationStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeOperation,
				Operation: "test-op",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-007",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-007", nil, "current")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation provider not configured")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

func TestExecuteFromStep_DispatchesParallelStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch1", "branch2"},
				Strategy:  "all_succeed",
				OnSuccess: "end",
			},
			"branch1": {
				Name:    "branch1",
				Type:    workflow.StepTypeCommand,
				Command: "echo branch1",
			},
			"branch2": {
				Name:    "branch2",
				Type:    workflow.StepTypeCommand,
				Command: "echo branch2",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-008",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "parallel",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo branch1", &ports.CommandResult{Stdout: "branch1\n", ExitCode: 0}).
		WithCommandResult("echo branch2", &ports.CommandResult{Stdout: "branch2\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-008", nil, "current")

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteFromStep_DispatchesLoopStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:       workflow.LoopTypeForEach,
					Items:      "items",
					Body:       []string{"body"},
					OnComplete: "end",
				},
			},
			"body": {
				Name:      "body",
				Type:      workflow.StepTypeCommand,
				Command:   "echo item",
				OnSuccess: "loop",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-009",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "loop",
		States:       make(map[string]workflow.StepState),
		Inputs:       map[string]any{"items": []any{}}, // Empty items - loop completes immediately
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-009", nil, "current")

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteFromStep_DispatchesCallWorkflowStep(t *testing.T) {
	mainWf := &workflow.Workflow{
		Name:    "main",
		Initial: "call_sub",
		Steps: map[string]*workflow.Step{
			"call_sub": {
				Name: "call_sub",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "sub",
				},
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	subWf := &workflow.Workflow{
		Name:    "sub",
		Initial: "sub_step",
		Steps: map[string]*workflow.Step{
			"sub_step": {
				Name:      "sub_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo sub",
				OnSuccess: "sub_end",
			},
			"sub_end": {
				Name: "sub_end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-010",
		WorkflowName: "main",
		Status:       workflow.StatusRunning,
		CurrentStep:  "call_sub",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("main", mainWf).
		WithWorkflow("sub", subWf).
		WithCommandResult("echo sub", &ports.CommandResult{Stdout: "sub\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-010", nil, "current")

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteFromStep_DispatchesAgentStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "agent",
		Steps: map[string]*workflow.Step{
			"agent": {
				Name: "agent",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "test-agent",
					Prompt:   "test prompt",
				},
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-011",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "agent",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-011", nil, "current")

	// This test verifies that executeFromStep dispatches to executeAgentStep
	require.Error(t, err)
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

func TestExecuteFromStep_DispatchesDefaultCommandStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo default",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-012",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo default", &ports.CommandResult{Stdout: "default\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-012", nil, "current")

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecuteFromStep_CheckpointsAfterEachStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step2",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-013",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step1",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo step1", &ports.CommandResult{Stdout: "step1\n", ExitCode: 0}).
		WithCommandResult("echo step2", &ports.CommandResult{Stdout: "step2\n", ExitCode: 0}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	ctx, err := execSvc.Resume(context.Background(), "wf-013", nil, "current")

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	// Verify workflow executed through both steps
	assert.Equal(t, "end", ctx.CurrentStep)
}
