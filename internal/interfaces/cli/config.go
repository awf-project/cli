package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
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
	Verbose         bool
	Quiet           bool
	NoColor         bool
	NoHints         bool
	OutputMode      OutputMode
	OutputFormat    ui.OutputFormat
	LogLevel        string
	ConfigPath      string
	StoragePath     string
	PluginsDir      string // Override plugin discovery directory (empty = use BuildPluginPaths)
	OtelExporter    string
	OtelServiceName string
	Facade          ports.WorkflowFacade // nil until wired via NewRootCommandWithFacade or main.go
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

// nopRecorder is a no-op ports.Recorder for the CLI-wide read-only facade, which never
// drives execution (the run command keeps the legacy execution path). Subscribe yields an
// already-closed channel so any accidental consumer terminates immediately.
type nopRecorder struct{}

func (nopRecorder) Record(context.Context, transcript.ExchangeEvent) error { //nolint:gocritic // hugeParam: ports.Recorder contract requires value type
	return nil
}

func (nopRecorder) Subscribe() (ch <-chan transcript.ExchangeEvent, cancel func()) {
	c := make(chan transcript.ExchangeEvent)
	close(c)
	return c, func() {}
}

func (nopRecorder) Close() error { return nil }

// buildFacade constructs a CLI-wide ports.WorkflowFacade for the read/validate operations
// (list, history, status, validate). Execution still uses the legacy path, so a no-op
// recorder and a zero ExecutionService suffice. It returns the facade and a cleanup that
// closes the history store. On any setup error it returns (nil, no-op) so callers fall
// back to the legacy path rather than failing.
func buildFacade(cfg *Config) (facade ports.WorkflowFacade, cleanup func()) {
	noop := func() {}

	repo := NewWorkflowRepository()
	discoverer := workflowpkg.NewPackDiscovererAdapter(workflowPackSearchDirs())

	workflowSvc := application.NewWorkflowService(repo, nil, nil, nil, nil)
	workflowSvc.SetPackDiscoverer(discoverer)

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
		nopRecorder{},
		application.NewSessionRegistry(),
	)

	return adapter, func() { _ = historyStore.Close() }
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
