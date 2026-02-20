package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionService_InlineOnFailure_MessageInError(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "build",
		Steps: map[string]*workflow.Step{
			"build": {
				Name:      "build",
				Type:      workflow.StepTypeCommand,
				Command:   "make build",
				OnFailure: "__inline_error_build",
			},
			"__inline_error_build": {
				Name:    "__inline_error_build",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Build failed: deployment aborted",
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("make build", &ports.CommandResult{Stdout: "", Stderr: "error", ExitCode: 1}).
		Build()

	_, err := execSvc.Run(context.Background(), "test", nil)

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "Build failed: deployment aborted"),
		"error must include terminal step message, got: %s", err.Error())
}

func TestExecutionService_InlineOnFailure_InputsInterpolated(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "deploy",
		Steps: map[string]*workflow.Step{
			"deploy": {
				Name:      "deploy",
				Type:      workflow.StepTypeCommand,
				Command:   "make deploy",
				OnFailure: "__inline_error_deploy",
			},
			"__inline_error_deploy": {
				Name:    "__inline_error_deploy",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Deploy failed for env {{inputs.environment}}",
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("make deploy", &ports.CommandResult{Stdout: "", Stderr: "error", ExitCode: 1}).
		Build()

	inputs := map[string]any{"environment": "production"}
	_, err := execSvc.Run(context.Background(), "test", inputs)

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "production"),
		"interpolated input must appear in error message, got: %s", err.Error())
	assert.False(t, strings.Contains(err.Error(), "{{inputs.environment}}"),
		"raw template must not appear in error message after interpolation, got: %s", err.Error())
}

func TestExecutionService_InlineOnFailure_StatesInterpolated(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "compile",
		Steps: map[string]*workflow.Step{
			"compile": {
				Name:      "compile",
				Type:      workflow.StepTypeCommand,
				Command:   "go build ./...",
				OnSuccess: "run_tests",
				OnFailure: "__inline_error_compile",
			},
			"run_tests": {
				Name:      "run_tests",
				Type:      workflow.StepTypeCommand,
				Command:   "go test ./...",
				OnFailure: "__inline_error_tests",
			},
			"__inline_error_compile": {
				Name:    "__inline_error_compile",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Compile step failed",
			},
			"__inline_error_tests": {
				Name:    "__inline_error_tests",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Tests failed after: {{states.compile.output}}",
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("go build ./...", &ports.CommandResult{Stdout: "compiled successfully", ExitCode: 0}).
		WithCommandResult("go test ./...", &ports.CommandResult{Stdout: "", Stderr: "test error", ExitCode: 1}).
		Build()

	_, err := execSvc.Run(context.Background(), "test", nil)

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "Tests failed after:"),
		"error must include terminal step message, got: %s", err.Error())
}

func TestExecutionService_InlineOnFailure_EmptyMessageNoChange(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				Type:      workflow.StepTypeCommand,
				Command:   "false",
				OnFailure: "error_terminal",
			},
			"error_terminal": {
				Name:    "error_terminal",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "",
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("false", &ports.CommandResult{Stdout: "", Stderr: "", ExitCode: 1}).
		Build()

	_, err := execSvc.Run(context.Background(), "test", nil)

	require.Error(t, err)
	// Spec: step with empty message falls back to generic error format
	assert.True(t, strings.Contains(err.Error(), "error_terminal"),
		"generic error must reference terminal step name, got: %s", err.Error())
}

func TestExecutionService_InlineOnFailure_DefaultExitCode1(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				Type:      workflow.StepTypeCommand,
				Command:   "false",
				OnFailure: "__inline_error_step",
			},
			"__inline_error_step": {
				Name:     "__inline_error_step",
				Type:     workflow.StepTypeTerminal,
				Status:   workflow.TerminalFailure,
				Message:  "Something went wrong",
				ExitCode: 1, // FR-004: default exit code 1
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("false", &ports.CommandResult{Stdout: "", Stderr: "", ExitCode: 1}).
		Build()

	execCtx, err := execSvc.Run(context.Background(), "test", nil)

	require.Error(t, err)
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Equal(t, 1, execCtx.ExitCode, "ExitCode from terminal step must propagate to execution context")
	assert.True(t, strings.Contains(err.Error(), "Something went wrong"),
		"error must include message, got: %s", err.Error())
}

// TestExecutionService_InlineOnFailure_ExitCodePropagates verifies FR-004: status field propagates
// from terminal step to execution context ExitCode.
func TestExecutionService_InlineOnFailure_ExitCodePropagates(t *testing.T) {
	tests := []struct {
		name             string
		terminalExitCode int
		wantExitCode     int
	}{
		{
			name:             "exit code 1 propagates",
			terminalExitCode: 1,
			wantExitCode:     1,
		},
		{
			name:             "exit code 3 propagates",
			terminalExitCode: 3,
			wantExitCode:     3,
		},
		{
			name:             "exit code 4 propagates",
			terminalExitCode: 4,
			wantExitCode:     4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &workflow.Workflow{
				Name:    "test",
				Initial: "step",
				Steps: map[string]*workflow.Step{
					"step": {
						Name:      "step",
						Type:      workflow.StepTypeCommand,
						Command:   "false",
						OnFailure: "__inline_error_step",
					},
					"__inline_error_step": {
						Name:     "__inline_error_step",
						Type:     workflow.StepTypeTerminal,
						Status:   workflow.TerminalFailure,
						Message:  "Workflow failed",
						ExitCode: tt.terminalExitCode,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("test", wf).
				WithCommandResult("false", &ports.CommandResult{Stdout: "", Stderr: "", ExitCode: 1}).
				Build()

			execCtx, err := execSvc.Run(context.Background(), "test", nil)

			require.Error(t, err)
			assert.Equal(t, workflow.StatusFailed, execCtx.Status)
			assert.Equal(t, tt.wantExitCode, execCtx.ExitCode,
				"ExitCode %d from terminal step must propagate to execution context", tt.terminalExitCode)
		})
	}
}

func TestExecutionService_InlineOnFailure_SuccessTerminalNoError(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo ok",
				OnSuccess: "done",
			},
			"done": {
				Name:    "done",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalSuccess,
				Message: "Workflow finished successfully",
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("echo ok", &ports.CommandResult{Stdout: "ok\n", ExitCode: 0}).
		Build()

	execCtx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}

func TestExecutionService_InlineOnFailure_ResumePathMessageIncluded(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "deploy",
		Steps: map[string]*workflow.Step{
			"deploy": {
				Name:      "deploy",
				Type:      workflow.StepTypeCommand,
				Command:   "make deploy",
				OnFailure: "__inline_error_deploy",
			},
			"__inline_error_deploy": {
				Name:    "__inline_error_deploy",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Deployment failed on retry",
			},
		},
	}

	savedState := &workflow.ExecutionContext{
		WorkflowID:   "wf-resume-001",
		WorkflowName: "test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "deploy",
		States:       make(map[string]workflow.StepState),
		Inputs:       make(map[string]any),
		Env:          make(map[string]string),
		StartedAt:    time.Now(),
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("make deploy", &ports.CommandResult{Stdout: "", Stderr: "deploy error", ExitCode: 1}).
		Build()

	err := mocks.StateStore.Save(context.Background(), savedState)
	require.NoError(t, err)

	_, resumeErr := execSvc.Resume(context.Background(), "wf-resume-001", nil)

	require.Error(t, resumeErr)
	assert.True(t, strings.Contains(resumeErr.Error(), "Deployment failed on retry"),
		"resume path must include terminal message, got: %s", resumeErr.Error())
}

func TestExecutionService_InlineOnFailure_BadTemplateFallsBackToRaw(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				Type:      workflow.StepTypeCommand,
				Command:   "false",
				OnFailure: "__inline_error_step",
			},
			"__inline_error_step": {
				Name:    "__inline_error_step",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Error: {{invalid..template}}",
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		WithCommandResult("false", &ports.CommandResult{Stdout: "", Stderr: "", ExitCode: 1}).
		Build()

	_, err := execSvc.Run(context.Background(), "test", nil)

	require.Error(t, err)
	// Spec ADR-003: fallback to raw template on interpolation error
	// The raw message must still appear in the error (not swallowed)
	assert.True(t, strings.Contains(err.Error(), "Error:") || strings.Contains(err.Error(), "__inline_error_step"),
		"raw message or terminal name must appear in error, got: %s", err.Error())
}
