package ports

import (
	"context"
	"io"
	"os"
)

// CLIProcess is an asynchronous subprocess handle returned by CLIExecutor.Start.
// Signal and Wait are safe to call concurrently; Wait is idempotent.
//
// On Windows, Signal(os.Interrupt) is best-effort; callers must treat the 5-second
// deadline as mandatory and fall back to Signal(os.Kill) unconditionally.
type CLIProcess interface {
	Signal(sig os.Signal) error
	Wait() error
	Done() <-chan struct{}
}

// CLIExecutor defines the contract for executing external CLI binaries.
// Unlike CommandExecutor (shell execution via detected shell), this executes
// binaries directly without shell interpretation.
//
// This interface is designed for testing agent providers that invoke external
// CLI tools (claude, gemini, codex, etc.) by allowing test code to inject
// mock implementations that return predefined responses.
type CLIExecutor interface {
	// Run executes a binary with given arguments.
	// Returns stdout and stderr as byte slices, plus any execution error.
	//
	// When stdoutW or stderrW are non-nil, output is tee'd to these writers
	// in real-time (streaming mode) while also being captured in the returned
	// byte slices. When nil, output is only captured (buffer mode).
	//
	// The context allows cancellation and timeout control.
	// If the context is cancelled, the execution should be terminated.
	//
	// Error cases:
	// - Binary not found: error != nil
	// - Non-zero exit code: error != nil (error should contain exit code info)
	// - Context cancelled/timeout: error will be context.Canceled or context.DeadlineExceeded
	Run(ctx context.Context, name string, stdoutW, stderrW io.Writer, args ...string) (stdout, stderr []byte, err error)

	// Start launches a binary without blocking and returns a CLIProcess handle
	// for signal, wait, and done-notification lifecycle control.
	Start(ctx context.Context, name string, args ...string) (CLIProcess, error)
}
