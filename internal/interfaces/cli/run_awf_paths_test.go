package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAWFPaths_ContainsScriptsDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	paths := xdg.AWFPaths()

	_, exists := paths["scripts_dir"]
	assert.True(t, exists, "xdg.AWFPaths() should include 'scripts_dir' key")
}

func TestBuildAWFPaths_ScriptsDirPointsToCorrectLocation(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Setenv("XDG_CONFIG_HOME", "")

	paths := xdg.AWFPaths()

	expected := filepath.Join(home, ".config", "awf", "scripts")
	assert.Equal(t, expected, paths["scripts_dir"],
		"scripts_dir should point to ~/.config/awf/scripts by default")
}

func TestBuildAWFPaths_ScriptsDirRespectsXDGConfigHome(t *testing.T) {
	customConfig := "/custom/config"
	t.Setenv("XDG_CONFIG_HOME", customConfig)

	paths := xdg.AWFPaths()

	expected := filepath.Join(customConfig, "awf", "scripts")
	assert.Equal(t, expected, paths["scripts_dir"],
		"scripts_dir should respect XDG_CONFIG_HOME environment variable")
}

func TestBuildAWFPaths_ScriptsDirIsSiblingToPromptsDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	paths := xdg.AWFPaths()

	scriptsDir := paths["scripts_dir"]
	promptsDir := paths["prompts_dir"]

	scriptsParent := filepath.Dir(scriptsDir)
	promptsParent := filepath.Dir(promptsDir)

	assert.Equal(t, promptsParent, scriptsParent,
		"scripts_dir and prompts_dir should be siblings under same parent directory")
}

func TestBuildAWFPaths_AllRequiredKeysPresent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	paths := xdg.AWFPaths()

	requiredKeys := []string{
		"prompts_dir",
		"scripts_dir",
		"config_dir",
		"data_dir",
		"workflows_dir",
		"plugins_dir",
	}

	for _, key := range requiredKeys {
		_, exists := paths[key]
		assert.True(t, exists, "xdg.AWFPaths() should include '%s' key", key)
	}
}

func TestBuildAWFPaths_ScriptsDirIsAbsolutePath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	paths := xdg.AWFPaths()

	scriptsDir := paths["scripts_dir"]
	assert.True(t, filepath.IsAbs(scriptsDir),
		"scripts_dir should be an absolute path, got: %s", scriptsDir)
}

func TestBuildAWFPaths_ScriptsDirConsistentAcrossMultipleCalls(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/stable/config")

	paths1 := xdg.AWFPaths()
	paths2 := xdg.AWFPaths()

	assert.Equal(t, paths1["scripts_dir"], paths2["scripts_dir"],
		"scripts_dir should be consistent across multiple calls to xdg.AWFPaths()")
}
