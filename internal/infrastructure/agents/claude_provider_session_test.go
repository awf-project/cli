package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T005: Resume flag and session ID extraction in ClaudeProvider.ExecuteConversation

func TestClaudeProvider_ExecuteConversation_ResumeFlag(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		wantResumeFlag bool
	}{
		{
			name:           "turn 2+ with session ID passes -r flag",
			sessionID:      "sess_abc123",
			wantResumeFlag: true,
		},
		{
			name:           "turn 1 without session ID omits -r flag",
			sessionID:      "",
			wantResumeFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"session_id":"sess_new","result":"response"}`), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.sessionID

			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			hasResumeFlag := false
			for i, arg := range calls[0].Args {
				if arg == "-r" && i+1 < len(calls[0].Args) {
					hasResumeFlag = true
					if tt.wantResumeFlag {
						assert.Equal(t, tt.sessionID, calls[0].Args[i+1])
					}
				}
			}
			assert.Equal(t, tt.wantResumeFlag, hasResumeFlag)
		})
	}
}

func TestClaudeProvider_ExecuteConversation_SessionIDExtracted(t *testing.T) {
	tests := []struct {
		name           string
		mockOutput     []byte
		wantSessionID  string
		wantOutputText string
	}{
		{
			name:           "session ID extracted from JSON output",
			mockOutput:     []byte(`{"session_id":"sess_extracted_123","result":"actual response text"}`),
			wantSessionID:  "sess_extracted_123",
			wantOutputText: "actual response text",
		},
		{
			name:           "no session_id field yields empty SessionID",
			mockOutput:     []byte(`{"result":"response without session"}`),
			wantSessionID:  "",
			wantOutputText: "response without session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantSessionID, result.State.SessionID)
			assert.Equal(t, tt.wantOutputText, result.Output)
		})
	}
}

func TestClaudeProvider_ExecuteConversation_SystemPromptOnFirstTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"session_id":"sess_1","result":"ok"}`), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	state := workflow.NewConversationState("")
	options := map[string]any{"system_prompt": "You are a code reviewer"}

	_, err := provider.ExecuteConversation(context.Background(), state, "Review this", options, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Args, "--system-prompt")
	assert.Contains(t, calls[0].Args, "You are a code reviewer")
}

func TestClaudeProvider_ExecuteConversation_NoSystemPromptOnResumeTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"session_id":"sess_resume","result":"ok"}`), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.SessionID = "existing_session_id"
	options := map[string]any{"system_prompt": "You are a code reviewer"}

	_, err := provider.ExecuteConversation(context.Background(), state, "Continue", options, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.NotContains(t, calls[0].Args, "--system-prompt")
}

func TestClaudeProvider_ExecuteConversation_GracefulFallback_NonJSON(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("plain text response without JSON"), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	state := workflow.NewConversationState("system")
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err, "extraction failure must not cause error")
	require.NotNil(t, result)
	assert.Empty(t, result.State.SessionID, "SessionID should be empty on extraction failure")
}

func TestClaudeProvider_ExecuteConversation_ForceJSONOutputFormat(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"session_id":"sess_1","result":"ok"}`), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	state := workflow.NewConversationState("system")

	_, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Args, "--output-format")
	assert.Contains(t, calls[0].Args, "stream-json")
}
