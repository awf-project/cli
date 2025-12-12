package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// CLIPrompt Tests (F020)
// =============================================================================

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

	ctx := interpolation.NewContext()
	ctx.Inputs["file"] = "test.txt"
	ctx.Inputs["count"] = 10
	ctx.States["validate"] = interpolation.StepStateData{
		Output:   "valid",
		ExitCode: 0,
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
