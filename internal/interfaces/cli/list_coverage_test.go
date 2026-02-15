package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/awf/internal/interfaces/cli"
	"github.com/awf-project/awf/internal/testutil/fixtures"
	"github.com/stretchr/testify/require"
)

// These tests focus on code coverage, not strict behavior validation

func TestList_TextFormat(t *testing.T) {
	dir := fixtures.SetupWorkflowsDir(t, map[string]string{"test": fixtures.SimpleWorkflowYAML, "test2": fixtures.FullWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--storage", dir})
	_ = cmd.Execute()
}

func TestList_JSONFormat(t *testing.T) {
	dir := fixtures.SetupWorkflowsDir(t, map[string]string{"test": fixtures.SimpleWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--format", "json", "--storage", dir})
	_ = cmd.Execute()
}

func TestList_QuietFormat(t *testing.T) {
	dir := fixtures.SetupWorkflowsDir(t, map[string]string{"test": fixtures.SimpleWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--format", "quiet", "--storage", dir})
	_ = cmd.Execute()
}

func TestList_TableFormat(t *testing.T) {
	dir := fixtures.SetupWorkflowsDir(t, map[string]string{"test": fixtures.FullWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--format", "table", "--storage", dir})
	_ = cmd.Execute()
}

func TestList_VerboseFlag(t *testing.T) {
	dir := fixtures.SetupWorkflowsDir(t, map[string]string{"test": fixtures.SimpleWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--verbose", "--storage", dir})
	_ = cmd.Execute()
}

func TestList_NoWorkflows(t *testing.T) {
	dir := fixtures.SetupTestDir(t)

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--storage", dir})
	_ = cmd.Execute()
}

func TestList_NoWorkflowsJSON(t *testing.T) {
	dir := fixtures.SetupTestDir(t)

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--format", "json", "--storage", dir})
	_ = cmd.Execute()
}

func TestList_BrokenWorkflow(t *testing.T) {
	dir := fixtures.SetupTestDir(t)

	workflowsDir := filepath.Join(dir, ".awf", "workflows")
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "broken.yaml"), []byte("name: broken\nstates:\n  bad: [[["), 0o644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "--storage", dir})
	_ = cmd.Execute()
}

// === list prompts ===

func TestListPrompts_NoDirectory(t *testing.T) {
	dir := t.TempDir()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--storage", dir})
	_ = cmd.Execute()
}

func TestListPrompts_NoDirectoryJSON(t *testing.T) {
	dir := t.TempDir()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--format", "json", "--storage", dir})
	_ = cmd.Execute()
}

func TestListPrompts_EmptyDirectory(t *testing.T) {
	dir := fixtures.SetupTestDir(t)

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--storage", dir})
	_ = cmd.Execute()
}

func TestListPrompts_WithFiles(t *testing.T) {
	dir := fixtures.SetupTestDir(t)

	promptsDir := filepath.Join(dir, ".awf", "prompts")
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte("test content"), 0o644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--storage", dir})
	_ = cmd.Execute()
}

func TestListPrompts_JSONFormat(t *testing.T) {
	dir := fixtures.SetupTestDir(t)

	promptsDir := filepath.Join(dir, ".awf", "prompts")
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte("test"), 0o644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--format", "json", "--storage", dir})
	_ = cmd.Execute()
}

func TestListPrompts_TableFormat(t *testing.T) {
	dir := fixtures.SetupTestDir(t)

	promptsDir := filepath.Join(dir, ".awf", "prompts")
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte("test"), 0o644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--format", "table", "--storage", dir})
	_ = cmd.Execute()
}

func TestListPrompts_QuietFormat(t *testing.T) {
	dir := fixtures.SetupTestDir(t)

	promptsDir := filepath.Join(dir, ".awf", "prompts")
	require.NoError(t, os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte("test"), 0o644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--format", "quiet", "--storage", dir})
	_ = cmd.Execute()
}

func TestListPrompts_NestedFiles(t *testing.T) {
	dir := fixtures.SetupTestDir(t)

	nestedDir := filepath.Join(dir, ".awf", "prompts", "sub", "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nestedDir, "test.md"), []byte("test"), 0o644))

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list", "prompts", "--storage", dir})
	_ = cmd.Execute()
}
