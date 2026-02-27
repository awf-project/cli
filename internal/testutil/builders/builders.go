package builders

import (
	"io"
	"sync"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
)

// This file contains fluent builder implementations for constructing test services,
// workflows, steps, and execution contexts with sensible defaults.

// ExecutionServiceBuilder provides a fluent API for constructing ExecutionService instances
// with sensible defaults, following the MockProvider pattern (ADR-003).
// Reduces test setup from 30+ lines to 2-3 lines (93% reduction).
// Thread-safe: uses sync.RWMutex to protect concurrent access.
type ExecutionServiceBuilder struct {
	mu         sync.RWMutex
	logger     ports.Logger
	executor   ports.CommandExecutor
	store      ports.StateStore
	repository ports.WorkflowRepository
	registry   ports.AgentRegistry
	evaluator  interface{} // application.ExpressionEvaluator - kept as interface{} to avoid circular import
	validator  ports.ExpressionValidator
	stdout     io.Writer
	stderr     io.Writer
}

// NewExecutionServiceBuilder creates a new ExecutionServiceBuilder with default dependencies.
// All dependencies default to mock implementations suitable for testing.
func NewExecutionServiceBuilder() *ExecutionServiceBuilder {
	return &ExecutionServiceBuilder{
		logger:     nil, // Will use default in Build()
		executor:   nil, // Will use default in Build()
		store:      nil, // Will use default in Build()
		repository: nil, // Will use default in Build()
		registry:   nil, // Will use default in Build()
		stdout:     nil, // Will use default in Build()
		stderr:     nil, // Will use default in Build()
	}
}

// WithLogger configures the logger for the ExecutionService.
// If nil, a default MockLogger will be used.
func (b *ExecutionServiceBuilder) WithLogger(logger ports.Logger) *ExecutionServiceBuilder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logger = logger
	return b
}

// WithExecutor configures the command executor for the ExecutionService.
// If nil, a default MockCommandExecutor will be used.
func (b *ExecutionServiceBuilder) WithExecutor(executor ports.CommandExecutor) *ExecutionServiceBuilder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.executor = executor
	return b
}

// WithStateStore configures the state store for the ExecutionService.
// If nil, a default MockStateStore will be used.
func (b *ExecutionServiceBuilder) WithStateStore(store ports.StateStore) *ExecutionServiceBuilder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.store = store
	return b
}

// WithWorkflowRepository configures the workflow repository for the ExecutionService.
// If nil, a default MockWorkflowRepository will be used.
func (b *ExecutionServiceBuilder) WithWorkflowRepository(repository ports.WorkflowRepository) *ExecutionServiceBuilder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.repository = repository
	return b
}

// WithAgentRegistry configures the agent registry for the ExecutionService.
// If nil, a default AgentRegistry will be created.
func (b *ExecutionServiceBuilder) WithAgentRegistry(registry ports.AgentRegistry) *ExecutionServiceBuilder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.registry = registry
	return b
}

// WithEvaluator configures the expression evaluator for conditional transitions.
// Accepts application.ExpressionEvaluator interface for evaluating "when" clauses.
func (b *ExecutionServiceBuilder) WithEvaluator(evaluator interface{}) *ExecutionServiceBuilder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.evaluator = evaluator
	return b
}

// WithValidator configures the expression validator for workflow validation.
// If nil, no expression validation will be performed (safe for tests not exercising ValidateWorkflow).
func (b *ExecutionServiceBuilder) WithValidator(validator ports.ExpressionValidator) *ExecutionServiceBuilder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.validator = validator
	return b
}

// WithOutputWriters configures stdout and stderr writers for the ExecutionService.
// If nil, default writers will be used (typically os.Stdout/os.Stderr).
func (b *ExecutionServiceBuilder) WithOutputWriters(stdout, stderr io.Writer) *ExecutionServiceBuilder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stdout = stdout
	b.stderr = stderr
	return b
}

// Build constructs the ExecutionService with the configured dependencies.
// Uses sensible defaults for any components not explicitly set.
// Returns a fully initialized ExecutionService ready for testing.
func (b *ExecutionServiceBuilder) Build() *application.ExecutionService {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Apply defaults for nil dependencies
	logger := b.logger
	if logger == nil {
		logger = mocks.NewMockLogger()
	}

	executor := b.executor
	if executor == nil {
		executor = mocks.NewMockCommandExecutor()
	}

	store := b.store
	if store == nil {
		store = mocks.NewMockStateStore()
	}

	repository := b.repository
	if repository == nil {
		repository = mocks.NewMockWorkflowRepository()
	}

	registry := b.registry
	if registry == nil {
		registry = mocks.NewMockAgentRegistry()
	}

	// Note: stdout/stderr are stored in builder for future extensibility
	// but not currently used by ExecutionService constructor
	_ = b.stdout
	_ = b.stderr

	// Create WorkflowService with expression validator
	// Use injected validator (nil is safe for tests not exercising ValidateWorkflow)
	workflowSvc := application.NewWorkflowService(repository, store, executor, logger, b.validator)

	// Create ParallelExecutor
	parallelExecutor := application.NewParallelExecutor(logger)

	// Create TemplateResolver for interpolation
	resolver := interpolation.NewTemplateResolver()

	// Create HistoryService (needs a MockHistoryStore)
	historyStore := mocks.NewMockHistoryStore()
	historySvc := application.NewHistoryService(historyStore, logger)

	// Create ExecutionService
	svc := application.NewExecutionService(
		workflowSvc,
		executor,
		parallelExecutor,
		store,
		logger,
		resolver,
		historySvc,
	)

	// Configure agent registry (C038: Use MockAgentRegistry as default)
	svc.SetAgentRegistry(registry)

	// Configure expression evaluator if provided (for conditional transitions)
	if b.evaluator != nil {
		// Type assertion to ports.ExpressionEvaluator (C042: migrated to port interface)
		if evaluator, ok := b.evaluator.(ports.ExpressionEvaluator); ok {
			svc.SetEvaluator(evaluator)
		}
	}

	return svc
}

// WorkflowBuilder provides a fluent API for constructing Workflow instances
// with sensible defaults, following the builder pattern (ADR-003).
// Simplifies test workflow creation with progressive configuration.
type WorkflowBuilder struct {
	workflow *workflow.Workflow
}

// NewWorkflowBuilder creates a new WorkflowBuilder with sensible defaults.
// Default workflow has:
// - Name: "test-workflow"
// - Initial: "start"
// - One step named "start" of type terminal with success status
func NewWorkflowBuilder() *WorkflowBuilder {
	return &WorkflowBuilder{
		workflow: &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name:   "start",
					Type:   workflow.StepTypeTerminal,
					Status: workflow.TerminalSuccess,
				},
			},
		},
	}
}

// WithName sets the workflow name.
func (b *WorkflowBuilder) WithName(name string) *WorkflowBuilder {
	b.workflow.Name = name
	return b
}

// WithDescription sets the workflow description.
func (b *WorkflowBuilder) WithDescription(description string) *WorkflowBuilder {
	b.workflow.Description = description
	return b
}

// WithVersion sets the workflow version.
func (b *WorkflowBuilder) WithVersion(version string) *WorkflowBuilder {
	b.workflow.Version = version
	return b
}

// WithAuthor sets the workflow author.
func (b *WorkflowBuilder) WithAuthor(author string) *WorkflowBuilder {
	b.workflow.Author = author
	return b
}

// WithTags sets the workflow tags.
func (b *WorkflowBuilder) WithTags(tags ...string) *WorkflowBuilder {
	b.workflow.Tags = tags
	return b
}

// WithInitial sets the initial state name.
func (b *WorkflowBuilder) WithInitial(initial string) *WorkflowBuilder {
	b.workflow.Initial = initial
	return b
}

// WithStep adds a step to the workflow.
// If a step with the same name exists, it is replaced.
func (b *WorkflowBuilder) WithStep(step *workflow.Step) *WorkflowBuilder {
	if b.workflow.Steps == nil {
		b.workflow.Steps = make(map[string]*workflow.Step)
	}
	b.workflow.Steps[step.Name] = step
	return b
}

// WithSteps adds multiple steps to the workflow.
func (b *WorkflowBuilder) WithSteps(steps ...*workflow.Step) *WorkflowBuilder {
	for _, step := range steps {
		b.WithStep(step)
	}
	return b
}

// WithInput adds an input parameter to the workflow.
func (b *WorkflowBuilder) WithInput(input *workflow.Input) *WorkflowBuilder {
	b.workflow.Inputs = append(b.workflow.Inputs, *input)
	return b
}

// WithEnv adds required environment variables.
func (b *WorkflowBuilder) WithEnv(vars ...string) *WorkflowBuilder {
	b.workflow.Env = append(b.workflow.Env, vars...)
	return b
}

// Build returns the constructed Workflow.
func (b *WorkflowBuilder) Build() *workflow.Workflow {
	return b.workflow
}

// StepBuilder provides a fluent API for constructing Step instances
// with sensible defaults for different step types.
// Supports all step types: command, parallel, terminal, for_each, while, operation, call_workflow, agent.
type StepBuilder struct {
	step *workflow.Step
}

// NewStepBuilder creates a new StepBuilder with minimal defaults.
// Default step is a terminal step with success status.
func NewStepBuilder(name string) *StepBuilder {
	return &StepBuilder{
		step: &workflow.Step{
			Name:   name,
			Type:   workflow.StepTypeTerminal,
			Status: workflow.TerminalSuccess,
		},
	}
}

// NewCommandStep creates a new StepBuilder for a command step.
func NewCommandStep(name, command string) *StepBuilder {
	return &StepBuilder{
		step: &workflow.Step{
			Name:    name,
			Type:    workflow.StepTypeCommand,
			Command: command,
		},
	}
}

// NewParallelStep creates a new StepBuilder for a parallel step.
func NewParallelStep(name string, branches ...string) *StepBuilder {
	return &StepBuilder{
		step: &workflow.Step{
			Name:     name,
			Type:     workflow.StepTypeParallel,
			Branches: branches,
			Strategy: "all_succeed",
		},
	}
}

// NewTerminalStep creates a new StepBuilder for a terminal step.
func NewTerminalStep(name string, status workflow.TerminalStatus) *StepBuilder {
	return &StepBuilder{
		step: &workflow.Step{
			Name:   name,
			Type:   workflow.StepTypeTerminal,
			Status: status,
		},
	}
}

// WithType sets the step type.
func (b *StepBuilder) WithType(stepType workflow.StepType) *StepBuilder {
	b.step.Type = stepType
	return b
}

// WithDescription sets the step description.
func (b *StepBuilder) WithDescription(description string) *StepBuilder {
	b.step.Description = description
	return b
}

// WithCommand sets the command for command-type steps.
func (b *StepBuilder) WithCommand(command string) *StepBuilder {
	b.step.Command = command
	return b
}

// WithScriptFile sets the script_file field for the step.
func (b *StepBuilder) WithScriptFile(scriptFile string) *StepBuilder {
	b.step.ScriptFile = scriptFile
	return b
}

// WithDir sets the working directory for command execution.
func (b *StepBuilder) WithDir(dir string) *StepBuilder {
	b.step.Dir = dir
	return b
}

// WithBranches sets the branches for parallel-type steps.
func (b *StepBuilder) WithBranches(branches ...string) *StepBuilder {
	b.step.Branches = branches
	return b
}

// WithStrategy sets the parallel execution strategy.
func (b *StepBuilder) WithStrategy(strategy string) *StepBuilder {
	b.step.Strategy = strategy
	return b
}

// WithMaxConcurrent sets the maximum concurrent branches for parallel steps.
func (b *StepBuilder) WithMaxConcurrent(maxConcurrent int) *StepBuilder {
	b.step.MaxConcurrent = maxConcurrent
	return b
}

// WithTimeout sets the step timeout in seconds.
func (b *StepBuilder) WithTimeout(seconds int) *StepBuilder {
	b.step.Timeout = seconds
	return b
}

// WithOnSuccess sets the next state on success (legacy).
func (b *StepBuilder) WithOnSuccess(nextState string) *StepBuilder {
	b.step.OnSuccess = nextState
	return b
}

// WithOnFailure sets the next state on failure (legacy).
func (b *StepBuilder) WithOnFailure(nextState string) *StepBuilder {
	b.step.OnFailure = nextState
	return b
}

// WithTransitions sets conditional transitions.
func (b *StepBuilder) WithTransitions(transitions workflow.Transitions) *StepBuilder {
	b.step.Transitions = transitions
	return b
}

// WithDependsOn sets step dependencies.
func (b *StepBuilder) WithDependsOn(deps ...string) *StepBuilder {
	b.step.DependsOn = deps
	return b
}

// WithRetry sets retry configuration.
func (b *StepBuilder) WithRetry(retry *workflow.RetryConfig) *StepBuilder {
	b.step.Retry = retry
	return b
}

// WithCapture sets output capture configuration.
func (b *StepBuilder) WithCapture(capture *workflow.CaptureConfig) *StepBuilder {
	b.step.Capture = capture
	return b
}

// WithContinueOnError sets whether to continue on error.
func (b *StepBuilder) WithContinueOnError(continueOnError bool) *StepBuilder {
	b.step.ContinueOnError = continueOnError
	return b
}

// WithStatus sets the terminal status for terminal-type steps.
func (b *StepBuilder) WithStatus(status workflow.TerminalStatus) *StepBuilder {
	b.step.Status = status
	return b
}

// WithLoop sets loop configuration for for_each and while steps.
func (b *StepBuilder) WithLoop(loop *workflow.LoopConfig) *StepBuilder {
	b.step.Loop = loop
	return b
}

// WithMessage sets the message template for terminal-type steps.
// The message is stored as-is and interpolated at runtime.
func (b *StepBuilder) WithMessage(message string) *StepBuilder {
	b.step.Message = message
	return b
}

// WithExitCode sets the process exit code for terminal-type steps (FR-004).
// Defaults to 0; inline error terminals default to 1 at parse time.
func (b *StepBuilder) WithExitCode(code int) *StepBuilder {
	b.step.ExitCode = code
	return b
}

// WithOperation sets operation details for operation-type steps.
func (b *StepBuilder) WithOperation(operation string, inputs map[string]any) *StepBuilder {
	b.step.Operation = operation
	b.step.OperationInputs = inputs
	return b
}

// WithCallWorkflow sets sub-workflow configuration for call_workflow steps.
func (b *StepBuilder) WithCallWorkflow(config *workflow.CallWorkflowConfig) *StepBuilder {
	b.step.CallWorkflow = config
	return b
}

// WithAgent sets AI agent configuration for agent-type steps.
func (b *StepBuilder) WithAgent(agent *workflow.AgentConfig) *StepBuilder {
	b.step.Agent = agent
	return b
}

// Build returns the constructed Step.
func (b *StepBuilder) Build() *workflow.Step {
	return b.step
}
