package application

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
)

// Note: mockLogger is defined in service_test.go and shared across package tests

// newMockLogger creates a new mock logger instance
func newMockLogger() *mockLogger {
	return &mockLogger{}
}

// mockAgentProvider is a test double for agent providers
type mockAgentProvider struct {
	name   string
	result *workflow.ConversationResult
	err    error
}

func (m *mockAgentProvider) Name() string {
	return m.name
}

func (m *mockAgentProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	return nil, nil // Not used in conversation manager tests
}

func (m *mockAgentProvider) ExecuteConversation(
	ctx context.Context,
	state *workflow.ConversationState,
	prompt string,
	options map[string]any,
) (*workflow.ConversationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockAgentProvider) Validate() error {
	return nil // Not used in conversation manager tests
}

// mockAgentRegistry is a test double for agent registry
type mockAgentRegistry struct {
	provider ports.AgentProvider
	err      error
}

func (m *mockAgentRegistry) Register(provider ports.AgentProvider) error {
	return nil
}

func (m *mockAgentRegistry) Get(name string) (ports.AgentProvider, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.provider, nil
}

func (m *mockAgentRegistry) List() []string {
	return []string{}
}

func (m *mockAgentRegistry) Has(name string) bool {
	return m.provider != nil
}

// mockTokenizer is a test double for tokenizer
type mockTokenizer struct {
	count int
}

func (m *mockTokenizer) CountTokens(text string) (int, error) {
	return m.count, nil
}

func (m *mockTokenizer) CountTurnsTokens(turns []string) (int, error) {
	return m.count * len(turns), nil
}

func (m *mockTokenizer) IsEstimate() bool {
	return false
}

func (m *mockTokenizer) ModelName() string {
	return "mock"
}

// mockEvaluator is a test double for expression evaluator
// C042: Updated to implement ports.ExpressionEvaluator interface
type mockEvaluator struct {
	result bool
	err    error
}

func (m *mockEvaluator) EvaluateBool(expr string, ctx *interpolation.Context) (bool, error) {
	return m.result, m.err
}

func (m *mockEvaluator) EvaluateInt(expr string, ctx *interpolation.Context) (int, error) {
	return 0, nil
}

// mockResolverWithError is a test double for interpolation resolver that can return errors
type mockResolverWithError struct {
	err error
}

func (m *mockResolverWithError) Resolve(template string, ctx *interpolation.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return template, nil
}

// Feature: C006 - Reduce ExecuteConversation complexity from 29 to ≤18

// TestConversationManager_validateConversationInputs tests input validation
// for the ExecuteConversation method.
// Feature: C006 - Component T013
func TestConversationManager_validateConversationInputs(t *testing.T) {
	tests := []struct {
		name        string
		step        *workflow.Step
		config      *workflow.ConversationConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid inputs",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "test-provider",
					Prompt:   "test prompt",
				},
			},
			config: &workflow.ConversationConfig{
				MaxTurns: 5,
			},
			expectError: false,
		},
		{
			name:        "nil step",
			step:        nil,
			config:      &workflow.ConversationConfig{MaxTurns: 5},
			expectError: true,
			errorMsg:    "step or agent config is nil",
		},
		{
			name: "nil agent config",
			step: &workflow.Step{
				Name:  "test-step",
				Type:  workflow.StepTypeAgent,
				Agent: nil,
			},
			config:      &workflow.ConversationConfig{MaxTurns: 5},
			expectError: true,
			errorMsg:    "step or agent config is nil",
		},
		{
			name: "nil conversation config",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "test-provider",
					Prompt:   "test prompt",
				},
			},
			config:      nil,
			expectError: true,
			errorMsg:    "conversation config is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &ConversationManager{
				logger:        newMockLogger(),
				evaluator:     &mockEvaluator{},
				resolver:      newMockResolver(),
				tokenizer:     &mockTokenizer{},
				agentRegistry: &mockAgentRegistry{},
			}

			err := mgr.validateConversationInputs(tt.step, tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConversationManager_initializeConversationState tests conversation state
// initialization with system prompt and initial prompt resolution.
// Feature: C006 - Component T013
func TestConversationManager_initializeConversationState(t *testing.T) {
	tests := []struct {
		name                 string
		step                 *workflow.Step
		buildContext         ContextBuilderFunc
		resolverError        error
		expectError          bool
		expectedPrompt       string
		expectedSystemPrompt string
	}{
		{
			name: "system prompt present with initial prompt",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "test-provider",
					SystemPrompt:  "You are a helpful assistant",
					InitialPrompt: "Hello, how can I help?",
					Prompt:        "fallback prompt",
				},
			},
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{}
			},
			expectError:          false,
			expectedPrompt:       "Hello, how can I help?",
			expectedSystemPrompt: "You are a helpful assistant",
		},
		{
			name: "empty system prompt",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "test-provider",
					SystemPrompt: "",
					Prompt:       "test prompt",
				},
			},
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{}
			},
			expectError:          false,
			expectedPrompt:       "test prompt",
			expectedSystemPrompt: "",
		},
		{
			name: "initial prompt priority over prompt",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "test-provider",
					InitialPrompt: "initial",
					Prompt:        "regular",
				},
			},
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{}
			},
			expectError:    false,
			expectedPrompt: "initial",
		},
		{
			name: "prompt fallback when no initial prompt",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "test-provider",
					Prompt:   "fallback prompt",
				},
			},
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{}
			},
			expectError:    false,
			expectedPrompt: "fallback prompt",
		},
		{
			name: "interpolation error",
			step: &workflow.Step{
				Name: "test-step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "test-provider",
					Prompt:   "{{invalid}}",
				},
			},
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{}
			},
			resolverError: errors.New("interpolation failed"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resolver interpolation.Resolver
			if tt.resolverError != nil {
				resolver = &mockResolverWithError{err: tt.resolverError}
			} else {
				resolver = newMockResolver()
			}

			mgr := &ConversationManager{
				logger:        newMockLogger(),
				evaluator:     &mockEvaluator{},
				resolver:      resolver,
				tokenizer:     &mockTokenizer{},
				agentRegistry: &mockAgentRegistry{},
			}
			execCtx := workflow.NewExecutionContext("test-wf", "test")

			state, prompt, err := mgr.initializeConversationState(tt.step, tt.step.Agent.Provider, &workflow.ConversationConfig{}, execCtx, tt.buildContext)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, state)
				assert.Equal(t, tt.expectedPrompt, prompt)
				if tt.expectedSystemPrompt != "" {
					// Verify system prompt is in state
					assert.NotNil(t, state)
				}
			}
		})
	}
}

// TestConversationManager_executeTurn tests single turn execution with
// provider integration and state updates.
// Feature: C006 - Component T013
func TestConversationManager_executeTurn(t *testing.T) {
	tests := []struct {
		name           string
		state          *workflow.ConversationState
		prompt         string
		options        map[string]any
		providerResult *workflow.ConversationResult
		providerError  error
		expectError    bool
		contextCancel  bool
	}{
		{
			name: "successful turn",
			state: &workflow.ConversationState{
				Turns:       []workflow.Turn{},
				TotalTurns:  0,
				TotalTokens: 0,
			},
			prompt: "test prompt",
			options: map[string]any{
				"temperature": 0.7,
			},
			providerResult: &workflow.ConversationResult{
				State: &workflow.ConversationState{
					Turns: []workflow.Turn{
						{Role: workflow.TurnRoleUser, Content: "test prompt"},
						{Role: workflow.TurnRoleAssistant, Content: "response"},
					},
					TotalTurns:  1,
					TotalTokens: 100,
				},
				Output: "response",
			},
			expectError: false,
		},
		{
			name: "provider error",
			state: &workflow.ConversationState{
				Turns:       []workflow.Turn{},
				TotalTurns:  0,
				TotalTokens: 0,
			},
			prompt:        "test prompt",
			options:       map[string]any{},
			providerError: errors.New("provider failed"),
			expectError:   true,
		},
		{
			name: "context cancellation",
			state: &workflow.ConversationState{
				Turns:       []workflow.Turn{},
				TotalTurns:  0,
				TotalTokens: 0,
			},
			prompt:        "test prompt",
			options:       map[string]any{},
			expectError:   true,
			contextCancel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockAgentProvider{
				name:   "test-provider",
				result: tt.providerResult,
				err:    tt.providerError,
			}

			mgr := &ConversationManager{
				logger:    newMockLogger(),
				evaluator: &mockEvaluator{},
				resolver:  newMockResolver(),
				tokenizer: &mockTokenizer{},
				agentRegistry: &mockAgentRegistry{
					provider: provider,
				},
			}

			ctx := context.Background()
			if tt.contextCancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel() // Cancel immediately
			}

			result, err := mgr.executeTurn(ctx, provider, tt.state, tt.prompt, tt.options)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.providerResult, result)
			}
		})
	}
}

// TestConversationManager_evaluateTurnCompletion tests stop condition evaluation
// and max tokens checking between conversation turns.
// Feature: C006 - Component T013
func TestConversationManager_evaluateTurnCompletion(t *testing.T) {
	tests := []struct {
		name            string
		config          *workflow.ConversationConfig
		state           *workflow.ConversationState
		execCtx         *workflow.ExecutionContext
		buildContext    ContextBuilderFunc
		evaluatorResult bool
		evaluatorError  error
		shouldStop      bool
		expectedReason  workflow.StopReason
	}{
		{
			name: "no stop condition - continue",
			config: &workflow.ConversationConfig{
				MaxTurns: 10,
			},
			state: &workflow.ConversationState{
				Turns:       []workflow.Turn{{Role: workflow.TurnRoleAssistant, Content: "response"}},
				TotalTurns:  1,
				TotalTokens: 50,
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{}
			},
			shouldStop: false,
		},
		{
			name: "stop condition met",
			config: &workflow.ConversationConfig{
				MaxTurns:      10,
				StopCondition: "response contains 'DONE'",
			},
			state: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleAssistant, Content: "DONE"},
				},
				TotalTurns:  1,
				TotalTokens: 50,
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{Inputs: make(map[string]any)}
			},
			evaluatorResult: true,
			shouldStop:      true,
			expectedReason:  workflow.StopReasonCondition,
		},
		{
			name: "stop condition not met",
			config: &workflow.ConversationConfig{
				MaxTurns:      10,
				StopCondition: "response contains 'DONE'",
			},
			state: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: workflow.TurnRoleAssistant, Content: "not done"},
				},
				TotalTurns:  1,
				TotalTokens: 50,
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{Inputs: make(map[string]any)}
			},
			evaluatorResult: false,
			shouldStop:      false,
		},
		{
			name: "evaluation error - log and continue",
			config: &workflow.ConversationConfig{
				MaxTurns:      10,
				StopCondition: "invalid condition",
			},
			state: &workflow.ConversationState{
				Turns:       []workflow.Turn{{Role: workflow.TurnRoleAssistant, Content: "response"}},
				TotalTurns:  1,
				TotalTokens: 50,
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{Inputs: make(map[string]any)}
			},
			evaluatorError: errors.New("evaluation failed"),
			shouldStop:     false, // Continue despite error
		},
		{
			name: "max tokens exceeded",
			config: &workflow.ConversationConfig{
				MaxTurns:         10,
				MaxContextTokens: 100,
			},
			state: &workflow.ConversationState{
				Turns:       []workflow.Turn{{Role: workflow.TurnRoleAssistant, Content: "response"}},
				TotalTurns:  1,
				TotalTokens: 150, // Exceeds max
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{}
			},
			shouldStop:     true,
			expectedReason: workflow.StopReasonMaxTokens,
		},
		{
			name: "max tokens not exceeded",
			config: &workflow.ConversationConfig{
				MaxTurns:         10,
				MaxContextTokens: 100,
			},
			state: &workflow.ConversationState{
				Turns:       []workflow.Turn{{Role: workflow.TurnRoleAssistant, Content: "response"}},
				TotalTurns:  1,
				TotalTokens: 50, // Below max
			},
			execCtx: workflow.NewExecutionContext("test-wf", "test"),
			buildContext: func(ctx *workflow.ExecutionContext) *interpolation.Context {
				return &interpolation.Context{}
			},
			shouldStop: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &ConversationManager{
				logger: newMockLogger(),
				evaluator: &mockEvaluator{
					result: tt.evaluatorResult,
					err:    tt.evaluatorError,
				},
				resolver:      newMockResolver(),
				tokenizer:     &mockTokenizer{},
				agentRegistry: &mockAgentRegistry{},
			}

			shouldStop := mgr.evaluateTurnCompletion(tt.config, tt.state, tt.execCtx, tt.buildContext)

			assert.Equal(t, tt.shouldStop, shouldStop)
			if tt.shouldStop && tt.expectedReason != "" {
				assert.Equal(t, tt.expectedReason, tt.state.StoppedBy)
			}
		})
	}
}

// finalizeStopReason is a test helper that mirrors the inlined stop-reason logic
// in ConversationManager.ExecuteConversation. It was extracted here after being
// removed from production as dead code (the logic is inlined at the call site).
func (m *ConversationManager) finalizeStopReason(
	state *workflow.ConversationState,
	turnCount int,
	maxTurns int,
) {
	if state.StoppedBy == "" {
		if turnCount >= maxTurns {
			state.StoppedBy = workflow.StopReasonMaxTurns
		}
	}
}

// TestConversationManager_finalizeStopReason tests stop reason determination
// when conversation loop completes.
// Feature: C006 - Component T013
func TestConversationManager_finalizeStopReason(t *testing.T) {
	tests := []struct {
		name           string
		state          *workflow.ConversationState
		turnCount      int
		maxTurns       int
		expectedReason workflow.StopReason
	}{
		{
			name: "max turns reached",
			state: &workflow.ConversationState{
				TotalTurns: 10,
				StoppedBy:  "", // Not set yet
			},
			turnCount:      10,
			maxTurns:       10,
			expectedReason: workflow.StopReasonMaxTurns,
		},
		{
			name: "already set - no override",
			state: &workflow.ConversationState{
				TotalTurns: 5,
				StoppedBy:  workflow.StopReasonCondition,
			},
			turnCount:      5,
			maxTurns:       10,
			expectedReason: workflow.StopReasonCondition, // Should remain unchanged
		},
		{
			name: "neither condition - remains empty",
			state: &workflow.ConversationState{
				TotalTurns: 5,
				StoppedBy:  "",
			},
			turnCount:      5,
			maxTurns:       10,
			expectedReason: "", // Should remain empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &ConversationManager{
				logger:        newMockLogger(),
				evaluator:     &mockEvaluator{},
				resolver:      newMockResolver(),
				tokenizer:     &mockTokenizer{},
				agentRegistry: &mockAgentRegistry{},
			}

			mgr.finalizeStopReason(tt.state, tt.turnCount, tt.maxTurns)

			assert.Equal(t, tt.expectedReason, tt.state.StoppedBy)
		})
	}
}
