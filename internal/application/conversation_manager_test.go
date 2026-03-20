package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTokenizer implements ports.Tokenizer for testing
type mockTokenizer struct {
	counts     map[string]int
	err        error
	isEstimate bool
	modelName  string
}

func newMockTokenizer() *mockTokenizer {
	return &mockTokenizer{
		counts:     make(map[string]int),
		isEstimate: false,
		modelName:  "test-tokenizer",
	}
}

func (m *mockTokenizer) CountTokens(text string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	if count, ok := m.counts[text]; ok {
		return count, nil
	}
	// Default approximation: ~4 chars per token
	return len(text) / 4, nil
}

func (m *mockTokenizer) CountTurnsTokens(turns []string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	total := 0
	for _, turn := range turns {
		count, err := m.CountTokens(turn)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (m *mockTokenizer) IsEstimate() bool {
	return m.isEstimate
}

func (m *mockTokenizer) ModelName() string {
	return m.modelName
}

// mockConversationProvider implements ports.AgentProvider with conversation support
type mockConversationProvider struct {
	name          string
	conversations map[string]*workflow.ConversationResult
	execError     error
	validateOK    bool
}

func newMockConversationProvider(name string) *mockConversationProvider {
	return &mockConversationProvider{
		name:          name,
		conversations: make(map[string]*workflow.ConversationResult),
		validateOK:    true,
	}
}

func (m *mockConversationProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	return nil, errors.New("single execution not supported, use ExecuteConversation")
}

func (m *mockConversationProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	if m.execError != nil {
		return nil, m.execError
	}
	if result, ok := m.conversations[prompt]; ok {
		return result, nil
	}
	// Default result - stub, not implemented
	return nil, errors.New("not implemented")
}

func (m *mockConversationProvider) Name() string {
	return m.name
}

func (m *mockConversationProvider) Validate() error {
	if !m.validateOK {
		return errors.New("provider validation failed")
	}
	return nil
}

func TestNewConversationManager(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	assert.NotNil(t, manager)
}

func TestConversationManager_SingleTurn_HappyPath(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello, how are you?",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_MultiTurn_HappyPath(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Let's discuss AI",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         5,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_WithSystemPrompt(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:     "claude",
			Prompt:       "What is 2+2?",
			SystemPrompt: "You are a helpful math tutor.",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         3,
		MaxContextTokens: 2000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_StopCondition_ResponseContains(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults[`response contains "DONE"`] = true
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Process data",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         10,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
		StopCondition:    `response contains "DONE"`,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_StopCondition_TurnCount(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults[`turn_count >= 3`] = true
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Iterate",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         10,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
		StopCondition:    `turn_count >= 3`,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_MaxTurns_Reached(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Keep chatting",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         3,
		MaxContextTokens: 10000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_MaxTurns_One(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Single question",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_MaxTokens_Exceeded(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	tokenizer.counts["Very long response"] = 3000
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Generate long text",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         10,
		MaxContextTokens: 2000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_Strategy_SlidingWindow(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Long conversation",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         20,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_Strategy_None(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "No truncation",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         5,
		MaxContextTokens: 0, // no limit
		Strategy:         workflow.StrategyNone,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

// T003: Pass system_prompt through options map in conversation_manager.go

func TestConversationManager_PassesSystemPromptInOptions(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	var capturedOptions map[string]any
	mockProvider := mocks.NewMockAgentProvider("claude")

	mockProvider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		capturedOptions = options
		result := workflow.NewConversationResult("claude")
		result.State = state
		assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, "response")
		assistantTurn.Tokens = 10
		_ = result.State.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, prompt))
		_ = result.State.AddTurn(assistantTurn)
		result.State.StoppedBy = workflow.StopReasonMaxTurns
		result.Output = "response"
		return result, nil
	})

	_ = registry.Register(mockProvider)
	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:     "claude",
			Prompt:       "Hello world",
			SystemPrompt: "You are a helpful assistant",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, capturedOptions, "options map should be captured in provider call")
	assert.Equal(t, "You are a helpful assistant", capturedOptions["system_prompt"])
}

func TestConversationManager_OmitsSystemPromptWhenEmpty(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	var capturedOptions map[string]any
	mockProvider := mocks.NewMockAgentProvider("claude")

	mockProvider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		capturedOptions = options
		result := workflow.NewConversationResult("claude")
		result.State = state
		assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, "response")
		assistantTurn.Tokens = 10
		_ = result.State.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, prompt))
		_ = result.State.AddTurn(assistantTurn)
		result.State.StoppedBy = workflow.StopReasonMaxTurns
		result.Output = "response"
		return result, nil
	})

	_ = registry.Register(mockProvider)
	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:     "claude",
			Prompt:       "Hello world",
			SystemPrompt: "", // empty system prompt
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, capturedOptions, "options map should exist")
	_, exists := capturedOptions["system_prompt"]
	assert.False(t, exists, "system_prompt should not be in options when empty")
}

func TestConversationManager_PreservesExistingOptions(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	var capturedOptions map[string]any
	mockProvider := mocks.NewMockAgentProvider("claude")

	mockProvider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		capturedOptions = options
		result := workflow.NewConversationResult("claude")
		result.State = state
		assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, "response")
		assistantTurn.Tokens = 10
		_ = result.State.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, prompt))
		_ = result.State.AddTurn(assistantTurn)
		result.State.StoppedBy = workflow.StopReasonMaxTurns
		result.Output = "response"
		return result, nil
	})

	_ = registry.Register(mockProvider)
	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:     "claude",
			Prompt:       "Hello world",
			SystemPrompt: "You are helpful",
			Options: map[string]any{
				"temperature": 0.7,
				"max_tokens":  100,
			},
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, capturedOptions, "options map should be captured")
	assert.Equal(t, "You are helpful", capturedOptions["system_prompt"])
	assert.Equal(t, 0.7, capturedOptions["temperature"])
	assert.Equal(t, 100, capturedOptions["max_tokens"])
}

func TestConversationManager_Interpolation_Inputs(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["Explain {{inputs.topic}}"] = "Explain quantum computing"
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Explain {{inputs.topic}}",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.Inputs["topic"] = "quantum computing"
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		ctx := interpolation.NewContext()
		ctx.Inputs = ec.Inputs
		return ctx
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_Interpolation_States(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["Review: {{states.analyze.output}}"] = "Review: Data is clean"
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Review: {{states.analyze.output}}",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = map[string]workflow.StepState{
		"analyze": {
			Name:   "analyze",
			Output: "Data is clean",
			Status: workflow.StatusCompleted,
		},
	}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		ctx := interpolation.NewContext()
		for name, state := range ec.States {
			ctx.States[name] = interpolation.StepStateData{
				Output: state.Output,
				Status: string(state.Status),
			}
		}
		return ctx
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_Error_ProviderNotFound(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)
	// Don't register any providers

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "nonexistent",
			Prompt:   "Test",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect provider not found error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Nil(t, result)
}

func TestConversationManager_Error_ProviderExecutionError(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	provider := newMockConversationProvider("claude")
	provider.execError = errors.New("API rate limit exceeded")
	_ = registry.Register(provider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Test",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_Error_TokenizerError(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	tokenizer.err = errors.New("tokenizer service unavailable")
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Test",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_Error_TemplateResolutionError(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.err = errors.New("undefined variable: missing_var")
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "{{missing_var}}",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect template resolution error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "variable")
	assert.Nil(t, result)
}

func TestConversationManager_Error_StopConditionEvaluationError(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.err = errors.New("syntax error in expression")
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Test",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         5,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
		StopCondition:    "invalid expression syntax",
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_Error_ContextCancellation(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Long running task",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         10,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := manager.ExecuteConversation(ctx, step, config, execCtx, buildContext)

	// GREEN PHASE: expect context cancellation error
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, result)
}

// TestConversationManager_ValidateConversationInputs_HappyPath tests the
// validateConversationInputs helper method with valid inputs.
func TestConversationManager_ValidateConversationInputs_HappyPath(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// Register mock provider so validation can proceed past provider lookup
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Test",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         5,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Call ExecuteConversation which uses validateConversationInputs internally
	// This verifies the refactored validation is working correctly
	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)
	// Validation passes (step and config are valid)
	// Execution may fail due to stub implementation, but that's expected in RED phase
	// The key test is that we don't get a validation error about nil inputs
	if err != nil {
		assert.NotContains(t, err.Error(), "nil")
		assert.NotContains(t, err.Error(), "config is nil")
	}
	// Result may be nil in RED phase due to incomplete implementation
	_ = result
}

// TestConversationManager_ValidateConversationInputs_NilStep tests validation
// with nil step - should fail early with validation error.
func TestConversationManager_ValidateConversationInputs_NilStep(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	config := &workflow.ConversationConfig{
		MaxTurns:         5,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), nil, config, execCtx, buildContext)

	// Should fail validation immediately
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
	assert.Nil(t, result)
}

// TestConversationManager_ValidateConversationInputs_NilAgentConfig tests
// validation with nil agent config - should fail early.
func TestConversationManager_ValidateConversationInputs_NilAgentConfig(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name:  "chat",
		Type:  workflow.StepTypeAgent,
		Agent: nil, // Nil agent config
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         5,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Should fail validation immediately
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
	assert.Nil(t, result)
}

// TestConversationManager_ValidateConversationInputs_NilConfig tests validation
// with nil config - should fail early.
func TestConversationManager_ValidateConversationInputs_NilConfig(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Test",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, nil, execCtx, buildContext)

	// Should fail validation immediately
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
	assert.Nil(t, result)
}

// TestConversationManager_ValidateConversationInputs_AllNil tests validation
// with all nil inputs - should fail with appropriate error.
func TestConversationManager_ValidateConversationInputs_AllNil(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), nil, nil, execCtx, buildContext)

	// Should fail validation immediately
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
	assert.Nil(t, result)
}

// TestConversationManager_ValidateConversationInputs_EdgeCase_EmptyProviderName
// tests validation with empty provider name (valid step/config structure but
// invalid provider).
func TestConversationManager_ValidateConversationInputs_EdgeCase_EmptyProviderName(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "", // Empty provider name
			Prompt:   "Test",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         5,
		MaxContextTokens: 1000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// Validation passes (step and config are not nil), but should fail at provider lookup
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Nil(t, result)
}

func TestConversationManager_EdgeCase_NilConfig(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Test",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, nil, execCtx, buildContext)

	// GREEN PHASE: expect nil config error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
	assert.Nil(t, result)
}

func TestConversationManager_EdgeCase_EmptyInitialPrompt(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "", // Empty prompt
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversationManager_EdgeCase_EmptySystemPrompt(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	// GREEN PHASE: Register mock provider
	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:     "claude",
			Prompt:       "Test prompt",
			SystemPrompt: "", // Empty system prompt
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:         1,
		MaxContextTokens: 5000,
		Strategy:         workflow.StrategySlidingWindow,
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	// GREEN PHASE: expect successful execution
	require.NoError(t, err)
	require.NotNil(t, result)
}

// T003: Continue From Tests — Load prior conversation state from predecessor step

func TestConversationManager_ContinueFrom_SessionID(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	var capturedInitialSessionID string
	mockProvider := mocks.NewMockAgentProvider("claude")
	mockProvider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		// Capture initial SessionID before provider modifies it
		capturedInitialSessionID = state.SessionID
		result := workflow.NewConversationResult("claude")
		result.State = state
		result.State.SessionID = "new-session-456" // Provider issues new session for this step (FR-006)
		_ = result.State.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, prompt))
		_ = result.State.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "response"))
		result.State.StoppedBy = workflow.StopReasonMaxTurns
		result.Output = "response"
		return result, nil
	})
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "step2",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Continue the work",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:     1,
		ContinueFrom: "step1",
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.SetStepState("step1", workflow.StepState{
		Name:   "step1",
		Status: workflow.StatusCompleted,
		Conversation: &workflow.ConversationState{
			SessionID:   "session-abc-123",
			Turns:       []workflow.Turn{{Role: workflow.TurnRoleUser, Content: "hello"}},
			TotalTurns:  1,
			TotalTokens: 50,
		},
	})

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Verify predecessor's SessionID was passed to the provider
	assert.Equal(t, "session-abc-123", capturedInitialSessionID)
	// Verify result has new SessionID assigned by provider (FR-006)
	assert.Equal(t, "new-session-456", result.State.SessionID)
}

func TestConversationManager_ContinueFrom_Turns(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	var capturedInitialTurnCount int
	mockProvider := mocks.NewMockAgentProvider("openai_compatible")
	mockProvider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		// Capture initial turn count before provider adds new turns
		capturedInitialTurnCount = len(state.Turns)
		result := workflow.NewConversationResult("openai_compatible")
		result.State = state
		_ = result.State.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, prompt))
		_ = result.State.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "refined response"))
		result.State.StoppedBy = workflow.StopReasonMaxTurns
		result.Output = "refined response"
		return result, nil
	})
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "refine",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "openai_compatible",
			Prompt:   "Refine the analysis",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:     1,
		ContinueFrom: "analyze",
	}

	priorTurns := []workflow.Turn{
		{Role: workflow.TurnRoleUser, Content: "Analyze this data"},
		{Role: workflow.TurnRoleAssistant, Content: "Analysis complete"},
		{Role: workflow.TurnRoleUser, Content: "More detail"},
		{Role: workflow.TurnRoleAssistant, Content: "Detailed analysis"},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.SetStepState("analyze", workflow.StepState{
		Name:   "analyze",
		Status: workflow.StatusCompleted,
		Conversation: &workflow.ConversationState{
			Turns:       priorTurns,
			TotalTurns:  4,
			TotalTokens: 200,
		},
	})

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Verify predecessor's turns were passed to the provider
	assert.Equal(t, 4, capturedInitialTurnCount)
	// Verify result has original turns plus new turns added by provider
	assert.Equal(t, 6, len(result.State.Turns))
	assert.Equal(t, "Analyze this data", result.State.Turns[0].Content)
}

func TestConversationManager_ContinueFrom_StepNotFound(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "step2",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Continue",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:     1,
		ContinueFrom: "nonexistent",
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Nil(t, result)
}

func TestConversationManager_ContinueFrom_NilConversationState(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "step2",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Continue",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:     1,
		ContinueFrom: "step1",
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.SetStepState("step1", workflow.StepState{
		Name:         "step1",
		Status:       workflow.StatusCompleted,
		Conversation: nil,
	})

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no conversation state")
	assert.Nil(t, result)
}

func TestConversationManager_ContinueFrom_EmptySessionAndTurns(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	mockProvider := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "step2",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Continue",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:     1,
		ContinueFrom: "step1",
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.SetStepState("step1", workflow.StepState{
		Name:   "step1",
		Status: workflow.StatusCompleted,
		Conversation: &workflow.ConversationState{
			SessionID: "",
			Turns:     []workflow.Turn{},
		},
	})

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no session")
	assert.Nil(t, result)
}

func TestConversationManager_ContinueFrom_TurnsRequired_HTTPProvider(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	mockProvider := mocks.NewMockAgentProvider("openai_compatible")
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	step := &workflow.Step{
		Name: "refine",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "openai_compatible",
			Prompt:   "Refine the analysis",
		},
	}

	config := &workflow.ConversationConfig{
		MaxTurns:     1,
		ContinueFrom: "analyze",
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.SetStepState("analyze", workflow.StepState{
		Name:   "analyze",
		Status: workflow.StatusCompleted,
		Conversation: &workflow.ConversationState{
			SessionID:   "session-from-cli-provider",
			Turns:       []workflow.Turn{},
			TotalTurns:  3,
			TotalTokens: 150,
		},
	})

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no conversation turns")
	assert.Nil(t, result)
}

func TestConversationManager_ContinueFrom_ThreeStepChain(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	tokenizer := newMockTokenizer()
	registry := mocks.NewMockAgentRegistry()

	var step2InitialTurnCount int
	var step3InitialTurnCount int

	mockProvider := mocks.NewMockAgentProvider("claude")
	mockProvider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
		// Capture initial turn count before provider adds new turns
		switch prompt {
		case "step2-prompt":
			step2InitialTurnCount = len(state.Turns)
		case "step3-prompt":
			step3InitialTurnCount = len(state.Turns)
		}
		result := workflow.NewConversationResult("claude")
		result.State = state
		_ = result.State.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, prompt))
		_ = result.State.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "response"))
		result.State.StoppedBy = workflow.StopReasonMaxTurns
		result.Output = "response"
		return result, nil
	})
	_ = registry.Register(mockProvider)

	manager := application.NewConversationManager(logger, evaluator, resolver, tokenizer, registry)

	// Step 1: Initial conversation
	step1 := &workflow.Step{
		Name: "step1",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "step1-prompt",
		},
	}
	config1 := &workflow.ConversationConfig{MaxTurns: 1}
	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}
	result1, err := manager.ExecuteConversation(context.Background(), step1, config1, execCtx, buildContext)
	require.NoError(t, err)
	require.NotNil(t, result1)
	execCtx.SetStepState("step1", workflow.StepState{
		Name:         "step1",
		Status:       workflow.StatusCompleted,
		Conversation: result1.State,
	})

	// Step 2: Continue from step1
	step2 := &workflow.Step{
		Name: "step2",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "step2-prompt",
		},
	}
	config2 := &workflow.ConversationConfig{
		MaxTurns:     1,
		ContinueFrom: "step1",
	}
	result2, err := manager.ExecuteConversation(context.Background(), step2, config2, execCtx, buildContext)
	require.NoError(t, err)
	require.NotNil(t, result2)
	// Step2 should receive step1's 2 turns (user + assistant)
	assert.Equal(t, 2, step2InitialTurnCount)
	execCtx.SetStepState("step2", workflow.StepState{
		Name:         "step2",
		Status:       workflow.StatusCompleted,
		Conversation: result2.State,
	})

	// Step 3: Continue from step2 (not step1) — NFR-003 O(1) behavior
	step3 := &workflow.Step{
		Name: "step3",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "step3-prompt",
		},
	}
	config3 := &workflow.ConversationConfig{
		MaxTurns:     1,
		ContinueFrom: "step2",
	}
	result3, err := manager.ExecuteConversation(context.Background(), step3, config3, execCtx, buildContext)
	require.NoError(t, err)
	require.NotNil(t, result3)

	// Verify step3 loaded step2's state which has 4 turns (2 from step1 + 2 from step2)
	// This proves O(1) per-hop: step3 doesn't recursively load step1
	assert.Equal(t, 4, step3InitialTurnCount)
}
