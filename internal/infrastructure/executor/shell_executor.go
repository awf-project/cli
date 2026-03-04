package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// ShellExecutorOption configures a ShellExecutor.
type ShellExecutorOption func(*ShellExecutor)

// WithShell overrides the detected shell path.
func WithShell(path string) ShellExecutorOption {
	return func(e *ShellExecutor) {
		e.shellPath = path
	}
}

type ShellExecutor struct {
	masker    *logger.SecretMasker
	shellPath string
}

// NewShellExecutor creates a new ShellExecutor.
func NewShellExecutor(opts ...ShellExecutorOption) *ShellExecutor {
	e := &ShellExecutor{
		masker:    logger.NewSecretMasker(),
		shellPath: detectShell(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// detectShell returns the user's preferred shell from $SHELL,
// falling back to /bin/sh if $SHELL is unset, relative, or invalid.
func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" || !filepath.IsAbs(shell) {
		return "/bin/sh"
	}
	if _, err := os.Stat(shell); err != nil {
		return "/bin/sh"
	}
	return shell
}

// Execute runs a command and returns the result.
func (e *ShellExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(cmd.Timeout)*time.Second)
		defer cancel()
	}

	var execCmd *exec.Cmd
	var cleanup func()
	if cmd.IsScriptFile && hasShebang(cmd.Program) {
		var err error
		execCmd, cleanup, err = e.executeScriptFile(ctx, cmd.Program)
		if err != nil {
			return nil, err
		}
		defer cleanup()
	} else {
		execCmd = exec.CommandContext(ctx, e.shellPath, "-c", cmd.Program) //nolint:gosec // G204: intentional dynamic shell
	}

	// process group for clean termination
	execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// kill entire process group on context cancellation (Go 1.20+)
	execCmd.Cancel = func() error {
		return syscall.Kill(-execCmd.Process.Pid, syscall.SIGKILL)
	}
	execCmd.WaitDelay = 100 * time.Millisecond

	if cmd.Dir != "" {
		execCmd.Dir = cmd.Dir
	}

	if len(cmd.Env) > 0 {
		execCmd.Env = os.Environ()
		for k, v := range cmd.Env {
			execCmd.Env = append(execCmd.Env, k+"="+v)
		}
	}

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

	err := execCmd.Run()

	// mask secrets in output
	stdoutStr := e.masker.MaskText(stdout.String(), cmd.Env)
	stderrStr := e.masker.MaskText(stderr.String(), cmd.Env)

	result := &ports.CommandResult{
		Stdout: stdoutStr,
		Stderr: stderrStr,
	}

	// handle context cancellation (process group already killed by Cancel func)
	if ctx.Err() != nil {
		result.ExitCode = -1
		return result, fmt.Errorf("command execution: %w", ctx.Err())
	}

	// extract exit code from ExitError
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil // non-zero exit is not an error for us
	}

	if err != nil {
		return result, fmt.Errorf("command execution: %w", err) // actual error (command not found, etc.)
	}

	result.ExitCode = 0
	return result, nil
}

func hasShebang(content string) bool {
	return strings.HasPrefix(content, "#!")
}

func (e *ShellExecutor) executeScriptFile(ctx context.Context, content string) (*exec.Cmd, func(), error) {
	f, err := os.CreateTemp("", "awf-script-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp script file: %w", err)
	}
	path := f.Name()

	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, nil, fmt.Errorf("write temp script file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, nil, fmt.Errorf("close temp script file: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil { //nolint:gosec // G302: intentional 0o700 for executable script
		_ = os.Remove(path)
		return nil, nil, fmt.Errorf("chmod temp script file: %w", err)
	}

	execCmd := exec.CommandContext(ctx, path) //nolint:gosec // G204: intentional direct script execution
	cleanup := func() { _ = os.Remove(path) }
	return execCmd, cleanup, nil
}
