//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domainerrors "github.com/vanoix/awf/internal/domain/errors"
	"github.com/vanoix/awf/internal/domain/ports"
	errfmt "github.com/vanoix/awf/internal/infrastructure/errors"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// =============================================================================
// C047: Structured Error Codes Taxonomy - Integration Tests
// Tests validate end-to-end error code handling across all layers:
// - Domain error taxonomy (ErrorCode, StructuredError)
// - Port abstractions (ErrorFormatter interface)
// - Infrastructure formatters (JSON, Human)
// - CLI integration (categorizeError, WriteError, error command)
// - Exit code mapping (USER→1, WORKFLOW→2, EXECUTION→3, SYSTEM→4)
// =============================================================================

const (
	// Expected error code count - all 14 error codes defined in spec
	expectedErrorCodeCount = 14
)

// TestC047_ErrorCommand_ListAllCodes validates the error command lists all error codes
// Given: no arguments to awf error
// When: command executes
// Then: all 14 error codes from catalog are listed
func TestC047_ErrorCommand_ListAllCodes(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)

	// Act
	cmd := exec.Command(binPath, "error")
	output, err := cmd.CombinedOutput()

	// Assert
	require.NoError(t, err, "awf error command should succeed")
	outputStr := string(output)

	// Verify all error codes are present
	allCodes := domainerrors.AllErrorCodes()
	assert.GreaterOrEqual(t, len(allCodes), expectedErrorCodeCount,
		"Should have at least %d error codes", expectedErrorCodeCount)

	for _, code := range allCodes {
		assert.Contains(t, outputStr, string(code),
			"Output should contain error code %s", code)
	}

	// Verify catalog entries have descriptions
	assert.Contains(t, outputStr, "Description:", "Output should include description headers")
	assert.Contains(t, outputStr, "Resolution:", "Output should include resolution headers")
}

// TestC047_ErrorCommand_LookupSpecificCode validates error command displays specific code details
// Given: valid error code argument (e.g., USER.INPUT.MISSING_FILE)
// When: command executes
// Then: description, resolution, and related codes are displayed
func TestC047_ErrorCommand_LookupSpecificCode(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	testCode := domainerrors.ErrorCodeUserInputMissingFile

	// Act
	cmd := exec.Command(binPath, "error", string(testCode))
	output, err := cmd.CombinedOutput()

	// Assert
	require.NoError(t, err, "awf error command should succeed")
	outputStr := string(output)

	// Verify error code is displayed
	assert.Contains(t, outputStr, string(testCode), "Output should contain error code")

	// Verify catalog entry content
	entry, found := domainerrors.GetCatalogEntry(testCode)
	require.True(t, found, "Error code should exist in catalog")

	assert.Contains(t, outputStr, entry.Description, "Output should contain description")
	assert.Contains(t, outputStr, entry.Resolution, "Output should contain resolution")

	// Verify related codes if any
	if len(entry.RelatedCodes) > 0 {
		assert.Contains(t, outputStr, "Related", "Output should contain related codes section")
	}
}

// TestC047_ErrorCommand_PrefixMatch validates prefix matching for error codes
// Given: partial error code prefix (e.g., WORKFLOW.VALIDATION)
// When: command executes
// Then: all matching codes are displayed
func TestC047_ErrorCommand_PrefixMatch(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	prefix := "WORKFLOW.VALIDATION"

	// Act
	cmd := exec.Command(binPath, "error", prefix)
	output, err := cmd.CombinedOutput()

	// Assert
	require.NoError(t, err, "awf error command should succeed")
	outputStr := string(output)

	// Verify all matching codes are present
	expectedCodes := []domainerrors.ErrorCode{
		domainerrors.ErrorCodeWorkflowValidationCycleDetected,
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		domainerrors.ErrorCodeWorkflowValidationInvalidTransition,
	}

	for _, code := range expectedCodes {
		assert.Contains(t, outputStr, string(code),
			"Output should contain error code %s", code)
	}

	// Verify non-matching codes are not present
	assert.NotContains(t, outputStr, "USER.INPUT",
		"Output should not contain non-matching codes")
}

// TestC047_ErrorCommand_InvalidCode validates error handling for unknown codes
// Given: non-existent error code
// When: command executes
// Then: error message indicates code not found
func TestC047_ErrorCommand_InvalidCode(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	invalidCode := "INVALID.CODE.HERE"

	// Act
	cmd := exec.Command(binPath, "error", invalidCode)
	output, err := cmd.CombinedOutput()

	// Assert
	assert.Error(t, err, "awf error command should fail for invalid code")
	outputStr := string(output)

	// Verify error message
	assert.Contains(t, outputStr, "not found",
		"Output should indicate error code not found")
	assert.Contains(t, outputStr, invalidCode,
		"Output should mention the invalid code")
}

// TestC047_ErrorCommand_JSONOutput validates JSON format for error command
// Given: --output=json flag with error code argument
// When: command executes
// Then: JSON object with code/description/resolution/related_codes
func TestC047_ErrorCommand_JSONOutput(t *testing.T) {
	// Arrange
	binPath := buildBinaryIfNeeded(t)
	testCode := domainerrors.ErrorCodeUserInputMissingFile

	// Act
	cmd := exec.Command(binPath, "--format", "json", "error", string(testCode))
	output, err := cmd.CombinedOutput()

	// Assert
	require.NoError(t, err, "awf error command should succeed")

	// Parse JSON
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err, "Output should be valid JSON")

	// Verify JSON structure
	assert.Equal(t, string(testCode), result["code"], "JSON should contain error code")
	assert.NotEmpty(t, result["description"], "JSON should contain description")
	assert.NotEmpty(t, result["resolution"], "JSON should contain resolution")

	// Verify related codes if present
	entry, _ := domainerrors.GetCatalogEntry(testCode)
	if len(entry.RelatedCodes) > 0 {
		relatedCodes, ok := result["related_codes"].([]interface{})
		require.True(t, ok, "JSON should contain related_codes array")
		assert.Greater(t, len(relatedCodes), 0, "Related codes should not be empty")
	}
}

// TestC047_StructuredError_ExitCodeMapping validates exit code mapping for each category
// Given: workflows that trigger each error category
// When: workflows execute and fail
// Then: exit codes match category (USER→1, WORKFLOW→2, EXECUTION→3, SYSTEM→4)
func TestC047_StructuredError_ExitCodeMapping(t *testing.T) {
	tests := []struct {
		name         string
		workflowFile string
		wantExitCode int
		errorCode    string
		description  string
	}{
		{
			name:         "USER error returns exit code 1",
			workflowFile: "exit-user-error.yaml",
			wantExitCode: 1,
			errorCode:    "USER",
			description:  "User-facing input errors should exit with code 1",
		},
		{
			name:         "WORKFLOW error returns exit code 2",
			workflowFile: "exit-workflow-error.yaml",
			wantExitCode: 2,
			errorCode:    "WORKFLOW",
			description:  "Workflow definition errors should exit with code 2",
		},
		{
			name:         "EXECUTION error returns exit code 3",
			workflowFile: "exit-execution-error.yaml",
			wantExitCode: 3,
			errorCode:    "EXECUTION",
			description:  "Runtime execution errors should exit with code 3",
		},
	}

	binPath := buildBinaryIfNeeded(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			workflowPath := filepath.Join("../fixtures/workflows", tt.workflowFile)
			require.FileExists(t, workflowPath, "Workflow fixture should exist")

			// Act
			cmd := exec.Command(binPath, "run", workflowPath)
			output, err := cmd.CombinedOutput()

			// Assert
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "Command should exit with error code")

			actualExitCode := exitErr.ExitCode()
			assert.Equal(t, tt.wantExitCode, actualExitCode,
				"Exit code should match category: %s", tt.description)

			// Verify error output contains expected category marker
			outputStr := string(output)
			t.Logf("Output: %s", outputStr)
			assert.Contains(t, outputStr, tt.errorCode,
				"Error output should contain category marker")
		})
	}
}

// TestC047_StructuredError_JSONFormat validates JSON output includes error_code field
// Given: workflow failure with --output=json
// When: error is written to output
// Then: JSON includes "error_code" field with hierarchical code
func TestC047_StructuredError_JSONFormat(t *testing.T) {
	// Arrange - create a test StructuredError
	testErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"test workflow file not found",
		map[string]any{"path": "/test/workflow.yaml"},
		nil,
	)

	// Create OutputWriter with JSON format
	var stdout, stderr bytes.Buffer
	writer := ui.NewOutputWriter(&stdout, &stderr, ui.FormatJSON, false)

	// Act - write error
	writer.WriteError(testErr, 1)

	// Assert - parse JSON output
	var errorResponse struct {
		Error     string                 `json:"error"`
		ErrorCode string                 `json:"error_code"`
		Code      int                    `json:"code"`
		Details   map[string]interface{} `json:"details,omitempty"`
	}

	output := stderr.Bytes()
	err := json.Unmarshal(output, &errorResponse)
	require.NoError(t, err, "Error output should be valid JSON")

	// Verify error_code field
	assert.Equal(t, string(domainerrors.ErrorCodeUserInputMissingFile),
		errorResponse.ErrorCode, "JSON should include hierarchical error code")
	assert.Equal(t, 1, errorResponse.Code, "JSON should include exit code")
	assert.Contains(t, errorResponse.Error, "test workflow file not found",
		"JSON should include error message")
}

// TestC047_StructuredError_HumanFormat validates human-readable output includes error code
// Given: workflow failure with text output format
// When: error is written to stderr
// Then: output includes [ERROR_CODE] reference
func TestC047_StructuredError_HumanFormat(t *testing.T) {
	// Arrange - create a test StructuredError
	testErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationCycleDetected,
		"state machine cycle detected",
		map[string]any{
			"cycle": []string{"state_a", "state_b", "state_a"},
		},
		nil,
	)

	// Create OutputWriter with text format
	var stdout, stderr bytes.Buffer
	writer := ui.NewOutputWriter(&stdout, &stderr, ui.FormatText, true) // no color for testing

	// Act - write error
	writer.WriteError(testErr, 2)

	// Assert - verify human-readable format
	output := stderr.String()

	// Verify error code in brackets
	assert.Contains(t, output, "[WORKFLOW.VALIDATION.CYCLE_DETECTED]",
		"Human-readable output should include error code in brackets")

	// Verify message
	assert.Contains(t, output, "state machine cycle detected",
		"Human-readable output should include error message")

	// Verify details section
	assert.Contains(t, output, "Details:",
		"Human-readable output should include details section")
	assert.Contains(t, output, "cycle:",
		"Human-readable output should include detail keys")
}

// TestC047_CategorizeError_StructuredErrorPath validates categorizeError detects StructuredError
// Given: StructuredError with defined ErrorCode
// When: categorizeError is called
// Then: exit code matches ErrorCode.ExitCode() mapping
func TestC047_CategorizeError_StructuredErrorPath(t *testing.T) {
	tests := []struct {
		name         string
		errorCode    domainerrors.ErrorCode
		wantExitCode int
	}{
		{
			name:         "USER error maps to exit code 1",
			errorCode:    domainerrors.ErrorCodeUserInputMissingFile,
			wantExitCode: 1,
		},
		{
			name:         "WORKFLOW error maps to exit code 2",
			errorCode:    domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			wantExitCode: 2,
		},
		{
			name:         "EXECUTION error maps to exit code 3",
			errorCode:    domainerrors.ErrorCodeExecutionCommandFailed,
			wantExitCode: 3,
		},
		{
			name:         "SYSTEM error maps to exit code 4",
			errorCode:    domainerrors.ErrorCodeSystemIOReadFailed,
			wantExitCode: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			structuredErr := domainerrors.NewStructuredError(
				tt.errorCode,
				"test error message",
				nil,
				nil,
			)

			// Act
			exitCode := structuredErr.ExitCode()

			// Assert
			assert.Equal(t, tt.wantExitCode, exitCode,
				"StructuredError exit code should match category mapping")
		})
	}
}

// TestC047_CategorizeError_FallbackPath validates string matching fallback for plain errors
// Given: plain error (not StructuredError) with recognizable message
// When: categorizeError is called
// Then: exit code matches string pattern mapping (backward compatible)
func TestC047_CategorizeError_FallbackPath(t *testing.T) {
	// Note: This test validates the fallback path in categorizeError()
	// It uses the CLI integration test fixtures that trigger plain errors
	binPath := buildBinaryIfNeeded(t)

	tests := []struct {
		name         string
		workflowFile string
		wantExitCode int
		errorPattern string
	}{
		{
			name:         "not found pattern maps to USER exit code",
			workflowFile: "invalid-missing-name.yaml",
			wantExitCode: 1,
			errorPattern: "not found",
		},
		{
			name:         "invalid pattern maps to WORKFLOW exit code",
			workflowFile: "invalid-syntax.yaml",
			wantExitCode: 2,
			errorPattern: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			workflowPath := filepath.Join("../fixtures/workflows", tt.workflowFile)
			require.FileExists(t, workflowPath, "Workflow fixture should exist")

			// Act
			cmd := exec.Command(binPath, "run", workflowPath)
			output, err := cmd.CombinedOutput()

			// Assert
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "Command should exit with error")

			actualExitCode := exitErr.ExitCode()
			assert.Equal(t, tt.wantExitCode, actualExitCode,
				"Exit code should match string pattern mapping")

			outputStr := string(output)
			assert.Contains(t, outputStr, tt.errorPattern,
				"Error output should contain expected pattern")
		})
	}
}

// TestC047_BackwardCompatibility_PlainErrors validates existing error behavior preserved
// Given: workflows using legacy error patterns (no StructuredError)
// When: workflows execute
// Then: exit codes and error messages match pre-C047 behavior
func TestC047_BackwardCompatibility_PlainErrors(t *testing.T) {
	// This test verifies that plain errors still work correctly
	// using existing integration test workflows
	binPath := buildBinaryIfNeeded(t)

	tests := []struct {
		name         string
		workflowFile string
		wantExitCode int
	}{
		{
			name:         "missing file error still works",
			workflowFile: "invalid-missing-name.yaml",
			wantExitCode: 1,
		},
		{
			name:         "syntax error still works",
			workflowFile: "invalid-syntax.yaml",
			wantExitCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			workflowPath := filepath.Join("../fixtures/workflows", tt.workflowFile)

			// Act
			cmd := exec.Command(binPath, "run", workflowPath)
			_, err := cmd.CombinedOutput()

			// Assert - backward compatibility: exit codes preserved
			exitErr, ok := err.(*exec.ExitError)
			require.True(t, ok, "Command should exit with error")

			actualExitCode := exitErr.ExitCode()
			assert.Equal(t, tt.wantExitCode, actualExitCode,
				"Backward compatibility: exit code should be preserved")
		})
	}
}

// TestC047_DomainLayerPurity_StdlibOnly validates domain/errors imports only stdlib
// Given: domain/errors package
// When: package is compiled
// Then: no external dependencies beyond standard library
func TestC047_DomainLayerPurity_StdlibOnly(t *testing.T) {
	// Arrange
	domainErrorsPath := filepath.Join("..", "..", "internal", "domain", "errors")

	// Act - check imports in all Go files
	files, err := filepath.Glob(filepath.Join(domainErrorsPath, "*.go"))
	require.NoError(t, err, "Should be able to glob domain/errors files")
	require.NotEmpty(t, files, "Domain errors package should have Go files")

	// Assert - verify no external imports
	forbiddenImports := []string{
		"github.com",
		"golang.org/x/",
		"go.uber.org",
		"github.com/spf13",
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		require.NoError(t, err, "Should be able to read file %s", file)

		fileContent := string(content)
		for _, forbidden := range forbiddenImports {
			assert.NotContains(t, fileContent, forbidden,
				"Domain layer should not import external packages: %s in %s",
				forbidden, filepath.Base(file))
		}
	}

	t.Log("Domain layer purity verified: only stdlib imports")
}

// TestC047_ErrorCatalog_Completeness validates every ErrorCode has catalog entry
// Given: all defined ErrorCode constants
// When: catalog is queried
// Then: every code has description, resolution, and related codes
func TestC047_ErrorCatalog_Completeness(t *testing.T) {
	// Arrange - get all defined error codes
	allCodes := domainerrors.AllErrorCodes()
	require.GreaterOrEqual(t, len(allCodes), expectedErrorCodeCount,
		"Should have at least %d error codes", expectedErrorCodeCount)

	// Act & Assert - verify each code has complete catalog entry
	for _, code := range allCodes {
		t.Run(string(code), func(t *testing.T) {
			entry, found := domainerrors.GetCatalogEntry(code)
			require.True(t, found, "Error code %s should exist in catalog", code)

			// Verify required fields
			assert.Equal(t, code, entry.Code, "Catalog entry code should match")
			assert.NotEmpty(t, entry.Description,
				"Error code %s should have description", code)
			assert.NotEmpty(t, entry.Resolution,
				"Error code %s should have resolution", code)

			// Verify description and resolution are meaningful
			assert.Greater(t, len(entry.Description), 20,
				"Description should be meaningful: %s", code)
			assert.Greater(t, len(entry.Resolution), 20,
				"Resolution should be meaningful: %s", code)
		})
	}
}

// TestC047_ErrorFormatter_JSONStructure validates JSONErrorFormatter output structure
// Given: StructuredError with all fields populated
// When: formatter formats the error
// Then: JSON includes code, message, details, timestamp, all properly serialized
func TestC047_ErrorFormatter_JSONStructure(t *testing.T) {
	// Arrange
	formatter := errfmt.NewJSONErrorFormatter()
	testTime := time.Date(2025, 1, 15, 10, 30, 45, 0, time.UTC)

	testErr := &domainerrors.StructuredError{
		Code:    domainerrors.ErrorCodeExecutionCommandTimeout,
		Message: "command execution timed out",
		Details: map[string]any{
			"command": "sleep 100",
			"timeout": "30s",
			"pid":     12345,
		},
		Cause:     fmt.Errorf("context deadline exceeded"),
		Timestamp: testTime,
	}

	// Act
	output := formatter.FormatError(testErr)

	// Assert - parse JSON
	var result map[string]interface{}
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Formatter output should be valid JSON")

	// Verify required fields
	assert.Equal(t, "EXECUTION.COMMAND.TIMEOUT", result["error_code"],
		"JSON should include error code")
	assert.Equal(t, "command execution timed out", result["message"],
		"JSON should include message")
	assert.NotEmpty(t, result["timestamp"], "JSON should include timestamp")

	// Verify details
	details, ok := result["details"].(map[string]interface{})
	require.True(t, ok, "JSON should include details as object")
	assert.Equal(t, "sleep 100", details["command"], "Details should be preserved")
	assert.Equal(t, "30s", details["timeout"], "Details should be preserved")
	assert.Equal(t, float64(12345), details["pid"], "Details should be preserved")

	// Verify timestamp format (ISO 8601 / RFC3339)
	timestamp, ok := result["timestamp"].(string)
	require.True(t, ok, "Timestamp should be string")
	_, err = time.Parse(time.RFC3339, timestamp)
	assert.NoError(t, err, "Timestamp should be RFC3339 formatted")
}

// TestC047_ErrorFormatter_HumanReadable validates HumanErrorFormatter colorized output
// Given: StructuredError with details
// When: formatter formats the error with colors enabled
// Then: output includes ANSI color codes and [ERROR_CODE] prefix
func TestC047_ErrorFormatter_HumanReadable(t *testing.T) {
	tests := []struct {
		name            string
		colorEnabled    bool
		expectAnsiCodes bool
	}{
		{
			name:            "with colors enabled",
			colorEnabled:    true,
			expectAnsiCodes: true,
		},
		{
			name:            "with colors disabled",
			colorEnabled:    false,
			expectAnsiCodes: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			formatter := errfmt.NewHumanErrorFormatter(tt.colorEnabled)
			testErr := domainerrors.NewStructuredError(
				domainerrors.ErrorCodeSystemIOPermissionDenied,
				"cannot write to file",
				map[string]any{
					"file": "/var/log/app.log",
					"user": "alice",
				},
				nil,
			)

			// Act
			output := formatter.FormatError(testErr)

			// Assert
			// Verify error code prefix
			assert.Contains(t, output, "[SYSTEM.IO.PERMISSION_DENIED]",
				"Output should include error code in brackets")

			// Verify message
			assert.Contains(t, output, "cannot write to file",
				"Output should include error message")

			// Verify details section
			assert.Contains(t, output, "Details:",
				"Output should include details section")
			assert.Contains(t, output, "file:",
				"Output should include detail keys")
			assert.Contains(t, output, "/var/log/app.log",
				"Output should include detail values")

			// Verify ANSI color codes based on configuration
			if tt.expectAnsiCodes {
				// ANSI color codes contain escape sequences
				assert.True(t, strings.Contains(output, "\x1b["),
					"Output should contain ANSI color codes when enabled")
			} else {
				// No ANSI codes when disabled
				assert.False(t, strings.Contains(output, "\x1b["),
					"Output should not contain ANSI color codes when disabled")
			}
		})
	}
}

// TestC047_WriteError_StructuredDetection validates WriteError detects StructuredError type
// Given: StructuredError passed to WriteError
// When: output format is JSON
// Then: error_code field is populated in ErrorResponse
func TestC047_WriteError_StructuredDetection(t *testing.T) {
	// Arrange
	testErr := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		"referenced state does not exist",
		map[string]any{
			"state":      "nonexistent_state",
			"referenced": "step_3",
		},
		nil,
	)

	var stdout, stderr bytes.Buffer
	writer := ui.NewOutputWriter(&stdout, &stderr, ui.FormatJSON, false)

	// Act
	writer.WriteError(testErr, 2)

	// Assert
	var errorResponse struct {
		Error     string                 `json:"error"`
		ErrorCode string                 `json:"error_code"`
		Code      int                    `json:"code"`
		Details   map[string]interface{} `json:"details,omitempty"`
	}

	output := stderr.Bytes()
	err := json.Unmarshal(output, &errorResponse)
	require.NoError(t, err, "Error output should be valid JSON")

	// Verify StructuredError was detected and error_code field populated
	assert.Equal(t, "WORKFLOW.VALIDATION.MISSING_STATE", errorResponse.ErrorCode,
		"WriteError should detect StructuredError and populate error_code")
	assert.Equal(t, "referenced state does not exist", errorResponse.Error,
		"Error message should be preserved")
	assert.Equal(t, 2, errorResponse.Code, "Exit code should be preserved")
}

// TestC047_EndToEnd_AcceptanceCriteria validates all acceptance criteria from spec
// Given: full awf CLI installation
// When: various workflows are executed
// Then: all acceptance criteria pass:
//   - Machine-readable codes in CATEGORY.SUBCATEGORY.SPECIFIC format
//   - Exit codes 1-4 map to error categories
//   - JSON output includes structured error objects
//   - Human output includes error code reference
//   - awf error command returns explanations
//   - All existing tests pass
func TestC047_EndToEnd_AcceptanceCriteria(t *testing.T) {
	binPath := buildBinaryIfNeeded(t)

	t.Run("AC1: Machine-readable error codes", func(t *testing.T) {
		// Verify all error codes follow CATEGORY.SUBCATEGORY.SPECIFIC format
		allCodes := domainerrors.AllErrorCodes()
		for _, code := range allCodes {
			assert.True(t, code.IsValid(),
				"Error code %s should follow CATEGORY.SUBCATEGORY.SPECIFIC format", code)

			// Verify parts are non-empty
			assert.NotEmpty(t, code.Category(), "Category should be non-empty for %s", code)
			assert.NotEmpty(t, code.Subcategory(), "Subcategory should be non-empty for %s", code)
			assert.NotEmpty(t, code.Specific(), "Specific should be non-empty for %s", code)
		}
	})

	t.Run("AC2: Exit code mapping", func(t *testing.T) {
		// Verify exit codes map correctly
		mapping := map[domainerrors.ErrorCode]int{
			domainerrors.ErrorCodeUserInputMissingFile:    1,
			domainerrors.ErrorCodeWorkflowParseYAMLSyntax: 2,
			domainerrors.ErrorCodeExecutionCommandFailed:  3,
			domainerrors.ErrorCodeSystemIOReadFailed:      4,
		}

		for code, expectedExit := range mapping {
			err := domainerrors.NewStructuredError(code, "test", nil, nil)
			assert.Equal(t, expectedExit, err.ExitCode(),
				"Error code %s should map to exit code %d", code, expectedExit)
		}
	})

	t.Run("AC3: JSON output structure", func(t *testing.T) {
		// Verify JSON output includes structured error objects
		testErr := domainerrors.NewStructuredError(
			domainerrors.ErrorCodeUserInputValidationFailed,
			"validation failed",
			map[string]any{"field": "name"},
			nil,
		)

		var stdout, stderr bytes.Buffer
		writer := ui.NewOutputWriter(&stdout, &stderr, ui.FormatJSON, false)
		writer.WriteError(testErr, 1)

		var result map[string]interface{}
		err := json.Unmarshal(stderr.Bytes(), &result)
		require.NoError(t, err, "JSON output should be valid")

		assert.Contains(t, result, "error_code", "JSON should include error_code field")
		assert.Contains(t, result, "error", "JSON should include error field")
		assert.Contains(t, result, "code", "JSON should include code field")
	})

	t.Run("AC4: Human-readable output", func(t *testing.T) {
		// Verify human output includes error code reference
		testErr := domainerrors.NewStructuredError(
			domainerrors.ErrorCodeWorkflowValidationCycleDetected,
			"cycle detected",
			nil,
			nil,
		)

		var stdout, stderr bytes.Buffer
		writer := ui.NewOutputWriter(&stdout, &stderr, ui.FormatText, true)
		writer.WriteError(testErr, 2)

		output := stderr.String()
		assert.Contains(t, output, "[WORKFLOW.VALIDATION.CYCLE_DETECTED]",
			"Human output should include error code in brackets")
	})

	t.Run("AC5: awf error command", func(t *testing.T) {
		// Verify awf error command returns explanations
		cmd := exec.Command(binPath, "error",
			string(domainerrors.ErrorCodeUserInputMissingFile))
		output, err := cmd.CombinedOutput()

		require.NoError(t, err, "awf error command should succeed")
		outputStr := string(output)

		assert.Contains(t, outputStr, "Description:",
			"Error command should include description")
		assert.Contains(t, outputStr, "Resolution:",
			"Error command should include resolution")
	})

	t.Run("AC6: Backward compatibility", func(t *testing.T) {
		// Verify existing tests still pass (tested in other test functions)
		// This is a meta-check that the test suite itself passes
		t.Log("Backward compatibility verified through other tests")
	})
}

// =============================================================================
// Test Helpers
// =============================================================================

// buildBinaryIfNeeded builds the AWF binary if not already present
func buildBinaryIfNeeded(t *testing.T) string {
	t.Helper()

	// Check if binary already exists in bin/
	binPath := filepath.Join("..", "..", "bin", "awf")
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}

	// Binary not found - build it
	t.Log("Building AWF binary for integration tests...")

	projectRoot := filepath.Join("..", "..")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/awf")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	t.Logf("Binary built successfully at %s", binPath)
	return binPath
}

// TestC047_ErrorFormatter_InterfaceCompliance validates compile-time interface compliance
// Given: ErrorFormatter implementations
// When: code compiles
// Then: all implementations satisfy the ErrorFormatter port interface
func TestC047_ErrorFormatter_InterfaceCompliance(t *testing.T) {
	// Compile-time verification of interface compliance
	var _ ports.ErrorFormatter = (*errfmt.JSONErrorFormatter)(nil)
	var _ ports.ErrorFormatter = (*errfmt.HumanErrorFormatter)(nil)

	t.Log("ErrorFormatter interface compliance verified at compile time")
}
