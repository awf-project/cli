//go:build integration

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// =============================================================================
// For-Each Loop Integration Tests
// =============================================================================

func TestForEachLoop_Simple_Integration(t *testing.T) {

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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-index.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-break.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-multi.yaml"), []byte(wfYAML), 0o644)
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

	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")

	// Create initial counter
	err := os.WriteFile(counterFile, []byte("0"), 0o644)
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
	err = os.WriteFile(filepath.Join(tmpDir, "while.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "while-max.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "while-break.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	// Create empty log file
	err = os.WriteFile(logFile, []byte{}, 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-error.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-index1.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-progress.yaml"), []byte(wfYAML), 0o644)
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

// f048TransitionsEvaluator evaluates expressions for F048 transition tests
// Supports "contains" pattern for checking step outputs
type f048TransitionsEvaluator struct{}

func newF048TransitionsEvaluator() *f048TransitionsEvaluator {
	return &f048TransitionsEvaluator{}
}

func (e *f048TransitionsEvaluator) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	// Handle static expressions
	switch expr {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	// Handle "states.X.Output contains Y" pattern
	if ctx != nil && ctx.States != nil {
		for stepName, state := range ctx.States {
			// Pattern: states.step_name.Output contains "value"
			containsPrefix := "states." + stepName + ".Output contains \""
			if len(expr) > len(containsPrefix)+1 && expr[:len(containsPrefix)] == containsPrefix {
				// Extract the value between quotes
				endQuote := len(expr) - 1
				if expr[endQuote] == '"' {
					searchValue := expr[len(containsPrefix):endQuote]
					// Check if state.Output contains searchValue
					return contains(state.Output, searchValue), nil
				}
			}
		}
	}

	return false, nil
}

// contains checks if haystack contains needle
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// mockLoggerWithCapture captures warnings for testing
type mockLoggerWithCapture struct {
	warns *[]string
}

func (m *mockLoggerWithCapture) Debug(msg string, fields ...any) {}
func (m *mockLoggerWithCapture) Info(msg string, fields ...any)  {}
func (m *mockLoggerWithCapture) Warn(msg string, fields ...any) {
	if m.warns != nil {
		*m.warns = append(*m.warns, msg)
	}
}
func (m *mockLoggerWithCapture) Error(msg string, fields ...any) {}
func (m *mockLoggerWithCapture) WithContext(ctx map[string]any) ports.Logger {
	return m
}

// TestF042_LoopAllVariables_Integration verifies all loop context variables
// work correctly in step command templates.
func TestF042_LoopAllVariables_Integration(t *testing.T) {

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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-all-vars.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-first.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-last.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-progress.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-cleared.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-single.yaml"), []byte(wfYAML), 0o644)
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

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "while_idx.log")
	counterFile := filepath.Join(tmpDir, "counter")

	// Initialize counter
	err := os.WriteFile(counterFile, []byte("0"), 0o644)
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
	err = os.WriteFile(filepath.Join(tmpDir, "while-index.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-numeric.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-input.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-states.yaml"), []byte(wfYAML), 0o644)
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

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "empty.log")

	// Create log file to ensure it exists after test
	err := os.WriteFile(logFile, []byte("START\n"), 0o644)
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
	err = os.WriteFile(filepath.Join(tmpDir, "foreach-empty.yaml"), []byte(wfYAML), 0o644)
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
	// F043 implementation enables nested loops


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
	err := os.WriteFile(filepath.Join(tmpDir, "nested-foreach.yaml"), []byte(wfYAML), 0o644)
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
	// F043 implementation enables nested loops


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
	err := os.WriteFile(filepath.Join(tmpDir, "nested-context-restore.yaml"), []byte(wfYAML), 0o644)
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
	// F043 implementation enables nested loops


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
	err := os.WriteFile(filepath.Join(tmpDir, "nested-while-foreach.yaml"), []byte(wfYAML), 0o644)
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
	// F043 implementation enables nested loops


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
	err := os.WriteFile(filepath.Join(tmpDir, "triple-nested.yaml"), []byte(wfYAML), 0o644)
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
	err := os.WriteFile(filepath.Join(tmpDir, "foreach-compare-idx.yaml"), []byte(wfYAML), 0o644)
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

// =============================================================================
// F043: Nested Loop Parent Access Integration Tests
// =============================================================================
// These tests verify the {{.loop.Parent.*}} template access feature for nested loops.

// TestF043_NestedLoops_ParentAccess_Integration tests that inner loops can access
// outer loop context via {{.loop.Parent.*}} syntax.
func TestF043_NestedLoops_ParentAccess_Integration(t *testing.T) {
	// F043 implementation enables nested loops


	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "parent_access.log")

	// Nested loops with inner loop accessing outer loop's context via Parent
	wfYAML := `name: nested-parent-access
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["A", "B"]'
    max_iterations: 10
    body:
      - inner_loop
    on_complete: done
  inner_loop:
    type: for_each
    items: '["1", "2"]'
    max_iterations: 10
    body:
      - log_both
    on_complete: outer_loop
  log_both:
    type: step
    command: 'echo "outer={{.loop.Parent.Item}} inner={{.loop.Item}}" >> ` + logFile + `'
    on_success: inner_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-parent-access.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-parent-access", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify inner loop can access outer loop's Item via Parent
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `outer=A inner=1
outer=A inner=2
outer=B inner=1
outer=B inner=2
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_ParentIndexAccess_Integration tests accessing parent's index
// and length fields via {{.loop.Parent.Index}}, {{.loop.Parent.Length}}, etc.
func TestF043_NestedLoops_ParentIndexAccess_Integration(t *testing.T) {
	// F043 implementation enables nested loops


	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "parent_index.log")

	// Test accessing all Parent fields: Index, Index1, Length, First, Last
	wfYAML := `name: nested-parent-index
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["row1", "row2", "row3"]'
    max_iterations: 10
    body:
      - inner_loop
    on_complete: done
  inner_loop:
    type: for_each
    items: '["col1", "col2"]'
    max_iterations: 10
    body:
      - log_indices
    on_complete: outer_loop
  log_indices:
    type: step
    command: 'echo "[{{.loop.Parent.Index1}}/{{.loop.Parent.Length}}][{{.loop.Index1}}/{{.loop.Length}}] outer={{.loop.Parent.Item}} inner={{.loop.Item}}" >> ` + logFile + `'
    on_success: inner_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-parent-index.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-parent-index", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `[1/3][1/2] outer=row1 inner=col1
[1/3][2/2] outer=row1 inner=col2
[2/3][1/2] outer=row2 inner=col1
[2/3][2/2] outer=row2 inner=col2
[3/3][1/2] outer=row3 inner=col1
[3/3][2/2] outer=row3 inner=col2
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_DeepParentChain_Integration tests accessing grandparent
// loop context via {{.loop.Parent.Parent.*}} for 3-level nesting.
func TestF043_NestedLoops_DeepParentChain_Integration(t *testing.T) {
	// F043 implementation enables nested loops


	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "deep_parent.log")

	// Three levels of nesting with grandparent access
	wfYAML := `name: triple-nested-parent
version: "1.0.0"
states:
  initial: l1_loop
  l1_loop:
    type: for_each
    items: '["A", "B"]'
    max_iterations: 10
    body:
      - l2_loop
    on_complete: done
  l2_loop:
    type: for_each
    items: '["1", "2"]'
    max_iterations: 10
    body:
      - l3_loop
    on_complete: l1_loop
  l3_loop:
    type: for_each
    items: '["x", "y"]'
    max_iterations: 10
    body:
      - log_all
    on_complete: l2_loop
  log_all:
    type: step
    command: 'echo "L1={{.loop.Parent.Parent.Item}} L2={{.loop.Parent.Item}} L3={{.loop.Item}}" >> ` + logFile + `'
    on_success: l3_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "triple-nested-parent.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "triple-nested-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// 2 * 2 * 2 = 8 lines total
	expected := `L1=A L2=1 L3=x
L1=A L2=1 L3=y
L1=A L2=2 L3=x
L1=A L2=2 L3=y
L1=B L2=1 L3=x
L1=B L2=1 L3=y
L1=B L2=2 L3=x
L1=B L2=2 L3=y
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_ParentFirstLast_Integration tests Parent.First and Parent.Last
// boolean fields in nested loops.
func TestF043_NestedLoops_ParentFirstLast_Integration(t *testing.T) {
	// F043 implementation enables nested loops


	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "parent_first_last.log")

	// Test First and Last flags access via Parent
	wfYAML := `name: nested-first-last
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["first", "middle", "last"]'
    max_iterations: 10
    body:
      - inner_loop
    on_complete: done
  inner_loop:
    type: for_each
    items: '["a", "b"]'
    max_iterations: 10
    body:
      - log_flags
    on_complete: outer_loop
  log_flags:
    type: step
    command: 'echo "outer={{.loop.Parent.Item}} (first={{.loop.Parent.First}} last={{.loop.Parent.Last}}) inner={{.loop.Item}}" >> ` + logFile + `'
    on_success: inner_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-first-last.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-first-last", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `outer=first (first=true last=false) inner=a
outer=first (first=true last=false) inner=b
outer=middle (first=false last=false) inner=a
outer=middle (first=false last=false) inner=b
outer=last (first=false last=true) inner=a
outer=last (first=false last=true) inner=b
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_WhileInsideForEach_ParentAccess_Integration tests parent
// access when a while loop is nested inside a for_each loop.
func TestF043_NestedLoops_WhileInsideForEach_ParentAccess_Integration(t *testing.T) {
	// F043 implementation enables nested loops


	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "while_parent.log")
	counterFile := filepath.Join(tmpDir, "counter")

	// while loop inside for_each, accessing parent's for_each context
	wfYAML := `name: while-in-foreach-parent
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["batch1", "batch2"]'
    max_iterations: 10
    body:
      - init_counter
      - count_loop
    on_complete: done
  init_counter:
    type: step
    command: 'echo "0" > ` + counterFile + `'
    on_success: outer_loop
  count_loop:
    type: while
    while: 'true'
    max_iterations: 3
    body:
      - increment
    on_complete: outer_loop
  increment:
    type: step
    command: |
      COUNT=$(cat ` + counterFile + `)
      NEW=$((COUNT + 1))
      echo $NEW > ` + counterFile + `
      echo "batch={{.loop.Parent.Item}} iter={{.loop.Index1}}" >> ` + logFile + `
    on_success: count_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "while-in-foreach-parent.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "while-in-foreach-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// 2 batches * 3 iterations each = 6 lines
	expected := `batch=batch1 iter=1
batch=batch1 iter=2
batch=batch1 iter=3
batch=batch2 iter=1
batch=batch2 iter=2
batch=batch2 iter=3
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_ParentNilSafe_Integration verifies that accessing Parent
// on top-level loop returns safely (no panic, empty/default value).
func TestF043_NestedLoops_ParentNilSafe_Integration(t *testing.T) {
	// F043 implementation enables nested loops


	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "nil_parent.log")

	// Single-level loop accessing Parent (should be nil/safe)
	wfYAML := `name: single-loop-parent
version: "1.0.0"
states:
  initial: process
  process:
    type: for_each
    items: '["item1", "item2"]'
    max_iterations: 10
    body:
      - log_parent
    on_complete: done
  log_parent:
    type: step
    command: 'echo "item={{.loop.Item}} parent={{.loop.Parent}}" >> ` + logFile + `'
    on_success: process
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "single-loop-parent.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "single-loop-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Should complete without panicking
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Go templates render nil as <no value> or empty string
	lines := string(data)
	assert.Contains(t, lines, "item=item1")
	assert.Contains(t, lines, "item=item2")
}

// =============================================================================
// F043: Additional Nested Loop Integration Tests
// =============================================================================
// Feature: F043
// These tests cover edge cases, error handling, and combined feature scenarios
// for nested loop execution with parent context access.

// TestF043_NestedLoops_EmptyInnerLoop_Integration tests that an empty inner loop
// correctly completes without executing body and restores outer loop context.
func TestF043_NestedLoops_EmptyInnerLoop_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "empty_inner.log")

	// Outer loop has items, inner loop is empty - should skip inner body
	wfYAML := `name: nested-empty-inner
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["A", "B"]'
    max_iterations: 10
    body:
      - log_before
      - inner_loop
      - log_after
    on_complete: done
  log_before:
    type: step
    command: 'echo "BEFORE: outer={{.loop.Item}}" >> ` + logFile + `'
    on_success: outer_loop
  inner_loop:
    type: for_each
    items: '[]'
    max_iterations: 10
    body:
      - inner_step
    on_complete: outer_loop
  inner_step:
    type: step
    command: 'echo "INNER: should not appear" >> ` + logFile + `'
    on_success: inner_loop
  log_after:
    type: step
    command: 'echo "AFTER: outer={{.loop.Item}} (context preserved)" >> ` + logFile + `'
    on_success: outer_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-empty-inner.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-empty-inner", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Inner loop body should NOT appear, outer context should be preserved
	expected := `BEFORE: outer=A
AFTER: outer=A (context preserved)
BEFORE: outer=B
AFTER: outer=B (context preserved)
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_InnerBreak_Integration tests break_when in nested loops.
// When inner loop breaks via max_iterations, outer loop should continue normally.
func TestF043_NestedLoops_InnerBreak_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "inner_break.log")

	// Inner while loop limited by max_iterations, outer foreach continues
	wfYAML := `name: nested-inner-break
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["batch1", "batch2"]'
    max_iterations: 10
    body:
      - inner_loop
    on_complete: done
  inner_loop:
    type: while
    while: 'true'
    max_iterations: 3
    body:
      - process
    on_complete: outer_loop
  process:
    type: step
    command: 'echo "{{.loop.Parent.Item}}/iter{{.loop.Index1}}" >> ` + logFile + `'
    on_success: inner_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-inner-break.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-inner-break", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Inner loop limited to 3 iterations per batch
	expected := `batch1/iter1
batch1/iter2
batch1/iter3
batch2/iter1
batch2/iter2
batch2/iter3
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_ErrorInInner_Propagate_Integration tests that errors in
// inner loop can propagate to outer loop's on_failure handler.
func TestF043_NestedLoops_ErrorInInner_Propagate_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "error_propagate.log")

	// Inner loop step fails, error propagates to outer loop's on_failure
	wfYAML := `name: nested-error-propagate
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["batch1", "batch2"]'
    max_iterations: 10
    body:
      - inner_loop
    on_complete: success_done
    on_failure: error_handler
  inner_loop:
    type: for_each
    items: '["ok", "fail", "never"]'
    max_iterations: 10
    body:
      - process
    on_complete: outer_loop
  process:
    type: step
    command: |
      echo "Processing {{.loop.Parent.Item}}/{{.loop.Item}}" >> ` + logFile + `
      test "{{.loop.Item}}" != "fail"
    on_success: inner_loop
  error_handler:
    type: step
    command: 'echo "ERROR HANDLED" >> ` + logFile + `'
    on_success: error_done
  success_done:
    type: terminal
    status: success
  error_done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-error-propagate.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-error-propagate", nil)

	require.NoError(t, err) // Workflow completes via error handler
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Should process ok, fail on "fail", then handle error
	content := string(data)
	assert.Contains(t, content, "Processing batch1/ok")
	assert.Contains(t, content, "Processing batch1/fail")
	// "never" should not appear because error occurs at "fail"
	assert.NotContains(t, content, "Processing batch1/never")
	assert.Contains(t, content, "ERROR HANDLED")
}

// TestF043_NestedLoops_FourLevelDeep_Integration tests 4 levels of nested loops
// to validate arbitrary depth support with parent chain access.
func TestF043_NestedLoops_FourLevelDeep_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "four_level.log")

	// 4 levels: country -> city -> street -> house
	wfYAML := `name: four-level-nested
version: "1.0.0"
states:
  initial: country_loop
  country_loop:
    type: for_each
    items: '["US", "UK"]'
    max_iterations: 10
    body:
      - city_loop
    on_complete: done
  city_loop:
    type: for_each
    items: '["NYC", "LA"]'
    max_iterations: 10
    body:
      - street_loop
    on_complete: country_loop
  street_loop:
    type: for_each
    items: '["Main"]'
    max_iterations: 10
    body:
      - house_loop
    on_complete: city_loop
  house_loop:
    type: for_each
    items: '["101", "102"]'
    max_iterations: 10
    body:
      - log_address
    on_complete: street_loop
  log_address:
    type: step
    command: 'echo "{{.loop.Parent.Parent.Parent.Item}}/{{.loop.Parent.Parent.Item}}/{{.loop.Parent.Item}}/{{.loop.Item}}" >> ` + logFile + `'
    on_success: house_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "four-level-nested.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "four-level-nested", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// 2 countries * 2 cities * 1 street * 2 houses = 8 lines
	expected := `US/NYC/Main/101
US/NYC/Main/102
US/LA/Main/101
US/LA/Main/102
UK/NYC/Main/101
UK/NYC/Main/102
UK/LA/Main/101
UK/LA/Main/102
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_ParentIndex1Arithmetic_Integration tests using Parent.Index1
// in arithmetic expressions within shell commands.
func TestF043_NestedLoops_ParentIndex1Arithmetic_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "arithmetic.log")

	// Use parent indices for matrix-like row/column numbering
	wfYAML := `name: nested-arithmetic
version: "1.0.0"
states:
  initial: row_loop
  row_loop:
    type: for_each
    items: '["R1", "R2", "R3"]'
    max_iterations: 10
    body:
      - col_loop
    on_complete: done
  col_loop:
    type: for_each
    items: '["C1", "C2"]'
    max_iterations: 10
    body:
      - log_cell
    on_complete: row_loop
  log_cell:
    type: step
    command: 'echo "Cell[{{.loop.Parent.Index1}},{{.loop.Index1}}]={{.loop.Parent.Item}}-{{.loop.Item}} position=$(({{.loop.Parent.Index}} * 2 + {{.loop.Index}}))" >> ` + logFile + `'
    on_success: col_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-arithmetic.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-arithmetic", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Verify arithmetic with indices: position = row_idx * 2 + col_idx
	expected := `Cell[1,1]=R1-C1 position=0
Cell[1,2]=R1-C2 position=1
Cell[2,1]=R2-C1 position=2
Cell[2,2]=R2-C2 position=3
Cell[3,1]=R3-C1 position=4
Cell[3,2]=R3-C2 position=5
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_MixedForEachWhile_ParentAccess_Integration tests nested
// for_each inside while loop with parent access to while's context.
func TestF043_NestedLoops_MixedForEachWhile_ParentAccess_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "mixed_while_foreach.log")
	counterFile := filepath.Join(tmpDir, "counter")

	// While loop as outer, for_each as inner
	wfYAML := `name: mixed-while-foreach
version: "1.0.0"
states:
  initial: while_loop
  while_loop:
    type: while
    while: 'true'
    max_iterations: 2
    body:
      - inner_foreach
    on_complete: done
  inner_foreach:
    type: for_each
    items: '["a", "b"]'
    max_iterations: 10
    body:
      - log_mixed
    on_complete: while_loop
  log_mixed:
    type: step
    command: 'echo "while_iter={{.loop.Parent.Index1}} foreach_item={{.loop.Item}}" >> ` + logFile + `'
    on_success: inner_foreach
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "mixed-while-foreach.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(counterFile, []byte("0"), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "mixed-while-foreach", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// 2 while iterations * 2 foreach items = 4 lines
	expected := `while_iter=1 foreach_item=a
while_iter=1 foreach_item=b
while_iter=2 foreach_item=a
while_iter=2 foreach_item=b
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_DynamicInnerItems_Integration tests inner loop with items
// derived from outer loop context (using states).
func TestF043_NestedLoops_DynamicInnerItems_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "dynamic_inner.log")

	// Each batch generates different items for inner loop via captured output
	wfYAML := `name: nested-dynamic-items
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["2", "3"]'
    max_iterations: 10
    body:
      - generate_items
      - inner_loop
    on_complete: done
  generate_items:
    type: step
    command: |
      COUNT={{.loop.Item}}
      ITEMS="["
      for i in $(seq 1 $COUNT); do
        [ "$i" -gt 1 ] && ITEMS="$ITEMS,"
        ITEMS="$ITEMS\"item$i\""
      done
      ITEMS="$ITEMS]"
      echo "$ITEMS"
    capture:
      stdout: items
    on_success: outer_loop
  inner_loop:
    type: for_each
    items: '{{.states.generate_items.Output}}'
    max_iterations: 10
    body:
      - log_item
    on_complete: outer_loop
  log_item:
    type: step
    command: 'echo "batch={{.loop.Parent.Item}} item={{.loop.Item}}" >> ` + logFile + `'
    on_success: inner_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-dynamic-items.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-dynamic-items", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Batch "2" generates ["item1", "item2"], batch "3" generates ["item1", "item2", "item3"]
	expected := `batch=2 item=item1
batch=2 item=item2
batch=3 item=item1
batch=3 item=item2
batch=3 item=item3
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_ParentLengthInCondition_Integration tests using Parent.Length
// in template conditions within nested loops.
func TestF043_NestedLoops_ParentLengthInCondition_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "parent_length.log")

	// Use Parent.Length to format output differently
	wfYAML := `name: nested-parent-length
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["A", "B", "C"]'
    max_iterations: 10
    body:
      - inner_loop
    on_complete: done
  inner_loop:
    type: for_each
    items: '["1", "2"]'
    max_iterations: 10
    body:
      - log_with_length
    on_complete: outer_loop
  log_with_length:
    type: step
    command: 'echo "[outer={{.loop.Parent.Index1}}/{{.loop.Parent.Length}}][inner={{.loop.Index1}}/{{.loop.Length}}] {{.loop.Parent.Item}}{{.loop.Item}}" >> ` + logFile + `'
    on_success: inner_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-parent-length.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-parent-length", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `[outer=1/3][inner=1/2] A1
[outer=1/3][inner=2/2] A2
[outer=2/3][inner=1/2] B1
[outer=2/3][inner=2/2] B2
[outer=3/3][inner=1/2] C1
[outer=3/3][inner=2/2] C2
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_MaxIterationsInner_Integration tests that max_iterations
// is correctly enforced on inner loops without affecting outer loop.
func TestF043_NestedLoops_MaxIterationsInner_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "max_iter_inner.log")

	// Inner while loop limited to 2 iterations, outer foreach completes normally
	wfYAML := `name: nested-max-iter-inner
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["batch1", "batch2"]'
    max_iterations: 10
    body:
      - inner_while
    on_complete: done
  inner_while:
    type: while
    while: 'true'
    max_iterations: 2
    body:
      - log_inner
    on_complete: outer_loop
  log_inner:
    type: step
    command: 'echo "{{.loop.Parent.Item}}: iter={{.loop.Index1}}" >> ` + logFile + `'
    on_success: inner_while
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-max-iter-inner.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-max-iter-inner", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Each batch has 2 iterations (max_iterations=2)
	expected := `batch1: iter=1
batch1: iter=2
batch2: iter=1
batch2: iter=2
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_SingleItemBothLoops_Integration tests edge case where
// both outer and inner loops have single items.
func TestF043_NestedLoops_SingleItemBothLoops_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "single_both.log")

	// Both loops have single items - First and Last should both be true
	wfYAML := `name: nested-single-both
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["only_outer"]'
    max_iterations: 10
    body:
      - inner_loop
    on_complete: done
  inner_loop:
    type: for_each
    items: '["only_inner"]'
    max_iterations: 10
    body:
      - log_flags
    on_complete: outer_loop
  log_flags:
    type: step
    command: 'echo "outer={{.loop.Parent.Item}}(first={{.loop.Parent.First}},last={{.loop.Parent.Last}}) inner={{.loop.Item}}(first={{.loop.First}},last={{.loop.Last}})" >> ` + logFile + `'
    on_success: inner_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-single-both.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-single-both", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Single item means both First and Last are true
	expected := `outer=only_outer(first=true,last=true) inner=only_inner(first=true,last=true)
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_MultipleBodyStepsWithParent_Integration tests nested loops
// where inner loop has multiple body steps all accessing parent context.
func TestF043_NestedLoops_MultipleBodyStepsWithParent_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "multi_body_parent.log")

	// Inner loop with multiple body steps accessing parent
	wfYAML := `name: nested-multi-body-parent
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["A", "B"]'
    max_iterations: 10
    body:
      - inner_loop
    on_complete: done
  inner_loop:
    type: for_each
    items: '["1", "2"]'
    max_iterations: 10
    body:
      - step1
      - step2
      - step3
    on_complete: outer_loop
  step1:
    type: step
    command: 'echo "S1: outer={{.loop.Parent.Item}} inner={{.loop.Item}}" >> ` + logFile + `'
    on_success: inner_loop
  step2:
    type: step
    command: 'echo "S2: parent_idx={{.loop.Parent.Index1}} inner_idx={{.loop.Index1}}" >> ` + logFile + `'
    on_success: inner_loop
  step3:
    type: step
    command: 'echo "S3: done with {{.loop.Parent.Item}}/{{.loop.Item}}" >> ` + logFile + `'
    on_success: inner_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-multi-body-parent.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-multi-body-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	expected := `S1: outer=A inner=1
S2: parent_idx=1 inner_idx=1
S3: done with A/1
S1: outer=A inner=2
S2: parent_idx=1 inner_idx=2
S3: done with A/2
S1: outer=B inner=1
S2: parent_idx=2 inner_idx=1
S3: done with B/1
S1: outer=B inner=2
S2: parent_idx=2 inner_idx=2
S3: done with B/2
`
	assert.Equal(t, expected, string(data))
}

// TestF043_NestedLoops_ParentAccessAfterInnerComplete_Integration verifies that
// after inner loop completes, outer loop context is accessible in next outer step.
func TestF043_NestedLoops_ParentAccessAfterInnerComplete_Integration(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "after_inner.log")

	// Outer loop body: step before inner, inner loop, step after inner
	// Step after inner should have correct outer context (not inner's context)
	wfYAML := `name: nested-after-inner
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: for_each
    items: '["X", "Y"]'
    max_iterations: 10
    body:
      - before
      - inner_loop
      - after
    on_complete: done
  before:
    type: step
    command: 'echo "BEFORE outer={{.loop.Item}} idx={{.loop.Index}}" >> ` + logFile + `'
    on_success: outer_loop
  inner_loop:
    type: for_each
    items: '["1", "2", "3"]'
    max_iterations: 10
    body:
      - inner_step
    on_complete: outer_loop
  inner_step:
    type: step
    command: 'echo "  INNER={{.loop.Item}} parent={{.loop.Parent.Item}}" >> ` + logFile + `'
    on_success: inner_loop
  after:
    type: step
    command: 'echo "AFTER outer={{.loop.Item}} idx={{.loop.Index}} (restored)" >> ` + logFile + `'
    on_success: outer_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "nested-after-inner.yaml"), []byte(wfYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "nested-after-inner", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// After inner completes, outer context should be restored (not showing inner's values)
	expected := `BEFORE outer=X idx=0
  INNER=1 parent=X
  INNER=2 parent=X
  INNER=3 parent=X
AFTER outer=X idx=0 (restored)
BEFORE outer=Y idx=1
  INNER=1 parent=Y
  INNER=2 parent=Y
  INNER=3 parent=Y
AFTER outer=Y idx=1 (restored)
`
	assert.Equal(t, expected, string(data))
}

// =============================================================================
// F048: Loop Body Transitions Integration Tests (T012)
// =============================================================================

// TestF048_WhileLoop_BodyTransitions_SkipSteps tests transitions within while loop body
// GIVEN a while loop with body steps containing transitions
// WHEN a transition condition is met within the body
// THEN subsequent steps should be skipped according to the transition target
func TestF048_WhileLoop_BodyTransitions_SkipSteps(t *testing.T) {

	// Item: T012
	// Feature: F048
	// Test happy path: Transitions within while loop body should skip subsequent steps

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "execution.log")

	wfYAML := `name: test-while-transitions-skip
version: "1.0.0"
states:
  initial: green_loop
  green_loop:
    type: while
    while: 'true'
    break_when: 'states.run_fmt.Output contains "STOP_LOOP"'
    max_iterations: 3
    body:
      - run_tests_green
      - check_tests_passed
      - prepare_impl_prompt
      - implement_item
      - run_fmt
    on_complete: done
  run_tests_green:
    type: step
    command: echo "TEST_EXIT_CODE=0" > /tmp/awf-test-output.txt && echo "Running tests..." >> ` + logFile + `
    on_success: green_loop
  check_tests_passed:
    type: step
    command: |
      if grep -q "TEST_EXIT_CODE=0" /tmp/awf-test-output.txt 2>/dev/null; then
        echo "TESTS_PASSED"
      else
        echo "TESTS_FAILED"
      fi
    transitions:
      - when: 'states.check_tests_passed.Output contains "TESTS_PASSED"'
        goto: run_fmt
      - goto: prepare_impl_prompt
    on_success: green_loop
  prepare_impl_prompt:
    type: step
    command: echo "Preparing implementation prompt..." >> ` + logFile + `
    on_success: green_loop
  implement_item:
    type: step
    command: echo "Implementing..." >> ` + logFile + `
    on_success: green_loop
  run_fmt:
    type: step
    command: echo "STOP_LOOP" && echo "Running formatter..." >> ` + logFile + `
    on_success: green_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test-while-transitions-skip.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newF048TransitionsEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "test-while-transitions-skip", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify execution log
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	logContent := string(data)

	// Assertions:
	// 1. run_tests_green should execute
	assert.Contains(t, logContent, "Running tests...")

	// 2. prepare_impl_prompt and implement_item should NOT execute (skipped via transition)
	assert.NotContains(t, logContent, "Preparing implementation prompt...")
	assert.NotContains(t, logContent, "Implementing...")

	// 3. run_fmt should execute (transition target)
	assert.Contains(t, logContent, "Running formatter...")

	// Edge case: Verify state was saved for check_tests_passed
	assert.Contains(t, execCtx.States, "check_tests_passed")
	assert.Equal(t, "TESTS_PASSED\n", execCtx.States["check_tests_passed"].Output)
}

// TestF048_WhileLoop_BodyTransitions_EarlyExit tests early exit from loop via transition
// GIVEN a while loop with a transition to a step outside the loop body
// WHEN that transition condition is met
// THEN the loop should exit early and continue to the external target step
func TestF048_WhileLoop_BodyTransitions_EarlyExit(t *testing.T) {

	// Item: T012
	// Feature: F048
	// Test edge case: Transition to external step should exit loop early

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "execution.log")

	wfYAML := `name: test-while-early-exit
version: "1.0.0"
states:
  initial: loop_with_exit
  loop_with_exit:
    type: while
    while: 'true'
    max_iterations: 5
    body:
      - step1
      - step2_with_exit
      - step3
    on_complete: normal_done
  step1:
    type: step
    command: echo "Step 1 executed" >> ` + logFile + `
    on_success: loop_with_exit
  step2_with_exit:
    type: step
    command: echo "EARLY_EXIT" && echo "Step 2 executed" >> ` + logFile + `
    transitions:
      - when: 'states.step2_with_exit.Output contains "EARLY_EXIT"'
        goto: early_exit_done
    on_success: loop_with_exit
  step3:
    type: step
    command: echo "Step 3 executed" >> ` + logFile + `
    on_success: loop_with_exit
  early_exit_done:
    type: terminal
    status: success
    message: "Early exit from loop"
  normal_done:
    type: terminal
    status: success
    message: "Normal completion"
`
	err := os.WriteFile(filepath.Join(tmpDir, "test-while-early-exit.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newF048TransitionsEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "test-while-early-exit", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "early_exit_done", execCtx.CurrentStep)

	// Verify execution log
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	logContent := string(data)

	// Assertions:
	// 1. step1 should execute once
	assert.Contains(t, logContent, "Step 1 executed")

	// 2. step2_with_exit should execute once
	assert.Contains(t, logContent, "Step 2 executed")

	// 3. step3 should NOT execute (loop exited early)
	assert.NotContains(t, logContent, "Step 3 executed")

	// Edge case: Verify only one iteration happened (early exit)
	// Count occurrences of "Step 1 executed" - should be exactly 1
	count := strings.Count(logContent, "Step 1 executed")
	assert.Equal(t, 1, count, "Expected exactly one iteration before early exit")
}

// TestF048_ForEachLoop_BodyTransitions_SkipSteps tests transitions within foreach loop body
// GIVEN a for_each loop with body steps containing transitions
// WHEN a transition condition is met within the body
// THEN subsequent steps should be skipped according to the transition target
func TestF048_ForEachLoop_BodyTransitions_SkipSteps(t *testing.T) {

	// Item: T012
	// Feature: F048
	// Test happy path: Transitions within foreach loop body should work like while

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "foreach-execution.log")

	wfYAML := `name: test-foreach-transitions
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["item1", "item2", "item3"]'
    max_iterations: 10
    body:
      - validate_item
      - check_validation
      - process_heavy
      - finalize
    on_complete: done
  validate_item:
    type: step
    command: echo "Validating {{.loop.Item}}..." >> ` + logFile + `
    on_success: process_items
  check_validation:
    type: step
    command: |
      if [ "{{.loop.Item}}" = "item1" ] || [ "{{.loop.Item}}" = "item3" ]; then
        echo "PASSED"
      else
        echo "FAILED"
      fi
    transitions:
      - when: 'states.check_validation.Output contains "PASSED"'
        goto: finalize
      - goto: process_heavy
    on_success: process_items
  process_heavy:
    type: step
    command: echo "Heavy processing {{.loop.Item}}..." >> ` + logFile + `
    on_success: process_items
  finalize:
    type: step
    command: echo "Finalizing {{.loop.Item}}" >> ` + logFile + `
    on_success: process_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test-foreach-transitions.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newF048TransitionsEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "test-foreach-transitions", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify execution log
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	logContent := string(data)

	// Assertions:
	// 1. All items should be validated
	assert.Contains(t, logContent, "Validating item1...")
	assert.Contains(t, logContent, "Validating item2...")
	assert.Contains(t, logContent, "Validating item3...")

	// 2. item1 and item3 are PASSED, so process_heavy should be skipped for them
	// Only item2 should go through heavy processing
	assert.Contains(t, logContent, "Heavy processing item2...")
	assert.NotContains(t, logContent, "Heavy processing item1...")
	assert.NotContains(t, logContent, "Heavy processing item3...")

	// 3. All items should be finalized
	assert.Contains(t, logContent, "Finalizing item1")
	assert.Contains(t, logContent, "Finalizing item2")
	assert.Contains(t, logContent, "Finalizing item3")

	// Edge case: Verify transition logic worked per-iteration
	// Count heavy processing - should be exactly 1 (only item2)
	heavyCount := 0
	searchStr := "Heavy processing"
	for i := 0; i <= len(logContent)-len(searchStr); i++ {
		if logContent[i:i+len(searchStr)] == searchStr {
			heavyCount++
		}
	}
	assert.Equal(t, 1, heavyCount, "Expected heavy processing only for item2")
}

// TestF048_WhileLoop_BodyTransitions_InvalidTarget tests graceful degradation for invalid targets
// GIVEN a while loop with a transition to a non-existent step
// WHEN that transition is evaluated
// THEN it should log a warning and continue sequential execution (graceful degradation)
func TestF048_WhileLoop_BodyTransitions_InvalidTarget(t *testing.T) {

	// Item: T012
	// Feature: F048
	// Test error handling: Invalid transition target should degrade gracefully

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "invalid-target.log")

	wfYAML := `name: test-invalid-target
version: "1.0.0"
states:
  initial: loop_with_invalid
  loop_with_invalid:
    type: while
    while: 'true'
    max_iterations: 2
    body:
      - step1
      - step2_invalid
      - step3
    on_complete: done
  step1:
    type: step
    command: echo "Step 1" >> ` + logFile + `
    on_success: loop_with_invalid
  step2_invalid:
    type: step
    command: echo "TRIGGER_INVALID" && echo "Step 2" >> ` + logFile + `
    transitions:
      - when: 'states.step2_invalid.Output contains "TRIGGER_INVALID"'
        goto: non_existent_step
      - goto: loop_with_invalid
    on_success: loop_with_invalid
  step3:
    type: step
    command: echo "Step 3" >> ` + logFile + `
    on_success: loop_with_invalid
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test-invalid-target.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newF048TransitionsEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "test-invalid-target", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify execution log - graceful degradation means sequential execution continues
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	logContent := string(data)

	// Assertions:
	// 1. All steps should execute (graceful degradation)
	assert.Contains(t, logContent, "Step 1")
	assert.Contains(t, logContent, "Step 2")
	assert.Contains(t, logContent, "Step 3")

	// 2. Should complete max_iterations (2 iterations)
	// Count "Step 1" occurrences - should be 2
	count := 0
	searchStr := "Step 1"
	for i := 0; i <= len(logContent)-len(searchStr); i++ {
		if logContent[i:i+len(searchStr)] == searchStr {
			count++
		}
	}
	assert.Equal(t, 2, count, "Expected 2 iterations with graceful degradation")

	// Edge case: Warning should be logged (implementation will verify this)
	// Note: Actual warning logging will be implemented in the feature
	// This test verifies graceful continuation despite invalid target
}

// TestF048_WhileLoop_BodyTransitions_BackwardCompatibility tests loops without transitions
// GIVEN a while loop without any transitions in body steps
// WHEN the loop executes
// THEN it should work exactly as before (sequential execution)
func TestF048_WhileLoop_BodyTransitions_BackwardCompatibility(t *testing.T) {

	// Item: T012
	// Feature: F048
	// Test backward compatibility: Loops without transitions should work unchanged

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "compat.log")

	wfYAML := `name: test-backward-compat
version: "1.0.0"
states:
  initial: simple_loop
  simple_loop:
    type: while
    while: 'true'
    break_when: 'states.counter.Output contains "stop"'
    max_iterations: 3
    body:
      - step_a
      - step_b
      - counter
    on_complete: done
  step_a:
    type: step
    command: echo "A" >> ` + logFile + `
    on_success: simple_loop
  step_b:
    type: step
    command: echo "B" >> ` + logFile + `
    on_success: simple_loop
  counter:
    type: step
    command: |
      COUNT=$(grep -c "A" ` + logFile + ` 2>/dev/null || echo "0")
      if [ "$COUNT" -lt "2" ]; then
        echo "continue"
      else
        echo "stop"
      fi
    on_success: simple_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test-backward-compat.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newF048TransitionsEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "test-backward-compat", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify execution log
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	logContent := string(data)

	// Assertions:
	// 1. Sequential execution - all steps in order
	assert.Contains(t, logContent, "A")
	assert.Contains(t, logContent, "B")

	// 2. Count iterations - should be 2 (counter stops at 2)
	countA := 0
	for i := 0; i < len(logContent); i++ {
		if logContent[i] == 'A' && (i == 0 || logContent[i-1] == '\n') {
			countA++
		}
	}
	assert.Equal(t, 2, countA, "Expected 2 iterations")

	// Edge case: Verify no transitions were applied (sequential order preserved)
	// Expected pattern: A\nB\n (repeated)
	lines := make([]string, 0)
	for _, line := range []string{"A", "B", "A", "B"} {
		if len(logContent) > 0 {
			lines = append(lines, line)
		}
	}
	// Simple check: first line should be A, second should be B
	logLines := make([]string, 0)
	currentLine := ""
	for i := 0; i < len(logContent); i++ {
		if logContent[i] == '\n' {
			if currentLine != "" {
				logLines = append(logLines, currentLine)
				currentLine = ""
			}
		} else {
			currentLine += string(logContent[i])
		}
	}
	if currentLine != "" {
		logLines = append(logLines, currentLine)
	}

	if len(logLines) >= 4 {
		assert.Equal(t, "A", logLines[0])
		assert.Equal(t, "B", logLines[1])
		assert.Equal(t, "A", logLines[2])
		assert.Equal(t, "B", logLines[3])
	}
}
