# Architecture

AWF follows Hexagonal (Ports and Adapters) / Clean Architecture with strict dependency inversion.

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     INTERFACES LAYER                        │
│      CLI (current)  │  API (future)  │  MQ (future)        │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                   APPLICATION LAYER                         │
│   WorkflowService │ ExecutionService │ PluginService        │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                      DOMAIN LAYER                           │
│   Workflow │ Step │ Plugin │ Ports (Interfaces)             │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                  INFRASTRUCTURE LAYER                       │
│   YAMLRepository │ JSONStateStore │ RPCPluginManager        │
└─────────────────────────────────────────────────────────────┘
```

## Dependency Rule

**Domain layer depends on nothing. All other layers depend inward toward domain.**

```
Interfaces → Application → Domain ← Infrastructure
```

- Domain defines interfaces (Ports)
- Infrastructure implements those interfaces (Adapters)
- Application orchestrates domain logic
- Interfaces handle external communication

## Layers

### Domain Layer

The core business logic. No external dependencies.

**Location:** `internal/domain/`

**Components:**
- `workflow/` - Workflow, Step, State, Context entities
- `plugin/` - Plugin manifest, operation schema, state entities
- `ports/` - Repository, StateStore, Executor, PluginManager interfaces

**Key Entities:**
```go
type Workflow struct {
    ID          string
    Name        string
    Version     string
    Description string
    Inputs      []InputDef
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
    Delete(id string) error
}

type Executor interface {
    Execute(ctx context.Context, cmd Command) (Result, error)
}

type PluginManager interface {
    Discover() ([]*plugin.Manifest, error)
    Load(name string) error
    Init(name string, config map[string]interface{}) error
    Shutdown(name string) error
}

// Interactive mode ports follow ISP with focused interfaces:
type StepPresenter interface {
    ShowHeader(workflowName string)
    ShowStepDetails(info *workflow.InteractiveStepInfo)
    ShowExecuting(stepName string)
    ShowStepResult(state *workflow.StepState, nextStep string)
}

type StatusPresenter interface {
    ShowAborted()
    ShowSkipped(stepName string, nextStep string)
    ShowCompleted(status workflow.ExecutionStatus)
    ShowError(err error)
}

type UserInteraction interface {
    PromptAction(hasRetry bool) (workflow.InteractiveAction, error)
    EditInput(name string, current any) (any, error)
    ShowContext(ctx *workflow.RuntimeContext)
}

// Composite for backward compatibility (io.ReadWriter pattern):
type InteractivePrompt interface {
    StepPresenter
    StatusPresenter
    UserInteraction
}
```

### Application Layer

Orchestrates use cases using domain entities and ports.

**Location:** `internal/application/`

**Services:**
- `WorkflowService` - Workflow loading, validation, listing
- `ExecutionService` - Workflow execution engine
- `StateManager` - State persistence management
- `TemplateService` - Template loading and resolution
- `PluginService` - Plugin lifecycle orchestration

**Key Responsibilities:**
- Coordinate domain operations
- Handle transactions/rollbacks
- Implement use case logic
- No direct infrastructure access (only through ports)

### Infrastructure Layer

Implements domain ports with concrete technologies.

**Location:** `internal/infrastructure/`

**Adapters:**
- `repository/` - YAML file loader implementing `Repository`
- `state/` - JSON file store implementing `StateStore`
- `executor/` - Shell executor implementing `Executor`
- `plugin/` - RPC plugin manager, manifest parser, state store
- `logger/` - Zap logger implementation
- `store/` - SQLite history storage
- `xdg/` - XDG directory discovery

**Implementation Details:**

```go
// YAML Repository
type YAMLRepository struct {
    paths []string  // Search paths for workflows
}

func (r *YAMLRepository) Load(name string) (*Workflow, error) {
    // Find and parse YAML file
}

// JSON State Store
type JSONStateStore struct {
    dir string  // Storage directory
}

func (s *JSONStateStore) Save(ctx *ExecutionContext) error {
    // Atomic write via temp file + rename
}

// Shell Executor
type ShellExecutor struct{}

func (e *ShellExecutor) Execute(ctx context.Context, cmd Command) (Result, error) {
    // Execute via /bin/sh -c
}

// RPC Plugin Manager
type RPCPluginManager struct {
    pluginsDir string
    clients    map[string]*plugin.Client
}

func (m *RPCPluginManager) Load(name string) error {
    // Start plugin process via HashiCorp go-plugin
}
```

### Interfaces Layer

External communication adapters.

**Location:** `internal/interfaces/`

**Components:**
- `cli/` - Cobra command handlers
- `cli/ui/` - Terminal UI components (colors, progress bars)

**Future:**
- REST API adapter
- gRPC adapter
- Message queue adapter

## Key Patterns

### Dependency Injection

Services receive dependencies through constructors:

```go
func NewExecutionService(
    repo ports.Repository,
    store ports.StateStore,
    executor ports.Executor,
) *ExecutionService {
    return &ExecutionService{
        repo:     repo,
        store:    store,
        executor: executor,
    }
}
```

### State Machine

Workflow execution follows a state machine pattern:

1. Load initial state
2. Execute state (step/parallel/loop)
3. Evaluate transitions
4. Move to next state
5. Repeat until terminal state

### Atomic Operations

State persistence uses atomic writes:

```go
// Write to temp file
tmpFile := fmt.Sprintf("%s.%d.%d.tmp", path, os.Getpid(), time.Now().UnixNano())
// Write content
// Rename to target (atomic on POSIX)
os.Rename(tmpFile, path)
```

### Context Propagation

Go contexts are used for cancellation and timeouts:

```go
ctx, cancel := context.WithTimeout(parentCtx, timeout)
defer cancel()

result, err := executor.Execute(ctx, cmd)
```

### Parallel Execution

Uses `errgroup` with semaphore for controlled concurrency:

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

## Data Flow

### Workflow Execution

```
CLI Command
    │
    ▼
WorkflowService.Load()
    │
    ▼
ExecutionService.Execute()
    │
    ├──► StateStore.Save() (persist state)
    │
    ├──► Executor.Execute() (run commands)
    │
    └──► StateStore.Load() (resume if interrupted)
```

### Signal Handling

```
SIGINT/SIGTERM
    │
    ▼
Context Cancellation
    │
    ├──► Executor stops command
    │
    ├──► StateStore saves checkpoint
    │
    └──► Clean exit with state preserved
```

## Testing Strategy

- **Domain:** Pure unit tests, no mocks needed
- **Application:** Mock ports for isolation
- **Infrastructure:** Integration tests with real resources
- **Interfaces:** End-to-end CLI tests

See [Testing](testing.md) for details.

## See Also

- [Project Structure](project-structure.md) - Directory organization
- [Testing](testing.md) - Testing conventions
- [Contributing](../../CONTRIBUTING.md) - Development workflow
