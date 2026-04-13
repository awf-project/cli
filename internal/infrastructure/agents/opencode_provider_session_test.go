package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T006: OpenCode ExecuteConversation with session resume
// Tests for ExecuteConversation() with JSON extraction and -c fallback

func TestOpenCodeProvider_ExecuteConversation_FirstTurn_HappyPath(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	// NDJSON format: step_start event with sessionID, followed by step_end with content
	mockOutput := []byte(`{"type":"step_start","sessionID":"ses_abc123","timestamp":1234567890}` + "\n" +
		`{"type":"step_end","output":"Generated code structure"}`)
	mockExec.SetOutput(mockOutput, nil)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
	state := workflow.NewConversationState("You are a code generator")

	result, err := provider.ExecuteConversation(context.Background(), state, "Create a Hello World program", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "opencode", result.Provider)
	assert.NotEmpty(t, result.Output)
	assert.NotNil(t, result.State)
	assert.Equal(t, "ses_abc123", result.State.SessionID, "session ID should be extracted from step_start event")
	assert.True(t, result.TokensEstimated)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
	assert.GreaterOrEqual(t, result.State.TotalTurns, 2, "should have at least user and assistant turns")
}

func TestOpenCodeProvider_ExecuteConversation_Resume_WithSessionID(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockOutput := []byte(`{"type":"step_start","sessionID":"ses_xyz789","timestamp":1234567890}` + "\n" +
		`{"type":"step_end","output":"Continued from session"}`)
	mockExec.SetOutput(mockOutput, nil)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	state := workflow.NewConversationState("You are a code generator")
	state.SessionID = "ses_xyz789"
	state.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleSystem, "You are a code generator"),
		*workflow.NewTurn(workflow.TurnRoleUser, "Create a Hello World program"),
		*workflow.NewTurn(workflow.TurnRoleAssistant, "Generated code"),
	}
	state.TotalTurns = 3

	result, err := provider.ExecuteConversation(context.Background(), state, "Add more features", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "opencode", result.Provider)
	assert.NotEmpty(t, result.Output)
	// Session ID should be preserved or updated from output
	assert.Equal(t, "ses_xyz789", result.State.SessionID)
	assert.GreaterOrEqual(t, result.State.TotalTurns, 4, "should accumulate turns across calls")
}

func TestOpenCodeProvider_ExecuteConversation_ContinueFallback(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockOutput := []byte(`{"type":"step_start","sessionID":"ses_new123","timestamp":1234567890}` + "\n" +
		`{"type":"step_end","output":"Continued"}`)
	mockExec.SetOutput(mockOutput, nil)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	// State with prior turns but no SessionID (extraction failed on previous turn)
	state := workflow.NewConversationState("")
	state.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleUser, "first prompt"),
		*workflow.NewTurn(workflow.TurnRoleAssistant, "first response"),
	}
	state.TotalTurns = 2

	result, err := provider.ExecuteConversation(context.Background(), state, "continue", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1, "should have executed one command")
	assert.Contains(t, calls[0].Args, "-c", "-c fallback should be present when prior turns exist but no SessionID")
	assert.NotContains(t, calls[0].Args, "-s", "-s should not be present without SessionID")
}

func TestOpenCodeProvider_ExecuteConversation_FirstTurnWithSystemPrompt(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockOutput := []byte(`{"type":"step_start","sessionID":"ses_sys999","timestamp":1234567890}` + "\n" +
		`{"type":"step_end","output":"Generated with system context"}`)
	mockExec.SetOutput(mockOutput, nil)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
	// Empty state means no system turn yet, so system_prompt option will be passed to CLI
	state := workflow.NewConversationState("")

	options := map[string]any{
		"system_prompt": "You are a specialized code generator",
	}

	result, err := provider.ExecuteConversation(context.Background(), state, "Generate test code", options, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ses_sys999", result.State.SessionID)

	// OpenCode CLI has no --system-prompt flag; the system prompt is inlined
	// into the first turn's message instead. Verify it is prepended to the prompt.
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.NotContains(t, calls[0].Args, "--system-prompt", "opencode has no --system-prompt flag")
	assert.NotContains(t, calls[0].Args, "-c", "-c should not be used when there are no prior turns")
	require.GreaterOrEqual(t, len(calls[0].Args), 2, "expected run <prompt> ... args")
	assert.Equal(t, "run", calls[0].Args[0])
	assert.Contains(t, calls[0].Args[1], "You are a specialized code generator", "system prompt should be inlined in first-turn message")
	assert.Contains(t, calls[0].Args[1], "Generate test code", "user prompt should remain in first-turn message")
}

func TestOpenCodeProvider_ExecuteConversation_ErrorPaths(t *testing.T) {
	tests := []struct {
		name       string
		mockOutput []byte
		expectErr  string // error message substring to check
	}{
		{
			name:       "malformed JSON returns empty sessionID",
			mockOutput: []byte("plain text output with no JSON"),
			expectErr:  "",
		},
		{
			name:       "missing sessionID field",
			mockOutput: []byte(`{"type":"step_start","timestamp":1234567890}`),
			expectErr:  "",
		},
		{
			name:       "sessionID is null",
			mockOutput: []byte(`{"type":"step_start","sessionID":null}`),
			expectErr:  "",
		},
		{
			name:       "empty output",
			mockOutput: []byte(""),
			expectErr:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)

			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
			state := workflow.NewConversationState("System prompt")

			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, nil, nil)

			// Extraction failures should not cause ExecuteConversation to error
			// They should gracefully fall back to empty SessionID
			require.NoError(t, err, "ExecuteConversation should succeed even with bad extraction")
			require.NotNil(t, result)
			assert.Empty(t, result.State.SessionID, "SessionID should be empty when extraction fails")
		})
	}
}

func TestOpenCodeProvider_ExecuteConversation_InputValidation(t *testing.T) {
	provider := NewOpenCodeProvider()
	state := workflow.NewConversationState("")

	tests := []struct {
		name   string
		prompt string
		state  *workflow.ConversationState
	}{
		{
			name:   "empty prompt",
			prompt: "",
			state:  state,
		},
		{
			name:   "whitespace-only prompt",
			prompt: "   \n  \t  ",
			state:  state,
		},
		{
			name:   "nil state",
			prompt: "valid prompt",
			state:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.ExecuteConversation(context.Background(), tt.state, tt.prompt, nil, nil, nil)

			assert.Error(t, err, "should validate inputs")
			assert.Nil(t, result)
		})
	}
}

func TestOpenCodeProvider_ExecuteConversation_ContextCancellation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("output"), nil)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
	state := workflow.NewConversationState("")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.ExecuteConversation(ctx, state, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestOpenCodeProvider_ExecuteConversation_CLIExecutionFailure(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(""), nil)
	mockExec.SetError(assert.AnError)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
	state := workflow.NewConversationState("")

	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}
