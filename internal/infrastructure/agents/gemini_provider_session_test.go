package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T008: Gemini session resume implementation
// Tests for extractSessionID() and session resume logic in ExecuteConversation

func TestGeminiProvider_extractSessionID(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		wantSessionID string
	}{
		{
			name:          "session ID from 'Session: <id>' format",
			output:        "Session: gemini-session-abc123\nGenerated response",
			wantSessionID: "gemini-session-abc123",
		},
		{
			name:          "session ID with complex alphanumeric",
			output:        "Session: sess_f47ac10b58cc4372a5670e02b2c3d479\n...",
			wantSessionID: "sess_f47ac10b58cc4372a5670e02b2c3d479",
		},
		{
			name:          "session ID as numeric",
			output:        "Session: 9876543210\nResponse content",
			wantSessionID: "9876543210",
		},
		{
			name:          "no Session line returns empty",
			output:        "No session info\nJust text output",
			wantSessionID: "",
		},
		{
			name:          "malformed Session line returns empty",
			output:        "Session:\nNo ID provided",
			wantSessionID: "",
		},
		{
			name:          "empty output returns empty",
			output:        "",
			wantSessionID: "",
		},
		{
			name:          "Session line with trailing whitespace",
			output:        "Session: gemini-sess-123  \nMore content",
			wantSessionID: "gemini-sess-123",
		},
		{
			name:          "multiple Session lines uses first",
			output:        "Session: first-id\nSession: second-id\nResponse",
			wantSessionID: "first-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractSessionIDFromLines(tt.output)
			assert.Equal(t, tt.wantSessionID, got)
			if tt.wantSessionID == "" {
				assert.Error(t, err, "extraction failure should return error")
			} else {
				assert.NoError(t, err, "successful extraction should not error")
			}
		})
	}
}

func TestGeminiProvider_ExecuteConversation_ResumeFlag(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		wantResumeFlag bool
	}{
		{
			name:           "turn 2+ with session ID uses resume flag",
			sessionID:      "gemini-session-abc",
			wantResumeFlag: true,
		},
		{
			name:           "turn 1 without session ID omits resume",
			sessionID:      "",
			wantResumeFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("Session: gemini-new-session\nGenerated response"), nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.sessionID

			result, err := provider.ExecuteConversation(context.Background(), state, "write a poem", nil)
			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			hasResumeFlag := false
			for i, arg := range calls[0].Args {
				if arg == "--resume" && i+1 < len(calls[0].Args) {
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

func TestGeminiProvider_ExecuteConversation_SessionIDExtracted(t *testing.T) {
	tests := []struct {
		name           string
		mockOutput     []byte
		wantSessionID  string
		wantOutputText string
	}{
		{
			name:           "session ID extracted from output",
			mockOutput:     []byte("Session: gemini-extracted-123\nThis is a generated poem"),
			wantSessionID:  "gemini-extracted-123",
			wantOutputText: "Session: gemini-extracted-123\nThis is a generated poem",
		},
		{
			name:           "no session line yields empty SessionID",
			mockOutput:     []byte("Text output without session info"),
			wantSessionID:  "",
			wantOutputText: "Text output without session info",
		},
		{
			name:           "empty session in line yields empty",
			mockOutput:     []byte("Session: \nsome text"),
			wantSessionID:  "",
			wantOutputText: "Session: \nsome text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test", nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantSessionID, result.State.SessionID)
			assert.Equal(t, tt.wantOutputText, result.Output)
		})
	}
}

func TestGeminiProvider_ExecuteConversation_SystemPromptOnFirstTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("Session: gemini-1\nGenerated response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	state := workflow.NewConversationState("")
	options := map[string]any{"system_prompt": "You are a helpful assistant"}

	_, err := provider.ExecuteConversation(context.Background(), state, "Generate a hello world", options)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Args, "--system-prompt")
	assert.Contains(t, calls[0].Args, "You are a helpful assistant")
}

func TestGeminiProvider_ExecuteConversation_NoSystemPromptOnResumeTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("Session: gemini-resume\nContinued response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.SessionID = "existing-session-id"
	options := map[string]any{"system_prompt": "You are a helpful assistant"}

	_, err := provider.ExecuteConversation(context.Background(), state, "Continue", options)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.NotContains(t, calls[0].Args, "--system-prompt")
}

func TestGeminiProvider_ExecuteConversation_GracefulFallback(t *testing.T) {
	tests := []struct {
		name       string
		mockOutput []byte
	}{
		{
			name:       "malformed output no extraction error",
			mockOutput: []byte("Response without session identifier line"),
		},
		{
			name:       "empty output no extraction error",
			mockOutput: []byte(""),
		},
		{
			name:       "invalid session format no error",
			mockOutput: []byte("Session:\nNo ID following colon"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", nil)

			require.NoError(t, err, "extraction failure must not cause error (graceful fallback)")
			require.NotNil(t, result)
			assert.Empty(t, result.State.SessionID, "SessionID should be empty when extraction fails")
			assert.NotEmpty(t, result.Output, "output should still be populated")
		})
	}
}

func TestGeminiProvider_ExecuteConversation_EmptyOutputHandling(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(""), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	state := workflow.NewConversationState("system")
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output, "output should be normalized to at least space when empty")
}

func TestGeminiProvider_ExecuteConversation_InvalidPrompt(t *testing.T) {
	provider := NewGeminiProvider()
	state := workflow.NewConversationState("")

	_, err := provider.ExecuteConversation(context.Background(), state, "   ", nil)

	assert.Error(t, err)
}

func TestGeminiProvider_ExecuteConversation_NilState(t *testing.T) {
	provider := NewGeminiProvider()

	_, err := provider.ExecuteConversation(context.Background(), nil, "test", nil)

	assert.Error(t, err)
}
