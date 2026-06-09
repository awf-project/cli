package tui

import (
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
	bridge, _, cleanup, err := buildBridge()
	defer cleanup()

	require.NoError(t, err)
	require.NotNil(t, bridge)
	assert.NotNil(t, bridge.workflows, "bridge must have a workflow lister")
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
