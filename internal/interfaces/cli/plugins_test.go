package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPluginPaths_ReturnsCorrectNumberOfPaths(t *testing.T) {
	// Unset env var for test
	t.Setenv("AWF_PLUGINS_PATH", "")

	// Without env var: should return exactly 2 paths (local + global)
	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2, "BuildPluginPaths should return exactly 2 paths when env var not set")
}

func TestBuildPluginPaths_EnvVarTakesPriority(t *testing.T) {
	customPath := "/custom/plugins/path"
	t.Setenv("AWF_PLUGINS_PATH", customPath)

	paths := cli.BuildPluginPaths()

	// AWF_PLUGINS_PATH is exclusive — only the env path is returned
	require.Len(t, paths, 1, "BuildPluginPaths should return 1 path when env var is set")
	assert.Equal(t, repository.SourceEnv, paths[0].Source, "Path should be env source")
	assert.Equal(t, customPath, paths[0].Path, "Path should be env path")
}

func TestBuildPluginPaths_LocalPathSecond(t *testing.T) {
	// Unset env var for test
	t.Setenv("AWF_PLUGINS_PATH", "")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	assert.Equal(t, repository.SourceLocal, paths[0].Source, "First path should be local source when no env")
	assert.Equal(t, xdg.LocalPluginsDir(), paths[0].Path, "First path should be local plugins directory")
}

func TestBuildPluginPaths_GlobalPathLast(t *testing.T) {
	// Unset env var for test
	t.Setenv("AWF_PLUGINS_PATH", "")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	assert.Equal(t, repository.SourceGlobal, paths[1].Source, "Last path should be global source")
	assert.Equal(t, xdg.AWFPluginsDir(), paths[1].Path, "Last path should be XDG plugins directory")
}

func TestBuildPluginPaths_PriorityOrder(t *testing.T) {
	// AWF_PLUGINS_PATH is exclusive — when set, only the env path is returned
	customPath := "/custom/plugins"
	t.Setenv("AWF_PLUGINS_PATH", customPath)

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 1)
	assert.Equal(t, repository.SourceEnv, paths[0].Source)
	assert.Equal(t, customPath, paths[0].Path)
}

func TestBuildPluginPaths_LocalPathIsRelative(t *testing.T) {
	// Unset env var for test
	t.Setenv("AWF_PLUGINS_PATH", "")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	localPath := paths[0].Path

	assert.False(t, filepath.IsAbs(localPath),
		"Local plugins path should be relative, got: %s", localPath)
	assert.Equal(t, ".awf/plugins", localPath,
		"Local plugins path should be .awf/plugins")
}

func TestBuildPluginPaths_GlobalPathIsAbsolute(t *testing.T) {
	// Unset env var for test
	t.Setenv("AWF_PLUGINS_PATH", "")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	globalPath := paths[1].Path

	assert.True(t, filepath.IsAbs(globalPath),
		"Global plugins path should be absolute, got: %s", globalPath)
}

func TestBuildPluginPaths_GlobalPathContainsAwfPlugins(t *testing.T) {
	// Unset env var for test
	t.Setenv("AWF_PLUGINS_PATH", "")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	globalPath := paths[1].Path

	assert.Contains(t, globalPath, "awf",
		"Global path should contain 'awf'")
	assert.True(t, strings.HasSuffix(globalPath, filepath.Join("awf", "plugins")),
		"Global path should end with awf/plugins, got: %s", globalPath)
}

func TestBuildPluginPaths_RespectsXDGDataHome(t *testing.T) {
	// Set custom XDG_DATA_HOME
	customData := "/custom/data/path"
	t.Setenv("XDG_DATA_HOME", customData)
	t.Setenv("AWF_PLUGINS_PATH", "")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	expectedGlobalPath := filepath.Join(customData, "awf", "plugins")
	assert.Equal(t, expectedGlobalPath, paths[1].Path,
		"Global path should respect XDG_DATA_HOME")
}

func TestBuildPluginPaths_DefaultsToLocalShare(t *testing.T) {
	// Unset both env vars
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("AWF_PLUGINS_PATH", "")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	homeDir, _ := os.UserHomeDir()
	expectedGlobalPath := filepath.Join(homeDir, ".local", "share", "awf", "plugins")
	assert.Equal(t, expectedGlobalPath, paths[1].Path,
		"Global path should default to ~/.local/share/awf/plugins")
}

func TestBuildPluginPaths_ConsistentResults(t *testing.T) {
	// Multiple calls should return consistent results
	paths1 := cli.BuildPluginPaths()
	paths2 := cli.BuildPluginPaths()

	require.Equal(t, len(paths1), len(paths2))

	for i := range paths1 {
		assert.Equal(t, paths1[i].Path, paths2[i].Path,
			"Path %d should be consistent across calls", i)
		assert.Equal(t, paths1[i].Source, paths2[i].Source,
			"Source %d should be consistent across calls", i)
	}
}

func TestBuildPluginPaths_SourcedPathStructure(t *testing.T) {
	// Unset env var for test
	t.Setenv("AWF_PLUGINS_PATH", "")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)

	for i, path := range paths {
		// Each path should have non-empty Path field
		assert.NotEmpty(t, path.Path,
			"SourcedPath[%d] should have non-empty Path", i)

		// Source should be a valid Source type
		assert.True(t, path.Source == repository.SourceLocal ||
			path.Source == repository.SourceGlobal ||
			path.Source == repository.SourceEnv,
			"SourcedPath[%d] should have valid Source", i)
	}
}

func TestBuildPluginPaths_EnvVarIsExclusive(t *testing.T) {
	// AWF_PLUGINS_PATH is exclusive — overrides local + global paths.
	// This differs from workflow paths (which merge all sources).
	// Rationale: env var override for plugins means "use only this directory",
	// preventing unexpected plugin discovery from local/global dirs.
	t.Setenv("AWF_PLUGINS_PATH", "/custom/plugins")

	pluginPaths := cli.BuildPluginPaths()

	require.Len(t, pluginPaths, 1)
	assert.Equal(t, repository.SourceEnv, pluginPaths[0].Source)
	assert.Equal(t, "/custom/plugins", pluginPaths[0].Path)
}

func TestBuildPluginPaths_WithoutEnvVar_ReturnsLocalAndGlobal(t *testing.T) {
	// Without env var, both local and global are returned
	t.Setenv("AWF_PLUGINS_PATH", "")

	pluginPaths := cli.BuildPluginPaths()

	require.Len(t, pluginPaths, 2)
	assert.Equal(t, repository.SourceLocal, pluginPaths[0].Source)
	assert.Equal(t, repository.SourceGlobal, pluginPaths[1].Source)
}

func TestBuildPluginPaths_TableDriven(t *testing.T) {
	// Table-driven test for various environment scenarios
	tests := []struct {
		name           string
		xdgDataHome    string
		awfPluginsPath string
		expectedLen    int
		expectLocalDir string
		expectGlobal   func() string
	}{
		{
			name:           "default_xdg_unset",
			xdgDataHome:    "",
			awfPluginsPath: "",
			expectedLen:    2,
			expectLocalDir: ".awf/plugins",
			expectGlobal: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, ".local", "share", "awf", "plugins")
			},
		},
		{
			name:           "custom_xdg_data_home",
			xdgDataHome:    "/tmp/custom-xdg",
			awfPluginsPath: "",
			expectedLen:    2,
			expectLocalDir: ".awf/plugins",
			expectGlobal: func() string {
				return filepath.Join("/tmp/custom-xdg", "awf", "plugins")
			},
		},
		{
			name:           "with_env_var",
			xdgDataHome:    "",
			awfPluginsPath: "/env/plugins",
			expectedLen:    1, // AWF_PLUGINS_PATH is exclusive — overrides local + global
			expectLocalDir: "",
			expectGlobal:   func() string { return "" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars using t.Setenv
			t.Setenv("XDG_DATA_HOME", tt.xdgDataHome)
			t.Setenv("AWF_PLUGINS_PATH", tt.awfPluginsPath)

			paths := cli.BuildPluginPaths()

			require.Len(t, paths, tt.expectedLen)

			// Find local and global paths
			var localPath, globalPath string
			for _, p := range paths {
				if p.Source == repository.SourceLocal {
					localPath = p.Path
				}
				if p.Source == repository.SourceGlobal {
					globalPath = p.Path
				}
			}

			assert.Equal(t, tt.expectLocalDir, localPath)
			assert.Equal(t, tt.expectGlobal(), globalPath)
		})
	}
}

func TestConfig_PluginsDirField(t *testing.T) {
	cfg := cli.DefaultConfig()

	// PluginsDir should be empty by default (use BuildPluginPaths)
	assert.Empty(t, cfg.PluginsDir,
		"PluginsDir should be empty by default")
}

func TestConfig_PluginsDirOverride(t *testing.T) {
	cfg := cli.DefaultConfig()
	cfg.PluginsDir = "/custom/override/plugins"

	assert.Equal(t, "/custom/override/plugins", cfg.PluginsDir,
		"PluginsDir should be settable")
}
