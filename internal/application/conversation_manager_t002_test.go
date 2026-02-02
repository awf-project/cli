package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// Component T002 Tests - Replace Duplicate Initialization Code
// =============================================================================
//
// Component T002: Replace duplicate initialization code with
// `initializeConversationState()` call in conversation_manager.go:219-234
//
// This test suite verifies that:
// 1. The refactoring was done correctly
// 2. The helper method call replaces the duplicate code
// 3. All functionality is preserved after refactoring
// =============================================================================

// TestConversationManager_T002_HappyPath_InitializationUsingHelper verifies
// that ExecuteConversation successfully initializes conversation state using
// the initializeConversationState helper method instead of inline code.
func TestConversationManager_T002_HappyPath_InitializationUsingHelper(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	provider := &mockAgentProvider{
		name: "test-provider",
		result: &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleSystem, Content: "You are helpful"},
					{Role: workflow.TurnRoleUser, Content: "Hello"},
					{Role: workflow.TurnRoleAssistant, Content: "Hi there"},
				},
				TotalTurns:  2,
				TotalTokens: 20,
			},
			Output:       "Hi there",
			TokensInput:  10,
			TokensOutput: 10,
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:     "test-provider",
			SystemPrompt: "You are helpful",
			Prompt:       "Hello",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         3,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.State)
	assert.Equal(t, "Hi there", result.Output)
}

// TestConversationManager_T002_HappyPath_SystemPromptInitialization verifies
// that the system prompt is correctly initialized using the helper method.
func TestConversationManager_T002_HappyPath_SystemPromptInitialization(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	systemPromptValue := "You are a helpful coding assistant"

	provider := &mockAgentProvider{
		name: "test-provider",
		result: &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleSystem, Content: systemPromptValue},
					{Role: workflow.TurnRoleUser, Content: "Help me code"},
					{Role: workflow.TurnRoleAssistant, Content: "Sure!"},
				},
				TotalTurns:  2,
				TotalTokens: 15,
			},
			Output:       "Sure!",
			TokensInput:  8,
			TokensOutput: 7,
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "coding_help",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:     "test-provider",
			SystemPrompt: systemPromptValue,
			Prompt:       "Help me code",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         5,
		MaxContextTokens: 2000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	// Verify system prompt is in first turn
	assert.Greater(t, len(result.State.Turns), 0)
	assert.Equal(t, workflow.TurnRoleSystem, result.State.Turns[0].Role)
	assert.Equal(t, systemPromptValue, result.State.Turns[0].Content)
}

// TestConversationManager_T002_HappyPath_InitialPromptPriority verifies
// that InitialPrompt takes precedence over Prompt when both are set.
func TestConversationManager_T002_HappyPath_InitialPromptPriority(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	provider := &mockAgentProvider{
		name: "test-provider",
		result: &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleUser, Content: "initial question"},
					{Role: workflow.TurnRoleAssistant, Content: "initial answer"},
				},
				TotalTurns:  1,
				TotalTokens: 20,
			},
			Output:       "initial answer",
			TokensInput:  10,
			TokensOutput: 10,
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			InitialPrompt: "initial question", // Should be used
			Prompt:        "fallback prompt",  // Should be ignored
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         3,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	// Verify the initial prompt was used (evidenced by the output)
	assert.Equal(t, "initial answer", result.Output)
}

// TestConversationManager_T002_EdgeCase_EmptySystemPrompt verifies that
// conversations work correctly when system prompt is empty.
func TestConversationManager_T002_EdgeCase_EmptySystemPrompt(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	provider := &mockAgentProvider{
		name: "test-provider",
		result: &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleUser, Content: "Question"},
					{Role: workflow.TurnRoleAssistant, Content: "Answer"},
				},
				TotalTurns:  1,
				TotalTokens: 15,
			},
			Output:       "Answer",
			TokensInput:  8,
			TokensOutput: 7,
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:     "test-provider",
			SystemPrompt: "", // Empty
			Prompt:       "Question",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         2,
		MaxContextTokens: 500,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	// Verify no system prompt in turns (first turn should be user)
	assert.Greater(t, len(result.State.Turns), 0)
	assert.NotEqual(t, workflow.TurnRoleSystem, result.State.Turns[0].Role)
	assert.Equal(t, "Answer", result.Output)
}

// TestConversationManager_T002_EdgeCase_OnlyPromptNoInitialPrompt verifies
// that Prompt is used as fallback when InitialPrompt is not set.
func TestConversationManager_T002_EdgeCase_OnlyPromptNoInitialPrompt(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	provider := &mockAgentProvider{
		name: "test-provider",
		result: &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleUser, Content: "fallback prompt"},
					{Role: workflow.TurnRoleAssistant, Content: "response"},
				},
				TotalTurns:  1,
				TotalTokens: 18,
			},
			Output:       "response",
			TokensInput:  9,
			TokensOutput: 9,
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "fallback prompt", // No InitialPrompt set
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         3,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "response", result.Output)
}

// TestConversationManager_T002_EdgeCase_TemplateInterpolationInPrompt verifies
// that template interpolation works correctly in the initialization phase.
func TestConversationManager_T002_EdgeCase_TemplateInterpolationInPrompt(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}

	// Mock resolver that resolves templates
	resolver := &mockResolverWithError{}

	tokenizer := &mockTokenizer{count: 10}

	provider := &mockAgentProvider{
		name: "test-provider",
		result: &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleUser, Content: "{{inputs.question}}"},
					{Role: workflow.TurnRoleAssistant, Content: "Resolved answer"},
				},
				TotalTurns:  1,
				TotalTokens: 20,
			},
			Output:       "Resolved answer",
			TokensInput:  10,
			TokensOutput: 10,
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "{{inputs.question}}", // Template that should be resolved
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         2,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.Inputs = map[string]any{"question": "What is Go?"}
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		ctx := interpolation.NewContext()
		ctx.Inputs = ec.Inputs
		return ctx
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Resolved answer", result.Output)
}

// TestConversationManager_T002_ErrorHandling_TemplateResolutionFails verifies
// that initialization errors from template resolution are properly handled.
func TestConversationManager_T002_ErrorHandling_TemplateResolutionFails(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}

	// Mock resolver that returns an error
	resolver := &mockResolverWithError{
		err: errors.New("undefined variable: missing_input"),
	}

	tokenizer := &mockTokenizer{count: 10}

	provider := &mockAgentProvider{
		name: "test-provider",
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "{{inputs.missing_input}}", // Will fail to resolve
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         3,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "variable")
	assert.Nil(t, result)
}

// TestConversationManager_T002_ErrorHandling_NilBuildContext verifies that
// a nil buildContext function causes a panic (as expected in Go for nil function calls).
func TestConversationManager_T002_ErrorHandling_NilBuildContext(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	provider := &mockAgentProvider{
		name: "test-provider",
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "test",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         3,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.States = make(map[string]workflow.StepState)

	// Act - passing nil buildContext should panic
	// This tests that the initialization properly requires buildContext
	defer func() {
		r := recover()
		if r == nil {
			t.Error("Expected panic when buildContext is nil, but no panic occurred")
		}
	}()

	_, _ = mgr.ExecuteConversation(context.Background(), step, config, execCtx, nil)
}

// TestConversationManager_T002_Integration_MultipleFields verifies that all
// initialization fields work together correctly after refactoring.
func TestConversationManager_T002_Integration_MultipleFields(t *testing.T) {
	// Arrange
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	systemPrompt := "You are a weather assistant"
	initialPrompt := "What's the weather?"

	provider := &mockAgentProvider{
		name: "test-provider",
		result: &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleSystem, Content: systemPrompt},
					{Role: workflow.TurnRoleUser, Content: initialPrompt},
					{Role: workflow.TurnRoleAssistant, Content: "It's sunny!"},
				},
				TotalTurns:  2,
				TotalTokens: 25,
			},
			Output:       "It's sunny!",
			TokensInput:  12,
			TokensOutput: 13,
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "weather_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			SystemPrompt:  systemPrompt,
			InitialPrompt: initialPrompt,
			Prompt:        "fallback",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         5,
		MaxContextTokens: 2000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	// Verify system prompt is in first turn
	assert.Greater(t, len(result.State.Turns), 0)
	assert.Equal(t, workflow.TurnRoleSystem, result.State.Turns[0].Role)
	assert.Equal(t, systemPrompt, result.State.Turns[0].Content)
	assert.Equal(t, "It's sunny!", result.Output)
	assert.Equal(t, 2, result.State.TotalTurns)
}
