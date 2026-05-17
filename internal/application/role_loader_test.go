package application_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
)

func TestResolveAgentRole_EmptyRef(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{}
	ctx := context.Background()

	role, err := application.ResolveAgentRole(ctx, repo, "", "/some/dir")

	assert.NoError(t, err)
	assert.Nil(t, role)
}

func TestResolveAgentRole_NameResolution(t *testing.T) {
	expectedRole := &workflow.AgentRole{
		Name:    "go-senior",
		Content: "expert in golang",
	}

	repo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(ctx context.Context, name string) (*workflow.AgentRole, error) {
			if name != "go-senior" {
				t.Errorf("LoadFunc called with wrong name: %q", name)
			}
			return expectedRole, nil
		},
	}

	ctx := context.Background()
	role, err := application.ResolveAgentRole(ctx, repo, "go-senior", "/some/dir")

	require.NoError(t, err)
	assert.Equal(t, expectedRole, role)
}

func TestResolveAgentRole_AbsolutePath(t *testing.T) {
	expectedRole := &workflow.AgentRole{
		Name:    "custom",
		Content: "custom content",
	}
	expectedPath := "/abs/path/to/role.yaml"

	repo := &mocks.MockAgentRoleRepository{
		LoadFromPathFunc: func(ctx context.Context, absolutePath string) (*workflow.AgentRole, error) {
			if absolutePath != expectedPath {
				t.Errorf("LoadFromPathFunc called with wrong path: %q", absolutePath)
			}
			return expectedRole, nil
		},
	}

	ctx := context.Background()
	role, err := application.ResolveAgentRole(ctx, repo, "/abs/path/to/role.yaml", "/some/dir")

	require.NoError(t, err)
	assert.Equal(t, expectedRole, role)
}

func TestResolveAgentRole_RelativePath(t *testing.T) {
	expectedRole := &workflow.AgentRole{
		Name:    "local",
		Content: "local role",
	}

	repo := &mocks.MockAgentRoleRepository{
		LoadFromPathFunc: func(ctx context.Context, absolutePath string) (*workflow.AgentRole, error) {
			expectedAbs := filepath.Join("/workflow/dir", "local")
			if absolutePath != expectedAbs {
				t.Errorf("LoadFromPathFunc called with wrong path: %q, expected %q", absolutePath, expectedAbs)
			}
			return expectedRole, nil
		},
	}

	ctx := context.Background()
	role, err := application.ResolveAgentRole(ctx, repo, "./local", "/workflow/dir")

	require.NoError(t, err)
	assert.Equal(t, expectedRole, role)
}

func TestResolveAgentRole_TildePath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	expectedRole := &workflow.AgentRole{
		Name:    "home-role",
		Content: "role in home",
	}
	expectedPath := filepath.Join(homeDir, "roles", "expert.yaml")

	repo := &mocks.MockAgentRoleRepository{
		LoadFromPathFunc: func(ctx context.Context, absolutePath string) (*workflow.AgentRole, error) {
			if absolutePath != expectedPath {
				t.Errorf("LoadFromPathFunc called with wrong path: %q, expected %q", absolutePath, expectedPath)
			}
			return expectedRole, nil
		},
	}

	ctx := context.Background()
	role, err := application.ResolveAgentRole(ctx, repo, "~/roles/expert.yaml", "/workflow/dir")

	require.NoError(t, err)
	assert.Equal(t, expectedRole, role)
}

func TestResolveAgentRole_PathWithBackslash(t *testing.T) {
	expectedRole := &workflow.AgentRole{
		Name:    "win-path",
		Content: "windows path",
	}

	pathCalled := false
	repo := &mocks.MockAgentRoleRepository{
		LoadFromPathFunc: func(ctx context.Context, absolutePath string) (*workflow.AgentRole, error) {
			pathCalled = true
			return expectedRole, nil
		},
	}

	ctx := context.Background()
	role, err := application.ResolveAgentRole(ctx, repo, `some\path\file`, "/workflow/dir")

	require.NoError(t, err)
	assert.True(t, pathCalled, "should have called LoadFromPath for backslash path")
	assert.Equal(t, expectedRole, role)
}

func TestResolveAgentRole_DotStartPath(t *testing.T) {
	expectedRole := &workflow.AgentRole{
		Name:    "dot-path",
		Content: "path starting with dot",
	}

	pathCalled := false
	repo := &mocks.MockAgentRoleRepository{
		LoadFromPathFunc: func(ctx context.Context, absolutePath string) (*workflow.AgentRole, error) {
			pathCalled = true
			return expectedRole, nil
		},
	}

	ctx := context.Background()
	role, err := application.ResolveAgentRole(ctx, repo, ".hidden", "/workflow/dir")

	require.NoError(t, err)
	assert.True(t, pathCalled, "should have called LoadFromPath for dot-start path")
	assert.Equal(t, expectedRole, role)
}

func TestResolveAgentRole_ErrorWrapping(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(ctx context.Context, name string) (*workflow.AgentRole, error) {
			return nil, context.DeadlineExceeded
		},
	}

	ctx := context.Background()
	_, err := application.ResolveAgentRole(ctx, repo, "missing-role", "/some/dir")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing-role")
}

func TestResolveAgentRole_PathErrorWrapping(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{
		LoadFromPathFunc: func(ctx context.Context, absolutePath string) (*workflow.AgentRole, error) {
			return nil, os.ErrNotExist
		},
	}

	ctx := context.Background()
	_, err := application.ResolveAgentRole(ctx, repo, "/path/to/role", "/some/dir")

	require.Error(t, err)
	// Error should mention the original unresolved ref
	assert.Contains(t, err.Error(), "/path/to/role")
}

func TestComposeSystemPrompt_BothNonEmpty(t *testing.T) {
	result := application.ComposeSystemPrompt("You are an expert", "Be concise")
	assert.Equal(t, "You are an expert\n\nBe concise", result)
}

func TestComposeSystemPrompt_RoleOnly(t *testing.T) {
	result := application.ComposeSystemPrompt("You are an expert", "")
	assert.Equal(t, "You are an expert", result)
}

func TestComposeSystemPrompt_InlineOnly(t *testing.T) {
	result := application.ComposeSystemPrompt("", "Be concise")
	assert.Equal(t, "Be concise", result)
}

func TestComposeSystemPrompt_BothEmpty(t *testing.T) {
	result := application.ComposeSystemPrompt("", "")
	assert.Equal(t, "", result)
}

func TestComposeSystemPrompt_PreservesWhitespace(t *testing.T) {
	role := "Line 1\nLine 2"
	inline := "Inline 1\nInline 2"
	result := application.ComposeSystemPrompt(role, inline)
	assert.Equal(t, "Line 1\nLine 2\n\nInline 1\nInline 2", result)
}

func TestResolveAgentRole_ContextError(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := application.ResolveAgentRole(ctx, repo, "test", "/dir")

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
