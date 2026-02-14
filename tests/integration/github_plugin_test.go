//go:build integration

// Feature: F054 - GitHub CLI Plugin for Declarative Operations
// This file contains integration tests for the GitHub operation provider.

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
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

	// Then: issue fields available as outputs
	require.NoError(t, err, "workflow execution should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify issue data is accessible
	state, exists := execCtx.GetStepState("test_get_issue")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	// Expected outputs: title, body, labels, state
	assert.Contains(t, state.Response, "title")
	assert.Contains(t, state.Response, "body")
	assert.Contains(t, state.Response, "labels")
	assert.Contains(t, state.Response, "state")
	assert.NotEmpty(t, state.Response["title"], "issue title should not be empty")
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

	// Then: error type github_not_found returned
	require.Error(t, err, "workflow should fail with not found error")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Contains(t, err.Error(), "github_not_found", "error should have github_not_found type")
	assert.Contains(t, err.Error(), "999999", "error should mention the invalid issue number")
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

	// Then: error type github_auth_error with remediation hint
	require.Error(t, err, "workflow should fail with auth error")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Contains(t, err.Error(), "github_auth_error", "error should have github_auth_error type")
	assert.Contains(t, err.Error(), "auth", "error should contain auth-related message")
	// Remediation hint should mention available auth methods
	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "gh") || strings.Contains(errorMsg, "GITHUB_TOKEN"),
		"error should mention available auth methods (gh CLI or GITHUB_TOKEN)",
	)
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

	// Then: PR created and URL/number available as outputs
	require.NoError(t, err, "PR creation should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify PR data
	state, exists := execCtx.GetStepState("create_pr")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	assert.Contains(t, state.Response, "url", "PR URL should be in output")
	assert.Contains(t, state.Response, "number", "PR number should be in output")
	assert.NotEmpty(t, state.Response["url"], "PR URL should not be empty")
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

	// Then: error type github_branch_not_found returned
	require.Error(t, err, "workflow should fail with branch not found error")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Contains(t, err.Error(), "github_branch_not_found", "error should have github_branch_not_found type")
	assert.Contains(t, err.Error(), "nonexistent-branch-12345", "error should mention the missing branch")
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

	// Create PR first time
	execCtx1, err := execSvc.Run(ctx, "github-pr-test", inputs)
	require.NoError(t, err, "first PR creation should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx1.Status)

	// When: attempt to create PR again with same branch
	execCtx2, err := execSvc.Run(ctx, "github-pr-test", inputs)

	// Then: existing PR URL returned with already_exists flag
	require.NoError(t, err, "second execution should not error")
	assert.Equal(t, workflow.StatusCompleted, execCtx2.Status)

	state, exists := execCtx2.GetStepState("create_pr")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	assert.Contains(t, state.Response, "url", "PR URL should be in output")
	assert.Contains(t, state.Response, "already_exists", "already_exists flag should be present")
	assert.True(t, state.Response["already_exists"].(bool), "already_exists should be true")
}

// TestGitHubSetProjectStatus_Success tests setting project field via github.set_project_status.
// Acceptance Criteria: Issue's project status updated
func TestGitHubSetProjectStatus_Success(t *testing.T) {
	skipInCI(t)
	skipIfCLIMissing(t, "gh")

	// Given: workflow with github.set_project_status specifying issue, project, status
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"issue":   "1",
		"project": "Test Project",
		"status":  "In Progress",
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-project-test", inputs)

	// Then: issue's project status updated
	require.NoError(t, err, "status update should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("set_status")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	assert.Contains(t, state.Response, "updated", "output should indicate update status")
	assert.True(t, state.Response["updated"].(bool), "update should be successful")
}

// TestGitHubSetProjectStatus_AddToProject tests adding issue to project before setting status.
// Acceptance Criteria: Issue added to project then status set
func TestGitHubSetProjectStatus_AddToProject(t *testing.T) {
	skipInCI(t)
	skipIfCLIMissing(t, "gh")

	// Given: workflow with github.set_project_status and issue not in project
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"issue":   "2", // Issue not yet in project
		"project": "Test Project",
		"status":  "Todo",
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-project-test", inputs)

	// Then: issue added to project then status set
	require.NoError(t, err, "should succeed after adding to project")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("set_status")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	assert.Contains(t, state.Response, "added_to_project", "output should indicate if added to project")
	assert.True(t, state.Response["added_to_project"].(bool), "issue should be added to project")
	assert.True(t, state.Response["updated"].(bool), "status should be updated")
}

// TestGitHubSetProjectStatus_InvalidValue tests error handling for invalid status value.
// Acceptance Criteria: Error includes valid options list
func TestGitHubSetProjectStatus_InvalidValue(t *testing.T) {
	skipInCI(t)

	// Given: workflow with github.set_project_status and invalid status value
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupTestWorkflowService(t, workflowsDir, statesDir)

	ctx := context.Background()
	inputs := map[string]any{
		"issue":   "1",
		"project": "Test Project",
		"status":  "InvalidStatusValue",
	}

	// When: step executes
	execCtx, err := execSvc.Run(ctx, "github-project-test", inputs)

	// Then: error includes valid options list
	require.Error(t, err, "should fail with invalid status value")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	errorMsg := err.Error()
	assert.Contains(t, errorMsg, "invalid", "error should mention invalid status")
	assert.Contains(t, errorMsg, "InvalidStatusValue", "error should mention the invalid value")
	// Should include valid options
	assert.True(t,
		strings.Contains(errorMsg, "Todo") || strings.Contains(errorMsg, "In Progress") || strings.Contains(errorMsg, "Done"),
		"error should list valid status options",
	)
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

	// Then: all 5 issues labeled and output contains success count
	require.NoError(t, err, "batch operation should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("test_batch")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	assert.Contains(t, state.Response, "total", "output should contain total count")
	assert.Contains(t, state.Response, "succeeded", "output should contain succeeded count")
	assert.Contains(t, state.Response, "failed", "output should contain failed count")
	assert.Equal(t, 5, state.Response["total"], "total should be 5")
	assert.Equal(t, 5, state.Response["succeeded"], "all should succeed")
	assert.Equal(t, 0, state.Response["failed"], "none should fail")
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

	// Then: successful operations complete, output shows partial results
	require.NoError(t, err, "best_effort should not error even with partial failure")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("test_batch")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	assert.Equal(t, 5, state.Response["total"], "total should be 5")
	assert.Equal(t, 4, state.Response["succeeded"], "4 should succeed")
	assert.Equal(t, 1, state.Response["failed"], "1 should fail")
	assert.Contains(t, state.Response, "results", "should contain detailed results")
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

	// Then: all operations rolled back where possible
	require.Error(t, err, "all_succeed strategy should error on any failure")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Contains(t, err.Error(), "rollback", "error should mention rollback")

	state, exists := execCtx.GetStepState("test_batch")
	// State might not exist if rollback succeeded
	if exists && state.Response != nil {
		assert.Equal(t, 0, state.Response["succeeded"], "rollback should undo all operations")
	}
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

	// Then: gh CLI auth is used (workflow succeeds without token)
	require.NoError(t, err, "should succeed using gh CLI auth")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("test_get_issue")
	require.True(t, exists)
	require.NotNil(t, state.Response)
	assert.Contains(t, state.Response, "title", "issue data should be retrieved")
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

	// Then: token auth used (workflow succeeds)
	require.NoError(t, err, "should succeed using GITHUB_TOKEN")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("test_get_issue")
	require.True(t, exists)
	require.NotNil(t, state.Response)
	assert.Contains(t, state.Response, "title", "issue data should be retrieved with token auth")
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

	// Then: error lists available auth methods
	require.Error(t, err, "should fail when no auth available")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	errorMsg := err.Error()
	assert.Contains(t, errorMsg, "auth", "error should mention authentication")
	// Should list available auth methods
	assert.True(t,
		strings.Contains(errorMsg, "gh") && strings.Contains(errorMsg, "GITHUB_TOKEN"),
		"error should list both gh CLI and GITHUB_TOKEN as auth options",
	)
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

	// Then: operation dispatched and output interpolated
	require.NoError(t, err, "workflow should parse and execute successfully")
	require.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify workflow parsed github.get_issue operation correctly
	state, exists := execCtx.GetStepState("test_get_issue")
	require.True(t, exists, "github operation step should exist")
	require.NotNil(t, state, "state should be saved")
	require.NotNil(t, state.Response, "operation should have response")
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

	// Then: outputs correctly interpolated in subsequent steps
	require.NoError(t, err, "workflow should complete successfully")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify second step used interpolated output from first step
	state, exists := execCtx.GetStepState("use_issue_data")
	require.True(t, exists)
	require.NotNil(t, state)

	// Second step should have access to first step's response via {{states.test_get_issue.response.title}}
	assert.NotNil(t, state.Response, "interpolation step should have response")
	assert.Contains(t, state.Response, "issue_title_from_prev_step", "should contain interpolated field")
	assert.NotEmpty(t, state.Response["issue_title_from_prev_step"], "interpolated value should not be empty")
}
