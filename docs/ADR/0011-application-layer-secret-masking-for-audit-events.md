# 0011. Application-Layer Secret Masking for Audit Events

Date: 2026-02-21
Status: Accepted
Issue: F071

## Context

Workflow inputs may contain secrets (API keys, passwords, tokens). The audit trail must mask these values before writing. The question is which layer owns the masking responsibility, given hexagonal architecture constraints.

## Decision

The application layer (`ExecutionService`) masks inputs before constructing `AuditEvent`, using an inlined prefix-check function (mirroring `SecretMasker.IsSecretKey()` logic). The `AuditTrailWriter` port accepts pre-sanitized `AuditEvent` structs with no knowledge of secrets.

Alternatives rejected:
- **Approach B (masking callback in port)** — introduces a masking interface for one consumer, violating Minimal Abstraction. Port complexity grows without benefit.
- **Approach C (infrastructure-only, no port)** — couples CLI interfaces directly to infrastructure; violates hexagonal architecture; not testable without file I/O.

## Consequences

### Positive
- Matches existing `HistoryStore` pattern: application layer transforms domain data before persistence.
- Port stays minimal (`Write` + `Close`); infrastructure adapter contains no business logic.
- Masking testable in application-layer unit tests without infrastructure dependencies.

### Negative
- Application layer contains masking logic (a few lines) rather than delegating to the infrastructure `SecretMasker`. If masking patterns change, both `masker.go` and `execution_service.go` must be updated.
