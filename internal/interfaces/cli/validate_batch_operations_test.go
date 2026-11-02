package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// operationWorkflow returns a workflow whose only step calls the given operation.
func operationWorkflow(name, operation string) []byte {
	return []byte(fmt.Sprintf(`name: %s
states:
  initial: call
  call:
    type: operation
    operation: %s
    inputs:
      url: "https://example.invalid"
      method: GET
    on_success: done
    on_failure: done
  done:
    type: terminal
    status: success
`, name, operation))
}

func batchValidationConfig(t *testing.T) (*Config, func()) {
	t.Helper()
	cfg := newTestConfig()
	cfg.StoragePath = t.TempDir()
	facade, cleanup := buildFacade(cfg)
	require.NotNil(t, facade)
	cfg.Facade = facade
	return cfg, cleanup
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWD) }) //nolint:errcheck // cleanup restores test environment
	require.NoError(t, os.Chdir(dir))
}

// TestRunValidateDir_UnknownOperation verifies that the directory form checks that the operation of a step exists, as
// validating the same workflow by name does.
func TestRunValidateDir_UnknownOperation(t *testing.T) {
	chdir(t, t.TempDir())
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "good.yaml"), operationWorkflow("good", "http.request"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), operationWorkflow("bad", "nope.missing"), 0o600))
	cmd, buf := newTestCmd(t)
	cfg, cleanup := batchValidationConfig(t)
	defer cleanup()

	err := runValidateDir(cmd, cfg, dir, true, 5*time.Second)

	require.Error(t, err, "a workflow that calls an unknown operation must fail the batch")
	assert.Contains(t, buf.String(), "OK    good")
	assert.Contains(t, buf.String(), "FAIL  bad")
	assert.Contains(t, buf.String(), `operation "nope.missing" not found`)
}

// TestRunValidatePack_UnknownOperation is the same check for the pack form, which already goes through the project
// workflow service.
func TestRunValidatePack_UnknownOperation(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	packDir := filepath.Join(projectDir, ".awf", "workflow-packs", "oppack")
	workflowDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "bad.yaml"), operationWorkflow("bad", "nope.missing"), 0o600))
	createPackManifest(t, packDir, "oppack", "1.0.0", []string{"bad"})
	createPackState(t, packDir, "oppack", "1.0.0", true)
	cmd, buf := newTestCmd(t)
	cfg, cleanup := batchValidationConfig(t)
	defer cleanup()

	err := runValidatePack(cmd, cfg, "oppack", true, 5*time.Second)

	require.Error(t, err)
	assert.Contains(t, buf.String(), `operation "nope.missing" not found`)
}
