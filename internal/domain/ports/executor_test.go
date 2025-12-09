package ports_test

import (
	"bytes"
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
		Program: "echo hello world",
		Dir:     "/tmp",
		Env:     map[string]string{"FOO": "bar"},
		Timeout: 30,
	}

	if cmd.Program != "echo hello world" {
		t.Errorf("expected Program 'echo hello world', got '%s'", cmd.Program)
	}
	if cmd.Env["FOO"] != "bar" {
		t.Errorf("expected Env FOO='bar', got '%s'", cmd.Env["FOO"])
	}
}

func TestCommandStructWithWriters(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := ports.Command{
		Program: "echo",
		Stdout:  &stdout,
		Stderr:  &stderr,
	}

	if cmd.Stdout == nil {
		t.Error("expected Stdout writer to be set")
	}
	if cmd.Stderr == nil {
		t.Error("expected Stderr writer to be set")
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
