package xdg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWFSkillsDir(t *testing.T) {
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
			want:     "/custom/config/awf/skills",
		},
		{
			name:     "defaults to ~/.config/awf/skills when unset",
			envValue: "",
			want:     filepath.Join(home, ".config", "awf", "skills"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("XDG_CONFIG_HOME", tt.envValue)
			} else {
				t.Setenv("XDG_CONFIG_HOME", "")
			}

			got := AWFSkillsDir()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAWFSkillsDir_IsUnderConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	configDir := AWFConfigDir()
	skillsDir := AWFSkillsDir()

	assert.True(t, strings.HasPrefix(skillsDir, configDir),
		"AWFSkillsDir (%s) should be under AWFConfigDir (%s)", skillsDir, configDir)
}

func TestAWFSkillsDir_EndsWithSkills(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	got := AWFSkillsDir()

	assert.True(t, filepath.Base(got) == "skills",
		"AWFSkillsDir should end with 'skills', got: %s", got)
}

func TestAWFSkillsDir_MirrorsPromptsPattern(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	promptsDir := AWFPromptsDir()
	skillsDir := AWFSkillsDir()

	promptsBase := filepath.Dir(promptsDir)
	skillsBase := filepath.Dir(skillsDir)

	assert.Equal(t, promptsBase, skillsBase,
		"AWFSkillsDir and AWFPromptsDir should be siblings under same parent directory")
}

func TestSkillsDir_ConsistentWithOtherDirs(t *testing.T) {
	customPath := "/custom/config/path"
	t.Setenv("XDG_CONFIG_HOME", customPath)

	workflowsDir := AWFWorkflowsDir()
	promptsDir := AWFPromptsDir()
	scriptsDir := AWFScriptsDir()
	skillsDir := AWFSkillsDir()

	assert.Equal(t, filepath.Join(customPath, "awf", "workflows"), workflowsDir)
	assert.Equal(t, filepath.Join(customPath, "awf", "prompts"), promptsDir)
	assert.Equal(t, filepath.Join(customPath, "awf", "scripts"), scriptsDir)
	assert.Equal(t, filepath.Join(customPath, "awf", "skills"), skillsDir)
}

func TestSkillsDir_TableDriven(t *testing.T) {
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
				return filepath.Join(home, ".config", "awf", "skills")
			},
		},
		{
			name:          "custom XDG_CONFIG_HOME",
			xdgConfigHome: "/opt/awf/config",
			want: func() string {
				return "/opt/awf/config/awf/skills"
			},
		},
		{
			name:          "XDG with trailing slash",
			xdgConfigHome: "/custom/config/",
			want: func() string {
				return filepath.Join("/custom", "config", "awf", "skills")
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

			assert.Equal(t, tt.want(), AWFSkillsDir())
		})
	}
}

func TestLocalSkillsDir(t *testing.T) {
	got := LocalSkillsDir()
	assert.Equal(t, ".awf/skills", got)
}

func TestLocalSkillsDir_IsRelative(t *testing.T) {
	got := LocalSkillsDir()

	assert.False(t, filepath.IsAbs(got),
		"LocalSkillsDir should be relative, got: %s", got)
}

func TestLocalSkillsDir_MirrorsLocalPromptsPattern(t *testing.T) {
	promptsDir := LocalPromptsDir()
	skillsDir := LocalSkillsDir()

	assert.True(t, strings.HasPrefix(promptsDir, ".awf/"),
		"LocalPromptsDir should be under .awf/")
	assert.True(t, strings.HasPrefix(skillsDir, ".awf/"),
		"LocalSkillsDir should be under .awf/")

	assert.Equal(t, ".awf/prompts", promptsDir)
	assert.Equal(t, ".awf/skills", skillsDir)
}

func TestSkillsDirs_ConsistentWithOtherLocalDirs(t *testing.T) {
	workflowsDir := LocalWorkflowsDir()
	promptsDir := LocalPromptsDir()
	pluginsDir := LocalPluginsDir()
	skillsDir := LocalSkillsDir()

	assert.Equal(t, ".awf/workflows", workflowsDir)
	assert.Equal(t, ".awf/prompts", promptsDir)
	assert.Equal(t, ".awf/plugins", pluginsDir)
	assert.Equal(t, ".awf/skills", skillsDir)
}

func TestAWFPathsIncludesSkillsDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	paths := AWFPaths()
	skillsDir, hasSkillsDir := paths["skills_dir"]

	assert.True(t, hasSkillsDir, "AWFPaths() should include skills_dir key")
	assert.Equal(t, AWFSkillsDir(), skillsDir, "skills_dir value should match AWFSkillsDir()")
}
