//go:build integration

package integration_test

import (
	"context"
	"fmt"
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
// F048 Loop Body Transitions - Integration Tests
// Consolidated from: f048_functional_test.go, loop_while_transitions_t009_test.go
// =============================================================================

// =============================================================================
// Feature: F048 - Loop Body Transitions
//
// Comprehensive functional tests validating that transitions defined in
// while/foreach loop body steps are honored correctly. These tests validate
// the complete feature from end-to-end using real dependencies.
//
// Reference: .specify/implementation/F048/spec-content.md
// =============================================================================

// =============================================================================
// Test Helpers
// =============================================================================

// f048TestStore is a simple in-memory state store for F048 functional tests
type f048TestStore struct {
	states map[string]*workflow.ExecutionContext
}

func newF048TestStore() *f048TestStore {
	return &f048TestStore{states: make(map[string]*workflow.ExecutionContext)}
}

func (s *f048TestStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	s.states[state.WorkflowID] = state
	return nil
}

func (s *f048TestStore) Load(ctx context.Context, id string) (*workflow.ExecutionContext, error) {
	if state, ok := s.states[id]; ok {
		return state, nil
	}
	return nil, nil
}

func (s *f048TestStore) Delete(ctx context.Context, id string) error {
	delete(s.states, id)
	return nil
}

func (s *f048TestStore) List(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(s.states))
	for id := range s.states {
		ids = append(ids, id)
	}
	return ids, nil
}

// f048TestLogger captures logs for assertion in tests
type f048TestLogger struct {
	logs []string
}

func (l *f048TestLogger) Debug(msg string, fields ...any) {
	if l.logs != nil {
		l.logs = append(l.logs, "DEBUG: "+msg)
	}
}

func (l *f048TestLogger) Info(msg string, fields ...any) {
	if l.logs != nil {
		l.logs = append(l.logs, "INFO: "+msg)
	}
}

func (l *f048TestLogger) Warn(msg string, fields ...any) {
	if l.logs != nil {
		l.logs = append(l.logs, "WARN: "+msg)
	}
}

func (l *f048TestLogger) Error(msg string, fields ...any) {
	if l.logs != nil {
		l.logs = append(l.logs, "ERROR: "+msg)
	}
}

func (l *f048TestLogger) WithContext(ctx map[string]any) ports.Logger {
	return l
}

// f048ExpressionEvaluator evaluates basic expressions for tests
type f048ExpressionEvaluator struct{}

func newF048ExpressionEvaluator() *f048ExpressionEvaluator {
	return &f048ExpressionEvaluator{}
}

func (e *f048ExpressionEvaluator) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	// Handle simple literals
	switch expr {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	// Handle contains expressions: 'states.X.Output contains "value"'
	if ctx != nil && ctx.States != nil {
		for stepName, state := range ctx.States {
			// Pattern: states.X.Output contains "VALUE"
			prefix := "states." + stepName + ".Output contains \""
			if strings.HasPrefix(expr, prefix) && strings.HasSuffix(expr, "\"") {
				searchValue := strings.TrimPrefix(expr, prefix)
				searchValue = strings.TrimSuffix(searchValue, "\"")
				return strings.Contains(state.Output, searchValue), nil
			}

			// Pattern: states.X.output contains "VALUE" (lowercase)
			prefixLower := "states." + stepName + ".output contains \""
			if strings.HasPrefix(expr, prefixLower) && strings.HasSuffix(expr, "\"") {
				searchValue := strings.TrimPrefix(expr, prefixLower)
				searchValue = strings.TrimSuffix(searchValue, "\"")
				return strings.Contains(state.Output, searchValue), nil
			}

			// Pattern: states.X.exit_code == 0
			if expr == "states."+stepName+".exit_code == 0" {
				return state.ExitCode == 0, nil
			}

			// Pattern: states.X.exit_code != 0
			if expr == "states."+stepName+".exit_code != 0" {
				return state.ExitCode != 0, nil
			}
		}
	}

	return false, nil
}

// setupF048Test creates a test environment with workflow service and dependencies
// Returns: execService, tmpDir, workflowName
func setupF048Test(t *testing.T, workflowYAML string) (*application.ExecutionService, string, string) {
	tmpDir := t.TempDir()

	// Extract workflow name from YAML (first line: "name: <workflow-name>")
	lines := strings.Split(workflowYAML, "\n")
	workflowName := "test"
	for _, line := range lines {
		if strings.HasPrefix(line, "name:") {
			workflowName = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			break
		}
	}

	// Write workflow to temp directory
	workflowFile := filepath.Join(tmpDir, workflowName+".yaml")
	err := os.WriteFile(workflowFile, []byte(workflowYAML), 0o644)
	require.NoError(t, err)

	// Create dependencies
	repo := repository.NewYAMLRepository(tmpDir)
	store := newF048TestStore()
	exec := executor.NewShellExecutor()
	logger := &f048TestLogger{logs: []string{}}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newF048ExpressionEvaluator()

	// Create services
	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	return execSvc, tmpDir, workflowName
}

// =============================================================================
// HAPPY PATH TESTS
// =============================================================================

// TestF048_HappyPath_SkipStepsInBody validates the main scenario from spec
//
// GIVEN a while loop with body steps containing transitions
// WHEN check_tests_passed outputs "TESTS_PASSED"
// THEN it should transition to run_fmt, skipping prepare_impl_prompt and implement_item
func TestF048_HappyPath_SkipStepsInBody(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	workflowYAML := `name: test-happy-path
version: "1.0.0"
states:
  initial: green_loop
  green_loop:
    type: while
    while: 'true'
    break_when: 'states.run_fmt.Output contains "STOP_LOOP"'
    max_iterations: 2
    body:
      - run_tests
      - check_results
      - prepare_prompt
      - implement
      - run_fmt
    on_complete: done
  run_tests:
    type: step
    command: 'echo "run_tests" >> ` + executionLog + `; echo "TESTS_RAN"'
    on_success: green_loop
  check_results:
    type: step
    command: 'echo "check_results" >> ` + executionLog + `; echo "TESTS_PASSED"'
    transitions:
      - when: 'states.check_results.Output contains "TESTS_PASSED"'
        goto: run_fmt
      - goto: prepare_prompt
  prepare_prompt:
    type: step
    command: 'echo "prepare_prompt" >> ` + executionLog + `; echo "PREPARED"'
    on_success: green_loop
  implement:
    type: step
    command: 'echo "implement" >> ` + executionLog + `; echo "IMPLEMENTED"'
    on_success: green_loop
  run_fmt:
    type: step
    command: 'echo "run_fmt" >> ` + executionLog + `; echo "STOP_LOOP"'
    on_success: green_loop
  done:
    type: terminal
    status: success
`

	execSvc, _, workflowName := setupF048Test(t, workflowYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act
	execCtx, err := execSvc.Run(ctx, workflowName, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify execution log - skipped steps should NOT appear
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	assert.Contains(t, output, "run_tests", "run_tests should execute")
	assert.Contains(t, output, "check_results", "check_results should execute")
	assert.Contains(t, output, "run_fmt", "run_fmt should execute (transition target)")
	assert.NotContains(t, output, "prepare_prompt", "prepare_prompt should be skipped")
	assert.NotContains(t, output, "implement", "implement should be skipped")
}

// TestF048_HappyPath_ForEachWithTransitions validates transitions work in foreach loops
// NOTE: This test is commented out because foreach is not yet supported.
// The test validates transitions within foreach loop bodies once the feature is available.
/*
func TestF048_HappyPath_ForEachWithTransitions(t *testing.T) {
	// Skipped - foreach not yet implemented
}
*/

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

// TestF048_EdgeCase_SkipToLastStep validates transition to the last step in body
//
// GIVEN a loop with transition to the last body step
// WHEN the transition is triggered
// THEN it should skip all intermediate steps
func TestF048_EdgeCase_SkipToLastStep(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	workflowYAML := `name: test-skip-to-last
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'states.step_last.Output contains "CONTINUE"'
    max_iterations: 2
    body:
      - step_first
      - step_middle1
      - step_middle2
      - step_last
    on_complete: done
  step_first:
    type: step
    command: 'echo "first" >> ` + executionLog + `'
    transitions:
      - goto: step_last
  step_middle1:
    type: step
    command: 'echo "middle1" >> ` + executionLog + `'
    on_success: test_loop
  step_middle2:
    type: step
    command: 'echo "middle2" >> ` + executionLog + `'
    on_success: test_loop
  step_last:
    type: step
    command: 'echo "last" >> ` + executionLog + `; echo "STOP"'
    on_success: test_loop
  done:
    type: terminal
    status: success
`

	execSvc, _, workflowName := setupF048Test(t, workflowYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act
	execCtx, err := execSvc.Run(ctx, workflowName, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	assert.Contains(t, output, "first")
	assert.Contains(t, output, "last")
	assert.NotContains(t, output, "middle1", "middle1 should be skipped")
	assert.NotContains(t, output, "middle2", "middle2 should be skipped")
}

// TestF048_EdgeCase_EarlyLoopExit validates transition outside loop body
//
// GIVEN a loop with transition to a step outside the body
// WHEN the transition is triggered
// THEN the loop should exit early and continue to the target step
func TestF048_EdgeCase_EarlyLoopExit(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	workflowYAML := `name: test-early-exit
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 10
    body:
      - check_condition
      - continue_loop
    on_complete: done
  check_condition:
    type: step
    command: 'echo "check" >> ` + executionLog + `; echo "EARLY_EXIT"'
    transitions:
      - when: 'states.check_condition.Output contains "EARLY_EXIT"'
        goto: cleanup
      - goto: continue_loop
  continue_loop:
    type: step
    command: 'echo "continue" >> ` + executionLog + `'
    on_success: test_loop
  cleanup:
    type: step
    command: 'echo "cleanup" >> ` + executionLog + `'
    on_success: done
  done:
    type: terminal
    status: success
`

	execSvc, _, workflowName := setupF048Test(t, workflowYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act
	execCtx, err := execSvc.Run(ctx, workflowName, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	assert.Contains(t, output, "check", "check should execute")
	assert.Contains(t, output, "cleanup", "cleanup should execute after early exit")
	assert.NotContains(t, output, "continue", "continue should be skipped")

	// Verify loop only ran once (early exit)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	checkCount := 0
	for _, line := range lines {
		if line == "check" {
			checkCount++
		}
	}
	assert.Equal(t, 1, checkCount, "loop should exit after first iteration")
}

// TestF048_EdgeCase_SingleStepBodyWithTransition validates single step body behavior
//
// GIVEN a loop with only one step in body
// WHEN that step has a transition outside the loop
// THEN the transition should be honored (early exit)
func TestF048_EdgeCase_SingleStepBodyWithTransition(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	workflowYAML := `name: test-single-step
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 5
    body:
      - single_step
    on_complete: done
  single_step:
    type: step
    command: 'echo "single" >> ` + executionLog + `; echo "EXIT"'
    transitions:
      - when: 'states.single_step.Output contains "EXIT"'
        goto: cleanup
      - goto: test_loop
  cleanup:
    type: step
    command: 'echo "cleanup" >> ` + executionLog + `'
    on_success: done
  done:
    type: terminal
    status: success
`

	execSvc, _, workflowName := setupF048Test(t, workflowYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act
	execCtx, err := execSvc.Run(ctx, workflowName, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 2, len(lines), "should execute single step once, then cleanup")
	assert.Equal(t, "single", lines[0])
	assert.Equal(t, "cleanup", lines[1])
}

// TestF048_EdgeCase_EmptyLoopBody validates behavior with empty body
// NOTE: Empty body is correctly rejected by validation - this is expected behavior
/*
func TestF048_EdgeCase_EmptyLoopBody(t *testing.T) {
	// Skipped - empty body is invalid and correctly rejected
}
*/

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

// TestF048_ErrorHandling_InvalidTransitionTarget validates graceful degradation
//
// GIVEN a loop with transition to non-existent step
// WHEN the transition is evaluated
// THEN it should log a warning and continue sequential execution (ADR-005)
func TestF048_ErrorHandling_InvalidTransitionTarget(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	workflowYAML := `name: test-invalid-target
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'states.step_b.Output contains "CONTINUE"'
    max_iterations: 2
    body:
      - step_a
      - step_b
    on_complete: done
  step_a:
    type: step
    command: 'echo "step_a" >> ` + executionLog + `'
    transitions:
      - goto: nonexistent_step
  step_b:
    type: step
    command: 'echo "step_b" >> ` + executionLog + `; echo "STOP"'
    on_success: test_loop
  done:
    type: terminal
    status: success
`

	execSvc, _, workflowName := setupF048Test(t, workflowYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act
	execCtx, err := execSvc.Run(ctx, workflowName, nil)

	// Assert - should complete despite invalid transition
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify both steps executed (fallback to sequential)
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	assert.Contains(t, output, "step_a")
	assert.Contains(t, output, "step_b")
}

// TestF048_ErrorHandling_TransitionWithError validates error propagation
//
// GIVEN a body step that fails with an error
// WHEN the step has transitions
// THEN the error should be propagated and transitions should not be evaluated
func TestF048_ErrorHandling_TransitionWithError(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	workflowYAML := `name: test-error-propagation
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'states.recovery.Output contains "CONTINUE"'
    max_iterations: 2
    body:
      - failing_step
      - should_not_run
    on_failure: recovery
    on_complete: done
  failing_step:
    type: step
    command: 'echo "failing" >> ` + executionLog + `; exit 1'
    transitions:
      - goto: should_not_run
  should_not_run:
    type: step
    command: 'echo "should_not_run" >> ` + executionLog + `'
    on_success: test_loop
  recovery:
    type: step
    command: 'echo "recovery" >> ` + executionLog + `; echo "STOP"'
    on_success: done
  done:
    type: terminal
    status: success
`

	execSvc, _, workflowName := setupF048Test(t, workflowYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act
	execCtx, err := execSvc.Run(ctx, workflowName, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	assert.Contains(t, output, "failing")
	assert.Contains(t, output, "recovery")
	assert.NotContains(t, output, "should_not_run", "should_not_run should not execute")
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

// TestF048_Integration_FixtureWorkflow validates the complete loop-while-transitions.yaml fixture
//
// GIVEN the official F048 test fixture workflow
// WHEN executed with passing tests
// THEN it should skip intermediate steps and complete successfully
// TestDeeplyNestedLoops_TransitionToParent validates 3+ level nested loops with transitions
//
// GIVEN a workflow with 3-level nested while loops
// WHEN innermost loop transitions to parent loop or outside
// THEN it should properly exit nested loops and continue to target
func TestDeeplyNestedLoops_TransitionToParent(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	workflowYAML := `name: deeply-nested-loops
version: "1.0.0"
states:
  initial: outer_loop
  outer_loop:
    type: while
    while: 'true'
    max_iterations: 2
    body:
      - outer_step
      - middle_loop
      - outer_end
    on_complete: done
  outer_step:
    type: step
    command: 'echo "outer_step" >> ` + executionLog + `'
    on_success: outer_loop
  middle_loop:
    type: while
    while: 'true'
    max_iterations: 2
    body:
      - middle_step
      - inner_loop
      - middle_end
    on_complete: outer_loop
  middle_step:
    type: step
    command: 'echo "middle_step" >> ` + executionLog + `'
    on_success: middle_loop
  inner_loop:
    type: while
    while: 'true'
    max_iterations: 2
    body:
      - inner_step
      - inner_check
    on_complete: middle_loop
  inner_step:
    type: step
    command: 'echo "inner_step" >> ` + executionLog + `'
    on_success: inner_loop
  inner_check:
    type: step
    command: 'echo "inner_check" >> ` + executionLog + `; echo "ESCAPE"'
    transitions:
      - when: 'states.inner_check.Output contains "ESCAPE"'
        goto: cleanup
      - goto: inner_loop
  middle_end:
    type: step
    command: 'echo "middle_end" >> ` + executionLog + `'
    on_success: middle_loop
  outer_end:
    type: step
    command: 'echo "outer_end" >> ` + executionLog + `'
    on_success: outer_loop
  cleanup:
    type: step
    command: 'echo "cleanup" >> ` + executionLog + `'
    on_success: done
  done:
    type: terminal
    status: success
`

	execSvc, _, workflowName := setupF048Test(t, workflowYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act
	execCtx, err := execSvc.Run(ctx, workflowName, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify execution log
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	// Verify nested loop execution
	assert.Contains(t, output, "outer_step", "outer loop should start")
	assert.Contains(t, output, "middle_step", "middle loop should execute")
	assert.Contains(t, output, "inner_step", "inner loop should execute")
	assert.Contains(t, output, "inner_check", "inner check should execute")
	assert.Contains(t, output, "cleanup", "should transition to cleanup from innermost loop")

	// Verify steps after transition are skipped
	assert.NotContains(t, output, "middle_end", "middle_end should be skipped after transition")
	assert.NotContains(t, output, "outer_end", "outer_end should be skipped after transition")
}

// TestLargeLoopBody_PerformanceValidation validates performance with large loop bodies
//
// GIVEN a loop with 100+ steps in the body
// WHEN the loop executes
// THEN it should complete within reasonable time (<5s) and handle transitions correctly
func TestLargeLoopBody_PerformanceValidation(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	// Build workflow with 100 steps in loop body
	var bodySteps []string
	for i := 1; i <= 100; i++ {
		bodySteps = append(bodySteps, "      - step_"+fmt.Sprintf("%d", i))
	}

	workflowYAML := `name: large-loop-body
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 1
    body:
` + strings.Join(bodySteps, "\n") + `
    on_complete: done
`

	// Add step definitions - first step has transition to skip to end
	for i := 1; i <= 100; i++ {
		stepName := fmt.Sprintf("step_%d", i)
		if i == 1 {
			// First step transitions to last step
			workflowYAML += `  ` + stepName + `:
    type: step
    command: 'echo "` + stepName + `" >> ` + executionLog + `'
    transitions:
      - goto: step_100
`
		} else if i == 100 {
			// Last step
			workflowYAML += `  ` + stepName + `:
    type: step
    command: 'echo "` + stepName + `" >> ` + executionLog + `'
    on_success: test_loop
`
		} else {
			// Middle steps (should be skipped)
			workflowYAML += `  ` + stepName + `:
    type: step
    command: 'echo "` + stepName + `" >> ` + executionLog + `'
    on_success: test_loop
`
		}
	}

	workflowYAML += `  done:
    type: terminal
    status: success
`

	execSvc, _, workflowName := setupF048Test(t, workflowYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Act - measure execution time
	start := time.Now()
	execCtx, err := execSvc.Run(ctx, workflowName, nil)
	duration := time.Since(start)

	// Assert - should complete successfully
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Assert - performance: should complete within 5 seconds
	// Since we skip 98 steps, it should be very fast
	assert.Less(t, duration, 5*time.Second, "execution should complete within 5 seconds")

	// Verify execution log
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	// Should execute step_1 and step_100 only
	assert.Contains(t, output, "step_1", "first step should execute")
	assert.Contains(t, output, "step_100", "last step should execute (transition target)")

	// Verify middle steps were skipped (sample a few)
	assert.NotContains(t, output, "step_2", "step_2 should be skipped")
	assert.NotContains(t, output, "step_50", "step_50 should be skipped")
	assert.NotContains(t, output, "step_99", "step_99 should be skipped")

	// Verify only 2 steps executed (not 100)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 2, len(lines), "should execute only 2 steps (first and last)")
}

func TestF048_Integration_FixtureWorkflow(t *testing.T) {
	// Load the actual fixture
	fixtureDir := "../../tests/fixtures/workflows"
	tmpDir := t.TempDir()
	testOutputFile := filepath.Join(tmpDir, "awf-test-output.txt")

	// Create test output file with passing test status
	err := os.WriteFile(testOutputFile, []byte("TEST_EXIT_CODE=0\n"), 0o644)
	require.NoError(t, err)

	// Read and modify fixture
	fixtureContent, err := os.ReadFile(filepath.Join(fixtureDir, "loop-while-transitions.yaml"))
	require.NoError(t, err)

	modifiedContent := strings.ReplaceAll(string(fixtureContent), "/tmp/awf-test-output.txt", testOutputFile)

	workflowName := "test-while-transitions"
	workflowFile := filepath.Join(tmpDir, workflowName+".yaml")
	err = os.WriteFile(workflowFile, []byte(modifiedContent), 0o644)
	require.NoError(t, err)

	// Setup services
	repo := repository.NewYAMLRepository(tmpDir)
	store := newF048TestStore()
	exec := executor.NewShellExecutor()
	logger := &f048TestLogger{logs: []string{}}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newF048ExpressionEvaluator()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act
	execCtx, err := execSvc.Run(ctx, workflowName, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify step execution - critical assertions from spec
	states := execCtx.States
	assert.Contains(t, states, "run_tests_green")
	assert.Contains(t, states, "check_tests_passed")
	assert.Contains(t, states, "run_fmt")
	assert.NotContains(t, states, "prepare_impl_prompt", "should skip when tests pass")
	assert.NotContains(t, states, "implement_item", "should skip when tests pass")
}

// TestF048_Integration_BackwardCompatibility validates loops without transitions still work
//
// GIVEN a loop without any transitions in body steps
// WHEN the loop executes
// THEN it should work exactly as before (sequential execution)
func TestF048_Integration_BackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	workflowYAML := `name: test-backward-compat
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'states.step_c.Output contains "CONTINUE"'
    max_iterations: 2
    body:
      - step_a
      - step_b
      - step_c
    on_complete: done
  step_a:
    type: step
    command: 'echo "a" >> ` + executionLog + `'
    on_success: test_loop
  step_b:
    type: step
    command: 'echo "b" >> ` + executionLog + `'
    on_success: test_loop
  step_c:
    type: step
    command: 'echo "c" >> ` + executionLog + `; echo "STOP"'
    on_success: test_loop
  done:
    type: terminal
    status: success
`

	execSvc, _, workflowName := setupF048Test(t, workflowYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act
	execCtx, err := execSvc.Run(ctx, workflowName, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify all steps executed sequentially
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	assert.Equal(t, "a\nb\nc\n", output, "should execute all steps sequentially")
}

// TestF048_Integration_ConditionalTransitions validates complex conditional logic
//
// GIVEN a loop with conditional transitions based on iteration state
// WHEN different conditions are met across iterations
// THEN appropriate steps should be skipped or executed per iteration
func TestF048_Integration_ConditionalTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")
	counterFile := filepath.Join(tmpDir, "counter")

	// Initialize counter
	err := os.WriteFile(counterFile, []byte("0"), 0o644)
	require.NoError(t, err)

	workflowYAML := `name: test-conditional
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 3
    break_when: 'states.check.Output contains "DONE"'
    body:
      - increment
      - check
      - action_skip
      - action_normal
    on_complete: done
  increment:
    type: step
    command: |
      COUNT=$(cat ` + counterFile + `)
      NEW=$((COUNT + 1))
      echo $NEW > ` + counterFile + `
      echo "increment_$NEW" >> ` + executionLog + `
    on_success: test_loop
  check:
    type: step
    command: |
      COUNT=$(cat ` + counterFile + `)
      echo "check_$COUNT" >> ` + executionLog + `
      if [ $COUNT -eq 2 ]; then
        echo "SKIP"
      elif [ $COUNT -eq 3 ]; then
        echo "DONE"
      else
        echo "CONTINUE"
      fi
    transitions:
      - when: 'states.check.Output contains "SKIP"'
        goto: action_normal
      - goto: action_skip
  action_skip:
    type: step
    command: 'echo "action_skip" >> ` + executionLog + `'
    on_success: test_loop
  action_normal:
    type: step
    command: 'echo "action_normal" >> ` + executionLog + `'
    on_success: test_loop
  done:
    type: terminal
    status: success
`

	execSvc, _, workflowName := setupF048Test(t, workflowYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act
	execCtx, err := execSvc.Run(ctx, workflowName, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Verify iteration 1: both actions executed
	assert.Contains(t, output, "increment_1")
	assert.Contains(t, output, "check_1")
	assert.Contains(t, output, "action_skip")

	// Verify action_skip appears only once (iteration 1)
	skipCount := 0
	for _, line := range lines {
		if line == "action_skip" {
			skipCount++
		}
	}
	assert.Equal(t, 1, skipCount, "action_skip should execute only in iteration 1")

	// Verify iteration 2: skipped action_skip
	assert.Contains(t, output, "increment_2")
	assert.Contains(t, output, "check_2")
}

// =============================================================================
// While Loop Transitions Tests (from T009)
// =============================================================================

// =============================================================================
// Item: T009
// Feature: F048 - Loop Body Transitions
//
// Integration tests for loop-while-transitions.yaml fixture
// Tests verify that transitions within while loop body steps are honored
// =============================================================================

// =============================================================================
// Test Helpers (T009-specific, duplicated from execution_test.go/loop_test.go)
// =============================================================================

// mockStateStore for T009 integration tests
type mockStateStoreT009 struct {
	states map[string]*workflow.ExecutionContext
}

func newMockStateStoreT009() *mockStateStoreT009 {
	return &mockStateStoreT009{states: make(map[string]*workflow.ExecutionContext)}
}

func (m *mockStateStoreT009) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	m.states[state.WorkflowID] = state
	return nil
}

func (m *mockStateStoreT009) Load(ctx context.Context, id string) (*workflow.ExecutionContext, error) {
	if state, ok := m.states[id]; ok {
		return state, nil
	}
	return nil, nil
}

func (m *mockStateStoreT009) Delete(ctx context.Context, id string) error {
	delete(m.states, id)
	return nil
}

func (m *mockStateStoreT009) List(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	return ids, nil
}

// mockLoggerT009 for T009 integration tests
type mockLoggerT009 struct {
	logs []string
}

func (m *mockLoggerT009) Debug(msg string, fields ...any) {
	if m.logs != nil {
		m.logs = append(m.logs, "DEBUG: "+msg)
	}
}

func (m *mockLoggerT009) Info(msg string, fields ...any) {
	if m.logs != nil {
		m.logs = append(m.logs, "INFO: "+msg)
	}
}

func (m *mockLoggerT009) Warn(msg string, fields ...any) {
	if m.logs != nil {
		m.logs = append(m.logs, "WARN: "+msg)
	}
}

func (m *mockLoggerT009) Error(msg string, fields ...any) {
	if m.logs != nil {
		m.logs = append(m.logs, "ERROR: "+msg)
	}
}

func (m *mockLoggerT009) WithContext(ctx map[string]any) ports.Logger {
	return m
}

// simpleExpressionEvaluatorT009 evaluates basic expressions for T009 tests
type simpleExpressionEvaluatorT009 struct{}

func newSimpleExpressionEvaluatorT009() *simpleExpressionEvaluatorT009 {
	return &simpleExpressionEvaluatorT009{}
}

func (e *simpleExpressionEvaluatorT009) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	// Handle common test expressions
	switch expr {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	// Handle contains expressions: 'states.X.Output contains "value"'
	if ctx != nil && ctx.States != nil {
		for stepName, state := range ctx.States {
			// Pattern: states.X.Output contains "VALUE"
			prefix := "states." + stepName + ".Output contains \""
			if strings.HasPrefix(expr, prefix) && strings.HasSuffix(expr, "\"") {
				searchValue := strings.TrimPrefix(expr, prefix)
				searchValue = strings.TrimSuffix(searchValue, "\"")
				return strings.Contains(state.Output, searchValue), nil
			}

			// Pattern: states.X.output contains "VALUE" (lowercase)
			prefixLower := "states." + stepName + ".output contains \""
			if strings.HasPrefix(expr, prefixLower) && strings.HasSuffix(expr, "\"") {
				searchValue := strings.TrimPrefix(expr, prefixLower)
				searchValue = strings.TrimSuffix(searchValue, "\"")
				return strings.Contains(state.Output, searchValue), nil
			}

			// Pattern: states.X.exit_code == 0
			if expr == "states."+stepName+".exit_code == 0" {
				return state.ExitCode == 0, nil
			}

			// Pattern: states.X.exit_code != 0
			if expr == "states."+stepName+".exit_code != 0" {
				return state.ExitCode != 0, nil
			}

			// Pattern: states.X.output == "value"
			if strings.HasPrefix(expr, "states."+stepName+".output == \"") {
				expectedValue := strings.TrimPrefix(expr, "states."+stepName+".output == \"")
				expectedValue = strings.TrimSuffix(expectedValue, "\"")
				actualValue := strings.TrimSpace(state.Output)
				return actualValue == expectedValue, nil
			}
		}
	}

	return false, nil
}

// =============================================================================
// Tests
// =============================================================================

// TestF048_WhileLoopBodyTransition_HappyPath tests the main scenario from the bug report
// GIVEN a while loop with body steps containing transitions
// WHEN check_tests_passed outputs "TESTS_PASSED"
// THEN it should transition to run_fmt, skipping prepare_impl_prompt and implement_item
func TestF048_WhileLoopBodyTransition_HappyPath(t *testing.T) {
	// Arrange: Load the fixture workflow
	fixtureDir := "../../tests/fixtures/workflows"
	tmpDir := t.TempDir()
	testOutputFile := filepath.Join(tmpDir, "awf-test-output.txt")

	// Create test output file with passing test status
	err := os.WriteFile(testOutputFile, []byte("TEST_EXIT_CODE=0\n"), 0o644)
	require.NoError(t, err)

	// Update fixture to use our temp file
	fixtureContent, err := os.ReadFile(filepath.Join(fixtureDir, "loop-while-transitions.yaml"))
	require.NoError(t, err)

	// Replace /tmp/awf-test-output.txt with our temp file
	modifiedContent := strings.ReplaceAll(string(fixtureContent), "/tmp/awf-test-output.txt", testOutputFile)

	workflowFile := filepath.Join(tmpDir, "test-while-transitions.yaml")
	err = os.WriteFile(workflowFile, []byte(modifiedContent), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStoreT009()
	exec := executor.NewShellExecutor()
	logger := &mockLoggerT009{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluatorT009()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Act: Execute the workflow
	execCtx, err := execSvc.Run(ctx, "test-while-transitions", nil)

	// Assert: Workflow should complete successfully
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Assert: Verify execution order - skipped steps should NOT appear in states
	// Note: This test will FAIL until F048 is implemented (RED phase)
	states := execCtx.States

	// These steps SHOULD have executed
	assert.Contains(t, states, "run_tests_green", "run_tests_green should execute")
	assert.Contains(t, states, "check_tests_passed", "check_tests_passed should execute")
	assert.Contains(t, states, "run_fmt", "run_fmt should execute (transition target)")

	// These steps SHOULD be skipped when tests pass
	assert.NotContains(t, states, "prepare_impl_prompt", "prepare_impl_prompt should be skipped")
	assert.NotContains(t, states, "implement_item", "implement_item should be skipped")
}

// TestF048_WhileLoopBodyTransition_EdgeCase_SkipToEnd tests transitioning to last step in body
// GIVEN a while loop with transition to the last body step
// WHEN the transition is triggered
// THEN it should skip all intermediate steps and execute the last step
func TestF048_WhileLoopBodyTransition_EdgeCase_SkipToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	wfYAML := `name: skip-to-end
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'states.step_last.Output contains "CONTINUE"'
    max_iterations: 2
    body:
      - step_first
      - step_middle
      - step_last
    on_complete: done
  step_first:
    type: step
    command: 'echo "first" >> ` + executionLog + `'
    transitions:
      - goto: step_last
  step_middle:
    type: step
    command: 'echo "middle" >> ` + executionLog + `'
    on_success: test_loop
  step_last:
    type: step
    command: 'echo "last" >> ` + executionLog + `; echo "STOP"'
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "skip-to-end.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStoreT009()
	exec := executor.NewShellExecutor()
	logger := &mockLoggerT009{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluatorT009()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "skip-to-end", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify execution log shows first and last, but NOT middle
	// Note: Will FAIL until F048 is implemented
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	assert.Contains(t, output, "first", "first step should execute")
	assert.Contains(t, output, "last", "last step should execute")
	assert.NotContains(t, output, "middle", "middle step should be skipped")
}

// TestF048_WhileLoopBodyTransition_EdgeCase_EarlyExit tests transition outside loop body
// GIVEN a while loop with transition to a step outside the body
// WHEN the transition is triggered
// THEN the loop should exit early and continue to the target step
func TestF048_WhileLoopBodyTransition_EdgeCase_EarlyExit(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	wfYAML := `name: early-exit
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 10
    body:
      - check_condition
      - continue_loop
    on_complete: done
  check_condition:
    type: step
    command: 'echo "check" >> ` + executionLog + `; echo "EARLY_EXIT"'
    transitions:
      - when: 'states.check_condition.Output contains "EARLY_EXIT"'
        goto: cleanup
      - goto: continue_loop
  continue_loop:
    type: step
    command: 'echo "continue" >> ` + executionLog + `'
    on_success: test_loop
  cleanup:
    type: step
    command: 'echo "cleanup" >> ` + executionLog + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "early.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStoreT009()
	exec := executor.NewShellExecutor()
	logger := &mockLoggerT009{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluatorT009()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "early", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify loop exited early and cleanup executed
	// Note: Will FAIL until F048 is implemented
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	assert.Contains(t, output, "check", "check step should execute")
	assert.Contains(t, output, "cleanup", "cleanup step should execute after early exit")
	assert.NotContains(t, output, "continue", "continue step should be skipped")
}

// TestF048_WhileLoopBodyTransition_EdgeCase_ConditionalSkip tests conditional transitions
// GIVEN a while loop with conditional transitions in body
// WHEN different conditions are met across iterations
// THEN appropriate steps should be skipped or executed
func TestF048_WhileLoopBodyTransition_EdgeCase_ConditionalSkip(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")
	counterFile := filepath.Join(tmpDir, "counter")

	// Initialize counter
	err := os.WriteFile(counterFile, []byte("0"), 0o644)
	require.NoError(t, err)

	wfYAML := `name: conditional-skip
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 3
    break_when: 'states.check.Output contains "DONE"'
    body:
      - increment
      - check
      - action_skip
      - action_normal
    on_complete: done
  increment:
    type: step
    command: |
      COUNT=$(cat ` + counterFile + `)
      NEW=$((COUNT + 1))
      echo $NEW > ` + counterFile + `
      echo "increment_$NEW" >> ` + executionLog + `
    on_success: test_loop
  check:
    type: step
    command: |
      COUNT=$(cat ` + counterFile + `)
      echo "check_$COUNT" >> ` + executionLog + `
      if [ $COUNT -eq 2 ]; then
        echo "SKIP"
      elif [ $COUNT -eq 3 ]; then
        echo "DONE"
      else
        echo "CONTINUE"
      fi
    transitions:
      - when: 'states.check.Output contains "SKIP"'
        goto: action_normal
      - goto: action_skip
  action_skip:
    type: step
    command: 'echo "action_skip" >> ` + executionLog + `'
    on_success: test_loop
  action_normal:
    type: step
    command: 'echo "action_normal" >> ` + executionLog + `'
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err = os.WriteFile(filepath.Join(tmpDir, "conditional-skip.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStoreT009()
	exec := executor.NewShellExecutor()
	logger := &mockLoggerT009{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluatorT009()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "conditional-skip", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify conditional execution pattern
	// Iteration 1: check → action_skip → action_normal
	// Iteration 2: check → action_normal (skip action_skip)
	// Iteration 3: check → DONE (exit loop)
	// Note: Will FAIL until F048 is implemented
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Iteration 1
	assert.Contains(t, output, "increment_1")
	assert.Contains(t, output, "check_1")
	assert.Contains(t, output, "action_skip")

	// Iteration 2 should skip action_skip
	assert.Contains(t, output, "increment_2")
	assert.Contains(t, output, "check_2")

	// Verify action_skip appears only once (iteration 1), not twice
	skipCount := 0
	for _, line := range lines {
		if line == "action_skip" {
			skipCount++
		}
	}
	assert.Equal(t, 1, skipCount, "action_skip should execute only in iteration 1")
}

// TestF048_WhileLoopBodyTransition_ErrorHandling_InvalidTarget tests graceful degradation
// GIVEN a while loop with transition to non-existent step
// WHEN the transition is evaluated
// THEN it should log a warning and continue sequential execution
func TestF048_WhileLoopBodyTransition_ErrorHandling_InvalidTarget(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	wfYAML := `name: invalid-target
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'states.step_b.Output contains "CONTINUE"'
    max_iterations: 2
    body:
      - step_a
      - step_b
    on_complete: done
  step_a:
    type: step
    command: 'echo "step_a" >> ` + executionLog + `'
    transitions:
      - goto: nonexistent_step
  step_b:
    type: step
    command: 'echo "step_b" >> ` + executionLog + `; echo "STOP"'
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "invalid-target.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStoreT009()
	exec := executor.NewShellExecutor()
	logger := &mockLoggerT009{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluatorT009()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "invalid-target", nil)

	// Should complete despite invalid transition (graceful degradation)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify both steps executed (fallback to sequential)
	// Note: May behave differently based on ADR-005 implementation
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	assert.Contains(t, output, "step_a")
	assert.Contains(t, output, "step_b")

	// Verify warning was logged
	assert.True(t, len(logger.logs) > 0, "Should have logged a warning")
}

// TestF048_WhileLoopBodyTransition_EdgeCase_SingleStepBody tests loop with single step body
// GIVEN a while loop with only one step in body
// WHEN that step has a transition
// THEN the transition should be honored (early exit scenario)
func TestF048_WhileLoopBodyTransition_EdgeCase_SingleStepBody(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	wfYAML := `name: single-step
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'true'
    max_iterations: 5
    body:
      - single_step
    on_complete: done
  single_step:
    type: step
    command: 'echo "single" >> ` + executionLog + `; echo "EXIT"'
    transitions:
      - when: 'states.single_step.Output contains "EXIT"'
        goto: cleanup
      - goto: test_loop
  cleanup:
    type: step
    command: 'echo "cleanup" >> ` + executionLog + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "single.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStoreT009()
	exec := executor.NewShellExecutor()
	logger := &mockLoggerT009{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluatorT009()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "single", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify loop executed once and exited via transition
	// Note: Will FAIL until F048 is implemented
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should see single execution followed by cleanup
	assert.Equal(t, 2, len(lines), "Should execute single step once, then cleanup")
	assert.Equal(t, "single", lines[0])
	assert.Equal(t, "cleanup", lines[1])
}

// TestF048_WhileLoopBodyTransition_EdgeCase_NoTransitions tests backward compatibility
// GIVEN a while loop without any transitions in body steps
// WHEN the loop executes
// THEN it should work exactly as before (sequential execution)
func TestF048_WhileLoopBodyTransition_EdgeCase_NoTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	executionLog := filepath.Join(tmpDir, "execution.log")

	wfYAML := `name: no-transitions
version: "1.0.0"
states:
  initial: test_loop
  test_loop:
    type: while
    while: 'states.step_c.Output contains "CONTINUE"'
    max_iterations: 2
    body:
      - step_a
      - step_b
      - step_c
    on_complete: done
  step_a:
    type: step
    command: 'echo "a" >> ` + executionLog + `'
    on_success: test_loop
  step_b:
    type: step
    command: 'echo "b" >> ` + executionLog + `'
    on_success: test_loop
  step_c:
    type: step
    command: 'echo "c" >> ` + executionLog + `; echo "STOP"'
    on_success: test_loop
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "no-transitions.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStoreT009()
	exec := executor.NewShellExecutor()
	logger := &mockLoggerT009{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := newSimpleExpressionEvaluatorT009()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, store, logger, resolver, nil, evaluator,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "no-transitions", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify all steps executed in sequential order
	data, err := os.ReadFile(executionLog)
	require.NoError(t, err)
	output := string(data)

	assert.Equal(t, "a\nb\nc\n", output, "Should execute all steps sequentially")
}
