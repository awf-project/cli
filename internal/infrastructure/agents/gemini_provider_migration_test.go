package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeminiProvider_Migration_Execute_HappyPath validates basic execution flow after migration to baseCLIProvider.
func TestGeminiProvider_Migration_Execute_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		mockStdout []byte
		wantOutput string
	}{
		{
			name:       "simple prompt",
			prompt:     "What is 2+2?",
			mockStdout: []byte("The answer is 4"),
			wantOutput: "The answer is 4",
		},
		{
			name:       "multiline response",
			prompt:     "explain",
			mockStdout: []byte("Line 1\nLine 2\nLine 3"),
			wantOutput: "Line 1\nLine 2\nLine 3",
		},
		{
			name:       "special characters",
			prompt:     "code",
			mockStdout: []byte("func() { return 42; }"),
			wantOutput: "func() { return 42; }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "gemini", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			assert.True(t, result.TokensEstimated, "TokensEstimated must be true")
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
		})
	}
}

// TestGeminiProvider_Migration_Execute_WithOptions validates option passing through hooks.
func TestGeminiProvider_Migration_Execute_WithOptions(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		options map[string]any
		want    string
	}{
		{
			name:    "with model option",
			prompt:  "test",
			options: map[string]any{"model": "gemini-pro"},
			want:    "response",
		},
		{
			name:    "with multiple options",
			prompt:  "test",
			options: map[string]any{"model": "gemini-1.5-pro", "temperature": 0.7},
			want:    "response",
		},
		{
			name:    "nil options",
			prompt:  "test",
			options: nil,
			want:    "response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.want), nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.want, result.Output)
		})
	}
}

// TestGeminiProvider_Migration_ExecuteConversation_HappyPath validates conversation flow after delegation to base.
func TestGeminiProvider_Migration_ExecuteConversation_HappyPath(t *testing.T) {
	state := &workflow.ConversationState{
		SessionID: "test-session-123",
		Turns: []workflow.Turn{
			{
				Role:    workflow.TurnRoleUser,
				Content: "Hello",
				Tokens:  5,
			},
		},
	}

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response from assistant"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), state, "continue", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "gemini", result.Provider)
	assert.Equal(t, "response from assistant", result.Output)
	assert.True(t, result.TokensEstimated)
	// Should have user turn + new user turn (from prompt) + assistant turn
	assert.Len(t, result.State.Turns, 3)
}

// TestGeminiProvider_Migration_ExecuteConversation_EmptyOutput validates empty output guard (FR-008).
func TestGeminiProvider_Migration_ExecuteConversation_EmptyOutput(t *testing.T) {
	state := &workflow.ConversationState{
		SessionID: "test-session",
		Turns:     []workflow.Turn{},
	}

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(""), nil) // Empty stdout
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, " ", result.Output, "Empty output must be normalized to single space per FR-008")
}

// TestGeminiProvider_Migration_Execute_EmptyPrompt validates prompt validation.
func TestGeminiProvider_Migration_Execute_EmptyPrompt(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt cannot be empty")
}

// TestGeminiProvider_Migration_Execute_WhitespacePrompt validates whitespace-only prompt.
func TestGeminiProvider_Migration_Execute_WhitespacePrompt(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "   \t\n  ", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt cannot be empty")
}

// TestGeminiProvider_Migration_Execute_ContextDeadlineExceeded validates timeout handling.
func TestGeminiProvider_Migration_Execute_ContextDeadlineExceeded(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond)

	result, err := provider.Execute(ctx, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

// TestGeminiProvider_Migration_Execute_ContextCanceled validates cancellation handling.
func TestGeminiProvider_Migration_Execute_ContextCanceled(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestGeminiProvider_Migration_Execute_CLIError validates CLI execution failures are wrapped correctly.
func TestGeminiProvider_Migration_Execute_CLIError(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr string
	}{
		{
			name:    "command not found",
			mockErr: errors.New("command not found"),
			wantErr: "execution failed",
		},
		{
			name:    "permission denied",
			mockErr: errors.New("permission denied"),
			wantErr: "execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestGeminiProvider_Migration_ExecuteConversation_InvalidState validates nil state handling.
func TestGeminiProvider_Migration_ExecuteConversation_InvalidState(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), nil, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestGeminiProvider_Migration_ExecuteConversation_CLIError validates error propagation in conversation.
func TestGeminiProvider_Migration_ExecuteConversation_CLIError(t *testing.T) {
	state := &workflow.ConversationState{
		SessionID: "test-session",
		Turns:     []workflow.Turn{},
	}

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetError(errors.New("execution failed"))
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "execution failed")
}

// TestGeminiProvider_Migration_BuildExecuteArgs validates hook method exists
// and forces stream-json NDJSON for consistent F082 display/text extraction.
func TestGeminiProvider_Migration_BuildExecuteArgs(t *testing.T) {
	provider := NewGeminiProvider()

	args, err := provider.buildExecuteArgs("test prompt", nil)

	assert.NoError(t, err)
	assert.Equal(t, []string{"--output-format", "stream-json", "-p", "test prompt"}, args)
}

// TestGeminiProvider_Migration_BuildConversationArgs validates conversation args hook:
// forces --output-format stream-json and uses --resume when a SessionID is present.
func TestGeminiProvider_Migration_BuildConversationArgs(t *testing.T) {
	provider := NewGeminiProvider()
	state := &workflow.ConversationState{
		SessionID: "test",
		Turns:     []workflow.Turn{},
	}

	args, err := provider.buildConversationArgs(state, "test", nil)

	assert.NoError(t, err)
	assert.Equal(t, []string{"--output-format", "stream-json", "--resume", "test", "-p", "test"}, args)
}

// TestGeminiProvider_Migration_ExtractSessionID_EmptyOutput validates empty output error.
func TestGeminiProvider_Migration_ExtractSessionID_EmptyOutput(t *testing.T) {
	provider := NewGeminiProvider()

	sessionID, err := provider.extractSessionID("")

	assert.Error(t, err)
	assert.Equal(t, "", sessionID)
	assert.Contains(t, err.Error(), "empty output")
}

// TestGeminiProvider_Migration_ExtractSessionID_ValidInitEvent validates valid NDJSON init event.
func TestGeminiProvider_Migration_ExtractSessionID_ValidInitEvent(t *testing.T) {
	provider := NewGeminiProvider()
	validOutput := `{"type":"init","timestamp":"2026-04-07T21:55:36.994Z","session_id":"test-session-123","model":"gemini-3"}`

	sessionID, err := provider.extractSessionID(validOutput)

	require.NoError(t, err)
	assert.Equal(t, "test-session-123", sessionID)
}

// TestGeminiProvider_Migration_ExtractSessionID_NoInitEvent validates missing init event.
func TestGeminiProvider_Migration_ExtractSessionID_NoInitEvent(t *testing.T) {
	provider := NewGeminiProvider()
	output := `{"type":"message","content":"hello"}`

	sessionID, err := provider.extractSessionID(output)

	assert.Error(t, err)
	assert.Equal(t, "", sessionID)
	assert.Contains(t, err.Error(), "init event not found")
}

// TestGeminiProvider_Migration_ExtractSessionID_InvalidJSON validates invalid JSON handling.
func TestGeminiProvider_Migration_ExtractSessionID_InvalidJSON(t *testing.T) {
	provider := NewGeminiProvider()

	sessionID, err := provider.extractSessionID(`{invalid json}`)

	assert.Error(t, err)
	assert.Equal(t, "", sessionID)
	assert.Contains(t, err.Error(), "init event not found")
}

// TestGeminiProvider_Migration_ExtractSessionID_MissingSessionField validates missing session_id field.
func TestGeminiProvider_Migration_ExtractSessionID_MissingSessionField(t *testing.T) {
	provider := NewGeminiProvider()
	output := `{"type":"init","timestamp":"2026-04-07T21:55:36.994Z"}`

	sessionID, err := provider.extractSessionID(output)

	assert.Error(t, err)
	assert.Equal(t, "", sessionID)
	assert.Contains(t, err.Error(), "session_id missing")
}

// TestGeminiProvider_Migration_ExtractSessionID_EmptySessionID validates empty session_id value.
func TestGeminiProvider_Migration_ExtractSessionID_EmptySessionID(t *testing.T) {
	provider := NewGeminiProvider()
	output := `{"type":"init","timestamp":"2026-04-07T21:55:36.994Z","session_id":""}`

	sessionID, err := provider.extractSessionID(output)

	assert.Error(t, err)
	assert.Equal(t, "", sessionID)
	assert.Contains(t, err.Error(), "session_id is empty")
}

// TestGeminiProvider_Migration_ExtractSessionID_InvalidSessionType validates non-string session_id.
func TestGeminiProvider_Migration_ExtractSessionID_InvalidSessionType(t *testing.T) {
	provider := NewGeminiProvider()
	output := `{"type":"init","session_id":123}`

	sessionID, err := provider.extractSessionID(output)

	assert.Error(t, err)
	assert.Equal(t, "", sessionID)
	assert.Contains(t, err.Error(), "session_id is not a string")
}

// TestGeminiProvider_Migration_NewBaseInitialization validates base delegation setup.
func TestGeminiProvider_Migration_NewBaseInitialization(t *testing.T) {
	provider := NewGeminiProvider()

	require.NotNil(t, provider.base)
	assert.NotNil(t, provider.base.hooks.buildExecuteArgs)
	assert.NotNil(t, provider.base.hooks.buildConversationArgs)
	assert.NotNil(t, provider.base.hooks.extractSessionID)
}

// TestGeminiProvider_Migration_WithExecutor validates functional option pattern.
func TestGeminiProvider_Migration_WithExecutor(t *testing.T) {
	customExec := mocks.NewMockCLIExecutor()
	customExec.SetOutput([]byte("custom response"), nil)

	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(customExec))
	require.NotNil(t, provider.executor)

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "custom response", result.Output)
}

// TestGeminiProvider_Migration_Execute_OutputWithStderr validates stderr combination.
func TestGeminiProvider_Migration_Execute_OutputWithStderr(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("stdout content"), []byte("stderr content"))
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "stdout contentstderr content", result.Output)
}

// TestGeminiProvider_Migration_Execute_JSONResponse validates JSON response parsing
// when the caller explicitly requests output_format: json (F082 intent routing).
func TestGeminiProvider_Migration_Execute_JSONResponse(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	jsonOutput := []byte(`{"result":"success","data":{"value":42}}`)
	mockExec.SetOutput(jsonOutput, nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", map[string]any{"output_format": "json"}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Response)
	assert.IsType(t, map[string]any{}, result.Response)
}

// TestGeminiProvider_Migration_Execute_TokensEstimated validates FR-007 fix (TokensEstimated true).
func TestGeminiProvider_Migration_Execute_TokensEstimated(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("This is sample output for token estimation"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.TokensEstimated, "FR-007: TokensEstimated must be true for all CLI providers")
}

// TestGeminiProvider_Migration_Execute_PreservesProviderField validates provider name in result.
func TestGeminiProvider_Migration_Execute_PreservesProviderField(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "gemini", result.Provider)
}
