package agents

import (
	"path/filepath"

	"github.com/awf-project/cli/internal/infrastructure/config"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
)

// localConfigPath is the personal, uncommitted counterpart of the project config:
// config.local.yaml next to .awf/config.yaml. Only its agents key is read.
func localConfigPath() string {
	return filepath.Join(filepath.Dir(xdg.LocalConfigPath()), "config.local.yaml")
}

// LoadAliases returns the agent aliases of three files, in increasing priority: the project
// configuration (.awf/config.yaml, already loaded as projectCfg, which may be nil), the local
// project configuration (.awf/config.local.yaml) and the user configuration
// ($AWF_CONFIG_HOME or $XDG_CONFIG_HOME/awf/config.yaml). For the same alias name, the entry of
// the higher-priority file wins. A missing file is not an error.
func LoadAliases(projectCfg *config.ProjectConfig) (map[string]Alias, error) {
	if projectCfg == nil {
		projectCfg = &config.ProjectConfig{}
	}
	localCfg, err := config.NewYAMLConfigLoader(localConfigPath()).Load()
	if err != nil {
		return nil, err
	}
	userCfg, err := config.NewYAMLConfigLoader(filepath.Join(xdg.AWFConfigDir(), "config.yaml")).Load()
	if err != nil {
		return nil, err
	}
	aliases := make(map[string]Alias, len(projectCfg.Agents)+len(localCfg.Agents)+len(userCfg.Agents))
	for _, entries := range []map[string]config.AgentAlias{projectCfg.Agents, localCfg.Agents, userCfg.Agents} {
		for name, a := range entries {
			aliases[name] = Alias{Provider: a.Provider, Options: a.Options}
		}
	}
	return aliases, nil
}
