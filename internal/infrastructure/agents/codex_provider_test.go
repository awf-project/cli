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

func TestCodexProvider_Execute_HappyPath_Integration(t *testing.T) {
	provider := NewCodexProvider()

	tests := []struct {
		name    string
		prompt  string
		options map[string]any
	}{
		{
			name:    "simple code generation",
			prompt:  "Write a function to reverse a string",
			options: nil,
		},
		{
			name:   "with language option",
			prompt: "Create a REST API endpoint",
			options: map[string]any{
				"language": "go",
			},
		},
		{
			name:   "with quiet mode",
			prompt: "Fix this bug",
			options: map[string]any{
				"quiet": true,
			},
		},
		{
			name:   "with max_tokens",
			prompt: "Explain recursion",
			options: map[string]any{
				"max_tokens": 200,
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.prompt, tt.options)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "codex", result.Provider)
			assert.NotEmpty(t, result.Output)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
		})
	}
}

func TestCodexProvider_Execute_EmptyPrompt_Integration(t *testing.T) {
	provider := NewCodexProvider()
	ctx := context.Background()

	result, err := provider.Execute(ctx, "", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
}

func TestCodexProvider_Execute_Timeout_Integration(t *testing.T) {
	provider := NewCodexProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond)

	result, err := provider.Execute(ctx, "Generate code", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCodexProvider_Execute_ContextCancellation_Integration(t *testing.T) {
	provider := NewCodexProvider()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "Write code", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCodexProvider_Execute_InvalidOptions_Integration(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr string
	}{
		{
			name: "negative max_tokens",
			options: map[string]any{
				"max_tokens": -100,
			},
			wantErr: "max_tokens",
		},
		{
			name: "invalid language",
			options: map[string]any{
				"language": 123, // Should be string
			},
			wantErr: "language",
		},
	}

	provider := NewCodexProvider()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, "Test", tt.options)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCodexProvider_Validate_CLINotInstalled_Integration(t *testing.T) {
	provider := NewCodexProvider()

	err := provider.Validate()
	if err != nil {
		assert.Contains(t, err.Error(), "codex")
	}
}

func TestCodexProvider_Validate_CLIInstalled_Integration(t *testing.T) {
	provider := NewCodexProvider()

	err := provider.Validate()
	assert.NoError(t, err)
}

func TestCodexProvider_Execute_CodeWithSpecialChars_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()

	prompt := `Write a function that uses "strings" and 'quotes'`

	result, err := provider.Execute(ctx, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestCodexProvider_Execute_MultilinePrompt_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()

	prompt := `Write a function that:
1. Takes a string
2. Reverses it
3. Returns the result`

	result, err := provider.Execute(ctx, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

// Component: provider_conversation_support
// Feature: F033

func TestCodexProvider_ExecuteConversation_HappyPath_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful coding assistant.")
	prompt := "Explain what a for loop is."
	options := map[string]any{
		"model": "gpt-4",
	}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "codex", result.Provider)
	assert.NotEmpty(t, result.Output)
	assert.NotNil(t, result.State)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
	assert.True(t, result.CompletedAt.After(result.StartedAt))
}

func TestCodexProvider_ExecuteConversation_EmptyState_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	state := &workflow.ConversationState{}
	prompt := "Hello"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.State)
}

func TestCodexProvider_ExecuteConversation_NilState_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	prompt := "Hello"

	result, err := provider.ExecuteConversation(ctx, nil, prompt, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "state")
}

func TestCodexProvider_ExecuteConversation_EmptyPrompt_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")

	result, err := provider.ExecuteConversation(ctx, state, "", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
}

func TestCodexProvider_ExecuteConversation_WithHistory_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful coding assistant.")

	// Add previous turns to conversation history
	state.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleSystem, "You are a helpful coding assistant."),
		*workflow.NewTurn(workflow.TurnRoleUser, "What is recursion?"),
		*workflow.NewTurn(workflow.TurnRoleAssistant, "Recursion is when a function calls itself."),
	}
	state.TotalTurns = 3
	state.TotalTokens = 100

	prompt := "Give me an example in Python"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
	assert.NotNil(t, result.State)
	assert.GreaterOrEqual(t, result.State.TotalTurns, 3)
}

func TestCodexProvider_ExecuteConversation_CodeGeneration_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a code generator.")
	prompt := "Write a function to calculate factorial in Go"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
	assert.Contains(t, result.Output, "func", "should contain Go function")
}

func TestCodexProvider_ExecuteConversation_ContextCancellation_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	state := workflow.NewConversationState("System prompt")
	prompt := "What is a variable?"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCodexProvider_ExecuteConversation_InvalidOptions_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("System prompt")
	prompt := "Hello"

	tests := []struct {
		name    string
		options map[string]any
		errMsg  string
	}{
		{
			name: "invalid temperature type",
			options: map[string]any{
				"temperature": "invalid",
			},
			errMsg: "temperature",
		},
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

func TestCodexProvider_ExecuteConversation_TokenCounting_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful coding assistant.")
	prompt := "Explain variables"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.TokensTotal, 0, "should have token count")
	assert.Equal(t, result.TokensInput+result.TokensOutput, result.TokensTotal)
	assert.True(t, result.TokensEstimated, "should be estimated for CLI provider")
}

func TestCodexProvider_ExecuteConversation_LargeHistory_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a helpful coding assistant.")

	// Add many turns to test large conversation history
	for i := 0; i < 15; i++ {
		state.Turns = append(
			state.Turns,
			*workflow.NewTurn(workflow.TurnRoleUser, "Question about topic "+string(rune(i+'A'))),
			*workflow.NewTurn(workflow.TurnRoleAssistant, "Answer about topic "+string(rune(i+'A'))),
		)
	}
	state.TotalTurns = 30
	state.TotalTokens = 3000

	prompt := "Summarize everything"

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestCodexProvider_ExecuteConversation_MultilineCode_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	state := workflow.NewConversationState("You are a code reviewer.")
	prompt := `Review this Go code:
func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}`

	result, err := provider.ExecuteConversation(ctx, state, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestCodexProvider_ExecuteConversation_StatePreservation_Integration(t *testing.T) {
	provider := NewCodexProvider()

	ctx := context.Background()
	initialState := workflow.NewConversationState("You are a helpful coding assistant.")
	initialState.Turns = []workflow.Turn{
		*workflow.NewTurn(workflow.TurnRoleSystem, "You are a helpful coding assistant."),
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
