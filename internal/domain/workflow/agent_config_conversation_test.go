// Component: T010 - Agent Config Conversation Tests
// Feature: C013 - Domain Test File Splitting
// Source: agent_config_test.go (lines 998-1773)
// Test count: 12 tests
// Scope: ConversationConfig validation, mode detection, system/initial prompts, backward compatibility

package workflow_test

import (
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
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
			name: "conversation mode with valid config",
			config: workflow.AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Start",
				Conversation: &workflow.ConversationConfig{
					MaxTurns:         10,
					MaxContextTokens: 100000,
					Strategy:         workflow.StrategySlidingWindow,
				},
			},
			wantErr:      false,
			validateConv: true,
		},
		{
			name: "conversation mode with nil conversation config",
			config: workflow.AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Start",
				Conversation:  nil,
			},
			wantErr: false,
		},
		{
			name: "single mode ignores conversation config",
			config: workflow.AgentConfig{
				Provider: "claude",
				Mode:     "single",
				Prompt:   "Test",
				Conversation: &workflow.ConversationConfig{
					MaxTurns: 10,
				},
			},
			wantErr: false,
		},
		{
			name: "conversation mode with invalid conversation config",
			config: workflow.AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Start",
				Conversation: &workflow.ConversationConfig{
					MaxTurns: -1, // Invalid
				},
			},
			wantErr: true,
			errMsg:  "max_turns",
		},
		{
			name: "conversation mode with stop condition",
			config: workflow.AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Review code",
				Conversation: &workflow.ConversationConfig{
					MaxTurns:      5,
					StopCondition: "response contains 'APPROVED'",
				},
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
				Provider:      "claude",
				Mode:          tt.mode,
				SystemPrompt:  tt.systemPrompt,
				InitialPrompt: "Test",
				Prompt:        "Test",
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

// workflow.AgentConfig InitialPrompt Tests

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
			config := workflow.AgentConfig{
				Provider:      "claude",
				Mode:          tt.mode,
				InitialPrompt: tt.initialPrompt,
				Prompt:        tt.prompt,
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
				Provider:      "claude",
				Mode:          tt.mode,
				Prompt:        "Test",
				InitialPrompt: "Test",
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
			config := workflow.AgentConfig{
				Provider:      "claude",
				Mode:          tt.mode,
				Prompt:        tt.prompt,
				InitialPrompt: tt.initialPrompt,
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
			name: "both prompts empty in conversation mode",
			config: workflow.AgentConfig{
				Mode:          "conversation",
				Prompt:        "",
				InitialPrompt: "",
			},
			expectedPrompt: "",
		},
		{
			name: "whitespace initial_prompt in conversation mode",
			config: workflow.AgentConfig{
				Mode:          "conversation",
				Prompt:        "Fallback",
				InitialPrompt: "   ",
			},
			expectedPrompt: "   ", // Returns as-is
		},
		{
			name: "multiline prompts",
			config: workflow.AgentConfig{
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

func TestAgentConfig_ConversationMode_Complete(t *testing.T) {
	config := workflow.AgentConfig{
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
		Conversation: &workflow.ConversationConfig{
			MaxTurns:         10,
			MaxContextTokens: 100000,
			Strategy:         workflow.StrategySlidingWindow,
			StopCondition:    "response contains 'APPROVED'",
		},
	}

	// Validate
	err := config.Validate(nil)
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
	assert.Equal(t, workflow.StrategySlidingWindow, config.Conversation.Strategy)
	assert.Contains(t, config.Conversation.StopCondition, "APPROVED")
}

func TestAgentConfig_ConversationMode_MinimalConfig(t *testing.T) {
	config := workflow.AgentConfig{
		Provider:      "claude",
		Mode:          "conversation",
		InitialPrompt: "Hello",
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
			wantErr: "initial_prompt or prompt is required",
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
		{
			name: "conversation with invalid config",
			config: workflow.AgentConfig{
				Provider:      "claude",
				Mode:          "conversation",
				InitialPrompt: "Test",
				Conversation: &workflow.ConversationConfig{
					MaxTurns: -1, // Invalid: negative
				},
			},
			wantErr: "max_turns",
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
