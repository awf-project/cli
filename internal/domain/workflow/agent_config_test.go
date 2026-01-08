package workflow

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Component: agent_config
// Feature: 39
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
			err := tt.config.Validate()
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
			err := config.Validate()
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
			err := config.Validate()
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
			err := config.Validate()
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
			err := config.Validate()
			require.NoError(t, err)
			assert.Equal(t, tt.command, config.Command)
		})
	}
}

// =============================================================================
// AgentResult Constructor Tests
// =============================================================================

func TestNewAgentResult(t *testing.T) {
	provider := "claude"

	result := NewAgentResult(provider)

	require.NotNil(t, result)
	assert.Equal(t, provider, result.Provider)
	assert.Empty(t, result.Output)
	assert.NotNil(t, result.Response)
	assert.Empty(t, result.Response)
	assert.Equal(t, 0, result.Tokens)
	assert.Nil(t, result.Error)
	assert.False(t, result.StartedAt.IsZero())
	assert.True(t, result.CompletedAt.IsZero())
}

func TestNewAgentResult_EmptyProvider(t *testing.T) {
	result := NewAgentResult("")

	require.NotNil(t, result)
	assert.Equal(t, "", result.Provider)
	assert.NotNil(t, result.Response)
}

func TestNewAgentResult_VariousProviders(t *testing.T) {
	providers := []string{
		"claude",
		"codex",
		"gemini",
		"opencode",
		"custom",
		"my-custom-llm",
	}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			result := NewAgentResult(provider)
			require.NotNil(t, result)
			assert.Equal(t, provider, result.Provider)
		})
	}
}

// =============================================================================
// AgentResult Duration Tests
// =============================================================================

func TestAgentResult_Duration(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(5*time.Second + 250*time.Millisecond)

	result := AgentResult{
		Provider:    "claude",
		StartedAt:   start,
		CompletedAt: end,
	}

	expected := 5*time.Second + 250*time.Millisecond
	assert.Equal(t, expected, result.Duration())
}

func TestAgentResult_Duration_ZeroTime(t *testing.T) {
	result := AgentResult{}
	assert.Equal(t, time.Duration(0), result.Duration())
}

func TestAgentResult_Duration_NotCompleted(t *testing.T) {
	result := NewAgentResult("claude")
	// CompletedAt is zero, so duration is negative
	duration := result.Duration()
	assert.Less(t, duration, time.Duration(0))
}

func TestAgentResult_Duration_Instant(t *testing.T) {
	now := time.Now()
	result := AgentResult{
		StartedAt:   now,
		CompletedAt: now,
	}
	assert.Equal(t, time.Duration(0), result.Duration())
}

func TestAgentResult_Duration_LongRunning(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)

	result := AgentResult{
		StartedAt:   start,
		CompletedAt: end,
	}

	assert.Equal(t, 10*time.Minute, result.Duration())
}

// =============================================================================
// AgentResult Success Tests
// =============================================================================

func TestAgentResult_Success(t *testing.T) {
	tests := []struct {
		name     string
		result   AgentResult
		expected bool
	}{
		{
			name: "success with nil error",
			result: AgentResult{
				Provider: "claude",
				Output:   "Analysis complete",
				Error:    nil,
			},
			expected: true,
		},
		{
			name: "failure with error",
			result: AgentResult{
				Provider: "claude",
				Error:    errors.New("execution failed"),
			},
			expected: false,
		},
		{
			name: "failure with timeout error",
			result: AgentResult{
				Provider: "codex",
				Error:    errors.New("timeout: agent exceeded 300s"),
			},
			expected: false,
		},
		{
			name: "failure with CLI not found",
			result: AgentResult{
				Provider: "gemini",
				Error:    errors.New("gemini: executable file not found in $PATH"),
			},
			expected: false,
		},
		{
			name:     "empty result",
			result:   AgentResult{},
			expected: true, // nil error = success
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.Success())
		})
	}
}

// =============================================================================
// AgentResult HasJSONResponse Tests
// =============================================================================

func TestAgentResult_HasJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		expected bool
	}{
		{
			name:     "empty response map",
			response: map[string]any{},
			expected: false,
		},
		{
			name:     "nil response",
			response: nil,
			expected: false,
		},
		{
			name: "single key response",
			response: map[string]any{
				"result": "analysis",
			},
			expected: true,
		},
		{
			name: "multiple keys response",
			response: map[string]any{
				"result": "analysis",
				"count":  42,
				"items":  []string{"a", "b"},
			},
			expected: true,
		},
		{
			name: "response with nil value",
			response: map[string]any{
				"result": nil,
			},
			expected: true, // has key, even if value is nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AgentResult{
				Response: tt.response,
			}
			assert.Equal(t, tt.expected, result.HasJSONResponse())
		})
	}
}

// =============================================================================
// AgentResult Output Tests
// =============================================================================

func TestAgentResult_Output(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "simple text",
			output: "Analysis complete",
		},
		{
			name:   "multiline text",
			output: "Line 1\nLine 2\nLine 3",
		},
		{
			name:   "JSON string",
			output: `{"result": "success", "count": 42}`,
		},
		{
			name:   "empty output",
			output: "",
		},
		{
			name:   "large output",
			output: string(make([]byte, 100000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AgentResult{
				Provider: "claude",
				Output:   tt.output,
			}
			assert.Equal(t, tt.output, result.Output)
		})
	}
}

// =============================================================================
// AgentResult Response Tests
// =============================================================================

func TestAgentResult_Response(t *testing.T) {
	result := NewAgentResult("claude")

	// Add parsed JSON response
	result.Response["result"] = "analysis completed"
	result.Response["count"] = 42
	result.Response["items"] = []string{"a", "b", "c"}
	result.Response["metadata"] = map[string]any{
		"duration": 1.5,
		"success":  true,
	}

	assert.Len(t, result.Response, 4)
	assert.Equal(t, "analysis completed", result.Response["result"])
	assert.Equal(t, 42, result.Response["count"])
	assert.Equal(t, []string{"a", "b", "c"}, result.Response["items"])
	assert.True(t, result.HasJSONResponse())
}

func TestAgentResult_Response_NilValue(t *testing.T) {
	result := NewAgentResult("claude")
	result.Response["empty"] = nil

	assert.Len(t, result.Response, 1)
	assert.Nil(t, result.Response["empty"])
	assert.True(t, result.HasJSONResponse())
}

// =============================================================================
// AgentResult Tokens Tests
// =============================================================================

func TestAgentResult_Tokens(t *testing.T) {
	tests := []struct {
		name   string
		tokens int
	}{
		{"zero tokens", 0},
		{"small usage", 100},
		{"medium usage", 4096},
		{"large usage", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AgentResult{
				Provider: "claude",
				Tokens:   tt.tokens,
			}
			assert.Equal(t, tt.tokens, result.Tokens)
		})
	}
}

// =============================================================================
// AgentResult Fields Tests
// =============================================================================

func TestAgentResult_AllFields(t *testing.T) {
	err := errors.New("timeout exceeded")
	start := time.Now()
	end := start.Add(5 * time.Second)

	result := AgentResult{
		Provider: "claude",
		Output:   "Partial analysis",
		Response: map[string]any{
			"status": "timeout",
		},
		Tokens:      2048,
		Error:       err,
		StartedAt:   start,
		CompletedAt: end,
	}

	assert.Equal(t, "claude", result.Provider)
	assert.Equal(t, "Partial analysis", result.Output)
	assert.Len(t, result.Response, 1)
	assert.Equal(t, 2048, result.Tokens)
	assert.Equal(t, err, result.Error)
	assert.Equal(t, start, result.StartedAt)
	assert.Equal(t, end, result.CompletedAt)
	assert.False(t, result.Success())
	assert.True(t, result.HasJSONResponse())
	assert.Equal(t, 5*time.Second, result.Duration())
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
	err := config.Validate()
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

func TestAgentResult_ExecutionLifecycle(t *testing.T) {
	// Simulate a complete agent execution lifecycle

	// Start execution
	result := NewAgentResult("claude")
	assert.Equal(t, "claude", result.Provider)
	assert.False(t, result.StartedAt.IsZero())
	assert.True(t, result.CompletedAt.IsZero())

	// Simulate agent processing
	time.Sleep(10 * time.Millisecond)

	// Capture output
	result.Output = "Security analysis complete. Found 3 issues."

	// Parse JSON response
	result.Response["issues_found"] = 3
	result.Response["severity"] = "medium"
	result.Response["recommendations"] = []string{"Fix XSS", "Add CSRF token", "Validate input"}

	// Record token usage
	result.Tokens = 2048

	// Complete execution
	result.CompletedAt = time.Now()

	// Verify final state
	assert.True(t, result.Success())
	assert.Greater(t, result.Duration(), time.Duration(0))
	assert.NotEmpty(t, result.Output)
	assert.True(t, result.HasJSONResponse())
	assert.Len(t, result.Response, 3)
	assert.Equal(t, 3, result.Response["issues_found"])
	assert.Greater(t, result.Tokens, 0)
}

func TestAgentResult_FailedExecution(t *testing.T) {
	// Simulate a failed agent execution

	result := NewAgentResult("codex")

	// Simulate execution that fails
	result.Error = errors.New("codex: executable file not found in $PATH")
	result.CompletedAt = time.Now()

	// Verify failure state
	assert.False(t, result.Success())
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "not found")
	assert.False(t, result.HasJSONResponse())
}

func TestAgentResult_JSONParseSuccess(t *testing.T) {
	// Simulate successful JSON parsing

	result := NewAgentResult("claude")

	// Raw JSON output
	result.Output = `{"analysis": "complete", "score": 95, "issues": []}`

	// Parsed response
	result.Response["analysis"] = "complete"
	result.Response["score"] = 95
	result.Response["issues"] = []any{}

	result.Tokens = 1024
	result.CompletedAt = time.Now()

	// Verify state
	assert.True(t, result.Success())
	assert.True(t, result.HasJSONResponse())
	assert.Len(t, result.Response, 3)
	assert.Equal(t, "complete", result.Response["analysis"])
	assert.Equal(t, 95, result.Response["score"])
}

func TestAgentResult_TextOnlyResponse(t *testing.T) {
	// Simulate text-only response (no JSON)

	result := NewAgentResult("opencode")

	result.Output = "The code looks good. No major issues found."
	result.Tokens = 512
	result.CompletedAt = time.Now()

	// Response map is empty (no JSON parsed)
	assert.True(t, result.Success())
	assert.False(t, result.HasJSONResponse())
	assert.Empty(t, result.Response)
	assert.NotEmpty(t, result.Output)
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
			err := config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedTimeout, config.GetTimeout())
			}
		})
	}
}

func TestAgentResult_ResponseTypes(t *testing.T) {
	result := NewAgentResult("claude")

	// Test various response types
	result.Response["string"] = "hello"
	result.Response["int"] = 42
	result.Response["float"] = 3.14
	result.Response["bool"] = true
	result.Response["slice"] = []string{"a", "b", "c"}
	result.Response["map"] = map[string]any{"key": "value"}
	result.Response["nil"] = nil
	result.Response["nested"] = map[string]any{
		"level2": map[string]any{
			"level3": "deep",
		},
	}

	assert.Len(t, result.Response, 8)
	assert.IsType(t, "", result.Response["string"])
	assert.IsType(t, 0, result.Response["int"])
	assert.IsType(t, 0.0, result.Response["float"])
	assert.IsType(t, false, result.Response["bool"])
	assert.IsType(t, []string{}, result.Response["slice"])
	assert.IsType(t, map[string]any{}, result.Response["map"])
	assert.Nil(t, result.Response["nil"])
	assert.True(t, result.HasJSONResponse())
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
			err := config.Validate()
			require.NoError(t, err)
			assert.Equal(t, tt.prompt, config.Prompt)
		})
	}
}

// =============================================================================
// Component: agent_config_extension
// Feature: F033 - Agent Conversations
// =============================================================================

// =============================================================================
// AgentConfig Mode Validation Tests
// =============================================================================

func TestAgentConfig_Mode_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  AgentConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "single mode with prompt",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Analyze this code",
				Mode:     "single",
			},
			wantErr: false,
		},
		{
			name: "conversation mode with initial_prompt",
			config: AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Start review",
				SystemPrompt:  "You are a code reviewer",
			},
			wantErr: false,
		},
		{
			name: "conversation mode with prompt only",
			config: AgentConfig{
				Provider: "claude",
				Mode:     "conversation",
				Prompt:   "Start review",
			},
			wantErr: false,
		},
		{
			name: "conversation mode with both prompts",
			config: AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				Prompt:        "Fallback prompt",
				InitialPrompt: "Start review",
			},
			wantErr: false,
		},
		{
			name: "empty mode defaults to single",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Mode:     "",
			},
			wantErr: false,
		},
		{
			name: "whitespace mode defaults to single",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Mode:     "   ",
			},
			wantErr: false,
		},
		{
			name: "uppercase SINGLE mode normalized",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Mode:     "SINGLE",
			},
			wantErr: false,
		},
		{
			name: "uppercase CONVERSATION mode normalized",
			config: AgentConfig{
				Provider:      "claude",
				InitialPrompt: "Test",
				Mode:          "CONVERSATION",
			},
			wantErr: false,
		},
		{
			name: "mixed case Single mode normalized",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Mode:     "Single",
			},
			wantErr: false,
		},
		{
			name: "invalid mode",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Mode:     "invalid",
			},
			wantErr: true,
			errMsg:  "mode must be 'single' or 'conversation'",
		},
		{
			name: "typo in mode",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Mode:     "conversaton",
			},
			wantErr: true,
			errMsg:  "mode",
		},
		{
			name: "conversation mode missing all prompts",
			config: AgentConfig{
				Provider: "claude",
				Mode:     "conversation",
			},
			wantErr: true,
			errMsg:  "initial_prompt or prompt is required",
		},
		{
			name: "conversation mode with empty initial_prompt and prompt",
			config: AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "",
				Prompt:        "",
			},
			wantErr: true,
			errMsg:  "initial_prompt or prompt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
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

// =============================================================================
// AgentConfig Conversation Field Tests
// =============================================================================

func TestAgentConfig_ConversationField(t *testing.T) {
	tests := []struct {
		name         string
		config       AgentConfig
		wantErr      bool
		errMsg       string
		validateConv bool
	}{
		{
			name: "conversation mode with valid config",
			config: AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Start",
				Conversation: &ConversationConfig{
					MaxTurns:         10,
					MaxContextTokens: 100000,
					Strategy:         StrategySlidingWindow,
				},
			},
			wantErr:      false,
			validateConv: true,
		},
		{
			name: "conversation mode with nil conversation config",
			config: AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Start",
				Conversation:  nil,
			},
			wantErr: false,
		},
		{
			name: "single mode ignores conversation config",
			config: AgentConfig{
				Provider: "claude",
				Mode:     "single",
				Prompt:   "Test",
				Conversation: &ConversationConfig{
					MaxTurns: 10,
				},
			},
			wantErr: false,
		},
		{
			name: "conversation mode with invalid conversation config",
			config: AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Start",
				Conversation: &ConversationConfig{
					MaxTurns: -1, // Invalid
				},
			},
			wantErr: true,
			errMsg:  "max_turns",
		},
		{
			name: "conversation mode with stop condition",
			config: AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Review code",
				Conversation: &ConversationConfig{
					MaxTurns:      5,
					StopCondition: "response contains 'APPROVED'",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				if tt.validateConv && tt.config.Conversation != nil {
					assert.NotNil(t, tt.config.Conversation)
				}
			}
		})
	}
}

// =============================================================================
// AgentConfig SystemPrompt Tests
// =============================================================================

func TestAgentConfig_SystemPrompt(t *testing.T) {
	tests := []struct {
		name         string
		systemPrompt string
		mode         string
		wantErr      bool
	}{
		{
			name:         "conversation mode with system prompt",
			systemPrompt: "You are a helpful code reviewer",
			mode:         "conversation",
			wantErr:      false,
		},
		{
			name:         "conversation mode with multiline system prompt",
			systemPrompt: "You are a code reviewer.\nBe thorough.\nBe concise.",
			mode:         "conversation",
			wantErr:      false,
		},
		{
			name:         "conversation mode with empty system prompt",
			systemPrompt: "",
			mode:         "conversation",
			wantErr:      false,
		},
		{
			name:         "single mode with system prompt (ignored)",
			systemPrompt: "You are a helper",
			mode:         "single",
			wantErr:      false,
		},
		{
			name:         "conversation mode with long system prompt",
			systemPrompt: string(make([]byte, 10000)),
			mode:         "conversation",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:      "claude",
				Mode:          tt.mode,
				SystemPrompt:  tt.systemPrompt,
				InitialPrompt: "Test",
				Prompt:        "Test",
			}
			err := config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.systemPrompt, config.SystemPrompt)
			}
		})
	}
}

// =============================================================================
// AgentConfig InitialPrompt Tests
// =============================================================================

func TestAgentConfig_InitialPrompt(t *testing.T) {
	tests := []struct {
		name          string
		initialPrompt string
		prompt        string
		mode          string
		wantErr       bool
		errMsg        string
	}{
		{
			name:          "conversation mode with initial prompt",
			initialPrompt: "Start reviewing",
			mode:          "conversation",
			wantErr:       false,
		},
		{
			name:          "conversation mode with template in initial prompt",
			initialPrompt: "Review this: {{inputs.code}}",
			mode:          "conversation",
			wantErr:       false,
		},
		{
			name:          "conversation mode prefers initial_prompt over prompt",
			initialPrompt: "Initial message",
			prompt:        "Fallback message",
			mode:          "conversation",
			wantErr:       false,
		},
		{
			name:          "conversation mode falls back to prompt",
			initialPrompt: "",
			prompt:        "Fallback message",
			mode:          "conversation",
			wantErr:       false,
		},
		{
			name:          "single mode ignores initial_prompt",
			initialPrompt: "Ignored",
			prompt:        "Used",
			mode:          "single",
			wantErr:       false,
		},
		{
			name:          "conversation mode with multiline initial prompt",
			initialPrompt: "Review this code:\n{{inputs.code}}\n\nBe thorough.",
			mode:          "conversation",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:      "claude",
				Mode:          tt.mode,
				InitialPrompt: tt.initialPrompt,
				Prompt:        tt.prompt,
			}
			err := config.Validate()
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

// =============================================================================
// AgentConfig IsConversationMode Tests
// =============================================================================

func TestAgentConfig_IsConversationMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected bool
	}{
		{
			name:     "single mode",
			mode:     "single",
			expected: false,
		},
		{
			name:     "conversation mode",
			mode:     "conversation",
			expected: true,
		},
		{
			name:     "empty mode (defaults to single after Validate)",
			mode:     "",
			expected: false,
		},
		{
			name:     "uppercase CONVERSATION",
			mode:     "CONVERSATION",
			expected: false, // Not normalized yet
		},
		{
			name:     "normalized conversation after validate",
			mode:     "conversation",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider: "claude",
				Mode:     tt.mode,
				Prompt:   "Test",
			}
			// For tests checking pre-validation behavior
			if tt.mode == "" || tt.mode == "CONVERSATION" {
				assert.Equal(t, tt.expected, config.IsConversationMode())
			} else {
				// For tests after normalization
				_ = config.Validate()
				assert.Equal(t, tt.expected, config.IsConversationMode())
			}
		})
	}
}

func TestAgentConfig_IsConversationMode_AfterValidation(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected bool
	}{
		{"empty defaults to single", "", false},
		{"whitespace defaults to single", "   ", false},
		{"SINGLE normalized to single", "SINGLE", false},
		{"CONVERSATION normalized to conversation", "CONVERSATION", true},
		{"Single normalized to single", "Single", false},
		{"Conversation normalized to conversation", "Conversation", true},
		{"single remains single", "single", false},
		{"conversation remains conversation", "conversation", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:      "claude",
				Mode:          tt.mode,
				Prompt:        "Test",
				InitialPrompt: "Test",
			}
			_ = config.Validate() // Normalize mode
			assert.Equal(t, tt.expected, config.IsConversationMode())
		})
	}
}

// =============================================================================
// AgentConfig GetEffectivePrompt Tests
// =============================================================================

func TestAgentConfig_GetEffectivePrompt(t *testing.T) {
	tests := []struct {
		name           string
		mode           string
		prompt         string
		initialPrompt  string
		expectedPrompt string
	}{
		{
			name:           "single mode uses prompt",
			mode:           "single",
			prompt:         "Main prompt",
			initialPrompt:  "Initial prompt",
			expectedPrompt: "Main prompt",
		},
		{
			name:           "conversation mode prefers initial_prompt",
			mode:           "conversation",
			prompt:         "Fallback prompt",
			initialPrompt:  "Initial message",
			expectedPrompt: "Initial message",
		},
		{
			name:           "conversation mode falls back to prompt",
			mode:           "conversation",
			prompt:         "Fallback prompt",
			initialPrompt:  "",
			expectedPrompt: "Fallback prompt",
		},
		{
			name:           "single mode ignores initial_prompt",
			mode:           "single",
			prompt:         "Main prompt",
			initialPrompt:  "Ignored",
			expectedPrompt: "Main prompt",
		},
		{
			name:           "conversation mode with both prompts",
			mode:           "conversation",
			prompt:         "Not used",
			initialPrompt:  "Used this one",
			expectedPrompt: "Used this one",
		},
		{
			name:           "empty mode defaults to single behavior",
			mode:           "",
			prompt:         "Main prompt",
			initialPrompt:  "Initial",
			expectedPrompt: "Main prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:      "claude",
				Mode:          tt.mode,
				Prompt:        tt.prompt,
				InitialPrompt: tt.initialPrompt,
			}
			_ = config.Validate() // Normalize mode
			assert.Equal(t, tt.expectedPrompt, config.GetEffectivePrompt())
		})
	}
}

func TestAgentConfig_GetEffectivePrompt_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		config         AgentConfig
		expectedPrompt string
	}{
		{
			name: "both prompts empty in conversation mode",
			config: AgentConfig{
				Mode:          "conversation",
				Prompt:        "",
				InitialPrompt: "",
			},
			expectedPrompt: "",
		},
		{
			name: "whitespace initial_prompt in conversation mode",
			config: AgentConfig{
				Mode:          "conversation",
				Prompt:        "Fallback",
				InitialPrompt: "   ",
			},
			expectedPrompt: "   ", // Returns as-is
		},
		{
			name: "multiline prompts",
			config: AgentConfig{
				Mode:          "conversation",
				Prompt:        "Line 1\nLine 2",
				InitialPrompt: "Init Line 1\nInit Line 2",
			},
			expectedPrompt: "Init Line 1\nInit Line 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedPrompt, tt.config.GetEffectivePrompt())
		})
	}
}

// =============================================================================
// AgentResult Conversation Field Tests
// =============================================================================

func TestAgentResult_ConversationField(t *testing.T) {
	tests := []struct {
		name         string
		conversation *ConversationResult
	}{
		{
			name:         "nil conversation result",
			conversation: nil,
		},
		{
			name: "empty conversation result",
			conversation: &ConversationResult{
				Provider: "claude",
				State: &ConversationState{
					Turns:       []Turn{},
					TotalTurns:  0,
					TotalTokens: 0,
				},
			},
		},
		{
			name: "conversation with turns",
			conversation: &ConversationResult{
				Provider: "claude",
				State: &ConversationState{
					Turns: []Turn{
						{Role: TurnRoleSystem, Content: "You are helpful", Tokens: 10},
						{Role: TurnRoleUser, Content: "Hello", Tokens: 5},
						{Role: TurnRoleAssistant, Content: "Hi there", Tokens: 8},
					},
					TotalTurns:  3,
					TotalTokens: 23,
					StoppedBy:   StopReasonMaxTurns,
				},
			},
		},
		{
			name: "conversation stopped by condition",
			conversation: &ConversationResult{
				Provider: "claude",
				State: &ConversationState{
					Turns: []Turn{
						{Role: TurnRoleUser, Content: "Review", Tokens: 5},
						{Role: TurnRoleAssistant, Content: "APPROVED", Tokens: 10},
					},
					TotalTurns:  2,
					TotalTokens: 15,
					StoppedBy:   StopReasonCondition,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &AgentResult{
				Provider:     "claude",
				Output:       "Final response",
				Conversation: tt.conversation,
			}

			if tt.conversation == nil {
				assert.Nil(t, result.Conversation)
			} else {
				require.NotNil(t, result.Conversation)
				require.NotNil(t, result.Conversation.State)
				assert.Equal(t, tt.conversation.State.TotalTurns, result.Conversation.State.TotalTurns)
				assert.Equal(t, tt.conversation.State.TotalTokens, result.Conversation.State.TotalTokens)
				assert.Equal(t, tt.conversation.State.StoppedBy, result.Conversation.State.StoppedBy)
				assert.Equal(t, len(tt.conversation.State.Turns), len(result.Conversation.State.Turns))
			}
		})
	}
}

func TestAgentResult_ConversationField_Integration(t *testing.T) {
	// Simulate a complete conversation execution
	result := NewAgentResult("claude")

	// Set up conversation result
	result.Conversation = &ConversationResult{
		Provider: "claude",
		State: &ConversationState{
			Turns: []Turn{
				{Role: TurnRoleSystem, Content: "You are a code reviewer", Tokens: 50},
				{Role: TurnRoleUser, Content: "Review this code: {{code}}", Tokens: 500},
				{Role: TurnRoleAssistant, Content: "I found 3 issues...", Tokens: 800},
				{Role: TurnRoleUser, Content: "Fix the issues", Tokens: 20},
				{Role: TurnRoleAssistant, Content: "Here's the fixed code... APPROVED", Tokens: 600},
			},
			TotalTurns:  5,
			TotalTokens: 1970,
			StoppedBy:   StopReasonCondition,
		},
		Output:      "Here's the fixed code... APPROVED",
		TokensTotal: 1970,
		CompletedAt: time.Now(),
	}

	// Set overall result fields
	result.Output = "Here's the fixed code... APPROVED"
	result.Tokens = 1970
	result.CompletedAt = time.Now()

	// Verify
	assert.True(t, result.Success())
	require.NotNil(t, result.Conversation)
	require.NotNil(t, result.Conversation.State)
	assert.Equal(t, 5, result.Conversation.State.TotalTurns)
	assert.Equal(t, 1970, result.Conversation.State.TotalTokens)
	assert.Equal(t, StopReasonCondition, result.Conversation.State.StoppedBy)
	assert.Len(t, result.Conversation.State.Turns, 5)
	assert.Equal(t, TurnRoleSystem, result.Conversation.State.Turns[0].Role)
	assert.Equal(t, TurnRoleAssistant, result.Conversation.State.Turns[4].Role)
}

// =============================================================================
// Integration Tests - Conversation Mode
// =============================================================================

func TestAgentConfig_ConversationMode_Complete(t *testing.T) {
	config := AgentConfig{
		Provider:     "claude",
		Mode:         "conversation",
		SystemPrompt: "You are a helpful code reviewer. Iterate until code meets standards.",
		InitialPrompt: `Review this code:
{{inputs.code}}

Say "APPROVED" when done.`,
		Options: map[string]any{
			"model":      "claude-sonnet-4-20250514",
			"max_tokens": 4096,
		},
		Timeout: 300,
		Conversation: &ConversationConfig{
			MaxTurns:         10,
			MaxContextTokens: 100000,
			Strategy:         StrategySlidingWindow,
			StopCondition:    "response contains 'APPROVED'",
		},
	}

	// Validate
	err := config.Validate()
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, "claude", config.Provider)
	assert.True(t, config.IsConversationMode())
	assert.Contains(t, config.SystemPrompt, "code reviewer")
	assert.Contains(t, config.InitialPrompt, "{{inputs.code}}")
	assert.Contains(t, config.GetEffectivePrompt(), "Review this code")
	require.NotNil(t, config.Conversation)
	assert.Equal(t, 10, config.Conversation.MaxTurns)
	assert.Equal(t, 100000, config.Conversation.MaxContextTokens)
	assert.Equal(t, StrategySlidingWindow, config.Conversation.Strategy)
	assert.Contains(t, config.Conversation.StopCondition, "APPROVED")
}

func TestAgentConfig_ConversationMode_MinimalConfig(t *testing.T) {
	config := AgentConfig{
		Provider:      "claude",
		Mode:          "conversation",
		InitialPrompt: "Hello",
	}

	err := config.Validate()
	require.NoError(t, err)
	assert.True(t, config.IsConversationMode())
	assert.Equal(t, "Hello", config.GetEffectivePrompt())
	assert.Nil(t, config.Conversation)
}

func TestAgentConfig_SingleMode_BackwardCompatibility(t *testing.T) {
	// Existing single-mode config should work without Mode field
	config := AgentConfig{
		Provider: "claude",
		Prompt:   "Analyze this code",
		Options: map[string]any{
			"model": "claude-sonnet-4-20250514",
		},
	}

	err := config.Validate()
	require.NoError(t, err)
	assert.False(t, config.IsConversationMode())
	assert.Equal(t, "single", config.Mode) // Normalized to "single"
	assert.Equal(t, "Analyze this code", config.GetEffectivePrompt())
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestAgentConfig_ConversationMode_Errors(t *testing.T) {
	tests := []struct {
		name    string
		config  AgentConfig
		wantErr string
	}{
		{
			name: "conversation mode without prompt",
			config: AgentConfig{
				Provider: "claude",
				Mode:     "conversation",
			},
			wantErr: "initial_prompt or prompt is required",
		},
		{
			name: "invalid mode value",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Mode:     "streaming",
			},
			wantErr: "mode must be 'single' or 'conversation'",
		},
		{
			name: "conversation with invalid config",
			config: AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Test",
				Conversation: &ConversationConfig{
					MaxTurns: -1, // Invalid: negative
				},
			},
			wantErr: "max_turns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
