//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/require"
)

type rootCommandTestEnvironment struct {
	workflowDir string
	storageDir  string
	xdgDataDir  string
	options     []cli.RootOption
}

func newRootCommandTestEnvironment(t *testing.T) rootCommandTestEnvironment {
	t.Helper()

	root := t.TempDir()
	env := rootCommandTestEnvironment{
		workflowDir: filepath.Join(root, "workflows"),
		storageDir:  filepath.Join(root, "storage"),
		xdgDataDir:  filepath.Join(root, "xdg-data"),
	}
	require.NoError(t, os.MkdirAll(env.workflowDir, 0o700))
	t.Setenv("AWF_WORKFLOWS_PATH", env.workflowDir)
	t.Setenv("XDG_DATA_HOME", env.xdgDataDir)
	t.Setenv("AWF_CONFIG_PATH", "")
	t.Setenv("AWF_AUDIT_LOG", "off")
	return env
}

func (e rootCommandTestEnvironment) writeWorkflow(t *testing.T, name, source string) string {
	t.Helper()
	path := filepath.Join(e.workflowDir, name+".yaml")
	require.NoError(t, os.WriteFile(path, []byte(source), 0o600))
	return path
}

func (e rootCommandTestEnvironment) execute(
	t *testing.T,
	ctx context.Context,
	workflowName string,
) (string, error) {
	return e.executeWithRunFlags(t, ctx, workflowName)
}

// executeWithRunFlags runs a workflow through the public CLI entry point with
// optional run-command flags. It keeps lifecycle tests on the same real wiring
// path while allowing individual scenarios to configure optional adapters.
func (e rootCommandTestEnvironment) executeWithRunFlags(
	t *testing.T,
	ctx context.Context,
	workflowName string,
	runFlags ...string,
) (string, error) {
	t.Helper()
	args := []string{"run", workflowName, "--skip-plugins"}
	args = append(args, runFlags...)
	return e.executeArgs(t, ctx, args...)
}

// executeArgs constructs a fresh public root command for each run or resume.
func (e rootCommandTestEnvironment) executeArgs(t *testing.T, ctx context.Context, args ...string) (string, error) {
	t.Helper()
	cmd := cli.NewRootCommand(e.options...)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	rootArgs := []string{"--storage", e.storageDir, "--no-color"}
	rootArgs = append(rootArgs, args...)
	cmd.SetArgs(rootArgs)
	err := cmd.ExecuteContext(ctx)
	return output.String(), err
}

func (e rootCommandTestEnvironment) loadExecutionByWorkflowName(
	t *testing.T,
	name string,
) *workflow.ExecutionContext {
	t.Helper()
	stateStore := store.NewJSONStore(filepath.Join(e.storageDir, "states"))
	ids, err := stateStore.List(context.Background())
	require.NoError(t, err)
	for _, id := range ids {
		execution, loadErr := stateStore.Load(context.Background(), id)
		require.NoError(t, loadErr)
		if execution.WorkflowName == name {
			return execution
		}
	}
	t.Fatalf("persisted execution for workflow %q not found", name)
	return nil
}

// setupTestDir creates a temporary directory for test isolation.
// Use this when you need a clean directory for XDG_CONFIG_HOME or similar.
// Does NOT change the current working directory.
func setupTestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// setupInitTestDir creates a temporary directory and changes the current
// working directory to it. The original working directory is restored when
// the test completes.
func setupInitTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("failed to restore original directory: %v", err)
		}
	})

	return tmpDir
}

// createTestWorkflow creates a workflow file in the .awf/workflows directory
// within the given base directory. Creates the directory structure if needed.
func createTestWorkflow(t *testing.T, baseDir, filename, content string) {
	t.Helper()

	workflowsDir := filepath.Join(baseDir, ".awf", "workflows")
	if err := os.MkdirAll(workflowsDir, 0o755); err != nil {
		t.Fatalf("failed to create workflows directory: %v", err)
	}

	workflowPath := filepath.Join(workflowsDir, filename)
	if err := os.WriteFile(workflowPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}
}

// runCLI creates a root command, sets the given args, executes it, and returns
// the combined stdout+stderr output along with any error.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	err := cmd.Execute()

	return out.String(), err
}

// runCLIFacade creates a root command wired with AutoFacade, prepends
// --storage <tmpDir> so buildFacade can create history.db in an isolated
// temp directory, and executes the command. It returns combined
// stdout+stderr output and any error. Use this for commands that are routed
// through the WorkflowFacade (validate, list, history, resume --list, etc.).
func runCLIFacade(t *testing.T, args ...string) (string, error) {
	t.Helper()

	tmpDir := t.TempDir()
	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--storage=" + tmpDir}, args...))

	err := cmd.Execute()

	return out.String(), err
}

// findRepoRoot walks up the directory tree to find the repository root
// by looking for go.mod file.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
