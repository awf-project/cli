package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/testutil/builders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyErrorType tests the error type classification logic
// that maps error messages to error categories (user/workflow/execution/system).
//
// Coverage target: 53.3% -> 100%
//
// This function is private (classifyErrorType), so we test it indirectly
// through public API by triggering workflow executions that produce various
// error types and verifying the error classification via workflow-level error hook log output.
//
// Strategy: Create workflows with WorkflowError hooks that log {{error.type}}.
// Execute the workflow with mocked commands that return non-zero exit codes
// with specific error messages in stderr. Capture the log output from the error hook
// and verify the classified error type.
func TestClassifyErrorType(t *testing.T) {
	tests := []struct {
		name              string
		commandStderr     string
		commandExitCode   int
		expectedType      string
		wantWorkflowError bool
	}{
		{
			name:              "nil error returns empty string",
			commandStderr:     "",
			commandExitCode:   0,
			expectedType:      "",
			wantWorkflowError: false,
		},
		{
			name:              "terminal failure classified as workflow error",
			commandStderr:     "terminal failure detected",
			commandExitCode:   1,
			expectedType:      "workflow",
			wantWorkflowError: true,
		},
		{
			name:              "step not found classified as workflow error",
			commandStderr:     "step not found: missing_step",
			commandExitCode:   1,
			expectedType:      "workflow",
			wantWorkflowError: true,
		},
		{
			name:              "invalid state classified as workflow error",
			commandStderr:     "invalid state transition",
			commandExitCode:   1,
			expectedType:      "workflow",
			wantWorkflowError: true,
		},
		{
			name:              "cycle detected classified as workflow error",
			commandStderr:     "cycle detected in workflow graph",
			commandExitCode:   1,
			expectedType:      "workflow",
			wantWorkflowError: true,
		},
		{
			name:              "exit code classified as execution error",
			commandStderr:     "command failed with exit code 1",
			commandExitCode:   1,
			expectedType:      "execution",
			wantWorkflowError: true,
		},
		{
			name:              "timeout classified as execution error",
			commandStderr:     "operation timeout after 30s",
			commandExitCode:   1,
			expectedType:      "execution",
			wantWorkflowError: true,
		},
		{
			name:              "context deadline classified as execution error",
			commandStderr:     "context deadline exceeded",
			commandExitCode:   1,
			expectedType:      "execution",
			wantWorkflowError: true,
		},
		{
			name:              "command failed classified as execution error",
			commandStderr:     "command failed: invalid syntax",
			commandExitCode:   1,
			expectedType:      "execution",
			wantWorkflowError: true,
		},
		{
			name:              "not found classified as user error",
			commandStderr:     "file not found: config.yaml",
			commandExitCode:   1,
			expectedType:      "user",
			wantWorkflowError: true,
		},
		{
			name:              "missing classified as user error",
			commandStderr:     "missing required input: name",
			commandExitCode:   1,
			expectedType:      "user",
			wantWorkflowError: true,
		},
		{
			name:              "invalid input classified as user error",
			commandStderr:     "invalid input: expected number",
			commandExitCode:   1,
			expectedType:      "user",
			wantWorkflowError: true,
		},
		{
			name:              "validation error classified as user error",
			commandStderr:     "validation failed: field required",
			commandExitCode:   1,
			expectedType:      "user",
			wantWorkflowError: true,
		},
		{
			name:              "permission denied classified as system error",
			commandStderr:     "permission denied: /etc/config",
			commandExitCode:   1,
			expectedType:      "system",
			wantWorkflowError: true,
		},
		{
			name:              "access denied classified as system error",
			commandStderr:     "access denied to resource",
			commandExitCode:   1,
			expectedType:      "system",
			wantWorkflowError: true,
		},
		{
			name:              "IO error classified as system error",
			commandStderr:     "IO error: disk full",
			commandExitCode:   1,
			expectedType:      "system",
			wantWorkflowError: true,
		},
		{
			name:              "file system error classified as system error",
			commandStderr:     "file system error: read only",
			commandExitCode:   1,
			expectedType:      "system",
			wantWorkflowError: true,
		},
		{
			name:              "unknown error defaults to execution type",
			commandStderr:     "something went wrong",
			commandExitCode:   1,
			expectedType:      "execution",
			wantWorkflowError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wf *workflow.Workflow
			if tt.wantWorkflowError {
				// Create workflow with error hook that logs error.type
				wf = createWorkflowWithErrorHook()
			} else {
				// Create workflow that succeeds (no error)
				wf = createSuccessfulWorkflow()
			}

			svc, mocks := NewTestHarness(t).
				WithWorkflow("test", wf).
				Build()

			if tt.wantWorkflowError {
				// Configure command to return non-zero exit code with stderr
				mocks.Executor.SetCommandResult("test command", &ports.CommandResult{
					Stdout:   "",
					Stderr:   tt.commandStderr,
					ExitCode: tt.commandExitCode,
				})
			} else {
				// Configure command to succeed
				mocks.Executor.SetCommandResult("echo success", &ports.CommandResult{
					Stdout:   "success\n",
					Stderr:   "",
					ExitCode: 0,
				})
			}

			_, err := svc.Run(context.Background(), "test", nil)

			if !tt.wantWorkflowError {
				// Success case: no error expected
				require.NoError(t, err)
				// Verify error hook was NOT triggered
				messages := mocks.Logger.GetMessages()
				for _, msg := range messages {
					assert.NotContains(t, msg.Msg, "ERROR_TYPE:")
				}
			} else {
				// Error case: verify error type classification
				require.Error(t, err)

				// Find the error hook log message
				messages := mocks.Logger.GetMessages()
				var foundErrorType bool
				var actualLogMessage string

				for _, msg := range messages {
					if !strings.Contains(msg.Msg, "ERROR_TYPE:") {
						continue
					}
					foundErrorType = true
					actualLogMessage = msg.Msg
					expectedLog := fmt.Sprintf("ERROR_TYPE:%s", tt.expectedType)
					assert.Equal(t, expectedLog, msg.Msg,
						"Expected error type '%s' to be logged by error hook", tt.expectedType)
					break
				}

				assert.True(t, foundErrorType,
					"Expected WorkflowError hook to log error type, but no ERROR_TYPE log found. All messages: %+v",
					messages)

				if foundErrorType {
					expectedLog := fmt.Sprintf("ERROR_TYPE:%s", tt.expectedType)
					assert.Equal(t, expectedLog, actualLogMessage)
				}
			}
		})
	}
}

// TestClassifyErrorType_EdgeCases tests edge cases for error classification.
func TestClassifyErrorType_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		errorMessage string
		expectedType string
	}{
		{
			name:         "multiple matching patterns - first match wins (terminal failure)",
			errorMessage: "terminal failure with exit code 1",
			expectedType: "workflow", // "terminal failure" is checked before "exit code"
		},
		{
			name:         "case sensitive matching for error keywords",
			errorMessage: "operation timeout occurred",
			expectedType: "execution", // matches "timeout" via strings.Contains (case-sensitive)
		},
		{
			name:         "partial keyword match in longer string",
			errorMessage: "file not found in directory",
			expectedType: "user", // contains "not found" substring
		},
		{
			name:         "whitespace only error message defaults to execution",
			errorMessage: "   ",
			expectedType: "execution", // no patterns match, defaults to execution
		},
		{
			name:         "error with multiple classification keywords",
			errorMessage: "validation failed: file not found",
			expectedType: "user", // "validation" matches first (both are user errors anyway)
		},
		{
			name:         "permission substring in larger context",
			errorMessage: "no permission granted for operation",
			expectedType: "system", // contains "permission"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := createWorkflowWithErrorHook()

			svc, mocks := NewTestHarness(t).
				WithWorkflow("test", wf).
				Build()

			mocks.Executor.SetCommandResult("test command", &ports.CommandResult{
				Stdout:   "",
				Stderr:   tt.errorMessage,
				ExitCode: 1,
			})

			_, err := svc.Run(context.Background(), "test", nil)

			require.Error(t, err)

			messages := mocks.Logger.GetMessages()
			var foundErrorType bool
			for _, msg := range messages {
				if strings.Contains(msg.Msg, "ERROR_TYPE:") {
					foundErrorType = true
					expectedLog := fmt.Sprintf("ERROR_TYPE:%s", tt.expectedType)
					assert.Equal(t, expectedLog, msg.Msg,
						"Expected error type '%s' for message '%s'",
						tt.expectedType, tt.errorMessage)
					break
				}
			}

			assert.True(t, foundErrorType, "Expected ERROR_TYPE log to be present")
		})
	}
}

// TestClassifyErrorType_ErrorHookContext tests that error classification
// is available in error hook interpolation context with all error metadata.
func TestClassifyErrorType_ErrorHookContext(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Hooks: workflow.WorkflowHooks{
			WorkflowError: workflow.Hook{
				{Log: "ERROR_TYPE:{{error.type}}"},
				{Log: "ERROR_MESSAGE:{{error.message}}"},
				{Log: "ERROR_STATE:{{error.state}}"},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {
				Name:    "start",
				Type:    workflow.StepTypeCommand,
				Command: "test command",
			},
		},
	}

	svc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()

	mocks.Executor.SetCommandResult("test command", &ports.CommandResult{
		Stdout:   "",
		Stderr:   "timeout exceeded",
		ExitCode: 1,
	})

	_, err := svc.Run(context.Background(), "test", nil)

	require.Error(t, err)
	messages := mocks.Logger.GetMessages()

	// Verify all error context fields are available
	var foundType, foundMessage, foundState bool
	for _, msg := range messages {
		if msg.Msg == "ERROR_TYPE:execution" {
			foundType = true
		}
		if strings.Contains(msg.Msg, "ERROR_MESSAGE:") && strings.Contains(msg.Msg, "timeout exceeded") {
			foundMessage = true
		}
		if msg.Msg == "ERROR_STATE:start" {
			foundState = true
		}
	}

	assert.True(t, foundType, "error.type should be available in error hook")
	assert.True(t, foundMessage, "error.message should be available in error hook")
	assert.True(t, foundState, "error.state should be available in error hook")
}

// createWorkflowWithErrorHook creates a workflow with a single command step
// and a WorkflowError hook that logs the error type. The command will be
// configured to fail with a non-zero exit code and specific stderr in the test.
func createWorkflowWithErrorHook() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Hooks: workflow.WorkflowHooks{
			WorkflowError: workflow.Hook{
				{Log: "ERROR_TYPE:{{error.type}}"},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {
				Name:    "start",
				Type:    workflow.StepTypeCommand,
				Command: "test command",
			},
		},
	}
}

// createSuccessfulWorkflow creates a workflow that completes successfully
// without triggering any error hooks.
func createSuccessfulWorkflow() *workflow.Workflow {
	return builders.NewWorkflowBuilder().
		WithName("test").
		WithInitial("start").
		WithStep(
			builders.NewCommandStep("start", "echo success").
				WithOnSuccess("done").
				Build(),
		).
		WithStep(
			builders.NewTerminalStep("done", workflow.TerminalSuccess).Build(),
		).
		Build()
}
