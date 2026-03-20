package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T009: OpenCode ExecuteConversation with session resume
// Tests for ExecuteConversation() and extractSessionID() methods

func TestOpenCodeProvider_extractSessionID(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantID  string
		wantErr bool
	}{
		{
			name:    "valid session line",
			output:  "Building code...\nSession: opencode-session-abc123\nDone",
			wantID:  "opencode-session-abc123",
			wantErr: false,
		},
		{
			name:    "session id with hex characters",
			output:  "Starting execution\nSession: 5f8a2b1c9e3d4f6a\nExecution complete",
			wantID:  "5f8a2b1c9e3d4f6a",
			wantErr: false,
		},
		{
			name:    "session id at beginning",
			output:  "Session: first-session-id\nOther output",
			wantID:  "first-session-id",
			wantErr: false,
		},
		{
			name:    "session id at end",
			output:  "Some output\nFinal line\nSession: last-session-id",
			wantID:  "last-session-id",
			wantErr: false,
		},
		{
			name:    "no session line found",
			output:  "Code generated successfully\nNo session info",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "empty output",
			output:  "",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "session keyword but no colon",
			output:  "Session opencode-123\nOther text",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "malformed session line missing id",
			output:  "Session: \nOther output",
			wantID:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := extractSessionIDFromLines(tt.output)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, id)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}

func TestOpenCodeProvider_ExecuteConversation_FirstTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockOutput := []byte("Generated code structure\nSession: sess-123456\nReady for next turn")
	mockExec.SetOutput(mockOutput, nil)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
	state := workflow.NewConversationState("You are a code generator")

	result, err := provider.ExecuteConversation(context.Background(), state, "Create a Hello World program", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "opencode", result.Provider)
	assert.NotEmpty(t, result.Output)
	assert.NotNil(t, result.State)
	// Session ID should be extracted from first turn output
	assert.Equal(t, "sess-123456", result.State.SessionID)
	assert.True(t, result.TokensEstimated)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
}

func TestOpenCodeProvider_ExecuteConversation_Resume(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	resumeOutput := []byte("Continued from session\nSession: sess-123456\nAdditional code generated")
	mockExec.SetOutput(resumeOutput, nil)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	// Setup state with existing session
	state := workflow.NewConversationState("You are a code generator")
	state.SessionID = "sess-123456"
	state.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleSystem, "You are a code generator"),
		*workflow.NewTurn(workflow.TurnRoleUser, "Create a Hello World program"),
		*workflow.NewTurn(workflow.TurnRoleAssistant, "Generated code"),
	}
	state.TotalTurns = 3

	result, err := provider.ExecuteConversation(context.Background(), state, "Add more features", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "opencode", result.Provider)
	assert.NotEmpty(t, result.Output)
	assert.NotNil(t, result.State)
	// Session ID should be preserved or updated from output
	assert.NotEmpty(t, result.State.SessionID)
	assert.GreaterOrEqual(t, result.State.TotalTurns, 5)
}

func TestOpenCodeProvider_ExecuteConversation_SystemPrompt(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockOutput := []byte("Generated with system context\nSession: sess-abc789\nDone")
	mockExec.SetOutput(mockOutput, nil)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
	state := workflow.NewConversationState("Custom system prompt")

	options := map[string]any{
		"system_prompt": "You are a specialized code generator",
	}

	result, err := provider.ExecuteConversation(context.Background(), state, "Generate test code", options)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "opencode", result.Provider)
	assert.NotEmpty(t, result.State.SessionID)
}

func TestOpenCodeProvider_ExecuteConversation_GracefulFallback(t *testing.T) {
	tests := []struct {
		name       string
		mockOutput []byte
		wantErr    bool
	}{
		{
			name:       "no session in output",
			mockOutput: []byte("Code generated successfully\nNo session information available"),
			wantErr:    false,
		},
		{
			name:       "malformed session line",
			mockOutput: []byte("Starting...\nSession \nCode output here"),
			wantErr:    false,
		},
		{
			name:       "empty output",
			mockOutput: []byte(""),
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)

			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
			state := workflow.NewConversationState("System prompt")
			state.SessionID = "previous-session-id"

			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				// Extraction failed, SessionID should be empty for stateless fallback
				assert.Empty(t, result.State.SessionID)
			}
		})
	}
}

func TestOpenCodeProvider_ExecuteConversation_ValidationErrors(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.ExecuteConversation(context.Background(), tt.state, tt.prompt, nil)

			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func TestOpenCodeProvider_ExecuteConversation_ContextErrors(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("output"), nil)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
	state := workflow.NewConversationState("")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.ExecuteConversation(ctx, state, "test", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestOpenCodeProvider_ExecuteConversation_CLIErrors(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(""), nil)
	mockExec.SetError(assert.AnError)

	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))
	state := workflow.NewConversationState("")

	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}
