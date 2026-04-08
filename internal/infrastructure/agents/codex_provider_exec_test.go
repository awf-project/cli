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

// Tests for T004: Codex Execute with new `exec --json` subcommand
// Verifies: args are `["exec", "--json", "<prompt>", ...]`; `--quiet` silently ignored

func TestCodexProvider_Execute_ExecJsonArgsStructure(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		options     map[string]any
		wantArgLen  int
		wantArgZero string
		wantArgOne  string
		wantArgTwo  string
	}{
		{
			name:        "minimal - just prompt",
			prompt:      "hello",
			options:     map[string]any{},
			wantArgLen:  3,
			wantArgZero: "exec",
			wantArgOne:  "--json",
			wantArgTwo:  "hello",
		},
		{
			name:        "with model",
			prompt:      "test prompt",
			options:     map[string]any{"model": "gpt-4"},
			wantArgLen:  5,
			wantArgZero: "exec",
			wantArgOne:  "--json",
			wantArgTwo:  "test prompt",
		},
		{
			name:        "with language",
			prompt:      "generate code",
			options:     map[string]any{"language": "python"},
			wantArgLen:  5,
			wantArgZero: "exec",
			wantArgOne:  "--json",
			wantArgTwo:  "generate code",
		},
		{
			name:        "with model and language",
			prompt:      "write function",
			options:     map[string]any{"model": "gpt-4o", "language": "go"},
			wantArgLen:  7,
			wantArgZero: "exec",
			wantArgOne:  "--json",
			wantArgTwo:  "write function",
		},
		{
			name:        "with dangerously_skip_permissions",
			prompt:      "risky",
			options:     map[string]any{"dangerously_skip_permissions": true},
			wantArgLen:  4,
			wantArgZero: "exec",
			wantArgOne:  "--json",
			wantArgTwo:  "risky",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("result"), nil)

			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1, "expected 1 CLI call")
			args := calls[0].Args
			assert.Equal(t, tt.wantArgLen, len(args), "arg count mismatch")
			assert.Equal(t, tt.wantArgZero, args[0], "args[0] should be 'exec'")
			assert.Equal(t, tt.wantArgOne, args[1], "args[1] should be '--json'")
			assert.Equal(t, tt.wantArgTwo, args[2], "args[2] should be prompt")
		})
	}
}

func TestCodexProvider_Execute_QuietOptionIgnored(t *testing.T) {
	tests := []struct {
		name       string
		options    map[string]any
		shouldHave string
		notHave    string
	}{
		{
			name:       "quiet true - should NOT add --quiet flag",
			options:    map[string]any{"quiet": true},
			shouldHave: "--json",
			notHave:    "--quiet",
		},
		{
			name:       "quiet false - should NOT add --quiet flag",
			options:    map[string]any{"quiet": false},
			shouldHave: "--json",
			notHave:    "--quiet",
		},
		{
			name:       "no quiet option - should NOT add --quiet flag",
			options:    map[string]any{},
			shouldHave: "--json",
			notHave:    "--quiet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("output"), nil)

			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
			_, err := provider.Execute(context.Background(), "test", tt.options, nil, nil)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			argStr := strings.Join(calls[0].Args, " ")
			assert.Contains(t, argStr, tt.shouldHave, "should contain "+tt.shouldHave)
			assert.NotContains(t, argStr, tt.notHave, "should NOT contain "+tt.notHave)
		})
	}
}

func TestCodexProvider_Execute_EmptyPromptError(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	tests := []string{"", "   ", "\t", "\n"}

	for _, emptyPrompt := range tests {
		result, err := provider.Execute(context.Background(), emptyPrompt, map[string]any{}, nil, nil)

		assert.Error(t, err, "should error on empty prompt: %q", emptyPrompt)
		assert.Nil(t, result)
	}
}

func TestCodexProvider_Execute_ContextCancelled(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "test", map[string]any{}, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestCodexProvider_Execute_CLIExecutionFailure(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetError(errors.New("codex not found"))

	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
	result, err := provider.Execute(context.Background(), "test", map[string]any{}, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "codex execution failed")
}

func TestCodexProvider_ExecuteConversation_FirstTurnExecJson(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"session_id":"codex-abc123","result":"response"}`), nil)

	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
	state := workflow.NewConversationState("conv-1")

	result, err := provider.ExecuteConversation(context.Background(), state, "hello", map[string]any{}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args
	assert.Equal(t, "exec", args[0], "first turn should use 'exec' subcommand")
	assert.Equal(t, "--json", args[1], "should have --json flag")
	assert.Contains(t, strings.Join(args, " "), "hello", "should contain prompt")
	assert.NotContains(t, strings.Join(args, " "), "--prompt", "should NOT have --prompt")
}

func TestCodexProvider_ExecuteConversation_ResumeTurnResumeCommand(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"session_id":"codex-xyz789","result":"continued"}`), nil)

	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
	state := workflow.NewConversationState("conv-1")
	state.SessionID = "codex-xyz789"

	result, err := provider.ExecuteConversation(context.Background(), state, "continue", map[string]any{}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	args := calls[0].Args
	assert.Equal(t, "resume", args[0], "resume turn should use 'resume' subcommand")
	assert.Equal(t, "codex-xyz789", args[1], "should have session ID as arg[1]")
	assert.Equal(t, "--json", args[2], "should have --json flag after session ID")
	assert.Contains(t, strings.Join(args, " "), "continue", "should contain prompt")
}

func TestCodexProvider_ExecuteConversation_NilStateError(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	result, err := provider.ExecuteConversation(context.Background(), nil, "test", map[string]any{}, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "conversation state cannot be nil")
}

func TestCodexProvider_ExecuteConversation_EmptyPromptError(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))
	state := workflow.NewConversationState("conv-1")

	result, err := provider.ExecuteConversation(context.Background(), state, "", map[string]any{}, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt cannot be empty")
}
