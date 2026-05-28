// Package xdg — tests dedicated to the roles namespace directory resolution.
//
// These tests are kept in a separate file from xdg_test.go because they cover
// the AWFRolesDir and LocalRolesDir helpers introduced by F100 (dedicated roles
// namespace). Isolating them makes it easy to locate tests for the
// AWF_CONFIG_HOME / XDG_CONFIG_HOME priority chain and the AWF_ROLES_PATH
// override, as well as the structural parity assertion between LocalRolesDir
// and LocalSkillsDir.
package xdg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWFRolesDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name          string
		awfConfigHome string
		xdgConfigHome string
		want          string
	}{
		{
			name:          "uses XDG_CONFIG_HOME when set",
			xdgConfigHome: "/custom/config",
			want:          "/custom/config/awf/roles",
		},
		{
			name: "defaults to ~/.config/awf/roles when unset",
			want: filepath.Join(home, ".config", "awf", "roles"),
		},
		{
			name:          "AWF_CONFIG_HOME takes priority over XDG_CONFIG_HOME",
			awfConfigHome: "/custom/awf-home",
			xdgConfigHome: "/some/xdg/config",
			want:          "/custom/awf-home/roles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AWF_CONFIG_HOME", tt.awfConfigHome)
			t.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)

			got := AWFRolesDir()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLocalRolesDir(t *testing.T) {
	got := LocalRolesDir()

	assert.Equal(t, ".awf/roles", got)
	assert.False(t, filepath.IsAbs(got), "LocalRolesDir must be a project-relative path")
	// Structural parity with the skills helper it mirrors.
	assert.Equal(t, filepath.Dir(LocalSkillsDir()), filepath.Dir(got), "roles and skills local dirs share the same base")
}
