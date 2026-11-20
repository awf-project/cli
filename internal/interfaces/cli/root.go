package cli

import (
	"fmt"
	"os"
	"sync"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"

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

// RootOption configures dependencies for an embedded root command.
type RootOption func(*Config)

// WithExecutionSetupOptions extends execution wiring for fresh and resumed runs.
func WithExecutionSetupOptions(opts ...application.SetupOption) RootOption {
	return func(cfg *Config) { cfg.ExecutionSetupOptions = append(cfg.ExecutionSetupOptions, opts...) }
}

// WithCommandExecutor supplies the command executor used by execution wiring.
func WithCommandExecutor(executor ports.CommandExecutor) RootOption {
	return func(cfg *Config) { cfg.CommandExecutor = executor }
}

// WithDiscoveredOperationProvider supplies the raw external operation provider
// used by execution composition. It is primarily useful for embedded commands
// whose host owns plugin discovery.
func WithDiscoveredOperationProvider(provider ports.OperationProvider) RootOption {
	return func(cfg *Config) { cfg.DiscoveredOperations = provider }
}

func NewRootCommand(opts ...RootOption) *cobra.Command {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	cmd, _ := newRootCommandWithConfig(cfg, false)
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
	return newRootCommandWithBuilders(DefaultConfig(), autoFacade, buildFacade, buildValidationFacade)
}

func newRootCommandWithBuilders(
	cfg *Config,
	autoFacade bool,
	build facadeBuilder,
	buildValidation validationFacadeBuilder,
) (cmd *cobra.Command, cleanup func()) {
	var facadeCleanup func()
	var cleanupOnce sync.Once
	releaseFacade := func() {
		if facadeCleanup == nil {
			return
		}
		release := facadeCleanup
		facadeCleanup = nil
		release()
	}

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
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Show migration notice if legacy directory exists
			if !cfg.Quiet {
				CheckMigration(cmd.ErrOrStderr())
			}
		},
	}

	cmd.SetVersionTemplate("awf version {{.Version}}\ncommit: " + Commit + "\nbuilt: " + BuildDate + "\n")
	cmd.Flags().Bool("version", false, "version for awf")

	// Global flags
	pf := cmd.PersistentFlags()
	pf.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose output")
	pf.BoolVarP(&cfg.Quiet, "quiet", "q", false, "Suppress non-error output")
	pf.BoolVar(&cfg.NoColor, "no-color", false, "Disable colored output")
	pf.BoolVar(&cfg.NoHints, "no-hints", false, "Disable error hint suggestions")
	pf.StringVar(&cfg.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	pf.StringVar(&cfg.ConfigPath, "config", "", "Path to config file")
	pf.StringVar(&cfg.StoragePath, "storage", cfg.StoragePath, "Path to storage directory")

	var formatStr string
	pf.StringVarP(&formatStr, "format", "f", "text", "Output format (text, json, table, quiet)")

	cmd.PersistentPreRunE = newRootPersistentPreRun(
		cfg, autoFacade, &formatStr, cmd.PersistentPreRun, build, buildValidation,
		func(release func()) { facadeCleanup = release },
	)

	// Subcommands
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

	return cmd, func() { cleanupOnce.Do(releaseFacade) }
}

func newRootCommandWithConfig(cfg *Config, autoFacade bool) (cmd *cobra.Command, cleanup func()) {
	return newRootCommandWithBuilders(cfg, autoFacade, buildFacade, buildValidationFacade)
}

func newRootPersistentPreRun(
	cfg *Config,
	autoFacade bool,
	formatStr *string,
	originalPreRun func(*cobra.Command, []string),
	build facadeBuilder,
	buildValidation validationFacadeBuilder,
	setCleanup func(func()),
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if cmd.Annotations[annotationSkipFormatValidation] == "true" {
			invokePersistentPreRun(originalPreRun, cmd, args)
			return nil
		}
		format, err := ui.ParseOutputFormat(*formatStr)
		if err != nil {
			cmd.PrintErrf("Error: %s\n", err)
			return &exitError{code: ExitUser, err: err, handled: true}
		}
		cfg.OutputFormat = format
		if err := initializeCommandFacade(cmd, cfg, autoFacade, build, buildValidation, setCleanup); err != nil {
			return err
		}
		invokePersistentPreRun(originalPreRun, cmd, args)
		return nil
	}
}

func initializeCommandFacade(
	cmd *cobra.Command,
	cfg *Config,
	autoFacade bool,
	build facadeBuilder,
	buildValidation validationFacadeBuilder,
	setCleanup func(func()),
) error {
	if !autoFacade || cfg.Facade != nil {
		return nil
	}
	if cmd.Annotations[annotationPluginAwareValidation] == "true" {
		skipPlugins, err := cmd.Flags().GetBool("skip-plugins")
		if err != nil {
			return fmt.Errorf("read validation plugin flag: %w", err)
		}
		// Preserve --skip-plugins before facade construction can start external
		// plugins or expose their operations to validation.
		if skipPlugins {
			if facade, cleanup := build(cfg); facade != nil {
				cfg.Facade = facade
				setCleanup(cleanup)
			}
			return nil
		}
		facade, cleanup, err := buildValidation(cfg)
		if err != nil {
			if cleanup != nil {
				cleanup()
			}
			return fmt.Errorf("initialize validation facade: %w", err)
		}
		if facade != nil {
			cfg.Facade = facade
			setCleanup(cleanup)
		}
		return nil
	}
	if facade, cleanup := build(cfg); facade != nil {
		cfg.Facade = facade
		setCleanup(cleanup)
	}
	return nil
}

func invokePersistentPreRun(preRun func(*cobra.Command, []string), cmd *cobra.Command, args []string) {
	if preRun != nil {
		preRun(cmd, args)
	}
}
