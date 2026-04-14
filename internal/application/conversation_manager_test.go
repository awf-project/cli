package application_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
)

// mockAgentProvider for conversation testing
type mockAgentProvider struct {
	name              string
	conversationError error
	conversationFunc  func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error)
}

func newMockAgentProvider(name string) *mockAgentProvider {
	return &mockAgentProvider{
		name: name,
		conversationFunc: func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
			result := workflow.NewConversationResult(name)
			result.State = state
			state.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "Agent response to: "+prompt))
			return result, nil
		},
	}
}

func (m *mockAgentProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	return nil, errors.New("single execution not supported in conversation mode")
}

func (m *mockAgentProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	if m.conversationError != nil {
		return nil, m.conversationError
	}
	if m.conversationFunc != nil {
		return m.conversationFunc(ctx, state, prompt, options, stdout, stderr)
	}
	return nil, errors.New("provider not configured")
}

func (m *mockAgentProvider) Name() string {
	return m.name
}

func (m *mockAgentProvider) Validate() error {
	return nil
}

// mockAgentRegistry for testing
type mockAgentRegistry struct {
	providers map[string]ports.AgentProvider
	err       error
}

func newMockAgentRegistry() *mockAgentRegistry {
	return &mockAgentRegistry{
		providers: make(map[string]ports.AgentProvider),
	}
}

func (m *mockAgentRegistry) Register(provider ports.AgentProvider) error {
	m.providers[provider.Name()] = provider
	return nil
}

func (m *mockAgentRegistry) Get(name string) (ports.AgentProvider, error) {
	if m.err != nil {
		return nil, m.err
	}
	if p, ok := m.providers[name]; ok {
		return p, nil
	}
	return nil, errors.New("provider not found: " + name)
}

func (m *mockAgentRegistry) List() []string {
	return nil
}

func (m *mockAgentRegistry) Has(name string) bool {
	_, ok := m.providers[name]
	return ok
}

// mockUserInputReader for testing
type mockUserInputReader struct {
	inputs []string
	index  int
	err    error
}

func newMockUserInputReader(inputs ...string) *mockUserInputReader {
	return &mockUserInputReader{
		inputs: inputs,
		index:  0,
	}
}

func (m *mockUserInputReader) ReadInput(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.index >= len(m.inputs) {
		return "", nil
	}
	input := m.inputs[m.index]
	m.index++
	return input, nil
}

// ============================================================================
// TESTS
// ============================================================================

func TestNewConversationManager(t *testing.T) {
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	manager := application.NewConversationManager(logger, resolver, registry)

	assert.NotNil(t, manager)
}

func TestConversationManager_ExecuteConversation_MultiTurn(t *testing.T) {
	// Arrange: conversation with 2 user messages + 1 empty to exit
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	provider := newMockAgentProvider("claude")
	_ = registry.Register(provider)

	manager := application.NewConversationManager(logger, resolver, registry)
	userInput := newMockUserInputReader(
		"Follow-up question", // Turn 2 user input
		"Another question",   // Turn 3 user input
		"",                   // Empty ends conversation
	)
	manager.SetUserInputReader(userInput)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}
	config := &workflow.ConversationConfig{}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := manager.ExecuteConversation(
		context.Background(),
		step,
		config,
		execCtx,
		buildContext,
		io.Discard,
		io.Discard,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, workflow.StopReasonUserExit, result.State.StoppedBy)
	// Total turns: initial prompt, follow-up, another question = 3 or more turns
	assert.GreaterOrEqual(t, len(result.State.Turns), 3)
}

func TestConversationManager_ExecuteConversation_Error_NoUserInputReader(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	provider := newMockAgentProvider("claude")
	_ = registry.Register(provider)

	manager := application.NewConversationManager(logger, resolver, registry)
	// Do NOT set UserInputReader

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}
	config := &workflow.ConversationConfig{}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := manager.ExecuteConversation(
		context.Background(),
		step,
		config,
		execCtx,
		buildContext,
		io.Discard,
		io.Discard,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "UserInputReader")
}

func TestConversationManager_ExecuteConversation_Error_NilStep(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	manager := application.NewConversationManager(logger, resolver, registry)
	manager.SetUserInputReader(newMockUserInputReader())

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := manager.ExecuteConversation(
		context.Background(),
		nil,
		&workflow.ConversationConfig{},
		execCtx,
		buildContext,
		io.Discard,
		io.Discard,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestConversationManager_ExecuteConversation_Error_NilConfig(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	manager := application.NewConversationManager(logger, resolver, registry)
	manager.SetUserInputReader(newMockUserInputReader())

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := manager.ExecuteConversation(
		context.Background(),
		step,
		nil,
		execCtx,
		buildContext,
		io.Discard,
		io.Discard,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestConversationManager_ExecuteConversation_Error_ProviderNotFound(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	manager := application.NewConversationManager(logger, resolver, registry)
	manager.SetUserInputReader(newMockUserInputReader())

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "nonexistent",
			Prompt:   "Hello",
		},
	}
	config := &workflow.ConversationConfig{}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := manager.ExecuteConversation(
		context.Background(),
		step,
		config,
		execCtx,
		buildContext,
		io.Discard,
		io.Discard,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestConversationManager_ExecuteConversation_Error_UserInputReaderFails(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	provider := newMockAgentProvider("claude")
	_ = registry.Register(provider)

	manager := application.NewConversationManager(logger, resolver, registry)

	failingReader := newMockUserInputReader()
	failingReader.err = errors.New("stdin read failed")
	manager.SetUserInputReader(failingReader)

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}
	config := &workflow.ConversationConfig{}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	_, err := manager.ExecuteConversation(
		context.Background(),
		step,
		config,
		execCtx,
		buildContext,
		io.Discard,
		io.Discard,
	)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stdin read failed")
}

func TestConversationManager_ExecuteConversation_Error_ProviderExecutionFails(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	failingProvider := newMockAgentProvider("claude")
	failingProvider.conversationError = errors.New("provider error")
	_ = registry.Register(failingProvider)

	manager := application.NewConversationManager(logger, resolver, registry)
	manager.SetUserInputReader(newMockUserInputReader())

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}
	config := &workflow.ConversationConfig{}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	_, err := manager.ExecuteConversation(
		context.Background(),
		step,
		config,
		execCtx,
		buildContext,
		io.Discard,
		io.Discard,
	)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider error")
}

func TestConversationManager_ExecuteConversation_Error_ContextCancellation(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	provider := newMockAgentProvider("claude")
	_ = registry.Register(provider)

	manager := application.NewConversationManager(logger, resolver, registry)
	manager.SetUserInputReader(newMockUserInputReader())

	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}
	config := &workflow.ConversationConfig{}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Create and immediately cancel context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	_, err := manager.ExecuteConversation(
		ctx,
		step,
		config,
		execCtx,
		buildContext,
		io.Discard,
		io.Discard,
	)

	// Assert
	assert.Error(t, err)
	// Either error should be context.Canceled or wrapped context.Canceled
}

func TestConversationManager_ExecuteConversation_Error_ContinueFromStepNotFound(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	provider := newMockAgentProvider("claude")
	_ = registry.Register(provider)

	manager := application.NewConversationManager(logger, resolver, registry)
	manager.SetUserInputReader(newMockUserInputReader())

	step := &workflow.Step{
		Name: "follow_up",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Continue",
		},
	}
	config := &workflow.ConversationConfig{
		ContinueFrom: "nonexistent_step",
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = make(map[string]workflow.StepState)

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := manager.ExecuteConversation(
		context.Background(),
		step,
		config,
		execCtx,
		buildContext,
		io.Discard,
		io.Discard,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestConversationManager_ExecuteConversation_Error_ContinueFromNoConversationState(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	resolver := newMockResolver()
	registry := newMockAgentRegistry()

	provider := newMockAgentProvider("claude")
	_ = registry.Register(provider)

	manager := application.NewConversationManager(logger, resolver, registry)
	manager.SetUserInputReader(newMockUserInputReader())

	// Create prior step state WITHOUT conversation state
	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")
	execCtx.States = map[string]workflow.StepState{
		"prior_step": {
			Name:   "prior_step",
			Status: workflow.StatusCompleted,
			// Conversation is nil
		},
	}

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	step := &workflow.Step{
		Name: "follow_up",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Continue",
		},
	}
	config := &workflow.ConversationConfig{
		ContinueFrom: "prior_step",
	}

	// Act
	result, err := manager.ExecuteConversation(
		context.Background(),
		step,
		config,
		execCtx,
		buildContext,
		io.Discard,
		io.Discard,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
}
