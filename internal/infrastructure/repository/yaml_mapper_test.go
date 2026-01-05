package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// Component: yaml_parsing
// Feature: 39 - Agent Step Type
// =============================================================================

// =============================================================================
// parseStepType Tests - Agent Case
// =============================================================================

func TestParseStepType_Agent(t *testing.T) {
	tests := []struct {
		input   string
		want    workflow.StepType
		wantErr bool
	}{
		{
			input: "agent",
			want:  workflow.StepTypeAgent,
		},
		{
			input: "AGENT",
			want:  workflow.StepTypeAgent,
		},
		{
			input: "Agent",
			want:  workflow.StepTypeAgent,
		},
		{
			input: "AgEnT",
			want:  workflow.StepTypeAgent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseStepType(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// mapAgentConfigFlat Tests - Happy Path
// =============================================================================

func TestMapAgentConfigFlat_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "claude provider with basic options",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "Analyze this code: {{inputs.code}}",
				Options: map[string]any{
					"model":       "claude-3-5-sonnet-20241022",
					"temperature": 0.7,
					"max_tokens":  4096,
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Prompt:   "Analyze this code: {{inputs.code}}",
				Options: map[string]any{
					"model":       "claude-3-5-sonnet-20241022",
					"temperature": 0.7,
					"max_tokens":  4096,
				},
			},
		},
		{
			name: "codex provider with interpolation",
			yamlStep: yamlStep{
				Provider: "codex",
				Prompt:   "Generate unit tests for {{states.parse.output}}",
				Options: map[string]any{
					"model":       "gpt-4",
					"temperature": 0.5,
				},
			},
			want: &workflow.AgentConfig{
				Provider: "codex",
				Prompt:   "Generate unit tests for {{states.parse.output}}",
				Options: map[string]any{
					"model":       "gpt-4",
					"temperature": 0.5,
				},
			},
		},
		{
			name: "gemini provider with minimal config",
			yamlStep: yamlStep{
				Provider: "gemini",
				Prompt:   "Summarize: {{inputs.text}}",
				Options:  map[string]any{},
			},
			want: &workflow.AgentConfig{
				Provider: "gemini",
				Prompt:   "Summarize: {{inputs.text}}",
				Options:  map[string]any{},
			},
		},
		{
			name: "opencode provider",
			yamlStep: yamlStep{
				Provider: "opencode",
				Prompt:   "Review PR: {{inputs.pr_diff}}",
				Options: map[string]any{
					"model": "deepseek-coder",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "opencode",
				Prompt:   "Review PR: {{inputs.pr_diff}}",
				Options: map[string]any{
					"model": "deepseek-coder",
				},
			},
		},
		{
			name: "custom provider",
			yamlStep: yamlStep{
				Provider: "custom",
				Prompt:   "Process {{inputs.data}}",
				Options: map[string]any{
					"endpoint": "http://localhost:8080/api",
					"api_key":  "SECRET_KEY",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "custom",
				Prompt:   "Process {{inputs.data}}",
				Options: map[string]any{
					"endpoint": "http://localhost:8080/api",
					"api_key":  "SECRET_KEY",
				},
			},
		},
		{
			name: "prompt with multiple interpolations",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "Compare {{inputs.file1}} and {{inputs.file2}}, focusing on {{states.analysis.output}}",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Prompt:   "Compare {{inputs.file1}} and {{inputs.file2}}, focusing on {{states.analysis.output}}",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "prompt without interpolation",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "Write a hello world program in Python",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Prompt:   "Write a hello world program in Python",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAgentConfigFlat(tt.yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Provider, got.Provider)
			assert.Equal(t, tt.want.Prompt, got.Prompt)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}

// =============================================================================
// mapAgentConfigFlat Tests - Edge Cases
// =============================================================================

func TestMapAgentConfigFlat_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "nil when provider is empty",
			yamlStep: yamlStep{
				Provider: "",
				Prompt:   "Some prompt",
				Options:  map[string]any{},
			},
			want: nil,
		},
		{
			name: "provider set but empty prompt",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Prompt:   "",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "provider set but nil options",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "Test prompt",
				Options:  nil,
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Prompt:   "Test prompt",
				Options:  nil,
			},
		},
		{
			name: "very long prompt",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt: `This is a very long prompt that might contain multiple paragraphs.

It includes code blocks:
` + "```go\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```" + `

And templates: {{inputs.data1}}, {{inputs.data2}}, {{states.step1.output}}

And more text explaining the task in great detail.`,
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Prompt: `This is a very long prompt that might contain multiple paragraphs.

It includes code blocks:
` + "```go\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```" + `

And templates: {{inputs.data1}}, {{inputs.data2}}, {{states.step1.output}}

And more text explaining the task in great detail.`,
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "complex nested options",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "Test",
				Options: map[string]any{
					"model":       "claude-3-5-sonnet-20241022",
					"temperature": 0.7,
					"max_tokens":  4096,
					"metadata": map[string]any{
						"user_id": "123",
						"tags":    []string{"test", "development"},
					},
					"stop_sequences": []string{"\n\nHuman:", "\n\nAssistant:"},
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Options: map[string]any{
					"model":       "claude-3-5-sonnet-20241022",
					"temperature": 0.7,
					"max_tokens":  4096,
					"metadata": map[string]any{
						"user_id": "123",
						"tags":    []string{"test", "development"},
					},
					"stop_sequences": []string{"\n\nHuman:", "\n\nAssistant:"},
				},
			},
		},
		{
			name: "special characters in prompt",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "Test with 'quotes' and \"double quotes\" and {{inputs.var}} and $env",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Prompt:   "Test with 'quotes' and \"double quotes\" and {{inputs.var}} and $env",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "numeric values in options",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "Test",
				Options: map[string]any{
					"temperature":  0.0,
					"max_tokens":   1,
					"top_p":        1.0,
					"top_k":        0,
					"presence":     -2.0,
					"frequency":    2.0,
					"repeat_count": 999999,
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Options: map[string]any{
					"temperature":  0.0,
					"max_tokens":   1,
					"top_p":        1.0,
					"top_k":        0,
					"presence":     -2.0,
					"frequency":    2.0,
					"repeat_count": 999999,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAgentConfigFlat(tt.yamlStep)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Provider, got.Provider)
			assert.Equal(t, tt.want.Prompt, got.Prompt)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}

// =============================================================================
// mapStep Integration Tests - Agent Type
// =============================================================================

func TestMapStep_AgentStep(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		wantStep func(*testing.T, *workflow.Step)
	}{
		{
			name: "basic agent step with claude",
			yamlStep: yamlStep{
				Type:        "agent",
				Description: "Analyze code with Claude",
				Provider:    "claude",
				Prompt:      "Review this code: {{inputs.code}}",
				Options: map[string]any{
					"model":       "claude-3-5-sonnet-20241022",
					"temperature": 0.7,
				},
				OnSuccess: "next_step",
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)
				assert.Equal(t, "Analyze code with Claude", step.Description)
				assert.Equal(t, "next_step", step.OnSuccess)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "claude", step.Agent.Provider)
				assert.Equal(t, "Review this code: {{inputs.code}}", step.Agent.Prompt)
				assert.Equal(t, "claude-3-5-sonnet-20241022", step.Agent.Options["model"])
				assert.Equal(t, 0.7, step.Agent.Options["temperature"])
			},
		},
		{
			name: "agent step with timeout",
			yamlStep: yamlStep{
				Type:     "agent",
				Provider: "codex",
				Prompt:   "Generate tests for {{states.parse.output}}",
				Options: map[string]any{
					"model": "gpt-4",
				},
				Timeout:   "5m",
				OnFailure: "error_handler",
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)
				assert.Equal(t, 300, step.Timeout) // 5 minutes = 300 seconds
				assert.Equal(t, "error_handler", step.OnFailure)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "codex", step.Agent.Provider)
				assert.Equal(t, "Generate tests for {{states.parse.output}}", step.Agent.Prompt)
			},
		},
		{
			name: "agent step with hooks",
			yamlStep: yamlStep{
				Type:     "agent",
				Provider: "gemini",
				Prompt:   "Summarize: {{inputs.text}}",
				Options:  map[string]any{"model": "gemini-pro"},
				Hooks: &yamlStepHooks{
					Pre: []yamlHookAction{
						{Command: "echo 'Starting agent'"},
					},
					Post: []yamlHookAction{
						{Command: "echo 'Agent completed'"},
					},
				},
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "gemini", step.Agent.Provider)

				require.NotNil(t, step.Hooks.Pre)
				assert.Len(t, step.Hooks.Pre, 1)
				require.NotNil(t, step.Hooks.Post)
				assert.Len(t, step.Hooks.Post, 1)
			},
		},
		{
			name: "agent step with retry config",
			yamlStep: yamlStep{
				Type:     "agent",
				Provider: "claude",
				Prompt:   "Process {{inputs.data}}",
				Options:  map[string]any{"model": "claude-3-5-sonnet-20241022"},
				Retry: &yamlRetry{
					MaxAttempts:  3,
					InitialDelay: "1s",
					MaxDelay:     "10s",
					Backoff:      "2.0",
					Multiplier:   2.0,
				},
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "claude", step.Agent.Provider)

				require.NotNil(t, step.Retry)
				assert.Equal(t, 3, step.Retry.MaxAttempts)
				assert.Equal(t, 1000, step.Retry.InitialDelayMs)
				assert.Equal(t, 10000, step.Retry.MaxDelayMs)
				assert.Equal(t, 2.0, step.Retry.Multiplier)
			},
		},
		{
			name: "agent step with capture",
			yamlStep: yamlStep{
				Type:     "agent",
				Provider: "opencode",
				Prompt:   "Generate code",
				Options:  map[string]any{"model": "deepseek-coder"},
				Capture: &yamlCapture{
					Stdout: "agent_output",
					Stderr: "agent_errors",
				},
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "opencode", step.Agent.Provider)

				require.NotNil(t, step.Capture)
				assert.Equal(t, "agent_output", step.Capture.Stdout)
				assert.Equal(t, "agent_errors", step.Capture.Stderr)
			},
		},
		{
			name: "agent step with transitions",
			yamlStep: yamlStep{
				Type:     "agent",
				Provider: "claude",
				Prompt:   "Classify: {{inputs.text}}",
				Options:  map[string]any{"model": "claude-3-5-sonnet-20241022"},
				Transitions: []yamlTransition{
					{When: "states.classify.output == 'positive'", Goto: "handle_positive"},
					{When: "states.classify.output == 'negative'", Goto: "handle_negative"},
					{Goto: "handle_neutral"},
				},
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "claude", step.Agent.Provider)

				require.Len(t, step.Transitions, 3)
				assert.Equal(t, "states.classify.output == 'positive'", step.Transitions[0].When)
				assert.Equal(t, "handle_positive", step.Transitions[0].Goto)
			},
		},
		{
			name: "agent step with continue_on_error",
			yamlStep: yamlStep{
				Type:            "agent",
				Provider:        "custom",
				Prompt:          "Try to process {{inputs.data}}",
				Options:         map[string]any{"endpoint": "http://localhost:8080"},
				ContinueOnError: true,
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)
				assert.True(t, step.ContinueOnError)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "custom", step.Agent.Provider)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, err := mapStep("test.yaml", "test_step", tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, step)
			assert.Equal(t, "test_step", step.Name)

			tt.wantStep(t, step)
		})
	}
}

// =============================================================================
// mapStep Integration Tests - Agent Preserves Other Fields
// =============================================================================

func TestMapStep_AgentPreservesOtherFields(t *testing.T) {
	yamlStep := yamlStep{
		Type:        "agent",
		Description: "Complex agent step",
		Provider:    "claude",
		Prompt:      "Process {{inputs.data}}",
		Options: map[string]any{
			"model": "claude-3-5-sonnet-20241022",
		},
		Timeout:         "2m",
		OnSuccess:       "success_state",
		OnFailure:       "failure_state",
		ContinueOnError: false,
		DependsOn:       []string{"step1", "step2"},
	}

	step, err := mapStep("test.yaml", "agent_step", yamlStep)

	require.NoError(t, err)
	require.NotNil(t, step)

	// Verify agent-specific fields
	assert.Equal(t, workflow.StepTypeAgent, step.Type)
	require.NotNil(t, step.Agent)
	assert.Equal(t, "claude", step.Agent.Provider)
	assert.Equal(t, "Process {{inputs.data}}", step.Agent.Prompt)

	// Verify other fields are preserved
	assert.Equal(t, "agent_step", step.Name)
	assert.Equal(t, "Complex agent step", step.Description)
	assert.Equal(t, 120, step.Timeout)
	assert.Equal(t, "success_state", step.OnSuccess)
	assert.Equal(t, "failure_state", step.OnFailure)
	assert.False(t, step.ContinueOnError)
	assert.Equal(t, []string{"step1", "step2"}, step.DependsOn)
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestMapStep_AgentStep_InvalidTimeout(t *testing.T) {
	yamlStep := yamlStep{
		Type:     "agent",
		Provider: "claude",
		Prompt:   "Test",
		Options:  map[string]any{"model": "claude-3-5-sonnet-20241022"},
		Timeout:  "invalid",
	}

	step, err := mapStep("test.yaml", "test_step", yamlStep)

	require.Error(t, err)
	assert.Nil(t, step)
	assert.Contains(t, err.Error(), "invalid duration")
}

func TestMapStep_AgentStep_NoProvider(t *testing.T) {
	yamlStep := yamlStep{
		Type:     "agent",
		Provider: "", // Empty provider
		Prompt:   "Test prompt",
		Options:  map[string]any{"model": "claude-3-5-sonnet-20241022"},
	}

	step, err := mapStep("test.yaml", "test_step", yamlStep)

	require.NoError(t, err)
	require.NotNil(t, step)
	assert.Equal(t, workflow.StepTypeAgent, step.Type)

	// Agent config should be nil when provider is empty
	assert.Nil(t, step.Agent)
}
