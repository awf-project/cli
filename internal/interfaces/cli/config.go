package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/analyzer"
	"github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/skills"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/infrastructure/tracker"
	infraTranscript "github.com/awf-project/cli/internal/infrastructure/transcript"
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/awf-project/cli/internal/interfaces/notifierwiring"
)

// OutputMode defines how command output is displayed.
type OutputMode int

const (
	OutputSilent    OutputMode = iota // default: no streaming
	OutputStreaming                   // real-time prefixed output
	OutputBuffered                    // show after completion
)

func (m OutputMode) String() string {
	switch m {
	case OutputSilent:
		return "silent"
	case OutputStreaming:
		return "streaming"
	case OutputBuffered:
		return "buffered"
	default:
		return "unknown"
	}
}

// ParseOutputMode parses a string to OutputMode.
func ParseOutputMode(s string) (OutputMode, error) {
	switch s {
	case "silent":
		return OutputSilent, nil
	case "streaming":
		return OutputStreaming, nil
	case "buffered":
		return OutputBuffered, nil
	default:
		return OutputSilent, fmt.Errorf("invalid output mode: %s (valid: silent, streaming, buffered)", s)
	}
}

// Config holds CLI configuration.
type Config struct {
	ExecutionSetupOptions []application.SetupOption
	CommandExecutor       ports.CommandExecutor
	DiscoveredOperations  ports.OperationProvider
	Verbose               bool
	Quiet                 bool
	NoColor               bool
	NoHints               bool
	OutputMode            OutputMode
	OutputFormat          ui.OutputFormat
	LogLevel              string
	ConfigPath            string
	StoragePath           string
	PluginsDir            string // Override plugin discovery directory (empty = use BuildPluginPaths)
	OtelExporter          string
	OtelServiceName       string
	Facade                ports.WorkflowFacade // nil until wired via NewRootCommandWithFacade or main.go
}

type facadeBuilder func(*Config) (ports.WorkflowFacade, func())

type validationFacadeBuilder func(*Config) (ports.WorkflowFacade, func(), error)

// validationFacadeDependencies are per-construction resource boundaries. They
// make lifecycle verification deterministic without mutable package test hooks.
type validationFacadeDependencies struct {
	initPlugins func(context.Context, *Config, ports.Logger) (*PluginSystemResult, error)
	openHistory func(string) (ports.HistoryStore, error)
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		Verbose:         false,
		Quiet:           false,
		NoColor:         false,
		NoHints:         false,
		OutputMode:      OutputSilent,
		OutputFormat:    ui.FormatText,
		LogLevel:        "info",
		ConfigPath:      "",
		StoragePath:     xdg.AWFDataDir(),
		OtelServiceName: "awf",
	}
}

// BuildWorkflowPaths returns the workflow paths in priority order:
// 1. AWF_WORKFLOWS_PATH env var
// 2. ./.awf/workflows/ (local project)
// 3. $XDG_CONFIG_HOME/awf/workflows/ (global)
func BuildWorkflowPaths() []repository.SourcedPath {
	var paths []repository.SourcedPath

	// 1. Environment variable (highest priority)
	if envPath := os.Getenv("AWF_WORKFLOWS_PATH"); envPath != "" {
		paths = append(paths, repository.SourcedPath{
			Path:   envPath,
			Source: repository.SourceEnv,
		})
	}

	// 2. Local project directory and 3. Global XDG directory (lowest priority)
	paths = append(
		paths,
		repository.SourcedPath{
			Path:   xdg.LocalWorkflowsDir(),
			Source: repository.SourceLocal,
		},
		repository.SourcedPath{
			Path:   xdg.AWFWorkflowsDir(),
			Source: repository.SourceGlobal,
		},
	)

	return paths
}

// NewWorkflowRepository creates a CompositeRepository with standard paths
func NewWorkflowRepository() *repository.CompositeRepository {
	return repository.NewCompositeRepository(BuildWorkflowPaths())
}

func newWorkflowRepositoryForIdentifier(identifier string) (repo *repository.CompositeRepository, workflowName string) {
	if !isExistingWorkflowFile(identifier) {
		return NewWorkflowRepository(), identifier
	}

	cleaned := filepath.Clean(identifier)
	return repository.NewCompositeRepository([]repository.SourcedPath{
		{Path: filepath.Dir(cleaned), Source: repository.SourceLocal},
	}), filepath.Base(cleaned)
}

// buildFacade constructs the plugin-free facade used by ordinary read commands and
// validate --skip-plugins. Execution paths build their own run-capable facade.
func buildFacade(cfg *Config) (facade ports.WorkflowFacade, cleanup func()) {
	noop := func() {}

	repo := NewWorkflowRepository()
	discoverer := workflowpkg.NewPackDiscovererAdapter(workflowPackSearchDirs())

	workflowSvc := application.NewWorkflowService(repo, nil, nil, nil, expression.NewExprValidator())
	workflowSvc.SetPackDiscoverer(discoverer)
	workflowSvc.SetTemplateAnalyzer(analyzer.NewInterpolationAnalyzer())
	// Wire the template-reference validation phase ({{error.*}} outside error hooks,
	// forward references, unknown namespaces) so facade-routed `awf validate` keeps the
	// full validation pipeline the legacy runValidate ran.
	// Validate builtins against the same schemas and notification defaults used by execution.
	logger := ports.NopLogger{}
	projectCfg, configErr := loadProjectConfig(logger)
	if configErr != nil {
		return nil, noop
	}
	builtins := application.NewBuiltinOperationProvider(
		logger, application.NotifyConfig{DefaultBackend: projectCfg.Notify.DefaultBackend},
	)
	notifierProvider := notifierwiring.ComposeOperationProvider(
		notifierwiring.Settings{
			Backend:        projectCfg.Notifier.Backend,
			DefaultChannel: projectCfg.Notifier.DefaultChannel,
		},
		true,
		nil,
	)
	trackerProvider := tracker.NewProvider(nil, projectCfg.Tracker.DefaultBackend, logger)
	workflowSvc.SetPluginOperationProvider(
		pluginmgr.NewCompositeOperationProvider(builtins, notifierProvider, trackerProvider),
	)
	workflowSvc.SetSkillRepository(skills.NewFilesystemSkillRepository(&cliLogger{silent: cfg.Quiet}))

	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(cfg.StoragePath, "history.db"))
	if err != nil {
		return nil, noop
	}
	historySvc := application.NewHistoryService(historyStore, &cliLogger{silent: cfg.Quiet})

	resolver := application.NewResolver(discoverer, repo)

	adapter := application.NewAdapter(
		workflowSvc,
		&application.ExecutionService{},
		historySvc,
		resolver,
		infraTranscript.NewNopRecorder(),
		application.NewSessionRegistry(),
	)

	return adapter, func() { _ = historyStore.Close() }
}

func buildValidationFacade(cfg *Config) (ports.WorkflowFacade, func(), error) {
	return buildValidationFacadeWithDependencies(cfg, validationFacadeDependencies{
		initPlugins: initPluginSystem,
		openHistory: func(path string) (ports.HistoryStore, error) {
			return store.NewSQLiteHistoryStore(path)
		},
	})
}

func buildValidationFacadeWithDependencies(cfg *Config, deps validationFacadeDependencies) (ports.WorkflowFacade, func(), error) {
	noop := func() {}
	logger := &cliLogger{
		formatter: ui.NewFormatter(os.Stderr, ui.FormatOptions{
			Verbose: cfg.Verbose,
			Quiet:   cfg.Quiet,
			NoColor: cfg.NoColor,
		}),
		verbose: cfg.Verbose,
		silent:  cfg.Quiet,
	}
	plugins, err := deps.initPlugins(context.Background(), cfg, logger)
	if err != nil {
		return nil, noop, err
	}

	pluginCleanup := plugins.Cleanup
	if pluginCleanup == nil {
		pluginCleanup = noop
	}
	projectCfg, err := loadProjectConfig(logger)
	if err != nil {
		pluginCleanup()
		return nil, noop, err
	}
	historyStore, err := deps.openHistory(filepath.Join(cfg.StoragePath, "history.db"))
	if err != nil {
		pluginCleanup()
		return nil, noop, err
	}

	repo := NewWorkflowRepository()
	discoverer := workflowpkg.NewPackDiscovererAdapter(workflowPackSearchDirs())
	workflowSvc := application.NewWorkflowService(repo, nil, nil, nil, expression.NewExprValidator())
	workflowSvc.SetPackDiscoverer(discoverer)
	workflowSvc.SetTemplateAnalyzer(analyzer.NewInterpolationAnalyzer())
	workflowSvc.SetPluginOperationProvider(buildValidationOperationProvider(
		plugins.Service,
		plugins.Manager,
		logger,
		notifierwiring.Settings{
			Backend:        projectCfg.Notifier.Backend,
			DefaultChannel: projectCfg.Notifier.DefaultChannel,
		},
	))
	if plugins.RPCManager != nil {
		workflowSvc.SetValidatorProvider(plugins.RPCManager.ValidatorProvider(0))
		workflowSvc.SetStepTypeProvider(plugins.RPCManager.StepTypeProvider(logger))
	}
	workflowSvc.SetSkillRepository(skills.NewFilesystemSkillRepository(logger))

	adapter := application.NewAdapter(
		workflowSvc,
		&application.ExecutionService{},
		application.NewHistoryService(historyStore, logger),
		application.NewResolver(discoverer, repo),
		infraTranscript.NewNopRecorder(),
		application.NewSessionRegistry(),
	)

	cleanup := sync.OnceFunc(func() {
		pluginCleanup()
		_ = historyStore.Close()
	})
	return adapter, cleanup, nil
}

// BuildPromptPaths returns the prompt paths in priority order:
// 1. ./.awf/prompts/ (local project)
// 2. $XDG_CONFIG_HOME/awf/prompts/ (global)
func BuildPromptPaths() []repository.SourcedPath {
	var paths []repository.SourcedPath

	// 1. Environment variable (highest priority)
	if envPath := os.Getenv("AWF_PROMPTS_PATH"); envPath != "" {
		paths = append(paths, repository.SourcedPath{
			Path:   envPath,
			Source: repository.SourceEnv,
		})
	}

	// 2. Local project directory and 3. Global XDG directory (lowest priority)
	paths = append(
		paths,
		repository.SourcedPath{
			Path:   xdg.LocalPromptsDir(),
			Source: repository.SourceLocal,
		},
		repository.SourcedPath{
			Path:   xdg.AWFPromptsDir(),
			Source: repository.SourceGlobal,
		},
	)

	return paths
}

// BuildPluginPaths returns plugin paths in priority order:
// 1. AWF_PLUGINS_PATH env var
// 2. ./.awf/plugins/ (local project)
// 3. $XDG_DATA_HOME/awf/plugins/ (global)
func BuildPluginPaths() []repository.SourcedPath {
	// Environment variable is exclusive — overrides local + global paths
	if envPath := os.Getenv("AWF_PLUGINS_PATH"); envPath != "" {
		return []repository.SourcedPath{
			{Path: envPath, Source: repository.SourceEnv},
		}
	}

	// Local project directory (highest priority) + Global XDG data directory
	return []repository.SourcedPath{
		{Path: xdg.LocalPluginsDir(), Source: repository.SourceLocal},
		{Path: xdg.AWFPluginsDir(), Source: repository.SourceGlobal},
	}
}
