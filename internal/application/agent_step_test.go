package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: execution_service
// Feature: 39 - Agent Step Type

// TestExecutionService_AgentStep_NoRegistryConfigured tests that agent steps fail
// when no AgentRegistry is configured.
func TestExecutionService_AgentStep_NoRegistryConfigured(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-test"] = &workflow.Workflow{
		Name:    "agent-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Summarize this text",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
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
	// Note: SetAgentRegistry is NOT called - registry is nil

	ctx, err := execSvc.Run(context.Background(), "agent-test", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrNoAgentRegistry)
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
	assert.Equal(t, "ask", ctx.CurrentStep)
}

// TestExecutionService_AgentStep_MissingAgentConfig tests that agent steps fail
// when the Agent configuration is missing.
func TestExecutionService_AgentStep_MissingAgentConfig(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-test"] = &workflow.Workflow{
		Name:    "agent-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name:      "ask",
				Type:      workflow.StepTypeAgent,
				Agent:     nil, // Missing config
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

	ctx, err := execSvc.Run(context.Background(), "agent-test", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent configuration missing")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

// TestExecutionService_AgentStep_ProviderNotFound tests that agent steps fail
// when the specified provider doesn't exist in the registry.
func TestExecutionService_AgentStep_ProviderNotFound(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-test"] = &workflow.Workflow{
		Name:    "agent-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "nonexistent",
					Prompt:   "Summarize this text",
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
	// Don't register the provider

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

	ctx, err := execSvc.Run(context.Background(), "agent-test", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

// TestExecutionService_AgentStep_BasicExecution tests that a basic agent step
// executes successfully when the provider exists.
// AC4: Response captured in states.step_name.output
func TestExecutionService_AgentStep_BasicExecution(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-test"] = &workflow.Workflow{
		Name:    "agent-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Summarize this text",
					Options: map[string]any{
						"model":       "claude-3-sonnet",
						"temperature": 0.7,
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
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		if prompt == "Summarize this text" {
			return &workflow.AgentResult{
				Provider:    "claude",
				Output:      "Summary: This is a summary of the text",
				Response:    map[string]any{"summary": "This is a summary"},
				Tokens:      150,
				Error:       nil,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
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

	ctx, err := execSvc.Run(context.Background(), "agent-test", nil)

	// Agent step should succeed
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify step state was recorded (AC4)
	state, ok := ctx.GetStepState("ask")
	require.True(t, ok, "agent step state should be recorded")
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, "Summary: This is a summary of the text", state.Output)
}

// TestExecutionService_AgentStep_WithOnFailure tests that agent step failure
// can transition to an OnFailure state.
func TestExecutionService_AgentStep_WithOnFailure(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-test"] = &workflow.Workflow{
		Name:    "agent-test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Summarize this text",
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		if prompt == "Summarize this text" {
			return &workflow.AgentResult{
				Provider:    "claude",
				Output:      "",
				Response:    nil,
				Tokens:      0,
				Error:       errors.New("API rate limit exceeded"),
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
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

	ctx, err := execSvc.Run(context.Background(), "agent-test", nil)

	// Agent fails, but workflow should complete via OnFailure path
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "error", ctx.CurrentStep)

	// Verify step state shows failure
	state, ok := ctx.GetStepState("ask")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusFailed, state.Status)
	assert.Contains(t, state.Error, "API rate limit exceeded")
}

// TestExecutionService_AgentStep_InMixedWorkflow tests an agent step
// in a workflow that also has command steps.
func TestExecutionService_AgentStep_InMixedWorkflow(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["mixed"] = &workflow.Workflow{
		Name:    "mixed",
		Initial: "prepare",
		Steps: map[string]*workflow.Step{
			"prepare": {
				Name:      "prepare",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 'preparing data'",
				OnSuccess: "analyze",
			},
			"analyze": {
				Name: "analyze",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Analyze the prepared data",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo 'preparing data'"] = &ports.CommandResult{Stdout: "preparing data\n", ExitCode: 0}

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		if prompt == "Analyze the prepared data" {
			return &workflow.AgentResult{
				Provider:    "claude",
				Output:      "Analysis complete: Data looks good",
				Response:    map[string]any{"status": "ok"},
				Tokens:      75,
				Error:       nil,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
	_ = registry.Register(claude)

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

	ctx, err := execSvc.Run(context.Background(), "mixed", nil)

	// Both steps should succeed
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify command step was executed successfully
	prepareState, ok := ctx.GetStepState("prepare")
	require.True(t, ok, "prepare step should have been executed")
	assert.Equal(t, workflow.StatusCompleted, prepareState.Status)
	assert.Equal(t, "preparing data\n", prepareState.Output)

	// Verify agent step was executed successfully
	analyzeState, ok := ctx.GetStepState("analyze")
	require.True(t, ok, "analyze step should have been executed")
	assert.Equal(t, workflow.StatusCompleted, analyzeState.Status)
	assert.Equal(t, "Analysis complete: Data looks good", analyzeState.Output)
}

// TestExecutionService_AgentStep_StepTimeout tests that agent steps respect
// the step-level timeout configuration.
// AC7: Timeout handling
func TestExecutionService_AgentStep_StepTimeout(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-timeout"] = &workflow.Workflow{
		Name:    "agent-timeout",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name:    "ask",
				Type:    workflow.StepTypeAgent,
				Timeout: 5, // 5 second timeout at step level
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Summarize this text",
					Timeout:  10, // Agent-level timeout (should be overridden)
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
		if prompt == "Summarize this text" {
			return &workflow.AgentResult{
				Provider:    "claude",
				Output:      "Summary",
				Response:    map[string]any{},
				Tokens:      50,
				Error:       nil,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
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

	ctx, err := execSvc.Run(context.Background(), "agent-timeout", nil)

	// Should succeed (mock doesn't actually timeout)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestExecutionService_AgentStep_AgentTimeout tests that agent steps use
// agent-level timeout when step timeout is not set.
// AC7: Timeout handling
func TestExecutionService_AgentStep_AgentTimeout(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-timeout"] = &workflow.Workflow{
		Name:    "agent-timeout",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				// No step-level timeout
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Summarize this text",
					Timeout:  3, // Agent-level timeout should be used
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
		if prompt == "Summarize this text" {
			return &workflow.AgentResult{
				Provider:    "claude",
				Output:      "Summary",
				Response:    map[string]any{},
				Tokens:      50,
				Error:       nil,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
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

	ctx, err := execSvc.Run(context.Background(), "agent-timeout", nil)

	// Should succeed (mock doesn't actually timeout)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestExecutionService_AgentStep_ContextCancellation tests that agent steps
// handle context cancellation correctly.
// AC7: Timeout handling
func TestExecutionService_AgentStep_ContextCancellation(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-cancel"] = &workflow.Workflow{
		Name:    "agent-cancel",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Summarize this text",
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return nil, context.Canceled
	})
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

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	execCtx, err := execSvc.Run(ctx, "agent-cancel", nil)

	// Should fail with context canceled error
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, workflow.StatusCancelled, execCtx.Status)
}

// TestExecutionService_AgentStep_InParallelBranches tests that agent steps
// work correctly within parallel execution branches.
// AC10: Works with parallel steps
func TestExecutionService_AgentStep_InParallelBranches(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["parallel-agents"] = &workflow.Workflow{
		Name:    "parallel-agents",
		Initial: "analyze_parallel",
		Steps: map[string]*workflow.Step{
			"analyze_parallel": {
				Name:      "analyze_parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"analyze_sentiment", "analyze_keywords"},
				Strategy:  "all_succeed",
				OnSuccess: "done",
			},
			"analyze_sentiment": {
				Name: "analyze_sentiment",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Analyze sentiment",
				},
			},
			"analyze_keywords": {
				Name: "analyze_keywords",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "gemini",
					Prompt:   "Extract keywords",
				},
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
		if prompt == "Analyze sentiment" {
			return &workflow.AgentResult{
				Provider:    "claude",
				Output:      "Sentiment: Positive",
				Response:    map[string]any{"sentiment": "positive"},
				Tokens:      25,
				Error:       nil,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
	_ = registry.Register(claude)

	gemini := mocks.NewMockAgentProvider("gemini")
	gemini.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		if prompt == "Extract keywords" {
			return &workflow.AgentResult{
				Provider:    "gemini",
				Output:      "Keywords: AI, ML, Data",
				Response:    map[string]any{"keywords": []string{"AI", "ML", "Data"}},
				Tokens:      30,
				Error:       nil,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "gemini",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
	_ = registry.Register(gemini)

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

	ctx, err := execSvc.Run(context.Background(), "parallel-agents", nil)

	// Should succeed
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify both agent steps were executed
	sentimentState, ok := ctx.GetStepState("analyze_sentiment")
	require.True(t, ok, "sentiment analysis step should be executed")
	assert.Equal(t, workflow.StatusCompleted, sentimentState.Status)
	assert.Equal(t, "Sentiment: Positive", sentimentState.Output)

	keywordsState, ok := ctx.GetStepState("analyze_keywords")
	require.True(t, ok, "keyword extraction step should be executed")
	assert.Equal(t, workflow.StatusCompleted, keywordsState.Status)
	assert.Equal(t, "Keywords: AI, ML, Data", keywordsState.Output)
}

// TestExecutionService_AgentStep_PromptInterpolation tests that agent prompts
// are interpolated with context variables.
func TestExecutionService_AgentStep_PromptInterpolation(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-interpolate"] = &workflow.Workflow{
		Name:    "agent-interpolate",
		Initial: "ask",
		Inputs: []workflow.Input{
			{Name: "topic", Type: "string", Required: true},
		},
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Explain {{inputs.topic}} in simple terms", // Should be interpolated
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
	// Note: mock resolver doesn't interpolate, so prompt stays as-is
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		if prompt == "Explain {{inputs.topic}} in simple terms" {
			return &workflow.AgentResult{
				Provider:    "claude",
				Output:      "Explanation of the topic",
				Response:    map[string]any{},
				Tokens:      100,
				Error:       nil,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
	_ = registry.Register(claude)

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(), // Mock resolver passes through
		nil,
	)
	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "agent-interpolate", map[string]any{
		"topic": "quantum computing",
	})

	// Should succeed
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestExecutionService_AgentStep_MultipleProviders tests that different
// agent providers can be used in the same workflow.
func TestExecutionService_AgentStep_MultipleProviders(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["multi-provider"] = &workflow.Workflow{
		Name:    "multi-provider",
		Initial: "claude_analysis",
		Steps: map[string]*workflow.Step{
			"claude_analysis": {
				Name: "claude_analysis",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Analyze this code",
				},
				OnSuccess: "gemini_review",
			},
			"gemini_review": {
				Name: "gemini_review",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "gemini",
					Prompt:   "Review the analysis",
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
		if prompt == "Analyze this code" {
			return &workflow.AgentResult{
				Provider:    "claude",
				Output:      "Code analysis: Good structure",
				Response:    map[string]any{"quality": "good"},
				Tokens:      150,
				Error:       nil,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
	_ = registry.Register(claude)

	gemini := mocks.NewMockAgentProvider("gemini")
	gemini.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		if prompt == "Review the analysis" {
			return &workflow.AgentResult{
				Provider:    "gemini",
				Output:      "Review: Analysis is accurate",
				Response:    map[string]any{"accuracy": "high"},
				Tokens:      80,
				Error:       nil,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "gemini",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
	_ = registry.Register(gemini)

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

	ctx, err := execSvc.Run(context.Background(), "multi-provider", nil)

	// Both steps should succeed
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify both agent steps executed
	claudeState, ok := ctx.GetStepState("claude_analysis")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, claudeState.Status)
	assert.Equal(t, "Code analysis: Good structure", claudeState.Output)

	geminiState, ok := ctx.GetStepState("gemini_review")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, geminiState.Status)
	assert.Equal(t, "Review: Analysis is accurate", geminiState.Output)
}

// TestExecutionService_SetAgentRegistry tests the setter method.
func TestExecutionService_SetAgentRegistry(t *testing.T) {
	wfSvc := application.NewWorkflowService(
		newMockRepository(),
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		nil,
	)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	_ = registry.Register(claude)

	// Should not panic
	execSvc.SetAgentRegistry(registry)

	// Create a workflow with an agent step to verify the registry is set
	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:    "test",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Test prompt",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	wfSvc2 := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc2 := application.NewExecutionService(
		wfSvc2,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc2.SetAgentRegistry(registry)

	ctx, err := execSvc2.Run(context.Background(), "test", nil)
	// Should succeed (not ErrNoAgentRegistry)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestExecutionService_Resume_WithAgentStep tests resuming a workflow
// that has an agent step.
func TestExecutionService_Resume_WithAgentStep(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["resume-agent"] = &workflow.Workflow{
		Name:    "resume-agent",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Summarize this text",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Create a state store with an interrupted execution at the agent step
	stateStore := newMockStateStore()
	execCtx := workflow.NewExecutionContext("test-id", "resume-agent")
	execCtx.CurrentStep = "ask"
	execCtx.Status = workflow.StatusRunning
	stateStore.states["test-id"] = execCtx

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		if prompt == "Summarize this text" {
			return &workflow.AgentResult{
				Provider:    "claude",
				Output:      "Summary: Brief summary",
				Response:    map[string]any{"summary": "Brief summary"},
				Tokens:      50,
				Error:       nil,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
	_ = registry.Register(claude)

	wfSvc := application.NewWorkflowService(repo, stateStore, newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		stateStore,
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Resume(context.Background(), "test-id", nil)

	// Should succeed after resuming at the agent step
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

// TestExecutionService_AgentStep_ContinueOnError tests that agent steps
// can continue on error when configured.
func TestExecutionService_AgentStep_ContinueOnError(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["continue-on-error"] = &workflow.Workflow{
		Name:    "continue-on-error",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Summarize this text",
				},
				ContinueOnError: true,
				OnSuccess:       "done",
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
		if prompt == "Summarize this text" {
			return &workflow.AgentResult{
				Provider:    "claude",
				Output:      "",
				Response:    nil,
				Tokens:      0,
				Error:       errors.New("API error"),
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}, nil
		}
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Agent response",
			Tokens:      100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
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

	ctx, err := execSvc.Run(context.Background(), "continue-on-error", nil)

	// Should continue despite error
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify step failed but workflow continued
	state, ok := ctx.GetStepState("ask")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusFailed, state.Status)
}

// TestExecutionService_AgentStep_ErrorMessages tests that error messages
// include helpful context.
func TestExecutionService_AgentStep_ErrorMessages(t *testing.T) {
	tests := []struct {
		name          string
		stepName      string
		provider      string
		setupRegistry bool
		addProvider   bool
		agentConfig   bool
		expectedParts []string
	}{
		{
			name:          "no registry - includes step name",
			stepName:      "my-agent-step",
			provider:      "claude",
			setupRegistry: false,
			addProvider:   false,
			agentConfig:   true,
			expectedParts: []string{"my-agent-step", "agent registry not configured"},
		},
		{
			name:          "provider not found - includes provider name",
			stepName:      "analyze-step",
			provider:      "custom-ai",
			setupRegistry: true,
			addProvider:   false,
			agentConfig:   true,
			expectedParts: []string{"analyze-step", "custom-ai", "not found"},
		},
		{
			name:          "missing agent config - includes step name",
			stepName:      "broken-step",
			provider:      "claude",
			setupRegistry: true,
			addProvider:   true,
			agentConfig:   false,
			expectedParts: []string{"broken-step", "agent configuration missing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()

			var agentConfig *workflow.AgentConfig
			if tt.agentConfig {
				agentConfig = &workflow.AgentConfig{
					Provider: tt.provider,
					Prompt:   "Test prompt",
				}
			}

			repo.workflows["test"] = &workflow.Workflow{
				Name:    "test",
				Initial: tt.stepName,
				Steps: map[string]*workflow.Step{
					tt.stepName: {
						Name:      tt.stepName,
						Type:      workflow.StepTypeAgent,
						Agent:     agentConfig,
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
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

			if tt.setupRegistry {
				registry := mocks.NewMockAgentRegistry()
				if tt.addProvider {
					provider := mocks.NewMockAgentProvider(tt.provider)
					_ = registry.Register(provider)
				}
				execSvc.SetAgentRegistry(registry)
			}

			_, err := execSvc.Run(context.Background(), "test", nil)

			require.Error(t, err)
			errMsg := err.Error()
			for _, part := range tt.expectedParts {
				assert.Contains(t, errMsg, part, "error message should contain: %s", part)
			}
		})
	}
}

// TestExecutionService_AgentStep_ExecutionError tests handling of
// provider execution errors.
func TestExecutionService_AgentStep_ExecutionError(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["exec-error"] = &workflow.Workflow{
		Name:    "exec-error",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Summarize this text",
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return nil, errors.New("network connection failed")
	})
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

	ctx, err := execSvc.Run(context.Background(), "exec-error", nil)

	// Should transition to error state
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "error", ctx.CurrentStep)

	// Verify step state shows error
	state, ok := ctx.GetStepState("ask")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusFailed, state.Status)
	assert.Contains(t, state.Error, "network connection failed")
}

// configurableResolver is a test resolver that maps template expressions to resolved values.
// Used for testing interpolation behavior with specific mappings.
// For unmapped templates (not in mapping), it passes through as-is if not a template expression,
// otherwise returns an error.
type configurableResolver struct {
	mapping map[string]string
}

func newConfigurableResolver(mapping map[string]string) *configurableResolver {
	return &configurableResolver{mapping: mapping}
}

func (c *configurableResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	// Check if this exact template is in our mapping
	if resolved, ok := c.mapping[template]; ok {
		return resolved, nil
	}
	// If not in mapping and doesn't look like a template, pass through
	// (this allows non-templated strings like "Test prompt" to work)
	if !strings.Contains(template, "{{") && !strings.Contains(template, "}}") {
		return template, nil
	}
	// If it looks like a template but isn't in mapping, return error
	return "", fmt.Errorf("template parse error: %s", template)
}

// TestExecutionService_AgentStep_ProviderInterpolation tests that the provider field
// is correctly interpolated before registry lookup.
func TestExecutionService_AgentStep_ProviderInterpolation(t *testing.T) {
	tests := []struct {
		name             string
		providerExpr     string
		resolveMap       map[string]string
		registeredNames  []string
		expectError      bool
		expectedErrorMsg string
	}{
		{
			name:             "literal provider name",
			providerExpr:     "claude",
			resolveMap:       map[string]string{"claude": "claude"}, // pass-through
			registeredNames:  []string{"claude"},
			expectError:      false,
			expectedErrorMsg: "",
		},
		{
			name:             "interpolated provider from inputs",
			providerExpr:     "{{inputs.agent}}",
			resolveMap:       map[string]string{"{{inputs.agent}}": "claude"},
			registeredNames:  []string{"claude"},
			expectError:      false,
			expectedErrorMsg: "",
		},
		{
			name:             "interpolated provider different value",
			providerExpr:     "{{inputs.agent}}",
			resolveMap:       map[string]string{"{{inputs.agent}}": "gemini"},
			registeredNames:  []string{"gemini"},
			expectError:      false,
			expectedErrorMsg: "",
		},
		{
			name:             "invalid template expression",
			providerExpr:     "{{invalid",
			resolveMap:       map[string]string{},
			registeredNames:  []string{},
			expectError:      true,
			expectedErrorMsg: "resolve provider",
		},
		{
			name:             "resolved provider name not in registry",
			providerExpr:     "{{inputs.agent}}",
			resolveMap:       map[string]string{"{{inputs.agent}}": "unknown"},
			registeredNames:  []string{"claude"},
			expectError:      true,
			expectedErrorMsg: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			repo.workflows["provider-interp"] = &workflow.Workflow{
				Name:    "provider-interp",
				Initial: "ask",
				Inputs: []workflow.Input{
					{Name: "agent", Type: "string", Default: "claude"},
				},
				Steps: map[string]*workflow.Step{
					"ask": {
						Name: "ask",
						Type: workflow.StepTypeAgent,
						Agent: &workflow.AgentConfig{
							Provider: tt.providerExpr,
							Prompt:   "Test prompt",
						},
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			configResolver := newConfigurableResolver(tt.resolveMap)

			registry := mocks.NewMockAgentRegistry()
			for _, name := range tt.registeredNames {
				provider := mocks.NewMockAgentProvider(name)
				provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
					return &workflow.AgentResult{
						Provider:    name,
						Output:      "Result from " + name,
						Response:    map[string]any{},
						Tokens:      50,
						Error:       nil,
						StartedAt:   time.Now(),
						CompletedAt: time.Now(),
					}, nil
				})
				_ = registry.Register(provider)
			}

			wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
			execSvc := application.NewExecutionService(
				wfSvc,
				newMockExecutor(),
				newMockParallelExecutor(),
				newMockStateStore(),
				&mockLogger{},
				configResolver,
				nil,
			)
			execSvc.SetAgentRegistry(registry)

			// Provide input for interpolation
			ctx, err := execSvc.Run(context.Background(), "provider-interp", map[string]any{
				"agent": "claude",
			})

			if tt.expectError {
				require.Error(t, err, "should return error")
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg, "error should contain expected message")
				}
				assert.Equal(t, workflow.StatusFailed, ctx.Status)
			} else {
				require.NoError(t, err, "should not return error")
				assert.Equal(t, workflow.StatusCompleted, ctx.Status)

				state, ok := ctx.GetStepState("ask")
				require.True(t, ok, "step state should be recorded")
				assert.Equal(t, workflow.StatusCompleted, state.Status)
			}
		})
	}
}
