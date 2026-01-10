//go:build integration

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

func TestOpenCodeProvider_Name(t *testing.T) {
	provider := NewOpenCodeProvider()

	assert.Equal(t, "opencode", provider.Name())
}

func TestOpenCodeProvider_Execute_HappyPath(t *testing.T) {
	provider := NewOpenCodeProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("OpenCode CLI not installed, skipping")
	}

	tests := []struct {
		name    string
		prompt  string
		options map[string]any
	}{
		{
			name:    "simple code request",
			prompt:  "Create a HTTP server",
			options: nil,
		},
		{
			name:   "with framework option",
			prompt: "Build a web app",
			options: map[string]any{
				"framework": "gin",
			},
		},
		{
			name:   "with verbose mode",
			prompt: "Debug this issue",
			options: map[string]any{
				"verbose": true,
			},
		},
		{
			name:   "with output directory",
			prompt: "Generate project structure",
			options: map[string]any{
				"output_dir": "/tmp/project",
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(ctx, tt.prompt, tt.options)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "opencode", result.Provider)
			assert.NotEmpty(t, result.Output)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
		})
	}
}

func TestOpenCodeProvider_Execute_EmptyPrompt(t *testing.T) {
	provider := NewOpenCodeProvider()
	ctx := context.Background()

	result, err := provider.Execute(ctx, "", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
}

func TestOpenCodeProvider_Execute_Timeout(t *testing.T) {
	provider := NewOpenCodeProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond)

	result, err := provider.Execute(ctx, "Generate code", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestOpenCodeProvider_Execute_ContextCancellation(t *testing.T) {
	provider := NewOpenCodeProvider()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "Run task", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestOpenCodeProvider_Execute_InvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr string
	}{
		{
			name: "invalid output_dir",
			options: map[string]any{
				"output_dir": 123, // Should be string
			},
			wantErr: "output_dir",
		},
		{
			name: "invalid verbose type",
			options: map[string]any{
				"verbose": "yes", // Should be bool
			},
			wantErr: "verbose",
		},
	}

	provider := NewOpenCodeProvider()
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

func TestOpenCodeProvider_Validate_CLINotInstalled(t *testing.T) {
	provider := NewOpenCodeProvider()

	err := provider.Validate()
	if err != nil {
		assert.Contains(t, err.Error(), "opencode")
	}
}

func TestOpenCodeProvider_Validate_CLIInstalled(t *testing.T) {
	provider := NewOpenCodeProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("OpenCode CLI not installed, skipping")
	}

	err := provider.Validate()
	assert.NoError(t, err)
}

func TestOpenCodeProvider_Execute_ComplexPrompt(t *testing.T) {
	provider := NewOpenCodeProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("OpenCode CLI not installed, skipping")
	}

	ctx := context.Background()

	prompt := `Create a microservice that:
- Handles HTTP requests
- Connects to PostgreSQL
- Implements CRUD operations
- Has unit tests`

	result, err := provider.Execute(ctx, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestOpenCodeProvider_Execute_SpecialCharactersInPrompt(t *testing.T) {
	provider := NewOpenCodeProvider()
	if err := provider.Validate(); err != nil {
		t.Skip("OpenCode CLI not installed, skipping")
	}

	ctx := context.Background()

	prompt := `Handle strings with "quotes", 'apostrophes', and $variables`

	result, err := provider.Execute(ctx, prompt, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}
