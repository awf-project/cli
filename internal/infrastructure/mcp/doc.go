// Package mcp implements the MCP (Model Context Protocol) infrastructure adapter
// that bridges AWF's internal tool providers to the official Go SDK transport.
// It is the infrastructure-side glue that exposes AWF workflow tools over the
// stdio channel consumed by AI agents (Claude, Gemini, Codex, and compatible clients).
//
// # Purpose
//
// This package wraps github.com/modelcontextprotocol/go-sdk/mcp to provide a minimal,
// safe adapter between the AWF domain's ports.ToolProvider interface and the MCP
// protocol. It occupies the infrastructure layer of the hexagonal architecture:
// it depends inward on domain ports and outward on the SDK transport. No application
// layer types appear in this package's public surface.
//
// The primary entry point for the CLI is the `awf mcp-serve` command, which
// instantiates Server, registers providers, and delegates to ServeStdio. The server
// exits when stdin closes or the context is cancelled.
//
// # Public Surface
//
// The public surface consists of three symbols:
//
//   - New(version string) *Server
//     Returns a Server with an empty tool registry. The version string is forwarded
//     to the SDK implementation metadata (ServerInfo.Version). Callers must call
//     RegisterProvider before ServeStdio; calling ServeStdio with no registered tools
//     is valid but produces an empty tools/list response.
//
//   - (*Server).RegisterProvider(p ports.ToolProvider) error
//     Iterates p.ListTools, deduplicates by name, and registers each tool on the
//     underlying SDK server via AddTool. Returns an error if any tool name is already
//     registered. Registration is expected to complete before ServeStdio is called;
//     calling RegisterProvider after ServeStdio may produce undefined behavior depending
//     on SDK internals (the SDK's tool registry is not documented as concurrency-safe
//     after Run starts).
//
//   - (*Server).ServeStdio(ctx context.Context) error
//     Runs the SDK's StdioTransport until ctx is cancelled or the connection closes.
//     The transport reads newline-delimited JSON-RPC 2.0 frames from stdin and writes
//     responses to stdout. Returns the SDK transport error, or nil on clean shutdown.
//     This method blocks until the connection terminates.
//
// # Internal Layout
//
// Three unexported files carry the implementation detail:
//
//   - mapping.go — Pure conversion helpers (toolToMCP, resultToMCP).
//     All functions are stateless and deterministic. No error side-effects.
//     No imports from application layer.
//
//   - handler.go — Constructs the SDK ToolHandler closure for each registered tool.
//     Wraps provider.CallTool with panic isolation (NFR-003): any panic in the provider
//     is caught by recover(), serialized as a generic error message, and returned as
//     IsError:true — the server continues processing subsequent requests. Stack traces
//     are never forwarded to the agent (information exfiltration risk; see Threat Model).
//
//   - server.go — Server struct, New, RegisterProvider, ServeStdio. Holds the SDK
//     server pointer and the name registry (names map[string]struct{}) used for
//     duplicate-registration detection.
//
// # Threat Model
//
// The MCP server is designed to run as a trusted local subprocess that communicates
// with an AI agent over stdio. The transport channel (stdin/stdout) is the only
// protocol surface. Threat scenarios addressed:
//
//   - Prompt injection: An agent may pass attacker-controlled values as tool arguments.
//     Tool handlers within ports.ToolProvider implementations must validate argument
//     values independently. This package does not validate argument content; it only
//     JSON-decodes the arguments map before forwarding to the provider.
//
//   - Information exfiltration via panics: Tool handler panics are caught in handler.go
//     and returned as a generic "panic recovered: %v" message (IsError:true). Internal
//     stack traces are never included in the response because they can leak file paths,
//     internal type names, and implementation detail useful for prompt-injection
//     reconnaissance. The panic value is formatted with %v, not %+v or runtime/debug,
//     to keep the message minimal.
//
//   - Oversized payloads (NFR-002): The SDK's StdioTransport enforces a 10 MiB per-
//     message ceiling on stdin frames. Frames exceeding this limit are rejected at the
//     transport layer before reaching handler.go. This cap matches the agent providers'
//     response body limit so neither direction truncates silently. Callers that need a
//     different ceiling must configure the SDK transport directly before wrapping it.
//
//   - Tool name collisions: RegisterProvider returns an error on duplicate tool names
//     so operator errors (two providers registering the same tool) are caught at startup
//     and surfaced to the caller, not silently overridden at runtime. The server does not
//     start serving if registration fails.
//
//   - Stderr contamination: All AWF diagnostic output (logs, debug traces) must be
//     directed to stderr. Stdout carries the JSON-RPC stream exclusively. Writing
//     non-JSON-RPC content to stdout corrupts the framing and breaks the agent
//     connection. This invariant is enforced by convention: this package writes nothing
//     to stdout directly; all output goes through the SDK transport.
//
// # Error Taxonomy
//
// Errors fall into three classes, each handled differently:
//
//   - SDK transport errors: Propagated directly from ServeStdio as the return value.
//     These indicate connection loss, context cancellation, or protocol framing failures.
//     The caller is responsible for deciding whether to restart the server.
//
//   - Provider errors (ports.ToolProvider.CallTool returns non-nil error): Translated
//     into an IsError:true CallToolResult with the error message as the sole text content
//     block. The JSON-RPC response itself is a success (no RPC-level error); the agent
//     receives the error as a tool result and decides how to proceed. This matches the
//     MCP convention for tool-level failures.
//
//   - Provider panics (NFR-003): Caught by the deferred recover in handler.go. Returned
//     as IsError:true CallToolResult with message "panic recovered: %v". The server
//     continues processing subsequent requests. Provider panics do not propagate to
//     ServeStdio and do not terminate the server process.
//
//   - Registration errors: Returned synchronously by RegisterProvider. The caller must
//     handle these before starting the serve loop (typically as a fatal startup error).
//
// # Dependency Contract
//
// This package is permitted to import:
//
//   - Standard library (context, encoding/json, fmt)
//   - github.com/modelcontextprotocol/go-sdk/mcp — The official MCP Go SDK. All
//     SDK types (Server, Tool, CallToolRequest, CallToolResult, StdioTransport,
//     ToolHandler, TextContent) are used only in unexported helpers and the Server
//     wrapper, not in the public surface. This insulates callers from SDK churn.
//     Tool input schemas are forwarded to the SDK as the raw map[string]any carried by
//     ports.ToolDefinition.InputSchema — no typed jsonschema package is imported (the SDK
//     pulls github.com/google/jsonschema-go only transitively).
//   - internal/domain/ports — ports.ToolProvider, ports.ToolDefinition, ports.ToolResult,
//     ports.ToolContent. These are the only internal imports permitted. Application or
//     interface layer imports are forbidden.
//
// It MUST NOT import:
//
//   - internal/application — hexagonal rule: infrastructure must not depend on application.
//   - internal/interfaces — same hexagonal rule.
//
// # SDK Substitution
//
// If github.com/modelcontextprotocol/go-sdk/mcp is replaced by a different MCP SDK,
// the changes are localized to this package: server.go (New, ServeStdio), handler.go
// (ToolHandler signature), and mapping.go (toolToMCP, resultToMCP). The public surface
// (Server, New, RegisterProvider, ServeStdio) and the ports.ToolProvider dependency
// remain unchanged. No application or interface layer changes are required.
package mcp
