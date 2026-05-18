package roles_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/roles"
)

// mockLogger captures Warn calls for testing
type mockLogger struct {
	warnings []string
}

func (m *mockLogger) Debug(msg string, fields ...any) {}
func (m *mockLogger) Info(msg string, fields ...any)  {}
func (m *mockLogger) Warn(msg string, fields ...any) {
	m.warnings = append(m.warnings, msg)
}
func (m *mockLogger) Error(msg string, fields ...any) {}
func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

var _ ports.Logger = (*mockLogger)(nil)

func TestNewFilesystemAgentRoleRepository(t *testing.T) {
	logger := &mockLogger{}
	repo := roles.NewFilesystemAgentRoleRepository(logger)
	require.NotNil(t, repo)
}

func TestLoad_PriorityOrder(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup: Create agents in different search paths
	localDir := filepath.Join(tmpDir, ".awf", "agents", "go-senior")
	require.NoError(t, os.MkdirAll(localDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(localDir, "AGENTS.md"),
		[]byte("---\ntitle: Local\n---\nLocal agent role"),
		0o644,
	))

	crossClientDir := filepath.Join(tmpDir, ".agents", "go-senior")
	require.NoError(t, os.MkdirAll(crossClientDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(crossClientDir, "AGENTS.md"),
		[]byte("---\ntitle: CrossClient\n---\nCross-client agent role"),
		0o644,
	))

	t.Run("returns first match in priority order", func(t *testing.T) {
		origDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(origDir)
		require.NoError(t, os.Chdir(tmpDir))

		logger := &mockLogger{}
		repo := roles.NewFilesystemAgentRoleRepository(logger)

		role, err := repo.Load(context.Background(), "go-senior")

		require.NoError(t, err)
		assert.NotNil(t, role)
		assert.Equal(t, "go-senior", role.Name)
		assert.Equal(t, localDir+"/AGENTS.md", filepath.Clean(role.SourcePath))
		assert.NotContains(t, role.Content, "---")
		assert.Contains(t, role.Content, "Local agent role")
	})

	t.Run("skips missing paths and continues search", func(t *testing.T) {
		origDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(origDir)
		require.NoError(t, os.Chdir(tmpDir))

		// Remove local agent, should find cross-client
		require.NoError(t, os.RemoveAll(filepath.Join(tmpDir, ".awf", "agents", "go-senior")))

		logger := &mockLogger{}
		repo := roles.NewFilesystemAgentRoleRepository(logger)

		role, err := repo.Load(context.Background(), "go-senior")

		require.NoError(t, err)
		assert.NotNil(t, role)
		assert.Contains(t, role.Content, "Cross-client agent role")
	})
}

func TestLoad_AWFAgentsPathOverride(t *testing.T) {
	tmpDir := t.TempDir()
	overridePath := filepath.Join(tmpDir, "override")

	overrideAgentDir := filepath.Join(overridePath, "custom-role")
	require.NoError(t, os.MkdirAll(overrideAgentDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(overrideAgentDir, "AGENTS.md"),
		[]byte("---\ntitle: Override\n---\nOverride agent role"),
		0o644,
	))

	localAgentDir := filepath.Join(tmpDir, ".awf", "agents", "custom-role")
	require.NoError(t, os.MkdirAll(localAgentDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(localAgentDir, "AGENTS.md"),
		[]byte("---\ntitle: Local\n---\nLocal agent role"),
		0o644,
	))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)
	require.NoError(t, os.Chdir(tmpDir))

	t.Run("AWF_AGENTS_PATH exclusive search when set", func(t *testing.T) {
		t.Setenv("AWF_AGENTS_PATH", overridePath)

		logger := &mockLogger{}
		repo := roles.NewFilesystemAgentRoleRepository(logger)

		role, err := repo.Load(context.Background(), "custom-role")

		require.NoError(t, err)
		assert.Contains(t, role.Content, "Override agent role")
	})

	t.Run("default search when AWF_AGENTS_PATH empty", func(t *testing.T) {
		t.Setenv("AWF_AGENTS_PATH", "")

		logger := &mockLogger{}
		repo := roles.NewFilesystemAgentRoleRepository(logger)

		role, err := repo.Load(context.Background(), "custom-role")

		require.NoError(t, err)
		assert.Contains(t, role.Content, "Local agent role")
	})
}

func TestLoad_PathTraversalRejection(t *testing.T) {
	logger := &mockLogger{}
	repo := roles.NewFilesystemAgentRoleRepository(logger)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "go-senior", false},
		{"dot-dot escape", "../escape", true},
		{"nested dot-dot", "foo/../bar", true},
		{"forward slash", "foo/bar", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, err := repo.Load(context.Background(), tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, role)
				assert.ErrorContains(t, err, "invalid")
			} else if tt.input == "go-senior" {
				// Valid name but not found, should return AgentRoleNotFoundError
				assert.Error(t, err)
				assert.Nil(t, role)
				var notFoundErr *workflow.AgentRoleNotFoundError
				assert.ErrorAs(t, err, &notFoundErr)
			}
		})
	}
}

func TestLoad_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)
	require.NoError(t, os.Chdir(tmpDir))

	logger := &mockLogger{}
	repo := roles.NewFilesystemAgentRoleRepository(logger)

	role, err := repo.Load(context.Background(), "missing")

	require.Error(t, err)
	assert.Nil(t, role)

	var notFoundErr *workflow.AgentRoleNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, "missing", notFoundErr.Name)
	assert.NotEmpty(t, notFoundErr.SearchPaths)
	assert.Len(t, notFoundErr.SearchPaths, 4)
}

func TestLoad_FrontmatterStripping(t *testing.T) {
	tmpDir := t.TempDir()

	agentDir := filepath.Join(tmpDir, ".awf", "agents", "test-role")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	content := `---
title: Test Agent
tags:
  - testing
---
# Agent Role

This is the actual content after frontmatter.`

	require.NoError(t, os.WriteFile(
		filepath.Join(agentDir, "AGENTS.md"),
		[]byte(content),
		0o644,
	))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)
	require.NoError(t, os.Chdir(tmpDir))

	logger := &mockLogger{}
	repo := roles.NewFilesystemAgentRoleRepository(logger)

	role, err := repo.Load(context.Background(), "test-role")

	require.NoError(t, err)
	assert.NotContains(t, role.Content, "---")
	assert.NotContains(t, role.Content, "title: Test Agent")
	assert.Contains(t, role.Content, "# Agent Role")
	assert.Contains(t, role.Content, "This is the actual content after frontmatter.")
}

func TestLoad_LargeFileWarning(t *testing.T) {
	tmpDir := t.TempDir()

	agentDir := filepath.Join(tmpDir, ".awf", "agents", "large-role")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create a file > 500KB
	largeContent := strings.Repeat("x", 501*1024)
	require.NoError(t, os.WriteFile(
		filepath.Join(agentDir, "AGENTS.md"),
		[]byte("---\n---\n"+largeContent),
		0o644,
	))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)
	require.NoError(t, os.Chdir(tmpDir))

	logger := &mockLogger{}
	repo := roles.NewFilesystemAgentRoleRepository(logger)

	role, err := repo.Load(context.Background(), "large-role")

	require.NoError(t, err)
	assert.NotNil(t, role)
	assert.NotEmpty(t, logger.warnings)
	assert.True(t, strings.Contains(logger.warnings[0], "500KB") || strings.Contains(logger.warnings[0], "exceeds"))
}

func TestLoad_PerformanceSub100KB(t *testing.T) {
	tmpDir := t.TempDir()

	agentDir := filepath.Join(tmpDir, ".awf", "agents", "perf-role")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create a file ~100KB
	content := strings.Repeat("y", 100*1024)
	require.NoError(t, os.WriteFile(
		filepath.Join(agentDir, "AGENTS.md"),
		[]byte("---\n---\n"+content),
		0o644,
	))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)
	require.NoError(t, os.Chdir(tmpDir))

	logger := &mockLogger{}
	repo := roles.NewFilesystemAgentRoleRepository(logger)

	start := time.Now()
	role, err := repo.Load(context.Background(), "perf-role")
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.NotNil(t, role)
	assert.Less(t, elapsed, 50*time.Millisecond, "load should complete in < 50ms")
}

func TestLoadFromPath_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()

	agentDir := filepath.Join(tmpDir, "agents", "test-role")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(agentDir, "AGENTS.md"),
		[]byte("---\n---\nAgent content from path"),
		0o644,
	))

	logger := &mockLogger{}
	repo := roles.NewFilesystemAgentRoleRepository(logger)

	role, err := repo.LoadFromPath(context.Background(), agentDir)

	require.NoError(t, err)
	assert.NotNil(t, role)
	assert.Contains(t, role.Content, "Agent content from path")
	assert.True(t, filepath.IsAbs(role.SourcePath))
}

func TestLoadFromPath_DirectFile(t *testing.T) {
	tmpDir := t.TempDir()

	agentDir := filepath.Join(tmpDir, "agents", "test-role")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	agentFilePath := filepath.Join(agentDir, "AGENTS.md")
	require.NoError(t, os.WriteFile(
		agentFilePath,
		[]byte("---\n---\nDirect file content"),
		0o644,
	))

	logger := &mockLogger{}
	repo := roles.NewFilesystemAgentRoleRepository(logger)

	role, err := repo.LoadFromPath(context.Background(), agentDir)

	require.NoError(t, err)
	assert.NotNil(t, role)
	assert.Contains(t, role.Content, "Direct file content")
}

func TestLoadFromPath_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	missingPath := filepath.Join(tmpDir, "nonexistent")

	logger := &mockLogger{}
	repo := roles.NewFilesystemAgentRoleRepository(logger)

	role, err := repo.LoadFromPath(context.Background(), missingPath)

	require.Error(t, err)
	assert.Nil(t, role)

	var notFoundErr *workflow.AgentRoleNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
	assert.NotEmpty(t, notFoundErr.SearchPaths)
}

func TestSearchPaths_LocalToGlobal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create agents at different priority levels
	paths := map[string]string{
		"local":       filepath.Join(tmpDir, ".awf", "agents", "priority-role"),
		"cross":       filepath.Join(tmpDir, ".agents", "priority-role"),
		"global":      filepath.Join(tmpDir, ".config", "awf", "agents", "priority-role"),
		"crossglobal": filepath.Join(tmpDir, ".agents", "priority-role"),
	}

	for _, p := range paths {
		require.NoError(t, os.MkdirAll(p, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(p, "AGENTS.md"),
			[]byte(fmt.Sprintf("---\n---\nContent from %s", p)),
			0o644,
		))
	}

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)
	require.NoError(t, os.Chdir(tmpDir))

	// Unset AWF_AGENTS_PATH to use default search
	t.Setenv("AWF_AGENTS_PATH", "")

	logger := &mockLogger{}
	repo := roles.NewFilesystemAgentRoleRepository(logger)

	role, err := repo.Load(context.Background(), "priority-role")

	require.NoError(t, err)
	assert.NotNil(t, role)
	// Should find local first (highest priority)
	assert.Contains(t, role.SourcePath, ".awf")
}

func TestLoad_NilLogger(t *testing.T) {
	tmpDir := t.TempDir()

	agentDir := filepath.Join(tmpDir, ".awf", "agents", "safe-role")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(agentDir, "AGENTS.md"),
		[]byte("---\n---\nContent"),
		0o644,
	))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)
	require.NoError(t, os.Chdir(tmpDir))

	// Should not panic with nil logger
	repo := roles.NewFilesystemAgentRoleRepository(nil)

	role, err := repo.Load(context.Background(), "safe-role")

	require.NoError(t, err)
	assert.NotNil(t, role)
}
