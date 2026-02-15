//go:build integration

package loops_test

// Feature: B003
//
// This file contains functional tests for bug B003: break_when condition evaluation in while loops.
// Bug description: break_when conditions in while loops were not being evaluated correctly,
// causing loops to continue through all iterations even when break conditions were satisfied.
//
// Root cause: Integration tests used simpleExpressionEvaluator mock which only recognized
// hardcoded values ("ready", "stop") and used lowercase keys (output, exit_code) instead of
// PascalCase (Output, ExitCode) per B001.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/awf/internal/infrastructure/expression"
	"github.com/awf-project/awf/internal/infrastructure/repository"
	"github.com/awf-project/awf/pkg/interpolation"
)

// TestBreakWhen_ExactStringMatch_Integration tests break_when with exact string equality.
// This is the primary reproduction case from B003 bug report.
//
// Given: A while loop with break_when checking for exact Output match
// When: Step outputs the expected string value
// Then: Loop should break after first successful iteration
func TestBreakWhen_ExactStringMatch_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	iterationFile := filepath.Join(tmpDir, "iterations.log")

	wfYAML := `name: break-exact-match
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 5
    break_when: 'states.check_status.Output == "SUCCESS"'
    body:
      - check_status
    on_complete: done
  check_status:
    type: step
    command: |
      echo "iteration" >> ` + iterationFile + `
      # Always output SUCCESS
      printf "%s" "SUCCESS"
    capture:
      stdout: Output
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "break-exact-match.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(iterationFile, []byte{}, 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "break-exact-match", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify loop broke after first iteration
	data, err := os.ReadFile(iterationFile)
	require.NoError(t, err)
	assert.Equal(t, "iteration\n", string(data),
		"Loop should break after first iteration when Output == 'SUCCESS'")

	// Verify state captured correctly
	state, exists := execCtx.States["check_status"]
	require.True(t, exists)
	assert.Equal(t, "SUCCESS", state.Output)
}

// TestBreakWhen_ContainsOperator_Integration tests break_when with contains operator.
//
// Given: A while loop with break_when using contains operator
// When: Step output contains the substring
// Then: Loop should break immediately
func TestBreakWhen_ContainsOperator_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	iterationFile := filepath.Join(tmpDir, "iterations.log")

	wfYAML := `name: break-contains
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 5
    break_when: 'states.check.Output contains "PASS"'
    body:
      - check
    on_complete: done
  check:
    type: step
    command: |
      echo "iteration" >> ` + iterationFile + `
      printf "%s" "TESTS_PASSED_WITH_WARNINGS"
    capture:
      stdout: Output
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "break-contains.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(iterationFile, []byte{}, 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "break-contains", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(iterationFile)
	require.NoError(t, err)
	assert.Equal(t, "iteration\n", string(data),
		"Loop should break when Output contains 'PASS'")
}

// TestBreakWhen_ExitCodeCondition_Integration tests break_when with ExitCode checks.
//
// Given: A while loop with break_when checking ExitCode
// When: Step exits with matching exit code
// Then: Loop should break
func TestBreakWhen_ExitCodeCondition_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	iterationFile := filepath.Join(tmpDir, "iterations.log")

	wfYAML := `name: break-exitcode
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 5
    break_when: 'states.check.ExitCode == 0'
    body:
      - check
    on_complete: done
  check:
    type: step
    command: |
      echo "iteration" >> ` + iterationFile + `
      exit 0
    on_success: test_loop
    on_failure: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "break-exitcode.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(iterationFile, []byte{}, 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "break-exitcode", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(iterationFile)
	require.NoError(t, err)
	assert.Equal(t, "iteration\n", string(data),
		"Loop should break when ExitCode == 0")
}

// TestBreakWhen_EmptyOutput_Integration tests break_when with empty string output.
//
// Given: A while loop checking for empty Output
// When: Step produces no output
// Then: Loop should break
func TestBreakWhen_EmptyOutput_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	iterationFile := filepath.Join(tmpDir, "iterations.log")

	wfYAML := `name: break-empty
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 5
    break_when: 'states.check.Output == ""'
    body:
      - check
    on_complete: done
  check:
    type: step
    command: |
      echo "iteration" >> ` + iterationFile + `
      printf ""
    capture:
      stdout: Output
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "break-empty.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(iterationFile, []byte{}, 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "break-empty", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(iterationFile)
	require.NoError(t, err)
	assert.Equal(t, "iteration\n", string(data),
		"Loop should break when Output is empty string")
}

// TestBreakWhen_UnicodeOutput_Integration tests break_when with Unicode characters.
//
// Given: A while loop checking for Unicode output
// When: Step outputs Unicode string
// Then: Loop should break correctly
func TestBreakWhen_UnicodeOutput_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	iterationFile := filepath.Join(tmpDir, "iterations.log")

	wfYAML := `name: break-unicode
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 5
    break_when: 'states.check.Output == "✓ 成功"'
    body:
      - check
    on_complete: done
  check:
    type: step
    command: |
      echo "iteration" >> ` + iterationFile + `
      printf "%s" "✓ 成功"
    capture:
      stdout: Output
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "break-unicode.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(iterationFile, []byte{}, 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "break-unicode", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(iterationFile)
	require.NoError(t, err)
	assert.Equal(t, "iteration\n", string(data),
		"Loop should break with Unicode characters in Output")

	state, exists := execCtx.States["check"]
	require.True(t, exists)
	assert.Equal(t, "✓ 成功", state.Output)
}

// TestBreakWhen_MultilineOutput_Integration tests break_when with multiline output.
//
// Given: A while loop checking for multiline output
// When: Step outputs multiple lines
// Then: Loop should handle multiline comparison correctly
func TestBreakWhen_MultilineOutput_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	iterationFile := filepath.Join(tmpDir, "iterations.log")

	wfYAML := `name: break-multiline
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 5
    break_when: 'states.check.Output contains "SUCCESS"'
    body:
      - check
    on_complete: done
  check:
    type: step
    command: |
      echo "iteration" >> ` + iterationFile + `
      printf "Line 1\nLine 2: SUCCESS\nLine 3"
    capture:
      stdout: Output
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "break-multiline.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(iterationFile, []byte{}, 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "break-multiline", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(iterationFile)
	require.NoError(t, err)
	assert.Equal(t, "iteration\n", string(data),
		"Loop should break when multiline Output contains expected string")
}

// TestBreakWhen_MaxIterationsNotReached_Integration verifies loop doesn't run to max when break_when satisfied early.
//
// Given: A while loop with max_iterations=10 and break_when condition
// When: break_when becomes true on iteration 2
// Then: Loop should stop at iteration 2, not run all 10
func TestBreakWhen_MaxIterationsNotReached_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	iterationFile := filepath.Join(tmpDir, "iterations.log")

	wfYAML := `name: break-early
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 10
    break_when: 'states.check.Output == "DONE"'
    body:
      - check
    on_complete: done
  check:
    type: step
    command: |
      echo "iteration" >> ` + iterationFile + `
      COUNT=$(wc -l < ` + iterationFile + ` 2>/dev/null || echo 0)
      if [ "$COUNT" -ge 2 ]; then
        printf "%s" "DONE"
      else
        printf "%s" "RUNNING"
      fi
    capture:
      stdout: Output
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "break-early.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(iterationFile, []byte{}, 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "break-early", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(iterationFile)
	require.NoError(t, err)
	// Should be exactly 2 iterations with newlines = "iteration\niteration\n"
	assert.Equal(t, "iteration\niteration\n", string(data),
		"Loop should stop at iteration 2, not run all 10 iterations")
}

// TestBreakWhen_NonexistentState_ContinuesLoop_Integration tests break_when referencing missing state.
//
// Given: A while loop with break_when referencing non-existent state
// When: Loop executes
// Then: Loop should continue (break_when evaluates to false for missing states)
func TestBreakWhen_NonexistentState_ContinuesLoop_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	iterationFile := filepath.Join(tmpDir, "iterations.log")

	wfYAML := `name: break-missing-state
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 3
    break_when: 'states.nonexistent_step.Output == "DONE"'
    body:
      - check
    on_complete: done
  check:
    type: step
    command: |
      echo "iteration" >> ` + iterationFile + `
      printf "OK"
    capture:
      stdout: Output
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "break-missing-state.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(iterationFile, []byte{}, 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "break-missing-state", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Should run all 3 iterations since break_when never evaluates to true
	data, err := os.ReadFile(iterationFile)
	require.NoError(t, err)
	assert.Equal(t, "iteration\niteration\niteration\n", string(data),
		"Loop should run all iterations when break_when references missing state")
}

// TestBreakWhen_WithMultipleBodySteps_Integration tests break_when evaluating after multi-step body.
//
// Given: A while loop with multiple steps in body and break_when checking last step
// When: All body steps complete
// Then: break_when should evaluate correctly using last step's state
func TestBreakWhen_WithMultipleBodySteps_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	iterationFile := filepath.Join(tmpDir, "iterations.log")

	wfYAML := `name: break-multi-step
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 5
    break_when: 'states.verify.Output == "VERIFIED"'
    body:
      - prepare
      - execute
      - verify
    on_complete: done
  prepare:
    type: step
    command: printf "prepared"
    capture:
      stdout: Output
    on_success: execute
  execute:
    type: step
    command: |
      echo "iteration" >> ` + iterationFile + `
      printf "executed"
    capture:
      stdout: Output
    on_success: verify
  verify:
    type: step
    command: printf "VERIFIED"
    capture:
      stdout: Output
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "break-multi-step.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(iterationFile, []byte{}, 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "break-multi-step", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(iterationFile)
	require.NoError(t, err)
	assert.Equal(t, "iteration\n", string(data),
		"Loop should break after first iteration when last body step satisfies break_when")

	// Verify all three steps executed and have correct state
	_, exists := execCtx.States["prepare"]
	require.True(t, exists, "prepare step should have executed")
	_, exists = execCtx.States["execute"]
	require.True(t, exists, "execute step should have executed")
	verifyState, exists := execCtx.States["verify"]
	require.True(t, exists, "verify step should have executed")
	assert.Equal(t, "VERIFIED", verifyState.Output)
}

// TestBreakWhen_NotEqualOperator_Integration tests break_when with != operator.
//
// Given: A while loop with break_when using != operator
// When: Output is not equal to expected value
// Then: Loop should break
func TestBreakWhen_NotEqualOperator_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	iterationFile := filepath.Join(tmpDir, "iterations.log")

	wfYAML := `name: break-not-equal
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 5
    break_when: 'states.check.Output != "RUNNING"'
    body:
      - check
    on_complete: done
  check:
    type: step
    command: |
      echo "iteration" >> ` + iterationFile + `
      printf "DONE"
    capture:
      stdout: Output
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "break-not-equal.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(iterationFile, []byte{}, 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "break-not-equal", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(iterationFile)
	require.NoError(t, err)
	assert.Equal(t, "iteration\n", string(data),
		"Loop should break when Output != 'RUNNING' evaluates to true")
}
