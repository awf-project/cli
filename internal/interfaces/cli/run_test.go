package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestRunCommand_NoArgs(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no workflow name provided")
	}
}

func TestRunCommand_WorkflowNotFound(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "nonexistent-workflow"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestRunCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'run' subcommand")
	}
}

func TestRunCommand_HasInputFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("input")
			if flag == nil {
				t.Error("expected 'run' command to have --input flag")
			}
			return
		}
	}

	t.Error("run command not found")
}

func TestRunCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Execute a workflow") {
		t.Errorf("expected help text, got: %s", output)
	}
}

func TestRunCommand_HasOutputFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("output")
			if flag == nil {
				t.Error("expected 'run' command to have --output flag")
			}
			return
		}
	}

	t.Error("run command not found")
}

func TestRunCommand_InvalidOutputMode(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "test-workflow", "--output=invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid output mode")
	}
	if !strings.Contains(err.Error(), "invalid output mode") {
		t.Errorf("expected 'invalid output mode' error, got: %v", err)
	}
}
