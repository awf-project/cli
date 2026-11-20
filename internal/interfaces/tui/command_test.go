package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCommand_Metadata(t *testing.T) {
	cmd := NewCommand()
	require.NotNil(t, cmd)

	assert.Equal(t, "tui", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotNil(t, cmd.RunE)
}

func TestValidateTerminal_FailsWithoutTerminal(t *testing.T) {
	t.Setenv("TERM", "")

	err := validateTerminal()

	require.Error(t, err)
	assert.ErrorIs(t, err, errNoTerminal)
}

func TestValidateTerminal_FailsWithDumbTerminal(t *testing.T) {
	t.Setenv("TERM", "dumb")

	err := validateTerminal()

	require.Error(t, err)
	assert.ErrorIs(t, err, errDumbTerminal)
}

func TestValidateTerminal_SucceedsWithCapableTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")

	err := validateTerminal()

	assert.NoError(t, err)
}

func TestRunTUI_FailsWithoutTerminal(t *testing.T) {
	t.Setenv("TERM", "")

	err := runTUI(NewCommand())

	require.Error(t, err)
	assert.ErrorIs(t, err, errNoTerminal)
}

func TestRunTUI_FailsWithDumbTerminal(t *testing.T) {
	t.Setenv("TERM", "dumb")

	err := runTUI(NewCommand())

	require.Error(t, err)
	assert.ErrorIs(t, err, errDumbTerminal)
}

func TestBuildBridge_ReturnsNonNilBridge(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	bridge, _, cleanup, err := buildBridge(context.Background())
	defer cleanup()

	require.NoError(t, err)
	require.NotNil(t, bridge)
	assert.NotNil(t, bridge.workflows, "bridge must have a workflow lister")
}

func TestBuildBridge_InstallsNotifierWithoutExternalPlugins(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("AWF_WORKFLOWS_PATH", filepath.Join(root, "workflows"))
	t.Setenv("AWF_AUDIT_LOG", "off")

	require.NoError(t, os.MkdirAll(filepath.Join(root, ".awf"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "workflows"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(
		"notifier:\n  backend: recording\n  default_channel: alerts\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "workflows", "notify.yaml"), []byte(`name: notify
version: "1.0"
states:
  initial: send
  send:
    type: operation
    operation: notifier.send
    inputs:
      text: hello
    on_success: done
  done:
    type: terminal
    status: success
`), 0o600))

	bridge, _, cleanup, err := buildBridge(context.Background())
	defer cleanup()

	require.NoError(t, err)
	require.NotNil(t, bridge)
	assert.NoError(t, bridge.workflows.ValidateWorkflow(context.Background(), "notify"))
}

func TestBuildWorkflowPaths_IncludesLocalAndGlobal(t *testing.T) {
	paths := buildWorkflowPaths()

	assert.GreaterOrEqual(t, len(paths), 2, "must include at least local and global paths")
}

func TestNopLogger_SatisfiesInterface(t *testing.T) {
	l := &nopLogger{}
	l.Debug("test")
	l.Info("test")
	l.Warn("test")
	l.Error("test")
	ctx := l.WithContext(map[string]any{"key": "val"})
	assert.NotNil(t, ctx)
}
