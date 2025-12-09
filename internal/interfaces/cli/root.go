package cli

import (
	"github.com/spf13/cobra"
)

// Version information (set at build time via ldflags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// NewRootCommand creates the root CLI command.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "awf",
		Short: "AI Workflow CLI - Orchestrate AI agents through YAML workflows",
		Long: `AWF (AI Workflow CLI) is a command-line tool for orchestrating AI agents
through configurable YAML workflows with state machine execution.

Examples:
  awf run analyze-code --input file=main.go
  awf validate my-workflow
  awf list
  awf status abc123`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add subcommands
	cmd.AddCommand(newVersionCommand())

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Display the version, commit hash, and build date of awf.",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("awf version %s\n", Version)
			cmd.Printf("commit: %s\n", Commit)
			cmd.Printf("built: %s\n", BuildDate)
		},
	}
}
