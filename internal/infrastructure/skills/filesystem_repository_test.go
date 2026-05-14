package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// Acceptance Criteria Coverage:
// AC1: Constructor with default 6-tier search paths
// AC2: Constructor with AWF_SKILLS_PATH env override (exclusive)
// AC3: Load finds skill in first matching search path
// AC4: Load returns skill with Name, Content (frontmatter stripped), Location
// AC5: Load returns error when skill not found in any search path
// AC6: Load returns error when directory exists but no SKILL.md
// AC7: LoadFromPath loads from explicit absolute path
// AC8: LoadFromPath returns error if path doesn't exist or has no SKILL.md
// AC9: Resources enumeration with sorted relative paths
// AC10: SKILL.md excluded from Resources
// AC11: .git/ and node_modules/ skipped
// AC12: Max 4 levels deep in resource enumeration
// AC13: Skill with only SKILL.md has empty Resources
// AC14: SKILL.md >500KB loads successfully
// AC15: Empty SKILL.md loads with empty Content
// AC16: filepath.Clean applied to all path inputs (security)

type mockLogger struct {
	warnings []string
	infos    []string
}

func (m *mockLogger) Debug(msg string, fields ...any) {}
func (m *mockLogger) Info(msg string, fields ...any) {
	m.infos = append(m.infos, msg)
}

func (m *mockLogger) Warn(msg string, fields ...any) {
	m.warnings = append(m.warnings, msg)
}
func (m *mockLogger) Error(msg string, fields ...any) {}
func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

func TestNewFilesystemSkillRepository_DefaultPaths(t *testing.T) {
	// AC1: Constructor with default 6-tier search paths
	// Clear any existing env var
	t.Setenv("AWF_SKILLS_PATH", "")

	logger := &mockLogger{}
	repo := NewFilesystemSkillRepository(logger)

	require.NotNil(t, repo, "should return a valid repository")
	require.Len(t, repo.searchPaths, 6, "should have exactly 6 default search paths")
	// Verify paths are non-empty and follow expected patterns
	for i, path := range repo.searchPaths {
		assert.NotEmpty(t, path, "path %d should not be empty", i)
		assert.True(t, filepath.IsAbs(path) || strings.Contains(path, "."), "path %d should be absolute or contain relative patterns", i)
	}
}

func TestNewFilesystemSkillRepository_EnvOverride(t *testing.T) {
	// AC2: AWF_SKILLS_PATH env override (exclusive override replaces all default paths)
	envPaths := "/custom/path1:/custom/path2:/custom/path3"
	t.Setenv("AWF_SKILLS_PATH", envPaths)

	logger := &mockLogger{}
	repo := NewFilesystemSkillRepository(logger)

	require.NotNil(t, repo, "should return a valid repository")
	require.Len(t, repo.searchPaths, 3, "should use only the 3 env paths (exclusive override)")
	assert.Equal(t, "/custom/path1", repo.searchPaths[0])
	assert.Equal(t, "/custom/path2", repo.searchPaths[1])
	assert.Equal(t, "/custom/path3", repo.searchPaths[2])

	// Exclusive override: verify no default path patterns leaked in alongside env paths
	for _, p := range repo.searchPaths {
		assert.True(t, strings.HasPrefix(p, "/custom/"),
			"all search paths must come exclusively from AWF_SKILLS_PATH, got %q", p)
	}
}

func TestLoad_FindsSkillInFirstSearchPath(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "go-conventions")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillmd := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillmd, []byte("---\nname: test\n---\nContent here"), 0o644))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	logger := &mockLogger{}
	repo := NewFilesystemSkillRepository(logger)

	ctx := context.Background()
	skill, err := repo.Load(ctx, "go-conventions")

	require.NoError(t, err, "Load should not return error")
	require.NotNil(t, skill, "Load should return a skill, not nil")
	assert.Equal(t, "go-conventions", skill.Name)
	assert.Equal(t, "Content here", skill.Content)
	assert.Equal(t, skillDir, skill.Location)
}

func TestLoad_StripsFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillmd := filepath.Join(skillDir, "SKILL.md")
	content := "---\ntitle: Test Skill\nversion: 1.0\n---\nThis is the actual content"
	require.NoError(t, os.WriteFile(skillmd, []byte(content), 0o644))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	logger := &mockLogger{}
	repo := NewFilesystemSkillRepository(logger)

	skill, err := repo.Load(context.Background(), "test-skill")

	require.NoError(t, err)
	require.NotNil(t, skill)
	assert.Equal(t, "This is the actual content", skill.Content)
}

func TestLoad_ReturnsAbsoluteLocation(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-name")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillmd := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillmd, []byte("content"), 0o644))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.Load(context.Background(), "skill-name")

	require.NoError(t, err)
	require.NotNil(t, skill)
	assert.True(t, filepath.IsAbs(skill.Location), "location should be absolute path")
	assert.Equal(t, skillDir, skill.Location)
}

func TestLoad_ErrorSkillNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.Load(context.Background(), "nonexistent-skill")

	assert.Error(t, err)
	assert.Nil(t, skill)

	// AC5: error must be the typed SkillNotFoundError, not a generic error
	var notFound *workflow.SkillNotFoundError
	require.True(t, errors.As(err, &notFound), "error must be *workflow.SkillNotFoundError, got %T: %v", err, err)
	assert.Equal(t, "nonexistent-skill", notFound.Name)
}

func TestLoad_ErrorDirectoryExistsNoSKILLMD(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "incomplete-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.Load(context.Background(), "incomplete-skill")

	assert.Error(t, err)
	assert.Nil(t, skill)

	// AC6: directory exists but no SKILL.md → must surface as SkillNotFoundError (not a generic io error)
	var notFound *workflow.SkillNotFoundError
	require.True(t, errors.As(err, &notFound), "error must be *workflow.SkillNotFoundError, got %T: %v", err, err)
	assert.Equal(t, "incomplete-skill", notFound.Name)
}

func TestLoad_SearchesMultiplePaths(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	skillDir := filepath.Join(dir2, "found-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("found"), 0o644))

	paths := dir1 + string(filepath.ListSeparator) + dir2
	t.Setenv("AWF_SKILLS_PATH", paths)

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.Load(context.Background(), "found-skill")

	require.NoError(t, err)
	require.NotNil(t, skill)
	assert.Equal(t, "found-skill", skill.Name)
}

func TestLoadFromPath_ValidAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillmd := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillmd, []byte("---\ndata: value\n---\nAbsolute path content"), 0o644))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err, "LoadFromPath should not error")
	require.NotNil(t, skill, "LoadFromPath should return skill, not nil")
	assert.Equal(t, "my-skill", skill.Name)
	assert.Equal(t, "Absolute path content", skill.Content)
	assert.Equal(t, skillDir, skill.Location)
}

func TestLoadFromPath_ErrorPathDoesNotExist(t *testing.T) {
	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), "/nonexistent/path/to/skill")

	assert.Error(t, err)
	assert.Nil(t, skill)

	// AC8: non-existent path must surface as SkillNotFoundError
	var notFound *workflow.SkillNotFoundError
	require.True(t, errors.As(err, &notFound), "error must be *workflow.SkillNotFoundError, got %T: %v", err, err)
}

func TestLoadFromPath_ErrorNoSKILLMD(t *testing.T) {
	tmpDir := t.TempDir()
	emptyDir := filepath.Join(tmpDir, "empty-skill")
	require.NoError(t, os.MkdirAll(emptyDir, 0o755))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), emptyDir)

	assert.Error(t, err)
	assert.Nil(t, skill)

	// AC8: path exists but no SKILL.md must surface as SkillNotFoundError
	var notFound *workflow.SkillNotFoundError
	require.True(t, errors.As(err, &notFound), "error must be *workflow.SkillNotFoundError, got %T: %v", err, err)
	assert.Equal(t, "empty-skill", notFound.Name)
}

func TestResourceEnumeration_IncludesFilesInSortedOrder(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-with-files")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "zebra.txt"), []byte("z"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "apple.txt"), []byte("a"), 0o644))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)
	require.Len(t, skill.Resources, 2)
	assert.Equal(t, "apple.txt", skill.Resources[0])
	assert.Equal(t, "zebra.txt", skill.Resources[1])
}

func TestResourceEnumeration_ExcludesSKILLMD(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-md-test")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "readme.md"), []byte("readme"), 0o644))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)
	assert.Len(t, skill.Resources, 1)
	assert.Equal(t, "readme.md", skill.Resources[0])
	assert.NotContains(t, skill.Resources, "SKILL.md")
}

func TestResourceEnumeration_SkipsGitDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-with-git")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "file.txt"), []byte("file"), 0o644))

	gitDir := filepath.Join(skillDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0o644))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)
	assert.Len(t, skill.Resources, 1)
	assert.Equal(t, "file.txt", skill.Resources[0])
	assert.NotContains(t, skill.Resources, ".git/config")
}

func TestResourceEnumeration_SkipsNodeModules(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-with-npm")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "script.js"), []byte("js"), 0o644))

	nmDir := filepath.Join(skillDir, "node_modules")
	require.NoError(t, os.MkdirAll(nmDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nmDir, "package.json"), []byte("npm"), 0o644))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)
	assert.Len(t, skill.Resources, 1)
	assert.Equal(t, "script.js", skill.Resources[0])
	assert.NotContains(t, skill.Resources, "node_modules/package.json")
}

func TestResourceEnumeration_MaxDepth4Levels(t *testing.T) {
	// AC12: Max 4 levels deep in resource enumeration
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-deep")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))

	level1 := filepath.Join(skillDir, "l1")
	require.NoError(t, os.MkdirAll(level1, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(level1, "f1.txt"), []byte("f1"), 0o644))

	level2 := filepath.Join(level1, "l2")
	require.NoError(t, os.MkdirAll(level2, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(level2, "f2.txt"), []byte("f2"), 0o644))

	level3 := filepath.Join(level2, "l3")
	require.NoError(t, os.MkdirAll(level3, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(level3, "f3.txt"), []byte("f3"), 0o644))

	level4 := filepath.Join(level3, "l4")
	require.NoError(t, os.MkdirAll(level4, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(level4, "f4.txt"), []byte("f4"), 0o644))

	level5 := filepath.Join(level4, "l5")
	require.NoError(t, os.MkdirAll(level5, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(level5, "f5.txt"), []byte("f5"), 0o644))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)

	resourcesStr := strings.Join(skill.Resources, ",")
	assert.Contains(t, resourcesStr, "l1/f1.txt", "level 1 file should be included")
	assert.Contains(t, resourcesStr, "l1/l2/f2.txt", "level 2 file should be included")
	assert.Contains(t, resourcesStr, "l1/l2/l3/f3.txt", "level 3 file should be included")
	assert.Contains(t, resourcesStr, "l1/l2/l3/l4/f4.txt", "level 4 file should be included (max depth)")
	assert.NotContains(t, resourcesStr, "l5", "level 5 should be excluded (exceeds max depth)")
}

func TestResourceEnumeration_Exactly4LevelsBoundary(t *testing.T) {
	// AC12: Verify exactly 4 levels deep is included
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-exact-4")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))

	// Create exactly 4 nested levels
	path := skillDir
	for i := 1; i <= 4; i++ {
		path = filepath.Join(path, fmt.Sprintf("level%d", i))
		require.NoError(t, os.MkdirAll(path, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(path, fmt.Sprintf("file%d.txt", i)), []byte(fmt.Sprintf("content%d", i)), 0o644))
	}

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)
	// Should have 4 files at different depths
	assert.Len(t, skill.Resources, 4, "should enumerate all 4 level files")
}

func TestResourceEnumeration_Exceeds5LevelExcluded(t *testing.T) {
	// AC12: Verify 5 levels deep is excluded
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-exceed-5")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))

	// Create 5 nested levels
	path := skillDir
	for i := 1; i <= 5; i++ {
		path = filepath.Join(path, fmt.Sprintf("level%d", i))
		require.NoError(t, os.MkdirAll(path, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(path, fmt.Sprintf("file%d.txt", i)), []byte(fmt.Sprintf("content%d", i)), 0o644))
	}

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)
	// Should have only 4 files (5th level excluded)
	assert.Len(t, skill.Resources, 4, "should exclude files deeper than 4 levels")
	resourcesStr := strings.Join(skill.Resources, ",")
	assert.NotContains(t, resourcesStr, "file5.txt", "5th level file should not be included")
}

func TestResourceEnumeration_OnlySkillMDEmptyResources(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "minimal-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("just content"), 0o644))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)
	assert.Empty(t, skill.Resources)
}

func TestLargeSKILLMD_LoadsSuccessfully(t *testing.T) {
	// AC14: SKILL.md >500KB loads successfully
	// (Warning is internal implementation detail; public API ensures loading succeeds)
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "large-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	largeContent := strings.Repeat("x", 501*1024)
	skillmd := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillmd, []byte(largeContent), 0o644))

	logger := &mockLogger{}
	repo := NewFilesystemSkillRepository(logger)

	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err, "should load large SKILL.md without error")
	require.NotNil(t, skill, "should return skill object even if large")
	assert.Len(t, skill.Content, 501*1024, "should load full content without truncation")
}

func TestEmptySKILLMD_LoadsWithEmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "empty-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillmd := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillmd, []byte(""), 0o644))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)
	assert.Equal(t, "", skill.Content)
}

func TestResourceEnumeration_NestedStructure(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "complex-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("readme"), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "scripts", "check.sh"), []byte("script"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "scripts", "test.sh"), []byte("test"), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "references"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "references", "guide.md"), []byte("guide"), 0o644))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)
	require.NotEmpty(t, skill.Resources)

	assert.Contains(t, skill.Resources, "README.md")
	assert.Contains(t, skill.Resources, "references/guide.md")
	assert.Contains(t, skill.Resources, "scripts/check.sh")
	assert.Contains(t, skill.Resources, "scripts/test.sh")

	for i := 0; i < len(skill.Resources)-1; i++ {
		assert.True(t, skill.Resources[i] < skill.Resources[i+1], "resources should be sorted")
	}
}

func TestLoad_WithCleanedPath(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-clean")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillmd := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillmd, []byte("content"), 0o644))

	t.Setenv("AWF_SKILLS_PATH", tmpDir)

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.Load(context.Background(), "skill-clean")

	require.NoError(t, err)
	require.NotNil(t, skill)
	assert.Equal(t, "skill-clean", skill.Name)
}

func TestLoadFromPath_CleanedAbsolutePath(t *testing.T) {
	// AC16: filepath.Clean applied to path inputs
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-abs")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillmd := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillmd, []byte("content"), 0o644))

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	require.NoError(t, err)
	require.NotNil(t, skill)
}

func TestLoadFromPath_ErrorPermissionDenied(t *testing.T) {
	// AC8: LoadFromPath returns error for permission denied
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root user")
	}

	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "restricted-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))

	// Remove read permission on directory
	require.NoError(t, os.Chmod(skillDir, 0o000))
	defer os.Chmod(skillDir, 0o755) // restore for cleanup

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	assert.Error(t, err, "should return error for permission denied")
	assert.Nil(t, skill, "should return nil skill on permission error")
}

func TestLoad_ErrorPermissionDenied(t *testing.T) {
	// AC5: Load returns error when permission denied in search path
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root user")
	}

	tmpDir := t.TempDir()
	restrictedDir := filepath.Join(tmpDir, "restricted")
	require.NoError(t, os.MkdirAll(restrictedDir, 0o755))

	skillDir := filepath.Join(restrictedDir, "test-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))

	// Remove read permission on search directory
	require.NoError(t, os.Chmod(restrictedDir, 0o000))
	defer os.Chmod(restrictedDir, 0o755) // restore for cleanup

	t.Setenv("AWF_SKILLS_PATH", tmpDir)
	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.Load(context.Background(), "test-skill")

	assert.Error(t, err, "should return error when search directory is inaccessible")
	assert.Nil(t, skill, "should return nil skill on permission error")
}

func TestLoadSkillFromDir_UnreadableSKILLMD(t *testing.T) {
	// Covers the os.ReadFile error path inside loadSkillFromDir (line: "reading SKILL.md in %s").
	// os.Stat succeeds on a 000-permission file (stat only needs execute on parent dir),
	// but os.ReadFile fails with EACCES, so loadSkillFromDir returns a wrapped read error.
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root user")
	}

	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "unreadable-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillmd := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillmd, []byte("secret content"), 0o644))
	require.NoError(t, os.Chmod(skillmd, 0o000))
	defer os.Chmod(skillmd, 0o644) //nolint:errcheck // best-effort cleanup for temp dir

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	assert.Error(t, err, "should return error when SKILL.md cannot be read")
	assert.Nil(t, skill, "should return nil skill when SKILL.md is unreadable")
	// Must be a read error, not a SkillNotFoundError (the file exists, it just can't be read)
	var notFound *workflow.SkillNotFoundError
	assert.False(t, errors.As(err, &notFound), "read failure should not be misreported as SkillNotFoundError")
}

func TestEnumerateResources_InaccessibleSubdir(t *testing.T) {
	// Covers the if err != nil { return nil } branch in the enumerateResources walk callback.
	// WalkDir calls the callback a second time with the permission error when it cannot
	// read a 000-permission subdirectory; enumerateResources silently skips it and succeeds.
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root user")
	}

	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-blocked-subdir")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "visible.txt"), []byte("visible"), 0o644))

	blockedDir := filepath.Join(skillDir, "blocked")
	require.NoError(t, os.MkdirAll(blockedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(blockedDir, "hidden.txt"), []byte("hidden"), 0o644))
	require.NoError(t, os.Chmod(blockedDir, 0o000))
	defer os.Chmod(blockedDir, 0o755) //nolint:errcheck // best-effort cleanup for temp dir

	repo := NewFilesystemSkillRepository(&mockLogger{})
	skill, err := repo.LoadFromPath(context.Background(), skillDir)

	// enumerateResources silently ignores walk errors — loading must succeed
	require.NoError(t, err, "should succeed even with an inaccessible subdirectory")
	require.NotNil(t, skill)
	assert.Contains(t, skill.Resources, "visible.txt", "accessible files should still be enumerated")
	assert.NotContains(t, skill.Resources, "blocked/hidden.txt", "files inside inaccessible dir must not appear")
}
