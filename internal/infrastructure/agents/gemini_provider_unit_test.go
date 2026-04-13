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

// Component: C025 - Unit Tests for GeminiProvider (WITHOUT integration build tag)
// These tests use MockCLIExecutor to avoid external CLI dependencies

func TestGeminiProvider_Execute_Success(t *testing.T) {
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
			options:    map[string]any{"model": "gemini-pro"},
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
			name:       "large output",
			prompt:     "generate",
			mockStdout: []byte("This is a very long output " + string(make([]byte, 1000))),
			wantOutput: "This is a very long output " + string(make([]byte, 1000)),
		},
		{
			name:       "empty output",
			prompt:     "silent",
			mockStdout: []byte(""),
			wantOutput: " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "gemini", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			// Token estimation is ~4 chars per token, so verify it's in reasonable range
			expectedTokens := len(tt.wantOutput) / 4
			assert.Equal(t, expectedTokens, result.Tokens)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
			assert.True(t, result.CompletedAt.After(result.StartedAt))
		})
	}
}

func TestGeminiProvider_Execute_JSONParsing(t *testing.T) {
	tests := []struct {
		name       string
		mockStdout []byte
		wantJSON   bool
	}{
		{
			name:       "valid json response",
			mockStdout: []byte(`{"result":"ok","count":42}`),
			wantJSON:   true,
		},
		{
			name:       "json-like object response",
			mockStdout: []byte(`{"status":"success"}`),
			wantJSON:   true,
		},
		{
			name:       "non-json response",
			mockStdout: []byte("plain text response"),
			wantJSON:   false,
		},
		{
			name:       "malformed json",
			mockStdout: []byte(`{"incomplete`),
			wantJSON:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			// F082: Response is only populated when caller explicitly asks for
			// output_format: json. Auto-detection from raw output no longer happens
			// in text intent (default).
			result, err := provider.Execute(context.Background(), "test", map[string]any{"output_format": "json"}, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			if tt.wantJSON {
				assert.NotNil(t, result.Response)
				assert.IsType(t, map[string]any{}, result.Response)
			} else {
				assert.Nil(t, result.Response)
			}
		})
	}
}

func TestGeminiProvider_Execute_ValidationErrors(t *testing.T) {
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
			name:    "valid gemini prefix model accepted",
			prompt:  "test",
			options: map[string]any{"model": "gemini-pro"},
			wantErr: "",
		},
		{
			name:    "valid gemini model: gemini-pro-vision",
			prompt:  "test",
			options: map[string]any{"model": "gemini-pro-vision"},
			wantErr: "",
		},
		{
			name:    "valid gemini model: gemini-ultra",
			prompt:  "test",
			options: map[string]any{"model": "gemini-ultra"},
			wantErr: "",
		},
		{
			name:    "valid gemini model: gemini-2.0-flash",
			prompt:  "test",
			options: map[string]any{"model": "gemini-2.0-flash"},
			wantErr: "",
		},
		{
			name:    "valid gemini model: gemini-1.5-pro-latest",
			prompt:  "test",
			options: map[string]any{"model": "gemini-1.5-pro-latest"},
			wantErr: "",
		},
		{
			name:    "invalid model: gpt-4 (wrong provider)",
			prompt:  "test",
			options: map[string]any{"model": "gpt-4"},
			wantErr: "must start with",
		},
		{
			name:    "invalid model: claude-3-opus (wrong provider)",
			prompt:  "test",
			options: map[string]any{"model": "claude-3-opus"},
			wantErr: "must start with",
		},
		{
			name:    "invalid model: empty string",
			prompt:  "test",
			options: map[string]any{"model": ""},
			wantErr: "must start with",
		},
		{
			name:    "invalid model: single word",
			prompt:  "test",
			options: map[string]any{"model": "toto"},
			wantErr: "must start with",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestGeminiProvider_Execute_ContextErrors(t *testing.T) {
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
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			ctx := tt.ctxFunc()
			result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestGeminiProvider_Execute_CLIErrors(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr string
	}{
		{
			name:    "cli execution error",
			mockErr: errors.New("command not found"),
			wantErr: "gemini execution failed",
		},
		{
			name:    "permission denied",
			mockErr: errors.New("permission denied"),
			wantErr: "gemini execution failed",
		},
		{
			name:    "timeout error",
			mockErr: context.DeadlineExceeded,
			wantErr: "gemini execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", nil, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestGeminiProvider_Execute_StdoutStderrCombination(t *testing.T) {
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
			wantOutput: " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.stdout, tt.stderr)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantOutput, result.Output)
		})
	}
}

func TestGeminiProvider_Execute_CLIArgumentConstruction(t *testing.T) {
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
			options:      map[string]any{"model": "gemini-pro"},
			mockOutput:   []byte("ok"),
			wantContains: []string{"--model", "gemini-pro", "test"},
		},
		{
			name:         "with json format",
			prompt:       "test",
			options:      map[string]any{"output_format": "json"},
			mockOutput:   []byte(`{"result":"ok"}`),
			wantContains: []string{"--output-format", "stream-json", "test"},
		},
		{
			name:         "with multiple options",
			prompt:       "test",
			options:      map[string]any{"model": "gemini-ultra", "output_format": "json"},
			mockOutput:   []byte("ok"),
			wantContains: []string{"--model", "gemini-ultra", "--output-format", "stream-json", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "gemini", calls[0].Name)

			// Verify all expected arguments are present
			for _, want := range tt.wantContains {
				assert.Contains(t, calls[0].Args, want)
			}
		})
	}
}

func TestGeminiProvider_ExecuteConversation_Success(t *testing.T) {
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
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := tt.stateSetup()
			result, err := provider.ExecuteConversation(context.Background(), state, tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "gemini", result.Provider)
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

func TestGeminiProvider_ExecuteConversation_ValidationErrors(t *testing.T) {
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
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			result, err := provider.ExecuteConversation(context.Background(), tt.state, tt.prompt, tt.options, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestGeminiProvider_ExecuteConversation_ContextErrors(t *testing.T) {
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
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("system")
			ctx := tt.ctxFunc()
			result, err := provider.ExecuteConversation(ctx, state, "test", nil, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestGeminiProvider_ExecuteConversation_CLIErrors(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr string
	}{
		{
			name:    "cli execution error",
			mockErr: errors.New("command not found"),
			wantErr: "gemini execution failed",
		},
		{
			name:    "permission denied",
			mockErr: errors.New("permission denied"),
			wantErr: "gemini execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("system")
			result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestGeminiProvider_ExecuteConversation_StatePreservation(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

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

func TestGeminiProvider_ExecuteConversation_TokenCounting(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("This is a response with multiple words"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	state := workflow.NewConversationState("You are helpful")
	result, err := provider.ExecuteConversation(context.Background(), state, "Hello", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.TokensTotal, 0)
	assert.Equal(t, result.TokensInput+result.TokensOutput, result.TokensTotal)
	assert.True(t, result.TokensEstimated)
}

func TestGeminiProvider_Name(t *testing.T) {
	provider := NewGeminiProvider()
	assert.Equal(t, "gemini", provider.Name())
}

func TestGeminiProvider_NewGeminiProvider(t *testing.T) {
	provider := NewGeminiProvider()
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.executor)
}

func TestGeminiProvider_NewGeminiProviderWithOptions(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	assert.NotNil(t, provider)
	assert.Equal(t, mockExec, provider.executor)
}

func TestGeminiProvider_Execute_EmptyState(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	state := &workflow.ConversationState{}
	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.State)
}

func TestGeminiProvider_Execute_OptionsNil(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "response", result.Output)
}

func TestGeminiProvider_Execute_MultipleOptions(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

	options := map[string]any{
		"model":         "gemini-pro",
		"output_format": "text",
	}

	result, err := provider.Execute(context.Background(), "test", options, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify CLI was called with correct arguments
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "gemini", calls[0].Name)
	assert.Contains(t, calls[0].Args, "--model")
	assert.Contains(t, calls[0].Args, "gemini-pro")
}

func TestGeminiProvider_Execute_DangerouslySkipPermissions(t *testing.T) {
	tests := []struct {
		name         string
		options      map[string]any
		mockOutput   []byte
		wantContains []string
		wantNotIn    []string
	}{
		{
			name:         "skip permissions enabled",
			options:      map[string]any{"dangerously_skip_permissions": true},
			mockOutput:   []byte("response"),
			wantContains: []string{"--approval-mode=yolo"},
		},
		{
			name:       "skip permissions disabled",
			options:    map[string]any{"dangerously_skip_permissions": false},
			mockOutput: []byte("response"),
			wantNotIn:  []string{"--approval-mode=yolo"},
		},
		{
			name:       "skip permissions not provided",
			options:    nil,
			mockOutput: []byte("response"),
			wantNotIn:  []string{"--approval-mode=yolo"},
		},
		{
			name:         "skip permissions with model",
			options:      map[string]any{"dangerously_skip_permissions": true, "model": "gemini-pro"},
			mockOutput:   []byte("response"),
			wantContains: []string{"--approval-mode=yolo", "--model", "gemini-pro"},
		},
		{
			name:         "skip permissions with output format",
			options:      map[string]any{"dangerously_skip_permissions": true, "output_format": "json"},
			mockOutput:   []byte(`{"result":"ok"}`),
			wantContains: []string{"--approval-mode=yolo", "--output-format", "stream-json"},
		},
		{
			name:         "skip permissions with all options",
			options:      map[string]any{"dangerously_skip_permissions": true, "model": "gemini-ultra", "output_format": "json"},
			mockOutput:   []byte(`{"result":"ok"}`),
			wantContains: []string{"--approval-mode=yolo", "--model", "gemini-ultra", "--output-format", "stream-json"},
		},
		{
			name:       "skip permissions as string value",
			options:    map[string]any{"dangerously_skip_permissions": "yes"},
			mockOutput: []byte("response"),
			wantNotIn:  []string{"--approval-mode=yolo"},
		},
		{
			name:       "skip permissions as empty object",
			options:    map[string]any{"dangerously_skip_permissions": struct{}{}},
			mockOutput: []byte("response"),
			wantNotIn:  []string{"--approval-mode=yolo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "gemini", calls[0].Name)

			for _, want := range tt.wantContains {
				assert.Contains(t, calls[0].Args, want, "expected arg %q not found in %v", want, calls[0].Args)
			}

			for _, notWant := range tt.wantNotIn {
				assert.NotContains(t, calls[0].Args, notWant, "unexpected arg %q found in %v", notWant, calls[0].Args)
			}
		})
	}
}

func TestGeminiProvider_ExecuteConversation_DangerouslySkipPermissions(t *testing.T) {
	tests := []struct {
		name         string
		options      map[string]any
		mockOutput   []byte
		sessionID    string
		wantContains []string
		wantNotIn    []string
	}{
		{
			name:         "skip permissions enabled",
			options:      map[string]any{"dangerously_skip_permissions": true},
			mockOutput:   []byte("response"),
			wantContains: []string{"--approval-mode=yolo"},
		},
		{
			name:       "skip permissions disabled",
			options:    map[string]any{"dangerously_skip_permissions": false},
			mockOutput: []byte("response"),
			wantNotIn:  []string{"--approval-mode=yolo"},
		},
		{
			name:       "skip permissions not provided",
			options:    nil,
			mockOutput: []byte("response"),
			wantNotIn:  []string{"--approval-mode=yolo"},
		},
		{
			name:       "skip permissions with system prompt",
			options:    map[string]any{"dangerously_skip_permissions": true, "system_prompt": "You are helpful"},
			mockOutput: []byte("response"),
			// Gemini CLI has no --system-prompt flag; system prompt is inlined
			// into the first-turn message. Verify the flag is absent and yolo still applies.
			wantContains: []string{"--approval-mode=yolo"},
			wantNotIn:    []string{"--system-prompt"},
		},
		{
			name:         "skip permissions with model and format",
			options:      map[string]any{"dangerously_skip_permissions": true, "model": "gemini-pro", "output_format": "json"},
			mockOutput:   []byte(`{"result":"ok"}`),
			wantContains: []string{"--approval-mode=yolo", "--model", "gemini-pro", "--output-format", "stream-json"},
		},
		{
			name:         "skip permissions with existing session",
			options:      map[string]any{"dangerously_skip_permissions": true},
			mockOutput:   []byte("response"),
			sessionID:    "session-123",
			wantContains: []string{"--approval-mode=yolo", "--resume"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("You are helpful")
			state.SessionID = tt.sessionID

			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "gemini", calls[0].Name)

			for _, want := range tt.wantContains {
				assert.Contains(t, calls[0].Args, want, "expected arg %q not found in %v", want, calls[0].Args)
			}

			for _, notWant := range tt.wantNotIn {
				assert.NotContains(t, calls[0].Args, notWant, "unexpected arg %q found in %v", notWant, calls[0].Args)
			}
		})
	}
}

// TestGeminiProvider_Execute_OutputFormatMapping_T002
// T002: Map output_format: json → stream-json in Execute
// Requirement: System MUST invoke Gemini CLI with `--output-format stream-json`
// when `output_format` is `json` or `stream-json` in step options
func TestGeminiProvider_Execute_OutputFormatMapping_T002(t *testing.T) {
	tests := []struct {
		name            string
		inputFormat     string
		expectedCLIFlag string
		description     string
	}{
		{
			name:            "json maps to stream-json",
			inputFormat:     "json",
			expectedCLIFlag: "stream-json",
			description:     "when output_format is json, CLI must receive stream-json",
		},
		{
			name:            "stream-json stays as stream-json",
			inputFormat:     "stream-json",
			expectedCLIFlag: "stream-json",
			description:     "when output_format is already stream-json, it should stay unchanged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("test response"), nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			options := map[string]any{"output_format": tt.inputFormat}
			result, err := provider.Execute(context.Background(), "test prompt", options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "test response", result.Output)

			// Verify the CLI was invoked with the correct output-format flag
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1, "expected exactly one CLI call")
			require.Equal(t, "gemini", calls[0].Name)

			// Find --output-format flag in args and verify its value
			foundFlag := false
			for i, arg := range calls[0].Args {
				if arg != "--output-format" {
					continue
				}
				require.Less(t, i+1, len(calls[0].Args), "--output-format must have a value")
				actualValue := calls[0].Args[i+1]
				assert.Equal(t, tt.expectedCLIFlag, actualValue,
					"T002: %s - expected CLI flag value %q but got %q",
					tt.description, tt.expectedCLIFlag, actualValue)
				foundFlag = true
				break
			}
			assert.True(t, foundFlag, "T002: --output-format flag not found in CLI arguments")
		})
	}
}

// TestGeminiProvider_ExecuteConversation_OutputFormatMapping_T002
// T002: Map output_format: json → stream-json in ExecuteConversation
// Requirement: System MUST force `--output-format stream-json` (instead of `json`)
// in Gemini's ExecuteConversation for session ID extraction
func TestGeminiProvider_ExecuteConversation_OutputFormatMapping_T002(t *testing.T) {
	tests := []struct {
		name            string
		inputFormat     string
		expectedCLIFlag string
		mockOutput      []byte
		description     string
		isFirstTurn     bool
	}{
		{
			name:            "first turn: json maps to stream-json",
			inputFormat:     "json",
			expectedCLIFlag: "stream-json",
			mockOutput:      []byte(`{"session_id":"abc123","result":"Assistant response"}`),
			description:     "first turn: when output_format is json, CLI must receive stream-json",
			isFirstTurn:     true,
		},
		{
			name:            "first turn: stream-json stays as stream-json",
			inputFormat:     "stream-json",
			expectedCLIFlag: "stream-json",
			mockOutput:      []byte(`{"session_id":"abc123","result":"Assistant response"}`),
			description:     "first turn: when output_format is already stream-json, it should stay unchanged",
			isFirstTurn:     true,
		},
		{
			name:            "resume turn: json maps to stream-json",
			inputFormat:     "json",
			expectedCLIFlag: "stream-json",
			mockOutput:      []byte(`{"session_id":"abc123","result":"Continued response"}`),
			description:     "resume turn: when output_format is json, CLI must receive stream-json",
			isFirstTurn:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(mockExec))

			state := workflow.NewConversationState("You are helpful")
			if !tt.isFirstTurn {
				state.SessionID = "abc123"
			}

			options := map[string]any{"output_format": tt.inputFormat}
			result, err := provider.ExecuteConversation(context.Background(), state, "test prompt", options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			// Verify the CLI was invoked with the correct output-format flag
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1, "expected exactly one CLI call")
			require.Equal(t, "gemini", calls[0].Name)

			// Find --output-format flag in args and verify its value
			foundFlag := false
			for i, arg := range calls[0].Args {
				if arg != "--output-format" {
					continue
				}
				require.Less(t, i+1, len(calls[0].Args), "--output-format must have a value")
				actualValue := calls[0].Args[i+1]
				assert.Equal(t, tt.expectedCLIFlag, actualValue,
					"T002: %s - expected CLI flag value %q but got %q",
					tt.description, tt.expectedCLIFlag, actualValue)
				foundFlag = true
				break
			}
			assert.True(t, foundFlag, "T002: --output-format flag not found in CLI arguments")
		})
	}
}

// Compile-time verification that test uses correct imports
var (
	_ = assert.Equal
	_ = require.NoError
)
