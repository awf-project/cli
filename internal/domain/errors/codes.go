//nolint:revive // Package name "errors" is intentional; fully qualified import path avoids stdlib conflict
package errors

import "strings"

// ErrorCode represents a hierarchical error identifier in CATEGORY.SUBCATEGORY.SPECIFIC format.
// Error codes enable machine-readable error handling, consistent exit code mapping,
// and programmatic error detection across all architectural layers.
//
// Format: CATEGORY.SUBCATEGORY.SPECIFIC
//   - CATEGORY: Top-level error classification (USER, WORKFLOW, EXECUTION, SYSTEM)
//   - SUBCATEGORY: Mid-level grouping by error type (INPUT, VALIDATION, COMMAND, IO)
//   - SPECIFIC: Precise error identifier (MISSING_FILE, CYCLE_DETECTED, etc.)
//
// Example: USER.INPUT.MISSING_FILE
//   - Category(): "USER"
//   - Subcategory(): "INPUT"
//   - Specific(): "MISSING_FILE"
//
// Error codes map to process exit codes:
//   - USER.* → exit code 1 (user-facing errors)
//   - WORKFLOW.* → exit code 2 (workflow definition errors)
//   - EXECUTION.* → exit code 3 (runtime execution errors)
//   - SYSTEM.* → exit code 4 (infrastructure errors)
type ErrorCode string

// Error code constants for USER category (exit code 1).
// User-facing input and configuration errors.
const (
	// ErrorCodeUserInputMissingFile indicates a required file was not found.
	ErrorCodeUserInputMissingFile ErrorCode = "USER.INPUT.MISSING_FILE"

	// ErrorCodeUserInputInvalidFormat indicates file format validation failed.
	ErrorCodeUserInputInvalidFormat ErrorCode = "USER.INPUT.INVALID_FORMAT"

	// ErrorCodeUserInputValidationFailed indicates input parameter validation error.
	ErrorCodeUserInputValidationFailed ErrorCode = "USER.INPUT.VALIDATION_FAILED"
)

// Error code constants for WORKFLOW category (exit code 2).
// Workflow definition parsing and validation errors.
const (
	// ErrorCodeWorkflowParseYAMLSyntax indicates YAML parsing error.
	ErrorCodeWorkflowParseYAMLSyntax ErrorCode = "WORKFLOW.PARSE.YAML_SYNTAX"

	// ErrorCodeWorkflowParseUnknownField indicates an unrecognized YAML field.
	ErrorCodeWorkflowParseUnknownField ErrorCode = "WORKFLOW.PARSE.UNKNOWN_FIELD"

	// ErrorCodeWorkflowValidationCycleDetected indicates a state machine cycle.
	ErrorCodeWorkflowValidationCycleDetected ErrorCode = "WORKFLOW.VALIDATION.CYCLE_DETECTED"

	// ErrorCodeWorkflowValidationMissingState indicates a referenced state is not defined.
	ErrorCodeWorkflowValidationMissingState ErrorCode = "WORKFLOW.VALIDATION.MISSING_STATE"

	// ErrorCodeWorkflowValidationInvalidTransition indicates a malformed transition rule.
	ErrorCodeWorkflowValidationInvalidTransition ErrorCode = "WORKFLOW.VALIDATION.INVALID_TRANSITION"
)

// Error code constants for EXECUTION category (exit code 3).
// Runtime execution failures during workflow execution.
const (
	// ErrorCodeExecutionCommandFailed indicates shell command exited non-zero.
	ErrorCodeExecutionCommandFailed ErrorCode = "EXECUTION.COMMAND.FAILED"

	// ErrorCodeExecutionCommandTimeout indicates command exceeded timeout.
	ErrorCodeExecutionCommandTimeout ErrorCode = "EXECUTION.COMMAND.TIMEOUT"

	// ErrorCodeExecutionParallelPartialFailure indicates some parallel branches failed.
	ErrorCodeExecutionParallelPartialFailure ErrorCode = "EXECUTION.PARALLEL.PARTIAL_FAILURE"

	// ErrorCodeExecutionPluginDisabled indicates an operation references a disabled plugin.
	ErrorCodeExecutionPluginDisabled ErrorCode = "EXECUTION.PLUGIN.DISABLED"
)

// Error code constants for SYSTEM category (exit code 4).
// Infrastructure and system-level failures.
const (
	// ErrorCodeSystemIOReadFailed indicates file read error.
	ErrorCodeSystemIOReadFailed ErrorCode = "SYSTEM.IO.READ_FAILED"

	// ErrorCodeSystemIOWriteFailed indicates file write error.
	ErrorCodeSystemIOWriteFailed ErrorCode = "SYSTEM.IO.WRITE_FAILED"

	// ErrorCodeSystemIOPermissionDenied indicates insufficient permissions.
	ErrorCodeSystemIOPermissionDenied ErrorCode = "SYSTEM.IO.PERMISSION_DENIED"
)

// Error code constants for USER.UPGRADE category (exit code 1).
const (
	// ErrorCodeUserUpgradeVersionNotFound indicates the requested version was not found in releases.
	ErrorCodeUserUpgradeVersionNotFound ErrorCode = "USER.UPGRADE.VERSION_NOT_FOUND"

	// ErrorCodeUserUpgradeAlreadyLatest indicates the binary is already up to date.
	ErrorCodeUserUpgradeAlreadyLatest ErrorCode = "USER.UPGRADE.ALREADY_LATEST"
)

// Error code constants for SYSTEM.UPGRADE category (exit code 4).
const (
	// ErrorCodeSystemUpgradeChecksumMismatch indicates SHA256 checksum verification failed.
	ErrorCodeSystemUpgradeChecksumMismatch ErrorCode = "SYSTEM.UPGRADE.CHECKSUM_MISMATCH"

	// ErrorCodeSystemUpgradeBinaryReplaceFailed indicates the binary replacement failed.
	ErrorCodeSystemUpgradeBinaryReplaceFailed ErrorCode = "SYSTEM.UPGRADE.BINARY_REPLACE_FAILED"

	// ErrorCodeSystemUpgradeDownloadFailed indicates the release download failed.
	ErrorCodeSystemUpgradeDownloadFailed ErrorCode = "SYSTEM.UPGRADE.DOWNLOAD_FAILED"
)

// Category extracts the top-level category from the error code.
// Returns empty string if the code format is invalid.
//
// Examples:
//   - "USER.INPUT.MISSING_FILE" → "USER"
//   - "WORKFLOW.PARSE.YAML_SYNTAX" → "WORKFLOW"
//   - "INVALID" → ""
func (ec ErrorCode) Category() string {
	parts := strings.SplitN(string(ec), ".", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// Subcategory extracts the middle classification from the error code.
// Returns empty string if the code format is invalid.
//
// Examples:
//   - "USER.INPUT.MISSING_FILE" → "INPUT"
//   - "WORKFLOW.PARSE.YAML_SYNTAX" → "PARSE"
//   - "USER" → ""
func (ec ErrorCode) Subcategory() string {
	parts := strings.Split(string(ec), ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// Specific extracts the granular error identifier from the error code.
// Returns empty string if the code format is invalid.
//
// Examples:
//   - "USER.INPUT.MISSING_FILE" → "MISSING_FILE"
//   - "WORKFLOW.PARSE.YAML_SYNTAX" → "YAML_SYNTAX"
//   - "USER.INPUT" → ""
func (ec ErrorCode) Specific() string {
	parts := strings.Split(string(ec), ".")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}

// IsValid checks if the error code follows the required CATEGORY.SUBCATEGORY.SPECIFIC format.
// Returns true if the code has exactly three dot-separated parts, false otherwise.
func (ec ErrorCode) IsValid() bool {
	if ec == "" {
		return false
	}
	parts := strings.Split(string(ec), ".")
	return len(parts) == 3 && parts[0] != "" && parts[1] != "" && parts[2] != ""
}

// ExitCode maps the error code category to the corresponding process exit code.
// Returns:
//   - 1 for USER.* errors
//   - 2 for WORKFLOW.* errors
//   - 3 for EXECUTION.* errors
//   - 4 for SYSTEM.* errors
//   - 1 for invalid or unrecognized categories (default to user error)
func (ec ErrorCode) ExitCode() int {
	category := ec.Category()
	switch category {
	case "USER":
		return 1
	case "WORKFLOW":
		return 2
	case "EXECUTION":
		return 3
	case "SYSTEM":
		return 4
	default:
		return 1 // Default to user error exit code
	}
}
