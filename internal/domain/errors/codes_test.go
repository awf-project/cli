package errors_test

import (
	"testing"

	"github.com/awf-project/awf/internal/domain/errors"
	"github.com/stretchr/testify/assert"
)

func TestErrorCodeConstants_USER_Category(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{
			name:     "ErrorCodeUserInputMissingFile",
			code:     errors.ErrorCodeUserInputMissingFile,
			expected: "USER.INPUT.MISSING_FILE",
		},
		{
			name:     "ErrorCodeUserInputInvalidFormat",
			code:     errors.ErrorCodeUserInputInvalidFormat,
			expected: "USER.INPUT.INVALID_FORMAT",
		},
		{
			name:     "ErrorCodeUserInputValidationFailed",
			code:     errors.ErrorCodeUserInputValidationFailed,
			expected: "USER.INPUT.VALIDATION_FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.code))
			assert.Equal(t, tt.expected, string(tt.code))
		})
	}
}

func TestErrorCodeConstants_WORKFLOW_Category(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{
			name:     "ErrorCodeWorkflowParseYAMLSyntax",
			code:     errors.ErrorCodeWorkflowParseYAMLSyntax,
			expected: "WORKFLOW.PARSE.YAML_SYNTAX",
		},
		{
			name:     "ErrorCodeWorkflowParseUnknownField",
			code:     errors.ErrorCodeWorkflowParseUnknownField,
			expected: "WORKFLOW.PARSE.UNKNOWN_FIELD",
		},
		{
			name:     "ErrorCodeWorkflowValidationCycleDetected",
			code:     errors.ErrorCodeWorkflowValidationCycleDetected,
			expected: "WORKFLOW.VALIDATION.CYCLE_DETECTED",
		},
		{
			name:     "ErrorCodeWorkflowValidationMissingState",
			code:     errors.ErrorCodeWorkflowValidationMissingState,
			expected: "WORKFLOW.VALIDATION.MISSING_STATE",
		},
		{
			name:     "ErrorCodeWorkflowValidationInvalidTransition",
			code:     errors.ErrorCodeWorkflowValidationInvalidTransition,
			expected: "WORKFLOW.VALIDATION.INVALID_TRANSITION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.code))
			assert.Equal(t, tt.expected, string(tt.code))
		})
	}
}

func TestErrorCodeConstants_EXECUTION_Category(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{
			name:     "ErrorCodeExecutionCommandFailed",
			code:     errors.ErrorCodeExecutionCommandFailed,
			expected: "EXECUTION.COMMAND.FAILED",
		},
		{
			name:     "ErrorCodeExecutionCommandTimeout",
			code:     errors.ErrorCodeExecutionCommandTimeout,
			expected: "EXECUTION.COMMAND.TIMEOUT",
		},
		{
			name:     "ErrorCodeExecutionParallelPartialFailure",
			code:     errors.ErrorCodeExecutionParallelPartialFailure,
			expected: "EXECUTION.PARALLEL.PARTIAL_FAILURE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.code))
			assert.Equal(t, tt.expected, string(tt.code))
		})
	}
}

func TestErrorCodeConstants_SYSTEM_Category(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{
			name:     "ErrorCodeSystemIOReadFailed",
			code:     errors.ErrorCodeSystemIOReadFailed,
			expected: "SYSTEM.IO.READ_FAILED",
		},
		{
			name:     "ErrorCodeSystemIOWriteFailed",
			code:     errors.ErrorCodeSystemIOWriteFailed,
			expected: "SYSTEM.IO.WRITE_FAILED",
		},
		{
			name:     "ErrorCodeSystemIOPermissionDenied",
			code:     errors.ErrorCodeSystemIOPermissionDenied,
			expected: "SYSTEM.IO.PERMISSION_DENIED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.code))
			assert.Equal(t, tt.expected, string(tt.code))
		})
	}
}

func TestErrorCodeConstants_AllValid(t *testing.T) {
	// Verify all constants follow CATEGORY.SUBCATEGORY.SPECIFIC format
	allCodes := []errors.ErrorCode{
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
		// SYSTEM category
		errors.ErrorCodeSystemIOReadFailed,
		errors.ErrorCodeSystemIOWriteFailed,
		errors.ErrorCodeSystemIOPermissionDenied,
	}

	for _, code := range allCodes {
		t.Run(string(code), func(t *testing.T) {
			assert.True(t, code.IsValid(), "code %s should be valid", code)
		})
	}
}

func TestErrorCodeConstants_UniqueValues(t *testing.T) {
	// Verify no duplicate error codes exist
	allCodes := []errors.ErrorCode{
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

	seen := make(map[string]bool)
	for _, code := range allCodes {
		strCode := string(code)
		assert.False(t, seen[strCode], "duplicate error code: %s", strCode)
		seen[strCode] = true
	}
}

func TestErrorCode_Category_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{
			name:     "USER category",
			code:     errors.ErrorCodeUserInputMissingFile,
			expected: "USER",
		},
		{
			name:     "WORKFLOW category",
			code:     errors.ErrorCodeWorkflowParseYAMLSyntax,
			expected: "WORKFLOW",
		},
		{
			name:     "EXECUTION category",
			code:     errors.ErrorCodeExecutionCommandFailed,
			expected: "EXECUTION",
		},
		{
			name:     "SYSTEM category",
			code:     errors.ErrorCodeSystemIOReadFailed,
			expected: "SYSTEM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.Category())
		})
	}
}

func TestErrorCode_Category_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{
			name:     "empty code",
			code:     errors.ErrorCode(""),
			expected: "",
		},
		{
			name:     "no dots - single word",
			code:     errors.ErrorCode("CATEGORY"),
			expected: "CATEGORY",
		},
		{
			name:     "one dot - two parts",
			code:     errors.ErrorCode("CATEGORY.SUBCATEGORY"),
			expected: "CATEGORY",
		},
		{
			name:     "multiple dots - many parts",
			code:     errors.ErrorCode("CATEGORY.SUB.SPECIFIC.EXTRA"),
			expected: "CATEGORY",
		},
		{
			name:     "leading dot",
			code:     errors.ErrorCode(".SUBCATEGORY.SPECIFIC"),
			expected: "",
		},
		{
			name:     "trailing dot",
			code:     errors.ErrorCode("CATEGORY.SUBCATEGORY."),
			expected: "CATEGORY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.Category())
		})
	}
}

func TestErrorCode_Subcategory_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{
			name:     "INPUT subcategory",
			code:     errors.ErrorCodeUserInputMissingFile,
			expected: "INPUT",
		},
		{
			name:     "PARSE subcategory",
			code:     errors.ErrorCodeWorkflowParseYAMLSyntax,
			expected: "PARSE",
		},
		{
			name:     "VALIDATION subcategory",
			code:     errors.ErrorCodeWorkflowValidationCycleDetected,
			expected: "VALIDATION",
		},
		{
			name:     "COMMAND subcategory",
			code:     errors.ErrorCodeExecutionCommandFailed,
			expected: "COMMAND",
		},
		{
			name:     "PARALLEL subcategory",
			code:     errors.ErrorCodeExecutionParallelPartialFailure,
			expected: "PARALLEL",
		},
		{
			name:     "IO subcategory",
			code:     errors.ErrorCodeSystemIOReadFailed,
			expected: "IO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.Subcategory())
		})
	}
}

func TestErrorCode_Subcategory_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{
			name:     "empty code",
			code:     errors.ErrorCode(""),
			expected: "",
		},
		{
			name:     "no dots - single word",
			code:     errors.ErrorCode("CATEGORY"),
			expected: "",
		},
		{
			name:     "one dot - two parts",
			code:     errors.ErrorCode("CATEGORY.SUBCATEGORY"),
			expected: "SUBCATEGORY",
		},
		{
			name:     "multiple dots - many parts",
			code:     errors.ErrorCode("CATEGORY.SUB.SPECIFIC.EXTRA"),
			expected: "SUB",
		},
		{
			name:     "leading dot",
			code:     errors.ErrorCode(".SUBCATEGORY.SPECIFIC"),
			expected: "SUBCATEGORY",
		},
		{
			name:     "middle empty part",
			code:     errors.ErrorCode("CATEGORY..SPECIFIC"),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.Subcategory())
		})
	}
}

func TestErrorCode_Specific_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{
			name:     "MISSING_FILE specific",
			code:     errors.ErrorCodeUserInputMissingFile,
			expected: "MISSING_FILE",
		},
		{
			name:     "INVALID_FORMAT specific",
			code:     errors.ErrorCodeUserInputInvalidFormat,
			expected: "INVALID_FORMAT",
		},
		{
			name:     "YAML_SYNTAX specific",
			code:     errors.ErrorCodeWorkflowParseYAMLSyntax,
			expected: "YAML_SYNTAX",
		},
		{
			name:     "CYCLE_DETECTED specific",
			code:     errors.ErrorCodeWorkflowValidationCycleDetected,
			expected: "CYCLE_DETECTED",
		},
		{
			name:     "FAILED specific",
			code:     errors.ErrorCodeExecutionCommandFailed,
			expected: "FAILED",
		},
		{
			name:     "PERMISSION_DENIED specific",
			code:     errors.ErrorCodeSystemIOPermissionDenied,
			expected: "PERMISSION_DENIED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.Specific())
		})
	}
}

func TestErrorCode_Specific_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{
			name:     "empty code",
			code:     errors.ErrorCode(""),
			expected: "",
		},
		{
			name:     "no dots - single word",
			code:     errors.ErrorCode("CATEGORY"),
			expected: "",
		},
		{
			name:     "one dot - two parts",
			code:     errors.ErrorCode("CATEGORY.SUBCATEGORY"),
			expected: "",
		},
		{
			name:     "two dots - three parts (valid)",
			code:     errors.ErrorCode("CATEGORY.SUBCATEGORY.SPECIFIC"),
			expected: "SPECIFIC",
		},
		{
			name:     "multiple dots - many parts",
			code:     errors.ErrorCode("CATEGORY.SUB.SPECIFIC.EXTRA"),
			expected: "SPECIFIC",
		},
		{
			name:     "trailing dot",
			code:     errors.ErrorCode("CATEGORY.SUBCATEGORY."),
			expected: "",
		},
		{
			name:     "last part empty",
			code:     errors.ErrorCode("CATEGORY.SUBCATEGORY.."),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.Specific())
		})
	}
}

func TestErrorCode_IsValid_HappyPath(t *testing.T) {
	tests := []struct {
		name string
		code errors.ErrorCode
	}{
		{
			name: "USER.INPUT.MISSING_FILE",
			code: errors.ErrorCodeUserInputMissingFile,
		},
		{
			name: "WORKFLOW.PARSE.YAML_SYNTAX",
			code: errors.ErrorCodeWorkflowParseYAMLSyntax,
		},
		{
			name: "EXECUTION.COMMAND.FAILED",
			code: errors.ErrorCodeExecutionCommandFailed,
		},
		{
			name: "SYSTEM.IO.READ_FAILED",
			code: errors.ErrorCodeSystemIOReadFailed,
		},
		{
			name: "custom valid code",
			code: errors.ErrorCode("CUSTOM.CATEGORY.ERROR"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.code.IsValid())
		})
	}
}

func TestErrorCode_IsValid_InvalidFormats(t *testing.T) {
	tests := []struct {
		name   string
		code   errors.ErrorCode
		reason string
	}{
		{
			name:   "empty string",
			code:   errors.ErrorCode(""),
			reason: "empty code should be invalid",
		},
		{
			name:   "no dots",
			code:   errors.ErrorCode("USERERROR"),
			reason: "missing hierarchy",
		},
		{
			name:   "one dot only",
			code:   errors.ErrorCode("USER.INPUT"),
			reason: "missing specific part",
		},
		{
			name:   "too many dots",
			code:   errors.ErrorCode("USER.INPUT.MISSING.FILE"),
			reason: "too many hierarchy levels",
		},
		{
			name:   "leading dot",
			code:   errors.ErrorCode(".INPUT.MISSING_FILE"),
			reason: "empty category",
		},
		{
			name:   "middle empty",
			code:   errors.ErrorCode("USER..MISSING_FILE"),
			reason: "empty subcategory",
		},
		{
			name:   "trailing dot",
			code:   errors.ErrorCode("USER.INPUT."),
			reason: "empty specific",
		},
		{
			name:   "all dots",
			code:   errors.ErrorCode(".."),
			reason: "all parts empty",
		},
		{
			name:   "single dot",
			code:   errors.ErrorCode("."),
			reason: "single separator only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, tt.code.IsValid(), tt.reason)
		})
	}
}

func TestErrorCode_ExitCode_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected int
	}{
		{
			name:     "USER category maps to exit code 1",
			code:     errors.ErrorCodeUserInputMissingFile,
			expected: 1,
		},
		{
			name:     "WORKFLOW category maps to exit code 2",
			code:     errors.ErrorCodeWorkflowParseYAMLSyntax,
			expected: 2,
		},
		{
			name:     "EXECUTION category maps to exit code 3",
			code:     errors.ErrorCodeExecutionCommandFailed,
			expected: 3,
		},
		{
			name:     "SYSTEM category maps to exit code 4",
			code:     errors.ErrorCodeSystemIOReadFailed,
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.ExitCode())
		})
	}
}

func TestErrorCode_ExitCode_AllConstants(t *testing.T) {
	// Verify all USER codes map to exit code 1
	userCodes := []errors.ErrorCode{
		errors.ErrorCodeUserInputMissingFile,
		errors.ErrorCodeUserInputInvalidFormat,
		errors.ErrorCodeUserInputValidationFailed,
	}
	for _, code := range userCodes {
		t.Run(string(code), func(t *testing.T) {
			assert.Equal(t, 1, code.ExitCode(), "USER category should map to exit code 1")
		})
	}

	// Verify all WORKFLOW codes map to exit code 2
	workflowCodes := []errors.ErrorCode{
		errors.ErrorCodeWorkflowParseYAMLSyntax,
		errors.ErrorCodeWorkflowParseUnknownField,
		errors.ErrorCodeWorkflowValidationCycleDetected,
		errors.ErrorCodeWorkflowValidationMissingState,
		errors.ErrorCodeWorkflowValidationInvalidTransition,
	}
	for _, code := range workflowCodes {
		t.Run(string(code), func(t *testing.T) {
			assert.Equal(t, 2, code.ExitCode(), "WORKFLOW category should map to exit code 2")
		})
	}

	// Verify all EXECUTION codes map to exit code 3
	executionCodes := []errors.ErrorCode{
		errors.ErrorCodeExecutionCommandFailed,
		errors.ErrorCodeExecutionCommandTimeout,
		errors.ErrorCodeExecutionParallelPartialFailure,
	}
	for _, code := range executionCodes {
		t.Run(string(code), func(t *testing.T) {
			assert.Equal(t, 3, code.ExitCode(), "EXECUTION category should map to exit code 3")
		})
	}

	// Verify all SYSTEM codes map to exit code 4
	systemCodes := []errors.ErrorCode{
		errors.ErrorCodeSystemIOReadFailed,
		errors.ErrorCodeSystemIOWriteFailed,
		errors.ErrorCodeSystemIOPermissionDenied,
	}
	for _, code := range systemCodes {
		t.Run(string(code), func(t *testing.T) {
			assert.Equal(t, 4, code.ExitCode(), "SYSTEM category should map to exit code 4")
		})
	}
}

func TestErrorCode_ExitCode_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected int
		reason   string
	}{
		{
			name:     "empty code defaults to 1",
			code:     errors.ErrorCode(""),
			expected: 1,
			reason:   "empty category defaults to user error",
		},
		{
			name:     "invalid format defaults to 1",
			code:     errors.ErrorCode("INVALID"),
			expected: 1,
			reason:   "single word defaults to user error",
		},
		{
			name:     "unknown category defaults to 1",
			code:     errors.ErrorCode("UNKNOWN.SUB.SPECIFIC"),
			expected: 1,
			reason:   "unrecognized category defaults to user error",
		},
		{
			name:     "lowercase category defaults to 1",
			code:     errors.ErrorCode("user.input.missing"),
			expected: 1,
			reason:   "case-sensitive category check, 'user' != 'USER'",
		},
		{
			name:     "mixed case category defaults to 1",
			code:     errors.ErrorCode("User.Input.Missing"),
			expected: 1,
			reason:   "case-sensitive category check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.ExitCode(), tt.reason)
		})
	}
}

func TestErrorCode_Taxonomy_Coverage(t *testing.T) {
	// Verify taxonomy covers all categories from spec
	categories := map[string][]errors.ErrorCode{
		"USER": {
			errors.ErrorCodeUserInputMissingFile,
			errors.ErrorCodeUserInputInvalidFormat,
			errors.ErrorCodeUserInputValidationFailed,
		},
		"WORKFLOW": {
			errors.ErrorCodeWorkflowParseYAMLSyntax,
			errors.ErrorCodeWorkflowParseUnknownField,
			errors.ErrorCodeWorkflowValidationCycleDetected,
			errors.ErrorCodeWorkflowValidationMissingState,
			errors.ErrorCodeWorkflowValidationInvalidTransition,
		},
		"EXECUTION": {
			errors.ErrorCodeExecutionCommandFailed,
			errors.ErrorCodeExecutionCommandTimeout,
			errors.ErrorCodeExecutionParallelPartialFailure,
		},
		"SYSTEM": {
			errors.ErrorCodeSystemIOReadFailed,
			errors.ErrorCodeSystemIOWriteFailed,
			errors.ErrorCodeSystemIOPermissionDenied,
		},
	}

	t.Run("all categories have codes", func(t *testing.T) {
		assert.Len(t, categories, 4, "should have exactly 4 categories")
		for category, codes := range categories {
			assert.NotEmpty(t, codes, "category %s should have at least one code", category)
		}
	})

	t.Run("all codes belong to correct category", func(t *testing.T) {
		for expectedCategory, codes := range categories {
			for _, code := range codes {
				assert.Equal(t, expectedCategory, code.Category(),
					"code %s should belong to category %s", code, expectedCategory)
			}
		}
	})
}

func TestErrorCode_Taxonomy_Subcategories(t *testing.T) {
	// Verify expected subcategories exist
	subcategoryTests := []struct {
		category    string
		subcategory string
		codes       []errors.ErrorCode
	}{
		{
			category:    "USER",
			subcategory: "INPUT",
			codes: []errors.ErrorCode{
				errors.ErrorCodeUserInputMissingFile,
				errors.ErrorCodeUserInputInvalidFormat,
				errors.ErrorCodeUserInputValidationFailed,
			},
		},
		{
			category:    "WORKFLOW",
			subcategory: "PARSE",
			codes: []errors.ErrorCode{
				errors.ErrorCodeWorkflowParseYAMLSyntax,
				errors.ErrorCodeWorkflowParseUnknownField,
			},
		},
		{
			category:    "WORKFLOW",
			subcategory: "VALIDATION",
			codes: []errors.ErrorCode{
				errors.ErrorCodeWorkflowValidationCycleDetected,
				errors.ErrorCodeWorkflowValidationMissingState,
				errors.ErrorCodeWorkflowValidationInvalidTransition,
			},
		},
		{
			category:    "EXECUTION",
			subcategory: "COMMAND",
			codes: []errors.ErrorCode{
				errors.ErrorCodeExecutionCommandFailed,
				errors.ErrorCodeExecutionCommandTimeout,
			},
		},
		{
			category:    "EXECUTION",
			subcategory: "PARALLEL",
			codes: []errors.ErrorCode{
				errors.ErrorCodeExecutionParallelPartialFailure,
			},
		},
		{
			category:    "SYSTEM",
			subcategory: "IO",
			codes: []errors.ErrorCode{
				errors.ErrorCodeSystemIOReadFailed,
				errors.ErrorCodeSystemIOWriteFailed,
				errors.ErrorCodeSystemIOPermissionDenied,
			},
		},
	}

	for _, tt := range subcategoryTests {
		t.Run(tt.category+"."+tt.subcategory, func(t *testing.T) {
			for _, code := range tt.codes {
				assert.Equal(t, tt.category, code.Category(),
					"code %s should have category %s", code, tt.category)
				assert.Equal(t, tt.subcategory, code.Subcategory(),
					"code %s should have subcategory %s", code, tt.subcategory)
			}
		})
	}
}
