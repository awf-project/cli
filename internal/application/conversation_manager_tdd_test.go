package application_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper: create basic execution context
func newTestExecutionContext(t *testing.T) *workflow.ExecutionContext {
	t.Helper()
	ec := workflow.NewExecutionContext("test-flow-id", "test-workflow")
	ec.States = make(map[string]workflow.StepState)
	return ec
}

// Helper: create basic build context function
func newTestBuildContext() func(*workflow.ExecutionContext) *interpolation.Context {
	return func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}
}

// Helper: create ConversationManager with minimal setup
func newTestConversationManager(t *testing.T, provider *mocks.MockAgentProvider) *application.ConversationManager {
	t.Helper()
	logger := mocks.NewMockLogger()
	resolver := newMockResolver()
	registry := mocks.NewMockAgentRegistry()

	if provider != nil {
		err := registry.Register(provider)
		require.NoError(t, err)
	}

	return application.NewConversationManager(logger, resolver, registry)
}

// TestConversationManager_ExecuteConversation_SingleTurn_HappyPath
// Verifies that ExecuteConversation completes after a single user message followed by empty input.
// The conversation should terminate with StopReasonUserExit.
func TestConversationManager_ExecuteConversation_SingleTurn_HappyPath(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")

	// Configure provider to return a valid ConversationResult on first turn
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		if state.Turns == nil {
			state.Turns = make([]workflow.Turn, 0)
		}
		state.Turns = append(
			state.Turns, //nolint:gocritic // separate appends aid readability in test closures
			workflow.Turn{Role: "user", Content: prompt},
			workflow.Turn{Role: "assistant", Content: "Hello! I'm here to help."},
		)

		return &workflow.ConversationResult{
			Provider: "claude",
			Output:   "Hello! I'm here to help.",
			State:    state,
		}, nil
	})

	manager := newTestConversationManager(t, provider)

	// User provides one message, then exits with empty input
	userInputReader := mocks.NewMockUserInputReader("yes, help me with this", "")
	manager.SetUserInputReader(userInputReader)

	step := &workflow.Step{
		Name: "chat-step",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello, can you help me?",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := newTestExecutionContext(t)
	buildContext := newTestBuildContext()

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext, io.Discard, io.Discard)

	// Assertions: should complete successfully with StopReasonUserExit
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, workflow.StopReasonUserExit, result.State.StoppedBy)
	assert.NotEmpty(t, result.Output)
	// 2 turns per executeTurn call: initial prompt + user message = 4 turns
	assert.Equal(t, 4, len(result.State.Turns))
}

// TestConversationManager_ExecuteConversation_MultiTurn_HappyPath
// Verifies that ExecuteConversation executes multiple turns (user input, agent response, repeat)
// until the user provides empty input.
func TestConversationManager_ExecuteConversation_MultiTurn_HappyPath(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")

	turnCount := 0
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		if state.Turns == nil {
			state.Turns = make([]workflow.Turn, 0)
		}
		state.Turns = append(state.Turns, workflow.Turn{
			Role:    "user",
			Content: prompt,
		})

		turnCount++
		response := "Response " + string(rune(48+turnCount)) //nolint:gosec // controlled test value, no overflow risk
		state.Turns = append(state.Turns, workflow.Turn{
			Role:    "assistant",
			Content: response,
		})

		return &workflow.ConversationResult{
			Provider: "claude",
			Output:   response,
			State:    state,
		}, nil
	})

	manager := newTestConversationManager(t, provider)

	// User provides two messages, then exits
	userInputReader := mocks.NewMockUserInputReader("First message", "Second message", "")
	manager.SetUserInputReader(userInputReader)

	step := &workflow.Step{
		Name: "chat-step",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Start the conversation",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := newTestExecutionContext(t)
	buildContext := newTestBuildContext()

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext, io.Discard, io.Discard)

	// Assertions: should execute 3 turns (initial + 2 user inputs), stop with UserExit
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, workflow.StopReasonUserExit, result.State.StoppedBy)
	// 3 turns: initial + 2 user-agent pairs = 6 messages (3 pairs of user+assistant)
	assert.Equal(t, 6, len(result.State.Turns))
}

// TestConversationManager_ExecuteConversation_WithSystemPrompt
// Verifies that system prompt is passed to the provider in options
// and that the conversation executes correctly with system guidance.
func TestConversationManager_ExecuteConversation_WithSystemPrompt(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")

	var capturedSystemPrompt string
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		// Capture system_prompt from options
		if sp, ok := options["system_prompt"]; ok {
			capturedSystemPrompt = sp.(string)
		}

		if state.Turns == nil {
			state.Turns = make([]workflow.Turn, 0)
		}
		state.Turns = append(
			state.Turns, //nolint:gocritic // separate appends aid readability in test closures
			workflow.Turn{Role: "user", Content: prompt},
			workflow.Turn{Role: "assistant", Content: "Math result: 4"},
		)

		return &workflow.ConversationResult{
			Provider: "claude",
			Output:   "Math result: 4",
			State:    state,
		}, nil
	})

	manager := newTestConversationManager(t, provider)
	manager.SetUserInputReader(mocks.NewMockUserInputReader(""))

	step := &workflow.Step{
		Name: "math-chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider:     "claude",
			Prompt:       "What is 2+2?",
			SystemPrompt: "You are a helpful math tutor.",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := newTestExecutionContext(t)
	buildContext := newTestBuildContext()

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext, io.Discard, io.Discard)

	// Assertions: system prompt should be passed to provider
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "You are a helpful math tutor.", capturedSystemPrompt)
	assert.Equal(t, workflow.StopReasonUserExit, result.State.StoppedBy)
}

// TestConversationManager_ExecuteConversation_ContinueFrom_HappyPath
// Verifies that ContinueFrom successfully resumes a prior conversation session,
// retaining prior turn history and session ID.
func TestConversationManager_ExecuteConversation_ContinueFrom_HappyPath(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")

	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		if state.Turns == nil {
			state.Turns = make([]workflow.Turn, 0)
		}
		state.Turns = append(
			state.Turns, //nolint:gocritic // separate appends aid readability in test closures
			workflow.Turn{Role: "user", Content: prompt},
			workflow.Turn{Role: "assistant", Content: "Continuing our conversation..."},
		)

		return &workflow.ConversationResult{
			Provider: "claude",
			Output:   "Continuing our conversation...",
			State:    state,
		}, nil
	})

	manager := newTestConversationManager(t, provider)
	manager.SetUserInputReader(mocks.NewMockUserInputReader(""))

	// Set up prior conversation state in execution context
	execCtx := newTestExecutionContext(t)
	priorState := workflow.NewConversationState("Prior system prompt")
	priorState.SessionID = "session-123"
	priorState.Turns = []workflow.Turn{
		{Role: "user", Content: "First message"},
		{Role: "assistant", Content: "First response"},
	}
	execCtx.States["prior-step"] = workflow.StepState{
		Conversation: priorState,
	}

	step := &workflow.Step{
		Name: "continue-chat",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Continue with a new question",
		},
	}

	config := &workflow.ConversationConfig{
		ContinueFrom: "prior-step",
	}

	buildContext := newTestBuildContext()

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext, io.Discard, io.Discard)

	// Assertions: should resume prior session, retain session ID and prior turns
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "session-123", result.State.SessionID)
	// Should have prior 2 turns + new 2 turns = 4 total
	assert.Equal(t, 4, len(result.State.Turns))
	assert.Equal(t, workflow.StopReasonUserExit, result.State.StoppedBy)
}

// TestConversationManager_ExecuteConversation_MissingUserInputReader_Error
// Verifies that ExecuteConversation returns a clear error when UserInputReader is not configured.
// This is critical for preventing silent failures in interactive mode.
func TestConversationManager_ExecuteConversation_MissingUserInputReader_Error(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		return &workflow.ConversationResult{
			Provider: "claude",
			Output:   "Response",
			State:    state,
		}, nil
	})

	manager := newTestConversationManager(t, provider)
	// Do NOT set UserInputReader

	step := &workflow.Step{
		Name: "chat-step",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := newTestExecutionContext(t)
	buildContext := newTestBuildContext()

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext, io.Discard, io.Discard)

	// Assertions: should return error about missing UserInputReader
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UserInputReader")
	assert.Nil(t, result)
}

// TestConversationManager_ExecuteConversation_NilStep_Error
// Verifies that ExecuteConversation returns an error when step is nil.
func TestConversationManager_ExecuteConversation_NilStep_Error(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")
	manager := newTestConversationManager(t, provider)
	manager.SetUserInputReader(mocks.NewMockUserInputReader(""))

	execCtx := newTestExecutionContext(t)
	buildContext := newTestBuildContext()

	result, err := manager.ExecuteConversation(context.Background(), nil, &workflow.ConversationConfig{}, execCtx, buildContext, io.Discard, io.Discard)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestConversationManager_ExecuteConversation_ProviderNotFound_Error
// Verifies that ExecuteConversation returns an error when the provider is not registered.
func TestConversationManager_ExecuteConversation_ProviderNotFound_Error(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")
	manager := newTestConversationManager(t, provider)
	manager.SetUserInputReader(mocks.NewMockUserInputReader(""))

	step := &workflow.Step{
		Name: "chat",
		Agent: &workflow.AgentConfig{
			Provider: "nonexistent-provider",
			Prompt:   "Hello",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := newTestExecutionContext(t)
	buildContext := newTestBuildContext()

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext, io.Discard, io.Discard)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestConversationManager_ExecuteConversation_ContextCancellation_Error
// Verifies that ExecuteConversation respects context cancellation and returns immediately
// when context is cancelled during the read phase.
func TestConversationManager_ExecuteConversation_ContextCancellation_Error(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")

	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		if state.Turns == nil {
			state.Turns = make([]workflow.Turn, 0)
		}
		state.Turns = append(
			state.Turns, //nolint:gocritic // separate appends aid readability in test closures
			workflow.Turn{Role: "user", Content: prompt},
			workflow.Turn{Role: "assistant", Content: "Response"},
		)

		return &workflow.ConversationResult{
			Provider: "claude",
			Output:   "Response",
			State:    state,
		}, nil
	})

	manager := newTestConversationManager(t, provider)

	// Create a user input reader that will be canceled during read
	userInputReader := mocks.NewMockUserInputReader()
	userInputReader.SetReadError(context.Canceled)
	manager.SetUserInputReader(userInputReader)

	step := &workflow.Step{
		Name: "chat",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := newTestExecutionContext(t)
	buildContext := newTestBuildContext()

	_, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext, io.Discard, io.Discard)

	// Assertions: should return error related to context cancellation
	assert.Error(t, err)
}

// TestConversationManager_ExecuteConversation_ContinueFromStepNotFound_Error
// Verifies that ExecuteConversation returns an error when ContinueFrom references
// a non-existent step in the execution context.
func TestConversationManager_ExecuteConversation_ContinueFromStepNotFound_Error(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")
	manager := newTestConversationManager(t, provider)
	manager.SetUserInputReader(mocks.NewMockUserInputReader(""))

	step := &workflow.Step{
		Name: "chat",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}

	config := &workflow.ConversationConfig{
		ContinueFrom: "nonexistent-prior-step",
	}

	execCtx := newTestExecutionContext(t)
	// No prior step in states
	buildContext := newTestBuildContext()

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext, io.Discard, io.Discard)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Nil(t, result)
}

// TestConversationManager_ExecuteConversation_ProviderExecutionError_PreserveLastResult
// Verifies that when a provider returns an error mid-conversation,
// the last successful result is preserved and the error is returned.
func TestConversationManager_ExecuteConversation_ProviderExecutionError_PreserveLastResult(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")

	callCount := 0
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		callCount++

		if state.Turns == nil {
			state.Turns = make([]workflow.Turn, 0)
		}
		state.Turns = append(state.Turns, workflow.Turn{
			Role:    "user",
			Content: prompt,
		})

		// Succeed on first call, fail on second
		if callCount == 1 {
			state.Turns = append(state.Turns, workflow.Turn{
				Role:    "assistant",
				Content: "First response",
			})
			return &workflow.ConversationResult{
				Provider: "claude",
				Output:   "First response",
				State:    state,
			}, nil
		}

		return nil, errors.New("provider error on second turn")
	})

	manager := newTestConversationManager(t, provider)
	manager.SetUserInputReader(mocks.NewMockUserInputReader("user message", ""))

	step := &workflow.Step{
		Name: "chat",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := newTestExecutionContext(t)
	buildContext := newTestBuildContext()

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext, io.Discard, io.Discard)

	// Assertions: should return error but preserve last result
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
	assert.NotNil(t, result.State)
}

// TestConversationManager_SetUserInputReader
// Verifies that SetUserInputReader correctly wires the dependency.
func TestConversationManager_SetUserInputReader(t *testing.T) {
	manager := newTestConversationManager(t, mocks.NewMockAgentProvider("claude"))

	reader := mocks.NewMockUserInputReader("test")
	manager.SetUserInputReader(reader)

	// Verify reader was set by attempting to use it
	// (If not set, ExecuteConversation would fail with "UserInputReader" error)
	provider := mocks.NewMockAgentProvider("test-provider")
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		return &workflow.ConversationResult{
			Provider: "test-provider",
			Output:   "test",
			State:    state,
		}, nil
	})

	registry := mocks.NewMockAgentRegistry()
	registry.Register(provider)

	manager2 := application.NewConversationManager(
		mocks.NewMockLogger(),
		newMockResolver(),
		registry,
	)
	manager2.SetUserInputReader(reader)

	step := &workflow.Step{
		Name: "test",
		Agent: &workflow.AgentConfig{
			Provider: "test-provider",
			Prompt:   "test",
		},
	}

	execCtx := newTestExecutionContext(t)
	buildContext := newTestBuildContext()

	result, err := manager2.ExecuteConversation(context.Background(), step, &workflow.ConversationConfig{}, execCtx, buildContext, io.Discard, io.Discard)

	// If reader was properly set, call should succeed (not fail on missing UserInputReader)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestConversationManager_ExecuteConversation_EmptyInputTerminates
// Verifies that providing an empty string (or whitespace-only) immediately terminates
// the conversation without requiring provider call.
func TestConversationManager_ExecuteConversation_EmptyInputTerminates(t *testing.T) {
	provider := mocks.NewMockAgentProvider("claude")

	callCount := 0
	provider.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		callCount++

		if state.Turns == nil {
			state.Turns = make([]workflow.Turn, 0)
		}
		state.Turns = append(
			state.Turns, //nolint:gocritic // separate appends aid readability in test closures
			workflow.Turn{Role: "user", Content: prompt},
			workflow.Turn{Role: "assistant", Content: "Response"},
		)

		return &workflow.ConversationResult{
			Provider: "claude",
			Output:   "Response",
			State:    state,
		}, nil
	})

	manager := newTestConversationManager(t, provider)

	// First empty input should terminate immediately
	userInputReader := mocks.NewMockUserInputReader("")
	manager.SetUserInputReader(userInputReader)

	step := &workflow.Step{
		Name: "chat",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Hello",
		},
	}

	config := &workflow.ConversationConfig{}
	execCtx := newTestExecutionContext(t)
	buildContext := newTestBuildContext()

	result, err := manager.ExecuteConversation(context.Background(), step, config, execCtx, buildContext, io.Discard, io.Discard)

	// Assertions: should exit after first turn without another provider call
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, workflow.StopReasonUserExit, result.State.StoppedBy)
	assert.Equal(t, 1, callCount)
}
