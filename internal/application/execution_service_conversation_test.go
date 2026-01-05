package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/agents"
)

// Component: execution_service_integration
// Feature: F033 - Agent Conversations

// =============================================================================
// HAPPY PATH TESTS
// =============================================================================

// TestExecutionService_ConversationStep_RoutingToConversationMode tests that
// when agent step has mode: conversation, it routes to executeConversationStep
func TestExecutionService_ConversationStep_RoutingToConversationMode(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["conv-test"] = &workflow.Workflow{
		Name:    "conv-test",
		Initial: "refine",
		Steps: map[string]*workflow.Step{
			"refine": {
				Name: "refine",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "claude",
					Mode:          "conversation",
					SystemPrompt:  "You are a code reviewer",
					InitialPrompt: "Review this code",
					Conversation: &workflow.ConversationConfig{
						MaxTurns:         10,
						MaxContextTokens: 100000,
						Strategy:         workflow.StrategySlidingWindow,
						StopCondition:    "response contains 'APPROVED'",
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := agents.NewAgentRegistry()
	claude := newMockConversationProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := newMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(
		&mockLogger{},
		nil,
		newMockResolver(),
		tokenizer,
		mockRegistry,
	)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
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
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "conv-test", nil)

	// STUB: Should fail with "not implemented" until GREEN phase
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
	assert.Equal(t, "refine", ctx.CurrentStep)
}

// TestExecutionService_ConversationStep_WithInputInterpolation tests that
// conversation steps properly interpolate inputs in initial_prompt
func TestExecutionService_ConversationStep_WithInputInterpolation(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["conv-input-test"] = &workflow.Workflow{
		Name:    "conv-input-test",
		Initial: "analyze",
		Inputs: []workflow.Input{
			{Name: "code", Type: "string", Required: true},
		},
		Steps: map[string]*workflow.Step{
			"analyze": {
				Name: "analyze",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "claude",
					Mode:          "conversation",
					SystemPrompt:  "You are a code analyzer",
					InitialPrompt: "Analyze this code: {{inputs.code}}",
					Conversation: &workflow.ConversationConfig{
						MaxTurns:      5,
						StopCondition: "response contains 'DONE'",
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := agents.NewAgentRegistry()
	claude := newMockConversationProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := newMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, nil, newMockResolver(), tokenizer, mockRegistry)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
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
	execSvc.SetConversationManager(convMgr)

	inputs := map[string]any{
		"code": "func main() { println(\"hello\") }",
	}

	ctx, err := execSvc.Run(context.Background(), "conv-input-test", inputs)

	// STUB: Should fail with "not implemented" until GREEN phase
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

// TestExecutionService_ConversationStep_WithHooks tests that pre/post hooks
// are executed for conversation steps (pre-hook executes before stub error)
func TestExecutionService_ConversationStep_WithHooks(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["conv-hooks"] = &workflow.Workflow{
		Name:    "conv-hooks",
		Initial: "chat",
		Steps: map[string]*workflow.Step{
			"chat": {
				Name: "chat",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "claude",
					Mode:          "conversation",
					InitialPrompt: "Start chat",
					Conversation: &workflow.ConversationConfig{
						MaxTurns: 3,
					},
				},
				Hooks: workflow.StepHooks{
					Pre: workflow.Hook{
						workflow.HookAction{
							Log: "Starting conversation",
						},
					},
					Post: workflow.Hook{
						workflow.HookAction{
							Log: "Conversation complete",
						},
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := agents.NewAgentRegistry()
	claude := newMockConversationProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := newMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, nil, newMockResolver(), tokenizer, mockRegistry)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
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
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "conv-hooks", nil)

	// STUB: Should fail with "not implemented" after pre-hook executes
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

// TestExecutionService_ConversationStep_SingleModeSkipsConversation tests that
// agent steps with mode: single (default) skip conversation execution
func TestExecutionService_ConversationStep_SingleModeSkipsConversation(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["single-mode"] = &workflow.Workflow{
		Name:    "single-mode",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Mode:     "single", // Explicit single mode
					Prompt:   "Summarize this",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := agents.NewAgentRegistry()
	claude := newMockAgentProvider("claude")
	claude.results["Summarize this"] = &workflow.AgentResult{
		Provider: "claude",
		Output:   "Summary complete",
		Tokens:   100,
	}
	_ = registry.Register(claude)

	// ConversationManager configured but should NOT be called for single mode
	tokenizer := newMockTokenizer()
	mockRegistry := newMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, nil, newMockResolver(), tokenizer, mockRegistry)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
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
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "single-mode", nil)

	// Should succeed with single execution (not conversation)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify step used single execution
	state, ok := ctx.GetStepState("ask")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, "Summary complete", state.Output)
}

// TestExecutionService_ConversationStep_EmptyModeDefaultsToSingle tests that
// when mode is empty/unset, it defaults to single execution
func TestExecutionService_ConversationStep_EmptyModeDefaultsToSingle(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["default-mode"] = &workflow.Workflow{
		Name:    "default-mode",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					// Mode not set - should default to "single"
					Prompt: "Execute this task",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := agents.NewAgentRegistry()
	claude := newMockAgentProvider("claude")
	claude.results["Execute this task"] = &workflow.AgentResult{
		Provider: "claude",
		Output:   "Task executed",
		Tokens:   80,
	}
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := newMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, nil, newMockResolver(), tokenizer, mockRegistry)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
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
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "default-mode", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	state, ok := ctx.GetStepState("ask")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, "Task executed", state.Output)
}

// TestExecutionService_ConversationStep_MinimalConversationConfig tests
// conversation execution with minimal (default) config values
func TestExecutionService_ConversationStep_MinimalConversationConfig(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["minimal-conv"] = &workflow.Workflow{
		Name:    "minimal-conv",
		Initial: "chat",
		Steps: map[string]*workflow.Step{
			"chat": {
				Name: "chat",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "claude",
					Mode:          "conversation",
					InitialPrompt: "Start",
					// Minimal conversation config (should use defaults)
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

	registry := agents.NewAgentRegistry()
	claude := newMockConversationProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := newMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, nil, newMockResolver(), tokenizer, mockRegistry)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
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
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "minimal-conv", nil)

	// STUB: Should fail with "not implemented"
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

// TestExecutionService_ConversationStep_NoConversationManagerConfigured tests
// that conversation steps fail gracefully when conversation manager is not set
func TestExecutionService_ConversationStep_NoConversationManagerConfigured(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["no-mgr"] = &workflow.Workflow{
		Name:    "no-mgr",
		Initial: "chat",
		Steps: map[string]*workflow.Step{
			"chat": {
				Name: "chat",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "claude",
					Mode:          "conversation",
					InitialPrompt: "Start conversation",
					Conversation: &workflow.ConversationConfig{
						MaxTurns: 5,
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := agents.NewAgentRegistry()
	claude := newMockConversationProvider("claude")
	_ = registry.Register(claude)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
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
	// NOTE: SetConversationManager is NOT called - manager is nil

	ctx, err := execSvc.Run(context.Background(), "no-mgr", nil)

	// Should fail (stub implementation will still error even if manager is nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
	assert.Equal(t, "chat", ctx.CurrentStep)
}

// TestExecutionService_ConversationStep_WithOnFailureTransition tests that
// OnFailure transitions would work for conversation steps (in GREEN phase)
func TestExecutionService_ConversationStep_WithOnFailureTransition(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["conv-failure"] = &workflow.Workflow{
		Name:    "conv-failure",
		Initial: "chat",
		Steps: map[string]*workflow.Step{
			"chat": {
				Name: "chat",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "claude",
					Mode:          "conversation",
					InitialPrompt: "Start",
					Conversation: &workflow.ConversationConfig{
						MaxTurns: 5,
					},
				},
				OnSuccess: "success",
				OnFailure: "error_handler",
			},
			"success": {
				Name: "success",
				Type: workflow.StepTypeTerminal,
			},
			"error_handler": {
				Name: "error_handler",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := agents.NewAgentRegistry()
	claude := newMockConversationProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := newMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, nil, newMockResolver(), tokenizer, mockRegistry)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
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
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "conv-failure", nil)

	// STUB: Should fail and NOT follow OnFailure (stub returns error directly)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

// TestExecutionService_ConversationStep_ContextCancellation tests that
// conversation execution respects context cancellation
func TestExecutionService_ConversationStep_ContextCancellation(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["conv-cancel"] = &workflow.Workflow{
		Name:    "conv-cancel",
		Initial: "chat",
		Steps: map[string]*workflow.Step{
			"chat": {
				Name: "chat",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "claude",
					Mode:          "conversation",
					InitialPrompt: "Long running conversation",
					Conversation: &workflow.ConversationConfig{
						MaxTurns: 100,
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := agents.NewAgentRegistry()
	claude := newMockConversationProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := newMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, nil, newMockResolver(), tokenizer, mockRegistry)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
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
	execSvc.SetConversationManager(convMgr)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	execCtx, err := execSvc.Run(ctx, "conv-cancel", nil)

	// STUB: Will fail - in GREEN phase this will respect context cancellation
	require.Error(t, err)
	// In RED phase, stub returns "not implemented" before checking context
	// In GREEN phase, context cancellation will be detected first
	assert.True(t,
		errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) ||
			err.Error() == "executeConversationStep: not implemented" ||
			err.Error() == "chat: executeConversationStep: not implemented",
		"expected context error or stub error, got: %v", err)
	// Status can be Failed or Cancelled depending on which error is hit first
	assert.True(t,
		execCtx.Status == workflow.StatusFailed || execCtx.Status == workflow.StatusCancelled,
		"expected Failed or Cancelled status, got: %v", execCtx.Status)
}

// TestExecutionService_ConversationStep_InterpolationContextAccess tests that
// conversation steps can access previous step states via interpolation
func TestExecutionService_ConversationStep_InterpolationContextAccess(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["conv-context"] = &workflow.Workflow{
		Name:    "conv-context",
		Initial: "setup",
		Inputs: []workflow.Input{
			{Name: "initial_data", Type: "string", Required: true},
		},
		Steps: map[string]*workflow.Step{
			"setup": {
				Name:      "setup",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 'setup complete'",
				OnSuccess: "chat",
			},
			"chat": {
				Name: "chat",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "claude",
					Mode:          "conversation",
					InitialPrompt: "Use data: {{inputs.initial_data}} and result: {{states.setup.output}}",
					Conversation: &workflow.ConversationConfig{
						MaxTurns: 3,
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := agents.NewAgentRegistry()
	claude := newMockConversationProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := newMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, nil, newMockResolver(), tokenizer, mockRegistry)

	executor := newMockExecutor()
	executor.results["echo 'setup complete'"] = &ports.CommandResult{
		Stdout:   "setup complete\n",
		ExitCode: 0,
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(
		wfSvc,
		executor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetAgentRegistry(registry)
	execSvc.SetConversationManager(convMgr)

	inputs := map[string]any{
		"initial_data": "test-data",
	}

	execCtx, err := execSvc.Run(context.Background(), "conv-context", inputs)

	// STUB: Should fail but setup step should have completed
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")

	// Verify setup step completed before conversation step failed
	setupState, ok := execCtx.GetStepState("setup")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, setupState.Status)
	assert.Equal(t, "setup complete\n", setupState.Output)
}
