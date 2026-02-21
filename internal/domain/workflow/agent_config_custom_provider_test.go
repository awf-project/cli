package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T007: provider: custom must be rejected with migration guidance.
// See: .specify/implementation/F070/spec-content.md

func TestAgentConfig_Validate_CustomProviderRejected(t *testing.T) {
	config := AgentConfig{
		Provider: "custom",
		Prompt:   "do something",
	}

	err := config.Validate(nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "custom")
	assert.Contains(t, err.Error(), "type: step")
	assert.Contains(t, err.Error(), "openai_compatible")
}

func TestAgentConfig_Validate_CustomProviderRejectedWithoutPrompt(t *testing.T) {
	config := AgentConfig{
		Provider: "custom",
	}

	err := config.Validate(nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "custom")
}

func TestAgentConfig_Validate_OpenAICompatibleProviderAccepted(t *testing.T) {
	config := AgentConfig{
		Provider: "openai_compatible",
		Prompt:   "do something",
		Options: map[string]any{
			"base_url": "http://localhost:11434/v1",
			"model":    "llama3.2",
		},
	}

	err := config.Validate(nil)

	require.NoError(t, err)
}

func TestAgentConfig_Validate_BuiltinProvidersStillAccepted(t *testing.T) {
	providers := []string{"claude", "codex", "gemini", "opencode"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			config := AgentConfig{
				Provider: provider,
				Prompt:   "do something",
			}

			err := config.Validate(nil)

			require.NoError(t, err)
		})
	}
}
