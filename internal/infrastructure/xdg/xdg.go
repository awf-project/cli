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

// AWFConfigDir returns the AWF config directory.
// Checks $AWF_CONFIG_HOME first, then falls back to $XDG_CONFIG_HOME/awf.
func AWFConfigDir() string {
	if dir := os.Getenv("AWF_CONFIG_HOME"); dir != "" {
		return dir
	}
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
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	legacyDir := filepath.Join(home, ".awf")
	_, statErr := os.Stat(legacyDir)
	return statErr == nil
}

// LocalWorkflowsDir returns the local project workflows directory
func LocalWorkflowsDir() string {
	return ".awf/workflows"
}

// LocalPromptsDir returns the local project prompts directory
func LocalPromptsDir() string {
	return ".awf/prompts"
}

// AWFSkillsDir returns the global skills directory ($XDG_CONFIG_HOME/awf/skills)
func AWFSkillsDir() string {
	return filepath.Join(AWFConfigDir(), "skills")
}

// LocalSkillsDir returns the local project skills directory
func LocalSkillsDir() string {
	return ".awf/skills"
}

// AWFAgentsDir returns the global agents directory ($XDG_CONFIG_HOME/awf/agents)
func AWFAgentsDir() string {
	return filepath.Join(AWFConfigDir(), "agents")
}

// LocalAgentsDir returns the local project agents directory
func LocalAgentsDir() string {
	return ".awf/agents"
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

// AWFPaths returns all AWF XDG directory paths as a map for template interpolation.
// Keys: prompts_dir, scripts_dir, config_dir, data_dir, workflows_dir, plugins_dir, skills_dir.
func AWFPaths() map[string]string {
	return map[string]string{
		"prompts_dir":   AWFPromptsDir(),
		"scripts_dir":   AWFScriptsDir(),
		"config_dir":    AWFConfigDir(),
		"data_dir":      AWFDataDir(),
		"workflows_dir": AWFWorkflowsDir(),
		"plugins_dir":   AWFPluginsDir(),
		"skills_dir":    AWFSkillsDir(),
	}
}

// PackAWFPaths returns AWF paths with pack_name set for pack context.
func PackAWFPaths(packName string) map[string]string {
	paths := AWFPaths()
	paths["pack_name"] = packName
	return paths
}

// LocalConfigPath returns the local project config file path.
// It checks AWF_CONFIG_PATH environment variable first, then defaults to ".awf/config.yaml".
func LocalConfigPath() string {
	if path := os.Getenv("AWF_CONFIG_PATH"); path != "" {
		return path
	}
	return ".awf/config.yaml"
}
