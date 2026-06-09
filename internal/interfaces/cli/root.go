package cli

import (
	"os"

	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/awf-project/cli/internal/interfaces/tui"
	"github.com/spf13/cobra"
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

func NewRootCommand() *cobra.Command {
	cmd, _ := newRootCommand(false)
	return cmd
}

// NewRootCommandAutoFacade builds the root command with a CLI-wide WorkflowFacade wired
// into cfg.Facade lazily (in PersistentPreRun, after flag parsing so --storage is honored).
// It returns the command and a cleanup that releases facade resources (closes the history
// store); main must call it after Execute returns.
func NewRootCommandAutoFacade() (cmd *cobra.Command, cleanup func()) {
	return newRootCommand(true)
}

func newRootCommand(autoFacade bool) (cmd *cobra.Command, cleanup func()) {
	cfg := DefaultConfig()
	var facadeCleanup func()

	cmd = &cobra.Command{
		Use:   "awf",
		Short: "AI Workflow Framework CLI - Orchestrate AI agents through YAML workflows",
		Long: `AWF (AI Workflow Framework CLI) is a command-line tool for orchestrating AI agents
through configurable YAML workflows with state machine execution.

Examples:
  awf run analyze-code --input file=main.go
  awf validate my-workflow
  awf list
  awf status abc123`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Show migration notice if legacy directory exists
			if !cfg.Quiet {
				CheckMigration(cmd.ErrOrStderr())
			}
		},
	}

	// Global flags
	pf := cmd.PersistentFlags()
	pf.BoolVarP(&cfg.Verbose, "verbose", "v", false, "Enable verbose output")
	pf.BoolVarP(&cfg.Quiet, "quiet", "q", false, "Suppress non-error output")
	pf.BoolVar(&cfg.NoColor, "no-color", false, "Disable colored output")
	pf.BoolVar(&cfg.NoHints, "no-hints", false, "Disable error hint suggestions")
	pf.StringVar(&cfg.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	pf.StringVar(&cfg.ConfigPath, "config", "", "Path to config file")
	pf.StringVar(&cfg.StoragePath, "storage", cfg.StoragePath, "Path to storage directory")

	var formatStr string
	pf.StringVarP(&formatStr, "format", "f", "text", "Output format (text, json, table, quiet)")

	// Parse format flag before each command.
	// mcp-serve is exempt: it communicates exclusively via JSON-RPC on stdio and
	// must never call os.Exit(1) for an irrelevant --format flag. All other commands
	// go through the normal format validation path.
	originalPreRun := cmd.PersistentPreRun
	cmd.PersistentPreRun = func(c *cobra.Command, args []string) {
		if c.Annotations[annotationSkipFormatValidation] == "true" {
			// Skip --format validation for commands that communicate via a structured
			// protocol (e.g. mcp-serve uses JSON-RPC on stdio). The annotation is set
			// on the command at construction time; using an annotation is more robust
			// than matching c.Name() because it survives command renames.
			if originalPreRun != nil {
				originalPreRun(c, args)
			}
			return
		}
		format, err := ui.ParseOutputFormat(formatStr)
		if err != nil {
			c.PrintErrf("Error: %s\n", err)
			os.Exit(1)
		}
		cfg.OutputFormat = format
		if autoFacade && cfg.Facade == nil {
			if facade, cleanup := buildFacade(cfg); facade != nil {
				cfg.Facade = facade
				facadeCleanup = cleanup
			}
		}
		if originalPreRun != nil {
			originalPreRun(c, args)
		}
	}

	// Subcommands
	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newInitCommand(cfg))
	cmd.AddCommand(newListCommand(cfg))
	cmd.AddCommand(newRunCommand(cfg))
	cmd.AddCommand(newResumeCommand(cfg))
	cmd.AddCommand(newResumeListCommand(cfg))
	cmd.AddCommand(newStatusCommand(cfg))
	cmd.AddCommand(newValidateCommand(cfg))
	cmd.AddCommand(newHistoryCommand(cfg))
	cmd.AddCommand(newPluginCommand(cfg))
	cmd.AddCommand(newWorkflowCommand(cfg))
	cmd.AddCommand(newConfigCommand(cfg))
	cmd.AddCommand(newDiagramCommand(cfg))
	cmd.AddCommand(newErrorCommand(cfg))
	cmd.AddCommand(newUpgradeCommand(cfg))
	cmd.AddCommand(tui.NewCommand())
	cmd.AddCommand(NewServeCommand())
	cmd.AddCommand(newMCPServeCommand(Deps{}))
	cmd.AddCommand(newACPServeCommand(Deps{}))

	return cmd, func() {
		if facadeCleanup != nil {
			facadeCleanup()
		}
	}
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
