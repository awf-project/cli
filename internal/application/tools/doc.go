// Package tools implements the application-layer MCP proxy infrastructure for F099:
// tool interception and routing in AI agent workflows.
//
// # Architecture Overview
//
// The package sits in the application layer and coordinates between domain ports and
// infrastructure adapters. It has two primary concerns:
//
//  1. ProxyService — lifecycle management of the MCP proxy subprocess (stdio path) or
//     in-process HTTP router (HTTP path).
//  2. Router — in-process dispatch of tool calls to registered ToolProvider adapters.
//
// Both components accept domain-level ports (ports.ToolProvider, ports.Logger, ports.Tracer)
// and do not import any infrastructure package, preserving the hexagonal dependency rule.
//
// # Two Proxy Paths
//
// ## Stdio Path (Claude, Gemini, Codex, OpenCode)
//
// ProxyService.StartForStdio writes a temporary JSON config file and spawns:
//
//	awf mcp-serve --config=<path>
//
// The subprocess runs the MCP server protocol over stdin/stdout. The provider's CLI
// receives the config path via a provider-specific flag (e.g., --mcp-config for Claude).
// Cleanup kills the subprocess and removes the temp file; it is idempotent.
//
// ## HTTP Path (OpenAI Compatible)
//
// ProxyService.StartForHTTP builds an in-process Router containing the same tool
// providers as the stdio path, but does not spawn a subprocess. The Router is injected
// directly into the OpenAI Compatible provider via SetToolRouter, enabling the multi-turn
// tool-call loop (T012) to dispatch calls without a round-trip through the network.
//
// # ProxyConfig
//
// ProxyConfig drives which tools are exposed:
//
//   - Enable: master switch; both StartForStdio and StartForHTTP return a noop when false.
//   - InterceptBuiltins: when true, the built-in tool provider (bash, glob, grep, read,
//     write, edit) is included as the first registered provider.
//   - PluginTools: each entry names a plugin and the subset of its operations to expose.
//     PluginToolAdapter translates operation schemas to ports.ToolDefinition values and
//     routes CallTool invocations back through the OperationProvider port.
//
// # Router
//
// Router implements a flat, name-keyed dispatch table over multiple ToolProvider adapters.
// Registration is append-only; name collisions return an error with the TOOL_COLLISION
// error code so callers can surface it explicitly rather than silently shadowing tools.
//
// ListTools returns all definitions from all registered providers in registration order.
// CallTool dispatches to the provider that owns the named tool, then logs timing and
// result via the Tracer and Logger ports. Unregistered tool names return UNKNOWN_TOOL.
//
// # ProviderFactory
//
// ProxyService accepts a ProviderFactory function at construction time rather than
// building providers directly. This injects T013's real adapter construction without
// requiring ProxyService to import the infrastructure/tools/builtins package. It also
// enables unit tests to supply a stub factory returning fixed providers.
//
// # Error Codes
//
// Domain-level error codes from internal/domain/errors are used for all structured
// errors returned by this package:
//
//   - TOOL_COLLISION — two providers registered the same tool name.
//   - UNKNOWN_TOOL   — CallTool received a name not registered by any provider.
//
// # Lifecycle Contract
//
// Both StartForStdio and StartForHTTP return a cleanup func() error. Callers MUST invoke
// cleanup after the agent exits, regardless of success or failure. Cleanup functions are
// idempotent: a second call returns nil without side effects.
//
// Defer order in execution paths:
//
//  1. MCP injector cleanup (stops the subprocess / releases in-process resources)
//  2. ToolProxyService cleanup (removes temp config files)
//
// The reverse-defer ordering in Go (LIFO) ensures the injector runs before the service
// teardown, matching the startup order of proxy-then-injector.
//
// # Thread Safety
//
// Router uses a sync.RWMutex. Register acquires the write lock; ListTools and CallTool
// acquire the read lock. ProxyService itself is not designed for concurrent StartForStdio
// or StartForHTTP calls on the same instance; each workflow step creates a fresh call.
package tools
