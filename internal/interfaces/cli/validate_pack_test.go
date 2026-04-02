package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalValidWorkflowYAML is a minimal valid workflow YAML for test fixtures.
const minimalValidWorkflowYAML = `name: test-wf
states:
  initial: start
  start:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
    status: success
`

// missingInitialWorkflowYAML is missing the required states.initial field.
const missingInitialWorkflowYAML = `name: broken-wf
states:
  start:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
    status: success
`

// newTestCmd creates a cobra.Command with output captured into the returned buffer.
func newTestCmd(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	return cmd, buf
}

// newTestConfig returns a minimal Config suitable for unit tests.
func newTestConfig() *Config {
	return &Config{
		NoColor: true,
	}
}

// TestRunValidateDir_ValidWorkflows verifies that a directory with two valid
// .yaml workflow files is validated without error and reports "OK" for each.
func TestRunValidateDir_ValidWorkflows(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "wf-one.yaml"), []byte(minimalValidWorkflowYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "wf-two.yaml"), []byte(minimalValidWorkflowYAML), 0o644))

	cmd, buf := newTestCmd(t)
	cfg := newTestConfig()

	err := runValidateDir(cmd, cfg, dir, true, 5*time.Second)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "OK")
	assert.Contains(t, output, "All 2 workflow(s) valid")
}

// TestRunValidateDir_InvalidWorkflow verifies that when one workflow is invalid
// the error count is reported and "FAIL" appears in output for the bad file.
func TestRunValidateDir_InvalidWorkflow(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "valid.yaml"), []byte(minimalValidWorkflowYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte(missingInitialWorkflowYAML), 0o644))

	cmd, buf := newTestCmd(t)
	cfg := newTestConfig()

	err := runValidateDir(cmd, cfg, dir, true, 5*time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 of 2")

	output := buf.String()
	assert.True(t, strings.Contains(output, "FAIL") || strings.Contains(output, "fail"),
		"expected FAIL marker in output, got: %s", output)
	assert.True(t, strings.Contains(output, "OK") || strings.Contains(output, "ok"),
		"expected OK marker for valid workflow, got: %s", output)
}

// TestRunValidateDir_EmptyDir verifies that an empty directory produces a
// human-readable message instead of an error.
func TestRunValidateDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	cmd, buf := newTestCmd(t)
	cfg := newTestConfig()

	err := runValidateDir(cmd, cfg, dir, true, 5*time.Second)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No .yaml files found")
}

// TestRunValidateDir_NonExistentDir verifies that a path that does not exist
// returns an error rather than panicking or silently succeeding.
func TestRunValidateDir_NonExistentDir(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")

	cmd, buf := newTestCmd(t)
	_ = buf
	cfg := newTestConfig()

	err := runValidateDir(cmd, cfg, nonExistent, true, 5*time.Second)

	require.Error(t, err)
}

// TestRunValidatePack_ResolvesPackDir verifies that runValidatePack finds an
// installed pack relative to the current working directory and validates its
// workflows successfully.
func TestRunValidatePack_ResolvesPackDir(t *testing.T) {
	// findPackDir searches ".awf/workflow-packs/<name>" relative to cwd.
	// We set cwd to our temp project root so the lookup resolves correctly.
	projectDir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWD) }) //nolint:errcheck // cleanup restores test environment
	require.NoError(t, os.Chdir(projectDir))

	// Build the pack structure under projectDir/.awf/workflow-packs/testpack/
	packDir := filepath.Join(projectDir, ".awf", "workflow-packs", "testpack")
	workflowDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "alpha.yaml"), []byte(minimalValidWorkflowYAML), 0o644))

	createPackManifest(t, packDir, "testpack", "1.0.0", []string{"alpha"})
	createPackState(t, packDir, "testpack", "1.0.0", true)

	cmd, buf := newTestCmd(t)
	cfg := newTestConfig()

	err = runValidatePack(cmd, cfg, "testpack", true, 5*time.Second)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "All 1 workflow(s) valid")
}

// TestRunValidatePack_NotFound verifies that a non-existent pack name returns
// an error that mentions "not found".
func TestRunValidatePack_NotFound(t *testing.T) {
	// Use a fresh temp directory as cwd so no packs are present.
	projectDir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWD) }) //nolint:errcheck // cleanup restores test environment
	require.NoError(t, os.Chdir(projectDir))

	cmd, buf := newTestCmd(t)
	_ = buf
	cfg := newTestConfig()

	err = runValidatePack(cmd, cfg, "ghost-pack", true, 5*time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestRunValidate_PackNamespaceSyntax verifies that a "packname/workflow" argument
// resolves the pack's workflows directory and validates the specific workflow.
func TestRunValidate_PackNamespaceSyntax(t *testing.T) {
	projectDir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWD) }) //nolint:errcheck // cleanup restores test environment
	require.NoError(t, os.Chdir(projectDir))

	packDir := filepath.Join(projectDir, ".awf", "workflow-packs", "mypack")
	workflowDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowDir, "myworkflow.yaml"),
		[]byte(minimalValidWorkflowYAML),
		0o644,
	))

	createPackManifest(t, packDir, "mypack", "1.0.0", []string{"myworkflow"})
	createPackState(t, packDir, "mypack", "1.0.0", true)

	cmd, buf := newTestCmd(t)
	cfg := newTestConfig()

	err = runValidate(cmd, cfg, "mypack/myworkflow", true, 5*time.Second)

	require.NoError(t, err)
	// Success message includes the workflow name (without the pack prefix).
	assert.Contains(t, buf.String(), "myworkflow")
}

// TestRunValidate_PackNamespaceSyntax_PackNotFound verifies that "missingpack/workflow"
// returns an error when the pack is not installed.
func TestRunValidate_PackNamespaceSyntax_PackNotFound(t *testing.T) {
	projectDir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWD) }) //nolint:errcheck // cleanup restores test environment
	require.NoError(t, os.Chdir(projectDir))

	cmd, buf := newTestCmd(t)
	_ = buf
	cfg := newTestConfig()

	err = runValidate(cmd, cfg, "missingpack/someworkflow", true, 5*time.Second)

	require.Error(t, err)
}

// TestRunValidate_StandardWorkflow_StillWorks is a regression test ensuring that
// a plain workflow name (no slash) still resolves through the standard repository
// path rather than the pack-namespace path.
func TestRunValidate_StandardWorkflow_StillWorks(t *testing.T) {
	// Point AWF_WORKFLOWS_PATH to a temp dir containing one valid workflow.
	workflowsDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowsDir, "plain-wf.yaml"),
		[]byte(minimalValidWorkflowYAML),
		0o644,
	))

	t.Setenv("AWF_WORKFLOWS_PATH", workflowsDir)

	cmd, buf := newTestCmd(t)
	cfg := newTestConfig()

	err := runValidate(cmd, cfg, "plain-wf", true, 5*time.Second)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "plain-wf")
}
