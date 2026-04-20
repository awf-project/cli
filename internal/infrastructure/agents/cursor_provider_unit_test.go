package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCursorProvider_Execute_WithOptions(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"result","result":"Done"}`), nil)
	provider := NewCursorProviderWithOptions(WithCursorExecutor(mockExec))

	_, err := provider.Execute(context.Background(), "analyze this", map[string]any{
		"model":                        "composer-2",
		"mode":                         "plan",
		"sandbox":                      "enabled",
		"dangerously_skip_permissions": true,
	}, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "agent", calls[0].Name)
	assert.Equal(t, []string{
		"-p", "analyze this", "--output-format", "stream-json",
		"--model", "composer-2",
		"--mode", "plan",
		"--force",
		"--sandbox", "enabled",
	}, calls[0].Args)
}

func TestCursorProvider_Execute_JsonFormatSetsResponse(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"result","result":"Final","duration_ms":42}`), nil)
	provider := NewCursorProviderWithOptions(WithCursorExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "hello", map[string]any{
		"output_format": "json",
	}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "cursor", result.Provider)
	assert.Equal(t, "result", result.Response["type"])
	assert.Equal(t, "Final", result.Response["result"])
}

func TestCursorProvider_Execute_TextExtractsResultField(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"result","result":"Readable text"}`), nil)
	provider := NewCursorProviderWithOptions(WithCursorExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "hello", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Readable text", result.Output)
	assert.Equal(t, len("Readable text")/4, result.Tokens)
}

func TestCursorProvider_ExecuteConversation_UsesResumeWhenSessionExists(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"system","subtype":"init","chat_id":"chat-42"}`), nil)
	provider := NewCursorProviderWithOptions(WithCursorExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.SessionID = "chat-previous"

	_, err := provider.ExecuteConversation(context.Background(), state, "continue", nil, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Args, "--resume")
	assert.Contains(t, calls[0].Args, "chat-previous")
}

func TestCursorProvider_ExecuteConversation_InlinesSystemPromptOnFirstTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"type":"system","subtype":"init","chat_id":"chat-42"}`), nil)
	provider := NewCursorProviderWithOptions(WithCursorExecutor(mockExec))

	state := workflow.NewConversationState("")
	_, err := provider.ExecuteConversation(context.Background(), state, "User ask", map[string]any{
		"system_prompt": "You are strict",
	}, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "-p", calls[0].Args[0])
	assert.Equal(t, "You are strict\n\nUser ask", calls[0].Args[1])
}

func TestCursorProvider_ExtractSessionID(t *testing.T) {
	provider := NewCursorProvider()

	t.Run("chat_id", func(t *testing.T) {
		id, err := provider.extractSessionID(`{"type":"system","subtype":"init","chat_id":"chat-1"}`)
		require.NoError(t, err)
		assert.Equal(t, "chat-1", id)
	})

	t.Run("chatId", func(t *testing.T) {
		id, err := provider.extractSessionID(`{"type":"system","subtype":"init","chatId":"chat-2"}`)
		require.NoError(t, err)
		assert.Equal(t, "chat-2", id)
	})

	t.Run("missing", func(t *testing.T) {
		_, err := provider.extractSessionID(`{"type":"system","subtype":"init"}`)
		assert.Error(t, err)
	})
}

func TestCursorProvider_ParseCursorStreamLine(t *testing.T) {
	provider := NewCursorProvider()

	got := provider.parseCursorStreamLine([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"},{"type":"text","text":"World"}]}}`))
	assert.Equal(t, "Hello\nWorld", got)

	got = provider.parseCursorStreamLine([]byte(`{"type":"tool_call","subtype":"started"}`))
	assert.Equal(t, "", got)
}

func TestValidateCursorOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
	}{
		{name: "nil options", options: nil, wantErr: false},
		{name: "valid mode ask", options: map[string]any{"mode": "ask"}, wantErr: false},
		{name: "valid mode plan", options: map[string]any{"mode": "plan"}, wantErr: false},
		{name: "valid sandbox enabled", options: map[string]any{"sandbox": "enabled"}, wantErr: false},
		{name: "valid sandbox disabled", options: map[string]any{"sandbox": "disabled"}, wantErr: false},
		{name: "invalid mode", options: map[string]any{"mode": "agent"}, wantErr: true},
		{name: "invalid sandbox", options: map[string]any{"sandbox": "auto"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCursorOptions(tt.options)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
