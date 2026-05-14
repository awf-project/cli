package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func TestValidateSkillRefs_NoSkillsReturnsNoWarnings(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				// No Skills field set
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateSkillRefs_ValidNameBasedSkill(t *testing.T) {
	tmpDir := t.TempDir()

	// Create skill directory with SKILL.md
	skillDir := filepath.Join(tmpDir, "go-conventions")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	skillMD := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillMD, []byte("skill content here"), 0o644))

	// Set AWF_SKILLS_PATH to use our test skill
	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Name: "go-conventions"},
				},
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateSkillRefs_ValidPathBasedSkill(t *testing.T) {
	tmpDir := t.TempDir()

	// Create skill directory with SKILL.md
	skillDir := filepath.Join(tmpDir, "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	skillMD := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillMD, []byte("skill content here"), 0o644))

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Path: skillDir},
				},
			},
		},
		SourceDir: tmpDir,
	}

	warnings, err := validateSkillRefs(wf)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateSkillRefs_PathBasedSkillResolvedRelativeToSourceDir(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, "workflows")
	skillsDir := filepath.Join(tmpDir, "skills")

	require.NoError(t, os.MkdirAll(workflowDir, 0o755))
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))

	// Create skill in the skills directory
	skillDir := filepath.Join(skillsDir, "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	skillMD := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillMD, []byte("skill content"), 0o644))

	// Reference the skill with a relative path from the workflow
	relativePath := filepath.Join("..", "skills", "my-skill")

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Path: relativePath},
				},
			},
		},
		SourceDir: workflowDir,
	}

	warnings, err := validateSkillRefs(wf)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateSkillRefs_MissingNameBasedSkillReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Name: "missing-skill"},
				},
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	require.Error(t, err)
	assert.Nil(t, warnings)
	var skillErr *workflow.SkillNotFoundError
	assert.True(t, errors.As(err, &skillErr), "error must be SkillNotFoundError, got: %T", err)
	assert.Contains(t, err.Error(), "missing-skill")
	assert.Contains(t, err.Error(), tmpDir)
}

func TestValidateSkillRefs_MissingPathBasedSkillReturnsError(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Path: "/nonexistent/skill/path"},
				},
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	require.Error(t, err)
	assert.Nil(t, warnings)
	var skillErr *workflow.SkillNotFoundError
	assert.True(t, errors.As(err, &skillErr), "error must be SkillNotFoundError, got: %T", err)
	assert.Contains(t, err.Error(), "/nonexistent/skill/path")
}

func TestValidateSkillRefs_DirectoryExistsButNoSKILLMD(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Name: "my-skill"},
				},
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	require.Error(t, err)
	assert.Nil(t, warnings)
	var validErr workflow.ValidationError
	assert.True(t, errors.As(err, &validErr), "error must be ValidationError, got: %T", err)
	assert.Equal(t, workflow.ErrSkillMissingSkillMD, validErr.Code)
	assert.Contains(t, err.Error(), "my-skill")
}

func TestValidateSkillRefs_EmptySkillContentReturnsWarning(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "empty-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	skillMD := filepath.Join(skillDir, "SKILL.md")
	// Create empty SKILL.md
	require.NoError(t, os.WriteFile(skillMD, []byte(""), 0o644))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Name: "empty-skill"},
				},
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	assert.NoError(t, err)
	assert.NotNil(t, warnings)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "empty-skill")
}

func TestValidateSkillRefs_MultipleSkillsOneInvalid(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first valid skill
	validSkillDir := filepath.Join(tmpDir, "valid-skill")
	require.NoError(t, os.MkdirAll(validSkillDir, 0o755))
	validSkillMD := filepath.Join(validSkillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(validSkillMD, []byte("valid content"), 0o644))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Name: "valid-skill"},
					{Name: "missing-skill"},
				},
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	require.Error(t, err)
	assert.Nil(t, warnings)
	var skillErr *workflow.SkillNotFoundError
	assert.True(t, errors.As(err, &skillErr), "error must be SkillNotFoundError, got: %T", err)
	assert.Contains(t, err.Error(), "missing-skill")
	assert.Contains(t, err.Error(), tmpDir)
}

func TestValidateSkillRefs_MultipleSkillsMultipleWarnings(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two empty skills
	emptySkill1Dir := filepath.Join(tmpDir, "empty-skill-1")
	require.NoError(t, os.MkdirAll(emptySkill1Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(emptySkill1Dir, "SKILL.md"), []byte(""), 0o644))

	emptySkill2Dir := filepath.Join(tmpDir, "empty-skill-2")
	require.NoError(t, os.MkdirAll(emptySkill2Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(emptySkill2Dir, "SKILL.md"), []byte(""), 0o644))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Name: "empty-skill-1"},
					{Name: "empty-skill-2"},
				},
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	assert.NoError(t, err)
	assert.NotNil(t, warnings)
	assert.Len(t, warnings, 2)
}

func TestValidateSkillRefs_MultipleStepsWithSkills(t *testing.T) {
	tmpDir := t.TempDir()

	skillDir := filepath.Join(tmpDir, "shared-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("skill content"), 0o644))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name: "step1",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Name: "shared-skill"},
				},
			},
			"step2": {
				Name: "step2",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Name: "shared-skill"},
				},
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateSkillRefs_EmptySkillsSliceReturnsNoWarnings(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:   "start",
				Type:   workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{}, // explicit empty slice, not nil
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateSkillRefs_SkillWithFrontmatterButNoContent(t *testing.T) {
	tmpDir := t.TempDir()

	skillDir := filepath.Join(tmpDir, "frontmatter-only-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	skillMD := filepath.Join(skillDir, "SKILL.md")
	// SKILL.md with only frontmatter (no actual content after ---)
	require.NoError(t, os.WriteFile(skillMD, []byte("---\nname: test\n---\n"), 0o644))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeAgent,
				Skills: []workflow.SkillReference{
					{Name: "frontmatter-only-skill"},
				},
			},
		},
		SourceDir: t.TempDir(),
	}

	warnings, err := validateSkillRefs(wf)

	assert.NoError(t, err)
	assert.NotNil(t, warnings)
	assert.Len(t, warnings, 1)
}
