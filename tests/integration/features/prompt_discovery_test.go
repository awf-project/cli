//go:build integration

// Feature: F044
// Package features_test contains integration tests for awf prompt discovery feature.
// These tests validate end-to-end behavior of XDG-compliant prompt discovery
// across local (.awf/prompts/) and global ($XDG_CONFIG_HOME/awf/prompts/) directories.
package features_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptDiscovery_ListPrompts_LocalOnly_Integration(t *testing.T) {
	// Setup: Create temp directory with local prompts only
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(localPrompts, 0o755))

	// Create local prompt files
	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "system.md"),
		[]byte("# System Prompt\nLocal system prompt content"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "task.md"),
		[]byte("# Task Prompt\nLocal task content"),
		0o644,
	))

	// Set XDG to non-existent directory to isolate local-only test
	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent-xdg"))
	require.NoError(t, os.Chdir(projectDir))

	// Execute: awf list prompts
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "system.md")
	assert.Contains(t, output, "task.md")
	assert.Contains(t, output, "local")
}

func TestPromptDiscovery_ListPrompts_GlobalOnly_Integration(t *testing.T) {
	// Setup: Create temp directory with global prompts only
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Create global prompts directory
	xdgDir := filepath.Join(tmpDir, "xdg-config")
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(globalPrompts, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(globalPrompts, "global-system.md"),
		[]byte("# Global System\nGlobal system prompt content"),
		0o644,
	))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	require.NoError(t, os.Chdir(projectDir))

	// Execute: awf list prompts
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "global-system.md")
	assert.Contains(t, output, "global")
}

func TestPromptDiscovery_ListPrompts_BothSources_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(localPrompts, 0o755))

	xdgDir := filepath.Join(tmpDir, "xdg-config")
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(globalPrompts, 0o755))

	// Create local prompts
	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "local-only.md"),
		[]byte("Local only content"),
		0o644,
	))

	// Create global prompts
	require.NoError(t, os.WriteFile(
		filepath.Join(globalPrompts, "global-only.md"),
		[]byte("Global only content"),
		0o644,
	))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	require.NoError(t, os.Chdir(projectDir))

	// Execute: awf list prompts
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "local-only.md")
	assert.Contains(t, output, "global-only.md")
	assert.Contains(t, output, "local")
	assert.Contains(t, output, "global")
}

func TestPromptDiscovery_LocalOverridesGlobal_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(localPrompts, 0o755))

	xdgDir := filepath.Join(tmpDir, "xdg-config")
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(globalPrompts, 0o755))

	// Same filename in both directories
	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "shared.md"),
		[]byte("LOCAL VERSION"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(globalPrompts, "shared.md"),
		[]byte("GLOBAL VERSION"),
		0o644,
	))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	require.NoError(t, os.Chdir(projectDir))

	// List should show only local version
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Should only show one shared.md with local source
	lines := strings.Split(output, "\n")
	sharedLines := 0
	for _, line := range lines {
		if strings.Contains(line, "shared.md") {
			sharedLines++
			assert.Contains(t, line, "local", "shared.md should be from local source")
		}
	}
	assert.Equal(t, 1, sharedLines, "should have exactly one shared.md entry")
}

func TestPromptDiscovery_NestedDirectories_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")

	// Create nested structure
	require.NoError(t, os.MkdirAll(filepath.Join(localPrompts, "agents", "claude"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(localPrompts, "tasks"), 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "agents", "claude", "system.md"),
		[]byte("Claude system prompt"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "tasks", "code-review.md"),
		[]byte("Code review task"),
		0o644,
	))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Should show nested paths in output
	assert.Contains(t, output, "agents/claude/system.md")
	assert.Contains(t, output, "tasks/code-review.md")
}

func TestPromptDiscovery_NestedOverride_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(filepath.Join(localPrompts, "nested"), 0o755))

	xdgDir := filepath.Join(tmpDir, "xdg-config")
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(filepath.Join(globalPrompts, "nested"), 0o755))

	// Same nested path in both
	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "nested", "deep.md"),
		[]byte("LOCAL NESTED"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(globalPrompts, "nested", "deep.md"),
		[]byte("GLOBAL NESTED"),
		0o644,
	))
	// Additional global-only nested prompt
	require.NoError(t, os.WriteFile(
		filepath.Join(globalPrompts, "nested", "global-only.md"),
		[]byte("GLOBAL ONLY NESTED"),
		0o644,
	))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Nested deep.md should be from local
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "nested/deep.md") {
			assert.Contains(t, line, "local", "nested/deep.md should be from local")
		}
	}
	// Global-only nested should still be visible
	assert.Contains(t, output, "nested/global-only.md", "should list global-only nested prompt")
}

func TestPromptDiscovery_EmptyDirectories_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(localPrompts, 0o755))

	xdgDir := filepath.Join(tmpDir, "xdg-config")
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(globalPrompts, 0o755))

	// Both directories exist but are empty

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No prompts found")
}

func TestPromptDiscovery_MissingDirectories_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	// Don't create .awf/prompts

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No prompts found")
}

func TestPromptDiscovery_VariousFileExtensions_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(localPrompts, 0o755))

	// Create prompts with various extensions
	extensions := map[string]string{
		"markdown.md":   "Markdown prompt",
		"text.txt":      "Text prompt",
		"custom.prompt": "Custom extension",
		"noext":         "No extension",
		"yaml.yaml":     "YAML prompt",
	}

	for name, content := range extensions {
		require.NoError(t, os.WriteFile(
			filepath.Join(localPrompts, name),
			[]byte(content),
			0o644,
		))
	}

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	for name := range extensions {
		assert.Contains(t, output, name, "should list prompt with extension: %s", name)
	}
}

func TestPromptDiscovery_JSONFormat_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(localPrompts, 0o755))

	xdgDir := filepath.Join(tmpDir, "xdg-config")
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(globalPrompts, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "local.md"),
		[]byte("Local content"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(globalPrompts, "global.md"),
		[]byte("Global content"),
		0o644,
	))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts", "--format", "json"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Verify JSON structure
	assert.Contains(t, output, `"name"`)
	assert.Contains(t, output, `"source"`)
	assert.Contains(t, output, `"local.md"`)
	assert.Contains(t, output, `"global.md"`)
}

func TestPromptResolution_LocalPrompt_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	wfDir := filepath.Join(projectDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(localPrompts, 0o755))
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create prompt file
	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "test-prompt.md"),
		[]byte("RESOLVED_PROMPT_CONTENT"),
		0o644,
	))

	// Create workflow that uses the prompt
	wfYAML := `name: prompt-test
version: "1.0.0"
inputs:
  - name: prompt
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "{{.inputs.prompt}}"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "prompt-test.yaml"), []byte(wfYAML), 0o644))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	origWF := os.Getenv("AWF_WORKFLOWS_PATH")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if origWF != "" {
			t.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "prompt-test",
		"--input", "prompt=@prompts/test-prompt.md",
		"--storage", tmpDir,
		"--output", "buffered",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "RESOLVED_PROMPT_CONTENT")
}

func TestPromptResolution_GlobalPrompt_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	wfDir := filepath.Join(projectDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	xdgDir := filepath.Join(tmpDir, "xdg-config")
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(globalPrompts, 0o755))

	// Create global prompt file
	require.NoError(t, os.WriteFile(
		filepath.Join(globalPrompts, "global-prompt.md"),
		[]byte("GLOBAL_PROMPT_CONTENT"),
		0o644,
	))

	// Create workflow
	wfYAML := `name: global-prompt-test
version: "1.0.0"
inputs:
  - name: prompt
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "{{.inputs.prompt}}"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "global-prompt-test.yaml"), []byte(wfYAML), 0o644))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	origWF := os.Getenv("AWF_WORKFLOWS_PATH")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if origWF != "" {
			t.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "global-prompt-test",
		"--input", "prompt=@prompts/global-prompt.md",
		"--storage", tmpDir,
		"--output", "buffered",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "GLOBAL_PROMPT_CONTENT")
}

func TestPromptResolution_LocalOverridesGlobal_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	wfDir := filepath.Join(projectDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(localPrompts, 0o755))
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	xdgDir := filepath.Join(tmpDir, "xdg-config")
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(globalPrompts, 0o755))

	// Same name in both - local should win
	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "shared.md"),
		[]byte("LOCAL_WINS"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(globalPrompts, "shared.md"),
		[]byte("GLOBAL_LOSES"),
		0o644,
	))

	// Create workflow
	wfYAML := `name: override-test
version: "1.0.0"
inputs:
  - name: prompt
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "{{.inputs.prompt}}"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "override-test.yaml"), []byte(wfYAML), 0o644))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	origWF := os.Getenv("AWF_WORKFLOWS_PATH")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if origWF != "" {
			t.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "override-test",
		"--input", "prompt=@prompts/shared.md",
		"--storage", tmpDir,
		"--output", "buffered",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "LOCAL_WINS")
	assert.NotContains(t, output, "GLOBAL_LOSES")
}

func TestPromptResolution_NestedPath_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	wfDir := filepath.Join(projectDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(filepath.Join(localPrompts, "agents", "claude"), 0o755))
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create nested prompt
	require.NoError(t, os.WriteFile(
		filepath.Join(localPrompts, "agents", "claude", "system.md"),
		[]byte("NESTED_CLAUDE_SYSTEM"),
		0o644,
	))

	// Create workflow
	wfYAML := `name: nested-prompt-test
version: "1.0.0"
inputs:
  - name: prompt
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "{{.inputs.prompt}}"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "nested-prompt-test.yaml"), []byte(wfYAML), 0o644))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	origWF := os.Getenv("AWF_WORKFLOWS_PATH")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if origWF != "" {
			t.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "nested-prompt-test",
		"--input", "prompt=@prompts/agents/claude/system.md",
		"--storage", tmpDir,
		"--output", "buffered",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "NESTED_CLAUDE_SYSTEM")
}

func TestPromptResolution_NotFound_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	wfDir := filepath.Join(projectDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow but no prompts
	wfYAML := `name: missing-prompt-test
version: "1.0.0"
inputs:
  - name: prompt
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "{{.inputs.prompt}}"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "missing-prompt-test.yaml"), []byte(wfYAML), 0o644))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	origWF := os.Getenv("AWF_WORKFLOWS_PATH")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if origWF != "" {
			t.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "missing-prompt-test",
		"--input", "prompt=@prompts/nonexistent.md",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPromptResolution_PathTraversal_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	wfDir := filepath.Join(projectDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow
	wfYAML := `name: traversal-test
version: "1.0.0"
inputs:
  - name: prompt
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "{{.inputs.prompt}}"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "traversal-test.yaml"), []byte(wfYAML), 0o644))

	origDir, _ := os.Getwd()
	origWF := os.Getenv("AWF_WORKFLOWS_PATH")
	defer func() {
		os.Chdir(origDir)
		if origWF != "" {
			t.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	require.NoError(t, os.Chdir(projectDir))

	tests := []struct {
		name  string
		input string
	}{
		{"parent directory traversal", "@prompts/../../../etc/passwd"},
		{"absolute path", "@prompts//etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{
				"run", "traversal-test",
				"--input", "prompt=" + tt.input,
				"--storage", tmpDir,
			})

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid prompt path")
		})
	}
}

func TestInitGlobal_CreatesDirectory_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	xdgDir := filepath.Join(tmpDir, "xdg-config")
	// Don't create the directory - init should create it

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"init", "--global"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify directory was created
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	info, err := os.Stat(globalPrompts)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify example.md was created
	examplePath := filepath.Join(globalPrompts, "example.md")
	_, err = os.Stat(examplePath)
	require.NoError(t, err)
}

func TestInitGlobal_PreservesExisting_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	xdgDir := filepath.Join(tmpDir, "xdg-config")
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(globalPrompts, 0o755))

	// Create existing prompt
	existingContent := "EXISTING_PROMPT_CONTENT"
	require.NoError(t, os.WriteFile(
		filepath.Join(globalPrompts, "my-prompt.md"),
		[]byte(existingContent),
		0o644,
	))

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"init", "--global"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify existing prompt was preserved
	content, err := os.ReadFile(filepath.Join(globalPrompts, "my-prompt.md"))
	require.NoError(t, err)
	assert.Equal(t, existingContent, string(content))
}

func TestInitGlobal_XDGCompliance_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Set custom XDG_CONFIG_HOME
	customXDG := filepath.Join(tmpDir, "custom-config-home")

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", customXDG)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"init", "--global"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify it used the custom XDG path
	expectedPath := filepath.Join(customXDG, "awf", "prompts")
	_, err = os.Stat(expectedPath)
	require.NoError(t, err)
}

func TestPromptDiscovery_WithFixtures_Integration(t *testing.T) {
	// Get project root
	origDir, err := os.Getwd()
	require.NoError(t, err)

	// This test uses the actual fixtures in tests/fixtures/prompts/
	fixtureLocal := filepath.Join(origDir, "..", "..", "fixtures", "prompts", "local")
	fixtureGlobal := filepath.Join(origDir, "..", "..", "fixtures", "prompts", "global")

	// Create temp project directory
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	localPrompts := filepath.Join(projectDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(localPrompts, 0o755))

	// Symlink or copy fixture files to local prompts
	// For simplicity, let's copy the files
	copyFile := func(src, dst string) error {
		content, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, content, 0o644)
	}

	// Copy local fixtures
	entries, err := os.ReadDir(fixtureLocal)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				err := copyFile(
					filepath.Join(fixtureLocal, entry.Name()),
					filepath.Join(localPrompts, entry.Name()),
				)
				require.NoError(t, err)
			}
		}
	}

	// Create XDG with global fixtures
	xdgDir := filepath.Join(tmpDir, "xdg")
	globalPrompts := filepath.Join(xdgDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(globalPrompts, 0o755))

	// Copy global fixtures
	entries, err = os.ReadDir(fixtureGlobal)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				err := copyFile(
					filepath.Join(fixtureGlobal, entry.Name()),
					filepath.Join(globalPrompts, entry.Name()),
				)
				require.NoError(t, err)
			}
		}
	}

	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			t.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify prompts from fixtures are discovered
	// Local-only prompt should exist
	assert.Contains(t, output, "local-only.md")

	// Global-only prompt should exist
	assert.Contains(t, output, "system.md")

	// Shared prompt should show local source (local overrides global)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "shared.md") {
			assert.Contains(t, line, "local", "shared.md should be from local source")
		}
	}
}
