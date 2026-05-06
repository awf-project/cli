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

// CopilotProvider unit tests covering Execute, ExecuteConversation, Name, Validate, and args building

func TestCopilotProvider_Name(t *testing.T) {
	provider := NewCopilotProvider()
	assert.Equal(t, "github_copilot", provider.Name())
}

func TestCopilotProvider_Execute_RawOutputFallback(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("plain text output"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "github_copilot", result.Provider)
	assert.Equal(t, "plain text output", result.Output)
	assert.True(t, result.TokensEstimated)
}

func TestCopilotProvider_Execute_WithOptions(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		options     map[string]any
		mockStdout  string
		wantCLIArgs []string
	}{
		{
			name:        "model option",
			prompt:      "test",
			options:     map[string]any{"model": "gpt-4o"},
			mockStdout:  `{"type":"assistant.message","data":{"content":"code","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantCLIArgs: []string{"-p", "test", "--output-format=json", "--silent", "--model=gpt-4o"},
		},
		{
			name:        "mode option",
			prompt:      "test",
			options:     map[string]any{"mode": "interactive"},
			mockStdout:  `{"type":"assistant.message","data":{"content":"code","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantCLIArgs: []string{"-p", "test", "--output-format=json", "--silent", "--mode=interactive"},
		},
		{
			name:        "effort option",
			prompt:      "test",
			options:     map[string]any{"effort": "high"},
			mockStdout:  `{"type":"assistant.message","data":{"content":"code","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantCLIArgs: []string{"-p", "test", "--output-format=json", "--silent", "--effort=high"},
		},
		{
			name:        "allowed_tools single",
			prompt:      "test",
			options:     map[string]any{"allowed_tools": []string{"browser"}},
			mockStdout:  `{"type":"assistant.message","data":{"content":"code","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantCLIArgs: []string{"-p", "test", "--output-format=json", "--silent", "--allow-tool=browser"},
		},
		{
			name:        "allowed_tools multiple",
			prompt:      "test",
			options:     map[string]any{"allowed_tools": []string{"browser", "python"}},
			mockStdout:  `{"type":"assistant.message","data":{"content":"code","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantCLIArgs: []string{"-p", "test", "--output-format=json", "--silent", "--allow-tool=browser", "--allow-tool=python"},
		},
		{
			name:        "denied_tools",
			prompt:      "test",
			options:     map[string]any{"denied_tools": []string{"bash"}},
			mockStdout:  `{"type":"assistant.message","data":{"content":"code","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantCLIArgs: []string{"-p", "test", "--output-format=json", "--silent", "--deny-tool=bash"},
		},
		{
			name:        "allow_all",
			prompt:      "test",
			options:     map[string]any{"allow_all": true},
			mockStdout:  `{"type":"assistant.message","data":{"content":"code","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantCLIArgs: []string{"-p", "test", "--output-format=json", "--silent", "--allow-all"},
		},
		{
			name:        "unknown options silently ignored",
			prompt:      "test",
			options:     map[string]any{"language": "python", "quiet": true},
			mockStdout:  `{"type":"assistant.message","data":{"content":"code","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantCLIArgs: []string{"-p", "test", "--output-format=json", "--silent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.mockStdout), nil)
			provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "copilot", calls[0].Name)
			// Check all expected args are present
			for _, arg := range tt.wantCLIArgs {
				assert.Contains(t, calls[0].Args, arg, "arg %s should be in CLI args", arg)
			}
		})
	}
}

func TestCopilotProvider_Execute_EmptyPrompt(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		wantErr string
	}{
		{
			name:    "empty string",
			prompt:  "",
			wantErr: "prompt cannot be empty",
		},
		{
			name:    "whitespace only",
			prompt:  "   \t\n  ",
			wantErr: "prompt cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
			provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, nil, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
			// Executor should not be called for empty prompt validation
			calls := mockExec.GetCalls()
			assert.Empty(t, calls)
		})
	}
}

func TestCopilotProvider_Execute_ContextCancellation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestCopilotProvider_Execute_ContextDeadline(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond)

	result, err := provider.Execute(ctx, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCopilotProvider_Execute_CLIError(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr string
	}{
		{
			name:    "command not found",
			mockErr: errors.New("command not found: copilot"),
			wantErr: "copilot execution failed",
		},
		{
			name:    "generic error",
			mockErr: errors.New("unknown error"),
			wantErr: "copilot execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCopilotProvider_ExecuteConversation_FirstTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"response\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	result, err := provider.ExecuteConversation(context.Background(), state, "first prompt", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "github_copilot", result.Provider)
	assert.Equal(t, "response", result.Output)
	assert.True(t, result.TokensEstimated)
}

func TestCopilotProvider_ExecuteConversation_ResumeTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"follow-up response\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.SessionID = "sess-abc123"

	result, err := provider.ExecuteConversation(context.Background(), state, "follow-up", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "follow-up response", result.Output)
	// Verify --resume flag was used
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args
	require.Contains(t, args, "--resume=sess-abc123")
}

func TestCopilotProvider_ExecuteConversation_FirstTurnWithSystemPrompt(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"response\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	result, err := provider.ExecuteConversation(
		context.Background(),
		state,
		"user prompt",
		map[string]any{"system_prompt": "You are a helper"},
		nil,
		nil,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Verify system prompt was embedded in the prompt
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args
	// Find the -p argument (should contain both system and user prompt)
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) {
			assert.Contains(t, args[i+1], "You are a helper")
			assert.Contains(t, args[i+1], "user prompt")
			break
		}
	}
}

func TestCopilotProvider_ExecuteConversation_StateCloning(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"response\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	originalState := workflow.NewConversationState("")
	originalState.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "initial"))

	result, err := provider.ExecuteConversation(context.Background(), originalState, "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Original state should be unchanged
	assert.Len(t, originalState.Turns, 1)
	assert.Equal(t, "initial", originalState.Turns[0].Content)
	// Result state should have new turns
	assert.Len(t, result.State.Turns, 3)
}

func TestCopilotProvider_ExecuteConversation_StateTurnsAppended(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"assistant response\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "first"))

	result, err := provider.ExecuteConversation(context.Background(), state, "second", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Should have original + 2 new turns (user + assistant)
	assert.Len(t, result.State.Turns, 3)
	assert.Equal(t, workflow.TurnRoleUser, result.State.Turns[1].Role)
	assert.Equal(t, "second", result.State.Turns[1].Content)
	assert.Equal(t, workflow.TurnRoleAssistant, result.State.Turns[2].Role)
	assert.Equal(t, "assistant response", result.State.Turns[2].Content)
}

func TestCopilotProvider_ExecuteConversation_NilState(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), nil, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "conversation state cannot be nil")
}

func TestCopilotProvider_ExecuteConversation_EmptyPrompt(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	result, err := provider.ExecuteConversation(context.Background(), state, "", nil, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prompt cannot be empty")
	assert.Nil(t, result)
}

func TestCopilotProvider_ExecuteConversation_ContextCancellation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := workflow.NewConversationState("")
	result, err := provider.ExecuteConversation(ctx, state, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestCopilotProvider_ExecuteConversation_CLIError(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetError(errors.New("command failed"))
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "copilot execution failed")
}

func TestCopilotProvider_Validate_BinaryMissing(t *testing.T) {
	t.Setenv("PATH", "")
	provider := NewCopilotProvider()

	err := provider.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "copilot CLI not found")
}

func TestCopilotProvider_NewCopilotProvider_DefaultExecutor(t *testing.T) {
	provider := NewCopilotProvider()
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.executor)
	_, ok := provider.executor.(*ExecCLIExecutor)
	assert.True(t, ok, "default executor should be ExecCLIExecutor")
}

func TestCopilotProvider_BuildExecuteArgs_MinimalInvocation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	_, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	// Should have: -p test --output-format=json --silent
	args := calls[0].Args
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "test")
	assert.Contains(t, args, "--output-format=json")
	assert.Contains(t, args, "--silent")
}

func TestCopilotProvider_BuildConversationArgs_FirstTurnNoSessionID(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	_, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args
	// First turn should use -p format, not --resume
	assert.NotContains(t, args, "--resume")
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "test")
}

func TestCopilotProvider_BuildConversationArgs_ResumeTurn(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.SessionID = "sess-xyz"
	_, err := provider.ExecuteConversation(context.Background(), state, "follow-up", nil, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args
	// Resume turn should use --resume=<id> format
	assert.Contains(t, args, "--resume=sess-xyz")
}

func TestCopilotProvider_AllowedToolsAsSliceAny(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	options := map[string]any{"allowed_tools": []any{"tool1", "tool2"}}
	_, err := provider.Execute(context.Background(), "test", options, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args
	assert.Contains(t, args, "--allow-tool=tool1")
	assert.Contains(t, args, "--allow-tool=tool2")
}

func TestCopilotProvider_DeniedToolsMultiple(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	options := map[string]any{"denied_tools": []string{"bash", "python"}}
	_, err := provider.Execute(context.Background(), "test", options, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args
	assert.Contains(t, args, "--deny-tool=bash")
	assert.Contains(t, args, "--deny-tool=python")
}

func TestCopilotProvider_AllowAllFalseOmitsFlag(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	options := map[string]any{"allow_all": false}
	_, err := provider.Execute(context.Background(), "test", options, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args
	assert.NotContains(t, args, "--allow-all")
}

func TestCopilotProvider_TokenEstimationOnExecute(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"hello world test\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	assert.True(t, result.TokensEstimated)
	assert.Greater(t, result.Tokens, 0)
}

func TestCopilotProvider_TokenEstimationOnExecuteConversation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"response text\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err)
	assert.True(t, result.TokensEstimated)
}
