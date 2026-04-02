package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/awf-project/cli/internal/testutil/fixtures"
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

// TestList_PackWorkflowsVisibleWithoutLocalWorkflows verifies that pack workflows
// appear in awf list output even when there are no local/global workflows (early return fix).
func TestList_PackWorkflowsVisibleWithoutLocalWorkflows(t *testing.T) {
	tmpHome := t.TempDir()
	projDir := filepath.Join(tmpHome, "project")

	// Create empty .awf/workflows/ (no local workflows)
	require.NoError(t, os.MkdirAll(filepath.Join(projDir, ".awf", "workflows"), 0o755))

	// Create a pack with a workflow that has a description
	packDir := filepath.Join(projDir, ".awf", "workflow-packs", "testpack")
	workflowsDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(`name: testpack
version: "1.0.0"
description: A test pack
author: test
license: MIT
awf_version: ">=0.5.0"
workflows:
  - mywf
`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(`{
  "name": "testpack",
  "enabled": true,
  "source_data": {"repository": "org/testpack", "version": "1.0.0"}
}`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "mywf.yaml"), []byte(`name: mywf
description: "My test workflow"
states:
  initial: done
  done:
    type: terminal
    status: success
`), 0o644))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projDir))
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore in cleanup

	// Isolate XDG to avoid global workflows leaking in
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpHome, "data"))
	t.Setenv("HOME", tmpHome)

	var stdout bytes.Buffer
	cmd := cli.NewRootCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list"})
	err = cmd.Execute()

	require.NoError(t, err)
	output := stdout.String()

	// Pack workflow must appear even though there are 0 local workflows
	require.Contains(t, output, "testpack/mywf", "pack workflow should appear when no local workflows exist")
	require.Contains(t, output, "My test workflow", "pack workflow description should be displayed")
	require.Contains(t, output, "pack", "source should be 'pack'")
	require.Contains(t, output, "1.0.0", "version should be displayed")
}

// TestList_PackWorkflowsMergedWithLocalWorkflows verifies that both pack and
// local workflows appear together in the output of awf list.
func TestList_PackWorkflowsMergedWithLocalWorkflows(t *testing.T) {
	tmpHome := t.TempDir()
	projDir := filepath.Join(tmpHome, "project")

	// Create a local workflow
	localDir := filepath.Join(projDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(localDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "local-wf.yaml"), []byte(`name: local-wf
description: "A local workflow"
states:
  initial: done
  done:
    type: terminal
    status: success
`), 0o644))

	// Create a pack workflow
	packDir := filepath.Join(projDir, ".awf", "workflow-packs", "mypack")
	workflowsDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(`name: mypack
version: "2.0.0"
description: My pack
author: test
license: MIT
awf_version: ">=0.5.0"
workflows:
  - pack-wf
`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(`{
  "name": "mypack",
  "enabled": true,
  "source_data": {"repository": "org/mypack", "version": "2.0.0"}
}`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "pack-wf.yaml"), []byte(`name: pack-wf
description: "A pack workflow"
states:
  initial: done
  done:
    type: terminal
    status: success
`), 0o644))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projDir))
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck // restore in cleanup

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpHome, "data"))
	t.Setenv("HOME", tmpHome)

	var stdout bytes.Buffer
	cmd := cli.NewRootCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list"})
	err = cmd.Execute()

	require.NoError(t, err)
	output := stdout.String()

	// Both local and pack workflows must appear
	require.Contains(t, output, "local-wf", "local workflow should appear")
	require.Contains(t, output, "A local workflow", "local workflow description should appear")
	require.Contains(t, output, "mypack/pack-wf", "pack workflow should appear")
	require.Contains(t, output, "A pack workflow", "pack workflow description should appear")
}
