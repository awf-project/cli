package agents

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/vanoix/awf/internal/domain/ports"
)

// ExecCLIExecutor implements CLIExecutor using os/exec for direct binary execution.
// Unlike shell execution via /bin/sh -c, this executes binaries directly without
// shell interpretation, making it suitable for invoking external CLI tools like
// claude, gemini, codex, etc.
type ExecCLIExecutor struct{}

// NewExecCLIExecutor creates a new production CLI executor.
func NewExecCLIExecutor() *ExecCLIExecutor {
	return &ExecCLIExecutor{}
}

// Run executes a binary with given arguments, returning stdout, stderr, and any error.
// Implements ports.CLIExecutor interface.
func (e *ExecCLIExecutor) Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error) {
	// Create command with context for cancellation/timeout support
	cmd := exec.CommandContext(ctx, name, args...)

	// Create buffers to capture output
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Execute the command
	execErr := cmd.Run()

	// Return captured output (never nil, use empty slices)
	stdoutBytes := stdoutBuf.Bytes()
	stderrBytes := stderrBuf.Bytes()

	// Ensure we never return nil slices
	if stdoutBytes == nil {
		stdoutBytes = []byte{}
	}
	if stderrBytes == nil {
		stderrBytes = []byte{}
	}

	// Wrap error with context if execution failed
	if execErr != nil {
		return stdoutBytes, stderrBytes, fmt.Errorf("CLI execution failed for '%s': %w", name, execErr)
	}

	return stdoutBytes, stderrBytes, nil
}

// Compile-time interface verification
var _ ports.CLIExecutor = (*ExecCLIExecutor)(nil)
