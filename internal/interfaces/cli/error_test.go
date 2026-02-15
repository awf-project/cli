package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/domain/errors"
	"github.com/awf-project/awf/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorCommand_Exists verifies that the error command is registered in the root command.
func TestErrorCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "error" {
			found = true
			break
		}
	}

	assert.True(t, found, "expected root command to have 'error' subcommand")
}

// TestErrorCommand_Usage verifies the error command has correct usage metadata.
func TestErrorCommand_Usage(t *testing.T) {
	cmd := cli.NewRootCommand()
	errorCmd, _, err := cmd.Find([]string{"error"})
	require.NoError(t, err)

	assert.Equal(t, "error", errorCmd.Name())
	assert.Equal(t, "error [code]", errorCmd.Use)
	assert.Contains(t, errorCmd.Short, "Look up error code")
	assert.NotEmpty(t, errorCmd.Long)
}

// TestErrorCommand_AcceptsOptionalArgument verifies the command accepts 0 or 1 arguments.
func TestErrorCommand_AcceptsOptionalArgument(t *testing.T) {
	cmd := cli.NewRootCommand()
	errorCmd, _, err := cmd.Find([]string{"error"})
	require.NoError(t, err)

	// 0 args should be valid (list all codes)
	err = errorCmd.Args(errorCmd, []string{})
	assert.NoError(t, err)

	// 1 arg should be valid (lookup specific code)
	err = errorCmd.Args(errorCmd, []string{"USER.INPUT.MISSING_FILE"})
	assert.NoError(t, err)
}

// TestErrorCommand_RejectsTooManyArguments verifies the command rejects more than 1 argument.
func TestErrorCommand_RejectsTooManyArguments(t *testing.T) {
	cmd := cli.NewRootCommand()
	errorCmd, _, err := cmd.Find([]string{"error"})
	require.NoError(t, err)

	// 2 args should be invalid
	err = errorCmd.Args(errorCmd, []string{"USER.INPUT.MISSING_FILE", "extra"})
	assert.Error(t, err)
}

// TestErrorCommand_ListAll tests listing all error codes when no argument is provided.
func TestErrorCommand_ListAll(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedInText []string
		minCodeCount   int
	}{
		{
			name: "no arguments lists all codes",
			args: []string{"error"},
			expectedInText: []string{
				"USER.INPUT.MISSING_FILE",
				"USER.INPUT.INVALID_FORMAT",
				"USER.INPUT.VALIDATION_FAILED",
				"WORKFLOW.PARSE.YAML_SYNTAX",
				"WORKFLOW.PARSE.UNKNOWN_FIELD",
				"WORKFLOW.VALIDATION.CYCLE_DETECTED",
				"WORKFLOW.VALIDATION.MISSING_STATE",
				"WORKFLOW.VALIDATION.INVALID_TRANSITION",
				"EXECUTION.COMMAND.FAILED",
				"EXECUTION.COMMAND.TIMEOUT",
				"EXECUTION.PARALLEL.PARTIAL_FAILURE",
				"SYSTEM.IO.READ_FAILED",
				"SYSTEM.IO.WRITE_FAILED",
				"SYSTEM.IO.PERMISSION_DENIED",
			},
			minCodeCount: 14, // Catalog has 14 entries
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			require.NoError(t, err)

			output := buf.String()

			// Verify all expected codes are present
			for _, expected := range tt.expectedInText {
				assert.Contains(t, output, expected,
					"expected output to contain error code %s", expected)
			}

			// Verify count (each code should appear at least once)
			for _, expected := range tt.expectedInText {
				count := strings.Count(output, expected)
				assert.GreaterOrEqual(t, count, 1,
					"expected code %s to appear at least once", expected)
			}
		})
	}
}

// TestErrorCommand_ListAll_JSON tests listing all error codes in JSON format.
func TestErrorCommand_ListAll_JSON(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"error", "--format", "json"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Parse JSON output
	var result interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON")

	// JSON should be an array of entries
	entries, ok := result.([]interface{})
	require.True(t, ok, "JSON output should be an array")
	assert.GreaterOrEqual(t, len(entries), 14, "should have at least 14 error codes")

	// Verify first entry has required fields
	if len(entries) > 0 {
		entry, ok := entries[0].(map[string]interface{})
		require.True(t, ok, "entry should be an object")
		assert.Contains(t, entry, "code")
		assert.Contains(t, entry, "description")
		assert.Contains(t, entry, "resolution")
		// related_codes is optional
	}
}

// TestErrorCommand_LookupValidCode tests looking up a specific valid error code.
func TestErrorCommand_LookupValidCode(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		description string
		resolution  string
	}{
		{
			name:        "USER.INPUT.MISSING_FILE",
			code:        "USER.INPUT.MISSING_FILE",
			description: "file was not found",
			resolution:  "Verify the file path",
		},
		{
			name:        "WORKFLOW.VALIDATION.CYCLE_DETECTED",
			code:        "WORKFLOW.VALIDATION.CYCLE_DETECTED",
			description: "cycle was detected",
			resolution:  "Review state transitions",
		},
		{
			name:        "EXECUTION.COMMAND.FAILED",
			code:        "EXECUTION.COMMAND.FAILED",
			description: "shell command executed",
			resolution:  "Check command output",
		},
		{
			name:        "SYSTEM.IO.PERMISSION_DENIED",
			code:        "SYSTEM.IO.PERMISSION_DENIED",
			description: "Insufficient permissions",
			resolution:  "Check file permissions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"error", tt.code})

			err := cmd.Execute()
			require.NoError(t, err)

			output := buf.String()

			// Verify error code is in output
			assert.Contains(t, output, tt.code)

			// Verify description appears
			assert.Contains(t, output, tt.description)

			// Verify resolution appears
			assert.Contains(t, output, tt.resolution)

			// Verify catalog entry structure is shown
			entry, found := errors.GetCatalogEntry(errors.ErrorCode(tt.code))
			require.True(t, found)
			if len(entry.RelatedCodes) > 0 {
				// Should show related codes if they exist
				assert.Contains(t, output, string(entry.RelatedCodes[0]))
			}
		})
	}
}

// TestErrorCommand_LookupValidCode_JSON tests looking up a specific error code in JSON format.
func TestErrorCommand_LookupValidCode_JSON(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"error", "USER.INPUT.MISSING_FILE", "--format", "json"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Parse JSON output
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON")

	// Verify required fields
	assert.Equal(t, "USER.INPUT.MISSING_FILE", result["code"])
	assert.NotEmpty(t, result["description"])
	assert.NotEmpty(t, result["resolution"])

	// Verify related codes if present
	if relatedCodes, ok := result["related_codes"].([]interface{}); ok && len(relatedCodes) > 0 {
		// Should have at least one related code
		assert.NotEmpty(t, relatedCodes)
	}
}

// TestErrorCommand_LookupInvalidCode tests looking up a non-existent error code.
func TestErrorCommand_LookupInvalidCode(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		expectedMsg string
		expectError bool
	}{
		{
			name:        "completely invalid code",
			code:        "INVALID.CODE.THAT.DOES.NOT.EXIST",
			expectedMsg: "not found",
			expectError: true,
		},
		{
			name:        "wrong format",
			code:        "INVALIDFORMAT",
			expectedMsg: "not found",
			expectError: true,
		},
		{
			name:        "partial code",
			code:        "USER.INPUT",
			expectedMsg: "not found",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"error", tt.code})

			_ = cmd.Execute()

			if tt.expectError {
				// Command should indicate error not found
				// (implementation may exit with error or print message)
				output := buf.String()
				assert.Contains(t, strings.ToLower(output), strings.ToLower(tt.expectedMsg))
			}
		})
	}
}

// TestErrorCommand_PrefixMatch tests prefix matching for error codes.
func TestErrorCommand_PrefixMatch(t *testing.T) {
	tests := []struct {
		name          string
		prefix        string
		expectedCodes []string
		minMatchCount int
	}{
		{
			name:   "USER category",
			prefix: "USER",
			expectedCodes: []string{
				"USER.INPUT.MISSING_FILE",
				"USER.INPUT.INVALID_FORMAT",
				"USER.INPUT.VALIDATION_FAILED",
			},
			minMatchCount: 3,
		},
		{
			name:   "USER.INPUT subcategory",
			prefix: "USER.INPUT",
			expectedCodes: []string{
				"USER.INPUT.MISSING_FILE",
				"USER.INPUT.INVALID_FORMAT",
				"USER.INPUT.VALIDATION_FAILED",
			},
			minMatchCount: 3,
		},
		{
			name:   "WORKFLOW.VALIDATION subcategory",
			prefix: "WORKFLOW.VALIDATION",
			expectedCodes: []string{
				"WORKFLOW.VALIDATION.CYCLE_DETECTED",
				"WORKFLOW.VALIDATION.MISSING_STATE",
				"WORKFLOW.VALIDATION.INVALID_TRANSITION",
			},
			minMatchCount: 3,
		},
		{
			name:   "EXECUTION category",
			prefix: "EXECUTION",
			expectedCodes: []string{
				"EXECUTION.COMMAND.FAILED",
				"EXECUTION.COMMAND.TIMEOUT",
				"EXECUTION.PARALLEL.PARTIAL_FAILURE",
			},
			minMatchCount: 3,
		},
		{
			name:   "SYSTEM category",
			prefix: "SYSTEM",
			expectedCodes: []string{
				"SYSTEM.IO.READ_FAILED",
				"SYSTEM.IO.WRITE_FAILED",
				"SYSTEM.IO.PERMISSION_DENIED",
			},
			minMatchCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"error", tt.prefix})

			err := cmd.Execute()
			require.NoError(t, err)

			output := buf.String()

			// Verify all expected codes are present
			matchCount := 0
			for _, expected := range tt.expectedCodes {
				if strings.Contains(output, expected) {
					matchCount++
				}
			}

			assert.GreaterOrEqual(t, matchCount, tt.minMatchCount,
				"expected at least %d matching codes for prefix %s",
				tt.minMatchCount, tt.prefix)
		})
	}
}

// TestErrorCommand_EdgeCases tests edge cases and special scenarios.
func TestErrorCommand_EdgeCases(t *testing.T) {
	t.Run("empty string argument", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"error", ""})

		_ = cmd.Execute()
		// Should handle empty string gracefully
		// (either error or list all, depending on implementation)
		output := buf.String()
		assert.NotEmpty(t, output)
	})

	t.Run("case sensitivity", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"error", "user.input.missing_file"})

		_ = cmd.Execute()
		// Lowercase should not match (codes are uppercase)
		// Implementation should indicate not found
		output := buf.String()
		assert.NotEmpty(t, output)
	})

	t.Run("whitespace in code", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"error", " USER.INPUT.MISSING_FILE "})

		_ = cmd.Execute()
		// Should handle whitespace (trim or error)
		output := buf.String()
		assert.NotEmpty(t, output)
	})

	t.Run("special characters", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"error", "USER@INPUT#MISSING"})

		_ = cmd.Execute()
		// Should handle special characters gracefully
		output := buf.String()
		assert.NotEmpty(t, output)
	})
}

// TestErrorCommand_OutputFormats tests different output formats.
func TestErrorCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		name         string
		format       string
		args         []string
		validateFunc func(t *testing.T, output string)
	}{
		{
			name:   "text format (default)",
			format: "",
			args:   []string{"error", "USER.INPUT.MISSING_FILE"},
			validateFunc: func(t *testing.T, output string) {
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
				assert.Contains(t, output, "file was not found")
			},
		},
		{
			name:   "text format (explicit)",
			format: "text",
			args:   []string{"error", "USER.INPUT.MISSING_FILE", "--format", "text"},
			validateFunc: func(t *testing.T, output string) {
				assert.Contains(t, output, "USER.INPUT.MISSING_FILE")
			},
		},
		{
			name:   "json format",
			format: "json",
			args:   []string{"error", "USER.INPUT.MISSING_FILE", "--format", "json"},
			validateFunc: func(t *testing.T, output string) {
				var result map[string]interface{}
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)
				assert.Equal(t, "USER.INPUT.MISSING_FILE", result["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			require.NoError(t, err)

			output := buf.String()
			tt.validateFunc(t, output)
		})
	}
}

// TestErrorCommand_AllCatalogEntriesValid verifies all catalog entries have valid error codes.
func TestErrorCommand_AllCatalogEntriesValid(t *testing.T) {
	catalog := errors.ErrorCatalog

	for code, entry := range catalog {
		t.Run(string(code), func(t *testing.T) {
			// Verify code is valid format
			assert.True(t, code.IsValid(),
				"error code %s should be valid format", code)

			// Verify entry code matches map key
			assert.Equal(t, code, entry.Code,
				"catalog entry code should match map key")

			// Verify entry has non-empty description
			assert.NotEmpty(t, entry.Description,
				"error code %s should have description", code)

			// Verify entry has non-empty resolution
			assert.NotEmpty(t, entry.Resolution,
				"error code %s should have resolution", code)

			// Verify related codes are valid
			for _, relatedCode := range entry.RelatedCodes {
				assert.True(t, relatedCode.IsValid(),
					"related code %s should be valid", relatedCode)
			}
		})
	}
}

// TestErrorCommand_CobraIntegration tests Cobra-specific behaviors.
func TestErrorCommand_CobraIntegration(t *testing.T) {
	t.Run("help flag", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"error", "--help"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Look up error code")
		assert.Contains(t, output, "awf error")
		assert.Contains(t, output, "Examples:")
	})

	t.Run("command structure", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		errorCmd, _, err := cmd.Find([]string{"error"})
		require.NoError(t, err)

		// Verify command has RunE function
		assert.NotNil(t, errorCmd.RunE)

		// Verify command has proper args validator
		assert.NotNil(t, errorCmd.Args)
	})
}

// TestErrorCommand_ExitCodes tests that the command respects exit code conventions.
func TestErrorCommand_ExitCodes(t *testing.T) {
	t.Run("successful lookup returns exit 0", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"error", "USER.INPUT.MISSING_FILE"})

		err := cmd.Execute()
		// Successful lookup should not return error
		require.NoError(t, err)
	})

	t.Run("list all returns exit 0", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"error"})

		err := cmd.Execute()
		// Successful list should not return error
		require.NoError(t, err)
	})
}

// TestErrorCommand_RelatedCodes tests that related codes are properly displayed.
func TestErrorCommand_RelatedCodes(t *testing.T) {
	// Find a code with related codes
	code := errors.ErrorCodeUserInputMissingFile
	entry, found := errors.GetCatalogEntry(code)
	require.True(t, found)
	require.NotEmpty(t, entry.RelatedCodes, "test requires a code with related codes")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"error", string(code)})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify at least one related code is mentioned
	foundRelated := false
	for _, related := range entry.RelatedCodes {
		if strings.Contains(output, string(related)) {
			foundRelated = true
			break
		}
	}

	assert.True(t, foundRelated, "output should contain at least one related code")
}
