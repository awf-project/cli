// Component: T010 - Agent Config Conversation Tests
// Feature: C013 - Domain Test File Splitting
// Source: agent_config_test.go (lines 998-1773)
// Test count: 12 tests
// Scope: ConversationConfig validation, mode detection, system/initial prompts, backward compatibility

package workflow_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workflow.AgentConfig Conversation Field Tests

func TestAgentConfig_ConversationField(t *testing.T) {
	tests := []struct {
		name         string
		config       workflow.AgentConfig
		wantErr      bool
		errMsg       string
		validateConv bool
	}{
		{
			name: "conversation mode with continue_from config",
			config: workflow.AgentConfig{
				Provider: "claude",
				Mode:     "conversation",
				Prompt:   "Start",
				Conversation: &workflow.ConversationConfig{
					ContinueFrom: "previous_step",
				},
			},
			wantErr:      false,
			validateConv: true,
		},
		{
			name: "conversation mode with nil conversation config",
			config: workflow.AgentConfig{
				Provider:     "claude",
				Mode:         "conversation",
				Prompt:       "Start",
				Conversation: nil,
			},
			wantErr: false,
		},
		{
			name: "single mode ignores conversation config",
			config: workflow.AgentConfig{
				Provider:     "claude",
				Mode:         "single",
				Prompt:       "Test",
				Conversation: &workflow.ConversationConfig{},
			},
			wantErr: false,
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
				if tt.validateConv && tt.config.Conversation != nil {
					assert.NotNil(t, tt.config.Conversation)
				}
			}
		})
	}
}

// workflow.AgentConfig SystemPrompt Tests

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
			config := workflow.AgentConfig{
				Provider:     "claude",
				Mode:         tt.mode,
				SystemPrompt: tt.systemPrompt,
				Prompt:       "Test",
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.systemPrompt, config.SystemPrompt)
			}
		})
	}
}

// workflow.AgentConfig Prompt Tests (conversation mode)

func TestAgentConfig_ConversationPrompt(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		mode    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "conversation mode with prompt",
			prompt:  "Start reviewing",
			mode:    "conversation",
			wantErr: false,
		},
		{
			name:    "conversation mode with template in prompt",
			prompt:  "Review this: {{inputs.code}}",
			mode:    "conversation",
			wantErr: false,
		},
		{
			name:    "conversation mode without prompt fails",
			prompt:  "",
			mode:    "conversation",
			wantErr: true,
			errMsg:  "prompt is required",
		},
		{
			name:    "single mode with prompt",
			prompt:  "Used",
			mode:    "single",
			wantErr: false,
		},
		{
			name:    "conversation mode with multiline prompt",
			prompt:  "Review this code:\n{{inputs.code}}\n\nBe thorough.",
			mode:    "conversation",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := workflow.AgentConfig{
				Provider: "claude",
				Mode:     tt.mode,
				Prompt:   tt.prompt,
			}
			err := config.Validate(nil)
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

// workflow.AgentConfig IsConversationMode Tests

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
			config := workflow.AgentConfig{
				Provider: "claude",
				Mode:     tt.mode,
				Prompt:   "Test",
			}
			// For tests checking pre-validation behavior
			if tt.mode == "" || tt.mode == "CONVERSATION" {
				assert.Equal(t, tt.expected, config.IsConversationMode())
			} else {
				// For tests after normalization
				_ = config.Validate(nil)
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
			config := workflow.AgentConfig{
				Provider: "claude",
				Mode:     tt.mode,
				Prompt:   "Test",
			}
			_ = config.Validate(nil) // Normalize mode
			assert.Equal(t, tt.expected, config.IsConversationMode())
		})
	}
}

// workflow.AgentConfig GetEffectivePrompt Tests

func TestAgentConfig_GetEffectivePrompt(t *testing.T) {
	tests := []struct {
		name           string
		mode           string
		prompt         string
		expectedPrompt string
	}{
		{
			name:           "single mode uses prompt",
			mode:           "single",
			prompt:         "Main prompt",
			expectedPrompt: "Main prompt",
		},
		{
			name:           "conversation mode uses prompt",
			mode:           "conversation",
			prompt:         "Conversation prompt",
			expectedPrompt: "Conversation prompt",
		},
		{
			name:           "empty mode returns prompt",
			mode:           "",
			prompt:         "Main prompt",
			expectedPrompt: "Main prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := workflow.AgentConfig{
				Provider: "claude",
				Mode:     tt.mode,
				Prompt:   tt.prompt,
			}
			_ = config.Validate(nil) // Normalize mode
			assert.Equal(t, tt.expectedPrompt, config.GetEffectivePrompt())
		})
	}
}

func TestAgentConfig_GetEffectivePrompt_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		config         workflow.AgentConfig
		expectedPrompt string
	}{
		{
			name: "empty prompt returns empty",
			config: workflow.AgentConfig{
				Mode:   "conversation",
				Prompt: "",
			},
			expectedPrompt: "",
		},
		{
			name: "multiline prompt",
			config: workflow.AgentConfig{
				Mode:   "conversation",
				Prompt: "Line 1\nLine 2",
			},
			expectedPrompt: "Line 1\nLine 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedPrompt, tt.config.GetEffectivePrompt())
		})
	}
}

func TestAgentConfig_ConversationMode_Complete(t *testing.T) {
	config := workflow.AgentConfig{
		Provider:     "claude",
		Mode:         "conversation",
		SystemPrompt: "You are a helpful code reviewer. Iterate until code meets standards.",
		Prompt: `Review this code:
{{inputs.code}}

Say "APPROVED" when done.`,
		Options: map[string]any{
			"model":      "claude-sonnet-4-20250514",
			"max_tokens": 4096,
		},
		Timeout: 300,
		Conversation: &workflow.ConversationConfig{
			ContinueFrom: "",
		},
	}

	err := config.Validate(nil)
	require.NoError(t, err)

	assert.Equal(t, "claude", config.Provider)
	assert.True(t, config.IsConversationMode())
	assert.Contains(t, config.SystemPrompt, "code reviewer")
	assert.Contains(t, config.GetEffectivePrompt(), "Review this code")
	require.NotNil(t, config.Conversation)
}

func TestAgentConfig_ConversationMode_MinimalConfig(t *testing.T) {
	config := workflow.AgentConfig{
		Provider: "claude",
		Mode:     "conversation",
		Prompt:   "Hello",
	}

	err := config.Validate(nil)
	require.NoError(t, err)
	assert.True(t, config.IsConversationMode())
	assert.Equal(t, "Hello", config.GetEffectivePrompt())
	assert.Nil(t, config.Conversation)
}

func TestAgentConfig_SingleMode_BackwardCompatibility(t *testing.T) {
	// Existing single-mode config should work without Mode field
	config := workflow.AgentConfig{
		Provider: "claude",
		Prompt:   "Analyze this code",
		Options: map[string]any{
			"model": "claude-sonnet-4-20250514",
		},
	}

	err := config.Validate(nil)
	require.NoError(t, err)
	assert.False(t, config.IsConversationMode())
	assert.Equal(t, "single", config.Mode) // Normalized to "single"
	assert.Equal(t, "Analyze this code", config.GetEffectivePrompt())
}

func TestAgentConfig_ConversationMode_Errors(t *testing.T) {
	tests := []struct {
		name    string
		config  workflow.AgentConfig
		wantErr string
	}{
		{
			name: "conversation mode without prompt",
			config: workflow.AgentConfig{
				Provider: "claude",
				Mode:     "conversation",
			},
			wantErr: "prompt is required in conversation mode",
		},
		{
			name: "invalid mode value",
			config: workflow.AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Mode:     "streaming",
			},
			wantErr: "mode must be 'single' or 'conversation'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate(nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
