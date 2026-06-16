//go:build integration

package skills_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupInitTestDir sets up a temporary test directory and changes to it
func setupInitTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tmpDir))

	t.Cleanup(func() {
		os.Chdir(originalDir)
	})

	return tmpDir
}

// createTestWorkflow creates a workflow file in the .awf/workflows directory
func createTestWorkflow(t *testing.T, baseDir, filename, content string) {
	t.Helper()

	workflowsDir := filepath.Join(baseDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	workflowPath := filepath.Join(workflowsDir, filename)
	require.NoError(t, os.WriteFile(workflowPath, []byte(content), 0o644))
}

// runCLI executes a CLI command and returns output.
// It uses NewRootCommandAutoFacade so the validate command is routed through
// the WorkflowFacade (required since F108). A --storage flag pointing at a
// temporary directory is prepended to every invocation so that buildFacade can
// create its SQLite history store without touching the user's real data directory.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Prepend --storage=<tmpdir> so buildFacade can create history.db.
	storageArgs := append([]string{"--storage=" + t.TempDir()}, args...)
	cmd.SetArgs(storageArgs)

	err := cmd.Execute()

	return out.String(), err
}

// Feature: F096
// TestValidateCommand_SkillsPresent verifies that workflows with valid skills pass validation
func TestValidateCommand_SkillsPresent(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a valid skill
	skillDir := filepath.Join(tmpDir, ".agents", "skills", "helper-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: helper-skill\n---\n\nHelper skill content"),
		0o644,
	))

	// Create workflow that uses the skill
	workflowContent := `name: skill-test-workflow
version: "1.0.0"
states:
  initial: process
  process:
    type: operation
    operation: claude.invoke
    skills:
      - helper-skill
    on_success: done
  done:
    type: terminal
`

	createTestWorkflow(t, tmpDir, "skill-test-workflow.yaml", workflowContent)

	t.Setenv("AWF_SKILLS_PATH", filepath.Join(tmpDir, ".agents", "skills"))

	output, err := runCLI(t, "validate", "skill-test-workflow")

	require.NoError(t, err)
	assert.Contains(t, output, "valid", "validation should succeed for valid skills")
}

// Feature: F096
// TestValidateCommand_SkillsMissing verifies that workflows with missing skills fail validation
func TestValidateCommand_SkillsMissing(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create workflow that references non-existent skill
	workflowContent := `name: missing-skill-workflow
version: "1.0.0"
states:
  initial: process
  process:
    type: operation
    operation: claude.invoke
    skills:
      - nonexistent-skill
    on_success: done
  done:
    type: terminal
`

	createTestWorkflow(t, tmpDir, "missing-skill-workflow.yaml", workflowContent)

	output, err := runCLI(t, "validate", "missing-skill-workflow")

	require.Error(t, err)
	assert.Contains(t, output, "nonexistent-skill")
}

// Feature: F096
// TestValidateCommand_SkillsPathBased verifies that path-based skill references work
func TestValidateCommand_SkillsPathBased(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create skill at explicit path relative to workflow
	skillDir := filepath.Join(tmpDir, ".awf", "workflows", "local-skills", "custom")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: custom\n---\n\nLocal skill content"),
		0o644,
	))

	// Create workflow that references skill by path
	workflowContent := `name: path-based-skill-workflow
version: "1.0.0"
states:
  initial: process
  process:
    type: operation
    operation: claude.invoke
    skills:
      - path: local-skills/custom
    on_success: done
  done:
    type: terminal
` // Note: path-based refs use map syntax {path: ...}

	createTestWorkflow(t, tmpDir, "path-based-skill-workflow.yaml", workflowContent)

	output, err := runCLI(t, "validate", "path-based-skill-workflow")

	require.NoError(t, err)
	assert.Contains(t, output, "valid")
}

// Feature: F096
// TestValidateCommand_SkillsEmptyContent verifies that empty skill content generates warnings
func TestValidateCommand_SkillsEmptyContent(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a skill with only frontmatter (empty content)
	skillDir := filepath.Join(tmpDir, ".agents", "skills", "empty-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: empty-skill\n---\n"),
		0o644,
	))

	// Create workflow that uses the empty skill
	workflowContent := `name: empty-skill-workflow
version: "1.0.0"
states:
  initial: process
  process:
    type: operation
    operation: claude.invoke
    skills:
      - empty-skill
    on_success: done
  done:
    type: terminal
`

	createTestWorkflow(t, tmpDir, "empty-skill-workflow.yaml", workflowContent)

	t.Setenv("AWF_SKILLS_PATH", filepath.Join(tmpDir, ".agents", "skills"))

	output, err := runCLI(t, "validate", "empty-skill-workflow")

	// Should succeed with warning
	require.NoError(t, err)
	assert.Contains(t, output, "empty-skill") // Warning should mention empty skill
}

// Feature: F096
// TestValidateCommand_SkillsMissingSKILLMD verifies error when SKILL.md is missing
func TestValidateCommand_SkillsMissingSKILLMD(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create skill directory without SKILL.md
	skillDir := filepath.Join(tmpDir, ".agents", "skills", "incomplete-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	// Intentionally don't create SKILL.md

	// Create workflow that references this skill
	workflowContent := `name: missing-skillmd-workflow
version: "1.0.0"
states:
  initial: process
  process:
    type: operation
    operation: claude.invoke
    skills:
      - incomplete-skill
    on_success: done
  done:
    type: terminal
`

	createTestWorkflow(t, tmpDir, "missing-skillmd-workflow.yaml", workflowContent)

	t.Setenv("AWF_SKILLS_PATH", filepath.Join(tmpDir, ".agents", "skills"))

	output, err := runCLI(t, "validate", "missing-skillmd-workflow")

	require.Error(t, err)
	assert.Contains(t, output, "incomplete-skill")
}

// Feature: F096
// TestValidateCommand_MultipleSkills verifies validation with multiple skills
func TestValidateCommand_MultipleSkills(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create multiple skills
	for _, skillName := range []string{"skill-a", "skill-b"} {
		skillDir := filepath.Join(tmpDir, ".agents", "skills", skillName)
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "SKILL.md"),
			[]byte("---\nname: "+skillName+"\n---\n\nContent for "+skillName),
			0o644,
		))
	}

	// Create workflow using both skills
	workflowContent := `name: multi-skill-workflow
version: "1.0.0"
states:
  initial: process
  process:
    type: operation
    operation: claude.invoke
    skills:
      - skill-a
      - skill-b
    on_success: done
  done:
    type: terminal
`

	createTestWorkflow(t, tmpDir, "multi-skill-workflow.yaml", workflowContent)

	t.Setenv("AWF_SKILLS_PATH", filepath.Join(tmpDir, ".agents", "skills"))

	output, err := runCLI(t, "validate", "multi-skill-workflow")

	require.NoError(t, err)
	assert.Contains(t, output, "valid")
}

// Feature: F096
// TestValidateCommand_NoSkills verifies validation still works when workflow has no skills
func TestValidateCommand_NoSkills(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create workflow without skills
	workflowContent := `name: no-skills-workflow
version: "1.0.0"
states:
  initial: process
  process:
    type: operation
    operation: claude.invoke
    on_success: done
  done:
    type: terminal
`

	createTestWorkflow(t, tmpDir, "no-skills-workflow.yaml", workflowContent)

	output, err := runCLI(t, "validate", "no-skills-workflow")

	require.NoError(t, err)
	assert.Contains(t, output, "valid")
}
