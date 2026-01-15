package xdg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "uses XDG_CONFIG_HOME when set",
			envValue: "/custom/config",
			want:     "/custom/config",
		},
		{
			name:     "defaults to ~/.config when unset",
			envValue: "",
			want:     filepath.Join(home, ".config"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			if tt.envValue != "" {
				t.Setenv("XDG_CONFIG_HOME", tt.envValue)
			} else {
				t.Setenv("XDG_CONFIG_HOME", "")
			}

			got := ConfigHome()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDataHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "uses XDG_DATA_HOME when set",
			envValue: "/custom/data",
			want:     "/custom/data",
		},
		{
			name:     "defaults to ~/.local/share when unset",
			envValue: "",
			want:     filepath.Join(home, ".local", "share"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("XDG_DATA_HOME", tt.envValue)
			} else {
				t.Setenv("XDG_DATA_HOME", "")
			}

			got := DataHome()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAWFConfigDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Setenv("XDG_CONFIG_HOME", "")
	got := AWFConfigDir()
	assert.Equal(t, filepath.Join(home, ".config", "awf"), got)
}

func TestAWFDataDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Setenv("XDG_DATA_HOME", "")
	got := AWFDataDir()
	assert.Equal(t, filepath.Join(home, ".local", "share", "awf"), got)
}

func TestAWFWorkflowsDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Setenv("XDG_CONFIG_HOME", "")
	got := AWFWorkflowsDir()
	assert.Equal(t, filepath.Join(home, ".config", "awf", "workflows"), got)
}

func TestAWFPromptsDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "uses XDG_CONFIG_HOME when set",
			envValue: "/custom/config",
			want:     "/custom/config/awf/prompts",
		},
		{
			name:     "defaults to ~/.config/awf/prompts when unset",
			envValue: "",
			want:     filepath.Join(home, ".config", "awf", "prompts"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("XDG_CONFIG_HOME", tt.envValue)
			} else {
				t.Setenv("XDG_CONFIG_HOME", "")
			}

			got := AWFPromptsDir()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAWFStatesDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Setenv("XDG_DATA_HOME", "")
	got := AWFStatesDir()
	assert.Equal(t, filepath.Join(home, ".local", "share", "awf", "states"), got)
}

func TestAWFLogsDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Setenv("XDG_DATA_HOME", "")
	got := AWFLogsDir()
	assert.Equal(t, filepath.Join(home, ".local", "share", "awf", "logs"), got)
}

func TestLegacyDirExists(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	legacyDir := filepath.Join(home, ".awf")

	// Check returns correct value based on actual state
	exists := LegacyDirExists()
	_, err = os.Stat(legacyDir)
	if err == nil {
		assert.True(t, exists, "should return true when ~/.awf exists")
	} else {
		assert.False(t, exists, "should return false when ~/.awf doesn't exist")
	}
}

func TestLocalWorkflowsDir(t *testing.T) {
	got := LocalWorkflowsDir()
	assert.Equal(t, ".awf/workflows", got)
}

func TestLocalPromptsDir(t *testing.T) {
	got := LocalPromptsDir()
	assert.Equal(t, ".awf/prompts", got)
}

// =============================================================================
// Plugin Directory Tests (T014)
// =============================================================================

func TestAWFPluginsDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "uses XDG_DATA_HOME when set",
			envValue: "/custom/data",
			want:     "/custom/data/awf/plugins",
		},
		{
			name:     "defaults to ~/.local/share/awf/plugins when unset",
			envValue: "",
			want:     filepath.Join(home, ".local", "share", "awf", "plugins"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("XDG_DATA_HOME", tt.envValue)
			} else {
				t.Setenv("XDG_DATA_HOME", "")
			}

			got := AWFPluginsDir()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAWFPluginsDir_IsUnderDataDir(t *testing.T) {
	// AWFPluginsDir should be under AWFDataDir
	t.Setenv("XDG_DATA_HOME", "")

	dataDir := AWFDataDir()
	pluginsDir := AWFPluginsDir()

	// Use strings.HasPrefix instead of deprecated filepath.HasPrefix
	assert.True(t, strings.HasPrefix(pluginsDir, dataDir),
		"AWFPluginsDir (%s) should be under AWFDataDir (%s)", pluginsDir, dataDir)
}

func TestAWFPluginsDir_EndsWithPlugins(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")

	got := AWFPluginsDir()

	assert.True(t, filepath.Base(got) == "plugins",
		"AWFPluginsDir should end with 'plugins', got: %s", got)
}

func TestLocalPluginsDir(t *testing.T) {
	got := LocalPluginsDir()
	assert.Equal(t, ".awf/plugins", got)
}

func TestLocalPluginsDir_IsRelative(t *testing.T) {
	got := LocalPluginsDir()

	assert.False(t, filepath.IsAbs(got),
		"LocalPluginsDir should be relative, got: %s", got)
}

func TestLocalPluginsDir_MirrorsLocalWorkflowsPattern(t *testing.T) {
	// LocalPluginsDir should follow same pattern as LocalWorkflowsDir
	workflowsDir := LocalWorkflowsDir()
	pluginsDir := LocalPluginsDir()

	// Both should be under .awf/
	assert.True(t, strings.HasPrefix(workflowsDir, ".awf/"),
		"LocalWorkflowsDir should be under .awf/")
	assert.True(t, strings.HasPrefix(pluginsDir, ".awf/"),
		"LocalPluginsDir should be under .awf/")

	// Both should have similar structure: .awf/<type>
	assert.Equal(t, ".awf/workflows", workflowsDir)
	assert.Equal(t, ".awf/plugins", pluginsDir)
}

func TestPluginsDirs_ConsistentWithOtherDirs(t *testing.T) {
	// Plugin dirs should follow same XDG patterns as other dirs

	// Test with custom XDG_DATA_HOME
	customPath := "/custom/data/path"
	t.Setenv("XDG_DATA_HOME", customPath)

	statesDir := AWFStatesDir()
	logsDir := AWFLogsDir()
	pluginsDir := AWFPluginsDir()

	// All should use the same base
	assert.Equal(t, filepath.Join(customPath, "awf", "states"), statesDir)
	assert.Equal(t, filepath.Join(customPath, "awf", "logs"), logsDir)
	assert.Equal(t, filepath.Join(customPath, "awf", "plugins"), pluginsDir)
}

func TestPluginsDirs_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		xdgDataHome string
		wantGlobal  func() string
		wantLocal   string
	}{
		{
			name:        "default XDG unset",
			xdgDataHome: "",
			wantGlobal: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, ".local", "share", "awf", "plugins")
			},
			wantLocal: ".awf/plugins",
		},
		{
			name:        "custom XDG_DATA_HOME",
			xdgDataHome: "/opt/awf/data",
			wantGlobal: func() string {
				return "/opt/awf/data/awf/plugins"
			},
			wantLocal: ".awf/plugins",
		},
		{
			name:        "XDG with trailing slash",
			xdgDataHome: "/custom/data/",
			wantGlobal: func() string {
				return filepath.Join("/custom", "data", "awf", "plugins")
			},
			wantLocal: ".awf/plugins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.xdgDataHome != "" {
				t.Setenv("XDG_DATA_HOME", tt.xdgDataHome)
			} else {
				t.Setenv("XDG_DATA_HOME", "")
			}

			assert.Equal(t, tt.wantGlobal(), AWFPluginsDir())
			assert.Equal(t, tt.wantLocal, LocalPluginsDir())
		})
	}
}

// =============================================================================
// Local Config Path Tests (T004 - F036)
// =============================================================================

func TestLocalConfigPath(t *testing.T) {
	got := LocalConfigPath()
	assert.Equal(t, ".awf/config.yaml", got)
}

func TestLocalConfigPath_IsRelative(t *testing.T) {
	got := LocalConfigPath()

	assert.False(t, filepath.IsAbs(got),
		"LocalConfigPath should be relative, got: %s", got)
}

func TestLocalConfigPath_IsUnderAWFDir(t *testing.T) {
	got := LocalConfigPath()

	assert.True(t, strings.HasPrefix(got, ".awf/"),
		"LocalConfigPath should be under .awf/, got: %s", got)
}

func TestLocalConfigPath_HasYAMLExtension(t *testing.T) {
	got := LocalConfigPath()

	assert.Equal(t, ".yaml", filepath.Ext(got),
		"LocalConfigPath should have .yaml extension, got: %s", filepath.Ext(got))
}

func TestLocalConfigPath_IsConfigFile(t *testing.T) {
	got := LocalConfigPath()

	// Extract filename without extension
	base := filepath.Base(got)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	assert.Equal(t, "config", name,
		"LocalConfigPath filename should be 'config', got: %s", name)
}

func TestLocalConfigPath_ConsistentWithLocalDirs(t *testing.T) {
	// LocalConfigPath should follow same .awf/ pattern as other local paths
	configPath := LocalConfigPath()
	workflowsDir := LocalWorkflowsDir()
	promptsDir := LocalPromptsDir()
	pluginsDir := LocalPluginsDir()

	// All should be under .awf/
	assert.True(t, strings.HasPrefix(configPath, ".awf/"),
		"LocalConfigPath should be under .awf/")
	assert.True(t, strings.HasPrefix(workflowsDir, ".awf/"),
		"LocalWorkflowsDir should be under .awf/")
	assert.True(t, strings.HasPrefix(promptsDir, ".awf/"),
		"LocalPromptsDir should be under .awf/")
	assert.True(t, strings.HasPrefix(pluginsDir, ".awf/"),
		"LocalPluginsDir should be under .awf/")
}

func TestLocalConfigPath_DoesNotDependOnEnv(t *testing.T) {
	// LocalConfigPath should always return the same value regardless of XDG env vars
	tests := []struct {
		name          string
		xdgConfigHome string
		xdgDataHome   string
	}{
		{
			name:          "no XDG vars set",
			xdgConfigHome: "",
			xdgDataHome:   "",
		},
		{
			name:          "XDG_CONFIG_HOME set",
			xdgConfigHome: "/custom/config",
			xdgDataHome:   "",
		},
		{
			name:          "XDG_DATA_HOME set",
			xdgConfigHome: "",
			xdgDataHome:   "/custom/data",
		},
		{
			name:          "both XDG vars set",
			xdgConfigHome: "/custom/config",
			xdgDataHome:   "/custom/data",
		},
	}

	expected := ".awf/config.yaml"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			defer func() {
			}()
			if tt.xdgConfigHome != "" {
				t.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)
			} else {
				t.Setenv("XDG_CONFIG_HOME", "")
			}

			if tt.xdgDataHome != "" {
				t.Setenv("XDG_DATA_HOME", tt.xdgDataHome)
			} else {
				t.Setenv("XDG_DATA_HOME", "")
			}

			got := LocalConfigPath()
			assert.Equal(t, expected, got,
				"LocalConfigPath should always return %s regardless of env vars", expected)
		})
	}
}

func TestLocalConfigPath_DirectoryAndFileSeparation(t *testing.T) {
	got := LocalConfigPath()

	dir := filepath.Dir(got)
	file := filepath.Base(got)

	assert.Equal(t, ".awf", dir,
		"LocalConfigPath directory should be .awf, got: %s", dir)
	assert.Equal(t, "config.yaml", file,
		"LocalConfigPath file should be config.yaml, got: %s", file)
}
