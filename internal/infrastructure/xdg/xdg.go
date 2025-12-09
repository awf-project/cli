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
