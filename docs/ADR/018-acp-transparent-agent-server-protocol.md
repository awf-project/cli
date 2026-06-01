---
title: "018: ACP Transparent Agent Server via JSON-RPC 2.0 stdio Subprocess"
---

**Status**: Accepted
**Date**: 2026-05-30
**Issue**: F102
**Supersedes**: N/A
**Superseded by**: N/A

## Context

AWF orchestrates AI agents through YAML workflows. Editors and IDE extensions (Zed, acp.nvim) that already drive those same agents want to use AWF as a transparent backend: the editor sends prompts, AWF dispatches them through configured workflows, and structured events flow back in real time — without the user switching to a terminal.

F102 must solve four problems that together constitute an external-facing API contract:

1. **Session lifecycle**: Editors need to open, prompt, and cancel named sessions that persist across multiple turns of a conversation, with AWF routing each prompt through an appropriate workflow.
2. **Event streaming**: Step lifecycle events (started, completed, failed), agent message chunks, tool calls, and thought chunks must reach the editor as structured notifications as they occur — not as a single bulk response at completion.
3. **Approval gates**: When a workflow step requires user confirmation (e.g., shell command execution), AWF must call back into the editor to request permission before proceeding.
4. **MCP server overlay**: Editors may provide their own MCP server configuration that must be merged with the workflow's per-step MCP proxy configuration, with editor entries taking precedence on key collision.

Two protocol-level questions are load-bearing beyond this feature:

- **Which protocol** governs the editor–AWF session contract? The answer locks in an external-facing API that editor plugin authors will compile against.
- **How is bidirectionality handled?** AWF must originate requests to the editor (for approval gates), not only respond to editor-originated requests. This is a structural departure from MCP's purely server-driven request/response model.

## Candidates

### Protocol

| Option | Pros | Cons |
|--------|------|------|
| **ACP (Agent Control Protocol) over JSON-RPC 2.0** | Adopted by Zed and acp.nvim; standardised session semantics (`session/new`, `session/prompt`, `session/cancel`, `session/request_permission`); JSON-RPC 2.0 base is well-understood; notifications are first-class (no response required) | Spec is younger than MCP; subset selection required |
| **MCP with custom session methods** | Reuses existing `pkg/mcpserver/` infrastructure verbatim | MCP has no session concept; grafting one on requires deviating from the MCP spec and confuses editors that treat MCP strictly |
| **Custom JSON-RPC over stdio** | Full schema control | No editor support out-of-box; every editor integration requires a bespoke adapter; no ecosystem tooling or shared test harnesses |
| **gRPC bidirectional streaming** | Native bidirectionality; strong typing via protobuf | No editor CLI support; requires protobuf toolchain for editor plugin authors; conflicts with go-plugin usage in ADR-015 |

### Bidirectionality Mechanism

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **`Server.CallClient` with `sync.Map` response channels** | Server generates a unique request ID, marshals an outbound JSON-RPC request, parks the goroutine on a buffered channel stored in a `sync.Map` keyed by ID, disambiguates inbound frames by probing for `"method":""` + matching ID | Minimal surface (one primitive); Go-idiomatic goroutine+channel; unambiguous frame routing; tested by `-race` | Caller must hold a `context.Context` with a deadline to prevent permanent parking |
| **Separate outbound connection (second stdio pair)** | Editor and AWF open two stdio channels: one for editor-originated requests, one for AWF-originated requests | True duplex isolation | Requires editor support for two-channel mode; doubles subprocess stdio plumbing; no ACP spec precedent |
| **Polling via notification acknowledgements** | AWF sends a `session/request_permission` notification; editor sends a `session/permission_response` request at its convenience | Zero in-process waiting | Race between cancel and delayed response; does not satisfy the synchronous approval-gate semantics required by FR-009 |

### Process Topology

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **A: Per-session subprocess `awf acp-serve`** | AWF exposes a hidden `awf acp-serve` Cobra subcommand; editor spawns one process per editing session; protocol served over stdin/stdout | Crash-isolated per session; proven pattern from ADR-017 (`awf mcp-serve`); signal-aware shutdown via `signal.NotifyContext`; hidden subcommand has no stability guarantees independent of binary | One extra AWF process per editor session (~10 MB RSS) |
| **B: Shared server multiplexed with HTTP serve** | ACP added as a second protocol surface inside `awf serve` alongside HTTP+SSE | Fewer processes | Couples HTTP serve evolution to ACP evolution; multiplexer complexity; entangles SSE subscriber model with session model |
| **C: External sidecar binary `awf-acp`** | Separate binary proxies to `awf run` subprocesses | Zero changes to main AWF binary | Duplicates provider/execution machinery; bypasses existing OTel hooks; version drift risk |

## Decision

**Protocol:** Adopt ACP over JSON-RPC 2.0 (stdio transport). Implement the required subset for v1: `initialize`, `initialized`, `session/new`, `session/prompt`, `session/cancel`, `session/request_permission` (server-originated), and `shutdown`. Prompts, resources, `fs/` tools, and `terminal/` methods are out of scope and deferred.

**Process topology:** Option A — per-session subprocess `awf acp-serve`. One `awf acp-serve` process is spawned by the editor per session. The subprocess serves ACP over stdin/stdout. Lifecycle is signal-aware: `signal.NotifyContext` with SIGTERM→SIGKILL grace matching `newMCPServeCommand`.

**Bidirectionality:** `Server.CallClient(ctx, method, params)` — a single outbound primitive on `acpserver.Server`. It serialises the request through a `sync.Mutex`-protected `json.Encoder` (same encoder used for responses, preventing interleave), parks the goroutine on a buffered channel stored in a `sync.Map` keyed by a server-generated integer ID, and returns when the matching inbound response is dispatched by the serve loop's `probe` path. The `probe` unmarshals only the `method` and `id` fields; frames with `method == ""` and a matching pending ID are treated as responses; all others as requests.

**Public package:** ACP engine lives in `pkg/acpserver/` (not `internal/`), with zero `internal/` imports enforced by an AST-based architecture test (`architecture_test.go`). This mirrors the `pkg/mcpserver/` invariant from ADR-017 and gives future external consumers a stable embeddable ACP engine.

**MCP merge-precedence rule (FR-011):** When an editor provides `session/new.mcpServers`, those entries are merged with the workflow's per-step MCP proxy configuration inside `ACPSessionService.handleSessionNew`. On key collision, the editor-provided entry wins. The merge result is stored on the `ACPSession` and overlaid at step-start time. This rule is implemented in the application layer; `ExecutionService` is not modified.

**Key rules established:**

- `pkg/acpserver` depends on Go stdlib only — no `internal/` imports, no framework deps. Verified at every CI run by `architecture_test.go`.
- `ACPClient` port (domain) has exactly one method for v1: `RequestPermission(ctx, toolCall, options) (bool, error)`. `fs/` and `terminal/` methods are deferred; pre-declaring stubs would leak out-of-scope features into the domain.
- `ACPRenderer` is instantiated per workflow step, not per session — tool-call ID deduplication (first occurrence → `tool_call`, subsequent → `tool_call_update`) must not bleed across steps.
- `USER.ACP.*` error codes extend the existing taxonomy at exit code 1: `INVALID_PROMPT`, `UNSUPPORTED_BLOCK`, `PROMPT_IN_FLIGHT`, `UNKNOWN_SESSION`, `PROTOCOL_VERSION_UNSUPPORTED`. No new exit-code category is introduced.
- `awf acp-serve` is `Hidden: true` — not user-facing; no independent stability guarantees; registered in `root.go` adjacent to `newMCPServeCommand`.
- ACP protocol version is a single integer constant in `pkg/acpserver/protocol.go`. Mismatches surface as `USER.ACP.PROTOCOL_VERSION_UNSUPPORTED` with a textual message rather than a silent failure.
- `FanoutPublisher` wraps the existing `ports.EventPublisher` — `ExecutionService` is not modified to support N publishers. The fan-out wiring happens at the interfaces layer.

## Consequences

**What becomes easier:**

- Editors (Zed, acp.nvim) can use AWF as a transparent workflow backend without shelling out to `awf run` and parsing stdout.
- Multi-turn conversational sessions are first-class: `ConversationManager` parking across `session/prompt` cycles is supported by the `ACPInputReader` channel bridge.
- Approval gates are synchronous from the workflow's perspective: `ACPClient.RequestPermission` blocks the step until the editor responds or the session context is cancelled.
- All step and workflow lifecycle events reach the editor as structured notifications via `WorkflowEventProjector` — no polling, no log scraping.
- Future fs/terminal methods can be added by extending the `ACPClient` port and implementing a new infrastructure adapter, with no changes to `ACPSessionService` or the engine.
- External consumers can embed `pkg/acpserver` to build custom ACP-enabled tooling; the stdlib-only invariant makes it a zero-overhead dependency.

**What becomes harder:**

- Adding new ACP methods (e.g., `fs/read`, `terminal/exec`) is a semver-visible change to `pkg/acpserver` and requires a coordinated release with editor plugin updates.
- Each `awf acp-serve` process consumes ~10 MB RSS. Long-lived editor sessions that never close their ACP process will hold that memory until the editor exits or explicitly closes the session.
- The `sync.Map`-tracked response channels in `Server.CallClient` require callers to always pass a `context.Context` with a deadline; an unbounded context would park the goroutine indefinitely if the editor never responds to a `session/request_permission`.
- Two near-identical Cobra serve scaffolds (`mcp_serve.go` + `acp_serve.go`) coexist; the `ServeInitializer` DRY extraction is intentionally deferred until a third serve variant stabilises.
- Windows support is deferred: `Setpgid` + `syscall.Kill(-pgid, ...)` for process-group teardown is POSIX-only. ACP integration tests gate on `//go:build integration && !windows`.

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Hexagonal Architecture | Compliant | `pkg/acpserver` has zero `internal/` imports (AST-enforced); domain gains only `ports.ACPClient` + error codes; application gets `ACPSessionService`; infrastructure adds `internal/infrastructure/acp/`; interface layer adds `acp_serve.go`; `.go-arch-lint.yml` extended with `pkg-acpserver` and `infra-acp` components |
| Go Idioms | Compliant | `context.Context` threads from `RunE` through `Server.Serve` and `Server.CallClient`; goroutine+buffered-channel+`sync.Map` for bidirectional dispatch; `errors.Join` for `FanoutPublisher.Close`; `signal.NotifyContext` for shutdown |
| Minimal Abstraction | Compliant | Single `ACPClient` port method for v1; `FanoutPublisher` is a 30-LOC wrapper (not a generic pub/sub framework); no `ServeInitializer` extracted yet (deferred per cleanup research) |
| Error Taxonomy | Compliant | Five new `USER.ACP.*` codes; no new exit-code category (all map to `cli.ExitUser` = 1 or `cli.ExitExecution` = 3 via existing switch in `ErrorCode.ExitCode()`) |
| Security First | Compliant | `SecretMasker.MaskText` applied to all `agent_message_chunk`, `agent_thought_chunk`, and `tool_call` args before emission; 10 MiB `bufio.Scanner` ceiling prevents OOM; `signal.NotifyContext` SIGTERM→SIGKILL prevents zombie processes |
| Test-Driven Development | Compliant | `pkg/acpserver/architecture_test.go` is the first test written (RED before production code); `≥85%` coverage required on concurrency-heavy code; `make test-race` mandatory for `pkg/acpserver/`, `internal/infrastructure/acp/`, and integration package |
| Documentation Co-location | Compliant | `pkg/acpserver/doc.go` and `internal/infrastructure/acp/doc.go` each ≥100 lines per project rule; YAML schema documented in struct comments |
