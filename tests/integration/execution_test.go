//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/pkg/interpolation"
)

// mockStateStore for integration tests
type mockStateStore struct {
	states map[string]*workflow.ExecutionContext
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{states: make(map[string]*workflow.ExecutionContext)}
}

func (m *mockStateStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	m.states[state.WorkflowID] = state
	return nil
}

func (m *mockStateStore) Load(ctx context.Context, id string) (*workflow.ExecutionContext, error) {
	if state, ok := m.states[id]; ok {
		return state, nil
	}
	return nil, nil
}

func (m *mockStateStore) Delete(ctx context.Context, id string) error {
	delete(m.states, id)
	return nil
}

func (m *mockStateStore) List(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	return ids, nil
}

// mockLogger for integration tests
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...any) {}
func (m *mockLogger) Info(msg string, fields ...any)  {}
func (m *mockLogger) Warn(msg string, fields ...any)  {}
func (m *mockLogger) Error(msg string, fields ...any) {}
func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

func TestLinearExecution_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// setup temp directory with workflow file
	tmpDir := t.TempDir()

	wfYAML := `name: integration-test
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "step 1 output"
    on_success: step2
  step2:
    type: step
    command: echo "step 2 output"
    on_success: done
  done:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	// wire up real components
	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}

	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger, resolver)

	// execute
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// verify outputs
	state1, ok := execCtx.GetStepState("step1")
	require.True(t, ok)
	assert.Equal(t, "step 1 output\n", state1.Output)
	assert.Equal(t, 0, state1.ExitCode)

	state2, ok := execCtx.GetStepState("step2")
	require.True(t, ok)
	assert.Equal(t, "step 2 output\n", state2.Output)
	assert.Equal(t, 0, state2.ExitCode)
}

func TestLinearExecution_FailurePath_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	wfYAML := `name: failure-test
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: exit 1
    on_success: success
    on_failure: error_handler
  success:
    type: terminal
  error_handler:
    type: step
    command: echo "handling error"
    on_success: done
  done:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "failure.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger, resolver)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "failure", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// verify failure path was taken
	startState, ok := execCtx.GetStepState("start")
	require.True(t, ok)
	assert.Equal(t, 1, startState.ExitCode)
	assert.Equal(t, workflow.StatusFailed, startState.Status)

	// verify error handler ran
	errorState, ok := execCtx.GetStepState("error_handler")
	require.True(t, ok)
	assert.Equal(t, "handling error\n", errorState.Output)
	assert.Equal(t, workflow.StatusCompleted, errorState.Status)
}

func TestLinearExecution_Timeout_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	wfYAML := `name: timeout-test
version: "1.0.0"
states:
  initial: slow
  slow:
    type: step
    command: sleep 10
    timeout: 1
    on_success: done
    on_failure: timeout_handler
  done:
    type: terminal
  timeout_handler:
    type: step
    command: echo "timeout occurred"
    on_success: done
`
	err := os.WriteFile(filepath.Join(tmpDir, "timeout.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger, resolver)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	execCtx, err := execSvc.Run(ctx, "timeout", nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// should complete within ~3 seconds (1s timeout + overhead + handler)
	assert.Less(t, elapsed, 5*time.Second)

	// verify timeout handler ran
	handlerState, ok := execCtx.GetStepState("timeout_handler")
	require.True(t, ok)
	assert.Equal(t, "timeout occurred\n", handlerState.Output)
}

func TestLinearExecution_WithValidFixture_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// use existing fixtures
	fixturesPath := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesPath)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger, resolver)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "valid-simple", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

func TestHooks_WorkflowAndStepHooks_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// workflow with hooks that write to a log file for verification
	logFile := filepath.Join(tmpDir, "hooks.log")
	wfYAML := `name: hooks-test
version: "1.0.0"

hooks:
  workflow_start:
    - command: echo "WORKFLOW_START" >> ` + logFile + `
  workflow_end:
    - command: echo "WORKFLOW_END" >> ` + logFile + `

states:
  initial: step1
  step1:
    type: step
    command: echo "MAIN" >> ` + logFile + `
    hooks:
      pre:
        - command: echo "PRE_STEP1" >> ` + logFile + `
      post:
        - command: echo "POST_STEP1" >> ` + logFile + `
    on_success: done
  done:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "hooks.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger, resolver)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "hooks", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// verify hook execution order
	logData, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := "WORKFLOW_START\nPRE_STEP1\nMAIN\nPOST_STEP1\nWORKFLOW_END\n"
	assert.Equal(t, expected, string(logData))
}

func TestHooks_WorkflowErrorHook_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "error_hooks.log")

	wfYAML := `name: error-hooks-test
version: "1.0.0"

hooks:
  workflow_start:
    - command: echo "START" >> ` + logFile + `
  workflow_error:
    - command: echo "ERROR_HOOK" >> ` + logFile + `
  workflow_end:
    - command: echo "END" >> ` + logFile + `

states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_success: done
  done:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "error_hooks.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger, resolver)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "error_hooks", nil)

	// workflow should fail
	require.Error(t, err)
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)

	// verify error hook was executed (not end hook)
	logData, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := "START\nERROR_HOOK\n"
	assert.Equal(t, expected, string(logData))
}

func TestHooks_MultipleSteps_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "multi_step_hooks.log")

	wfYAML := `name: multi-step-hooks
version: "1.0.0"

hooks:
  workflow_start:
    - command: echo "WORKFLOW_START" >> ` + logFile + `
  workflow_end:
    - command: echo "WORKFLOW_END" >> ` + logFile + `

states:
  initial: step1
  step1:
    type: step
    command: echo "STEP1" >> ` + logFile + `
    hooks:
      pre:
        - command: echo "PRE1" >> ` + logFile + `
      post:
        - command: echo "POST1" >> ` + logFile + `
    on_success: step2
  step2:
    type: step
    command: echo "STEP2" >> ` + logFile + `
    hooks:
      pre:
        - command: echo "PRE2" >> ` + logFile + `
      post:
        - command: echo "POST2" >> ` + logFile + `
    on_success: done
  done:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "multi_hooks.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger, resolver)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "multi_hooks", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// verify execution order
	logData, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := "WORKFLOW_START\nPRE1\nSTEP1\nPOST1\nPRE2\nSTEP2\nPOST2\nWORKFLOW_END\n"
	assert.Equal(t, expected, string(logData))
}

func TestHooks_LogAction_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// workflow using log hooks (not command hooks)
	wfYAML := `name: log-hooks-test
version: "1.0.0"

hooks:
  workflow_start:
    - log: "Starting workflow..."
  workflow_end:
    - log: "Workflow completed!"

states:
  initial: step1
  step1:
    type: step
    command: echo "hello"
    hooks:
      pre:
        - log: "Before step1"
      post:
        - log: "After step1"
    on_success: done
  done:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "log_hooks.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{} // logs go to mock (no-op)
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger, resolver)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "log_hooks", nil)

	// should complete successfully even though log hooks don't write to file
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

func TestStreamingOutput_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	wfYAML := `name: streaming-test
version: "1.0.0"
states:
  initial: output_step
  output_step:
    type: step
    command: |
      echo "stdout line 1"
      echo "stderr line 1" >&2
      echo "stdout line 2"
    on_success: done
  done:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "streaming.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger, resolver)

	// Capture streaming output
	var stdoutBuf, stderrBuf bytes.Buffer
	execSvc.SetOutputWriters(&stdoutBuf, &stderrBuf)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "streaming", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify streaming output captured
	assert.Contains(t, stdoutBuf.String(), "stdout line 1")
	assert.Contains(t, stdoutBuf.String(), "stdout line 2")
	assert.Contains(t, stderrBuf.String(), "stderr line 1")
}

func TestValidFixtures_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// test that all valid fixtures can be loaded and validated
	fixturesPath := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesPath)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)

	ctx := context.Background()

	fixtures := []string{
		"valid-simple",
		"valid-full",
		"valid-with-hooks",
		"valid-parallel",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			wf, err := wfSvc.GetWorkflow(ctx, name)
			require.NoError(t, err, "should load fixture %s", name)
			require.NotNil(t, wf, "workflow should not be nil for %s", name)

			err = wfSvc.ValidateWorkflow(ctx, name)
			require.NoError(t, err, "should validate fixture %s", name)
		})
	}
}
