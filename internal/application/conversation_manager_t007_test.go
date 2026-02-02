package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// Component T007 Tests - Multi-Turn Prompt Resolution
// =============================================================================
//
// Component T007: Add unit tests for multi-turn prompt resolution
// - Verify prompt template interpolation works correctly across turns
// - Test dynamic variables in prompts resolve properly with changing context
// - Ensure InitialPrompt vs Prompt distinction is maintained
//
// This test suite verifies that:
// 1. Prompt templates with {{variables}} are resolved correctly in each turn
// 2. Dynamic context variables (states, inputs) update between turns
// 3. InitialPrompt is used for turn 1, Prompt is used for subsequent turns
// 4. Resolution errors are handled gracefully
// =============================================================================

// TestConversationManager_T007_PromptResolution_HappyPath verifies basic
// prompt resolution across multiple turns with template variables.
func TestConversationManager_T007_PromptResolution_HappyPath(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false} // Never stop
	tokenizer := &mockTokenizer{count: 10}

	var receivedPrompts []string
	resolvedValues := []string{"Hello World", "Continue Task", "Finish Task"}

	resolver := &mockResolverT007{
		resolveFunc: func(template string, ctx *interpolation.Context) (string, error) {
			idx := len(receivedPrompts)
			if idx < len(resolvedValues) {
				return resolvedValues[idx], nil
			}
			return template, nil
		},
	}

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output: "Response 1",
			},
			{
				State: &workflow.ConversationState{
					TotalTurns:  2,
					TotalTokens: 40,
				},
				Output: "Response 2",
			},
			{
				State: &workflow.ConversationState{
					TotalTurns:  3,
					TotalTokens: 60,
				},
				Output: "Response 3",
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "template_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			Prompt:        "{{inputs.action}} Task",
			InitialPrompt: "{{inputs.greeting}} World",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 3,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.Inputs = map[string]any{
		"greeting": "Hello",
		"action":   "Continue",
	}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		ctx := interpolation.NewContext()
		ctx.Inputs = ec.Inputs
		return ctx
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	requireNoResolutionError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.State.TotalTurns)

	// Verify prompts were resolved correctly
	require.Len(t, receivedPrompts, 3)
	assertPromptResolved(t, "Hello World", receivedPrompts[0], "turn 1 should use resolved InitialPrompt")
	assertPromptResolved(t, "Continue Task", receivedPrompts[1], "turn 2 should use resolved Prompt")
	assertPromptResolved(t, "Finish Task", receivedPrompts[2], "turn 3 should use resolved Prompt")
}

// TestConversationManager_T007_PromptResolution_WithDynamicInputs verifies
// that prompts with {{inputs.var}} resolve correctly when inputs change
// between turns.
func TestConversationManager_T007_PromptResolution_WithDynamicInputs(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	tokenizer := &mockTokenizer{count: 10}

	var receivedPrompts []string
	turnCount := 0

	resolver := &mockResolverT007{
		resolveFunc: func(template string, ctx *interpolation.Context) (string, error) {
			if ctx.Inputs == nil {
				return template, nil
			}
			if topic, ok := ctx.Inputs["topic"].(string); ok {
				return fmt.Sprintf("Discuss %s", topic), nil
			}
			return template, nil
		},
	}

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output: "Discussed Go",
			},
			{
				State: &workflow.ConversationState{
					TotalTurns:  2,
					TotalTokens: 40,
				},
				Output: "Discussed Testing",
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "dynamic_input_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "Discuss {{inputs.topic}}",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 2,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.Inputs = map[string]any{"topic": "Go"}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		ctx := interpolation.NewContext()
		ctx.Inputs = ec.Inputs
		// Simulate dynamic input update
		turnCount++
		if turnCount > 1 {
			ec.Inputs["topic"] = "Testing"
		}
		return ctx
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	requireNoResolutionError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.State.TotalTurns)

	// Verify dynamic inputs were resolved
	require.Len(t, receivedPrompts, 2)
	assertPromptResolved(t, "Discuss Go", receivedPrompts[0], "turn 1 should resolve with initial input")
	assertPromptResolved(t, "Discuss Testing", receivedPrompts[1], "turn 2 should resolve with updated input")
}

// TestConversationManager_T007_PromptResolution_WithStateReferences verifies
// that prompts can reference previous step states using {{states.step.output}}.
func TestConversationManager_T007_PromptResolution_WithStateReferences(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	tokenizer := &mockTokenizer{count: 10}

	var receivedPrompts []string
	callCount := 0

	resolver := &mockResolverT007{
		resolveFunc: func(template string, ctx *interpolation.Context) (string, error) {
			callCount++
			// First call: InitialPrompt
			if callCount == 1 {
				return "What is 2+2?", nil
			}
			// Second call: Prompt with state reference
			if ctx.States != nil {
				if prevState, ok := ctx.States["math_step"]; ok {
					return fmt.Sprintf("The answer was %s, now solve 3+3", prevState.Output), nil
				}
			}
			return template, nil
		},
	}

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output: "4",
			},
			{
				State: &workflow.ConversationState{
					TotalTurns:  2,
					TotalTokens: 40,
				},
				Output: "6",
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "state_ref_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			Prompt:        "The answer was {{states.math_step.output}}, now solve 3+3",
			InitialPrompt: "What is 2+2?",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 2,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.States = map[string]workflow.StepState{
		"math_step": {
			Output: "4",
			Status: workflow.StatusCompleted,
		},
	}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return buildTestContext(ec.Inputs, ec.States)
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	requireNoResolutionError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.State.TotalTurns)

	// Verify state references were resolved
	require.Len(t, receivedPrompts, 2)
	assertPromptResolved(t, "What is 2+2?", receivedPrompts[0], "turn 1 should use InitialPrompt")
	assertPromptResolved(t, "The answer was 4, now solve 3+3", receivedPrompts[1], "turn 2 should resolve state reference")
}

// TestConversationManager_T007_PromptResolution_InitialPromptVsPrompt verifies
// the distinction between InitialPrompt (turn 1) and Prompt (turns 2+).
func TestConversationManager_T007_PromptResolution_InitialPromptVsPrompt(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	tokenizer := &mockTokenizer{count: 10}

	var receivedPrompts []string
	callCount := 0

	resolver := &mockResolverT007{
		resolveFunc: func(template string, ctx *interpolation.Context) (string, error) {
			callCount++
			// First call: InitialPrompt
			if callCount == 1 {
				return "START", nil
			}
			// Subsequent calls: Prompt
			return "CONTINUE", nil
		},
	}

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output: "Response 1",
			},
			{
				State: &workflow.ConversationState{
					TotalTurns:  2,
					TotalTokens: 40,
				},
				Output: "Response 2",
			},
			{
				State: &workflow.ConversationState{
					TotalTurns:  3,
					TotalTokens: 60,
				},
				Output: "Response 3",
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "initial_vs_prompt_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			Prompt:        "{{workflow.action}}-CONTINUE",
			InitialPrompt: "{{workflow.action}}-START",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 3,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	requireNoResolutionError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.State.TotalTurns)

	// Verify InitialPrompt used for turn 1, Prompt for turns 2+
	require.Len(t, receivedPrompts, 3)
	assertPromptResolved(t, "START", receivedPrompts[0], "turn 1 should use InitialPrompt")
	assertPromptResolved(t, "CONTINUE", receivedPrompts[1], "turn 2 should use Prompt")
	assertPromptResolved(t, "CONTINUE", receivedPrompts[2], "turn 3 should use Prompt")
}

// TestConversationManager_T007_PromptResolution_ContextUpdates verifies
// that the execution context is properly updated between turns and prompt
// resolution reflects these updates.
func TestConversationManager_T007_PromptResolution_ContextUpdates(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	tokenizer := &mockTokenizer{count: 10}

	var receivedPrompts []string
	turnCounter := 0

	resolver := &mockResolverT007{
		resolveFunc: func(template string, ctx *interpolation.Context) (string, error) {
			if ctx.Inputs == nil {
				return template, nil
			}
			if counter, ok := ctx.Inputs["counter"].(int); ok {
				return fmt.Sprintf("Turn %d", counter), nil
			}
			return template, nil
		},
	}

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output: "Response 1",
			},
			{
				State: &workflow.ConversationState{
					TotalTurns:  2,
					TotalTokens: 40,
				},
				Output: "Response 2",
			},
			{
				State: &workflow.ConversationState{
					TotalTurns:  3,
					TotalTokens: 60,
				},
				Output: "Response 3",
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "context_update_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "Turn {{inputs.counter}}",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 3,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.Inputs = map[string]any{"counter": 1}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		turnCounter++
		ec.Inputs["counter"] = turnCounter
		ctx := interpolation.NewContext()
		ctx.Inputs = ec.Inputs
		return ctx
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	requireNoResolutionError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.State.TotalTurns)

	// Verify context updates reflected in resolved prompts
	require.Len(t, receivedPrompts, 3)
	assertPromptResolved(t, "Turn 1", receivedPrompts[0], "turn 1 should show counter=1")
	assertPromptResolved(t, "Turn 2", receivedPrompts[1], "turn 2 should show counter=2")
	assertPromptResolved(t, "Turn 3", receivedPrompts[2], "turn 3 should show counter=3")
}

// TestConversationManager_T007_PromptResolution_EmptyTemplate verifies
// behavior when prompt template is empty or contains only whitespace.
func TestConversationManager_T007_PromptResolution_EmptyTemplate(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	tokenizer := &mockTokenizer{count: 10}

	var receivedPrompts []string

	resolver := &mockResolverT007{
		resolveFunc: func(template string, ctx *interpolation.Context) (string, error) {
			return template, nil // Return as-is
		},
	}

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output: "Response",
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "empty_template_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			Prompt:        "",   // Empty
			InitialPrompt: "Hi", // Non-empty
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 1,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	requireNoResolutionError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.State.TotalTurns)

	// Verify empty template handled correctly
	require.Len(t, receivedPrompts, 1)
	assertPromptResolved(t, "Hi", receivedPrompts[0], "should use InitialPrompt for first turn")
}

// TestConversationManager_T007_PromptResolution_ResolutionError verifies
// that errors during prompt resolution are properly propagated.
func TestConversationManager_T007_PromptResolution_ResolutionError(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	tokenizer := &mockTokenizer{count: 10}

	expectedErr := errors.New("variable not found: unknown_var")

	resolver := &mockResolverT007{
		resolveFunc: func(template string, ctx *interpolation.Context) (string, error) {
			if template == "{{inputs.unknown_var}}" {
				return "", expectedErr
			}
			return template, nil
		},
	}

	provider := &mockAgentProvider{
		name: "test-provider",
		result: &workflow.ConversationResult{
			State: &workflow.ConversationState{
				TotalTurns:  1,
				TotalTokens: 20,
			},
			Output: "Response",
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "error_resolution_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			InitialPrompt: "{{inputs.unknown_var}}",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 1,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	assert.Error(t, err, "should propagate resolution error")
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, result, "should not return result on error")
}

// TestConversationManager_T007_PromptResolution_MultipleVariables verifies
// prompts with multiple template variables resolve correctly.
func TestConversationManager_T007_PromptResolution_MultipleVariables(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	tokenizer := &mockTokenizer{count: 10}

	var receivedPrompts []string

	resolver := &mockResolverT007{
		resolveFunc: func(template string, ctx *interpolation.Context) (string, error) {
			if ctx.Inputs == nil {
				return template, nil
			}
			name, _ := ctx.Inputs["name"].(string)
			task, _ := ctx.Inputs["task"].(string)
			priority, _ := ctx.Inputs["priority"].(string)
			return fmt.Sprintf("Hello %s, %s task: %s", name, priority, task), nil
		},
	}

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output: "Response",
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "multi_var_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "Hello {{inputs.name}}, {{inputs.priority}} task: {{inputs.task}}",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 1,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.Inputs = map[string]any{
		"name":     "Alice",
		"task":     "Review PR",
		"priority": "urgent",
	}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		ctx := interpolation.NewContext()
		ctx.Inputs = ec.Inputs
		return ctx
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	requireNoResolutionError(t, err)
	require.NotNil(t, result)

	// Verify multiple variables resolved
	require.Len(t, receivedPrompts, 1)
	assertPromptResolved(t, "Hello Alice, urgent task: Review PR", receivedPrompts[0], "should resolve all variables")
}

// TestConversationManager_T007_PromptResolution_NestedVariables verifies
// that nested variable references resolve correctly.
func TestConversationManager_T007_PromptResolution_NestedVariables(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	tokenizer := &mockTokenizer{count: 10}

	var receivedPrompts []string

	resolver := &mockResolverT007{
		resolveFunc: func(template string, ctx *interpolation.Context) (string, error) {
			// Simulate nested variable resolution
			// {{states[inputs.step_name].output}} -> {{states.prev_step.output}} -> "42"
			if ctx.States != nil && ctx.Inputs != nil {
				if stepName, ok := ctx.Inputs["step_name"].(string); ok {
					if state, exists := ctx.States[stepName]; exists {
						return fmt.Sprintf("Previous result: %s", state.Output), nil
					}
				}
			}
			return template, nil
		},
	}

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output: "Response",
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "nested_var_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "{{states[inputs.step_name].output}}",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 1,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.Inputs = map[string]any{"step_name": "prev_step"}
	execCtx.States = map[string]workflow.StepState{
		"prev_step": {
			Output: "42",
			Status: workflow.StatusCompleted,
		},
	}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return buildTestContext(ec.Inputs, ec.States)
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	requireNoResolutionError(t, err)
	require.NotNil(t, result)

	// Verify nested variable resolved
	require.Len(t, receivedPrompts, 1)
	assertPromptResolved(t, "Previous result: 42", receivedPrompts[0], "should resolve nested variable")
}

// TestConversationManager_T007_PromptResolution_TurnCountVariable verifies
// that turn_count variable is available and updates correctly across turns.
func TestConversationManager_T007_PromptResolution_TurnCountVariable(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	tokenizer := &mockTokenizer{count: 10}

	var receivedPrompts []string
	currentTurn := 0

	resolver := &mockResolverT007{
		resolveFunc: func(template string, ctx *interpolation.Context) (string, error) {
			currentTurn++
			return fmt.Sprintf("This is turn %d", currentTurn), nil
		},
	}

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output: "Response 1",
			},
			{
				State: &workflow.ConversationState{
					TotalTurns:  2,
					TotalTokens: 40,
				},
				Output: "Response 2",
			},
			{
				State: &workflow.ConversationState{
					TotalTurns:  3,
					TotalTokens: 60,
				},
				Output: "Response 3",
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "turn_count_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "This is turn {{turn_count}}",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 3,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	requireNoResolutionError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.State.TotalTurns)

	// Verify turn count increments correctly
	require.Len(t, receivedPrompts, 3)
	assertPromptResolved(t, "This is turn 1", receivedPrompts[0], "turn 1 count should be 1")
	assertPromptResolved(t, "This is turn 2", receivedPrompts[1], "turn 2 count should be 2")
	assertPromptResolved(t, "This is turn 3", receivedPrompts[2], "turn 3 count should be 3")
}

// =============================================================================
// Test Helpers for Component T007
// =============================================================================

// mockResolverT007 implements TemplateResolver for T007 testing
type mockResolverT007 struct {
	// resolveFunc allows custom resolution logic per test
	resolveFunc func(template string, ctx *interpolation.Context) (string, error)
}

func (m *mockResolverT007) Resolve(template string, ctx *interpolation.Context) (string, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(template, ctx)
	}
	// Default: return template as-is (no resolution)
	return template, nil
}

// buildTestContext creates an interpolation.Context for testing
func buildTestContext(inputs map[string]any, states map[string]workflow.StepState) *interpolation.Context {
	ctx := interpolation.NewContext()
	ctx.Inputs = inputs

	for name := range states {
		ctx.States[name] = interpolation.StepStateData{
			Output: states[name].Output,
			Status: string(states[name].Status),
		}
	}

	return ctx
}

// assertPromptResolved verifies that a prompt was resolved correctly
func assertPromptResolved(t *testing.T, expected, actual string, msgAndArgs ...any) {
	t.Helper()
	assert.Equal(t, expected, actual, msgAndArgs...)
}

// requireNoResolutionError verifies that prompt resolution succeeded
func requireNoResolutionError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	require.NoError(t, err, msgAndArgs...)
}
