//go:build integration

// Feature: F111
package cli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginInitIntegration_RunsGeneratedMakeTestAndMakeBuildWithoutManualEdits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	awfBin := buildPluginInitIntegrationAWF(t, ctx)
	env := pluginInitIntegrationEnv(t)
	workspace := t.TempDir()
	outputDir := filepath.Join(workspace, "awf-plugin-example")

	runPluginInitIntegrationCommand(
		t, ctx, workspace, env, awfBin,
		"plugin", "init", "awf-plugin-example",
		"--kind", "operation",
		"--output", outputDir,
	)
	runPluginInitIntegrationCommand(t, ctx, outputDir, env, "make", "test")
	runPluginInitIntegrationCommand(t, ctx, outputDir, env, "make", "build")
}

func TestPluginInitIntegration_WithIsolatedAWFPathsInstallsEnablesByDistributionNameListsOperationsAndRunsDemo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	awfBin := buildPluginInitIntegrationAWF(t, ctx)
	env := pluginInitIntegrationEnv(t)
	workspace := t.TempDir()
	outputDir := filepath.Join(workspace, "awf-plugin-example")
	storageDir := filepath.Join(workspace, "storage")

	runPluginInitIntegrationCommand(
		t, ctx, workspace, env, awfBin,
		"plugin", "init", "awf-plugin-example",
		"--kind", "operation",
		"--output", outputDir,
		"--storage", storageDir,
	)
	runPluginInitIntegrationCommand(t, ctx, outputDir, env, "make", "install-local")

	enableOutput := runPluginInitIntegrationCommand(
		t, ctx, outputDir, env, awfBin,
		"plugin", "enable", "awf-plugin-example",
		"--storage", storageDir,
	)
	assert.Contains(t, enableOutput, "example")

	operationsOutput := runPluginInitIntegrationCommand(
		t, ctx, outputDir, env, awfBin,
		"plugin", "list", "--operations",
		"--storage", storageDir,
	)
	assert.Contains(t, operationsOutput, "example.echo")

	runOutput := runPluginInitIntegrationCommand(
		t, ctx, outputDir, env, awfBin,
		"run", "examples/demo.yaml",
		"--storage", storageDir,
	)
	assert.Contains(t, runOutput, "Workflow completed.")
}

func TestPluginInitIntegration_UnsupportedKindFailsWithoutCreatingOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	awfBin := buildPluginInitIntegrationAWF(t, ctx)
	env := pluginInitIntegrationEnv(t)
	workspace := t.TempDir()
	outputDir := filepath.Join(workspace, "awf-plugin-validator")

	output := runPluginInitIntegrationCommandExpectError(
		t, ctx, workspace, env, awfBin,
		"plugin", "init", "awf-plugin-validator",
		"--kind", "validator",
		"--output", outputDir,
	)

	assert.Contains(t, output, `unsupported plugin init kind "validator"; supported kind is "operation"`)
	assert.NoDirExists(t, outputDir)
}

func buildPluginInitIntegrationAWF(t *testing.T, ctx context.Context) string {
	t.Helper()

	repoRoot := pluginInitIntegrationRepoRoot(t)
	binPath := filepath.Join(t.TempDir(), "awf")
	runPluginInitIntegrationCommand(t, ctx, repoRoot, os.Environ(), "go", "build", "-o", binPath, "./cmd/awf")
	return binPath
}

func pluginInitIntegrationEnv(t *testing.T) []string {
	t.Helper()

	root := t.TempDir()
	home := filepath.Join(root, "home")
	pluginsDir := filepath.Join(home, ".local", "share", "awf", "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	return append(
		os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_DATA_HOME="+filepath.Join(root, "data"),
		"XDG_STATE_HOME="+filepath.Join(root, "state"),
		"AWF_PLUGINS_PATH="+pluginsDir,
	)
}

func pluginInitIntegrationRepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

func runPluginInitIntegrationCommand(
	t *testing.T,
	ctx context.Context,
	dir string,
	env []string,
	name string,
	args ...string,
) string {
	t.Helper()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s %v failed in %s:\n%s", name, args, dir, output)
	return string(output)
}

func runPluginInitIntegrationCommandExpectError(
	t *testing.T,
	ctx context.Context,
	dir string,
	env []string,
	name string,
	args ...string,
) string {
	t.Helper()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	require.Error(t, err, "%s %v unexpectedly succeeded in %s:\n%s", name, args, dir, output)
	return string(output)
}
