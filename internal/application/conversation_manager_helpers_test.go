package application

import (
	"context"
	"io"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
)

// Simple resolver mock for testing
type testResolver struct{}

func (t *testResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	return template, nil
}

// TestValidateConversationInputs_HappyPath tests valid inputs are accepted
func TestValidateConversationInputs_HappyPath(t *testing.T) {
	manager := &ConversationManager{}

	step := &workflow.Step{
		Name: "chat",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}
	config := &workflow.ConversationConfig{}

	err := manager.validateConversationInputs(step, config)

	assert.NoError(t, err)
}

// TestValidateConversationInputs_NilStep tests nil step is rejected
func TestValidateConversationInputs_NilStep(t *testing.T) {
	manager := &ConversationManager{}
	config := &workflow.ConversationConfig{}

	err := manager.validateConversationInputs(nil, config)

	assert.Error(t, err)
}

// TestValidateConversationInputs_NilAgent tests nil agent config is rejected
func TestValidateConversationInputs_NilAgent(t *testing.T) {
	manager := &ConversationManager{}

	step := &workflow.Step{
		Name: "chat",
	}
	config := &workflow.ConversationConfig{}

	err := manager.validateConversationInputs(step, config)

	assert.Error(t, err)
}

// TestValidateConversationInputs_NilConfig_Allowed verifies that a nil
// ConversationConfig is accepted: all its fields are optional post-F083, so a
// nil config is treated as an empty config (no ContinueFrom reference).
func TestValidateConversationInputs_NilConfig_Allowed(t *testing.T) {
	manager := &ConversationManager{}

	step := &workflow.Step{
		Name: "chat",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}

	err := manager.validateConversationInputs(step, nil)

	assert.NoError(t, err)
}

// TestInitializeConversationState_FreshStart tests creating new conversation state
func TestInitializeConversationState_FreshStart(t *testing.T) {
	logger := mocks.NewMockLogger()
	resolver := &testResolver{}
	registry := mocks.NewMockAgentRegistry()
	manager := NewConversationManager(logger, resolver, registry)

	step := &workflow.Step{
		Name: "chat",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello world",
		},
	}
	config := &workflow.ConversationConfig{}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	state, resolvedPrompt, err := manager.initializeConversationState(step, "claude", config, execCtx, buildContext)

	assert.NoError(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, "Hello world", resolvedPrompt)
}

// TestInitializeConversationState_ContinueFrom_WithSessionID tests resuming conversation
func TestInitializeConversationState_ContinueFrom_WithSessionID(t *testing.T) {
	logger := mocks.NewMockLogger()
	resolver := &testResolver{}
	registry := mocks.NewMockAgentRegistry()
	manager := NewConversationManager(logger, resolver, registry)

	priorState := workflow.NewConversationState("")
	priorState.SessionID = "session-abc123"
	priorState.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "First question"))
	priorState.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "First answer"))

	step := &workflow.Step{
		Name: "follow_up",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Continue from session",
		},
	}
	config := &workflow.ConversationConfig{
		ContinueFrom: "prior_step",
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = map[string]workflow.StepState{
		"prior_step": {
			Name:         "prior_step",
			Conversation: priorState,
		},
	}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	state, resolvedPrompt, err := manager.initializeConversationState(step, "claude", config, execCtx, buildContext)

	assert.NoError(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, "session-abc123", state.SessionID)
	assert.Equal(t, "Continue from session", resolvedPrompt)
	assert.Equal(t, 2, len(state.Turns))
}

// TestInitializeConversationState_ContinueFrom_StepNotFound tests missing prior step
func TestInitializeConversationState_ContinueFrom_StepNotFound(t *testing.T) {
	logger := mocks.NewMockLogger()
	resolver := &testResolver{}
	registry := mocks.NewMockAgentRegistry()
	manager := NewConversationManager(logger, resolver, registry)

	step := &workflow.Step{
		Name: "follow_up",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Continue",
		},
	}
	config := &workflow.ConversationConfig{
		ContinueFrom: "nonexistent",
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	state, _, err := manager.initializeConversationState(step, "claude", config, execCtx, buildContext)

	assert.Error(t, err)
	assert.Nil(t, state)
	assert.Contains(t, err.Error(), "not found")
}

// TestInitializeConversationState_ContinueFrom_NoConversationState tests prior step without conversation
func TestInitializeConversationState_ContinueFrom_NoConversationState(t *testing.T) {
	logger := mocks.NewMockLogger()
	resolver := &testResolver{}
	registry := mocks.NewMockAgentRegistry()
	manager := NewConversationManager(logger, resolver, registry)

	step := &workflow.Step{
		Name: "follow_up",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Continue",
		},
	}
	config := &workflow.ConversationConfig{
		ContinueFrom: "prior_step",
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = map[string]workflow.StepState{
		"prior_step": {
			Name: "prior_step",
		},
	}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	state, _, err := manager.initializeConversationState(step, "claude", config, execCtx, buildContext)

	assert.Error(t, err)
	assert.Nil(t, state)
	assert.Contains(t, err.Error(), "no conversation state")
}

// TestExecuteTurn_HappyPath tests single turn execution succeeds
func TestExecuteTurn_HappyPath(t *testing.T) {
	logger := mocks.NewMockLogger()
	resolver := &testResolver{}
	registry := mocks.NewMockAgentRegistry()

	provider := mocks.NewMockAgentProvider("claude")
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		result := workflow.NewConversationResult("claude")
		result.State = state
		state.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "Agent response"))
		return result, nil
	})
	_ = registry.Register(provider)

	manager := NewConversationManager(logger, resolver, registry)

	state := workflow.NewConversationState("")
	options := map[string]any{}

	result, err := manager.executeTurn(
		context.Background(),
		provider,
		state,
		"Test prompt",
		options,
		io.Discard,
		io.Discard,
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.State)
}

// TestExecuteTurn_ContextCancellation tests context cancellation is respected
func TestExecuteTurn_ContextCancellation(t *testing.T) {
	logger := mocks.NewMockLogger()
	resolver := &testResolver{}
	registry := mocks.NewMockAgentRegistry()

	provider := mocks.NewMockAgentProvider("claude")
	manager := NewConversationManager(logger, resolver, registry)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := workflow.NewConversationState("")
	options := map[string]any{}

	result, err := manager.executeTurn(
		ctx,
		provider,
		state,
		"Test prompt",
		options,
		io.Discard,
		io.Discard,
	)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestExecuteTurn_ProviderError tests provider error is returned
func TestExecuteTurn_ProviderError(t *testing.T) {
	logger := mocks.NewMockLogger()
	resolver := &testResolver{}
	registry := mocks.NewMockAgentRegistry()

	failingProvider := mocks.NewMockAgentProvider("claude")
	failingProvider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		return nil, assert.AnError
	})
	manager := NewConversationManager(logger, resolver, registry)

	state := workflow.NewConversationState("")
	options := map[string]any{}

	result, err := manager.executeTurn(
		context.Background(),
		failingProvider,
		state,
		"Test prompt",
		options,
		io.Discard,
		io.Discard,
	)

	assert.Error(t, err)
	assert.Nil(t, result)
}
