//go:build integration

package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestStatusCommand_NoArgs(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no workflow ID provided")
	}
}

func TestStatusCommand_NotFound(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "nonexistent-id"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent workflow ID")
	}

	output := buf.String()
	errOutput := buf.String()
	combined := output + errOutput
	if !strings.Contains(combined, "not found") && err != nil && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' message, got output: %s, err: %v", combined, err)
	}
}

func TestStatusCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "status" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'status' subcommand")
	}
}

func TestStatusCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "workflow") {
		t.Errorf("expected help text about workflow, got: %s", output)
	}
}
