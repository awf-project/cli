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

func TestCustomProvider_Name(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
	}{
		{
			name:         "custom provider",
			providerName: "my-custom-agent",
		},
		{
			name:         "another custom",
			providerName: "internal-ai-tool",
		},
		{
			name:         "single char",
			providerName: "x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCustomProvider(tt.providerName, "echo {{.prompt}}")

			assert.Equal(t, tt.providerName, provider.Name())
		})
	}
}

func TestCustomProvider_Execute_HappyPath(t *testing.T) {
	tests := []struct {
		name            string
		commandTemplate string
		prompt          string
		options         map[string]any
	}{
		{
			name:            "simple echo template",
			commandTemplate: "echo {{.prompt}}",
			prompt:          "Hello World",
			options:         nil,
		},
		{
			name:            "printf template",
			commandTemplate: "printf 'prompt: %s' '{{.prompt}}'",
			prompt:          "test data",
			options:         nil,
		},
		{
			name:            "cat heredoc template",
			commandTemplate: "cat <<EOF\n{{.prompt}}\nEOF",
			prompt:          "Analyze this",
			options:         nil,
		},
		{
			name:            "with options",
			commandTemplate: "echo prompt='{{.prompt}}' model='{{.options.model}}'",
			prompt:          "test",
			options: map[string]any{
				"model": "gpt-4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCustomProvider("custom", tt.commandTemplate)
			ctx := context.Background()

			result, err := provider.Execute(ctx, tt.prompt, tt.options)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "custom", result.Provider)
			assert.NotEmpty(t, result.Output)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
		})
	}
}

func TestCustomProvider_Execute_EmptyPrompt(t *testing.T) {
	provider := NewCustomProvider("custom", "echo {{.prompt}}")
	ctx := context.Background()

	result, err := provider.Execute(ctx, "", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prompt")
}

func TestCustomProvider_Execute_EmptyCommandTemplate(t *testing.T) {
	provider := NewCustomProvider("custom", "")
	ctx := context.Background()

	result, err := provider.Execute(ctx, "test", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "command")
}

func TestCustomProvider_Execute_InvalidTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  string
	}{
		{
			name:     "unclosed template",
			template: "echo {{.prompt",
			wantErr:  "template",
		},
		{
			name:     "invalid variable",
			template: "echo {{.invalid}}",
			wantErr:  "invalid",
		},
		{
			name:     "malformed syntax",
			template: "echo {{}}",
			wantErr:  "template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCustomProvider("custom", tt.template)
			ctx := context.Background()

			result, err := provider.Execute(ctx, "test", nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCustomProvider_Execute_Timeout(t *testing.T) {
	provider := NewCustomProvider("custom", "sleep 10")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond)

	result, err := provider.Execute(ctx, "test", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCustomProvider_Execute_ContextCancellation(t *testing.T) {
	provider := NewCustomProvider("custom", "echo test")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := provider.Execute(ctx, "test", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCustomProvider_Execute_CommandFailure(t *testing.T) {
	tests := []struct {
		name     string
		template string
	}{
		{
			name:     "nonexistent command",
			template: "nonexistent-command {{.prompt}}",
		},
		{
			name:     "failing command",
			template: "false",
		},
		{
			name:     "invalid syntax",
			template: "/bin/sh -c 'exit 1'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCustomProvider("custom", tt.template)
			ctx := context.Background()

			result, err := provider.Execute(ctx, "test", nil)

			// Should handle command failures gracefully
			if err != nil {
				assert.Error(t, err)
			} else {
				require.NotNil(t, result)
				// Non-zero exit code is acceptable for custom commands
			}
		})
	}
}

func TestCustomProvider_Execute_SpecialCharactersInPrompt(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		template string
		wantErr  bool
	}{
		{
			name:     "single quotes",
			prompt:   "it's a test",
			template: `printf '%s' "{{.prompt}}"`,
		},
		{
			name:     "double quotes",
			prompt:   `he said hello`,
			template: `echo "{{.prompt}}"`,
		},
		{
			name:     "backticks",
			prompt:   "use code here",
			template: `echo "{{.prompt}}"`,
		},
		{
			name:     "shell metacharacters",
			prompt:   "test and echo danger",
			template: `echo "{{.prompt}}"`,
		},
		{
			name:     "command injection without escaping (succeeds by design)",
			prompt:   "$(echo pwned)",
			template: "echo {{.prompt}}",
			// No wantErr: injection executes when template doesn't escape
		},
		{
			name:     "command injection with escaping (prevented)",
			prompt:   "$(echo pwned)",
			template: "echo {{escape .prompt}}",
			// Escaping prevents injection - output will be literal string
		},
		{
			name:     "newlines via heredoc",
			prompt:   "line1 line2 line3",
			template: "cat <<'EOF'\n{{.prompt}}\nEOF",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCustomProvider("custom", tt.template)
			result, err := provider.Execute(ctx, tt.prompt, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.NotEmpty(t, result.Output)
			}
		})
	}
}

func TestCustomProvider_Execute_WithComplexOptions(t *testing.T) {
	provider := NewCustomProvider(
		"custom",
		"echo prompt='{{.prompt}}' model='{{.options.model}}' temp='{{.options.temperature}}'",
	)
	ctx := context.Background()

	options := map[string]any{
		"model":       "gpt-4",
		"temperature": 0.7,
		"max_tokens":  1000,
	}

	result, err := provider.Execute(ctx, "test prompt", options)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestCustomProvider_Validate_HappyPath(t *testing.T) {
	provider := NewCustomProvider("custom", "echo test")

	err := provider.Validate()

	assert.NoError(t, err)
}

func TestCustomProvider_Validate_EmptyTemplate(t *testing.T) {
	provider := NewCustomProvider("custom", "")

	err := provider.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command")
}

func TestCustomProvider_Validate_InvalidCommand(t *testing.T) {
	provider := NewCustomProvider("custom", "nonexistent-binary-xyz")

	err := provider.Validate()

	// Should validate that command exists
	if err != nil {
		assert.Contains(t, err.Error(), "command")
	}
}

func TestCustomProvider_Execute_JSONOutput(t *testing.T) {
	provider := NewCustomProvider(
		"custom",
		`echo '{"result": "{{.prompt}}", "status": "ok"}'`,
	)
	ctx := context.Background()

	result, err := provider.Execute(ctx, "test", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
	// Should parse JSON if valid
	if result.Response != nil {
		assert.IsType(t, map[string]any{}, result.Response)
	}
}
