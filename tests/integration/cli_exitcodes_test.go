//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// CLI Exit Code Integration Tests (C011 - US4)
// Tests validate CLI exit codes match error taxonomy:
// - Exit code 1 for user errors (bad input, missing file, validation failure)
// - Exit code 2 for workflow errors (invalid state reference, cycle)
// - Exit code 3 for execution errors (command failed, timeout)
// - Exit code 4 for system errors (IO, permissions)
// - Exit code 0 for successful completion
//
// Implementation Note (ADR-002):
// Tests spawn actual `awf run` subprocess via exec.Command() to verify
// CLI-level exit code mapping (internal/interfaces/cli/run.go:82-102).
// This provides high-fidelity validation of user-facing behavior.
// =============================================================================

// getBinaryPath returns the path to the awf binary, building it if necessary
func getBinaryPath(t *testing.T) string {
	t.Helper()

	binaryPath := filepath.Join("../../bin/awf")

	// Build binary if it doesn't exist
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Log("Building awf binary...")
		cmd := exec.Command("make", "build")
		cmd.Dir = "../.."
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "make build failed: %s", string(output))
	}

	absPath, err := filepath.Abs(binaryPath)
	require.NoError(t, err, "failed to get absolute path")

	return absPath
}

// TestCLI_ExitCode0_Success_Integration verifies exit code 0 for successful workflows
// Feature: C011 - Task T011
// Strategy: Run valid workflow via subprocess, verify exit code 0
func TestCLI_ExitCode0_Success_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		inputs       []string // CLI input flags
		wantExitCode int
	}{
		{
			name:         "successful workflow returns exit code 0",
			workflowFile: "valid-simple.yaml",
			inputs:       nil, // No inputs required
			wantExitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Get binary and fixture paths
			binaryPath := getBinaryPath(t)
			fixtureDir, err := filepath.Abs("../fixtures/workflows")
			require.NoError(t, err)

			// Act: Spawn awf run subprocess
			args := []string{"run", tt.workflowFile}
			args = append(args, tt.inputs...)

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			output, err := cmd.CombinedOutput()

			// Assert: Exit code verification
			if tt.wantExitCode == 0 {
				require.NoError(t, err, "workflow should succeed: %s", string(output))
			} else {
				require.Error(t, err, "workflow should fail")
				if exitErr, ok := err.(*exec.ExitError); ok {
					assert.Equal(t, tt.wantExitCode, exitErr.ExitCode(), "exit code mismatch")
				} else {
					t.Fatalf("expected *exec.ExitError, got %T", err)
				}
			}
		})
	}
}

// TestCLI_ExitCode1_UserError_Integration verifies exit code 1 for user errors
// Feature: C011 - Task T011
// Strategy: Trigger user errors (missing input, validation failure), verify exit code 1
func TestCLI_ExitCode1_UserError_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		inputs       []string
		wantExitCode int
		wantStderr   string // Expected error message fragment
	}{
		{
			name:         "missing required input returns exit code 1",
			workflowFile: "exit-user-error.yaml",
			inputs:       nil, // Missing required input
			wantExitCode: 1,
			wantStderr:   "required", // Error about required input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Get binary and fixture paths
			binaryPath := getBinaryPath(t)
			fixtureDir, err := filepath.Abs("../fixtures/workflows")
			require.NoError(t, err)

			// Act: Spawn subprocess with missing input
			args := []string{"run", tt.workflowFile}
			args = append(args, tt.inputs...)

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Assert - Layer 1: Command should fail
			require.Error(t, err, "command should fail for user error")

			// Assert - Layer 2: Exit code should be 1
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "expected *exec.ExitError, got %T", err)
			assert.Equal(t, tt.wantExitCode, exitErr.ExitCode(), "should return exit code 1 for user error")

			// Assert - Layer 3: Error message should indicate user error
			assert.Contains(t, strings.ToLower(outputStr), tt.wantStderr, "error message should mention %s", tt.wantStderr)
		})
	}
}

// TestCLI_ExitCode1_InvalidInputValue_Integration verifies exit code 1 for validation failures
// Feature: C011 - Task T011
// Strategy: Provide invalid input value (violates validation rule), verify exit code 1
func TestCLI_ExitCode1_InvalidInputValue_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		inputs       []string
		wantExitCode int
		wantStderr   string
	}{
		{
			name:         "validation failure returns exit code 1",
			workflowFile: "exit-user-error.yaml",
			inputs:       []string{"--email=invalid_email"}, // Invalid email format
			wantExitCode: 1,
			wantStderr:   "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Get binary and fixture paths
			binaryPath := getBinaryPath(t)
			fixtureDir, err := filepath.Abs("../fixtures/workflows")
			require.NoError(t, err)

			// Act: Spawn subprocess with invalid input
			args := []string{"run", tt.workflowFile}
			args = append(args, tt.inputs...)

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Assert - Layer 1: Command should fail
			require.Error(t, err, "command should fail for validation error")

			// Assert - Layer 2: Exit code should be 1
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "expected *exec.ExitError, got %T", err)
			assert.Equal(t, tt.wantExitCode, exitErr.ExitCode(), "should return exit code 1 for validation error")

			// Assert - Layer 3: Error message should indicate validation failure
			assert.Contains(t, strings.ToLower(outputStr), tt.wantStderr, "error message should mention validation")
		})
	}
}

// TestCLI_ExitCode2_InvalidStateReference_Integration verifies exit code 2 for workflow errors
// Feature: C011 - Task T011
// Strategy: Use workflow with invalid state reference, verify exit code 2
func TestCLI_ExitCode2_InvalidStateReference_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		inputs       []string
		wantExitCode int
		wantStderr   string
	}{
		{
			name:         "invalid state reference returns exit code 2",
			workflowFile: "exit-workflow-error.yaml",
			inputs:       nil,
			wantExitCode: 2,
			wantStderr:   "state", // Error about invalid state
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Get binary and fixture paths
			binaryPath := getBinaryPath(t)
			fixtureDir, err := filepath.Abs("../fixtures/workflows")
			require.NoError(t, err)

			// Act: Spawn subprocess with invalid workflow
			args := []string{"run", tt.workflowFile}
			args = append(args, tt.inputs...)

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Assert - Layer 1: Command should fail
			require.Error(t, err, "command should fail for workflow error")

			// Assert - Layer 2: Exit code should be 2
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "expected *exec.ExitError, got %T", err)
			assert.Equal(t, tt.wantExitCode, exitErr.ExitCode(), "should return exit code 2 for workflow error")

			// Assert - Layer 3: Error message should indicate workflow problem
			assert.Contains(t, strings.ToLower(outputStr), tt.wantStderr, "error message should mention state issue")
		})
	}
}

// TestCLI_ExitCode2_CyclicStateMachine_Integration verifies exit code 2 for workflow cycles
// Feature: C011 - Task T011
// Strategy: Use workflow with cycle, verify exit code 2
func TestCLI_ExitCode2_CyclicStateMachine_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		inputs       []string
		wantExitCode int
		wantStderr   string
	}{
		{
			name:         "cyclic state machine returns exit code 2",
			workflowFile: "exit-workflow-error.yaml", // Fixture with cycle
			inputs:       []string{"--trigger-cycle=true"},
			wantExitCode: 2,
			wantStderr:   "cycle", // Error about cycle detection
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Get binary and fixture paths
			binaryPath := getBinaryPath(t)
			fixtureDir, err := filepath.Abs("../fixtures/workflows")
			require.NoError(t, err)

			// Act: Spawn subprocess
			args := []string{"run", tt.workflowFile}
			args = append(args, tt.inputs...)

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Assert - Layer 1: Command should fail
			require.Error(t, err, "command should fail for cyclic workflow")

			// Assert - Layer 2: Exit code should be 2
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "expected *exec.ExitError, got %T", err)
			assert.Equal(t, tt.wantExitCode, exitErr.ExitCode(), "should return exit code 2 for cycle")

			// Assert - Layer 3: Error message should mention cycle
			assert.Contains(t, strings.ToLower(outputStr), tt.wantStderr, "error message should mention cycle")
		})
	}
}

// TestCLI_ExitCode3_CommandFailure_Integration verifies exit code 3 for execution errors
// Feature: C011 - Task T011
// Strategy: Run workflow with failing command, verify exit code 3
func TestCLI_ExitCode3_CommandFailure_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		inputs       []string
		wantExitCode int
		wantStderr   string
	}{
		{
			name:         "command failure returns exit code 3",
			workflowFile: "exit-execution-error.yaml",
			inputs:       nil,
			wantExitCode: 3,
			wantStderr:   "exit", // Error about command exit code
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Get binary and fixture paths
			binaryPath := getBinaryPath(t)
			fixtureDir, err := filepath.Abs("../fixtures/workflows")
			require.NoError(t, err)

			// Act: Spawn subprocess with failing command
			args := []string{"run", tt.workflowFile}
			args = append(args, tt.inputs...)

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Assert - Layer 1: Command should fail
			require.Error(t, err, "command should fail for execution error")

			// Assert - Layer 2: Exit code should be 3
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "expected *exec.ExitError, got %T", err)
			assert.Equal(t, tt.wantExitCode, exitErr.ExitCode(), "should return exit code 3 for execution error")

			// Assert - Layer 3: Error message should indicate execution failure
			assert.Contains(t, strings.ToLower(outputStr), tt.wantStderr, "error message should mention command failure")
		})
	}
}

// TestCLI_ExitCode3_Timeout_Integration verifies exit code 3 for workflow timeout
// Feature: C011 - Task T011
// Strategy: Run workflow that times out, verify exit code 3
func TestCLI_ExitCode3_Timeout_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		inputs       []string
		wantExitCode int
		wantStderr   string
	}{
		{
			name:         "workflow timeout returns exit code 3",
			workflowFile: "exit-execution-error.yaml",
			inputs:       []string{"--timeout=1s"}, // Short timeout
			wantExitCode: 3,
			wantStderr:   "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Get binary and fixture paths
			binaryPath := getBinaryPath(t)
			fixtureDir, err := filepath.Abs("../fixtures/workflows")
			require.NoError(t, err)

			// Act: Spawn subprocess with timeout
			args := []string{"run", tt.workflowFile}
			args = append(args, tt.inputs...)

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Assert - Layer 1: Command should fail
			require.Error(t, err, "command should fail for timeout")

			// Assert - Layer 2: Exit code should be 3
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "expected *exec.ExitError, got %T", err)
			assert.Equal(t, tt.wantExitCode, exitErr.ExitCode(), "should return exit code 3 for timeout")

			// Assert - Layer 3: Error message should mention timeout
			assert.Contains(t, strings.ToLower(outputStr), tt.wantStderr, "error message should mention timeout")
		})
	}
}

// TestCLI_ExitCode4_FileNotFound_Integration verifies exit code 4 for system errors (missing file)
// Feature: C011 - Task T011
// Strategy: Try to run non-existent workflow file, verify exit code 4
func TestCLI_ExitCode4_FileNotFound_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		inputs       []string
		wantExitCode int
		wantStderr   string
	}{
		{
			name:         "missing workflow file returns exit code 4",
			workflowFile: "non-existent-workflow.yaml",
			inputs:       nil,
			wantExitCode: 4,
			wantStderr:   "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Get binary and fixture paths
			binaryPath := getBinaryPath(t)
			fixtureDir, err := filepath.Abs("../fixtures/workflows")
			require.NoError(t, err)

			// Act: Spawn subprocess with non-existent file
			args := []string{"run", tt.workflowFile}
			args = append(args, tt.inputs...)

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Assert - Layer 1: Command should fail
			require.Error(t, err, "command should fail for missing file")

			// Assert - Layer 2: Exit code should be 4
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "expected *exec.ExitError, got %T", err)
			assert.Equal(t, tt.wantExitCode, exitErr.ExitCode(), "should return exit code 4 for file not found")

			// Assert - Layer 3: Error message should indicate file problem
			assert.Contains(t, strings.ToLower(outputStr), tt.wantStderr, "error message should mention file not found")
		})
	}
}

// TestCLI_ExitCode4_PermissionDenied_Integration verifies exit code 4 for permission errors
// Feature: C011 - Task T011
// Strategy: Create unreadable workflow file, verify exit code 4
func TestCLI_ExitCode4_PermissionDenied_Integration(t *testing.T) {

	// Skip on systems where we can't test permissions reliably
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	tests := []struct {
		name         string
		workflowYAML string // Inline YAML to create unreadable file
		wantExitCode int
		wantStderr   string
	}{
		{
			name: "unreadable workflow file returns exit code 4",
			workflowYAML: `name: test
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`,
			wantExitCode: 4,
			wantStderr:   "permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Create unreadable workflow file
			binaryPath := getBinaryPath(t)
			tmpDir := t.TempDir()
			workflowPath := filepath.Join(tmpDir, "unreadable.yaml")

			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))
			require.NoError(t, os.Chmod(workflowPath, 0o000)) // Make unreadable

			// Clean up: restore permissions on cleanup
			t.Cleanup(func() {
				_ = os.Chmod(workflowPath, 0o644)
			})

			// Act: Spawn subprocess with unreadable file
			cmd := exec.Command(binaryPath, "run", "unreadable.yaml")
			cmd.Dir = tmpDir

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Assert - Layer 1: Command should fail
			require.Error(t, err, "command should fail for permission denied")

			// Assert - Layer 2: Exit code should be 4
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "expected *exec.ExitError, got %T", err)
			assert.Equal(t, tt.wantExitCode, exitErr.ExitCode(), "should return exit code 4 for permission denied")

			// Assert - Layer 3: Error message should indicate permission problem
			assert.Contains(t, strings.ToLower(outputStr), tt.wantStderr, "error message should mention permission")
		})
	}
}

// TestCLI_ExitCodeMapping_ComprehensiveScenarios_Integration tests all exit code scenarios
// Feature: C011 - Task T011 (Comprehensive edge cases)
// Strategy: Table-driven test covering all exit codes with various scenarios
func TestCLI_ExitCodeMapping_ComprehensiveScenarios_Integration(t *testing.T) {

	tests := []struct {
		name         string
		workflowFile string
		inputs       []string
		setupFunc    func(t *testing.T, dir string) // Optional setup
		wantExitCode int
		wantStderr   string
	}{
		{
			name:         "exit 0: successful workflow completion",
			workflowFile: "valid-simple.yaml",
			inputs:       nil,
			wantExitCode: 0,
			wantStderr:   "",
		},
		{
			name:         "exit 1: missing required input",
			workflowFile: "exit-user-error.yaml",
			inputs:       nil,
			wantExitCode: 1,
			wantStderr:   "required",
		},
		{
			name:         "exit 2: invalid state reference",
			workflowFile: "exit-workflow-error.yaml",
			inputs:       nil,
			wantExitCode: 2,
			wantStderr:   "state",
		},
		{
			name:         "exit 3: command execution failure",
			workflowFile: "exit-execution-error.yaml",
			inputs:       nil,
			wantExitCode: 3,
			wantStderr:   "exit",
		},
		{
			name:         "exit 4: workflow file not found",
			workflowFile: "does-not-exist.yaml",
			inputs:       nil,
			wantExitCode: 4,
			wantStderr:   "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			binaryPath := getBinaryPath(t)
			fixtureDir, err := filepath.Abs("../fixtures/workflows")
			require.NoError(t, err)

			if tt.setupFunc != nil {
				tt.setupFunc(t, fixtureDir)
			}

			// Act
			args := []string{"run", tt.workflowFile}
			args = append(args, tt.inputs...)

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "AWF_WORKFLOWS_PATH="+fixtureDir)

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Assert
			if tt.wantExitCode == 0 {
				require.NoError(t, err, "workflow should succeed: %s", outputStr)
			} else {
				require.Error(t, err, "workflow should fail")
				exitErr, ok := err.(*exec.ExitError)
				require.True(t, ok, "expected *exec.ExitError, got %T", err)
				assert.Equal(t, tt.wantExitCode, exitErr.ExitCode(), "exit code mismatch for scenario: %s", tt.name)

				if tt.wantStderr != "" {
					assert.Contains(t, strings.ToLower(outputStr), tt.wantStderr, "error message should contain: %s", tt.wantStderr)
				}
			}
		})
	}
}
