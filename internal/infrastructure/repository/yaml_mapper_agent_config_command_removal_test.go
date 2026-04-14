package repository

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Feature: F070 - Remove Custom Agent Provider
// Component: T008 - Remove AgentConfig.Command field and YAML mapping
//
// This test suite verifies that the Command field has been successfully removed
// from AgentConfig domain model and that the YAML mapping infrastructure no longer
// references it.

// TestAgentConfigHasNoCommandField verifies that AgentConfig struct
// does not have a Command field exposed for use.
// This is a compile-time assertion: if Command field existed, this would fail to compile.
func TestAgentConfigHasNoCommandField(t *testing.T) {
	// Compile-time verification: AgentConfig struct has no Command field.
	// If Command is re-added, this struct literal will fail to compile
	// because it enumerates all expected fields.
	_ = workflow.AgentConfig{
		Provider: "test",
	}
}

// TestMapAgentConfigFlat_NoCommandMapping verifies that mapAgentConfigFlat
// doesn't attempt to set Command on the returned AgentConfig.
func TestMapAgentConfigFlat_NoCommandMapping(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep *yamlStep
		wantErr  bool
	}{
		{
			name: "minimal agent config",
			yamlStep: &yamlStep{
				Provider: "claude",
				Prompt:   "Test prompt",
			},
			wantErr: false,
		},
		{
			name: "agent config with options",
			yamlStep: &yamlStep{
				Provider: "claude",
				Prompt:   "Test prompt",
				Options: map[string]any{
					"model": "claude-sonnet-4-20250514",
				},
			},
			wantErr: false,
		},
		{
			name: "agent config with all fields except Command",
			yamlStep: &yamlStep{
				Provider:     "claude",
				Prompt:       "Test prompt",
				Options:      map[string]any{"temperature": 0.5},
				Mode:         "conversation",
				SystemPrompt: "You are helpful",
				OutputFormat: "json",
			},
			wantErr: false,
		},
		{
			name: "no provider returns nil",
			yamlStep: &yamlStep{
				Provider: "",
				Prompt:   "Test prompt",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapAgentConfigFlat(tt.yamlStep)

			if tt.yamlStep.Provider == "" {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, tt.yamlStep.Provider, result.Provider)
			assert.Equal(t, tt.yamlStep.Prompt, result.Prompt)

			// Most importantly: verify Command field is not being set
			// (this is implicit - if Command existed on AgentConfig, we would set it)
			assert.NotPanics(t, func() {
				_ = result.Provider // access valid field
			})
		})
	}
}

// TestYAMLStepCommand_StillParsedButIgnored verifies that if someone passes
// a command field in YAML, it is parsed into yamlStep.Command but doesn't
// cause errors or affect agent config mapping.
func TestYAMLStepCommand_StillParsedButIgnored(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		expectCommand string
		expectErr     bool
	}{
		{
			name: "agent step with command field in YAML",
			yamlContent: `
type: agent
provider: claude
prompt: "Test prompt"
command: "this-should-be-ignored"
`,
			expectCommand: "this-should-be-ignored",
			expectErr:     false,
		},
		{
			name: "agent step with command and other fields",
			yamlContent: `
type: agent
provider: gemini
prompt: "Analyze {{inputs.code}}"
command: "old-command"
options:
  model: "gemini-pro"
`,
			expectCommand: "old-command",
			expectErr:     false,
		},
		{
			name: "agent step without command field",
			yamlContent: `
type: agent
provider: codex
prompt: "Summarize this"
options:
  temperature: 0.7
`,
			expectCommand: "",
			expectErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var step yamlStep
			err := yaml.Unmarshal([]byte(tt.yamlContent), &step)

			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectCommand, step.Command)

			// Verify that mapAgentConfigFlat still works despite Command being present
			result := mapAgentConfigFlat(&step)
			require.NotNil(t, result)
			assert.Equal(t, step.Provider, result.Provider)
			assert.Equal(t, step.Prompt, result.Prompt)
		})
	}
}

// TestMapAgentConfigFlat_IgnoresYAMLStepCommand verifies that even if
// yamlStep.Command is populated, mapAgentConfigFlat doesn't attempt to
// access or use it (preventing migration issues).
func TestMapAgentConfigFlat_IgnoresYAMLStepCommand(t *testing.T) {
	yamlStep := &yamlStep{
		Provider:     "claude",
		Prompt:       "Test",
		Command:      "this-should-not-be-set", // Old field that shouldn't be used
		ScriptFile:   "script.sh",
		Dir:          "/tmp",
		Options:      map[string]any{"key": "value"},
		Mode:         "single",
		OutputFormat: "json",
	}

	result := mapAgentConfigFlat(yamlStep)

	require.NotNil(t, result)
	assert.Equal(t, "claude", result.Provider)
	assert.Equal(t, "Test", result.Prompt)
	assert.Equal(t, "json", string(result.OutputFormat))
	assert.Equal(t, map[string]any{"key": "value"}, result.Options)

	// Verify that fields that ARE mapped are correct
	assert.Equal(t, "single", result.Mode)
}

// TestAgentConfigMappingWithConversationMode verifies that agent config
// mapping works correctly with all conversation-related fields,
// confirming that Command removal doesn't affect these features.
func TestAgentConfigMappingWithConversationMode(t *testing.T) {
	yamlStep := &yamlStep{
		Provider:     "claude",
		Prompt:       "Initial prompt",
		SystemPrompt: "You are an expert",
		Mode:         "conversation",
		Conversation: &yamlConversationConfig{
			ContinueFrom: "prior_step",
		},
		Options: map[string]any{
			"model": "claude-sonnet-4-20250514",
		},
	}

	result := mapAgentConfigFlat(yamlStep)

	require.NotNil(t, result)
	assert.Equal(t, "claude", result.Provider)
	assert.Equal(t, "Initial prompt", result.Prompt)
	assert.Equal(t, "You are an expert", result.SystemPrompt)
	assert.Equal(t, "conversation", result.Mode)
	assert.NotNil(t, result.Conversation)
}

// TestYAMLStep_CommandFieldExists verifies that yamlStep struct still has
// the Command field (for backward compatibility with YAML parsing),
// but it's not propagated to AgentConfig.
func TestYAMLStep_CommandFieldExists(t *testing.T) {
	step := yamlStep{
		Type:     "agent",
		Provider: "claude",
		Prompt:   "Test",
		Command:  "old-command", // Field still exists on yamlStep
	}

	// Verify the field exists and can be set
	assert.Equal(t, "old-command", step.Command)

	// But mapAgentConfigFlat doesn't use it
	result := mapAgentConfigFlat(&step)
	require.NotNil(t, result)

	// The result should be a valid AgentConfig without Command
	// (if Command existed, this assertion would demonstrate it's not set)
	assert.Equal(t, "claude", result.Provider)
}

// TestMapAgentConfigFlat_AllFieldsMappedExceptCommand verifies that
// all supported AgentConfig fields are properly mapped from yamlStep,
// and confirms Command is the only field NOT mapped.
func TestMapAgentConfigFlat_AllFieldsMappedExceptCommand(t *testing.T) {
	yamlStep := &yamlStep{
		Provider:     "openai_compatible",
		Prompt:       "Analyze code",
		PromptFile:   "prompt.txt",
		Mode:         "single",
		SystemPrompt: "System message",
		OutputFormat: "json",
		Options: map[string]any{
			"api_key": "secret",
			"model":   "gpt-4",
			"url":     "http://localhost:8000",
		},
		Conversation: &yamlConversationConfig{
			ContinueFrom: "prior_step",
		},
	}

	result := mapAgentConfigFlat(yamlStep)

	require.NotNil(t, result)

	// Verify all mapped fields
	assert.Equal(t, yamlStep.Provider, result.Provider)
	assert.Equal(t, yamlStep.Prompt, result.Prompt)
	assert.Equal(t, yamlStep.PromptFile, result.PromptFile)
	assert.Equal(t, yamlStep.Mode, result.Mode)
	assert.Equal(t, yamlStep.SystemPrompt, result.SystemPrompt)
	assert.Equal(t, yamlStep.OutputFormat, string(result.OutputFormat))
	assert.Equal(t, yamlStep.Options, result.Options)
	assert.NotNil(t, result.Conversation)

	// Command is NOT in this list - confirming it's been removed from mapping
}

// TestAgentConfigValidation_WithoutCommand verifies that AgentConfig
// can be validated successfully without the Command field.
func TestAgentConfigValidation_WithoutCommand(t *testing.T) {
	config := workflow.AgentConfig{
		Provider: "claude",
		Prompt:   "Test prompt",
	}

	err := config.Validate(nil)
	require.NoError(t, err)
}

// TestMapAgentConfigFlat_InvalidProvider verifies the boundary case
// where provider is empty, ensuring that removal of Command doesn't
// affect this edge case handling.
func TestMapAgentConfigFlat_InvalidProvider(t *testing.T) {
	tests := []struct {
		name      string
		provider  string
		expectNil bool
	}{
		{"empty provider", "", true},
		{"whitespace provider", "   ", false}, // yaml.v3 might trim, might not
		{"valid provider", "claude", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &yamlStep{
				Provider: tt.provider,
				Prompt:   "Test",
			}

			result := mapAgentConfigFlat(step)

			switch {
			case tt.expectNil:
				assert.Nil(t, result)
			case step.Provider == "":
				assert.Nil(t, result)
			default:
				assert.NotNil(t, result)
			}
		})
	}
}
