// Package testutil provides shared test infrastructure for AWF CLI tests.
//
// This package centralizes common mocks, builders, assertions, and fixtures
// to reduce test boilerplate and ensure consistent test patterns across the codebase.
// The testutil package is the result of C007 Test Infrastructure Modernization,
// which consolidated 23 duplicate mock types and reduced test setup from 30+ lines to 2-3 lines.
//
// # Architecture
//
// The testutil package follows hexagonal architecture principles:
//   - Mocks implement domain port interfaces (WorkflowRepository, StateStore, CommandExecutor, Logger)
//   - All mocks are thread-safe with sync.Mutex protection for parallel test execution
//   - Builders use fluent API pattern with sensible defaults and With*() chaining methods
//   - Assertion helpers provide domain-specific validation with clear error messages
//   - Workflow fixtures generate valid test workflows for common patterns
//
// # Thread Safety
//
// All mocks in this package are thread-safe and can be used in parallel tests.
// State-collecting mocks use sync.Mutex or sync.RWMutex for concurrent access protection.
// This enables running tests with `go test -race -parallel=4` without data races.
//
// # Components
//
// ## Mocks (mocks.go)
//
// Thread-safe implementations of domain port interfaces:
//   - MockWorkflowRepository: In-memory workflow storage with configurable errors
//   - MockStateStore: In-memory state persistence with configurable errors
//   - MockCommandExecutor: Command execution simulator with call recording
//   - MockLogger: Log message capture with level filtering
//   - MockHistoryStore: Execution history tracking for testing
//
// ## Builders (builders.go)
//
// Fluent builders for constructing test objects:
//   - ExecutionServiceBuilder: Build ExecutionService with sensible defaults
//   - WorkflowBuilder: Construct valid workflows with progressive configuration
//   - StepBuilder: Create workflow steps of any type (command, parallel, terminal)
//
// ## Fixtures (fixtures.go)
//
// Workflow factory functions for common test patterns:
//   - SimpleWorkflow: Single-step workflow for basic testing
//   - LinearWorkflow: Sequential multi-step workflow with transitions
//   - ParallelWorkflow: Concurrent execution with configurable strategy
//   - LoopWorkflow: Iterative workflows with for_each or while loops
//   - ConversationWorkflow: Agent-based workflows with conversation patterns
//
// # Usage Examples
//
// ## Basic Mock Usage
//
// Create thread-safe mocks for testing:
//
//	repo := testutil.NewMockWorkflowRepository()
//	repo.AddWorkflow("test", workflow)
//
//	store := testutil.NewMockStateStore()
//	executor := testutil.NewMockCommandExecutor()
//	executor.SetResult("success", nil)
//
//	logger := testutil.NewMockLogger()
//
// ## Fluent Builder Pattern
//
// Build ExecutionService with minimal code (reduces 30+ lines to 2-3 lines):
//
//	// Basic service with defaults
//	svc := testutil.NewExecutionServiceBuilder().Build(t)
//
//	// Customized service with specific mocks
//	svc := testutil.NewExecutionServiceBuilder().
//	    WithLogger(logger).
//	    WithExecutor(executor).
//	    WithStateStore(store).
//	    Build(t)
//
//	// Service with workflow repository
//	repo := testutil.NewMockWorkflowRepository()
//	repo.AddWorkflow("test", workflow)
//	svc := testutil.NewExecutionServiceBuilder().
//	    WithWorkflowRepository(repo).
//	    Build(t)
//
// ## Workflow Construction
//
// Build workflows progressively with fluent API:
//
//	// Simple workflow
//	wf := testutil.NewWorkflowBuilder().
//	    WithName("test-workflow").
//	    WithInitial("start").
//	    WithStep(testutil.NewCommandStep("start", "echo hello")).
//	    Build()
//
//	// Linear workflow with transitions
//	wf := testutil.NewWorkflowBuilder().
//	    WithName("linear").
//	    WithInitial("step1").
//	    WithStep(testutil.NewCommandStep("step1", "echo one").
//	        WithOnSuccess("step2")).
//	    WithStep(testutil.NewCommandStep("step2", "echo two").
//	        WithOnSuccess("end")).
//	    WithStep(testutil.NewTerminalStep("end")).
//	    Build()
//
//	// Parallel workflow with strategy
//	wf := testutil.NewWorkflowBuilder().
//	    WithName("parallel").
//	    WithInitial("parallel").
//	    WithStep(testutil.NewParallelStep("parallel", []string{"branch1", "branch2"}).
//	        WithStrategy("all_succeed").
//	        WithOnSuccess("end")).
//	    WithStep(testutil.NewCommandStep("branch1", "echo one")).
//	    WithStep(testutil.NewCommandStep("branch2", "echo two")).
//	    WithStep(testutil.NewTerminalStep("end")).
//	    Build()
//
// ## Workflow Fixtures
//
// Use pre-built fixtures for common patterns:
//
//	// Simple single-step workflow
//	wf := testutil.SimpleWorkflow("test-name")
//
//	// Linear 3-step workflow
//	wf := testutil.LinearWorkflow("linear-name", 3)
//
//	// Parallel workflow with 2 branches and all_succeed strategy
//	wf := testutil.ParallelWorkflow("parallel-name", 2, "all_succeed")
//
//	// For-each loop over items
//	wf := testutil.LoopWorkflow("loop-name", "for_each", []string{"a", "b", "c"})
//
//	// While loop with condition
//	wf := testutil.LoopWorkflow("while-name", "while", "{{states.check.output}} == 'continue'")
//
//	// Conversation workflow with agent
//	wf := testutil.ConversationWorkflow("chat-name", "claude-sonnet", "openai")
//
// ## Complete Test Example
//
// Typical test function using testutil (reduces LOC by 15%+):
//
//	func TestWorkflowExecution(t *testing.T) {
//	    // Arrange: Build service with test fixtures
//	    executor := testutil.NewMockCommandExecutor()
//	    executor.SetResult("success", nil)
//
//	    workflow := testutil.SimpleWorkflow("test")
//
//	    svc := testutil.NewExecutionServiceBuilder().
//	        WithExecutor(executor).
//	        Build(t)
//
//	    // Act: Execute workflow
//	    ctx, err := svc.Execute(context.Background(), workflow, nil)
//
//	    // Assert: Verify results
//	    require.NoError(t, err)
//	    assert.Equal(t, "end", ctx.CurrentState)
//	    assert.Equal(t, "success", ctx.States["start"].Output)
//	}
//
// # Migration Guide
//
// ## Replacing os.Setenv with t.Setenv
//
// Before (thread-unsafe):
//
//	os.Setenv("AWF_WORKFLOWS_PATH", "/tmp/workflows")
//	defer os.Unsetenv("AWF_WORKFLOWS_PATH")
//
// After (thread-safe):
//
//	t.Setenv("AWF_WORKFLOWS_PATH", "/tmp/workflows")
//
// The t.Setenv method automatically restores the original value after the test,
// preventing test pollution and enabling safe parallel execution.
//
// ## Replacing Manual Mocks
//
// Before (duplicated mock):
//
//	type mockExecutor struct {
//	    result string
//	    err    error
//	}
//
//	func (m *mockExecutor) Execute(ctx context.Context, cmd ports.Command) (*operation.Result, error) {
//	    if m.err != nil {
//	        return nil, m.err
//	    }
//	    return &operation.Result{Output: m.result}, nil
//	}
//
//	executor := &mockExecutor{result: "success"}
//
// After (testutil mock):
//
//	executor := testutil.NewMockCommandExecutor()
//	executor.SetResult("success", nil)
//
// ## Replacing Manual Service Setup
//
// Before (30+ lines):
//
//	logger := &mockLogger{messages: make([]string, 0)}
//	executor := &mockExecutor{result: "success"}
//	store := &mockStateStore{states: make(map[string]*workflow.ExecutionState)}
//	repo := &mockRepository{workflows: make(map[string]*workflow.Workflow)}
//	repo.workflows["test"] = &workflow.Workflow{
//	    Name:    "test",
//	    Initial: "start",
//	    Steps: map[string]*workflow.Step{
//	        "start": {Name: "start", Type: "terminal"},
//	    },
//	}
//
//	svc := application.NewExecutionService(logger, executor, store, repo)
//
// After (2-3 lines):
//
//	executor := testutil.NewMockCommandExecutor()
//	executor.SetResult("success", nil)
//	svc := testutil.NewExecutionServiceBuilder().
//	    WithExecutor(executor).
//	    Build(t)
//
// # Design Principles
//
// ## ADR-005: Specialized Mock Locality
//
// Keep specialized mocks colocated with tests unless reused 3+ times.
// Only general-purpose mocks belong in testutil.
//
// Examples of specialized mocks that stay in test files:
//   - retryCountingExecutor: Tracks retry attempts for specific retry logic tests
//   - timeoutExecutor: Simulates command timeouts for timeout handling tests
//   - errorMockExecutor: Returns specific error types for error handling tests
//
// When to move a mock to testutil:
//   - Used in 3+ test files across different packages
//   - Implements a standard port interface with general-purpose behavior
//   - Provides thread-safe state collection needed by multiple tests
//
// ## Sensible Defaults
//
// All builders provide valid defaults for fields:
//   - ExecutionServiceBuilder: Creates fully functional service with mock dependencies
//   - WorkflowBuilder: Generates valid workflows with required fields populated
//   - StepBuilder: Produces steps that pass validation
//
// Progressive configuration via With*() methods allows customization only where needed.
//
// ## Thread Safety First
//
// All mocks use sync.Mutex or sync.RWMutex from day one:
//   - MockWorkflowRepository: sync.RWMutex for concurrent read/write operations
//   - MockStateStore: sync.RWMutex for state access patterns
//   - MockCommandExecutor: sync.Mutex for call recording
//   - MockLogger: sync.Mutex for message capture
//
// Validated with `go test -race ./...` to ensure data race freedom.
//
// # Performance Considerations
//
// Builders add minimal allocation overhead:
//   - Acceptable for test code (not production)
//   - Reduces overall test complexity despite slight memory cost
//   - Benchmark critical paths if performance becomes concern
//
// Mock call recording uses slices with mutex protection:
//   - Efficient for typical test workloads (< 1000 calls)
//   - Call Clear() between test cases to reset state
//
// # Related Documentation
//
// See also:
//   - .specify/implementation/C007/spec-content.md: Full specification
//   - .specify/implementation/C007/plan.md: Implementation plan and ADRs
//   - .specify/implementation/C007/tasks.md: Task breakdown and checklist
package testutil
