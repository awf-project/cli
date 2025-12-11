# F001: Architecture Hexagonale de Base

## Metadata
- **Status**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: critical
- **Estimation**: L

## Description

Set up the foundational hexagonal (ports & adapters) architecture for AWF. This establishes the project structure, dependency inversion principles, and layer boundaries that all other features build upon.

The domain layer contains pure business logic with no external dependencies. Application layer orchestrates use cases. Infrastructure provides concrete implementations. Interfaces layer handles CLI interactions.

## Acceptance Criteria

- [x] Project compiles with `go build ./...`
- [x] Domain layer has zero external imports (except stdlib)
- [x] All layer dependencies point inward (toward domain)
- [x] Port interfaces defined in domain/ports/
- [x] Adapters implement port interfaces
- [x] Dependency injection via constructors (no globals)

## Dependencies

- **Blocked by**: _none_
- **Unblocks**: F002, F003, F004, F005, F006, F007, F008

## Impacted Files

```
cmd/awf/main.go
internal/domain/workflow/workflow.go
internal/domain/workflow/step.go
internal/domain/workflow/context.go
internal/domain/ports/repository.go
internal/domain/ports/executor.go
internal/domain/ports/store.go
internal/domain/ports/logger.go
internal/application/service.go
internal/infrastructure/
internal/interfaces/cli/
go.mod
Makefile
```

## Technical Tasks

- [x] Initialize Go module
  - [x] `go mod init github.com/vanoix/awf`
  - [x] Add core dependencies (cobra, yaml.v3, zap)
- [x] Create directory structure
  - [x] cmd/awf/
  - [x] internal/domain/workflow/
  - [x] internal/domain/ports/
  - [x] internal/application/
  - [x] internal/infrastructure/
  - [x] internal/interfaces/cli/
  - [x] pkg/
- [x] Define domain entities
  - [x] Workflow struct
  - [x] Step struct
  - [x] ExecutionContext struct
- [x] Define port interfaces
  - [x] WorkflowRepository
  - [x] StateStore
  - [x] CommandExecutor
  - [x] Logger
- [x] Create application service skeleton
- [x] Set up CLI entry point with Cobra
- [x] Create Makefile with build/test targets
- [x] Verify compilation

## Notes

Architecture diagram:
```
Interfaces → Application → Domain ← Infrastructure
                              ↑
                            Ports
```

Domain must be pure Go with no framework dependencies. Use constructor injection for all dependencies.
