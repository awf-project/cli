package repository

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: 39 - Agent Step Type

// parseStepType Tests - Agent Case

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

// mapAgentConfigFlat Tests - Happy Path

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
			got := mapAgentConfigFlat(&tt.yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Provider, got.Provider)
			assert.Equal(t, tt.want.Prompt, got.Prompt)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}

// mapAgentConfigFlat Tests - Edge Cases

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
			got := mapAgentConfigFlat(&tt.yamlStep)

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

// mapStep Integration Tests - Agent Type

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
					Backoff:      "exponential",
					Multiplier:   func() *float64 { v := 2.0; return &v }(),
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
			step, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, step)
			assert.Equal(t, "test_step", step.Name)

			tt.wantStep(t, step)
		})
	}
}

// mapStep Integration Tests - Agent Preserves Other Fields

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

	step, err := mapStep("test.yaml", "agent_step", &yamlStep)

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

func TestMapStep_AgentStep_InvalidTimeout(t *testing.T) {
	yamlStep := yamlStep{
		Type:     "agent",
		Provider: "claude",
		Prompt:   "Test",
		Options:  map[string]any{"model": "claude-3-5-sonnet-20241022"},
		Timeout:  "invalid",
	}

	step, err := mapStep("test.yaml", "test_step", &yamlStep)

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

	step, err := mapStep("test.yaml", "test_step", &yamlStep)

	require.NoError(t, err)
	require.NotNil(t, step)
	assert.Equal(t, workflow.StepTypeAgent, step.Type)

	// Agent config should be nil when provider is empty
	assert.Nil(t, step.Agent)
}

// Feature: F033/F083 - Agent Conversations

// mapConversationConfig Tests - Happy Path
// F083: ConversationConfig only retains ContinueFrom. Removed fields
// (max_turns, max_context_tokens, strategy, stop_condition, inject_context)
// now produce parse errors via validateConversationConfigRemovedFields.

func TestMapConversationConfig_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		yamlConfig *yamlConversationConfig
		want       *workflow.ConversationConfig
	}{
		{
			name:       "nil config returns nil",
			yamlConfig: nil,
			want:       nil,
		},
		{
			name: "minimal config with only ContinueFrom",
			yamlConfig: &yamlConversationConfig{
				ContinueFrom: "previous_step",
			},
			want: &workflow.ConversationConfig{
				ContinueFrom: "previous_step",
			},
		},
		{
			name:       "empty config returns non-nil with empty ContinueFrom",
			yamlConfig: &yamlConversationConfig{},
			want: &workflow.ConversationConfig{
				ContinueFrom: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapConversationConfig(tt.yamlConfig)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.want.ContinueFrom, got.ContinueFrom)
		})
	}
}

// mapConversationConfig Tests - Edge Cases
// F083: Removed fields produce parse errors; only ContinueFrom is mapped.

func TestMapConversationConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		yamlConfig   *yamlConversationConfig
		wantNil      bool
		wantContinue string
	}{
		{
			name:    "nil config returns nil",
			wantNil: true,
		},
		{
			name:       "all fields empty returns config with empty ContinueFrom",
			yamlConfig: &yamlConversationConfig{},
		},
		{
			name: "only continue_from is mapped",
			yamlConfig: &yamlConversationConfig{
				ContinueFrom: "step_one",
			},
			wantContinue: "step_one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapConversationConfig(tt.yamlConfig)

			if tt.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tt.wantContinue, got.ContinueFrom)
		})
	}
}

// mapAgentConfigFlat Tests - Conversation Mode Happy Path

func TestMapAgentConfigFlat_ConversationMode_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "conversation mode with system_prompt and prompt",
			yamlStep: yamlStep{
				Provider:     "claude",
				Mode:         "conversation",
				SystemPrompt: "You are a code reviewer. Iterate until quality standards are met.",
				Prompt:       "Review this code:\n{{inputs.code}}",
				Options: map[string]any{
					"model":      "claude-sonnet-4-20250514",
					"max_tokens": 4096,
				},
				Conversation: &yamlConversationConfig{
					ContinueFrom: "",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Mode:         "conversation",
				SystemPrompt: "You are a code reviewer. Iterate until quality standards are met.",
				Prompt:       "Review this code:\n{{inputs.code}}",
				Options: map[string]any{
					"model":      "claude-sonnet-4-20250514",
					"max_tokens": 4096,
				},
				Conversation: &workflow.ConversationConfig{
					ContinueFrom: "",
				},
			},
		},
		{
			name: "conversation mode with continue_from",
			yamlStep: yamlStep{
				Provider: "claude",
				Mode:     "conversation",
				Prompt:   "Also consider these requirements:\n{{inputs.additional_requirements}}",
				Conversation: &yamlConversationConfig{
					ContinueFrom: "refine_code",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Mode:     "conversation",
				Prompt:   "Also consider these requirements:\n{{inputs.additional_requirements}}",
				Conversation: &workflow.ConversationConfig{
					ContinueFrom: "refine_code",
				},
			},
		},
		{
			name: "conversation mode with prompt only",
			yamlStep: yamlStep{
				Provider:     "gemini",
				Mode:         "conversation",
				SystemPrompt: "You are an expert assistant.",
				Prompt:       "Help me with {{inputs.task}}",
			},
			want: &workflow.AgentConfig{
				Provider:     "gemini",
				Mode:         "conversation",
				SystemPrompt: "You are an expert assistant.",
				Prompt:       "Help me with {{inputs.task}}",
			},
		},
		{
			name: "conversation mode minimal config",
			yamlStep: yamlStep{
				Provider:     "codex",
				Mode:         "conversation",
				Prompt:       "Generate tests",
				Conversation: &yamlConversationConfig{},
			},
			want: &workflow.AgentConfig{
				Provider:     "codex",
				Mode:         "conversation",
				Prompt:       "Generate tests",
				Conversation: &workflow.ConversationConfig{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAgentConfigFlat(&tt.yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Provider, got.Provider)
			assert.Equal(t, tt.want.Mode, got.Mode)
			assert.Equal(t, tt.want.SystemPrompt, got.SystemPrompt)
			assert.Equal(t, tt.want.Prompt, got.Prompt)

			if tt.want.Conversation != nil {
				require.NotNil(t, got.Conversation)
				assert.Equal(t, tt.want.Conversation.ContinueFrom, got.Conversation.ContinueFrom)
			} else {
				assert.Nil(t, got.Conversation)
			}
		})
	}
}

// mapAgentConfigFlat Tests - Conversation Mode Edge Cases

func TestMapAgentConfigFlat_ConversationMode_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "conversation mode without conversation config",
			yamlStep: yamlStep{
				Provider:     "claude",
				Mode:         "conversation",
				SystemPrompt: "You are helpful.",
				Prompt:       "Hello",
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Mode:         "conversation",
				SystemPrompt: "You are helpful.",
				Prompt:       "Hello",
				Conversation: nil,
			},
		},
		{
			name: "single mode with conversation config (only ContinueFrom mapped)",
			yamlStep: yamlStep{
				Provider: "claude",
				Mode:     "single",
				Prompt:   "Do task",
				Conversation: &yamlConversationConfig{
					ContinueFrom: "prior",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Mode:     "single",
				Prompt:   "Do task",
				Conversation: &workflow.ConversationConfig{
					ContinueFrom: "prior",
				},
			},
		},
		{
			name: "no mode specified (defaults at domain level)",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "Do task",
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Prompt:       "Do task",
				Mode:         "",
				Conversation: nil,
			},
		},
		{
			name: "conversation mode with multiline system_prompt",
			yamlStep: yamlStep{
				Provider: "claude",
				Mode:     "conversation",
				SystemPrompt: `You are a code reviewer.

Follow these guidelines:
1. Check for bugs
2. Verify coding standards
3. Suggest improvements`,
				Prompt: "Review: {{inputs.code}}",
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Mode:     "conversation",
				SystemPrompt: `You are a code reviewer.

Follow these guidelines:
1. Check for bugs
2. Verify coding standards
3. Suggest improvements`,
				Prompt: "Review: {{inputs.code}}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAgentConfigFlat(&tt.yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Provider, got.Provider)
			assert.Equal(t, tt.want.Mode, got.Mode)
			assert.Equal(t, tt.want.SystemPrompt, got.SystemPrompt)
			assert.Equal(t, tt.want.Prompt, got.Prompt)

			if tt.want.Conversation == nil {
				assert.Nil(t, got.Conversation)
			} else {
				require.NotNil(t, got.Conversation)
				assert.Equal(t, tt.want.Conversation.ContinueFrom, got.Conversation.ContinueFrom)
			}
		})
	}
}

// mapStep Integration Tests - Agent Conversation Mode

func TestMapStep_AgentConversationMode(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		wantErr  bool
		wantStep func(*testing.T, *workflow.Step)
	}{
		{
			name: "conversation mode with prompt and continue_from",
			yamlStep: yamlStep{
				Type:         "agent",
				Description:  "Iterative code review",
				Provider:     "claude",
				Mode:         "conversation",
				SystemPrompt: "You are a code reviewer.",
				Prompt:       "Review this code:\n{{inputs.code}}",
				Options: map[string]any{
					"model":      "claude-sonnet-4-20250514",
					"max_tokens": 4096,
				},
				Conversation: &yamlConversationConfig{
					ContinueFrom: "",
				},
				Timeout:   "10m",
				OnSuccess: "deploy",
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)
				assert.Equal(t, "Iterative code review", step.Description)
				assert.Equal(t, 600, step.Timeout)
				assert.Equal(t, "deploy", step.OnSuccess)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "claude", step.Agent.Provider)
				assert.Equal(t, "conversation", step.Agent.Mode)
				assert.Equal(t, "You are a code reviewer.", step.Agent.SystemPrompt)
				assert.Equal(t, "Review this code:\n{{inputs.code}}", step.Agent.Prompt)

				require.NotNil(t, step.Agent.Conversation)
				assert.Equal(t, "", step.Agent.Conversation.ContinueFrom)
			},
		},
		{
			name: "conversation mode with continue_from",
			yamlStep: yamlStep{
				Type:     "agent",
				Provider: "claude",
				Mode:     "conversation",
				Prompt:   "Also consider: {{inputs.requirements}}",
				Conversation: &yamlConversationConfig{
					ContinueFrom: "refine_code",
				},
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "conversation", step.Agent.Mode)

				require.NotNil(t, step.Agent.Conversation)
				assert.Equal(t, "refine_code", step.Agent.Conversation.ContinueFrom)
			},
		},
		{
			name: "conversation mode with hooks and retry",
			yamlStep: yamlStep{
				Type:         "agent",
				Provider:     "gemini",
				Mode:         "conversation",
				SystemPrompt: "You are helpful.",
				Prompt:       "Start task",
				Hooks: &yamlStepHooks{
					Pre: []yamlHookAction{
						{Log: "Starting conversation"},
					},
					Post: []yamlHookAction{
						{Log: "Conversation completed"},
					},
				},
				Retry: &yamlRetry{
					MaxAttempts:  3,
					InitialDelay: "2s",
				},
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "conversation", step.Agent.Mode)

				require.NotNil(t, step.Hooks.Pre)
				require.NotNil(t, step.Hooks.Post)
				require.NotNil(t, step.Retry)
				assert.Equal(t, 3, step.Retry.MaxAttempts)
			},
		},
		{
			name: "conversation mode with transitions",
			yamlStep: yamlStep{
				Type:         "agent",
				Provider:     "claude",
				Mode:         "conversation",
				SystemPrompt: "Classify sentiment.",
				Prompt:       "Classify: {{inputs.text}}",
				Transitions: []yamlTransition{
					{When: "states.classify.output contains 'positive'", Goto: "handle_positive"},
					{When: "states.classify.output contains 'negative'", Goto: "handle_negative"},
					{Goto: "handle_neutral"},
				},
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "conversation", step.Agent.Mode)

				require.Len(t, step.Transitions, 3)
				assert.Equal(t, "handle_positive", step.Transitions[0].Goto)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, step)
			assert.Equal(t, "test_step", step.Name)

			tt.wantStep(t, step)
		})
	}
}

// mapRetry Tests

func TestMapRetry_NilInput(t *testing.T) {
	got, err := mapRetry(nil)

	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestMapRetry_HappyPath(t *testing.T) {
	tests := []struct {
		name string
		y    *yamlRetry
		want *workflow.RetryConfig
	}{
		{
			name: "all fields specified with multiplier",
			y: &yamlRetry{
				MaxAttempts:        3,
				InitialDelay:       "100ms",
				MaxDelay:           "5s",
				Backoff:            "exponential",
				Multiplier:         ptr(2.5),
				Jitter:             0.1,
				RetryableExitCodes: []int{1, 2},
			},
			want: &workflow.RetryConfig{
				MaxAttempts:        3,
				InitialDelayMs:     100,
				MaxDelayMs:         5000,
				Backoff:            "exponential",
				Multiplier:         2.5,
				Jitter:             0.1,
				RetryableExitCodes: []int{1, 2},
			},
		},
		{
			name: "multiplier omitted defaults to 2.0",
			y: &yamlRetry{
				MaxAttempts:  2,
				InitialDelay: "50ms",
				MaxDelay:     "2s",
				Backoff:      "linear",
				Multiplier:   nil,
				Jitter:       0.05,
			},
			want: &workflow.RetryConfig{
				MaxAttempts:    2,
				InitialDelayMs: 50,
				MaxDelayMs:     2000,
				Backoff:        "linear",
				Multiplier:     2.0,
				Jitter:         0.05,
			},
		},
		{
			name: "durations in go format",
			y: &yamlRetry{
				MaxAttempts:  2,
				InitialDelay: "1s",
				MaxDelay:     "10s",
				Backoff:      "constant",
			},
			want: &workflow.RetryConfig{
				MaxAttempts:    2,
				InitialDelayMs: 1000,
				MaxDelayMs:     10000,
				Backoff:        "constant",
				Multiplier:     2.0,
			},
		},
		{
			name: "durations as integer seconds",
			y: &yamlRetry{
				MaxAttempts:  3,
				InitialDelay: "30",
				MaxDelay:     "120",
				Backoff:      "exponential",
			},
			want: &workflow.RetryConfig{
				MaxAttempts:    3,
				InitialDelayMs: 30000,
				MaxDelayMs:     120000,
				Backoff:        "exponential",
				Multiplier:     2.0,
			},
		},
		{
			name: "empty delays default to 0ms",
			y: &yamlRetry{
				MaxAttempts:        1,
				InitialDelay:       "",
				MaxDelay:           "",
				Backoff:            "constant",
				RetryableExitCodes: nil,
			},
			want: &workflow.RetryConfig{
				MaxAttempts:        1,
				InitialDelayMs:     0,
				MaxDelayMs:         0,
				Backoff:            "constant",
				Multiplier:         2.0,
				RetryableExitCodes: nil,
			},
		},
		{
			name: "multiplier explicit zero is preserved",
			y: &yamlRetry{
				MaxAttempts:  1,
				InitialDelay: "100ms",
				Multiplier:   ptr(0.0),
			},
			want: &workflow.RetryConfig{
				MaxAttempts:    1,
				InitialDelayMs: 100,
				Multiplier:     0.0,
			},
		},
		{
			name: "millisecond durations",
			y: &yamlRetry{
				MaxAttempts:  2,
				InitialDelay: "500ms",
				MaxDelay:     "30s",
			},
			want: &workflow.RetryConfig{
				MaxAttempts:    2,
				InitialDelayMs: 500,
				MaxDelayMs:     30000,
				Multiplier:     2.0,
			},
		},
		{
			name: "minute durations",
			y: &yamlRetry{
				MaxAttempts:  2,
				InitialDelay: "1m",
				MaxDelay:     "5m",
			},
			want: &workflow.RetryConfig{
				MaxAttempts:    2,
				InitialDelayMs: 60000,
				MaxDelayMs:     300000,
				Multiplier:     2.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapRetry(tt.y)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.MaxAttempts, got.MaxAttempts)
			assert.Equal(t, tt.want.InitialDelayMs, got.InitialDelayMs)
			assert.Equal(t, tt.want.MaxDelayMs, got.MaxDelayMs)
			assert.Equal(t, tt.want.Backoff, got.Backoff)
			assert.Equal(t, tt.want.Multiplier, got.Multiplier)
			assert.Equal(t, tt.want.Jitter, got.Jitter)
			assert.Equal(t, tt.want.RetryableExitCodes, got.RetryableExitCodes)
		})
	}
}

func TestMapRetry_ErrorPaths(t *testing.T) {
	tests := []struct {
		name        string
		y           *yamlRetry
		wantErr     bool
		errContains string
	}{
		{
			name: "invalid initial_delay format",
			y: &yamlRetry{
				InitialDelay: "not-a-duration",
				MaxAttempts:  1,
			},
			wantErr:     true,
			errContains: "initial_delay",
		},
		{
			name: "invalid max_delay format",
			y: &yamlRetry{
				InitialDelay: "100ms",
				MaxDelay:     "invalid",
				MaxAttempts:  1,
			},
			wantErr:     true,
			errContains: "max_delay",
		},
		{
			name: "initial_delay with invalid characters",
			y: &yamlRetry{
				InitialDelay: "100xs",
				MaxAttempts:  1,
			},
			wantErr:     true,
			errContains: "initial_delay",
		},
		{
			name: "max_delay with invalid characters",
			y: &yamlRetry{
				MaxDelay:    "5z",
				MaxAttempts: 1,
			},
			wantErr:     true,
			errContains: "max_delay",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapRetry(tt.y)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

// parseDuration Tests

func TestParseDuration_HappyPath(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int64
	}{
		{
			name: "milliseconds format",
			s:    "100ms",
			want: 100000000,
		},
		{
			name: "seconds format",
			s:    "30s",
			want: 30000000000,
		},
		{
			name: "minutes format",
			s:    "2m",
			want: 120000000000,
		},
		{
			name: "hours format",
			s:    "1h",
			want: 3600000000000,
		},
		{
			name: "combined format",
			s:    "1m30s",
			want: 90000000000,
		},
		{
			name: "integer as seconds",
			s:    "60",
			want: 60000000000,
		},
		{
			name: "single digit integer",
			s:    "5",
			want: 5000000000,
		},
		{
			name: "zero",
			s:    "0",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.s)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Nanoseconds())
		})
	}
}

func TestParseDuration_ErrorPaths(t *testing.T) {
	tests := []struct {
		name string
		s    string
	}{
		{
			name: "invalid format",
			s:    "not-a-duration",
		},
		{
			name: "invalid unit",
			s:    "100xs",
		},
		{
			name: "invalid characters",
			s:    "abc",
		},
		{
			name: "float seconds without unit",
			s:    "1.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.s)

			require.Error(t, err)
			assert.Equal(t, int64(0), got.Nanoseconds())
		})
	}
}

// Helper function to create a pointer to float64
func ptr(f float64) *float64 {
	return &f
}
