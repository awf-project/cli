//go:build integration

// Feature: C059

package plugins_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubOperationRegistry_ExactlyEightOperations(t *testing.T) {
	ops := github.AllOperations()

	require.Len(t, ops, 8)

	names := make(map[string]bool, len(ops))
	for _, op := range ops {
		names[op.Name] = true
	}

	expected := []string{
		"github.get_issue",
		"github.get_pr",
		"github.create_pr",
		"github.create_issue",
		"github.add_labels",
		"github.list_comments",
		"github.add_comment",
		"github.batch",
	}
	for _, name := range expected {
		assert.True(t, names[name], "missing operation: %s", name)
	}

	assert.False(t, names["github.set_project_status"],
		"set_project_status stub should have been removed")
}

func TestGitHubProvider_RejectsDeletedOperation(t *testing.T) {
	client := github.NewClient(nil)
	provider := github.NewGitHubOperationProvider(client, nil)

	_, err := provider.Execute(context.Background(), "github.set_project_status", map[string]any{
		"project_id": "PVT_123",
		"item_id":    "PVTI_456",
		"status":     "Done",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operation")
}

func TestGitHubProvider_DispatchesRemainingOperations(t *testing.T) {
	ops := github.AllOperations()
	client := github.NewClient(nil)
	provider := github.NewGitHubOperationProvider(client, nil)

	for _, op := range ops {
		t.Run(op.Name, func(t *testing.T) {
			_, ok := provider.GetOperation(op.Name)
			require.True(t, ok, "operation %s should be registered", op.Name)
		})
	}

	assert.Len(t, provider.ListOperations(), 8)
}

func TestGitHubClient_RunHTTPRemoved(t *testing.T) {
	clientType := reflect.TypeOf(&github.Client{})

	_, hasRunHTTP := clientType.MethodByName("RunHTTP")
	assert.False(t, hasRunHTTP, "RunHTTP method should have been removed from Client")

	_, hasRunGH := clientType.MethodByName("RunGH")
	assert.True(t, hasRunGH, "RunGH method should still exist on Client")

	_, hasDetectRepo := clientType.MethodByName("DetectRepo")
	assert.True(t, hasDetectRepo, "DetectRepo method should still exist on Client")
}

func TestGitHubPackage_NoReferencesToDeletedSymbols(t *testing.T) {
	repoRoot := getRepoRoot(t)
	githubDir := filepath.Join(repoRoot, "internal", "infrastructure", "github")

	entries, err := os.ReadDir(githubDir)
	require.NoError(t, err)

	deletedSymbols := []string{"set_project_status", "handleSetProjectStatus", "RunHTTP"}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(githubDir, entry.Name()))
		require.NoError(t, err)

		for _, sym := range deletedSymbols {
			assert.NotContains(t, string(content), sym,
				"%s: should not contain reference to deleted symbol %q", entry.Name(), sym)
		}
	}

	deletedFiles := []string{
		"handler_set_project_status.go",
		"handler_set_project_status_test.go",
	}
	for _, f := range deletedFiles {
		_, err := os.Stat(filepath.Join(githubDir, f))
		assert.ErrorIs(t, err, os.ErrNotExist, "%s should have been deleted", f)
	}

	deletedFixture := filepath.Join(repoRoot, "tests", "fixtures", "workflows", "github-project-test.yaml")
	_, err = os.Stat(deletedFixture)
	assert.ErrorIs(t, err, os.ErrNotExist, "github-project-test.yaml fixture should have been deleted")
}
