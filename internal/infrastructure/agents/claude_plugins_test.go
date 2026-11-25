package agents

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const installedPlugins = `{
  "version": 2,
  "plugins": {
    "egm@egm": [{"scope": "user", "installPath": "/cache/egm/egm/0.1.0"}],
    "pm@epopee": [{"scope": "project", "projectPath": "/work/epopee", "installPath": "/cache/epopee/pm/1"}],
    "lint@tools": [{"scope": "local", "projectPath": "/work/other", "installPath": "/cache/tools/lint/1"}]
  }
}`

func claudeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "plugins"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugins", "installed_plugins.json"), []byte(content), 0o600))
	return dir
}

func TestClaudePluginRegistry_Installed(t *testing.T) {
	configDir := claudeConfig(t, installedPlugins)
	tests := []struct {
		name       string
		projectDir string
		plugin     string
		want       bool
	}{
		{name: "user plugin by full name", projectDir: "/anywhere", plugin: "egm@egm", want: true},
		{name: "user plugin by name only", projectDir: "/anywhere", plugin: "egm", want: true},
		{name: "project plugin inside its project", projectDir: "/work/epopee/sub", plugin: "pm@epopee", want: true},
		{name: "project plugin outside its project", projectDir: "/work/epopee-two", plugin: "pm@epopee"},
		{name: "local plugin of another project", projectDir: "/work/epopee", plugin: "lint@tools"},
		{name: "other marketplace", projectDir: "/anywhere", plugin: "egm@other"},
		{name: "unknown plugin", projectDir: "/anywhere", plugin: "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installed, err := NewClaudePluginRegistryAt(configDir, tt.projectDir).Installed(context.Background(), "claude", tt.plugin)

			require.NoError(t, err)
			assert.Equal(t, tt.want, installed)
		})
	}
}

func TestClaudePluginRegistry_NoRegistryFileMeansNoPlugin(t *testing.T) {
	installed, err := NewClaudePluginRegistryAt(t.TempDir(), "/anywhere").Installed(context.Background(), "claude", "egm@egm")

	require.NoError(t, err)
	assert.False(t, installed)
}

func TestClaudePluginRegistry_InvalidRegistryFile(t *testing.T) {
	_, err := NewClaudePluginRegistryAt(claudeConfig(t, "{"), "/anywhere").Installed(context.Background(), "claude", "egm@egm")

	assert.ErrorContains(t, err, "parse installed Claude plugins")
}

func TestClaudePluginRegistry_SupportsClaudeOnly(t *testing.T) {
	registry := NewClaudePluginRegistryAt(t.TempDir(), "/anywhere")

	assert.True(t, registry.Supports("claude"))
	assert.False(t, registry.Supports("codex"))
	_, err := registry.Installed(context.Background(), "codex", "x")
	assert.Error(t, err)
}
