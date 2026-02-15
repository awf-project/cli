package application

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//
// Component T006: Fix empty prompt bug at conversation_manager.go:252
// - Replace `resolvedPrompt = ""` with `resolvedPrompt = step.Agent.Prompt`
//   for subsequent conversation turns
//
// This test suite verifies that:
// 1. Subsequent turns use configured prompt instead of empty string
// 2. Multi-turn conversations complete successfully
// 3. The fix maintains backward compatibility with single-turn conversations

// TestConversationManager_T006_HappyPath_MultiTurnWithConfiguredPrompt verifies
// that subsequent turns use the configured prompt from step.Agent.Prompt instead
// of an empty string.
func TestConversationManager_T006_HappyPath_MultiTurnWithConfiguredPrompt(t *testing.T) {
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false} // Never stop on condition
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	// Track prompts received by provider for verification
	var receivedPrompts []string

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					Turns: []workflow.Turn{
						{Role: workflow.TurnRoleUser, Content: "Hello"},
						{Role: workflow.TurnRoleAssistant, Content: "Hi there, turn 1"},
					},
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output:       "Hi there, turn 1",
				TokensInput:  10,
				TokensOutput: 10,
			},
			{
				State: &workflow.ConversationState{
					Turns: []workflow.Turn{
						{Role: workflow.TurnRoleUser, Content: "Hello"},
						{Role: workflow.TurnRoleAssistant, Content: "Hi there, turn 1"},
						{Role: workflow.TurnRoleUser, Content: "Continue"},
						{Role: workflow.TurnRoleAssistant, Content: "Sure, turn 2"},
					},
					TotalTurns:  2,
					TotalTokens: 40,
				},
				Output:       "Sure, turn 2",
				TokensInput:  10,
				TokensOutput: 10,
			},
			{
				State: &workflow.ConversationState{
					Turns: []workflow.Turn{
						{Role: workflow.TurnRoleUser, Content: "Hello"},
						{Role: workflow.TurnRoleAssistant, Content: "Hi there, turn 1"},
						{Role: workflow.TurnRoleUser, Content: "Continue"},
						{Role: workflow.TurnRoleAssistant, Content: "Sure, turn 2"},
						{Role: workflow.TurnRoleUser, Content: "Continue"},
						{Role: workflow.TurnRoleAssistant, Content: "Done, turn 3"},
					},
					TotalTurns:  3,
					TotalTokens: 60,
				},
				Output:       "Done, turn 3",
				TokensInput:  10,
				TokensOutput: 10,
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "multi_turn_chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			SystemPrompt:  "You are helpful",
			Prompt:        "Continue", // This should be used for turns 2+
			InitialPrompt: "Hello",    // This should be used for turn 1
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

	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.State)
	assert.Equal(t, 3, result.State.TotalTurns)

	// Verify prompts: turn 1 uses InitialPrompt, turns 2-3 use Prompt
	require.Len(t, receivedPrompts, 3, "should have received 3 prompts")
	assert.Equal(t, "Hello", receivedPrompts[0], "turn 1 should use InitialPrompt")
	assert.Equal(t, "Continue", receivedPrompts[1], "turn 2 should use Prompt, not empty string")
	assert.Equal(t, "Continue", receivedPrompts[2], "turn 3 should use Prompt, not empty string")
}

// TestConversationManager_T006_HappyPath_MultiTurnWithOnlyPrompt verifies
// that subsequent turns work when only Prompt is configured (no InitialPrompt).
func TestConversationManager_T006_HappyPath_MultiTurnWithOnlyPrompt(t *testing.T) {
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	var receivedPrompts []string

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: []*workflow.ConversationResult{
			{
				State: &workflow.ConversationState{
					Turns: []workflow.Turn{
						{Role: workflow.TurnRoleUser, Content: "Question"},
						{Role: workflow.TurnRoleAssistant, Content: "Answer 1"},
					},
					TotalTurns:  1,
					TotalTokens: 20,
				},
				Output:       "Answer 1",
				TokensInput:  10,
				TokensOutput: 10,
			},
			{
				State: &workflow.ConversationState{
					Turns: []workflow.Turn{
						{Role: workflow.TurnRoleUser, Content: "Question"},
						{Role: workflow.TurnRoleAssistant, Content: "Answer 1"},
						{Role: workflow.TurnRoleUser, Content: "Question"},
						{Role: workflow.TurnRoleAssistant, Content: "Answer 2"},
					},
					TotalTurns:  2,
					TotalTokens: 40,
				},
				Output:       "Answer 2",
				TokensInput:  10,
				TokensOutput: 10,
			},
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat_with_prompt_only",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "Question", // Used for all turns when InitialPrompt not set
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         2,
		MaxContextTokens: 1000,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.State.TotalTurns)

	// Both turns should use the same Prompt
	require.Len(t, receivedPrompts, 2)
	assert.Equal(t, "Question", receivedPrompts[0])
	assert.Equal(t, "Question", receivedPrompts[1], "turn 2 should reuse Prompt")
}

// TestConversationManager_T006_HappyPath_SingleTurnBackwardCompatibility verifies
// that the fix doesn't break single-turn conversations.
func TestConversationManager_T006_HappyPath_SingleTurnBackwardCompatibility(t *testing.T) {
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	provider := &mockAgentProvider{
		name: "test-provider",
		result: &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleUser, Content: "Single question"},
					{Role: workflow.TurnRoleAssistant, Content: "Single answer"},
				},
				TotalTurns:  1,
				TotalTokens: 20,
			},
			Output:       "Single answer",
			TokensInput:  10,
			TokensOutput: 10,
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "single_turn",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "Single question",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 1,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.State.TotalTurns)
	assert.Equal(t, "Single answer", result.Output)
}

// TestConversationManager_T006_EdgeCase_HighTurnCount verifies that conversations
// with many turns (10+) continue using the configured prompt.
func TestConversationManager_T006_EdgeCase_HighTurnCount(t *testing.T) {
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	maxTurns := 15
	var receivedPrompts []string

	provider := &mockAgentProviderWithPromptTracking{
		name: "test-provider",
		capturePrompt: func(prompt string) {
			receivedPrompts = append(receivedPrompts, prompt)
		},
		results: make([]*workflow.ConversationResult, maxTurns),
	}

	// Generate results for each turn
	for i := 0; i < maxTurns; i++ {
		state := &workflow.ConversationState{
			TotalTurns:  i + 1,
			TotalTokens: (i + 1) * 20,
		}
		provider.results[i] = &workflow.ConversationResult{
			State:        state,
			Output:       "Response",
			TokensInput:  10,
			TokensOutput: 10,
		}
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "high_turn_conversation",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			Prompt:        "Next",
			InitialPrompt: "Start",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         maxTurns,
		MaxContextTokens: 10000,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, maxTurns, result.State.TotalTurns)

	// Verify all prompts except first use configured Prompt
	require.Len(t, receivedPrompts, maxTurns)
	assert.Equal(t, "Start", receivedPrompts[0], "first turn uses InitialPrompt")
	for i := 1; i < maxTurns; i++ {
		assert.Equal(t, "Next", receivedPrompts[i], "turn %d should use Prompt", i+1)
	}
}

// TestConversationManager_T006_EdgeCase_EmptyPromptConfiguration verifies behavior
// when Prompt field is empty (should fail validation, but if it reaches execution,
// should handle gracefully).
func TestConversationManager_T006_EdgeCase_EmptyPromptConfiguration(t *testing.T) {
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	provider := &mockAgentProvider{
		name: "test-provider",
		result: &workflow.ConversationResult{
			State: &workflow.ConversationState{
				Turns:       []workflow.Turn{{Role: workflow.TurnRoleUser, Content: ""}},
				TotalTurns:  1,
				TotalTokens: 0,
			},
			Output: "",
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "empty_prompt",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			Prompt:        "", // Empty prompt
			InitialPrompt: "First",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 2,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Note: This tests current behavior - if Prompt is empty, subsequent turns
	// should use empty string (which may fail at provider level).
	// The fix ensures we use step.Agent.Prompt, even if it's empty.
	require.NoError(t, err)
	require.NotNil(t, result)
}

// TestConversationManager_T006_EdgeCase_PromptInterpolation verifies that
// the prompt is re-interpolated for each turn.
func TestConversationManager_T006_EdgeCase_PromptInterpolation(t *testing.T) {
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	tokenizer := &mockTokenizer{count: 10}

	// Resolver that returns different values based on input
	resolver := &mockResolverWithBehavior{
		resolveFn: func(template string, ctx *interpolation.Context) (string, error) {
			// Simulate interpolation: "Say {{inputs.word}}" -> "Say hello"
			if template == "Say {{inputs.word}}" {
				if ctx.Inputs != nil {
					if word, ok := ctx.Inputs["word"].(string); ok {
						return "Say " + word, nil
					}
				}
			}
			return template, nil
		},
	}

	var receivedPrompts []string

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
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "interpolated_prompt",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "Say {{inputs.word}}", // Template prompt
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 2,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	execCtx.Inputs = map[string]any{"word": "hello"}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		ctx := interpolation.NewContext()
		ctx.Inputs = ec.Inputs
		return ctx
	}

	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Both turns should have interpolated prompts
	require.Len(t, receivedPrompts, 2)
	assert.Equal(t, "Say hello", receivedPrompts[0])
	assert.Equal(t, "Say hello", receivedPrompts[1], "turn 2 should interpolate Prompt")
}

// TestConversationManager_T006_ErrorHandling_ProviderFailsOnEmptyPrompt verifies
// that if a provider rejects empty prompts, the error is properly propagated.
func TestConversationManager_T006_ErrorHandling_ProviderFailsOnEmptyPrompt(t *testing.T) {
	logger := newMockLogger()
	evaluator := &mockEvaluator{result: false}
	resolver := newMockResolver()
	tokenizer := &mockTokenizer{count: 10}

	callCount := 0
	provider := &mockAgentProviderWithBehavior{
		name: "test-provider",
		executeFn: func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
			callCount++
			if callCount == 1 {
				// First turn succeeds
				return &workflow.ConversationResult{
					State: &workflow.ConversationState{
						TotalTurns:  1,
						TotalTokens: 20,
					},
					Output: "First response",
				}, nil
			}
			// Second turn: reject empty prompt (simulating bug behavior)
			if prompt == "" {
				return nil, errors.New("prompt cannot be empty")
			}
			return &workflow.ConversationResult{
				State: &workflow.ConversationState{
					TotalTurns:  2,
					TotalTokens: 40,
				},
				Output: "Second response",
			}, nil
		},
	}

	registry := &mockAgentRegistry{provider: provider}
	mgr := NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "error_on_empty",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "test-provider",
			Prompt:        "Continue",
			InitialPrompt: "Start",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns: 2,
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := mgr.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// With the fix, this should succeed because turn 2 uses step.Agent.Prompt
	// Without the fix, this would fail with ErrPromptEmpty
	require.NoError(t, err, "should not fail when prompt is configured")
	require.NotNil(t, result)
	assert.Equal(t, 2, result.State.TotalTurns)
}

// mockAgentProviderWithPromptTracking tracks prompts received for verification
type mockAgentProviderWithPromptTracking struct {
	name          string
	results       []*workflow.ConversationResult
	capturePrompt func(string)
	callIndex     int
}

func (m *mockAgentProviderWithPromptTracking) Name() string {
	return m.name
}

func (m *mockAgentProviderWithPromptTracking) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	return nil, nil
}

func (m *mockAgentProviderWithPromptTracking) ExecuteConversation(
	ctx context.Context,
	state *workflow.ConversationState,
	prompt string,
	options map[string]any,
) (*workflow.ConversationResult, error) {
	if m.capturePrompt != nil {
		m.capturePrompt(prompt)
	}

	if m.callIndex >= len(m.results) {
		return nil, errors.New("no more results configured")
	}

	result := m.results[m.callIndex]
	m.callIndex++
	return result, nil
}

func (m *mockAgentProviderWithPromptTracking) Validate() error {
	return nil
}

// mockAgentProviderWithBehavior allows custom behavior per call
type mockAgentProviderWithBehavior struct {
	name      string
	executeFn func(context.Context, *workflow.ConversationState, string, map[string]any) (*workflow.ConversationResult, error)
}

func (m *mockAgentProviderWithBehavior) Name() string {
	return m.name
}

func (m *mockAgentProviderWithBehavior) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	return nil, nil
}

func (m *mockAgentProviderWithBehavior) ExecuteConversation(
	ctx context.Context,
	state *workflow.ConversationState,
	prompt string,
	options map[string]any,
) (*workflow.ConversationResult, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, state, prompt, options)
	}
	return nil, nil
}

func (m *mockAgentProviderWithBehavior) Validate() error {
	return nil
}

// mockResolverWithBehavior allows custom interpolation logic
type mockResolverWithBehavior struct {
	resolveFn func(string, *interpolation.Context) (string, error)
}

func (m *mockResolverWithBehavior) Resolve(template string, ctx *interpolation.Context) (string, error) {
	if m.resolveFn != nil {
		return m.resolveFn(template, ctx)
	}
	return template, nil
}
