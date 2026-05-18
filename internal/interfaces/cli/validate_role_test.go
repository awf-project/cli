package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/roles"
)

func TestValidateRoleRefs_NoRolesReturnsNoWarnings(t *testing.T) {
	tmpDir := t.TempDir()
	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					// No Role field
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateRoleRefs_ValidNameBasedRole(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role directory with AGENTS.md
	roleDir := filepath.Join(tmpDir, ".awf", "agents", "go-senior")
	require.NoError(t, os.MkdirAll(roleDir, 0o755))
	agentsMD := filepath.Join(roleDir, "AGENTS.md")
	require.NoError(t, os.WriteFile(agentsMD, []byte("You are a senior Go engineer."), 0o644))

	// Override AWF_CONFIG_HOME BEFORE repo construction — the constructor
	// reads this eagerly and bakes search paths at init time.
	configHome := filepath.Join(tmpDir, ".awf")
	t.Setenv("AWF_CONFIG_HOME", configHome)

	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Role:     "go-senior",
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateRoleRefs_ValidPathBasedRole(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role directory with AGENTS.md
	roleDir := filepath.Join(tmpDir, "my-role")
	require.NoError(t, os.MkdirAll(roleDir, 0o755))
	agentsMD := filepath.Join(roleDir, "AGENTS.md")
	require.NoError(t, os.WriteFile(agentsMD, []byte("Role content here"), 0o644))

	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Role:     "./my-role",
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateRoleRefs_MissingRoleDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("AWF_CONFIG_HOME", filepath.Join(tmpDir, ".awf"))
	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Role:     "missing-role",
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	require.Error(t, err)
	assert.Nil(t, warnings)
	var validErr workflow.ValidationError
	assert.True(t, errors.As(err, &validErr), "error must be ValidationError, got: %T", err)
	assert.Equal(t, workflow.ErrRoleNotFound, validErr.Code)
	assert.Contains(t, err.Error(), "missing-role")
}

func TestValidateRoleRefs_MissingAgentsMD(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role directory but without AGENTS.md
	roleDir := filepath.Join(tmpDir, ".awf", "agents", "incomplete-role")
	require.NoError(t, os.MkdirAll(roleDir, 0o755))

	configHome := filepath.Join(tmpDir, ".awf")
	t.Setenv("AWF_CONFIG_HOME", configHome)
	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Role:     "incomplete-role",
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	require.Error(t, err)
	assert.Nil(t, warnings)
	var validErr workflow.ValidationError
	assert.True(t, errors.As(err, &validErr), "error must be ValidationError, got: %T", err)
	assert.Equal(t, workflow.ErrRoleMissingAgentsMD, validErr.Code)
	assert.Contains(t, err.Error(), "incomplete-role")
}

func TestValidateRoleRefs_EmptyAgentsMDBody(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role with empty AGENTS.md
	roleDir := filepath.Join(tmpDir, "empty-role")
	require.NoError(t, os.MkdirAll(roleDir, 0o755))
	agentsMD := filepath.Join(roleDir, "AGENTS.md")
	require.NoError(t, os.WriteFile(agentsMD, []byte(""), 0o644))

	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Role:     "./empty-role",
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	assert.NoError(t, err)
	assert.NotNil(t, warnings)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "empty-role")
	assert.Contains(t, strings.ToLower(warnings[0]), "empty")
}

func TestValidateRoleRefs_AgentsMDExceedsSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role with AGENTS.md > 500KB
	roleDir := filepath.Join(tmpDir, "large-role")
	require.NoError(t, os.MkdirAll(roleDir, 0o755))
	agentsMD := filepath.Join(roleDir, "AGENTS.md")
	// Create a file larger than 500KB
	content := strings.Repeat("x", 501*1024)
	require.NoError(t, os.WriteFile(agentsMD, []byte(content), 0o644))

	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Role:     "./large-role",
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	assert.NoError(t, err)
	assert.NotNil(t, warnings)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "large-role")
	assert.Contains(t, strings.ToLower(warnings[0]), "500")
}

func TestValidateRoleRefs_CombinedRoleAndSystemPromptExceeds10KB(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role with content that combined with system_prompt exceeds 10KB
	roleDir := filepath.Join(tmpDir, "combined-role")
	require.NoError(t, os.MkdirAll(roleDir, 0o755))
	agentsMD := filepath.Join(roleDir, "AGENTS.md")
	roleContent := strings.Repeat("x", 6*1024)
	require.NoError(t, os.WriteFile(agentsMD, []byte(roleContent), 0o644))

	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Role:         "./combined-role",
					SystemPrompt: strings.Repeat("y", 5*1024),
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	assert.NoError(t, err)
	assert.NotNil(t, warnings)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "combined-role")
	assert.Contains(t, strings.ToLower(warnings[0]), "10")
}

func TestValidateRoleRefs_PathTraversalAttempt(t *testing.T) {
	tmpDir := t.TempDir()
	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Role:     "../../../etc/passwd",
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	require.Error(t, err)
	assert.Nil(t, warnings)
	// Should return hard error (ValidationError) for path traversal
	var validErr workflow.ValidationError
	assert.True(t, errors.As(err, &validErr), "error must be ValidationError, got: %T", err)
}

func TestValidateRoleRefs_MultipleStepsMultipleRoles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two role directories
	role1Dir := filepath.Join(tmpDir, "role1")
	require.NoError(t, os.MkdirAll(role1Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(role1Dir, "AGENTS.md"), []byte("Role 1"), 0o644))

	role2Dir := filepath.Join(tmpDir, "role2")
	require.NoError(t, os.MkdirAll(role2Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(role2Dir, "AGENTS.md"), []byte("Role 2"), 0o644))

	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name: "step1",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Role:     "./role1",
				},
			},
			"step2": {
				Name: "step2",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "gemini",
					Role:     "./role2",
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateRoleRefs_StepWithoutAgentConfig(t *testing.T) {
	tmpDir := t.TempDir()
	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:    "start",
				Type:    workflow.StepTypeCommand,
				Command: "echo 'hello'",
				// No Agent field
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateRoleRefs_AgentStepWithEmptyRole(t *testing.T) {
	tmpDir := t.TempDir()
	repo := roles.NewFilesystemAgentRoleRepository(nil)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Role:     "", // explicitly empty
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateRoleRefs(wf, repo)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}
