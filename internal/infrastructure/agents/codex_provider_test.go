package agents

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: agent_providers
// Feature: 39

func TestCodexProvider_Name(t *testing.T) {
	provider := NewCodexProvider()

	assert.Equal(t, "codex", provider.Name())
}

func TestCodexProvider_Execute_HappyPath(t *testing.T) {
	provider := NewCodexProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Codex CLI not installed, skipping")
	}

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

func TestCodexProvider_Execute_EmptyPrompt(t *testing.T) {
	provider := NewCodexProvider()
	ctx := context.Background()

	result, err := provider.Execute(ctx, "", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
}

func TestCodexProvider_Execute_Timeout(t *testing.T) {
	provider := NewCodexProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond)

	result, err := provider.Execute(ctx, "Generate code", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCodexProvider_Execute_ContextCancellation(t *testing.T) {
	provider := NewCodexProvider()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "Write code", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCodexProvider_Execute_InvalidOptions(t *testing.T) {
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

func TestCodexProvider_Validate_CLINotInstalled(t *testing.T) {
	provider := NewCodexProvider()

	err := provider.Validate()

	if err != nil {
		assert.Contains(t, err.Error(), "codex")
	}
}

func TestCodexProvider_Validate_CLIInstalled(t *testing.T) {
	provider := NewCodexProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Codex CLI not installed, skipping")
	}

	err := provider.Validate()
	assert.NoError(t, err)
}

func TestCodexProvider_Execute_CodeWithSpecialChars(t *testing.T) {
	provider := NewCodexProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Codex CLI not installed, skipping")
	}

	ctx := context.Background()

	prompt := `Write a function that uses "strings" and 'quotes'`

	result, err := provider.Execute(ctx, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestCodexProvider_Execute_MultilinePrompt(t *testing.T) {
	provider := NewCodexProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Codex CLI not installed, skipping")
	}

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
