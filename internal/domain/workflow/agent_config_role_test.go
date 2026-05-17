package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentConfig_Role_EmptyAccepted(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Prompt:   "hello",
		Role:     "",
	}

	err := config.Validate(nil)

	require.NoError(t, err)
	assert.Equal(t, "", config.Role)
}

func TestAgentConfig_Role_TraversalRejected(t *testing.T) {
	tests := []struct {
		name string
		role string
	}{
		{"double-dot prefix", "../escape"},
		{"double-dot in middle", "foo/../bar"},
		{"double-dot only", ".."},
		{"double-dot suffix", "roles/.."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider: "claude",
				Prompt:   "hello",
				Role:     tt.role,
			}

			err := config.Validate(nil)

			require.Error(t, err)
			valErr, ok := err.(ValidationError)
			require.True(t, ok, "expected ValidationError, got %T", err)
			assert.Equal(t, ValidationLevelError, valErr.Level)
			assert.Equal(t, ErrRoleNotFound, valErr.Code)
			assert.Contains(t, valErr.Message, "..")
		})
	}
}

func TestAgentConfig_Role_PlainNameAccepted(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Prompt:   "hello",
		Role:     "go-senior",
	}

	err := config.Validate(nil)

	require.NoError(t, err)
	assert.Equal(t, "go-senior", config.Role)
}

func TestAgentConfig_Role_RelativePathAccepted(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Prompt:   "hello",
		Role:     "./roles/backend",
	}

	err := config.Validate(nil)

	require.NoError(t, err)
	assert.Equal(t, "./roles/backend", config.Role)
}

func TestAgentConfig_Role_AbsolutePathAccepted(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Prompt:   "hello",
		Role:     "/abs/path/to/role",
	}

	err := config.Validate(nil)

	require.NoError(t, err)
	assert.Equal(t, "/abs/path/to/role", config.Role)
}

func TestAgentConfig_Role_TildePathAccepted(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Prompt:   "hello",
		Role:     "~/agents/foo",
	}

	err := config.Validate(nil)

	require.NoError(t, err)
	assert.Equal(t, "~/agents/foo", config.Role)
}

func TestAgentConfig_Role_TemplateExpressionAccepted(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Prompt:   "hello",
		Role:     "{{inputs.persona}}",
	}

	err := config.Validate(nil)

	require.NoError(t, err)
	assert.Equal(t, "{{inputs.persona}}", config.Role)
}
