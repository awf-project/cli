package agents

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
)

// ExecCLIExecutor implements CLIExecutor using os/exec for direct binary execution.
// Unlike shell execution via detected shell ($SHELL), this executes binaries directly without
// shell interpretation, making it suitable for invoking external CLI tools like
// claude, gemini, codex, etc.
type ExecCLIExecutor struct{}

func NewExecCLIExecutor() *ExecCLIExecutor {
	return &ExecCLIExecutor{}
}

func (e *ExecCLIExecutor) Run(ctx context.Context, name string, stdoutW, stderrW io.Writer, args ...string) (stdout, stderr []byte, err error) {
	return e.RunWithEnv(ctx, name, nil, stdoutW, stderrW, args...)
}

func (e *ExecCLIExecutor) RunWithEnv(ctx context.Context, name string, env map[string]string, stdoutW, stderrW io.Writer, args ...string) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for key, value := range env {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}

	// Process group management for clean termination
	// Setpgid: true creates a new process group with the child as leader
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Kill entire process group on context cancellation (Go 1.20+)
	// Using negative PID sends SIGKILL to the entire process group
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 100 * time.Millisecond

	var stdoutBuf, stderrBuf bytes.Buffer
	if stdoutW != nil {
		cmd.Stdout = io.MultiWriter(&stdoutBuf, stdoutW)
	} else {
		cmd.Stdout = &stdoutBuf
	}
	if stderrW != nil {
		cmd.Stderr = io.MultiWriter(&stderrBuf, stderrW)
	} else {
		cmd.Stderr = &stderrBuf
	}

	if startErr := cmd.Start(); startErr != nil {
		return []byte{}, []byte{}, fmt.Errorf("CLI start failed for '%s': %w", name, startErr)
	}

	execErr := cmd.Wait()

	stdoutBytes := stdoutBuf.Bytes()
	stderrBytes := stderrBuf.Bytes()

	if ctx.Err() != nil {
		// Context cancelled or timed out: kill orphaned descendants that cmd.Cancel may have missed
		if cmd.Process != nil {
			pid := cmd.Process.Pid
			killDescendants(pid)
		}
		return stdoutBytes, stderrBytes, fmt.Errorf("CLI execution cancelled for '%s': %w", name, ctx.Err())
	}

	if execErr != nil {
		return stdoutBytes, stderrBytes, fmt.Errorf("CLI execution failed for '%s': %w", name, execErr)
	}

	return stdoutBytes, stderrBytes, nil
}

// killDescendants recursively kills all descendant processes of the given PID.
// This handles cases where child processes create their own process groups.
func killDescendants(pid int) {
	children := findChildPIDs(pid)

	for _, child := range children {
		killDescendants(child)
	}

	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func findChildPIDs(parentPID int) []int {
	var children []int

	procDirs, err := filepath.Glob("/proc/[0-9]*")
	if err != nil {
		return children
	}

	for _, procDir := range procDirs {
		statPath := filepath.Join(procDir, "stat")
		data, err := os.ReadFile(statPath)
		if err != nil {
			continue
		}

		// Parse stat file: pid (comm) state ppid ...
		// Find the closing parenthesis to handle command names with spaces
		statStr := string(data)
		closeParenIdx := len(statStr) - 1
		for i := len(statStr) - 1; i >= 0; i-- {
			if statStr[i] == ')' {
				closeParenIdx = i
				break
			}
		}

		// Fields after (comm) are space-separated
		fields := bytes.Fields([]byte(statStr[closeParenIdx+2:]))
		if len(fields) < 2 {
			continue
		}

		// Field 0 after (comm) is state, field 1 is ppid
		ppid, err := strconv.Atoi(string(fields[1]))
		if err != nil {
			continue
		}

		if ppid == parentPID {
			// Extract PID from proc path
			pidStr := filepath.Base(procDir)
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}
			children = append(children, pid)
		}
	}

	return children
}

// osProcessAdapter wraps *exec.Cmd to implement ports.CLIProcess.
// Wait is idempotent: whichever goroutine wins the sync.Once race drives cmd.Wait
// and closes doneCh; all other callers return immediately after once.Do.
type osProcessAdapter struct {
	cmd     *exec.Cmd
	once    sync.Once
	waitErr error
	doneCh  chan struct{}
}

func (a *osProcessAdapter) Signal(sig os.Signal) error {
	if a.cmd.Process == nil {
		return nil
	}
	return a.cmd.Process.Signal(sig)
}

func (a *osProcessAdapter) Wait() error {
	a.once.Do(func() {
		a.waitErr = a.cmd.Wait()
		close(a.doneCh)
	})
	return a.waitErr
}

func (a *osProcessAdapter) Done() <-chan struct{} {
	return a.doneCh
}

// Start launches a binary without blocking.
// A background goroutine drives cmd.Wait so that Done() is closed when the process exits.
func (e *ExecCLIExecutor) Start(ctx context.Context, name string, args ...string) (ports.CLIProcess, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	adapter := &osProcessAdapter{
		cmd:    cmd,
		doneCh: make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("CLI start failed for '%s': %w", name, err)
	}

	go func() {
		adapter.once.Do(func() {
			adapter.waitErr = cmd.Wait()
			close(adapter.doneCh)
		})
	}()

	return adapter, nil
}

// Compile-time interface verification
var (
	_ ports.CLIExecutor = (*ExecCLIExecutor)(nil)
	_ ports.CLIProcess  = (*osProcessAdapter)(nil)
)
