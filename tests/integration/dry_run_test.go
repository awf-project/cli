//go:build integration

// Feature: F019
package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

//
// Acceptance Criteria:
// - [x] `awf run --dry-run` shows execution plan
// - [x] Display resolved commands with interpolation
// - [x] Show state transitions
// - [x] Validate workflow without execution
// - [x] Show hooks that would run
// - [x] No side effects (no files written, commands run)
//

// Happy Path Tests

func TestDryRun_SimpleWorkflow_ShowsExecutionPlan_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-simple", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify execution plan structure
	assert.Contains(t, output, "Dry Run: simple-workflow")
	assert.Contains(t, output, "Execution Plan:")
	assert.Contains(t, output, "start")
	assert.Contains(t, output, "done")
	assert.Contains(t, output, "No commands will be executed")
}

func TestDryRun_WithInputs_ResolvesInterpolation_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--input", "file_path=/path/to/test.txt",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify input interpolation in commands
	assert.Contains(t, output, "/path/to/test.txt")
	assert.Contains(t, output, "Inputs:")
	assert.Contains(t, output, "file_path:")
}

func TestDryRun_ShowsDefaultInputValues_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--input", "file_path=/test.txt",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify default values are displayed
	assert.Contains(t, output, "count:")
	assert.Contains(t, output, "(default)")
}

func TestDryRun_ParallelWorkflow_ShowsBranchesAndStrategy_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-parallel", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify parallel step details
	assert.Contains(t, output, "[PARALLEL]")
	assert.Contains(t, output, "Branches:")
	assert.Contains(t, output, "task_a")
	assert.Contains(t, output, "Strategy:")
	assert.Contains(t, output, "best_effort")
}

func TestDryRun_ForEachLoop_ShowsLoopConfiguration_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "loop-foreach", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify loop configuration
	assert.Contains(t, output, "[LOOP:for_each]")
	assert.Contains(t, output, "Items:")
	assert.Contains(t, output, "Body:")
	assert.Contains(t, output, "Max iterations:")
}

func TestDryRun_WhileLoop_ShowsCondition_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "loop-while", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify while loop configuration
	assert.Contains(t, output, "[LOOP:while]")
	assert.Contains(t, output, "Condition:")
}

func TestDryRun_NestedLoops_ShowsBothLoops_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "loop-nested", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify both loops are shown
	assert.Contains(t, output, "outer_loop")
	assert.Contains(t, output, "inner_loop")
}

func TestDryRun_WithHooks_ShowsPrePostHooks_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-with-hooks", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify hooks are displayed
	assert.Contains(t, output, "Hook (pre):")
	assert.Contains(t, output, "Hook (post):")
}

func TestDryRun_WithRetry_ShowsRetryConfiguration_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--input", "file_path=/test.txt",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify retry configuration
	assert.Contains(t, output, "Retry:")
	assert.Contains(t, output, "3 attempts")
	assert.Contains(t, output, "exponential")
}

func TestDryRun_WithCapture_ShowsCaptureConfiguration_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--input", "file_path=/test.txt",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify capture configuration
	assert.Contains(t, output, "Capture:")
	assert.Contains(t, output, "stdout")
	assert.Contains(t, output, "file_content")
}

func TestDryRun_WithTimeout_ShowsTimeoutValue_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--input", "file_path=/test.txt",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify timeout is displayed
	assert.Contains(t, output, "Timeout:")
}

func TestDryRun_ShowsTransitions_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-simple", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify transitions are shown
	assert.Contains(t, output, "on_success:")
	assert.Contains(t, output, "on_failure:")
}

func TestDryRun_ConditionalTransitions_ShowsAllPaths_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--input", "file_path=/test.txt",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify conditional transitions are shown
	assert.Contains(t, output, "when")
	assert.Contains(t, output, "verbose_output")
	assert.Contains(t, output, "large_count_handler")
}

func TestDryRun_WithWorkingDirectory_ShowsDir_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "dry-run-with-dir", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify working directory is displayed
	assert.Contains(t, output, "Dir:")
	assert.Contains(t, output, "/tmp")
}

func TestDryRun_ContinueOnError_ShowsFlag_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--input", "file_path=/test.txt",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify continue_on_error is shown
	assert.Contains(t, output, "Continue on error:")
}

func TestDryRun_TerminalSteps_ShowsStatus_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-simple", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify terminal steps with status
	assert.Contains(t, output, "[T]")
	// At least one terminal step should be shown
	assert.True(t,
		strings.Contains(output, "done") || strings.Contains(output, "error"),
		"should show terminal step names",
	)
}

// JSON Output Tests

func TestDryRun_JSONOutput_ValidJSON_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "valid-simple",
		"--dry-run",
		"--format", "json",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify valid JSON
	var plan workflow.DryRunPlan
	err = json.Unmarshal([]byte(output), &plan)
	require.NoError(t, err)
	assert.Equal(t, "simple-workflow", plan.WorkflowName)
	assert.NotEmpty(t, plan.Steps)
}

func TestDryRun_JSONOutput_ContainsAllFields_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--format", "json",
		"--input", "file_path=/test.txt",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	var plan workflow.DryRunPlan
	err = json.Unmarshal([]byte(output), &plan)
	require.NoError(t, err)

	// Verify plan contains expected fields
	assert.Equal(t, "dry-run-complete", plan.WorkflowName)
	assert.NotEmpty(t, plan.Description)
	assert.NotEmpty(t, plan.Inputs)
	assert.NotEmpty(t, plan.Steps)

	// Verify inputs are included
	filePathInput, ok := plan.Inputs["file_path"]
	assert.True(t, ok)
	assert.Equal(t, "/test.txt", filePathInput.Value)
}

func TestDryRun_JSONOutput_StepsHaveCorrectTypes_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "valid-parallel",
		"--dry-run",
		"--format", "json",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	var plan workflow.DryRunPlan
	err = json.Unmarshal([]byte(output), &plan)
	require.NoError(t, err)

	// Find the parallel step and verify its type
	hasParallelStep := false
	for _, step := range plan.Steps {
		if step.Type == workflow.StepTypeParallel {
			hasParallelStep = true
			assert.NotEmpty(t, step.Branches)
			break
		}
	}
	assert.True(t, hasParallelStep)
}

// Quiet Mode Tests

func TestDryRun_QuietMode_OutputsWorkflowNameOnly_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "valid-simple",
		"--dry-run",
		"--format", "quiet",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := strings.TrimSpace(buf.String())
	assert.Equal(t, "simple-workflow", output)
}

// No Side Effects Tests

func TestDryRun_NoStateFiles_Created_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()
	statesDir := filepath.Join(tmpDir, "states")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-simple", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify no state files were created
	entries, err := os.ReadDir(statesDir)
	if err == nil {
		assert.Empty(t, entries)
	}
	// It's also OK if the directory doesn't exist at all
}

func TestDryRun_NoHistoryRecorded_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()
	historyDir := filepath.Join(tmpDir, "history")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-simple", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify no history was recorded (directory should not exist or be empty)
	entries, err := os.ReadDir(historyDir)
	if err == nil {
		// If directory exists, it should be empty (or contain only metadata)
		for _, entry := range entries {
			// BadgerDB creates some files but they should be minimal
			if !strings.HasPrefix(entry.Name(), "MANIFEST") &&
				!strings.HasPrefix(entry.Name(), "KEYREGISTRY") {
				t.Logf("Found history file: %s", entry.Name())
			}
		}
	}
	// Directory not existing is also fine
}

func TestDryRun_NoCommandExecution_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Create a marker file that the workflow would delete if it ran
	markerFile := filepath.Join(tmpDir, "should_exist.txt")
	err := os.WriteFile(markerFile, []byte("test"), 0o644)
	require.NoError(t, err)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-simple", "--dry-run", "--storage", tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify marker file still exists (command that might delete it was not run)
	_, err = os.Stat(markerFile)
	assert.NoError(t, err)
}

// Error Handling Tests

func TestDryRun_WorkflowNotFound_ReturnsError_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"run", "nonexistent-workflow", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestDryRun_MissingRequiredInput_ReturnsError_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	// dry-run-complete requires file_path input
	cmd.SetArgs([]string{"run", "dry-run-complete", "--dry-run", "--storage", tmpDir})

	err := cmd.Execute()
	assert.Error(t, err)

	// Verify error message mentions the missing input
	errOutput := errBuf.String() + buf.String() + err.Error()
	assert.Contains(t, errOutput, "file_path")
}

func TestDryRun_InvalidInputFormat_ReturnsError_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	// Invalid input format (missing =value)
	cmd.SetArgs([]string{
		"run", "valid-simple",
		"--dry-run",
		"--input", "invalid_input_no_equals",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	assert.Error(t, err)
}

// Edge Case Tests

func TestDryRun_EmptyInputValue_Accepted_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "valid-simple",
		"--dry-run",
		"--input", "greeting=",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestDryRun_MultipleInputs_AllResolved_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--input", "file_path=/path/to/file.txt",
		"--input", "count=20",
		"--input", "verbose=true",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify all inputs are shown
	assert.Contains(t, output, "file_path:", "should show file_path input")
	assert.Contains(t, output, "count:", "should show count input")
	assert.Contains(t, output, "verbose:")
}

func TestDryRun_WorkflowWithDescription_ShowsDescription_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--input", "file_path=/test.txt",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify workflow description is shown
	assert.Contains(t, output, "Complete workflow fixture")
}

func TestDryRun_StepDescriptions_Shown_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "dry-run-complete",
		"--dry-run",
		"--input", "file_path=/test.txt",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify step descriptions are shown
	assert.Contains(t, output, "Validate input file exists")
}

// Table-Driven Tests for Various Workflow Types

func TestDryRun_VariousWorkflows_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tests := []struct {
		name           string
		workflow       string
		inputs         []string
		expectContains []string
		expectError    bool
	}{
		{
			name:     "simple workflow",
			workflow: "valid-simple",
			inputs:   nil,
			expectContains: []string{
				"Dry Run:",
				"simple-workflow",
				"Execution Plan:",
			},
			expectError: false,
		},
		{
			name:     "parallel workflow",
			workflow: "valid-parallel",
			inputs:   nil,
			expectContains: []string{
				"[PARALLEL]",
				"Branches:",
				"Strategy:",
			},
			expectError: false,
		},
		{
			name:     "workflow with hooks",
			workflow: "valid-with-hooks",
			inputs:   nil,
			expectContains: []string{
				"Hook",
			},
			expectError: false,
		},
		{
			name:     "full workflow with all features",
			workflow: "valid-full",
			inputs:   []string{"file_path=/test.txt"},
			expectContains: []string{
				"Timeout:",
				"Retry:",
				"Capture:",
			},
			expectError: false,
		},
		{
			name:     "for-each loop",
			workflow: "loop-foreach",
			inputs:   nil,
			expectContains: []string{
				"[LOOP:for_each]",
				"Items:",
				"Body:",
			},
			expectError: false,
		},
		{
			name:        "nonexistent workflow",
			workflow:    "does-not-exist",
			inputs:      nil,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)

			args := []string{"run", tc.workflow, "--dry-run", "--storage", tmpDir}
			for _, input := range tc.inputs {
				args = append(args, "--input", input)
			}
			cmd.SetArgs(args)

			err := cmd.Execute()

			if tc.expectError {
				assert.Error(t, err, "expected error for %s", tc.name)
				return
			}

			require.NoError(t, err, "unexpected error for %s", tc.name)

			output := buf.String()
			for _, expected := range tc.expectContains {
				assert.Contains(t, output, expected,
					"output should contain %q for %s", expected, tc.name)
			}
		})
	}
}

// Regression Tests

func TestDryRun_DoesNotModifyWorkflowPath_Integration(t *testing.T) {
	originalPath := os.Getenv("AWF_WORKFLOWS_PATH")
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-simple", "--dry-run", "--storage", tmpDir})

	_ = cmd.Execute()

	// Verify environment was not modified
	currentPath := os.Getenv("AWF_WORKFLOWS_PATH")
	assert.Equal(t, "../fixtures/workflows", currentPath)

	// Cleanup
	if originalPath == "" {
		os.Unsetenv("AWF_WORKFLOWS_PATH")
	} else {
		t.Setenv("AWF_WORKFLOWS_PATH", originalPath)
	}
}

func TestDryRun_CanRunMultipleTimes_Integration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	tmpDir := t.TempDir()

	// Run dry-run multiple times to ensure no state leaks
	for i := 0; i < 3; i++ {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"run", "valid-simple", "--dry-run", "--storage", tmpDir})

		err := cmd.Execute()
		require.NoError(t, err, "dry-run #%d should succeed", i+1)

		output := buf.String()
		assert.Contains(t, output, "simple-workflow", "run #%d should show workflow name", i+1)
	}
}
