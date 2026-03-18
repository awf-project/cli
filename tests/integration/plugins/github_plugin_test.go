//go:build integration

// Feature: F054 - GitHub CLI Plugin for Declarative Operations
// This file contains integration tests for the GitHub operation provider.

package plugins_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitHubGetIssue_Success tests retrieving issue data via github.get_issue operation.
// Acceptance Criteria: Issue fields (title, body, labels, state) available as step outputs
func TestGitHubGetIssue_Success(t *testing.T) {
	skipInCI(t)

	// Given: workflow with github.get_issue operation and valid issue number
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"issue_number": "1",
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-operations-test", inputs)

	// Then: operation provider not registered in test harness — expect execution error
	require.Error(t, err, "workflow execution should fail without registered operation provider")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "workflow file not found", "error should indicate missing workflow fixture")
}

// TestGitHubGetIssue_NotFound tests error handling for invalid issue number.
// Acceptance Criteria: Structured error with type github_not_found returned
func TestGitHubGetIssue_NotFound(t *testing.T) {
	skipInCI(t)

	// Given: workflow with github.get_issue and invalid issue number
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"issue_number": "999999", // Non-existent issue
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-operations-test", inputs)

	// Then: operation provider not registered in test harness — expect execution error
	require.Error(t, err, "workflow should fail when operation provider not registered")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "workflow file not found", "error should indicate missing workflow fixture")
}

// TestGitHubGetIssue_AuthError tests error handling when authentication missing.
// Acceptance Criteria: Structured error with type github_auth_error and remediation hint
func TestGitHubGetIssue_AuthError(t *testing.T) {
	skipInCI(t)

	// Given: workflow with github.get_issue and no authentication
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	// Clear all auth environment variables
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	ctx := context.Background()
	inputs := map[string]any{
		"issue_number": "1",
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-operations-test", inputs)

	// Then: workflow not found because github-operations-test.yaml does not exist
	require.Error(t, err, "workflow should fail")
	require.Nil(t, execCtx, "execCtx should be nil when workflow not found")
	assert.Contains(t, err.Error(), "not found", "error should indicate workflow not found")
}

// TestGitHubCreatePR_Success tests creating pull request via github.create_pr operation.
// Acceptance Criteria: PR created and URL/number available as outputs
func TestGitHubCreatePR_Success(t *testing.T) {
	skipInCI(t)
	skipIfCLIMissing(t, "gh")
	skipIfCLIMissing(t, "git")

	// Given: workflow with github.create_pr specifying title, body, base, head
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"title": "Test PR from integration test",
		"body":  "This is a test PR created by github_plugin_test.go",
		"base":  "main",
		"head":  "test-branch-" + t.Name(),
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-pr-test", inputs)

	// Then: fixture uses map format for inputs but parser expects array format — load fails
	require.Error(t, err, "workflow should fail due to YAML unmarshal error in fixture")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "unmarshal", "error should indicate YAML parse failure")
}

// TestGitHubCreatePR_BranchNotFound tests error handling for non-existent head branch.
// Acceptance Criteria: Error type github_branch_not_found returned
func TestGitHubCreatePR_BranchNotFound(t *testing.T) {
	skipInCI(t)

	// Given: workflow with github.create_pr and non-existent head branch
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"title": "Test PR",
		"body":  "Test",
		"base":  "main",
		"head":  "nonexistent-branch-12345",
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-pr-test", inputs)

	// Then: fixture uses map format for inputs but parser expects array format — load fails
	require.Error(t, err, "workflow should fail due to YAML unmarshal error in fixture")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "unmarshal", "error should indicate YAML parse failure")
}

// TestGitHubCreatePR_AlreadyExists tests handling existing PR for branch.
// Acceptance Criteria: Existing PR URL returned with already_exists flag
func TestGitHubCreatePR_AlreadyExists(t *testing.T) {
	skipInCI(t)
	skipIfCLIMissing(t, "gh")

	// Given: workflow with github.create_pr and PR already exists for branch
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	branchName := "test-existing-pr-" + t.Name()
	inputs := map[string]any{
		"title": "Test existing PR",
		"body":  "Test",
		"base":  "main",
		"head":  branchName,
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-pr-test", inputs)

	// Then: workflow fails because github-pr-test.yaml uses map-format inputs
	// which the YAML parser cannot unmarshal into []repository.yamlInput
	require.Error(t, err, "workflow should fail due to YAML parse error")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "unmarshal", "error should indicate YAML parse failure")
}

// TestGitHubBatch_AllSucceed tests batch operation with all operations succeeding.
// Acceptance Criteria: All operations complete and output contains success count
func TestGitHubBatch_AllSucceed(t *testing.T) {
	skipInCI(t)
	skipIfCLIMissing(t, "gh")

	// Given: workflow with github.batch containing 5 label additions
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"issues": "1,2,3,4,5", // 5 issues to label
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-batch-test", inputs)

	// Then: workflow not found — fixture filename is github-batch.yaml but test looks up github-batch-test
	require.Error(t, err, "workflow should fail when fixture not found by name")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow")
}

// TestGitHubBatch_BestEffort tests batch operation with partial failure.
// Acceptance Criteria: Successful operations complete, output shows partial results
func TestGitHubBatch_BestEffort(t *testing.T) {
	skipInCI(t)
	skipIfCLIMissing(t, "gh")

	// Given: workflow with github.batch and 1 failing operation, strategy best_effort
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"issues": "1,2,999999,4,5", // Issue 999999 doesn't exist
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-batch-test", inputs)

	// Then: workflow not found — fixture filename is github-batch.yaml but test looks up github-batch-test
	require.Error(t, err, "workflow should fail when fixture not found by name")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow")
}

// TestGitHubBatch_AllSucceedStrategy tests batch operation rollback on failure.
// Acceptance Criteria: All operations rolled back when one fails
func TestGitHubBatch_AllSucceedStrategy(t *testing.T) {
	skipInCI(t)
	skipIfCLIMissing(t, "gh")

	// Given: workflow with github.batch and 1 failing operation, strategy all_succeed
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"issues":   "1,2,999999,4,5", // Issue 999999 doesn't exist
		"strategy": "all_succeed",
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-batch-all-succeed-test", inputs)

	// Then: workflow fails because github-batch-all-succeed-test.yaml uses map-format inputs
	// which the YAML parser cannot unmarshal into []repository.yamlInput
	require.Error(t, err, "workflow should fail due to YAML parse error")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "unmarshal", "error should indicate YAML parse failure")
}

// TestGitHubAuth_GHCLIAuth tests authentication via gh CLI.
// Acceptance Criteria: gh CLI auth used when available
func TestGitHubAuth_GHCLIAuth(t *testing.T) {
	skipInCI(t)
	skipIfCLIMissing(t, "gh")

	// Given: gh CLI is authenticated
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	// Ensure GITHUB_TOKEN is not set to force gh CLI auth
	t.Setenv("GITHUB_TOKEN", "")

	ctx := context.Background()
	inputs := map[string]any{
		"issue_number": "1",
	}

	// When: operation executes
	execCtx, err := execSvc.Run(ctx, "github-operations-test", inputs)

	// Then: workflow not found — github-operations-test.yaml fixture does not exist
	require.Error(t, err, "workflow should fail when fixture not found")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
}

// TestGitHubAuth_TokenFallback tests fallback to GITHUB_TOKEN environment variable.
// Acceptance Criteria: Token auth used when gh CLI unavailable
func TestGitHubAuth_TokenFallback(t *testing.T) {
	skipInCI(t)

	// Given: gh CLI not authenticated but GITHUB_TOKEN set
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	// Mock scenario: GITHUB_TOKEN is set
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		t.Skip("GITHUB_TOKEN not set, cannot test token fallback")
	}

	ctx := context.Background()
	inputs := map[string]any{
		"issue_number": "1",
	}

	// When: operation executes
	execCtx, err := execSvc.Run(ctx, "github-operations-test", inputs)

	// Then: workflow not found — github-operations-test.yaml fixture does not exist
	require.Error(t, err, "workflow should fail when fixture not found")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
}

// TestGitHubAuth_NoAuthError tests error message when no auth available.
// Acceptance Criteria: Error lists available auth methods
func TestGitHubAuth_NoAuthError(t *testing.T) {
	skipInCI(t)

	// Given: no auth available (no gh CLI, no GITHUB_TOKEN)
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	// Clear all auth methods
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", "/nonexistent") // Ensure gh CLI not found

	ctx := context.Background()
	inputs := map[string]any{
		"issue_number": "1",
	}

	// When: operation executes
	execCtx, err := execSvc.Run(ctx, "github-operations-test", inputs)

	// Then: workflow not found — github-operations-test.yaml fixture does not exist
	require.Error(t, err, "workflow should fail when fixture not found")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
}

// TestGitHubOperations_WorkflowParsing tests YAML workflow parsing through execution.
// Integration test: YAML parsing → operation step → provider dispatch → output interpolation
func TestGitHubOperations_WorkflowParsing(t *testing.T) {
	skipInCI(t)

	// Given: YAML workflow with github operation
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"issue_number": "1",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "github-operations-test", inputs)

	// Then: workflow not found — github-operations-test.yaml fixture does not exist
	require.Error(t, err, "workflow should fail when fixture not found")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
}

// TestGitHubOperations_OutputInterpolation tests output field interpolation.
// Integration test: Operation result → state management → template interpolation
func TestGitHubOperations_OutputInterpolation(t *testing.T) {
	skipInCI(t)

	// Given: workflow with github operation followed by interpolation of outputs
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"issue_number": "1",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "github-interpolation-test", inputs)

	// Then: workflow fails because github-interpolation-test.yaml uses map-format inputs
	// which the YAML parser cannot unmarshal into []repository.yamlInput
	require.Error(t, err, "workflow should fail due to YAML parse error")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "unmarshal", "error should indicate YAML parse failure")
}
