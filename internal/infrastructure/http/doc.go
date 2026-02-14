// Package http implements infrastructure adapters for HTTP request operations.
//
// The http package provides a concrete implementation of the OperationProvider port
// defined in the domain layer, enabling workflow steps to perform declarative HTTP
// requests (GET, POST, PUT, DELETE) without shell scripting or curl commands. The
// provider uses Go's net/http client with configurable timeouts and retry support
// for transient failures.
//
// # Architecture Role
//
// In the hexagonal architecture:
//   - Implements domain/ports.OperationProvider (HTTPOperationProvider adapter)
//   - Registers 1 HTTP operation (http.request) via OperationRegistry at startup
//   - Application layer orchestrates operation steps via the OperationProvider port
//   - Domain layer defines operation contracts without HTTP client coupling
//
// All HTTP-specific types remain internal to this infrastructure adapter. The domain
// layer reuses existing OperationSchema, OperationResult, and InputSchema types without
// requiring new entities. This prevents domain layer pollution while maintaining full
// compile-time type safety.
//
// # Operation Types
//
// ## Declarative Operations (operations.go)
//
// Supported HTTP operations:
//   - http.request: Perform HTTP request with configurable method, headers, body, and timeout
//
// The operation accepts these inputs:
//   - url (required): Target URL (must start with http:// or https://)
//   - method (required): HTTP method (GET, POST, PUT, DELETE, case-insensitive)
//   - headers (optional): HTTP headers as key-value pairs
//   - body (optional): Request body as raw string
//   - timeout (optional): Request timeout in seconds (default: 30)
//   - retryable_status_codes (optional): Status codes treated as retryable failures (e.g., 429, 502, 503)
//
// The operation returns these outputs:
//   - status_code: HTTP response status code
//   - body: Response body content
//   - headers: Response headers
//   - body_truncated: Whether the response body was truncated
//
// # Provider Implementation
//
// ## HTTPOperationProvider (provider.go)
//
// Operation execution and dispatch:
//   - ListOperations: Enumerate registered HTTP operations
//   - GetOperation: Retrieve operation schema by name
//   - Execute: Dispatch to HTTP request handler based on operation type
//
// The provider wraps Go's net/http.Client with per-request timeout configuration
// derived from operation inputs. It validates required inputs (url, method), extracts
// headers and body, and captures the full response into structured outputs accessible
// via template interpolation (e.g., {{states.step_name.output.status_code}}).
//
// Helper functions handle input extraction and validation:
//   - validateRequiredInputs: Ensures url and method are present
//   - extractHeaders: Parses header map from operation inputs
//   - extractBody: Extracts request body string
//   - extractTimeout: Resolves timeout with 30-second default
//   - extractRetryableStatusCodes: Parses retryable status code list
//   - handleRequestError: Wraps transport errors into operation results
//   - isRetryableStatus: Checks if a status code matches retryable list
//   - buildOutputs: Constructs output map from HTTP response
//
// # Error Handling
//
// Operations return structured OperationResult with success/failure status.
//
// Common failure patterns:
//   - Missing required inputs (url, method): Returns failure result immediately
//   - DNS resolution or connection error: Returns failure with descriptive message
//   - Timeout exceeded: Returns failure when request duration exceeds configured timeout
//   - Retryable status codes: Returns failure with retryable flag for workflow retry logic
//
// # Integration Points
//
// ## CLI Wiring (internal/interfaces/cli/run.go)
//
// Provider registration at startup:
//   - Create HTTPOperationProvider instance with http.Client and logger
//   - Register in CompositeOperationProvider alongside GitHub and Notify providers
//   - Connect to ExecutionService via SetOperationProvider()
//   - Follows F054 GitHub plugin wiring pattern
//
// The provider is instantiated in the composition root (run.go) and injected into
// the application layer via dependency inversion. This enables compile-time wiring
// without RPC overhead (ADR-001).
//
// # Performance and Security
//
// Performance characteristics:
//   - Per-request timeout: Configurable via inputs, defaults to 30 seconds
//   - No connection pooling configuration: Uses net/http default transport
//   - Retryable status codes: Enables workflow-level retry for transient failures
//
// Security measures:
//   - URL validation: Requires http:// or https:// scheme
//   - Context cancellation: Propagated to HTTP client for graceful shutdown
//   - No credential logging: Headers containing auth tokens handled by AWF secret masking
//   - Input validation prevents injection via structured operation inputs
package http
