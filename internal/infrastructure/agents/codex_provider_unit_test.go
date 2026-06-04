package agents

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			wantOutput: " ",
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
			name:    "valid model: gpt-4o",
			prompt:  "test",
			options: map[string]any{"model": "gpt-4o"},
			wantErr: "",
		},
		{
			name:    "valid model: gpt-3.5-turbo",
			prompt:  "test",
			options: map[string]any{"model": "gpt-3.5-turbo"},
			wantErr: "",
		},
		{
			name:    "valid model: codex-mini",
			prompt:  "test",
			options: map[string]any{"model": "codex-mini"},
			wantErr: "",
		},
		{
			name:    "valid model: o1",
			prompt:  "test",
			options: map[string]any{"model": "o1"},
			wantErr: "",
		},
		{
			name:    "valid model: o3",
			prompt:  "test",
			options: map[string]any{"model": "o3"},
			wantErr: "",
		},
		{
			name:    "valid model: o4-mini",
			prompt:  "test",
			options: map[string]any{"model": "o4-mini"},
			wantErr: "",
		},
		{
			name:    "invalid model: claude-3-opus (wrong provider)",
			prompt:  "test",
			options: map[string]any{"model": "claude-3-opus"},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
		{
			name:    "invalid model: gemini-pro (wrong provider)",
			prompt:  "test",
			options: map[string]any{"model": "gemini-pro"},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
		{
			name:    "invalid model: empty string",
			prompt:  "test",
			options: map[string]any{"model": ""},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
		{
			name:    "invalid model: toto (random string)",
			prompt:  "test",
			options: map[string]any{"model": "toto"},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
		{
			name:    "invalid model: o (single character, no digit)",
			prompt:  "test",
			options: map[string]any{"model": "o"},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
		{
			name:    "invalid model: ollama (starts with o but not o-series)",
			prompt:  "test",
			options: map[string]any{"model": "ollama"},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
		{
			name:    "invalid model: oracle (starts with o but not o-series)",
			prompt:  "test",
			options: map[string]any{"model": "oracle"},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
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

// TestCodexProvider_Execute_StdoutStderrCombination exercises the non-NDJSON fallback path:
// plain stdout/stderr carries no assistant_message event, so extractCodexAssistantText reports
// hadText=false and Output is whatever baseCLIProvider.combineOutput produced. It validates the
// base combine/fallback behavior, not the F103 NDJSON extraction overwrite.
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
			wantOutput: " ",
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
		name       string
		mockStdout []byte
	}{
		{name: "small output", mockStdout: []byte("test")},
		{name: "medium output", mockStdout: []byte("This is a longer output with multiple words")},
		{name: "large output", mockStdout: make([]byte, 1000)},
		{name: "empty output", mockStdout: []byte("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			// Plain (non-NDJSON) output is used verbatim, except an empty body becomes the " "
			// fallback. The ApproximationTokenizer estimates len/4 tokens; derive the expectation
			// from that contract instead of hardcoding so the test tracks the tokenizer's ratio.
			wantOutput := string(tt.mockStdout)
			if wantOutput == "" {
				wantOutput = " "
			}
			assert.Equal(t, len(wantOutput)/4, result.Tokens)
		})
	}
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
			name:    "valid model: gpt-4o",
			options: map[string]any{"model": "gpt-4o"},
			wantErr: "",
		},
		{
			name:    "valid model: gpt-3.5-turbo",
			options: map[string]any{"model": "gpt-3.5-turbo"},
			wantErr: "",
		},
		{
			name:    "valid model: o1",
			options: map[string]any{"model": "o1"},
			wantErr: "",
		},
		{
			name:    "invalid model: claude-3-opus",
			options: map[string]any{"model": "claude-3-opus"},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
		{
			name:    "invalid model: gemini-pro",
			options: map[string]any{"model": "gemini-pro"},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
		{
			name:    "invalid model: toto",
			options: map[string]any{"model": "toto"},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
		{
			name:    "invalid model: ollama",
			options: map[string]any{"model": "ollama"},
			wantErr: "must start with 'gpt-', 'codex-', or be an o-series",
		},
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

func TestCodexProvider_Validate_DoesNotPanic(t *testing.T) {
	// Validate() resolves 'codex' in PATH, so its result is environment-specific
	// (error when absent, nil when installed). This test only guarantees the method
	// can be invoked without panicking.
	provider := NewCodexProvider()
	_ = provider.Validate()
}

func TestCodexProvider_NewCodexProvider_DefaultExecutor(t *testing.T) {
	provider := NewCodexProvider()
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.executor)
	_, ok := provider.executor.(*ExecCLIExecutor)
	assert.True(t, ok, "default executor should be ExecCLIExecutor")
}

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

func TestCodexProvider_extractCodexTextContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantText string
	}{
		{
			name:     "single assistant_message",
			input:    ndjsonLine("hello"),
			wantText: "hello",
		},
		{
			name:     "multiple assistant_messages joined with newline",
			input:    ndjsonLine("foo") + "\n" + ndjsonLine("bar"),
			wantText: "foo\nbar",
		},
		{
			name:     "empty assistant_message text",
			input:    ndjsonLine(""),
			wantText: "",
		},
		{
			name:     "plain non-NDJSON text",
			input:    "second response",
			wantText: "",
		},
		{
			name:     "empty input",
			input:    "",
			wantText: "",
		},
		{
			name:     "NUL byte inside JSON string value",
			input:    "{\"type\":\"item.completed\",\"item\":{\"item_type\":\"assistant_message\",\"text\":\"hell\x00o\"}}",
			wantText: "hell\x00o",
		},
		{
			name:     "function_call only produces no text",
			input:    `{"type":"item.completed","item":{"item_type":"function_call","name":"bash","arguments":"{}"}}`,
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewCodexProvider()
			got := p.extractCodexTextContent(tt.input)
			assert.Equal(t, tt.wantText, got)
		})
	}
}

func TestCodexProvider_extractCodexAssistantText(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantText    string
		wantHadText bool
	}{
		{
			name:        "single assistant_message with text",
			input:       ndjsonLine("hello"),
			wantText:    "hello",
			wantHadText: true,
		},
		{
			name:        "multiple assistant_messages joined with newline",
			input:       ndjsonLine("foo") + "\n" + ndjsonLine("bar"),
			wantText:    "foo\nbar",
			wantHadText: true,
		},
		{
			name:        "empty assistant_message text — hadText true despite empty string",
			input:       ndjsonLine(""),
			wantText:    "",
			wantHadText: true,
		},
		{
			name:        "plain non-NDJSON text — hadText false",
			input:       "second response",
			wantText:    "",
			wantHadText: false,
		},
		{
			name:        "empty input",
			input:       "",
			wantText:    "",
			wantHadText: false,
		},
		{
			name:        "NUL byte inside JSON string value — no truncation",
			input:       "{\"type\":\"item.completed\",\"item\":{\"item_type\":\"assistant_message\",\"text\":\"hell\x00o\"}}",
			wantText:    "hell\x00o",
			wantHadText: true,
		},
		{
			name:        "function_call only — hadText false",
			input:       `{"type":"item.completed","item":{"item_type":"function_call","name":"bash","arguments":"{}"}}`,
			wantText:    "",
			wantHadText: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewCodexProvider()
			gotText, gotHadText := p.extractCodexAssistantText(tt.input)
			assert.Equal(t, tt.wantText, gotText)
			assert.Equal(t, tt.wantHadText, gotHadText)
		})
	}
}

func TestCodexProvider_extractCodexTokenUsage(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNil    bool
		wantInput  int
		wantOutput int
		wantTotal  int
	}{
		{
			name:       "turn.completed with full usage",
			input:      `{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":5}}`,
			wantInput:  10,
			wantOutput: 5,
			wantTotal:  15,
		},
		{
			name:       "usage among multiple events",
			input:      `{"type":"thread.started","thread_id":"t1"}` + "\n" + `{"type":"turn.completed","usage":{"input_tokens":3,"output_tokens":7}}`,
			wantInput:  3,
			wantOutput: 7,
			wantTotal:  10,
		},
		{
			name:       "partial usage defaults missing field to zero",
			input:      `{"type":"turn.completed","usage":{"input_tokens":4}}`,
			wantInput:  4,
			wantOutput: 0,
			wantTotal:  4,
		},
		{
			name:    "turn.completed without usage field",
			input:   `{"type":"turn.completed"}`,
			wantNil: true,
		},
		{
			name:    "usage present but not an object",
			input:   `{"type":"turn.completed","usage":"nope"}`,
			wantNil: true,
		},
		{
			name:    "no turn.completed event",
			input:   `{"type":"item.completed","item":{"item_type":"assistant_message","text":"hi"}}`,
			wantNil: true,
		},
		{
			name:    "empty output",
			input:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewCodexProvider()
			usage := p.extractCodexTokenUsage(tt.input)
			if tt.wantNil {
				assert.Nil(t, usage)
				return
			}
			require.NotNil(t, usage)
			assert.Equal(t, tt.wantInput, usage.InputTokens)
			assert.Equal(t, tt.wantOutput, usage.OutputTokens)
			assert.Equal(t, tt.wantTotal, usage.TotalTokens)
		})
	}
}

// TestCodexProvider_RealStream_0133 feeds the exact NDJSON stream emitted by
// codex-cli 0.133.0 (captured from `codex exec --json`) and asserts the assistant text
// is extracted cleanly and the real usage is read. Regression guard for the schema drift
// (item.type=agent_message) that previously left the raw envelope in state.Output.
func TestCodexProvider_RealStream_0133(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"thread.started","thread_id":"019e91f0-2fd4-7a20-9fc2-03994766eec3"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"PONG"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":18809,"cached_input_tokens":3456,"output_tokens":6,"reasoning_output_tokens":0}}`,
	}, "\n")

	p := NewCodexProvider()

	text, hadText := p.extractCodexAssistantText(stream)
	assert.True(t, hadText, "agent_message must be recognized as assistant text")
	assert.Equal(t, "PONG", text, "Output must be the clean answer, not the raw NDJSON envelope")

	usage := p.extractCodexTokenUsage(stream)
	require.NotNil(t, usage, "turn.completed.usage must be read")
	assert.Equal(t, 18809, usage.InputTokens)
	assert.Equal(t, 6, usage.OutputTokens)
}

func TestCodexProvider_Execute_OutputExtractionAllFormats(t *testing.T) {
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":"clean extracted text"}}`
	tests := []struct {
		name         string
		outputFormat string
	}{
		{name: "json format", outputFormat: "json"},
		{name: "stream-json format", outputFormat: "stream-json"},
		{name: "text format", outputFormat: "text"},
		{name: "empty format", outputFormat: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(ndjson), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			options := map[string]any{"output_format": tt.outputFormat}

			result, err := provider.Execute(context.Background(), "test", options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "clean extracted text", result.Output)
		})
	}
}

func TestCodexProvider_Execute_ResponsePopulation_ValidJSON(t *testing.T) {
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":"{\"answer\":42}"}}`
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Response)
	assert.Equal(t, float64(42), result.Response["answer"])
}

func TestCodexProvider_Execute_ResponsePopulation_InvalidJSON(t *testing.T) {
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":"plain prose text"}}`
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Response)
	assert.Equal(t, "plain prose text", result.Output)
}

func TestCodexProvider_Execute_ResponsePopulation_EmptyExtraction(t *testing.T) {
	rawMock := "raw plain text output"
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(rawMock), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Response)
	assert.Equal(t, rawMock, result.Output)
}

func TestCodexProvider_Execute_TokenRecountOnExtracted(t *testing.T) {
	// recount uses extracted length ("hi" = 0 tokens), not raw NDJSON length (~19 tokens)
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":"hi"}}`
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.TokensEstimated)
	assert.Equal(t, len("hi")/4, result.Tokens)
}

func TestCodexProvider_ExecuteConversation_EmptyAssistantMessage(t *testing.T) {
	// Response nil: base uses struct literal init, not make(); update if base switches to pre-init.
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":""}}`
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), workflow.NewConversationState(""), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "", result.Output)
	assert.Nil(t, result.Response)
	// F103: the presence-aware overwrite must re-derive token counts from the extracted
	// (empty) text rather than leaving the inflated count the base computed on the raw NDJSON.
	assert.Equal(t, 0, result.TokensOutput)
	assert.Equal(t, result.TokensInput, result.TokensTotal)
	// And the trailing assistant turn Content must match the overwritten Output.
	require.NotEmpty(t, result.State.Turns)
	last := result.State.Turns[len(result.State.Turns)-1]
	assert.Equal(t, workflow.TurnRoleAssistant, last.Role)
	assert.Equal(t, "", last.Content)
}

func TestCodexProvider_ExecuteConversation_ResponsePopulation_ValidJSON(t *testing.T) {
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":"{\"answer\":42}"}}`
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), workflow.NewConversationState(""), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Response)
	assert.Equal(t, float64(42), result.Response["answer"])
}

func TestCodexProvider_ExecuteConversation_JSONResponse(t *testing.T) {
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":"{\"key\":\"val\"}"}}`
	tests := []struct {
		name         string
		outputFormat string
	}{
		{name: "json format", outputFormat: "json"},
		{name: "stream-json format", outputFormat: "stream-json"},
		{name: "text format", outputFormat: "text"},
		{name: "absent format", outputFormat: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(ndjson), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			options := map[string]any{"output_format": tt.outputFormat}

			result, err := provider.ExecuteConversation(context.Background(), workflow.NewConversationState(""), "test", options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Response, "Response must be populated for all output_format values")
			assert.Equal(t, "val", result.Response["key"])
		})
	}
}

func TestCodexProvider_ExecuteConversation_NilResponseOnPlainText(t *testing.T) {
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":"plain prose text"}}`
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), workflow.NewConversationState(""), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Response)
	assert.Equal(t, "plain prose text", result.Output)
}

func TestCodexProvider_ExecuteConversation_MultiEventNDJSON(t *testing.T) {
	ndjson := ndjsonLine("foo") + "\n" + ndjsonLine("bar")

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
	state := workflow.NewConversationState("")

	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "foo\nbar", result.Output)
}

func TestCodexProvider_ExecuteConversation_NULBytesInStream(t *testing.T) {
	ndjson := "{\"type\":\"item.completed\",\"item\":{\"item_type\":\"assistant_message\",\"text\":\"before\x00after\"}}"

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
	state := workflow.NewConversationState("")

	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Output, "after", "post-NUL characters must not be truncated")
}

func TestCodexProvider_ExecuteConversation_NonJSONText(t *testing.T) {
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":"hello world"}}`

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
	state := workflow.NewConversationState("")

	result, err := provider.ExecuteConversation(context.Background(), state, "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "hello world", result.Output)
	assert.Nil(t, result.Response)
}

func TestCodexProvider_Execute_MultiEventNDJSON(t *testing.T) {
	ndjson := ndjsonLine("foo") + "\n" + ndjsonLine("bar")

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "foo\nbar", result.Output)
}

func TestCodexProvider_Execute_EmptyAssistantMessage(t *testing.T) {
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":""}}`

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "", result.Output)
	assert.Nil(t, result.Response)
}

func TestCodexProvider_Execute_NULBytesInStream(t *testing.T) {
	ndjson := "{\"type\":\"item.completed\",\"item\":{\"item_type\":\"assistant_message\",\"text\":\"before\x00after\"}}"

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Output, "after", "post-NUL characters must not be truncated")
}

func TestCodexProvider_Execute_NonJSONText(t *testing.T) {
	ndjson := `{"type":"item.completed","item":{"item_type":"assistant_message","text":"hello world"}}`

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjson), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "hello world", result.Output)
	assert.Nil(t, result.Response)
}

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

	require.Len(t, args, 5, "resume turn should have exec, resume, sessionID, --json, prompt")
	assert.Equal(t, "exec", args[0], "first arg should be exec subcommand")
	assert.Equal(t, "resume", args[1], "second arg should be resume subcommand")
	assert.Equal(t, "codex-abc123def456", args[2], "third arg should be session ID")
	assert.Equal(t, "--json", args[3], "fourth arg should be --json flag")
	assert.Equal(t, "follow up", args[4], "fifth arg should be prompt")
}
