package repository

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F065 - Output Format for Agent Steps

// mapAgentConfigFlat Tests - OutputFormat Happy Path

func TestMapAgentConfigFlat_OutputFormat_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "output format json",
			yamlStep: yamlStep{
				Provider:     "claude",
				Prompt:       "Generate JSON response",
				OutputFormat: "json",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Prompt:       "Generate JSON response",
				OutputFormat: workflow.OutputFormat("json"),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "output format text",
			yamlStep: yamlStep{
				Provider:     "claude",
				Prompt:       "Generate text response",
				OutputFormat: "text",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Prompt:       "Generate text response",
				OutputFormat: workflow.OutputFormat("text"),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "output format json with codex provider",
			yamlStep: yamlStep{
				Provider:     "codex",
				Prompt:       "Extract data as JSON",
				OutputFormat: "json",
				Options: map[string]any{
					"model":       "gpt-4",
					"temperature": 0.3,
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "codex",
				Prompt:       "Extract data as JSON",
				OutputFormat: workflow.OutputFormat("json"),
				Options: map[string]any{
					"model":       "gpt-4",
					"temperature": 0.3,
				},
			},
		},
		{
			name: "output format text with gemini provider",
			yamlStep: yamlStep{
				Provider:     "gemini",
				Prompt:       "Explain concept",
				OutputFormat: "text",
				Options: map[string]any{
					"model": "gemini-pro",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "gemini",
				Prompt:       "Explain concept",
				OutputFormat: workflow.OutputFormat("text"),
				Options: map[string]any{
					"model": "gemini-pro",
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
			assert.Equal(t, tt.want.OutputFormat, got.OutputFormat)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}

// mapAgentConfigFlat Tests - OutputFormat Edge Cases

func TestMapAgentConfigFlat_OutputFormat_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "output format not specified defaults to empty",
			yamlStep: yamlStep{
				Provider: "claude",
				Prompt:   "No output format",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Prompt:       "No output format",
				OutputFormat: workflow.OutputFormat(""),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "output format empty string",
			yamlStep: yamlStep{
				Provider:     "claude",
				Prompt:       "Empty format",
				OutputFormat: "",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Prompt:       "Empty format",
				OutputFormat: workflow.OutputFormat(""),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "output format with uppercase JSON",
			yamlStep: yamlStep{
				Provider:     "claude",
				Prompt:       "Generate response",
				OutputFormat: "JSON",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Prompt:       "Generate response",
				OutputFormat: workflow.OutputFormat("JSON"),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "output format with mixed case Text",
			yamlStep: yamlStep{
				Provider:     "claude",
				Prompt:       "Generate response",
				OutputFormat: "Text",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Prompt:       "Generate response",
				OutputFormat: workflow.OutputFormat("Text"),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "output format with whitespace",
			yamlStep: yamlStep{
				Provider:     "claude",
				Prompt:       "Generate response",
				OutputFormat: " json ",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Prompt:       "Generate response",
				OutputFormat: workflow.OutputFormat(" json "),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "invalid output format value",
			yamlStep: yamlStep{
				Provider:     "claude",
				Prompt:       "Generate response",
				OutputFormat: "xml",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Prompt:       "Generate response",
				OutputFormat: workflow.OutputFormat("xml"),
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
			assert.Equal(t, tt.want.OutputFormat, got.OutputFormat)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}

// mapAgentConfigFlat Tests - OutputFormat with Other Fields

func TestMapAgentConfigFlat_OutputFormat_WithPromptFile(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "output format json with prompt file",
			yamlStep: yamlStep{
				Provider:     "claude",
				PromptFile:   "prompts/extract-data.md",
				OutputFormat: "json",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				PromptFile:   "prompts/extract-data.md",
				OutputFormat: workflow.OutputFormat("json"),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "output format text with prompt file",
			yamlStep: yamlStep{
				Provider:     "claude",
				PromptFile:   "prompts/analyze.md",
				OutputFormat: "text",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				PromptFile:   "prompts/analyze.md",
				OutputFormat: workflow.OutputFormat("text"),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "output format with prompt file and inline prompt",
			yamlStep: yamlStep{
				Provider:     "claude",
				Prompt:       "Inline prompt",
				PromptFile:   "prompts/base.md",
				OutputFormat: "json",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Prompt:       "Inline prompt",
				PromptFile:   "prompts/base.md",
				OutputFormat: workflow.OutputFormat("json"),
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
			assert.Equal(t, tt.want.OutputFormat, got.OutputFormat)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}

func TestMapAgentConfigFlat_OutputFormat_WithConversationMode(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.AgentConfig
	}{
		{
			name: "conversation mode with json output format",
			yamlStep: yamlStep{
				Provider:     "claude",
				Mode:         "conversation",
				SystemPrompt: "You are a data extraction assistant",
				Prompt:       "Extract entities from: {{.inputs.text}}",
				OutputFormat: "json",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Mode:         "conversation",
				SystemPrompt: "You are a data extraction assistant",
				Prompt:       "Extract entities from: {{.inputs.text}}",
				OutputFormat: workflow.OutputFormat("json"),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "conversation mode with text output format",
			yamlStep: yamlStep{
				Provider:     "claude",
				Mode:         "conversation",
				SystemPrompt: "You are a helpful assistant",
				Prompt:       "Help with: {{.inputs.task}}",
				OutputFormat: "text",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Mode:         "conversation",
				SystemPrompt: "You are a helpful assistant",
				Prompt:       "Help with: {{.inputs.task}}",
				OutputFormat: workflow.OutputFormat("text"),
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
		},
		{
			name: "single mode with output format",
			yamlStep: yamlStep{
				Provider:     "claude",
				Mode:         "single",
				Prompt:       "Extract data",
				OutputFormat: "json",
				Options: map[string]any{
					"model": "claude-3-5-sonnet-20241022",
				},
			},
			want: &workflow.AgentConfig{
				Provider:     "claude",
				Mode:         "single",
				Prompt:       "Extract data",
				OutputFormat: workflow.OutputFormat("json"),
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
			assert.Equal(t, tt.want.Mode, got.Mode)
			assert.Equal(t, tt.want.SystemPrompt, got.SystemPrompt)
			assert.Equal(t, tt.want.Prompt, got.Prompt)
			assert.Equal(t, tt.want.OutputFormat, got.OutputFormat)
			assert.Equal(t, tt.want.Options, got.Options)
		})
	}
}

func TestMapAgentConfigFlat_OutputFormat_NoProvider(t *testing.T) {
	yamlStep := yamlStep{
		Provider:     "",
		Prompt:       "This should be ignored",
		OutputFormat: "json",
		Options: map[string]any{
			"model": "claude-3-5-sonnet-20241022",
		},
	}

	got := mapAgentConfigFlat(&yamlStep)

	assert.Nil(t, got)
}
