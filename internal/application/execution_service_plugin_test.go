package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/builders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executePluginOperation Tests
// Feature: C054 - Increase Application Layer Test Coverage
// Component: T006 - plugin_operation_coverage
//
// This file tests the executePluginOperation function which handles plugin
// operation lifecycle including timeouts, context cancellation, error handling,
// hook execution, input resolution, and output serialization.
//
// Coverage target: 59.0% -> 80%+
//
// This function is private (executePluginOperation), so we test it indirectly
// through the public API by constructing workflows with operation steps and
// observing execution behavior through step states and error propagation.

// TestExecutePluginOperation_Timeout verifies that operation steps respect timeout configuration
// and handle timeout errors appropriately.
func TestExecutePluginOperation_Timeout(t *testing.T) {
	tests := []struct {
		name           string
		timeout        int
		providerDelay  time.Duration
		expectTimeout  bool
		continueOnErr  bool
		onFailure      string
		expectedStatus workflow.ExecutionStatus
		expectedStep   string
	}{
		{
			name:           "operation completes within timeout",
			timeout:        5,
			providerDelay:  10 * time.Millisecond,
			expectTimeout:  false,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
		},
		{
			name:           "operation exceeds timeout with deadline exceeded",
			timeout:        1,
			providerDelay:  2 * time.Second,
			expectTimeout:  true,
			expectedStatus: workflow.StatusCancelled,
			expectedStep:   "operation",
		},
		{
			name:           "timeout with OnFailure transition",
			timeout:        1,
			providerDelay:  2 * time.Second,
			expectTimeout:  true,
			onFailure:      "failure_step",
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "failure_step",
		},
		{
			name:           "timeout with ContinueOnError=true",
			timeout:        1,
			providerDelay:  2 * time.Second,
			expectTimeout:  true,
			continueOnErr:  true,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockOperationProviderWithDelay(tt.providerDelay)
			provider.addOperation("slow.operation", "Slow operation", "test-plugin")

			step := builders.NewStepBuilder("operation").
				WithType(workflow.StepTypeOperation).
				WithOperation("slow.operation", nil).
				WithTimeout(tt.timeout).
				WithOnSuccess("done").
				Build()

			if tt.continueOnErr {
				step.ContinueOnError = true
			}
			if tt.onFailure != "" {
				step.OnFailure = tt.onFailure
			}

			wf := &workflow.Workflow{
				Name:    "timeout-test",
				Initial: "operation",
				Steps: map[string]*workflow.Step{
					"operation": step,
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
					"failure_step": {
						Name: "failure_step",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("timeout-test", wf).
				Build()
			execSvc.SetOperationProvider(provider)

			ctx, err := execSvc.Run(context.Background(), "timeout-test", nil)

			if tt.expectTimeout && !tt.continueOnErr && tt.onFailure == "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "context deadline exceeded")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedStatus, ctx.Status)
			assert.Equal(t, tt.expectedStep, ctx.CurrentStep)

			// Verify step state records timeout error
			if tt.expectTimeout {
				state, exists := ctx.States["operation"]
				require.True(t, exists)
				assert.Equal(t, workflow.StatusFailed, state.Status)
				assert.Contains(t, state.Error, "context deadline exceeded")
			}
		})
	}
}

// TestExecutePluginOperation_ContextCancellation verifies that operation steps
// handle parent context cancellation (workflow-level cancellation) correctly.
func TestExecutePluginOperation_ContextCancellation(t *testing.T) {
	tests := []struct {
		name               string
		cancelBeforeExec   bool
		providerDelay      time.Duration
		expectedStatus     workflow.ExecutionStatus
		expectedErrMessage string
	}{
		{
			name:               "parent context cancelled during operation execution",
			cancelBeforeExec:   false,
			providerDelay:      200 * time.Millisecond,
			expectedStatus:     workflow.StatusCancelled,
			expectedErrMessage: "context canceled",
		},
		{
			name:               "parent context already cancelled before execution",
			cancelBeforeExec:   true,
			providerDelay:      10 * time.Millisecond,
			expectedStatus:     workflow.StatusCancelled,
			expectedErrMessage: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockOperationProviderWithDelay(tt.providerDelay)
			provider.addOperation("test.operation", "Test operation", "test-plugin")

			wf := &workflow.Workflow{
				Name:    "cancel-test",
				Initial: "operation",
				Steps: map[string]*workflow.Step{
					"operation": builders.NewStepBuilder("operation").
						WithType(workflow.StepTypeOperation).
						WithOperation("test.operation", nil).
						WithOnSuccess("done").
						Build(),
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("cancel-test", wf).
				Build()
			execSvc.SetOperationProvider(provider)

			// Create cancellable context
			ctx, cancel := context.WithCancel(context.Background())
			if tt.cancelBeforeExec {
				cancel()
			} else {
				// Cancel after a short delay during execution
				go func() {
					time.Sleep(50 * time.Millisecond)
					cancel()
				}()
			}

			execCtx, err := execSvc.Run(ctx, "cancel-test", nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrMessage)
			assert.Equal(t, tt.expectedStatus, execCtx.Status)

			// Verify step state set to StatusFailed on cancellation
			state, exists := execCtx.States["operation"]
			require.True(t, exists)
			assert.Equal(t, workflow.StatusFailed, state.Status)
			assert.Contains(t, state.Error, "context canceled")
		})
	}
}

// TestExecutePluginOperation_ContinueOnError verifies that operation steps
// with ContinueOnError=true transition to OnSuccess even on execution errors.
func TestExecutePluginOperation_ContinueOnError(t *testing.T) {
	tests := []struct {
		name           string
		continueOnErr  bool
		providerError  error
		operationFail  bool
		expectedStatus workflow.ExecutionStatus
		expectedStep   string
		expectError    bool
	}{
		{
			name:           "execution error + ContinueOnError=true -> OnSuccess transition",
			continueOnErr:  true,
			providerError:  errors.New("provider failed"),
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			expectError:    false,
		},
		{
			name:           "execution error + ContinueOnError=false -> error propagation",
			continueOnErr:  false,
			providerError:  errors.New("provider failed"),
			expectedStatus: workflow.StatusFailed,
			expectedStep:   "operation",
			expectError:    true,
		},
		{
			name:           "operation failure (Success=false) + ContinueOnError=true -> OnSuccess",
			continueOnErr:  true,
			operationFail:  true,
			expectedStatus: workflow.StatusCompleted,
			expectedStep:   "done",
			expectError:    false,
		},
		{
			name:           "operation failure + ContinueOnError=false -> error propagation",
			continueOnErr:  false,
			operationFail:  true,
			expectedStatus: workflow.StatusFailed,
			expectedStep:   "operation",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockOperationProvider()
			provider.addOperation("test.operation", "Test operation", "test-plugin")

			if tt.providerError != nil {
				provider.execError = tt.providerError
			} else if tt.operationFail {
				provider.results["test.operation"] = &pluginmodel.OperationResult{
					Success: false,
					Error:   "operation execution failed",
				}
			}

			step := builders.NewStepBuilder("operation").
				WithType(workflow.StepTypeOperation).
				WithOperation("test.operation", nil).
				WithOnSuccess("done").
				Build()
			step.ContinueOnError = tt.continueOnErr

			wf := &workflow.Workflow{
				Name:    "continue-test",
				Initial: "operation",
				Steps: map[string]*workflow.Step{
					"operation": step,
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("continue-test", wf).
				Build()
			execSvc.SetOperationProvider(provider)

			ctx, err := execSvc.Run(context.Background(), "continue-test", nil)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedStatus, ctx.Status)
			assert.Equal(t, tt.expectedStep, ctx.CurrentStep)

			// Step state records error even when continuing
			state, exists := ctx.States["operation"]
			require.True(t, exists)
			if tt.providerError != nil || tt.operationFail {
				assert.Equal(t, workflow.StatusFailed, state.Status)
				assert.NotEmpty(t, state.Error)
			}
		})
	}
}

// TestExecutePluginOperation_ExecutionErrorPropagation verifies that execution errors
// without fallback paths (no OnFailure, no ContinueOnError) propagate correctly.
func TestExecutePluginOperation_ExecutionErrorPropagation(t *testing.T) {
	tests := []struct {
		name          string
		providerError error
		expectedMsg   string
	}{
		{
			name:          "provider Execute() returns error, no OnFailure -> error propagates",
			providerError: errors.New("provider internal error"),
			expectedMsg:   "provider internal error",
		},
		{
			name:          "provider Execute() returns custom error -> error propagates",
			providerError: errors.New("custom error: network timeout"),
			expectedMsg:   "custom error: network timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockOperationProvider()
			provider.addOperation("test.operation", "Test operation", "test-plugin")
			provider.execError = tt.providerError

			wf := &workflow.Workflow{
				Name:    "error-test",
				Initial: "operation",
				Steps: map[string]*workflow.Step{
					"operation": builders.NewStepBuilder("operation").
						WithType(workflow.StepTypeOperation).
						WithOperation("test.operation", nil).
						Build(),
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("error-test", wf).
				Build()
			execSvc.SetOperationProvider(provider)

			ctx, err := execSvc.Run(context.Background(), "error-test", nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "operation")
			assert.Contains(t, err.Error(), tt.expectedMsg)

			// Step state set to StatusFailed
			state, exists := ctx.States["operation"]
			require.True(t, exists)
			assert.Equal(t, workflow.StatusFailed, state.Status)
			assert.Contains(t, state.Error, tt.expectedMsg)
		})
	}
}

// TestExecutePluginOperation_OperationFailureWithoutMessage verifies that operation
// failures (Success=false) without an Error field use default error message.
func TestExecutePluginOperation_OperationFailureWithoutMessage(t *testing.T) {
	tests := []struct {
		name         string
		resultError  string
		expectedMsg  string
		hasOnFailure bool
	}{
		{
			name:        "OperationResult{Success: false, Error: \"\"} -> default \"operation failed\"",
			resultError: "",
			expectedMsg: "operation failed",
		},
		{
			name:        "OperationResult{Success: false, Error: \"custom\"} -> use custom message",
			resultError: "custom failure message",
			expectedMsg: "custom failure message",
		},
		{
			name:         "failure with OnFailure path -> no error propagation",
			resultError:  "failure with fallback",
			hasOnFailure: true,
			expectedMsg:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockOperationProvider()
			provider.addOperation("test.operation", "Test operation", "test-plugin")
			provider.results["test.operation"] = &pluginmodel.OperationResult{
				Success: false,
				Error:   tt.resultError,
			}

			step := builders.NewStepBuilder("operation").
				WithType(workflow.StepTypeOperation).
				WithOperation("test.operation", nil).
				Build()

			steps := map[string]*workflow.Step{
				"operation": step,
			}

			if tt.hasOnFailure {
				step.OnFailure = "failure_step"
				steps["failure_step"] = &workflow.Step{
					Name: "failure_step",
					Type: workflow.StepTypeTerminal,
				}
			}

			wf := &workflow.Workflow{
				Name:    "failure-test",
				Initial: "operation",
				Steps:   steps,
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("failure-test", wf).
				Build()
			execSvc.SetOperationProvider(provider)

			ctx, err := execSvc.Run(context.Background(), "failure-test", nil)

			if tt.hasOnFailure {
				require.NoError(t, err)
				assert.Equal(t, "failure_step", ctx.CurrentStep)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedMsg)
			}

			// Step state records appropriate error message
			state, exists := ctx.States["operation"]
			require.True(t, exists)
			assert.Equal(t, workflow.StatusFailed, state.Status)
			if tt.resultError != "" {
				assert.Equal(t, tt.resultError, state.Error)
			}
		})
	}
}

// TestExecutePluginOperation_PrePostHooks verifies that pre and post hooks
// execute correctly around plugin operations.
func TestExecutePluginOperation_PrePostHooks(t *testing.T) {
	tests := []struct {
		name            string
		operationFails  bool
		preHookCommand  string
		postHookCommand string
		expectPreLog    string
		expectPostLog   string
	}{
		{
			name:           "pre-hook executes before operation",
			preHookCommand: "echo pre-hook",
			expectPreLog:   "pre-hook",
			operationFails: false,
		},
		{
			name:            "post-hook executes after successful operation",
			postHookCommand: "echo post-hook",
			expectPostLog:   "post-hook",
			operationFails:  false,
		},
		{
			name:            "post-hook executes after failed operation",
			postHookCommand: "echo post-failure",
			expectPostLog:   "post-failure",
			operationFails:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := setupHookTestProvider(tt.operationFails)
			wf := buildHookTestWorkflow(tt.preHookCommand, tt.postHookCommand, tt.operationFails)

			execSvc, mocks := NewTestHarness(t).
				WithWorkflow("hook-test", wf).
				Build()
			execSvc.SetOperationProvider(provider)

			configureHookMocks(mocks, tt.preHookCommand, tt.expectPreLog, tt.postHookCommand, tt.expectPostLog)

			_, _ = execSvc.Run(context.Background(), "hook-test", nil)

			assertHookExecution(t, mocks.Executor.GetCalls(), tt.preHookCommand, tt.postHookCommand)
		})
	}
}

// setupHookTestProvider creates a mock provider for hook testing.
func setupHookTestProvider(operationFails bool) *mockOperationProvider {
	provider := newMockOperationProvider()
	provider.addOperation("test.operation", "Test operation", "test-plugin")

	if operationFails {
		provider.results["test.operation"] = &pluginmodel.OperationResult{
			Success: false,
			Error:   "operation failed",
		}
	}
	return provider
}

// buildHookTestWorkflow constructs a workflow with pre/post hooks for testing.
func buildHookTestWorkflow(preHookCommand, postHookCommand string, operationFails bool) *workflow.Workflow {
	hooks := workflow.StepHooks{}
	if preHookCommand != "" {
		hooks.Pre = workflow.Hook{
			{Command: preHookCommand},
		}
	}
	if postHookCommand != "" {
		hooks.Post = workflow.Hook{
			{Command: postHookCommand},
		}
	}

	step := builders.NewStepBuilder("operation").
		WithType(workflow.StepTypeOperation).
		WithOperation("test.operation", nil).
		WithOnSuccess("done").
		Build()
	step.Hooks = hooks

	if operationFails {
		step.OnFailure = "failure_step"
	}

	return &workflow.Workflow{
		Name:    "hook-test",
		Initial: "operation",
		Steps: map[string]*workflow.Step{
			"operation": step,
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"failure_step": {
				Name: "failure_step",
				Type: workflow.StepTypeTerminal,
			},
		},
	}
}

// configureHookMocks sets up mock executor with expected hook command results.
func configureHookMocks(mocks *TestMocks, preHookCommand, expectPreLog, postHookCommand, expectPostLog string) {
	if preHookCommand != "" {
		mocks.Executor.SetCommandResult(preHookCommand, &ports.CommandResult{Stdout: expectPreLog, ExitCode: 0})
	}
	if postHookCommand != "" {
		mocks.Executor.SetCommandResult(postHookCommand, &ports.CommandResult{Stdout: expectPostLog, ExitCode: 0})
	}
}

// assertHookExecution verifies that expected hooks were executed.
func assertHookExecution(t *testing.T, calls []*ports.Command, preHookCommand, postHookCommand string) {
	if preHookCommand != "" {
		found := false
		for _, call := range calls {
			if call.Program == preHookCommand {
				found = true
				break
			}
		}
		assert.True(t, found, "pre-hook should execute")
	}
	if postHookCommand != "" {
		found := false
		for _, call := range calls {
			if call.Program == postHookCommand {
				found = true
				break
			}
		}
		assert.True(t, found, "post-hook should execute")
	}
}

// TestExecutePluginOperation_InputResolution verifies that OperationInputs
// are correctly resolved via interpolation before passing to provider.
func TestExecutePluginOperation_InputResolution(t *testing.T) {
	tests := []struct {
		name          string
		inputs        map[string]any
		workflowInput map[string]any
		expectedValue any
		expectError   bool
	}{
		{
			name:          "string input with template -> interpolated",
			inputs:        map[string]any{"message": "Hello {{inputs.name}}"},
			workflowInput: map[string]any{"name": "World"},
			expectedValue: "Hello World",
		},
		{
			name: "map input with nested templates -> recursively interpolated",
			inputs: map[string]any{
				"config": map[string]any{
					"key1": "value-{{inputs.id}}",
					"key2": "static",
				},
			},
			workflowInput: map[string]any{"id": "123"},
			expectedValue: map[string]any{"key1": "value-123", "key2": "static"},
		},
		{
			name: "array input -> each element interpolated",
			inputs: map[string]any{
				"items": []any{"{{inputs.first}}", "static", "{{inputs.second}}"},
			},
			workflowInput: map[string]any{"first": "A", "second": "B"},
			expectedValue: []any{"A", "static", "B"},
		},
		{
			name:          "primitive input -> passed through unchanged",
			inputs:        map[string]any{"count": 42, "enabled": true},
			workflowInput: map[string]any{},
			expectedValue: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockOperationProviderWithCapture()
			provider.addOperation("test.operation", "Test operation", "test-plugin")

			step := builders.NewStepBuilder("operation").
				WithType(workflow.StepTypeOperation).
				WithOperation("test.operation", tt.inputs).
				WithOnSuccess("done").
				Build()

			wf := &workflow.Workflow{
				Name:    "input-test",
				Initial: "operation",
				Steps: map[string]*workflow.Step{
					"operation": step,
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("input-test", wf).
				Build()
			execSvc.SetOperationProvider(provider)

			ctx, err := execSvc.Run(context.Background(), "input-test", tt.workflowInput)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, workflow.StatusCompleted, ctx.Status)

				// Check captured inputs match expected
				capturedInputs := provider.capturedInputs["test.operation"]
				require.NotNil(t, capturedInputs, "provider should have captured inputs")

				// Verify specific input value based on test case
				for key := range tt.inputs {
					switch key {
					case "message", "config", "items", "count":
						assert.Equal(t, tt.expectedValue, capturedInputs[key])
					}
				}
			}
		})
	}
}

// TestExecutePluginOperation_OutputSerialization verifies that operation outputs
// are correctly serialized and stored in step state.
func TestExecutePluginOperation_OutputSerialization(t *testing.T) {
	tests := []struct {
		name           string
		outputs        map[string]any
		expectInState  bool
		expectedOutput any
	}{
		{
			name:           "outputs map serialized to step state",
			outputs:        map[string]any{"result": "success"},
			expectInState:  true,
			expectedOutput: "result=success",
		},
		{
			name:          "nil outputs -> empty step output",
			outputs:       nil,
			expectInState: false,
		},
		{
			name: "nested output structures serialized correctly",
			outputs: map[string]any{
				"data": map[string]any{
					"nested": "value",
					"array":  []any{1, 2, 3},
				},
			},
			expectInState:  true,
			expectedOutput: "data=map[array:[1 2 3] nested:value]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockOperationProvider()
			provider.addOperation("test.operation", "Test operation", "test-plugin")
			provider.results["test.operation"] = &pluginmodel.OperationResult{
				Success: true,
				Outputs: tt.outputs,
			}

			wf := &workflow.Workflow{
				Name:    "output-test",
				Initial: "operation",
				Steps: map[string]*workflow.Step{
					"operation": builders.NewStepBuilder("operation").
						WithType(workflow.StepTypeOperation).
						WithOperation("test.operation", nil).
						WithOnSuccess("done").
						Build(),
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("output-test", wf).
				Build()
			execSvc.SetOperationProvider(provider)

			ctx, err := execSvc.Run(context.Background(), "output-test", nil)

			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, ctx.Status)

			state, exists := ctx.States["operation"]
			require.True(t, exists)

			if tt.expectInState {
				assert.NotNil(t, state.Output)
				assert.Equal(t, tt.expectedOutput, state.Output)
			}
		})
	}
}

// TestExecutePluginOperation_StepStateRecording verifies that step state
// is correctly recorded for all execution paths.
func TestExecutePluginOperation_StepStateRecording(t *testing.T) {
	tests := []struct {
		name           string
		providerError  error
		operationFails bool
		resultOutputs  map[string]any
		expectedStatus workflow.ExecutionStatus
		expectedOutput any
		expectError    bool
	}{
		{
			name:           "success: StatusCompleted, Output set, times recorded",
			resultOutputs:  map[string]any{"result": "ok"},
			expectedStatus: workflow.StatusCompleted,
			expectedOutput: "result=ok",
			expectError:    false,
		},
		{
			name:           "execution error: StatusFailed, Error set, times recorded",
			providerError:  errors.New("provider error"),
			expectedStatus: workflow.StatusFailed,
			expectError:    false,
		},
		{
			name:           "operation failure: StatusFailed, Error from result, times recorded",
			operationFails: true,
			expectedStatus: workflow.StatusFailed,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockOperationProvider()
			provider.addOperation("test.operation", "Test operation", "test-plugin")

			switch {
			case tt.providerError != nil:
				provider.execError = tt.providerError
			case tt.operationFails:
				provider.results["test.operation"] = &pluginmodel.OperationResult{
					Success: false,
					Error:   "operation failed",
				}
			default:
				provider.results["test.operation"] = &pluginmodel.OperationResult{
					Success: true,
					Outputs: tt.resultOutputs,
				}
			}

			step := builders.NewStepBuilder("operation").
				WithType(workflow.StepTypeOperation).
				WithOperation("test.operation", nil).
				WithOnSuccess("done").
				Build()

			if tt.providerError != nil || tt.operationFails {
				step.OnFailure = "failure_step"
			}

			wf := &workflow.Workflow{
				Name:    "state-test",
				Initial: "operation",
				Steps: map[string]*workflow.Step{
					"operation": step,
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
					"failure_step": {
						Name: "failure_step",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("state-test", wf).
				Build()
			execSvc.SetOperationProvider(provider)

			ctx, err := execSvc.Run(context.Background(), "state-test", nil)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			state, exists := ctx.States["operation"]
			require.True(t, exists)
			assert.Equal(t, tt.expectedStatus, state.Status)
			assert.Equal(t, "operation", state.Name)
			assert.False(t, state.StartedAt.IsZero(), "StartedAt should be recorded")
			assert.False(t, state.CompletedAt.IsZero(), "CompletedAt should be recorded")
			assert.Equal(t, 1, state.Attempt)

			// Verify error or output
			switch {
			case tt.providerError != nil:
				assert.Contains(t, state.Error, tt.providerError.Error())
			case tt.operationFails:
				assert.Equal(t, "operation failed", state.Error)
			case tt.expectedOutput != nil:
				assert.Equal(t, tt.expectedOutput, state.Output)
			}
		})
	}
}

// mockOperationProviderWithDelay simulates slow operations for timeout testing.
type mockOperationProviderWithDelay struct {
	*mockOperationProvider
	delay time.Duration
}

func newMockOperationProviderWithDelay(delay time.Duration) *mockOperationProviderWithDelay {
	return &mockOperationProviderWithDelay{
		mockOperationProvider: newMockOperationProvider(),
		delay:                 delay,
	}
}

func (m *mockOperationProviderWithDelay) Execute(
	ctx context.Context,
	name string,
	inputs map[string]any,
) (*pluginmodel.OperationResult, error) {
	// Simulate delay
	select {
	case <-time.After(m.delay):
		return m.mockOperationProvider.Execute(ctx, name, inputs)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// mockOperationProviderWithCapture captures inputs for verification.
type mockOperationProviderWithCapture struct {
	*mockOperationProvider
	capturedInputs map[string]map[string]any
}

func newMockOperationProviderWithCapture() *mockOperationProviderWithCapture {
	return &mockOperationProviderWithCapture{
		mockOperationProvider: newMockOperationProvider(),
		capturedInputs:        make(map[string]map[string]any),
	}
}

func (m *mockOperationProviderWithCapture) Execute(
	ctx context.Context,
	name string,
	inputs map[string]any,
) (*pluginmodel.OperationResult, error) {
	// Capture inputs
	m.capturedInputs[name] = inputs
	return m.mockOperationProvider.Execute(ctx, name, inputs)
}
