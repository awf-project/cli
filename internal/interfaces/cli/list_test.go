package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestListCommand_NoWorkflows(t *testing.T) {
	// Use temp directory for XDG to isolate from global workflows
	tmpDir := t.TempDir()
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

	// Also ensure no local workflows directory exists
	originalWD, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No workflows found") {
		t.Errorf("expected 'No workflows found' message, got: %s", output)
	}
}

func TestListCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'list' subcommand")
	}
}

func TestListPromptsCommand(t *testing.T) {
	t.Run("subcommand exists", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		listCmd, _, err := cmd.Find([]string{"list"})
		require.NoError(t, err)

		found := false
		for _, sub := range listCmd.Commands() {
			if sub.Name() == "prompts" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected 'list' command to have 'prompts' subcommand")
	})

	t.Run("has alias 'p'", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		listCmd, _, err := cmd.Find([]string{"list"})
		require.NoError(t, err)

		for _, sub := range listCmd.Commands() {
			if sub.Name() == "prompts" {
				assert.Contains(t, sub.Aliases, "p")
				return
			}
		}
		t.Error("prompts subcommand not found")
	})

	t.Run("displays no prompts message when directory is empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		// Isolate from global prompts
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", tmpDir)
		defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

		// Create empty prompts directory
		promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "prompts"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "No prompts found")
	})

	t.Run("displays prompts directory not found hint", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		// Isolate from global prompts
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", tmpDir)
		defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

		// Don't create .awf/prompts directory

		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "prompts"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		// Should hint to run awf init
		assert.Contains(t, output, "awf init")
	})

	t.Run("lists prompt files with metadata", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		// Create prompts directory with files
		promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		// Create test prompt files
		require.NoError(t, os.WriteFile(
			filepath.Join(promptsDir, "system.md"),
			[]byte("You are an AI assistant"),
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(promptsDir, "task.txt"),
			[]byte("Analyze the code"),
			0o644,
		))

		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "prompts"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "system.md")
		assert.Contains(t, output, "task.txt")
	})

	t.Run("lists nested prompt directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		// Create nested prompt structure
		nestedDir := filepath.Join(tmpDir, ".awf", "prompts", "ai", "agents")
		require.NoError(t, os.MkdirAll(nestedDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(nestedDir, "claude.md"),
			[]byte("Claude system prompt"),
			0o644,
		))

		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "prompts"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		// Should show relative path from prompts directory
		assert.Contains(t, output, "ai/agents/claude.md")
	})

	t.Run("outputs JSON format", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(promptsDir, "test.md"),
			[]byte("Test content"),
			0o644,
		))

		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "prompts", "--format", "json"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		// Should be valid JSON with prompt info
		assert.Contains(t, output, `"name"`)
		assert.Contains(t, output, `"test.md"`)
		assert.Contains(t, output, `"size"`)
	})

	t.Run("outputs table format", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(promptsDir, "prompt.md"),
			[]byte("Content"),
			0o644,
		))

		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "prompts", "--format", "table"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		// Table should have headers
		assert.Contains(t, output, "NAME")
		assert.Contains(t, output, "SIZE")
	})

	t.Run("outputs quiet format with names only", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		// Isolate from global prompts
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", tmpDir)
		defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

		promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(promptsDir, "first.md"),
			[]byte("First"),
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(promptsDir, "second.txt"),
			[]byte("Second"),
			0o644,
		))

		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"list", "prompts", "--format", "quiet"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		// Quiet mode: just the names, one per line
		assert.Len(t, lines, 2)
	})

	t.Run("help text explains @prompts/ usage", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		listCmd, _, err := cmd.Find([]string{"list"})
		require.NoError(t, err)

		for _, sub := range listCmd.Commands() {
			if sub.Name() == "prompts" {
				assert.Contains(t, sub.Long, "@prompts/")
				return
			}
		}
		t.Error("prompts subcommand not found")
	})
}
