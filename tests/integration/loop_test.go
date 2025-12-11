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
// F042: Index1 (1-based index) Integration Tests
// =============================================================================

func TestForEachLoop_WithIndex1_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "index1.log")

	wfYAML := `name: foreach-index1
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: 10
    body:
      - log_index1
    on_complete: done
  log_index1:
    type: step
    command: 'echo "{{.loop.Index1}}: {{.loop.Item}}" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-index1.yaml"), []byte(wfYAML), 0644)
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

	execCtx, err := execSvc.Run(ctx, "foreach-index1", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify 1-based index output: 1: a, 2: b, 3: c
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "1: a\n2: b\n3: c\n", string(data))
}

func TestForEachLoop_Index1WithLength_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "progress.log")

	wfYAML := `name: foreach-progress
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["x", "y", "z"]'
    max_iterations: 10
    body:
      - log_progress
    on_complete: done
  log_progress:
    type: step
    command: 'echo "Processing {{.loop.Index1}}/{{.loop.Length}}: {{.loop.Item}}" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-progress.yaml"), []byte(wfYAML), 0644)
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

	execCtx, err := execSvc.Run(ctx, "foreach-progress", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify progress-style output
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	expected := "Processing 1/3: x\nProcessing 2/3: y\nProcessing 3/3: z\n"
	assert.Equal(t, expected, string(data))
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

// =============================================================================
// Feature: F042 - Loop Context Variables Functional Tests
// =============================================================================
//
// These tests validate that loop context variables (index, index1, item, first,
// last, length) work correctly in various scenarios including conditional
// expressions, nested loops, and context lifecycle management.

// loopContextEvaluator evaluates expressions that include loop context variables.
// This enables testing `when` conditions that use loop.* variables.
type loopContextEvaluator struct{}

func newLoopContextEvaluator() *loopContextEvaluator {
	return &loopContextEvaluator{}
}

func (e *loopContextEvaluator) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	// Handle static expressions
	switch expr {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	// Handle loop context expressions
	if ctx != nil && ctx.Loop != nil {
		switch expr {
		case "loop.first":
			return ctx.Loop.First, nil
		case "loop.last":
			return ctx.Loop.Last, nil
		case "loop.first == true":
			return ctx.Loop.First, nil
		case "loop.last == true":
			return ctx.Loop.Last, nil
		case "loop.first == false":
			return !ctx.Loop.First, nil
		case "loop.last == false":
			return !ctx.Loop.Last, nil
		case "loop.index == 0":
			return ctx.Loop.Index == 0, nil
		case "loop.index > 0":
			return ctx.Loop.Index > 0, nil
		case "loop.index < loop.length - 1":
			return ctx.Loop.Index < ctx.Loop.Length-1, nil
		}
	}

	// Fall back to simple evaluator for state checks
	if ctx != nil && ctx.States != nil {
		for stepName, state := range ctx.States {
			if expr == "states."+stepName+".exit_code == 0" {
				return state.ExitCode == 0, nil
			}
			if expr == "states."+stepName+".exit_code != 0" {
				return state.ExitCode != 0, nil
			}
		}
	}

	return false, nil
}

// TestF042_LoopAllVariables_Integration verifies all loop context variables
// work correctly in step command templates.
func TestF042_LoopAllVariables_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "all_vars.log")

	// Workflow that outputs all loop variables
	wfYAML := `name: foreach-all-vars
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["alpha", "beta", "gamma"]'
    max_iterations: 10
    body:
      - log_all
    on_complete: done
  log_all:
    type: step
    command: 'echo "idx={{.loop.Index}} idx1={{.loop.Index1}} item={{.loop.Item}} first={{.loop.First}} last={{.loop.Last}} len={{.loop.Length}}" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-all-vars.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-all-vars", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify all variables were correctly interpolated
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `idx=0 idx1=1 item=alpha first=true last=false len=3
idx=1 idx1=2 item=beta first=false last=false len=3
idx=2 idx1=3 item=gamma first=false last=true len=3
`
	assert.Equal(t, expected, string(data))
}

// TestF042_LoopFirst_ConditionalLogic_Integration tests using loop.First
// to perform special handling on the first iteration.
func TestF042_LoopFirst_ConditionalLogic_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "first.log")

	// Workflow that adds a header only on first iteration
	wfYAML := `name: foreach-first
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["row1", "row2", "row3"]'
    max_iterations: 10
    body:
      - write_header
      - write_data
    on_complete: done
  write_header:
    type: step
    command: '{{if .loop.First}}echo "=== HEADER ===" >> ` + logFile + `{{else}}echo "skip header" > /dev/null{{end}}'
    on_success: process_items
  write_data:
    type: step
    command: 'echo "DATA: {{.loop.Item}}" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-first.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-first", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify header appears only once, before all data rows
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `=== HEADER ===
DATA: row1
DATA: row2
DATA: row3
`
	assert.Equal(t, expected, string(data))
}

// TestF042_LoopLast_ConditionalLogic_Integration tests using loop.Last
// to perform special handling on the last iteration.
func TestF042_LoopLast_ConditionalLogic_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "last.log")

	// Workflow that adds a footer only on last iteration
	wfYAML := `name: foreach-last
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["item1", "item2", "item3"]'
    max_iterations: 10
    body:
      - write_item
      - write_footer
    on_complete: done
  write_item:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + logFile + `'
    on_success: process_items
  write_footer:
    type: step
    command: '{{if .loop.Last}}echo "=== END ===" >> ` + logFile + `{{else}}true{{end}}'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-last.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-last", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify footer appears only at the end
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `item1
item2
item3
=== END ===
`
	assert.Equal(t, expected, string(data))
}

// TestF042_LoopIndex1_ProgressOutput_Integration tests using Index1
// for human-readable progress output.
func TestF042_LoopIndex1_ProgressOutput_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "progress.log")

	// Workflow that shows progress with 1-based numbering
	wfYAML := `name: foreach-progress
version: "1.0.0"
states:
  initial: process_files
  process_files:
    type: for_each
    items: '["doc1.pdf", "doc2.pdf", "doc3.pdf", "doc4.pdf", "doc5.pdf"]'
    max_iterations: 10
    body:
      - show_progress
    on_complete: done
  show_progress:
    type: step
    command: 'echo "[{{.loop.Index1}}/{{.loop.Length}}] Processing {{.loop.Item}}" >> ` + logFile + `'
    on_success: process_files
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-progress.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-progress", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify 1-based progress output
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `[1/5] Processing doc1.pdf
[2/5] Processing doc2.pdf
[3/5] Processing doc3.pdf
[4/5] Processing doc4.pdf
[5/5] Processing doc5.pdf
`
	assert.Equal(t, expected, string(data))
}

// TestF042_LoopContextClearedAfterCompletion_Integration verifies that
// loop context is cleared after the loop completes.
func TestF042_LoopContextClearedAfterCompletion_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "cleared.log")

	// Workflow that has a step after the loop which should not have loop context
	wfYAML := `name: foreach-cleared
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["x", "y"]'
    max_iterations: 10
    body:
      - log_item
    on_complete: after_loop
  log_item:
    type: step
    command: 'echo "IN LOOP: {{.loop.Item}}" >> ` + logFile + `'
    on_success: process_items
  after_loop:
    type: step
    command: 'echo "AFTER LOOP: workflow={{.workflow.Name}}" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-cleared.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-cleared", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify loop context was used during loop but cleared after
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `IN LOOP: x
IN LOOP: y
AFTER LOOP: workflow=foreach-cleared
`
	assert.Equal(t, expected, string(data))

	// Also verify that execCtx.CurrentLoop is nil after workflow completes
	assert.Nil(t, execCtx.CurrentLoop, "loop context should be cleared after completion")
}

// TestF042_LoopSingleItem_EdgeCase_Integration tests edge case with single item.
func TestF042_LoopSingleItem_EdgeCase_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "single.log")

	// Single item should have first=true AND last=true
	wfYAML := `name: foreach-single
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["only_one"]'
    max_iterations: 10
    body:
      - log_flags
    on_complete: done
  log_flags:
    type: step
    command: 'echo "first={{.loop.First}} last={{.loop.Last}} idx={{.loop.Index}} idx1={{.loop.Index1}} len={{.loop.Length}}" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-single.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-single", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Single item should be both first and last
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := "first=true last=true idx=0 idx1=1 len=1\n"
	assert.Equal(t, expected, string(data))
}

// TestF042_WhileLoopIndex_Integration tests loop variables in while loops.
// While loops have Length=-1 since count is unknown upfront.
func TestF042_WhileLoopIndex_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "while_idx.log")
	counterFile := filepath.Join(tmpDir, "counter")

	// Initialize counter
	err := os.WriteFile(counterFile, []byte("0"), 0644)
	require.NoError(t, err)

	// While loop that uses loop.Index
	wfYAML := `name: while-index
version: "1.0.0"
states:
  initial: count_loop
  count_loop:
    type: while
    while: 'true'
    max_iterations: 5
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
      echo "iteration={{.loop.Index}} first={{.loop.First}} length={{.loop.Length}}" >> ` + logFile + `
      test $NEW -lt 4
    on_success: count_loop
    on_failure: count_loop
  done:
    type: terminal
    status: success
`
	err = os.WriteFile(filepath.Join(tmpDir, "while-index.yaml"), []byte(wfYAML), 0644)
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

	execCtx, err := execSvc.Run(ctx, "while-index", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify while loop index tracking (length=-1 for while loops)
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `iteration=0 first=true length=-1
iteration=1 first=false length=-1
iteration=2 first=false length=-1
iteration=3 first=false length=-1
`
	assert.Equal(t, expected, string(data))
}

// TestF042_LoopWithNumericItems_Integration tests loop with numeric items.
func TestF042_LoopWithNumericItems_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "numeric.log")

	// Loop over numeric items
	wfYAML := `name: foreach-numeric
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '[10, 20, 30, 40]'
    max_iterations: 10
    body:
      - calculate
    on_complete: done
  calculate:
    type: step
    command: 'echo "Position {{.loop.Index1}}: value={{.loop.Item}} doubled=$(({{.loop.Item}} * 2))" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-numeric.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-numeric", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify numeric items work correctly
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `Position 1: value=10 doubled=20
Position 2: value=20 doubled=40
Position 3: value=30 doubled=60
Position 4: value=40 doubled=80
`
	assert.Equal(t, expected, string(data))
}

// TestF042_LoopWithInputItems_Integration tests loop items from workflow inputs.
func TestF042_LoopWithInputItems_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "input_items.log")

	// Loop over items from input
	wfYAML := `name: foreach-input
version: "1.0.0"
inputs:
  - name: files
    type: string
states:
  initial: process_files
  process_files:
    type: for_each
    items: '{{.inputs.files}}'
    max_iterations: 20
    body:
      - process
    on_complete: done
  process:
    type: step
    command: 'echo "File {{.loop.Index1}}/{{.loop.Length}}: {{.loop.Item}}" >> ` + logFile + `'
    on_success: process_files
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-input.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass items as JSON array via input
	inputs := map[string]any{
		"files": `["config.yaml", "main.go", "test.go"]`,
	}

	execCtx, err := execSvc.Run(ctx, "foreach-input", inputs)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify items from input work correctly
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `File 1/3: config.yaml
File 2/3: main.go
File 3/3: test.go
`
	assert.Equal(t, expected, string(data))
}

// TestF042_LoopWithStatesReference_Integration tests using loop variables
// combined with states reference to access previous step outputs.
func TestF042_LoopWithStatesReference_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "states_ref.log")

	// Loop that uses both loop and states variables
	wfYAML := `name: foreach-states
version: "1.0.0"
states:
  initial: init
  init:
    type: step
    command: 'echo "INITIALIZED"'
    capture:
      stdout: output
    on_success: process_items
  process_items:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: 10
    body:
      - process
    on_complete: done
  process:
    type: step
    command: 'echo "Item {{.loop.Index1}}: {{.loop.Item}} | init={{.states.init.Output}}" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-states.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-states", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify both loop and states variables work together
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Note: captured output includes newline, so each line has the newline from states.init.Output
	expected := `Item 1: a | init=INITIALIZED

Item 2: b | init=INITIALIZED

Item 3: c | init=INITIALIZED

`
	assert.Equal(t, expected, string(data))
}

// TestF042_EmptyLoop_EdgeCase_Integration tests edge case with empty items array.
func TestF042_EmptyLoop_EdgeCase_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "empty.log")

	// Create log file to ensure it exists after test
	err := os.WriteFile(logFile, []byte("START\n"), 0644)
	require.NoError(t, err)

	// Empty items should complete immediately without executing body
	wfYAML := `name: foreach-empty
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '[]'
    max_iterations: 10
    body:
      - never_run
    on_complete: finish
  never_run:
    type: step
    command: 'echo "SHOULD NOT APPEAR" >> ` + logFile + `'
    on_success: process_items
  finish:
    type: step
    command: 'echo "FINISHED" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err = os.WriteFile(filepath.Join(tmpDir, "foreach-empty.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-empty", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify empty loop skips body but completes workflow
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `START
FINISHED
`
	assert.Equal(t, expected, string(data))
}

// =============================================================================
// Nested Loops Integration Tests
// =============================================================================
//
// NOTE: Nested loops are NOT YET IMPLEMENTED.
// These tests document the expected behavior for when nested loop support
// is added. They are currently skipped.
// See: docs/plans/features/v0.3.0/F042-loop-context-variables.md
//      "Nested loops: defer to future if complex"

// TestF042_NestedForEachLoops_Integration tests nested for_each loops with
// independent loop contexts for outer and inner loops.
func TestF042_NestedForEachLoops_Integration(t *testing.T) {
	t.Skip("PENDING: nested loops not yet implemented - this test documents expected behavior")

	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "nested.log")

	// Nested loops: outer iterates over rows, inner iterates over columns
	wfYAML := `name: nested-foreach
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["row1", "row2"]'
    max_iterations: 10
    body:
      - log_outer_start
      - inner_loop
    on_complete: done
  log_outer_start:
    type: step
    command: 'echo "OUTER[{{.loop.Index1}}/{{.loop.Length}}]: {{.loop.Item}} (first={{.loop.First}}, last={{.loop.Last}})" >> ` + logFile + `'
    on_success: outer_loop
  inner_loop:
    type: for_each
    items: '["colA", "colB", "colC"]'
    max_iterations: 10
    body:
      - log_inner
    on_complete: outer_loop
  log_inner:
    type: step
    command: 'echo "  INNER[{{.loop.Index1}}/{{.loop.Length}}]: {{.loop.Item}}" >> ` + logFile + `'
    on_success: inner_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-foreach.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "nested-foreach", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify nested loop execution order and context isolation
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `OUTER[1/2]: row1 (first=true, last=false)
  INNER[1/3]: colA
  INNER[2/3]: colB
  INNER[3/3]: colC
OUTER[2/2]: row2 (first=false, last=true)
  INNER[1/3]: colA
  INNER[2/3]: colB
  INNER[3/3]: colC
`
	assert.Equal(t, expected, string(data))
}

// TestF042_NestedLoops_ContextRestored_Integration verifies that outer loop
// context is properly restored after inner loop completes.
func TestF042_NestedLoops_ContextRestored_Integration(t *testing.T) {
	t.Skip("PENDING: nested loops not yet implemented - this test documents expected behavior")

	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "context_restore.log")

	// Test that outer loop context is restored after inner loop
	wfYAML := `name: nested-context-restore
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["A", "B"]'
    max_iterations: 10
    body:
      - before_inner
      - inner_loop
      - after_inner
    on_complete: done
  before_inner:
    type: step
    command: 'echo "BEFORE: outer={{.loop.Item}} idx={{.loop.Index}}" >> ` + logFile + `'
    on_success: outer_loop
  inner_loop:
    type: for_each
    items: '["1", "2"]'
    max_iterations: 10
    body:
      - inner_step
    on_complete: outer_loop
  inner_step:
    type: step
    command: 'echo "  INNER: item={{.loop.Item}} idx={{.loop.Index}}" >> ` + logFile + `'
    on_success: inner_loop
  after_inner:
    type: step
    command: 'echo "AFTER: outer={{.loop.Item}} idx={{.loop.Index}} (context restored)" >> ` + logFile + `'
    on_success: outer_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-context-restore.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "nested-context-restore", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify outer context is restored after inner loop
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `BEFORE: outer=A idx=0
  INNER: item=1 idx=0
  INNER: item=2 idx=1
AFTER: outer=A idx=0 (context restored)
BEFORE: outer=B idx=1
  INNER: item=1 idx=0
  INNER: item=2 idx=1
AFTER: outer=B idx=1 (context restored)
`
	assert.Equal(t, expected, string(data))
}

// TestF042_NestedWhileInForEach_Integration tests a while loop nested inside
// a for_each loop.
func TestF042_NestedWhileInForEach_Integration(t *testing.T) {
	t.Skip("PENDING: nested loops not yet implemented - this test documents expected behavior")

	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "nested_while.log")
	counterFile := filepath.Join(tmpDir, "counter")

	// For each item, run a while loop that counts up
	wfYAML := `name: nested-while-foreach
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["batch1", "batch2"]'
    max_iterations: 10
    body:
      - start_batch
      - count_loop
    on_complete: done
  start_batch:
    type: step
    command: |
      echo "0" > ` + counterFile + `
      echo "=== {{.loop.Item}} ===" >> ` + logFile + `
    on_success: outer_loop
  count_loop:
    type: while
    while: 'true'
    max_iterations: 5
    break_when: 'states.increment.exit_code != 0'
    body:
      - increment
    on_complete: outer_loop
  increment:
    type: step
    command: |
      COUNT=$(cat ` + counterFile + `)
      NEW=$((COUNT + 1))
      echo $NEW > ` + counterFile + `
      echo "  count={{.loop.Index1}} value=$NEW" >> ` + logFile + `
      test $NEW -lt 3
    on_success: count_loop
    on_failure: count_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-while-foreach.yaml"), []byte(wfYAML), 0644)
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

	execCtx, err := execSvc.Run(ctx, "nested-while-foreach", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify nested while loop executes correctly within for_each
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `=== batch1 ===
  count=1 value=1
  count=2 value=2
  count=3 value=3
=== batch2 ===
  count=1 value=1
  count=2 value=2
  count=3 value=3
`
	assert.Equal(t, expected, string(data))
}

// TestF042_TripleNestedLoops_Integration tests three levels of nested loops.
func TestF042_TripleNestedLoops_Integration(t *testing.T) {
	t.Skip("PENDING: nested loops not yet implemented - this test documents expected behavior")

	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "triple.log")

	// Three nested loops: categories -> items -> variants
	wfYAML := `name: triple-nested
version: "1.0.0"
states:
  initial: categories
  categories:
    type: for_each
    items: '["cat1", "cat2"]'
    max_iterations: 10
    body:
      - log_category
      - items
    on_complete: done
  log_category:
    type: step
    command: 'echo "CATEGORY: {{.loop.Item}}" >> ` + logFile + `'
    on_success: categories
  items:
    type: for_each
    items: '["item1", "item2"]'
    max_iterations: 10
    body:
      - log_item
      - variants
    on_complete: categories
  log_item:
    type: step
    command: 'echo "  ITEM: {{.loop.Item}}" >> ` + logFile + `'
    on_success: items
  variants:
    type: for_each
    items: '["v1", "v2"]'
    max_iterations: 10
    body:
      - log_variant
    on_complete: items
  log_variant:
    type: step
    command: 'echo "    VARIANT: {{.loop.Item}}" >> ` + logFile + `'
    on_success: variants
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "triple-nested.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "triple-nested", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify all three levels execute correctly
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `CATEGORY: cat1
  ITEM: item1
    VARIANT: v1
    VARIANT: v2
  ITEM: item2
    VARIANT: v1
    VARIANT: v2
CATEGORY: cat2
  ITEM: item1
    VARIANT: v1
    VARIANT: v2
  ITEM: item2
    VARIANT: v1
    VARIANT: v2
`
	assert.Equal(t, expected, string(data))
}

// TestF042_LoopIndexZeroBasedVsOneBased_Integration explicitly compares
// Index (0-based) vs Index1 (1-based) to ensure they differ correctly.
func TestF042_LoopIndexZeroBasedVsOneBased_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "compare.log")

	// Compare both index formats side by side
	wfYAML := `name: foreach-compare-idx
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["first", "second", "third", "fourth", "fifth"]'
    max_iterations: 10
    body:
      - compare
    on_complete: done
  compare:
    type: step
    command: 'echo "0-based={{.loop.Index}} | 1-based={{.loop.Index1}} | diff=$(({{.loop.Index1}} - {{.loop.Index}}))" >> ` + logFile + `'
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-compare-idx.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newLoopContextEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "foreach-compare-idx", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify Index1 is always Index + 1
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `0-based=0 | 1-based=1 | diff=1
0-based=1 | 1-based=2 | diff=1
0-based=2 | 1-based=3 | diff=1
0-based=3 | 1-based=4 | diff=1
0-based=4 | 1-based=5 | diff=1
`
	assert.Equal(t, expected, string(data))
}
