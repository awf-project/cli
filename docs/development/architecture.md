---
title: "Architecture"
---

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
│   Workflow │ Step │ Plugin │ Operation │ Ports (Interfaces)  │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                  INFRASTRUCTURE LAYER                       │
│ YAMLRepository │ JSONStateStore │ AgentProviders │ GitHub │ Notify │
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
- `operation/` - Operation interface, OperationRegistry, input validation (ValidateInputs)
- `ports/` - Repository, StateStore, Executor, PluginManager, OperationProvider interfaces

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

**Operation Interface and Registry (F057):**
```go
// Operation defines the contract for executable operations.
type Operation interface {
    Name() string
    Execute(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error)
    Schema() *plugin.OperationSchema
}

// OperationRegistry manages operation lifecycle with thread-safe access.
// Implements ports.OperationProvider for seamless ExecutionService integration.
type OperationRegistry struct {
    operations map[string]Operation
    mu         sync.RWMutex
}
```

The `Operation` interface is distinct from `OperationProvider`: an `Operation` is a single executable operation (e.g., `http.get`), while `OperationProvider` manages a collection. The registry bridges the two by implementing `OperationProvider` using registered `Operation` instances.

`ValidateInputs(schema, inputs)` is a standalone function that checks required fields, type correctness (`string`, `integer`, `boolean`, `array`, `object`), and applies defaults for optional inputs. It handles JSON float64→int coercion.

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
    // Discover finds plugins in the plugins directory.
    Discover(ctx context.Context) ([]*plugin.PluginInfo, error)
    // Load loads a plugin by name.
    Load(ctx context.Context, name string) error
    // Init initializes a loaded plugin.
    Init(ctx context.Context, name string, config map[string]any) error
    // Shutdown stops a running plugin.
    Shutdown(ctx context.Context, name string) error
    // ShutdownAll stops all running plugins.
    ShutdownAll(ctx context.Context) error
    // Get returns plugin info by name.
    Get(name string) (*plugin.PluginInfo, bool)
    // List returns all known plugins.
    List() []*plugin.PluginInfo
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
- `agents/` - AI agent providers (Claude, Gemini, Codex, OpenCode, OpenAI-Compatible) implementing `AgentProvider`
- `config/` - Configuration file loading
- `diagram/` - Workflow diagram generation (DOT/Graphviz)
- `errors/` - Error formatting adapters implementing `ErrorFormatter`
- `executor/` - Shell executor implementing `Executor`
- `expression/` - Expression evaluator implementing `ExpressionEvaluator` and `ExpressionValidator`
- `github/` - Built-in GitHub operation provider implementing `OperationProvider` (issue/PR/label/project operations, batch executor, auth fallback)
- `logger/` - Zap logger implementation (console, JSON, multi-logger, secret masking)
- `notify/` - Built-in notification operation provider implementing `OperationProvider` (desktop, webhook backends)
- `pluginmgr/` - Plugin lifecycle (manifest, state, gRPC connections); delegates transport to `pkg/registry/`
- `repository/` - YAML file loader implementing `Repository`
- `store/` - JSON state store implementing `StateStore`, SQLite history storage
- `tokenizer/` - Token counting for conversation context management
- `xdg/` - XDG directory discovery

**Shared Packages (`pkg/`):**
- `pkg/registry/` - Shared GitHub Releases transport (versioning, downloads, checksum verification) used by plugin system and workflow pack system
- `pkg/httpx/` - HTTP client abstractions (`HTTPDoer` interface, size-limited reads)
- `pkg/plugin/sdk/` - Plugin author SDK (`Serve()`, `BasePlugin`, input helpers)

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
    // Execute via detected shell ($SHELL or /bin/sh fallback)
}

// RPC Plugin Manager (gRPC transport via HashiCorp go-plugin, C067)
type RPCPluginManager struct {
    pluginsDir  string
    plugins     map[string]*PluginInfo
    connections map[string]*pluginConnection  // gRPC connections protected by mu RWMutex
    mu          sync.RWMutex
}

func (m *RPCPluginManager) Init(ctx context.Context, name string) error {
    // 1. Discover binary at ~/.local/share/awf/plugins/<name>/awf-plugin-<name>
    // 2. Verify version compatibility from plugin.yaml
    // 3. Start plugin process via HashiCorp go-plugin
    // 4. Establish gRPC connection with 5s timeout (NFR-002)
    // 5. Call PluginService.Init RPC
}

func (m *RPCPluginManager) Execute(ctx context.Context, op string, inputs map[string]any) (*OperationResult, error) {
    // 1. Route operation to correct plugin based on op name
    // 2. Call OperationService.Execute via gRPC
    // 3. Convert proto OperationResult to domain type
    // 4. Inject PluginName from connection context
}

func (m *RPCPluginManager) Shutdown(ctx context.Context, name string) error {
    // 1. Call PluginService.Shutdown RPC
    // 2. Kill process via go-plugin
    // 3. Clean up connection from map
}

func (m *RPCPluginManager) ShutdownAll(ctx context.Context) error {
    // 1. For each running plugin, set 5s deadline
    // 2. Call Shutdown on each
    // 3. Kill all processes unconditionally
    // 4. Accumulate errors with errors.Join()
}
```

**Plugin System Architecture:**

AWF uses HashiCorp go-plugin with gRPC for external plugin transport (ADR 015). Plugins are separate processes that communicate with the host via typed gRPC services:

```go
// PluginService: plugin lifecycle management
service PluginService {
    rpc GetInfo(GetInfoRequest) returns (PluginInfo) {}
    rpc Init(InitRequest) returns (InitResponse) {}
    rpc Shutdown(ShutdownRequest) returns (ShutdownResponse) {}
}

// OperationService: operation dispatch
service OperationService {
    rpc ListOperations(ListRequest) returns (ListResponse) {}
    rpc GetOperation(GetRequest) returns (OperationSchema) {}
    rpc Execute(ExecuteRequest) returns (ExecuteResponse) {}
}
```

**Key Features:**
- **Process Isolation** — Plugin crashes don't affect host workflow execution
- **Version Compatibility** — Manifest version checked before Init; incompatible plugins fail with exit code 1
- **Thread-Safe** — connections map protected by RWMutex; concurrent Execute calls validated with `-race`
- **Timeout Protection** — Plugin startup enforced with 5s deadline via goroutine+select pattern
- **Error Handling** — Binary not found (exit 4), version mismatch (exit 1), plugin crash (exit 3)

**Plugin Discovery:**

Plugins are discovered from `$XDG_DATA_HOME/awf/plugins/` (default: `~/.local/share/awf/plugins/`). Each plugin requires:
- Directory named `awf-plugin-<name>`
- Executable binary `awf-plugin-<name>` in that directory
- Manifest `plugin.yaml` with version and capability declarations

**SDK for Plugin Authors:**

Plugin authors implement the SDK interface and call `sdk.Serve()` from main:

```go
type MyPlugin struct {
    sdk.BasePlugin
}

func (p *MyPlugin) Operations() []string {
    return []string{"my_op"}
}

func (p *MyPlugin) HandleOperation(ctx context.Context, name string, inputs map[string]any) (*sdk.OperationResult, error) {
    return sdk.NewSuccessResult(output, data), nil
}

func main() {
    sdk.Serve(&MyPlugin{
        BasePlugin: sdk.BasePlugin{
            PluginName:    "awf-plugin-myplugin",
            PluginVersion: "1.0.0",
        },
    })
}
```

The SDK handles:
- go-plugin handshake and lifecycle
- gRPC server setup and method delegation
- JSON serialization for `map[string]any` fields
- Input validation helpers (GetStringDefault, GetIntDefault, etc.)
- Result construction (NewSuccessResult, NewErrorResult)

**Built-in Providers:**

AWF ships with 3 built-in operation providers:
- `github.*` — Issue/PR operations (in-process, zero IPC overhead)
- `http.request` — REST API calls (in-process)
- `notify.send` — Workflow completion alerts (in-process)

Built-in providers are synthesized as PluginInfo entries and appear in `awf plugin list` with type `builtin`. They can be disabled/enabled like external plugins.

**Error Handling:**

Plugin errors map to domain error codes:
- Binary not found → SYSTEM.IO (exit 4)
- Version incompatible → USER.VALIDATION (exit 1)
- Plugin crash during Execute → EXECUTION.PLUGIN (exit 3)
- Connection timeout → EXECUTION.TIMEOUT (exit 3)

See [ADR-015](../ADR/015-grpc-go-plugin-transport-for-external-plugins.md) for design rationale and trade-offs.

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

### Public Packages

Reusable utilities safe for external consumption.

**Location:** `pkg/`

**Packages:**
- `interpolation/` - Template variable substitution and shell escaping
- `registry/` - Package/plugin registry transport layer (C070) — GitHub Releases API client, semantic versioning, download & extraction utilities
- `validation/` - Input validation rule evaluation
- `retry/` - Backoff strategies (exponential, linear, constant)

These packages have no domain/infrastructure dependencies and can be imported by external projects.

## Key Patterns

### Dependency Injection

Services receive dependencies through constructors:

```go
func NewWorkflowService(
    repo ports.Repository,
    store ports.StateStore,
    executor ports.Executor,
    logger ports.Logger,
    validator ports.ExpressionValidator,
) *WorkflowService {
    return &WorkflowService{
        repo:      repo,
        store:     store,
        executor:  executor,
        logger:    logger,
        validator: validator,
    }
}
```

All dependencies are domain port interfaces, never infrastructure implementations. The interfaces layer (CLI) wires concrete adapters at the composition root.

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

## Interface Design Decisions

### ExpressionValidator: Constructor Injection (C051)

The `WorkflowService` previously instantiated `expression.NewExprValidator()` directly, violating the Dependency Inversion Principle by importing infrastructure from the application layer. This was fixed by:

1. Adding `validator ports.ExpressionValidator` as a constructor parameter
2. Removing the `internal/infrastructure/expression` import from `service.go`
3. Wiring the concrete `ExpressionValidator` adapter at CLI call sites (composition root)

The `ExpressionValidator` port interface was already defined in `internal/domain/ports/expression_validator.go`. The fix makes the application layer depend only on the port abstraction.

### Architecture Enforcement (C051)

Architecture constraints are enforced automatically using [go-arch-lint](https://github.com/fe3dback/go-arch-lint), configured in `.go-arch-lint.yml`. This provides AST-based validation of the full dependency graph across all layers.

**Usage**:
```bash
make lint-arch       # Check architecture constraints
make lint-arch-map   # Show component-to-package mapping
```

**Integration**:
- `lint-arch` target is integrated into `make quality` gate
- Runs in CI via `.github/workflows/quality.yml` on every push
- Blocks merge if violations detected
- All dependencies checked via AST analysis (deepScan mode)

**Configuration Structure** (`.go-arch-lint.yml`):

| Section | Purpose |
|---------|---------|
| `version: 3` | Configuration version |
| `workdir: internal` | Scope for component paths |
| `commonVendors` | Shared libraries (all components) |
| `commonComponents` | Shared packages (all components) |
| `vendors` | Vendor library definitions |
| `components` | 20 components across 4 layers |
| `deps` | Dependency rules per component |

**Validation Rules by Layer**:

| Layer | Count | Max Dependencies | Rationale |
|-------|-------|---------|-----------|
| Domain | 5 | stdlib + sync only | Pure business logic |
| Application | 1 | domain only + stdlib | Orchestration, no infra |
| Infrastructure | 12 | domain + vendors + stdlib | Concrete implementations |
| Interfaces | 2 | all layers + stdlib | Delivery/wiring |
| Testutil | 1 | domain + selected infra | Test helpers |

**Example: Application Layer Rule**:
```yaml
application:
  mayDependOn:
    - domain-workflow
    - domain-ports
    - domain-errors
    - domain-plugin
  canUse:
    - go-stdlib
    - go-sync
```

This means the application layer **can** depend on domain components and stdlib only — any attempt to import from infrastructure, vendor libraries, or other layers will fail the build.

**Debugging Component Mapping**:
```bash
$ make lint-arch-map
Component mapping:
  domain-workflow     → internal/domain/workflow
  domain-ports        → internal/domain/ports
  application         → internal/application
  infra-expression    → internal/infrastructure/expression
  interfaces-cli      → internal/interfaces/cli
  ... (14 more)
```

**C042 + C051 Alignment**:
- **C042** tests verify DIP compliance at runtime via integration tests
- **C051** enforces DIP compliance at build time via static analysis
- Together they provide defense-in-depth: testing catches logic errors, linting catches architectural violations

### PluginManager: Keep Unified (C050)

The PluginManager interface (7 methods) was evaluated for ISP compliance and kept unified.

**Rationale:**
- Single consumer: Only PluginService uses PluginManager, calling all 7 methods
- Cross-concern coupling: DisablePlugin uses both Get() (query) and Shutdown() (lifecycle) together
- 7 methods is within acceptable threshold (Go stdlib net.Conn has 8)
- Plugin subsystem already has 8 focused interfaces averaging 3.75 methods each

**Contrast with C049:** InteractivePrompt (11 methods, 3 consumers) was split into focused interfaces because different consumers needed distinct method subsets.

## See Also

- [Project Structure](project-structure.md) - Directory organization
- [Testing](testing.md) - Testing conventions
- [Contributing](../../CONTRIBUTING.md) - Development workflow
