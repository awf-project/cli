# AWF Architecture

## Overview

AWF follows Hexagonal (Ports and Adapters) / Clean Architecture.

```
┌─────────────────────────────────────────────────────────────┐
│                     INTERFACES LAYER                        │
│      CLI (current)  │  API (future)  │  MQ (future)        │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                   APPLICATION LAYER                         │
│   WorkflowService │ ExecutionService │ StateManager         │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                      DOMAIN LAYER                           │
│   Workflow │ Step │ ExecutionContext │ Ports (Interfaces)   │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                  INFRASTRUCTURE LAYER                       │
│   YAMLRepository │ JSONStateStore │ ShellExecutor          │
└─────────────────────────────────────────────────────────────┘
```

## Dependency Rule

**Domain layer depends on nothing. All other layers depend inward.**

```
Interfaces → Application → Domain ← Infrastructure
```

## Project Structure

```
cmd/awf/main.go              # CLI entry point
internal/
├── domain/
│   ├── workflow/            # Workflow, Step, State entities
│   └── ports/               # Repository, StateStore, Executor interfaces
├── application/             # WorkflowService, ExecutionService
├── infrastructure/          # YAML repo, JSON store, Shell executor
└── interfaces/cli/          # Cobra commands
pkg/                         # Public: interpolation, validation, retry
```

## Domain Layer

Core business logic. No external dependencies.

**Location:** `internal/domain/`

**Key Entities:**

```go
type Workflow struct {
    ID          string
    Name        string
    States      map[string]State
    Initial     string
}

type State interface {
    GetName() string
    GetType() StateType
}

type ExecutionContext struct {
    WorkflowID string
    Inputs     map[string]interface{}
    States     map[string]StateResult
}
```

**Ports (Interfaces):**

```go
type Repository interface {
    Load(name string) (*Workflow, error)
    List() ([]WorkflowInfo, error)
}

type StateStore interface {
    Save(ctx *ExecutionContext) error
    Load(id string) (*ExecutionContext, error)
}

type Executor interface {
    Execute(ctx context.Context, cmd Command) (Result, error)
}
```

## Application Layer

Orchestrates use cases using domain and ports.

**Location:** `internal/application/`

**Services:**
- `WorkflowService` - Loading, validation, listing
- `ExecutionService` - Execution engine
- `StateManager` - State persistence
- `TemplateService` - Template resolution

## Infrastructure Layer

Implements domain ports with concrete tech.

**Location:** `internal/infrastructure/`

**Adapters:**
- `repository/` - YAML file loader
- `state/` - JSON state store
- `executor/` - Shell executor
- `history/` - BadgerDB history

## Key Patterns

### Dependency Injection

```go
func NewExecutionService(
    repo ports.Repository,
    store ports.StateStore,
    executor ports.Executor,
) *ExecutionService
```

### State Machine Execution

1. Load initial state
2. Execute state
3. Evaluate transitions
4. Move to next state
5. Repeat until terminal

### Atomic Operations

```go
// Write to temp, then rename (atomic on POSIX)
tmpFile := fmt.Sprintf("%s.%d.%d.tmp", path, os.Getpid(), time.Now().UnixNano())
os.WriteFile(tmpFile, data, 0644)
os.Rename(tmpFile, path)
```

### Parallel Execution

```go
g, ctx := errgroup.WithContext(ctx)
sem := make(chan struct{}, maxConcurrent)

for _, step := range steps {
    g.Go(func() error {
        sem <- struct{}{}
        defer func() { <-sem }()
        return executeStep(ctx, step)
    })
}
return g.Wait()
```

## Build Commands

```bash
make build          # Build to ./bin/awf
make install        # Install to /usr/local/bin
make test           # All tests
make test-unit      # Unit tests
make lint           # golangci-lint
make fmt            # go fmt
```

## Testing Strategy

- **Domain:** Pure unit tests
- **Application:** Mock ports
- **Infrastructure:** Integration tests
- **Interfaces:** E2E CLI tests
