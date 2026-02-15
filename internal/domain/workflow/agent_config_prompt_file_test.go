package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F063 - Prompt File Loading for Agent Steps
// Component: T001 - Add PromptFile field and Validate() mutual exclusivity
// Tests: PromptFile field validation and mutual exclusivity with Prompt

func TestAgentConfig_PromptFile_SingleMode_Valid(t *testing.T) {
	tests := []struct {
		name       string
		promptFile string
	}{
		{
			name:       "relative path",
			promptFile: "prompts/analyze.md",
		},
		{
			name:       "absolute path",
			promptFile: "/etc/awf/prompts/plan.md",
		},
		{
			name:       "tilde path",
			promptFile: "~/prompts/custom.md",
		},
		{
			name:       "template variable path",
			promptFile: "{{.awf.prompts_dir}}/plan/research.md",
		},
		{
			name:       "nested relative path",
			promptFile: "../../shared/prompts/analyze.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:   "claude",
				PromptFile: tt.promptFile,
			}
			err := config.Validate(nil)
			assert.NoError(t, err)
		})
	}
}

func TestAgentConfig_PromptFile_MutualExclusivity_BothSet(t *testing.T) {
	config := AgentConfig{
		Provider:   "claude",
		Prompt:     "Analyze this code",
		PromptFile: "prompts/analyze.md",
	}
	err := config.Validate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestAgentConfig_PromptFile_SingleMode_NeitherSet(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Mode:     "single",
	}
	err := config.Validate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt or prompt_file is required")
}

func TestAgentConfig_PromptFile_SingleMode_OnlyPromptFile(t *testing.T) {
	config := AgentConfig{
		Provider:   "claude",
		PromptFile: "prompts/test.md",
		Mode:       "single",
	}
	err := config.Validate(nil)
	assert.NoError(t, err)
}

func TestAgentConfig_PromptFile_SingleMode_OnlyPrompt(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Prompt:   "Test prompt",
		Mode:     "single",
	}
	err := config.Validate(nil)
	assert.NoError(t, err)
}

func TestAgentConfig_PromptFile_ConversationMode_NotSupported(t *testing.T) {
	config := AgentConfig{
		Provider:   "claude",
		PromptFile: "prompts/conversation.md",
		Mode:       "conversation",
	}
	err := config.Validate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported in conversation mode")
}

func TestAgentConfig_PromptFile_ConversationMode_PromptAllowed(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Prompt:   "Initial conversation prompt",
		Mode:     "conversation",
	}
	err := config.Validate(nil)
	assert.NoError(t, err)
}

func TestAgentConfig_PromptFile_ConversationMode_InitialPromptAllowed(t *testing.T) {
	config := AgentConfig{
		Provider:      "claude",
		InitialPrompt: "Start conversation",
		Mode:          "conversation",
	}
	err := config.Validate(nil)
	assert.NoError(t, err)
}

func TestAgentConfig_PromptFile_EdgeCases_EmptyString(t *testing.T) {
	config := AgentConfig{
		Provider:   "claude",
		PromptFile: "",
		Prompt:     "Valid prompt",
	}
	err := config.Validate(nil)
	assert.NoError(t, err)
}

func TestAgentConfig_PromptFile_EdgeCases_WhitespaceOnly(t *testing.T) {
	config := AgentConfig{
		Provider:   "claude",
		PromptFile: "   ",
		Prompt:     "",
	}
	err := config.Validate(nil)
	assert.NoError(t, err)
}

func TestAgentConfig_PromptFile_ComplexPaths(t *testing.T) {
	tests := []struct {
		name       string
		promptFile string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "path with spaces",
			promptFile: "prompts/my prompt file.md",
			wantErr:    false,
		},
		{
			name:       "path with special chars",
			promptFile: "prompts/@special-#prompt$.md",
			wantErr:    false,
		},
		{
			name:       "path with unicode",
			promptFile: "prompts/日本語.md",
			wantErr:    false,
		},
		{
			name:       "mixed template and literal",
			promptFile: "{{.env.HOME}}/prompts/analyze.md",
			wantErr:    false,
		},
		{
			name:       "multiple template variables",
			promptFile: "{{.awf.config_dir}}/{{.inputs.workflow_name}}/prompt.md",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:   "claude",
				PromptFile: tt.promptFile,
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAgentConfig_PromptFile_WithOtherFields(t *testing.T) {
	config := AgentConfig{
		Provider:   "claude",
		PromptFile: "prompts/analyze.md",
		Options: map[string]any{
			"model":      "claude-sonnet-4-20250514",
			"max_tokens": 4096,
		},
		Timeout: 120,
	}
	err := config.Validate(nil)
	require.NoError(t, err)
	assert.Equal(t, "prompts/analyze.md", config.PromptFile)
	assert.Equal(t, "claude", config.Provider)
	assert.Equal(t, 120, config.Timeout)
}

func TestAgentConfig_PromptFile_MutualExclusivity_NonEmptyPrompt(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		promptFile string
		wantErr    bool
	}{
		{
			name:       "both non-empty",
			prompt:     "Inline prompt",
			promptFile: "prompts/file.md",
			wantErr:    true,
		},
		{
			name:       "both with templates",
			prompt:     "Code: {{inputs.code}}",
			promptFile: "{{.awf.prompts_dir}}/analyze.md",
			wantErr:    true,
		},
		{
			name:       "empty prompt, non-empty file",
			prompt:     "",
			promptFile: "prompts/file.md",
			wantErr:    false,
		},
		{
			name:       "non-empty prompt, empty file",
			prompt:     "Inline prompt",
			promptFile: "",
			wantErr:    false,
		},
		{
			name:       "whitespace prompt, non-empty file",
			prompt:     "   ",
			promptFile: "prompts/file.md",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:   "claude",
				Prompt:     tt.prompt,
				PromptFile: tt.promptFile,
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "mutually exclusive")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAgentConfig_PromptFile_GetEffectivePrompt_DoesNotReturnFile(t *testing.T) {
	config := AgentConfig{
		Provider:   "claude",
		PromptFile: "prompts/analyze.md",
	}
	effectivePrompt := config.GetEffectivePrompt()
	assert.Empty(t, effectivePrompt)
}

func TestAgentConfig_PromptFile_ConversationMode_Combinations(t *testing.T) {
	tests := []struct {
		name          string
		prompt        string
		promptFile    string
		initialPrompt string
		wantErr       bool
		errMsg        string
	}{
		{
			name:       "promptFile only",
			promptFile: "prompts/conv.md",
			wantErr:    true,
			errMsg:     "not supported in conversation mode",
		},
		{
			name:          "promptFile with initialPrompt",
			promptFile:    "prompts/conv.md",
			initialPrompt: "Start",
			wantErr:       true,
			errMsg:        "not supported in conversation mode",
		},
		{
			name:       "promptFile with prompt",
			promptFile: "prompts/conv.md",
			prompt:     "Initial",
			wantErr:    true,
			errMsg:     "mutually exclusive",
		},
		{
			name:          "promptFile with both prompt and initialPrompt",
			promptFile:    "prompts/conv.md",
			prompt:        "Initial",
			initialPrompt: "Start",
			wantErr:       true,
		},
		{
			name:          "initialPrompt only (valid)",
			initialPrompt: "Start conversation",
			wantErr:       false,
		},
		{
			name:    "prompt only (valid)",
			prompt:  "Initial message",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				Prompt:        tt.prompt,
				PromptFile:    tt.promptFile,
				InitialPrompt: tt.initialPrompt,
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
