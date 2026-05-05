package agents

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T004: Copilot provider delegation tests verify that CopilotProvider correctly
// delegates Execute and ExecuteConversation to baseCLIProvider through hooks.
// Tests fail against stub (buildCopilotExecuteArgs/buildCopilotConversationArgs return nil)
// and pass after implementation provides proper CLI arguments.

func TestCopilotProvider_Execute_DelegationToBase(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		options    map[string]any
		mockStdout string
		wantOutput string
	}{
		{
			name:       "simple prompt delegation",
			prompt:     "Hello world",
			options:    nil,
			mockStdout: `{"type":"assistant.message","data":{"content":"response text","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantOutput: "response text",
		},
		{
			name:       "prompt with model option delegation",
			prompt:     "test prompt",
			options:    map[string]any{"model": "gpt-4o"},
			mockStdout: `{"type":"assistant.message","data":{"content":"model response","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantOutput: "model response",
		},
		{
			name:       "prompt with multiple options",
			prompt:     "create function",
			options:    map[string]any{"model": "gpt-4", "mode": "plan"},
			mockStdout: `{"type":"assistant.message","data":{"content":"function code","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			wantOutput: "function code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.mockStdout), nil)
			provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "github_copilot", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			assert.True(t, result.TokensEstimated, "Execute must set TokensEstimated=true")
		})
	}
}

func TestCopilotProvider_Execute_EmptyPrompt_ValidationByBase(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
	// Base validates before calling hook
	calls := mockExec.GetCalls()
	assert.Empty(t, calls)
}

func TestCopilotProvider_Execute_ContextCancellation_DelegationByBase(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	// Base checks context before hook execution
	calls := mockExec.GetCalls()
	assert.Empty(t, calls)
}

func TestCopilotProvider_ExecuteConversation_DelegationToBase(t *testing.T) {
	tests := []struct {
		name            string
		systemPrompt    string
		userPrompt      string
		options         map[string]any
		mockStdout      string
		expectSessionID bool
	}{
		{
			name:            "simple conversation turn",
			systemPrompt:    "You are helpful",
			userPrompt:      "What is recursion?",
			options:         nil,
			mockStdout:      `{"type":"assistant.message","data":{"content":"Recursion is...","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			expectSessionID: false,
		},
		{
			name:            "conversation with session event",
			systemPrompt:    "Code assistant",
			userPrompt:      "Write a function",
			options:         map[string]any{"model": "gpt-4o"},
			mockStdout:      `{"type":"assistant.message","data":{"content":"response","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"sess-abc","exitCode":0}`,
			expectSessionID: true,
		},
		{
			name:            "conversation with options",
			systemPrompt:    "System",
			userPrompt:      "Query",
			options:         map[string]any{"mode": "autopilot", "effort": "high"},
			mockStdout:      `{"type":"assistant.message","data":{"content":"answer","messageId":"m1"}}` + "\n" + `{"type":"result","sessionId":"s1","exitCode":0}`,
			expectSessionID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.mockStdout), nil)
			provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

			state := workflow.NewConversationState(tt.systemPrompt)
			result, err := provider.ExecuteConversation(context.Background(), state, tt.userPrompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "github_copilot", result.Provider)
			assert.NotNil(t, result.State)
		})
	}
}

func TestCopilotProvider_ExecuteConversation_FirstTurn_ArgsStructure(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	_, err := provider.ExecuteConversation(context.Background(), state, "first prompt", nil, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args

	// First turn should have: -p <prompt> --output-format=json --silent
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "first prompt")
	assert.Contains(t, args, "--output-format=json")
	assert.Contains(t, args, "--silent")
	assert.NotContains(t, args, "--resume")
}

func TestCopilotProvider_ExecuteConversation_ResumeTurn_ArgsStructure(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"code\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.SessionID = "sess-123"
	_, err := provider.ExecuteConversation(context.Background(), state, "follow-up", nil, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args

	// Resume turn should have: --resume=<id> -p <prompt> --output-format=json --silent
	assert.Contains(t, args, "--resume=sess-123")
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "follow-up")
	assert.Contains(t, args, "--output-format=json")
	assert.Contains(t, args, "--silent")
}

func TestCopilotProvider_ExecuteConversation_FirstTurn_SystemPromptInlined(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"response\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	_, err := provider.ExecuteConversation(
		context.Background(),
		state,
		"user query",
		map[string]any{"system_prompt": "You are an AI"},
		nil,
		nil,
	)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args

	// System prompt should be inlined in the first prompt argument
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) {
			prompt := args[i+1]
			assert.Contains(t, prompt, "You are an AI")
			assert.Contains(t, prompt, "user query")
			break
		}
	}
}

func TestCopilotProvider_ExecuteConversation_ResumeTurn_SystemPromptNotInlined(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"response\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.SessionID = "sess-abc"
	_, err := provider.ExecuteConversation(
		context.Background(),
		state,
		"follow-up",
		map[string]any{"system_prompt": "You are an AI"},
		nil,
		nil,
	)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args

	// On resume turn, system_prompt is silently ignored (preserved for future use)
	// The prompt should just be the user's follow-up
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) {
			prompt := args[i+1]
			// Should be just the follow-up, not the system prompt
			assert.Equal(t, "follow-up", prompt)
			break
		}
	}
}

func TestCopilotProvider_ExecuteConversation_EmptyPrompt_ValidationByBase(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	result, err := provider.ExecuteConversation(context.Background(), state, "", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
}

func TestCopilotProvider_ExecuteConversation_NilState_ValidationByBase(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), nil, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "conversation state")
}

func TestCopilotProvider_ExecuteConversation_ContextCancellation_DelegationByBase(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := workflow.NewConversationState("")
	result, err := provider.ExecuteConversation(ctx, state, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	// Base checks context before hook execution
	calls := mockExec.GetCalls()
	assert.Empty(t, calls)
}

func TestCopilotProvider_ExecuteConversation_StatePreservation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"response\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	originalState := workflow.NewConversationState("")
	originalState.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "initial"))
	originalLen := len(originalState.Turns)

	result, err := provider.ExecuteConversation(context.Background(), originalState, "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Original state should be unchanged
	assert.Len(t, originalState.Turns, originalLen)
	assert.Equal(t, "initial", originalState.Turns[0].Content)

	// Result state should be a clone with new turns
	assert.NotEqual(t, originalState, result.State)
	assert.Len(t, result.State.Turns, originalLen+2) // +2 for user and assistant
}

func TestCopilotProvider_ExecuteConversation_TurnsAppendedCorrectly(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"assistant output\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
	provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "first"))
	state.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "first response"))

	result, err := provider.ExecuteConversation(context.Background(), state, "second", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have 4 turns: first 2 + new user + new assistant
	assert.Len(t, result.State.Turns, 4)

	// Verify new turns
	assert.Equal(t, workflow.TurnRoleUser, result.State.Turns[2].Role)
	assert.Equal(t, "second", result.State.Turns[2].Content)
	assert.Equal(t, workflow.TurnRoleAssistant, result.State.Turns[3].Role)
	assert.Equal(t, "assistant output", result.State.Turns[3].Content)
}

func TestCopilotProvider_ExecuteConversation_WithOptions_ArgsConstruction(t *testing.T) {
	tests := []struct {
		name        string
		options     map[string]any
		wantArgList []string
	}{
		{
			name:        "model option",
			options:     map[string]any{"model": "gpt-4o"},
			wantArgList: []string{"--model=gpt-4o"},
		},
		{
			name:        "mode option",
			options:     map[string]any{"mode": "interactive"},
			wantArgList: []string{"--mode=interactive"},
		},
		{
			name:        "effort option",
			options:     map[string]any{"effort": "high"},
			wantArgList: []string{"--effort=high"},
		},
		{
			name:        "multiple options",
			options:     map[string]any{"model": "gpt-4", "mode": "plan"},
			wantArgList: []string{"--model=gpt-4", "--mode=plan"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("{\"type\":\"assistant.message\",\"data\":{\"content\":\"response\",\"messageId\":\"m1\"}}\n{\"type\":\"result\",\"sessionId\":\"s1\",\"exitCode\":0}"), nil)
			provider := NewCopilotProviderWithOptions(WithCopilotExecutor(mockExec))

			state := workflow.NewConversationState("")
			_, err := provider.ExecuteConversation(context.Background(), state, "test", tt.options, nil, nil)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			for _, wantArg := range tt.wantArgList {
				assert.Contains(t, calls[0].Args, wantArg)
			}
		})
	}
}
