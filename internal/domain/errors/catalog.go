//nolint:revive // Package name "errors" is intentional; fully qualified import path avoids stdlib conflict
package errors

// CatalogEntry describes an error code with human-readable documentation,
// resolution guidance, and related error codes.
type CatalogEntry struct {
	// Code is the hierarchical error identifier.
	Code ErrorCode

	// Description explains what this error means in user-friendly terms.
	Description string

	// Resolution provides actionable guidance on how to fix the error.
	Resolution string

	// RelatedCodes lists other error codes commonly encountered with this error.
	RelatedCodes []ErrorCode
}

// ErrorCatalog maps error codes to their documentation entries.
// Used by the `awf error <code>` CLI command for error code lookup.
var ErrorCatalog = map[ErrorCode]CatalogEntry{
	ErrorCodeUserInputMissingFile: {
		Code:         ErrorCodeUserInputMissingFile,
		Description:  "The specified file was not found at the given path.",
		Resolution:   "Verify the file path is correct and the file exists. Check for typos in the filename or path.",
		RelatedCodes: []ErrorCode{ErrorCodeUserInputInvalidFormat, ErrorCodeSystemIOReadFailed},
	},
	ErrorCodeUserInputInvalidFormat: {
		Code:         ErrorCodeUserInputInvalidFormat,
		Description:  "The file format does not match expected structure or contains invalid syntax.",
		Resolution:   "Check the file format against the documentation. Ensure YAML syntax is valid if applicable.",
		RelatedCodes: []ErrorCode{ErrorCodeWorkflowParseYAMLSyntax, ErrorCodeUserInputValidationFailed},
	},
	ErrorCodeUserInputValidationFailed: {
		Code:         ErrorCodeUserInputValidationFailed,
		Description:  "Input parameter validation failed due to invalid or missing required values.",
		Resolution:   "Review the command-line arguments and flags. Use --help for usage information.",
		RelatedCodes: []ErrorCode{ErrorCodeUserInputMissingFile, ErrorCodeUserInputInvalidFormat},
	},

	ErrorCodeWorkflowParseYAMLSyntax: {
		Code:         ErrorCodeWorkflowParseYAMLSyntax,
		Description:  "YAML parsing error due to syntax violation or malformed structure.",
		Resolution:   "Validate YAML syntax using a YAML linter. Check for indentation errors, missing colons, or invalid characters.",
		RelatedCodes: []ErrorCode{ErrorCodeWorkflowParseUnknownField, ErrorCodeUserInputInvalidFormat},
	},
	ErrorCodeWorkflowParseUnknownField: {
		Code:         ErrorCodeWorkflowParseUnknownField,
		Description:  "The workflow definition contains an unrecognized field name.",
		Resolution:   "Check the workflow schema documentation. Remove or rename the unrecognized field.",
		RelatedCodes: []ErrorCode{ErrorCodeWorkflowParseYAMLSyntax},
	},
	ErrorCodeWorkflowValidationCycleDetected: {
		Code:         ErrorCodeWorkflowValidationCycleDetected,
		Description:  "A cycle was detected in the workflow state machine transitions.",
		Resolution:   "Review state transitions to identify and break the cycle. Ensure all paths lead to a terminal state.",
		RelatedCodes: []ErrorCode{ErrorCodeWorkflowValidationInvalidTransition, ErrorCodeWorkflowValidationMissingState},
	},
	ErrorCodeWorkflowValidationMissingState: {
		Code:         ErrorCodeWorkflowValidationMissingState,
		Description:  "A state referenced in a transition does not exist in the workflow definition.",
		Resolution:   "Add the missing state definition or update the transition to reference an existing state.",
		RelatedCodes: []ErrorCode{ErrorCodeWorkflowValidationCycleDetected, ErrorCodeWorkflowValidationInvalidTransition},
	},
	ErrorCodeWorkflowValidationInvalidTransition: {
		Code:         ErrorCodeWorkflowValidationInvalidTransition,
		Description:  "A transition rule is malformed or violates state machine constraints.",
		Resolution:   "Verify transition syntax. Check that source and target states are valid and transition logic is correct.",
		RelatedCodes: []ErrorCode{ErrorCodeWorkflowValidationMissingState, ErrorCodeWorkflowValidationCycleDetected},
	},

	ErrorCodeExecutionCommandFailed: {
		Code:         ErrorCodeExecutionCommandFailed,
		Description:  "A shell command executed during workflow execution exited with a non-zero status code.",
		Resolution:   "Check command output for error details. Verify the command syntax and required dependencies are installed.",
		RelatedCodes: []ErrorCode{ErrorCodeExecutionCommandTimeout, ErrorCodeSystemIOPermissionDenied},
	},
	ErrorCodeExecutionCommandTimeout: {
		Code:         ErrorCodeExecutionCommandTimeout,
		Description:  "A command execution exceeded the configured timeout duration.",
		Resolution:   "Increase the timeout value if the operation is expected to take longer, or optimize the command for faster execution.",
		RelatedCodes: []ErrorCode{ErrorCodeExecutionCommandFailed},
	},
	ErrorCodeExecutionParallelPartialFailure: {
		Code:         ErrorCodeExecutionParallelPartialFailure,
		Description:  "Some branches in a parallel execution block failed while others succeeded.",
		Resolution:   "Review logs for failed branches. Fix underlying issues in failed steps or adjust parallel strategy.",
		RelatedCodes: []ErrorCode{ErrorCodeExecutionCommandFailed, ErrorCodeExecutionCommandTimeout},
	},

	ErrorCodeSystemIOReadFailed: {
		Code:         ErrorCodeSystemIOReadFailed,
		Description:  "An I/O error occurred while attempting to read from a file or stream.",
		Resolution:   "Check file permissions, disk space, and file system health. Verify the file is not locked by another process.",
		RelatedCodes: []ErrorCode{ErrorCodeSystemIOPermissionDenied, ErrorCodeUserInputMissingFile},
	},
	ErrorCodeSystemIOWriteFailed: {
		Code:         ErrorCodeSystemIOWriteFailed,
		Description:  "An I/O error occurred while attempting to write to a file or stream.",
		Resolution:   "Check available disk space and write permissions. Verify the target directory exists and is writable.",
		RelatedCodes: []ErrorCode{ErrorCodeSystemIOPermissionDenied},
	},
	ErrorCodeSystemIOPermissionDenied: {
		Code:         ErrorCodeSystemIOPermissionDenied,
		Description:  "Insufficient permissions to access the requested file or directory.",
		Resolution:   "Check file permissions with ls -l. Use chmod to grant necessary permissions or run with appropriate user privileges.",
		RelatedCodes: []ErrorCode{ErrorCodeSystemIOReadFailed, ErrorCodeSystemIOWriteFailed},
	},
}

// GetCatalogEntry retrieves the catalog entry for the given error code.
// Returns the entry and true if found, or an empty entry and false if not found.
func GetCatalogEntry(code ErrorCode) (CatalogEntry, bool) {
	entry, found := ErrorCatalog[code]
	return entry, found
}

// AllErrorCodes returns a sorted list of all defined error codes.
// Used by the `awf error` command to list all available error codes.
func AllErrorCodes() []ErrorCode {
	codes := make([]ErrorCode, 0, len(ErrorCatalog))
	for code := range ErrorCatalog {
		codes = append(codes, code)
	}
	return codes
}
