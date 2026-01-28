//go:build integration

// Component: T003
// Feature: C028
package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
	"github.com/vanoix/awf/internal/testutil"
)

// setupValidateTest creates a test directory with workflows and sets AWF_WORKFLOWS_PATH
func setupValidateTest(t *testing.T, workflows map[string]string) string {
	t.Helper()
	dir := testutil.SetupWorkflowsDir(t, workflows)
	t.Setenv("AWF_WORKFLOWS_PATH", filepath.Join(dir, ".awf/workflows"))
	return dir
}

// TestRunValidate_TextFormat_Success tests valid workflow with text output format
func TestRunValidate_TextFormat_Success(t *testing.T) {
	// Arrange: create test directory with valid workflow
	setupValidateTest(t, map[string]string{
		"test.yaml": testutil.SimpleWorkflowYAML,
	})

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "test"})

	// Act: execute validate command
	err := cmd.Execute()

	// Assert: command succeeds and outputs success message
	require.NoError(t, err, "validate should succeed for valid workflow")
	output := buf.String()
	assert.Contains(t, output, "Workflow 'test' is valid", "should contain success message")
}

// TestRunValidate_JSONFormat_Success tests valid workflow with JSON output format
func TestRunValidate_JSONFormat_Success(t *testing.T) {
	// Arrange: create test directory with valid workflow
	setupValidateTest(t, map[string]string{
		"test.yaml": testutil.SimpleWorkflowYAML,
	})

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "test", "--format", "json"})

	// Act: execute validate command
	err := cmd.Execute()

	// Assert: command succeeds and outputs valid JSON
	require.NoError(t, err, "validate should succeed for valid workflow")
	output := buf.String()

	// Parse JSON to ensure it's valid
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON")

	// Verify JSON structure
	assert.Equal(t, true, result["valid"], "JSON should indicate workflow is valid")
	assert.Equal(t, "test", result["workflow"], "JSON should contain workflow name")
	assert.Nil(t, result["errors"], "JSON should not contain errors for valid workflow")
}

// TestRunValidate_QuietFormat_Success tests valid workflow with quiet output format
func TestRunValidate_QuietFormat_Success(t *testing.T) {
	// Arrange: create test directory with valid workflow
	setupValidateTest(t, map[string]string{
		"test.yaml": testutil.SimpleWorkflowYAML,
	})

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "test", "--format", "quiet"})

	// Act: execute validate command
	err := cmd.Execute()

	// Assert: command succeeds
	require.NoError(t, err, "validate should succeed for valid workflow")
	output := strings.TrimSpace(buf.String())

	// Quiet format outputs just "valid" or "invalid" (see output.go:306-311)
	assert.Equal(t, "valid", output, "quiet format should output 'valid'")
}

// TestRunValidate_TableFormat_Success tests valid workflow with table output format
func TestRunValidate_TableFormat_Success(t *testing.T) {
	// Arrange: create test directory with valid workflow
	setupValidateTest(t, map[string]string{
		"test-full.yaml": testutil.FullWorkflowYAML,
	})

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "test-full", "--format", "table"})

	// Act: execute validate command
	err := cmd.Execute()

	// Assert: command succeeds and outputs table format
	require.NoError(t, err, "validate should succeed for valid workflow")
	output := buf.String()

	// Table format should contain workflow details
	assert.Contains(t, output, "test-full", "table should contain workflow name")
	assert.Contains(t, output, "valid", "table should show valid status")
	// FullWorkflowYAML has inputs, verify they're shown
	assert.Contains(t, output, "var1", "table should show input names")
}

// TestRunValidate_WorkflowNotFound_TextError tests missing workflow with text error output
func TestRunValidate_WorkflowNotFound_TextError(t *testing.T) {
	// Arrange: create empty test directory
	dir := testutil.SetupTestDir(t)
	t.Setenv("AWF_WORKFLOWS_PATH", filepath.Join(dir, ".awf/workflows"))

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "nonexistent"})

	// Act: execute validate command
	err := cmd.Execute()

	// Assert: command fails with appropriate error
	require.Error(t, err, "validate should fail for nonexistent workflow")
	assert.Contains(t, err.Error(), "not found", "error should indicate workflow not found")
}

// TestRunValidate_WorkflowNotFound_JSONError tests missing workflow with JSON error output
func TestRunValidate_WorkflowNotFound_JSONError(t *testing.T) {
	// Arrange: create empty test directory
	dir := testutil.SetupTestDir(t)
	t.Setenv("AWF_WORKFLOWS_PATH", filepath.Join(dir, ".awf/workflows"))

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "nonexistent", "--format", "json"})

	// Act: execute validate command
	err := cmd.Execute()

	// Assert: Looking at validate.go:56-59, JSON format returns writer.WriteError() which may not fail
	// The output should contain error information regardless
	output := buf.String()

	// JSON error output should be parseable
	if output != "" {
		var result map[string]interface{}
		jsonErr := json.Unmarshal([]byte(output), &result)
		require.NoError(t, jsonErr, "JSON error output should be valid JSON")

		// Verify error is communicated (either via 'error' field or error message)
		hasError := result["error"] != nil || err != nil
		assert.True(t, hasError, "should indicate an error occurred")
	} else {
		// If no output, the command should have failed
		require.Error(t, err, "validate should fail for nonexistent workflow")
	}
}

// TestRunValidate_InvalidWorkflow_AllFormats tests validation errors across all output formats
func TestRunValidate_InvalidWorkflow_AllFormats(t *testing.T) {
	// Test table: different formats handle invalid workflows differently
	tests := []struct {
		name         string
		format       string
		expectJSON   bool
		expectError  bool
		errorPattern string
	}{
		{
			name:         "text format shows error",
			format:       "text",
			expectJSON:   false,
			expectError:  true,
			errorPattern: "nonexistent",
		},
		{
			name:         "json format outputs error JSON",
			format:       "json",
			expectJSON:   true,
			expectError:  true,
			errorPattern: "nonexistent",
		},
		{
			name:         "table format shows error",
			format:       "table",
			expectJSON:   false,
			expectError:  true,
			errorPattern: "nonexistent",
		},
		{
			name:         "quiet format outputs invalid text",
			format:       "quiet",
			expectJSON:   false,
			expectError:  true,
			errorPattern: "", // quiet just outputs "invalid"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create test directory with invalid workflow
			setupValidateTest(t, map[string]string{
				"bad.yaml": testutil.BadWorkflowYAML,
			})

			cmd := cli.NewRootCommand()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			args := []string{"validate", "bad"}
			if tt.format != "text" {
				args = append(args, "--format", tt.format)
			}
			cmd.SetArgs(args)

			// Act: execute validate command
			err := cmd.Execute()

			// Assert: command behavior
			output := buf.String()

			// Verify output format
			if tt.expectJSON && output != "" {
				var result map[string]interface{}
				jsonErr := json.Unmarshal([]byte(output), &result)
				require.NoError(t, jsonErr, "output should be valid JSON, got: %s", output)

				// JSON error format varies depending on where the error occurs:
				// 1. Early errors (workflow not found/load fails) use WriteError: {code, error}
				// 2. Validation errors use WriteValidation: {valid, workflow, errors}

				if _, hasError := result["error"]; hasError {
					// WriteError format - verify error field exists
					assert.NotEmpty(t, result["error"], "JSON should contain error message")
				} else {
					// WriteValidation format - verify valid=false and errors exist
					valid, ok := result["valid"].(bool)
					require.True(t, ok, "JSON should have 'valid' boolean field, got: %v", result)
					assert.False(t, valid, "JSON should indicate workflow is invalid")

					errors, ok := result["errors"].([]interface{})
					require.True(t, ok, "JSON should have 'errors' array field, got: %v", result)
					assert.NotEmpty(t, errors, "JSON should contain validation errors")
				}
			}

			// For non-JSON formats or if command returned error, verify error occurred
			if !tt.expectJSON || output == "" {
				if tt.expectError {
					require.Error(t, err, "validate should fail for invalid workflow")
				}
			}

			// Verify error message contains expected pattern
			if tt.errorPattern != "" && err != nil {
				combinedOutput := output + " " + err.Error()
				assert.Contains(t, combinedOutput, tt.errorPattern,
					"output should contain error pattern: %s", tt.errorPattern)
			}
		})
	}
}

// TestRunValidate_TemplateRefValidation tests the template reference validation path
func TestRunValidate_TemplateRefValidation(t *testing.T) {
	// This test verifies that workflows with template references are validated
	// Template validation happens after basic structure validation

	// Arrange: create workflow with template reference (but no template exists)
	workflowWithTemplate := `name: with-template
states:
  initial: start
  start:
    type: step
    template_ref:
      name: nonexistent-template
    on_success: done
  done:
    type: terminal
`
	setupValidateTest(t, map[string]string{
		"template-test.yaml": workflowWithTemplate,
	})

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "template-test"})

	// Act: execute validate command
	err := cmd.Execute()

	// Assert: validation should fail due to missing template
	require.Error(t, err, "validate should fail when template reference is invalid")

	// Error message should mention the template issue
	errMsg := err.Error()
	assert.True(t,
		strings.Contains(errMsg, "template") || strings.Contains(errMsg, "step"),
		"error should mention template or step validation failure")
}

// TestRunValidate_VerboseOutput tests verbose flag produces additional output
func TestRunValidate_VerboseOutput(t *testing.T) {
	// Arrange: create test directory with full workflow (has description, version)
	setupValidateTest(t, map[string]string{
		"test-full.yaml": testutil.FullWorkflowYAML,
	})

	// Test without verbose flag
	t.Run("without verbose", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := &bytes.Buffer{}
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"validate", "test-full"})

		err := cmd.Execute()
		require.NoError(t, err, "validate should succeed")

		output := buf.String()
		// Without verbose, should just show success message
		assert.Contains(t, output, "is valid", "should contain success message")

		// Should NOT contain detailed workflow info
		assert.NotContains(t, output, "Version:", "should not show version without verbose")
		assert.NotContains(t, output, "Description:", "should not show description without verbose")
	})

	// Test with verbose flag
	t.Run("with verbose", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := &bytes.Buffer{}
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"validate", "test-full", "--verbose"})

		err := cmd.Execute()
		require.NoError(t, err, "validate should succeed")

		output := buf.String()
		// With verbose, should show detailed workflow info
		assert.Contains(t, output, "is valid", "should contain success message")
		assert.Contains(t, output, "Version:", "verbose should show version")
		assert.Contains(t, output, "Description:", "verbose should show description")
		assert.Contains(t, output, "Initial:", "verbose should show initial state")
		assert.Contains(t, output, "Steps:", "verbose should show step count")
		assert.Contains(t, output, "Inputs:", "verbose should show inputs section")
	})
}
