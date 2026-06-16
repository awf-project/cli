package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
)

// mockPackDiscoverer is a simple test implementation of ports.PackDiscoverer.
type mockPackDiscoverer struct {
	workflows map[string]map[string]*workflow.Workflow
	err       error
}

func newMockPackDiscoverer() *mockPackDiscoverer {
	return &mockPackDiscoverer{
		workflows: make(map[string]map[string]*workflow.Workflow),
	}
}

func (m *mockPackDiscoverer) DiscoverWorkflows(ctx context.Context) ([]workflow.WorkflowEntry, error) {
	return nil, nil
}

func (m *mockPackDiscoverer) LoadWorkflow(ctx context.Context, packName, workflowName string) (*workflow.Workflow, error) {
	if m.err != nil {
		return nil, m.err
	}
	pack, ok := m.workflows[packName]
	if !ok {
		return nil, nil // Pack not found, resolver should fall back to repository
	}
	return pack[workflowName], nil // May return nil if workflow not found in pack
}

func (m *mockPackDiscoverer) addWorkflow(packName, workflowName string, wf *workflow.Workflow) {
	if _, ok := m.workflows[packName]; !ok {
		m.workflows[packName] = make(map[string]*workflow.Workflow)
	}
	m.workflows[packName][workflowName] = wf
}

func TestResolver_ResolveEmptyIdentifier(t *testing.T) {
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()
	resolver := NewResolver(packDiscoverer, repo)

	ctx := context.Background()
	_, err := resolver.Resolve(ctx, "")

	require.Error(t, err)
	se, ok := err.(*domainerrors.StructuredError)
	require.True(t, ok, "error should be StructuredError")
	assert.Equal(t, domainerrors.ErrorCodeUserFacadeIdentifierEmpty, se.Code)
}

func TestResolver_ResolveBareNameViaRepository(t *testing.T) {
	// A bare workflow name (no pack separator) resolves by name against the
	// repository — the single-core convention shared by CLI, TUI, HTTP and ACP.
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()

	expectedWf := &workflow.Workflow{Name: "get-current-time"}
	repo.AddWorkflow("get-current-time", expectedWf)

	resolver := NewResolver(packDiscoverer, repo)

	ctx := context.Background()
	wf, err := resolver.Resolve(ctx, "get-current-time")

	require.NoError(t, err)
	require.NotNil(t, wf)
	assert.Equal(t, "get-current-time", wf.Name)
}

func TestResolver_ResolveBareNameNotFound(t *testing.T) {
	// A bare name with no matching repository workflow is "workflow not found",
	// the same outcome as the explicit "*/name" wildcard miss.
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()
	resolver := NewResolver(packDiscoverer, repo)

	ctx := context.Background()
	_, err := resolver.Resolve(ctx, "no-such-workflow")

	require.Error(t, err)
	se, ok := err.(*domainerrors.StructuredError)
	require.True(t, ok, "error should be StructuredError")
	assert.Equal(t, domainerrors.ErrorCodeUserFacadeWorkflowNotFound, se.Code)
}

func TestResolver_EmptySegmentIsMalformed(t *testing.T) {
	// A separator with an empty pack ("/wf") or empty workflow ("pack/") segment
	// is genuinely malformed — distinct from a bare local name.
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()
	resolver := NewResolver(packDiscoverer, repo)

	for _, id := range []string{"pack/", "/wf"} {
		t.Run(id, func(t *testing.T) {
			_, err := resolver.Resolve(context.Background(), id)
			require.Error(t, err)
			se, ok := err.(*domainerrors.StructuredError)
			require.True(t, ok, "error should be StructuredError")
			assert.Equal(t, domainerrors.ErrorCodeUserFacadeIdentifierMalformed, se.Code)
		})
	}
}

func TestResolver_ResolveFromPackDiscoverer(t *testing.T) {
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()

	expectedWf := &workflow.Workflow{
		Name: "pack-workflow",
	}
	packDiscoverer.addWorkflow("my-pack", "pack-workflow", expectedWf)

	resolver := NewResolver(packDiscoverer, repo)

	ctx := context.Background()
	wf, err := resolver.Resolve(ctx, "my-pack/pack-workflow")

	require.NoError(t, err)
	require.NotNil(t, wf)
	assert.Equal(t, "pack-workflow", wf.Name)
}

func TestResolver_ResolveFallsBackToRepository(t *testing.T) {
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()

	expectedWf := &workflow.Workflow{
		Name: "repo-workflow",
	}
	repo.AddWorkflow("repo-workflow", expectedWf)

	resolver := NewResolver(packDiscoverer, repo)

	ctx := context.Background()
	wf, err := resolver.Resolve(ctx, "*/repo-workflow")

	require.NoError(t, err)
	require.NotNil(t, wf)
	assert.Equal(t, "repo-workflow", wf.Name)
}

func TestResolver_PackNotFound(t *testing.T) {
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()
	resolver := NewResolver(packDiscoverer, repo)

	ctx := context.Background()
	_, err := resolver.Resolve(ctx, "nonexistent-pack/workflow")

	require.Error(t, err)
	se, ok := err.(*domainerrors.StructuredError)
	require.True(t, ok, "error should be StructuredError")
	assert.Equal(t, domainerrors.ErrorCodeUserFacadePackNotFound, se.Code)
}

func TestResolver_WorkflowNotFoundInRepository(t *testing.T) {
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()
	resolver := NewResolver(packDiscoverer, repo)

	ctx := context.Background()
	_, err := resolver.Resolve(ctx, "*/nonexistent-workflow")

	require.Error(t, err)
	se, ok := err.(*domainerrors.StructuredError)
	require.True(t, ok, "error should be StructuredError")
	assert.Equal(t, domainerrors.ErrorCodeUserFacadeWorkflowNotFound, se.Code)
}

func TestResolver_CanonicalFormatPackAndWorkflow(t *testing.T) {
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()

	wf1 := &workflow.Workflow{Name: "workflow-1"}
	wf2 := &workflow.Workflow{Name: "workflow-2"}

	packDiscoverer.addWorkflow("pack-1", "workflow-1", wf1)
	packDiscoverer.addWorkflow("pack-2", "workflow-2", wf2)

	resolver := NewResolver(packDiscoverer, repo)
	ctx := context.Background()

	result1, err1 := resolver.Resolve(ctx, "pack-1/workflow-1")
	require.NoError(t, err1)
	assert.Equal(t, "workflow-1", result1.Name)

	result2, err2 := resolver.Resolve(ctx, "pack-2/workflow-2")
	require.NoError(t, err2)
	assert.Equal(t, "workflow-2", result2.Name)
}

func TestResolver_MultipleErrorsReturnStructured(t *testing.T) {
	tests := []struct {
		name            string
		identifier      string
		expectedErrCode domainerrors.ErrorCode
	}{
		{
			name:            "empty identifier",
			identifier:      "",
			expectedErrCode: domainerrors.ErrorCodeUserFacadeIdentifierEmpty,
		},
		{
			name:            "empty workflow segment",
			identifier:      "pack/",
			expectedErrCode: domainerrors.ErrorCodeUserFacadeIdentifierMalformed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockWorkflowRepository()
			packDiscoverer := newMockPackDiscoverer()
			resolver := NewResolver(packDiscoverer, repo)

			ctx := context.Background()
			_, err := resolver.Resolve(ctx, tt.identifier)

			require.Error(t, err)
			se, ok := err.(*domainerrors.StructuredError)
			require.True(t, ok)
			assert.Equal(t, tt.expectedErrCode, se.Code)
		})
	}
}

func TestResolver_BareNameResolvesUniformlyAcrossWires(t *testing.T) {
	// A bare local workflow name carries no separator, so every wire convention
	// passes it through unchanged and the resolver routes it to the repository.
	// This is the single-core guarantee: the same bare name behaves identically
	// regardless of which interface (CLI, HTTP, ACP, MCP) originated it.
	// CLI and HTTP carry identifiers in canonical form natively (no encoding),
	// so they are expressed as plain identity lambdas here.
	tests := []struct {
		name          string
		wireTransform func(string) string
	}{
		{name: "CLI", wireTransform: func(s string) string { return s }},
		{name: "HTTP", wireTransform: func(s string) string { return s }},
		{name: "ACP", wireTransform: EncodeForACP},
		{name: "MCP", wireTransform: EncodeForMCP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockWorkflowRepository()
			packDiscoverer := newMockPackDiscoverer()
			repo.AddWorkflow("get-current-time", &workflow.Workflow{Name: "get-current-time"})
			resolver := NewResolver(packDiscoverer, repo)

			canonical := tt.wireTransform("get-current-time")
			wf, err := resolver.Resolve(context.Background(), canonical)

			require.NoError(t, err)
			require.NotNil(t, wf)
			assert.Equal(t, "get-current-time", wf.Name)
		})
	}
}

func TestResolver_MultipleSlashesAreSupported(t *testing.T) {
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()

	wf := &workflow.Workflow{Name: "my-wf"}
	packDiscoverer.addWorkflow("my-pack", "namespace/my-wf", wf)

	resolver := NewResolver(packDiscoverer, repo)

	ctx := context.Background()
	result, err := resolver.Resolve(ctx, "my-pack/namespace/my-wf")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "my-wf", result.Name)
}
