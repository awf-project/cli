---
title: "020: ACP Server Migration to Official coder/acp-go-sdk"
---

**Status**: Accepted
**Date**: 2026-06-05
**Issue**: F105
**Supersedes**: ADR 018 (implementation detail, not decision)
**Superseded by**: N/A

## Context

AWF's ACP server (introduced in ADR 018) was initially implemented as a custom JSON-RPC 2.0 server in `pkg/acpserver/` (~1620 lines across 10 files). This custom implementation:

1. Duplicates protocol conformance logic already solved by the official SDK
2. Increases maintenance burden when the ACP spec evolves or becomes semver-public
3. Blocks extensions that depend on SDK features (e.g., structured protocol updates in F108)
4. Requires custom error handling, session lifecycle management, and stdout serialization guarantees
5. Provides no advantage over the battle-tested official implementation

The official `github.com/coder/acp-go-sdk` (v0.13.x+) provides:

- Complete ACP protocol implementation with proper error handling
- Maintained by Coder with tight spec alignment
- Stdio transport with configurable payload caps
- Panic-safe handler execution primitives
- Regular updates aligned with official ACP releases
- Clean handler signatures supporting the required methods (Initialize, NewSession, Prompt, Cancel)

## Decision

Migrate the ACP server implementation from the custom `pkg/acpserver/` to the official SDK, wrapped in a new `internal/infrastructure/acp/` adapter package that:

1. Implements the `acp.Agent` interface from the SDK, delegating to `internal/application/acp_errors.go` taxonomy and application-layer `ACPSessionService`
2. Exposes the SDK connection lifecycle via `acp.NewAgentSideConnection(agent, stdout, stdin)` with proper `<-conn.Done()` cleanup
3. Wires `RequestPermission` transport through `ports.ACPClient` to `conn.RequestPermission`
4. Isolates SDK-specific types from the CLI layer, maintaining hexagonal architecture
5. Preserves 100% user-facing behavior parity with the legacy implementation (iso-functional)
6. Maintains panic isolation via `defer recover()` in handler wrappers with SDK-independent error recovery
7. Includes comprehensive test coverage (>85%) exercising the SDK's transport layer and concurrency invariants

## Rationale

### Architecture Compliance

The migration preserves the hexagonal layering principle by placing the SDK adapter in `internal/infrastructure/acp/` rather than directly using the SDK in `interfaces/cli/`. This allows:

- **Substitutability**: Future SDK upgrades or replacements require changes in one package only
- **Type isolation**: SDK types stay within the adapter; the CLI depends only on domain ports (ports.ACPClient for permission transport)
- **Clear ownership**: Protocol implementation logic is cleanly separated from command wiring and session coordination
- **Error taxonomy preservation**: Application layer `acp_errors.go` remains the single source of truth for `ACPHandlerError` kinds, mapped to SDK error variants in infra-only `toACPError`

This pattern mirrors the successful F104 MCP server migration (ADR 019) and follows the project's architectural rules. The `RequestPermission` transport binding is wired as an infrastructure adapter (`internal/infrastructure/acp/permission.go`) following the ports-and-adapters pattern.

### Gating and Risk Mitigation

A mandatory SPIKE (US3 in F105 spec) validates eight protocol-shape unknowns before any production code change or deletion:

1. SDK's `acp.Agent` interface signature and connection lifecycle (`NewAgentSideConnection`, `Done()`, `SetLogger`)
2. Handler signatures for Initialize, NewSession, Prompt, Cancel
3. Parking semantics for multi-turn prompts (per-prompt completion hook support)
4. SessionUpdate emission API (typed variants vs free-form payload)
5. Payload cap configuration (10 MiB read limit)
6. RequestPermission outbound call signature and stdout serialization
7. Error type mappings and SDK error variants
8. Protocol version number and minimum Go version requirements

SPIKE failure (any unknown unresolved) aborts F105 entirely per FR-014; this gate makes the big-bang migration approach safe.

## Consequences

**What becomes easier:**

- ACP server implementation gains maintenance parity with MCP (both via official SDKs)
- Future ACP spec additions (e.g., new session methods, structured content types for F108) are covered by SDK releases rather than custom protocol code
- Payload cap, error handling, and concurrent dispatch safety are guaranteed by the SDK rather than custom invariant tests
- Handler panics are caught and translated to proper ACP errors without exposing stack traces
- Two nearly-identical serve scaffolds (`mcp_serve.go` + `acp_serve.go`) can now follow identical SDK patterns, setting up a future DRY extraction

**What becomes harder:**

- Debugging ACP session issues requires familiarity with the SDK's internal error paths (though these are well-documented)
- Each `awf acp-serve` process consumes ~10 MB RSS. Long-lived editor sessions that never close their ACP process will hold that memory until the editor exits or explicitly closes the session (same as before)
- SDK version lock (v0.13.x) must be actively maintained; point releases are evaluated for breakage before upgrade
- Windows support remains deferred: signal-aware shutdown and process-group cleanup use POSIX-only syscalls (`Setpgid`, `syscall.Kill(-pgid, ...)`); ACP integration tests gate on `//go:build integration && !windows`

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Hexagonal Architecture | Compliant | SDK confined to `internal/infrastructure/acp/` via AST-based architecture test; domain gains no SDK types; application gets infra adapter for `RequestPermission` via `ports.ACPClient`; error taxonomy (`ACPHandlerError`) stays in application layer (24 in-package consumers); `.go-arch-lint.yml` updated with `go-sdk-acp` vendor and `infra-acp` component |
| Go Idioms | Compliant | `context.Context` threads from `acp_serve.RunE` through `conn.Serve` and handler dispatch; goroutine+channel for shutdown coordination; defer panic recovery with named returns following F104 pattern |
| Minimal Abstraction | Compliant | SDK adapter is infrastructure-only; single `ports.ACPClient` port method for permission requests; no new domain ports or abstractions |
| Error Taxonomy | Compliant | Application layer `ACPHandlerError` kinds map to SDK error variants in `toACPError`; five `USER.ACP.*` codes preserved (INVALID_PARAMS, UNSUPPORTED_BLOCK, PROMPT_IN_FLIGHT, UNKNOWN_SESSION, PROTOCOL_VERSION_UNSUPPORTED) |
| Security First | Compliant | `SecretMasker.MaskText` applied to all `agent_message_chunk`, `agent_thought_chunk`, and `tool_call` args before emission; 10 MiB `bufio.Scanner` ceiling (verified / configured against SDK default) prevents OOM; `signal.NotifyContext` SIGTERM→SIGKILL prevents zombie processes |
| Test-Driven Development | Compliant | SPIKE harness validates all 8 unknowns before production code starts; adapter coverage >85% on `internal/infrastructure/acp/` required (NFR-001); `make test-race` mandatory for concurrency-heavy code |
| Documentation Co-location | Compliant | `internal/infrastructure/acp/doc.go` ≥145 lines documenting Purpose, Public Surface, Internal Layout, Threat Model, Error Taxonomy, Dependency Contract, SDK Substitution patterns |

## Notes

**Deletion of `pkg/acpserver/`:**

The entire `pkg/acpserver/` package (10 files: doc.go, protocol.go, server.go, types.go, protocol_test.go, server_test.go, types_test.go, architecture_test.go, goroutine_leak_test.go, writeframe_internal_test.go) is deleted as part of F105 completion. This is the intended outcome: the custom implementation is fully replaced by the SDK adapter.

**ADR-018 Relationship:**

ADR-018 decided on the ACP protocol and the per-session subprocess architecture (`awf acp-serve`). This decision stands unchanged and is not superseded by F105. What F105 supersedes is the implementation detail in ADR-018's "Public package" section: moving from stdlib-only `pkg/acpserver/` to SDK-wrapped `internal/infrastructure/acp/` with `internal/domain/ports/acp_client.go` for the permission transport port.

**Comparison to F104 (MCP Migration):**

This migration follows the identical playbook as F104 (ADR 019):
- Mandatory SPIKE gate resolving SDK unknowns before production code
- New infrastructure adapter in `internal/infrastructure/{service}/`
- AST architecture test enforcing SDK confinement
- Panic isolation via defer recover wrappers
- Per-step or per-handler renderer/emitter preservation
- `.go-arch-lint.yml` updated with vendor stanza and component registration
- Big-bang approach with SPIKE failure abort gate

The F105 spec explicitly notes "F104 (MCP migration to the official go-sdk, commit 9740292) is the live blueprint for this work."
