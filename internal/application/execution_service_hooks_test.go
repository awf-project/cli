package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/agents"
)

// =============================================================================
// Agent Hook Integration Tests
// Feature: C008 - Test File Restructuring
// Component: extract_hooks_tests (T008)
// =============================================================================
//
// This file contains all agent hook integration tests for ExecutionService.
// Tests verify pre-hook and post-hook execution with agent steps, including
// successful and failed agent executions.
//
// Extracted from: execution_service_test.go (lines 1932-2170)
// Test count: 3 hook-related tests
// =============================================================================

// Mock types are defined in execution_service_specialized_mocks_test.go
// Mock helper functions are defined in execution_service_helpers_test.go

func TestExecutionService_AgentStep_WithPreHook_Success(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-prehook"] = &workflow.Workflow{
		Name:    "agent-prehook",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Summarize this text",
				},
				Hooks: workflow.StepHooks{
					Pre: workflow.Hook{
						{Log: "Pre-hook: About to execute agent step"},
						{Command: "echo 'pre-hook executed'"},
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

	executor := newMockExecutor()
	executor.results["echo 'pre-hook executed'"] = &ports.CommandResult{
		Stdout:   "pre-hook executed\n",
		ExitCode: 0,
	}

	registry := agents.NewAgentRegistry()
	claude := newMockAgentProvider("claude")
	claude.results["Summarize this text"] = &workflow.AgentResult{
		Provider:    "claude",
		Output:      "Summary: This is the summary",
		Response:    map[string]any{"summary": "This is the summary"},
		Tokens:      75,
		Error:       nil,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}
	_ = registry.Register(claude)

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

	ctx, err := execSvc.Run(context.Background(), "agent-prehook", nil)

	// Should succeed with pre-hook executed before agent step
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify agent step executed successfully
	state, ok := ctx.GetStepState("ask")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, "Summary: This is the summary", state.Output)

	// Verify pre-hook command was executed
	_, wasExecuted := executor.results["echo 'pre-hook executed'"]
	assert.True(t, wasExecuted, "pre-hook command should have been executed")
}

// TestExecutionService_AgentStep_WithPostHook_OnSuccess tests that post-hooks
// execute after successful agent steps.
func TestExecutionService_AgentStep_WithPostHook_OnSuccess(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-posthook"] = &workflow.Workflow{
		Name:    "agent-posthook",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Analyze the data",
				},
				Hooks: workflow.StepHooks{
					Post: workflow.Hook{
						{Log: "Post-hook: Agent step completed successfully"},
						{Command: "echo 'post-hook executed'"},
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

	executor := newMockExecutor()
	executor.results["echo 'post-hook executed'"] = &ports.CommandResult{
		Stdout:   "post-hook executed\n",
		ExitCode: 0,
	}

	registry := agents.NewAgentRegistry()
	claude := newMockAgentProvider("claude")
	claude.results["Analyze the data"] = &workflow.AgentResult{
		Provider:    "claude",
		Output:      "Analysis: Data is valid",
		Response:    map[string]any{"status": "valid"},
		Tokens:      50,
		Error:       nil,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}
	_ = registry.Register(claude)

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

	ctx, err := execSvc.Run(context.Background(), "agent-posthook", nil)

	// Should succeed with post-hook executed after agent step
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify agent step executed successfully
	state, ok := ctx.GetStepState("ask")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, "Analysis: Data is valid", state.Output)

	// Verify post-hook command was executed
	_, wasExecuted := executor.results["echo 'post-hook executed'"]
	assert.True(t, wasExecuted, "post-hook command should have been executed")
}

// TestExecutionService_AgentStep_WithPostHook_OnFailure tests that post-hooks
// execute even when agent steps fail.
func TestExecutionService_AgentStep_WithPostHook_OnFailure(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent-posthook-fail"] = &workflow.Workflow{
		Name:    "agent-posthook-fail",
		Initial: "ask",
		Steps: map[string]*workflow.Step{
			"ask": {
				Name: "ask",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Process the input",
				},
				Hooks: workflow.StepHooks{
					Post: workflow.Hook{
						{Log: "Post-hook: Agent step finished (may have failed)"},
						{Command: "echo 'post-hook cleanup'"},
					},
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

	executor := newMockExecutor()
	executor.results["echo 'post-hook cleanup'"] = &ports.CommandResult{
		Stdout:   "post-hook cleanup\n",
		ExitCode: 0,
	}

	registry := agents.NewAgentRegistry()
	claude := newMockAgentProvider("claude")
	claude.results["Process the input"] = &workflow.AgentResult{
		Provider:    "claude",
		Output:      "",
		Response:    nil,
		Tokens:      0,
		Error:       errors.New("API rate limit exceeded"),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}
	_ = registry.Register(claude)

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

	ctx, err := execSvc.Run(context.Background(), "agent-posthook-fail", nil)

	// Should complete (via OnFailure transition) despite agent failure
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "error", ctx.CurrentStep)

	// Verify agent step failed
	state, ok := ctx.GetStepState("ask")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusFailed, state.Status)
	assert.Contains(t, state.Error, "API rate limit exceeded")

	// Verify post-hook command was executed even though agent failed
	_, wasExecuted := executor.results["echo 'post-hook cleanup'"]
	assert.True(t, wasExecuted, "post-hook command should execute even on agent failure")
}
