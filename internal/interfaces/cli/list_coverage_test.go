package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// These tests focus on code coverage, not strict behavior validation

func setupWorkflows(t *testing.T, workflows map[string]string) func() {
	t.Helper()
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))
	for name, content := range workflows {
		require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, name+".yaml"), []byte(content), 0644))
	}
	require.NoError(t, os.Chdir(tmpDir))
	return func() { _ = os.Chdir(origDir) }
}

func TestList_TextFormat(t *testing.T) {
	wfs := map[string]string{"test": simpleWF, "test2": fullWF}
	cleanup := setupWorkflows(t, wfs)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list"})
	_ = cmd.Execute()
}

func TestList_JSONFormat(t *testing.T) {
	wfs := map[string]string{"test": simpleWF}
	cleanup := setupWorkflows(t, wfs)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--format", "json"})
	_ = cmd.Execute()
}

func TestList_QuietFormat(t *testing.T) {
	wfs := map[string]string{"test": simpleWF}
	cleanup := setupWorkflows(t, wfs)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--format", "quiet"})
	_ = cmd.Execute()
}

func TestList_TableFormat(t *testing.T) {
	wfs := map[string]string{"test": fullWF}
	cleanup := setupWorkflows(t, wfs)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--format", "table"})
	_ = cmd.Execute()
}

func TestList_VerboseFlag(t *testing.T) {
	wfs := map[string]string{"test": simpleWF}
	cleanup := setupWorkflows(t, wfs)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--verbose"})
	_ = cmd.Execute()
}

func TestList_NoWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list"})
	_ = cmd.Execute()
}

func TestList_NoWorkflowsJSON(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--format", "json"})
	_ = cmd.Execute()
}

func TestList_BrokenWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "broken.yaml"), []byte("name: broken\nstates:\n  bad: [[["), 0644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list"})
	_ = cmd.Execute()
}

// === list prompts ===

func TestListPrompts_NoDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts"})
	_ = cmd.Execute()
}

func TestListPrompts_NoDirectoryJSON(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--format", "json"})
	_ = cmd.Execute()
}

func TestListPrompts_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts"})
	_ = cmd.Execute()
}

func TestListPrompts_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte("test content"), 0644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts"})
	_ = cmd.Execute()
}

func TestListPrompts_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte("test"), 0644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--format", "json"})
	_ = cmd.Execute()
}

func TestListPrompts_TableFormat(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte("test"), 0644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--format", "table"})
	_ = cmd.Execute()
}

func TestListPrompts_QuietFormat(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte("test"), 0644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--format", "quiet"})
	_ = cmd.Execute()
}

func TestListPrompts_NestedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	nestedDir := filepath.Join(tmpDir, ".awf", "prompts", "sub", "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nestedDir, "test.md"), []byte("test"), 0644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts"})
	_ = cmd.Execute()
}
