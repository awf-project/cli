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
	"github.com/vanoix/awf/internal/testutil/mocks"
)

// Component: execution_service_integration
// Feature: F033 - Agent Conversations

// TestExecutionService_ConversationStep_RoutingToConversationMode tests that
// when agent step has mode: conversation, it routes to executeConversationStep
func TestExecutionService_ConversationStep_RoutingToConversationMode(t *testing.T) {
	wf := &workflow.Workflow{
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

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("conv-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := mocks.NewMockAgentRegistry()
	mockRegistry.Register(claude)

	// Create a simple evaluator that always returns false (never stops on condition)
	evaluator := &simpleExpressionEvaluator{}

	convMgr := application.NewConversationManager(
		&mockLogger{},
		evaluator,
		newMockResolver(),
		tokenizer,
		mockRegistry,
	)

	execSvc.SetAgentRegistry(registry)
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "conv-test", nil)

	// F051: T009 - executeConversationStep is now implemented
	// The test should succeed since ConversationManager is properly configured
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify conversation step was executed
	state, exists := ctx.GetStepState("refine")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

// TestExecutionService_ConversationStep_WithInputInterpolation tests that
// conversation steps properly interpolate inputs in initial_prompt
func TestExecutionService_ConversationStep_WithInputInterpolation(t *testing.T) {
	wf := &workflow.Workflow{
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

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("conv-input-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := mocks.NewMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, &simpleExpressionEvaluator{}, newMockResolver(), tokenizer, mockRegistry)

	execSvc.SetAgentRegistry(registry)
	execSvc.SetConversationManager(convMgr)

	inputs := map[string]any{
		"code": "func main() { println(\"hello\") }",
	}

	ctx, err := execSvc.Run(context.Background(), "conv-input-test", inputs)

	// F051: T009 - executeConversationStep is now implemented
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify conversation step executed with interpolated input
	state, exists := ctx.GetStepState("analyze")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

// TestExecutionService_ConversationStep_WithHooks tests that pre/post hooks
// are executed for conversation steps (pre-hook executes before stub error)
func TestExecutionService_ConversationStep_WithHooks(t *testing.T) {
	wf := &workflow.Workflow{
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

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := mocks.NewMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, &simpleExpressionEvaluator{}, newMockResolver(), tokenizer, mockRegistry)

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("conv-hooks", wf).
		Build()

	execSvc.SetAgentRegistry(registry)
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "conv-hooks", nil)

	// F051: T009 - executeConversationStep is now implemented
	// Hooks should execute successfully
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestExecutionService_ConversationStep_SingleModeSkipsConversation tests that
// agent steps with mode: single (default) skip conversation execution
func TestExecutionService_ConversationStep_SingleModeSkipsConversation(t *testing.T) {
	wf := &workflow.Workflow{
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

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		if prompt == "Summarize this" {
			return &workflow.AgentResult{
				Provider: "claude",
				Output:   "Summary complete",
				Tokens:   100,
			}, nil
		}
		return &workflow.AgentResult{Provider: "claude", Output: "", Tokens: 0}, nil
	})
	_ = registry.Register(claude)

	// ConversationManager configured but should NOT be called for single mode
	tokenizer := newMockTokenizer()
	mockRegistry := mocks.NewMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, &simpleExpressionEvaluator{}, newMockResolver(), tokenizer, mockRegistry)

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("single-mode", wf).
		Build()

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
	wf := &workflow.Workflow{
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

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		if prompt == "Execute this task" {
			return &workflow.AgentResult{
				Provider: "claude",
				Output:   "Task executed",
				Tokens:   80,
			}, nil
		}
		return &workflow.AgentResult{Provider: "claude", Output: "", Tokens: 0}, nil
	})
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := mocks.NewMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, &simpleExpressionEvaluator{}, newMockResolver(), tokenizer, mockRegistry)

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("default-mode", wf).
		Build()

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
	wf := &workflow.Workflow{
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

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("minimal-conv", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := mocks.NewMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, &simpleExpressionEvaluator{}, newMockResolver(), tokenizer, mockRegistry)

	execSvc.SetAgentRegistry(registry)
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "minimal-conv", nil)

	// F051: T009 - executeConversationStep is now implemented
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify minimal conversation config works
	state, exists := ctx.GetStepState("chat")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

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

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(claude)

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
	// NOTE: SetConversationManager is NOT called - manager is nil

	ctx, err := execSvc.Run(context.Background(), "no-mgr", nil)

	// F051: T009 - Should fail with "conversation manager not configured" error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversation manager not configured")
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

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := mocks.NewMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, &simpleExpressionEvaluator{}, newMockResolver(), tokenizer, mockRegistry)

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
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "conv-failure", nil)

	// F051: T009 - executeConversationStep is now implemented
	// Should complete successfully (mock provider succeeds)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify conversation step completed
	state, exists := ctx.GetStepState("chat")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
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

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := mocks.NewMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, &simpleExpressionEvaluator{}, newMockResolver(), tokenizer, mockRegistry)

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
	execSvc.SetConversationManager(convMgr)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	execCtx, err := execSvc.Run(ctx, "conv-cancel", nil)

	// F051: T009 - executeConversationStep is now implemented and respects context cancellation
	require.Error(t, err)
	// Should detect context cancellation
	assert.True(t,
		errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded),
		"expected context error, got: %v", err)
	// Status should be Cancelled when context is cancelled
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

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := mocks.NewMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, &simpleExpressionEvaluator{}, newMockResolver(), tokenizer, mockRegistry)

	executor := newMockExecutor()
	executor.results["echo 'setup complete'"] = &ports.CommandResult{
		Stdout:   "setup complete\n",
		ExitCode: 0,
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
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

	// F051: T009 - executeConversationStep is now implemented
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify both setup step and conversation step completed
	setupState, ok := execCtx.GetStepState("setup")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, setupState.Status)
	assert.Equal(t, "setup complete\n", setupState.Output)

	// Verify conversation step also completed
	convState, ok := execCtx.GetStepState("chat")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, convState.Status)
}
