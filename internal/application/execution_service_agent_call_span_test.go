package application_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestAgentCallSpan_EmitsProviderAttribute verifies that agent.call span emits
// the provider attribute with the resolved provider name from the agent config.
func TestAgentCallSpan_EmitsProviderAttribute(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "agent_step",
		Steps: map[string]*workflow.Step{
			"agent_step": {
				Name: "agent_step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "test prompt",
					Options:  map[string]any{},
				},
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockAgentSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "agent.call").
		Return(context.Background(), mockAgentSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	mockAgentProvider := new(MockAgentProvider)
	mockAgentProvider.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&workflow.AgentResult{
			Provider:  "claude",
			Output:    "response",
			Tokens:    150,
			StartedAt: time.Now(),
		}, nil)

	mockRegistry := new(MockAgentRegistry)
	mockRegistry.On("Get", "claude").Return(mockAgentProvider, nil)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()
	svc.SetTracer(mockTracer)
	svc.SetAgentRegistry(mockRegistry)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	mockAgentSpan.AssertCalled(t, "SetAttribute", "provider", "claude")
}

// TestAgentCallSpan_EmitsModelAttribute verifies that agent.call span emits
// the model attribute extracted from step.Agent.Options["model"].
func TestAgentCallSpan_EmitsModelAttribute(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "agent_step",
		Steps: map[string]*workflow.Step{
			"agent_step": {
				Name: "agent_step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "test prompt",
					Options: map[string]any{
						"model": "claude-3-sonnet-20240229",
					},
				},
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockAgentSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "agent.call").
		Return(context.Background(), mockAgentSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	mockAgentProvider := new(MockAgentProvider)
	mockAgentProvider.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&workflow.AgentResult{
			Provider:  "claude",
			Output:    "response",
			Tokens:    200,
			StartedAt: time.Now(),
		}, nil)

	mockRegistry := new(MockAgentRegistry)
	mockRegistry.On("Get", "claude").Return(mockAgentProvider, nil)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()
	svc.SetTracer(mockTracer)
	svc.SetAgentRegistry(mockRegistry)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	mockAgentSpan.AssertCalled(t, "SetAttribute", "model", "claude-3-sonnet-20240229")
}

// TestAgentCallSpan_EmitsTokensUsedAttribute verifies that agent.call span emits
// the tokens_used attribute with the actual token count from the agent result.
func TestAgentCallSpan_EmitsTokensUsedAttribute(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "agent_step",
		Steps: map[string]*workflow.Step{
			"agent_step": {
				Name: "agent_step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "test prompt",
					Options:  map[string]any{},
				},
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockAgentSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "agent.call").
		Return(context.Background(), mockAgentSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	mockAgentProvider := new(MockAgentProvider)
	mockAgentProvider.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&workflow.AgentResult{
			Provider:  "claude",
			Output:    "response",
			Tokens:    350,
			StartedAt: time.Now(),
		}, nil)

	mockRegistry := new(MockAgentRegistry)
	mockRegistry.On("Get", "claude").Return(mockAgentProvider, nil)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()
	svc.SetTracer(mockTracer)
	svc.SetAgentRegistry(mockRegistry)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	mockAgentSpan.AssertCalled(t, "SetAttribute", "tokens_used", 350)
}

// TestAgentCallSpan_EmitsAllAttributesTogether verifies that agent.call span
// emits all three attributes (provider, model, tokens_used) in a single execution.
func TestAgentCallSpan_EmitsAllAttributesTogether(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "agent_step",
		Steps: map[string]*workflow.Step{
			"agent_step": {
				Name: "agent_step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "gemini",
					Prompt:   "test prompt",
					Options: map[string]any{
						"model": "gemini-2.0-flash",
					},
				},
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockAgentSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "agent.call").
		Return(context.Background(), mockAgentSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	mockAgentProvider := new(MockAgentProvider)
	mockAgentProvider.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&workflow.AgentResult{
			Provider:  "gemini",
			Output:    "response",
			Tokens:    425,
			StartedAt: time.Now(),
		}, nil)

	mockRegistry := new(MockAgentRegistry)
	mockRegistry.On("Get", "gemini").Return(mockAgentProvider, nil)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()
	svc.SetTracer(mockTracer)
	svc.SetAgentRegistry(mockRegistry)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	mockAgentSpan.AssertCalled(t, "SetAttribute", "provider", "gemini")
	mockAgentSpan.AssertCalled(t, "SetAttribute", "model", "gemini-2.0-flash")
	mockAgentSpan.AssertCalled(t, "SetAttribute", "tokens_used", 425)
}

// TestAgentCallSpan_HandlesZeroTokens verifies that agent.call span correctly
// handles the case where the agent result reports 0 tokens.
func TestAgentCallSpan_HandlesZeroTokens(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "agent_step",
		Steps: map[string]*workflow.Step{
			"agent_step": {
				Name: "agent_step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "test-provider",
					Prompt:   "test prompt",
					Options:  map[string]any{},
				},
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockAgentSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "agent.call").
		Return(context.Background(), mockAgentSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	mockAgentProvider := new(MockAgentProvider)
	mockAgentProvider.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&workflow.AgentResult{
			Provider:  "test-provider",
			Output:    "response",
			Tokens:    0,
			StartedAt: time.Now(),
		}, nil)

	mockRegistry := new(MockAgentRegistry)
	mockRegistry.On("Get", "test-provider").Return(mockAgentProvider, nil)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()
	svc.SetTracer(mockTracer)
	svc.SetAgentRegistry(mockRegistry)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	mockAgentSpan.AssertCalled(t, "SetAttribute", "tokens_used", 0)
}

// TestAgentCallSpan_EmitsWhenNoModelOption verifies that agent.call span
// handles the case where no model is specified in step.Agent.Options.
func TestAgentCallSpan_EmitsWhenNoModelOption(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "agent_step",
		Steps: map[string]*workflow.Step{
			"agent_step": {
				Name: "agent_step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "test prompt",
					Options:  map[string]any{},
				},
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	mockTracer := new(MockTracer)
	mockAgentSpan := newMockSpan()
	mockOtherSpan := newMockSpan()

	mockTracer.On("Start", mock.Anything, "agent.call").
		Return(context.Background(), mockAgentSpan)
	mockTracer.On("Start", mock.Anything, mock.Anything).
		Return(context.Background(), mockOtherSpan)

	mockAgentProvider := new(MockAgentProvider)
	mockAgentProvider.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&workflow.AgentResult{
			Provider:  "claude",
			Output:    "response",
			Tokens:    100,
			StartedAt: time.Now(),
		}, nil)

	mockRegistry := new(MockAgentRegistry)
	mockRegistry.On("Get", "claude").Return(mockAgentProvider, nil)

	svc, _ := NewTestHarness(t).
		WithWorkflow("test", wf).
		Build()
	svc.SetTracer(mockTracer)
	svc.SetAgentRegistry(mockRegistry)

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	mockAgentSpan.AssertCalled(t, "SetAttribute", "provider", "claude")
	mockAgentSpan.AssertCalled(t, "SetAttribute", "tokens_used", 100)
}

// MockAgentProvider is a test double implementing ports.AgentProvider.
type MockAgentProvider struct {
	mock.Mock
}

func (m *MockAgentProvider) Execute(ctx context.Context, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	args := m.Called(ctx, prompt, opts, stdout, stderr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.AgentResult), args.Error(1)
}

func (m *MockAgentProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	args := m.Called(ctx, state, prompt, opts, stdout, stderr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workflow.ConversationResult), args.Error(1)
}

func (m *MockAgentProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAgentProvider) Validate() error {
	args := m.Called()
	return args.Error(0)
}

// MockAgentRegistry is a test double implementing ports.AgentRegistry.
type MockAgentRegistry struct {
	mock.Mock
}

func (m *MockAgentRegistry) Register(provider ports.AgentProvider) error {
	args := m.Called(provider)
	return args.Error(0)
}

func (m *MockAgentRegistry) Get(name string) (ports.AgentProvider, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(ports.AgentProvider), args.Error(1)
}

func (m *MockAgentRegistry) List() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockAgentRegistry) Has(name string) bool {
	args := m.Called(name)
	return args.Bool(0)
}
