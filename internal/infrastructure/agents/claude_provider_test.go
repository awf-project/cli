//go:build integration

package agents

import (
	"context"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: agent_providers
// Feature: 39

func TestClaudeProvider_Execute_HappyPath_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name    string
		prompt  string
		options map[string]any
	}{
		{
			name:    "simple prompt",
			prompt:  "What is 2+2?",
			options: nil,
		},
		{
			name:   "prompt with model alias",
			prompt: "Explain Go interfaces briefly",
			options: map[string]any{
				"model": "sonnet",
			},
		},
		{
			name:   "prompt with output format",
			prompt: "List 3 programming languages as JSON array",
			options: map[string]any{
				"output_format": "json",
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.prompt, tt.options)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "claude", result.Provider)
			assert.NotEmpty(t, result.Output)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
			assert.True(t, result.CompletedAt.After(result.StartedAt))
		})
	}
}

func TestClaudeProvider_Execute_JSONResponse_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()

	options := map[string]any{
		"output_format": "json",
	}

	result, err := provider.Execute(ctx, "List 3 colors", options)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Response, "JSON response should be parsed")
	assert.IsType(t, map[string]any{}, result.Response)
}

func TestClaudeProvider_Execute_TokenUsage_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()

	result, err := provider.Execute(ctx, "Hello", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.Tokens, 0, "Should capture token usage")
}

func TestClaudeProvider_Execute_EmptyPrompt_Integration(t *testing.T) {
	provider := NewClaudeProvider()
	ctx := context.Background()

	result, err := provider.Execute(ctx, "", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
}

func TestClaudeProvider_Execute_Timeout_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond) // Ensure timeout

	result, err := provider.Execute(ctx, "This should timeout", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestClaudeProvider_Execute_ContextCancellation_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := provider.Execute(ctx, "This should be cancelled", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestClaudeProvider_Execute_InvalidOptions_Integration(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr string
	}{
		{
			name: "negative max_tokens",
			options: map[string]any{
				"max_tokens": -1,
			},
			wantErr: "max_tokens",
		},
		{
			name: "invalid temperature",
			options: map[string]any{
				"temperature": 2.5, // Should be 0-1
			},
			wantErr: "temperature",
		},
		{
			name: "unknown model",
			options: map[string]any{
				"model": "invalid-model",
			},
			wantErr: "model",
		},
	}

	provider := NewClaudeProvider()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, "Test prompt", tt.options)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestClaudeProvider_Validate_CLINotInstalled_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	err := provider.Validate()
	// Should fail if claude CLI is not in PATH
	// In CI environments without Claude installed, this should error
	if err != nil {
		assert.Contains(t, err.Error(), "claude")
	}
}

func TestClaudeProvider_Validate_CLIInstalled_Integration(t *testing.T) {
	// Skip if Claude CLI not available
	provider := NewClaudeProvider()

	err := provider.Validate()
	assert.NoError(t, err)
}

func TestClaudeProvider_Execute_LargePrompt_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()

	// Generate large prompt (simulate real-world use case)
	largePrompt := ""
	for i := 0; i < 1000; i++ {
		largePrompt += "This is a test sentence. "
	}

	result, err := provider.Execute(ctx, largePrompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestClaudeProvider_Execute_SpecialCharactersInPrompt_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	tests := []struct {
		name   string
		prompt string
	}{
		{
			name:   "quotes",
			prompt: `He said "Hello World"`,
		},
		{
			name:   "backticks",
			prompt: "Use `code` blocks",
		},
		{
			name:   "newlines",
			prompt: "Line 1\nLine 2\nLine 3",
		},
		{
			name:   "unicode",
			prompt: "Émojis: 🚀 and unicode: 你好",
		},
		{
			name:   "shell metacharacters",
			prompt: "Test $VAR && echo 'danger' || rm -rf /",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.prompt, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.NotEmpty(t, result.Output)
		})
	}
}

// Component: provider_conversation_support
// Feature: F033

func TestClaudeProvider_ExecuteConversation_HappyPath_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful assistant.")
	prompt := "What is 2+2?"
	options := map[string]any{
		"model": "sonnet",
	}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "claude", result.Provider)
	assert.NotEmpty(t, result.Output)
	assert.NotNil(t, result.State)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
	assert.True(t, result.CompletedAt.After(result.StartedAt))
}

func TestClaudeProvider_ExecuteConversation_EmptyState_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	state := &workflow.ConversationState{}
	prompt := "Hello"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.State)
}

func TestClaudeProvider_ExecuteConversation_NilState_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	prompt := "Hello"

	result, err := provider.ExecuteConversation(ctx, nil, prompt, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "state")
}

func TestClaudeProvider_ExecuteConversation_EmptyPrompt_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")

	result, err := provider.ExecuteConversation(ctx, state, "", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
}

func TestClaudeProvider_ExecuteConversation_WithHistory_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful assistant.")

	// Add previous turns to conversation history
	state.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleSystem, "You are a helpful assistant."),
		*workflow.NewTurn(workflow.TurnRoleUser, "What is 2+2?"),
		*workflow.NewTurn(workflow.TurnRoleAssistant, "2+2 equals 4."),
	}
	state.TotalTurns = 3
	state.TotalTokens = 100

	prompt := "What about 3+3?"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
	assert.NotNil(t, result.State)
	assert.GreaterOrEqual(t, result.State.TotalTurns, 3)
}

func TestClaudeProvider_ExecuteConversation_JSONResponse_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful assistant.")
	prompt := "List 3 colors as JSON"
	options := map[string]any{
		"output_format": "json",
	}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Response, "JSON response should be parsed")
	assert.IsType(t, map[string]any{}, result.Response)
}

func TestClaudeProvider_ExecuteConversation_ContextCancellation_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	state := workflow.NewConversationState("System prompt")
	prompt := "What is 2+2?"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestClaudeProvider_ExecuteConversation_ContextTimeout_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure timeout

	state := workflow.NewConversationState("System prompt")
	prompt := "What is 2+2?"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestClaudeProvider_ExecuteConversation_InvalidOptions_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")
	prompt := "Hello"

	tests := []struct {
		name    string
		options map[string]any
		errMsg  string
	}{
		{
			name: "negative temperature",
			options: map[string]any{
				"temperature": -0.5,
			},
			errMsg: "temperature",
		},
		{
			name: "temperature too high",
			options: map[string]any{
				"temperature": 2.5,
			},
			errMsg: "temperature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.ExecuteConversation(ctx, state, prompt, tt.options)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestClaudeProvider_ExecuteConversation_TokenCounting_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful assistant.")
	prompt := "Hello, how are you?"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.TokensTotal, 0, "should have token count")
	assert.Equal(t, result.TokensInput+result.TokensOutput, result.TokensTotal)
	assert.True(t, result.TokensEstimated, "should be estimated for CLI provider")
}

func TestClaudeProvider_ExecuteConversation_LargeHistory_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful assistant.")

	// Add many turns to test large conversation history
	for i := 0; i < 20; i++ {
		state.Turns = append(state.Turns,
			*workflow.NewTurn(workflow.TurnRoleUser, "Question "+string(rune(i+'A'))),
			*workflow.NewTurn(workflow.TurnRoleAssistant, "Answer "+string(rune(i+'A'))),
		)
	}
	state.TotalTurns = 40
	state.TotalTokens = 5000

	prompt := "Final question?"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestClaudeProvider_ExecuteConversation_MultilinePrompt_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a code reviewer.")
	prompt := `Review this code:
func add(a, b int) int {
    return a + b
}`

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestClaudeProvider_ExecuteConversation_WithAllowedTools_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful assistant.")
	prompt := "What files are in the current directory?"
	options := map[string]any{
		"allowedTools": "bash,read,write",
	}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestClaudeProvider_ExecuteConversation_StatePreservation_Integration(t *testing.T) {
	provider := NewClaudeProvider()

	ctx := context.Background()
	initialState := workflow.NewConversationState("You are a helpful assistant.")
	initialState.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleSystem, "You are a helpful assistant."),
	}
	initialState.TotalTurns = 1
	initialState.TotalTokens = 50

	prompt := "Hello"

	result, err := provider.ExecuteConversation(ctx, initialState, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.State)

	// State should have additional turns
	assert.GreaterOrEqual(t, result.State.TotalTurns, initialState.TotalTurns)
	assert.GreaterOrEqual(t, result.State.TotalTokens, initialState.TotalTokens)
}
