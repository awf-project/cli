package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type finalizationTestTrace struct {
	mu                  sync.Mutex
	calls               []string
	terminalCheckpoints []*workflow.ExecutionContext
	historyRecords      []*workflow.ExecutionRecord
	auditEvents         []workflow.AuditEvent
	domainEvents        []*pluginmodel.DomainEvent
	hookCalls           map[string]int
	commandCalls        int
	checkpointErr       error
	checkpointErrors    []error
	historyErr          error
	historyErrors       []error
	auditErr            error
	stepFailureErr      error
	stepFailureSuccess  bool
	historyStarted      chan struct{}
	historyRelease      chan struct{}
	allContextsActive   bool
}

func newFinalizationTestTrace() *finalizationTestTrace {
	return &finalizationTestTrace{hookCalls: make(map[string]int), allContextsActive: true}
}

func (t *finalizationTestTrace) recordContext(ctx context.Context) {
	if ctx.Err() != nil {
		t.allContextsActive = false
	}
}

func (t *finalizationTestTrace) snapshot() (calls []string, records []*workflow.ExecutionRecord, audits []workflow.AuditEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.calls...), append([]*workflow.ExecutionRecord(nil), t.historyRecords...),
		append([]workflow.AuditEvent(nil), t.auditEvents...)
}

type finalizationTestStore struct {
	mu     sync.Mutex
	trace  *finalizationTestTrace
	states map[string]*workflow.ExecutionContext
}

func (s *finalizationTestStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state.WorkflowID] = state
	if !state.CompletedAt.IsZero() && (state.Status == workflow.StatusCompleted ||
		state.Status == workflow.StatusFailed || state.Status == workflow.StatusCancelled) {
		s.trace.calls = append(s.trace.calls, "checkpoint")
		s.trace.terminalCheckpoints = append(s.trace.terminalCheckpoints, state)
		s.trace.recordContext(ctx)
		if len(s.trace.checkpointErrors) > 0 {
			err := s.trace.checkpointErrors[0]
			s.trace.checkpointErrors = s.trace.checkpointErrors[1:]
			return err
		}
		return s.trace.checkpointErr
	}
	return nil
}

func (s *finalizationTestStore) Load(_ context.Context, id string) (*workflow.ExecutionContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.states[id]
	if state == nil {
		return nil, nil
	}
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	var loaded workflow.ExecutionContext
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, err
	}
	// JSONStore returns a distinct context, but retaining monotonic clock data in
	// this fake keeps exact-replay assertions focused on lifecycle fields rather
	// than encoding details of time.Time.
	loaded.StartedAt = state.StartedAt
	loaded.UpdatedAt = state.UpdatedAt
	loaded.CompletedAt = state.GetCompletedAt()
	return &loaded, nil
}

func (s *finalizationTestStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, id)
	return nil
}

func (s *finalizationTestStore) List(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.states))
	for id := range s.states {
		ids = append(ids, id)
	}
	return ids, nil
}

type finalizationTestHistoryStore struct{ trace *finalizationTestTrace }

func (s *finalizationTestHistoryStore) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
	s.trace.mu.Lock()
	s.trace.calls = append(s.trace.calls, "history")
	recordCopy := *record
	s.trace.historyRecords = append(s.trace.historyRecords, &recordCopy)
	s.trace.recordContext(ctx)
	err := s.trace.historyErr
	if len(s.trace.historyErrors) > 0 {
		err = s.trace.historyErrors[0]
		s.trace.historyErrors = s.trace.historyErrors[1:]
	}
	started, release := s.trace.historyStarted, s.trace.historyRelease
	s.trace.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if release != nil {
		<-release
	}
	return err
}

func (s *finalizationTestHistoryStore) List(context.Context, *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	s.trace.mu.Lock()
	defer s.trace.mu.Unlock()
	return s.trace.historyRecords, nil
}

func (*finalizationTestHistoryStore) GetStats(context.Context, *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	return &workflow.HistoryStats{}, nil
}

func (*finalizationTestHistoryStore) Cleanup(context.Context, time.Duration) (int, error) {
	return 0, nil
}
func (*finalizationTestHistoryStore) Close() error { return nil }

type finalizationTestAuditWriter struct{ trace *finalizationTestTrace }

func (w *finalizationTestAuditWriter) Write(ctx context.Context, event *workflow.AuditEvent) error {
	w.trace.mu.Lock()
	defer w.trace.mu.Unlock()
	w.trace.auditEvents = append(w.trace.auditEvents, *event)
	if event.Event == workflow.EventWorkflowCompleted {
		w.trace.calls = append(w.trace.calls, "audit")
		w.trace.recordContext(ctx)
		return w.trace.auditErr
	}
	return nil
}

func (*finalizationTestAuditWriter) Close() error { return nil }

type finalizationTestEventPublisher struct{ trace *finalizationTestTrace }

func (p *finalizationTestEventPublisher) Publish(ctx context.Context, event *pluginmodel.DomainEvent) error {
	p.trace.domainEvents = append(p.trace.domainEvents, event)
	if event.Type == workflow.EventWorkflowCompleted || event.Type == workflow.EventWorkflowFailed {
		p.trace.calls = append(p.trace.calls, "event")
		p.trace.recordContext(ctx)
	}
	return nil
}

func (*finalizationTestEventPublisher) Close() error { return nil }

type finalizationTestExecutor struct{ trace *finalizationTestTrace }

func (e *finalizationTestExecutor) Execute(_ context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	e.trace.mu.Lock()
	e.trace.commandCalls++
	e.trace.mu.Unlock()
	switch cmd.Program {
	case "step-failure":
		if e.trace.stepFailureSuccess {
			return &ports.CommandResult{}, nil
		}
		if e.trace.stepFailureErr != nil {
			return &ports.CommandResult{ExitCode: 23}, e.trace.stepFailureErr
		}
		return &ports.CommandResult{ExitCode: 23, Stderr: "synthetic step failure"}, nil
	case "cancel":
		return nil, context.Canceled
	case "deadline":
		return nil, context.DeadlineExceeded
	case "cancel-with-provider-error":
		return nil, errors.New("synthetic provider shutdown error")
	case "panic", "start-panic":
		panic("CONFIDENTIAL_PANIC_CANARY")
	case "start-failure":
		return nil, errors.New("synthetic start-hook failure")
	case "end-hook", "error-hook", "cancel-hook", "end-hook-failure":
		e.trace.calls = append(e.trace.calls, "hook")
		e.trace.hookCalls[cmd.Program]++
		if cmd.Program == "end-hook-failure" {
			return nil, errors.New("synthetic terminal-hook failure")
		}
	}
	return &ports.CommandResult{}, nil
}

type finalizationTestLogEntry struct {
	level   string
	message string
	fields  []any
}

type finalizationTestLogger struct {
	mu      sync.Mutex
	entries []string
	logs    []finalizationTestLogEntry
}

func (*finalizationTestLogger) Debug(string, ...any) {}
func (*finalizationTestLogger) Info(string, ...any)  {}
func (l *finalizationTestLogger) Warn(message string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprint(append([]any{message}, fields...)...))
	l.logs = append(l.logs, finalizationTestLogEntry{
		level: "warn", message: message, fields: append([]any(nil), fields...),
	})
}

func (l *finalizationTestLogger) Error(message string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprint(append([]any{message}, fields...)...))
	l.logs = append(l.logs, finalizationTestLogEntry{
		level: "error", message: message, fields: append([]any(nil), fields...),
	})
}
func (l *finalizationTestLogger) WithContext(map[string]any) ports.Logger { return l }

func (l *finalizationTestLogger) snapshot() ([]string, []finalizationTestLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.entries...), append([]finalizationTestLogEntry(nil), l.logs...)
}

type finalizationTestRig struct {
	service *ExecutionService
	store   *finalizationTestStore
	trace   *finalizationTestTrace
	logger  *finalizationTestLogger
}

func newFinalizationTestRig(t *testing.T, wf *workflow.Workflow, resume bool) *finalizationTestRig {
	t.Helper()

	trace := newFinalizationTestTrace()
	store := &finalizationTestStore{trace: trace, states: make(map[string]*workflow.ExecutionContext)}
	executor := &finalizationTestExecutor{trace: trace}
	logger := &finalizationTestLogger{}
	repository := testmocks.NewMockWorkflowRepository()
	repository.AddWorkflow(wf.Name, wf)
	wfSvc := NewWorkflowService(repository, store, executor, logger, testmocks.NewMockExpressionValidator())
	historySvc := NewHistoryService(&finalizationTestHistoryStore{trace: trace}, logger)
	service := NewExecutionService(
		wfSvc,
		executor,
		nil,
		store,
		logger,
		interpolation.NewTemplateResolver(),
		historySvc,
	)
	service.SetAuditTrailWriter(&finalizationTestAuditWriter{trace: trace})
	service.SetEventPublisher(&finalizationTestEventPublisher{trace: trace})

	if resume {
		execCtx := workflow.NewExecutionContext("b017-resume", wf.Name)
		execCtx.Status = workflow.StatusFailed
		execCtx.CurrentStep = wf.Initial
		store.states[execCtx.WorkflowID] = execCtx
	}

	return &finalizationTestRig{service: service, store: store, trace: trace, logger: logger}
}

func finalizationTestWorkflow(command string, terminalStatus workflow.TerminalStatus, terminalCode int, startHook, endHook string) *workflow.Workflow {
	steps := map[string]*workflow.Step{
		"terminal": {
			Name: "terminal", Type: workflow.StepTypeTerminal, Status: terminalStatus, ExitCode: terminalCode,
		},
	}
	initial := "terminal"
	if command != "" {
		initial = "execute"
		steps[initial] = &workflow.Step{
			Name: initial, Type: workflow.StepTypeCommand, Command: command,
			OnSuccess: "terminal",
		}
	}

	wf := &workflow.Workflow{Name: "b017-finalization", Initial: initial, Steps: steps}
	if startHook != "" {
		wf.Hooks.WorkflowStart = workflow.Hook{{Command: startHook}}
	}
	wf.Hooks.WorkflowEnd = workflow.Hook{{Command: endHook}}
	wf.Hooks.WorkflowError = workflow.Hook{{Command: "error-hook"}}
	wf.Hooks.WorkflowCancel = workflow.Hook{{Command: "cancel-hook"}}
	return wf
}

//nolint:gocognit // The lifecycle matrix keeps all terminal-path assertions in one discoverable regression test.
func TestExecutionFinalizationCoversEveryTerminalOutcome(t *testing.T) {
	type lifecycleCase struct {
		name                          string
		resume                        bool
		command                       string
		terminalStatus                workflow.TerminalStatus
		terminalCode                  int
		startHook                     string
		wantStatus                    workflow.ExecutionStatus
		wantExitCode                  int
		wantErr                       error
		wantErrText                   string
		wantHook                      string
		assertPanicLogConfidentiality bool
		cancelBeforeRun               bool
	}

	tests := []lifecycleCase{
		{name: "fresh success", wantStatus: workflow.StatusCompleted, wantHook: "end-hook"},
		{name: "resumed success", resume: true, wantStatus: workflow.StatusCompleted, wantHook: "end-hook"},
		{name: "fresh explicit failure", terminalStatus: workflow.TerminalFailure, terminalCode: 17, wantStatus: workflow.StatusFailed, wantExitCode: 17, wantErrText: "terminal failure", wantHook: "error-hook"},
		{name: "resumed explicit failure", resume: true, terminalStatus: workflow.TerminalFailure, terminalCode: 17, wantStatus: workflow.StatusFailed, wantExitCode: 17, wantErrText: "terminal failure", wantHook: "error-hook"},
		{name: "fresh step failure", command: "step-failure", wantStatus: workflow.StatusFailed, wantExitCode: 23, wantErrText: "synthetic step failure", wantHook: "error-hook"},
		{name: "resumed step failure", resume: true, command: "step-failure", wantStatus: workflow.StatusFailed, wantExitCode: 23, wantErrText: "synthetic step failure", wantHook: "error-hook"},
		{name: "cancellation", command: "cancel", wantStatus: workflow.StatusCancelled, wantExitCode: 3, wantErr: context.Canceled, wantErrText: context.Canceled.Error(), wantHook: "cancel-hook"},
		{name: "context cancellation with provider error", command: "cancel-with-provider-error", wantStatus: workflow.StatusCancelled, wantExitCode: 3, wantErrText: "synthetic provider shutdown error", wantHook: "cancel-hook", cancelBeforeRun: true},
		{name: "deadline timeout", command: "deadline", wantStatus: workflow.StatusCancelled, wantExitCode: 3, wantErr: context.DeadlineExceeded, wantErrText: context.DeadlineExceeded.Error(), wantHook: "cancel-hook"},
		{name: "fresh workflow-start hook failure", startHook: "start-failure", wantStatus: workflow.StatusFailed, wantExitCode: 3, wantErrText: "synthetic start-hook failure", wantHook: "error-hook"},
		{name: "resumed workflow-start hook failure", resume: true, startHook: "start-failure", wantStatus: workflow.StatusFailed, wantExitCode: 3, wantErrText: "synthetic start-hook failure", wantHook: "error-hook"},
		{name: "fresh recovered panic", startHook: "start-panic", wantStatus: workflow.StatusFailed, wantExitCode: 3, wantErrText: "panic", wantHook: "error-hook", assertPanicLogConfidentiality: true},
		{name: "resumed recovered panic", resume: true, startHook: "start-panic", wantStatus: workflow.StatusFailed, wantExitCode: 3, wantErrText: "panic", wantHook: "error-hook", assertPanicLogConfidentiality: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := finalizationTestWorkflow(tt.command, tt.terminalStatus, tt.terminalCode, tt.startHook, "end-hook")
			rig := newFinalizationTestRig(t, wf, tt.resume)

			var execCtx *workflow.ExecutionContext
			var runErr error
			var recovered any
			runCtx := context.Background()
			if tt.cancelBeforeRun {
				cancelledCtx, cancel := context.WithCancel(runCtx)
				cancel()
				runCtx = cancelledCtx
			}
			func() {
				defer func() { recovered = recover() }()
				if tt.resume {
					execCtx, runErr = rig.service.Resume(runCtx, "b017-resume", nil, "current")
					return
				}
				execCtx, runErr = rig.service.RunWithWorkflowAndRunID(runCtx, wf, nil, "b017-fresh")
			}()

			require.Nil(t, recovered, "orchestration boundary must recover panics")
			require.NotNil(t, execCtx)
			assert.Equal(t, tt.wantStatus, execCtx.Status)
			assert.Equal(t, tt.wantExitCode, execCtx.ExitCode)
			assert.False(t, execCtx.CompletedAt.IsZero())
			if tt.wantErrText == "" {
				assert.NoError(t, runErr)
			} else {
				require.Error(t, runErr)
				assert.Contains(t, runErr.Error(), tt.wantErrText)
			}
			if tt.wantErr != nil {
				assert.ErrorIs(t, runErr, tt.wantErr)
			}
			if tt.assertPanicLogConfidentiality {
				for _, entry := range rig.logger.entries {
					assert.NotContains(t, entry, "CONFIDENTIAL_PANIC_CANARY")
				}
			}

			require.Len(t, rig.trace.terminalCheckpoints, 2)
			require.Len(t, rig.trace.historyRecords, 1)
			completedAudits := make([]workflow.AuditEvent, 0, 1)
			for _, event := range rig.trace.auditEvents {
				if event.Event == workflow.EventWorkflowCompleted {
					completedAudits = append(completedAudits, event)
				}
			}
			require.Len(t, completedAudits, 1)
			terminalEvents := make([]*pluginmodel.DomainEvent, 0, 1)
			for _, event := range rig.trace.domainEvents {
				if event.Type == workflow.EventWorkflowCompleted || event.Type == workflow.EventWorkflowFailed {
					terminalEvents = append(terminalEvents, event)
				}
			}
			require.Len(t, terminalEvents, 1)

			executionID := execCtx.WorkflowID
			assert.Equal(t, execCtx.HistoryRecordID, rig.trace.historyRecords[0].ID)
			assert.Equal(t, executionID, rig.trace.historyRecords[0].WorkflowID)
			assert.Equal(t, executionID, completedAudits[0].ExecutionID)
			assert.Equal(t, executionID, terminalEvents[0].Metadata["workflow_id"])
			assert.Equal(t, execCtx.ExitCode, rig.trace.historyRecords[0].ExitCode)
			require.NotNil(t, completedAudits[0].ExitCode)
			assert.Equal(t, execCtx.ExitCode, *completedAudits[0].ExitCode)
			assert.Equal(t, []string{"audit", "checkpoint", "history", "checkpoint", "event", "hook"}, rig.trace.calls)
			assert.Equal(t, 1, rig.trace.hookCalls[tt.wantHook])
			assert.True(t, rig.trace.allContextsActive, "terminal projections must use a non-cancelled context")
		})
	}

	t.Run("pre-start failure creates no terminal lifecycle", func(t *testing.T) {
		wf := finalizationTestWorkflow("", workflow.TerminalSuccess, 0, "", "end-hook")
		wf.Inputs = []workflow.Input{{Name: "required", Type: "string", Required: true}}
		rig := newFinalizationTestRig(t, wf, false)

		execCtx, err := rig.service.RunWithWorkflowAndRunID(context.Background(), wf, nil, "b017-preflight")

		require.Error(t, err)
		assert.Nil(t, execCtx)
		assert.Empty(t, rig.trace.auditEvents)
		assert.Empty(t, rig.trace.historyRecords)
		assert.Empty(t, rig.trace.terminalCheckpoints)
	})

	for _, projection := range []string{"checkpoint", "history", "audit"} {
		t.Run(projection+" failure remains best effort", func(t *testing.T) {
			wf := finalizationTestWorkflow("", workflow.TerminalSuccess, 0, "", "end-hook")
			rig := newFinalizationTestRig(t, wf, false)
			injectedErr := errors.New("synthetic " + projection + " failure")
			switch projection {
			case "checkpoint":
				rig.trace.checkpointErr = injectedErr
			case "history":
				rig.trace.historyErr = injectedErr
			case "audit":
				rig.trace.auditErr = injectedErr
			}

			execCtx, err := rig.service.RunWithWorkflowAndRunID(context.Background(), wf, map[string]any{
				"PASSWORD": "CONFIDENTIAL_PROJECTION_CANARY",
			}, "b017-injection")

			require.NoError(t, err)
			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
			assert.Equal(t, 1, rig.trace.hookCalls["end-hook"])
			wantCalls := map[string][]string{
				"checkpoint": {"audit", "checkpoint", "event", "hook"},
				"history":    {"audit", "checkpoint", "history", "event", "hook"},
				"audit":      {"audit", "checkpoint", "history", "checkpoint", "event", "hook"},
			}[projection]
			assert.Equal(t, wantCalls, rig.trace.calls)
			matchingLogs := 0
			for _, entry := range rig.logger.entries {
				if strings.Contains(entry, injectedErr.Error()) {
					matchingLogs++
				}
				assert.NotContains(t, entry, "CONFIDENTIAL_PROJECTION_CANARY")
			}
			wantMatchingLogs := 0
			if projection == "audit" {
				wantMatchingLogs = 1
			}
			assert.Equal(t, wantMatchingLogs, matchingLogs, "history diagnostics must not expose raw persistence errors")
		})
	}

	t.Run("terminal hook failure is logged once without confidential values", func(t *testing.T) {
		wf := finalizationTestWorkflow("", workflow.TerminalSuccess, 0, "", "end-hook-failure")
		rig := newFinalizationTestRig(t, wf, false)

		execCtx, err := rig.service.RunWithWorkflowAndRunID(context.Background(), wf, map[string]any{
			"PASSWORD": "CONFIDENTIAL_INPUT_CANARY",
		}, "b017-hook-log")

		require.NoError(t, err)
		require.NotNil(t, execCtx)
		matchingLogs := 0
		for _, entry := range rig.logger.entries {
			if strings.Contains(entry, "synthetic terminal-hook failure") {
				matchingLogs++
				assert.Contains(t, entry, execCtx.WorkflowID)
				assert.Contains(t, entry, execCtx.WorkflowName)
			}
			assert.NotContains(t, entry, "CONFIDENTIAL_INPUT_CANARY")
			assert.NotContains(t, entry, "CONFIDENTIAL_PANIC_CANARY")
		}
		assert.Equal(t, 1, matchingLogs)
		assert.Len(t, rig.trace.historyRecords, 1)
	})
}

func TestExecutionService_DuplicateTerminationRecordsOnce(t *testing.T) {
	wf := finalizationTestWorkflow("resume-command", workflow.TerminalSuccess, 0, "", "end-hook")
	rig := newFinalizationTestRig(t, wf, false)
	started := time.Date(2026, time.October, 24, 10, 0, 0, 0, time.UTC)
	pending := workflow.NewExecutionContext("stable-workflow-id", wf.Name)
	pending.StartedAt = started
	pending.CurrentStep = "execute"
	pending.SetStatus(workflow.StatusFailed)
	pending.SetCompletedAt(started.Add(3 * time.Second))
	pending.ExitCode = 17
	pending.SetStepState("zeta", workflow.StepState{Status: workflow.StatusFailed, Error: "CONFIDENTIAL_ZETA_CANARY", CompletedAt: pending.CompletedAt})
	pending.SetStepState("alpha", workflow.StepState{Status: workflow.StatusFailed, Error: "deterministic failure", CompletedAt: pending.CompletedAt})
	require.NoError(t, pending.BeginInitialHistoryAttempt("attempt-record-1"))
	rig.store.states[pending.WorkflowID] = pending
	rig.trace.historyErrors = []error{errors.New("uncertain insert"), nil, nil}
	rig.trace.historyStarted = make(chan struct{}, 1)
	rig.trace.historyRelease = make(chan struct{})

	start := make(chan struct{})
	errCh := make(chan error, 12)
	var workers sync.WaitGroup
	for range 12 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := rig.service.Resume(context.Background(), pending.WorkflowID, nil, "current")
			errCh <- err
		}()
	}
	close(start)
	select {
	case <-rig.trace.historyStarted:
	case <-time.After(time.Second):
		t.Error("public resume boundary did not reach history persistence")
	}
	_, records, _ := rig.trace.snapshot()
	assert.Len(t, records, 1, "duplicate terminal notifications must not enter the in-flight writer")
	var failures int
	for range 11 {
		select {
		case concurrentErr := <-errCh:
			if concurrentErr != nil {
				failures++
			}
		case <-time.After(time.Second):
			t.Fatal("concurrent duplicate did not return while the winning writer was in flight")
		}
	}
	close(rig.trace.historyRelease)
	workers.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			failures++
		}
	}
	assert.Positive(t, failures, "the failed winning persistence must be observable through Resume")

	resumed, err := rig.service.Resume(context.Background(), pending.WorkflowID, nil, "current")
	require.NoError(t, err)
	require.NotNil(t, resumed)
	_, records, audits := rig.trace.snapshot()
	require.GreaterOrEqual(t, len(records), 3)
	first, replay := records[0], records[1]
	assert.Equal(t, *first, *replay, "an uncertain insert retry must replay the exact terminal record")
	assert.Equal(t, "attempt-record-1", first.ID)
	assert.Equal(t, "stable-workflow-id", first.WorkflowID)
	assert.Equal(t, 1, first.Attempt)
	assert.Equal(t, workflow.StatusFailed.String(), first.Status)
	assert.Equal(t, started, first.StartedAt)
	assert.Equal(t, started.Add(3*time.Second), first.CompletedAt)
	assert.Equal(t, int64(3000), first.DurationMs)
	assert.Equal(t, 17, first.ExitCode)
	assert.Equal(t, "deterministic failure", first.ErrorMessage, "tied timestamps must select the lexicographically first failed step")
	var completionAudits int
	for _, event := range audits {
		if event.Event == workflow.EventWorkflowCompleted {
			completionAudits++
		}
	}
	assert.Equal(t, 3, completionAudits,
		"a separately loaded context may retry best-effort audit; only each in-memory context is latched")

	t.Run("absent audit writer consumes only the audit latch", func(t *testing.T) {
		caseRig := newFinalizationTestRig(t, wf, false)
		caseRig.service.SetAuditTrailWriter(nil)
		caseCtx := workflow.NewExecutionContext("no-audit-workflow", wf.Name)
		caseCtx.StartedAt = started
		caseCtx.CurrentStep = "execute"
		caseCtx.SetStatus(workflow.StatusFailed)
		caseCtx.SetCompletedAt(started.Add(3 * time.Second))
		caseCtx.ExitCode = 17
		caseCtx.SetStepState("execute", workflow.StepState{
			Status: workflow.StatusFailed, Error: "stable failure", CompletedAt: caseCtx.CompletedAt,
		})
		require.NoError(t, caseCtx.BeginInitialHistoryAttempt("no-audit-record-1"))
		caseRig.trace.historyErrors = []error{errors.New("retryable history failure"), nil}

		firstErr := caseRig.service.finalizeAttemptHistory(context.Background(), caseCtx, "stable failure", nil)
		require.Error(t, firstErr)
		caseRig.service.SetAuditTrailWriter(&finalizationTestAuditWriter{trace: caseRig.trace})
		require.NoError(t, caseRig.service.finalizeAttemptHistory(context.Background(), caseCtx, "stable failure", nil))

		_, caseRecords, caseAudits := caseRig.trace.snapshot()
		require.Len(t, caseRecords, 2)
		assert.Equal(t, *caseRecords[0], *caseRecords[1], "history retry must preserve the original record")
		assert.Empty(t, caseAudits, "a later writer must not receive the already-consumed best-effort audit")
		_, _, _, recorded := caseCtx.GetHistoryAttempt()
		assert.True(t, recorded)
	})
}

func TestExecutionService_HistoryPersistenceFailurePreservesExecutionOutcome(t *testing.T) {
	wf := finalizationTestWorkflow("step-failure", workflow.TerminalSuccess, 0, "", "end-hook")
	rig := newFinalizationTestRig(t, wf, false)
	executionErr := errors.New("stable execution failure")
	rig.trace.stepFailureErr = executionErr
	rig.trace.historyErrors = []error{errors.New("CONFIDENTIAL_STORE_CANARY"), nil}

	execCtx, runErr := rig.service.RunWithWorkflowAndRunID(context.Background(), wf, map[string]any{
		"PASSWORD": "CONFIDENTIAL_INPUT_CANARY",
	}, "stable-failed-run")

	require.Error(t, runErr)
	assert.ErrorIs(t, runErr, executionErr)
	require.NotNil(t, execCtx)
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Equal(t, 23, execCtx.ExitCode)
	predecessorRecordID := execCtx.HistoryRecordID

	// The retry reconciles the failed predecessor, then executes a successful next attempt.
	rig.trace.stepFailureErr = nil
	rig.trace.stepFailureSuccess = true
	resumed, resumeErr := rig.service.Resume(context.Background(), execCtx.WorkflowID, nil, "current")
	require.NoError(t, resumeErr)
	require.NotNil(t, resumed)
	assert.ErrorIs(t, runErr, executionErr, "history persistence failure must not replace the workflow error")
	_, records, _ := rig.trace.snapshot()
	require.GreaterOrEqual(t, len(records), 3)
	assert.Equal(t, *records[0], *records[1])
	assert.Equal(t, predecessorRecordID, records[0].ID)
	assert.Equal(t, execCtx.WorkflowID, records[0].WorkflowID)
	assert.Equal(t, 1, records[0].Attempt)

	entries, logs := rig.logger.snapshot()
	var warnings []finalizationTestLogEntry
	for _, entry := range logs {
		if entry.message == "history persistence failed" {
			warnings = append(warnings, entry)
		}
	}
	require.Len(t, warnings, 1)
	assert.Equal(t, "warn", warnings[0].level)
	require.Len(t, warnings[0].fields, 6)
	assert.Equal(t, []any{
		"workflow_id", execCtx.WorkflowID,
		"history_record_id", predecessorRecordID,
		"attempt", 1,
	}, warnings[0].fields, "the warning must contain only approved metadata")

	for _, entry := range entries {
		assert.NotContains(t, entry, "CONFIDENTIAL_INPUT_CANARY")
		assert.NotContains(t, entry, "CONFIDENTIAL_STORE_CANARY")
	}
	assert.NotContains(t, fmt.Sprint(warnings), records[0].ErrorMessage)

	t.Run("terminal snapshot failure prevents an unreplayable insert", func(t *testing.T) {
		caseRig := newFinalizationTestRig(t, wf, false)
		caseRig.trace.checkpointErr = errors.New("snapshot unavailable")

		caseCtx, caseErr := caseRig.service.RunWithWorkflowAndRunID(context.Background(), wf, nil, "snapshot-failure-run")

		require.Error(t, caseErr)
		assert.Contains(t, caseErr.Error(), "synthetic step failure")
		require.NotNil(t, caseCtx)
		_, caseRecords, caseAudits := caseRig.trace.snapshot()
		assert.Empty(t, caseRecords)
		var completionAudits int
		for _, event := range caseAudits {
			if event.Event == workflow.EventWorkflowCompleted {
				completionAudits++
			}
		}
		assert.Equal(t, 1, completionAudits, "best-effort audit remains independent of history snapshot persistence")
		_, _, _, recorded := caseCtx.GetHistoryAttempt()
		assert.False(t, recorded)
	})

	t.Run("marker save failure keeps the in-memory commit and prevents duplicate write", func(t *testing.T) {
		caseWF := finalizationTestWorkflow("", workflow.TerminalSuccess, 0, "", "end-hook")
		caseRig := newFinalizationTestRig(t, caseWF, false)
		caseRig.trace.checkpointErrors = []error{nil, errors.New("marker unavailable")}

		caseCtx, caseErr := caseRig.service.RunWithWorkflowAndRunID(context.Background(), caseWF, nil, "marker-failure-run")

		require.NoError(t, caseErr)
		require.NotNil(t, caseCtx)
		_, _, _, recorded := caseCtx.GetHistoryAttempt()
		assert.True(t, recorded)
		_, caseRecords, _ := caseRig.trace.snapshot()
		assert.Len(t, caseRecords, 1)
		var markerDiagnostics int
		for _, entry := range caseRig.logger.entries {
			if strings.Contains(entry, "history") && strings.Contains(entry, "marker") {
				markerDiagnostics++
			}
		}
		assert.Equal(t, 1, markerDiagnostics)
	})
}

func TestExecutionService_ResumePreservesPendingAttemptHistory(t *testing.T) {
	newPendingRig := func(t *testing.T) (*finalizationTestRig, *workflow.ExecutionContext) {
		t.Helper()
		wf := finalizationTestWorkflow("resume-command", workflow.TerminalSuccess, 0, "", "end-hook")
		rig := newFinalizationTestRig(t, wf, false)
		started := time.Date(2026, time.October, 24, 9, 0, 0, 0, time.UTC)
		pending := workflow.NewExecutionContext("stable-resume-id", wf.Name)
		pending.StartedAt = started
		pending.CurrentStep = "execute"
		pending.SetStatus(workflow.StatusFailed)
		pending.SetCompletedAt(started.Add(time.Minute))
		pending.ExitCode = 31
		pending.SetStepState("execute", workflow.StepState{
			Status: workflow.StatusFailed, ExitCode: 31, Error: "predecessor failure", CompletedAt: pending.CompletedAt,
		})
		require.NoError(t, pending.BeginInitialHistoryAttempt("pending-record-1"))
		rig.store.states[pending.WorkflowID] = pending
		return rig, pending
	}

	t.Run("failed reconciliation blocks the resumed command and preserves identity", func(t *testing.T) {
		rig, pending := newPendingRig(t)
		persistErr := errors.New("history unavailable")
		rig.trace.historyErr = persistErr

		got, err := rig.service.Resume(context.Background(), pending.WorkflowID, nil, "current")

		assert.ErrorIs(t, err, persistErr)
		assert.Nil(t, got)
		rig.trace.mu.Lock()
		assert.Zero(t, rig.trace.commandCalls)
		rig.trace.mu.Unlock()
		assert.Equal(t, "pending-record-1", pending.HistoryRecordID)
		assert.Equal(t, 1, pending.HistoryAttempt)
		assert.Equal(t, workflow.StatusFailed, pending.Status)
		assert.Equal(t, time.Minute, pending.CompletedAt.Sub(pending.StartedAt))
	})

	t.Run("uncertain predecessor replay preserves both attempts", func(t *testing.T) {
		rig, pending := newPendingRig(t)
		rig.trace.historyErrors = []error{errors.New("uncertain predecessor insert"), nil, nil}

		first, firstErr := rig.service.Resume(context.Background(), pending.WorkflowID, nil, "current")
		assert.Error(t, firstErr)
		assert.Nil(t, first)
		resumed, resumeErr := rig.service.Resume(context.Background(), pending.WorkflowID, nil, "current")

		require.NoError(t, resumeErr)
		require.NotNil(t, resumed)
		_, records, _ := rig.trace.snapshot()
		require.Len(t, records, 3)
		assert.Equal(t, *records[0], *records[1], "the acknowledged but unmarked predecessor must replay exactly")
		assert.Equal(t, "pending-record-1", records[0].ID)
		assert.Equal(t, 1, records[0].Attempt)
		assert.Equal(t, 2, records[2].Attempt)
		assert.Equal(t, records[0].ID, records[2].ResumedFromID)
		assert.Equal(t, records[0].WorkflowID, records[2].WorkflowID)
		assert.NotEqual(t, records[0].ID, records[2].ID)
	})

	t.Run("invalid resume is rejected before reconciliation", func(t *testing.T) {
		rig, pending := newPendingRig(t)
		pending.CurrentStep = "removed-step"

		got, err := rig.service.Resume(context.Background(), pending.WorkflowID, nil, "current")

		assert.Error(t, err)
		assert.Nil(t, got)
		_, records, audits := rig.trace.snapshot()
		assert.Empty(t, records)
		assert.Empty(t, audits)
	})

	t.Run("legacy context still normalizes after validation", func(t *testing.T) {
		rig, pending := newPendingRig(t)
		pending.HistoryRecordID = ""
		pending.HistoryAttempt = 0

		resumed, err := rig.service.Resume(context.Background(), pending.WorkflowID, nil, "current")

		require.NoError(t, err)
		require.NotNil(t, resumed)
		assert.Equal(t, 1, resumed.HistoryAttempt)
		assert.NotEmpty(t, resumed.HistoryRecordID)
		_, records, _ := rig.trace.snapshot()
		require.Len(t, records, 1)
		assert.Equal(t, 1, records[0].Attempt)
	})

	t.Run("disabled history does not block resume", func(t *testing.T) {
		rig, pending := newPendingRig(t)
		rig.service.historySvc = NewHistoryService(nil, rig.logger)

		resumed, err := rig.service.Resume(context.Background(), pending.WorkflowID, nil, "current")

		require.NoError(t, err)
		require.NotNil(t, resumed)
		rig.trace.mu.Lock()
		assert.Positive(t, rig.trace.commandCalls)
		rig.trace.mu.Unlock()
		_, records, _ := rig.trace.snapshot()
		assert.Empty(t, records)
		require.NotEmpty(t, rig.trace.terminalCheckpoints)
		checkpoint := rig.trace.terminalCheckpoints[len(rig.trace.terminalCheckpoints)-1]
		assert.Equal(t, workflow.StatusCompleted, checkpoint.GetStatus())
		assert.False(t, checkpoint.GetCompletedAt().IsZero(), "disabled history must still persist the terminal snapshot")
	})
}

func TestExecutionService_ParentAndChildAttemptsKeepIndependentLineage(t *testing.T) {
	trace := newFinalizationTestTrace()
	store := &finalizationTestStore{trace: trace, states: make(map[string]*workflow.ExecutionContext)}
	executor := &finalizationTestExecutor{trace: trace}
	logger := &finalizationTestLogger{}
	child := finalizationTestWorkflow("child-command", workflow.TerminalSuccess, 0, "", "")
	child.Name = "child-history-workflow"
	parent := &workflow.Workflow{
		Name: "parent-history-workflow", Initial: "call-child",
		Steps: map[string]*workflow.Step{
			"call-child": {
				Name: "call-child", Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{Workflow: child.Name}, OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}
	repository := testmocks.NewMockWorkflowRepository()
	repository.AddWorkflow(parent.Name, parent)
	repository.AddWorkflow(child.Name, child)
	wfSvc := NewWorkflowService(repository, store, executor, logger, testmocks.NewMockExpressionValidator())
	service := NewExecutionService(wfSvc, executor, nil, store, logger, interpolation.NewTemplateResolver(),
		NewHistoryService(&finalizationTestHistoryStore{trace: trace}, logger))

	parentCtx, err := service.RunWithWorkflowAndRunID(context.Background(), parent, nil, "stable-parent-run")

	require.NoError(t, err)
	require.NotNil(t, parentCtx)
	_, records, _ := trace.snapshot()
	require.Len(t, records, 2)
	recordByName := make(map[string]*workflow.ExecutionRecord, 2)
	for _, record := range records {
		recordByName[record.WorkflowName] = record
	}
	parentRecord := recordByName[parent.Name]
	childRecord := recordByName[child.Name]
	require.NotNil(t, parentRecord)
	require.NotNil(t, childRecord)
	assert.Equal(t, "stable-parent-run", parentRecord.WorkflowID)
	assert.Equal(t, parentCtx.HistoryRecordID, parentRecord.ID)
	assert.Equal(t, 1, parentRecord.Attempt)
	assert.Empty(t, parentRecord.ResumedFromID)
	assert.NotEqual(t, parentRecord.WorkflowID, childRecord.WorkflowID)
	assert.NotEqual(t, parentRecord.ID, childRecord.ID)
	assert.Equal(t, 1, childRecord.Attempt)
	assert.Empty(t, childRecord.ResumedFromID, "call-workflow ancestry must not become resume lineage")
}

func TestNormalizeTerminalExitCode(t *testing.T) {
	baseTime := time.Date(2026, time.October, 17, 12, 0, 0, 0, time.UTC)
	wrappedSystemErr := fmt.Errorf("finalize workflow: %w", domainerrors.NewSystemError(
		domainerrors.ErrorCodeSystemIOPermissionDenied,
		"permission denied",
		nil,
		nil,
	))

	tests := []struct {
		name          string
		status        workflow.ExecutionStatus
		exitCode      int
		currentStep   string
		states        map[string]workflow.StepState
		completionErr error
		repeatErr     error
		expected      int
	}{
		{
			name:     "preserves successful completion with zero exit code",
			status:   workflow.StatusCompleted,
			expected: 0,
		},
		{
			name:     "preserves existing non-zero execution code",
			status:   workflow.StatusFailed,
			exitCode: 42,
			expected: 42,
		},
		{
			name:     "preserves explicit terminal code on completed execution",
			status:   workflow.StatusCompleted,
			exitCode: 9,
			expected: 9,
		},
		{
			name:        "prefers current failed step",
			status:      workflow.StatusFailed,
			currentStep: "current",
			states: map[string]workflow.StepState{
				"current": {Status: workflow.StatusFailed, ExitCode: 7, CompletedAt: baseTime},
				"later":   {Status: workflow.StatusFailed, ExitCode: 8, CompletedAt: baseTime.Add(time.Hour)},
			},
			expected: 7,
		},
		{
			name:   "selects latest completed failed step",
			status: workflow.StatusFailed,
			states: map[string]workflow.StepState{
				"earlier": {Status: workflow.StatusFailed, ExitCode: 6, CompletedAt: baseTime},
				"latest":  {Status: workflow.StatusFailed, ExitCode: 8, CompletedAt: baseTime.Add(time.Hour)},
			},
			expected: 8,
		},
		{
			name:   "breaks equal completion timestamps by lexicographically smallest key",
			status: workflow.StatusFailed,
			states: map[string]workflow.StepState{
				"zeta":  {Status: workflow.StatusFailed, ExitCode: 9, CompletedAt: baseTime},
				"alpha": {Status: workflow.StatusFailed, ExitCode: 5, CompletedAt: baseTime},
			},
			expected: 5,
		},
		{
			name:        "falls back when current step is missing",
			status:      workflow.StatusFailed,
			currentStep: "missing",
			states: map[string]workflow.StepState{
				"failed": {Status: workflow.StatusFailed, ExitCode: 6, CompletedAt: baseTime},
			},
			expected: 6,
		},
		{
			name:        "falls back when current failed step has zero code",
			status:      workflow.StatusFailed,
			currentStep: "current",
			states: map[string]workflow.StepState{
				"current": {Status: workflow.StatusFailed, ExitCode: 0, CompletedAt: baseTime.Add(time.Hour)},
				"failed":  {Status: workflow.StatusFailed, ExitCode: 6, CompletedAt: baseTime},
			},
			expected: 6,
		},
		{
			name:        "falls back when current step is not failed",
			status:      workflow.StatusFailed,
			currentStep: "current",
			states: map[string]workflow.StepState{
				"current": {Status: workflow.StatusCompleted, ExitCode: 7, CompletedAt: baseTime.Add(time.Hour)},
				"failed":  {Status: workflow.StatusFailed, ExitCode: 6, CompletedAt: baseTime},
			},
			expected: 6,
		},
		{
			name:   "ignores undated fallback failed states",
			status: workflow.StatusFailed,
			states: map[string]workflow.StepState{
				"undated": {Status: workflow.StatusFailed, ExitCode: 7},
			},
			expected: 3,
		},
		{
			name:   "preserves successful completion after recovered failed step",
			status: workflow.StatusCompleted,
			states: map[string]workflow.StepState{
				"recovered": {Status: workflow.StatusFailed, ExitCode: 7, CompletedAt: baseTime},
			},
			completionErr: errors.New("recovered failure"),
			expected:      0,
		},
		{
			name:          "uses wrapped structured error exit code",
			status:        workflow.StatusFailed,
			completionErr: wrappedSystemErr,
			expected:      4,
		},
		{
			name:          "uses execution fallback for uncategorized failure",
			status:        workflow.StatusFailed,
			completionErr: errors.New("uncategorized failure"),
			expected:      3,
		},
		{
			name:          "uses execution fallback for cancellation",
			status:        workflow.StatusCancelled,
			completionErr: context.Canceled,
			expected:      3,
		},
		{
			name:        "is idempotent when normalization is repeated",
			status:      workflow.StatusFailed,
			currentStep: "failed",
			states: map[string]workflow.StepState{
				"failed": {Status: workflow.StatusFailed, ExitCode: 7, CompletedAt: baseTime},
			},
			completionErr: errors.New("first failure"),
			repeatErr:     wrappedSystemErr,
			expected:      7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execCtx := workflow.NewExecutionContext("execution-id", "workflow")
			execCtx.Status = tt.status
			execCtx.ExitCode = tt.exitCode
			execCtx.CurrentStep = tt.currentStep
			for name, state := range tt.states {
				execCtx.SetStepState(name, state)
			}

			normalizeTerminalExitCode(execCtx, tt.completionErr)
			if tt.repeatErr != nil {
				normalizeTerminalExitCode(execCtx, tt.repeatErr)
			}

			assert.Equal(t, tt.expected, execCtx.ExitCode)
		})
	}
}

func BenchmarkNormalizeTerminalExitCodeP95(b *testing.B) {
	baseTime := time.Date(2026, time.October, 17, 12, 0, 0, 0, time.UTC)
	wrappedExecutionErr := fmt.Errorf("execute workflow: %w", domainerrors.NewExecutionError(
		domainerrors.ErrorCodeExecutionCommandFailed,
		"command failed",
		nil,
		nil,
	))

	contexts := make([]*workflow.ExecutionContext, 6)
	for index := range contexts {
		contexts[index] = workflow.NewExecutionContext("benchmark", "workflow")
		contexts[index].Status = workflow.StatusFailed
	}
	contexts[0].ExitCode = 2
	contexts[1].CurrentStep = "current"
	contexts[1].SetStepState("current", workflow.StepState{
		Status: workflow.StatusFailed, ExitCode: 7, CompletedAt: baseTime,
	})
	for index := range 100 {
		name := fmt.Sprintf("step-%03d", index)
		contexts[2].SetStepState(name, workflow.StepState{
			Status: workflow.StatusFailed, ExitCode: index + 1, CompletedAt: baseTime.Add(time.Duration(index/2) * time.Second),
		})
	}
	errorsByCase := []error{
		nil,
		nil,
		nil,
		wrappedExecutionErr,
		errors.New("uncategorized failure"),
		context.Canceled,
	}
	contexts[5].Status = workflow.StatusCancelled
	durations := make([]time.Duration, b.N)

	b.ResetTimer()
	for iteration := range b.N {
		caseIndex := iteration % len(contexts)
		b.StopTimer()
		if caseIndex != 0 {
			contexts[caseIndex].ExitCode = 0
		}
		b.StartTimer()

		startedAt := time.Now()
		normalizeTerminalExitCode(contexts[caseIndex], errorsByCase[caseIndex])
		elapsed := time.Since(startedAt)
		b.StopTimer()
		durations[iteration] = elapsed
	}

	sort.Slice(durations, func(left, right int) bool {
		return durations[left] < durations[right]
	})
	nearestRank := int(math.Ceil(0.95*float64(len(durations)))) - 1
	p95 := durations[nearestRank]
	b.ReportMetric(float64(p95.Nanoseconds()), "p95-ns/op")
	if p95 >= time.Millisecond {
		b.Fatalf("p95 normalization latency %s is >= 1ms", p95)
	}
}

// TestResolveAuditUser tests the resolveAuditUser method.
// This method resolves the OS username for audit trail entries.
func TestResolveAuditUser(t *testing.T) {
	tests := []struct {
		name        string
		expectEmpty bool
	}{
		{
			name:        "resolves user identity",
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc := &ExecutionService{}

			user := execSvc.resolveAuditUser()

			if tt.expectEmpty {
				assert.Empty(t, user)
			} else {
				// Should return non-empty string (actual user or fallback to "unknown")
				assert.NotEmpty(t, user)
			}
		})
	}
}

// TestEmitAuditStarted tests the emitAuditStarted method.
// This method emits a workflow.started audit event with masked inputs.
func TestEmitAuditStarted(t *testing.T) {
	tests := []struct {
		name           string
		inputs         map[string]any
		expectedEvent  string
		expectedFields []string
		withWriter     bool
	}{
		{
			name: "emits started event with basic inputs",
			inputs: map[string]any{
				"env":     "staging",
				"version": "1.0",
			},
			expectedEvent:  workflow.EventWorkflowStarted,
			expectedFields: []string{"execution_id", "timestamp", "user", "workflow_name", "inputs"},
			withWriter:     true,
		},
		{
			name:           "emits started event with empty inputs",
			inputs:         map[string]any{},
			expectedEvent:  workflow.EventWorkflowStarted,
			expectedFields: []string{},
			withWriter:     true,
		},
		{
			name:           "emits started event with nil inputs",
			inputs:         nil,
			expectedEvent:  workflow.EventWorkflowStarted,
			expectedFields: []string{},
			withWriter:     true,
		},
		{
			name: "skips emit when writer is nil",
			inputs: map[string]any{
				"env": "staging",
			},
			expectedEvent:  "",
			expectedFields: []string{},
			withWriter:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockWriter *testmocks.MockAuditTrailWriter
			var auditWriter ports.AuditTrailWriter
			if tt.withWriter {
				mockWriter = testmocks.NewMockAuditTrailWriter()
				auditWriter = mockWriter
			}

			execSvc := &ExecutionService{
				auditTrailWriter: auditWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			ctx := context.Background()

			execSvc.emitAuditStarted(ctx, execCtx, tt.inputs)

			if tt.withWriter && mockWriter != nil {
				events := mockWriter.GetEvents()
				require.Len(t, events, 1, "should record one started event")

				event := events[0]
				assert.Equal(t, tt.expectedEvent, event.Event)
				assert.Equal(t, "test-exec-id", event.ExecutionID)
				assert.Equal(t, "test-workflow", event.WorkflowName)
				assert.Equal(t, 1, event.SchemaVersion)
				assert.NotEmpty(t, event.Timestamp)
				assert.NotEmpty(t, event.User)

				// Verify inputs are captured
				if len(tt.inputs) > 0 {
					assert.NotNil(t, event.Inputs)
					assert.Equal(t, len(tt.inputs), len(event.Inputs))
				}
			}
		})
	}
}

// TestEmitAuditStartedWithMaskedSecrets tests that secret inputs are masked
// in the audit started event.
func TestEmitAuditStartedWithMaskedSecrets(t *testing.T) {
	tests := []struct {
		name           string
		inputs         map[string]any
		expectedMasked map[string]string
	}{
		{
			name: "preserves all input fields",
			inputs: map[string]any{
				"api_key": "sk-secret123",
				"env":     "prod",
			},
			expectedMasked: map[string]string{
				"api_key": "***",
				"env":     "prod",
			},
		},
		{
			name: "preserves mixed inputs",
			inputs: map[string]any{
				"SECRET_TOKEN": "my-secret",
				"debug":        "true",
			},
			expectedMasked: map[string]string{
				"SECRET_TOKEN": "***",
				"debug":        "true",
			},
		},
		{
			name: "preserves password field",
			inputs: map[string]any{
				"PASSWORD": "hunter2",
				"username": "alice",
			},
			expectedMasked: map[string]string{
				"PASSWORD": "***",
				"username": "alice",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			ctx := context.Background()

			// Pass masked inputs (application layer should do this)
			maskedInputs := make(map[string]any)
			for k, v := range tt.inputs {
				maskedInputs[k] = v
			}

			execSvc.emitAuditStarted(ctx, execCtx, maskedInputs)

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			assert.Equal(t, workflow.EventWorkflowStarted, event.Event)

			// Note: actual masking is done by application layer before calling emit
			// This test verifies the fields are preserved in the audit event
			if event.Inputs != nil {
				for key := range tt.expectedMasked {
					assert.Contains(t, event.Inputs, key)
				}
			}
		})
	}
}

// TestEmitAuditCompleted tests the emitAuditCompleted method.
// This method emits a workflow.completed audit event.
func TestEmitAuditCompleted(t *testing.T) {
	tests := []struct {
		name           string
		status         workflow.ExecutionStatus
		exitCode       int
		errorMsg       string
		expectedEvent  string
		expectedStatus string
		withWriter     bool
	}{
		{
			name:           "emits completed event with success status",
			status:         workflow.StatusCompleted,
			exitCode:       0,
			errorMsg:       "",
			expectedEvent:  workflow.EventWorkflowCompleted,
			expectedStatus: "success",
			withWriter:     true,
		},
		{
			name:           "emits completed event with failure status",
			status:         workflow.StatusFailed,
			exitCode:       1,
			errorMsg:       "step 'deploy' failed: connection timeout",
			expectedEvent:  workflow.EventWorkflowCompleted,
			expectedStatus: "failure",
			withWriter:     true,
		},
		{
			name:           "emits completed event with non-zero exit code",
			status:         workflow.StatusFailed,
			exitCode:       127,
			errorMsg:       "command not found",
			expectedEvent:  workflow.EventWorkflowCompleted,
			expectedStatus: "failure",
			withWriter:     true,
		},
		{
			name:           "skips emit when writer is nil",
			status:         workflow.StatusCompleted,
			exitCode:       0,
			errorMsg:       "",
			expectedEvent:  "",
			expectedStatus: "",
			withWriter:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockWriter *testmocks.MockAuditTrailWriter
			var auditWriter ports.AuditTrailWriter
			if tt.withWriter {
				mockWriter = testmocks.NewMockAuditTrailWriter()
				auditWriter = mockWriter
			}

			execSvc := &ExecutionService{
				auditTrailWriter: auditWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = tt.status
			execCtx.ExitCode = tt.exitCode
			ctx := context.Background()

			execSvc.emitAuditCompleted(ctx, execCtx, tt.errorMsg)

			if tt.withWriter && mockWriter != nil {
				events := mockWriter.GetEvents()
				require.Len(t, events, 1, "should record one completed event")

				event := events[0]
				assert.Equal(t, tt.expectedEvent, event.Event)
				assert.Equal(t, "test-exec-id", event.ExecutionID)
				assert.Equal(t, "test-workflow", event.WorkflowName)
				assert.Equal(t, tt.exitCode, *event.ExitCode)
				assert.NotEmpty(t, event.Timestamp)
				assert.NotEmpty(t, event.User)

				// Verify status field
				assert.Equal(t, tt.expectedStatus, event.Status)

				// Verify error message
				if tt.errorMsg != "" {
					assert.Equal(t, tt.errorMsg, event.Error)
				}

				// Verify duration is recorded (should be small but non-zero)
				assert.NotNil(t, event.DurationMs)
				assert.Greater(t, *event.DurationMs, int64(0))
			}
		})
	}
}

// TestEmitAuditCompletedCalculatesDuration tests that duration is correctly
// calculated from execution context start time.
func TestEmitAuditCompletedCalculatesDuration(t *testing.T) {
	tests := []struct {
		name           string
		timeSince      time.Duration
		expectedMinDms int64
	}{
		{
			name:           "calculates duration for short execution",
			timeSince:      10 * time.Millisecond,
			expectedMinDms: 5,
		},
		{
			name:           "calculates duration for longer execution",
			timeSince:      500 * time.Millisecond,
			expectedMinDms: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = workflow.StatusCompleted
			execCtx.ExitCode = 0
			execCtx.StartedAt = time.Now().Add(-tt.timeSince)

			ctx := context.Background()

			execSvc.emitAuditCompleted(ctx, execCtx, "")

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			require.NotNil(t, event.DurationMs)
			assert.GreaterOrEqual(t, *event.DurationMs, tt.expectedMinDms)
		})
	}
}

// TestEmitAuditCompletedWithEmptyError tests that empty error message is not included.
func TestEmitAuditCompletedWithEmptyError(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0
	ctx := context.Background()

	execSvc.emitAuditCompleted(ctx, execCtx, "")

	events := mockWriter.GetEvents()
	require.Len(t, events, 1)

	event := events[0]
	assert.Empty(t, event.Error, "error field should be empty for successful completion")
}

// TestSetAuditTrailWriter tests the SetAuditTrailWriter method.
func TestSetAuditTrailWriter(t *testing.T) {
	tests := []struct {
		name           string
		writer         *testmocks.MockAuditTrailWriter
		expectSetEvent bool
	}{
		{
			name:           "sets audit trail writer",
			writer:         testmocks.NewMockAuditTrailWriter(),
			expectSetEvent: true,
		},
		{
			name:           "handles nil writer",
			writer:         nil,
			expectSetEvent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc := &ExecutionService{}

			if tt.writer != nil {
				execSvc.SetAuditTrailWriter(tt.writer)
			}

			// Verify the writer was set by emitting an event
			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			ctx := context.Background()

			execSvc.emitAuditStarted(ctx, execCtx, nil)

			if tt.expectSetEvent && tt.writer != nil {
				events := tt.writer.GetEvents()
				require.Len(t, events, 1)
				assert.Equal(t, workflow.EventWorkflowStarted, events[0].Event)
			}
		})
	}
}

// TestAuditWriteFailureDoesNotBlock tests that audit write failures don't affect execution.
// Per FR-006: workflow execution must complete even if audit trail write fails.
func TestAuditWriteFailureDoesNotBlock(t *testing.T) {
	tests := []struct {
		name       string
		failOnCall string
	}{
		{
			name:       "write failure does not block execution",
			failOnCall: "write",
		},
		{
			name:       "close failure does not block execution",
			failOnCall: "close",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()

			switch tt.failOnCall {
			case "write":
				mockWriter.SetWriteError(errors.New("write permission denied"))
			case "close":
				mockWriter.SetCloseError(errors.New("close failed"))
			}

			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			ctx := context.Background()

			// These should not panic or error even though the writer fails
			execSvc.emitAuditStarted(ctx, execCtx, nil)
			execSvc.emitAuditCompleted(ctx, execCtx, "")
		})
	}
}

// TestAuditEventFieldOrdering tests that audit events have correct field ordering.
// Per spec: event, execution_id, timestamp, user, workflow_name, then event-specific fields.
func TestAuditEventFieldOrdering(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	ctx := context.Background()

	// Emit started event
	execSvc.emitAuditStarted(ctx, execCtx, map[string]any{"key": "value"})

	events := mockWriter.GetEvents()
	require.Len(t, events, 1)

	event := events[0]

	// Verify all expected common fields are present
	assert.NotEmpty(t, event.Event)
	assert.NotEmpty(t, event.ExecutionID)
	assert.NotZero(t, event.Timestamp)
	assert.NotEmpty(t, event.User)
	assert.NotEmpty(t, event.WorkflowName)
	assert.Equal(t, 1, event.SchemaVersion)
}

// TestAuditEventWithLongDuration tests duration calculation for long-running executions.
func TestAuditEventWithLongDuration(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0
	// Simulate a 5-second execution
	execCtx.StartedAt = time.Now().Add(-5 * time.Second)

	ctx := context.Background()

	execSvc.emitAuditCompleted(ctx, execCtx, "")

	events := mockWriter.GetEvents()
	require.Len(t, events, 1)

	event := events[0]
	require.NotNil(t, event.DurationMs)
	// Should be roughly 5000ms (5 seconds), allow variance
	assert.GreaterOrEqual(t, *event.DurationMs, int64(4900))
	assert.LessOrEqual(t, *event.DurationMs, int64(5100))
}

// TestAuditStartedEventSchema tests the schema version and structure.
func TestAuditStartedEventSchema(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	ctx := context.Background()

	execSvc.emitAuditStarted(ctx, execCtx, nil)

	events := mockWriter.GetEvents()
	require.Len(t, events, 1)

	event := events[0]

	// Verify started event structure (per spec)
	assert.Equal(t, workflow.EventWorkflowStarted, event.Event)
	assert.Equal(t, 1, event.SchemaVersion)
	assert.Equal(t, "test-exec-id", event.ExecutionID)
	assert.Equal(t, "test-workflow", event.WorkflowName)
	assert.NotZero(t, event.Timestamp)
	assert.NotEmpty(t, event.User)

	// Completed-specific fields should be nil/empty
	assert.Empty(t, event.Status)
	assert.Nil(t, event.ExitCode)
	assert.Nil(t, event.DurationMs)
	assert.Empty(t, event.Error)
}

// TestAuditCompletedEventSchema tests the schema version and structure.
func TestAuditCompletedEventSchema(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0
	ctx := context.Background()

	execSvc.emitAuditCompleted(ctx, execCtx, "")

	events := mockWriter.GetEvents()
	require.Len(t, events, 1)

	event := events[0]

	// Verify completed event structure (per spec)
	assert.Equal(t, workflow.EventWorkflowCompleted, event.Event)
	assert.Equal(t, 1, event.SchemaVersion)
	assert.Equal(t, "test-exec-id", event.ExecutionID)
	assert.Equal(t, "test-workflow", event.WorkflowName)
	assert.NotZero(t, event.Timestamp)
	assert.NotEmpty(t, event.User)

	// Completed-specific fields should be populated
	assert.NotEmpty(t, event.Status)
	assert.NotNil(t, event.ExitCode)
	assert.NotNil(t, event.DurationMs)
}

// TestAuditStartedAndCompletedPairShareExecutionID verifies paired events
func TestAuditStartedAndCompletedPairShareExecutionID(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execID := "pair-test-id"
	execCtx := workflow.NewExecutionContext(execID, "test-workflow")
	ctx := context.Background()

	// Clear any previous events
	mockWriter.Clear()

	// Emit both events
	execSvc.emitAuditStarted(ctx, execCtx, nil)
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0
	execSvc.emitAuditCompleted(ctx, execCtx, "")

	events := mockWriter.GetEvents()
	require.Len(t, events, 2)

	// Both events should have the same execution_id
	assert.Equal(t, events[0].ExecutionID, execID)
	assert.Equal(t, events[1].ExecutionID, execID)
	assert.Equal(t, events[0].ExecutionID, events[1].ExecutionID)

	// Verify event types
	assert.Equal(t, workflow.EventWorkflowStarted, events[0].Event)
	assert.Equal(t, workflow.EventWorkflowCompleted, events[1].Event)
}

// TestRecordExecutionEnd tests the recordExecutionEnd helper method.
// This method consolidates recordHistory() + emitAuditCompleted() to reduce code duplication.
func TestRecordExecutionEnd(t *testing.T) {
	tests := []struct {
		name         string
		status       workflow.ExecutionStatus
		exitCode     int
		errorMsg     string
		withWriter   bool
		withRecorder bool
	}{
		{
			name:         "consolidates both calls on success",
			status:       workflow.StatusCompleted,
			exitCode:     0,
			errorMsg:     "",
			withWriter:   true,
			withRecorder: true,
		},
		{
			name:         "consolidates both calls on failure",
			status:       workflow.StatusFailed,
			exitCode:     1,
			errorMsg:     "step failed: timeout",
			withWriter:   true,
			withRecorder: true,
		},
		{
			name:         "handles nil writer gracefully",
			status:       workflow.StatusCompleted,
			exitCode:     0,
			errorMsg:     "",
			withWriter:   false,
			withRecorder: true,
		},
		{
			name:         "works without history recorder",
			status:       workflow.StatusCompleted,
			exitCode:     0,
			errorMsg:     "",
			withWriter:   true,
			withRecorder: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			var auditWriter ports.AuditTrailWriter
			if tt.withWriter {
				auditWriter = mockWriter
			}

			execSvc := &ExecutionService{
				store:            testmocks.NewMockStateStore(),
				auditTrailWriter: auditWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = tt.status
			execCtx.ExitCode = tt.exitCode
			execCtx.StartedAt = time.Now().Add(-100 * time.Millisecond)
			ctx := context.Background()

			// Call the consolidated helper
			execSvc.recordExecutionEnd(ctx, execCtx, nil, tt.errorMsg)

			// When writer is present, should emit audit completed event
			if tt.withWriter && mockWriter != nil {
				events := mockWriter.GetEvents()
				require.Len(t, events, 1, "should emit one audit event")

				event := events[0]
				assert.Equal(t, workflow.EventWorkflowCompleted, event.Event)
				assert.Equal(t, "test-exec-id", event.ExecutionID)
				assert.Equal(t, "test-workflow", event.WorkflowName)
				assert.Equal(t, tt.exitCode, *event.ExitCode)
				assert.NotNil(t, event.DurationMs)
				assert.GreaterOrEqual(t, *event.DurationMs, int64(50))
			}
		})
	}
}

func TestRecordExecutionEndPersistsSameNormalizedExitCode(t *testing.T) {
	tests := []struct {
		name          string
		status        workflow.ExecutionStatus
		completionErr error
		errorMsg      string
		expectedCode  int
	}{
		{
			name:   "wrapped structured error",
			status: workflow.StatusFailed,
			completionErr: fmt.Errorf("finalize workflow: %w", domainerrors.NewSystemError(
				domainerrors.ErrorCodeSystemIOPermissionDenied,
				"permission denied",
				nil,
				nil,
			)),
			errorMsg:     "finalize workflow: permission denied",
			expectedCode: 4,
		},
		{
			name:          "cancellation",
			status:        workflow.StatusCancelled,
			completionErr: context.Canceled,
			errorMsg:      "workflow cancelled",
			expectedCode:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			historyStore := testmocks.NewMockHistoryStore()
			auditWriter := testmocks.NewMockAuditTrailWriter()
			logger := testmocks.NewMockLogger()
			execSvc := &ExecutionService{
				store:            testmocks.NewMockStateStore(),
				historySvc:       NewHistoryService(historyStore, logger),
				auditTrailWriter: auditWriter,
				logger:           logger,
			}
			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = tt.status
			execCtx.CompletedAt = execCtx.StartedAt.Add(time.Second)

			execSvc.recordExecutionEnd(context.Background(), execCtx, tt.completionErr, tt.errorMsg)

			saved, loadErr := execSvc.store.Load(context.Background(), execCtx.WorkflowID)
			require.NoError(t, loadErr)
			require.NotNil(t, saved)
			assert.Equal(t, tt.expectedCode, saved.ExitCode)
			assert.Equal(t, tt.status, saved.GetStatus())
			assert.Equal(t, execCtx.GetCompletedAt(), saved.GetCompletedAt())

			records, err := historyStore.List(context.Background(), nil)
			require.NoError(t, err)
			require.Len(t, records, 1)
			events := auditWriter.GetEvents()
			require.Len(t, events, 1)
			require.NotNil(t, events[0].ExitCode)
			assert.Equal(t, tt.expectedCode, execCtx.ExitCode)
			assert.Equal(t, tt.expectedCode, records[0].ExitCode)
			assert.Equal(t, tt.expectedCode, *events[0].ExitCode)
			assert.Equal(t, records[0].ExitCode, *events[0].ExitCode)
			assert.Equal(t, tt.errorMsg, events[0].Error)
		})
	}
}

// TestRecordExecutionEndWithMultipleExitCodes tests exit code handling.
func TestRecordExecutionEndWithMultipleExitCodes(t *testing.T) {
	tests := []struct {
		name        string
		exitCode    int
		status      workflow.ExecutionStatus
		expectedMsg string
	}{
		{
			name:     "records exit code 0 for success",
			exitCode: 0,
			status:   workflow.StatusCompleted,
		},
		{
			name:     "records exit code 1 for failure",
			exitCode: 1,
			status:   workflow.StatusFailed,
		},
		{
			name:     "records exit code 127 for command not found",
			exitCode: 127,
			status:   workflow.StatusFailed,
		},
		{
			name:     "records exit code 255 for fatal error",
			exitCode: 255,
			status:   workflow.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				store:            testmocks.NewMockStateStore(),
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = tt.status
			execCtx.ExitCode = tt.exitCode
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, nil, "")

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			assert.Equal(t, tt.exitCode, *event.ExitCode)
		})
	}
}

// TestRecordExecutionEndWithErrorMessages tests error message handling.
func TestRecordExecutionEndWithErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
		hasError bool
	}{
		{
			name:     "records empty error for success",
			errorMsg: "",
			hasError: false,
		},
		{
			name:     "records error message for failure",
			errorMsg: "step 'deploy' failed: connection timeout",
			hasError: true,
		},
		{
			name:     "records complex error message",
			errorMsg: "workflow failed: step 'backup' exited with code 2: insufficient disk space",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				store:            testmocks.NewMockStateStore(),
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = workflow.StatusCompleted
			execCtx.ExitCode = 0
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, nil, tt.errorMsg)

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			if tt.hasError {
				assert.NotEmpty(t, event.Error, "error message should be captured")
			} else {
				assert.Empty(t, event.Error, "error message should be empty for success")
			}
		})
	}
}

// TestRecordExecutionEndWithNilWriter tests that nil writer doesn't cause panics.
func TestRecordExecutionEndWithNilWriter(t *testing.T) {
	execSvc := &ExecutionService{
		store:            testmocks.NewMockStateStore(),
		auditTrailWriter: nil,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0
	ctx := context.Background()

	// Should not panic
	assert.NotPanics(t, func() {
		execSvc.recordExecutionEnd(ctx, execCtx, nil, "")
	})
}

// TestRecordExecutionEndDurationTracking tests duration calculation accuracy.
func TestRecordExecutionEndDurationTracking(t *testing.T) {
	tests := []struct {
		name               string
		timeOffset         time.Duration
		minExpectedDms     int64
		maxExpectedDmsRoom int64
	}{
		{
			name:               "tracks 50ms execution",
			timeOffset:         50 * time.Millisecond,
			minExpectedDms:     30,
			maxExpectedDmsRoom: 100,
		},
		{
			name:               "tracks 100ms execution",
			timeOffset:         100 * time.Millisecond,
			minExpectedDms:     80,
			maxExpectedDmsRoom: 150,
		},
		{
			name:               "tracks 1s execution",
			timeOffset:         1 * time.Second,
			minExpectedDms:     900,
			maxExpectedDmsRoom: 1200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				store:            testmocks.NewMockStateStore(),
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = workflow.StatusCompleted
			execCtx.ExitCode = 0
			execCtx.StartedAt = time.Now().Add(-tt.timeOffset)
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, nil, "")

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			require.NotNil(t, event.DurationMs)
			assert.GreaterOrEqual(t, *event.DurationMs, tt.minExpectedDms,
				"duration should be at least %d ms", tt.minExpectedDms)
		})
	}
}

// TestRecordExecutionEndWithVariousStatuses tests all execution statuses.
func TestRecordExecutionEndWithVariousStatuses(t *testing.T) {
	tests := []struct {
		name           string
		status         workflow.ExecutionStatus
		expectedStatus string
		exitCode       int
	}{
		{
			name:           "maps StatusCompleted to success",
			status:         workflow.StatusCompleted,
			expectedStatus: "success",
			exitCode:       0,
		},
		{
			name:           "maps StatusFailed to failure",
			status:         workflow.StatusFailed,
			expectedStatus: "failure",
			exitCode:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				store:            testmocks.NewMockStateStore(),
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = tt.status
			execCtx.ExitCode = tt.exitCode
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, nil, "")

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			assert.Equal(t, tt.expectedStatus, event.Status)
		})
	}
}

// TestRecordExecutionEndPreservesExecutionID tests that execution ID is preserved.
func TestRecordExecutionEndPreservesExecutionID(t *testing.T) {
	tests := []struct {
		name   string
		execID string
		wfName string
	}{
		{
			name:   "preserves simple execution ID",
			execID: "simple-id",
			wfName: "simple-workflow",
		},
		{
			name:   "preserves UUID execution ID",
			execID: "550e8400-e29b-41d4-a716-446655440000",
			wfName: "uuid-workflow",
		},
		{
			name:   "preserves complex workflow name",
			execID: "test-exec",
			wfName: "deploy-app-with-backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				store:            testmocks.NewMockStateStore(),
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext(tt.execID, tt.wfName)
			execCtx.Status = workflow.StatusCompleted
			execCtx.ExitCode = 0
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, nil, "")

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			assert.Equal(t, tt.execID, event.ExecutionID)
			assert.Equal(t, tt.wfName, event.WorkflowName)
		})
	}
}

// TestRecordExecutionEndWriterFailureHandling tests that writer failures are handled gracefully.
func TestRecordExecutionEndWriterFailureHandling(t *testing.T) {
	tests := []struct {
		name       string
		failOnCall string
	}{
		{
			name:       "handles write failure gracefully",
			failOnCall: "write",
		},
		{
			name:       "handles close failure gracefully",
			failOnCall: "close",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()

			switch tt.failOnCall {
			case "write":
				mockWriter.SetWriteError(errors.New("permission denied"))
			case "close":
				mockWriter.SetCloseError(errors.New("flush failed"))
			}

			execSvc := &ExecutionService{
				store:            testmocks.NewMockStateStore(),
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = workflow.StatusCompleted
			execCtx.ExitCode = 0
			ctx := context.Background()

			// Should not panic or error even though writer fails
			assert.NotPanics(t, func() {
				execSvc.recordExecutionEnd(ctx, execCtx, nil, "")
			})
		})
	}
}

// TestRecordExecutionEndContextCancellation tests behavior with cancelled context.
func TestRecordExecutionEndContextCancellation(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		store:            testmocks.NewMockStateStore(),
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should not panic even with cancelled context
	assert.NotPanics(t, func() {
		execSvc.recordExecutionEnd(ctx, execCtx, nil, "")
	})
}

// TestRecordExecutionEndIntegration tests the helper with multiple scenarios.
func TestRecordExecutionEndIntegration(t *testing.T) {
	tests := []struct {
		name           string
		scenario       string
		initialStatus  workflow.ExecutionStatus
		finalStatus    workflow.ExecutionStatus
		finalExitCode  int
		duration       time.Duration
		expectAuditLog bool
	}{
		{
			name:           "success scenario",
			scenario:       "workflow completes successfully",
			initialStatus:  workflow.StatusRunning,
			finalStatus:    workflow.StatusCompleted,
			finalExitCode:  0,
			duration:       100 * time.Millisecond,
			expectAuditLog: true,
		},
		{
			name:           "failure scenario",
			scenario:       "workflow fails at terminal step",
			initialStatus:  workflow.StatusRunning,
			finalStatus:    workflow.StatusFailed,
			finalExitCode:  3,
			duration:       50 * time.Millisecond,
			expectAuditLog: true,
		},
		{
			name:           "long-running success",
			scenario:       "long execution that completes",
			initialStatus:  workflow.StatusRunning,
			finalStatus:    workflow.StatusCompleted,
			finalExitCode:  0,
			duration:       5 * time.Second,
			expectAuditLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				store:            testmocks.NewMockStateStore(),
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("integration-test-id", "integration-workflow")
			execCtx.Status = tt.initialStatus
			execCtx.StartedAt = time.Now().Add(-tt.duration)
			execCtx.Status = tt.finalStatus
			execCtx.ExitCode = tt.finalExitCode
			execCtx.CompletedAt = time.Now()
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, nil, "")

			if tt.expectAuditLog {
				events := mockWriter.GetEvents()
				require.Len(t, events, 1, "should have one audit event for %s", tt.scenario)

				event := events[0]
				assert.Equal(t, workflow.EventWorkflowCompleted, event.Event)
				assert.Equal(t, "integration-test-id", event.ExecutionID)
				assert.Equal(t, "integration-workflow", event.WorkflowName)
				assert.NotNil(t, event.DurationMs)
				assert.GreaterOrEqual(t, *event.DurationMs, int64(tt.duration.Milliseconds()-50))
			}
		})
	}
}
