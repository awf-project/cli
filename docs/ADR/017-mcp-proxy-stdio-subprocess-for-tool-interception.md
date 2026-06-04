---
title: "017: MCP Proxy via Per-Step stdio Subprocess for Tool Interception"
---

**Status**: Accepted
**Date**: 2026-05-23
**Issue**: F099
**Supersedes**: N/A
**Superseded by**: N/A

## Context

AWF orchestrates AI agents (Claude, Gemini, Codex, OpenCode, OpenAI Compatible) that invoke file system and shell tools as part of workflow execution. Currently those tool calls are entirely opaque to AWF: the agent CLI receives a prompt, runs, and returns output — AWF cannot intercept, audit, or extend the tool set the agent uses.

F099 must solve three problems simultaneously:

1. **Interception**: Make AWF's 6 built-in tools (`Read`, `Write`, `Edit`, `Bash`, `Glob`, `Grep`) available to agents via a structured protocol, so that tool calls are observable and auditable (OTel spans, structured logs).
2. **Extension**: Allow AWF gRPC plugins (existing `ports.OperationProvider` implementations) to expose their operations as agent tools without agents knowing about AWF's plugin model.
3. **Multi-provider support**: The mechanism must work across five agents with different injection APIs — four stdio CLIs and one HTTP provider — without requiring provider-specific tool-call logic in the domain or application layers.

Two protocol-level questions are load-bearing beyond this feature:

- **Which protocol** governs the host–agent tool call contract? The answer locks in an external-facing API that plugin SDK authors and workflow authors will depend on.
- **What process topology** delivers that protocol? The answer determines crash isolation, subprocess lifecycle complexity, and client compatibility across all five providers.

## Candidates

### Protocol

| Option | Pros | Cons |
|--------|------|------|
| **MCP 2024-11-05 (Model Context Protocol)** | Already supported by Claude, Gemini, Codex, OpenCode; JSON-RPC 2.0 base; standardized `tools/list` + `tools/call` semantics; schema-first tool definitions | Subset selection required; not all features needed |
| **Custom JSON-RPC over stdio** | Full control over schema | No CLI support out-of-box; every provider needs a custom adapter; no ecosystem tooling |
| **OpenAI `tools[]` HTTP format only** | Native to OpenAI Compatible provider; well-documented | Not supported by stdio CLIs (Claude, Gemini, Codex, OpenCode); two protocols required anyway |

### Process Topology

| Option | Description | Files changed | Risk |
|--------|-------------|---------------|------|
| **A: In-process MCP server** | AWF embeds the MCP server as a goroutine; agents connect via UNIX socket | ~10 | High — UNIX socket transport is nonstandard for Claude/Gemini CLI; stdio is the documented path |
| **B: Per-step subprocess `awf mcp-serve`** | AWF spawns `awf mcp-serve --config=<tmp>` per step; agents connect via stdio JSON-RPC | ~15 | Medium — subprocess lifecycle, but proven pattern from `RPCPluginManager` |
| **C: External MCP server via go-plugin gRPC** | Proxy as a go-plugin gRPC plugin loaded by AWF | ~25 | High — unnecessary extra layer; harder to debug; changes the plugin model |

## Decision

**Protocol:** Adopt MCP 2024-11-05 (latest stable as of 2026-01-01). Implement only the required subset: `initialize`, `initialized`, `tools/list`, `tools/call`, `shutdown`. Prompts, resources, `notifications/progress`, and sampling are out of scope and deferred.

**Process topology:** Option B — per-step subprocess `awf mcp-serve`. One `awf mcp-serve` process is spawned per step where `mcp_proxy.enable: true`. The subprocess serves MCP over stdin/stdout. The parent `awf run` process spawns it via `ToolProxyService.Start()` and tears it down via `ToolProxyService.Close()`.

**Server implementation:** The MCP server implementation initially lived in `pkg/mcpserver/` but was migrated to `internal/infrastructure/mcp/` in F104 to adopt the official `github.com/modelcontextprotocol/go-sdk` (see ADR 019). The adapter wraps the official SDK while maintaining identical user-facing behavior and continues to enforce zero `internal/` imports at the adapter boundary via lint rules and AST-based architecture tests.

**OpenAI Compatible exception:** The HTTP provider cannot use stdio; instead, `ToolRouter` is invoked in-process and its tool definitions are injected as `tools[]` in the Chat Completions request. This is an explicit split: stdio providers use subprocess MCP, HTTP provider uses in-process `tools[]`.

**Key rules established:**

- `pkg/mcpserver` depends on Go stdlib only — no `internal/` imports, no framework deps.
- `ToolProvider` port in domain; `BuiltinToolProvider` + `PluginToolAdapter` in infrastructure; `ToolRouter` + `ToolProxyService` in application.
- Tool names follow `<plugin>_<op>` (snake-case, single underscore) to satisfy MCP client name constraints. Dots are forbidden (Claude rejects them).
- Collision detection is fatal at step startup (registration time), not at call time.
- Subprocess lifecycle uses goroutine + buffered channel + 5s SIGTERM→SIGKILL deadline, matching `RPCPluginManager.connectWithTimeout` exactly.
- `awf mcp-serve` is `Hidden: true` — not user-facing; no stability guarantees independent of AWF binary version.
- `USER.MCP_PROXY.*` validation codes extend the error taxonomy (exit code 1) with six new codes: `UNKNOWN_KEY`, `UNKNOWN_PLUGIN`, `UNKNOWN_OPERATION`, `NAME_COLLISION`, `EMPTY_PROXY`, `UNSUPPORTED_PROVIDER`.

## Consequences

**What becomes easier:**

- Tool calls from all five agent providers are observable: each `tools/call` produces an OTel span and a structured zap log line.
- AWF gRPC plugins can expose operations to agents with no changes to the plugin manifest — `PluginToolAdapter` wraps the existing `ports.OperationProvider`.
- New tools can be added by implementing `ports.ToolProvider` without touching any agent provider code.
- External consumers can embed `pkg/mcpserver` to build custom MCP-enabled tooling.
- Subprocess crash isolation: a panic in `awf mcp-serve` is visible to the parent as a subprocess exit error but does not crash `awf run`.

**What becomes harder:**

- Each step with `mcp_proxy.enable: true` spawns an extra Go process (~10 MB RSS). At AWF's current scale this is acceptable; at high parallelism it requires monitoring.
- Codex and OpenCode have no `--tools ""` equivalent. Built-in tools cannot be disabled via flag injection; the proxy coexists with native tools and emits a startup `WARN` log. This is an accepted limitation documented in the YAML validation.
- MCP protocol version upgrades require coordinated changes to `pkg/mcpserver`, the hidden `mcp-serve` subcommand, and the per-provider config injection. The committed MCP version (2024-11-05) becomes the wire contract.
- `pkg/mcpserver` becoming public means adding new MCP methods (e.g., `notifications/progress`) is a semver-visible change.
- The OpenAI Compatible provider requires a separate in-process tools path (`tools[]` + `tool_choice` + multi-turn tool-call loop), maintained in parallel with the stdio subprocess path.

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Hexagonal Architecture | Compliant | Domain port `ports.ToolProvider`; application `ToolRouter`/`ToolProxyService`; infrastructure adapters; `pkg/mcpserver` has zero `internal/` imports; `.go-arch-lint.yml` extended with `pkg-mcpserver` and `infra-tools` components scoped appropriately |
| Go Idioms | Compliant | `context.Context` on all blocking ops; goroutine+buffered-channel+select for subprocess lifecycle; `errors.Is`/`fmt.Errorf` wrapping throughout |
| Minimal Abstraction | Compliant | No `ToolPolicy`/`ToolMiddleware`/`ToolCache` ports — decorator pattern is available if needed but not added prematurely; single function-value extension on `cliProviderHooks` (not a new interface) |
| Error Taxonomy | Compliant | Six new `USER.MCP_PROXY.*` codes extend the existing taxonomy; no new exit code category required (all are user/configuration errors, exit code 1) |
| Security First | Compliant | `Bash` tool delegates to `ShellExecutor` (existing shell-escaping, secret masking); subprocess uses SIGTERM→SIGKILL (no zombies); tmp config file written atomically (PID+timestamp suffix) |
| Test-Driven Development | Compliant | Table-driven unit tests per component; AST-based architecture tests for `pkg/mcpserver` import invariant; `make test-race` required on all application/infrastructure new code |
| Documentation Co-location | Compliant | `doc.go` per new package; YAML schema documented in `mcp_proxy.go` struct comments |
