package agents

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/awf-project/awf/internal/domain/ports"
)

// ExecCLIExecutor implements CLIExecutor using os/exec for direct binary execution.
// Unlike shell execution via /bin/sh -c, this executes binaries directly without
// shell interpretation, making it suitable for invoking external CLI tools like
// claude, gemini, codex, etc.
type ExecCLIExecutor struct{}

func NewExecCLIExecutor() *ExecCLIExecutor {
	return &ExecCLIExecutor{}
}

func (e *ExecCLIExecutor) Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, name, args...)

	// Process group management for clean termination
	// Setpgid: true creates a new process group with the child as leader
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Kill entire process group on context cancellation (Go 1.20+)
	// Using negative PID sends SIGKILL to the entire process group
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 100 * time.Millisecond

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if startErr := cmd.Start(); startErr != nil {
		return []byte{}, []byte{}, fmt.Errorf("CLI start failed for '%s': %w", name, startErr)
	}

	// Ensure complete cleanup after completion (kills orphaned subagents recursively)
	// This runs AFTER cmd.Wait() returns, cleaning up any descendants still running
	defer func() {
		if cmd.Process != nil {
			pid := cmd.Process.Pid
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			killDescendants(pid)
		}
	}()

	execErr := cmd.Wait()

	stdoutBytes := stdoutBuf.Bytes()
	stderrBytes := stderrBuf.Bytes()

	if stdoutBytes == nil {
		stdoutBytes = []byte{}
	}
	if stderrBytes == nil {
		stderrBytes = []byte{}
	}

	if ctx.Err() != nil {
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

// Compile-time interface verification
var _ ports.CLIExecutor = (*ExecCLIExecutor)(nil)
