package application

import (
	"context"
	"io"
	"testing"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationManager_ExecuteConversation_ComposedSystemPrompt(t *testing.T) {
	logger := mocks.NewMockLogger()
	registry := mocks.NewMockAgentRegistry()

	capturedOptions := make(map[string]any)

	provider := mocks.NewMockAgentProvider("claude")
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		capturedOptions = opts

		return &workflow.ConversationResult{
			State:    state,
			Provider: "claude",
		}, nil
	})
	_ = registry.Register(provider)

	roleRepo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(ctx context.Context, name string) (*workflow.AgentRole, error) {
			if name == "mentor" {
				return &workflow.AgentRole{
					Name:    "mentor",
					Content: "You are a mentor",
				}, nil
			}
			return nil, domainerrors.NewUserError(
				domainerrors.ErrorCodeUserInputMissingRole,
				"role not found: "+name,
				map[string]any{"role": name},
				nil,
			)
		},
	}

	resolver := &testResolverWithValues{
		values: map[string]string{},
	}

	manager := NewConversationManager(logger, resolver, registry)
	manager.SetAgentRoleRepository(roleRepo)
	manager.SetUserInputReader(&mocks.MockUserInputReader{})

	step := &workflow.Step{
		Name: "mentor_chat",
		Agent: &workflow.AgentConfig{
			Provider:     "claude",
			Prompt:       "Help me learn",
			Role:         "mentor",
			SystemPrompt: "Be patient",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(
		context.Background(),
		step,
		config,
		execCtx,
		buildContext,
		"",
		io.Discard,
		io.Discard,
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	systemPrompt, ok := capturedOptions["system_prompt"]
	require.True(t, ok, "system_prompt should be set in cloned options")

	composed, ok := systemPrompt.(string)
	require.True(t, ok, "system_prompt should be a string")

	assert.Contains(t, composed, "You are a mentor", "should contain role content")
	assert.Contains(t, composed, "Be patient", "should contain inline system prompt")
}

func TestConversationManager_ExecuteConversation_OriginalOptionsNotMutated(t *testing.T) {
	logger := mocks.NewMockLogger()
	registry := mocks.NewMockAgentRegistry()

	capturedOptions := make(map[string]any)

	provider := mocks.NewMockAgentProvider("claude")
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		capturedOptions = opts

		return &workflow.ConversationResult{
			State:    state,
			Provider: "claude",
		}, nil
	})
	_ = registry.Register(provider)

	roleRepo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(ctx context.Context, name string) (*workflow.AgentRole, error) {
			if name == "expert" {
				return &workflow.AgentRole{
					Name:    "expert",
					Content: "Expert role",
				}, nil
			}
			return nil, domainerrors.NewUserError(
				domainerrors.ErrorCodeUserInputMissingRole,
				"role not found: "+name,
				map[string]any{"role": name},
				nil,
			)
		},
	}

	resolver := &testResolverWithValues{
		values: map[string]string{},
	}

	manager := NewConversationManager(logger, resolver, registry)
	manager.SetAgentRoleRepository(roleRepo)
	manager.SetUserInputReader(&mocks.MockUserInputReader{})

	originalOptions := map[string]any{
		"temperature": 0.7,
	}

	step := &workflow.Step{
		Name: "expert_chat",
		Agent: &workflow.AgentConfig{
			Provider:     "claude",
			Prompt:       "Analyze this",
			Role:         "expert",
			Options:      originalOptions,
			SystemPrompt: "Be analytical",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	_, err := manager.ExecuteConversation(
		context.Background(),
		step,
		config,
		execCtx,
		buildContext,
		"",
		io.Discard,
		io.Discard,
	)

	require.NoError(t, err)

	assert.NotContains(t, originalOptions, "system_prompt",
		"original options map should not be mutated with system_prompt")
	assert.Equal(t, 0.7, originalOptions["temperature"],
		"original options should retain their values")

	assert.Contains(t, capturedOptions, "system_prompt",
		"cloned options should have system_prompt")
	assert.Equal(t, 0.7, capturedOptions["temperature"],
		"cloned options should contain original values")
}

func TestConversationManager_ExecuteConversation_RoleOnly(t *testing.T) {
	logger := mocks.NewMockLogger()
	registry := mocks.NewMockAgentRegistry()

	capturedOptions := make(map[string]any)

	provider := mocks.NewMockAgentProvider("claude")
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		capturedOptions = opts

		return &workflow.ConversationResult{
			State:    state,
			Provider: "claude",
		}, nil
	})
	_ = registry.Register(provider)

	roleRepo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(ctx context.Context, name string) (*workflow.AgentRole, error) {
			if name == "assistant" {
				return &workflow.AgentRole{
					Name:    "assistant",
					Content: "You are helpful",
				}, nil
			}
			return nil, domainerrors.NewUserError(
				domainerrors.ErrorCodeUserInputMissingRole,
				"role not found: "+name,
				map[string]any{"role": name},
				nil,
			)
		},
	}

	resolver := &testResolverWithValues{
		values: map[string]string{},
	}

	manager := NewConversationManager(logger, resolver, registry)
	manager.SetAgentRoleRepository(roleRepo)
	manager.SetUserInputReader(&mocks.MockUserInputReader{})

	step := &workflow.Step{
		Name: "assistant_chat",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
			Role:     "assistant",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := workflow.NewExecutionContext("test-id", "test-workflow")

	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(
		context.Background(),
		step,
		config,
		execCtx,
		buildContext,
		"",
		io.Discard,
		io.Discard,
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	systemPrompt, ok := capturedOptions["system_prompt"]
	require.True(t, ok, "system_prompt should be set")

	composed, ok := systemPrompt.(string)
	require.True(t, ok)

	assert.Equal(t, "You are helpful", composed,
		"should contain only role content when no inline SystemPrompt")
}
