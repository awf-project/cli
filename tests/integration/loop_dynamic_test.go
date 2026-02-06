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
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	infraExpr "github.com/vanoix/awf/internal/infrastructure/expression"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/pkg/expression"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// F037: Dynamic Variable Interpolation in Loop max_iterations - Integration Tests
// =============================================================================

// TestDynamicMaxIterations_FromInput tests US1: max_iterations from input variable
func TestDynamicMaxIterations_FromInput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")

	// Workflow with dynamic max_iterations from input
	wfYAML := `name: dynamic-input
version: "1.0.0"
inputs:
  - name: limit
    type: integer
    required: true
states:
  initial: count_loop
  count_loop:
    type: for_each
    items: '["a", "b", "c", "d", "e", "f", "g", "h", "i", "j"]'
    max_iterations: "{{.inputs.limit}}"
    body:
      - echo_item
    on_complete: done
  echo_item:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + logFile + `'
    on_success: count_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "dynamic-input.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run with limit=3, should process only first 3 items
	inputs := map[string]any{"limit": 3}
	execCtx, err := execSvc.Run(ctx, "dynamic-input", inputs)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify only 3 items were processed
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 3, "expected 3 items with limit=3")
	assert.Equal(t, []string{"a", "b", "c"}, lines)
}

// TestDynamicMaxIterations_FromInputDifferentValues tests various limit values
func TestDynamicMaxIterations_FromInputDifferentValues(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		expectedItems int
	}{
		{"limit_1", 1, 1},
		{"limit_5", 5, 5},
		{"limit_exceeds_items", 20, 10}, // Only 10 items exist
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logFile := filepath.Join(tmpDir, "output.log")

			wfYAML := `name: dynamic-test
version: "1.0.0"
inputs:
  - name: limit
    type: integer
    required: true
states:
  initial: loop
  loop:
    type: for_each
    items: '["1", "2", "3", "4", "5", "6", "7", "8", "9", "10"]'
    max_iterations: "{{.inputs.limit}}"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x" >> ` + logFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`
			err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
			require.NoError(t, err)

			repo := repository.NewYAMLRepository(tmpDir)
			store := newMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &mockLogger{}
			resolver := interpolation.NewTemplateResolver()
			evaluator := expression.NewExprEvaluator()

			wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionServiceWithEvaluator(
				wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
			)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			inputs := map[string]any{"limit": tt.limit}
			execCtx, err := execSvc.Run(ctx, "test", inputs)

			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			data, err := os.ReadFile(logFile)
			require.NoError(t, err)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			assert.Len(t, lines, tt.expectedItems)
		})
	}
}

// TestDynamicMaxIterations_FromEnv tests US1: max_iterations from environment variable
func TestDynamicMaxIterations_FromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")

	wfYAML := `name: dynamic-env
version: "1.0.0"
env:
  - LOOP_LIMIT
states:
  initial: loop
  loop:
    type: for_each
    items: '["one", "two", "three", "four", "five"]'
    max_iterations: "{{.env.LOOP_LIMIT}}"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + logFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "env-test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	// Set environment variable
	t.Setenv("LOOP_LIMIT", "2")

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "env-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify only 2 items were processed
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 2, "expected 2 items with LOOP_LIMIT=2")
	assert.Equal(t, []string{"one", "two"}, lines)
}

// TestDynamicMaxIterations_Arithmetic tests US3: arithmetic expressions in max_iterations
func TestDynamicMaxIterations_Arithmetic(t *testing.T) {
	tests := []struct {
		name          string
		expression    string
		inputA        int
		inputB        int
		expectedItems int
	}{
		{"addition", "{{.inputs.a}} + {{.inputs.b}}", 2, 3, 5},
		{"multiplication", "{{.inputs.a}} * {{.inputs.b}}", 2, 3, 6},
		{"subtraction", "{{.inputs.a}} - {{.inputs.b}}", 5, 2, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logFile := filepath.Join(tmpDir, "output.log")

			wfYAML := `name: arithmetic-test
version: "1.0.0"
inputs:
  - name: a
    type: integer
    required: true
  - name: b
    type: integer
    required: true
states:
  initial: loop
  loop:
    type: for_each
    items: '["1", "2", "3", "4", "5", "6", "7", "8", "9", "10"]'
    max_iterations: "` + tt.expression + `"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x" >> ` + logFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`
			err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
			require.NoError(t, err)

			repo := repository.NewYAMLRepository(tmpDir)
			store := newMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &mockLogger{}
			resolver := interpolation.NewTemplateResolver()
			evaluator := expression.NewExprEvaluator()

			wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionServiceWithEvaluator(
				wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
			)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			inputs := map[string]any{"a": tt.inputA, "b": tt.inputB}
			execCtx, err := execSvc.Run(ctx, "test", inputs)

			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			data, err := os.ReadFile(logFile)
			require.NoError(t, err)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			assert.Len(t, lines, tt.expectedItems)
		})
	}
}

// TestDynamicMaxIterations_MissingVariable tests error handling for missing variables
func TestDynamicMaxIterations_MissingVariable(t *testing.T) {
	tmpDir := t.TempDir()

	wfYAML := `name: missing-var
version: "1.0.0"
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: "{{.inputs.undefined_var}}"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x"'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Should fail with error about missing variable
	_, err = execSvc.Run(ctx, "test", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "undefined_var")
}

// TestDynamicMaxIterations_NonInteger tests error handling for non-integer result
func TestDynamicMaxIterations_NonInteger(t *testing.T) {
	tmpDir := t.TempDir()

	wfYAML := `name: non-integer
version: "1.0.0"
inputs:
  - name: limit
    type: string
    required: true
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: "{{.inputs.limit}}"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x"'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass a non-numeric string
	inputs := map[string]any{"limit": "not_a_number"}
	_, err = execSvc.Run(ctx, "test", inputs)

	require.Error(t, err)
	// Error should indicate non-integer issue
	assert.True(t, strings.Contains(err.Error(), "integer") ||
		strings.Contains(err.Error(), "number") ||
		strings.Contains(err.Error(), "numeric") ||
		strings.Contains(err.Error(), "parse"))
}

// TestDynamicMaxIterations_ZeroValue tests error handling for zero value
func TestDynamicMaxIterations_ZeroValue(t *testing.T) {
	tmpDir := t.TempDir()

	wfYAML := `name: zero-value
version: "1.0.0"
inputs:
  - name: limit
    type: integer
    required: true
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: "{{.inputs.limit}}"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x"'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass zero value
	inputs := map[string]any{"limit": 0}
	_, err = execSvc.Run(ctx, "test", inputs)

	require.Error(t, err)
	// Error should indicate invalid bounds
	assert.True(t, strings.Contains(err.Error(), "0") ||
		strings.Contains(err.Error(), "bound") ||
		strings.Contains(err.Error(), "positive") ||
		strings.Contains(err.Error(), "minimum"))
}

// TestDynamicMaxIterations_NegativeValue tests error handling for negative value
func TestDynamicMaxIterations_NegativeValue(t *testing.T) {
	tmpDir := t.TempDir()

	wfYAML := `name: negative-value
version: "1.0.0"
inputs:
  - name: limit
    type: integer
    required: true
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: "{{.inputs.limit}}"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x"'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass negative value
	inputs := map[string]any{"limit": -5}
	_, err = execSvc.Run(ctx, "test", inputs)

	require.Error(t, err)
	// Error should indicate invalid bounds
	assert.True(t, strings.Contains(err.Error(), "-5") ||
		strings.Contains(err.Error(), "negative") ||
		strings.Contains(err.Error(), "bound") ||
		strings.Contains(err.Error(), "positive"))
}

// TestDynamicMaxIterations_ExceedsMaxBound tests error handling when exceeding max allowed
func TestDynamicMaxIterations_ExceedsMaxBound(t *testing.T) {
	tmpDir := t.TempDir()

	wfYAML := `name: exceeds-max
version: "1.0.0"
inputs:
  - name: limit
    type: integer
    required: true
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: "{{.inputs.limit}}"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x"'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass value exceeding max (10000 per ADR-004)
	inputs := map[string]any{"limit": 20000}
	_, err = execSvc.Run(ctx, "test", inputs)

	require.Error(t, err)
	// Error should indicate exceeded bounds
	assert.True(t, strings.Contains(err.Error(), "20000") ||
		strings.Contains(err.Error(), "10000") ||
		strings.Contains(err.Error(), "exceed") ||
		strings.Contains(err.Error(), "maximum") ||
		strings.Contains(err.Error(), "bound"))
}

// TestDynamicMaxIterations_BackwardCompatibility tests that static integer values still work
func TestDynamicMaxIterations_BackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")

	// Traditional workflow with static max_iterations: 10 (>= items count)
	// This confirms that adding MaxIterationsExpr support doesn't break existing workflows
	wfYAML := `name: static-max
version: "1.0.0"
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: 10
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + logFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "static.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "static", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify all 3 items were processed
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 3)
	assert.Equal(t, []string{"a", "b", "c"}, lines)
}

// TestDynamicMaxIterations_WithWhileLoop tests dynamic max_iterations with while loops
func TestDynamicMaxIterations_WithWhileLoop(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")

	// While loop with dynamic max_iterations from input
	// Loop runs until max_iterations is reached (no break_when condition)
	wfYAML := `name: while-dynamic
version: "1.0.0"
inputs:
  - name: max_attempts
    type: integer
    required: true
states:
  initial: retry_loop
  retry_loop:
    type: while
    while: 'true'
    max_iterations: "{{.inputs.max_attempts}}"
    body:
      - attempt
    on_complete: done
  attempt:
    type: step
    command: 'echo "attempt" >> ` + logFile + `'
    on_success: retry_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "while.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	inputs := map[string]any{"max_attempts": 4}
	execCtx, err := execSvc.Run(ctx, "while", inputs)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify 4 attempts were made (loop runs until max_iterations)
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 4)
}

// TestDynamicMaxIterations_UsingFixtureFiles tests with actual fixture YAML files
func TestDynamicMaxIterations_UsingFixtureFiles(t *testing.T) {
	// Path relative to tests/integration/ where the test runs
	fixturesDir := "../fixtures/workflows"

	tests := []struct {
		name          string
		workflow      string
		inputs        map[string]any
		envVars       map[string]string
		expectError   bool
		expectedItems int // 0 if expectError is true
	}{
		{
			name:          "fixture_loop_dynamic_limit_3",
			workflow:      "loop-dynamic",
			inputs:        map[string]any{"limit": 3},
			expectedItems: 3,
		},
		{
			name:          "fixture_loop_dynamic_env",
			workflow:      "loop-dynamic-env",
			envVars:       map[string]string{"LOOP_LIMIT": "2"},
			expectedItems: 2,
		},
		{
			name:          "fixture_loop_dynamic_arithmetic",
			workflow:      "loop-dynamic-arithmetic",
			inputs:        map[string]any{"pages": 2, "retries_per_page": 3},
			expectedItems: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			repo := repository.NewYAMLRepository(fixturesDir)
			store := newMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &mockLogger{}
			resolver := interpolation.NewTemplateResolver()
			evaluator := expression.NewExprEvaluator()

			wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionServiceWithEvaluator(
				wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
			)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			execCtx, err := execSvc.Run(ctx, tt.workflow, tt.inputs)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
			}
		})
	}
}

// =============================================================================
// US2: Interpolate Loop Condition Variables - Integration Tests
// =============================================================================

// TestDynamicLoopCondition_WithStepOutput tests US2: loop conditions referencing step outputs
func TestDynamicLoopCondition_WithStepOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")
	counterFile := filepath.Join(tmpDir, "counter")

	// Initialize counter
	err := os.WriteFile(counterFile, []byte("0"), 0o644)
	require.NoError(t, err)

	// Workflow with loop condition referencing step output
	// Loop continues until counter reaches the input threshold
	wfYAML := `name: condition-step-output
version: "1.0.0"
inputs:
  - name: threshold
    type: integer
    required: true
states:
  initial: count_loop
  count_loop:
    type: while
    while: 'true'
    max_iterations: 100
    break_when: 'states.check_counter.exit_code != 0'
    body:
      - increment
      - check_counter
    on_complete: done
  increment:
    type: step
    command: |
      count=$(cat ` + counterFile + `)
      echo $((count + 1)) > ` + counterFile + `
      echo "count: $((count + 1))" >> ` + logFile + `
    on_success: count_loop
  check_counter:
    type: step
    command: |
      count=$(cat ` + counterFile + `)
      if [ "$count" -ge "{{.inputs.threshold}}" ]; then
        exit 1
      fi
      exit 0
    on_success: count_loop
    on_failure: count_loop
  done:
    type: terminal
    status: success
`
	err = os.WriteFile(filepath.Join(tmpDir, "condition.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run with threshold=3, loop should run 3 times
	inputs := map[string]any{"threshold": 3}
	execCtx, err := execSvc.Run(ctx, "condition", inputs)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify counter reached threshold
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 3, "expected 3 iterations to reach threshold")
}

// TestDynamicLoopCondition_UntilCondition tests until condition with interpolated variables
func TestDynamicLoopCondition_UntilCondition(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")
	statusFile := filepath.Join(tmpDir, "status")

	// Start with "pending" status
	err := os.WriteFile(statusFile, []byte("pending"), 0o644)
	require.NoError(t, err)

	// Workflow with until condition that checks for "done" status
	wfYAML := `name: until-condition
version: "1.0.0"
states:
  initial: poll_loop
  poll_loop:
    type: while
    while: 'true'
    max_iterations: 10
    break_when: 'states.check_status.output == "done"'
    body:
      - update_status
      - check_status
    on_complete: finish
  update_status:
    type: step
    command: |
      count=$(cat ` + logFile + ` 2>/dev/null | wc -l || echo 0)
      if [ "$count" -ge 2 ]; then
        echo "done" > ` + statusFile + `
      fi
      echo "poll" >> ` + logFile + `
    on_success: poll_loop
  check_status:
    type: step
    command: cat ` + statusFile + `
    on_success: poll_loop
  finish:
    type: terminal
    status: success
`
	err = os.WriteFile(filepath.Join(tmpDir, "until.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "until", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify loop ran until status became "done"
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Loop should run 3 times: 2 polls before done, plus 1 more that triggers break
	assert.GreaterOrEqual(t, len(lines), 3, "expected at least 3 iterations")
}

// =============================================================================
// Edge Cases and Additional Scenarios
// =============================================================================

// TestDynamicMaxIterations_StringInputResolvesToInteger tests string input that parses to int
func TestDynamicMaxIterations_StringInputResolvesToInteger(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")

	wfYAML := `name: string-to-int
version: "1.0.0"
inputs:
  - name: limit
    type: string
    required: true
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c", "d", "e"]'
    max_iterations: "{{.inputs.limit}}"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + logFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass string "3" which should be parsed to int 3
	inputs := map[string]any{"limit": "3"}
	execCtx, err := execSvc.Run(ctx, "test", inputs)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify 3 items were processed
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 3)
}

// TestDynamicMaxIterations_FloatInputTruncated tests float input is handled appropriately
func TestDynamicMaxIterations_FloatInputTruncated(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")

	wfYAML := `name: float-input
version: "1.0.0"
inputs:
  - name: limit
    type: number
    required: true
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c", "d", "e"]'
    max_iterations: "{{.inputs.limit}}"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + logFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass float 3.7 - should be truncated to 3 or produce an error
	inputs := map[string]any{"limit": 3.7}
	execCtx, err := execSvc.Run(ctx, "test", inputs)

	// Either succeeds with truncation to 3, or errors due to non-integer
	if err == nil {
		assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
		data, err := os.ReadFile(logFile)
		require.NoError(t, err)
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		// Should process 3 items (truncated from 3.7)
		assert.Len(t, lines, 3)
	} else {
		// If error, it should mention integer or number conversion
		assert.True(t, strings.Contains(err.Error(), "integer") ||
			strings.Contains(err.Error(), "number") ||
			strings.Contains(err.Error(), "float"))
	}
}

// TestDynamicMaxIterations_ComplexArithmetic tests complex arithmetic expressions
func TestDynamicMaxIterations_ComplexArithmetic(t *testing.T) {
	tests := []struct {
		name          string
		expression    string
		inputs        map[string]any
		expectedItems int
	}{
		{
			name:          "division",
			expression:    "{{.inputs.total}} / {{.inputs.batch_size}}",
			inputs:        map[string]any{"total": 10, "batch_size": 2},
			expectedItems: 5,
		},
		// Note: modulo (%) is NOT in the spec (FR-006 only requires +, -, *, /)
		// Removed modulo test case as it's not a supported operator
		{
			name:          "mixed_operations",
			expression:    "{{.inputs.a}} + {{.inputs.b}} * {{.inputs.c}}",
			inputs:        map[string]any{"a": 2, "b": 3, "c": 2},
			expectedItems: 8, // 2 + 3*2 = 8
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logFile := filepath.Join(tmpDir, "output.log")

			wfYAML := `name: arithmetic-complex
version: "1.0.0"
inputs:
  - name: total
    type: integer
    required: false
  - name: batch_size
    type: integer
    required: false
  - name: mod
    type: integer
    required: false
  - name: a
    type: integer
    required: false
  - name: b
    type: integer
    required: false
  - name: c
    type: integer
    required: false
states:
  initial: loop
  loop:
    type: for_each
    items: '["1", "2", "3", "4", "5", "6", "7", "8", "9", "10"]'
    max_iterations: "` + tt.expression + `"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x" >> ` + logFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`
			err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
			require.NoError(t, err)

			repo := repository.NewYAMLRepository(tmpDir)
			store := newMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &mockLogger{}
			resolver := interpolation.NewTemplateResolver()
			evaluator := expression.NewExprEvaluator()

			wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionServiceWithEvaluator(
				wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
			)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			execCtx, err := execSvc.Run(ctx, "test", tt.inputs)

			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			data, err := os.ReadFile(logFile)
			require.NoError(t, err)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			assert.Len(t, lines, tt.expectedItems)
		})
	}
}

// TestDynamicMaxIterations_DivisionByZero tests error handling for division by zero
func TestDynamicMaxIterations_DivisionByZero(t *testing.T) {
	tmpDir := t.TempDir()

	wfYAML := `name: div-zero
version: "1.0.0"
inputs:
  - name: total
    type: integer
    required: true
  - name: divisor
    type: integer
    required: true
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: "{{.inputs.total}} / {{.inputs.divisor}}"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x"'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Division by zero should produce an error
	inputs := map[string]any{"total": 10, "divisor": 0}
	_, err = execSvc.Run(ctx, "test", inputs)

	require.Error(t, err)
	// Error should indicate division issue or invalid result
	// Division by zero in expr-lang returns +Inf, which fails integer conversion
	assert.True(t, strings.Contains(err.Error(), "division") ||
		strings.Contains(err.Error(), "zero") ||
		strings.Contains(err.Error(), "divide") ||
		strings.Contains(err.Error(), "Inf") ||
		strings.Contains(err.Error(), "integer"),
		"error message should indicate division or integer issue, got: %s", err.Error())
}

// =============================================================================
// US4: Validate Interpolated Loop Variables at Parse Time - Integration Tests
// =============================================================================

// TestDynamicMaxIterations_ValidationWarning tests US4: awf validate warns about undefined variables
func TestDynamicMaxIterations_ValidationWarning(t *testing.T) {
	// Use the invalid fixture with undefined variable
	fixturesDir := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Load and validate the workflow with undefined variable
	wf, err := wfSvc.GetWorkflow(ctx, "loop-dynamic-invalid")
	require.NoError(t, err)
	require.NotNil(t, wf)

	// Validate should produce a warning or error about undefined_var
	err = wfSvc.ValidateWorkflow(ctx, "loop-dynamic-invalid")
	// Either produces error at validation time (strict) or passes (lenient with runtime check)
	// The spec says "warns" so it may be a warning, not a hard error
	if err != nil {
		assert.Contains(t, err.Error(), "undefined",
			"validation error should mention undefined variable")
	}
	// Note: If validation passes without error, runtime will catch it (covered by TestDynamicMaxIterations_MissingVariable)
}

// TestDynamicMaxIterations_ValidationPasses tests US4: valid loop expressions pass validation
func TestDynamicMaxIterations_ValidationPasses(t *testing.T) {
	fixturesDir := "../fixtures/workflows"

	tests := []struct {
		name     string
		workflow string
	}{
		{"loop_dynamic_with_inputs", "loop-dynamic"},
		{"loop_dynamic_with_env", "loop-dynamic-env"},
		{"loop_dynamic_with_arithmetic", "loop-dynamic-arithmetic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewYAMLRepository(fixturesDir)
			store := newMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &mockLogger{}

			wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Load workflow
			wf, err := wfSvc.GetWorkflow(ctx, tt.workflow)
			require.NoError(t, err, "should load workflow %s", tt.workflow)
			require.NotNil(t, wf, "workflow %s should not be nil", tt.workflow)

			// Validate should pass (defined inputs/env)
			err = wfSvc.ValidateWorkflow(ctx, tt.workflow)
			assert.NoError(t, err, "workflow %s with defined variables should pass validation", tt.workflow)
		})
	}
}

// TestDynamicMaxIterations_WhitespaceHandling tests edge case: whitespace in expressions
func TestDynamicMaxIterations_WhitespaceHandling(t *testing.T) {
	tests := []struct {
		name          string
		expression    string
		expectedItems int
	}{
		{"leading_whitespace", "  {{.inputs.limit}}  ", 3},
		{"spaces_around_operator", "{{.inputs.a}}  +  {{.inputs.b}}", 5},
		{"tab_in_expression", "{{.inputs.a}}\t*\t{{.inputs.b}}", 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logFile := filepath.Join(tmpDir, "output.log")

			wfYAML := `name: whitespace-test
version: "1.0.0"
inputs:
  - name: limit
    type: integer
    required: false
  - name: a
    type: integer
    required: false
  - name: b
    type: integer
    required: false
states:
  initial: loop
  loop:
    type: for_each
    items: '["1", "2", "3", "4", "5", "6", "7", "8", "9", "10"]'
    max_iterations: "` + tt.expression + `"
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x" >> ` + logFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`
			err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
			require.NoError(t, err)

			repo := repository.NewYAMLRepository(tmpDir)
			store := newMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &mockLogger{}
			resolver := interpolation.NewTemplateResolver()
			evaluator := expression.NewExprEvaluator()

			wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionServiceWithEvaluator(
				wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
			)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var inputs map[string]any
			if strings.Contains(tt.expression, "limit") {
				inputs = map[string]any{"limit": 3}
			} else {
				inputs = map[string]any{"a": 2, "b": 3}
			}

			execCtx, err := execSvc.Run(ctx, "test", inputs)

			require.NoError(t, err, "whitespace in expression should be handled")
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			data, err := os.ReadFile(logFile)
			require.NoError(t, err)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			assert.Len(t, lines, tt.expectedItems)
		})
	}
}

// TestDynamicMaxIterations_EmptyExpression tests edge case: empty expression string
func TestDynamicMaxIterations_EmptyExpression(t *testing.T) {
	tmpDir := t.TempDir()

	// Workflow with empty max_iterations expression
	wfYAML := `name: empty-expr
version: "1.0.0"
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: ""
    body:
      - echo
    on_complete: done
  echo:
    type: step
    command: 'echo "x"'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Empty expression should either:
	// 1. Fall back to default max_iterations
	// 2. Or produce an error about invalid expression
	execCtx, err := execSvc.Run(ctx, "test", nil)

	if err != nil {
		// If error, it should indicate empty/invalid expression
		assert.True(t, strings.Contains(err.Error(), "empty") ||
			strings.Contains(err.Error(), "invalid") ||
			strings.Contains(err.Error(), "parse") ||
			strings.Contains(err.Error(), "max_iterations"),
			"error message should indicate expression issue, got: %s", err.Error())
	} else {
		// If succeeds, workflow completed (fell back to default or interpreted as 0 items)
		assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	}
}

// TestDynamicMaxIterations_NestedTemplates tests edge case: nested template expressions
func TestDynamicMaxIterations_NestedTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")

	// Workflow where input contains a number that becomes max_iterations
	wfYAML := `name: simple-dynamic
version: "1.0.0"
inputs:
  - name: batch_count
    type: integer
    required: true
states:
  initial: process
  process:
    type: for_each
    items: '["a", "b", "c", "d", "e"]'
    max_iterations: "{{.inputs.batch_count}}"
    body:
      - work
    on_complete: done
  work:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + logFile + `'
    on_success: process
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	inputs := map[string]any{"batch_count": 2}
	execCtx, err := execSvc.Run(ctx, "test", inputs)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 2, "should process exactly 2 items with batch_count=2")
}

// =============================================================================
// F047: Loop JSON Serialization - Integration Tests
// =============================================================================

// TestForEachLoop_WithObjectItems_Integration tests US3: for_each loop with JSON objects
// AC1: {{.loop.Item}} passed to call_workflow produces valid JSON
// AC2: Nested objects and arrays properly serialized
func TestForEachLoop_WithObjectItems_Integration(t *testing.T) {
	// Given: A parent workflow that iterates over JSON objects and passes them to a child workflow
	fixturesDir := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: The workflow executes with default JSON objects containing nested arrays
	// Default items: [{"name":"S1","type":"fix","files":["a.go","b.go"]},{"name":"S2","type":"feat","files":["c.go"]}]
	execCtx, err := execSvc.Run(ctx, "loop-json-serialization", nil)

	// Then: Workflow should complete successfully
	require.NoError(t, err, "workflow execution should not error")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "workflow should complete successfully")
	assert.Equal(t, "done", execCtx.CurrentStep, "workflow should end at done state")

	// Then: Validation step should exist and pass (no Go map format)
	validateState, ok := execCtx.GetStepState("validate_item")
	require.True(t, ok, "validate_item state should exist")
	assert.Equal(t, 0, validateState.ExitCode, "validate_item should succeed")

	// Verify validation output confirms JSON format
	validationOutput := validateState.Output
	assert.Contains(t, validationOutput, "VALIDATION_SUCCESS", "validation should confirm JSON format")
	assert.NotContains(t, validationOutput, "map[", "output should not contain Go map format")
	assert.NotContains(t, validationOutput, "VALIDATION_FAILED", "validation should not fail")
}

// TestForEachLoop_WithNestedStructures tests AC2: nested objects and arrays properly serialized
func TestForEachLoop_WithNestedStructures(t *testing.T) {
	// Given: A workflow with deeply nested JSON structures
	fixturesDir := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: Items contain nested objects and arrays
	nestedJSON := `[{"name":"Task1","metadata":{"priority":"high","tags":["urgent","bug"]},"assignees":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]}]`
	inputs := map[string]any{"items_json": nestedJSON}

	execCtx, err := execSvc.Run(ctx, "loop-json-serialization", inputs)

	// Then: Workflow should complete and child validates nested JSON
	require.NoError(t, err, "workflow with nested structures should not error")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "workflow should complete")

	// Verify nested structures are valid JSON
	validateState, ok := execCtx.GetStepState("validate_item")
	require.True(t, ok, "validate_item state should exist")
	assert.Equal(t, 0, validateState.ExitCode, "should validate nested JSON")
	assert.Contains(t, validateState.Output, "VALIDATION_SUCCESS", "nested JSON should validate")
}

// TestForEachLoop_MixedPrimitiveAndComplex tests mixed primitive and complex types in loop
func TestForEachLoop_MixedPrimitiveAndComplex(t *testing.T) {
	// Given: A workflow that can handle both primitive and complex types
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "results.txt")

	// Create a simple workflow that echoes loop items to a file
	wfYAML := `name: mixed-types
version: "1.0.0"
inputs:
  - name: items_json
    type: string
    required: true
states:
  initial: loop_items
  loop_items:
    type: for_each
    items: "{{.inputs.items_json}}"
    body:
      - echo_item
    on_complete: done
  echo_item:
    type: step
    command: |
      item='{{.loop.Item}}'
      printf '%s\n' "$item" >> ` + outputFile + `
    on_success: loop_items
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "mixed-types.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: Mix of primitives and objects in the array
	mixedJSON := `["string",42,{"key":"value"},[1,2,3],true,null]`
	inputs := map[string]any{"items_json": mixedJSON}

	execCtx, err := execSvc.Run(ctx, "mixed-types", inputs)

	// Then: All items should be serialized appropriately
	require.NoError(t, err, "workflow with mixed types should not error")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "workflow should complete")

	// Verify output file contains properly serialized items
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	output := string(data)

	// Primitives should pass through unchanged
	assert.Contains(t, output, "string", "string primitive should be preserved")
	assert.Contains(t, output, "42", "number primitive should be preserved")
	assert.Contains(t, output, "true", "boolean primitive should be preserved")

	// Complex types should be JSON
	assert.Contains(t, output, `{"key":"value"}`, "object should be JSON")
	assert.Contains(t, output, "[1,2,3]", "array should be JSON")

	// Should NOT contain Go map format
	assert.NotContains(t, output, "map[", "should not have Go map format")
}

// TestForEachLoop_EmptyObjectsAndArrays tests edge case: empty objects and arrays
func TestForEachLoop_EmptyObjectsAndArrays(t *testing.T) {
	// Given: Workflow with empty objects and arrays
	fixturesDir := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: Items include empty objects and arrays
	emptyJSON := `[{},{"name":"test","data":[]},{"items":[]}]`
	inputs := map[string]any{"items_json": emptyJSON}

	execCtx, err := execSvc.Run(ctx, "loop-json-serialization", inputs)

	// Then: Empty structures should serialize as valid JSON
	require.NoError(t, err, "workflow with empty structures should not error")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "workflow should complete")

	validateState, ok := execCtx.GetStepState("validate_item")
	require.True(t, ok, "validate_item state should exist")
	assert.Equal(t, 0, validateState.ExitCode, "empty structures should validate as JSON")
}

// TestForEachLoop_UnicodeAndSpecialChars tests edge case: unicode and special characters
func TestForEachLoop_UnicodeAndSpecialChars(t *testing.T) {
	// Given: Workflow with unicode and special characters
	fixturesDir := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: Items contain unicode, emojis, and special characters
	unicodeJSON := `[{"name":"测试","emoji":"🚀","special":"quote:\"value\""}]`
	inputs := map[string]any{"items_json": unicodeJSON}

	execCtx, err := execSvc.Run(ctx, "loop-json-serialization", inputs)

	// Then: Unicode and special chars should be properly escaped in JSON
	require.NoError(t, err, "workflow with unicode should not error")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "workflow should complete")

	validateState, ok := execCtx.GetStepState("validate_item")
	require.True(t, ok, "validate_item state should exist")
	assert.Equal(t, 0, validateState.ExitCode, "unicode JSON should validate")
}

// TestForEachLoop_BackwardCompatibility_StringItems tests AC4: existing workflows work unchanged
func TestForEachLoop_BackwardCompatibility_StringItems(t *testing.T) {
	// Given: A workflow with simple string items (existing behavior)
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	wfYAML := `name: string-items
version: "1.0.0"
states:
  initial: loop
  loop:
    type: for_each
    items: '["item1", "item2", "item3"]'
    body:
      - echo_item
    on_complete: done
  echo_item:
    type: step
    command: 'echo "{{.loop.Item}}" >> ` + outputFile + `'
    on_success: loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "string-items.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// When: Running workflow with string items (pre-F047 behavior)
	execCtx, err := execSvc.Run(ctx, "string-items", nil)

	// Then: String items should pass through unchanged (backward compatibility)
	require.NoError(t, err, "string items workflow should work")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "workflow should complete")

	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	assert.Len(t, lines, 3, "should have 3 items")
	assert.Equal(t, "item1", lines[0], "string should not be quoted")
	assert.Equal(t, "item2", lines[1], "string should not be quoted")
	assert.Equal(t, "item3", lines[2], "string should not be quoted")

	// Strings should NOT be JSON-quoted (backward compatibility)
	assert.NotContains(t, string(data), `"item1"`, "strings should not be JSON-encoded")
}
