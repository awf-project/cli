package errors_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/errors"
	"github.com/stretchr/testify/assert"
)

func TestCatalogEntry_Structure(t *testing.T) {
	tests := []struct {
		name        string
		entry       errors.CatalogEntry
		wantCode    errors.ErrorCode
		wantDesc    string
		wantRes     string
		wantRelated int
	}{
		{
			name: "Complete entry with all fields",
			entry: errors.CatalogEntry{
				Code:         errors.ErrorCodeUserInputMissingFile,
				Description:  "Test description",
				Resolution:   "Test resolution",
				RelatedCodes: []errors.ErrorCode{errors.ErrorCodeUserInputInvalidFormat},
			},
			wantCode:    errors.ErrorCodeUserInputMissingFile,
			wantDesc:    "Test description",
			wantRes:     "Test resolution",
			wantRelated: 1,
		},
		{
			name: "Entry with no related codes",
			entry: errors.CatalogEntry{
				Code:         errors.ErrorCodeExecutionCommandTimeout,
				Description:  "Command timeout",
				Resolution:   "Increase timeout",
				RelatedCodes: []errors.ErrorCode{},
			},
			wantCode:    errors.ErrorCodeExecutionCommandTimeout,
			wantDesc:    "Command timeout",
			wantRes:     "Increase timeout",
			wantRelated: 0,
		},
		{
			name: "Entry with multiple related codes",
			entry: errors.CatalogEntry{
				Code:        errors.ErrorCodeWorkflowValidationCycleDetected,
				Description: "Cycle detected",
				Resolution:  "Break the cycle",
				RelatedCodes: []errors.ErrorCode{
					errors.ErrorCodeWorkflowValidationInvalidTransition,
					errors.ErrorCodeWorkflowValidationMissingState,
				},
			},
			wantCode:    errors.ErrorCodeWorkflowValidationCycleDetected,
			wantDesc:    "Cycle detected",
			wantRes:     "Break the cycle",
			wantRelated: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantCode, tt.entry.Code)
			assert.Equal(t, tt.wantDesc, tt.entry.Description)
			assert.Equal(t, tt.wantRes, tt.entry.Resolution)
			assert.Len(t, tt.entry.RelatedCodes, tt.wantRelated)
		})
	}
}

func TestErrorCatalog_AllDefinedCodesHaveEntries(t *testing.T) {
	// All error code constants should have catalog entries
	requiredCodes := []errors.ErrorCode{
		// USER category
		errors.ErrorCodeUserInputMissingFile,
		errors.ErrorCodeUserInputInvalidFormat,
		errors.ErrorCodeUserInputValidationFailed,
		// WORKFLOW category
		errors.ErrorCodeWorkflowParseYAMLSyntax,
		errors.ErrorCodeWorkflowParseUnknownField,
		errors.ErrorCodeWorkflowValidationCycleDetected,
		errors.ErrorCodeWorkflowValidationMissingState,
		errors.ErrorCodeWorkflowValidationInvalidTransition,
		// EXECUTION category
		errors.ErrorCodeExecutionCommandFailed,
		errors.ErrorCodeExecutionCommandTimeout,
		errors.ErrorCodeExecutionParallelPartialFailure,
		errors.ErrorCodeExecutionPluginDisabled,
		// SYSTEM category
		errors.ErrorCodeSystemIOReadFailed,
		errors.ErrorCodeSystemIOWriteFailed,
		errors.ErrorCodeSystemIOPermissionDenied,
	}

	for _, code := range requiredCodes {
		t.Run(string(code), func(t *testing.T) {
			entry, found := errors.GetCatalogEntry(code)
			assert.True(t, found, "Error code %s should have a catalog entry", code)
			assert.Equal(t, code, entry.Code, "Catalog entry Code field should match")
			assert.NotEmpty(t, entry.Description, "Description should not be empty")
			assert.NotEmpty(t, entry.Resolution, "Resolution should not be empty")
		})
	}
}

func TestErrorCatalog_EntryCompleteness(t *testing.T) {
	tests := []struct {
		name string
		code errors.ErrorCode
	}{
		{"USER.INPUT.MISSING_FILE", errors.ErrorCodeUserInputMissingFile},
		{"USER.INPUT.INVALID_FORMAT", errors.ErrorCodeUserInputInvalidFormat},
		{"USER.INPUT.VALIDATION_FAILED", errors.ErrorCodeUserInputValidationFailed},
		{"WORKFLOW.PARSE.YAML_SYNTAX", errors.ErrorCodeWorkflowParseYAMLSyntax},
		{"WORKFLOW.PARSE.UNKNOWN_FIELD", errors.ErrorCodeWorkflowParseUnknownField},
		{"WORKFLOW.VALIDATION.CYCLE_DETECTED", errors.ErrorCodeWorkflowValidationCycleDetected},
		{"WORKFLOW.VALIDATION.MISSING_STATE", errors.ErrorCodeWorkflowValidationMissingState},
		{"WORKFLOW.VALIDATION.INVALID_TRANSITION", errors.ErrorCodeWorkflowValidationInvalidTransition},
		{"EXECUTION.COMMAND.FAILED", errors.ErrorCodeExecutionCommandFailed},
		{"EXECUTION.COMMAND.TIMEOUT", errors.ErrorCodeExecutionCommandTimeout},
		{"EXECUTION.PARALLEL.PARTIAL_FAILURE", errors.ErrorCodeExecutionParallelPartialFailure},
		{"SYSTEM.IO.READ_FAILED", errors.ErrorCodeSystemIOReadFailed},
		{"SYSTEM.IO.WRITE_FAILED", errors.ErrorCodeSystemIOWriteFailed},
		{"SYSTEM.IO.PERMISSION_DENIED", errors.ErrorCodeSystemIOPermissionDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, found := errors.GetCatalogEntry(tt.code)
			assert.True(t, found)

			// Verify entry is complete
			assert.Equal(t, tt.code, entry.Code)
			assert.NotEmpty(t, entry.Description, "Description must not be empty")
			assert.NotEmpty(t, entry.Resolution, "Resolution must not be empty")
			// RelatedCodes can be empty but should be initialized
			assert.NotNil(t, entry.RelatedCodes)
		})
	}
}

func TestErrorCatalog_RelatedCodesAreValid(t *testing.T) {
	// Verify all RelatedCodes in catalog entries are valid error codes
	for code, entry := range errors.ErrorCatalog {
		t.Run(string(code), func(t *testing.T) {
			for _, relatedCode := range entry.RelatedCodes {
				// Related code should be a valid ErrorCode format
				assert.True(t, relatedCode.IsValid(), "Related code %s should be valid", relatedCode)

				// Related code should exist in catalog
				_, found := errors.GetCatalogEntry(relatedCode)
				assert.True(t, found, "Related code %s should exist in catalog", relatedCode)
			}
		})
	}
}

func TestErrorCatalog_NoEmptyStrings(t *testing.T) {
	// Verify no catalog entries have empty Description or Resolution
	for code, entry := range errors.ErrorCatalog {
		t.Run(string(code), func(t *testing.T) {
			assert.NotEmpty(t, entry.Description, "Description should not be empty")
			assert.NotEmpty(t, entry.Resolution, "Resolution should not be empty")
		})
	}
}

func TestErrorCatalog_DescriptionQuality(t *testing.T) {
	// Verify descriptions are meaningful (longer than 10 chars)
	for code, entry := range errors.ErrorCatalog {
		t.Run(string(code), func(t *testing.T) {
			assert.GreaterOrEqual(t, len(entry.Description), 10,
				"Description should be meaningful (at least 10 chars)")
			assert.GreaterOrEqual(t, len(entry.Resolution), 10,
				"Resolution should be meaningful (at least 10 chars)")
		})
	}
}

func TestErrorCatalog_NoDuplicateCodes(t *testing.T) {
	// Verify ErrorCatalog map has no duplicate keys
	// (This is enforced by Go's map type, but test validates catalog structure)
	seenCodes := make(map[errors.ErrorCode]bool)

	for code := range errors.ErrorCatalog {
		assert.False(t, seenCodes[code], "Error code %s should not be duplicated", code)
		seenCodes[code] = true
	}

	assert.Equal(t, len(errors.ErrorCatalog), len(seenCodes),
		"Number of unique codes should match catalog size")
}

func TestGetCatalogEntry_ValidCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		wantCode errors.ErrorCode
		wantDesc string
		wantRes  string
	}{
		{
			name:     "USER.INPUT.MISSING_FILE",
			code:     errors.ErrorCodeUserInputMissingFile,
			wantCode: errors.ErrorCodeUserInputMissingFile,
			wantDesc: "The specified file was not found at the given path.",
			wantRes:  "Verify the file path is correct and the file exists. Check for typos in the filename or path.",
		},
		{
			name:     "WORKFLOW.PARSE.YAML_SYNTAX",
			code:     errors.ErrorCodeWorkflowParseYAMLSyntax,
			wantCode: errors.ErrorCodeWorkflowParseYAMLSyntax,
			wantDesc: "YAML parsing error due to syntax violation or malformed structure.",
			wantRes:  "Validate YAML syntax using a YAML linter. Check for indentation errors, missing colons, or invalid characters.",
		},
		{
			name:     "EXECUTION.COMMAND.FAILED",
			code:     errors.ErrorCodeExecutionCommandFailed,
			wantCode: errors.ErrorCodeExecutionCommandFailed,
			wantDesc: "A shell command executed during workflow execution exited with a non-zero status code.",
			wantRes:  "Check command output for error details. Verify the command syntax and required dependencies are installed.",
		},
		{
			name:     "SYSTEM.IO.PERMISSION_DENIED",
			code:     errors.ErrorCodeSystemIOPermissionDenied,
			wantCode: errors.ErrorCodeSystemIOPermissionDenied,
			wantDesc: "Insufficient permissions to access the requested file or directory.",
			wantRes:  "Check file permissions with ls -l. Use chmod to grant necessary permissions or run with appropriate user privileges.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, found := errors.GetCatalogEntry(tt.code)

			assert.True(t, found)
			assert.Equal(t, tt.wantCode, entry.Code)
			assert.Equal(t, tt.wantDesc, entry.Description)
			assert.Equal(t, tt.wantRes, entry.Resolution)
		})
	}
}

func TestGetCatalogEntry_ExecutionPluginDisabled(t *testing.T) {
	entry, found := errors.GetCatalogEntry(errors.ErrorCodeExecutionPluginDisabled)
	assert.True(t, found, "catalog entry should exist")
	assert.Equal(t, errors.ErrorCodeExecutionPluginDisabled, entry.Code)
	assert.NotEmpty(t, entry.Description)
	assert.NotEmpty(t, entry.Resolution)
	assert.Contains(t, entry.Resolution, "awf plugin enable")
}

func TestGetCatalogEntry_InvalidCode(t *testing.T) {
	tests := []struct {
		name string
		code errors.ErrorCode
	}{
		{"Empty code", ""},
		{"Invalid format (no dots)", "INVALID"},
		{"Invalid format (one dot)", "INVALID.CODE"},
		{"Invalid format (four dots)", "INVALID.CODE.WITH.EXTRA"},
		{"Nonexistent but valid format", "UNKNOWN.ERROR.CODE"},
		{"Lowercase code", "user.input.missing_file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, found := errors.GetCatalogEntry(tt.code)

			assert.False(t, found, "GetCatalogEntry should return false for invalid code")
			assert.Equal(t, errors.ErrorCode(""), entry.Code, "Entry Code should be empty")
			assert.Empty(t, entry.Description, "Entry Description should be empty")
			assert.Empty(t, entry.Resolution, "Entry Resolution should be empty")
			assert.Nil(t, entry.RelatedCodes, "Entry RelatedCodes should be nil")
		})
	}
}

func TestGetCatalogEntry_ReturnsZeroValue(t *testing.T) {
	// Verify GetCatalogEntry returns zero-value CatalogEntry for not found
	entry, found := errors.GetCatalogEntry("NONEXISTENT.CODE.VALUE")

	assert.False(t, found)

	// Zero-value CatalogEntry should be empty
	assert.Equal(t, errors.ErrorCode(""), entry.Code)
	assert.Equal(t, "", entry.Description)
	assert.Equal(t, "", entry.Resolution)
	assert.Nil(t, entry.RelatedCodes)
}

func TestAllErrorCodes_ReturnsAllCodes(t *testing.T) {
	codes := errors.AllErrorCodes()

	// Should return all 15 defined error codes
	assert.Len(t, codes, 15, "Should return all 15 defined error codes")

	// Verify all codes are valid
	for _, code := range codes {
		assert.True(t, code.IsValid(), "Code %s should be valid", code)
	}
}

func TestAllErrorCodes_ContainsKnownCodes(t *testing.T) {
	codes := errors.AllErrorCodes()

	// Convert to map for easier lookup
	codeSet := make(map[errors.ErrorCode]bool)
	for _, code := range codes {
		codeSet[code] = true
	}

	// Verify all known codes are present
	knownCodes := []errors.ErrorCode{
		errors.ErrorCodeUserInputMissingFile,
		errors.ErrorCodeUserInputInvalidFormat,
		errors.ErrorCodeUserInputValidationFailed,
		errors.ErrorCodeWorkflowParseYAMLSyntax,
		errors.ErrorCodeWorkflowParseUnknownField,
		errors.ErrorCodeWorkflowValidationCycleDetected,
		errors.ErrorCodeWorkflowValidationMissingState,
		errors.ErrorCodeWorkflowValidationInvalidTransition,
		errors.ErrorCodeExecutionCommandFailed,
		errors.ErrorCodeExecutionCommandTimeout,
		errors.ErrorCodeExecutionParallelPartialFailure,
		errors.ErrorCodeSystemIOReadFailed,
		errors.ErrorCodeSystemIOWriteFailed,
		errors.ErrorCodeSystemIOPermissionDenied,
	}

	for _, code := range knownCodes {
		assert.True(t, codeSet[code], "AllErrorCodes should contain %s", code)
	}
}

func TestAllErrorCodes_CoversAllCategories(t *testing.T) {
	codes := errors.AllErrorCodes()

	// Track which categories are present
	categories := make(map[string]bool)
	for _, code := range codes {
		categories[code.Category()] = true
	}

	// Should have all four categories
	assert.True(t, categories["USER"], "Should have USER category codes")
	assert.True(t, categories["WORKFLOW"], "Should have WORKFLOW category codes")
	assert.True(t, categories["EXECUTION"], "Should have EXECUTION category codes")
	assert.True(t, categories["SYSTEM"], "Should have SYSTEM category codes")
}

func TestAllErrorCodes_NoDuplicates(t *testing.T) {
	codes := errors.AllErrorCodes()

	// Check for duplicates
	seen := make(map[errors.ErrorCode]bool)
	for _, code := range codes {
		assert.False(t, seen[code], "Code %s should not be duplicated", code)
		seen[code] = true
	}
}

func TestAllErrorCodes_NonEmpty(t *testing.T) {
	codes := errors.AllErrorCodes()

	assert.NotEmpty(t, codes, "AllErrorCodes should return at least one code")
	assert.Greater(t, len(codes), 0, "Should have at least one error code")
}

func TestAllErrorCodes_ConsistentAcrossCalls(t *testing.T) {
	// AllErrorCodes should return the same codes on multiple calls
	codes1 := errors.AllErrorCodes()
	codes2 := errors.AllErrorCodes()

	assert.Equal(t, len(codes1), len(codes2), "Should return same number of codes")

	// Convert to sets for comparison (order may differ)
	set1 := make(map[errors.ErrorCode]bool)
	set2 := make(map[errors.ErrorCode]bool)

	for _, code := range codes1 {
		set1[code] = true
	}
	for _, code := range codes2 {
		set2[code] = true
	}

	assert.Equal(t, set1, set2, "Should return the same set of codes")
}

func TestIntegration_CatalogCodesAreValid(t *testing.T) {
	// Every code in the catalog should pass IsValid()
	for code := range errors.ErrorCatalog {
		t.Run(string(code), func(t *testing.T) {
			assert.True(t, code.IsValid(), "Catalog code %s should be valid", code)
		})
	}
}

func TestIntegration_CatalogCodesMatchExitCodes(t *testing.T) {
	// Verify catalog codes map to correct exit codes
	tests := []struct {
		category string
		exitCode int
		codes    []errors.ErrorCode
	}{
		{
			category: "USER",
			exitCode: 1,
			codes: []errors.ErrorCode{
				errors.ErrorCodeUserInputMissingFile,
				errors.ErrorCodeUserInputInvalidFormat,
				errors.ErrorCodeUserInputValidationFailed,
			},
		},
		{
			category: "WORKFLOW",
			exitCode: 2,
			codes: []errors.ErrorCode{
				errors.ErrorCodeWorkflowParseYAMLSyntax,
				errors.ErrorCodeWorkflowParseUnknownField,
				errors.ErrorCodeWorkflowValidationCycleDetected,
			},
		},
		{
			category: "EXECUTION",
			exitCode: 3,
			codes: []errors.ErrorCode{
				errors.ErrorCodeExecutionCommandFailed,
				errors.ErrorCodeExecutionCommandTimeout,
			},
		},
		{
			category: "SYSTEM",
			exitCode: 4,
			codes: []errors.ErrorCode{
				errors.ErrorCodeSystemIOReadFailed,
				errors.ErrorCodeSystemIOWriteFailed,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			for _, code := range tt.codes {
				entry, found := errors.GetCatalogEntry(code)
				assert.True(t, found)
				assert.Equal(t, tt.exitCode, code.ExitCode(),
					"Code %s should map to exit code %d", code, tt.exitCode)
				assert.Equal(t, tt.category, code.Category(),
					"Code %s should have category %s", code, tt.category)
				assert.Equal(t, code, entry.Code, "Entry code should match")
			}
		})
	}
}

func TestErrorCatalog_HandlesPanicGracefully(t *testing.T) {
	// GetCatalogEntry should not panic with any input
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GetCatalogEntry panicked: %v", r)
		}
	}()

	// Try various inputs that should not panic
	_, _ = errors.GetCatalogEntry("")
	_, _ = errors.GetCatalogEntry("INVALID")
	_, _ = errors.GetCatalogEntry("TOO.MANY.DOTS.HERE.NOW")
}

func TestAllErrorCodes_HandlesPanicGracefully(t *testing.T) {
	// AllErrorCodes should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("AllErrorCodes panicked: %v", r)
		}
	}()

	codes := errors.AllErrorCodes()
	assert.NotNil(t, codes)
}
