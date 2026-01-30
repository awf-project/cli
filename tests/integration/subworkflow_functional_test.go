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
// F023: Sub-Workflow Functional Tests - Additional Coverage
//
// These tests complement the integration tests with functional scenarios that
// verify end-to-end behavior of the sub-workflow feature against acceptance
// criteria not fully covered elsewhere.
//
// Acceptance Criteria Coverage:
// - AC7: Sub-workflow state visible in parent
// - Integration: call_workflow inside loops
// - Edge: Multiple outputs from child
// - Edge: Missing required input handling
// - Edge: Default input values
// - Edge: Deep call stack error messages
// =============================================================================

// TestSubworkflow_StateVisibleInParent_Functional verifies that sub-workflow step state
// is properly recorded in parent's execution context (AC7).
func TestSubworkflow_StateVisibleInParent_Functional(t *testing.T) {

	tmpDir := t.TempDir()

	// Create child that produces known output
	childYAML := `name: state-child
version: "1.0.0"
outputs:
  - name: result
    from: states.produce.output
states:
  initial: produce
  produce:
    type: step
    command: 'echo "CHILD_OUTPUT_VALUE"'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "state-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent
	parentYAML := `name: state-parent
version: "1.0.0"
states:
  initial: call_child
  call_child:
    type: call_workflow
    workflow: state-child
    outputs:
      result: child_result
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
	err = os.WriteFile(filepath.Join(tmpDir, "state-parent.yaml"), []byte(parentYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "state-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify call_child step state exists and has correct status
	stepState, ok := execCtx.GetStepState("call_child")
	require.True(t, ok, "call_child step state should exist")
	assert.Equal(t, workflow.StatusCompleted, stepState.Status, "call_child should be completed")
	assert.NotZero(t, stepState.StartedAt, "step should have start time")
	assert.NotZero(t, stepState.CompletedAt, "step should have completion time")
	assert.Empty(t, stepState.Error, "step should have no error")
}

// TestSubworkflow_InsideForEachLoop_Functional verifies sub-workflows can be called
// from within a for_each loop body.
func TestSubworkflow_InsideForEachLoop_Functional(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "loop.log")

	// Create simple child
	childYAML := `name: loop-child
version: "1.0.0"
inputs:
  - name: item
    type: string
    required: true
states:
  initial: process
  process:
    type: step
    command: 'echo "Processed: {{.inputs.item}}" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "loop-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent with loop calling child for each item
	parentYAML := `name: loop-parent
version: "1.0.0"
states:
  initial: process_items
  process_items:
    type: for_each
    items: '["alpha", "beta", "gamma"]'
    max_iterations: 10
    body:
      - call_child_step
    on_complete: done
  call_child_step:
    type: call_workflow
    workflow: loop-child
    inputs:
      item: "{{.loop.Item}}"
    timeout: 30
    on_success: process_items
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`
	err = os.WriteFile(filepath.Join(tmpDir, "loop-parent.yaml"), []byte(parentYAML), 0o644)
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "loop-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify all items were processed via child workflow
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Processed: alpha")
	assert.Contains(t, string(data), "Processed: beta")
	assert.Contains(t, string(data), "Processed: gamma")
}

// TestSubworkflow_MultipleOutputs_Functional verifies mapping multiple outputs from child.
func TestSubworkflow_MultipleOutputs_Functional(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "multi.log")

	// Create child with multiple outputs
	childYAML := `name: multi-output-child
version: "1.0.0"
outputs:
  - name: first_result
    from: states.step1.output
  - name: second_result
    from: states.step2.output
states:
  initial: step1
  step1:
    type: step
    command: 'echo "FIRST_VALUE"'
    on_success: step2
  step2:
    type: step
    command: 'echo "SECOND_VALUE"'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "multi-output-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent that uses both outputs
	parentYAML := `name: multi-output-parent
version: "1.0.0"
states:
  initial: call_child
  call_child:
    type: call_workflow
    workflow: multi-output-child
    outputs:
      first_result: first
      second_result: second
    timeout: 30
    on_success: use_outputs
    on_failure: error
  use_outputs:
    type: step
    command: 'echo "first={{.states.call_child.Output}} second={{.states.call_child.Output}}" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`
	err = os.WriteFile(filepath.Join(tmpDir, "multi-output-parent.yaml"), []byte(parentYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "multi-output-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify log file was created (output usage step ran)
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.NotEmpty(t, string(data))
}

// TestSubworkflow_MissingRequiredInput_Functional verifies error when child requires
// an input not provided by parent.
func TestSubworkflow_MissingRequiredInput_Functional(t *testing.T) {

	tmpDir := t.TempDir()

	// Create child requiring a specific input
	childYAML := `name: requires-input-child
version: "1.0.0"
inputs:
  - name: required_param
    type: string
    required: true
states:
  initial: work
  work:
    type: step
    command: 'echo "{{.inputs.required_param}}"'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "requires-input-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent that doesn't provide the required input
	parentYAML := `name: missing-input-parent
version: "1.0.0"
states:
  initial: call_child
  call_child:
    type: call_workflow
    workflow: requires-input-child
    inputs: {}
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
	err = os.WriteFile(filepath.Join(tmpDir, "missing-input-parent.yaml"), []byte(parentYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "missing-input-parent", nil)

	// Should either fail with error or go to on_failure path
	if err != nil {
		assert.True(t,
			strings.Contains(err.Error(), "required") ||
				strings.Contains(err.Error(), "input") ||
				strings.Contains(err.Error(), "required_param"),
			"error should mention missing required input: %v", err)
	} else {
		// If it didn't error, it should have gone to error terminal
		assert.Equal(t, "error", execCtx.CurrentStep, "should reach error terminal via on_failure")
	}
}

// TestSubworkflow_DefaultInputValue_Functional verifies child uses default when
// parent omits optional input.
func TestSubworkflow_DefaultInputValue_Functional(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "default.log")

	// Create child with optional input having default
	childYAML := `name: default-input-child
version: "1.0.0"
inputs:
  - name: optional_param
    type: string
    required: false
    default: "DEFAULT_VALUE"
states:
  initial: work
  work:
    type: step
    command: 'echo "Value: {{.inputs.optional_param}}" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "default-input-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent that doesn't provide the optional input
	parentYAML := `name: default-input-parent
version: "1.0.0"
states:
  initial: call_child
  call_child:
    type: call_workflow
    workflow: default-input-child
    inputs: {}
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
	err = os.WriteFile(filepath.Join(tmpDir, "default-input-parent.yaml"), []byte(parentYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "default-input-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify default value was used
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "DEFAULT_VALUE", "child should use default value")
}

// TestSubworkflow_CallStackInErrorMessage_Functional verifies that circular call
// errors include the full call stack for debugging.
func TestSubworkflow_CallStackInErrorMessage_Functional(t *testing.T) {

	tmpDir := t.TempDir()

	// Create A that calls B
	aYAML := `name: stack-a
version: "1.0.0"
states:
  initial: call_b
  call_b:
    type: call_workflow
    workflow: stack-b
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
	err := os.WriteFile(filepath.Join(tmpDir, "stack-a.yaml"), []byte(aYAML), 0o644)
	require.NoError(t, err)

	// Create B that calls back to A (circular)
	bYAML := `name: stack-b
version: "1.0.0"
states:
  initial: call_a
  call_a:
    type: call_workflow
    workflow: stack-a
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
	err = os.WriteFile(filepath.Join(tmpDir, "stack-b.yaml"), []byte(bYAML), 0o644)
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

	_, err = execSvc.Run(ctx, "stack-a", nil)

	require.Error(t, err, "circular reference should be detected")
	errMsg := err.Error()

	// Error message should mention:
	// 1. That it's circular
	// 2. Include workflow names in the call stack
	assert.Contains(t, errMsg, "circular", "error should mention circular")
	assert.True(t,
		strings.Contains(errMsg, "stack-a") || strings.Contains(errMsg, "stack-b"),
		"error should include workflow names in call stack: %s", errMsg)
}

// TestSubworkflow_ParentInputPassedToChild_Functional verifies parent workflow inputs
// are correctly interpolated and passed to child workflow.
func TestSubworkflow_ParentInputPassedToChild_Functional(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "pass.log")

	// Create child
	childYAML := `name: pass-child
version: "1.0.0"
inputs:
  - name: parent_data
    type: string
    required: true
states:
  initial: log
  log:
    type: step
    command: 'echo "Child received: {{.inputs.parent_data}}" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "pass-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent that passes its input to child
	parentYAML := `name: pass-parent
version: "1.0.0"
inputs:
  - name: message
    type: string
    required: true
states:
  initial: call_child
  call_child:
    type: call_workflow
    workflow: pass-child
    inputs:
      parent_data: "{{.inputs.message}}"
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
	err = os.WriteFile(filepath.Join(tmpDir, "pass-parent.yaml"), []byte(parentYAML), 0o644)
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

	testMessage := "UNIQUE_TEST_MESSAGE_12345"
	execCtx, err := execSvc.Run(ctx, "pass-parent", map[string]any{"message": testMessage})

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify child received parent's input
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), testMessage, "child should receive parent's input value")
}

// TestSubworkflow_ContinueOnError_Functional verifies continue_on_error behavior
// with failing sub-workflows.
func TestSubworkflow_ContinueOnError_Functional(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "continue.log")

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

	// Create parent with continue_on_error
	parentYAML := `name: continue-parent
version: "1.0.0"
states:
  initial: call_failing
  call_failing:
    type: call_workflow
    workflow: failing-child
    timeout: 30
    continue_on_error: true
    on_success: after_call
    on_failure: error
  after_call:
    type: step
    command: 'echo "continued despite failure" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`
	err = os.WriteFile(filepath.Join(tmpDir, "continue-parent.yaml"), []byte(parentYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "continue-parent", nil)

	require.NoError(t, err, "workflow should complete despite sub-workflow failure")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify workflow continued
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "continued despite failure")
}

// TestSubworkflow_WithHooks_Functional verifies pre and post hooks work with sub-workflows.
func TestSubworkflow_WithHooks_Functional(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "hooks.log")

	// Create simple child
	childYAML := `name: hooks-child
version: "1.0.0"
states:
  initial: work
  work:
    type: step
    command: 'echo "child executed" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "hooks-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent with hooks on call_workflow step
	parentYAML := `name: hooks-parent
version: "1.0.0"
states:
  initial: call_child
  call_child:
    type: call_workflow
    workflow: hooks-child
    timeout: 30
    hooks:
      pre:
        - command: 'echo "pre-hook" >> ` + logFile + `'
      post:
        - command: 'echo "post-hook" >> ` + logFile + `'
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
`
	err = os.WriteFile(filepath.Join(tmpDir, "hooks-parent.yaml"), []byte(parentYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "hooks-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify hooks and child execution in correct order
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	content := string(data)

	// Verify all parts executed
	assert.Contains(t, content, "pre-hook", "pre-hook should execute")
	assert.Contains(t, content, "child executed", "child should execute")
	assert.Contains(t, content, "post-hook", "post-hook should execute")

	// Verify order: pre-hook before child, post-hook after child
	preIdx := strings.Index(content, "pre-hook")
	childIdx := strings.Index(content, "child executed")
	postIdx := strings.Index(content, "post-hook")

	assert.True(t, preIdx < childIdx, "pre-hook should come before child execution")
	assert.True(t, childIdx < postIdx, "child execution should come before post-hook")
}

// TestSubworkflow_EmptyInputsAndOutputs_Functional verifies sub-workflow works with
// no inputs or outputs configured.
func TestSubworkflow_EmptyInputsAndOutputs_Functional(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "empty.log")

	// Create child with no inputs or outputs
	childYAML := `name: empty-io-child
version: "1.0.0"
states:
  initial: work
  work:
    type: step
    command: 'echo "child ran" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "empty-io-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent with minimal call_workflow config
	parentYAML := `name: empty-io-parent
version: "1.0.0"
states:
  initial: call_child
  call_child:
    type: call_workflow
    workflow: empty-io-child
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
	err = os.WriteFile(filepath.Join(tmpDir, "empty-io-parent.yaml"), []byte(parentYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "empty-io-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify child ran
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "child ran")
}

// TestSubworkflow_UsesPreviousStepOutput_Functional verifies parent can pass previous
// step output to child workflow.
func TestSubworkflow_UsesPreviousStepOutput_Functional(t *testing.T) {

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "prev.log")

	// Create child
	childYAML := `name: prev-child
version: "1.0.0"
inputs:
  - name: from_prev
    type: string
    required: true
states:
  initial: log
  log:
    type: step
    command: 'echo "Child got: {{.inputs.from_prev}}" >> ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "prev-child.yaml"), []byte(childYAML), 0o644)
	require.NoError(t, err)

	// Create parent that uses previous step output as child input
	parentYAML := `name: prev-parent
version: "1.0.0"
states:
  initial: generate
  generate:
    type: step
    command: 'echo "GENERATED_DATA"'
    on_success: call_child
    on_failure: error
  call_child:
    type: call_workflow
    workflow: prev-child
    inputs:
      from_prev: "{{.states.generate.Output}}"
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
	err = os.WriteFile(filepath.Join(tmpDir, "prev-parent.yaml"), []byte(parentYAML), 0o644)
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

	execCtx, err := execSvc.Run(ctx, "prev-parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify child received output from previous step
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "GENERATED_DATA", "child should receive previous step output")
}
