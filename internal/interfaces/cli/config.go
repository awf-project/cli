package cli

import (
	"fmt"
	"os"

	"github.com/awf-project/cli/internal/infrastructure/repository"
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
	paths = append(paths,
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
	paths = append(paths,
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
