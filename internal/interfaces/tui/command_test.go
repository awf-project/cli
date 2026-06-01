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

// TestResolvePackWorkflow_TUI_RejectsInvalidPackName verifies the TUI
// resolvePackWorkflow function validates packName via the shared ValidateName
// rule before any filepath.Join. The error must contain "invalid name",
// not "not found" — confirming the guard fires before filesystem access.
//
// This is the S2 security fix: eliminating the divergent validation path in TUI.
func TestResolvePackWorkflow_TUI_RejectsInvalidPackName(t *testing.T) {
	ctx := t.Context()

	invalidPackNames := []struct {
		name  string
		input string
	}{
		{"path traversal dot-dot", "../../etc"},
		{"absolute path", "/etc/passwd"},
		{"slash separator", "pack/sub"},
		{"uppercase letter", "MyPack"},
		{"starts with digit", "1pack"},
		{"dot-dot alone", ".."},
		{"empty string", ""},
	}
	for _, tt := range invalidPackNames {
		t.Run(tt.name, func(t *testing.T) {
			wf, packDir, err := resolvePackWorkflow(ctx, tt.input, "someworkflow")
			require.Error(t, err, "packName %q must be rejected", tt.input)
			assert.Nil(t, wf)
			assert.Empty(t, packDir)
			assert.Contains(t, err.Error(), "invalid name",
				"expected validation error for packName %q, got: %v", tt.input, err)
		})
	}
}

// TestResolvePackWorkflow_TUI_RejectsInvalidWorkflowName verifies the TUI
// resolvePackWorkflow function validates workflowName before filesystem access.
func TestResolvePackWorkflow_TUI_RejectsInvalidWorkflowName(t *testing.T) {
	ctx := t.Context()

	invalidWorkflowNames := []struct {
		name  string
		input string
	}{
		{"path traversal dot-dot", "../../passwd"},
		{"slash separator", "sub/workflow"},
		{"uppercase letter", "MyWorkflow"},
		{"starts with digit", "1workflow"},
		{"empty string", ""},
	}
	for _, tt := range invalidWorkflowNames {
		t.Run(tt.name, func(t *testing.T) {
			wf, packDir, err := resolvePackWorkflow(ctx, "validpack", tt.input)
			require.Error(t, err, "workflowName %q must be rejected", tt.input)
			assert.Nil(t, wf)
			assert.Empty(t, packDir)
			assert.Contains(t, err.Error(), "invalid name",
				"expected validation error for workflowName %q, got: %v", tt.input, err)
		})
	}
}
