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

// =============================================================================
// Component: yaml_conversation_parsing
// Feature: F033 - Agent Conversations
// =============================================================================

// =============================================================================
// mapConversationConfig Tests - Happy Path
// =============================================================================

func TestMapConversationConfig_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		yamlConfig *yamlConversationConfig
		want       *workflow.ConversationConfig
	}{
		{
			name: "full conversation config with sliding_window",
			yamlConfig: &yamlConversationConfig{
				MaxTurns:         10,
				MaxContextTokens: 100000,
				Strategy:         "sliding_window",
				StopCondition:    "response contains 'APPROVED'",
				ContinueFrom:     "",
				InjectContext:    "",
			},
			want: &workflow.ConversationConfig{
				MaxTurns:         10,
				MaxContextTokens: 100000,
				Strategy:         workflow.StrategySlidingWindow,
				StopCondition:    "response contains 'APPROVED'",
				ContinueFrom:     "",
				InjectContext:    "",
			},
		},
		{
			name: "conversation config with summarize strategy",
			yamlConfig: &yamlConversationConfig{
				MaxTurns:         20,
				MaxContextTokens: 50000,
				Strategy:         "summarize",
				StopCondition:    "response contains 'DONE'",
			},
			want: &workflow.ConversationConfig{
				MaxTurns:         20,
				MaxContextTokens: 50000,
				Strategy:         workflow.StrategySummarize,
				StopCondition:    "response contains 'DONE'",
			},
		},
		{
			name: "conversation config with truncate_middle strategy",
			yamlConfig: &yamlConversationConfig{
				MaxTurns:         15,
				MaxContextTokens: 75000,
				Strategy:         "truncate_middle",
				StopCondition:    "states.refine.output == 'complete'",
			},
			want: &workflow.ConversationConfig{
				MaxTurns:         15,
				MaxContextTokens: 75000,
				Strategy:         workflow.StrategyTruncateMiddle,
				StopCondition:    "states.refine.output == 'complete'",
			},
		},
		{
			name: "minimal conversation config with defaults",
			yamlConfig: &yamlConversationConfig{
				MaxTurns: 5,
			},
			want: &workflow.ConversationConfig{
				MaxTurns:         5,
				MaxContextTokens: 0,
				Strategy:         workflow.StrategyNone,
				StopCondition:    "",
			},
		},
		{
			name: "conversation config with continue_from",
			yamlConfig: &yamlConversationConfig{
				MaxTurns:      10,
				ContinueFrom:  "previous_conversation",
				InjectContext: "Also consider: {{inputs.requirements}}",
			},
			want: &workflow.ConversationConfig{
				MaxTurns:      10,
				Strategy:      workflow.StrategyNone,
				ContinueFrom:  "previous_conversation",
				InjectContext: "Also consider: {{inputs.requirements}}",
			},
		},
		{
			name: "max conversation turns 100",
			yamlConfig: &yamlConversationConfig{
				MaxTurns:         100,
				MaxContextTokens: 200000,
				Strategy:         "sliding_window",
			},
			want: &workflow.ConversationConfig{
				MaxTurns:         100,
				MaxContextTokens: 200000,
				Strategy:         workflow.StrategySlidingWindow,
			},
		},
		{
			name: "large token limit",
			yamlConfig: &yamlConversationConfig{
				MaxTurns:         10,
				MaxContextTokens: 1000000,
				Strategy:         "sliding_window",
			},
			want: &workflow.ConversationConfig{
				MaxTurns:         10,
				MaxContextTokens: 1000000,
				Strategy:         workflow.StrategySlidingWindow,
			},
		},
		{
			name: "complex stop condition expression",
			yamlConfig: &yamlConversationConfig{
				MaxTurns:      10,
				StopCondition: "(response contains 'SUCCESS' || response contains 'APPROVED') && states.validate.status == 'passed'",
			},
			want: &workflow.ConversationConfig{
				MaxTurns:      10,
				Strategy:      workflow.StrategyNone,
				StopCondition: "(response contains 'SUCCESS' || response contains 'APPROVED') && states.validate.status == 'passed'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapConversationConfig(tt.yamlConfig)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.MaxTurns, got.MaxTurns)
			assert.Equal(t, tt.want.MaxContextTokens, got.MaxContextTokens)
			assert.Equal(t, tt.want.Strategy, got.Strategy)
			assert.Equal(t, tt.want.StopCondition, got.StopCondition)
			assert.Equal(t, tt.want.ContinueFrom, got.ContinueFrom)
			assert.Equal(t, tt.want.InjectContext, got.InjectContext)
		})
	}
}

// =============================================================================
// mapConversationConfig Tests - Edge Cases
// =============================================================================

func TestMapConversationConfig_EdgeCases(t *testing.T) {
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
			name: "empty strategy defaults to StrategyNone",
			yamlConfig: &yamlConversationConfig{
				MaxTurns: 10,
				Strategy: "",
			},
			want: &workflow.ConversationConfig{
				MaxTurns: 10,
				Strategy: workflow.StrategyNone,
			},
		},
		{
			name: "unknown strategy defaults to StrategyNone",
			yamlConfig: &yamlConversationConfig{
				MaxTurns: 10,
				Strategy: "unknown_strategy",
			},
			want: &workflow.ConversationConfig{
				MaxTurns: 10,
				Strategy: workflow.StrategyNone,
			},
		},
		{
			name: "zero max_turns",
			yamlConfig: &yamlConversationConfig{
				MaxTurns: 0,
				Strategy: "sliding_window",
			},
			want: &workflow.ConversationConfig{
				MaxTurns: 0,
				Strategy: workflow.StrategySlidingWindow,
			},
		},
		{
			name: "zero max_context_tokens",
			yamlConfig: &yamlConversationConfig{
				MaxTurns:         10,
				MaxContextTokens: 0,
				Strategy:         "sliding_window",
			},
			want: &workflow.ConversationConfig{
				MaxTurns:         10,
				MaxContextTokens: 0,
				Strategy:         workflow.StrategySlidingWindow,
			},
		},
		{
			name:       "all fields empty",
			yamlConfig: &yamlConversationConfig{},
			want: &workflow.ConversationConfig{
				MaxTurns:         0,
				MaxContextTokens: 0,
				Strategy:         workflow.StrategyNone,
				StopCondition:    "",
				ContinueFrom:     "",
				InjectContext:    "",
			},
		},
		{
			name: "empty strings for text fields",
			yamlConfig: &yamlConversationConfig{
				MaxTurns:      10,
				StopCondition: "",
				ContinueFrom:  "",
				InjectContext: "",
			},
			want: &workflow.ConversationConfig{
				MaxTurns:      10,
				Strategy:      workflow.StrategyNone,
				StopCondition: "",
				ContinueFrom:  "",
				InjectContext: "",
			},
		},
		{
			name: "very long inject_context",
			yamlConfig: &yamlConversationConfig{
				MaxTurns: 10,
				InjectContext: `This is a very long context injection that might include:
- Multiple lines of instructions
- Code blocks and examples
- Template variables like {{inputs.data}} and {{states.prev.output}}
- Special characters: <>&"'
- And much more detailed information`,
			},
			want: &workflow.ConversationConfig{
				MaxTurns: 10,
				Strategy: workflow.StrategyNone,
				InjectContext: `This is a very long context injection that might include:
- Multiple lines of instructions
- Code blocks and examples
- Template variables like {{inputs.data}} and {{states.prev.output}}
- Special characters: <>&"'
- And much more detailed information`,
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
			assert.Equal(t, tt.want.MaxTurns, got.MaxTurns)
			assert.Equal(t, tt.want.MaxContextTokens, got.MaxContextTokens)
			assert.Equal(t, tt.want.Strategy, got.Strategy)
			assert.Equal(t, tt.want.StopCondition, got.StopCondition)
			assert.Equal(t, tt.want.ContinueFrom, got.ContinueFrom)
			assert.Equal(t, tt.want.InjectContext, got.InjectContext)
		})
	}
}

// =============================================================================
// mapAgentConfigFlat Tests - Conversation Mode Happy Path
// =============================================================================

func TestMapAgentConfigFlat_ConversationMode_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "conversation mode with system_prompt and initial_prompt",
			yamlStep: yamlStep{
				Provider:      "claude",
				Mode:          "conversation",
				SystemPrompt:  "You are a code reviewer. Iterate until quality standards are met.",
				InitialPrompt: "Review this code:\n{{inputs.code}}",
				Options: map[string]any{
					"model":      "claude-sonnet-4-20250514",
					"max_tokens": 4096,
				},
				Conversation: &yamlConversationConfig{
					MaxTurns:         10,
					MaxContextTokens: 100000,
					Strategy:         "sliding_window",
					StopCondition:    "response contains 'APPROVED'",
				},
			},
			want: &workflow.AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				SystemPrompt:  "You are a code reviewer. Iterate until quality standards are met.",
				InitialPrompt: "Review this code:\n{{inputs.code}}",
				Options: map[string]any{
					"model":      "claude-sonnet-4-20250514",
					"max_tokens": 4096,
				},
				Conversation: &workflow.ConversationConfig{
					MaxTurns:         10,
					MaxContextTokens: 100000,
					Strategy:         workflow.StrategySlidingWindow,
					StopCondition:    "response contains 'APPROVED'",
				},
			},
		},
		{
			name: "conversation mode with continue_from",
			yamlStep: yamlStep{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Also consider these requirements:\n{{inputs.additional_requirements}}",
				Conversation: &yamlConversationConfig{
					MaxTurns:     5,
					ContinueFrom: "refine_code",
				},
			},
			want: &workflow.AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Also consider these requirements:\n{{inputs.additional_requirements}}",
				Conversation: &workflow.ConversationConfig{
					MaxTurns:     5,
					ContinueFrom: "refine_code",
					Strategy:     workflow.StrategyNone,
				},
			},
		},
		{
			name: "conversation mode with all strategies",
			yamlStep: yamlStep{
				Provider:     "gemini",
				Mode:         "conversation",
				SystemPrompt: "You are an expert assistant.",
				Prompt:       "Help me with {{inputs.task}}",
				Conversation: &yamlConversationConfig{
					MaxTurns:         20,
					MaxContextTokens: 50000,
					Strategy:         "summarize",
					StopCondition:    "response contains 'COMPLETE'",
					InjectContext:    "Remember to follow best practices.",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "gemini",
				Mode:         "conversation",
				SystemPrompt: "You are an expert assistant.",
				Prompt:       "Help me with {{inputs.task}}",
				Conversation: &workflow.ConversationConfig{
					MaxTurns:         20,
					MaxContextTokens: 50000,
					Strategy:         workflow.StrategySummarize,
					StopCondition:    "response contains 'COMPLETE'",
					InjectContext:    "Remember to follow best practices.",
				},
			},
		},
		{
			name: "conversation mode minimal config",
			yamlStep: yamlStep{
				Provider: "codex",
				Mode:     "conversation",
				Prompt:   "Generate tests",
				Conversation: &yamlConversationConfig{
					MaxTurns: 5,
				},
			},
			want: &workflow.AgentConfig{
				Provider: "codex",
				Mode:     "conversation",
				Prompt:   "Generate tests",
				Conversation: &workflow.ConversationConfig{
					MaxTurns: 5,
					Strategy: workflow.StrategyNone,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAgentConfigFlat(tt.yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Provider, got.Provider)
			assert.Equal(t, tt.want.Mode, got.Mode)
			assert.Equal(t, tt.want.SystemPrompt, got.SystemPrompt)
			assert.Equal(t, tt.want.InitialPrompt, got.InitialPrompt)
			assert.Equal(t, tt.want.Prompt, got.Prompt)

			if tt.want.Conversation != nil {
				require.NotNil(t, got.Conversation)
				assert.Equal(t, tt.want.Conversation.MaxTurns, got.Conversation.MaxTurns)
				assert.Equal(t, tt.want.Conversation.MaxContextTokens, got.Conversation.MaxContextTokens)
				assert.Equal(t, tt.want.Conversation.Strategy, got.Conversation.Strategy)
				assert.Equal(t, tt.want.Conversation.StopCondition, got.Conversation.StopCondition)
				assert.Equal(t, tt.want.Conversation.ContinueFrom, got.Conversation.ContinueFrom)
				assert.Equal(t, tt.want.Conversation.InjectContext, got.Conversation.InjectContext)
			} else {
				assert.Nil(t, got.Conversation)
			}
		})
	}
}

// =============================================================================
// mapAgentConfigFlat Tests - Conversation Mode Edge Cases
// =============================================================================

func TestMapAgentConfigFlat_ConversationMode_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "conversation mode without conversation config",
			yamlStep: yamlStep{
				Provider:      "claude",
				Mode:          "conversation",
				SystemPrompt:  "You are helpful.",
				InitialPrompt: "Hello",
			},
			want: &workflow.AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				SystemPrompt:  "You are helpful.",
				InitialPrompt: "Hello",
				Conversation:  nil,
			},
		},
		{
			name: "single mode with conversation config (should still map)",
			yamlStep: yamlStep{
				Provider: "claude",
				Mode:     "single",
				Prompt:   "Do task",
				Conversation: &yamlConversationConfig{
					MaxTurns: 10,
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Mode:     "single",
				Prompt:   "Do task",
				Conversation: &workflow.ConversationConfig{
					MaxTurns: 10,
					Strategy: workflow.StrategyNone,
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
			name: "conversation mode with empty system_prompt",
			yamlStep: yamlStep{
				Provider:      "claude",
				Mode:          "conversation",
				SystemPrompt:  "",
				InitialPrompt: "Start conversation",
				Conversation: &yamlConversationConfig{
					MaxTurns: 10,
				},
			},
			want: &workflow.AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				SystemPrompt:  "",
				InitialPrompt: "Start conversation",
				Conversation: &workflow.ConversationConfig{
					MaxTurns: 10,
					Strategy: workflow.StrategyNone,
				},
			},
		},
		{
			name: "conversation mode with both prompt and initial_prompt",
			yamlStep: yamlStep{
				Provider:      "claude",
				Mode:          "conversation",
				Prompt:        "Fallback prompt",
				InitialPrompt: "Initial message",
				Conversation: &yamlConversationConfig{
					MaxTurns: 10,
				},
			},
			want: &workflow.AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				Prompt:        "Fallback prompt",
				InitialPrompt: "Initial message",
				Conversation: &workflow.ConversationConfig{
					MaxTurns: 10,
					Strategy: workflow.StrategyNone,
				},
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
3. Suggest improvements

Say "APPROVED" when satisfied.`,
				InitialPrompt: "Review: {{inputs.code}}",
				Conversation: &yamlConversationConfig{
					MaxTurns:      10,
					StopCondition: "response contains 'APPROVED'",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Mode:     "conversation",
				SystemPrompt: `You are a code reviewer.

Follow these guidelines:
1. Check for bugs
2. Verify coding standards
3. Suggest improvements

Say "APPROVED" when satisfied.`,
				InitialPrompt: "Review: {{inputs.code}}",
				Conversation: &workflow.ConversationConfig{
					MaxTurns:      10,
					StopCondition: "response contains 'APPROVED'",
					Strategy:      workflow.StrategyNone,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAgentConfigFlat(tt.yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Provider, got.Provider)
			assert.Equal(t, tt.want.Mode, got.Mode)
			assert.Equal(t, tt.want.SystemPrompt, got.SystemPrompt)
			assert.Equal(t, tt.want.InitialPrompt, got.InitialPrompt)
			assert.Equal(t, tt.want.Prompt, got.Prompt)

			if tt.want.Conversation == nil {
				assert.Nil(t, got.Conversation)
			} else {
				require.NotNil(t, got.Conversation)
				assert.Equal(t, tt.want.Conversation.MaxTurns, got.Conversation.MaxTurns)
				assert.Equal(t, tt.want.Conversation.Strategy, got.Conversation.Strategy)
			}
		})
	}
}

// =============================================================================
// mapStep Integration Tests - Agent Conversation Mode
// =============================================================================

func TestMapStep_AgentConversationMode(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		wantStep func(*testing.T, *workflow.Step)
	}{
		{
			name: "full conversation mode step",
			yamlStep: yamlStep{
				Type:          "agent",
				Description:   "Iterative code review",
				Provider:      "claude",
				Mode:          "conversation",
				SystemPrompt:  "You are a code reviewer.",
				InitialPrompt: "Review this code:\n{{inputs.code}}",
				Options: map[string]any{
					"model":      "claude-sonnet-4-20250514",
					"max_tokens": 4096,
				},
				Conversation: &yamlConversationConfig{
					MaxTurns:         10,
					MaxContextTokens: 100000,
					Strategy:         "sliding_window",
					StopCondition:    "response contains 'APPROVED'",
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
				assert.Equal(t, "Review this code:\n{{inputs.code}}", step.Agent.InitialPrompt)

				require.NotNil(t, step.Agent.Conversation)
				assert.Equal(t, 10, step.Agent.Conversation.MaxTurns)
				assert.Equal(t, 100000, step.Agent.Conversation.MaxContextTokens)
				assert.Equal(t, workflow.StrategySlidingWindow, step.Agent.Conversation.Strategy)
				assert.Equal(t, "response contains 'APPROVED'", step.Agent.Conversation.StopCondition)
			},
		},
		{
			name: "conversation mode with continue_from",
			yamlStep: yamlStep{
				Type:          "agent",
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Also consider: {{inputs.requirements}}",
				Conversation: &yamlConversationConfig{
					MaxTurns:      5,
					ContinueFrom:  "refine_code",
					InjectContext: "Focus on performance.",
				},
			},
			wantStep: func(t *testing.T, step *workflow.Step) {
				assert.Equal(t, workflow.StepTypeAgent, step.Type)

				require.NotNil(t, step.Agent)
				assert.Equal(t, "conversation", step.Agent.Mode)

				require.NotNil(t, step.Agent.Conversation)
				assert.Equal(t, 5, step.Agent.Conversation.MaxTurns)
				assert.Equal(t, "refine_code", step.Agent.Conversation.ContinueFrom)
				assert.Equal(t, "Focus on performance.", step.Agent.Conversation.InjectContext)
			},
		},
		{
			name: "conversation mode with hooks and retry",
			yamlStep: yamlStep{
				Type:          "agent",
				Provider:      "gemini",
				Mode:          "conversation",
				SystemPrompt:  "You are helpful.",
				InitialPrompt: "Start task",
				Conversation: &yamlConversationConfig{
					MaxTurns: 10,
					Strategy: "summarize",
				},
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
				require.NotNil(t, step.Agent.Conversation)
				assert.Equal(t, workflow.StrategySummarize, step.Agent.Conversation.Strategy)

				require.NotNil(t, step.Hooks.Pre)
				require.NotNil(t, step.Hooks.Post)
				require.NotNil(t, step.Retry)
				assert.Equal(t, 3, step.Retry.MaxAttempts)
			},
		},
		{
			name: "conversation mode with transitions",
			yamlStep: yamlStep{
				Type:          "agent",
				Provider:      "claude",
				Mode:          "conversation",
				SystemPrompt:  "Classify sentiment.",
				InitialPrompt: "Classify: {{inputs.text}}",
				Conversation: &yamlConversationConfig{
					MaxTurns:      3,
					StopCondition: "response contains 'CLASSIFICATION:'",
				},
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
			step, err := mapStep("test.yaml", "test_step", tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, step)
			assert.Equal(t, "test_step", step.Name)

			tt.wantStep(t, step)
		})
	}
}

// =============================================================================
// Strategy Mapping Tests
// =============================================================================

func TestMapConversationConfig_StrategyMapping(t *testing.T) {
	tests := []struct {
		yamlStrategy   string
		domainStrategy workflow.ContextWindowStrategy
	}{
		{"sliding_window", workflow.StrategySlidingWindow},
		{"summarize", workflow.StrategySummarize},
		{"truncate_middle", workflow.StrategyTruncateMiddle},
		{"", workflow.StrategyNone},
		{"SLIDING_WINDOW", workflow.StrategyNone}, // case-sensitive
		{"invalid", workflow.StrategyNone},
		{"sliding-window", workflow.StrategyNone}, // exact match required
	}

	for _, tt := range tests {
		t.Run("strategy_"+tt.yamlStrategy, func(t *testing.T) {
			yamlConfig := &yamlConversationConfig{
				MaxTurns: 10,
				Strategy: tt.yamlStrategy,
			}

			got := mapConversationConfig(yamlConfig)

			require.NotNil(t, got)
			assert.Equal(t, tt.domainStrategy, got.Strategy)
		})
	}
}
