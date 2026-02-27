package main

import (
	"fmt"
	"os"

	"github.com/awf-project/cli/internal/interfaces/cli"
)

func main() {
	cmd := cli.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		// Check for exitError with specific exit code
		if exitErr, ok := err.(interface{ ExitCode() int }); ok {
			// Skip printing if error was already formatted by WriteError
			if handled, ok := err.(interface{ Handled() bool }); !ok || !handled.Handled() {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(exitErr.ExitCode())
		}
		// Default to user error for unknown errors
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitUser)
	}
}
