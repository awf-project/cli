---
title: "016: HTTP Interface Adapter with Huma v2 and SSE Streaming"
---

**Status**: Accepted
**Date**: 2026-05-17
**Issue**: F097
**Supersedes**: N/A
**Superseded by**: N/A

## Context

ADR-001 listed "API (future)" as a planned interface layer alongside CLI. F097 implements that delivery mechanism: a REST/HTTP API that lets external systems (CI pipelines, dashboards, IDE extensions) trigger, monitor, and query AWF workflow executions without shelling out to the CLI.

Two decisions within F097 are architecturally load-bearing beyond the feature itself:

1. **HTTP framework selection** — introducing Huma v2 + chi v5 as new infrastructure. Huma generates OpenAPI 3.1 schemas automatically from Go types, which locks in how the API contract is expressed and validated for all future endpoints. Replacing it later requires rewriting all handler signatures.

2. **Streaming protocol for execution events** — workflow executions are long-running (seconds to minutes). External subscribers need real-time step updates. The choice between SSE, WebSockets, or a push-from-application model defines the client integration surface for every future streaming feature.

## Candidates

### HTTP Framework

| Option | Pros | Cons |
|--------|------|------|
| **Huma v2 + chi v5** | OpenAPI 3.1 auto-generation from Go types; type-safe input validation; chi is standard `net/http` compatible; no separate doc tooling | Less battle-tested than gin/echo; Huma v2 handler signature is non-standard Go |
| **gin** | Mature, widely known, fast | No native OpenAPI generation; separate swagger annotation toolchain needed; docs drift from code |
| **echo** | Balanced performance and ergonomics | Same OpenAPI gap as gin; less active maintenance |
| **net/http + ogen** (spec-first codegen) | Strict contract; spec drives implementation | Requires maintaining a `.yaml` spec separately; adds codegen step to CI |

### Streaming Protocol

| Option | Pros | Cons |
|--------|------|------|
| **SSE (polling-based)** | Unidirectional; simple client-side (`EventSource` API); firewall-friendly (HTTP/1.1); stateless per subscriber | O(subscribers) polling goroutines; couples cadence to internal poll interval |
| **WebSockets** | Bidirectional; lower per-message overhead for high-frequency events | Overkill for unidirectional workflow events; more complex lifecycle (upgrade, ping/pong, reconnect) |
| **Long-polling** | Universally compatible; no keep-alive concern | Thundering herd on state change; harder to implement correctly at scale |
| **Push from ExecutionService** | Zero polling overhead; events pushed on state transition | Requires a new observer port in the application layer — cross-layer coupling that violates Principle 6 for an interface-layer concern |

## Decision

**Framework:** Huma v2 + chi v5.

Huma v2 is the only Go library that generates valid OpenAPI 3.1 (not 2.0 or 3.0) directly from Go struct types without a separate code-gen step. Chi's standard `net/http` compatibility avoids wrapping the existing request context. AWF's primary API consumers are developer tooling and CI; an always-in-sync OpenAPI spec eliminates documentation maintenance burden.

**Streaming:** SSE with 200ms polling of `ExecutionContext.GetAllStepStates()`.

AWF workflow events are unidirectional (server → client). SSE is the standard HTTP mechanism for this pattern. The 200ms cadence matches the existing TUI poll interval (`tui/tab_monitoring.go:71: monitoringTickInterval`) and satisfies NFR-002 (p95 ≤ 100ms latency at ≤ 50 subscribers). The push-from-ExecutionService alternative was rejected because it would require a new domain port and observer registration pattern — introducing application-layer complexity to solve an interface-layer concern.

**Arch-lint scoping:** Huma and chi are declared as vendor blocks usable only by `interfaces-api`, mirroring how `bubbletea` is scoped to `interfaces-tui`. This prevents accidental import from domain or application layers.

## Consequences

**What becomes easier:**
- External systems integrate with AWF without shelling out to the CLI.
- OpenAPI 3.1 spec is always in sync with the implementation; no separate doc maintenance.
- SSE subscribers can use the standard browser `EventSource` API or `curl --no-buffer`.
- New endpoints follow the established Huma handler pattern without further architectural decisions.

**What becomes harder:**
- Replacing Huma v2 requires rewriting all handler signatures and regenerating the OpenAPI spec.
- SSE polling creates O(subscribers) goroutines per active execution; large subscriber counts require monitoring.
- WebSocket upgrades are not possible through the same SSE endpoint; bidirectional communication would require a separate endpoint and a new framework decision.
- Breaking changes to endpoint paths or response shapes require semver major bumps once external consumers build against the OpenAPI contract.

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Hexagonal Architecture | Compliant | HTTP types confined to `interfaces-api`; Huma + chi vendor-scoped to that layer; infrastructure wiring in `interfaces-cli/serve.go`; no HTTP imports in domain or application |
| Go Idioms | Compliant | Standard `net/http` compatible chi router; `context.Context` propagation through all handlers; SSE goroutines select on `r.Context().Done()` before every poll iteration |
| Minimal Abstraction | Compliant | SSE polling reuses existing `GetAllStepStates()` with no new domain ports; 3 local port interfaces are intentional consumer-defined redundancy per ADR-001 pattern |
| Error Taxonomy | Compliant | Existing `StructuredError` codes map to HTTP semantics via middleware (`USER→400`, `WORKFLOW→422`, `EXECUTION→500`, `SYSTEM→503`); no new exit codes required |
| Security First | Compliant | Default `--host=127.0.0.1` loopback binding; non-loopback is opt-in via `--host`; secret masking unchanged at infrastructure layer |
| Test-Driven Development | Compliant | Unit tests per handler; goroutine-leak test for SSE (delta ≤ 5); 50-concurrent-subscriber test for NFR-002; `make test-race` required before merge |
