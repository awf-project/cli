package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
)

func TestShellExecutor_Execute_SimpleCommand(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		wantStdout string
		wantCode   int
	}{
		{
			name:       "echo hello",
			command:    "echo hello",
			wantStdout: "hello\n",
			wantCode:   0,
		},
		{
			name:       "multiline output",
			command:    "echo line1; echo line2",
			wantStdout: "line1\nline2\n",
			wantCode:   0,
		},
		{
			name:       "empty output",
			command:    "true",
			wantStdout: "",
			wantCode:   0,
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{Program: tt.command}
			result, err := executor.Execute(context.Background(), cmd)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, result.Stdout)
			assert.Equal(t, tt.wantCode, result.ExitCode)
		})
	}
}

func TestShellExecutor_Execute_CapturesStderr(t *testing.T) {
	executor := NewShellExecutor()
	cmd := ports.Command{Program: "echo error >&2"}

	result, err := executor.Execute(context.Background(), cmd)

	require.NoError(t, err)
	assert.Equal(t, "error\n", result.Stderr)
	assert.Empty(t, result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
}

func TestShellExecutor_Execute_BothStdoutAndStderr(t *testing.T) {
	executor := NewShellExecutor()
	cmd := ports.Command{Program: "echo out; echo err >&2"}

	result, err := executor.Execute(context.Background(), cmd)

	require.NoError(t, err)
	assert.Equal(t, "out\n", result.Stdout)
	assert.Equal(t, "err\n", result.Stderr)
	assert.Equal(t, 0, result.ExitCode)
}

func TestShellExecutor_Execute_NonZeroExitCode(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantCode int
	}{
		{
			name:     "exit 1",
			command:  "exit 1",
			wantCode: 1,
		},
		{
			name:     "exit 42",
			command:  "exit 42",
			wantCode: 42,
		},
		{
			name:     "false command",
			command:  "false",
			wantCode: 1,
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{Program: tt.command}
			result, err := executor.Execute(context.Background(), cmd)

			// non-zero exit code is not an error
			require.NoError(t, err)
			assert.Equal(t, tt.wantCode, result.ExitCode)
		})
	}
}

func TestShellExecutor_Execute_Timeout(t *testing.T) {
	executor := NewShellExecutor()
	cmd := ports.Command{
		Program: "sleep 10",
		Timeout: 1, // 1 second timeout
	}

	start := time.Now()
	result, err := executor.Execute(context.Background(), cmd)
	elapsed := time.Since(start)

	// should complete within ~2 seconds (1s timeout + overhead)
	assert.Less(t, elapsed, 3*time.Second)
	// timeout should return context error
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	// result should still be returned (partial)
	assert.NotNil(t, result)
	assert.Equal(t, -1, result.ExitCode)
}

func TestShellExecutor_Execute_ContextTimeout(t *testing.T) {
	executor := NewShellExecutor()
	cmd := ports.Command{Program: "sleep 10"}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	result, err := executor.Execute(ctx, cmd)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 3*time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NotNil(t, result)
}

func TestShellExecutor_Execute_ContextCancellation(t *testing.T) {
	executor := NewShellExecutor()
	cmd := ports.Command{Program: "sleep 10"}

	ctx, cancel := context.WithCancel(context.Background())

	// cancel after 500ms
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result, err := executor.Execute(ctx, cmd)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 2*time.Second)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotNil(t, result)
}

func TestShellExecutor_Execute_WorkingDirectory(t *testing.T) {
	executor := NewShellExecutor()
	tmpDir := t.TempDir()

	cmd := ports.Command{
		Program: "pwd",
		Dir:     tmpDir,
	}

	result, err := executor.Execute(context.Background(), cmd)

	require.NoError(t, err)
	assert.Equal(t, tmpDir+"\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
}

func TestShellExecutor_Execute_Environment(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		command    string
		wantStdout string
	}{
		{
			name:       "single env var",
			env:        map[string]string{"FOO": "bar"},
			command:    "echo $FOO",
			wantStdout: "bar\n",
		},
		{
			name:       "multiple env vars",
			env:        map[string]string{"FOO": "hello", "BAR": "world"},
			command:    "echo $FOO $BAR",
			wantStdout: "hello world\n",
		},
		{
			name:       "override existing env",
			env:        map[string]string{"HOME": "/custom/home"},
			command:    "echo $HOME",
			wantStdout: "/custom/home\n",
		},
	}

	executor := NewShellExecutor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ports.Command{
				Program: tt.command,
				Env:     tt.env,
			}

			result, err := executor.Execute(context.Background(), cmd)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, result.Stdout)
		})
	}
}

func TestShellExecutor_ImplementsInterface(t *testing.T) {
	var _ ports.CommandExecutor = (*ShellExecutor)(nil)
}
