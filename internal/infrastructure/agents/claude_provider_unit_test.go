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

// Component: C025 - Unit Tests for ClaudeProvider (WITHOUT integration build tag)
// These tests use MockCLIExecutor to avoid external CLI dependencies

func TestClaudeProvider_Execute_Success(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		options    map[string]any
		mockStdout []byte
		wantOutput string
	}{
		{
			name:       "simple prompt",
			prompt:     "What is 2+2?",
			mockStdout: []byte("4"),
			wantOutput: "4",
		},
		{
			name:       "prompt with model option",
			prompt:     "test",
			options:    map[string]any{"model": "sonnet"},
			mockStdout: []byte("response"),
			wantOutput: "response",
		},
		{
			name:       "prompt with json output format",
			prompt:     "list colors",
			options:    map[string]any{"output_format": "json"},
			mockStdout: []byte(`{"colors":["red","blue","green"]}`),
			wantOutput: `{"colors":["red","blue","green"]}`,
		},
		{
			name:       "prompt with allowed tools",
			prompt:     "test",
			options:    map[string]any{"allowed_tools": "bash,read"},
			mockStdout: []byte("ok"),
			wantOutput: "ok",
		},
		{
			name:       "large output",
			prompt:     "generate",
			mockStdout: []byte("This is a very long output " + string(make([]byte, 1000))),
			wantOutput: "This is a very long output " + string(make([]byte, 1000)),
		},
		{
			name:       "empty output",
			prompt:     "silent",
			mockStdout: []byte(""),
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "claude", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			// Token estimation is ~4 chars per token, so verify it's in reasonable range
			expectedTokens := len(tt.wantOutput) / 4
			assert.Equal(t, expectedTokens, result.Tokens)
			assert.True(t, result.TokensEstimated)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
			assert.True(t, result.CompletedAt.After(result.StartedAt))
		})
	}
}

func TestClaudeProvider_Execute_JSONParsing(t *testing.T) {
	tests := []struct {
		name       string
		mockStdout []byte
		options    map[string]any
		wantJSON   bool
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid NDJSON result event",
			mockStdout: []byte(`{"type":"system","subtype":"init"}` + "\n" + `{"type":"result","subtype":"success","result":"ok","session_id":"s1"}`),
			options:    map[string]any{"output_format": "json"},
			wantJSON:   true,
			wantErr:    false,
		},
		{
			name:       "malformed NDJSON yields nil Response (graceful)",
			mockStdout: []byte(`{"result":"incomplete`),
			options:    map[string]any{"output_format": "json"},
			wantJSON:   false,
			wantErr:    false,
		},
		{
			name:       "non-json response with json format yields nil Response (graceful)",
			mockStdout: []byte("plain text response"),
			options:    map[string]any{"output_format": "json"},
			wantJSON:   false,
			wantErr:    false,
		},
		{
			name:       "no explicit format option does not populate Response",
			mockStdout: []byte(`{"type":"result","subtype":"success","result":"ok","session_id":"s2"}`),
			options:    nil,
			wantJSON:   false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", tt.options, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.wantJSON {
					assert.NotNil(t, result.Response)
					assert.IsType(t, map[string]any{}, result.Response)
				} else {
					assert.Nil(t, result.Response)
				}
			}
		})
	}
}

func TestClaudeProvider_Execute_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		options map[string]any
		wantErr string
	}{
		{
			name:    "empty prompt",
			prompt:  "",
			wantErr: "prompt cannot be empty",
		},
		{
			name:    "whitespace-only prompt",
			prompt:  "   \t\n  ",
			wantErr: "prompt cannot be empty",
		},
		{
			name:    "invalid model format",
			prompt:  "test",
			options: map[string]any{"model": "invalid-model"},
			wantErr: "invalid model format",
		},
		{
			name:    "valid model alias",
			prompt:  "test",
			options: map[string]any{"model": "sonnet"},
			wantErr: "",
		},
		{
			name:    "valid claude model",
			prompt:  "test",
			options: map[string]any{"model": "claude-3-opus-20240229"},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestClaudeProvider_Execute_ContextErrors(t *testing.T) {
	tests := []struct {
		name    string
		ctxFunc func() context.Context
		wantErr string
	}{
		{
			name: "context deadline exceeded",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()
				time.Sleep(10 * time.Millisecond)
				return ctx
			},
			wantErr: "context deadline exceeded",
		},
		{
			name: "context canceled",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			ctx := tt.ctxFunc()
			result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestClaudeProvider_Execute_CLIErrors(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr string
	}{
		{
			name:    "cli execution error",
			mockErr: errors.New("command not found"),
			wantErr: "claude execution failed",
		},
		{
			name:    "permission denied",
			mockErr: errors.New("permission denied"),
			wantErr: "claude execution failed",
		},
		{
			name:    "timeout error",
			mockErr: context.DeadlineExceeded,
			wantErr: "claude execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", nil, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestClaudeProvider_Execute_StdoutStderrCombination(t *testing.T) {
	tests := []struct {
		name       string
		stdout     []byte
		stderr     []byte
		wantOutput string
	}{
		{
			name:       "stdout only",
			stdout:     []byte("stdout content"),
			stderr:     nil,
			wantOutput: "stdout content",
		},
		{
			name:       "stderr only",
			stdout:     nil,
			stderr:     []byte("stderr content"),
			wantOutput: "stderr content",
		},
		{
			name:       "both stdout and stderr",
			stdout:     []byte("stdout "),
			stderr:     []byte("stderr"),
			wantOutput: "stdout stderr",
		},
		{
			name:       "both empty",
			stdout:     []byte{},
			stderr:     []byte{},
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.stdout, tt.stderr)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantOutput, result.Output)
		})
	}
}

func TestClaudeProvider_Execute_CLIArgumentConstruction(t *testing.T) {
	tests := []struct {
		name         string
		prompt       string
		options      map[string]any
		mockOutput   []byte
		wantContains []string
	}{
		{
			name:         "basic prompt",
			prompt:       "test prompt",
			options:      nil,
			mockOutput:   []byte("ok"),
			wantContains: []string{"-p", "test prompt"},
		},
		{
			name:         "with model",
			prompt:       "test",
			options:      map[string]any{"model": "opus"},
			mockOutput:   []byte("ok"),
			wantContains: []string{"-p", "test", "--model", "opus"},
		},
		{
			name:         "with json format",
			prompt:       "test",
			options:      map[string]any{"output_format": "json"},
			mockOutput:   []byte(`{"result":"ok"}`),
			wantContains: []string{"-p", "test", "--output-format", "stream-json"},
		},
		{
			name:         "with allowed tools",
			prompt:       "test",
			options:      map[string]any{"allowed_tools": "bash,read"},
			mockOutput:   []byte("ok"),
			wantContains: []string{"-p", "test", "--allowedTools", "bash,read"},
		},
		{
			name:         "with dangerous skip permissions",
			prompt:       "test",
			options:      map[string]any{"dangerously_skip_permissions": true},
			mockOutput:   []byte("ok"),
			wantContains: []string{"-p", "test", "--dangerously-skip-permissions"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "claude", calls[0].Name)

			// Verify all expected arguments are present
			for _, want := range tt.wantContains {
				assert.Contains(t, calls[0].Args, want)
			}
		})
	}
}

func TestClaudeProvider_ExecuteConversation_Success(t *testing.T) {
	tests := []struct {
		name         string
		prompt       string
		options      map[string]any
		stateSetup   func() *workflow.ConversationState
		mockStdout   []byte
		wantOutput   string
		minTurns     int
		checkHistory bool
	}{
		{
			name:   "new conversation",
			prompt: "Hello",
			stateSetup: func() *workflow.ConversationState {
				return workflow.NewConversationState("You are helpful")
			},
			mockStdout:   []byte("Hi there!"),
			wantOutput:   "Hi there!",
			minTurns:     2, // system + user + assistant
			checkHistory: true,
		},
		{
			name:   "existing conversation",
			prompt: "What about 3+3?",
			stateSetup: func() *workflow.ConversationState {
				state := workflow.NewConversationState("You are helpful")
				state.Turns = []workflow.Turn{
					*workflow.NewTurn(workflow.TurnRoleSystem, "You are helpful"),
					*workflow.NewTurn(workflow.TurnRoleUser, "What is 2+2?"),
					*workflow.NewTurn(workflow.TurnRoleAssistant, "4"),
				}
				state.TotalTurns = 3
				state.TotalTokens = 50
				return state
			},
			mockStdout:   []byte("6"),
			wantOutput:   "6",
			minTurns:     5,
			checkHistory: true,
		},
		{
			name:   "with json format",
			prompt: "list colors",
			stateSetup: func() *workflow.ConversationState {
				return workflow.NewConversationState("You are helpful")
			},
			options:      map[string]any{"output_format": "json"},
			mockStdout:   []byte(`{"colors":["red","blue"]}`),
			wantOutput:   `{"colors":["red","blue"]}`,
			minTurns:     2,
			checkHistory: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			state := tt.stateSetup()
			result, err := provider.ExecuteConversation(context.Background(), state, tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "claude", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			assert.NotNil(t, result.State)
			assert.GreaterOrEqual(t, result.State.TotalTurns, tt.minTurns)
			assert.True(t, result.TokensEstimated)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())

			if tt.checkHistory {
				// Verify user turn was added
				found := false
				for _, turn := range result.State.Turns {
					if turn.Role == workflow.TurnRoleUser && turn.Content == tt.prompt {
						found = true
						break
					}
				}
				assert.True(t, found, "user turn not found in history")
			}
		})
	}
}

func TestClaudeProvider_ExecuteConversation_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		state   *workflow.ConversationState
		prompt  string
		options map[string]any
		wantErr string
	}{
		{
			name:    "nil state",
			state:   nil,
			prompt:  "test",
			wantErr: "conversation state cannot be nil",
		},
		{
			name:    "empty prompt",
			state:   workflow.NewConversationState("system"),
			prompt:  "",
			wantErr: "prompt cannot be empty",
		},
		{
			name:    "whitespace-only prompt",
			state:   workflow.NewConversationState("system"),
			prompt:  "   \t\n  ",
			wantErr: "prompt cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			result, err := provider.ExecuteConversation(context.Background(), tt.state, tt.prompt, tt.options, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestClaudeProvider_ExecuteConversation_ContextErrors(t *testing.T) {
	tests := []struct {
		name    string
		ctxFunc func() context.Context
		wantErr string
	}{
		{
			name: "context deadline exceeded",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()
				time.Sleep(10 * time.Millisecond)
				return ctx
			},
			wantErr: "context deadline exceeded",
		},
		{
			name: "context canceled",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			state := workflow.NewConversationState("system")
			ctx := tt.ctxFunc()
			result, err := provider.ExecuteConversation(ctx, state, "test", nil, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestClaudeProvider_ExecuteConversation_CLIErrors(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr string
	}{
		{
			name:    "cli execution error",
			mockErr: errors.New("command not found"),
			wantErr: "claude execution failed",
		},
		{
			name:    "permission denied",
			mockErr: errors.New("permission denied"),
			wantErr: "claude execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestClaudeProvider_ExecuteConversation_StatePreservation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	initialState := workflow.NewConversationState("You are helpful")
	initialState.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleSystem, "You are helpful"),
		*workflow.NewTurn(workflow.TurnRoleUser, "Hello"),
		*workflow.NewTurn(workflow.TurnRoleAssistant, "Hi"),
	}
	initialState.TotalTurns = 3
	initialState.TotalTokens = 50

	// Store initial values
	initialTurnCount := len(initialState.Turns)
	initialTotalTurns := initialState.TotalTurns
	initialTotalTokens := initialState.TotalTokens

	result, err := provider.ExecuteConversation(context.Background(), initialState, "How are you?", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.State)

	// Original state should NOT be modified (cloned)
	assert.Equal(t, initialTurnCount, len(initialState.Turns))
	assert.Equal(t, initialTotalTurns, initialState.TotalTurns)
	assert.Equal(t, initialTotalTokens, initialState.TotalTokens)

	// Result state should have new turns
	assert.Greater(t, result.State.TotalTurns, initialState.TotalTurns)
	assert.Greater(t, len(result.State.Turns), len(initialState.Turns))
}

func TestClaudeProvider_ExecuteConversation_TokenCounting(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("This is a response with multiple words"), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	state := workflow.NewConversationState("You are helpful")
	result, err := provider.ExecuteConversation(context.Background(), state, "Hello", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.TokensTotal, 0)
	assert.Equal(t, result.TokensInput+result.TokensOutput, result.TokensTotal)
	assert.True(t, result.TokensEstimated)
}

func TestClaudeProvider_ExecuteConversation_JSONParsing(t *testing.T) {
	tests := []struct {
		name       string
		mockStdout []byte
		options    map[string]any
		wantJSON   bool
		wantErr    bool
	}{
		{
			name:       "valid NDJSON result event populates Response",
			mockStdout: []byte(`{"type":"result","subtype":"success","result":"ok","session_id":"s1"}`),
			options:    map[string]any{"output_format": "json"},
			wantJSON:   true,
			wantErr:    false,
		},
		{
			name:       "malformed NDJSON yields nil Response without error (graceful)",
			mockStdout: []byte(`{"result":"incomplete`),
			options:    map[string]any{"output_format": "json"},
			wantJSON:   false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test", tt.options, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.wantJSON {
					assert.NotNil(t, result.Response)
				}
			}
		})
	}
}

func TestClaudeProvider_ExecuteConversation_GracefulFallback(t *testing.T) {
	tests := []struct {
		name        string
		mockStdout  []byte
		options     map[string]any
		wantErr     bool
		wantOutput  string
		wantEmptyID bool
	}{
		{
			name:        "non-json output (plain text)",
			mockStdout:  []byte("This is plain text response"),
			options:     map[string]any{},
			wantErr:     false,
			wantOutput:  "This is plain text response",
			wantEmptyID: true,
		},
		{
			name:        "malformed json with no user option",
			mockStdout:  []byte(`{"result":"incomplete`),
			options:     map[string]any{},
			wantErr:     false,
			wantOutput:  `{"result":"incomplete`,
			wantEmptyID: true,
		},
		{
			name:        "json missing result field",
			mockStdout:  []byte(`{"session_id":"12345","other":"data"}`),
			options:     map[string]any{},
			wantErr:     false,
			wantOutput:  `{"session_id":"12345","other":"data"}`,
			wantEmptyID: false,
		},
		{
			name:        "empty output",
			mockStdout:  []byte(""),
			options:     map[string]any{},
			wantErr:     false,
			wantOutput:  "",
			wantEmptyID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test", tt.options, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.wantOutput, result.Output)

				if tt.wantEmptyID {
					assert.Empty(t, result.State.SessionID)
				}
			}
		})
	}
}

func TestClaudeProvider_Name(t *testing.T) {
	provider := NewClaudeProvider()
	assert.Equal(t, "claude", provider.Name())
}

func TestClaudeProvider_Execute_EmptyState(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	state := &workflow.ConversationState{}
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.State)
}

func TestClaudeProvider_Execute_OptionsNil(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "response", result.Output)
}

func TestClaudeProvider_Execute_MultipleOptions(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

	options := map[string]any{
		"model":         "sonnet",
		"output_format": "text",
		"allowed_tools": "bash",
	}

	result, err := provider.Execute(context.Background(), "test", options, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify CLI was called with correct arguments
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "claude", calls[0].Name)
	assert.Contains(t, calls[0].Args, "--model")
	assert.Contains(t, calls[0].Args, "sonnet")
}

func TestClaudeProvider_ExecuteConversation_CLIArgumentConstruction(t *testing.T) {
	tests := []struct {
		name         string
		prompt       string
		options      map[string]any
		mockOutput   []byte
		wantContains []string
	}{
		{
			name:         "basic conversation prompt",
			prompt:       "Hello",
			options:      nil,
			mockOutput:   []byte("Hi there!"),
			wantContains: []string{"-p", "Hello", "--output-format", "stream-json"},
		},
		{
			name:         "with model",
			prompt:       "test",
			options:      map[string]any{"model": "opus"},
			mockOutput:   []byte("response"),
			wantContains: []string{"-p", "test", "--model", "opus", "--output-format", "stream-json"},
		},
		{
			name:         "with allowed tools",
			prompt:       "test",
			options:      map[string]any{"allowed_tools": "bash,read"},
			mockOutput:   []byte("response"),
			wantContains: []string{"-p", "test", "--allowedTools", "bash,read", "--output-format", "stream-json"},
		},
		{
			name:         "with dangerous skip permissions",
			prompt:       "test",
			options:      map[string]any{"dangerously_skip_permissions": true},
			mockOutput:   []byte("response"),
			wantContains: []string{"-p", "test", "--dangerously-skip-permissions", "--output-format", "stream-json"},
		},
		{
			name:   "with all options",
			prompt: "test",
			options: map[string]any{
				"model":                        "sonnet",
				"allowed_tools":                "bash",
				"dangerously_skip_permissions": true,
			},
			mockOutput: []byte("response"),
			wantContains: []string{
				"-p", "test",
				"--model", "sonnet",
				"--allowedTools", "bash",
				"--dangerously-skip-permissions",
				"--output-format", "stream-json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			state := workflow.NewConversationState("You are helpful")
			result, err := provider.ExecuteConversation(context.Background(), state, tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "claude", calls[0].Name)

			for _, want := range tt.wantContains {
				assert.Contains(t, calls[0].Args, want)
			}
		})
	}
}

func TestClaudeProvider_Execute_AllowedToolsOption(t *testing.T) {
	tests := []struct {
		name         string
		options      map[string]any
		wantContains []string
	}{
		{
			name:         "without allowed_tools",
			options:      nil,
			wantContains: []string{"-p", "prompt"},
		},
		{
			name:         "with allowed_tools",
			options:      map[string]any{"allowed_tools": "bash,cat,grep"},
			wantContains: []string{"-p", "prompt", "--allowedTools", "bash,cat,grep"},
		},
		{
			name:         "with empty allowed_tools",
			options:      map[string]any{"allowed_tools": ""},
			wantContains: []string{"-p", "prompt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "prompt", tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			for _, want := range tt.wantContains {
				assert.Contains(t, calls[0].Args, want)
			}
		})
	}
}

func TestClaudeProvider_Execute_DangerouslySkipPermissionsOption(t *testing.T) {
	tests := []struct {
		name         string
		options      map[string]any
		wantContains []string
	}{
		{
			name:         "without dangerously_skip_permissions",
			options:      nil,
			wantContains: []string{"-p", "prompt"},
		},
		{
			name:         "with dangerously_skip_permissions true",
			options:      map[string]any{"dangerously_skip_permissions": true},
			wantContains: []string{"-p", "prompt", "--dangerously-skip-permissions"},
		},
		{
			name:         "with dangerously_skip_permissions false",
			options:      map[string]any{"dangerously_skip_permissions": false},
			wantContains: []string{"-p", "prompt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "prompt", tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			for _, want := range tt.wantContains {
				assert.Contains(t, calls[0].Args, want)
			}
		})
	}
}

func TestClaudeProvider_ExecuteConversation_AllowedToolsOption(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
	}{
		{
			name:    "without allowed_tools",
			options: nil,
			wantErr: false,
		},
		{
			name:    "with allowed_tools",
			options: map[string]any{"allowed_tools": "bash,cat"},
			wantErr: false,
		},
		{
			name:    "with empty allowed_tools",
			options: map[string]any{"allowed_tools": ""},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			state := workflow.NewConversationState("You are helpful")
			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", tt.options, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "claude", result.Provider)
			}
		})
	}
}

func TestClaudeProvider_ExecuteConversation_DangerouslySkipPermissionsOption(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
	}{
		{
			name:    "without dangerously_skip_permissions",
			options: nil,
			wantErr: false,
		},
		{
			name:    "with dangerously_skip_permissions true",
			options: map[string]any{"dangerously_skip_permissions": true},
			wantErr: false,
		},
		{
			name:    "with dangerously_skip_permissions false",
			options: map[string]any{"dangerously_skip_permissions": false},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			state := workflow.NewConversationState("You are helpful")
			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", tt.options, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "claude", result.Provider)
			}
		})
	}
}

func TestClaudeProvider_OldCamelCaseKeysIgnored(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name:    "old allowedTools key is ignored",
			options: map[string]any{"allowedTools": "bash"},
		},
		{
			name:    "old dangerouslySkipPermissions key is ignored",
			options: map[string]any{"dangerouslySkipPermissions": true},
		},
		{
			name: "both old keys are ignored",
			options: map[string]any{
				"allowedTools":               "bash",
				"dangerouslySkipPermissions": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewClaudeProviderWithOptions(WithClaudeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			assert.NotContains(t, calls[0].Args, "--dangerously-skip-permissions")
		})
	}
}

// Compile-time verification that test uses correct imports
var (
	_ = assert.Equal
	_ = require.NoError
)
