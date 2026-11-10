package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/infrastructure/config"
)

func TestLoadAliases_UserWinsOverProject(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AWF_CONFIG_HOME", dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(`
agents:
  einstein:
    provider: mistral_vibe
`), 0o600))

	project := &config.ProjectConfig{Agents: map[string]config.AgentAlias{
		"einstein":   {Provider: "claude", Options: map[string]any{"model": "opus"}},
		"sklodowska": {Provider: "codex"},
	}}

	got, err := LoadAliases(project)

	require.NoError(t, err)
	assert.Equal(t, map[string]Alias{
		"einstein":   {Provider: "mistral_vibe"},
		"sklodowska": {Provider: "codex"},
	}, got)
}

func TestLoadAliases_NoUserFile(t *testing.T) {
	t.Setenv("AWF_CONFIG_HOME", t.TempDir())
	project := &config.ProjectConfig{Agents: map[string]config.AgentAlias{"einstein": {Provider: "claude"}}}

	got, err := LoadAliases(project)

	require.NoError(t, err)
	assert.Equal(t, map[string]Alias{"einstein": {Provider: "claude"}}, got)
}

func TestLoadAliases_InvalidUserFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AWF_CONFIG_HOME", dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("agents: [not, a, map]"), 0o600))

	_, err := LoadAliases(&config.ProjectConfig{})

	require.Error(t, err)
}

func TestLoadAliases_LocalFileBetweenProjectAndUser(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("AWF_CONFIG_HOME", userDir)
	projectDir := t.TempDir()
	t.Setenv("AWF_CONFIG_PATH", filepath.Join(projectDir, "config.yaml"))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "config.local.yaml"), []byte(`
agents:
  einstein:
    provider: codex
  sklodowska:
    provider: mistral_vibe
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`
agents:
  sklodowska:
    provider: gemini
`), 0o600))
	project := &config.ProjectConfig{Agents: map[string]config.AgentAlias{
		"einstein": {Provider: "claude"},
		"curie":    {Provider: "claude"},
	}}

	got, err := LoadAliases(project)

	require.NoError(t, err)
	assert.Equal(t, map[string]Alias{
		"einstein":   {Provider: "codex"},
		"sklodowska": {Provider: "gemini"},
		"curie":      {Provider: "claude"},
	}, got, "local overrides project, user overrides local")
}

func TestLoadAliases_InvalidLocalFile(t *testing.T) {
	t.Setenv("AWF_CONFIG_HOME", t.TempDir())
	projectDir := t.TempDir()
	t.Setenv("AWF_CONFIG_PATH", filepath.Join(projectDir, "config.yaml"))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "config.local.yaml"), []byte("agents: [x]"), 0o600))

	_, err := LoadAliases(&config.ProjectConfig{})

	require.Error(t, err)
}
