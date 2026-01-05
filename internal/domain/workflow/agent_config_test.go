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
