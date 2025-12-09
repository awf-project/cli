package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestNewRootCommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	if cmd.Use != "awf" {
		t.Errorf("expected Use 'awf', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

func TestRootCommandHelp(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "awf") {
		t.Error("expected help output to contain 'awf'")
	}
	if !strings.Contains(output, "AI Workflow") {
		t.Error("expected help output to contain 'AI Workflow'")
	}
}

func TestVersionCommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "awf version") {
		t.Errorf("expected version output, got: %s", output)
	}
}

func TestVersionCommandFlags(t *testing.T) {
	// Set version info
	cli.Version = "1.0.0"
	cli.Commit = "abc123"
	cli.BuildDate = "2024-01-01"

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"version"})

	_ = cmd.Execute()
	output := buf.String()

	if !strings.Contains(output, "1.0.0") {
		t.Errorf("expected version '1.0.0' in output: %s", output)
	}
	if !strings.Contains(output, "abc123") {
		t.Errorf("expected commit 'abc123' in output: %s", output)
	}
}

func TestRootCommandHasVersionSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "version" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'version' subcommand")
	}
}

func TestRootCommand_HasAllSubcommands(t *testing.T) {
	cmd := cli.NewRootCommand()

	expectedCommands := []string{"version", "list", "run", "status", "validate"}

	for _, expected := range expectedCommands {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected root command to have '%s' subcommand", expected)
		}
	}
}

func TestRootCommand_GlobalFlags(t *testing.T) {
	cmd := cli.NewRootCommand()

	flags := []string{"verbose", "quiet", "no-color", "log-level", "config", "storage"}

	for _, flag := range flags {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("expected global flag '--%s' to exist", flag)
		}
	}
}

func TestRootCommand_VerboseShortFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().ShorthandLookup("v")
	if flag == nil {
		t.Error("expected -v shorthand for --verbose")
	}
}

func TestRootCommand_QuietShortFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().ShorthandLookup("q")
	if flag == nil {
		t.Error("expected -q shorthand for --quiet")
	}
}
