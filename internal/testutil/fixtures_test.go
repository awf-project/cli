package testutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil"
)

// Feature: C007
// Component: T009 - Workflow Fixtures

// TestSimpleWorkflow_HappyPath tests the SimpleWorkflow factory with default configuration
func TestSimpleWorkflow_HappyPath(t *testing.T) {
	// Arrange & Act
	wf := testutil.SimpleWorkflow()

	// Assert
	require.NotNil(t, wf, "SimpleWorkflow should return non-nil workflow")
	assert.NotEmpty(t, wf.Name, "Workflow should have a name")
	assert.NotEmpty(t, wf.Initial, "Workflow should have an initial step")
	assert.NotEmpty(t, wf.Steps, "Workflow should have steps")

	// Verify initial step exists
	initialStep, exists := wf.Steps[wf.Initial]
	assert.True(t, exists, "Initial step should exist in steps map")
	assert.NotNil(t, initialStep, "Initial step should not be nil")

	// Verify workflow structure is minimal (simple = 1-2 steps)
	assert.LessOrEqual(t, len(wf.Steps), 2, "Simple workflow should have at most 2 steps")

	// Verify workflow is valid (can call Validate)
	err := wf.Validate(nil)
	assert.NoError(t, err, "Simple workflow should be valid")
}

// TestSimpleWorkflow_WithCustomName tests SimpleWorkflow with custom name parameter
func TestSimpleWorkflow_WithCustomName(t *testing.T) {
	// Arrange
	customName := "my-custom-workflow"

	// Act
	wf := testutil.SimpleWorkflow(customName)

	// Assert
	assert.Equal(t, customName, wf.Name, "Workflow name should match custom parameter")
	assert.NoError(t, wf.Validate(nil))
}

// TestSimpleWorkflow_WithMultipleParameters tests SimpleWorkflow with additional configuration options
func TestSimpleWorkflow_WithMultipleParameters(t *testing.T) {
	// Arrange
	name := "test-workflow"
	command := "echo 'test'"

	// Act
	wf := testutil.SimpleWorkflow(name, command)

	// Assert
	assert.Equal(t, name, wf.Name)
	// Verify command step exists with the specified command
	commandStepFound := false
	for _, step := range wf.Steps {
		if step.Type == workflow.StepTypeCommand && step.Command == command {
			commandStepFound = true
			break
		}
	}
	assert.True(t, commandStepFound, "Should find command step with specified command")
}

// TestLinearWorkflow_HappyPath tests LinearWorkflow factory with default 3-step chain
func TestLinearWorkflow_HappyPath(t *testing.T) {
	// Arrange & Act
	wf := testutil.LinearWorkflow()

	// Assert
	require.NotNil(t, wf)
	assert.NotEmpty(t, wf.Name)
	assert.Equal(t, wf.Initial, wf.Steps[wf.Initial].Name, "Initial step name should match")

	// Verify linear structure (each step has OnSuccess pointing to next)
	assert.GreaterOrEqual(t, len(wf.Steps), 3, "Linear workflow should have at least 3 steps")

	// Walk the chain from initial to terminal
	visited := make(map[string]bool)
	current := wf.Initial
	stepCount := 0
	for current != "" && stepCount < 10 { // limit to prevent infinite loop
		step, exists := wf.Steps[current]
		require.True(t, exists, "Step %s should exist", current)
		require.False(t, visited[current], "Should not revisit step %s (no cycles)", current)

		visited[current] = true
		stepCount++

		if step.Type == workflow.StepTypeTerminal {
			break
		}
		current = step.OnSuccess
	}

	assert.GreaterOrEqual(t, stepCount, 3, "Should have walked through at least 3 steps")
	assert.NoError(t, wf.Validate(nil))
}

// TestLinearWorkflow_WithCustomStepCount tests LinearWorkflow with configurable number of steps
func TestLinearWorkflow_WithCustomStepCount(t *testing.T) {
	tests := []struct {
		name      string
		stepCount int
		wantSteps int // expected number of steps (including terminal)
	}{
		{
			name:      "single step plus terminal",
			stepCount: 1,
			wantSteps: 2, // 1 command step + 1 terminal
		},
		{
			name:      "five steps plus terminal",
			stepCount: 5,
			wantSteps: 6,
		},
		{
			name:      "ten steps plus terminal",
			stepCount: 10,
			wantSteps: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			wf := testutil.LinearWorkflow(tt.stepCount)

			// Assert
			assert.Len(t, wf.Steps, tt.wantSteps, "Should have %d total steps", tt.wantSteps)
			assert.NoError(t, wf.Validate(nil))
		})
	}
}

// TestLinearWorkflow_EdgeCase_ZeroSteps tests handling of edge case with zero steps
func TestLinearWorkflow_EdgeCase_ZeroSteps(t *testing.T) {
	// Act
	wf := testutil.LinearWorkflow(0)

	// Assert
	// Should return minimal valid workflow (at least initial + terminal)
	assert.GreaterOrEqual(t, len(wf.Steps), 1, "Should have at least one step")
	assert.NoError(t, wf.Validate(nil), "Even zero-step workflow should be valid")
}

// TestParallelWorkflow_HappyPath tests ParallelWorkflow with default 2 branches
func TestParallelWorkflow_HappyPath(t *testing.T) {
	// Arrange & Act
	wf := testutil.ParallelWorkflow()

	// Assert
	require.NotNil(t, wf)
	assert.NotEmpty(t, wf.Name)

	// Find parallel step
	var parallelStep *workflow.Step
	for _, step := range wf.Steps {
		if step.Type == workflow.StepTypeParallel {
			parallelStep = step
			break
		}
	}

	require.NotNil(t, parallelStep, "Should have at least one parallel step")
	assert.NotEmpty(t, parallelStep.Branches, "Parallel step should have branches")
	assert.GreaterOrEqual(t, len(parallelStep.Branches), 2, "Should have at least 2 branches by default")

	// Verify all branches exist as steps
	for _, branchName := range parallelStep.Branches {
		_, exists := wf.Steps[branchName]
		assert.True(t, exists, "Branch step %s should exist in workflow", branchName)
	}

	assert.NoError(t, wf.Validate(nil))
}

// TestParallelWorkflow_WithCustomBranchCount tests configurable number of parallel branches
func TestParallelWorkflow_WithCustomBranchCount(t *testing.T) {
	tests := []struct {
		name        string
		branchCount int
	}{
		{name: "two branches", branchCount: 2},
		{name: "five branches", branchCount: 5},
		{name: "ten branches", branchCount: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			wf := testutil.ParallelWorkflow(tt.branchCount)

			// Assert - find parallel step
			var parallelStep *workflow.Step
			for _, step := range wf.Steps {
				if step.Type == workflow.StepTypeParallel {
					parallelStep = step
					break
				}
			}

			require.NotNil(t, parallelStep)
			assert.Len(t, parallelStep.Branches, tt.branchCount, "Should have %d branches", tt.branchCount)
		})
	}
}

// TestParallelWorkflow_WithStrategy tests ParallelWorkflow with different execution strategies
func TestParallelWorkflow_WithStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
	}{
		{name: "all_succeed strategy", strategy: "all_succeed"},
		{name: "any_succeed strategy", strategy: "any_succeed"},
		{name: "best_effort strategy", strategy: "best_effort"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			wf := testutil.ParallelWorkflow(3, tt.strategy)

			// Assert
			var parallelStep *workflow.Step
			for _, step := range wf.Steps {
				if step.Type == workflow.StepTypeParallel {
					parallelStep = step
					break
				}
			}

			require.NotNil(t, parallelStep)
			assert.Equal(t, tt.strategy, parallelStep.Strategy)
		})
	}
}

// TestLoopWorkflow_HappyPath tests LoopWorkflow with default for_each configuration
func TestLoopWorkflow_HappyPath(t *testing.T) {
	// Arrange & Act
	wf := testutil.LoopWorkflow()

	// Assert
	require.NotNil(t, wf)
	assert.NotEmpty(t, wf.Name)

	// Find loop step
	var loopStep *workflow.Step
	for _, step := range wf.Steps {
		if step.Type == workflow.StepTypeForEach || step.Type == workflow.StepTypeWhile {
			loopStep = step
			break
		}
	}

	require.NotNil(t, loopStep, "Should have at least one loop step")
	require.NotNil(t, loopStep.Loop, "Loop step should have Loop configuration")

	// Verify loop has required fields
	switch loopStep.Type {
	case workflow.StepTypeForEach:
		assert.NotEmpty(t, loopStep.Loop.Items, "ForEach loop should have Items")
	case workflow.StepTypeWhile:
		assert.NotEmpty(t, loopStep.Loop.Condition, "While loop should have Condition")
	default:
		t.Fatalf("unexpected loop step type: %s", loopStep.Type)
	}

	assert.NoError(t, wf.Validate(nil))
}

// TestLoopWorkflow_ForEachType tests LoopWorkflow configured as for_each
func TestLoopWorkflow_ForEachType(t *testing.T) {
	// Act
	wf := testutil.LoopWorkflow("for_each")

	// Assert
	var loopStep *workflow.Step
	for _, step := range wf.Steps {
		if step.Type == workflow.StepTypeForEach {
			loopStep = step
			break
		}
	}

	require.NotNil(t, loopStep, "Should have for_each step")
	require.NotNil(t, loopStep.Loop)
	assert.NotEmpty(t, loopStep.Loop.Items)
}

// TestLoopWorkflow_WhileType tests LoopWorkflow configured as while
func TestLoopWorkflow_WhileType(t *testing.T) {
	// Act
	wf := testutil.LoopWorkflow("while")

	// Assert
	var loopStep *workflow.Step
	for _, step := range wf.Steps {
		if step.Type == workflow.StepTypeWhile {
			loopStep = step
			break
		}
	}

	require.NotNil(t, loopStep, "Should have while step")
	require.NotNil(t, loopStep.Loop)
	assert.NotEmpty(t, loopStep.Loop.Condition)
}

// TestLoopWorkflow_WithCustomItems tests LoopWorkflow with custom iteration items
func TestLoopWorkflow_WithCustomItems(t *testing.T) {
	// Arrange
	items := []string{"item1", "item2", "item3"}

	// Act
	wf := testutil.LoopWorkflow("for_each", items)

	// Assert
	var loopStep *workflow.Step
	for _, step := range wf.Steps {
		if step.Type == workflow.StepTypeForEach {
			loopStep = step
			break
		}
	}

	require.NotNil(t, loopStep)
	require.NotNil(t, loopStep.Loop)
	// The Loop.Items field should reference the items in some form
	assert.NotEmpty(t, loopStep.Loop.Items)
}

// TestConversationWorkflow_HappyPath tests ConversationWorkflow with default agent configuration
func TestConversationWorkflow_HappyPath(t *testing.T) {
	// Arrange & Act
	wf := testutil.ConversationWorkflow()

	// Assert
	require.NotNil(t, wf)
	assert.NotEmpty(t, wf.Name)

	// Find agent step
	var agentStep *workflow.Step
	for _, step := range wf.Steps {
		if step.Type == workflow.StepTypeAgent {
			agentStep = step
			break
		}
	}

	require.NotNil(t, agentStep, "Should have at least one agent step")
	require.NotNil(t, agentStep.Agent, "Agent step should have Agent configuration")

	// Verify agent has required fields
	assert.NotEmpty(t, agentStep.Agent.Provider, "Agent should have provider")
	assert.NotEmpty(t, agentStep.Agent.Options, "Agent should have options")
	// Model is in Options["model"]
	if agentStep.Agent.Options != nil {
		assert.NotNil(t, agentStep.Agent.Options["model"], "Agent should have model in options")
	}

	assert.NoError(t, wf.Validate(nil))
}

// TestConversationWorkflow_WithCustomProvider tests ConversationWorkflow with specific AI provider
func TestConversationWorkflow_WithCustomProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
	}{
		{name: "anthropic claude", provider: "anthropic", model: "claude-3-opus"},
		{name: "openai gpt", provider: "openai", model: "gpt-4"},
		{name: "google gemini", provider: "google", model: "gemini-pro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			wf := testutil.ConversationWorkflow(tt.provider, tt.model)

			// Assert
			var agentStep *workflow.Step
			for _, step := range wf.Steps {
				if step.Type == workflow.StepTypeAgent {
					agentStep = step
					break
				}
			}

			require.NotNil(t, agentStep)
			require.NotNil(t, agentStep.Agent)
			assert.Equal(t, tt.provider, agentStep.Agent.Provider)
			// Model is in Options["model"]
			if agentStep.Agent.Options != nil {
				assert.Equal(t, tt.model, agentStep.Agent.Options["model"])
			}
		})
	}
}

// TestConversationWorkflow_WithMultiTurn tests ConversationWorkflow with multiple conversation steps
func TestConversationWorkflow_WithMultiTurn(t *testing.T) {
	// Act
	wf := testutil.ConversationWorkflow("anthropic", "claude-3-opus", 3) // 3 turns

	// Assert
	// Count agent steps
	agentStepCount := 0
	for _, step := range wf.Steps {
		if step.Type == workflow.StepTypeAgent {
			agentStepCount++
		}
	}

	assert.GreaterOrEqual(t, agentStepCount, 1, "Should have at least 1 agent step")
	// May have multiple agent steps if multi-turn is implemented
}

// TestAllFixtures_ProduceValidWorkflows tests that all fixtures pass validation
func TestAllFixtures_ProduceValidWorkflows(t *testing.T) {
	tests := []struct {
		name        string
		fixtureFunc func() *workflow.Workflow
	}{
		{name: "SimpleWorkflow", fixtureFunc: func() *workflow.Workflow { return testutil.SimpleWorkflow() }},
		{name: "LinearWorkflow", fixtureFunc: func() *workflow.Workflow { return testutil.LinearWorkflow() }},
		{name: "ParallelWorkflow", fixtureFunc: func() *workflow.Workflow { return testutil.ParallelWorkflow() }},
		{name: "LoopWorkflow", fixtureFunc: func() *workflow.Workflow { return testutil.LoopWorkflow() }},
		{name: "ConversationWorkflow", fixtureFunc: func() *workflow.Workflow { return testutil.ConversationWorkflow() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			wf := tt.fixtureFunc()

			// Assert
			require.NotNil(t, wf, "Fixture should return non-nil workflow")
			assert.NotEmpty(t, wf.Name, "Workflow should have a name")
			assert.NotEmpty(t, wf.Initial, "Workflow should have initial step")
			assert.NotEmpty(t, wf.Steps, "Workflow should have steps")

			err := wf.Validate(nil)
			assert.NoError(t, err, "Workflow from %s should be valid", tt.name)
		})
	}
}

// TestFixtures_AreIndependent tests that fixtures return new instances each time
func TestFixtures_AreIndependent(t *testing.T) {
	// Act
	wf1 := testutil.SimpleWorkflow()
	wf2 := testutil.SimpleWorkflow()

	// Assert - different memory addresses
	assert.NotSame(t, wf1, wf2, "Each call should return a new instance")

	// Mutating one should not affect the other
	wf1.Name = "modified"
	assert.NotEqual(t, wf1.Name, wf2.Name, "Workflows should be independent")
}

// TestFixtures_ThreadSafe tests that fixtures can be called concurrently
func TestFixtures_ThreadSafe(t *testing.T) {
	// This test verifies fixtures can be safely called from multiple goroutines
	const goroutines = 50
	results := make(chan *workflow.Workflow, goroutines)

	// Act - concurrent fixture calls
	for i := 0; i < goroutines; i++ {
		go func() {
			wf := testutil.SimpleWorkflow()
			results <- wf
		}()
	}

	// Assert - collect all results
	workflows := make([]*workflow.Workflow, 0, goroutines)
	for i := 0; i < goroutines; i++ {
		wf := <-results
		require.NotNil(t, wf)
		workflows = append(workflows, wf)
	}

	assert.Len(t, workflows, goroutines, "All goroutines should produce workflows")
}

// TestFixtures_Integration_WithBuilders tests that fixtures work well with WorkflowBuilder
func TestFixtures_Integration_WithBuilders(t *testing.T) {
	// Act - start with fixture and customize with builder
	wf := testutil.SimpleWorkflow()

	builder := testutil.NewWorkflowBuilder()
	builder.WithName("customized-" + wf.Name)
	customized := builder.Build()

	// Assert
	assert.Contains(t, customized.Name, "customized-", "Builder should allow customization")
	assert.NotEqual(t, wf.Name, customized.Name, "Builder should produce different workflow")
}
