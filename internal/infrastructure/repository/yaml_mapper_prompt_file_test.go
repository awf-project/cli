package repository

import (
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F063 - Prompt File Loading for Agent Steps

// mapAgentConfigFlat Tests - PromptFile Happy Path

func TestMapAgentConfigFlat_PromptFile_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "relative path to prompt file",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "prompts/analyze.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "prompts/analyze.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "tilde expansion in prompt file path",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "~/custom/prompt.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "~/custom/prompt.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "awf template variable in prompt file path",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "{{.awf.prompts_dir}}/plan/research.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "{{.awf.prompts_dir}}/plan/research.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "absolute path to prompt file",
			yamlStep: yamlStep{
				Provider:   "gemini",
				PromptFile: "/var/lib/awf/prompts/system.md",
				Options: map[string]any{
					"model": "gemini-pro",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "gemini",
				PromptFile: "/var/lib/awf/prompts/system.md",
				Options: map[string]any{
					"model": "gemini-pro",
				},
			},
		},
		{
			name: "prompt file with nested directory structure",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "prompts/feature-001/agent/step-02.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "prompts/feature-001/agent/step-02.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "prompt file with template interpolation in path",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "{{.awf.config_dir}}/prompts/{{.inputs.feature}}.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "{{.awf.config_dir}}/prompts/{{.inputs.feature}}.md",
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
			assert.Equal(t, tt.want.PromptFile, got.PromptFile)
			assert.Equal(t, tt.want.Prompt, got.Prompt)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}

// mapAgentConfigFlat Tests - PromptFile Edge Cases

func TestMapAgentConfigFlat_PromptFile_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "empty prompt file field",
			yamlStep: yamlStep{
				Provider:   "claude",
				Prompt:     "Inline prompt text",
				PromptFile: "",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				Prompt:     "Inline prompt text",
				PromptFile: "",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "prompt file with only filename no extension",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "prompt",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "prompt",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "prompt file with special characters in path",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "prompts/my-prompt_v2.0.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "prompts/my-prompt_v2.0.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "prompt file with spaces in path",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "prompts/my prompt file.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "prompts/my prompt file.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "prompt file with unicode characters in path",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "prompts/インストール手順.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "prompts/インストール手順.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "prompt file with multiple path separators",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "prompts//subdir///file.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "prompts//subdir///file.md",
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
			assert.Equal(t, tt.want.PromptFile, got.PromptFile)
			assert.Equal(t, tt.want.Prompt, got.Prompt)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}

// mapAgentConfigFlat Tests - PromptFile Mutual Exclusivity

func TestMapAgentConfigFlat_PromptFile_MutualExclusivity(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "both prompt and prompt file set",
			yamlStep: yamlStep{
				Provider:   "claude",
				Prompt:     "Inline prompt text",
				PromptFile: "prompts/analyze.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				Prompt:     "Inline prompt text",
				PromptFile: "prompts/analyze.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "only prompt set",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "Inline prompt text",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
				Prompt:   "Inline prompt text",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "only prompt file set",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "prompts/analyze.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "prompts/analyze.md",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "neither prompt nor prompt file set",
			yamlStep: yamlStep{
				Provider: "claude",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider: "claude",
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
			assert.Equal(t, tt.want.PromptFile, got.PromptFile)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}

// mapAgentConfigFlat Tests - PromptFile with Conversation Mode

func TestMapAgentConfigFlat_PromptFile_WithConversationMode(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "conversation mode with prompt file",
			yamlStep: yamlStep{
				Provider:      "claude",
				PromptFile:    "prompts/conversation.md",
				Mode:          "conversation",
				SystemPrompt:  "You are a helpful assistant",
				InitialPrompt: "Hello, I need help with {{.inputs.task}}",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:      "claude",
				PromptFile:    "prompts/conversation.md",
				Mode:          "conversation",
				SystemPrompt:  "You are a helpful assistant",
				InitialPrompt: "Hello, I need help with {{.inputs.task}}",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "single mode with prompt file",
			yamlStep: yamlStep{
				Provider:   "claude",
				PromptFile: "prompts/single.md",
				Mode:       "single",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:   "claude",
				PromptFile: "prompts/single.md",
				Mode:       "single",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "conversation mode with both initial prompt and prompt file",
			yamlStep: yamlStep{
				Provider:      "claude",
				PromptFile:    "prompts/base.md",
				InitialPrompt: "Override prompt",
				Mode:          "conversation",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:      "claude",
				PromptFile:    "prompts/base.md",
				InitialPrompt: "Override prompt",
				Mode:          "conversation",
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
			assert.Equal(t, tt.want.PromptFile, got.PromptFile)
			assert.Equal(t, tt.want.Mode, got.Mode)
			assert.Equal(t, tt.want.SystemPrompt, got.SystemPrompt)
			assert.Equal(t, tt.want.InitialPrompt, got.InitialPrompt)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}
