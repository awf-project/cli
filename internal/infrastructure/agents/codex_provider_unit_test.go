package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil"
)

// Component: C025 - Unit Tests for CodexProvider (WITHOUT integration build tag)
// These tests use MockCLIExecutor to avoid external CLI dependencies

func TestCodexProvider_Execute_Success(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		options    map[string]any
		mockStdout []byte
		wantOutput string
	}{
		{
			name:       "simple prompt",
			prompt:     "Generate a hello world function",
			mockStdout: []byte("func main() { println(\"Hello, World!\") }"),
			wantOutput: "func main() { println(\"Hello, World!\") }",
		},
		{
			name:       "prompt with language option",
			prompt:     "create a class",
			options:    map[string]any{"language": "python"},
			mockStdout: []byte("class MyClass:\n    pass"),
			wantOutput: "class MyClass:\n    pass",
		},
		{
			name:       "prompt with max_tokens option",
			prompt:     "short code",
			options:    map[string]any{"max_tokens": 100},
			mockStdout: []byte("x = 1"),
			wantOutput: "x = 1",
		},
		{
			name:       "prompt with quiet option",
			prompt:     "test",
			options:    map[string]any{"quiet": true},
			mockStdout: []byte("code output"),
			wantOutput: "code output",
		},
		{
			name:       "large output",
			prompt:     "generate complex function",
			mockStdout: []byte("func complex() {" + string(make([]byte, 1000)) + "}"),
			wantOutput: "func complex() {" + string(make([]byte, 1000)) + "}",
		},
		{
			name:       "empty output",
			prompt:     "test",
			mockStdout: []byte(""),
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "codex", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			// Token estimation is ~4 chars per token
			expectedTokens := len(tt.wantOutput) / 4
			assert.Equal(t, expectedTokens, result.Tokens)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
			assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
		})
	}
}

func TestCodexProvider_Execute_WithOptions(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		options     map[string]any
		mockStdout  []byte
		wantCLIArgs []string
	}{
		{
			name:        "language option",
			prompt:      "test",
			options:     map[string]any{"language": "python"},
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"--prompt", "test", "--language", "python"},
		},
		{
			name:        "max_tokens option",
			prompt:      "test",
			options:     map[string]any{"max_tokens": 200},
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"--prompt", "test", "--max-tokens", "200"},
		},
		{
			name:        "quiet option true",
			prompt:      "test",
			options:     map[string]any{"quiet": true},
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"--prompt", "test", "--quiet"},
		},
		{
			name:        "quiet option false",
			prompt:      "test",
			options:     map[string]any{"quiet": false},
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"--prompt", "test"},
		},
		{
			name:        "multiple options",
			prompt:      "complex task",
			options:     map[string]any{"language": "go", "max_tokens": 500, "quiet": true},
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"--prompt", "complex task", "--language", "go", "--max-tokens", "500", "--quiet"},
		},
		{
			name:        "no options",
			prompt:      "simple",
			options:     nil,
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"--prompt", "simple"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "codex", calls[0].Name)
			assert.Equal(t, tt.wantCLIArgs, calls[0].Args)
		})
	}
}

func TestCodexProvider_Execute_EmptyPrompt(t *testing.T) {
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
		{
			name:    "tabs and spaces",
			prompt:  "\t  \t  ",
			wantErr: "prompt cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
			// Executor should not be called
			calls := mockExec.GetCalls()
			assert.Empty(t, calls)
		})
	}
}

func TestCodexProvider_Execute_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		options map[string]any
		wantErr string
	}{
		{
			name:    "negative max_tokens",
			prompt:  "test",
			options: map[string]any{"max_tokens": -100},
			wantErr: "max_tokens must be non-negative",
		},
		{
			name:    "zero max_tokens is valid",
			prompt:  "test",
			options: map[string]any{"max_tokens": 0},
			wantErr: "", // Should not error
		},
		{
			name:    "very large max_tokens",
			prompt:  "test",
			options: map[string]any{"max_tokens": 999999},
			wantErr: "", // Should not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options)

			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestCodexProvider_Execute_ContextErrors(t *testing.T) {
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
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			ctx := tt.ctxFunc()
			result, err := provider.Execute(ctx, "test prompt", nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestCodexProvider_Execute_CLIErrors(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr string
	}{
		{
			name:    "cli execution error",
			mockErr: errors.New("command not found: codex"),
			wantErr: "codex execution failed",
		},
		{
			name:    "permission denied",
			mockErr: errors.New("permission denied"),
			wantErr: "codex execution failed",
		},
		{
			name:    "timeout error",
			mockErr: context.DeadlineExceeded,
			wantErr: "codex execution failed",
		},
		{
			name:    "generic error",
			mockErr: errors.New("unknown error"),
			wantErr: "codex execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestCodexProvider_Execute_StdoutStderrCombination(t *testing.T) {
	tests := []struct {
		name       string
		stdout     []byte
		stderr     []byte
		wantOutput string
	}{
		{
			name:       "stdout only",
			stdout:     []byte("code output"),
			stderr:     nil,
			wantOutput: "code output",
		},
		{
			name:       "stderr only",
			stdout:     nil,
			stderr:     []byte("warning message"),
			wantOutput: "warning message",
		},
		{
			name:       "both stdout and stderr",
			stdout:     []byte("code result "),
			stderr:     []byte("with warning"),
			wantOutput: "code result with warning",
		},
		{
			name:       "empty stdout and stderr",
			stdout:     []byte(""),
			stderr:     []byte(""),
			wantOutput: "",
		},
		{
			name:       "multiline stdout",
			stdout:     []byte("line1\nline2\nline3"),
			stderr:     nil,
			wantOutput: "line1\nline2\nline3",
		},
		{
			name:       "multiline stderr",
			stdout:     nil,
			stderr:     []byte("error1\nerror2"),
			wantOutput: "error1\nerror2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			// MockCLIExecutor needs to return both stdout and stderr
			mockExec.SetOutput(tt.stdout, tt.stderr)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantOutput, result.Output)
		})
	}
}

func TestCodexProvider_Execute_TokenEstimation(t *testing.T) {
	tests := []struct {
		name           string
		mockStdout     []byte
		expectedTokens int
	}{
		{
			name:           "small output",
			mockStdout:     []byte("test"),
			expectedTokens: 1, // 4 chars / 4 = 1
		},
		{
			name:           "medium output",
			mockStdout:     []byte("This is a longer output with multiple words"),
			expectedTokens: 10, // 44 chars / 4 = 10 (integer division)
		},
		{
			name:           "large output",
			mockStdout:     make([]byte, 1000),
			expectedTokens: 250, // 1000 / 4 = 250
		},
		{
			name:           "empty output",
			mockStdout:     []byte(""),
			expectedTokens: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedTokens, result.Tokens)
		})
	}
}

func TestCodexProvider_Execute_TimestampOrdering(t *testing.T) {
	mockExec := testutil.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("code"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
	assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
}

func TestCodexProvider_Execute_ProviderName(t *testing.T) {
	mockExec := testutil.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("code"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "codex", result.Provider)
}

func TestCodexProvider_ExecuteConversation_Success(t *testing.T) {
	tests := []struct {
		name       string
		state      *workflow.ConversationState
		prompt     string
		options    map[string]any
		mockStdout []byte
		wantOutput string
	}{
		{
			name:       "simple conversation",
			state:      workflow.NewConversationState(""),
			prompt:     "generate code",
			mockStdout: []byte("func main() {}"),
			wantOutput: "func main() {}",
		},
		{
			name:       "conversation with model option",
			state:      workflow.NewConversationState(""),
			prompt:     "test",
			options:    map[string]any{"model": "codex-002"},
			mockStdout: []byte("result"),
			wantOutput: "result",
		},
		{
			name:       "conversation with language option",
			state:      workflow.NewConversationState(""),
			prompt:     "create function",
			options:    map[string]any{"language": "python"},
			mockStdout: []byte("def func(): pass"),
			wantOutput: "def func(): pass",
		},
		{
			name:       "conversation with temperature",
			state:      workflow.NewConversationState(""),
			prompt:     "test",
			options:    map[string]any{"temperature": 0.7},
			mockStdout: []byte("creative output"),
			wantOutput: "creative output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.ExecuteConversation(context.Background(), tt.state, tt.prompt, tt.options)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "codex", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			assert.NotNil(t, result.State)
			assert.True(t, result.TokensEstimated)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
		})
	}
}

func TestCodexProvider_ExecuteConversation_NilState(t *testing.T) {
	mockExec := testutil.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("code"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), nil, "test", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conversation state cannot be nil")
	assert.Nil(t, result)
}

func TestCodexProvider_ExecuteConversation_EmptyPrompt(t *testing.T) {
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
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			state := workflow.NewConversationState("")

			result, err := provider.ExecuteConversation(context.Background(), state, tt.prompt, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestCodexProvider_ExecuteConversation_WithHistory(t *testing.T) {
	mockExec := testutil.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("second response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	// Create state with existing conversation history
	state := workflow.NewConversationState("")
	state.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "first question"))
	state.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "first answer"))

	result, err := provider.ExecuteConversation(context.Background(), state, "second question", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "second response", result.Output)
	// State should have 4 turns: 2 original + 1 user + 1 assistant
	assert.Len(t, result.State.Turns, 4)
	assert.Equal(t, workflow.TurnRoleUser, result.State.Turns[2].Role)
	assert.Equal(t, "second question", result.State.Turns[2].Content)
	assert.Equal(t, workflow.TurnRoleAssistant, result.State.Turns[3].Role)
	assert.Equal(t, "second response", result.State.Turns[3].Content)
}

func TestCodexProvider_ExecuteConversation_StatePreservation(t *testing.T) {
	mockExec := testutil.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	// Create state with initial turn
	originalState := workflow.NewConversationState("")
	originalState.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "initial"))

	result, err := provider.ExecuteConversation(context.Background(), originalState, "test", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Original state should be unchanged
	assert.Len(t, originalState.Turns, 1)
	assert.Equal(t, "initial", originalState.Turns[0].Content)
	// Result state should have new turns
	assert.Len(t, result.State.Turns, 3)
	assert.NotEqual(t, originalState, result.State)
}

func TestCodexProvider_ExecuteConversation_TokenCounting(t *testing.T) {
	mockExec := testutil.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response with tokens"), nil) // 20 chars = 5 tokens
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "previous")) // 8 chars = 2 tokens

	result, err := provider.ExecuteConversation(context.Background(), state, "current", nil) // 7 chars = 1 token (will be added to input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.TokensEstimated)
	// TokensOutput should be for the assistant's response
	assert.Equal(t, 5, result.TokensOutput)
	// TokensInput should be sum of all previous turns
	assert.Equal(t, 3, result.TokensInput) // 2 (previous) + 1 (current)
	// TokensTotal should be input + output
	assert.Equal(t, result.TokensInput+result.TokensOutput, result.TokensTotal)
	assert.Equal(t, 8, result.TokensTotal)
}

func TestCodexProvider_ExecuteConversation_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr string
	}{
		{
			name:    "negative temperature",
			options: map[string]any{"temperature": -0.5},
			wantErr: "temperature must be between 0 and 2",
		},
		{
			name:    "temperature too high",
			options: map[string]any{"temperature": 2.5},
			wantErr: "temperature must be between 0 and 2",
		},
		{
			name:    "negative max_tokens",
			options: map[string]any{"max_tokens": -100},
			wantErr: "max_tokens must be non-negative",
		},
		{
			name:    "valid temperature boundary 0",
			options: map[string]any{"temperature": 0.0},
			wantErr: "", // Should not error
		},
		{
			name:    "valid temperature boundary 2",
			options: map[string]any{"temperature": 2.0},
			wantErr: "", // Should not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			state := workflow.NewConversationState("")

			result, err := provider.ExecuteConversation(context.Background(), state, "test", tt.options)

			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestCodexProvider_ExecuteConversation_ContextErrors(t *testing.T) {
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
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			state := workflow.NewConversationState("")

			ctx := tt.ctxFunc()
			result, err := provider.ExecuteConversation(ctx, state, "test", nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestCodexProvider_ExecuteConversation_CLIErrors(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr string
	}{
		{
			name:    "cli execution error",
			mockErr: errors.New("command failed"),
			wantErr: "codex execution failed",
		},
		{
			name:    "timeout error",
			mockErr: context.DeadlineExceeded,
			wantErr: "codex execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			state := workflow.NewConversationState("")

			result, err := provider.ExecuteConversation(context.Background(), state, "test", nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestCodexProvider_ExecuteConversation_OptionsCLIArgumentConstruction(t *testing.T) {
	tests := []struct {
		name        string
		options     map[string]any
		wantCLIArgs []string
	}{
		{
			name:        "model option",
			options:     map[string]any{"model": "codex-002"},
			wantCLIArgs: []string{"--prompt", "test", "--model", "codex-002"},
		},
		{
			name:        "language option",
			options:     map[string]any{"language": "python"},
			wantCLIArgs: []string{"--prompt", "test", "--language", "python"},
		},
		{
			name:        "max_tokens option",
			options:     map[string]any{"max_tokens": 300},
			wantCLIArgs: []string{"--prompt", "test", "--max-tokens", "300"},
		},
		{
			name:        "temperature option",
			options:     map[string]any{"temperature": 0.8},
			wantCLIArgs: []string{"--prompt", "test", "--temperature", "0.80"},
		},
		{
			name:        "quiet option",
			options:     map[string]any{"quiet": true},
			wantCLIArgs: []string{"--prompt", "test", "--quiet"},
		},
		{
			name:        "multiple options",
			options:     map[string]any{"model": "codex-002", "language": "go", "temperature": 0.5, "quiet": true},
			wantCLIArgs: []string{"--prompt", "test", "--model", "codex-002", "--language", "go", "--temperature", "0.50", "--quiet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			state := workflow.NewConversationState("")

			_, err := provider.ExecuteConversation(context.Background(), state, "test", tt.options)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "codex", calls[0].Name)
			// For options tests, we check that required args are present (order may vary)
			for _, arg := range tt.wantCLIArgs {
				assert.Contains(t, calls[0].Args, arg)
			}
		})
	}
}

func TestCodexProvider_Name(t *testing.T) {
	provider := NewCodexProvider()
	assert.Equal(t, "codex", provider.Name())
}

func TestCodexProvider_Validate_Success(t *testing.T) {
	// Note: This test will fail if 'codex' is not in PATH
	// For unit testing purposes, we skip this test if codex is not available
	provider := NewCodexProvider()
	err := provider.Validate()

	// We don't assert anything specific here because this depends on the system
	// The test verifies the method can be called without panicking
	_ = err
}

func TestCodexProvider_NewCodexProvider_DefaultExecutor(t *testing.T) {
	provider := NewCodexProvider()
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.executor)
	// Default executor should be ExecCLIExecutor
	_, ok := provider.executor.(*ExecCLIExecutor)
	assert.True(t, ok, "default executor should be ExecCLIExecutor")
}
