// Package github implements infrastructure adapters for GitHub CLI operations.
//
// The github package provides concrete implementations of the OperationProvider port
// defined in the domain layer, enabling workflow steps to perform declarative GitHub
// operations (issue retrieval, PR creation, project status updates) without shell
// scripting or jq parsing. The provider uses gh CLI as the primary backend with
// direct HTTP API fallback for environments without gh installed.
//
// # Architecture Role
//
// In the hexagonal architecture:
//   - Implements domain/ports.OperationProvider (GitHubOperationProvider adapter)
//   - Registers 9 GitHub operations via OperationRegistry at startup
//   - Application layer orchestrates operation steps via the OperationProvider port
//   - Domain layer defines operation contracts without GitHub coupling
//
// All GitHub-specific types remain internal to this infrastructure adapter. The domain
// layer reuses existing OperationSchema, OperationResult, and InputSchema types without
// requiring new entities. This prevents domain layer pollution while maintaining full
// compile-time type safety.
//
// # Operation Types
//
// ## Declarative Operations (operations.go)
//
// Supported GitHub operations:
//   - github.get_issue: Retrieve issue metadata (title, body, labels, state)
//   - github.get_pr: Retrieve pull request metadata
//   - github.create_issue: Create new issue with title, body, labels
//   - github.create_pr: Create pull request with title, body, base, head
//   - github.add_labels: Add labels to issue or PR
//   - github.set_project_status: Update GitHub Projects v2 field values
//   - github.list_comments: Retrieve issue/PR comments
//   - github.add_comment: Add comment to issue or PR
//   - github.batch: Execute multiple GitHub operations with concurrency control
//
// Each operation is registered as an OperationSchema with input validation, output
// schema, and field selection support (FR-002).
//
// # Provider Implementation
//
// ## GitHubOperationProvider (provider.go)
//
// Operation execution and dispatch:
//   - ListOperations: Enumerate registered GitHub operations
//   - GetOperation: Retrieve operation schema by name
//   - Execute: Dispatch to operation handler based on operation type
//
// The provider uses a central dispatch method with switch-case routing to operation
// handlers. This keeps all GitHub logic in one cohesive package without requiring
// interface-per-operation abstractions (ADR-003).
//
// ## BatchExecutor (batch.go)
//
// Batch operation processing:
//   - Execute: Run multiple GitHub operations with configurable concurrency
//   - Strategies: all_succeed, any_succeed, best_effort (US4)
//   - Uses golang.org/x/sync/errgroup with semaphore for concurrency control
//   - Returns aggregate result with success/failure counts
//
// Batch execution reuses the proven errgroup + semaphore pattern from AWF's parallel
// step execution (ADR-004).
//
// # Client and Authentication
//
// ## GitHubClient (client.go)
//
// Backend execution layer:
//   - RunGH: Invoke gh CLI with context cancellation via os/exec
//   - RunHTTP: Not yet implemented — returns error directing users to gh CLI
//   - DetectRepo: Auto-detect repository from git remote URL parsing (uses sync.Once for thread safety)
//
// Uses os/exec for gh CLI invocation. Retry logic for rate limiting is planned but not yet implemented.
//
// ## AuthManager (auth.go)
//
// Authentication resolution:
//   - DetectAuth: Determine active auth method (gh CLI, GITHUB_TOKEN, none)
//   - Priority chain: gh auth status > GITHUB_TOKEN env var > none
//   - No fallback to HTTP API currently (RunHTTP not implemented)
//
// # Error Handling
//
// Operations return standard Go errors via fmt.Errorf with contextual messages.
//
// Planned: Structured error types for common GitHub failures:
//   - github_not_found: Issue/PR does not exist
//   - github_auth_error: Authentication failed or missing
//   - github_branch_not_found: Head/base branch missing
//   - github_rate_limit: Rate limit exceeded
//   - github_invalid_field: Invalid project field value
//
// Current implementation wraps errors with fmt.Errorf for basic error context.
//
// # Integration Points
//
// ## CLI Wiring (internal/interfaces/cli/run.go)
//
// Provider registration at startup:
//   - Create GitHubOperationProvider instance
//   - Register operations via OperationRegistry
//   - Connect to ExecutionService via SetOperationProvider()
//   - Follows F039 agent registry wiring pattern
//
// The provider is instantiated in the composition root (run.go) and injected into
// the application layer via dependency inversion. This enables compile-time wiring
// without RPC overhead (ADR-001).
//
// # Performance and Security
//
// Performance characteristics:
//   - Batch concurrency: Planned with configurable concurrency limit
//   - Rate limiting: Planned integration with pkg/retry
//
// Security measures:
//   - AuthMethod.String() masks tokens in logs (returns "token(***)")
//   - DetectAuth never logs token values
//   - Input validation prevents command injection via gh CLI argument structure
//   - Field selection support planned to limit output size and exposure
package github
