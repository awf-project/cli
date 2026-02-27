package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
)

func TestValidateCommand_NoArgs(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no workflow name provided")
	}
}

func TestValidateCommand_NotFound(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "nonexistent-workflow"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestValidateCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "validate" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'validate' subcommand")
	}
}

func TestValidateCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Validate") {
		t.Errorf("expected help text, got: %s", output)
	}
}
