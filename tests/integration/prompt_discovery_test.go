//go:build integration

// Feature: F044
// Package integration contains integration tests for awf prompt discovery feature.
// These tests validate end-to-end behavior of XDG-compliant prompt discovery
// across local (.awf/prompts/) and global ($XDG_CONFIG_HOME/awf/prompts/) directories.
package integration_test

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

// =============================================================================
// HAPPY PATH TESTS - Normal usage scenarios
// =============================================================================

func TestPromptDiscovery_ListPrompts_LocalOnly_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent-xdg"))
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
	assert.Contains(t, output, "system.md", "should list local system.md")
	assert.Contains(t, output, "task.md", "should list local task.md")
	assert.Contains(t, output, "local", "should show 'local' source")
}

func TestPromptDiscovery_ListPrompts_GlobalOnly_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
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
	assert.Contains(t, output, "global-system.md", "should list global prompt")
	assert.Contains(t, output, "global", "should show 'global' source")
}

func TestPromptDiscovery_ListPrompts_BothSources_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
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
	assert.Contains(t, output, "local-only.md", "should list local prompt")
	assert.Contains(t, output, "global-only.md", "should list global prompt")
	assert.Contains(t, output, "local", "should show local source")
	assert.Contains(t, output, "global", "should show global source")
}

// =============================================================================
// LOCAL TAKES PRECEDENCE OVER GLOBAL (US2)
// =============================================================================

func TestPromptDiscovery_LocalOverridesGlobal_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
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

// =============================================================================
// NESTED DIRECTORY SUPPORT (FR-006)
// =============================================================================

func TestPromptDiscovery_NestedDirectories_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
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
	assert.Contains(t, output, "agents/claude/system.md", "should list nested prompt")
	assert.Contains(t, output, "tasks/code-review.md", "should list nested task prompt")
}

func TestPromptDiscovery_NestedOverride_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
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

// =============================================================================
// EDGE CASES
// =============================================================================

func TestPromptDiscovery_EmptyDirectories_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No prompts found", "should show no prompts message")
}

func TestPromptDiscovery_MissingDirectories_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	// Don't create .awf/prompts

	origDir, _ := os.Getwd()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Chdir(origDir)
		if origXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	require.NoError(t, os.Chdir(projectDir))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "prompts"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No prompts found", "should show no prompts message for missing dirs")
}

func TestPromptDiscovery_VariousFileExtensions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
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

// =============================================================================
// JSON OUTPUT FORMAT
// =============================================================================

func TestPromptDiscovery_JSONFormat_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
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
	assert.Contains(t, output, `"name"`, "JSON output should contain name field")
	assert.Contains(t, output, `"source"`, "JSON output should contain source field")
	assert.Contains(t, output, `"local.md"`, "JSON should contain local.md")
	assert.Contains(t, output, `"global.md"`, "JSON should contain global.md")
}

// =============================================================================
// PROMPT RESOLUTION IN RUN COMMAND (@prompts/ prefix)
// =============================================================================

func TestPromptResolution_LocalPrompt_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if origWF != "" {
			os.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
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
	assert.Contains(t, output, "RESOLVED_PROMPT_CONTENT", "prompt content should be resolved and passed to workflow")
}

func TestPromptResolution_GlobalPrompt_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if origWF != "" {
			os.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
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
	assert.Contains(t, output, "GLOBAL_PROMPT_CONTENT", "global prompt content should be resolved")
}

func TestPromptResolution_LocalOverridesGlobal_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if origWF != "" {
			os.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
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
	assert.Contains(t, output, "LOCAL_WINS", "local prompt should override global")
	assert.NotContains(t, output, "GLOBAL_LOSES", "global prompt should not appear")
}

func TestPromptResolution_NestedPath_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if origWF != "" {
			os.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
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
	assert.Contains(t, output, "NESTED_CLAUDE_SYSTEM", "nested prompt should be resolved")
}

// =============================================================================
// ERROR HANDLING
// =============================================================================

func TestPromptResolution_NotFound_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if origWF != "" {
			os.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
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
	require.Error(t, err, "should error when prompt not found")
	assert.Contains(t, err.Error(), "not found", "error should mention not found")
}

func TestPromptResolution_PathTraversal_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("AWF_WORKFLOWS_PATH", origWF)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
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
			require.Error(t, err, "should error on path traversal attempt")
			assert.Contains(t, err.Error(), "invalid prompt path", "error should mention invalid path")
		})
	}
}

// =============================================================================
// AWF INIT --GLOBAL
// =============================================================================

func TestInitGlobal_CreatesDirectory_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
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
	require.NoError(t, err, "global prompts directory should exist")
	assert.True(t, info.IsDir(), "should be a directory")

	// Verify example.md was created
	examplePath := filepath.Join(globalPrompts, "example.md")
	_, err = os.Stat(examplePath)
	require.NoError(t, err, "example.md should exist")
}

func TestInitGlobal_PreservesExisting_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
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
	assert.Equal(t, existingContent, string(content), "existing prompt should be preserved")
}

func TestInitGlobal_XDGCompliance_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", customXDG)
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
	require.NoError(t, err, "should create directory at custom XDG_CONFIG_HOME")
}

// =============================================================================
// USING FIXTURES FROM tests/fixtures/prompts/
// =============================================================================

func TestPromptDiscovery_WithFixtures_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Get project root
	origDir, err := os.Getwd()
	require.NoError(t, err)

	// This test uses the actual fixtures in tests/fixtures/prompts/
	fixtureLocal := filepath.Join(origDir, "..", "fixtures", "prompts", "local")
	fixtureGlobal := filepath.Join(origDir, "..", "fixtures", "prompts", "global")

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
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgDir)
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
	assert.Contains(t, output, "local-only.md", "should find local-only fixture")

	// Global-only prompt should exist
	assert.Contains(t, output, "system.md", "should find global system fixture")

	// Shared prompt should show local source (local overrides global)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "shared.md") {
			assert.Contains(t, line, "local", "shared.md should be from local source")
		}
	}
}
