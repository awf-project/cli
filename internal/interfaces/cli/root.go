package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// Version information (set at build time via ldflags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// App holds CLI dependencies and configuration.
type App struct {
	Config    *Config
	Formatter *ui.Formatter
}

// NewApp creates a new CLI app with the given config.
func NewApp(cfg *Config) *App {
	return &App{
		Config: cfg,
		Formatter: ui.NewFormatter(os.Stdout, ui.FormatOptions{
			Verbose: cfg.Verbose,
			Quiet:   cfg.Quiet,
			NoColor: cfg.NoColor,
		}),
	}
}

// NewRootCommand creates the root CLI command.
func NewRootCommand() *cobra.Command {
	cfg := DefaultConfig()

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

	// Global flags
	pf := cmd.PersistentFlags()
	pf.BoolVarP(&cfg.Verbose, "verbose", "v", false, "Enable verbose output")
	pf.BoolVarP(&cfg.Quiet, "quiet", "q", false, "Suppress non-error output")
	pf.BoolVar(&cfg.NoColor, "no-color", false, "Disable colored output")
	pf.StringVar(&cfg.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	pf.StringVar(&cfg.ConfigPath, "config", "", "Path to config file")
	pf.StringVar(&cfg.StoragePath, "storage", cfg.StoragePath, "Path to storage directory")

	// Subcommands
	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newListCommand(cfg))
	cmd.AddCommand(newRunCommand(cfg))
	cmd.AddCommand(newStatusCommand(cfg))
	cmd.AddCommand(newValidateCommand(cfg))

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
