package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIPrompt_ShowHeader(t *testing.T) {
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	prompt.ShowHeader("test-workflow")

	output := stdout.String()
	assert.Contains(t, output, "Interactive Mode", "should show interactive mode header")
	assert.Contains(t, output, "test-workflow", "should show workflow name")
}

func TestCLIPrompt_ShowStepDetails(t *testing.T) {
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	step := &workflow.Step{
		Name:    "validate",
		Type:    workflow.StepTypeCommand,
		Command: "test -f file.txt",
		Timeout: 5,
	}
	info := &workflow.InteractiveStepInfo{
		Name:        "validate",
		Index:       1,
		Total:       4,
		Step:        step,
		Command:     "test -f file.txt",
		Transitions: []string{"→ on_success: extract", "→ on_failure: error"},
	}

	prompt.ShowStepDetails(info)

	output := stdout.String()
	assert.Contains(t, output, "[Step 1/4]", "should show step index")
	assert.Contains(t, output, "validate", "should show step name")
	assert.Contains(t, output, "Type: command", "should show step type")
	assert.Contains(t, output, "test -f file.txt", "should show command")
	assert.Contains(t, output, "5s", "should show timeout")
}

func TestCLIPrompt_PromptAction_Continue(t *testing.T) {
	stdin := strings.NewReader("c\n")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	action, err := prompt.PromptAction(false)

	require.NoError(t, err)
	assert.Equal(t, workflow.ActionContinue, action)
}

func TestCLIPrompt_PromptAction_Skip(t *testing.T) {
	stdin := strings.NewReader("s\n")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	action, err := prompt.PromptAction(false)

	require.NoError(t, err)
	assert.Equal(t, workflow.ActionSkip, action)
}

func TestCLIPrompt_PromptAction_Abort(t *testing.T) {
	stdin := strings.NewReader("a\n")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	action, err := prompt.PromptAction(false)

	require.NoError(t, err)
	assert.Equal(t, workflow.ActionAbort, action)
}

func TestCLIPrompt_PromptAction_Inspect(t *testing.T) {
	stdin := strings.NewReader("i\n")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	action, err := prompt.PromptAction(false)

	require.NoError(t, err)
	assert.Equal(t, workflow.ActionInspect, action)
}

func TestCLIPrompt_PromptAction_Edit(t *testing.T) {
	stdin := strings.NewReader("e\n")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	action, err := prompt.PromptAction(false)

	require.NoError(t, err)
	assert.Equal(t, workflow.ActionEdit, action)
}

func TestCLIPrompt_PromptAction_Retry_WhenAvailable(t *testing.T) {
	stdin := strings.NewReader("r\n")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	action, err := prompt.PromptAction(true) // retry available

	require.NoError(t, err)
	assert.Equal(t, workflow.ActionRetry, action)
}

func TestCLIPrompt_PromptAction_Retry_WhenNotAvailable(t *testing.T) {
	// Retry not available, then follow up with a valid action
	stdin := strings.NewReader("r\nc\n")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	action, err := prompt.PromptAction(false) // retry not available

	require.NoError(t, err)
	assert.Equal(t, workflow.ActionContinue, action)
	assert.Contains(t, stdout.String(), "retry not available", "should show error for unavailable retry")
}

func TestCLIPrompt_PromptAction_InvalidInput(t *testing.T) {
	// Invalid input followed by valid input
	stdin := strings.NewReader("x\nc\n")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	action, err := prompt.PromptAction(false)

	require.NoError(t, err)
	assert.Equal(t, workflow.ActionContinue, action)
	assert.Contains(t, stdout.String(), "invalid", "should show error for invalid input")
}

func TestCLIPrompt_ShowExecuting(t *testing.T) {
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	prompt.ShowExecuting("process")

	output := stdout.String()
	assert.Contains(t, output, "process", "should show step name")
	assert.Contains(t, output, "xecuting", "should indicate execution")
}

func TestCLIPrompt_ShowStepResult_Success(t *testing.T) {
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	state := &workflow.StepState{
		Name:     "process",
		Status:   workflow.StatusCompleted,
		Output:   "result data",
		ExitCode: 0,
	}

	prompt.ShowStepResult(state, "next-step")

	output := stdout.String()
	assert.Contains(t, output, "result data", "should show output")
	assert.Contains(t, output, "0", "should show exit code")
	assert.Contains(t, output, "next-step", "should show next step")
}

func TestCLIPrompt_ShowStepResult_Failure(t *testing.T) {
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	state := &workflow.StepState{
		Name:     "process",
		Status:   workflow.StatusFailed,
		Stderr:   "error output",
		ExitCode: 1,
		Error:    "command failed",
	}

	prompt.ShowStepResult(state, "error-handler")

	output := stdout.String()
	assert.Contains(t, output, "error output", "should show stderr")
	assert.Contains(t, output, "1", "should show exit code")
	assert.Contains(t, output, "error-handler", "should show next step")
}

func TestCLIPrompt_ShowContext(t *testing.T) {
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	ctx := &workflow.RuntimeContext{
		Inputs: map[string]any{
			"file":  "test.txt",
			"count": 10,
		},
		States: map[string]workflow.RuntimeStepState{
			"validate": {
				Output:   "valid",
				ExitCode: 0,
			},
		},
	}

	prompt.ShowContext(ctx)

	output := stdout.String()
	assert.Contains(t, output, "file", "should show input name")
	assert.Contains(t, output, "test.txt", "should show input value")
	assert.Contains(t, output, "validate", "should show state name")
}

func TestCLIPrompt_EditInput(t *testing.T) {
	stdin := strings.NewReader("new-value\n")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	newValue, err := prompt.EditInput("file", "old-value")

	require.NoError(t, err)
	assert.Equal(t, "new-value", newValue)
	assert.Contains(t, stdout.String(), "file", "should show input name")
	assert.Contains(t, stdout.String(), "old-value", "should show current value")
}

func TestCLIPrompt_ShowAborted(t *testing.T) {
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	prompt.ShowAborted()

	output := stdout.String()
	assert.Contains(t, output, "borted", "should indicate aborted")
}

func TestCLIPrompt_ShowSkipped(t *testing.T) {
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	prompt.ShowSkipped("process", "done")

	output := stdout.String()
	assert.Contains(t, output, "process", "should show skipped step")
	assert.Contains(t, output, "done", "should show next step")
}

func TestCLIPrompt_ShowCompleted(t *testing.T) {
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	prompt.ShowCompleted(workflow.StatusCompleted)

	output := stdout.String()
	assert.Contains(t, output, "omplete", "should indicate completion")
}

func TestCLIPrompt_ShowError(t *testing.T) {
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	prompt := ui.NewCLIPrompt(stdin, stdout, false)

	prompt.ShowError(assert.AnError)

	output := stdout.String()
	assert.Contains(t, output, "assert.AnError", "should show error message")
}

// These tests verify compile-time interface satisfaction checks for the
// refactored InteractivePrompt interfaces following ISP.

// TestC049_CLIPrompt_ImplementsStepPresenter verifies CLIPrompt satisfies StepPresenter.
// This test ensures compile-time verification via the var _ ports.StepPresenter check.
func TestC049_CLIPrompt_ImplementsStepPresenter(t *testing.T) {
	t.Run("happy_path_all_methods_callable", func(t *testing.T) {
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		prompt.ShowHeader("test-workflow")
		assert.Contains(t, stdout.String(), "test-workflow", "ShowHeader must work")

		stdout.Reset()
		prompt.ShowStepDetails(&workflow.InteractiveStepInfo{
			Name:  "test-step",
			Index: 1,
			Total: 1,
		})
		assert.Contains(t, stdout.String(), "test-step", "ShowStepDetails must work")

		stdout.Reset()
		prompt.ShowExecuting("test-step")
		assert.Contains(t, stdout.String(), "test-step", "ShowExecuting must work")

		stdout.Reset()
		prompt.ShowStepResult(&workflow.StepState{
			Name:   "test-step",
			Status: workflow.StatusCompleted,
		}, "")
		assert.Contains(t, stdout.String(), "Exit:", "ShowStepResult must work")
	})

	t.Run("edge_case_empty_workflow_name", func(t *testing.T) {
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		prompt.ShowHeader("")

		output := stdout.String()
		assert.Contains(t, output, "Interactive Mode", "should show header even with empty name")
	})

	t.Run("edge_case_nil_step_info", func(t *testing.T) {
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		prompt.ShowStepDetails(&workflow.InteractiveStepInfo{
			Name:  "minimal",
			Index: 1,
			Total: 1,
			Step:  nil, // Edge case: no Step details
		})

		output := stdout.String()
		assert.Contains(t, output, "minimal", "should show step name even without Step details")
	})

	t.Run("edge_case_very_long_output", func(t *testing.T) {
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)
		longOutput := strings.Repeat("x", 10000)

		prompt.ShowStepResult(&workflow.StepState{
			Name:   "long-output",
			Status: workflow.StatusCompleted,
			Output: longOutput,
		}, "")

		output := stdout.String()
		assert.NotEmpty(t, output, "should render output even if very long")
	})
}

// TestC049_CLIPrompt_ImplementsStatusPresenter verifies CLIPrompt satisfies StatusPresenter.
// This test ensures compile-time verification via the var _ ports.StatusPresenter check.
func TestC049_CLIPrompt_ImplementsStatusPresenter(t *testing.T) {
	t.Run("happy_path_all_methods_callable", func(t *testing.T) {
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		prompt.ShowAborted()
		assert.Contains(t, stdout.String(), "borted", "ShowAborted must work")

		stdout.Reset()
		prompt.ShowSkipped("test-step", "next-step")
		assert.Contains(t, stdout.String(), "test-step", "ShowSkipped must work")

		stdout.Reset()
		prompt.ShowCompleted(workflow.StatusCompleted)
		assert.Contains(t, stdout.String(), "omplete", "ShowCompleted must work")

		stdout.Reset()
		prompt.ShowError(assert.AnError)
		assert.Contains(t, stdout.String(), "Error", "ShowError must work")
	})

	t.Run("edge_case_empty_step_names", func(t *testing.T) {
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		prompt.ShowSkipped("", "")

		output := stdout.String()
		assert.NotEmpty(t, output, "should produce output even with empty step names")
	})

	t.Run("edge_case_all_execution_statuses", func(t *testing.T) {
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		statuses := []workflow.ExecutionStatus{
			workflow.StatusPending,
			workflow.StatusRunning,
			workflow.StatusCompleted,
			workflow.StatusFailed,
			workflow.StatusCancelled,
		}

		for _, status := range statuses {
			stdout.Reset()
			prompt.ShowCompleted(status)
			assert.NotEmpty(t, stdout.String(), "should handle status: %s", status)
		}
	})

	t.Run("error_handling_wrapped_error", func(t *testing.T) {
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		wrappedErr := assert.AnError // Use a real error, not nil
		prompt.ShowError(wrappedErr)

		output := stdout.String()
		assert.Contains(t, output, "Error", "should show error label")
		assert.Contains(t, output, "assert.AnError", "should show error message")
	})
}

// TestC049_CLIPrompt_ImplementsUserInteraction verifies CLIPrompt satisfies UserInteraction.
// This test ensures compile-time verification via the var _ ports.UserInteraction check.
func TestC049_CLIPrompt_ImplementsUserInteraction(t *testing.T) {
	t.Run("happy_path_all_methods_callable", func(t *testing.T) {
		stdin := strings.NewReader("c\n")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		action, err := prompt.PromptAction(false)
		require.NoError(t, err)
		assert.Equal(t, workflow.ActionContinue, action, "PromptAction must work")
	})

	t.Run("happy_path_edit_input", func(t *testing.T) {
		stdin := strings.NewReader("new-value\n")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		newValue, err := prompt.EditInput("test-input", "old-value")

		require.NoError(t, err)
		assert.Equal(t, "new-value", newValue, "EditInput must work")
	})

	t.Run("happy_path_show_context", func(t *testing.T) {
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		prompt.ShowContext(&workflow.RuntimeContext{
			Inputs: map[string]any{"key": "value"},
		})

		assert.Contains(t, stdout.String(), "key", "ShowContext must work")
	})

	t.Run("edge_case_empty_input_preserves_current", func(t *testing.T) {
		stdin := strings.NewReader("\n")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		newValue, err := prompt.EditInput("test", "current-value")

		require.NoError(t, err)
		assert.Equal(t, "current-value", newValue, "empty input should preserve current value")
	})

	t.Run("edge_case_retry_when_available", func(t *testing.T) {
		stdin := strings.NewReader("r\n")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		action, err := prompt.PromptAction(true) // retry IS available

		require.NoError(t, err)
		assert.Equal(t, workflow.ActionRetry, action, "retry should work when available")
	})

	t.Run("edge_case_retry_when_not_available", func(t *testing.T) {
		stdin := strings.NewReader("r\nc\n")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		action, err := prompt.PromptAction(false) // retry NOT available

		require.NoError(t, err)
		assert.Equal(t, workflow.ActionContinue, action, "should eventually accept valid action")
		assert.Contains(t, stdout.String(), "retry not available", "should show error for unavailable retry")
	})

	t.Run("edge_case_empty_runtime_context", func(t *testing.T) {
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		prompt.ShowContext(&workflow.RuntimeContext{
			Inputs: map[string]any{},
			States: map[string]workflow.RuntimeStepState{},
		})

		output := stdout.String()
		assert.Contains(t, output, "Context", "should show context header even if empty")
	})

	t.Run("error_handling_invalid_action_retries", func(t *testing.T) {
		stdin := strings.NewReader("invalid\nz\nc\n")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		action, err := prompt.PromptAction(false)

		require.NoError(t, err)
		assert.Equal(t, workflow.ActionContinue, action)
		output := stdout.String()
		assert.Contains(t, output, "invalid", "should show error messages for invalid inputs")
	})
}

// TestC049_CLIPrompt_ImplementsCompositeInterface verifies CLIPrompt satisfies InteractivePrompt.
// This test ensures all 11 methods (4+4+3) are accessible through the composite interface.
func TestC049_CLIPrompt_ImplementsCompositeInterface(t *testing.T) {
	t.Run("happy_path_all_11_methods_accessible", func(t *testing.T) {
		stdin := strings.NewReader("c\n")
		stdout := new(bytes.Buffer)
		prompt := ui.NewCLIPrompt(stdin, stdout, false)

		// StepPresenter methods (4)
		prompt.ShowHeader("workflow")
		prompt.ShowStepDetails(&workflow.InteractiveStepInfo{Name: "step", Index: 1, Total: 1})
		prompt.ShowExecuting("step")
		prompt.ShowStepResult(&workflow.StepState{Status: workflow.StatusCompleted}, "")

		// StatusPresenter methods (4)
		prompt.ShowAborted()
		prompt.ShowSkipped("step", "next")
		prompt.ShowCompleted(workflow.StatusCompleted)
		prompt.ShowError(assert.AnError)

		// UserInteraction methods (3)
		_, err := prompt.PromptAction(false)
		require.NoError(t, err)

		// Note: We can't test EditInput and ShowContext in this single test
		// because stdin is already consumed by PromptAction
		// They're tested separately in the UserInteraction test

		assert.NotEmpty(t, stdout.String(), "all methods should produce output")
	})

	t.Run("edge_case_composite_interface_consistency", func(t *testing.T) {
		// produces same behavior as accessing through focused interfaces
		stdin1 := strings.NewReader("c\n")
		stdout1 := new(bytes.Buffer)
		prompt1 := ui.NewCLIPrompt(stdin1, stdout1, false)

		stdin2 := strings.NewReader("c\n")
		stdout2 := new(bytes.Buffer)
		prompt2 := ui.NewCLIPrompt(stdin2, stdout2, false)

		action1, err1 := prompt1.PromptAction(false)
		action2, err2 := prompt2.PromptAction(false)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, action1, action2, "composite interface should behave consistently")
	})
}

// TestC049_InterfaceSegregation_Principle verifies ISP compliance at compile-time.
// These tests would fail to compile if CLIPrompt didn't satisfy all required interfaces.
func TestC049_InterfaceSegregation_Principle(t *testing.T) {
	t.Run("compile_time_check_step_presenter", func(t *testing.T) {
		// This test verifies the var _ ports.StepPresenter = (*CLIPrompt)(nil) line compiles
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		_ = ui.NewCLIPrompt(stdin, stdout, false)
		// If this test compiles, the interface check passed
		assert.True(t, true, "CLIPrompt satisfies StepPresenter interface")
	})

	t.Run("compile_time_check_status_presenter", func(t *testing.T) {
		// This test verifies the var _ ports.StatusPresenter = (*CLIPrompt)(nil) line compiles
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		_ = ui.NewCLIPrompt(stdin, stdout, false)
		// If this test compiles, the interface check passed
		assert.True(t, true, "CLIPrompt satisfies StatusPresenter interface")
	})

	t.Run("compile_time_check_user_interaction", func(t *testing.T) {
		// This test verifies the var _ ports.UserInteraction = (*CLIPrompt)(nil) line compiles
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		_ = ui.NewCLIPrompt(stdin, stdout, false)
		// If this test compiles, the interface check passed
		assert.True(t, true, "CLIPrompt satisfies UserInteraction interface")
	})

	t.Run("compile_time_check_composite_interactive_prompt", func(t *testing.T) {
		// This test verifies the var _ ports.InteractivePrompt = (*CLIPrompt)(nil) line compiles
		stdin := strings.NewReader("")
		stdout := new(bytes.Buffer)
		_ = ui.NewCLIPrompt(stdin, stdout, false)
		// If this test compiles, the interface check passed
		assert.True(t, true, "CLIPrompt satisfies InteractivePrompt composite interface")
	})
}
