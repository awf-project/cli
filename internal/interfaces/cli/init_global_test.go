package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// TestInitGlobalCommand tests the `awf init --global` feature (F044 US4).
// These tests use XDG test fixtures from tests/fixtures/xdg/.
func TestInitGlobalCommand(t *testing.T) {
	// Get project root for fixtures path
	projectRoot, err := filepath.Abs("../../..")
	require.NoError(t, err)
	fixturesPath := filepath.Join(projectRoot, "tests", "fixtures", "xdg")

	t.Run("creates global prompts directory when it does not exist", func(t *testing.T) {
		// Create temp directory to simulate fresh XDG_CONFIG_HOME
		tmpDir := t.TempDir()

		// Set XDG_CONFIG_HOME to temp directory
		originalXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
		t.Cleanup(func() {
			if originalXDGConfigHome == "" {
				os.Unsetenv("XDG_CONFIG_HOME")
			} else {
				os.Setenv("XDG_CONFIG_HOME", originalXDGConfigHome)
			}
		})
		os.Setenv("XDG_CONFIG_HOME", tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify global prompts directory was created
		globalPromptsDir := filepath.Join(tmpDir, "awf", "prompts")
		info, err := os.Stat(globalPromptsDir)
		require.NoError(t, err, "global prompts directory should be created")
		assert.True(t, info.IsDir())
	})

	t.Run("creates example prompt in global directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		originalXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
		t.Cleanup(func() {
			if originalXDGConfigHome == "" {
				os.Unsetenv("XDG_CONFIG_HOME")
			} else {
				os.Setenv("XDG_CONFIG_HOME", originalXDGConfigHome)
			}
		})
		os.Setenv("XDG_CONFIG_HOME", tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify example prompt was created
		examplePrompt := filepath.Join(tmpDir, "awf", "prompts", "example.md")
		_, err = os.Stat(examplePrompt)
		require.NoError(t, err, "example prompt should be created in global directory")

		// Verify content is meaningful
		content, err := os.ReadFile(examplePrompt)
		require.NoError(t, err)
		assert.NotEmpty(t, content)
		assert.Contains(t, string(content), "#", "should contain markdown heading")
	})

	t.Run("preserves existing prompts with --force flag", func(t *testing.T) {
		// Use existing fixture with pre-existing prompts
		originalXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
		t.Cleanup(func() {
			if originalXDGConfigHome == "" {
				os.Unsetenv("XDG_CONFIG_HOME")
			} else {
				os.Setenv("XDG_CONFIG_HOME", originalXDGConfigHome)
			}
		})

		// Create temp directory and copy fixture to it (so we don't modify fixtures)
		tmpDir := t.TempDir()
		configFixture := filepath.Join(fixturesPath, "config")

		// Copy fixture structure to temp
		err := copyDir(configFixture, tmpDir)
		require.NoError(t, err)

		os.Setenv("XDG_CONFIG_HOME", tmpDir)

		// Verify pre-existing prompt exists before init
		existingPrompt := filepath.Join(tmpDir, "awf", "prompts", "global-example.md")
		_, err = os.Stat(existingPrompt)
		require.NoError(t, err, "fixture should have pre-existing prompt")

		// Run init --global --force
		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global", "--force"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err = cmd.Execute()
		require.NoError(t, err)

		// Verify existing prompt is preserved (not deleted)
		_, err = os.Stat(existingPrompt)
		require.NoError(t, err, "existing prompts should be preserved")

		// Verify content wasn't modified
		content, err := os.ReadFile(existingPrompt)
		require.NoError(t, err)
		assert.Contains(t, string(content), "pre-existing global prompt")
	})

	t.Run("skips if global prompts directory already exists without --force", func(t *testing.T) {
		// Use fixture with existing prompts
		originalXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
		t.Cleanup(func() {
			if originalXDGConfigHome == "" {
				os.Unsetenv("XDG_CONFIG_HOME")
			} else {
				os.Setenv("XDG_CONFIG_HOME", originalXDGConfigHome)
			}
		})

		// Copy fixture to temp
		tmpDir := t.TempDir()
		configFixture := filepath.Join(fixturesPath, "config")
		err := copyDir(configFixture, tmpDir)
		require.NoError(t, err)

		os.Setenv("XDG_CONFIG_HOME", tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err = cmd.Execute()
		require.NoError(t, err)

		// Should show message about already initialized
		output := out.String()
		assert.Contains(t, output, "already", "should indicate global config exists")
	})

	t.Run("respects custom XDG_CONFIG_HOME", func(t *testing.T) {
		// Create a custom config home path
		customConfigHome := t.TempDir()

		originalXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
		t.Cleanup(func() {
			if originalXDGConfigHome == "" {
				os.Unsetenv("XDG_CONFIG_HOME")
			} else {
				os.Setenv("XDG_CONFIG_HOME", originalXDGConfigHome)
			}
		})
		os.Setenv("XDG_CONFIG_HOME", customConfigHome)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify prompts directory was created in custom location
		promptsDir := filepath.Join(customConfigHome, "awf", "prompts")
		info, err := os.Stat(promptsDir)
		require.NoError(t, err, "should create prompts in custom XDG_CONFIG_HOME")
		assert.True(t, info.IsDir())
	})

	t.Run("displays success message with created path", func(t *testing.T) {
		tmpDir := t.TempDir()

		originalXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
		t.Cleanup(func() {
			if originalXDGConfigHome == "" {
				os.Unsetenv("XDG_CONFIG_HOME")
			} else {
				os.Setenv("XDG_CONFIG_HOME", originalXDGConfigHome)
			}
		})
		os.Setenv("XDG_CONFIG_HOME", tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--global"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Should show success message with path
		output := out.String()
		assert.Contains(t, output, "prompts", "output should mention prompts")
		// Should reference the actual path or awf
		assert.Contains(t, output, "awf", "output should mention awf directory")
	})
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .gitkeep files
		if info.Name() == ".gitkeep" {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}
