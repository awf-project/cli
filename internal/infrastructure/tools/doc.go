// Package tools provides infrastructure adapters that implement the
// domain/ports.ToolProvider interface. It is the "adapter" half of the F099 MCP
// proxy: the application/tools.Router (and the standalone `awf mcp-serve`
// subprocess) call into these adapters to expose concrete tool implementations
// to the in-process or stdio MCP surface.
//
// # Sub-packages
//
//   - builtins: file-operation and shell tools (Read, Write, Edit, Bash, Glob,
//     Grep) implemented as pure Go functions. The provider is constructed once
//     at startup and lives for the lifetime of the proxy. Every ToolDefinition
//     this provider emits carries Source = "builtin".
//
//   - (root) plugin_adapter.go / schema_mapper.go: wrap an external MCP plugin
//     binary loaded by infrastructure/pluginmgr and expose its operations as
//     individual tools. ToolDefinitions emitted by this adapter carry
//     Source = "plugin:<name>".
//
// # Naming Conventions
//
// Built-in tools use PascalCase (Read, Write, Edit, Bash, Glob, Grep) to align
// with the names Anthropic-class agents (Claude Code, OpenCode) emit in their
// tool_use events. Plugin tools use snake_case with a "<plugin>_<operation>"
// prefix to make collisions impossible across plugins and to keep their names
// distinguishable from the built-ins at a glance.
//
// This is the only deliberate exception to the snake_case convention documented
// in ADR 017: aligning built-in names with native-agent vocabulary lets the proxy
// act as a drop-in replacement for the agent's own tools — the model already
// knows how to call Read; we don't need to retrain it on read.
//
// # Security Boundary
//
// The builtins.Provider takes a WithRootDir option that scopes file-touching
// handlers (Read, Write, Edit, Glob, Grep, and Bash cwd) to a single directory
// subtree. In production wiring (interfaces/cli/mcp_serve.go) this is bound to
// the subprocess's working directory — i.e. the workspace — so a prompt-injection
// asking the agent to read ~/.ssh/id_rsa cannot escape the workspace via a tool
// call. Plugins are unaffected by this restriction: their security model is owned
// by the plugin author and enforced inside the plugin process.
//
// Path validation is lexical (filepath.Clean + filepath.Abs + prefix check). It
// does not call filepath.EvalSymlinks because doing so makes tests fragile across
// OS temp-dir layouts and introduces additional TOCTOU surface. Operators needing
// strong isolation should run mcp-serve inside a chroot, container, or sandbox.
//
// Built-in Read and Edit also enforce a 5 MiB cap (builtins.MaxReadBytes) per
// single invocation to keep prompt-injection from OOM-killing the subprocess by
// pointing the agent at /dev/zero, large logs, or generated content. The agent
// can still page through large files via the Read offset/limit arguments.
//
// # Architecture Role
//
// In the hexagonal architecture both adapters implement domain/ports.ToolProvider
// so the application layer can call ListTools / CallTool / Close uniformly,
// regardless of whether the tool is a built-in Go function or a remote plugin
// process. Lifecycle (Close) is owned by whoever constructed the provider —
// typically the proxy service in application/tools — and Close is intentionally
// a no-op on Provider since the built-in provider holds no external resources.
//
// # Test Strategy
//
// Unit tests live next to each handler (read_test.go, write_test.go, etc.) and
// drive the provider through CallTool to exercise the full schema-validation +
// dispatch + result-mapping pipeline. The integration tests under
// tests/integration/mcp/ spawn a real `awf mcp-serve` subprocess and speak
// JSON-RPC against it, which is the canonical end-to-end test for both the
// builtins adapter and the plugin adapter.
package tools
