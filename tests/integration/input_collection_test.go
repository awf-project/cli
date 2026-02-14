//go:build integration

// Feature: F046
package integration_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

//
// Acceptance Criteria:
// - [ ] Detect missing required inputs and prompt interactively
// - [ ] Display enum options for constrained inputs
// - [ ] Validate enum selections and retry on invalid input
// - [ ] Execute command with collected values
// - [ ] Skip optional inputs when empty
// - [ ] Use default values for optional inputs
// - [ ] Show validation errors with helpful messages
// - [ ] Allow error recovery with corrected values
// - [ ] Handle Ctrl+C gracefully
//

// US1: Prompt for Required Inputs - Happy Path Tests

// F046: US1-AC1 - Prompt for missing required inputs
// Given: I run a command with missing required inputs
// When: The CLI detects missing required parameters
// Then: It prompts me interactively for each missing required input
func TestInputCollection_PromptForMissingRequired_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Simulate user input: provide required_string value
	stdin := strings.NewReader("test-value\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "input-collection-test", "--storage", tmpDir})

	err := cmd.Execute()

	// Should succeed after prompting for missing input
	require.NoError(t, err, "should succeed after collecting missing input")

	output := stdout.String()

	// Should show prompt for required input
	assert.Contains(t, output, "required_string", "should prompt for required_string")

	// Should execute workflow with collected value and default optional value
	assert.Contains(t, output, "test-value", "should use collected required_string value")
	assert.Contains(t, output, "42", "should use default value for optional_number")
}

// F046: US1-AC2 - Display enum options
// Given: A required input has an enum constraint
// When: I am prompted
// Then: I see the available enum options and can select one
func TestInputCollection_DisplayEnumOptions_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Simulate user input: select option 2 (staging)
	stdin := strings.NewReader("2\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "input-enum-test", "--storage", tmpDir})

	err := cmd.Execute()

	require.NoError(t, err, "should succeed after enum selection")

	output := stdout.String()

	// Should display enum options as numbered list
	assert.Contains(t, output, "1)", "should show numbered option 1")
	assert.Contains(t, output, "dev", "should show enum option 'dev'")
	assert.Contains(t, output, "staging", "should show enum option 'staging'")
	assert.Contains(t, output, "prod", "should show enum option 'prod'")

	// Should execute with selected value (2 = staging)
	assert.Contains(t, output, "Deploying to staging", "should deploy to staging environment")
}

// F046: US1-AC3 - Validate enum selection
// Given: I provide an invalid enum value when prompted
// When: I submit
// Then: I see an error message and the prompt repeats
func TestInputCollection_InvalidEnumRetry_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Simulate user input: invalid selection 4, then valid selection 1
	stdin := strings.NewReader("4\n1\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "input-enum-test", "--storage", tmpDir})

	err := cmd.Execute()

	require.NoError(t, err, "should succeed after retry with valid selection")

	output := stdout.String()

	// Should show error for invalid selection
	assert.Contains(t, output, "Invalid", "should show validation error for invalid selection")

	// Should execute with corrected value (1 = dev)
	assert.Contains(t, output, "Deploying to dev", "should deploy to dev after valid selection")
}

// F046: US1-AC4 - Execute with collected values
// Given: All required inputs are satisfied interactively
// When: I complete all prompts
// Then: The command executes with the collected values
func TestInputCollection_ExecuteWithCollectedValues_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Simulate user input: required_string value
	stdin := strings.NewReader("hello-world\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "input-collection-test", "--storage", tmpDir})

	err := cmd.Execute()

	require.NoError(t, err, "should execute successfully with collected inputs")

	output := stdout.String()

	// Should execute workflow with collected value
	assert.Contains(t, output, "hello-world", "should use collected required_string value")
	assert.Contains(t, output, "42", "should use default optional_number value")
	assert.Contains(t, output, "completed successfully", "should complete workflow")
}

// US2: Skip Optional Inputs - Happy Path Tests

// F046: US2-AC1 - Skip optional inputs
// Given: I am prompted for an optional input
// When: I press Enter without providing a value
// Then: The prompt accepts the empty input and moves to the next field
func TestInputCollection_SkipOptionalInput_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Simulate user input: required_name, skip optional_timeout, skip optional_no_default
	stdin := strings.NewReader("my-name\n\n\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "input-optional-test", "--storage", tmpDir})

	err := cmd.Execute()

	require.NoError(t, err, "should succeed after skipping optional inputs")

	output := stdout.String()

	// Should use collected required value
	assert.Contains(t, output, "my-name", "should use collected required_name")

	// Should use default value for skipped optional_timeout
	assert.Contains(t, output, "30", "should use default timeout value when skipped")

	// Should complete successfully
	assert.Contains(t, output, "completed successfully", "should complete workflow")
}

// F046: US2-AC2 - Use default values
// Given: An optional input has a default value
// When: I skip it
// Then: The default value is used
func TestInputCollection_UseDefaultValue_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Simulate user input: required value, skip optional (should use default 42)
	stdin := strings.NewReader("test\n\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "input-collection-test", "--storage", tmpDir})

	err := cmd.Execute()

	require.NoError(t, err, "should succeed with default value")

	output := stdout.String()

	// Should use provided required value
	assert.Contains(t, output, "test", "should use collected required_string")

	// Should use default value for skipped optional
	assert.Contains(t, output, "42", "should use default value 42 for optional_number")

	// Should complete successfully
	assert.Contains(t, output, "completed successfully", "should complete workflow")
}

// US3: Validation and Error Recovery - Tests

// F046: US3-AC1 - Validation with error messages
// Given: I provide an invalid value for an input with validation rules
// When: I submit
// Then: I see a specific error message explaining the constraint
func TestInputCollection_ValidationErrorMessage_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Simulate user input: invalid enum "production", then valid "prod"
	stdin := strings.NewReader("production\nprod\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "input-enum-test", "--storage", tmpDir})

	err := cmd.Execute()

	require.NoError(t, err, "should succeed after validation error and correction")

	output := stdout.String()

	// Should show validation error
	assert.Contains(t, output, "Invalid", "should show validation error for 'production'")

	// Should execute with corrected value
	assert.Contains(t, output, "Deploying to prod", "should deploy to prod after correction")
}

// F046: US3-AC2 - Error recovery
// Given: Input validation fails
// When: I provide a corrected value
// Then: The prompt accepts it and continues
func TestInputCollection_ErrorRecovery_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Simulate user input: invalid then valid enum value
	stdin := strings.NewReader("invalid\ndev\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{"run", "input-enum-test", "--storage", tmpDir})

	err := cmd.Execute()

	require.NoError(t, err, "should succeed after error recovery")

	output := stdout.String()

	// Should show error for invalid value
	assert.Contains(t, output, "Invalid", "should show validation error")

	// Should execute with corrected value
	assert.Contains(t, output, "Deploying to dev", "should deploy after providing valid value")

	// Should complete successfully
	assert.Contains(t, output, "completed successfully", "should complete workflow")
}

// F046: US3-AC3 - Graceful cancellation
// Given: I want to cancel interactive mode
// When: I press Ctrl+C
// Then: The command exits gracefully
func TestInputCollection_GracefulCancellation_Integration(t *testing.T) {
	t.Skip("Ctrl+C simulation requires signal handling, testing via manual QA")

	// Note: This test is difficult to automate because Ctrl+C sends SIGINT
	// Manual test procedure:
	// 1. Run: awf run input-collection-test (without --input flag)
	// 2. When prompted for required_string, press Ctrl+C
	// 3. Verify: Process exits cleanly with message "Input cancelled"
	// 4. Verify: No panic, no hanging process
}

// Edge Cases and Error Handling

// Test non-interactive stdin (no TTY) with missing inputs
func TestInputCollection_NonInteractiveStdin_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	// Don't set stdin - simulates non-interactive environment
	cmd.SetArgs([]string{"run", "input-collection-test", "--storage", tmpDir})

	err := cmd.Execute()

	// Should fail when required inputs missing and stdin is not a terminal
	require.Error(t, err, "should fail when stdin is not a terminal and inputs missing")

	// Error message should explain the problem
	errOutput := stderr.String()
	if errOutput == "" {
		errOutput = err.Error()
	}
	assert.Contains(t, errOutput, "missing", "error should mention missing inputs")
}

// Test all inputs provided via flags (no prompting needed)
func TestInputCollection_AllInputsProvided_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"run", "input-collection-test",
		"--input", "required_string=provided-value",
		"--input", "optional_number=99",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Should succeed without prompting when all inputs provided
	require.NoError(t, err, "should succeed without prompting")

	output := stdout.String()

	// Should use provided input values
	assert.Contains(t, output, "provided-value", "should use provided required_string")
	assert.Contains(t, output, "99", "should use provided optional_number")

	// Should NOT show input prompts
	assert.NotContains(t, output, "Enter value for", "should not prompt when all inputs provided")
}

// Test partial inputs provided (some via flags, some prompts needed)
func TestInputCollection_PartialInputsProvided_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Provide optional_timeout via flag, prompt for required_name
	stdin := strings.NewReader("interactive-name\n\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "input-optional-test",
		"--input", "optional_timeout=60",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	require.NoError(t, err, "should succeed with partial inputs")

	output := stdout.String()

	// Should prompt only for missing required input
	assert.Contains(t, output, "required_name", "should prompt for missing required_name")

	// Should use collected value
	assert.Contains(t, output, "interactive-name", "should use collected required_name")

	// Should use flag value (not default)
	assert.Contains(t, output, "60", "should use flag value 60 for optional_timeout")

	// Should complete successfully
	assert.Contains(t, output, "completed successfully", "should complete workflow")
}
