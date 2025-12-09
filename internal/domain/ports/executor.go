package ports

import (
	"context"
	"io"
)

// Command represents a command to be executed.
// Note: Program is passed to /bin/sh -c as a shell command string,
// allowing shell features like pipes and redirects. Use ShellEscape()
// from pkg/interpolation for user-provided values.
type Command struct {
	Program string
	Dir     string
	Env     map[string]string
	Timeout int       // seconds, 0 means default
	Stdout  io.Writer // optional: streaming output for stdout
	Stderr  io.Writer // optional: streaming output for stderr
}

// CommandResult holds the output of an executed command.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandExecutor defines the contract for executing shell commands.
type CommandExecutor interface {
	Execute(ctx context.Context, cmd Command) (*CommandResult, error)
}
