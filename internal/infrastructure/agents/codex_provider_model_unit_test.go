package agents

import (
	"context"
	"slices"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T002: Codex provider tests for removing unsupported flags and adding model parity
// Tests verify that --max-tokens and --temperature are NOT passed,
// and that --model IS passed in Execute() for parity with ExecuteConversation()

func TestCodexProvider_Execute_ModelFlag(t *testing.T) {
	tests := []struct {
		name        string
		options     map[string]any
		wantCLIArgs []string
	}{
		{
			name:        "model option passed",
			options:     map[string]any{"model": "gpt-4o"},
			wantCLIArgs: []string{"exec", "--json", "test prompt", "--model", "gpt-4o"},
		},
		{
			name:        "no model option",
			options:     nil,
			wantCLIArgs: []string{"exec", "--json", "test prompt"},
		},
		{
			name:        "model with language option",
			options:     map[string]any{"model": "code-davinci", "language": "python"},
			wantCLIArgs: []string{"exec", "--json", "test prompt", "--language", "python", "--model", "code-davinci"},
		},
		{
			name:        "model with quiet option",
			options:     map[string]any{"model": "gpt-3.5", "quiet": true},
			wantCLIArgs: []string{"exec", "--json", "test prompt", "--model", "gpt-3.5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("result"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "codex", calls[0].Name)
			assert.Equal(t, tt.wantCLIArgs, calls[0].Args)
		})
	}
}

func TestCodexProvider_Execute_MaxTokensNotPassed(t *testing.T) {
	tests := []struct {
		name       string
		options    map[string]any
		expectFlag bool
	}{
		{
			name:       "max_tokens option should NOT be passed to CLI",
			options:    map[string]any{"max_tokens": 100},
			expectFlag: false,
		},
		{
			name:       "max_tokens with other options should NOT pass max_tokens flag",
			options:    map[string]any{"max_tokens": 500, "language": "go"},
			expectFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			_, err := provider.Execute(context.Background(), "test", tt.options, nil, nil)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			args := calls[0].Args
			// Verify --max-tokens flag is NOT present
			assert.False(t, slices.Contains(args, "--max-tokens"), "Execute() should not pass --max-tokens flag")
		})
	}
}

func TestCodexProvider_Execute_TemperatureNotPassed(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name:    "temperature option should NOT be passed to CLI",
			options: map[string]any{"temperature": 0.7},
		},
		{
			name:    "temperature with other options should NOT pass temperature flag",
			options: map[string]any{"temperature": 0.5, "language": "python"},
		},
		{
			name:    "temperature as float should NOT be passed",
			options: map[string]any{"temperature": 1.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			_, err := provider.Execute(context.Background(), "test", tt.options, nil, nil)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			args := calls[0].Args
			// Verify --temperature flag is NOT present
			assert.False(t, slices.Contains(args, "--temperature"), "Execute() should not pass --temperature flag")
		})
	}
}

func TestCodexProvider_ExecuteConversation_MaxTokensNotPassed(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name:    "max_tokens option should NOT be passed to CLI in conversation mode",
			options: map[string]any{"max_tokens": 100},
		},
		{
			name:    "max_tokens with model should NOT pass max_tokens flag",
			options: map[string]any{"max_tokens": 500, "model": "gpt-4o"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &workflow.ConversationState{
				Turns: []workflow.Turn{},
			}

			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			_, err := provider.ExecuteConversation(context.Background(), state, "user prompt", tt.options, nil, nil)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			args := calls[0].Args
			// Verify --max-tokens flag is NOT present
			assert.False(t, slices.Contains(args, "--max-tokens"), "ExecuteConversation() should not pass --max-tokens flag")
		})
	}
}

func TestCodexProvider_ExecuteConversation_TemperatureNotPassed(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name:    "temperature option should NOT be passed to CLI in conversation mode",
			options: map[string]any{"temperature": 0.7},
		},
		{
			name:    "temperature with other options should NOT pass temperature flag",
			options: map[string]any{"temperature": 0.3, "model": "gpt-4o"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &workflow.ConversationState{
				Turns: []workflow.Turn{},
			}

			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("response"), nil)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

			_, err := provider.ExecuteConversation(context.Background(), state, "user prompt", tt.options, nil, nil)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			args := calls[0].Args
			// Verify --temperature flag is NOT present
			assert.False(t, slices.Contains(args, "--temperature"), "ExecuteConversation() should not pass --temperature flag")
		})
	}
}

func TestCodexProvider_Execute_ModelPriority(t *testing.T) {
	// Verify that model option is handled correctly in Execute()
	// (should appear in CLI args when provided)
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("code"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	options := map[string]any{
		"model":       "gpt-4o",
		"language":    "go",
		"max_tokens":  100,  // Should NOT be passed
		"temperature": 0.7,  // Should NOT be passed
		"quiet":       true, // Should NOT be passed (removed from Codex CLI)
	}

	_, err := provider.Execute(context.Background(), "test", options, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)

	args := calls[0].Args

	assert.True(t, slices.Contains(args, "--model"), "--model should be passed when provided")
	assert.True(t, slices.Contains(args, "--language"), "--language should be passed when provided")
	assert.False(t, slices.Contains(args, "--quiet"), "--quiet should NOT be passed (removed from Codex CLI)")
	assert.False(t, slices.Contains(args, "--max-tokens"), "--max-tokens should NOT be passed")
	assert.False(t, slices.Contains(args, "--temperature"), "--temperature should NOT be passed")
}

func TestCodexProvider_ExecuteConversation_NoUnsupportedFlags(t *testing.T) {
	// Verify ExecuteConversation doesn't pass unsupported flags
	// even when provided in options
	state := &workflow.ConversationState{
		Turns: []workflow.Turn{},
	}

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("response"), nil)
	provider := NewCodexProviderWithOptions(WithCodexExecutor(mockExec))

	options := map[string]any{
		"model":       "gpt-4o",
		"max_tokens":  100, // Should NOT be passed
		"temperature": 0.7, // Should NOT be passed
		"language":    "python",
	}

	_, err := provider.ExecuteConversation(context.Background(), state, "prompt", options, nil, nil)

	require.NoError(t, err)
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)

	args := calls[0].Args
	// Verify unsupported flags are absent
	assert.False(t, slices.Contains(args, "--max-tokens"), "--max-tokens should not be in ExecuteConversation args")
	assert.False(t, slices.Contains(args, "--temperature"), "--temperature should not be in ExecuteConversation args")

	// Verify supported flags ARE present
	assert.True(t, slices.Contains(args, "--model"), "--model should be passed when provided")
	assert.True(t, slices.Contains(args, "--language"), "--language should be passed when provided")
}
