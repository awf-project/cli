---
title: "004: Domain Operation Registry with Infrastructure Coexistence"
---

**Status**: Accepted
**Date**: 2025-12-01
**Supersedes**: N/A
**Superseded by**: N/A

## Context

AWF supports plugin operations (GitHub, notifications, HTTP) that extend workflow capabilities. Operations need registration, discovery, and execution. The question is where the registry lives: domain (pure, testable) or infrastructure (concrete, direct access to adapters).

Both registries serve different lifecycle concerns: domain owns the contract, infrastructure owns the wiring.

## Candidates

| Option | Pros | Cons |
|--------|------|------|
| Domain registry only | Pure, testable, no infra dependency | Can't auto-discover infrastructure plugins |
| Infrastructure registry only | Direct access to adapters, auto-wiring | Domain depends on infrastructure, untestable |
| Composite (domain + infrastructure) | Separation of concerns, testable domain, flexible infra | Two registries to maintain, routing complexity |

## Decision

Implement OperationRegistry in domain layer implementing `ports.OperationProvider`. Infrastructure maintains its own plugin registry for lifecycle management (init, cleanup). A CompositeOperationProvider in application layer merges both.

Rules:
- Domain registry: pure Go, no external deps, implements port directly on domain type
- Infrastructure registry: manages concrete adapters (HTTP providers, etc.)
- Application layer wires both via CompositeOperationProvider
- Preserve both registries — never collapse into one

## Consequences

**What becomes easier:**
- Unit testing operations without infrastructure
- Adding pure-domain operations (validators, transformers)
- Infrastructure plugins have independent lifecycle (init/cleanup)

**What becomes harder:**
- Operation lookup traverses two registries
- New contributors must understand the composite pattern

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Hexagonal Architecture | Compliant | Domain defines port, infrastructure implements adapter |
| Go Idioms | Compliant | Interface in consumer (domain), struct in provider (infrastructure) |
| Minimal Abstraction | Compliant | Composite justified by two concrete lifecycle needs |
