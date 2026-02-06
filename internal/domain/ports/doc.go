// Package ports defines the boundary interfaces for the hexagonal architecture.
//
// Port interfaces establish contracts between the domain layer and external concerns,
// enabling dependency inversion and testability. All adapters in the infrastructure
// layer implement these ports, ensuring the domain layer remains pure and agnostic
// of external implementation details.
//
// # Architecture Role
//
// In the hexagonal architecture pattern:
//   - Domain layer defines port interfaces (this package)
//   - Infrastructure layer implements ports as adapters (repositories, executors, stores)
//   - Application layer orchestrates domain operations through ports
//   - Domain layer depends on nothing; all dependencies point inward
//
// This inverted dependency structure allows:
//   - Testing domain logic with mock implementations
//   - Swapping infrastructure implementations without domain changes
//   - Clear boundaries between business logic and technical concerns
//
// # Port Interfaces by Concern
//
// ## Repository Ports
//
// Workflow and template persistence:
//   - WorkflowRepository: Load, list, and check existence of workflow definitions
//   - TemplateRepository: Load, list, and check existence of workflow templates
//
// ## State Persistence Ports
//
// Runtime state and history management:
//   - StateStore: Save, load, delete, and list workflow execution states
//   - HistoryStore: Record, query, and cleanup workflow execution history
//   - PluginStore: Persist plugin state across sessions
//   - PluginConfig: Manage plugin enabled state and configuration
//   - PluginStateStore: Combined persistence and configuration interface
//
// ## Execution Ports
//
// Command and step execution contracts:
//   - CommandExecutor: Execute shell commands via /bin/sh -c with streaming support
//   - CLIExecutor: Execute external binaries directly without shell interpretation
//   - StepExecutor: Execute a single workflow step (used by parallel execution)
//   - ParallelExecutor: Execute multiple branches concurrently with strategy support
//
// ## Agent Integration Ports
//
// AI agent CLI invocation and conversation management:
//   - AgentProvider: Execute AI agent prompts (single-shot and conversation modes)
//   - AgentRegistry: Register, retrieve, and list available agent providers
//   - Tokenizer: Count tokens for context window management (exact or approximate)
//
// ## Plugin System Ports
//
// Plugin lifecycle and operation management:
//   - Plugin: Base interface all plugins must implement (init, shutdown)
//   - PluginManager: Discover, load, initialize, and shutdown plugins
//   - OperationProvider: Execute plugin-provided operations
//   - PluginRegistry: Register and unregister plugin operations
//   - PluginLoader: Discover and load plugins from filesystem
//
// ## Interactive Mode Ports
//
// User interaction during workflow execution:
//   - InteractivePrompt: Step-by-step execution control (run, skip, retry, abort, edit)
//   - InputCollector: Pre-execution collection of missing workflow inputs
//
// ## Expression Evaluation Ports
//
// Runtime expression handling:
//   - ExpressionEvaluator: Evaluate boolean and integer expressions with runtime context
//   - ExpressionValidator: Validate expression syntax at compile-time
//
// ## Logging Ports
//
// Structured logging abstraction:
//   - Logger: Domain logging interface (debug, info, warn, error) with context support
//
// # Usage Patterns
//
// Ports are consumed by:
//  1. Application services (dependency injection)
//  2. Domain entities (passed as parameters when needed)
//  3. Test code (mock implementations)
//
// Example: Injecting ports into application service
//
//	type WorkflowService struct {
//	    repo   ports.WorkflowRepository
//	    store  ports.StateStore
//	    logger ports.Logger
//	}
//
//	func NewWorkflowService(
//	    repo ports.WorkflowRepository,
//	    store ports.StateStore,
//	    executor ports.Executor,
//	    logger ports.Logger,
//	    validator ports.ExpressionValidator,
//	) *WorkflowService {
//	    return &WorkflowService{
//	        repo:      repo,
//	        store:     store,
//	        executor:  executor,
//	        logger:    logger,
//	        validator: validator,
//	    }
//	}
//
// Example: Testing with mock implementations
//
//	func TestWorkflowExecution(t *testing.T) {
//	    mockRepo := testutil.NewMockWorkflowRepository()
//	    mockStore := testutil.NewMockStateStore()
//	    mockLogger := testutil.NewMockLogger()
//
//	    service := NewWorkflowService(mockRepo, mockStore, mockExecutor, mockLogger, mockValidator)
//	    // Test service behavior
//	}
//
// # Port Design Principles
//
// When implementing port interfaces:
//   - Accept context.Context for cancellation and timeout support
//   - Return domain errors (not infrastructure errors) when possible
//   - Use domain types in signatures (workflow.Workflow, not YAML structs)
//   - Keep interfaces small and focused (Interface Segregation Principle)
//   - Document contract expectations and error conditions
//
// # Related Packages
//
//   - internal/domain/workflow: Core domain entities (Workflow, Step, State, Context)
//   - internal/application: Services that consume ports
//   - internal/infrastructure: Concrete implementations of ports
//   - internal/testutil: Mock implementations for testing
package ports
