package ports

import (
	"context"
	"io"
)

// Note: Program is passed to the user's detected shell (via $SHELL, fallback
// /bin/sh) as a shell command string, allowing shell features like pipes and
// redirects. Use ShellEscape() from pkg/interpolation for user-provided values.
type Command struct {
	Program string
	Dir     string
	Env     map[string]string
	Timeout int
	Stdout  io.Writer
	Stderr  io.Writer
}

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type CommandExecutor interface {
	Execute(ctx context.Context, cmd *Command) (*CommandResult, error)
}
