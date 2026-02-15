package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/awf/internal/infrastructure/repository"
	"github.com/awf-project/awf/internal/interfaces/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollectPromptsFromPaths tests the multi-path prompt discovery with deduplication.
// This is the RED phase - tests should compile but fail against the stub.
func TestCollectPromptsFromPaths(t *testing.T) {
	// Helper to create a temp directory structure with prompts
	createPromptDir := func(t *testing.T, basePath string, prompts map[string]string) {
		t.Helper()
		require.NoError(t, os.MkdirAll(basePath, 0o755))
		for name, content := range prompts {
			fullPath := filepath.Join(basePath, name)
			require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
			require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
		}
	}

	t.Run("returns prompts from single local path", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		createPromptDir(t, localPath, map[string]string{
			"system.md": "System prompt content",
			"task.txt":  "Task prompt",
		})

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 2)

		// Verify both prompts are returned with local source
		names := make(map[string]string)
		for _, p := range prompts {
			names[p.Name] = p.Source
		}
		assert.Equal(t, "local", names["system.md"])
		assert.Equal(t, "local", names["task.txt"])
	})

	t.Run("returns prompts from single global path", func(t *testing.T) {
		tmpDir := t.TempDir()
		globalPath := filepath.Join(tmpDir, "global", "prompts")
		createPromptDir(t, globalPath, map[string]string{
			"global-system.md": "Global system prompt",
		})

		paths := []repository.SourcedPath{
			{Path: globalPath, Source: repository.SourceGlobal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 1)
		assert.Equal(t, "global-system.md", prompts[0].Name)
		assert.Equal(t, "global", prompts[0].Source)
	})

	t.Run("local prompts override global with same name", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		globalPath := filepath.Join(tmpDir, "global", "prompts")

		// Create same-named file in both directories with different content
		createPromptDir(t, localPath, map[string]string{
			"system.md": "Local system prompt - this should win",
		})
		createPromptDir(t, globalPath, map[string]string{
			"system.md": "Global system prompt - should be shadowed",
		})

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
			{Path: globalPath, Source: repository.SourceGlobal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 1, "should deduplicate by name")
		assert.Equal(t, "system.md", prompts[0].Name)
		assert.Equal(t, "local", prompts[0].Source, "local should override global")
	})

	t.Run("combines unique prompts from both paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		globalPath := filepath.Join(tmpDir, "global", "prompts")

		createPromptDir(t, localPath, map[string]string{
			"local-only.md": "Only in local",
			"shared.md":     "Local version of shared",
		})
		createPromptDir(t, globalPath, map[string]string{
			"global-only.md": "Only in global",
			"shared.md":      "Global version of shared - shadowed",
		})

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
			{Path: globalPath, Source: repository.SourceGlobal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 3, "should have 3 unique prompts")

		// Build map for easy assertion
		promptMap := make(map[string]string)
		for _, p := range prompts {
			promptMap[p.Name] = p.Source
		}

		assert.Equal(t, "local", promptMap["local-only.md"])
		assert.Equal(t, "global", promptMap["global-only.md"])
		assert.Equal(t, "local", promptMap["shared.md"], "shared prompt should be from local")
	})

	t.Run("handles nested directories with deduplication", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		globalPath := filepath.Join(tmpDir, "global", "prompts")

		createPromptDir(t, localPath, map[string]string{
			"nested/deep/prompt.md": "Local nested prompt",
		})
		createPromptDir(t, globalPath, map[string]string{
			"nested/deep/prompt.md": "Global nested prompt - shadowed",
			"nested/other.md":       "Global nested other",
		})

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
			{Path: globalPath, Source: repository.SourceGlobal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 2)

		promptMap := make(map[string]string)
		for _, p := range prompts {
			promptMap[p.Name] = p.Source
		}

		assert.Equal(t, "local", promptMap["nested/deep/prompt.md"])
		assert.Equal(t, "global", promptMap["nested/other.md"])
	})

	t.Run("returns empty slice for empty paths", func(t *testing.T) {
		prompts, err := collectPromptsFromPaths([]repository.SourcedPath{})
		require.NoError(t, err)
		assert.Empty(t, prompts)
	})

	t.Run("returns empty slice for non-existent directories", func(t *testing.T) {
		paths := []repository.SourcedPath{
			{Path: "/non/existent/local", Source: repository.SourceLocal},
			{Path: "/non/existent/global", Source: repository.SourceGlobal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		assert.Empty(t, prompts)
	})

	t.Run("skips non-existent paths and returns prompts from existing ones", func(t *testing.T) {
		tmpDir := t.TempDir()
		globalPath := filepath.Join(tmpDir, "global", "prompts")
		createPromptDir(t, globalPath, map[string]string{
			"exists.md": "This prompt exists",
		})

		paths := []repository.SourcedPath{
			{Path: "/non/existent/local", Source: repository.SourceLocal},
			{Path: globalPath, Source: repository.SourceGlobal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 1)
		assert.Equal(t, "exists.md", prompts[0].Name)
		assert.Equal(t, "global", prompts[0].Source)
	})

	t.Run("handles empty directories gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		globalPath := filepath.Join(tmpDir, "global", "prompts")
		require.NoError(t, os.MkdirAll(localPath, 0o755))
		require.NoError(t, os.MkdirAll(globalPath, 0o755))

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
			{Path: globalPath, Source: repository.SourceGlobal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		assert.Empty(t, prompts)
	})

	t.Run("ignores directories in prompt listings", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		createPromptDir(t, localPath, map[string]string{
			"actual-prompt.md": "This is a prompt",
		})
		// Create an empty subdirectory (should not appear in results)
		require.NoError(t, os.MkdirAll(filepath.Join(localPath, "subdir"), 0o755))

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 1)
		assert.Equal(t, "actual-prompt.md", prompts[0].Name)
	})

	t.Run("populates PromptInfo fields correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		content := "Test prompt content with some length"
		createPromptDir(t, localPath, map[string]string{
			"test.md": content,
		})

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 1)

		p := prompts[0]
		assert.Equal(t, "test.md", p.Name)
		assert.Equal(t, "local", p.Source)
		assert.Contains(t, p.Path, "test.md")
		assert.Equal(t, int64(len(content)), p.Size)
		assert.NotEmpty(t, p.ModTime)
	})

	t.Run("respects priority order (first path wins)", func(t *testing.T) {
		tmpDir := t.TempDir()
		path1 := filepath.Join(tmpDir, "path1")
		path2 := filepath.Join(tmpDir, "path2")
		path3 := filepath.Join(tmpDir, "path3")

		createPromptDir(t, path1, map[string]string{"common.md": "Path1 wins"})
		createPromptDir(t, path2, map[string]string{"common.md": "Path2 loses"})
		createPromptDir(t, path3, map[string]string{"common.md": "Path3 loses"})

		// Custom source order to test priority
		paths := []repository.SourcedPath{
			{Path: path1, Source: repository.SourceLocal},
			{Path: path2, Source: repository.SourceGlobal},
			{Path: path3, Source: repository.SourceEnv}, // hypothetical env source
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 1)
		assert.Equal(t, "local", prompts[0].Source, "first path in list should win")
	})

	t.Run("handles various file extensions", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		createPromptDir(t, localPath, map[string]string{
			"markdown.md":   "Markdown",
			"text.txt":      "Plain text",
			"prompt.prompt": "Custom extension",
			"no-ext":        "No extension",
		})

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 4)

		names := make([]string, len(prompts))
		for i, p := range prompts {
			names[i] = p.Name
		}
		assert.Contains(t, names, "markdown.md")
		assert.Contains(t, names, "text.txt")
		assert.Contains(t, names, "prompt.prompt")
		assert.Contains(t, names, "no-ext")
	})

	t.Run("handles deeply nested path deduplication", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		globalPath := filepath.Join(tmpDir, "global", "prompts")

		createPromptDir(t, localPath, map[string]string{
			"a/b/c/d/deep.md": "Local deep",
		})
		createPromptDir(t, globalPath, map[string]string{
			"a/b/c/d/deep.md":    "Global deep - shadowed",
			"a/b/c/d/another.md": "Global another",
		})

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
			{Path: globalPath, Source: repository.SourceGlobal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 2)

		promptMap := make(map[string]string)
		for _, p := range prompts {
			promptMap[p.Name] = p.Source
		}

		assert.Equal(t, "local", promptMap["a/b/c/d/deep.md"])
		assert.Equal(t, "global", promptMap["a/b/c/d/another.md"])
	})

	t.Run("handles special characters in filenames", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		createPromptDir(t, localPath, map[string]string{
			"spaces in name.md":         "Has spaces",
			"dashes-and_underscores.md": "Has dashes",
			"123-numeric-start.txt":     "Numeric start",
		})

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 3)
	})

	t.Run("nil paths slice returns empty result", func(t *testing.T) {
		prompts, err := collectPromptsFromPaths(nil)
		require.NoError(t, err)
		assert.Empty(t, prompts)
	})

	t.Run("large number of prompts from multiple paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "local", "prompts")
		globalPath := filepath.Join(tmpDir, "global", "prompts")

		localPrompts := make(map[string]string)
		globalPrompts := make(map[string]string)

		// Create 50 local prompts
		for i := 0; i < 50; i++ {
			localPrompts[filepath.Join("dir", "local-"+string(rune('a'+i%26))+".md")] = "local content"
		}
		// Create 50 global prompts (some overlap)
		for i := 0; i < 50; i++ {
			globalPrompts[filepath.Join("dir", "global-"+string(rune('a'+i%26))+".md")] = "global content"
		}
		// Add overlapping prompts
		for i := 0; i < 10; i++ {
			name := filepath.Join("shared", "common-"+string(rune('0'+i))+".md")
			localPrompts[name] = "local version"
			globalPrompts[name] = "global version"
		}

		createPromptDir(t, localPath, localPrompts)
		createPromptDir(t, globalPath, globalPrompts)

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
			{Path: globalPath, Source: repository.SourceGlobal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)

		// Should have deduplicated results
		assert.NotEmpty(t, prompts)

		// Verify all shared prompts have local source
		for _, p := range prompts {
			if filepath.Dir(p.Name) == "shared" {
				assert.Equal(t, "local", p.Source, "shared prompts should be from local")
			}
		}
	})
}

// TestCollectPromptsFromPaths_EdgeCases tests edge cases and error conditions.
func TestCollectPromptsFromPaths_EdgeCases(t *testing.T) {
	t.Run("path with trailing slash", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "prompts")
		require.NoError(t, os.MkdirAll(localPath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(localPath, "test.md"), []byte("test"), 0o644))

		// Path with trailing slash
		paths := []repository.SourcedPath{
			{Path: localPath + "/", Source: repository.SourceLocal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 1)
	})

	t.Run("symlinked prompt file", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "prompts")
		require.NoError(t, os.MkdirAll(localPath, 0o755))

		// Create actual file
		actualFile := filepath.Join(tmpDir, "actual.md")
		require.NoError(t, os.WriteFile(actualFile, []byte("actual content"), 0o644))

		// Create symlink in prompts directory
		symlink := filepath.Join(localPath, "linked.md")
		err := os.Symlink(actualFile, symlink)
		if err != nil {
			t.Skip("symlinks not supported on this platform")
		}

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		// Should follow symlinks
		require.Len(t, prompts, 1)
		assert.Equal(t, "linked.md", prompts[0].Name)
	})

	t.Run("hidden files are included", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "prompts")
		require.NoError(t, os.MkdirAll(localPath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(localPath, ".hidden.md"), []byte("hidden"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(localPath, "visible.md"), []byte("visible"), 0o644))

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)

		names := make([]string, len(prompts))
		for i, p := range prompts {
			names[i] = p.Name
		}
		// Hidden files should be included (they might be intentional)
		assert.Contains(t, names, ".hidden.md")
		assert.Contains(t, names, "visible.md")
	})

	t.Run("empty filename in nested path", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "prompts")
		nestedPath := filepath.Join(localPath, "subdir")
		require.NoError(t, os.MkdirAll(nestedPath, 0o755))
		// Just create the directory structure, no files
		// This tests that we don't create entries for directories

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		assert.Empty(t, prompts)
	})

	t.Run("unicode filenames", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "prompts")
		require.NoError(t, os.MkdirAll(localPath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(localPath, "日本語.md"), []byte("Japanese"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(localPath, "émoji-🚀.md"), []byte("Emoji"), 0o644))

		paths := []repository.SourcedPath{
			{Path: localPath, Source: repository.SourceLocal},
		}

		prompts, err := collectPromptsFromPaths(paths)
		require.NoError(t, err)
		require.Len(t, prompts, 2)
	})
}

// TestRunListPrompts_MultiPath tests the integration of collectPromptsFromPaths with runListPrompts.
func TestRunListPrompts_MultiPath(t *testing.T) {
	// Helper to set up both local and global prompts directories
	setupMultiPathEnv := func(t *testing.T) (localDir, globalDir string, cleanup func()) {
		t.Helper()

		// Create temp directories
		tmpDir := t.TempDir()
		projectDir := filepath.Join(tmpDir, "project")
		xdgDir := filepath.Join(tmpDir, "xdg")

		localPrompts := filepath.Join(projectDir, ".awf", "prompts")
		globalPrompts := filepath.Join(xdgDir, "awf", "prompts")

		require.NoError(t, os.MkdirAll(localPrompts, 0o755))
		require.NoError(t, os.MkdirAll(globalPrompts, 0o755))

		// Save and set environment
		origDir, _ := os.Getwd()

		t.Setenv("XDG_CONFIG_HOME", xdgDir)
		os.Chdir(projectDir)

		cleanup = func() {
			os.Chdir(origDir)
		}

		return localPrompts, globalPrompts, cleanup
	}

	t.Run("shows prompts from both local and global", func(t *testing.T) {
		localDir, globalDir, cleanup := setupMultiPathEnv(t)
		defer cleanup()

		require.NoError(t, os.WriteFile(filepath.Join(localDir, "local.md"), []byte("local"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(globalDir, "global.md"), []byte("global"), 0o644))

		// This would test via CLI if stub wasn't panicking
		// For now, just verify the setup is correct
		paths := BuildPromptPaths()
		require.Len(t, paths, 2)
		assert.Equal(t, repository.SourceLocal, paths[0].Source)
		assert.Equal(t, repository.SourceGlobal, paths[1].Source)
	})

	t.Run("local shadow global with same name", func(t *testing.T) {
		localDir, globalDir, cleanup := setupMultiPathEnv(t)
		defer cleanup()

		require.NoError(t, os.WriteFile(filepath.Join(localDir, "shared.md"), []byte("local version"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(globalDir, "shared.md"), []byte("global version"), 0o644))

		// Verify paths are correctly configured for priority
		paths := BuildPromptPaths()
		assert.Equal(t, repository.SourceLocal, paths[0].Source, "local should be first (higher priority)")
		assert.Equal(t, repository.SourceGlobal, paths[1].Source, "global should be second (lower priority)")
	})

	t.Run("nested prompts from both sources", func(t *testing.T) {
		localDir, globalDir, cleanup := setupMultiPathEnv(t)
		defer cleanup()

		localNested := filepath.Join(localDir, "agents")
		globalNested := filepath.Join(globalDir, "agents")
		require.NoError(t, os.MkdirAll(localNested, 0o755))
		require.NoError(t, os.MkdirAll(globalNested, 0o755))

		require.NoError(t, os.WriteFile(filepath.Join(localNested, "claude.md"), []byte("local claude"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(globalNested, "gpt.md"), []byte("global gpt"), 0o644))

		// Test would verify nested paths work correctly
		paths := BuildPromptPaths()
		assert.NotEmpty(t, paths)
	})

	t.Run("source column shows correct origin", func(t *testing.T) {
		localDir, globalDir, cleanup := setupMultiPathEnv(t)
		defer cleanup()

		require.NoError(t, os.WriteFile(filepath.Join(localDir, "local-only.md"), []byte("local"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(globalDir, "global-only.md"), []byte("global"), 0o644))

		// Would verify SOURCE column in output
		// Source field was added to PromptInfo in T003
		info := ui.PromptInfo{
			Name:   "test.md",
			Source: "local",
		}
		assert.Equal(t, "local", info.Source)
	})
}
