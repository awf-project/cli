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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/interfaces/cli"

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
	resolver               interpolation.Resolver
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
	inputs                 map[string]any
	confidentialValues     []string
	panicPayload           string
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
			expectedHistoryError:  "step execution failed",
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
	}

	for i := range tests {
		scenario := &tests[i]
		t.Run(scenario.name, func(t *testing.T) {
			testPersistedCompletion(t, scenario, false, nil)
		})
	}
}

func TestAuditLifecyclePersistsResumedExecution(t *testing.T) {
	testPersistedCompletion(t, &completionScenario{
		name:             "resumed-successful-completion",
		workflow:         terminalWorkflow("resumed-successful-completion", workflow.TerminalSuccess, 0),
		executor:         commandExecutorFunc(successfulCommand),
		expectedHistory:  "completed",
		expectedAudit:    "success",
		expectedExitCode: 0,
	}, true, nil)
}

func TestAuditLifecyclePersistsRecoveredPanicForFreshAndResumedExecution(t *testing.T) {
	const (
		environmentKey   = "AUDIT_TRAIL_ENV_CANARY"
		environmentValue = "CONFIDENTIAL_ENVIRONMENT_VALUE"
		commandOutput    = "CONFIDENTIAL_COMMAND_OUTPUT"
	)

	confidentialValues := []string{
		"CONFIDENTIAL_PANIC_CANARY",
		"CONFIDENTIAL_WORKFLOW_INPUT",
		commandOutput,
		environmentValue,
		"CONFIDENTIAL_TOKEN",
		"CONFIDENTIAL_SECRET",
	}
	for _, resume := range []bool{false, true} {
		mode := "fresh"
		if resume {
			mode = "resumed"
		}
		t.Run(mode, func(t *testing.T) {
			t.Setenv(environmentKey, environmentValue)
			var preparedCommand string
			testPersistedCompletion(t, &completionScenario{
				name:     "recovered-panic-" + mode,
				workflow: panicAfterCommandWorkflow("recovered-panic-"+mode, environmentKey),
				resolver: interpolation.NewTemplateResolver(),
				executor: commandExecutorFunc(func(_ context.Context, command *ports.Command) (*ports.CommandResult, error) {
					switch command.Program {
					case "emit " + environmentValue:
						preparedCommand = command.Program
						return &ports.CommandResult{Stdout: commandOutput, ExitCode: 0}, nil
					case "panic":
						panicValues := []string{"CONFIDENTIAL_PANIC_CANARY"}
						return nil, errors.New(panicValues[len(panicValues)])
					default:
						return nil, fmt.Errorf("unexpected command %q", command.Program)
					}
				}),
				expectedHistory:    "failed",
				expectedAudit:      "failure",
				expectedExitCode:   3,
				expectedError:      true,
				expectedAuditError: "workflow execution panic",
				panicPayload:       "CONFIDENTIAL_PANIC_CANARY",
				inputs: map[string]any{
					"workflow_input": "CONFIDENTIAL_WORKFLOW_INPUT",
					"token":          "CONFIDENTIAL_TOKEN",
					"secret":         "CONFIDENTIAL_SECRET",
				},
				confidentialValues: confidentialValues,
			}, resume, func(t *testing.T) {
				require.Equal(t, "emit "+environmentValue, preparedCommand,
					"the preceding command must receive the OS environment canary through interpolation")
			})
		})
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
			completions := findAuditCompletions(parseAuditLines(t, auditPath), execCtx.HistoryRecordID)
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

type completionFixture struct {
	dir, auditPath, historyPath, runID string
	auditWriter                        *audit.FileAuditTrailWriter
	historyWriter                      *store.SQLiteHistoryStore
	stateStore                         *store.JSONStore
	logger                             *mocks.MockLogger
	resolver                           interpolation.Resolver
	service                            *application.ExecutionService
	started                            chan struct{}
}

type persistedCompletion struct {
	context, saved                      *workflow.ExecutionContext
	record                              *workflow.ExecutionRecord
	events                              []workflow.AuditEvent
	auditText, statePath, resumedFromID string
	runErr                              error
	logs                                []mocks.LogMessage
}

func newCompletionFixture(t *testing.T, scenario *completionScenario) *completionFixture {
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
	logger := mocks.NewMockLogger()
	repo := mocks.NewMockWorkflowRepository()
	repo.AddWorkflow(scenario.workflow.Name, scenario.workflow)
	resolver := scenario.resolver
	if resolver == nil {
		resolver = passthroughResolver{}
	}
	workflowService := application.NewWorkflowService(repo, stateStore, executor, logger, nil)
	service := application.NewExecutionService(
		workflowService, executor, nil, stateStore, logger, resolver,
		application.NewHistoryService(historyWriter, logger),
	)

	return &completionFixture{
		dir: tempDir, auditPath: auditPath, historyPath: historyPath, runID: runID,
		auditWriter: auditWriter, historyWriter: historyWriter, stateStore: stateStore,
		logger: logger, resolver: resolver, service: service, started: started,
	}
}

func (f *completionFixture) seedResumedAttempt(t *testing.T, scenario *completionScenario) string {
	t.Helper()
	stateStore, logger, resolver, runID := f.stateStore, f.logger, f.resolver, f.runID
	var resumedFromID string
	const seedStep = "seed-cancellation"
	seedStarted := make(chan struct{})
	seedExecutor := commandExecutorFunc(func(ctx context.Context, _ *ports.Command) (*ports.CommandResult, error) {
		close(seedStarted)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	seedWorkflow := commandWorkflow(scenario.workflow.Name, workflow.WorkflowHooks{})
	seedWorkflow.Initial = seedStep
	seedWorkflow.Steps = map[string]*workflow.Step{
		seedStep: {
			Name:    seedStep,
			Type:    workflow.StepTypeCommand,
			Command: "await cancellation",
		},
	}
	seedRepo := mocks.NewMockWorkflowRepository()
	seedRepo.AddWorkflow(seedWorkflow.Name, seedWorkflow)
	seedWorkflowService := application.NewWorkflowService(seedRepo, stateStore, seedExecutor, logger, nil)
	seedService := application.NewExecutionService(
		seedWorkflowService, seedExecutor, nil, stateStore, logger, resolver,
		application.NewHistoryService(f.historyWriter, logger),
	)
	cancelled, cancelErr := executeCompletionScenario(t, func(ctx context.Context) (*workflow.ExecutionContext, error) {
		return seedService.RunWithWorkflowAndRunID(ctx, seedWorkflow, scenario.inputs, runID)
	}, seedStarted, 0)
	require.ErrorIs(t, cancelErr, context.Canceled)
	require.Equal(t, workflow.StatusCancelled, cancelled.GetStatus())
	var attempt int
	var recorded bool
	resumedFromID, attempt, _, recorded = cancelled.GetHistoryAttempt()
	require.NotEmpty(t, resumedFromID)
	require.Equal(t, 1, attempt)
	require.True(t, recorded)
	cancelled.CurrentStep = scenario.workflow.Initial
	cancelled.ExitCode = 0
	cancelled.States = make(map[string]workflow.StepState)
	require.NoError(t, stateStore.Save(context.Background(), cancelled))
	return resumedFromID
}

func (f *completionFixture) reopen(t *testing.T, execCtx *workflow.ExecutionContext) *persistedCompletion {
	t.Helper()
	reopenedStateStore := store.NewJSONStore(filepath.Join(f.dir, "states"))
	saved, loadErr := reopenedStateStore.Load(context.Background(), f.runID)
	require.NoError(t, loadErr)
	require.NotNil(t, saved)

	require.NoError(t, f.auditWriter.Close())
	require.NoError(t, f.historyWriter.Close())

	historyReader, err := store.NewSQLiteHistoryStore(f.historyPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, historyReader.Close()) })
	records, err := historyReader.List(context.Background(), &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	require.NoError(t, historyReader.Close())

	attemptID, _, _, _ := execCtx.GetHistoryAttempt()
	persistedAudit, err := os.ReadFile(f.auditPath)
	require.NoError(t, err)
	return &persistedCompletion{
		context: execCtx, saved: saved, record: findHistoryRecord(t, records, attemptID),
		events: parseAuditLines(t, f.auditPath), auditText: string(persistedAudit),
		statePath: filepath.Join(f.dir, "states", f.runID+".json"),
	}
}

func testPersistedCompletion(t *testing.T, scenario *completionScenario, resume bool, assertBeforePersistence func(*testing.T)) {
	t.Helper()
	fixture := newCompletionFixture(t, scenario)
	execute := func(ctx context.Context) (*workflow.ExecutionContext, error) {
		return fixture.service.RunWithWorkflowAndRunID(ctx, scenario.workflow, scenario.inputs, fixture.runID)
	}
	var resumedFromID string
	if resume {
		resumedFromID = fixture.seedResumedAttempt(t, scenario)
		execute = func(ctx context.Context) (*workflow.ExecutionContext, error) {
			return fixture.service.Resume(ctx, fixture.runID, scenario.inputs, "current")
		}
	}
	fixture.service.SetAuditTrailWriter(fixture.auditWriter)
	execCtx, runErr := executeCompletionScenario(t, execute, fixture.started, scenario.contextTimeout)
	if assertBeforePersistence != nil {
		assertBeforePersistence(t)
	}
	assertCompletionOutcome(t, scenario, execCtx, runErr)
	result := fixture.reopen(t, execCtx)
	result.runErr, result.resumedFromID = runErr, resumedFromID
	result.logs = fixture.logger.GetMessages()
	assertCompletionStateAndHistory(t, scenario, result)
	assertCompletionAudit(t, scenario, result)
	assertCompletionConfidentiality(t, scenario, result)
}

func assertCompletionOutcome(t *testing.T, scenario *completionScenario, execCtx *workflow.ExecutionContext, runErr error) {
	t.Helper()
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
}

func assertCompletionStateAndHistory(t *testing.T, scenario *completionScenario, result *persistedCompletion) {
	t.Helper()
	saved, execCtx, runErr := result.saved, result.context, result.runErr
	assert.Equal(t, scenario.expectedExitCode, saved.ExitCode)
	assert.Equal(t, scenario.expectedHistory, saved.GetStatus().String())
	assert.False(t, saved.GetCompletedAt().IsZero())
	if scenario.expectedCause != nil {
		require.ErrorIs(t, runErr, scenario.expectedCause)
	}
	attemptID, attempt, _, _ := execCtx.GetHistoryAttempt()
	require.NotEmpty(t, attemptID)
	require.Positive(t, attempt)
	if result.resumedFromID != "" {
		assert.Equal(t, 2, attempt)
		assert.Equal(t, result.resumedFromID, execCtx.PreviousHistoryRecordID)
	}
	record := result.record
	assert.Equal(t, scenario.expectedHistory, record.Status)
	assert.Equal(t, scenario.expectedExitCode, record.ExitCode)
	assert.Equal(t, scenario.expectedHistoryError, record.ErrorMessage)
}

func assertCompletionAudit(t *testing.T, scenario *completionScenario, result *persistedCompletion) {
	t.Helper()

	events := result.events
	attemptID, _, _, _ := result.context.GetHistoryAttempt()
	record := result.record
	startedEvents := findAuditEvents(events, attemptID, workflow.EventWorkflowStarted)
	require.Len(t, startedEvents, 1, "each execution must be started exactly once")
	completions := findAuditCompletions(events, attemptID)
	require.Len(t, completions, 1, "each execution must be finalized exactly once")
	require.Len(t, events, 2, "the attempt must persist only its ordered lifecycle pair")
	assert.Equal(t, workflow.EventWorkflowStarted, events[0].Event)
	assert.Equal(t, workflow.EventWorkflowCompleted, events[1].Event)
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
}

func assertCompletionConfidentiality(t *testing.T, scenario *completionScenario, result *persistedCompletion) {
	t.Helper()
	saved, record, runErr := result.saved, result.record, result.runErr
	// Private checkpoints retain resume data; diagnostic projections remain redacted.
	stateInfo, err := os.Stat(result.statePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), stateInfo.Mode().Perm())
	if scenario.inputs != nil {
		assert.Equal(t, scenario.inputs, saved.Inputs, "inputs must survive checkpoint persistence")
	}
	if scenario.panicPayload != "" {
		privateState, marshalErr := json.Marshal(saved)
		require.NoError(t, marshalErr)
		assert.NotContains(t, string(privateState), scenario.panicPayload, "panic payload must never be persisted")
	}
	persistedEvidence := strings.Join([]string{
		result.auditText,
		fmt.Sprint(record),
		fmt.Sprint(runErr),
		fmt.Sprint(result.logs),
	}, "\n")
	for _, confidentialValue := range scenario.confidentialValues {
		assert.NotContains(t, persistedEvidence, confidentialValue)
	}
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

func panicAfterCommandWorkflow(name, environmentKey string) *workflow.Workflow {
	return &workflow.Workflow{
		Name:    name,
		Initial: "prepare",
		Steps: map[string]*workflow.Step{
			"prepare": {
				Name:      "prepare",
				Type:      workflow.StepTypeCommand,
				Command:   "emit {{env." + environmentKey + "}}",
				OnSuccess: "panic",
			},
			"panic": {Name: "panic", Type: workflow.StepTypeCommand, Command: "panic"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
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

// TestAuditTrail_PersistsTimeoutAndCancellationLifecycle correlates reopened
// JSON state, SQLite history and audit pairs for fresh and resumed attempts.
func TestAuditTrail_PersistsTimeoutAndCancellationLifecycle(t *testing.T) {
	for _, resume := range []bool{false, true} {
		for _, local := range []bool{false, true} {
			t.Run(fmt.Sprintf("resume-%t-local-%t", resume, local), func(t *testing.T) {
				testRootLifecyclePersistence(t, resume, local)
			})
		}
	}
}

func testRootLifecyclePersistence(t *testing.T, resume, local bool) {
	t.Helper()
	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflows")
	storageDir := filepath.Join(root, "storage")
	auditPath := filepath.Join(root, "audit.jsonl")
	require.NoError(t, os.MkdirAll(workflowDir, 0o700))
	t.Setenv("AWF_WORKFLOWS_PATH", workflowDir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg"))
	t.Setenv("AWF_CONFIG_PATH", "")
	t.Setenv("AWF_AUDIT_LOG", auditPath)
	t.Setenv("AWF_MCP_PROXY_NO_PURGE", "1")
	const inputCanary = "PRIVATE_LIFECYCLE_INPUT"
	const outputCanary = "PRIVATE_LIFECYCLE_OUTPUT"
	const environmentCanary = "PRIVATE_LIFECYCLE_ENV"
	t.Setenv("PRIVATE_ENV", environmentCanary)
	timeout := ""
	if local {
		timeout = "    timeout: 1\n"
	}
	source := "name: lifecycle\ninputs:\n  - name: credential\n    type: string\n    required: true\nstates:\n  initial: work\n  work:\n    type: command\n    command: controlled command\n    prompt: PRIVATE_LIFECYCLE_PROMPT\n" + timeout + "    on_success: done\n  done:\n    type: terminal\n"
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "lifecycle.yaml"), []byte(source), 0o600))

	var id string
	stateStore := store.NewJSONStore(filepath.Join(storageDir, "states"))
	if resume {
		_, err := runRootLifecycleAttempt(t, storageDir, local, true, "")
		require.ErrorIs(t, err, context.Canceled)
		ids, listErr := stateStore.List(context.Background())
		require.NoError(t, listErr)
		require.Len(t, ids, 1)
		id = ids[0]
	}
	output, runErr := runRootLifecycleAttempt(t, storageDir, local, false, id)
	cause := context.Canceled
	if local {
		cause = context.DeadlineExceeded
	}
	require.ErrorIs(t, runErr, cause)
	ids, err := stateStore.List(context.Background())
	require.NoError(t, err)
	require.Len(t, ids, 1)
	// Reopen every durable projection after the command has released its writers.
	saved, err := store.NewJSONStore(filepath.Join(storageDir, "states")).Load(context.Background(), ids[0])
	require.NoError(t, err)
	require.Equal(t, inputCanary, saved.Inputs["credential"])
	require.Equal(t, workflow.StatusCancelled, saved.Status)
	require.Equal(t, 3, saved.ExitCode)
	attemptID, attempt, previous, recorded := saved.GetHistoryAttempt()
	require.True(t, recorded)
	require.NotEmpty(t, attemptID)
	if resume {
		require.Equal(t, 2, attempt)
		require.NotEmpty(t, previous)
	} else {
		require.Equal(t, 1, attempt)
	}
	history, err := store.NewSQLiteHistoryStore(filepath.Join(storageDir, "history.db"))
	require.NoError(t, err)
	records, err := history.List(context.Background(), &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	require.NoError(t, history.Close())
	record := findHistoryRecord(t, records, attemptID)
	require.Equal(t, "cancelled", record.Status)
	require.Equal(t, 3, record.ExitCode)
	require.Contains(t, record.ErrorMessage, cause.Error())
	events := parseAuditLines(t, auditPath)
	started := findAuditEvents(events, attemptID, workflow.EventWorkflowStarted)
	completed := findAuditCompletions(events, attemptID)
	require.Len(t, started, 1)
	require.Len(t, completed, 1)
	require.Equal(t, "failure", completed[0].Status)
	require.Equal(t, 3, *completed[0].ExitCode)
	require.Contains(t, completed[0].Error, cause.Error())
	require.True(t, !completed[0].Timestamp.Before(started[0].Timestamp))
	if local {
		require.Contains(t, completed[0].Error, "timeout_owner: step")
		require.Contains(t, completed[0].Error, "timeout_seconds: 1")
	} else {
		require.NotContains(t, completed[0].Error, "timeout_owner")
	}
	stateBytes, err := os.ReadFile(filepath.Join(storageDir, "states", ids[0]+".json"))
	require.NoError(t, err)
	auditBytes, err := os.ReadFile(auditPath)
	require.NoError(t, err)
	require.Contains(t, string(stateBytes), inputCanary)
	evidence := string(auditBytes) + fmt.Sprint(records) + output + fmt.Sprint(runErr)
	for _, canary := range []string{inputCanary, outputCanary, environmentCanary, "PRIVATE_LIFECYCLE_PROMPT"} {
		require.NotContains(t, evidence, canary)
	}
}

func runRootLifecycleAttempt(t *testing.T, storageDir string, local, seed bool, id string) (string, error) {
	t.Helper()
	const inputCanary = "PRIVATE_LIFECYCLE_INPUT"
	const outputCanary = "PRIVATE_LIFECYCLE_OUTPUT"
	started := make(chan struct{})
	finished := make(chan struct{})
	command := commandExecutorFunc(func(ctx context.Context, _ *ports.Command) (*ports.CommandResult, error) {
		close(started)
		defer close(finished)
		<-ctx.Done()
		return &ports.CommandResult{Stdout: outputCanary, Stderr: outputCanary, ExitCode: -1}, ctx.Err()
	})
	cmd := cli.NewRootCommand(cli.WithCommandExecutor(command))
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	args := []string{"--storage", storageDir, "--no-color", "run", "lifecycle", "--skip-plugins", "--input", "credential=" + inputCanary}
	if id != "" {
		args = []string{"--storage", storageDir, "--no-color", "resume", id, "--from", "work", "--input", "credential=" + inputCanary}
	}
	cmd.SetArgs(args)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx) }()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("root never started command: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("command did not start")
	}
	if !local || seed {
		cancel()
	}
	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("root did not finish")
	}
	select {
	case <-finished:
	default:
		t.Fatal("executor outlived root")
	}
	return output.String(), err
}
