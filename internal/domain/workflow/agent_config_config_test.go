package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Component: agent_config_config
// Feature: 39
// Extracted from: agent_config_test.go
// Tests: 10 configuration validation tests
// =============================================================================

// =============================================================================
// Constants Tests
// =============================================================================

func TestAgentConstants(t *testing.T) {
	assert.Equal(t, 300, DefaultAgentTimeout)
	assert.Greater(t, DefaultAgentTimeout, 0)
}

// =============================================================================
// AgentConfig Validate Tests
// =============================================================================

func TestAgentConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  AgentConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Analyze this code: {{inputs.code}}",
				Options: map[string]any{
					"model":      "claude-sonnet-4-20250514",
					"max_tokens": 4096,
				},
				Timeout: 120,
			},
			wantErr: false,
		},
		{
			name: "valid config with minimal fields",
			config: AgentConfig{
				Provider: "codex",
				Prompt:   "Simple prompt",
			},
			wantErr: false,
		},
		{
			name: "valid config with zero timeout (uses default)",
			config: AgentConfig{
				Provider: "gemini",
				Prompt:   "Test prompt",
				Timeout:  0,
			},
			wantErr: false,
		},
		{
			name: "valid config with empty options",
			config: AgentConfig{
				Provider: "opencode",
				Prompt:   "Test",
				Options:  map[string]any{},
			},
			wantErr: false,
		},
		{
			name: "valid config with nil options",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Options:  nil,
			},
			wantErr: false,
		},
		{
			name: "valid custom provider with command",
			config: AgentConfig{
				Provider: "custom",
				Prompt:   "Test prompt",
				Command:  "my-llm --prompt {{prompt}}",
			},
			wantErr: false,
		},
		{
			name: "missing provider",
			config: AgentConfig{
				Prompt: "Test prompt",
			},
			wantErr: true,
			errMsg:  "provider",
		},
		{
			name: "empty provider",
			config: AgentConfig{
				Provider: "",
				Prompt:   "Test prompt",
			},
			wantErr: true,
			errMsg:  "provider",
		},
		{
			name: "missing prompt",
			config: AgentConfig{
				Provider: "claude",
			},
			wantErr: true,
			errMsg:  "prompt",
		},
		{
			name: "empty prompt",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "",
			},
			wantErr: true,
			errMsg:  "prompt",
		},
		{
			name: "negative timeout",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Timeout:  -1,
			},
			wantErr: true,
			errMsg:  "timeout",
		},
		{
			name: "large negative timeout",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Timeout:  -1000,
			},
			wantErr: true,
			errMsg:  "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAgentConfig_Validate_ProviderVariants(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{"claude", "claude", false},
		{"codex", "codex", false},
		{"gemini", "gemini", false},
		{"opencode", "opencode", false},
		{"custom", "custom", false},
		{"uppercase CLAUDE", "CLAUDE", false},
		{"mixed case Claude", "Claude", false},
		{"hyphenated name", "my-custom-llm", false},
		{"underscored name", "my_custom_llm", false},
		{"provider with version", "claude-v4", false},
		{"provider with dots", "llm.provider", false},
		{"single character", "a", false},
		{"whitespace only", "   ", true}, // should fail - effectively empty
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider: tt.provider,
				Prompt:   "Test prompt",
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAgentConfig_Validate_PromptVariants(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		wantErr bool
	}{
		{"simple text", "Analyze this code", false},
		{"with template", "Code: {{inputs.code}}", false},
		{"multiline", "Line 1\nLine 2\nLine 3", false},
		{"with special chars", "Test: @#$%^&*()", false},
		{"unicode", "日本語のテキスト", false},
		{"very long prompt", string(make([]byte, 10000)), false},
		{"single character", "A", false},
		{"whitespace only", "   ", false}, // not validated at this level
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider: "claude",
				Prompt:   tt.prompt,
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// AgentConfig GetTimeout Tests
// =============================================================================

func TestAgentConfig_GetTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  int
		expected time.Duration
	}{
		{
			name:     "zero returns default",
			timeout:  0,
			expected: DefaultAgentTimeout * time.Second,
		},
		{
			name:     "positive returns configured value",
			timeout:  60,
			expected: 60 * time.Second,
		},
		{
			name:     "large positive value",
			timeout:  3600,
			expected: 3600 * time.Second,
		},
		{
			name:     "exactly default value",
			timeout:  DefaultAgentTimeout,
			expected: DefaultAgentTimeout * time.Second,
		},
		{
			name:     "one second",
			timeout:  1,
			expected: 1 * time.Second,
		},
		{
			name:     "negative returns default",
			timeout:  -1,
			expected: DefaultAgentTimeout * time.Second,
		},
		{
			name:     "large negative returns default",
			timeout:  -1000,
			expected: DefaultAgentTimeout * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Timeout:  tt.timeout,
			}
			assert.Equal(t, tt.expected, config.GetTimeout())
		})
	}
}

// =============================================================================
// AgentConfig Options Tests
// =============================================================================

func TestAgentConfig_Options(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			name:    "nil options",
			options: nil,
		},
		{
			name:    "empty options",
			options: map[string]any{},
		},
		{
			name: "model option",
			options: map[string]any{
				"model": "claude-sonnet-4-20250514",
			},
		},
		{
			name: "multiple options",
			options: map[string]any{
				"model":         "claude-sonnet-4-20250514",
				"max_tokens":    4096,
				"temperature":   0.7,
				"output_format": "json",
			},
		},
		{
			name: "various types",
			options: map[string]any{
				"string_val": "value",
				"int_val":    42,
				"float_val":  3.14,
				"bool_val":   true,
				"slice_val":  []string{"a", "b"},
				"map_val":    map[string]string{"key": "val"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Options:  tt.options,
			}
			err := config.Validate(nil)
			require.NoError(t, err)
			if tt.options != nil {
				assert.Equal(t, tt.options, config.Options)
			}
		})
	}
}

// =============================================================================
// AgentConfig Custom Command Tests
// =============================================================================

func TestAgentConfig_CustomCommand(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		command  string
	}{
		{
			name:     "custom provider with command template",
			provider: "custom",
			command:  "my-llm --prompt {{prompt}}",
		},
		{
			name:     "custom with complex template",
			provider: "custom",
			command:  "llm exec --input {{prompt}} --model {{options.model}} --json",
		},
		{
			name:     "custom with path",
			provider: "custom",
			command:  "/usr/local/bin/my-ai {{prompt}}",
		},
		{
			name:     "empty command (custom provider)",
			provider: "custom",
			command:  "",
		},
		{
			name:     "command with built-in provider (ignored)",
			provider: "claude",
			command:  "ignored-command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider: tt.provider,
				Prompt:   "Test prompt",
				Command:  tt.command,
			}
			err := config.Validate(nil)
			require.NoError(t, err)
			assert.Equal(t, tt.command, config.Command)
		})
	}
}

// =============================================================================
// Integration-style Tests
// =============================================================================

func TestAgentConfig_CompleteExample(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Prompt: `Analyze this code for security issues:
{{inputs.code_path}}

Focus on: {{inputs.focus_areas}}`,
		Options: map[string]any{
			"model":         "claude-sonnet-4-20250514",
			"max_tokens":    4096,
			"temperature":   0.0,
			"output_format": "json",
		},
		Timeout: 180,
	}

	// Validate structure
	err := config.Validate(nil)
	require.NoError(t, err)

	// Check field values
	assert.Equal(t, "claude", config.Provider)
	assert.Contains(t, config.Prompt, "{{inputs.code_path}}")
	assert.Len(t, config.Options, 4)
	assert.Equal(t, 180*time.Second, config.GetTimeout())

	// Check individual options
	assert.Equal(t, "claude-sonnet-4-20250514", config.Options["model"])
	assert.Equal(t, 4096, config.Options["max_tokens"])
}

// =============================================================================
// Edge Cases and Boundary Conditions
// =============================================================================

func TestAgentConfig_TimeoutBoundaries(t *testing.T) {
	tests := []struct {
		name            string
		timeout         int
		expectedTimeout time.Duration
		wantErr         bool
	}{
		{"minimum valid (1)", 1, 1 * time.Second, false},
		{"zero (uses default)", 0, DefaultAgentTimeout * time.Second, false},
		{"large timeout (1 hour)", 3600, 3600 * time.Second, false},
		{"very large timeout (1 day)", 86400, 86400 * time.Second, false},
		{"negative (-1)", -1, DefaultAgentTimeout * time.Second, true},
		{"large negative", -9999, DefaultAgentTimeout * time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Timeout:  tt.timeout,
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedTimeout, config.GetTimeout())
			}
		})
	}
}

func TestAgentConfig_PromptTemplateVariations(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{
			name:   "inputs reference",
			prompt: "Analyze: {{inputs.file}}",
		},
		{
			name:   "states reference",
			prompt: "Previous result: {{states.prep.output}}",
		},
		{
			name:   "loop reference",
			prompt: "Item: {{loop.item}}, Index: {{loop.index}}",
		},
		{
			name:   "env reference",
			prompt: "API Key: {{env.API_KEY}}",
		},
		{
			name:   "mixed templates",
			prompt: "File: {{inputs.file}}, Result: {{states.analyze.output}}, Key: {{env.KEY}}",
		},
		{
			name: "multiline with templates",
			prompt: `Analyze the file at: {{inputs.code_path}}

Using these settings:
- Model: {{inputs.model}}
- Previous analysis: {{states.scan.response}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider: "claude",
				Prompt:   tt.prompt,
			}
			err := config.Validate(nil)
			require.NoError(t, err)
			assert.Equal(t, tt.prompt, config.Prompt)
		})
	}
}
