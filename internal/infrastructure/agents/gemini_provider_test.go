//go:build integration

package agents

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: agent_providers
// Feature: 39

func TestGeminiProvider_Name(t *testing.T) {
	provider := NewGeminiProvider()

	assert.Equal(t, "gemini", provider.Name())
}

func TestGeminiProvider_Execute_HappyPath(t *testing.T) {
	provider := NewGeminiProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Gemini CLI not installed, skipping")
	}

	tests := []struct {
		name    string
		prompt  string
		options map[string]any
	}{
		{
			name:    "simple question",
			prompt:  "What is machine learning? Answer briefly.",
			options: nil,
		},
		{
			name:   "with output format",
			prompt: "List 3 programming languages as JSON",
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
			assert.Equal(t, "gemini", result.Provider)
			assert.NotEmpty(t, result.Output)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
		})
	}
}

func TestGeminiProvider_Execute_EmptyPrompt(t *testing.T) {
	provider := NewGeminiProvider()
	ctx := context.Background()

	result, err := provider.Execute(ctx, "", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
}

func TestGeminiProvider_Execute_Timeout(t *testing.T) {
	provider := NewGeminiProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond)

	result, err := provider.Execute(ctx, "Explain quantum computing", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestGeminiProvider_Execute_ContextCancellation(t *testing.T) {
	provider := NewGeminiProvider()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "Answer this", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestGeminiProvider_Execute_InvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr string
	}{
		{
			name: "invalid temperature range",
			options: map[string]any{
				"temperature": -0.5,
			},
			wantErr: "temperature",
		},
		{
			name: "invalid model name",
			options: map[string]any{
				"model": "nonexistent-model",
			},
			wantErr: "model",
		},
		{
			name: "invalid safety_settings type",
			options: map[string]any{
				"safety_settings": 123,
			},
			wantErr: "safety_settings",
		},
	}

	provider := NewGeminiProvider()
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

func TestGeminiProvider_Validate_CLINotInstalled(t *testing.T) {
	provider := NewGeminiProvider()

	err := provider.Validate()

	if err != nil {
		assert.Contains(t, err.Error(), "gemini")
	}
}

func TestGeminiProvider_Validate_CLIInstalled(t *testing.T) {
	provider := NewGeminiProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Gemini CLI not installed, skipping")
	}

	err := provider.Validate()
	assert.NoError(t, err)
}

func TestGeminiProvider_Execute_LongPrompt(t *testing.T) {
	provider := NewGeminiProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Gemini CLI not installed, skipping")
	}

	ctx := context.Background()

	// Test with moderately long prompt (not too long to exceed command line limits)
	longPrompt := "Summarize the following text briefly: "
	for i := 0; i < 50; i++ {
		longPrompt += "This is sentence number " + fmt.Sprintf("%d", i) + ". "
	}

	result, err := provider.Execute(ctx, longPrompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestGeminiProvider_Execute_SpecialCharacters(t *testing.T) {
	provider := NewGeminiProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("Gemini CLI not installed, skipping")
	}

	ctx := context.Background()

	prompt := "Explain: 'quotes', \"double quotes\", and $variables"

	result, err := provider.Execute(ctx, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}
