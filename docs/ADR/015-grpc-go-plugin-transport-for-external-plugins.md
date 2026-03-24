---
title: "015: gRPC via go-plugin as External Plugin Transport"
---

**Status**: Accepted
**Date**: 2026-03-24
**Issue**: C067
**Supersedes**: N/A
**Superseded by**: N/A

## Context

AWF's plugin system has `RPCPluginManager` with stubs for `Init`, `Shutdown`, `ShutdownAll`, and `ports.OperationProvider`. The stubs return `ErrRPCNotImplemented`; no real process management or IPC exists. A transport mechanism must be chosen before any external plugin can execute.

The chosen mechanism defines an **external-facing API contract**: plugin authors compile their binaries against it. Once external plugins exist in the wild, changing the wire format is a breaking change requiring plugin recompilation.

Three factors constrain the decision:
1. Plugin processes must be isolated from the host (third-party code must not crash the host)
2. The connection handshake config must be shared between host and plugin SDK with a single source of truth
3. The protobuf generation toolchain (protoc) should not be required in all CI environments

## Candidates

| Option | Process isolation | Wire format | Auth | Build complexity |
|--------|------------------|-------------|------|-----------------|
| **go-plugin (gRPC)** | Yes — subprocess with managed lifecycle | Protobuf (generated, committed) | mTLS via go-plugin defaults | Requires proto-gen for regeneration; committed .pb.go avoids CI toolchain |
| **go-plugin (netrpc)** | Yes — subprocess with managed lifecycle | Go encoding/gob | None (deprecated in go-plugin v2) | Simple but non-standard types require custom codecs |
| **Unix socket + JSON** | Yes — subprocess | JSON (no schema) | None | Zero toolchain; schema drift risk; no codegen safety |
| **In-process plugin (Go plugin)** | No — crashes propagate | Go ABI | N/A | Requires plugin authors to target exact same Go version + OS |
| **WASM (e.g. Extism)** | Yes — sandbox | msgpack/JSON | N/A | WASM runtime dependency; immature Go toolchain support |

## Decision

Use **HashiCorp go-plugin v1** with gRPC transport. The protobuf service contract is defined in `proto/plugin/v1/plugin.proto` and generated files are committed to the repository. Plugin SDK authors call `sdk.Serve(plugin)` from `pkg/plugin/sdk/`; the handshake config is exported from that package as the single source of truth imported by both host and SDK.

Two gRPC services:
- `PluginService`: `GetInfo`, `Init`, `Shutdown` — lifecycle management
- `OperationService`: `ListOperations`, `GetOperation`, `Execute` — operation dispatch

**Key rationale:**
- go-plugin handles mTLS, health checks, and process lifecycle, eliminating custom infrastructure for each concern.
- `netrpc` transport is deprecated in go-plugin v2; gRPC is the maintained path.
- Committed generated files mean `make proto-gen` is optional (regeneration only); no protoc required in CI.
- Process isolation prevents a crashed third-party plugin from taking down the host workflow execution.

**Trade-offs accepted:**
- go-plugin and gRPC become transitive dependencies for plugin authors (via `pkg/plugin/sdk`).
- `map<string, bytes>` fields in protobuf require JSON encode/decode in the SDK bridge — a minor complexity accepted to avoid a flat string-only API.
- Any future wire format change (e.g., additional RPC methods) requires regenerating .pb.go files and releasing a new SDK version.

## Consequences

**What becomes easier:**
- Plugin processes crash without killing the host — isolation boundary enforced by OS.
- mTLS authentication between host and plugin is automatic via go-plugin; no custom cert management.
- Plugin authors have a minimal SDK contract (`sdk.Serve`, one interface) without knowing gRPC details.
- Future services (e.g., `StreamService` for long-running plugins) can be added to the proto without breaking existing plugins.

**What becomes harder:**
- Plugin authors must compile against the SDK and the generated proto types; version pinning is required.
- Debugging requires understanding that plugin execution crosses a process boundary (logs, panics are in the subprocess).
- Changing the protobuf wire format is a breaking change requiring coordinated plugin SDK + host releases.
- The `connectWithTimeout` implementation uses a goroutine+select pattern instead of a context-native approach, because go-plugin's `client.Client()` is not context-aware.

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Hexagonal Architecture | Compliant | gRPC adapters in infrastructure (`grpc_host.go`); domain ports and domain types unchanged; SDK in `pkg/plugin/sdk/` as a common component |
| Go Idioms | Compliant | `context.Context` on all blocking ops; `errors.Join` for multi-error shutdown; goroutine+select for timeout |
| Minimal Abstraction | Compliant | No new domain types or ports; `pluginConnection` struct unexported; conversion isolated to single adapter file |
| Security First | Compliant | mTLS via go-plugin defaults; process isolation for third-party code; no custom cert management surface |
| Test-Driven Development | Compliant | Unit tests per component; integration tests for echo plugin lifecycle |
| Error Taxonomy | Compliant | Binary not found → SYSTEM.IO (exit 4); version mismatch → USER.VALIDATION (exit 1); plugin crash → EXECUTION.PLUGIN (exit 3) |
