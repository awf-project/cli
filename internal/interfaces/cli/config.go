package cli

import (
	"fmt"
	"os"

	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/infrastructure/xdg"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
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
	Verbose      bool
	Quiet        bool
	NoColor      bool
	OutputMode   OutputMode
	OutputFormat ui.OutputFormat
	LogLevel     string
	ConfigPath   string
	StoragePath  string
	PluginsDir   string // Override plugin discovery directory (empty = use BuildPluginPaths)
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		Verbose:      false,
		Quiet:        false,
		NoColor:      false,
		OutputMode:   OutputSilent,
		OutputFormat: ui.FormatText,
		LogLevel:     "info",
		ConfigPath:   "",
		StoragePath:  xdg.AWFDataDir(),
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

	// 2. Local project directory
	paths = append(paths, repository.SourcedPath{
		Path:   xdg.LocalWorkflowsDir(),
		Source: repository.SourceLocal,
	})

	// 3. Global XDG directory (lowest priority)
	paths = append(paths, repository.SourcedPath{
		Path:   xdg.AWFWorkflowsDir(),
		Source: repository.SourceGlobal,
	})

	return paths
}

// NewWorkflowRepository creates a CompositeRepository with standard paths
func NewWorkflowRepository() *repository.CompositeRepository {
	return repository.NewCompositeRepository(BuildWorkflowPaths())
}

// BuildPromptPaths returns the prompt paths in priority order:
// 1. ./.awf/prompts/ (local project)
// 2. $XDG_CONFIG_HOME/awf/prompts/ (global)
func BuildPromptPaths() []repository.SourcedPath {
	return []repository.SourcedPath{
		{
			Path:   xdg.LocalPromptsDir(),
			Source: repository.SourceLocal,
		},
		{
			Path:   xdg.AWFPromptsDir(),
			Source: repository.SourceGlobal,
		},
	}
}

// BuildPluginPaths returns plugin paths in priority order:
// 1. AWF_PLUGINS_PATH env var
// 2. ./.awf/plugins/ (local project)
// 3. $XDG_DATA_HOME/awf/plugins/ (global)
func BuildPluginPaths() []repository.SourcedPath {
	var paths []repository.SourcedPath

	// 1. Environment variable (highest priority)
	if envPath := os.Getenv("AWF_PLUGINS_PATH"); envPath != "" {
		paths = append(paths, repository.SourcedPath{
			Path:   envPath,
			Source: repository.SourceEnv,
		})
	}

	// 2. Local project directory
	paths = append(paths, repository.SourcedPath{
		Path:   xdg.LocalPluginsDir(),
		Source: repository.SourceLocal,
	})

	// 3. Global XDG data directory (lowest priority)
	paths = append(paths, repository.SourcedPath{
		Path:   xdg.AWFPluginsDir(),
		Source: repository.SourceGlobal,
	})

	return paths
}
