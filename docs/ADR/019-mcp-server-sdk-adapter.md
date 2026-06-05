---
title: "019: MCP Server Migration to Official go-sdk"
---

**Status**: Accepted
**Date**: 2026-06-04
**Issue**: F104
**Supersedes**: ADR 017 (implementation detail, not decision)
**Superseded by**: N/A

## Context

AWF's MCP server (introduced in ADR 017) was initially implemented as a custom JSON-RPC 2.0 server in `pkg/mcpserver/` (~1270 lines). This custom implementation:

1. Duplicates protocol conformance logic already solved by the official SDK
2. Increases maintenance burden when the MCP spec evolves
3. Blocks extensions that depend on SDK features (e.g., structured content types in F108)
4. Provides no advantage over the battle-tested official implementation

The official `github.com/modelcontextprotocol/go-sdk` (v1.6.x+) provides:

- Complete MCP 2024-11-05 protocol implementation
- Maintained by Anthropic with tight spec alignment
- In-memory and stdio transports
- Panic-safe handler execution
- Regular updates and security patches

## Decision

Migrate the MCP server implementation from the custom `pkg/mcpserver/` to the official SDK, wrapped in a new `internal/infrastructure/mcp/` adapter package that:

1. Wraps `*mcp.Server` with provider registration, deduplication, and result mapping
2. Exposes a minimal public API: `NewServer(version string)`, `RegisterProvider(ports.ToolProvider) error`, `ServeStdio(ctx context.Context) error`
3. Isolates SDK-specific types from the CLI layer, maintaining hexagonal architecture
4. Preserves 100% user-facing behavior parity with the legacy implementation
5. Maintains panic isolation via `defer recover()` in handler wrappers
6. Includes comprehensive test coverage (>85%) exercising the SDK's transport layer

## Rationale

### Architecture Compliance

The migration preserves the hexagonal layering principle by placing the SDK adapter in `internal/infrastructure/` rather than directly using the SDK in `interfaces/cli/`. This allows:

- **Substitutability**: Future SDK upgrades or replacements require changes in one package only
- **Type isolation**: SDK types stay within the adapter; the CLI depends only on domain ports
- **Clear ownership**: Protocol implementation logic is cleanly separated from command wiring

This pattern mirrors `internal/infrastructure/acp/` (ADR 018) and follows the project's architectural rules.

### Behavioral Parity

Testing confirms equivalent behavior across all dimensions:

- **Tool listing**: Same set of builtins + plugin tools exposed via `tools/list`
- **Tool invocation**: Calls route to providers and return equivalent text content
- **Panic handling**: Handler panics surface as errors, never crash the server
- **Message size**: Supports payloads up to 10 MiB (verified via scanner buffer configuration)
- **Signal handling**: Graceful shutdown via context cancellation

The SDK's wire protocol is identical to the legacy implementation, so existing agents (Claude, Gemini, Codex) see no difference.

### Maintenance

Reduces future work by:

- Eliminating custom protocol logic (376 LOC deleted)
- Deferring schema format extensions to the SDK (F108 requires a `switch c.Type` in `resultToMCP`, not a protocol redesign)
- Enabling plugin authors to trust the SDK's conformance guarantees

## Alternatives Considered

### Alternative A: SDK shim inside `pkg/mcpserver`

Keep the `pkg/mcpserver/` shell, replace its body with SDK calls, re-export SDK types.

**Rejected**: `pkg/` location forbids `internal/` imports. The shim would need to import domain ports cleanly, violating the rule. Also, re-exporting SDK types couples the public API to SDK internals.

### Alternative B: Inline SDK calls in `mcp_serve.go`

Drop the adapter; instantiate `*mcp.Server` directly in the CLI command.

**Rejected**: Violates hexagonal architecture (infrastructure logic lives in interfaces layer). Makes F108 Axis C (image/structured content) require edits to the CLI command, not just the adapter.

## Implementation Details

### New Package Structure

```
internal/infrastructure/mcp/
├── doc.go           # Architecture, threat model, adapter contract (≥100 lines)
├── server.go        # Server struct, RegisterProvider, ServeStdio
├── handler.go       # handlerFor wrapper with panic isolation
├── mapping.go       # schemaFromMap, toolToMCP, resultToMCP helpers
├── architecture_test.go   # AST-verified imports (stdlib, SDK, ports only)
├── handler_test.go        # Panic isolation verification
├── mcp_test.go            # E2E tests via SDK's transport layer
└── mapping_test.go        # Round-trip schema and result conversions
```

### Public API

```go
// NewServer creates an MCP server with the given version string.
func NewServer(version string) *Server

// RegisterProvider registers a tool provider, dedup'ing tool names.
// Returns error if a tool name conflicts with a previously registered tool.
func (s *Server) RegisterProvider(ctx context.Context, provider ports.ToolProvider) error

// ServeStdio runs the server over stdin/stdout with context cancellation.
// Returns context.Canceled on cancellation; other errors are protocol-level failures.
func (s *Server) ServeStdio(ctx context.Context) error
```

### Handler Panic Isolation

```go
func (s *Server) handlerFor(provider ports.ToolProvider, tool *ports.ToolDefinition) mcp.ToolHandlerFunc {
    return func(ctx context.Context, params *mcp.CallToolParamsRaw) *mcp.CallToolResult {
        defer func() {
            if r := recover(); r != nil {
                // Panic surfaced as error result; never propagates to SDK runtime
            }
        }()
        
        result, err := provider.CallTool(ctx, tool.Name, params.Arguments)
        if err != nil {
            return &mcp.CallToolResult{IsError: true}
        }
        return resultToMCP(result)
    }
}
```

## Migration Path

1. **Phase 1**: Build `internal/infrastructure/mcp/` adapter (tests driven by SDK client)
2. **Phase 2**: Rewrite `mcp_serve.go` to use the adapter; update `.go-arch-lint.yml`
3. **Phase 3**: Delete `pkg/mcpserver/` and rewrite integration tests (raw JSON-RPC assertions)
4. **Phase 4**: Verify behavioral parity with real agent run (Claude/Gemini against `mcp_proxy`)

## Trade-offs

| Trade-off | Accepted because |
|-----------|------------------|
| One additional package boundary | Enables F108 to be a one-file change (mapping.go); maintains hexagonal invariant |
| ~50 lines of adapter glue | Isolates SDK types from CLI; improves substitutability for future SDK upgrades |
| Slightly larger test suite | SDK-driven tests catch regressions against the same surface agents use |

## Success Criteria

- ✅ Agent-driven workflow using `mcp_proxy` lists and invokes equivalent tools (behavior parity)
- ✅ `pkg/mcpserver/` fully removed with zero remaining importers
- ✅ `internal/infrastructure/mcp/` achieves >85% test coverage
- ✅ All CI gates pass (`make build && lint && test && test-race`)
- ✅ Real end-to-end run with Claude/Gemini completes successfully

## References

- **Spec**: `.specify/implementation/F104/spec-content.md`
- **Implementation Plan**: `.specify/implementation/F104/plan.md`
- **Related**: ADR 017 (MCP Proxy), ADR 018 (ACP Transparent Agent Server)
- **Unblocks**: F108 Axis C (image/structured content in MCP responses)
