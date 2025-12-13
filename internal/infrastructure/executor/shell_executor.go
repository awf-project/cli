package executor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/vanoix/awf/internal/domain/ports"
)

// ShellExecutor executes shell commands via /bin/sh -c.
type ShellExecutor struct{}

// NewShellExecutor creates a new ShellExecutor.
func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{}
}

// Execute runs a command and returns the result.
func (e *ShellExecutor) Execute(ctx context.Context, cmd ports.Command) (*ports.CommandResult, error) {
	// apply command-level timeout if specified
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(cmd.Timeout)*time.Second)
		defer cancel()
	}

	// build command with shell
	execCmd := exec.CommandContext(ctx, "/bin/sh", "-c", cmd.Program)

	// process group for clean termination
	execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// working directory
	if cmd.Dir != "" {
		execCmd.Dir = cmd.Dir
	}

	// environment variables
	if len(cmd.Env) > 0 {
		execCmd.Env = os.Environ()
		for k, v := range cmd.Env {
			execCmd.Env = append(execCmd.Env, k+"="+v)
		}
	}

	// capture output (always needed for result)
	var stdout, stderr bytes.Buffer

	// setup stdout writer with optional streaming
	if cmd.Stdout != nil {
		execCmd.Stdout = io.MultiWriter(&stdout, cmd.Stdout)
	} else {
		execCmd.Stdout = &stdout
	}

	// setup stderr writer with optional streaming
	if cmd.Stderr != nil {
		execCmd.Stderr = io.MultiWriter(&stderr, cmd.Stderr)
	} else {
		execCmd.Stderr = &stderr
	}

	// execute
	err := execCmd.Run()

	result := &ports.CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	// handle context cancellation - kill process group
	if ctx.Err() != nil {
		if execCmd.Process != nil {
			_ = syscall.Kill(-execCmd.Process.Pid, syscall.SIGKILL)
		}
		result.ExitCode = -1
		return result, ctx.Err()
	}

	// extract exit code from ExitError
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil // non-zero exit is not an error for us
	}

	if err != nil {
		return result, err // actual error (command not found, etc.)
	}

	result.ExitCode = 0
	return result, nil
}
