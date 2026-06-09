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

	// ErrorCodeUserInputMissingSkill indicates a required skill was not found.
	ErrorCodeUserInputMissingSkill ErrorCode = "USER.INPUT.MISSING_SKILL"

	// ErrorCodeUserInputMissingRole indicates a required agent role was not found.
	ErrorCodeUserInputMissingRole ErrorCode = "USER.INPUT.MISSING_ROLE"
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

	// ErrorCodeExecutionEventDeliveryFailed indicates event delivery to a subscriber failed.
	ErrorCodeExecutionEventDeliveryFailed ErrorCode = "EXECUTION.EVENT.DELIVERY_FAILED"

	// ErrorCodeExecutionEventCycleDetected indicates a publish loop was detected in event routing.
	ErrorCodeExecutionEventCycleDetected ErrorCode = "EXECUTION.EVENT.CYCLE_DETECTED"

	// ErrorCodeExecutionEventBufferFull indicates the event buffer capacity was exceeded.
	ErrorCodeExecutionEventBufferFull ErrorCode = "EXECUTION.EVENT.BUFFER_FULL"

	// ErrorCodeExecutionPluginChecksumMismatch indicates plugin binary checksum verification failed.
	ErrorCodeExecutionPluginChecksumMismatch ErrorCode = "EXECUTION.PLUGIN.CHECKSUM_MISMATCH"

	// ErrorCodeExecutionPluginBrokerEmitDenied indicates a plugin attempted to emit an event it is not authorized to emit.
	ErrorCodeExecutionPluginBrokerEmitDenied ErrorCode = "EXECUTION.PLUGIN.BROKER_EMIT_DENIED"

	// ErrorCodeExecutionPluginStreamSetupFailed indicates a streaming connection to a plugin could not be established.
	ErrorCodeExecutionPluginStreamSetupFailed ErrorCode = "EXECUTION.PLUGIN.STREAM_SETUP_FAILED"
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

// Error code constants for USER.MCP_PROXY category (exit code 1).
const (
	// ErrorCodeUserMCPProxyUnknownKey indicates the MCP proxy configuration contains an unrecognized key.
	ErrorCodeUserMCPProxyUnknownKey ErrorCode = "USER.MCP_PROXY.UNKNOWN_KEY"

	// ErrorCodeUserMCPProxyUnknownPlugin indicates a plugin referenced by mcp_proxy.plugin_tools is not installed or enabled.
	ErrorCodeUserMCPProxyUnknownPlugin ErrorCode = "USER.MCP_PROXY.UNKNOWN_PLUGIN"

	// ErrorCodeUserMCPProxyUnknownOperation indicates an exposed operation name is not provided by the referenced plugin.
	ErrorCodeUserMCPProxyUnknownOperation ErrorCode = "USER.MCP_PROXY.UNKNOWN_OPERATION"

	// ErrorCodeUserMCPProxyNameCollision indicates two exposed tools resolve to the same MCP tool name.
	ErrorCodeUserMCPProxyNameCollision ErrorCode = "USER.MCP_PROXY.NAME_COLLISION"

	// ErrorCodeUserMCPProxyEmptyProxy indicates mcp_proxy is enabled but exposes neither built-ins nor plugin tools.
	ErrorCodeUserMCPProxyEmptyProxy ErrorCode = "USER.MCP_PROXY.EMPTY_PROXY"

	// ErrorCodeUserMCPProxyUnsupportedProvider indicates the active agent provider does not support MCP tool interception.
	ErrorCodeUserMCPProxyUnsupportedProvider ErrorCode = "USER.MCP_PROXY.UNSUPPORTED_PROVIDER"

	// ErrorCodeUserMCPProxyInfiniteLoopGuard indicates the tool-call loop ended with finish_reason="tool_calls" but no tool calls were emitted.
	ErrorCodeUserMCPProxyInfiniteLoopGuard ErrorCode = "USER.MCP_PROXY.INFINITE_LOOP_GUARD"
)

// Error code constants for USER.FACADE category (exit code 1).
// Facade interface resolution errors declared by T055; consumed by T054 Resolver.
const (
	// ErrorCodeUserFacadeIdentifierEmpty indicates the facade identifier provided by the caller is empty.
	ErrorCodeUserFacadeIdentifierEmpty ErrorCode = "USER.FACADE.IDENTIFIER_EMPTY"

	// ErrorCodeUserFacadeIdentifierMalformed indicates the facade identifier does not follow the expected format.
	ErrorCodeUserFacadeIdentifierMalformed ErrorCode = "USER.FACADE.IDENTIFIER_MALFORMED"

	// ErrorCodeUserFacadePackNotFound indicates no pack matching the requested identifier could be located.
	ErrorCodeUserFacadePackNotFound ErrorCode = "USER.FACADE.PACK_NOT_FOUND"

	// ErrorCodeUserFacadeWorkflowNotFound indicates no workflow matching the requested identifier exists in the resolved pack.
	ErrorCodeUserFacadeWorkflowNotFound ErrorCode = "USER.FACADE.WORKFLOW_NOT_FOUND"

	// ErrorCodeUserFacadeSessionClosed indicates the target facade session has already been closed and cannot accept further operations.
	ErrorCodeUserFacadeSessionClosed ErrorCode = "USER.FACADE.SESSION_CLOSED"

	// ErrorCodeUserFacadeInputRejected indicates the input supplied to the facade was rejected by validation.
	ErrorCodeUserFacadeInputRejected ErrorCode = "USER.FACADE.INPUT_REJECTED"

	// ErrorCodeUserFacadeDuplicateResponse indicates a response was submitted for a facade request that has already received a response.
	ErrorCodeUserFacadeDuplicateResponse ErrorCode = "USER.FACADE.DUPLICATE_RESPONSE"
)

// Error code constants for USER.ACP category (exit code 1).
// ACP-specific codes (F102).
const (
	ErrorCodeUserACPInvalidPrompt              ErrorCode = "USER.ACP.INVALID_PROMPT"
	ErrorCodeUserACPUnsupportedBlock           ErrorCode = "USER.ACP.UNSUPPORTED_BLOCK"
	ErrorCodeUserACPPromptInFlight             ErrorCode = "USER.ACP.PROMPT_IN_FLIGHT"
	ErrorCodeUserACPUnknownSession             ErrorCode = "USER.ACP.UNKNOWN_SESSION"
	ErrorCodeUserACPProtocolVersionUnsupported ErrorCode = "USER.ACP.PROTOCOL_VERSION_UNSUPPORTED"
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

// Error code constants for SYSTEM.INTERNAL category (exit code 4).
// Sentinel returned by the application-layer MapError when no mapping case covers the variant;
// prevents silent failures while keeping the mapping closed (fail-closed pattern, NFR-005).
const (
	ErrorCodeSystemInternalUnmapped ErrorCode = "SYSTEM.INTERNAL.UNMAPPED"
)

// Category extracts the top-level category from the error code.
// Returns the first dot-separated segment; returns empty string only when the
// code itself is empty or starts with a dot.
//
// Examples:
//   - "USER.INPUT.MISSING_FILE" → "USER"
//   - "WORKFLOW.PARSE.YAML_SYNTAX" → "WORKFLOW"
//   - "INVALID" → "INVALID"
//   - "" → ""
//   - ".INPUT.MISSING_FILE" → ""
func (ec ErrorCode) Category() string {
	parts := strings.SplitN(string(ec), ".", 2)
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
