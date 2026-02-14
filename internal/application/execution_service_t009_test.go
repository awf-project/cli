package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

//
// Component T009: Implement executeConversationStep() in execution_service.go:1850-1873
// Purpose: Delegate conversation orchestration to ConversationManager
//
// This test suite verifies that executeConversationStep:
// 1. Validates conversation manager is configured
// 2. Extracts conversation config from step.Agent.Conversation
// 3. Delegates to ConversationManager.ExecuteConversation()
// 4. Maps ConversationResult to StepState
// 5. Persists conversation state correctly
//
// Test Structure:
// - Happy Path: Successful delegation and result mapping
// - Edge Cases: Minimal config, empty responses, single turn
// - Error Handling: Nil manager, invalid config, provider errors

// TestExecuteConversationStep_T009_HappyPath_SingleTurnSuccess tests that
// executeConversationStep successfully delegates a single-turn conversation
// to ConversationManager and maps the result to StepState.
func TestExecuteConversationStep_T009_HappyPath_SingleTurnSuccess(t *testing.T) {
	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "claude",
			Mode:          "conversation",
			SystemPrompt:  "You are helpful",
			InitialPrompt: "Hello",
			Conversation: &workflow.ConversationConfig{
				MaxTurns: 1,
			},
		},
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	// Mock ConversationManager that returns successful result
	convState := &workflow.ConversationState{
		Turns: []workflow.Turn{
			{Role: workflow.TurnRoleUser, Content: "Hello"},
			{Role: workflow.TurnRoleAssistant, Content: "Hi there!"},
		},
		TotalTurns:  1,
		TotalTokens: 50,
		StoppedBy:   workflow.StopReasonMaxTurns,
	}
	mockConvMgr := &mockConversationManagerT009{
		result: &workflow.ConversationResult{
			Provider:     "claude",
			State:        convState,
			Output:       "Hi there!",
			TokensInput:  25,
			TokensOutput: 25,
			TokensTotal:  50,
			StartedAt:    time.Now(),
			CompletedAt:  time.Now(),
		},
	}
	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	nextStep, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, "", nextStep) // Empty string means use OnSuccess transition

	// Verify step state was updated in execution context
	state, exists := execCtx.GetStepState("chat")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, "Hi there!", state.Output)
	assert.Equal(t, 50, state.TokensUsed)

	// Verify conversation state was persisted
	assert.NotNil(t, state.Conversation)
	assert.Len(t, state.Conversation.Turns, 2)
	assert.Equal(t, workflow.StopReasonMaxTurns, state.Conversation.StoppedBy)
}

// TestExecuteConversationStep_T009_HappyPath_MultiTurnSuccess tests that
// executeConversationStep handles multi-turn conversations correctly.
func TestExecuteConversationStep_T009_HappyPath_MultiTurnSuccess(t *testing.T) {
	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "claude",
			Mode:          "conversation",
			SystemPrompt:  "You are a coder",
			InitialPrompt: "Write a function",
			Conversation: &workflow.ConversationConfig{
				MaxTurns:         5,
				MaxContextTokens: 100000,
				Strategy:         workflow.StrategySlidingWindow,
				StopCondition:    "response contains 'DONE'",
			},
		},
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	// Mock 3-turn conversation
	convState := &workflow.ConversationState{
		Turns: []workflow.Turn{
			{Role: workflow.TurnRoleUser, Content: "Write a function"},
			{Role: workflow.TurnRoleAssistant, Content: "Sure, what function?"},
			{Role: workflow.TurnRoleUser, Content: "Write a function"},
			{Role: workflow.TurnRoleAssistant, Content: "func hello() { print(\"hi\") }"},
			{Role: workflow.TurnRoleUser, Content: "Write a function"},
			{Role: workflow.TurnRoleAssistant, Content: "DONE"},
		},
		TotalTurns:  3,
		TotalTokens: 250,
		StoppedBy:   workflow.StopReasonCondition,
	}
	mockConvMgr := &mockConversationManagerT009{
		result: &workflow.ConversationResult{
			Provider:    "claude",
			State:       convState,
			Output:      "DONE",
			TokensTotal: 250,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		},
	}
	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	nextStep, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, "", nextStep)

	state, exists := execCtx.GetStepState("chat")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, "DONE", state.Output)
	assert.Equal(t, 250, state.TokensUsed)

	// Verify multi-turn conversation state
	assert.NotNil(t, state.Conversation)
	assert.Len(t, state.Conversation.Turns, 6) // 3 user + 3 assistant turns
	assert.Equal(t, workflow.StopReasonCondition, state.Conversation.StoppedBy)
}

// TestExecuteConversationStep_T009_HappyPath_WithInputInterpolation tests that
// executeConversationStep passes buildContext function to ConversationManager
// for interpolating InitialPrompt with workflow inputs and step states.
func TestExecuteConversationStep_T009_HappyPath_WithInputInterpolation(t *testing.T) {
	step := &workflow.Step{
		Name: "analyze",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "claude",
			Mode:          "conversation",
			InitialPrompt: "Analyze code: {{inputs.code}}",
			Conversation: &workflow.ConversationConfig{
				MaxTurns: 2,
			},
		},
	}

	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	execCtx.Inputs = map[string]any{
		"code": "func main() {}",
	}
	// Track that buildContext was called
	var buildContextCalled bool

	mockConvMgr := &mockConversationManagerWithBuildContextT009{
		result: &workflow.ConversationResult{
			Provider:    "claude",
			State:       &workflow.ConversationState{},
			Output:      "Analysis complete",
			TokensTotal: 100,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		},
		onExecute: func(buildCtx ContextBuilderFunc, execCtx *workflow.ExecutionContext) {
			buildContextCalled = true
			// Verify buildContext can be called and produces valid context
			intCtx := buildCtx(execCtx)
			assert.NotNil(t, intCtx)
			assert.Equal(t, "func main() {}", intCtx.Inputs["code"])
		},
	}
	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	_, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.True(t, buildContextCalled, "buildContext function should have been passed and called")
}

// TestExecuteConversationStep_T009_EdgeCase_MinimalConfig tests that
// executeConversationStep works with minimal conversation config (defaults).
func TestExecuteConversationStep_T009_EdgeCase_MinimalConfig(t *testing.T) {
	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "claude",
			Mode:          "conversation",
			InitialPrompt: "Hello",
			Conversation:  &workflow.ConversationConfig{}, // Empty config - use defaults
		},
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	mockConvMgr := &mockConversationManagerT009{
		result: &workflow.ConversationResult{
			Provider:    "claude",
			State:       &workflow.ConversationState{},
			Output:      "Hi",
			TokensTotal: 10,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		},
	}
	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	nextStep, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, "", nextStep)

	state, exists := execCtx.GetStepState("chat")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

// TestExecuteConversationStep_T009_EdgeCase_EmptyOutput tests handling of
// conversations where final turn produces empty assistant response.
func TestExecuteConversationStep_T009_EdgeCase_EmptyOutput(t *testing.T) {
	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "claude",
			Mode:          "conversation",
			InitialPrompt: "Say nothing",
			Conversation: &workflow.ConversationConfig{
				MaxTurns: 1,
			},
		},
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	convState := &workflow.ConversationState{
		Turns: []workflow.Turn{
			{Role: workflow.TurnRoleUser, Content: "Say nothing"},
			{Role: workflow.TurnRoleAssistant, Content: ""}, // Empty response
		},
		TotalTurns:  1,
		TotalTokens: 5,
		StoppedBy:   workflow.StopReasonMaxTurns,
	}
	mockConvMgr := &mockConversationManagerT009{
		result: &workflow.ConversationResult{
			Provider:    "claude",
			State:       convState,
			Output:      "", // Empty output
			TokensTotal: 5,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		},
	}
	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	nextStep, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, "", nextStep)

	state, exists := execCtx.GetStepState("chat")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, "", state.Output) // Empty output is valid
}

// TestExecuteConversationStep_T009_EdgeCase_ContextCancellation tests that
// executeConversationStep respects context cancellation.
func TestExecuteConversationStep_T009_EdgeCase_ContextCancellation(t *testing.T) {
	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "claude",
			Mode:          "conversation",
			InitialPrompt: "Long task",
			Conversation: &workflow.ConversationConfig{
				MaxTurns: 100,
			},
		},
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	// Mock manager that checks context
	mockConvMgr := &mockConversationManagerWithContextT009{
		executeFunc: func(ctx context.Context, step *workflow.Step, config *workflow.ConversationConfig, execCtx *workflow.ExecutionContext, buildContext ContextBuilderFunc) (*workflow.ConversationResult, error) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, errors.New("should not reach here")
		},
	}
	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.executeConversationStep(ctx, step, execCtx)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

// TestExecuteConversationStep_T009_Error_NoConversationManager tests that
// executeConversationStep returns error when ConversationManager is nil.
func TestExecuteConversationStep_T009_Error_NoConversationManager(t *testing.T) {
	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "claude",
			Mode:          "conversation",
			InitialPrompt: "Hello",
			Conversation: &workflow.ConversationConfig{
				MaxTurns: 1,
			},
		},
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	svc := &ExecutionService{
		conversationMgr: nil, // Manager not configured
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	_, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversation manager not configured")
}

// TestExecuteConversationStep_T009_Error_NilConversationConfig tests that
// executeConversationStep returns error when step.Agent.Conversation is nil.
func TestExecuteConversationStep_T009_Error_NilConversationConfig(t *testing.T) {
	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "claude",
			Mode:          "conversation",
			InitialPrompt: "Hello",
			Conversation:  nil, // Missing conversation config
		},
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	mockConvMgr := &mockConversationManagerT009{}

	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	_, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversation config is nil")
}

// TestExecuteConversationStep_T009_Error_NilAgentConfig tests that
// executeConversationStep returns error when step.Agent is nil.
func TestExecuteConversationStep_T009_Error_NilAgentConfig(t *testing.T) {
	step := &workflow.Step{
		Name:  "chat",
		Type:  workflow.StepTypeAgent,
		Agent: nil, // Missing agent config
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	mockConvMgr := &mockConversationManagerT009{}

	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	_, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent config is nil")
}

// TestExecuteConversationStep_T009_Error_ConversationManagerFailure tests that
// executeConversationStep propagates errors from ConversationManager.
func TestExecuteConversationStep_T009_Error_ConversationManagerFailure(t *testing.T) {
	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "claude",
			Mode:          "conversation",
			InitialPrompt: "Hello",
			Conversation: &workflow.ConversationConfig{
				MaxTurns: 1,
			},
		},
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	// Mock manager that returns error
	mockConvMgr := &mockConversationManagerT009{
		err: errors.New("provider authentication failed"),
	}
	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	_, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider authentication failed")
}

// TestExecuteConversationStep_T009_Error_ProviderNotFound tests error handling
// when agent provider is not registered in the registry.
func TestExecuteConversationStep_T009_Error_ProviderNotFound(t *testing.T) {
	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "unknown-provider", // Not registered
			Mode:          "conversation",
			InitialPrompt: "Hello",
			Conversation: &workflow.ConversationConfig{
				MaxTurns: 1,
			},
		},
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	// Mock manager that propagates "provider not found" error
	mockConvMgr := &mockConversationManagerT009{
		err: errors.New("provider not found: unknown-provider"),
	}
	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	_, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider not found")
}

// TestExecuteConversationStep_T009_Error_InterpolationFailure tests error handling
// when initial prompt interpolation fails.
func TestExecuteConversationStep_T009_Error_InterpolationFailure(t *testing.T) {
	step := &workflow.Step{
		Name: "chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:      "claude",
			Mode:          "conversation",
			InitialPrompt: "Use {{invalid.reference}}", // Invalid interpolation
			Conversation: &workflow.ConversationConfig{
				MaxTurns: 1,
			},
		},
	}
	execCtx := workflow.NewExecutionContext("test-wf", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	// Mock manager that propagates interpolation error
	mockConvMgr := &mockConversationManagerT009{
		err: errors.New("interpolation failed: undefined variable 'invalid'"),
	}
	svc := &ExecutionService{
		conversationMgr: mockConvMgr,
		logger:          newMockLogger(),
		resolver:        newMockResolver(),
	}
	_, err := svc.executeConversationStep(context.Background(), step, execCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "interpolation failed")
}

// mockConversationManagerT009 is a simple mock that returns predefined result/error.
type mockConversationManagerT009 struct {
	result *workflow.ConversationResult
	err    error
}

func (m *mockConversationManagerT009) ExecuteConversation(
	ctx context.Context,
	step *workflow.Step,
	config *workflow.ConversationConfig,
	execCtx *workflow.ExecutionContext,
	buildContext ContextBuilderFunc,
) (*workflow.ConversationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// mockConversationManagerWithContextT009 allows testing context cancellation.
type mockConversationManagerWithContextT009 struct {
	executeFunc func(ctx context.Context, step *workflow.Step, config *workflow.ConversationConfig, execCtx *workflow.ExecutionContext, buildContext ContextBuilderFunc) (*workflow.ConversationResult, error)
}

func (m *mockConversationManagerWithContextT009) ExecuteConversation(
	ctx context.Context,
	step *workflow.Step,
	config *workflow.ConversationConfig,
	execCtx *workflow.ExecutionContext,
	buildContext ContextBuilderFunc,
) (*workflow.ConversationResult, error) {
	return m.executeFunc(ctx, step, config, execCtx, buildContext)
}

// mockConversationManagerWithBuildContextT009 captures buildContext calls for verification.
type mockConversationManagerWithBuildContextT009 struct {
	result    *workflow.ConversationResult
	onExecute func(ContextBuilderFunc, *workflow.ExecutionContext)
}

func (m *mockConversationManagerWithBuildContextT009) ExecuteConversation(
	ctx context.Context,
	step *workflow.Step,
	config *workflow.ConversationConfig,
	execCtx *workflow.ExecutionContext,
	buildContext ContextBuilderFunc,
) (*workflow.ConversationResult, error) {
	if m.onExecute != nil {
		m.onExecute(buildContext, execCtx)
	}
	return m.result, nil
}

// Note: mockLogger and mockResolver are defined in execution_service_helpers_test.go
