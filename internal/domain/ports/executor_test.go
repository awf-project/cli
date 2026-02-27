package ports_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

type mockExecutor struct{}

func (m *mockExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	return &ports.CommandResult{
		Stdout:   "mock output",
		ExitCode: 0,
	}, nil
}

func TestCommandExecutor(t *testing.T) {
	mock := &mockExecutor{}
	cmd := &ports.Command{Program: "test"}

	result, err := mock.Execute(context.Background(), cmd)

	assert.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "mock output", result.Stdout)
}
