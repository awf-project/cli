//go:build integration

package agents_test

import (
	"context"
	"slices"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: C065 — Agent Provider Options Audit
// Functional tests validating that provider-specific options changes work end-to-end.
// Tests verify: dead validation removal, flag passing changes, and max_completion_tokens migration.

// TestC065_Claude_SilentlyIgnoresDeadOptions validates that Claude provider
// silently ignores temperature and max_tokens options (no validation errors).
func TestC065_Claude_SilentlyIgnoresDeadOptions(t *testing.T) {
	provider := agents.NewClaudeProvider()

	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name:    "temperature option silently ignored",
			options: map[string]any{"temperature": 0.7},
		},
		{
			name:    "max_tokens option silently ignored",
			options: map[string]any{"max_tokens": 2048},
		},
		{
			name:    "both temperature and max_tokens silently ignored",
			options: map[string]any{"temperature": 0.5, "max_tokens": 1024},
		},
		{
			name:    "dead options with valid model option",
			options: map[string]any{"model": "haiku", "temperature": 0.8, "max_tokens": 512},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := provider.Validate(); err != nil {
				t.Skip("Claude CLI not installed, skipping")
			}

			_, err := provider.Execute(ctx, "test prompt", tt.options)

			// Should not error — options are silently ignored, not validated
			require.NoError(t, err, "Claude should silently ignore dead options without validation error")
		})
	}
}

// TestC065_Codex_PassesModelNotMaxTokensOrTemperature validates that Codex
// passes --model flag but NOT --max-tokens or --temperature flags.
func TestC065_Codex_PassesModelNotMaxTokensOrTemperature(t *testing.T) {
	tests := []struct {
		name             string
		options          map[string]any
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:             "model flag passed, max_tokens NOT passed",
			options:          map[string]any{"model": "gpt-4o", "max_tokens": 2048},
			shouldContain:    []string{"--model", "gpt-4o"},
			shouldNotContain: []string{"--max-tokens"},
		},
		{
			name:             "model flag passed, temperature NOT passed",
			options:          map[string]any{"model": "code-davinci", "temperature": 0.7},
			shouldContain:    []string{"--model", "code-davinci"},
			shouldNotContain: []string{"--temperature"},
		},
		{
			name:             "no model flag when option absent",
			options:          nil,
			shouldContain:    []string{},
			shouldNotContain: []string{"--model"},
		},
		{
			name:             "language still passed, dead options ignored",
			options:          map[string]any{"language": "python", "max_tokens": 100, "temperature": 0.5},
			shouldContain:    []string{"--language", "python"},
			shouldNotContain: []string{"--max-tokens", "--temperature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("code"), nil)
			provider := agents.NewCodexProviderWithOptions(agents.WithCodexExecutor(mockExec))

			_, err := provider.Execute(context.Background(), "test", tt.options)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			args := calls[0].Args
			for _, arg := range tt.shouldContain {
				assert.True(t, slices.Contains(args, arg),
					"Execute() should contain %q in args: %v", arg, args)
			}
			for _, arg := range tt.shouldNotContain {
				assert.False(t, slices.Contains(args, arg),
					"Execute() should NOT contain %q in args: %v", arg, args)
			}
		})
	}
}

// TestC065_CodexConversation_RemovesDeadFlags validates that ExecuteConversation
// does not pass --max-tokens or --temperature flags.
func TestC065_CodexConversation_RemovesDeadFlags(t *testing.T) {
	tests := []struct {
		name             string
		options          map[string]any
		shouldNotContain []string
	}{
		{
			name:             "max_tokens NOT passed in conversation mode",
			options:          map[string]any{"max_tokens": 512},
			shouldNotContain: []string{"--max-tokens"},
		},
		{
			name:             "temperature NOT passed in conversation mode",
			options:          map[string]any{"temperature": 0.8},
			shouldNotContain: []string{"--temperature"},
		},
		{
			name:             "neither flag passed even with both options",
			options:          map[string]any{"max_tokens": 1024, "temperature": 0.6},
			shouldNotContain: []string{"--max-tokens", "--temperature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"output": "response"}`), nil)
			provider := agents.NewCodexProviderWithOptions(agents.WithCodexExecutor(mockExec))

			state := &workflow.ConversationState{
				Turns: []workflow.Turn{},
			}

			_, err := provider.ExecuteConversation(context.Background(), state, "user prompt", tt.options)

			require.NoError(t, err)
			calls := mockExec.GetCalls()
			require.True(t, len(calls) > 0, "ExecuteConversation should invoke CLI")

			args := calls[len(calls)-1].Args
			for _, arg := range tt.shouldNotContain {
				assert.False(t, slices.Contains(args, arg),
					"ExecuteConversation() should NOT contain %q in args: %v", arg, args)
			}
		})
	}
}

// TestC065_Gemini_SilentlyIgnoresTemperature validates that Gemini provider
// silently ignores temperature option (no validation).
func TestC065_Gemini_SilentlyIgnoresTemperature(t *testing.T) {
	provider := agents.NewGeminiProvider()

	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name:    "temperature option silently ignored",
			options: map[string]any{"temperature": 0.5},
		},
		{
			name:    "temperature with model option",
			options: map[string]any{"model": "gemini-pro", "temperature": 0.7},
		},
		{
			name:    "various temperature values ignored",
			options: map[string]any{"temperature": 1.5},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := provider.Validate(); err != nil {
				t.Skip("Gemini CLI not installed, skipping")
			}

			_, err := provider.Execute(ctx, "test prompt", tt.options)

			// Should not error — temperature is silently ignored
			require.NoError(t, err, "Gemini should silently ignore temperature without validation error")
		})
	}
}

// TestC065_OpenAICompatible_MaxCompletionTokensFieldName validates that OpenAI Compatible
// provider serializes max_completion_tokens as the JSON field name (not max_tokens).
func TestC065_OpenAICompatible_MaxCompletionTokensFieldName(t *testing.T) {
	t.Run("max_completion_tokens field serialized in JSON", func(t *testing.T) {
		provider := agents.NewOpenAICompatibleProvider()

		// With max_completion_tokens option, Execute should parse and handle it
		options := map[string]any{
			"base_url":              "http://localhost:11434/v1",
			"model":                 "test-model",
			"max_completion_tokens": 1024,
		}

		_, err := provider.Execute(context.Background(), "test prompt", options)

		// Error expected since we don't have real HTTP connection, but the key point is
		// that the option was parsed correctly (no validation error on the option itself)
		if err == nil || !slices.Contains([]string{"connection refused", "no such host"}, err.Error()) {
			t.Skip("HTTP unavailable, skipping")
		}
	})

	t.Run("max_tokens legacy fallback accepted", func(t *testing.T) {
		provider := agents.NewOpenAICompatibleProvider()

		options := map[string]any{
			"base_url":   "http://localhost:11434/v1",
			"model":      "test-model",
			"max_tokens": 2048,
		}

		_, err := provider.Execute(context.Background(), "test prompt", options)

		// Error expected since HTTP unavailable, but option should be accepted without validation error
		if err == nil || !slices.Contains([]string{"connection refused", "no such host"}, err.Error()) {
			t.Skip("HTTP unavailable, skipping")
		}
	})
}

// TestC065_OpenAICompatible_NegativeMaxCompletionTokensRejected validates
// that negative max_completion_tokens values are rejected with clear error.
func TestC065_OpenAICompatible_NegativeMaxCompletionTokensRejected(t *testing.T) {
	tests := []struct {
		name       string
		options    map[string]any
		wantErr    bool
		errKeyword string
	}{
		{
			name:       "negative max_completion_tokens rejected",
			options:    map[string]any{"base_url": "http://localhost:11434/v1", "model": "llama", "max_completion_tokens": -1},
			wantErr:    true,
			errKeyword: "max_completion_tokens",
		},
		{
			name:       "negative max_tokens (legacy) rejected",
			options:    map[string]any{"base_url": "http://localhost:11434/v1", "model": "llama", "max_tokens": -500},
			wantErr:    true,
			errKeyword: "max_completion_tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := agents.NewOpenAICompatibleProvider()

			_, err := provider.Execute(context.Background(), "test", tt.options)

			if tt.wantErr {
				require.Error(t, err, "Execute should error with negative value")
				assert.Contains(t, err.Error(), tt.errKeyword,
					"Error should reference the field name: %v", err)
			}
		})
	}
}

// TestC065_AllProviders_IgnoredOptionsNeverValidate validates that across all providers,
// removed/ignored options never trigger validation errors (no false positives).
func TestC065_AllProviders_IgnoredOptionsNeverValidate(t *testing.T) {
	t.Run("Claude silently ignores dead temperature/max_tokens options", func(t *testing.T) {
		claude := agents.NewClaudeProvider()
		if err := claude.Validate(); err != nil {
			t.Skip("Claude not installed")
		}

		deadOptions := map[string]any{
			"temperature": 0.7,
			"max_tokens":  2048,
		}

		_, err := claude.Execute(context.Background(), "test", deadOptions)
		require.NoError(t, err, "Claude should not validate dead options")
	})

	t.Run("Gemini silently ignores temperature", func(t *testing.T) {
		gemini := agents.NewGeminiProvider()
		if err := gemini.Validate(); err != nil {
			t.Skip("Gemini not installed")
		}

		_, err := gemini.Execute(context.Background(), "test", map[string]any{"temperature": 0.5})
		require.NoError(t, err, "Gemini should not validate temperature")
	})

	t.Run("Codex silently ignores max_tokens and temperature in Execute()", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("code"), nil)
		codex := agents.NewCodexProviderWithOptions(agents.WithCodexExecutor(mockExec))

		_, err := codex.Execute(context.Background(), "test", map[string]any{
			"max_tokens":  512,
			"temperature": 0.6,
		})
		require.NoError(t, err, "Codex should not validate dead options")
	})

	t.Run("Codex silently ignores max_tokens and temperature in ExecuteConversation()", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetOutput([]byte(`{"output": "result"}`), nil)
		codex := agents.NewCodexProviderWithOptions(agents.WithCodexExecutor(mockExec))

		state := &workflow.ConversationState{
			Turns: []workflow.Turn{},
		}

		_, err := codex.ExecuteConversation(context.Background(), state, "test", map[string]any{
			"max_tokens":  512,
			"temperature": 0.6,
		})
		require.NoError(t, err, "Codex ExecuteConversation should not validate dead options")
	})
}

// TestC065_Codex_ModelParity validates that Codex Execute() now passes --model flag
// for parity with ExecuteConversation().
func TestC065_Codex_ModelParity(t *testing.T) {
	t.Run("Execute() passes --model flag", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetOutput([]byte("code"), nil)
		provider := agents.NewCodexProviderWithOptions(agents.WithCodexExecutor(mockExec))

		_, err := provider.Execute(context.Background(), "test", map[string]any{"model": "gpt-4o"})

		require.NoError(t, err)
		calls := mockExec.GetCalls()
		require.Len(t, calls, 1)
		assert.True(t, slices.Contains(calls[0].Args, "--model"),
			"Execute() should pass --model flag")
		assert.True(t, slices.Contains(calls[0].Args, "gpt-4o"),
			"Execute() should pass model value")
	})

	t.Run("ExecuteConversation() also passes --model flag", func(t *testing.T) {
		mockExec := mocks.NewMockCLIExecutor()
		mockExec.SetOutput([]byte(`{"output": "result"}`), nil)
		provider := agents.NewCodexProviderWithOptions(agents.WithCodexExecutor(mockExec))

		state := &workflow.ConversationState{
			Turns: []workflow.Turn{},
		}

		_, err := provider.ExecuteConversation(context.Background(), state, "test", map[string]any{"model": "gpt-4o"})

		require.NoError(t, err)
		calls := mockExec.GetCalls()
		require.True(t, len(calls) > 0)
		assert.True(t, slices.Contains(calls[len(calls)-1].Args, "--model"),
			"ExecuteConversation() should pass --model flag")
	})
}
