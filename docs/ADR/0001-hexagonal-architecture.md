# 0001: Hexagonal Architecture

**Status**: Accepted
**Date**: 2025-12-01
**Supersedes**: N/A
**Superseded by**: N/A

## Context

AWF is a CLI tool that orchestrates AI agents through YAML workflows. The core domain (workflows, state machines, execution context) must remain independent from delivery mechanisms (CLI today, API/MQ future) and infrastructure choices (YAML parsing, JSON state store, shell execution).

Without strict boundary enforcement, domain logic leaks into CLI handlers or infrastructure adapters, making testing painful and swapping implementations expensive.

## Candidates

| Option | Pros | Cons |
|--------|------|------|
| Hexagonal/Clean Architecture | Testable domain, swappable adapters, clear boundaries | More files, indirection cost, strict discipline required |
| Layered (traditional MVC) | Familiar, less boilerplate | Domain couples to framework, hard to test without DB/CLI |
| Flat structure (single package) | Simple, fast to start | Becomes unmaintainable past ~5K LOC, circular deps |

## Decision

Adopt hexagonal architecture with four layers and strict dependency inversion:

```
internal/
├── domain/          # Entities, value objects, ports (interfaces)
├── application/     # Use cases, services (WorkflowService, ExecutionService)
├── infrastructure/  # Adapters (YAMLRepository, JSONStateStore, ShellExecutor)
└── interfaces/      # Entry points (cli/)
```

Rules:
- Domain layer depends on nothing
- Application layer depends only on domain
- Infrastructure layer implements domain ports
- Interfaces layer depends on application
- Ports (interfaces) defined in domain, implemented in infrastructure

## Consequences

**What becomes easier:**
- Unit testing domain and application layers without infrastructure
- Adding new delivery mechanisms (API, MQ) without touching domain
- Swapping infrastructure (e.g., SQLite → PostgreSQL) by implementing same port

**What becomes harder:**
- Simple features require touching multiple layers
- New contributors need to understand the layer boundaries
- go-arch-lint rules must be maintained to enforce boundaries

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Hexagonal Architecture | Compliant | This ADR defines the principle |
| Go Idioms | Compliant | Interfaces in consumer package, composition over inheritance |
| Minimal Abstraction | Compliant | Ports only where multiple implementations exist or are planned |
