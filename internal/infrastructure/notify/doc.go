// Package notify implements infrastructure adapters for notification operations.
//
// The notify package provides concrete implementations of the OperationProvider port
// defined in the domain layer, enabling workflow steps to send notifications through
// multiple backends (desktop, webhook) without shell scripting. The provider supports
// built-in notification backends with 10-second HTTP timeout for network-based backends
// and platform detection for desktop notifications.
//
// # Architecture Role
//
// In the hexagonal architecture:
//   - Implements domain/ports.OperationProvider (NotifyOperationProvider adapter)
//   - Registers 1 notification operation (notify.send) via OperationRegistry at startup
//   - Application layer orchestrates notification steps via the OperationProvider port
//   - Domain layer defines operation contracts without notification backend coupling
//
// All notification-specific types remain internal to this infrastructure adapter. The
// domain layer reuses existing OperationSchema, OperationResult, and InputSchema types
// without requiring new entities. This prevents domain layer pollution while maintaining
// full compile-time type safety.
//
// # Operation Types
//
// ## Declarative Operations (operations.go)
//
// Supported notification operations:
//   - notify.send: Send notification via configured backend (desktop, webhook)
//
// Each operation is registered as an OperationSchema with input validation, output
// schema, and backend selection support.
//
// # Provider Implementation
//
// ## NotifyOperationProvider (provider.go)
//
// Operation execution and dispatch:
//   - ListOperations: Enumerate registered notification operations
//   - GetOperation: Retrieve operation schema by name
//   - Execute: Dispatch to backend based on backend input parameter
//
// The provider uses a map-based dispatch with Backend interface to route to the
// appropriate notification backend. This keeps all notification logic in one cohesive
// package without requiring interface-per-backend abstractions (ADR-003).
//
// # Notification Backends
//
// ## Backend Interface (types.go)
//
// All backends implement a common Send interface:
//   - Send(ctx context.Context, payload NotificationPayload) (*BackendResult, error)
//
// ## DesktopBackend (desktop.go)
//
// Platform-native desktop notifications:
//   - Linux: Uses notify-send via libnotify
//   - macOS: Uses osascript with 'display notification' AppleScript
//   - Detects platform at runtime and fails gracefully on unsupported systems
//
// ## WebhookBackend (webhook.go)
//
// Generic HTTP webhook integration:
//   - JSON POST with standard AWF payload structure
//   - Supports any webhook-compatible service (Discord, Teams, PagerDuty, etc.)
//   - Fields: workflow, status, duration, message, outputs
//
// # Configuration
//
// ## NotifyConfig (types.go)
//
// Configuration resolution from .awf/config.yaml:
//   - default_backend: Backend to use when not specified in operation inputs
//
// Configuration values are loaded at provider initialization and injected into
// backend instances.
//
// # Error Handling
//
// Operations return standard Go errors via fmt.Errorf with contextual messages.
//
// Common error patterns:
//   - notify_missing_config: Required configuration value not provided
//   - notify_backend_not_found: Requested backend not available
//   - notify_missing_input: Required backend-specific input not provided
//   - notify_http_error: HTTP backend returned non-2xx status
//   - notify_timeout: Backend request exceeded 10-second timeout
//   - notify_platform_unsupported: Desktop backend unavailable on platform
//
// All errors are wrapped with fmt.Errorf for error chain support.
//
// # Integration Points
//
// ## CLI Wiring (internal/interfaces/cli/run.go)
//
// Provider registration at startup:
//   - Create NotifyOperationProvider instance with config
//   - Create CompositeOperationProvider wrapping GitHub + Notify providers
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
//   - HTTP timeout: 10 seconds for network-based backends (webhook)
//   - Desktop notifications: Platform-specific, typically <100ms
//   - No retries: Failed notifications return error immediately
//
// Security measures:
//   - webhook URLs and tokens never logged
//   - Input validation prevents command injection in desktop backend
//   - Context cancellation propagated to all backends
//   - HTTP client follows redirects only for same-host (prevents SSRF)
package notify
