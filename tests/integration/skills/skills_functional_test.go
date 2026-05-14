//go:build integration

package skills_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F096
func TestSkillLoading_Integration(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Setup skill directory with SKILL.md
	skillDir := filepath.Join(tmpDir, "test-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillContent := `---
name: test-skill
description: Test skill
---

This is a test skill content with helpful information.
`
	skillFile := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillFile, []byte(skillContent), 0o644))

	// Create repo with tmpDir as search path
	t.Setenv("AWF_SKILLS_PATH", tmpDir)
	repo := skills.NewFilesystemSkillRepository(nil)

	// Load skill by name
	skill, err := repo.Load(ctx, "test-skill")

	require.NoError(t, err)
	assert.Equal(t, "test-skill", skill.Name)
	assert.NotEmpty(t, skill.Content)
	assert.Equal(t, skillDir, skill.Location)
}

// Feature: F096
func TestSkillFormatting_NoResources(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "simple-skill",
		Content:   "Skill body here",
		Location:  "/path/to/skill",
		Resources: []string{},
	}

	formatted := application.FormatSkillContent(skill)

	assert.Contains(t, formatted, `<skill_content name="simple-skill">`)
	assert.Contains(t, formatted, "Skill body here")
	assert.Contains(t, formatted, "Skill directory: /path/to/skill")
	assert.Contains(t, formatted, "Relative paths in this skill are relative to the skill directory")
	assert.Contains(t, formatted, "</skill_content>")
	assert.NotContains(t, formatted, "<skill_resources>")
}

// Feature: F096
func TestSkillFormatting_WithResources(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "complex-skill",
		Content:   "Skill with dependencies",
		Location:  "/path/to/complex-skill",
		Resources: []string{"config.json", "lib/helper.js"},
	}

	formatted := application.FormatSkillContent(skill)

	assert.Contains(t, formatted, `<skill_content name="complex-skill">`)
	assert.Contains(t, formatted, "<skill_resources>")
	assert.Contains(t, formatted, "<file>config.json</file>")
	assert.Contains(t, formatted, "<file>lib/helper.js</file>")
	assert.Contains(t, formatted, "</skill_resources>")
	assert.Contains(t, formatted, "</skill_content>")
}

// Feature: F096
func TestSkillResolution_NameBased(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Setup two skills
	for _, name := range []string{"auth-skill", "debug-skill"} {
		skillDir := filepath.Join(tmpDir, name)
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "SKILL.md"),
			[]byte("---\nname: "+name+"\n---\n\nContent for "+name),
			0o644,
		))
	}

	t.Setenv("AWF_SKILLS_PATH", tmpDir)
	repo := skills.NewFilesystemSkillRepository(nil)

	// Test name-based resolution
	refs := []workflow.SkillReference{
		{Name: "auth-skill"},
		{Name: "debug-skill"},
	}

	formatted, err := application.ResolveAndFormatSkills(ctx, repo, refs, tmpDir)

	require.NoError(t, err)
	assert.Contains(t, formatted, "auth-skill")
	assert.Contains(t, formatted, "debug-skill")
	assert.Contains(t, formatted, "\n\n") // Double newline separator
}

// Feature: F096
func TestSkillResolution_PathBased(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Setup skill at explicit path
	skillDir := filepath.Join(tmpDir, "my-project", "skills", "custom")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: custom\n---\n\nCustom skill content"),
		0o644,
	))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)
	repo := skills.NewFilesystemSkillRepository(nil)

	// Test relative path resolution
	refs := []workflow.SkillReference{
		{Path: "my-project/skills/custom"},
	}

	formatted, err := application.ResolveAndFormatSkills(ctx, repo, refs, tmpDir)

	require.NoError(t, err)
	assert.Contains(t, formatted, "Custom skill content")
	assert.Contains(t, formatted, skillDir)
}

// Feature: F096
func TestSkillResolution_EmptyRefs(t *testing.T) {
	ctx := context.Background()
	repo := skills.NewFilesystemSkillRepository(nil)

	formatted, err := application.ResolveAndFormatSkills(ctx, repo, []workflow.SkillReference{}, "/any/dir")

	require.NoError(t, err)
	assert.Equal(t, "", formatted)
}

// Feature: F096
func TestSkillResolution_MissingSkill(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	t.Setenv("AWF_SKILLS_PATH", tmpDir)
	repo := skills.NewFilesystemSkillRepository(nil)

	refs := []workflow.SkillReference{
		{Name: "nonexistent-skill"},
	}

	_, err := application.ResolveAndFormatSkills(ctx, repo, refs, tmpDir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill")
}

// Feature: F096
func TestSkillReference_IsPathBased(t *testing.T) {
	tests := []struct {
		name     string
		ref      workflow.SkillReference
		expected bool
	}{
		{
			name:     "name-based reference",
			ref:      workflow.SkillReference{Name: "my-skill"},
			expected: false,
		},
		{
			name:     "path-based reference",
			ref:      workflow.SkillReference{Path: "/path/to/skill"},
			expected: true,
		},
		{
			name:     "zero-value reference",
			ref:      workflow.SkillReference{},
			expected: false,
		},
		{
			name:     "relative path-based reference",
			ref:      workflow.SkillReference{Path: "./relative/skill"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ref.IsPathBased()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Feature: F096
// TestSkillLoading_WithResources validates that resource enumeration works
func TestSkillLoading_WithResources(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Setup skill with resource files
	skillDir := filepath.Join(tmpDir, "resourced-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: resourced\n---\n\nContent"),
		0o644,
	))

	// Add resource files
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "config.yaml"), []byte("key: value"), 0o644))
	libDir := filepath.Join(skillDir, "lib")
	require.NoError(t, os.MkdirAll(libDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "helper.js"), []byte("// helper"), 0o644))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)
	repo := skills.NewFilesystemSkillRepository(nil)

	skill, err := repo.Load(ctx, "resourced-skill")

	require.NoError(t, err)
	assert.NotEmpty(t, skill.Resources)
	assert.Contains(t, skill.Resources, "config.yaml")
	assert.True(t, hasString(skill.Resources, "lib/helper.js"), "should enumerate nested resources")
}

// Feature: F096
// TestSkillFormatting_XmlEscaping ensures dangerous characters are handled
func TestSkillFormatting_SpecialCharacters(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "skill-with-chars",
		Content:   "Content with <angle> brackets and & ampersand",
		Location:  "/path/to/skill",
		Resources: []string{},
	}

	formatted := application.FormatSkillContent(skill)

	// Skill content is injected as-is (per spec), so these characters appear verbatim
	assert.Contains(t, formatted, "Content with <angle> brackets")
	assert.Contains(t, formatted, "& ampersand")
}

// Feature: F096
// TestSkillLoading_FrontmatterStripping validates that frontmatter is removed
func TestSkillLoading_FrontmatterStripping(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	skillDir := filepath.Join(tmpDir, "frontmatter-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillContent := `---
name: frontmatter-skill
description: A skill with frontmatter
tags:
  - testing
---

This is the actual skill content.
It should not contain the YAML frontmatter.
`

	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte(skillContent),
		0o644,
	))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)
	repo := skills.NewFilesystemSkillRepository(nil)

	skill, err := repo.Load(ctx, "frontmatter-skill")

	require.NoError(t, err)
	assert.NotContains(t, skill.Content, "---")
	assert.NotContains(t, skill.Content, "name: frontmatter-skill")
	assert.Contains(t, skill.Content, "This is the actual skill content")
}

// Feature: F096
// TestSkillValidation_Validate verifies that empty skill content is detected
func TestSkillContent_EmptyDetection(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "empty-skill",
		Content:   "",
		Location:  "/path/to/empty",
		Resources: []string{},
	}

	// Format should still work, but callers can detect empty content
	formatted := application.FormatSkillContent(skill)
	assert.NotEmpty(t, formatted)
	assert.Equal(t, "", skill.Content) // Verify it's truly empty
}

// TestSkillMultiple_Concatenation validates that multiple skills are properly joined
// Feature: F096
func TestSkillResolution_MultipleSkillsConcatenation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Setup three skills
	for i, name := range []string{"skill-1", "skill-2", "skill-3"} {
		skillDir := filepath.Join(tmpDir, name)
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		content := "---\nname: " + name + "\n---\n\nContent " + string(rune(i+49))
		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "SKILL.md"),
			[]byte(content),
			0o644,
		))
	}

	t.Setenv("AWF_SKILLS_PATH", tmpDir)
	repo := skills.NewFilesystemSkillRepository(nil)

	refs := []workflow.SkillReference{
		{Name: "skill-1"},
		{Name: "skill-2"},
		{Name: "skill-3"},
	}

	formatted, err := application.ResolveAndFormatSkills(ctx, repo, refs, tmpDir)

	require.NoError(t, err)

	// Verify all three skills are present
	assert.Contains(t, formatted, "skill-1")
	assert.Contains(t, formatted, "skill-2")
	assert.Contains(t, formatted, "skill-3")

	// Verify they're separated by double newlines
	parts := strings.Split(formatted, "\n\n")
	assert.Greater(t, len(parts), 1, "multiple skills should be separated by double newlines")
}

// Helper function
func hasString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
