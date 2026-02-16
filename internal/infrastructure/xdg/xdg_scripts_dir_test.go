package xdg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWFScriptsDir(t *testing.T) {
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
			want:     "/custom/config/awf/scripts",
		},
		{
			name:     "defaults to ~/.config/awf/scripts when unset",
			envValue: "",
			want:     filepath.Join(home, ".config", "awf", "scripts"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("XDG_CONFIG_HOME", tt.envValue)
			} else {
				t.Setenv("XDG_CONFIG_HOME", "")
			}

			got := AWFScriptsDir()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAWFScriptsDir_IsUnderConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	configDir := AWFConfigDir()
	scriptsDir := AWFScriptsDir()

	assert.True(t, strings.HasPrefix(scriptsDir, configDir),
		"AWFScriptsDir (%s) should be under AWFConfigDir (%s)", scriptsDir, configDir)
}

func TestAWFScriptsDir_EndsWithScripts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	got := AWFScriptsDir()

	assert.True(t, filepath.Base(got) == "scripts",
		"AWFScriptsDir should end with 'scripts', got: %s", got)
}

func TestAWFScriptsDir_MirrorsPromptsPattern(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	promptsDir := AWFPromptsDir()
	scriptsDir := AWFScriptsDir()

	promptsBase := filepath.Dir(promptsDir)
	scriptsBase := filepath.Dir(scriptsDir)

	assert.Equal(t, promptsBase, scriptsBase,
		"AWFScriptsDir and AWFPromptsDir should be siblings under same parent directory")
}

func TestScriptsDir_ConsistentWithOtherDirs(t *testing.T) {
	customPath := "/custom/config/path"
	t.Setenv("XDG_CONFIG_HOME", customPath)

	workflowsDir := AWFWorkflowsDir()
	promptsDir := AWFPromptsDir()
	scriptsDir := AWFScriptsDir()

	assert.Equal(t, filepath.Join(customPath, "awf", "workflows"), workflowsDir)
	assert.Equal(t, filepath.Join(customPath, "awf", "prompts"), promptsDir)
	assert.Equal(t, filepath.Join(customPath, "awf", "scripts"), scriptsDir)
}

func TestScriptsDir_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		xdgConfigHome string
		want          func() string
	}{
		{
			name:          "default XDG unset",
			xdgConfigHome: "",
			want: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, ".config", "awf", "scripts")
			},
		},
		{
			name:          "custom XDG_CONFIG_HOME",
			xdgConfigHome: "/opt/awf/config",
			want: func() string {
				return "/opt/awf/config/awf/scripts"
			},
		},
		{
			name:          "XDG with trailing slash",
			xdgConfigHome: "/custom/config/",
			want: func() string {
				return filepath.Join("/custom", "config", "awf", "scripts")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.xdgConfigHome != "" {
				t.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)
			} else {
				t.Setenv("XDG_CONFIG_HOME", "")
			}

			assert.Equal(t, tt.want(), AWFScriptsDir())
		})
	}
}
