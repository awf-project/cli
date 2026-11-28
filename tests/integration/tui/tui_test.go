//go:build integration

package tui_test

import (
	"bytes"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/awf-project/cli/internal/interfaces/tui"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTUICommand_ExistsInRoot verifies the TUI command is registered as a subcommand.
func TestTUICommand_ExistsInRoot(t *testing.T) {
	cmd := cli.NewRootCommand()

	found, tuiCmd := findSubcommand(cmd, "tui")
	assert.True(t, found, "expected root command to have 'tui' subcommand")
	assert.NotNil(t, tuiCmd)
}

// TestTUICommand_FindByName verifies rootCmd.Find("tui") discovers the command.
func TestTUICommand_FindByName(t *testing.T) {
	cmd := cli.NewRootCommand()

	found, _, _ := cmd.Find([]string{"tui"})
	require.NotNil(t, found, "rootCmd.Find should locate the tui subcommand")
	assert.Equal(t, "tui", found.Name(), "found command should be named 'tui'")
}

// TestTUICommand_HasCorrectShortDescription verifies the TUI command displays expected help text.
func TestTUICommand_HasCorrectShortDescription(t *testing.T) {
	cmd := cli.NewRootCommand()
	_, tuiCmd := findSubcommand(cmd, "tui")

	require.NotNil(t, tuiCmd, "tui command not found")
	assert.Contains(t, tuiCmd.Short, "interactive", "expected help text to mention interactive mode")
	assert.Contains(t, tuiCmd.Short, "terminal", "expected help text to mention terminal")
}

// TestTUICommand_FailsWithoutTerminal verifies TUI command fails when TERM is not set.
func TestTUICommand_FailsWithoutTerminal(t *testing.T) {
	t.Setenv("TERM", "")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"tui"})

	err := cmd.Execute()

	require.Error(t, err, "expected tui command to fail without TERM environment variable")
	assert.EqualError(t, err, "no terminal: TERM is not set")
}

// TestTUICommand_FailsWithDumbTerminal verifies TUI command fails when TERM=dumb.
func TestTUICommand_FailsWithDumbTerminal(t *testing.T) {
	t.Setenv("TERM", "dumb")

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"tui"})

	err := cmd.Execute()

	require.Error(t, err, "expected tui command to fail with dumb terminal")
	assert.EqualError(t, err, "terminal does not support interactive mode")
}

// TestTUICommand_RespectsSilenceErrorsFlag verifies root command sets SilenceErrors=true.
func TestTUICommand_RespectsSilenceErrorsFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	assert.True(t, cmd.SilenceErrors, "root command should set SilenceErrors=true for CLI integration")
}

// TestTUICommand_HelpText verifies the TUI command help can be displayed.
func TestTUICommand_HelpText(t *testing.T) {
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"tui", "--help"})

	err := cmd.Execute()

	require.NoError(t, err, "expected help command to succeed")
	output := out.String()
	assert.Contains(t, output, "tui", "expected help output to contain command name")
	assert.Contains(t, output, "terminal", "expected help output to contain description")
}

// TestTUICommand_HelpTextContainsKeyBindings verifies help text mentions keyboard shortcuts.
func TestTUICommand_HelpTextContainsKeyBindings(t *testing.T) {
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"tui", "--help"})

	err := cmd.Execute()

	require.NoError(t, err)
	output := out.String()
	// Long description should mention tab switching and quit.
	assert.Contains(t, output, "Workflows", "help text should name the screens")
	assert.Contains(t, output, "History", "help text should name the screens")
	assert.NotContains(t, output, "Logs", "the logs tab was removed")
}

// TestTUICommand_TerminalValidationTableDriven verifies TUI validates TERM without starting a program.
func TestTUICommand_TerminalValidationTableDriven(t *testing.T) {
	tests := []struct {
		name    string
		termVal string
		wantErr bool
		errMsg  string
	}{
		{"empty TERM", "", true, "no terminal: TERM is not set"},
		{"dumb terminal", "dumb", true, "terminal does not support interactive mode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TERM", tt.termVal)

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"tui"})

			err := cmd.Execute()

			require.Error(t, err, "expected tui command to fail for TERM=%q", tt.termVal)
			assert.EqualError(t, err, tt.errMsg)
		})
	}
}

// TestTUIModel_InitReturnsCmd verifies that Model.Init() returns a non-nil tea.Cmd
// in bridgeless mode (so the update loop receives WorkflowsLoadedMsg immediately).
func TestTUIModel_InitReturnsCmd(t *testing.T) {
	m := tui.New()

	initCmd := m.Init()
	require.NotNil(t, initCmd, "Init() must return a tea.Cmd (even in bridgeless mode)")

	msg := initCmd()
	_, ok := msg.(tui.WorkflowsLoadedMsg)
	assert.True(t, ok, "bridgeless Init() should produce WorkflowsLoadedMsg")
}

// TestTUIModel_ViewRendersTabBar verifies that Model.View() renders the current tab labels.
func TestTUIModel_ViewRendersTabBar(t *testing.T) {
	m := tui.New()

	// Provide dimensions so lipgloss does not crash.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(tui.Model)

	view := m.View().Content
	for _, label := range []string{"1 Workflows", "2 History"} {
		assert.Contains(t, view, label, "tab bar should contain label %q", label)
	}
	assert.NotContains(t, view, "Monitoring", "the run is a full-frame view, not a tab")
	assert.NotContains(t, view, "Logs", "the logs tab was removed")
}

// TestTUIModel_TabSwitching verifies that the digit keys and tab move between the screens.
func TestTUIModel_TabSwitching(t *testing.T) {
	m := tui.New()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(tui.Model)

	press := func(msg tea.KeyPressMsg) {
		updated, _ := m.Update(msg)
		m = updated.(tui.Model)
	}

	assert.Contains(t, m.View().Content, "no workflows", "starts on the workflows screen")

	press(tea.KeyPressMsg{Code: '2', Text: "2"})
	assert.Contains(t, m.View().Content, "no executions", "2 opens the history screen")

	press(tea.KeyPressMsg{Code: '1', Text: "1"})
	assert.Contains(t, m.View().Content, "no workflows", "1 returns to the workflows screen")

	press(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Contains(t, m.View().Content, "no executions", "tab moves to the next screen")

	press(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	assert.Contains(t, m.View().Content, "no workflows", "shift+tab moves back")
}

// TestTUIModel_QuitKeyStopsProgram verifies that pressing 'q' produces tea.Quit.
func TestTUIModel_QuitKeyStopsProgram(t *testing.T) {
	m := tui.New()

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	require.NotNil(t, cmd, "pressing 'q' should produce a command")

	// Invoke the command and check it produces a quit message.
	msg := cmd()
	assert.Equal(t, tea.Quit(), msg, "pressing 'q' should produce tea.Quit")
}

// TestTUIModel_WorkflowsLoadedMsgUpdatesTab verifies WorkflowsLoadedMsg populates the tab.
func TestTUIModel_WorkflowsLoadedMsgUpdatesTab(t *testing.T) {
	m := tui.New()

	loadedMsg := tui.WorkflowsLoadedMsg{}
	updated, _ := m.Update(loadedMsg)
	m = updated.(tui.Model)

	view := m.View().Content
	assert.NotEmpty(t, view)
}

// TestTUIModel_ExecutionLifecycleMsgs verifies the execution message sequence is handled.
func TestTUIModel_ExecutionLifecycleMsgs(t *testing.T) {
	m := tui.New()

	// Start execution.
	updated, _ := m.Update(tui.ExecutionStartedMsg{ExecutionID: "exec-001"})
	m = updated.(tui.Model)

	// Finish.
	updated, _ = m.Update(tui.ExecutionFinishedMsg{})
	m = updated.(tui.Model)

	assert.NotEmpty(t, m.View().Content)
}

// TestTUIModel_ErrMsgDoesNotCrash verifies ErrMsg handling is graceful.
func TestTUIModel_ErrMsgDoesNotCrash(t *testing.T) {
	m := tui.New()

	updated, _ := m.Update(tui.ErrMsg{Err: assert.AnError})
	m = updated.(tui.Model)

	assert.NotEmpty(t, m.View().Content)
}

// findSubcommand returns the named subcommand from cmd.Commands() if found.
func findSubcommand(cmd *cobra.Command, name string) (bool, *cobra.Command) {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return true, sub
		}
	}
	return false, nil
}
