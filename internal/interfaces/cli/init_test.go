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

func TestInitCommand(t *testing.T) {
	t.Run("creates .awf/workflows directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify directory was created
		workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
		info, err := os.Stat(workflowsDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("creates example workflow", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify example workflow was created
		exampleFile := filepath.Join(tmpDir, ".awf", "workflows", "example.yaml")
		_, err = os.Stat(exampleFile)
		require.NoError(t, err)

		// Verify content
		content, err := os.ReadFile(exampleFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "name: example")
	})

	t.Run("creates .awf/storage directory with subdirs", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify storage directory was created with subdirs
		storageDir := filepath.Join(tmpDir, ".awf", "storage")
		info, err := os.Stat(storageDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())

		statesDir := filepath.Join(storageDir, "states")
		info, err = os.Stat(statesDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())

		logsDir := filepath.Join(storageDir, "logs")
		info, err = os.Stat(logsDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("creates .awf.yaml config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify config file was created
		configFile := filepath.Join(tmpDir, ".awf.yaml")
		_, err = os.Stat(configFile)
		require.NoError(t, err)

		// Verify content has expected fields
		content, err := os.ReadFile(configFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "version:")
		assert.Contains(t, string(content), "log_level:")
	})

	t.Run("skips if .awf directory already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		// Create .awf directory first
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0755))

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Should show message about already initialized
		assert.Contains(t, out.String(), "already initialized")
	})

	t.Run("--force overwrites existing configuration", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		// Create existing .awf directory with custom content
		awfDir := filepath.Join(tmpDir, ".awf")
		workflowsDir := filepath.Join(awfDir, "workflows")
		require.NoError(t, os.MkdirAll(workflowsDir, 0755))

		// Create a custom workflow file
		customFile := filepath.Join(workflowsDir, "custom.yaml")
		require.NoError(t, os.WriteFile(customFile, []byte("custom: true"), 0644))

		// Create existing config
		configFile := filepath.Join(tmpDir, ".awf.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte("old: config"), 0644))

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init", "--force"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify example workflow was created (force should create missing files)
		exampleFile := filepath.Join(workflowsDir, "example.yaml")
		_, err = os.Stat(exampleFile)
		require.NoError(t, err)

		// Custom file should still exist (we don't delete user files)
		_, err = os.Stat(customFile)
		require.NoError(t, err)

		// Config should be updated
		content, err := os.ReadFile(configFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "version:")
		assert.NotContains(t, string(content), "old: config")
	})

	t.Run("displays next steps message", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify next steps are shown
		output := out.String()
		assert.Contains(t, output, "awf list")
		assert.Contains(t, output, "awf run example")
	})

	t.Run("creates .awf/prompts directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify prompts directory was created
		promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
		info, err := os.Stat(promptsDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("creates example prompt file", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"init"})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify example prompt was created
		exampleFile := filepath.Join(tmpDir, ".awf", "prompts", "example.md")
		_, err = os.Stat(exampleFile)
		require.NoError(t, err)

		// Verify content has meaningful example
		content, err := os.ReadFile(exampleFile)
		require.NoError(t, err)
		assert.NotEmpty(t, content)
		// Should contain markdown content
		assert.Contains(t, string(content), "#")
	})

	t.Run("help text mentions prompts directory", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		initCmd, _, err := cmd.Find([]string{"init"})
		require.NoError(t, err)

		longDesc := initCmd.Long
		assert.Contains(t, longDesc, "prompts")
		assert.Contains(t, longDesc, ".awf/prompts/")
	})
}
