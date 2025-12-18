package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/infrastructure/xdg"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// =============================================================================
// BuildPluginPaths Tests (T014)
// =============================================================================

func TestBuildPluginPaths_ReturnsCorrectNumberOfPaths(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalEnv != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalEnv)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()
	os.Unsetenv("AWF_PLUGINS_PATH")

	// Without env var: should return exactly 2 paths (local + global)
	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2, "BuildPluginPaths should return exactly 2 paths when env var not set")
}

func TestBuildPluginPaths_EnvVarTakesPriority(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalEnv != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalEnv)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()

	// Set custom env var
	customPath := "/custom/plugins/path"
	os.Setenv("AWF_PLUGINS_PATH", customPath)

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 3, "BuildPluginPaths should return 3 paths when env var is set")
	assert.Equal(t, repository.SourceEnv, paths[0].Source, "First path should be env source")
	assert.Equal(t, customPath, paths[0].Path, "First path should be env path")
}

func TestBuildPluginPaths_LocalPathSecond(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalEnv != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalEnv)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()
	os.Unsetenv("AWF_PLUGINS_PATH")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	assert.Equal(t, repository.SourceLocal, paths[0].Source, "First path should be local source when no env")
	assert.Equal(t, xdg.LocalPluginsDir(), paths[0].Path, "First path should be local plugins directory")
}

func TestBuildPluginPaths_GlobalPathLast(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalEnv != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalEnv)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()
	os.Unsetenv("AWF_PLUGINS_PATH")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	assert.Equal(t, repository.SourceGlobal, paths[1].Source, "Last path should be global source")
	assert.Equal(t, xdg.AWFPluginsDir(), paths[1].Path, "Last path should be XDG plugins directory")
}

func TestBuildPluginPaths_PriorityOrder(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalEnv != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalEnv)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()

	customPath := "/custom/plugins"
	os.Setenv("AWF_PLUGINS_PATH", customPath)

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 3)

	// Verify complete ordering: env -> local -> global
	expectedOrder := []struct {
		source repository.Source
		path   string
	}{
		{repository.SourceEnv, customPath},
		{repository.SourceLocal, xdg.LocalPluginsDir()},
		{repository.SourceGlobal, xdg.AWFPluginsDir()},
	}

	for i, expected := range expectedOrder {
		assert.Equal(t, expected.source, paths[i].Source,
			"Path %d should have source %v", i, expected.source)
		assert.Equal(t, expected.path, paths[i].Path,
			"Path %d should have path %s", i, expected.path)
	}
}

func TestBuildPluginPaths_LocalPathIsRelative(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalEnv != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalEnv)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()
	os.Unsetenv("AWF_PLUGINS_PATH")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	localPath := paths[0].Path

	assert.False(t, filepath.IsAbs(localPath),
		"Local plugins path should be relative, got: %s", localPath)
	assert.Equal(t, ".awf/plugins", localPath,
		"Local plugins path should be .awf/plugins")
}

func TestBuildPluginPaths_GlobalPathIsAbsolute(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalEnv != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalEnv)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()
	os.Unsetenv("AWF_PLUGINS_PATH")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	globalPath := paths[1].Path

	assert.True(t, filepath.IsAbs(globalPath),
		"Global plugins path should be absolute, got: %s", globalPath)
}

func TestBuildPluginPaths_GlobalPathContainsAwfPlugins(t *testing.T) {
	// Save and restore environment
	originalEnv := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalEnv != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalEnv)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()
	os.Unsetenv("AWF_PLUGINS_PATH")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	globalPath := paths[1].Path

	assert.Contains(t, globalPath, "awf",
		"Global path should contain 'awf'")
	assert.True(t, strings.HasSuffix(globalPath, filepath.Join("awf", "plugins")),
		"Global path should end with awf/plugins, got: %s", globalPath)
}

func TestBuildPluginPaths_RespectsXDGDataHome(t *testing.T) {
	// Save and restore original env
	originalXDG := os.Getenv("XDG_DATA_HOME")
	originalPlugins := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalXDG != "" {
			os.Setenv("XDG_DATA_HOME", originalXDG)
		} else {
			os.Unsetenv("XDG_DATA_HOME")
		}
		if originalPlugins != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalPlugins)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()

	// Set custom XDG_DATA_HOME
	customData := "/custom/data/path"
	os.Setenv("XDG_DATA_HOME", customData)
	os.Unsetenv("AWF_PLUGINS_PATH")

	paths := cli.BuildPluginPaths()

	require.Len(t, paths, 2)
	expectedGlobalPath := filepath.Join(customData, "awf", "plugins")
	assert.Equal(t, expectedGlobalPath, paths[1].Path,
		"Global path should respect XDG_DATA_HOME")
}

func TestBuildPluginPaths_DefaultsToLocalShare(t *testing.T) {
	// Save and restore original env
	originalXDG := os.Getenv("XDG_DATA_HOME")
	originalPlugins := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalXDG != "" {
			os.Setenv("XDG_DATA_HOME", originalXDG)
		} else {
			os.Unsetenv("XDG_DATA_HOME")
		}
		if originalPlugins != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalPlugins)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()

	// Unset both env vars
	os.Unsetenv("XDG_DATA_HOME")
	os.Unsetenv("AWF_PLUGINS_PATH")

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
	// Save and restore environment
	originalEnv := os.Getenv("AWF_PLUGINS_PATH")
	defer func() {
		if originalEnv != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalEnv)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
	}()
	os.Unsetenv("AWF_PLUGINS_PATH")

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

func TestBuildPluginPaths_MirrorsWorkflowPathsPattern(t *testing.T) {
	// BuildPluginPaths follows same pattern as BuildWorkflowPaths
	// Both should support env var (if set), local, global in same order

	// Save and restore environment
	originalPlugins := os.Getenv("AWF_PLUGINS_PATH")
	originalWorkflows := os.Getenv("AWF_WORKFLOWS_PATH")
	defer func() {
		if originalPlugins != "" {
			os.Setenv("AWF_PLUGINS_PATH", originalPlugins)
		} else {
			os.Unsetenv("AWF_PLUGINS_PATH")
		}
		if originalWorkflows != "" {
			os.Setenv("AWF_WORKFLOWS_PATH", originalWorkflows)
		} else {
			os.Unsetenv("AWF_WORKFLOWS_PATH")
		}
	}()

	// Set both env vars
	os.Setenv("AWF_PLUGINS_PATH", "/custom/plugins")
	os.Setenv("AWF_WORKFLOWS_PATH", "/custom/workflows")

	pluginPaths := cli.BuildPluginPaths()
	workflowPaths := cli.BuildWorkflowPaths()

	// Both should have 3 paths when env vars are set
	require.Len(t, pluginPaths, 3)
	require.Len(t, workflowPaths, 3)

	// Both should have same source order: env -> local -> global
	assert.Equal(t, repository.SourceEnv, pluginPaths[0].Source)
	assert.Equal(t, repository.SourceEnv, workflowPaths[0].Source)

	assert.Equal(t, repository.SourceLocal, pluginPaths[1].Source)
	assert.Equal(t, repository.SourceLocal, workflowPaths[1].Source)

	assert.Equal(t, repository.SourceGlobal, pluginPaths[2].Source)
	assert.Equal(t, repository.SourceGlobal, workflowPaths[2].Source)
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
			expectedLen:    3,
			expectLocalDir: ".awf/plugins",
			expectGlobal: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, ".local", "share", "awf", "plugins")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			originalXDG := os.Getenv("XDG_DATA_HOME")
			originalPlugins := os.Getenv("AWF_PLUGINS_PATH")
			defer func() {
				if originalXDG != "" {
					os.Setenv("XDG_DATA_HOME", originalXDG)
				} else {
					os.Unsetenv("XDG_DATA_HOME")
				}
				if originalPlugins != "" {
					os.Setenv("AWF_PLUGINS_PATH", originalPlugins)
				} else {
					os.Unsetenv("AWF_PLUGINS_PATH")
				}
			}()

			if tt.xdgDataHome != "" {
				os.Setenv("XDG_DATA_HOME", tt.xdgDataHome)
			} else {
				os.Unsetenv("XDG_DATA_HOME")
			}

			if tt.awfPluginsPath != "" {
				os.Setenv("AWF_PLUGINS_PATH", tt.awfPluginsPath)
			} else {
				os.Unsetenv("AWF_PLUGINS_PATH")
			}

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

// =============================================================================
// Config.PluginsDir Tests (T014)
// =============================================================================

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
