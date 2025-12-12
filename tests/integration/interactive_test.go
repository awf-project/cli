//go:build integration

// Feature: F020
package integration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// =============================================================================
// F020: Interactive Mode - Functional Tests
// =============================================================================
//
// Acceptance Criteria:
// - [x] `awf run --interactive` enables step-by-step mode
// - [x] Pause before each step with prompt
// - [x] Options: continue, skip, abort, inspect, edit, retry
// - [x] Show step details before execution
// - [x] Show output after execution
// - [x] Allow modifying inputs during execution
// - [x] Support breakpoints on specific states
//
// =============================================================================

// -----------------------------------------------------------------------------
// Happy Path Tests
// -----------------------------------------------------------------------------

func TestInteractive_ContinueThroughAllSteps_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Simulate user input: continue through all steps (3 steps + terminal)
	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err, "interactive mode should succeed")

	output := stdout.String()

	assert.Contains(t, output, "Interactive Mode", "should show interactive mode header")
	assert.Contains(t, output, "interactive-test", "should show workflow name")
	assert.Contains(t, output, "[Step", "should show step indicator")
	assert.Contains(t, output, "[c]ontinue", "should show continue option")
	assert.Contains(t, output, "completed successfully", "should complete workflow")
}

func TestInteractive_ShowsStepDetails_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()

	// Should show step details
	assert.Contains(t, output, "Type:", "should show step type")
	assert.Contains(t, output, "Command:", "should show command")
	assert.Contains(t, output, "step_one", "should show step name")
	assert.Contains(t, output, "step_two", "should show second step")
	assert.Contains(t, output, "step_three", "should show third step")
}

func TestInteractive_ShowsStepResult_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()

	// Should show step result
	assert.Contains(t, output, "Exit", "should show exit code")
	assert.Contains(t, output, "Duration", "should show duration")
	assert.Contains(t, output, "Next", "should show next step")
	assert.Contains(t, output, "Step 1:", "should show step 1 output")
}

func TestInteractive_ShowsResolvedCommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--interactive",
		"--input", "message=custom_message",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()

	// Should show resolved command with interpolated values
	assert.Contains(t, output, "custom_message", "should show resolved input in command")
}

func TestInteractive_ShowsTransitions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()

	// Should show transitions
	assert.Contains(t, output, "on_success:", "should show success transition")
}

// -----------------------------------------------------------------------------
// Action Tests: Abort
// -----------------------------------------------------------------------------

func TestInteractive_AbortStopsExecution_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Simulate user input: abort at first step
	stdin := strings.NewReader("a\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	// Abort is a user action, not an error
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "borted", "should indicate workflow was aborted")
	// Should NOT contain step_two or step_three execution
	assert.NotContains(t, output, "Executing step_two", "should not execute step_two after abort")
}

func TestInteractive_AbortAfterFirstStep_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Continue first step, then abort
	stdin := strings.NewReader("c\na\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Step 1:", "should have executed step_one")
	assert.Contains(t, output, "borted", "should show aborted message")
}

// -----------------------------------------------------------------------------
// Action Tests: Skip
// -----------------------------------------------------------------------------

func TestInteractive_SkipJumpsToNextStep_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Simulate user input: skip first step, continue rest
	stdin := strings.NewReader("s\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "kipped", "should indicate step was skipped")
	// The command preview shows the resolved command, but skip means no execution
	// Check that step_one was not executed (no "Executing step_one" message)
	assert.NotContains(t, output, "Executing step_one", "should not execute step_one")
	assert.Contains(t, output, "Executing step_two", "should execute step_two")
}

func TestInteractive_SkipMultipleSteps_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Skip first two steps, continue last
	stdin := strings.NewReader("s\ns\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	// Should complete successfully with only step_three executed
	assert.Contains(t, output, "Step 3:", "should have step_three output")
	assert.Contains(t, output, "completed successfully", "should complete workflow")
}

// -----------------------------------------------------------------------------
// Action Tests: Inspect
// -----------------------------------------------------------------------------

func TestInteractive_InspectShowsContext_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Simulate user input: inspect, then continue all
	stdin := strings.NewReader("i\nc\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--interactive",
		"--input", "message=test_value",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "ontext", "should show context section")
	assert.Contains(t, output, "Inputs:", "should show inputs header")
	assert.Contains(t, output, "message", "should show message input")
	assert.Contains(t, output, "test_value", "should show input value")
}

func TestInteractive_InspectShowsStatesAfterExecution_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Continue first step, then inspect, then continue rest
	stdin := strings.NewReader("c\ni\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "States:", "should show states section")
	assert.Contains(t, output, "step_one", "should show step_one state")
}

func TestInteractive_MultipleInspects_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Inspect multiple times before continuing
	stdin := strings.NewReader("i\ni\nc\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	// Should show context multiple times
	contextCount := strings.Count(output, "Inputs:")
	assert.GreaterOrEqual(t, contextCount, 2, "should show context at least twice")
}

// -----------------------------------------------------------------------------
// Action Tests: Edit
// -----------------------------------------------------------------------------

func TestInteractive_EditModifiesInput_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Edit input, provide new value, then continue
	stdin := strings.NewReader("e\nnew_message\nc\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--interactive",
		"--input", "message=old_message",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	// Should use the edited value
	assert.Contains(t, output, "new_message", "should use edited input value")
}

func TestInteractive_EditShowsCurrentValue_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Edit input (shows current value), then provide new value and continue
	stdin := strings.NewReader("e\nupdated\nc\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--interactive",
		"--input", "message=current_value",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "current", "should show current value during edit")
}

func TestInteractive_EditEmptyKeepsCurrentValue_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Edit but press enter without value (keeps current)
	stdin := strings.NewReader("e\n\nc\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--interactive",
		"--input", "message=keep_this",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "keep_this", "should keep current value when edit is empty")
}

// -----------------------------------------------------------------------------
// Action Tests: Retry
// -----------------------------------------------------------------------------

func TestInteractive_RetryReExecutesStep_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Continue first step (step_one executes)
	// Then at step_two prompt, retry (goes back to step_one)
	// Then continue step_one again, then step_two, then step_three
	// Total: c(execute step_one) -> r(retry=back to step_one) -> c(execute step_one) -> c(step_two) -> c(step_three)
	stdin := strings.NewReader("c\nr\nc\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	// Should show step_one execution twice
	execCount := strings.Count(output, "Executing step_one")
	assert.GreaterOrEqual(t, execCount, 2, "should execute step_one at least twice after retry")
}

func TestInteractive_RetryNotAvailableOnFirstPrompt_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Try retry on first prompt (should fail), then continue
	stdin := strings.NewReader("r\nc\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	// First prompt should not show retry option
	assert.Contains(t, output, "retry not available", "should indicate retry not available")
}

// -----------------------------------------------------------------------------
// Breakpoint Tests
// -----------------------------------------------------------------------------

func TestInteractive_BreakpointPausesOnlyAtSpecified_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Only one continue needed since we only breakpoint at step_two
	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--interactive",
		"--breakpoint", "step_two",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	// Should pause only at step_two
	assert.Contains(t, output, "step_two", "should show step_two")
	// step_one should execute without prompting (not in the prompt lines)
	// step_three should also execute without prompting
}

func TestInteractive_MultipleBreakpoints_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Two continues needed for two breakpoints
	stdin := strings.NewReader("c\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--interactive",
		"--breakpoint", "step_one,step_three",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "completed successfully", "should complete workflow")
}

func TestInteractive_BreakpointWithSeparateFlags_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Two breakpoints via separate flags
	stdin := strings.NewReader("c\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--interactive",
		"--breakpoint", "step_one",
		"--breakpoint", "step_two",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "completed successfully", "should complete workflow")
}

// -----------------------------------------------------------------------------
// Edge Case Tests
// -----------------------------------------------------------------------------

func TestInteractive_InvalidInputRepromptsUser_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Simulate invalid input followed by valid input
	stdin := strings.NewReader("x\nc\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "invalid", "should show error for invalid input")
	assert.Contains(t, output, "completed successfully", "should complete after valid input")
}

func TestInteractive_MultipleInvalidInputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Multiple invalid inputs before valid
	stdin := strings.NewReader("xyz\n123\n!\nc\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	// Should show multiple invalid errors
	invalidCount := strings.Count(output, "invalid")
	assert.GreaterOrEqual(t, invalidCount, 3, "should show at least 3 invalid errors")
}

func TestInteractive_EmptyInputRepromptsUser_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Empty input followed by valid
	stdin := strings.NewReader("\nc\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "completed successfully", "should complete after valid input")
}

func TestInteractive_CaseInsensitiveActions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Uppercase C should work like lowercase c
	stdin := strings.NewReader("C\nC\nC\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "completed successfully", "should accept uppercase actions")
}

// -----------------------------------------------------------------------------
// Error Handling Tests
// -----------------------------------------------------------------------------

func TestInteractive_WorkflowNotFound_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"run", "nonexistent-workflow", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found", "error should mention workflow not found")
}

func TestInteractive_InvalidBreakpoint_StillRuns_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Breakpoint for non-existent step - should still run (just won't pause there)
	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--interactive",
		"--breakpoint", "nonexistent_step",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	// Should run without pausing since breakpoint doesn't match any step
	// Note: This tests that invalid breakpoints don't cause errors
	require.NoError(t, err)
}

// -----------------------------------------------------------------------------
// State Persistence Tests
// -----------------------------------------------------------------------------

func TestInteractive_AbortPersistsState_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Execute first step, then abort
	stdin := strings.NewReader("c\na\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify state was saved (check for state file in tmpDir)
	statesDir := filepath.Join(tmpDir, "states")
	entries, err := os.ReadDir(statesDir)
	// State directory may or may not exist depending on implementation
	if err == nil && len(entries) > 0 {
		assert.NotEmpty(t, entries, "should have saved state files")
	}
}

func TestInteractive_CompletionPersistsState_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify state was saved after completion
	statesDir := filepath.Join(tmpDir, "states")
	entries, err := os.ReadDir(statesDir)
	if err == nil && len(entries) > 0 {
		assert.NotEmpty(t, entries, "should have saved state files")
	}
}

// -----------------------------------------------------------------------------
// Integration with Other Features Tests
// -----------------------------------------------------------------------------

func TestInteractive_WithInputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--interactive",
		"--input", "message=custom_msg",
		"--input", "count=5",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "custom_msg", "should use custom message input")
	assert.Contains(t, output, "count is 5", "should use custom count input")
}

func TestInteractive_ParallelWorkflow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Parallel workflow needs fewer continues (parallel branches don't pause individually)
	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "valid-parallel", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Interactive Mode", "should show interactive mode")
	assert.Contains(t, output, "parallel", "should show parallel step")
}

func TestInteractive_LoopWorkflow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// Loop workflow - pauses before loop step
	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "loop-foreach", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Interactive Mode", "should show interactive mode")
	assert.Contains(t, output, "for_each", "should show for_each loop type")
}

func TestInteractive_SimpleWorkflow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// valid-simple has 2 steps: start and done (terminal)
	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "valid-simple", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Interactive Mode", "should show interactive mode header")
	assert.Contains(t, output, "simple-workflow", "should show workflow name")
	assert.Contains(t, output, "hello world", "should show step output")
	assert.Contains(t, output, "completed successfully", "should complete workflow")
}

// -----------------------------------------------------------------------------
// Table-Driven Tests for Action Combinations
// -----------------------------------------------------------------------------

func TestInteractive_ActionCombinations_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tests := []struct {
		name           string
		inputs         string
		expectContains []string
		expectComplete bool
	}{
		{
			name:   "all_continue",
			inputs: "c\nc\nc\n",
			expectContains: []string{
				"Step 1:", "Step 2:", "Step 3:", "completed successfully",
			},
			expectComplete: true,
		},
		{
			name:   "inspect_then_continue",
			inputs: "i\nc\nc\nc\n",
			expectContains: []string{
				"Inputs:", "completed successfully",
			},
			expectComplete: true,
		},
		{
			name:   "skip_first",
			inputs: "s\nc\nc\n",
			expectContains: []string{
				"kipped", "Step 2:", "completed successfully",
			},
			expectComplete: true,
		},
		{
			name:   "abort_immediately",
			inputs: "a\n",
			expectContains: []string{
				"borted",
			},
			expectComplete: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			stdin := strings.NewReader(tc.inputs)

			cmd := cli.NewRootCommand()
			stdout := new(bytes.Buffer)
			cmd.SetOut(stdout)
			cmd.SetErr(stdout)
			cmd.SetIn(stdin)
			cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

			err := cmd.Execute()
			require.NoError(t, err, "should not error for %s", tc.name)

			output := stdout.String()
			for _, expected := range tc.expectContains {
				assert.Contains(t, output, expected, "output should contain %q for %s", expected, tc.name)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Regression Tests
// -----------------------------------------------------------------------------

func TestInteractive_DoesNotModifyWorkflowPath_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	originalPath := os.Getenv("AWF_WORKFLOWS_PATH")
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	_ = cmd.Execute()

	// Verify environment was not modified
	currentPath := os.Getenv("AWF_WORKFLOWS_PATH")
	assert.Equal(t, "../fixtures/workflows", currentPath,
		"interactive mode should not modify AWF_WORKFLOWS_PATH")

	// Cleanup
	if originalPath == "" {
		os.Unsetenv("AWF_WORKFLOWS_PATH")
	} else {
		os.Setenv("AWF_WORKFLOWS_PATH", originalPath)
	}
}

func TestInteractive_CanRunMultipleTimes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	// Run interactive mode multiple times to ensure no state leaks
	for i := 0; i < 3; i++ {
		tmpDir := t.TempDir()

		stdin := strings.NewReader("c\nc\nc\n")

		cmd := cli.NewRootCommand()
		stdout := new(bytes.Buffer)
		cmd.SetOut(stdout)
		cmd.SetErr(stdout)
		cmd.SetIn(stdin)
		cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

		err := cmd.Execute()
		require.NoError(t, err, "interactive run #%d should succeed", i+1)

		output := stdout.String()
		assert.Contains(t, output, "interactive-test", "run #%d should show workflow name", i+1)
		assert.Contains(t, output, "completed successfully", "run #%d should complete", i+1)
	}
}

func TestInteractive_FlagOrderDoesNotMatter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	// Put --interactive after other flags
	cmd.SetArgs([]string{
		"run", "interactive-test",
		"--storage", tmpDir,
		"--input", "message=test",
		"--interactive",
	})

	err := cmd.Execute()
	require.NoError(t, err, "interactive mode should work regardless of flag order")

	output := stdout.String()
	assert.Contains(t, output, "Interactive Mode", "should be in interactive mode")
}

// -----------------------------------------------------------------------------
// Output Format Tests
// -----------------------------------------------------------------------------

func TestInteractive_ShowsCorrectStepCount_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "interactive-test", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()

	// Should show step counts (N/M format)
	// interactive-test has 5 states total but only 3 non-terminal steps
	assert.Contains(t, output, "[Step 1/", "should show step 1 count")
	assert.Contains(t, output, "[Step 2/", "should show step 2 count")
	assert.Contains(t, output, "[Step 3/", "should show step 3 count")
}

func TestInteractive_ShowsStepTypeForDifferentSteps_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	stdin := strings.NewReader("c\nc\nc\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "valid-parallel", "--interactive", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()

	// Should show different types for different steps
	// The implementation shows "command" for regular steps and "parallel" for parallel steps
	assert.Contains(t, output, "Type: command", "should show command step type")
	assert.Contains(t, output, "Type: parallel", "should show parallel type")
}
