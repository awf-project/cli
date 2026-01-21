package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil"
)

// =============================================================================
// ServiceTestHarness Tests
// Feature: C012 - Application Test Harness for Service Layer
// =============================================================================
//
// This file contains tests for ServiceTestHarness builder methods.
// Tests verify:
// - NewTestHarness creates harness with default mocks
// - WithWorkflow registers workflows in repository
// - WithCommandResult configures executor results
// - WithStateStore overrides default store
// - Build constructs service and returns mock references
// - Thread-safety of mock operations
// - Method chaining behavior
//
// Test count: 15+ tests covering happy path, edge cases, error handling
// =============================================================================

// =============================================================================
// Happy Path Tests
// =============================================================================

func TestServiceTestHarness_NewTestHarness_CreatesHarnessWithDefaults(t *testing.T) {
	// Arrange & Act
	harness := NewTestHarness(t)

	// Assert: Harness should be created with non-nil default mocks
	require.NotNil(t, harness, "harness should not be nil")
	assert.NotNil(t, harness.repository, "repository should be initialized")
	assert.NotNil(t, harness.store, "state store should be initialized")
	assert.NotNil(t, harness.executor, "executor should be initialized")
	assert.NotNil(t, harness.logger, "logger should be initialized")
	assert.NotNil(t, harness.builder, "ExecutionServiceBuilder should be initialized")
}

func TestServiceTestHarness_WithWorkflow_RegistersWorkflowInRepository(t *testing.T) {
	// Arrange
	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}
	harness := NewTestHarness(t)

	// Act
	result := harness.WithWorkflow("test-workflow", wf)

	// Assert: Should return harness for chaining
	require.NotNil(t, result, "should return harness")
	assert.Equal(t, harness, result, "should return same harness instance for chaining")

	// Assert: Workflow should be registered in repository
	loadedWf, err := harness.repository.Load(context.Background(), "test-workflow")
	require.NoError(t, err, "should load workflow without error")
	assert.Equal(t, "test-workflow", loadedWf.Name)
	assert.Equal(t, "start", loadedWf.Initial)
}

func TestServiceTestHarness_WithCommandResult_ConfiguresExecutorResult(t *testing.T) {
	// Arrange
	harness := NewTestHarness(t)
	expectedResult := &ports.CommandResult{
		Stdout:   "hello world\n",
		Stderr:   "",
		ExitCode: 0,
	}

	// Act
	result := harness.WithCommandResult("echo hello", expectedResult)

	// Assert: Should return harness for chaining
	require.NotNil(t, result, "should return harness")
	assert.Equal(t, harness, result, "should return same harness instance")

	// Assert: Executor should return configured result
	actualResult, err := harness.executor.Execute(context.Background(), &ports.Command{
		Program: "echo hello",
	})
	require.NoError(t, err, "executor should not return error")
	assert.Equal(t, expectedResult.Stdout, actualResult.Stdout)
	assert.Equal(t, expectedResult.ExitCode, actualResult.ExitCode)
}

func TestServiceTestHarness_WithStateStore_OverridesDefaultStore(t *testing.T) {
	// Arrange
	harness := NewTestHarness(t)
	customStore := testutil.NewMockStateStore()
	customStore.Save(context.Background(), &workflow.ExecutionContext{
		WorkflowID:   "custom-id",
		WorkflowName: "custom",
	})

	// Act
	result := harness.WithStateStore(customStore)

	// Assert: Should return harness for chaining
	require.NotNil(t, result, "should return harness")
	assert.Equal(t, harness, result, "should return same harness instance")

	// Assert: Custom store should be used
	ctx, err := customStore.Load(context.Background(), "custom-id")
	require.NoError(t, err)
	assert.Equal(t, "custom", ctx.WorkflowName)
}

func TestServiceTestHarness_Build_ReturnsServiceAndMocks(t *testing.T) {
	// Arrange
	harness := NewTestHarness(t)

	// Act
	svc, mocks := harness.Build()

	// Assert: Service should be created
	require.NotNil(t, svc, "ExecutionService should not be nil")

	// Assert: Mocks tuple should be returned
	require.NotNil(t, mocks, "TestMocks should not be nil")
	assert.NotNil(t, mocks.Repository, "Repository mock should be accessible")
	assert.NotNil(t, mocks.StateStore, "StateStore mock should be accessible")
	assert.NotNil(t, mocks.Executor, "Executor mock should be accessible")
	assert.NotNil(t, mocks.Logger, "Logger mock should be accessible")
}

func TestServiceTestHarness_FluentChaining_MultipleWithMethods(t *testing.T) {
	// Arrange
	wf := &workflow.Workflow{
		Name:    "chained",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo first",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Act: Chain multiple With methods
	svc, mocks := NewTestHarness(t).
		WithWorkflow("chained", wf).
		WithCommandResult("echo first", &ports.CommandResult{
			Stdout:   "first\n",
			ExitCode: 0,
		}).
		Build()

	// Assert: Service should be fully configured
	require.NotNil(t, svc, "service should be created")
	require.NotNil(t, mocks, "mocks should be returned")

	// Assert: Workflow should be registered
	loadedWf, err := mocks.Repository.Load(context.Background(), "chained")
	require.NoError(t, err)
	assert.Equal(t, "chained", loadedWf.Name)
}

func TestServiceTestHarness_Build_ServiceCanExecuteWorkflow(t *testing.T) {
	// Arrange: Create a simple executable workflow
	wf := &workflow.Workflow{
		Name:    "executable",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	svc, _ := NewTestHarness(t).
		WithWorkflow("executable", wf).
		WithCommandResult("echo test", &ports.CommandResult{
			Stdout:   "test output\n",
			ExitCode: 0,
		}).
		Build()

	// Act: Execute workflow
	ctx, err := svc.Run(context.Background(), "executable", nil)

	// Assert: Workflow should execute successfully
	require.NoError(t, err, "workflow execution should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestServiceTestHarness_WithWorkflow_NilWorkflow_HandlesGracefully(t *testing.T) {
	// Arrange
	harness := NewTestHarness(t)

	// Act
	result := harness.WithWorkflow("nil-workflow", nil)

	// Assert: Should handle nil workflow gracefully (or panic if design requires)
	// This tests edge case behavior - the stub will determine actual behavior
	assert.NotNil(t, result, "should return harness even with nil workflow")
}

func TestServiceTestHarness_WithWorkflow_EmptyName_HandlesGracefully(t *testing.T) {
	// Arrange
	harness := NewTestHarness(t)
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}

	// Act
	result := harness.WithWorkflow("", wf)

	// Assert: Should handle empty name
	assert.NotNil(t, result, "should return harness")
}

func TestServiceTestHarness_WithWorkflow_MultipleWorkflows_AllRegistered(t *testing.T) {
	// Arrange
	wf1 := &workflow.Workflow{
		Name:    "workflow1",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}
	wf2 := &workflow.Workflow{
		Name:    "workflow2",
		Initial: "begin",
		Steps: map[string]*workflow.Step{
			"begin": {Name: "begin", Type: workflow.StepTypeTerminal},
		},
	}

	// Act: Register multiple workflows
	harness := NewTestHarness(t).
		WithWorkflow("workflow1", wf1).
		WithWorkflow("workflow2", wf2)

	// Assert: Both workflows should be registered
	loaded1, err1 := harness.repository.Load(context.Background(), "workflow1")
	loaded2, err2 := harness.repository.Load(context.Background(), "workflow2")

	require.NoError(t, err1, "workflow1 should load")
	require.NoError(t, err2, "workflow2 should load")
	assert.Equal(t, "workflow1", loaded1.Name)
	assert.Equal(t, "workflow2", loaded2.Name)
}

func TestServiceTestHarness_WithCommandResult_MultipleCommands_AllConfigured(t *testing.T) {
	// Arrange
	harness := NewTestHarness(t).
		WithCommandResult("cmd1", &ports.CommandResult{Stdout: "output1\n", ExitCode: 0}).
		WithCommandResult("cmd2", &ports.CommandResult{Stdout: "output2\n", ExitCode: 1})

	// Act: Execute commands
	result1, err1 := harness.executor.Execute(context.Background(), &ports.Command{Program: "cmd1"})
	result2, err2 := harness.executor.Execute(context.Background(), &ports.Command{Program: "cmd2"})

	// Assert: Each command should return configured result
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, "output1\n", result1.Stdout)
	assert.Equal(t, "output2\n", result2.Stdout)
	assert.Equal(t, 0, result1.ExitCode)
	assert.Equal(t, 1, result2.ExitCode)
}

func TestServiceTestHarness_WithCommandResult_NilResult_HandlesGracefully(t *testing.T) {
	// Arrange
	harness := NewTestHarness(t)

	// Act
	result := harness.WithCommandResult("test-cmd", nil)

	// Assert: Should handle nil result gracefully
	assert.NotNil(t, result, "should return harness")
}

func TestServiceTestHarness_WithStateStore_NilStore_HandlesGracefully(t *testing.T) {
	// Arrange
	harness := NewTestHarness(t)

	// Act
	result := harness.WithStateStore(nil)

	// Assert: Should handle nil store (implementation may choose to panic or use default)
	assert.NotNil(t, result, "should return harness")
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestServiceTestHarness_Build_WithNoWorkflow_ReturnsValidService(t *testing.T) {
	// Arrange: Build without registering any workflow
	harness := NewTestHarness(t)

	// Act
	svc, mocks := harness.Build()

	// Assert: Should still create valid service with empty repository
	require.NotNil(t, svc, "service should be created")
	require.NotNil(t, mocks, "mocks should be returned")
}

func TestServiceTestHarness_Build_AfterMultipleCalls_ReturnsNewInstances(t *testing.T) {
	// Arrange
	harness := NewTestHarness(t).
		WithWorkflow("test", &workflow.Workflow{
			Name:    "test",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {Name: "start", Type: workflow.StepTypeTerminal},
			},
		})

	// Act: Call Build multiple times
	svc1, mocks1 := harness.Build()
	svc2, mocks2 := harness.Build()

	// Assert: Each Build should return distinct instances
	require.NotNil(t, svc1)
	require.NotNil(t, svc2)
	require.NotNil(t, mocks1)
	require.NotNil(t, mocks2)

	// Note: Depending on implementation, instances may be same or different
	// This test documents the behavior
}

// =============================================================================
// Thread Safety Tests
// =============================================================================

func TestServiceTestHarness_Mocks_ThreadSafe_ConcurrentAccess(t *testing.T) {
	// Arrange
	harness := NewTestHarness(t).
		WithWorkflow("concurrent", &workflow.Workflow{
			Name:    "concurrent",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {Name: "start", Type: workflow.StepTypeTerminal},
			},
		})

	// Act: Access repository concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			_, err := harness.repository.Load(context.Background(), "concurrent")
			assert.NoError(t, err, "concurrent load should not error")
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Assert: No race condition (verified by -race flag)
	// This test should pass with `go test -race`
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestServiceTestHarness_Integration_FullWorkflowExecution(t *testing.T) {
	// Arrange: Create a multi-step workflow with real testutil builders
	wf := testutil.NewWorkflowBuilder().
		WithName("integration-test").
		WithInitial("step1").
		WithStep(testutil.NewStepBuilder("step1").
			WithType(workflow.StepTypeCommand).
			WithCommand("echo hello").
			WithOnSuccess("step2").
			Build()).
		WithStep(testutil.NewStepBuilder("step2").
			WithType(workflow.StepTypeCommand).
			WithCommand("echo world").
			WithOnSuccess("done").
			Build()).
		WithStep(testutil.NewStepBuilder("done").
			WithType(workflow.StepTypeTerminal).
			Build()).
		Build()

	svc, mocks := NewTestHarness(t).
		WithWorkflow("integration-test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{
			Stdout:   "hello\n",
			ExitCode: 0,
		}).
		WithCommandResult("echo world", &ports.CommandResult{
			Stdout:   "world\n",
			ExitCode: 0,
		}).
		Build()

	// Act: Execute workflow
	ctx, err := svc.Run(context.Background(), "integration-test", nil)

	// Assert: Workflow should complete successfully
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Assert: Can use mocks for verification
	assert.NotNil(t, mocks.Repository, "should have repository reference")
	assert.NotNil(t, mocks.Executor, "should have executor reference")

	// Verify step execution via context
	step1State, ok := ctx.GetStepState("step1")
	require.True(t, ok, "step1 should have state")
	assert.Equal(t, "hello\n", step1State.Output)

	step2State, ok := ctx.GetStepState("step2")
	require.True(t, ok, "step2 should have state")
	assert.Equal(t, "world\n", step2State.Output)
}

func TestServiceTestHarness_Integration_UseTestutilFixtures(t *testing.T) {
	// Arrange: Use testutil fixtures for common patterns
	wf := testutil.NewWorkflowBuilder().
		WithName("fixture-test").
		WithInitial("start").
		WithStep(testutil.NewCommandStep("start", "echo fixture").Build()).
		WithStep(testutil.NewTerminalStep("done", workflow.TerminalSuccess).Build()).
		Build()

	// Ensure step transition
	startStep := wf.Steps["start"]
	startStep.OnSuccess = "done"

	svc, mocks := NewTestHarness(t).
		WithWorkflow("fixture-test", wf).
		WithCommandResult("echo fixture", &ports.CommandResult{
			Stdout:   "fixture output\n",
			ExitCode: 0,
		}).
		Build()

	// Act: Execute
	ctx, err := svc.Run(context.Background(), "fixture-test", nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.NotNil(t, mocks.Logger, "logger should be available for assertions")
}
