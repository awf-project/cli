package xdg

import (
	"os"
	"path/filepath"
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
			orig := os.Getenv("XDG_CONFIG_HOME")
			defer func() { _ = os.Setenv("XDG_CONFIG_HOME", orig) }()

			if tt.envValue != "" {
				_ = os.Setenv("XDG_CONFIG_HOME", tt.envValue)
			} else {
				_ = os.Unsetenv("XDG_CONFIG_HOME")
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
			orig := os.Getenv("XDG_DATA_HOME")
			defer func() { _ = os.Setenv("XDG_DATA_HOME", orig) }()

			if tt.envValue != "" {
				_ = os.Setenv("XDG_DATA_HOME", tt.envValue)
			} else {
				_ = os.Unsetenv("XDG_DATA_HOME")
			}

			got := DataHome()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAWFConfigDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	orig := os.Getenv("XDG_CONFIG_HOME")
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", orig) }()

	_ = os.Unsetenv("XDG_CONFIG_HOME")
	got := AWFConfigDir()
	assert.Equal(t, filepath.Join(home, ".config", "awf"), got)
}

func TestAWFDataDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	orig := os.Getenv("XDG_DATA_HOME")
	defer func() { _ = os.Setenv("XDG_DATA_HOME", orig) }()

	_ = os.Unsetenv("XDG_DATA_HOME")
	got := AWFDataDir()
	assert.Equal(t, filepath.Join(home, ".local", "share", "awf"), got)
}

func TestAWFWorkflowsDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	orig := os.Getenv("XDG_CONFIG_HOME")
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", orig) }()

	_ = os.Unsetenv("XDG_CONFIG_HOME")
	got := AWFWorkflowsDir()
	assert.Equal(t, filepath.Join(home, ".config", "awf", "workflows"), got)
}

func TestAWFStatesDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	orig := os.Getenv("XDG_DATA_HOME")
	defer func() { _ = os.Setenv("XDG_DATA_HOME", orig) }()

	_ = os.Unsetenv("XDG_DATA_HOME")
	got := AWFStatesDir()
	assert.Equal(t, filepath.Join(home, ".local", "share", "awf", "states"), got)
}

func TestAWFLogsDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	orig := os.Getenv("XDG_DATA_HOME")
	defer func() { _ = os.Setenv("XDG_DATA_HOME", orig) }()

	_ = os.Unsetenv("XDG_DATA_HOME")
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
