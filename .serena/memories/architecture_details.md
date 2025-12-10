# Architecture Details

## Hexagonal Architecture Layers

```
┌─────────────────────────────────────────────────────────────┐
│                     INTERFACES LAYER                        │
│  internal/interfaces/cli/                                   │
│  - Cobra commands (run, validate, list, status)            │
│  - Dependency injection wiring                              │
│  - Output formatting (ui/)                                  │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                   APPLICATION LAYER                         │
│  internal/application/                                      │
│  - WorkflowService: CRUD operations                        │
│  - ExecutionService: State machine execution               │
│  - HookExecutor: Hook processing                           │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                      DOMAIN LAYER                           │
│  internal/domain/workflow/: Entities                       │
│  internal/domain/ports/: Interfaces                        │
│  CRITICAL: ZERO external dependencies                      │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                  INFRASTRUCTURE LAYER                       │
│  internal/infrastructure/                                   │
│  - repository/: YAML workflow loading                      │
│  - store/: JSON state persistence                          │
│  - executor/: Shell command execution                      │
│  - logger/: Console/JSON logging                           │
└─────────────────────────────────────────────────────────────┘
```

## Port Interfaces (internal/domain/ports/)

| Port | Purpose | Implementations |
|------|---------|-----------------|
| `WorkflowRepository` | Load/List/Exists workflows | `YAMLRepository`, `CompositeRepository` |
| `CommandExecutor` | Execute shell commands | `ShellExecutor` |
| `StateStore` | Save/Load execution state | `JSONStore` |
| `Logger` | Logging contract | `ConsoleLogger`, `JSONLogger`, `MultiLogger` |

## Key Design Decisions

### Command Execution
- Uses `/bin/sh -c` with shell string (not argument array)
- `ShellEscape()` in `pkg/interpolation` for user inputs
- Process groups for clean termination

### State Persistence
- Atomic writes via temp file + rename
- File locking with `syscall.Flock`
- Unique temp files: `{id}.{pid}.{nanos}.tmp`

### Parallel Execution
- `errgroup` with semaphore for controlled concurrency
- Strategies: `all_succeed`, `any_succeed`, `best_effort`

### Security
- Secret masking in logs (`SECRET_`, `API_KEY`, `PASSWORD`)
- Command injection prevention via escaping
- No shell expansion of untrusted input

## Important Patterns

### Dependency Injection (CLI layer)
```go
repo := NewWorkflowRepository()           // Returns *CompositeRepository
stateStore := store.NewJSONStore(...)     // Returns *JSONStore
shellExecutor := executor.NewShellExecutor()
wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger)
```

### Error Aggregation (Go 1.20+)
```go
var parseErrors []error
// ... collect errors ...
if len(parseErrors) > 0 {
    return errors.Join(parseErrors...)
}
```
