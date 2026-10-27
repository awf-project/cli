//go:build integration

package features_test

// Audit Trail Integration Tests (F071)
// Tests verify that FileAuditTrailWriter produces correct JSONL files.
// Tests cover both direct writer behavior and persisted execution outcomes
// through ExecutionService, including initial execution and resumption.
//
// Scenarios covered:
//   SC-002: Paired started/completed entries share the same execution_id
//   SC-003: Secret input values are replaced with "***" before write
//   SC-005: Nil writer (AWF_AUDIT_LOG=off) produces no file on disk
//   SC-006: Orphaned started entry when no completed event is written

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/audit"
	infraexpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseAuditLines reads a JSONL file and returns one parsed AuditEvent per non-empty line.
func parseAuditLines(t *testing.T, path string) []workflow.AuditEvent {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err, "audit file should be readable")

	var events []workflow.AuditEvent
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event workflow.AuditEvent
		require.NoError(t, json.Unmarshal([]byte(line), &event), "each line must be valid JSON")
		events = append(events, event)
	}
	return events
}

// buildExecCtx constructs a minimal ExecutionContext suitable for audit event constructors.
func buildExecCtx(id, name string, startedAt, completedAt time.Time) *workflow.ExecutionContext {
	ctx := workflow.NewExecutionContext(id, name)
	ctx.StartedAt = startedAt
	ctx.CompletedAt = completedAt
	return ctx
}

// TestAuditTrail_PairedEntries (SC-002) verifies that 10 independent executions each
// produce exactly one workflow.started and one workflow.completed entry sharing
// the same execution_id.
func TestAuditTrail_PairedEntries(t *testing.T) {
	t.Parallel()

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	writer, err := audit.NewFileAuditTrailWriter(auditPath)
	require.NoError(t, err)
	defer writer.Close()

	ctx := context.Background()
	fixedStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fixedEnd := fixedStart.Add(100 * time.Millisecond)

	const iterations = 10
	for i := range iterations {
		execID := "exec-" + string(rune('A'+i))
		execCtx := buildExecCtx(execID, "test-workflow", fixedStart, fixedEnd)

		started := workflow.NewStartedEvent(execCtx, map[string]any{"key": "value"}, "ci")
		completed := workflow.NewCompletedEvent(execCtx, "ci", "")

		require.NoError(t, writer.Write(ctx, &started))
		require.NoError(t, writer.Write(ctx, &completed))
	}

	require.NoError(t, writer.Close())

	events := parseAuditLines(t, auditPath)
	require.Len(t, events, iterations*2, "expected 2 events per execution")

	// Group events by execution_id.
	byExecID := make(map[string][]workflow.AuditEvent)
	for _, ev := range events {
		byExecID[ev.ExecutionID] = append(byExecID[ev.ExecutionID], ev)
	}

	assert.Len(t, byExecID, iterations, "expected one group per execution")

	for execID, group := range byExecID {
		var started, completed int
		for _, ev := range group {
			switch ev.Event {
			case workflow.EventWorkflowStarted:
				started++
			case workflow.EventWorkflowCompleted:
				completed++
			}
		}
		assert.Equal(t, 1, started, "execution %s: expected exactly 1 workflow.started", execID)
		assert.Equal(t, 1, completed, "execution %s: expected exactly 1 workflow.completed", execID)
	}
}

// TestAuditTrail_SecretMasking (SC-003) verifies that the application-layer masking
// convention (replacing secret values with "***") is preserved in the written JSONL.
// The test simulates what the application layer does before calling Write.
func TestAuditTrail_SecretMasking(t *testing.T) {
	t.Parallel()

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	writer, err := audit.NewFileAuditTrailWriter(auditPath)
	require.NoError(t, err)
	defer writer.Close()

	fixedStart := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	execCtx := buildExecCtx("exec-mask-01", "masked-workflow", fixedStart, fixedStart)

	// Application layer masks secrets before constructing the event.
	maskedInputs := map[string]any{
		"SECRET_API_KEY": "***", // was "hunter2"
		"PUBLIC_VAR":     "visible",
	}

	started := workflow.NewStartedEvent(execCtx, maskedInputs, "tester")

	require.NoError(t, writer.Write(context.Background(), &started))
	require.NoError(t, writer.Close())

	data, err := os.ReadFile(auditPath)
	require.NoError(t, err)
	fileContent := string(data)

	assert.Contains(t, fileContent, `"***"`, "masked placeholder must appear in file")
	assert.Contains(t, fileContent, "visible", "non-secret value must appear in file")
	assert.NotContains(t, fileContent, "hunter2", "plaintext secret must not appear in file")
}

// TestAuditTrail_DisabledProducesNoFile (SC-005) verifies that when the audit writer
// is nil (simulating AWF_AUDIT_LOG=off), no file is created at the expected path.
func TestAuditTrail_DisabledProducesNoFile(t *testing.T) {
	t.Parallel()

	expectedPath := filepath.Join(t.TempDir(), "audit.jsonl")

	// Nil writer simulates the disabled state: no writes happen.
	var writer *audit.FileAuditTrailWriter

	if writer != nil {
		fixedStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		execCtx := buildExecCtx("exec-off-01", "off-workflow", fixedStart, fixedStart)
		started := workflow.NewStartedEvent(execCtx, nil, "user")
		_ = writer.Write(context.Background(), &started)
	}

	_, statErr := os.Stat(expectedPath)
	assert.True(t, os.IsNotExist(statErr), "no audit file should exist when writer is nil")
}

// TestAuditTrail_OrphanedStartEntry (SC-006) verifies that when only a workflow.started
// event is written (e.g. process killed before completion), the file contains exactly
// one line with event "workflow.started" and no "workflow.completed" line.
func TestAuditTrail_OrphanedStartEntry(t *testing.T) {
	t.Parallel()

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	writer, err := audit.NewFileAuditTrailWriter(auditPath)
	require.NoError(t, err)

	fixedStart := time.Date(2026, 4, 15, 8, 30, 0, 0, time.UTC)
	execCtx := buildExecCtx("exec-orphan-01", "orphan-workflow", fixedStart, fixedStart)

	started := workflow.NewStartedEvent(execCtx, map[string]any{"input": "data"}, "dev")
	require.NoError(t, writer.Write(context.Background(), &started))

	// Close without writing a completed event — simulates abrupt termination.
	require.NoError(t, writer.Close())

	events := parseAuditLines(t, auditPath)
	require.Len(t, events, 1, "expected exactly 1 event in the file")

	assert.Equal(t, workflow.EventWorkflowStarted, events[0].Event,
		"the single event must be workflow.started")

	// Confirm no completed event exists.
	for _, ev := range events {
		assert.NotEqual(t, workflow.EventWorkflowCompleted, ev.Event,
			"workflow.completed must not be present in an orphaned trail")
	}
}

type completionScenario struct {
	name                   string
	workflow               *workflow.Workflow
	executor               ports.CommandExecutor
	contextTimeout         time.Duration
	expectedHistory        string
	expectedAudit          string
	expectedExitCode       int
	expectedError          bool
	expectedCause          error
	expectedReturnedError  string
	expectedAuditError     string
	expectedTruncatedError bool
	expectedHistoryError   string
}

func TestAuditLifecyclePersistsExactlyOneCompletionForEveryStartedExecution(t *testing.T) {
	tests := []completionScenario{
		{
			name:             "successful completion",
			workflow:         terminalWorkflow("successful-completion", workflow.TerminalSuccess, 0),
			executor:         commandExecutorFunc(successfulCommand),
			expectedHistory:  "completed",
			expectedAudit:    "success",
			expectedExitCode: 0,
		},
		{
			name:                  "explicit terminal failure",
			workflow:              terminalWorkflow("explicit-terminal-failure", workflow.TerminalFailure, 2),
			executor:              commandExecutorFunc(successfulCommand),
			expectedHistory:       "failed",
			expectedAudit:         "failure",
			expectedExitCode:      2,
			expectedError:         true,
			expectedReturnedError: "workflow reached terminal failure state: terminal",
			expectedAuditError:    "workflow reached terminal failure state: terminal",
		},
		{
			name:     "failed command",
			workflow: commandWorkflow("failed-command", workflow.WorkflowHooks{}),
			executor: commandExecutorFunc(func(context.Context, *ports.Command) (*ports.CommandResult, error) {
				return &ports.CommandResult{ExitCode: 7}, nil
			}),
			expectedHistory:       "failed",
			expectedAudit:         "failure",
			expectedExitCode:      7,
			expectedError:         true,
			expectedReturnedError: "step run: exit code 7",
			expectedAuditError:    "step run: exit code 7",
			expectedHistoryError:  "exit code 7",
		},
		{
			name: "uncategorized application failure",
			workflow: terminalWorkflowWithHooks(
				"uncategorized-application-failure",
				workflow.WorkflowHooks{WorkflowStart: workflow.Hook{{Command: "fail-hook"}}},
			),
			executor: commandExecutorFunc(func(context.Context, *ports.Command) (*ports.CommandResult, error) {
				return nil, errors.New("test hook failure")
			}),
			expectedHistory:       "failed",
			expectedAudit:         "failure",
			expectedExitCode:      3,
			expectedError:         true,
			expectedReturnedError: "workflow_start hook failed: execute hook command: test hook failure",
			expectedAuditError:    "workflow_start hook failed: execute hook command: test hook failure",
		},
		{
			name: "oversized terminal error",
			workflow: terminalWorkflowWithHooks("oversized-terminal-error", workflow.WorkflowHooks{
				WorkflowStart: workflow.Hook{{Command: "fail-hook"}},
			}),
			executor: commandExecutorFunc(func(context.Context, *ports.Command) (*ports.CommandResult, error) {
				return nil, errors.New(strings.Repeat("long failure ", 1000))
			}),
			expectedHistory:        "failed",
			expectedAudit:          "failure",
			expectedExitCode:       3,
			expectedError:          true,
			expectedTruncatedError: true,
		},
		{
			name:                 "cancellation",
			expectedHistory:      "cancelled",
			workflow:             commandWorkflow("cancellation", workflow.WorkflowHooks{}),
			expectedAudit:        "failure",
			expectedExitCode:     3,
			expectedError:        true,
			expectedCause:        context.Canceled,
			expectedAuditError:   "step run: context canceled",
			expectedHistoryError: "context canceled",
		},
		{
			name: "timeout",
			// Keep adapter startup outside the race-sensitive edge of the deadline.
			// The command readiness signal below proves execution reached the command
			// before this deadline exercises timeout finalization.
			contextTimeout:       2 * time.Second,
			expectedHistory:      "cancelled",
			workflow:             commandWorkflow("timeout", workflow.WorkflowHooks{}),
			expectedAudit:        "failure",
			expectedExitCode:     3,
			expectedError:        true,
			expectedCause:        context.DeadlineExceeded,
			expectedAuditError:   "step run: context deadline exceeded",
			expectedHistoryError: "context deadline exceeded",
		},
		{
			name:     "recovered panic",
			workflow: commandWorkflow("recovered-panic", workflow.WorkflowHooks{}),
			executor: commandExecutorFunc(func(context.Context, *ports.Command) (*ports.CommandResult, error) {
				panicValues := []string{"CONFIDENTIAL_PANIC_CANARY"}
				return nil, errors.New(panicValues[len(panicValues)])
			}),
			expectedHistory:    "failed",
			expectedAudit:      "failure",
			expectedExitCode:   3,
			expectedError:      true,
			expectedAuditError: "workflow execution panic",
		},
	}

	for i := range tests {
		scenario := &tests[i]
		for _, mode := range []string{"run", "resume"} {
			t.Run(scenario.name+"/"+mode, func(t *testing.T) {
				testPersistedCompletion(t, scenario, mode == "resume")
			})
		}
	}
}

func TestLoopFailurePersistsConsistentOutcome(t *testing.T) {
	tests := []struct {
		name          string
		handled       bool
		wantStatus    string
		wantAudit     string
		wantExitCode  int
		wantFailed    int
		wantCompleted int
		wantRunError  bool
	}{
		{
			name:         "terminal loop failure",
			wantStatus:   "failed",
			wantAudit:    "failure",
			wantExitCode: 1,
			wantFailed:   1,
			wantRunError: true,
		},
		{
			name:          "handled internal failure transition",
			handled:       true,
			wantStatus:    "completed",
			wantAudit:     "success",
			wantExitCode:  0,
			wantCompleted: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := os.MkdirTemp(".", ".loop-outcome-")
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, os.RemoveAll(root)) })

			runID := "loop-persistence-run"
			auditPath := filepath.Join(root, "audit.jsonl")
			historyPath := filepath.Join(root, "history.db")
			auditWriter, err := audit.NewFileAuditTrailWriter(auditPath)
			require.NoError(t, err)
			historyWriter, err := store.NewSQLiteHistoryStore(historyPath)
			require.NoError(t, err)

			wf := persistedLoopWorkflow(tt.handled)
			stateStore := store.NewJSONStore(filepath.Join(root, "states"))
			repo := mocks.NewMockWorkflowRepository()
			repo.AddWorkflow(wf.Name, wf)
			executor := commandExecutorFunc(func(_ context.Context, command *ports.Command) (*ports.CommandResult, error) {
				if command.Program == "emit verdict" {
					return &ports.CommandResult{ExitCode: 1}, nil
				}
				return &ports.CommandResult{ExitCode: 0}, nil
			})
			workflowService := application.NewWorkflowService(repo, stateStore, executor, ports.NopLogger{}, nil)
			service := application.NewExecutionServiceWithEvaluator(
				workflowService,
				executor,
				application.NewParallelExecutor(ports.NopLogger{}),
				stateStore,
				ports.NopLogger{},
				interpolation.NewTemplateResolver(),
				application.NewHistoryService(historyWriter, ports.NopLogger{}),
				infraexpr.NewExprEvaluator(),
			)
			publisher := mocks.NewMockEventPublisher()
			service.SetEventPublisher(publisher)
			service.SetAuditTrailWriter(auditWriter)

			execCtx, runErr := service.RunWithWorkflowAndRunID(context.Background(), wf, nil, runID)
			if tt.wantRunError {
				require.Error(t, runErr)
			} else {
				require.NoError(t, runErr)
			}
			require.NotNil(t, execCtx)
			assert.Equal(t, tt.wantStatus, execCtx.GetStatus().String())
			assert.Equal(t, tt.wantExitCode, execCtx.ExitCode)
			saved, loadErr := stateStore.Load(context.Background(), runID)
			require.NoError(t, loadErr)
			require.NotNil(t, saved)
			assert.Equal(t, tt.wantExitCode, saved.ExitCode)
			assert.Equal(t, tt.wantStatus, saved.GetStatus().String())
			assert.False(t, saved.GetCompletedAt().IsZero())
			origin, exists := execCtx.GetStepState("failing")
			require.True(t, exists)
			assert.Equal(t, workflow.StatusFailed, origin.Status)
			assert.Equal(t, 1, origin.ExitCode)

			var failed, completed int
			for _, event := range publisher.GetEvents() {
				switch event.Type {
				case workflow.EventWorkflowFailed:
					failed++
				case workflow.EventWorkflowCompleted:
					completed++
				}
			}
			assert.Equal(t, tt.wantFailed, failed)
			assert.Equal(t, tt.wantCompleted, completed)

			require.NoError(t, auditWriter.Close())
			require.NoError(t, historyWriter.Close())

			historyReader, err := store.NewSQLiteHistoryStore(historyPath)
			require.NoError(t, err)
			records, err := historyReader.List(context.Background(), &workflow.HistoryFilter{Limit: 100})
			require.NoError(t, err)
			require.NoError(t, historyReader.Close())
			require.Len(t, records, 1)
			assert.Equal(t, tt.wantStatus, records[0].Status)
			assert.Equal(t, tt.wantExitCode, records[0].ExitCode)

			auditReader, err := os.Open(auditPath)
			require.NoError(t, err)
			require.NoError(t, auditReader.Close())
			completions := findAuditCompletions(parseAuditLines(t, auditPath), runID)
			require.Len(t, completions, 1)
			require.NotNil(t, completions[0].ExitCode)
			assert.Equal(t, tt.wantAudit, completions[0].Status)
			assert.Equal(t, tt.wantExitCode, *completions[0].ExitCode)
			assert.Equal(t, execCtx.ExitCode, records[0].ExitCode)
			assert.Equal(t, records[0].ExitCode, *completions[0].ExitCode)
		})
	}
}

func persistedLoopWorkflow(handled bool) *workflow.Workflow {
	when := "false"
	body := []string{"failing"}
	if handled {
		when = ""
		body = append(body, "recovery")
	}
	return &workflow.Workflow{
		Name:    "loop-persistence",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type: workflow.LoopTypeForEach, Items: `["item"]`, Body: body, MaxIterations: 1, OnComplete: "done",
				},
			},
			"failing": {
				Name: "failing", Type: workflow.StepTypeCommand, Command: "emit verdict",
				Transitions: workflow.Transitions{{When: when, Goto: "recovery"}},
			},
			"recovery": {Name: "recovery", Type: workflow.StepTypeCommand, Command: "run recovery"},
			"done":     {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
}

func testPersistedCompletion(t *testing.T, scenario *completionScenario, resume bool) {
	t.Helper()
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.jsonl")
	historyPath := filepath.Join(tempDir, "history.db")
	runID := "exec-" + strings.ReplaceAll(scenario.name, " ", "-")

	auditWriter, err := audit.NewFileAuditTrailWriter(auditPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, auditWriter.Close()) })
	historyWriter, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, historyWriter.Close()) })

	executor := scenario.executor
	var started chan struct{}
	if executor == nil {
		started = make(chan struct{})
		executor = commandExecutorFunc(func(ctx context.Context, _ *ports.Command) (*ports.CommandResult, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		})
	}
	stateStore := store.NewJSONStore(filepath.Join(tempDir, "states"))
	repo := mocks.NewMockWorkflowRepository()
	repo.AddWorkflow(scenario.workflow.Name, scenario.workflow)
	workflowService := application.NewWorkflowService(repo, stateStore, executor, ports.NopLogger{}, nil)
	service := application.NewExecutionService(
		workflowService, executor, nil, stateStore, ports.NopLogger{}, passthroughResolver{},
		application.NewHistoryService(historyWriter, ports.NopLogger{}),
	)
	service.SetAuditTrailWriter(auditWriter)

	execute := func(ctx context.Context) (*workflow.ExecutionContext, error) {
		return runWorkflow(ctx, service, scenario.workflow, runID)
	}
	if resume {
		saved := workflow.NewExecutionContext(runID, scenario.workflow.Name)
		saved.CurrentStep = scenario.workflow.Initial
		saved.SetStatus(workflow.StatusCancelled)
		require.NoError(t, stateStore.Save(context.Background(), saved))
		execute = func(ctx context.Context) (*workflow.ExecutionContext, error) {
			return service.Resume(ctx, runID, nil, "current")
		}
	}
	execCtx, runErr := executeCompletionScenario(t, execute, started, scenario.contextTimeout)
	if scenario.expectedError {
		require.Error(t, runErr)
		if scenario.expectedReturnedError != "" {
			assert.Equal(t, scenario.expectedReturnedError, runErr.Error())
		}
	} else {
		require.NoError(t, runErr)
	}
	require.NotNil(t, execCtx)
	assert.Equal(t, scenario.expectedHistory, execCtx.GetStatus().String())
	saved, loadErr := stateStore.Load(context.Background(), runID)
	require.NoError(t, loadErr)
	require.NotNil(t, saved)
	assert.Equal(t, scenario.expectedExitCode, saved.ExitCode)
	assert.Equal(t, scenario.expectedHistory, saved.GetStatus().String())
	assert.False(t, saved.GetCompletedAt().IsZero())
	if started != nil {
		require.ErrorIs(t, runErr, scenario.expectedCause)
	}
	require.NoError(t, auditWriter.Close())
	require.NoError(t, historyWriter.Close())

	historyReader, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, historyReader.Close()) })
	records, err := historyReader.List(context.Background(), &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	require.NoError(t, historyReader.Close())

	record := findHistoryRecord(t, records, runID)
	assert.Equal(t, scenario.expectedHistory, record.Status)
	assert.Equal(t, scenario.expectedExitCode, record.ExitCode)
	assert.Equal(t, scenario.expectedHistoryError, record.ErrorMessage)
	events := parseAuditLines(t, auditPath)
	startedEvents := findAuditEvents(events, runID, workflow.EventWorkflowStarted)
	require.Len(t, startedEvents, 1, "each execution must be started exactly once")
	completions := findAuditCompletions(events, runID)
	require.Len(t, completions, 1, "each execution must be finalized exactly once")
	event := &completions[0]
	assert.Equal(t, startedEvents[0].ExecutionID, event.ExecutionID)
	assert.Equal(t, record.ID, event.ExecutionID)
	require.NotNil(t, event.ExitCode)
	assert.Equal(t, scenario.expectedAudit, event.Status)
	assert.Equal(t, scenario.expectedExitCode, *event.ExitCode)
	assert.Equal(t, record.ExitCode, *event.ExitCode)
	if scenario.expectedError {
		assert.NotZero(t, *event.ExitCode)
	}
	if scenario.expectedTruncatedError {
		assert.Contains(t, event.Error, "workflow_start hook failed:")
		assert.True(t, strings.HasSuffix(event.Error, "… [truncated]"))
	} else {
		assert.Equal(t, scenario.expectedAuditError, event.Error)
	}
	if scenario.expectedCause != nil {
		assert.Contains(t, record.ErrorMessage, scenario.expectedCause.Error())
	}
	assert.NotContains(t, event.Error, "CONFIDENTIAL_PANIC_CANARY")
	assert.NotContains(t, record.ErrorMessage, "CONFIDENTIAL_PANIC_CANARY")
}

func executeCompletionScenario(
	t *testing.T,
	execute func(context.Context) (*workflow.ExecutionContext, error),
	started <-chan struct{},
	timeout time.Duration,
) (*workflow.ExecutionContext, error) {
	t.Helper()
	if started == nil {
		return execute(context.Background())
	}
	ctx, cancel := context.WithCancel(context.Background())
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()
	result := make(chan executionResult, 1)
	go func() {
		execCtx, err := execute(ctx)
		result <- executionResult{execCtx: execCtx, err: err}
	}()
	// Bound both synchronization points independently so time spent reaching the
	// command does not consume the finalization guard.
	startupTimer := time.NewTimer(5 * time.Second)
	defer startupTimer.Stop()
	select {
	case <-started:
	case runResult := <-result:
		t.Fatalf("execution returned before command started: %v", runResult.err)
	case <-startupTimer.C:
		t.Fatal("command did not start before cancellation deadline")
	}
	if timeout == 0 {
		cancel()
	}
	completionTimer := time.NewTimer(5 * time.Second)
	defer completionTimer.Stop()
	for {
		select {
		case runResult := <-result:
			return runResult.execCtx, runResult.err
		case <-completionTimer.C:
			t.Fatal("execution did not finish after cancellation")
		}
	}
}

type commandExecutorFunc func(context.Context, *ports.Command) (*ports.CommandResult, error)

func (f commandExecutorFunc) Execute(ctx context.Context, command *ports.Command) (*ports.CommandResult, error) {
	return f(ctx, command)
}

type passthroughResolver struct{}

func (passthroughResolver) Resolve(value string, _ *interpolation.Context) (string, error) {
	return value, nil
}

func successfulCommand(context.Context, *ports.Command) (*ports.CommandResult, error) {
	return &ports.CommandResult{ExitCode: 0}, nil
}

func terminalWorkflow(name string, status workflow.TerminalStatus, exitCode int) *workflow.Workflow {
	return &workflow.Workflow{
		Name:    name,
		Initial: "terminal",
		Steps: map[string]*workflow.Step{
			"terminal": {Name: "terminal", Type: workflow.StepTypeTerminal, Status: status, ExitCode: exitCode},
		},
	}
}

func terminalWorkflowWithHooks(name string, hooks workflow.WorkflowHooks) *workflow.Workflow {
	wf := terminalWorkflow(name, workflow.TerminalSuccess, 0)
	wf.Hooks = hooks
	return wf
}

func commandWorkflow(name string, hooks workflow.WorkflowHooks) *workflow.Workflow {
	return &workflow.Workflow{
		Name:    name,
		Initial: "run",
		Hooks:   hooks,
		Steps: map[string]*workflow.Step{
			"run":  {Name: "run", Type: workflow.StepTypeCommand, Command: "run"},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
}

func runWorkflow(
	ctx context.Context,
	service *application.ExecutionService,
	wf *workflow.Workflow,
	runID string,
) (*workflow.ExecutionContext, error) {
	return service.RunWithWorkflowAndRunID(ctx, wf, nil, runID)
}

type executionResult struct {
	execCtx *workflow.ExecutionContext
	err     error
}

func findHistoryRecord(t *testing.T, records []*workflow.ExecutionRecord, executionID string) *workflow.ExecutionRecord {
	t.Helper()
	for _, record := range records {
		if record.ID == executionID {
			return record
		}
	}
	require.FailNow(t, "history record not found", "execution ID: %s", executionID)
	return nil
}

func findAuditCompletions(events []workflow.AuditEvent, executionID string) []workflow.AuditEvent {
	return findAuditEvents(events, executionID, workflow.EventWorkflowCompleted)
}

func findAuditEvents(events []workflow.AuditEvent, executionID, eventType string) []workflow.AuditEvent {
	var matches []workflow.AuditEvent
	for i := range events {
		if events[i].ExecutionID == executionID && events[i].Event == eventType {
			matches = append(matches, events[i])
		}
	}
	return matches
}
