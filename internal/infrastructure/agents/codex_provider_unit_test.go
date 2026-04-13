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
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

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
			name:        "model option",
			prompt:      "test",
			options:     map[string]any{"model": "gpt-4o"},
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"exec", "--json", "test", "--model", "gpt-4o"},
		},
		{
			name:        "unknown options silently ignored",
			prompt:      "test",
			options:     map[string]any{"language": "python", "quiet": true},
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"exec", "--json", "test"},
		},
		{
			name:        "no options",
			prompt:      "simple",
			options:     nil,
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"exec", "--json", "simple"},
		},
		{
			name:        "dangerously_skip_permissions true maps to bypass flag",
			prompt:      "test",
			options:     map[string]any{"dangerously_skip_permissions": true},
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"exec", "--json", "test", "--dangerously-bypass-approvals-and-sandbox"},
		},
		{
			name:        "dangerously_skip_permissions false omits bypass flag",
			prompt:      "test",
			options:     map[string]any{"dangerously_skip_permissions": false},
			mockStdout:  []byte("code"),
			wantCLIArgs: []string{"exec", "--json", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

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
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, nil, nil, nil)

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
			name:    "max_tokens is silently ignored (no validation error)",
			prompt:  "test",
			options: map[string]any{"max_tokens": -100},
			wantErr: "",
		},
		{
			name:    "temperature is silently ignored (no validation error)",
			prompt:  "test",
			options: map[string]any{"temperature": 2.5},
			wantErr: "",
		},
		{
			name:    "unknown options are silently ignored",
			prompt:  "test",
			options: map[string]any{"unknown_option": "value"},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

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
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			ctx := tt.ctxFunc()
			result, err := provider.Execute(ctx, "test prompt", nil, nil, nil)

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
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

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
			mockExec := mocks.NewMockCLIExecutor()
			// MockCLIExecutor needs to return both stdout and stderr
			mockExec.SetOutput(tt.stdout, tt.stderr)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

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
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedTokens, result.Tokens)
		})
	}
}

func TestCodexProvider_Execute_TimestampOrdering(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("code"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
	assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
}

func TestCodexProvider_Execute_ProviderName(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("code"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.ExecuteConversation(context.Background(), tt.state, tt.prompt, tt.options, nil, nil)

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
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("code"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), nil, "test", nil, nil, nil)

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
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			state := workflow.NewConversationState("")

			result, err := provider.ExecuteConversation(context.Background(), state, tt.prompt, nil, nil, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

func TestCodexProvider_ExecuteConversation_WithHistory(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("second response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	// Create state with existing conversation history
	state := workflow.NewConversationState("")
	state.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "first question"))
	state.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "first answer"))

	result, err := provider.ExecuteConversation(context.Background(), state, "second question", nil, nil, nil)

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
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	// Create state with initial turn
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
	assert.NotEqual(t, originalState, result.State)
}

func TestCodexProvider_ExecuteConversation_TokenCounting(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response with tokens"), nil) // 20 chars = 5 tokens
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "previous")) // 8 chars = 2 tokens

	result, err := provider.ExecuteConversation(context.Background(), state, "current", nil, nil, nil) // 7 chars = 1 token (will be added to input)

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
			name:    "temperature is silently ignored (no validation error)",
			options: map[string]any{"temperature": 2.5},
			wantErr: "",
		},
		{
			name:    "max_tokens is silently ignored (no validation error)",
			options: map[string]any{"max_tokens": -100},
			wantErr: "",
		},
		{
			name:    "unknown options are silently ignored",
			options: map[string]any{"unknown_option": "value"},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			state := workflow.NewConversationState("")

			result, err := provider.ExecuteConversation(context.Background(), state, "test", tt.options, nil, nil)

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
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			state := workflow.NewConversationState("")

			ctx := tt.ctxFunc()
			result, err := provider.ExecuteConversation(ctx, state, "test", nil, nil, nil)

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
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			state := workflow.NewConversationState("")

			result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

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
			wantCLIArgs: []string{"exec", "--json", "test", "--model", "codex-002"},
		},
		{
			name:        "unknown options silently ignored",
			options:     map[string]any{"language": "python", "quiet": true},
			wantCLIArgs: []string{"exec", "--json", "test"},
		},
		{
			name:        "model option with unknowns",
			options:     map[string]any{"model": "codex-002", "language": "go", "quiet": true},
			wantCLIArgs: []string{"exec", "--json", "test", "--model", "codex-002"},
		},
		{
			name:        "dangerously_skip_permissions true maps to bypass flag",
			options:     map[string]any{"dangerously_skip_permissions": true},
			wantCLIArgs: []string{"exec", "--json", "test", "--dangerously-bypass-approvals-and-sandbox"},
		},
		{
			name:        "dangerously_skip_permissions false omits bypass flag",
			options:     map[string]any{"dangerously_skip_permissions": false},
			wantCLIArgs: []string{"exec", "--json", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			state := workflow.NewConversationState("")

			_, err := provider.ExecuteConversation(context.Background(), state, "test", tt.options, nil, nil)

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
	_, ok := provider.executor.(*ExecCLIExecutor)
	assert.True(t, ok, "default executor should be ExecCLIExecutor")
}

// T008: Explicit tests for `exec --json` arg structure assertion
func TestCodexProvider_Execute_ExecJSONStructure(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		options     map[string]any
		mockStdout  []byte
		wantCLIArgs []string
	}{
		{
			name:        "basic exec --json structure",
			prompt:      "hello",
			options:     nil,
			mockStdout:  []byte("output"),
			wantCLIArgs: []string{"exec", "--json", "hello"},
		},
		{
			name:        "exec --json with model",
			prompt:      "code",
			options:     map[string]any{"model": "gpt-4"},
			mockStdout:  []byte("result"),
			wantCLIArgs: []string{"exec", "--json", "code", "--model", "gpt-4"},
		},
		{
			name:        "unknown options silently ignored",
			prompt:      "test",
			options:     map[string]any{"language": "go", "quiet": true},
			mockStdout:  []byte("output"),
			wantCLIArgs: []string{"exec", "--json", "test"},
		},
		{
			name:        "dangerously_skip_permissions adds bypass flag",
			prompt:      "test",
			options:     map[string]any{"dangerously_skip_permissions": true},
			mockStdout:  []byte("output"),
			wantCLIArgs: []string{"exec", "--json", "test", "--dangerously-bypass-approvals-and-sandbox"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1, "executor should be called once")
			assert.Equal(t, "codex", calls[0].Name)
			assert.Equal(t, tt.wantCLIArgs, calls[0].Args, "args must match exactly")
		})
	}
}

// T008: Verify --json flag is first arg after exec subcommand
func TestCodexProvider_Execute_JSONFlagPosition(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("result"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	_, err := provider.Execute(context.Background(), "test prompt", nil, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args

	require.Len(t, args, 3, "should have 3 args: exec, --json, prompt")
	assert.Equal(t, "exec", args[0])
	assert.Equal(t, "--json", args[1])
	assert.Equal(t, "test prompt", args[2])
}

// T008: ExecuteConversation uses exec --json on first turn
func TestCodexProvider_ExecuteConversation_FirstTurnExecJSON(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("")
	_, err := provider.ExecuteConversation(context.Background(), state, "first prompt", nil, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args

	require.Len(t, args, 3, "first turn should have exec, --json, prompt")
	assert.Equal(t, "exec", args[0], "first arg should be exec subcommand")
	assert.Equal(t, "--json", args[1], "second arg should be --json flag")
	assert.Equal(t, "first prompt", args[2], "third arg should be prompt")
}

// T008: ExecuteConversation uses resume subcommand on resume turn
func TestCodexProvider_ExecuteConversation_ResumeJSONStructure(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	state := workflow.NewConversationState("")
	state.SessionID = "codex-abc123def456"
	_, err := provider.ExecuteConversation(context.Background(), state, "follow up", nil, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args

	require.Len(t, args, 4, "resume turn should have resume, sessionID, --json, prompt")
	assert.Equal(t, "resume", args[0], "first arg should be resume subcommand")
	assert.Equal(t, "codex-abc123def456", args[1], "second arg should be session ID")
	assert.Equal(t, "--json", args[2], "third arg should be --json flag")
	assert.Equal(t, "follow up", args[3], "fourth arg should be prompt")
}
