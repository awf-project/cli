package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginWorkflow stops on a step that exits with 75, then calls the echo plugin operation. It resumes from `check`.
const pluginWorkflow = `name: plugin-pause
version: "1.0.0"
inputs:
  - name: answer
    type: string
    default: "none"
states:
  initial: check
  check:
    type: step
    command: "test '{{.inputs.answer}}' != 'none'"
    on_success: say
    on_failure: wait
  wait:
    type: step
    command: exit 75
    on_success: say
    on_failure: stopped
  say:
    type: operation
    operation: echo.echo
    inputs:
      text: "{{.inputs.answer}}"
    on_success: done
    on_failure: stopped
  done:
    type: terminal
    status: success
  stopped:
    type: terminal
    status: failure
`

// pluginEnv builds the echo example plugin into a plugins directory, writes the workflow, and points AWF at both with
// isolated storage. It skips when the plugin cannot be built.
func pluginEnv(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("builds a plugin binary")
	}
	root, err := filepath.Abs("../../..")
	require.NoError(t, err)

	pluginsDir := t.TempDir()
	pluginDir := filepath.Join(pluginsDir, "awf-plugin-echo")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	build := exec.CommandContext(t.Context(), "go", "build", "-o", filepath.Join(pluginDir, "awf-plugin-echo"), "./examples/plugins/awf-plugin-echo")
	build.Dir = root
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Skipf("cannot build the echo plugin: %v\n%s", buildErr, out)
	}
	manifest, err := os.ReadFile(filepath.Join(root, "examples", "plugins", "awf-plugin-echo", "plugin.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), manifest, 0o644)) //nolint:gosec // path built from t.TempDir()

	workflows := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workflows, "plugin-pause.yaml"), []byte(pluginWorkflow), 0o644))

	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)
	t.Setenv("AWF_WORKFLOWS_PATH", workflows)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Chdir(t.TempDir()) // no project configuration
}

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd, cleanup := cli.NewRootCommandAutoFacade()
	defer cleanup()
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestValidate_WorkflowCallingPluginOperation(t *testing.T) {
	pluginEnv(t)

	out, err := execute(t, "validate", "plugin-pause")

	require.NoError(t, err, "a workflow that calls a plugin operation validates: %s", out)
}

func TestValidate_SkipPluginsKnowsBuiltinsOnly(t *testing.T) {
	pluginEnv(t)

	out, err := execute(t, "validate", "plugin-pause", "--skip-plugins")

	require.Error(t, err)
	assert.Contains(t, out+err.Error(), `operation "echo.echo" not found`)
}

func TestValidate_UnknownOperationStaysRefusedWithPlugins(t *testing.T) {
	pluginEnv(t)
	workflows := os.Getenv("AWF_WORKFLOWS_PATH")
	broken := strings.NewReplacer("echo.echo", "echo.missing", "plugin-pause", "plugin-missing-op").Replace(pluginWorkflow)
	require.NoError(t, os.WriteFile(filepath.Join(workflows, "plugin-missing-op.yaml"), []byte(broken), 0o644)) //nolint:gosec // path built from t.TempDir()

	out, err := execute(t, "validate", "plugin-missing-op")

	require.Error(t, err)
	assert.Contains(t, out+err.Error(), `operation "echo.missing" not found`)
}

func TestResume_WorkflowCallingPluginOperation(t *testing.T) {
	pluginEnv(t)

	_, runErr := execute(t, "run", "plugin-pause")
	require.Error(t, runErr, "the first run stops on the failure terminal state")
	states, err := filepath.Glob(filepath.Join(os.Getenv("XDG_DATA_HOME"), "awf", "states", "*.json"))
	require.NoError(t, err)
	require.Len(t, states, 1)
	id := strings.TrimSuffix(filepath.Base(states[0]), ".json")

	out, err := execute(t, "resume", id, "--from", "check", "--input", "answer=A")

	require.NoError(t, err, "resume must know the plugin operations that run knows: %s", out)
	state, err := os.ReadFile(states[0]) //nolint:gosec // path found under the isolated XDG_DATA_HOME of this test
	require.NoError(t, err)
	assert.Contains(t, string(state), `"Status": "completed"`)
}
