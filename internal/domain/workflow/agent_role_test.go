package workflow

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRole_ZeroValue(t *testing.T) {
	var role AgentRole
	assert.Equal(t, "", role.Name)
	assert.Equal(t, "", role.SourcePath)
	assert.Equal(t, "", role.Content)
}

func TestAgentRole_Fields(t *testing.T) {
	role := AgentRole{
		Name:       "backend-engineer",
		SourcePath: "/project/.agents/backend-engineer.md",
		Content:    "# Backend Engineer\n\nYou are a backend engineer.",
	}

	assert.Equal(t, "backend-engineer", role.Name)
	assert.Equal(t, "/project/.agents/backend-engineer.md", role.SourcePath)
	assert.Equal(t, "# Backend Engineer\n\nYou are a backend engineer.", role.Content)
}

func TestAgentRoleNotFoundError_ErrorMessage(t *testing.T) {
	tests := []struct {
		name        string
		err         *AgentRoleNotFoundError
		wantContain []string
	}{
		{
			name: "single search path",
			err: &AgentRoleNotFoundError{
				Name:        "frontend",
				SearchPaths: []string{"/project/.agents"},
			},
			wantContain: []string{"frontend", "/project/.agents"},
		},
		{
			name: "multiple search paths",
			err: &AgentRoleNotFoundError{
				Name:        "devops",
				SearchPaths: []string{"/project/.agents", "/home/user/.awf/agents", "/usr/share/awf/agents"},
			},
			wantContain: []string{"devops", "/project/.agents", "/home/user/.awf/agents", "/usr/share/awf/agents"},
		},
		{
			name: "empty search paths",
			err: &AgentRoleNotFoundError{
				Name:        "missing-role",
				SearchPaths: []string{},
			},
			wantContain: []string{"missing-role"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			for _, fragment := range tt.wantContain {
				assert.Contains(t, msg, fragment)
			}
		})
	}
}

func TestAgentRoleNotFoundError_Unwrap(t *testing.T) {
	underlying := errors.New("permission denied")
	err := &AgentRoleNotFoundError{
		Name:        "ops",
		SearchPaths: []string{"/agents"},
		Underlying:  underlying,
	}

	require.Equal(t, underlying, err.Unwrap())
	require.True(t, errors.Is(err, underlying))
}

func TestAgentRoleNotFoundError_UnwrapNil(t *testing.T) {
	err := &AgentRoleNotFoundError{
		Name:        "ops",
		SearchPaths: []string{"/agents"},
	}

	assert.Nil(t, err.Unwrap())
}

func TestValidationCode_RoleConstants(t *testing.T) {
	constants := []ValidationCode{
		ErrRoleNotFound,
		ErrRoleMissingAgentsMD,
	}

	for _, c := range constants {
		assert.NotEmpty(t, string(c), "constant must have a non-empty string value")
	}

	seen := make(map[ValidationCode]bool)
	for _, c := range constants {
		assert.False(t, seen[c], "constant %q is duplicated", c)
		seen[c] = true
	}
}
