package xdg

import (
	"os"
	"path/filepath"
)

// ConfigHome returns XDG_CONFIG_HOME or defaults to ~/.config
func ConfigHome() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// DataHome returns XDG_DATA_HOME or defaults to ~/.local/share
func DataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

// AWFConfigDir returns the AWF config directory ($XDG_CONFIG_HOME/awf)
func AWFConfigDir() string {
	return filepath.Join(ConfigHome(), "awf")
}

// AWFDataDir returns the AWF data directory ($XDG_DATA_HOME/awf)
func AWFDataDir() string {
	return filepath.Join(DataHome(), "awf")
}

// AWFWorkflowsDir returns the global workflows directory
func AWFWorkflowsDir() string {
	return filepath.Join(AWFConfigDir(), "workflows")
}

// AWFPromptsDir returns the global prompts directory ($XDG_CONFIG_HOME/awf/prompts)
func AWFPromptsDir() string {
	return filepath.Join(AWFConfigDir(), "prompts")
}

// AWFScriptsDir returns the global scripts directory ($XDG_CONFIG_HOME/awf/scripts)
func AWFScriptsDir() string {
	return filepath.Join(AWFConfigDir(), "scripts")
}

// AWFStatesDir returns the states storage directory
func AWFStatesDir() string {
	return filepath.Join(AWFDataDir(), "states")
}

// AWFLogsDir returns the logs storage directory
func AWFLogsDir() string {
	return filepath.Join(AWFDataDir(), "logs")
}

// LegacyDirExists checks if the old ~/.awf directory exists
func LegacyDirExists() bool {
	home, _ := os.UserHomeDir()
	legacyDir := filepath.Join(home, ".awf")
	_, err := os.Stat(legacyDir)
	return err == nil
}

// LocalWorkflowsDir returns the local project workflows directory
func LocalWorkflowsDir() string {
	return ".awf/workflows"
}

// LocalPromptsDir returns the local project prompts directory
func LocalPromptsDir() string {
	return ".awf/prompts"
}

// AWFPluginsDir returns the global plugins directory ($XDG_DATA_HOME/awf/plugins)
func AWFPluginsDir() string {
	return filepath.Join(AWFDataDir(), "plugins")
}

// LocalPluginsDir returns the local project plugins directory
func LocalPluginsDir() string {
	return ".awf/plugins"
}

// AWFWorkflowPacksDir returns the global workflow packs directory ($XDG_DATA_HOME/awf/workflow-packs)
func AWFWorkflowPacksDir() string {
	return filepath.Join(AWFDataDir(), "workflow-packs")
}

// LocalWorkflowPacksDir returns the local project workflow packs directory
func LocalWorkflowPacksDir() string {
	return ".awf/workflow-packs"
}

// LocalConfigPath returns the local project config file path.
// It checks AWF_CONFIG_PATH environment variable first, then defaults to ".awf/config.yaml".
func LocalConfigPath() string {
	if path := os.Getenv("AWF_CONFIG_PATH"); path != "" {
		return path
	}
	return ".awf/config.yaml"
}
