package ports

import (
	"context"
	"io"
)

// Command represents a command to execute.
//
// When IsScriptFile is false (default), Program is passed to the user's detected
// shell (via $SHELL, fallback /bin/sh) as a shell command string, allowing shell
// features like pipes and redirects.
//
// When IsScriptFile is true and Program starts with a shebang (#!), the content
// is written to a temporary file, made executable, and executed directly — letting
// the kernel dispatch the correct interpreter. If no shebang is present, execution
// falls back to $SHELL -c for backward compatibility.
//
// Use ShellEscape() from pkg/interpolation for user-provided values.
type Command struct {
	Program      string
	Dir          string
	Env          map[string]string
	IsScriptFile bool
	Stdout       io.Writer
	Stderr       io.Writer
}

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type CommandExecutor interface {
	Execute(ctx context.Context, cmd *Command) (*CommandResult, error)
}
