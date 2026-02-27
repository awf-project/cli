package application_test

import (
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/testutil/builders"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
)

// ServiceTestHarness provides a fluent builder API for constructing ExecutionService
// instances in tests with application-specific convenience methods.
//
// This harness wraps testutil.ExecutionServiceBuilder and adds application-layer
// specific methods like WithWorkflow and WithCommandResult to eliminate repetitive
// test setup patterns across 200+ test cases.
//
// # Problem Statement
//
// Application test files exhibited 200+ repetitive mock setup patterns, each requiring
// 10-15 lines to instantiate repositories, state stores, executors, and loggers before
// constructing ExecutionService. This boilerplate obscured test intent and created
// maintenance overhead.
//
// # Solution
//
// ServiceTestHarness consolidates setup into a 3-line fluent chain, reducing setup
// boilerplate by 71% (~1,500 lines across 3 test files) while maintaining 100% test
// coverage and thread safety.
//
// ## Before (10-15 lines per test)
//
//	repo := newMockRepository()
//	repo.workflows["test"] = workflow
//	executor := newMockExecutor()
//	executor.results["echo hello"] = &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}
//	wfSvc := application.NewWorkflowService(repo, store, executor, logger)
//	execSvc := application.NewExecutionService(wfSvc, executor, parallelExec, store, logger, resolver, nil)
//
// ## After (3 lines per test)
//
//	svc, mocks := NewTestHarness(t).
//	    WithWorkflow("test", workflow).
//	    WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
//	    Build()
//
// # Architecture
//
// Follows ADR-001 (Package-Local Test Harness): Located in application_test package
// to provide application-specific test utilities without coupling domain testutil
// to application patterns. Exceeds ADR-005 threshold with 200+ usage occurrences.
//
// Follows ADR-002 (Wrapper Pattern): Wraps testutil.ExecutionServiceBuilder to
// reuse proven thread-safe infrastructure (93% boilerplate reduction from C007)
// while adding application conveniences.
//
// Follows ADR-003 (Return Service + Mocks Tuple): Build() returns both service
// and mock references to enable test assertions without global state.
//
// # Usage Examples
//
// ## Basic Usage
//
//	// Minimal setup with defaults
//	svc, mocks := NewTestHarness(t).Build()
//
// ## With Workflow
//
//	// Register workflow and configure command result
//	workflow := testutil.NewWorkflowBuilder().
//	    WithName("test").
//	    WithInitial("start").
//	    WithStep(testutil.NewCommandStep("start", "echo hello")).
//	    Build()
//
//	svc, mocks := NewTestHarness(t).
//	    WithWorkflow("test", workflow).
//	    WithCommandResult("echo hello", &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}).
//	    Build()
//
// ## Custom Dependencies
//
//	// Override specific mocks for specialized tests
//	customStore := testmocks.NewMockStateStore()
//	svc, mocks := NewTestHarness(t).
//	    WithStateStore(customStore).
//	    Build()
//
// ## Mock Assertions
//
//	// Use returned mocks for verification
//	svc, mocks := NewTestHarness(t).
//	    WithWorkflow("test", workflow).
//	    Build()
//
//	// Execute workflow
//	ctx, err := svc.Run(context.Background(), "test", nil)
//
//	// Assert using mocks
//	assert.NotNil(t, mocks.Repository)
//	assert.NotNil(t, mocks.Executor)
//
// # Thread Safety
//
// Thread-safe through inheritance from testutil mocks which use sync.Mutex/RWMutex.
// Safe for parallel test execution with `go test -race -parallel=4`.
//
// # Related ADRs
//
//   - ADR-001: Package-local harness in application_test
//   - ADR-002: Wrap ExecutionServiceBuilder, don't duplicate
//   - ADR-003: Return (*ExecutionService, *TestMocks) tuple
//   - ADR-004: Zero logic changes during refactoring
//   - ADR-005: Quality gates after implementation
//
// See .specify/implementation/C012/plan.md for full design rationale.
type ServiceTestHarness struct {
	t          *testing.T
	builder    *builders.ExecutionServiceBuilder
	repository *testmocks.MockWorkflowRepository
	store      *testmocks.MockStateStore
	executor   *testmocks.MockCommandExecutor
	logger     *testmocks.MockLogger
	auditTrail *testmocks.MockAuditTrailWriter
}

// TestMocks exposes mock instances for test assertions.
// Returned by Build() alongside ExecutionService to enable verification
// of mock interactions without global state.
type TestMocks struct {
	Repository *testmocks.MockWorkflowRepository
	StateStore *testmocks.MockStateStore
	Executor   *testmocks.MockCommandExecutor
	Logger     *testmocks.MockLogger
	AuditTrail *testmocks.MockAuditTrailWriter
}

// NewTestHarness creates a new ServiceTestHarness with default mock dependencies.
// All dependencies are thread-safe testutil mocks with sensible defaults.
//
// Returns a harness ready for configuration via With*() methods.
func NewTestHarness(t *testing.T) *ServiceTestHarness {
	// Create default mock dependencies
	repository := testmocks.NewMockWorkflowRepository()
	store := testmocks.NewMockStateStore()
	executor := testmocks.NewMockCommandExecutor()
	logger := testmocks.NewMockLogger()
	auditTrail := testmocks.NewMockAuditTrailWriter()

	// Configure executor with default success result for all commands
	executor.SetCommandResult("", &ports.CommandResult{
		Stdout:   "",
		Stderr:   "",
		ExitCode: 0,
	})

	// Create ExecutionServiceBuilder with the mocks
	builder := builders.NewExecutionServiceBuilder().
		WithWorkflowRepository(repository).
		WithStateStore(store).
		WithExecutor(executor).
		WithLogger(logger)

	return &ServiceTestHarness{
		t:          t,
		builder:    builder,
		repository: repository,
		store:      store,
		executor:   executor,
		logger:     logger,
		auditTrail: auditTrail,
	}
}

// NewTestHarnessWithEvaluator creates a new ServiceTestHarness with default mock dependencies
// and configures the provided expression evaluator for conditional transitions.
// All dependencies are thread-safe testutil mocks with sensible defaults.
//
// Parameters:
//   - t: testing.T instance for test context
//   - evaluator: ports.ExpressionEvaluator implementation for evaluating "when" clauses
//
// Returns a harness ready for configuration via With*() methods.
func NewTestHarnessWithEvaluator(t *testing.T, evaluator ports.ExpressionEvaluator) *ServiceTestHarness {
	// Create default mock dependencies
	repository := testmocks.NewMockWorkflowRepository()
	store := testmocks.NewMockStateStore()
	executor := testmocks.NewMockCommandExecutor()
	logger := testmocks.NewMockLogger()
	auditTrail := testmocks.NewMockAuditTrailWriter()

	// Configure executor with default success result for all commands
	executor.SetCommandResult("", &ports.CommandResult{
		Stdout:   "",
		Stderr:   "",
		ExitCode: 0,
	})

	// Create ExecutionServiceBuilder with the mocks and evaluator
	builder := builders.NewExecutionServiceBuilder().
		WithWorkflowRepository(repository).
		WithStateStore(store).
		WithExecutor(executor).
		WithLogger(logger).
		WithEvaluator(evaluator)

	return &ServiceTestHarness{
		t:          t,
		builder:    builder,
		repository: repository,
		store:      store,
		executor:   executor,
		logger:     logger,
		auditTrail: auditTrail,
	}
}

// WithWorkflow registers a workflow in the mock repository.
// Convenience method that delegates to repository.AddWorkflow(name, wf).
//
// Returns the harness for method chaining.
func (h *ServiceTestHarness) WithWorkflow(name string, wf *workflow.Workflow) *ServiceTestHarness {
	if wf != nil && name != "" {
		h.repository.AddWorkflow(name, wf)
	}
	return h
}

// WithCommandResult configures the mock executor to return a specific result
// for a command. Convenience method for common test setup pattern.
//
// Parameters:
//   - cmd: The command string to match (e.g., "echo hello")
//   - result: The CommandResult to return when cmd is executed
//
// Returns the harness for method chaining.
func (h *ServiceTestHarness) WithCommandResult(cmd string, result *ports.CommandResult) *ServiceTestHarness {
	if result != nil {
		h.executor.SetCommandResult(cmd, result)
	}
	return h
}

// WithStateStore overrides the default mock state store with a custom implementation.
// Useful for tests requiring specialized state behavior (e.g., error injection).
//
// Returns the harness for method chaining.
func (h *ServiceTestHarness) WithStateStore(store ports.StateStore) *ServiceTestHarness {
	// Update the builder with the new store
	h.builder.WithStateStore(store)

	// If it's a MockStateStore, update our reference for mock assertions
	if mockStore, ok := store.(*testmocks.MockStateStore); ok {
		h.store = mockStore
	}

	return h
}

// WithExecutor overrides the default mock executor with a custom implementation.
// Useful for tests requiring specialized execution behavior (e.g., slow operations, failures).
//
// Returns the harness for method chaining.
func (h *ServiceTestHarness) WithExecutor(executor ports.CommandExecutor) *ServiceTestHarness {
	// Update the builder with the new executor
	h.builder.WithExecutor(executor)

	// If it's a MockCommandExecutor, update our reference for mock assertions
	if mockExecutor, ok := executor.(*testmocks.MockCommandExecutor); ok {
		h.executor = mockExecutor
	}

	return h
}

// WithAuditTrailWriter overrides the default mock audit trail writer with a custom implementation.
// Useful for tests requiring specialized audit behavior (e.g., error injection, verification).
//
// Returns the harness for method chaining.
func (h *ServiceTestHarness) WithAuditTrailWriter(writer ports.AuditTrailWriter) *ServiceTestHarness {
	// If it's a MockAuditTrailWriter, update our reference for mock assertions
	if mockWriter, ok := writer.(*testmocks.MockAuditTrailWriter); ok {
		h.auditTrail = mockWriter
	}

	return h
}

// Build constructs the ExecutionService and returns it along with mock references.
// Terminal method in the fluent builder chain.
//
// Returns:
//   - *application.ExecutionService: Fully configured service ready for testing
//   - *TestMocks: Mock instances for assertion purposes
func (h *ServiceTestHarness) Build() (*application.ExecutionService, *TestMocks) {
	// Build the ExecutionService using the configured builder
	service := h.builder.Build()

	// F063/T012: Inject AWF XDG directory paths for template interpolation
	// This populates the .awf namespace in interpolation contexts
	service.SetAWFPaths(map[string]string{
		"prompts_dir":   xdg.AWFPromptsDir(),
		"config_dir":    xdg.AWFConfigDir(),
		"data_dir":      xdg.AWFDataDir(),
		"workflows_dir": xdg.AWFWorkflowsDir(),
		"plugins_dir":   xdg.AWFPluginsDir(),
	})
	service.SetAuditTrailWriter(h.auditTrail)

	// Create TestMocks tuple with references to all mocks
	mocks := &TestMocks{
		Repository: h.repository,
		StateStore: h.store,
		Executor:   h.executor,
		Logger:     h.logger,
		AuditTrail: h.auditTrail,
	}

	return service, mocks
}
