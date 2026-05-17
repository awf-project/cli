package application_test

import (
	"context"
	"io"
	"testing"

	"github.com/awf-project/cli/internal/application"
	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testResolverWithValues struct {
	values map[string]string
}

func (r *testResolverWithValues) Resolve(template string, _ *interpolation.Context) (string, error) {
	if v, ok := r.values[template]; ok {
		return v, nil
	}
	return template, nil
}

func TestExecutionService_AgentStep_SingleMode_RoleAndSystemPrompt(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["role-test"] = &workflow.Workflow{
		Name:    "role-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "What is this?",
					Role:         "expert",
					SystemPrompt: "Be concise",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := mocks.NewMockAgentRegistry()
	provider := mocks.NewMockAgentProvider("claude")
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		systemPrompt, ok := opts["system_prompt"]
		require.True(t, ok, "system_prompt should be set in options")

		composed, ok := systemPrompt.(string)
		require.True(t, ok, "system_prompt should be a string")

		assert.Contains(t, composed, "expert role content", "should contain role content")
		assert.Contains(t, composed, "Be concise", "should contain inline system prompt")

		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "This is an expert analysis",
		}, nil
	})
	_ = registry.Register(provider)

	roleRepo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(ctx context.Context, name string) (*workflow.AgentRole, error) {
			if name == "expert" {
				return &workflow.AgentRole{
					Name:    "expert",
					Content: "expert role content",
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetAgentRegistry(registry)
	execSvc.SetAgentRoleRepository(roleRepo)

	ctx, err := execSvc.Run(context.Background(), "role-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_AgentStep_SingleMode_RoleOnly(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["role-test"] = &workflow.Workflow{
		Name:    "role-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "What is this?",
					Role:     "assistant",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := mocks.NewMockAgentRegistry()
	provider := mocks.NewMockAgentProvider("claude")
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		systemPrompt, ok := opts["system_prompt"]
		require.True(t, ok, "system_prompt should be set when role is provided")

		composed, ok := systemPrompt.(string)
		require.True(t, ok, "system_prompt should be a string")

		assert.Equal(t, "assistant role content", composed, "should only contain role content")

		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "Hello",
		}, nil
	})
	_ = registry.Register(provider)

	roleRepo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(ctx context.Context, name string) (*workflow.AgentRole, error) {
			if name == "assistant" {
				return &workflow.AgentRole{
					Name:    "assistant",
					Content: "assistant role content",
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetAgentRegistry(registry)
	execSvc.SetAgentRoleRepository(roleRepo)

	ctx, err := execSvc.Run(context.Background(), "role-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_AgentStep_SingleMode_NoRole_BackwardCompat(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["role-test"] = &workflow.Workflow{
		Name:    "role-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "What is this?",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := mocks.NewMockAgentRegistry()
	provider := mocks.NewMockAgentProvider("claude")
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		_, ok := opts["system_prompt"]
		assert.False(t, ok, "system_prompt should not be set when both role and SystemPrompt are empty")

		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "Hello",
		}, nil
	})
	_ = registry.Register(provider)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "role-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_AgentStep_SingleMode_SystemPromptOnly(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["role-test"] = &workflow.Workflow{
		Name:    "role-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "What is this?",
					SystemPrompt: "Be helpful",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := mocks.NewMockAgentRegistry()
	provider := mocks.NewMockAgentProvider("claude")
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		systemPrompt, ok := opts["system_prompt"]
		require.True(t, ok, "system_prompt should be set when SystemPrompt is provided")

		composed, ok := systemPrompt.(string)
		require.True(t, ok, "system_prompt should be a string")

		assert.Equal(t, "Be helpful", composed, "should equal inline SystemPrompt when no role")

		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "Hello",
		}, nil
	})
	_ = registry.Register(provider)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "role-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_AgentStep_SingleMode_InterpolatedRole(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["role-test"] = &workflow.Workflow{
		Name:    "role-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "What is this?",
					Role:     "{{inputs.persona}}",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := mocks.NewMockAgentRegistry()
	provider := mocks.NewMockAgentProvider("claude")
	provider.SetExecuteFunc(func(ctx context.Context, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		systemPrompt, ok := opts["system_prompt"]
		require.True(t, ok, "system_prompt should be set")

		composed, ok := systemPrompt.(string)
		require.True(t, ok, "system_prompt should be a string")

		assert.Equal(t, "designer role content", composed)

		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "Hello",
		}, nil
	})
	_ = registry.Register(provider)

	roleRepo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(ctx context.Context, name string) (*workflow.AgentRole, error) {
			if name == "designer" {
				return &workflow.AgentRole{
					Name:    "designer",
					Content: "designer role content",
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
		values: map[string]string{
			"{{inputs.persona}}": "designer",
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		resolver,
		nil,
	)
	execSvc.SetAgentRegistry(registry)
	execSvc.SetAgentRoleRepository(roleRepo)

	ctx, err := execSvc.Run(context.Background(), "role-test", map[string]any{
		"persona": "designer",
	})

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_AgentStep_ResumableMode_WithRole(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["role-test"] = &workflow.Workflow{
		Name:    "role-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "What is this?",
					Role:         "expert",
					SystemPrompt: "Be concise",
					Conversation: &workflow.ConversationConfig{},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := mocks.NewMockAgentRegistry()
	provider := mocks.NewMockAgentProvider("claude")
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, opts map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		systemPrompt, ok := opts["system_prompt"]
		require.True(t, ok, "system_prompt should be set in resumable mode with role")

		composed, ok := systemPrompt.(string)
		require.True(t, ok, "system_prompt should be a string")

		assert.Contains(t, composed, "expert role content", "should contain role content")
		assert.Contains(t, composed, "Be concise", "should contain inline system prompt")

		return &workflow.ConversationResult{
			Provider: "claude",
			State:    state,
			Output:   "Expert analysis",
		}, nil
	})
	_ = registry.Register(provider)

	roleRepo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(ctx context.Context, name string) (*workflow.AgentRole, error) {
			if name == "expert" {
				return &workflow.AgentRole{
					Name:    "expert",
					Content: "expert role content",
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetAgentRegistry(registry)
	execSvc.SetAgentRoleRepository(roleRepo)

	ctx, err := execSvc.Run(context.Background(), "role-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_AgentStep_SingleMode_MissingRepository(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["role-test"] = &workflow.Workflow{
		Name:    "role-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "What is this?",
					Role:     "expert",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := mocks.NewMockAgentRegistry()
	_ = registry.Register(mocks.NewMockAgentProvider("claude"))

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "role-test", nil)

	require.Error(t, err)
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}
