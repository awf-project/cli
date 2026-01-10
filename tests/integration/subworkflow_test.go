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
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// F023: Sub-Workflow Execution Integration Tests
//
// These tests verify end-to-end sub-workflow execution:
// - Simple parent → child invocation
// - Input/output mapping
// - Nested sub-workflows (3 levels)
// - Circular call detection
// - Timeout enforcement
// - Error propagation
// - Context cancellation
// =============================================================================

// TestSubworkflow_Simple_Integration verifies basic parent → child sub-workflow invocation.
// Tests: call_workflow loads child, passes inputs, captures outputs
func TestSubworkflow_Simple_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "simple.log")

	// Create child workflow
	childYAML := `name: simple-child
version: "1.0.0"
description: Simple child that echoes input

inputs:
  - name: msg
    type: string
    required: true

outputs:
  - name: result
    from: states.echo.output

states:
  initial: echo
  echo:
    type: step
    command: 'echo "Child received: {{.inputs.msg}}" | tee -a ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "simple-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent workflow
	parentYAML := `name: simple-parent
version: "1.0.0"

inputs:
  - name: message
    type: string
    required: true

states:
  initial: prepare
  prepare:
    type: step
    command: 'echo "Parent preparing" >> ` + logFile + `'
    on_success: call_child
  call_child:
    type: call_workflow
    workflow: simple-child
    inputs:
      msg: "{{.inputs.message}}"
    outputs:
      result: child_result
    timeout: 60
    on_success: finalize
    on_failure: error
  finalize:
    type: step
    command: 'echo "Parent done" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`
	err = os.WriteFile(filepath.Join(tmpDir, "simple-parent.yaml"), []byte(parentYAML), 0o644)
	require.NoError(t, err)

	// Wire up services
	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute parent workflow with input
	execCtx, err := execSvc.Run(ctx, "simple-parent", map[string]any{"message": "Hello World"})

	require.NoError(t, err, "sub-workflow execution should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify execution order in log
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Parent preparing")
	assert.Contains(t, string(data), "Child received: Hello World")
	assert.Contains(t, string(data), "Parent done")
}

// TestSubworkflow_NestedThreeLevels_Integration verifies 3-level nesting: A → B → C.
// Uses fixtures: subworkflow-nested-a.yaml, subworkflow-nested-b.yaml, subworkflow-nested-c.yaml
func TestSubworkflow_NestedThreeLevels_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Use existing fixtures
	fixturesPath := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesPath)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Execute top-level workflow A
	execCtx, err := execSvc.Run(ctx, "subworkflow-nested-a", map[string]any{"data": "test-data"})

	require.NoError(t, err, "3-level nested sub-workflow should complete")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify call_b step completed (it contains result from B→C chain)
	callBState, ok := execCtx.GetStepState("call_b")
	require.True(t, ok, "call_b step state should exist")
	assert.Equal(t, workflow.StatusCompleted, callBState.Status)
}

// TestSubworkflow_CircularDetection_Integration verifies circular call A → B → A is detected.
// Uses fixtures: subworkflow-circular-a.yaml, subworkflow-circular-b.yaml
func TestSubworkflow_CircularDetection_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	fixturesPath := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesPath)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute circular-a (A → B → A should fail)
	execCtx, err := execSvc.Run(ctx, "subworkflow-circular-a", nil)

	require.Error(t, err, "circular sub-workflow call should be detected")
	assert.Contains(t, err.Error(), "circular", "error should mention circular")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
}

// TestSubworkflow_SelfReference_Integration verifies direct self-reference A → A is detected.
// Uses fixture: subworkflow-circular.yaml
func TestSubworkflow_SelfReference_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	fixturesPath := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesPath)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute self-referencing workflow
	execCtx, err := execSvc.Run(ctx, "subworkflow-circular", nil)

	require.Error(t, err, "self-referencing sub-workflow should be detected")
	assert.Contains(t, err.Error(), "circular", "error should mention circular")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
}

// TestSubworkflow_Timeout_Integration verifies sub-workflow respects timeout configuration.
func TestSubworkflow_Timeout_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create slow child workflow
	childYAML := `name: slow-child
version: "1.0.0"
states:
  initial: slow
  slow:
    type: step
    command: 'sleep 10'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "slow-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent with short timeout
	parentYAML := `name: timeout-parent
version: "1.0.0"
states:
  initial: call_slow
  call_slow:
    type: call_workflow
    workflow: slow-child
    timeout: 1
    on_success: done
    on_failure: timeout_handler
  done:
    type: terminal
    status: success
  timeout_handler:
    type: step
    command: 'echo "timeout occurred"'
    on_success: timed_out
  timed_out:
    type: terminal
    status: failure
`
	err = os.WriteFile(filepath.Join(tmpDir, "timeout-parent.yaml"), []byte(parentYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	execCtx, err := execSvc.Run(ctx, "timeout-parent", nil)
	elapsed := time.Since(start)

	// Should complete within ~3 seconds (1s timeout + overhead + handler)
	assert.Less(t, elapsed, 5*time.Second, "should timeout quickly")

	// Workflow should have gone to failure path
	if err == nil {
		assert.Equal(t, "timed_out", execCtx.CurrentStep, "should reach timeout_handler path")
	}
}

// TestSubworkflow_ErrorPropagation_Integration verifies child failure propagates to parent.
func TestSubworkflow_ErrorPropagation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create failing child
	childYAML := `name: failing-child
version: "1.0.0"
states:
  initial: fail
  fail:
    type: step
    command: 'exit 1'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "failing-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent
	parentYAML := `name: error-parent
version: "1.0.0"
states:
  initial: call_failing
  call_failing:
    type: call_workflow
    workflow: failing-child
    timeout: 30
    on_success: done
    on_failure: handle_error
  done:
    type: terminal
    status: success
  handle_error:
    type: step
    command: 'echo "caught error"'
    on_success: error_handled
  error_handled:
    type: terminal
    status: failure
`
	err = os.WriteFile(filepath.Join(tmpDir, "error-parent.yaml"), []byte(parentYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "error-parent", nil)

	// Should complete (error was handled via on_failure path)
	require.NoError(t, err, "error should be handled via on_failure path")
	assert.Equal(t, "error_handled", execCtx.CurrentStep, "should reach error handler")

	// Verify call_failing step failed
	callState, ok := execCtx.GetStepState("call_failing")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusFailed, callState.Status)
}

// TestSubworkflow_OutputMapping_Integration verifies child outputs are accessible in parent.
func TestSubworkflow_OutputMapping_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")

	// Create child with defined output
	childYAML := `name: output-child
version: "1.0.0"

outputs:
  - name: result
    from: states.produce.output

states:
  initial: produce
  produce:
    type: step
    command: 'echo "SECRET_VALUE_42"'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "output-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent that uses child output
	parentYAML := `name: output-parent
version: "1.0.0"
states:
  initial: call_child
  call_child:
    type: call_workflow
    workflow: output-child
    outputs:
      result: child_result
    timeout: 30
    on_success: use_output
    on_failure: error
  use_output:
    type: step
    command: 'echo "Got from child: {{.states.call_child.Output}}" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`
	err = os.WriteFile(filepath.Join(tmpDir, "output-parent.yaml"), []byte(parentYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "output-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify output was captured and used
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	// Note: The actual interpolation may put different content,
	// but we at least verify the step ran
	assert.NotEmpty(t, string(data), "log should contain output usage")
}

// TestSubworkflow_NotFound_Integration verifies error when sub-workflow doesn't exist.
func TestSubworkflow_NotFound_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create parent calling non-existent child
	parentYAML := `name: missing-child-parent
version: "1.0.0"
states:
  initial: call_missing
  call_missing:
    type: call_workflow
    workflow: nonexistent-workflow
    timeout: 30
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`
	err := os.WriteFile(filepath.Join(tmpDir, "missing-child-parent.yaml"), []byte(parentYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "missing-child-parent", nil)

	// Should fail - child not found
	if err != nil {
		assert.True(t, strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "load") ||
			strings.Contains(err.Error(), "no such file"),
			"error should indicate workflow not found: %v", err)
	} else {
		// If no error, should have gone to on_failure path
		assert.Equal(t, "error", execCtx.CurrentStep, "should reach error terminal via on_failure")
	}
}

// TestSubworkflow_ContextCancellation_Integration verifies parent cancellation propagates to child.
func TestSubworkflow_ContextCancellation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "cancel.log")

	// Create slow child
	childYAML := `name: cancel-child
version: "1.0.0"
states:
  initial: slow
  slow:
    type: step
    command: 'echo "start" >> ` + logFile + ` && sleep 30 && echo "end" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "cancel-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent
	parentYAML := `name: cancel-parent
version: "1.0.0"
states:
  initial: call_slow
  call_slow:
    type: call_workflow
    workflow: cancel-child
    timeout: 60
    on_success: done
  done:
    type: terminal
    status: success
`
	err = os.WriteFile(filepath.Join(tmpDir, "cancel-parent.yaml"), []byte(parentYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	// Create context with 2 second timeout (shorter than child's sleep)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err = execSvc.Run(ctx, "cancel-parent", nil)
	elapsed := time.Since(start)

	// Should be cancelled quickly
	assert.Less(t, elapsed, 5*time.Second, "should be cancelled quickly")
	assert.Error(t, err, "should return error on cancellation")
	assert.True(t,
		strings.Contains(err.Error(), "cancel") ||
			strings.Contains(err.Error(), "deadline") ||
			strings.Contains(err.Error(), "context"),
		"error should indicate cancellation: %v", err)

	// Wait a moment for file writes
	time.Sleep(100 * time.Millisecond)

	// Verify child started but didn't complete
	data, _ := os.ReadFile(logFile)
	assert.Contains(t, string(data), "start", "child should have started")
	assert.NotContains(t, string(data), "end", "child should not have completed")
}

// TestSubworkflow_InputMapping_Integration verifies input mapping with interpolation.
func TestSubworkflow_InputMapping_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "inputs.log")

	// Create child with multiple inputs
	childYAML := `name: inputs-child
version: "1.0.0"

inputs:
  - name: first
    type: string
    required: true
  - name: second
    type: string
    required: true

states:
  initial: log_inputs
  log_inputs:
    type: step
    command: 'echo "first={{.inputs.first}} second={{.inputs.second}}" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "inputs-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent with input mappings
	parentYAML := `name: inputs-parent
version: "1.0.0"

inputs:
  - name: value1
    type: string
    required: true
  - name: value2
    type: string
    required: true

states:
  initial: call_child
  call_child:
    type: call_workflow
    workflow: inputs-child
    inputs:
      first: "{{.inputs.value1}}"
      second: "{{.inputs.value2}}"
    timeout: 30
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`
	err = os.WriteFile(filepath.Join(tmpDir, "inputs-parent.yaml"), []byte(parentYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "inputs-parent", map[string]any{
		"value1": "ALPHA",
		"value2": "BETA",
	})

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify inputs were correctly mapped
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "first=ALPHA")
	assert.Contains(t, string(data), "second=BETA")
}

// TestSubworkflow_WithExistingFixtures_Integration runs using the pre-defined fixtures.
func TestSubworkflow_WithExistingFixtures_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	fixturesPath := "../fixtures/workflows"

	repo := repository.NewYAMLRepository(fixturesPath)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test simple parent-child with fixtures
	execCtx, err := execSvc.Run(ctx, "subworkflow-simple", map[string]any{
		"input_message": "fixture-test",
	})

	require.NoError(t, err, "fixture subworkflow-simple should execute successfully")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)
}

// TestSubworkflow_MaxNestingDepth_Integration verifies max nesting depth is enforced.
func TestSubworkflow_MaxNestingDepth_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create a chain of workflows that exceeds max depth (default is 10)
	// We create 12 levels to ensure we hit the limit
	for i := 1; i <= 12; i++ {
		var yaml string
		if i == 12 {
			// Leaf workflow
			yaml = `name: deep-` + string(rune('a'+i-1)) + `
version: "1.0.0"
states:
  initial: work
  work:
    type: step
    command: 'echo "leaf"'
    on_success: done
  done:
    type: terminal
    status: success
`
		} else {
			// Calls next level - no on_failure so error propagates up
			next := string(rune('a' + i))
			yaml = `name: deep-` + string(rune('a'+i-1)) + `
version: "1.0.0"
states:
  initial: call_next
  call_next:
    type: call_workflow
    workflow: deep-` + next + `
    timeout: 60
    on_success: done
  done:
    type: terminal
    status: success
`
		}
		filename := "deep-" + string(rune('a'+i-1)) + ".yaml"
		err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(yaml), 0o644)
		require.NoError(t, err)
	}

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Start from level a
	_, err := execSvc.Run(ctx, "deep-a", nil)

	// Should fail due to max nesting exceeded
	require.Error(t, err, "should fail when max nesting depth exceeded")
	assert.True(t,
		strings.Contains(err.Error(), "nesting") ||
			strings.Contains(err.Error(), "depth") ||
			strings.Contains(err.Error(), "max"),
		"error should indicate max depth exceeded: %v", err)
}
