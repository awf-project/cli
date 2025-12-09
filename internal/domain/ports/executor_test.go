package ports_test

import (
	"context"
	"testing"

	"github.com/vanoix/awf/internal/domain/ports"
)

type mockExecutor struct{}

func (m *mockExecutor) Execute(ctx context.Context, cmd ports.Command) (*ports.CommandResult, error) {
	return &ports.CommandResult{
		Stdout:   "mock output",
		ExitCode: 0,
	}, nil
}

func TestCommandExecutorInterface(t *testing.T) {
	var _ ports.CommandExecutor = (*mockExecutor)(nil)
}

func TestCommandStruct(t *testing.T) {
	cmd := ports.Command{
		Program: "echo",
		Args:    []string{"hello", "world"},
		Dir:     "/tmp",
		Env:     map[string]string{"FOO": "bar"},
		Timeout: 30,
	}

	if cmd.Program != "echo" {
		t.Errorf("expected Program 'echo', got '%s'", cmd.Program)
	}
	if len(cmd.Args) != 2 {
		t.Errorf("expected 2 Args, got %d", len(cmd.Args))
	}
	if cmd.Env["FOO"] != "bar" {
		t.Errorf("expected Env FOO='bar', got '%s'", cmd.Env["FOO"])
	}
}

func TestCommandResultStruct(t *testing.T) {
	result := ports.CommandResult{
		Stdout:   "success",
		Stderr:   "warning",
		ExitCode: 0,
	}

	if result.Stdout != "success" {
		t.Errorf("expected Stdout 'success', got '%s'", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
}

func TestMockExecutorExecute(t *testing.T) {
	mock := &mockExecutor{}
	cmd := ports.Command{Program: "test"}

	result, err := mock.Execute(context.Background(), cmd)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
}
