package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudePluginRegistry reads the plugins installed in Claude Code from plugins/installed_plugins.json of the Claude
// configuration directory (CLAUDE_CONFIG_DIR, else ~/.claude). It implements ports.AgentPluginRegistry.
type ClaudePluginRegistry struct {
	configDir  string
	projectDir string
}

// NewClaudePluginRegistry returns a registry for the Claude configuration directory of the environment, and the
// current directory as the project that project and local installations must belong to.
func NewClaudePluginRegistry() *ClaudePluginRegistry {
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			configDir = filepath.Join(home, ".claude")
		}
	}
	projectDir, _ := os.Getwd()
	return NewClaudePluginRegistryAt(configDir, projectDir)
}

// NewClaudePluginRegistryAt returns a registry for the given Claude configuration directory and project directory.
func NewClaudePluginRegistryAt(configDir, projectDir string) *ClaudePluginRegistry {
	return &ClaudePluginRegistry{configDir: configDir, projectDir: projectDir}
}

// Supports reports whether the provider is claude, the only agent CLI whose plugins the registry reads.
func (r *ClaudePluginRegistry) Supports(provider string) bool {
	return provider == "claude"
}

type claudeInstalledPlugins struct {
	Plugins map[string][]struct {
		Scope       string `json:"scope"`
		ProjectPath string `json:"projectPath"`
		InstallPath string `json:"installPath"`
	} `json:"plugins"`
}

// Installed reports whether the plugin, written "name@marketplace" or "name", is installed for the user, or for a
// project that contains the current directory. A missing installed_plugins.json means that no plugin is installed.
func (r *ClaudePluginRegistry) Installed(_ context.Context, provider, plugin string) (bool, error) {
	if !r.Supports(provider) {
		return false, fmt.Errorf("agent provider %q is not supported", provider)
	}
	data, err := os.ReadFile(filepath.Join(r.configDir, "plugins", "installed_plugins.json"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read installed Claude plugins: %w", err)
	}
	var installed claudeInstalledPlugins
	if err := json.Unmarshal(data, &installed); err != nil {
		return false, fmt.Errorf("parse installed Claude plugins: %w", err)
	}
	for key, entries := range installed.Plugins {
		if key != plugin && !strings.HasPrefix(key, plugin+"@") {
			continue
		}
		for _, entry := range entries {
			if entry.Scope == "user" || r.inProject(entry.ProjectPath) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *ClaudePluginRegistry) inProject(projectPath string) bool {
	if projectPath == "" || r.projectDir == "" {
		return false
	}
	rel, err := filepath.Rel(projectPath, r.projectDir)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
