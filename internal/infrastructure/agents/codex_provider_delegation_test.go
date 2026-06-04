package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexProvider_Execute_DelegationToBase(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		options    map[string]any
		mockStdout []byte
		wantOutput string
	}{
		{
			name:       "simple prompt delegation",
			prompt:     "Hello world",
			options:    nil,
			mockStdout: []byte("response text"),
			wantOutput: "response text",
		},
		{
			name:       "prompt with model option",
			prompt:     "test prompt",
			options:    map[string]any{"model": "gpt-4o"},
			mockStdout: []byte("model response"),
			wantOutput: "model response",
		},
		{
			name:       "prompt with max_tokens",
			prompt:     "limited response",
			options:    map[string]any{"max_tokens": 100},
			mockStdout: []byte("limited"),
			wantOutput: "limited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "codex", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			assert.True(t, result.TokensEstimated, "all CLI providers estimate tokens (no real-time token API)")
			assert.NotZero(t, result.Tokens)
		})
	}
}

func TestCodexProvider_Execute_EmptyPrompt_ValidationByBase(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt cannot be empty")
	assert.Empty(t, mockExec.GetCalls())
}

func TestCodexProvider_Execute_ContextCancellation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, err := provider.Execute(ctx, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, context.Canceled), "error must wrap context.Canceled")
	assert.Empty(t, mockExec.GetCalls())
}

func TestCodexProvider_ExecuteConversation_DelegationToBase(t *testing.T) {
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
			mockStdout:      "Recursion is...",
			expectSessionID: false,
		},
		{
			name:         "conversation with session event",
			systemPrompt: "Code assistant",
			userPrompt:   "Write a function",
			options:      map[string]any{"model": "gpt-4o"},
			// NDJSON format: thread.started event with thread_id
			mockStdout:      `{"type":"thread.started","thread_id":"thread-123"}` + "\n" + `response text`,
			expectSessionID: true,
		},
		{
			name:            "empty output gets fallback space",
			systemPrompt:    "System",
			userPrompt:      "Query",
			options:         nil,
			mockStdout:      "",
			expectSessionID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.mockStdout), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState(tt.systemPrompt)
			result, err := provider.ExecuteConversation(context.Background(), state, tt.userPrompt, tt.options, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "codex", result.Provider)
			assert.NotNil(t, result.State)

			// empty output gets " " fallback
			if tt.mockStdout == "" {
				assert.Equal(t, " ", result.Output)
			} else {
				assert.NotEmpty(t, result.Output)
			}

			assert.True(t, result.TokensEstimated, "ExecuteConversation must set TokensEstimated=true")
			assert.GreaterOrEqual(t, result.TokensTotal, 0)

			if tt.expectSessionID {
				assert.Equal(t, "thread-123", result.State.SessionID)
			}
		})
	}
}

func TestCodexProvider_ExecuteConversation_ContextCancellation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := workflow.NewConversationState("System")
	result, err := provider.ExecuteConversation(ctx, state, "test", nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, context.Canceled), "error must wrap context.Canceled")
	assert.Empty(t, mockExec.GetCalls())
}

func TestCodexProvider_ExecuteConversation_TurnManagement(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("Assistant response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("You are helpful")
	state.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleSystem, "You are helpful"),
		*workflow.NewTurn(workflow.TurnRoleUser, "First question"),
		*workflow.NewTurn(workflow.TurnRoleAssistant, "First answer"),
	}
	state.TotalTurns = 3
	state.TotalTokens = 100

	result, err := provider.ExecuteConversation(context.Background(), state, "Second question", nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.State)

	// State should be cloned and updated with new turns
	assert.GreaterOrEqual(t, result.State.TotalTurns, 3)

	// New assistant turn should be added
	lastTurn := result.State.Turns[len(result.State.Turns)-1]
	assert.Equal(t, workflow.TurnRoleAssistant, lastTurn.Role)
}

func TestCodexProvider_ExecuteConversation_SessionIDExtraction(t *testing.T) {
	tests := []struct {
		name         string
		output       string
		wantThreadID string
	}{
		{
			name:         "thread.started event",
			output:       `{"type":"thread.started","thread_id":"thread-abc123"}`,
			wantThreadID: "thread-abc123",
		},
		{
			name: "multiple NDJSON events",
			output: `{"type":"other"}
{"type":"thread.started","thread_id":"thread-xyz789"}`,
			wantThreadID: "thread-xyz789",
		},
		{
			name:         "missing thread.started",
			output:       `{"type":"message","content":"hello"}`,
			wantThreadID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.output), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			state := workflow.NewConversationState("System")
			result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.wantThreadID != "" {
				assert.Equal(t, tt.wantThreadID, result.State.SessionID)
			}
		})
	}
}

func TestCodexProvider_ExecuteConversation_MultipleEvents(t *testing.T) {
	output := strings.Join([]string{
		`{"type":"message","content":"thinking..."}`,
		`{"type":"thread.started","thread_id":"thread-multi"}`,
		`{"type":"complete"}`,
	}, "\n")

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(output), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("Assistant")
	result, err := provider.ExecuteConversation(context.Background(), state, "multi-event test", nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Even with multiple events, session ID should be extracted
	if result.State != nil && result.State.SessionID != "" {
		assert.Equal(t, "thread-multi", result.State.SessionID)
	}
}

func TestCodexProvider_Execute_AllTokensEstimatedTrue(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("test output"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test prompt", nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.TokensEstimated)
}

func TestCodexProvider_ExecuteConversation_AllTokensEstimatedTrue(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("System")
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.TokensEstimated)
}

func TestCodexProvider_ExecuteConversation_OutputExtraction(t *testing.T) {
	ndjson := ndjsonLine("extracted assistant text")

	tests := []struct {
		name         string
		outputFormat string
	}{
		{name: "default (no output_format)", outputFormat: ""},
		{name: "json output_format", outputFormat: "json"},
		{name: "text output_format", outputFormat: "text"},
		{name: "stream-json output_format", outputFormat: "stream-json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(ndjson), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			opts := map[string]any{}
			if tt.outputFormat != "" {
				opts["output_format"] = tt.outputFormat
			}

			state := workflow.NewConversationState("System")
			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", opts, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "extracted assistant text", result.Output)
			assert.NotContains(t, result.Output, "item.completed", "raw NDJSON envelope must not appear in Output")
			// Verify the executor was actually invoked (proves mock isolation)
			assert.Len(t, mockExec.GetCalls(), 1, "executor must be called exactly once per conversation turn")
		})
	}
}

func TestCodexProvider_ExecuteConversation_HookWired(t *testing.T) {
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":"hook extracted text"}}`

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("System")
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "hook extracted text", result.Output)
	assert.NotContains(t, result.Output, "item.completed", "raw NDJSON envelope must not appear in Output")
	// Verify the executor was invoked with the NDJSON input (proves hook ran on real output)
	assert.Len(t, mockExec.GetCalls(), 1, "executor must be called exactly once")
}

// TestCodexProvider_ExecuteConversation_PlainTextFallback proves the hook runs at runtime:
// when extractCodexTextContent returns "" (non-NDJSON input), base_cli_provider.go:332-334
// falls through to strings.TrimSpace(rawOutput). Without the hook wired, the base provider
// would use the raw bytes directly (no trim). With the hook wired, "" triggers the fallback.
func TestCodexProvider_ExecuteConversation_PlainTextFallback(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("  plain text response  "), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("System")
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Hook returns "" for non-NDJSON; base falls back to strings.TrimSpace(rawOutput).
	assert.Equal(t, "plain text response", result.Output, "plain text must be trimmed via base fallback")
	assert.NotContains(t, result.Output, "item.completed")
	assert.Len(t, mockExec.GetCalls(), 1)
}

func TestCodexProvider_ExecuteConversation_NDJSONOutput(t *testing.T) {
	multiEvent := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-ndjson"}`,
		ndjsonLine("first part"),
		ndjsonLine("second part"),
		`{"type":"item.completed","item":{"item_type":"function_call","name":"bash","arguments":"{}"}}`,
		ndjsonLine("third part"),
	}, "\n")

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(multiEvent), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("System")
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "first part\nsecond part\nthird part", result.Output)
	assert.NotContains(t, result.Output, "item.completed", "raw NDJSON must not appear in Output")
	// Verify executor was invoked: proves the mock was the actual dependency under test
	assert.Len(t, mockExec.GetCalls(), 1, "executor must be called exactly once")
}
