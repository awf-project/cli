// Package errors provides structured error handling with hierarchical error codes.
//
// This package defines the error taxonomy for AWF CLI, enabling machine-readable
// error identification, consistent error formatting, and programmatic error handling.
// It follows hexagonal architecture principles with zero infrastructure dependencies.
//
// # Architecture Role
//
// In the hexagonal architecture pattern:
//   - Domain layer defines error types and codes (this package)
//   - ErrorFormatter port interface for presentation concerns (domain/ports)
//   - Infrastructure layer implements formatters as adapters
//   - All layers use StructuredError for consistent error taxonomy
//
// # Core Types
//
// ## Error Codes (codes.go)
//
// Hierarchical error identifier system:
//   - ErrorCode: String type representing CATEGORY.SUBCATEGORY.SPECIFIC codes
//   - Category(): Extract top-level category (USER, WORKFLOW, EXECUTION, SYSTEM)
//   - Subcategory(): Extract middle classification (INPUT, VALIDATION, COMMAND, etc.)
//   - Specific(): Extract granular error identifier
//
// Error code categories map to exit codes:
//   - USER.* → exit code 1 (bad input, missing file)
//   - WORKFLOW.* → exit code 2 (invalid state, cycles)
//   - EXECUTION.* → exit code 3 (command failed, timeout)
//   - SYSTEM.* → exit code 4 (I/O errors, permissions)
//
// ## Structured Error (structured_error.go)
//
// Cross-cutting error type with taxonomy support:
//   - StructuredError: Error with Code, Message, Details, Cause, Timestamp
//   - Error(): Implements standard error interface
//   - Unwrap(): Supports error wrapping and errors.Is/As chains
//   - ExitCode(): Map error code to process exit code
//
// ## Error Catalog (catalog.go)
//
// Error code documentation and resolution hints:
//   - ErrorInfo: Description, resolution guidance, related codes
//   - GetErrorInfo(): Lookup error metadata by code
//   - ListErrorCodes(): Enumerate all defined error codes
//
// # Error Code Taxonomy
//
// ## USER Errors (exit code 1)
//
// User-facing input and configuration errors:
//   - USER.INPUT.MISSING_FILE: Required file not found
//   - USER.INPUT.INVALID_FORMAT: File format validation failed
//   - USER.INPUT.VALIDATION_FAILED: Input parameter validation error
//
// ## WORKFLOW Errors (exit code 2)
//
// Workflow definition and validation errors:
//   - WORKFLOW.PARSE.YAML_SYNTAX: YAML parsing error
//   - WORKFLOW.PARSE.UNKNOWN_FIELD: Unrecognized YAML field
//   - WORKFLOW.VALIDATION.CYCLE_DETECTED: State machine cycle detected
//   - WORKFLOW.VALIDATION.MISSING_STATE: Referenced state not defined
//   - WORKFLOW.VALIDATION.INVALID_TRANSITION: Malformed transition rule
//
// ## EXECUTION Errors (exit code 3)
//
// Runtime execution failures:
//   - EXECUTION.COMMAND.FAILED: Shell command exited non-zero
//   - EXECUTION.COMMAND.TIMEOUT: Command exceeded timeout
//   - EXECUTION.PARALLEL.PARTIAL_FAILURE: Some parallel branches failed
//
// ## SYSTEM Errors (exit code 4)
//
// Infrastructure and system-level failures:
//   - SYSTEM.IO.READ_FAILED: File read error
//   - SYSTEM.IO.WRITE_FAILED: File write error
//   - SYSTEM.IO.PERMISSION_DENIED: Insufficient permissions
//
// # Constructor Functions
//
// Convenience constructors for common patterns:
//
//	// Create with error code and message
//	err := errors.NewStructuredError(
//	    errors.ErrorCodeUserInputMissingFile,
//	    "workflow.yaml not found",
//	)
//
//	// Wrap existing error with code
//	err := errors.WrapError(
//	    cause,
//	    errors.ErrorCodeSystemIOReadFailed,
//	    "failed to read config",
//	)
//
//	// Add contextual details
//	err := err.WithDetails(map[string]any{
//	    "path":      "/path/to/file",
//	    "operation": "stat",
//	})
//
// # Usage Examples
//
// ## Creating Structured Errors
//
// Basic error with code and message:
//
//	func LoadWorkflow(path string) (*Workflow, error) {
//	    if _, err := os.Stat(path); os.IsNotExist(err) {
//	        return nil, errors.NewStructuredError(
//	            errors.ErrorCodeUserInputMissingFile,
//	            fmt.Sprintf("workflow file not found: %s", path),
//	        )
//	    }
//	    // ...
//	}
//
// ## Wrapping Errors
//
// Preserve cause while adding structure:
//
//	func SaveState(ctx *ExecutionContext) error {
//	    data, err := json.Marshal(ctx)
//	    if err != nil {
//	        return errors.WrapError(
//	            err,
//	            errors.ErrorCodeSystemIOWriteFailed,
//	            "failed to serialize state",
//	        )
//	    }
//	    // ...
//	}
//
// ## Adding Details
//
// Include debugging context:
//
//	err := errors.NewStructuredError(
//	    errors.ErrorCodeExecutionCommandFailed,
//	    "build command failed",
//	).WithDetails(map[string]any{
//	    "command":   "go build",
//	    "exit_code": 2,
//	    "step":      "compile",
//	})
//
// ## Error Detection
//
// Check for specific error codes:
//
//	if se, ok := err.(*errors.StructuredError); ok {
//	    if se.Code.Category() == "USER" {
//	        // User-facing error message
//	    }
//	}
//
// ## Exit Code Mapping
//
// Convert error to process exit code:
//
//	func main() {
//	    if err := run(); err != nil {
//	        if se, ok := err.(*errors.StructuredError); ok {
//	            os.Exit(se.ExitCode())
//	        }
//	        os.Exit(1)
//	    }
//	}
//
// # Design Principles
//
// ## Hierarchical Structure
//
// Three-level taxonomy enables:
//   - Broad categorization by error source (category)
//   - Mid-level grouping by error type (subcategory)
//   - Precise identification (specific)
//
// Example: USER.INPUT.MISSING_FILE
//   - Category: USER (user-facing error)
//   - Subcategory: INPUT (input validation)
//   - Specific: MISSING_FILE (file not found)
//
// ## Machine-Readable
//
// Error codes enable:
//   - Programmatic error handling in CI/CD
//   - Searchable error documentation
//   - Telemetry and error tracking
//   - Consistent error messages across formats
//
// ## Backward Compatibility
//
// During migration:
//   - Exit codes preserved (1-4 mapping unchanged)
//   - String-based error detection still works
//   - Gradual migration without breaking changes
//
// ## Pure Domain
//
// Zero infrastructure dependencies:
//   - stdlib only (errors, fmt, time, strings)
//   - No file I/O, HTTP, or external systems
//   - Presentation delegated to ErrorFormatter port
//
// # Related Packages
//
//   - internal/domain/ports: ErrorFormatter port interface
//   - internal/domain/workflow: ValidationError for workflow validation
//   - internal/infrastructure/errors: Concrete formatter implementations
//   - internal/interfaces/cli: Error command and output integration
//
//nolint:revive // Package name "errors" is intentional; fully qualified import path avoids stdlib conflict
package errors
