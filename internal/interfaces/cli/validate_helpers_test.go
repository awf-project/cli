package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func TestSkillDirExists(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(base, "present"), 0o755))

	t.Run("directory present", func(t *testing.T) {
		assert.True(t, skillDirExists("present", []string{base}))
	})
	t.Run("directory absent", func(t *testing.T) {
		assert.False(t, skillDirExists("missing", []string{base}))
	})
	t.Run("empty search paths", func(t *testing.T) {
		assert.False(t, skillDirExists("anything", nil))
	})
	t.Run("found on second path", func(t *testing.T) {
		assert.True(t, skillDirExists("present", []string{filepath.Join(base, "nope"), base}))
	})
}

func TestRoleDirExistsWithoutAgentsMD(t *testing.T) {
	base := t.TempDir()
	roleDir := filepath.Join(base, "go-senior")
	require.NoError(t, os.Mkdir(roleDir, 0o755))
	filePath := filepath.Join(base, "afile")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o644))

	t.Run("single path: existing directory (path-ref)", func(t *testing.T) {
		nf := &workflow.AgentRoleNotFoundError{Name: "go-senior", SearchPaths: []string{roleDir}, IsPathRef: true}
		assert.True(t, roleDirExistsWithoutAgentsMD(nf))
	})
	t.Run("single path: a file, not a directory (path-ref)", func(t *testing.T) {
		nf := &workflow.AgentRoleNotFoundError{Name: "afile", SearchPaths: []string{filePath}, IsPathRef: true}
		assert.False(t, roleDirExistsWithoutAgentsMD(nf))
	})
	t.Run("single path: missing (path-ref)", func(t *testing.T) {
		nf := &workflow.AgentRoleNotFoundError{Name: "ghost", SearchPaths: []string{filepath.Join(base, "ghost")}, IsPathRef: true}
		assert.False(t, roleDirExistsWithoutAgentsMD(nf))
	})
	t.Run("multi path: dir found on second search path", func(t *testing.T) {
		nf := &workflow.AgentRoleNotFoundError{
			Name:        "go-senior",
			SearchPaths: []string{filepath.Join(base, "elsewhere"), base},
		}
		assert.True(t, roleDirExistsWithoutAgentsMD(nf))
	})
	t.Run("multi path: dir absent on all search paths", func(t *testing.T) {
		nf := &workflow.AgentRoleNotFoundError{
			Name:        "go-senior",
			SearchPaths: []string{filepath.Join(base, "a"), filepath.Join(base, "b")},
		}
		assert.False(t, roleDirExistsWithoutAgentsMD(nf))
	})
}

func TestRoleContentWarnings(t *testing.T) {
	t.Run("empty content warns", func(t *testing.T) {
		got := roleContentWarnings(&workflow.AgentRole{Name: "r"}, "")
		require.Len(t, got, 1)
		assert.Contains(t, got[0], "empty AGENTS.md body")
	})
	t.Run("within limits returns nil", func(t *testing.T) {
		assert.Nil(t, roleContentWarnings(&workflow.AgentRole{Name: "r", Content: "small body"}, "short prompt"))
	})
	t.Run("oversized AGENTS.md warns from RawSizeBytes (no filesystem access)", func(t *testing.T) {
		got := roleContentWarnings(&workflow.AgentRole{
			Name:         "r",
			Content:      "non-empty",
			RawSizeBytes: workflow.AgentRoleSizeWarnBytes + 1,
		}, "")
		require.Len(t, got, 1)
		assert.Contains(t, got[0], "exceeds 500KB")
	})
	t.Run("combined role + system_prompt over threshold warns", func(t *testing.T) {
		content := strings.Repeat("a", 6000)
		prompt := strings.Repeat("b", 6000) // 12000 > 10KB, SourcePath empty so file size is 0
		got := roleContentWarnings(&workflow.AgentRole{Name: "r", Content: content}, prompt)
		require.Len(t, got, 1)
		assert.Contains(t, got[0], "exceeds 10KB")
	})
}
