package cli

// validate_batch_delegation_test.go validates that runValidateDir and
// runValidatePack route through ports.BatchValidator when cfg.Facade
// implements the interface, and that renderBatchResults produces the
// same human-readable format as the legacy direct-service path.
//
// Tests that exercise the legacy fallback path (bare Config without a
// wired facade) are covered by validate_pack_test.go; this file only
// covers the facade-delegation path introduced in F108.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
)

// -- renderBatchResults unit tests ------------------------------------------

// TestRenderBatchResults_AllValid verifies the "All N workflow(s) valid" summary.
func TestRenderBatchResults_AllValid(t *testing.T) {
	cmd, buf := newTestCmd(t)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	results := []ports.FileValidationResult{
		{Name: "wf-one", Valid: true},
		{Name: "wf-two", Valid: true},
	}

	err := renderBatchResults(cmd, formatter, results, "testdir")

	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "All 2 workflow(s) valid")
	assert.NotContains(t, out, "FAIL")
}

// TestRenderBatchResults_PartialFailure verifies FAIL markers and error count.
func TestRenderBatchResults_PartialFailure(t *testing.T) {
	cmd, buf := newTestCmd(t)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	results := []ports.FileValidationResult{
		{Name: "valid-wf", Valid: true},
		{
			Name:  "broken-wf",
			Valid: false,
			Errors: []ports.ValidationError{
				{Message: "missing initial state"},
			},
		},
	}

	err := renderBatchResults(cmd, formatter, results, "testdir")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 of 2")
	out := buf.String()
	assert.Contains(t, out, "FAIL")
	assert.Contains(t, out, "missing initial state")
	assert.Contains(t, out, "OK")
}

// TestRenderBatchResults_Empty verifies the "No .yaml files" message when empty.
func TestRenderBatchResults_Empty(t *testing.T) {
	cmd, buf := newTestCmd(t)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	err := renderBatchResults(cmd, formatter, nil, "emptydir")

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No .yaml files found")
}

// -- runValidateDir facade delegation tests ---------------------------------

// TestRunValidateDir_FacadeDelegation verifies that a wired BatchValidator facade
// is preferred over the direct service path when cfg.Facade implements the port.
func TestRunValidateDir_FacadeDelegation(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "my-wf.yaml"), []byte(minimalValidWorkflowYAML), 0o644))

	cmd, buf := newTestCmd(t)
	cfg := newTestConfig()
	cfg.StoragePath = t.TempDir()
	facade, cleanup := buildFacade(cfg)
	require.NotNil(t, facade, "facade must be wired for BatchValidator delegation")
	defer cleanup()
	cfg.Facade = facade

	err := runValidateDir(cmd, cfg, dir, true, 5*time.Second)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "All 1 workflow(s) valid")
}

// TestRunValidateDir_FacadeDelegation_InvalidWorkflow verifies that per-file
// failures are rendered via the facade-delegation path.
func TestRunValidateDir_FacadeDelegation_InvalidWorkflow(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "valid.yaml"), []byte(minimalValidWorkflowYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte(missingInitialWorkflowYAML), 0o644))

	cmd, buf := newTestCmd(t)
	cfg := newTestConfig()
	cfg.StoragePath = t.TempDir()
	facade, cleanup := buildFacade(cfg)
	require.NotNil(t, facade)
	defer cleanup()
	cfg.Facade = facade

	err := runValidateDir(cmd, cfg, dir, true, 5*time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 of 2")
	out := buf.String()
	assert.Contains(t, out, "FAIL")
	assert.Contains(t, out, "OK")
}

// TestRunValidateDir_FacadeDelegation_NonExistentDir verifies that a missing
// directory surfaces as an error through the facade path.
func TestRunValidateDir_FacadeDelegation_NonExistentDir(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")

	cmd, _ := newTestCmd(t)
	cfg := newTestConfig()
	cfg.StoragePath = t.TempDir()
	facade, cleanup := buildFacade(cfg)
	require.NotNil(t, facade)
	defer cleanup()
	cfg.Facade = facade

	err := runValidateDir(cmd, cfg, nonExistent, true, 5*time.Second)

	require.Error(t, err)
}

// -- runValidatePack facade delegation tests --------------------------------

// TestRunValidatePack_FacadeDelegation verifies that runValidatePack routes through
// ports.BatchValidator.ValidatePack when the facade implements the interface.
func TestRunValidatePack_FacadeDelegation(t *testing.T) {
	projectDir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWD) }) //nolint:errcheck // cleanup restores test environment
	require.NoError(t, os.Chdir(projectDir))

	packDir := filepath.Join(projectDir, ".awf", "workflow-packs", "myfacadepack")
	workflowDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "flow.yaml"), []byte(minimalValidWorkflowYAML), 0o644))
	createPackManifest(t, packDir, "myfacadepack", "1.0.0", []string{"flow"})
	createPackState(t, packDir, "myfacadepack", "1.0.0", true)

	cmd, buf := newTestCmd(t)
	cfg := newTestConfig()
	cfg.StoragePath = t.TempDir()
	facade, cleanup := buildFacade(cfg)
	require.NotNil(t, facade)
	defer cleanup()
	cfg.Facade = facade

	err = runValidatePack(cmd, cfg, "myfacadepack", true, 5*time.Second)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "All 1 workflow(s) valid")
}

// TestRunValidatePack_FacadeDelegation_NotFound verifies that an unknown pack
// returns an error containing "not found" through the facade path.
func TestRunValidatePack_FacadeDelegation_NotFound(t *testing.T) {
	projectDir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origWD) }) //nolint:errcheck // cleanup restores test environment
	require.NoError(t, os.Chdir(projectDir))

	cmd, _ := newTestCmd(t)
	cfg := newTestConfig()
	cfg.StoragePath = t.TempDir()
	facade, cleanup := buildFacade(cfg)
	require.NotNil(t, facade)
	defer cleanup()
	cfg.Facade = facade

	err = runValidatePack(cmd, cfg, "ghost-facade-pack", true, 5*time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
