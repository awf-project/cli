package ports

import (
	"context"
	"io"
)

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
}
