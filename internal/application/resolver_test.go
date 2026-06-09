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

func TestResolver_ResolveMissingPackSegment(t *testing.T) {
	repo := mocks.NewMockWorkflowRepository()
	packDiscoverer := newMockPackDiscoverer()
	resolver := NewResolver(packDiscoverer, repo)

	ctx := context.Background()
	_, err := resolver.Resolve(ctx, "workflow-without-slash")

	require.Error(t, err)
	se, ok := err.(*domainerrors.StructuredError)
	require.True(t, ok, "error should be StructuredError")
	assert.Equal(t, domainerrors.ErrorCodeUserFacadeIdentifierMalformed, se.Code)
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
			name:            "missing slash",
			identifier:      "nopack",
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

func TestResolver_MalformedIdentifierUniformErrorCode(t *testing.T) {
	// Each wire convention sends a malformed identifier (no separator in canonical form).
	// After wire-to-canonical transformation, the resolver MUST return the same ErrorCode
	// regardless of which convention originated the input.
	tests := []struct {
		name          string
		wireInput     string
		wireTransform func(string) string
	}{
		{name: "CLI", wireInput: "nopack", wireTransform: WireFromCLI},
		{name: "HTTP", wireInput: "nopack", wireTransform: WireFromHTTP},
		{name: "ACP", wireInput: "nopack", wireTransform: WireFromACP},
		{name: "MCP", wireInput: "nopack", wireTransform: WireFromMCP},
	}

	codes := make([]domainerrors.ErrorCode, 0, len(tests))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockWorkflowRepository()
			packDiscoverer := newMockPackDiscoverer()
			resolver := NewResolver(packDiscoverer, repo)

			canonical := tt.wireTransform(tt.wireInput)
			_, err := resolver.Resolve(context.Background(), canonical)

			require.Error(t, err)
			se, ok := err.(*domainerrors.StructuredError)
			require.True(t, ok, "error must be a StructuredError")
			assert.Equal(t, domainerrors.ErrorCodeUserFacadeIdentifierMalformed, se.Code)
			codes = append(codes, se.Code)
		})
	}

	// All 4 wire conventions must produce identical ErrorCode.
	for i := 1; i < len(codes); i++ {
		assert.Equal(t, codes[0], codes[i],
			"wire convention at index %d produced different ErrorCode than index 0", i)
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
