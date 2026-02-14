//go:build integration

package execution_test

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
	infraExpr "github.com/vanoix/awf/internal/infrastructure/expression"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/pkg/interpolation"
)

// mockStateStore for integration tests (copied from execution_test.go)
type retryMockStateStore struct {
	states map[string]*workflow.ExecutionContext
}

func newRetryMockStateStore() *retryMockStateStore {
	return &retryMockStateStore{states: make(map[string]*workflow.ExecutionContext)}
}

func (m *retryMockStateStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	m.states[state.WorkflowID] = state
	return nil
}

func (m *retryMockStateStore) Load(ctx context.Context, id string) (*workflow.ExecutionContext, error) {
	if state, ok := m.states[id]; ok {
		return state, nil
	}
	return nil, nil
}

func (m *retryMockStateStore) Delete(ctx context.Context, id string) error {
	delete(m.states, id)
	return nil
}

func (m *retryMockStateStore) List(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	return ids, nil
}

// retryMockLogger for integration tests
type retryMockLogger struct{}

func (m *retryMockLogger) Debug(msg string, fields ...any) {}
func (m *retryMockLogger) Info(msg string, fields ...any)  {}
func (m *retryMockLogger) Warn(msg string, fields ...any)  {}
func (m *retryMockLogger) Error(msg string, fields ...any) {}
func (m *retryMockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

func TestRetry_SucceedsOnNthAttempt_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")

	// Create a workflow that uses a file counter to fail first 2 times, succeed on 3rd
	// This tests real retry behavior with actual shell commands
	wfYAML := `name: retry-counter
version: "1.0.0"
states:
  initial: flaky
  flaky:
    type: step
    command: |
      COUNT=$(cat "` + counterFile + `" 2>/dev/null || echo "0")
      COUNT=$((COUNT + 1))
      echo $COUNT > "` + counterFile + `"
      if [ $COUNT -lt 3 ]; then
        echo "Attempt $COUNT failed" >&2
        exit 1
      fi
      echo "Success on attempt $COUNT"
    retry:
      max_attempts: 5
      initial_delay: 50ms
      max_delay: 500ms
      backoff: constant
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "retry-counter.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	// Wire up real components
	repo := repository.NewYAMLRepository(tmpDir)
	store := newRetryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &retryMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	// Execute
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "retry-counter", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep, "should succeed after retries")

	// Verify final counter value
	counterData, err := os.ReadFile(counterFile)
	require.NoError(t, err)
	assert.Contains(t, string(counterData), "3", "counter should be 3 after 3 attempts")

	// Verify step state
	state, ok := execCtx.GetStepState("flaky")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Contains(t, state.Output, "Success on attempt 3")
}

func TestRetry_ExhaustsAllAttempts_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "attempts.log")

	// Workflow that always fails (never succeeds)
	wfYAML := `name: retry-exhaust
version: "1.0.0"
states:
  initial: always_fail
  always_fail:
    type: step
    command: |
      echo "attempt" >> "` + logFile + `"
      exit 1
    retry:
      max_attempts: 3
      initial_delay: 10ms
      max_delay: 100ms
      backoff: constant
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "retry-exhaust.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newRetryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &retryMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "retry-exhaust", nil)

	require.NoError(t, err) // workflow completes via error path
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "error", execCtx.CurrentStep, "should end at error terminal after exhausting retries")

	// Count attempts
	logData, err := os.ReadFile(logFile)
	require.NoError(t, err)
	// Count "attempt" occurrences (3 lines)
	lines := 0
	for _, b := range logData {
		if b == '\n' {
			lines++
		}
	}
	assert.Equal(t, 3, lines, "should have made exactly 3 attempts")
}

func TestRetry_ExitCodeFiltering_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")

	// Workflow that retries only on exit code 1, not on exit code 3
	wfYAML := `name: retry-exit-codes
version: "1.0.0"
states:
  initial: filtered
  filtered:
    type: step
    command: |
      COUNT=$(cat "` + counterFile + `" 2>/dev/null || echo "0")
      COUNT=$((COUNT + 1))
      echo $COUNT > "` + counterFile + `"
      # Exit with code 3 on first try (not retryable)
      if [ $COUNT -eq 1 ]; then
        exit 3
      fi
      exit 0
    retry:
      max_attempts: 5
      initial_delay: 10ms
      max_delay: 100ms
      backoff: constant
      retryable_exit_codes: [1, 2]  # only retry on 1 or 2, not 3
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "retry-exit-codes.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newRetryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &retryMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "retry-exit-codes", nil)

	require.NoError(t, err)
	assert.Equal(t, "error", execCtx.CurrentStep, "should go to error because exit code 3 is not retryable")

	// Verify only 1 attempt was made (no retry on exit code 3)
	counterData, err := os.ReadFile(counterFile)
	require.NoError(t, err)
	assert.Contains(t, string(counterData), "1", "should have made only 1 attempt")

	// Verify exit code in state
	state, ok := execCtx.GetStepState("filtered")
	require.True(t, ok)
	assert.Equal(t, 3, state.ExitCode)
}

func TestRetry_ExponentialBackoff_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")
	timestampFile := filepath.Join(tmpDir, "timestamps")

	// Workflow with exponential backoff - record timestamps to verify delays
	wfYAML := `name: retry-exponential
version: "1.0.0"
states:
  initial: exp_retry
  exp_retry:
    type: step
    command: |
      COUNT=$(cat "` + counterFile + `" 2>/dev/null || echo "0")
      COUNT=$((COUNT + 1))
      echo $COUNT > "` + counterFile + `"
      date +%s%N >> "` + timestampFile + `"
      if [ $COUNT -lt 3 ]; then
        exit 1
      fi
      exit 0
    retry:
      max_attempts: 5
      initial_delay: 100ms
      max_delay: 2s
      backoff: exponential
      multiplier: 2
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "retry-exponential.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newRetryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &retryMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	execCtx, err := execSvc.Run(ctx, "retry-exponential", nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// With exponential backoff:
	// - Attempt 1: immediate
	// - Attempt 2: after ~100ms delay (100ms * 2^0)
	// - Attempt 3: after ~200ms delay (100ms * 2^1)
	// Total minimum: ~300ms
	assert.GreaterOrEqual(t, elapsed, 200*time.Millisecond, "should have waited for exponential backoff delays")
}

func TestRetry_LinearBackoff_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")

	wfYAML := `name: retry-linear
version: "1.0.0"
states:
  initial: lin_retry
  lin_retry:
    type: step
    command: |
      COUNT=$(cat "` + counterFile + `" 2>/dev/null || echo "0")
      COUNT=$((COUNT + 1))
      echo $COUNT > "` + counterFile + `"
      if [ $COUNT -lt 3 ]; then
        exit 1
      fi
      exit 0
    retry:
      max_attempts: 5
      initial_delay: 50ms
      max_delay: 1s
      backoff: linear
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "retry-linear.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newRetryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &retryMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	execCtx, err := execSvc.Run(ctx, "retry-linear", nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// With linear backoff:
	// - Attempt 1: immediate
	// - Attempt 2: after 100ms (50ms * 2)
	// - Attempt 3: after 150ms (50ms * 3)
	// Total minimum: ~250ms
	assert.GreaterOrEqual(t, elapsed, 150*time.Millisecond, "should have waited for linear backoff delays")
}

func TestRetry_WithJitter_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")

	wfYAML := `name: retry-jitter
version: "1.0.0"
states:
  initial: jitter_retry
  jitter_retry:
    type: step
    command: |
      COUNT=$(cat "` + counterFile + `" 2>/dev/null || echo "0")
      COUNT=$((COUNT + 1))
      echo $COUNT > "` + counterFile + `"
      if [ $COUNT -lt 2 ]; then
        exit 1
      fi
      exit 0
    retry:
      max_attempts: 3
      initial_delay: 100ms
      max_delay: 1s
      backoff: constant
      jitter: 0.5
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "retry-jitter.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newRetryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &retryMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "retry-jitter", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify counter file
	counterData, err := os.ReadFile(counterFile)
	require.NoError(t, err)
	assert.Contains(t, string(counterData), "2", "should have made 2 attempts")
}

func TestRetry_MaxDelayCap_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")

	// High multiplier but low max_delay should cap the delay
	wfYAML := `name: retry-max-cap
version: "1.0.0"
states:
  initial: capped_retry
  capped_retry:
    type: step
    command: |
      COUNT=$(cat "` + counterFile + `" 2>/dev/null || echo "0")
      COUNT=$((COUNT + 1))
      echo $COUNT > "` + counterFile + `"
      if [ $COUNT -lt 4 ]; then
        exit 1
      fi
      exit 0
    retry:
      max_attempts: 5
      initial_delay: 100ms
      max_delay: 150ms
      backoff: exponential
      multiplier: 10
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "retry-max-cap.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newRetryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &retryMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	execCtx, err := execSvc.Run(ctx, "retry-max-cap", nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Without cap: delays would be 100ms, 1s, 10s = ~11.1s total
	// With cap at 150ms: delays are 100ms, 150ms, 150ms = ~400ms max
	assert.Less(t, elapsed, 2*time.Second, "delays should be capped at max_delay")
}

func TestRetry_NoRetryOnSuccess_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")

	// Command succeeds immediately - should not retry
	wfYAML := `name: retry-success-immediate
version: "1.0.0"
states:
  initial: success_step
  success_step:
    type: step
    command: |
      COUNT=$(cat "` + counterFile + `" 2>/dev/null || echo "0")
      COUNT=$((COUNT + 1))
      echo $COUNT > "` + counterFile + `"
      exit 0
    retry:
      max_attempts: 5
      initial_delay: 100ms
      max_delay: 1s
      backoff: constant
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	err := os.WriteFile(filepath.Join(tmpDir, "retry-success.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newRetryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &retryMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "retry-success", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify only 1 attempt was made
	counterData, err := os.ReadFile(counterFile)
	require.NoError(t, err)
	assert.Contains(t, string(counterData), "1", "should have made only 1 attempt on success")
}
