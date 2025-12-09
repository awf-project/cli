package ports

import (
	"context"
	"io"
)

// Command represents a command to be executed.
type Command struct {
	Program string
	Args    []string
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
