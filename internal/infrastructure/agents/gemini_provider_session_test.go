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
// Tests for session resume logic in ExecuteConversation

func TestGeminiProvider_ExecuteConversation_ForcesStreamJsonFormat(t *testing.T) {
	tests := []struct {
		name              string
		userProvidedOpt   map[string]any
		shouldHaveFormat  bool
		expectedFormatVal string
	}{
		{
			name:              "no format option provided - stream-json forced",
			userProvidedOpt:   nil,
			shouldHaveFormat:  true,
			expectedFormatVal: "stream-json",
		},
		{
			name:              "user provides json format - converted to stream-json",
			userProvidedOpt:   map[string]any{"output_format": "json"},
			shouldHaveFormat:  true,
			expectedFormatVal: "stream-json",
		},
		{
			name:              "user provides stream-json - stays as stream-json",
			userProvidedOpt:   map[string]any{"output_format": "stream-json"},
			shouldHaveFormat:  true,
			expectedFormatVal: "stream-json",
		},
		{
			name:              "with other options - format still forced",
			userProvidedOpt:   map[string]any{"model": "gemini-pro", "dangerously_skip_permissions": true},
			shouldHaveFormat:  true,
			expectedFormatVal: "stream-json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"type":"init","session_id":"gemini-test-123"}`+"\nResponse text"), nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", tt.userProvidedOpt, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			found := false
			for i, arg := range calls[0].Args {
				if arg == "--output-format" && i+1 < len(calls[0].Args) {
					found = true
					assert.Equal(t, tt.expectedFormatVal, calls[0].Args[i+1])
				}
			}
			assert.True(t, found, "expected --output-format flag in args")
		})
	}
}

func TestGeminiProvider_ExecuteConversation_ExtractsSessionID(t *testing.T) {
	tests := []struct {
		name             string
		mockOutput       []byte
		wantSessionID    string
		expectExtraction bool
	}{
		{
			name:             "valid init event with session_id",
			mockOutput:       []byte(`{"type":"init","session_id":"031da63a-73be-42f5-ae0d-890aae0b6323"}` + "\nAssistant response"),
			wantSessionID:    "031da63a-73be-42f5-ae0d-890aae0b6323",
			expectExtraction: true,
		},
		{
			name:             "init event with other fields",
			mockOutput:       []byte(`{"type":"init","timestamp":"2026-04-07T21:55:36.994Z","session_id":"abc-def-123","model":"gemini-pro"}` + "\nResponse"),
			wantSessionID:    "abc-def-123",
			expectExtraction: true,
		},
		{
			name:             "init event without session_id field",
			mockOutput:       []byte(`{"type":"init","model":"gemini-pro"}` + "\nResponse"),
			wantSessionID:    "",
			expectExtraction: false,
		},
		{
			name:             "no init event in output",
			mockOutput:       []byte("Just plain text response without JSON"),
			wantSessionID:    "",
			expectExtraction: false,
		},
		{
			name:             "empty output",
			mockOutput:       []byte(""),
			wantSessionID:    "",
			expectExtraction: false,
		},
		{
			name:             "init event with null session_id",
			mockOutput:       []byte(`{"type":"init","session_id":null}` + "\nResponse"),
			wantSessionID:    "",
			expectExtraction: false,
		},
		{
			name:             "session_id in non-init event is ignored",
			mockOutput:       []byte(`{"type":"response","session_id":"wrong-id"}` + "\nResponse"),
			wantSessionID:    "",
			expectExtraction: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "prompt", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.State)

			if tt.expectExtraction {
				assert.Equal(t, tt.wantSessionID, result.State.SessionID, "session ID should be extracted from output")
			} else {
				assert.Empty(t, result.State.SessionID, "session ID should be empty when extraction fails")
			}
		})
	}
}

func TestGeminiProvider_ExecuteConversation_ResumeUsesRealSessionID(t *testing.T) {
	tests := []struct {
		name           string
		priorSessionID string
		wantResumeFlag bool
		expectedID     string
	}{
		{
			name:           "resume turn with real session UUID",
			priorSessionID: "031da63a-73be-42f5-ae0d-890aae0b6323",
			wantResumeFlag: true,
			expectedID:     "031da63a-73be-42f5-ae0d-890aae0b6323",
		},
		{
			name:           "first turn without session ID omits resume",
			priorSessionID: "",
			wantResumeFlag: false,
			expectedID:     "",
		},
		{
			name:           "resume with numeric string ID",
			priorSessionID: "12345",
			wantResumeFlag: true,
			expectedID:     "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"type":"init","session_id":"new-session-456"}`+"\nResponse"), nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("system")
			state.SessionID = tt.priorSessionID

			result, err := provider.ExecuteConversation(context.Background(), state, "continue prompt", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			found := false
			for i, arg := range calls[0].Args {
				if arg == "--resume" {
					found = true
					if tt.wantResumeFlag {
						require.Greater(t, len(calls[0].Args), i+1, "resume flag should have value")
						assert.Equal(t, tt.expectedID, calls[0].Args[i+1])
					}
				}
			}
			assert.Equal(t, tt.wantResumeFlag, found, "resume flag presence matches expectation")
		})
	}
}

func TestGeminiProvider_ExecuteConversation_UpdatesStateWithExtractedID(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	extractedID := "extracted-session-xyz"
	mockExec.SetOutput([]byte(`{"type":"init","session_id":"`+extractedID+`"}`+"\nGenerated response text"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	state := workflow.NewConversationState("system")
	initialID := ""
	state.SessionID = initialID

	result, err := provider.ExecuteConversation(context.Background(), state, "create poem", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.State)

	assert.NotEqual(t, initialID, result.State.SessionID, "session ID should be updated from extraction")
	assert.Equal(t, extractedID, result.State.SessionID)
}

func TestGeminiProvider_ExecuteConversation_NDJSONOutputPreserved(t *testing.T) {
	ndjsonOutput := `{"type":"init","session_id":"session-abc"}
{"type":"response","text":"Hello world"}
{"type":"status","code":"complete"}`

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjsonOutput), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	state := workflow.NewConversationState("system")
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, ndjsonOutput, result.Output, "raw NDJSON output should be preserved in result")
}

func TestGeminiProvider_ExecuteConversation_ConversationStateUpdated(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"init","session_id":"session-123"}`+"\nBot: Hello back"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	state := workflow.NewConversationState("system")
	prompt := "User: Hello"

	result, err := provider.ExecuteConversation(context.Background(), state, prompt, nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.State)

	assert.Greater(t, len(result.State.Turns), 0, "conversation should have at least assistant turn")
	assert.Equal(t, workflow.TurnRoleAssistant, result.State.Turns[len(result.State.Turns)-1].Role, "last turn should be assistant")
}

func TestGeminiProvider_ExecuteConversation_HandlesExtractionFailureGracefully(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("No JSON output at all - malformed response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	state := workflow.NewConversationState("system")
	state.SessionID = "prior-session"

	result, err := provider.ExecuteConversation(context.Background(), state, "prompt", nil, nil, nil)

	require.NoError(t, err, "execution should not fail when extraction fails")
	require.NotNil(t, result)
	assert.Empty(t, result.State.SessionID, "session ID should be cleared when extraction fails")
}
