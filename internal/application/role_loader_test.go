package application_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/roles"
	"github.com/awf-project/cli/internal/testutil"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
)

// errAssert is a sentinel error for resolver-failure test cases.
var errAssert = errors.New("resolver failure")

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

// TestResolveAgentRole_ByNameIgnoresWorkflowDir documents FR-008 / SC-005:
// a bare name ref searches CWD-local paths only; workflowDir is not consulted.
func TestResolveAgentRole_ByNameIgnoresWorkflowDir(t *testing.T) {
	projectDir := t.TempDir()
	workflowDir := t.TempDir() // deliberately distinct from projectDir

	localRoleDir := filepath.Join(projectDir, ".awf", "roles", "go-senior")
	require.NoError(t, os.MkdirAll(localRoleDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(localRoleDir, "AGENTS.md"),
		[]byte("---\n---\nCWD-local go-senior content"),
		0o644,
	))

	testutil.ChdirIsolated(t, projectDir)

	repo := roles.NewFilesystemAgentRoleRepository(nil)
	ctx := context.Background()

	role, err := application.ResolveAgentRole(ctx, repo, "go-senior", workflowDir)

	require.NoError(t, err)
	require.NotNil(t, role)
	assert.Equal(t, "go-senior", role.Name)
	assert.Contains(t, role.Content, "CWD-local go-senior content")
}

// TestExpandRolePath_RelativeToWorkflowDir documents FR-009:
// a ./... ref is joined onto workflowDir, not CWD.
func TestExpandRolePath_RelativeToWorkflowDir(t *testing.T) {
	projectDir := t.TempDir()  // CWD — no roles here
	workflowDir := t.TempDir() // has the role

	roleDir := filepath.Join(workflowDir, "roles", "go-remote")
	require.NoError(t, os.MkdirAll(roleDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(roleDir, "AGENTS.md"),
		[]byte("---\n---\nWorkflowDir-relative go-remote content"),
		0o644,
	))

	testutil.ChdirIsolated(t, projectDir)

	repo := roles.NewFilesystemAgentRoleRepository(nil)
	ctx := context.Background()

	role, err := application.ResolveAgentRole(ctx, repo, "./roles/go-remote", workflowDir)

	require.NoError(t, err)
	require.NotNil(t, role)
	assert.Equal(t, "go-remote", role.Name)
	assert.Contains(t, role.Content, "WorkflowDir-relative go-remote content")
}

// TestExpandRolePath_RelativeDoesNotUseCWD documents FR-009 negative case:
// even when the role exists at CWD/./..., it must not be resolved because workflowDir
// is authoritative for path refs.
func TestExpandRolePath_RelativeDoesNotUseCWD(t *testing.T) {
	projectDir := t.TempDir()  // CWD — role exists here but must NOT be used
	workflowDir := t.TempDir() // workflow location — no roles here

	cwdRoleDir := filepath.Join(projectDir, "roles", "go-remote")
	require.NoError(t, os.MkdirAll(cwdRoleDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(cwdRoleDir, "AGENTS.md"),
		[]byte("---\n---\nCWD-only content"),
		0o644,
	))

	testutil.ChdirIsolated(t, projectDir)

	repo := roles.NewFilesystemAgentRoleRepository(nil)
	ctx := context.Background()

	// resolves to workflowDir/roles/go-remote which does not exist
	_, err := application.ResolveAgentRole(ctx, repo, "./roles/go-remote", workflowDir)

	require.Error(t, err)
	var notFoundErr *workflow.AgentRoleNotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
}

// roleErrResolver implements interpolation.Resolver and always fails — used to
// exercise the template-resolution error branch of BuildRoleSystemPrompt.
type roleErrResolver struct{ err error }

func (r *roleErrResolver) Resolve(string, *interpolation.Context) (string, error) {
	return "", r.err
}

func agentStep(role, systemPrompt string) *workflow.Step {
	return &workflow.Step{Agent: &workflow.AgentConfig{Role: role, SystemPrompt: systemPrompt}}
}

func TestBuildRoleSystemPrompt_NilAgentReturnsEmpty(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{}
	resolver := &testResolverWithValues{}

	got, err := application.BuildRoleSystemPrompt(context.Background(), repo, resolver, &workflow.Step{}, "/dir", interpolation.NewContext())

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestBuildRoleSystemPrompt_NoRoleReturnsInlineOnly(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{}
	resolver := &testResolverWithValues{}

	got, err := application.BuildRoleSystemPrompt(context.Background(), repo, resolver, agentStep("", "be concise"), "/dir", interpolation.NewContext())

	require.NoError(t, err)
	assert.Equal(t, "be concise", got)
}

func TestBuildRoleSystemPrompt_ByNameComposesWithInline(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(_ context.Context, name string) (*workflow.AgentRole, error) {
			assert.Equal(t, "go-senior", name)
			return &workflow.AgentRole{Name: name, Content: "expert in golang"}, nil
		},
	}
	resolver := &testResolverWithValues{}

	got, err := application.BuildRoleSystemPrompt(context.Background(), repo, resolver, agentStep("go-senior", "be concise"), "/dir", interpolation.NewContext())

	require.NoError(t, err)
	assert.Equal(t, "expert in golang\n\nbe concise", got)
}

func TestBuildRoleSystemPrompt_ByNameRoleOnly(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(_ context.Context, name string) (*workflow.AgentRole, error) {
			return &workflow.AgentRole{Name: name, Content: "expert in golang"}, nil
		},
	}
	resolver := &testResolverWithValues{}

	got, err := application.BuildRoleSystemPrompt(context.Background(), repo, resolver, agentStep("go-senior", ""), "/dir", interpolation.NewContext())

	require.NoError(t, err)
	assert.Equal(t, "expert in golang", got)
}

func TestBuildRoleSystemPrompt_TemplateInterpolatedBeforeLookup(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(_ context.Context, name string) (*workflow.AgentRole, error) {
			assert.Equal(t, "go-senior", name, "role template must be resolved before repo lookup")
			return &workflow.AgentRole{Name: name, Content: "expert"}, nil
		},
	}
	resolver := &testResolverWithValues{values: map[string]string{"{{inputs.persona}}": "go-senior"}}

	got, err := application.BuildRoleSystemPrompt(context.Background(), repo, resolver, agentStep("{{inputs.persona}}", ""), "/dir", interpolation.NewContext())

	require.NoError(t, err)
	assert.Equal(t, "expert", got)
}

func TestBuildRoleSystemPrompt_NilRepoWithRoleErrors(t *testing.T) {
	resolver := &testResolverWithValues{}

	_, err := application.BuildRoleSystemPrompt(context.Background(), nil, resolver, agentStep("go-senior", ""), "/dir", interpolation.NewContext())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "go-senior")
}

func TestBuildRoleSystemPrompt_TemplateResolveErrorPropagates(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{}
	resolver := &roleErrResolver{err: errAssert}

	_, err := application.BuildRoleSystemPrompt(context.Background(), repo, resolver, agentStep("{{bad}}", ""), "/dir", interpolation.NewContext())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve role template")
}

func TestBuildRoleSystemPrompt_RepoErrorPropagates(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(context.Context, string) (*workflow.AgentRole, error) {
			return nil, &workflow.AgentRoleNotFoundError{Name: "missing", SearchPaths: []string{".awf/roles"}}
		},
	}
	resolver := &testResolverWithValues{}

	_, err := application.BuildRoleSystemPrompt(context.Background(), repo, resolver, agentStep("missing", ""), "/dir", interpolation.NewContext())

	require.Error(t, err)
	var notFound *workflow.AgentRoleNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestBuildRoleSystemPrompt_NilRoleFallsBackToInline(t *testing.T) {
	repo := &mocks.MockAgentRoleRepository{
		LoadFunc: func(context.Context, string) (*workflow.AgentRole, error) {
			return nil, nil // resolver returned no role
		},
	}
	resolver := &testResolverWithValues{}

	got, err := application.BuildRoleSystemPrompt(context.Background(), repo, resolver, agentStep("ghost", "inline only"), "/dir", interpolation.NewContext())

	require.NoError(t, err)
	assert.Equal(t, "inline only", got)
}
