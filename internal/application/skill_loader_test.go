package application_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatSkillContent_BasicXMLStructure verifies the basic XML wrapping structure.
func TestFormatSkillContent_BasicXMLStructure(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "test-skill",
		Content:   "# Test Content",
		Location:  "/absolute/path/to/skill",
		Resources: []string{},
	}

	result := application.FormatSkillContent(skill)

	assert.Contains(t, result, `<skill_content name="test-skill">`)
	assert.Contains(t, result, `</skill_content>`)
	assert.Contains(t, result, "# Test Content")
}

// TestFormatSkillContent_SkillNameAttribute verifies the skill_content tag includes the correct name attribute.
func TestFormatSkillContent_SkillNameAttribute(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "my-awesome-skill",
		Content:   "Content here",
		Location:  "/path/to/skill",
		Resources: []string{},
	}

	result := application.FormatSkillContent(skill)

	assert.Contains(t, result, `<skill_content name="my-awesome-skill">`)
}

// TestFormatSkillContent_MarkdownBodyIncluded verifies the markdown body is included in the output.
func TestFormatSkillContent_MarkdownBodyIncluded(t *testing.T) {
	body := "# Skill Documentation\n\nThis is a test skill.\n\nWith multiple paragraphs."
	skill := &workflow.Skill{
		Name:      "doc-skill",
		Content:   body,
		Location:  "/path/to/skill",
		Resources: []string{},
	}

	result := application.FormatSkillContent(skill)

	assert.Contains(t, result, body)
}

// TestFormatSkillContent_SkillDirectoryPath verifies the skill directory path is included.
func TestFormatSkillContent_SkillDirectoryPath(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "test-skill",
		Content:   "Content",
		Location:  "/absolute/path/to/skill",
		Resources: []string{},
	}

	result := application.FormatSkillContent(skill)

	assert.Contains(t, result, "Skill directory: /absolute/path/to/skill")
}

// TestFormatSkillContent_RelativePathHint verifies the relative path hint is included.
func TestFormatSkillContent_RelativePathHint(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "test-skill",
		Content:   "Content",
		Location:  "/some/path",
		Resources: []string{},
	}

	result := application.FormatSkillContent(skill)

	assert.Contains(t, result, "Relative paths in this skill are relative to the skill directory.")
}

// TestFormatSkillContent_ClosingTag verifies the output ends with the closing tag.
func TestFormatSkillContent_ClosingTag(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "test-skill",
		Content:   "Content",
		Location:  "/path",
		Resources: []string{},
	}

	result := application.FormatSkillContent(skill)

	assert.Contains(t, result, "</skill_content>")
}

// TestFormatSkillContent_WithResources verifies the skill_resources block is included when resources exist.
func TestFormatSkillContent_WithResources(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "test-skill",
		Content:   "Content",
		Location:  "/path/to/skill",
		Resources: []string{"file1.txt", "file2.md", "subdir/file3.json"},
	}

	result := application.FormatSkillContent(skill)

	assert.Contains(t, result, "<skill_resources>")
	assert.Contains(t, result, "</skill_resources>")
	assert.Contains(t, result, "<file>file1.txt</file>")
	assert.Contains(t, result, "<file>file2.md</file>")
	assert.Contains(t, result, "<file>subdir/file3.json</file>")
}

// TestFormatSkillContent_WithoutResources verifies the skill_resources block is omitted when resources are empty.
func TestFormatSkillContent_WithoutResources(t *testing.T) {
	skill := &workflow.Skill{
		Name:      "test-skill",
		Content:   "Content",
		Location:  "/path/to/skill",
		Resources: []string{},
	}

	result := application.FormatSkillContent(skill)

	assert.NotContains(t, result, "<skill_resources>")
	assert.NotContains(t, result, "</skill_resources>")
}

// TestFormatSkillContent_MultipleResources verifies all resources are formatted with file tags.
func TestFormatSkillContent_MultipleResources(t *testing.T) {
	resources := []string{"doc.md", "schema.json", "template.yaml"}
	skill := &workflow.Skill{
		Name:      "test-skill",
		Content:   "Content",
		Location:  "/path",
		Resources: resources,
	}

	result := application.FormatSkillContent(skill)

	for _, resource := range resources {
		assert.Contains(t, result, "<file>"+resource+"</file>")
	}
}

// TestResolveAndFormatSkills_EmptyRefs verifies empty refs slice returns empty string with no error.
func TestResolveAndFormatSkills_EmptyRefs(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	result, err := application.ResolveAndFormatSkills(ctx, repo, []workflow.SkillReference{}, "/workflow/dir")

	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

// TestResolveAndFormatSkills_NameBasedRef verifies name-based ref calls repo.Load.
func TestResolveAndFormatSkills_NameBasedRef(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	skill := &workflow.Skill{
		Name:      "my-skill",
		Content:   "Test content",
		Location:  "/skills/my-skill",
		Resources: []string{},
	}
	repo.SetSkill("my-skill", skill)

	refs := []workflow.SkillReference{
		{Name: "my-skill"},
	}

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, "/workflow/dir")

	assert.NoError(t, err)
	assert.Contains(t, result, "Test content")
	assert.Contains(t, result, `<skill_content name="my-skill">`)
}

// TestResolveAndFormatSkills_PathBasedRefAbsolute verifies path-based absolute ref calls repo.LoadFromPath directly.
func TestResolveAndFormatSkills_PathBasedRefAbsolute(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	skill := &workflow.Skill{
		Name:      "absolute-skill",
		Content:   "Absolute path content",
		Location:  "/absolute/path/to/skill",
		Resources: []string{},
	}
	repo.SetSkill("/absolute/path/to/skill", skill)

	refs := []workflow.SkillReference{
		{Path: "/absolute/path/to/skill"},
	}

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, "/workflow/dir")

	assert.NoError(t, err)
	assert.Contains(t, result, "Absolute path content")
	assert.Contains(t, result, `<skill_content name="absolute-skill">`)
}

// TestResolveAndFormatSkills_PathBasedRefRelative verifies relative path is joined with workflow dir.
func TestResolveAndFormatSkills_PathBasedRefRelative(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	workflowDir := "/home/user/workflows"
	relPath := "skills/my-skill"
	expectedAbsPath := filepath.Join(workflowDir, filepath.Clean(relPath))

	skill := &workflow.Skill{
		Name:      "relative-skill",
		Content:   "Relative path content",
		Location:  expectedAbsPath,
		Resources: []string{},
	}
	repo.SetSkill(expectedAbsPath, skill)

	refs := []workflow.SkillReference{
		{Path: relPath},
	}

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, workflowDir)

	assert.NoError(t, err)
	assert.Contains(t, result, "Relative path content")
}

// TestResolveAndFormatSkills_CleanPathTraversal verifies filepath.Clean is applied to path-based refs.
func TestResolveAndFormatSkills_CleanPathTraversal(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	workflowDir := "/home/user/workflows"
	dirtyPath := "subdir/../skills/my-skill"
	cleanedPath := filepath.Clean(dirtyPath)
	expectedAbsPath := filepath.Join(workflowDir, cleanedPath)

	skill := &workflow.Skill{
		Name:      "clean-skill",
		Content:   "Cleaned path content",
		Location:  expectedAbsPath,
		Resources: []string{},
	}
	repo.SetSkill(expectedAbsPath, skill)

	refs := []workflow.SkillReference{
		{Path: dirtyPath},
	}

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, workflowDir)

	assert.NoError(t, err)
	assert.Contains(t, result, "Cleaned path content")
}

// TestResolveAndFormatSkills_MultipleSeparation verifies multiple skills are separated by double newline.
func TestResolveAndFormatSkills_MultipleSeparation(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	skill1 := &workflow.Skill{
		Name:      "skill-1",
		Content:   "Content 1",
		Location:  "/path/1",
		Resources: []string{},
	}
	skill2 := &workflow.Skill{
		Name:      "skill-2",
		Content:   "Content 2",
		Location:  "/path/2",
		Resources: []string{},
	}
	repo.SetSkill("skill-1", skill1)
	repo.SetSkill("skill-2", skill2)

	refs := []workflow.SkillReference{
		{Name: "skill-1"},
		{Name: "skill-2"},
	}

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, "/workflow/dir")

	assert.NoError(t, err)
	assert.Contains(t, result, `<skill_content name="skill-1">`)
	assert.Contains(t, result, `<skill_content name="skill-2">`)
	assert.Contains(t, result, "</skill_content>\n\n<skill_content")
}

// TestResolveAndFormatSkills_MissingSkillError verifies missing skill error is propagated immediately.
func TestResolveAndFormatSkills_MissingSkillError(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	refs := []workflow.SkillReference{
		{Name: "missing-skill"},
	}

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, "/workflow/dir")

	assert.Error(t, err)
	assert.Equal(t, "", result)
}

// TestResolveAndFormatSkills_MixedNameAndPathRefs verifies both name and path refs work together.
func TestResolveAndFormatSkills_MixedNameAndPathRefs(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	skill1 := &workflow.Skill{
		Name:      "skill-by-name",
		Content:   "Content from name-based ref",
		Location:  "/registry/skill-by-name",
		Resources: []string{},
	}
	skill2 := &workflow.Skill{
		Name:      "skill-by-path",
		Content:   "Content from path-based ref",
		Location:  "/workflow/dir/skills/local-skill",
		Resources: []string{},
	}
	repo.SetSkill("skill-by-name", skill1)
	repo.SetSkill("/workflow/dir/skills/local-skill", skill2)

	refs := []workflow.SkillReference{
		{Name: "skill-by-name"},
		{Path: "skills/local-skill"},
	}

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, "/workflow/dir")

	assert.NoError(t, err)
	assert.Contains(t, result, "Content from name-based ref")
	assert.Contains(t, result, "Content from path-based ref")
}

// TestResolveAndFormatSkills_SkillsInDeclarationOrder verifies multiple refs are concatenated in order.
func TestResolveAndFormatSkills_SkillsInDeclarationOrder(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	skill1 := &workflow.Skill{
		Name:      "first",
		Content:   "First skill",
		Location:  "/path/1",
		Resources: []string{},
	}
	skill2 := &workflow.Skill{
		Name:      "second",
		Content:   "Second skill",
		Location:  "/path/2",
		Resources: []string{},
	}
	skill3 := &workflow.Skill{
		Name:      "third",
		Content:   "Third skill",
		Location:  "/path/3",
		Resources: []string{},
	}
	repo.SetSkill("first", skill1)
	repo.SetSkill("second", skill2)
	repo.SetSkill("third", skill3)

	refs := []workflow.SkillReference{
		{Name: "first"},
		{Name: "second"},
		{Name: "third"},
	}

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, "/workflow/dir")

	assert.NoError(t, err)

	pos1 := strings.Index(result, "First skill")
	pos2 := strings.Index(result, "Second skill")
	pos3 := strings.Index(result, "Third skill")

	require.NotEqual(t, -1, pos1, "First skill not found")
	require.NotEqual(t, -1, pos2, "Second skill not found")
	require.NotEqual(t, -1, pos3, "Third skill not found")
	assert.True(t, pos1 < pos2, "First skill should appear before second")
	assert.True(t, pos2 < pos3, "Second skill should appear before third")
}

// TestResolveAndFormatSkills_SingleSkillNoSeparator verifies single skill has no trailing separator.
func TestResolveAndFormatSkills_SingleSkillNoSeparator(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	skill := &workflow.Skill{
		Name:      "single-skill",
		Content:   "Content",
		Location:  "/path",
		Resources: []string{},
	}
	repo.SetSkill("single-skill", skill)

	refs := []workflow.SkillReference{
		{Name: "single-skill"},
	}

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, "/workflow/dir")

	assert.NoError(t, err)
	assert.NotContains(t, result, "\n\n</skill_content>\n\n")
}

// TestResolveAndFormatSkills_RepositoryErrorPropagation verifies repo errors are propagated.
func TestResolveAndFormatSkills_RepositoryErrorPropagation(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockSkillRepository()

	skill := &workflow.Skill{
		Name:      "skill1",
		Content:   "Content 1",
		Location:  "/path/1",
		Resources: []string{},
	}
	repo.SetSkill("skill1", skill)

	// Setup repo to fail on second Load call by clearing and not setting skill2
	refs := []workflow.SkillReference{
		{Name: "skill1"},
		{Name: "skill2"}, // This skill doesn't exist
	}

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, "/workflow/dir")

	assert.Error(t, err)
	assert.Equal(t, "", result)
}

// TestResolveAndFormatSkills_ContextPropagation verifies context is passed to repo methods.
func TestResolveAndFormatSkills_ContextPropagation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := mocks.NewMockSkillRepository()

	skill := &workflow.Skill{
		Name:      "test-skill",
		Content:   "Content",
		Location:  "/path",
		Resources: []string{},
	}
	repo.SetSkill("test-skill", skill)

	refs := []workflow.SkillReference{
		{Name: "test-skill"},
	}

	cancel()

	result, err := application.ResolveAndFormatSkills(ctx, repo, refs, "/workflow/dir")

	assert.Error(t, err)
	assert.Equal(t, "", result)
}
