//go:build integration

package integration_test

import (
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

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger)

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

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger)

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

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger)

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

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	execSvc := application.NewExecutionService(wfSvc, exec, store, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "valid-simple", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}
