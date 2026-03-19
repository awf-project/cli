// Package executor provides infrastructure adapters for command execution.
//
// This package implements the CommandExecutor port from the domain layer,
// providing shell command execution with process management:
//   - ShellExecutor: Executes commands via detected shell ($SHELL, fallback /bin/sh) with context cancellation
//
// Timeout Ownership:
//   - Timeout responsibility owned by application layer (ExecutionService.executeStep)
//   - ShellExecutor respects context cancellation; timeout enforcement is caller's responsibility
//   - Timeout pre-enforcement prevents duplicate timeout stacks (previous bug: timeout applied in both ExecutionService and ShellExecutor)
//
// Architecture:
//   - Domain defines: CommandExecutor port interface, Command and CommandResult types
//   - Infrastructure provides: ShellExecutor adapter with secret masking
//   - Application injects: Executor via dependency injection
//
// Example usage:
//
//	executor := executor.NewShellExecutor()
//	cmd := &ports.Command{Program: "echo hello"}
//	result, err := executor.Execute(ctx, cmd)
//	if err != nil {
//	    // Handle execution error
//	}
//	// Use result.Stdout, result.Stderr, result.ExitCode
//
// Security:
//   - Commands run via detected shell (supports pipes, redirects)
//   - Secret masking for environment variables (SECRET_*, API_KEY*, PASSWORD*)
//   - Process group management for clean termination
//   - Context cancellation propagates to running processes
//
// Component: C056 Infrastructure Package Documentation
// Layer: Infrastructure
package executor
