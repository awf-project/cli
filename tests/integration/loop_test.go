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
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// For-Each Loop Integration Tests
// =============================================================================

func TestForEachLoop_Simple_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")

	wfYAML := `name: foreach-simple
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["apple", "banana", "cherry"]'
    max_iterations: 10
    body:
      - echo_item
    on_complete: done
  echo_item:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify output
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "apple\nbanana\ncherry\n", string(data))
}

func TestForEachLoop_WithIndex_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "index.log")

	wfYAML := `name: foreach-index
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: 10
    body:
      - log_index
    on_complete: done
  log_index:
    type: step
    command: 'echo "{{.loop.Index}}: {{.loop.Item}}" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-index.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-index", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify index was correctly interpolated
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "0: a\n1: b\n2: c\n", string(data))
}

func TestForEachLoop_WithBreak_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "break.log")

	wfYAML := `name: foreach-break
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["1", "2", "stop", "4", "5"]'
    max_iterations: 10
    break_when: 'states.check.output == "stop"'
    body:
      - check
    on_complete: done
  check:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + logFile + ` && echo "{{.loop.Item}}"'
    capture:
      stdout: output
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-break.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-break", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify loop broke at "stop"
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	// Should only have processed 1, 2, stop (breaks after processing stop)
	assert.Equal(t, "1\n2\nstop\n", string(data))
}

func TestForEachLoop_MultipleBodySteps_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "multi.log")

	wfYAML := `name: foreach-multi
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["x", "y"]'
    max_iterations: 10
    body:
      - step1
      - step2
      - step3
    on_complete: done
  step1:
    type: step
    command: 'echo "S1:{{.loop.Item}}" >> ` + logFile + `'
    on_success: process_items
  step2:
    type: step
    command: 'echo "S2:{{.loop.Item}}" >> ` + logFile + `'
    on_success: process_items
  step3:
    type: step
    command: 'echo "S3:{{.loop.Item}}" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-multi.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-multi", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify all steps executed in order
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	expected := "S1:x\nS2:x\nS3:x\nS1:y\nS2:y\nS3:y\n"
	assert.Equal(t, expected, string(data))
}

// =============================================================================
// While Loop Integration Tests
// =============================================================================

func TestWhileLoop_Simple_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")

	// Create initial counter
	err := os.WriteFile(counterFile, []byte("0"), 0644)
	require.NoError(t, err)

	wfYAML := `name: while-simple
version: "1.0.0"
states:
  initial: count_loop
  count_loop:
    type: while
    while: 'true'
    max_iterations: 10
    break_when: 'states.increment.exit_code != 0'
    body:
      - increment
    on_complete: done
  increment:
    type: step
    command: |
      COUNT=$(cat ` + counterFile + `)
      NEW=$((COUNT + 1))
      echo $NEW > ` + counterFile + `
      test $NEW -lt 5
    on_success: count_loop
    on_failure: count_loop
  done:
    type: terminal
    status: success
`
	err = os.WriteFile(filepath.Join(tmpDir, "while.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "while", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify counter reached 5
	data, err := os.ReadFile(counterFile)
	require.NoError(t, err)
	assert.Equal(t, "5\n", string(data))
}

func TestWhileLoop_MaxIterations_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "max.log")

	wfYAML := `name: while-max
version: "1.0.0"
states:
  initial: infinite_loop
  infinite_loop:
    type: while
    while: 'true'
    max_iterations: 5
    body:
      - log_iteration
    on_complete: done
  log_iteration:
    type: step
    command: 'echo "iteration" >> ` + logFile + `'
    on_success: infinite_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "while-max.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := &alwaysTrueEvaluator{}

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "while-max", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify only 5 iterations occurred
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "iteration\niteration\niteration\niteration\niteration\n", string(data))
}

func TestWhileLoop_WithBreak_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "break.log")

	wfYAML := `name: while-break
version: "1.0.0"
states:
  initial: poll_loop
  poll_loop:
    type: while
    while: 'true'
    max_iterations: 100
    break_when: 'states.check.output == "ready"'
    body:
      - check
    on_complete: done
  check:
    type: step
    command: |
      COUNT=$(wc -l < ` + logFile + ` 2>/dev/null || echo 0)
      echo "poll" >> ` + logFile + `
      if [ "$COUNT" -ge 3 ]; then
        echo "ready"
      else
        echo "waiting"
      fi
    capture:
      stdout: output
    on_success: poll_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "while-break.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)
	// Create empty log file
	err = os.WriteFile(logFile, []byte{}, 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "while-break", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify loop stopped after "ready" condition
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "poll\npoll\npoll\npoll\n", string(data)) // 4 polls before ready
}

// =============================================================================
// Error Handling Integration Tests
// =============================================================================

func TestForEachLoop_StepError_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "error.log")

	wfYAML := `name: foreach-error
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["ok", "fail", "never"]'
    max_iterations: 10
    body:
      - check_item
    on_complete: done
    on_failure: error_handler
  check_item:
    type: step
    command: |
      echo "{{.loop.Item}}" >> ` + logFile + `
      test "{{.loop.Item}}" != "fail"
    on_success: process_items
  error_handler:
    type: step
    command: 'echo "ERROR" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-error.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-error", nil)

	require.NoError(t, err) // Workflow completes via error handler
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify error handling occurred
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "ok\nfail\nERROR\n", string(data))
}

// =============================================================================
// Fixtures Integration Tests
// =============================================================================

func TestLoopFixtures_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	fixturesPath := "../fixtures/workflows"

	// Check if loop fixtures exist
	foreachPath := filepath.Join(fixturesPath, "loop-foreach.yaml")
	whilePath := filepath.Join(fixturesPath, "loop-while.yaml")

	// Skip if fixtures don't exist yet
	if _, err := os.Stat(foreachPath); os.IsNotExist(err) {
		t.Skip("loop fixtures not yet created")
	}
	if _, err := os.Stat(whilePath); os.IsNotExist(err) {
		t.Skip("loop fixtures not yet created")
	}

	repo := repository.NewYAMLRepository(fixturesPath)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("foreach fixture", func(t *testing.T) {
		execCtx, err := execSvc.Run(ctx, "loop-foreach", nil)
		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	})

	t.Run("while fixture", func(t *testing.T) {
		execCtx, err := execSvc.Run(ctx, "loop-while", nil)
		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	})
}

// =============================================================================
// Test Helpers
// =============================================================================

// simpleExpressionEvaluator evaluates basic expressions for integration tests
type simpleExpressionEvaluator struct{}

func newSimpleExpressionEvaluator() *simpleExpressionEvaluator {
	return &simpleExpressionEvaluator{}
}

func (e *simpleExpressionEvaluator) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	// Handle common test expressions
	switch expr {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	// Check for states.X.output == "value" pattern
	if len(expr) > 0 {
		// Simple implementation for integration tests
		// In real implementation, use proper expression evaluator

		// Check exit_code patterns
		if ctx != nil && ctx.States != nil {
			for stepName, state := range ctx.States {
				// states.X.exit_code == 0
				if expr == "states."+stepName+".exit_code == 0" {
					return state.ExitCode == 0, nil
				}
				// states.X.exit_code != 0
				if expr == "states."+stepName+".exit_code != 0" {
					return state.ExitCode != 0, nil
				}
				// states.X.output == "value" (simplified)
				if expr == `states.`+stepName+`.output == "ready"` {
					return state.Output == "ready\n" || state.Output == "ready", nil
				}
				if expr == `states.`+stepName+`.output == "stop"` {
					return state.Output == "stop\n" || state.Output == "stop", nil
				}
			}
		}
	}

	return false, nil
}

// alwaysTrueEvaluator always returns true (for testing max_iterations)
type alwaysTrueEvaluator struct{}

func (e *alwaysTrueEvaluator) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	return true, nil
}
