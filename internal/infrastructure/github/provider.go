package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/awf-project/awf/internal/domain/plugin"
	"github.com/awf-project/awf/internal/domain/ports"
)

// GHRunner abstracts GitHub CLI command execution for testability.
// Production code uses *Client; tests use mockGHRunner.
type GHRunner interface {
	RunGH(ctx context.Context, args []string) ([]byte, error)
}

// GitHubOperationProvider implements ports.OperationProvider for GitHub operations.
// Dispatches to operation-specific handlers for the 8 GitHub operation types.
//
// Operations are organized by priority tier:
//   - P1 (Must Have): get_issue, get_pr, create_pr, create_issue
//   - P2 (Should Have): add_labels, list_comments, add_comment, batch
type GitHubOperationProvider struct {
	runner GHRunner
	logger ports.Logger

	// operations holds the registry of all 8 operation schemas
	operations map[string]*plugin.OperationSchema
}

func NewGitHubOperationProvider(runner GHRunner, logger ports.Logger) *GitHubOperationProvider {
	// Build operation registry from schema definitions
	ops := AllOperations()
	registry := make(map[string]*plugin.OperationSchema, len(ops))
	for i := range ops {
		registry[ops[i].Name] = &ops[i]
	}

	return &GitHubOperationProvider{
		runner:     runner,
		logger:     logger,
		operations: registry,
	}
}

func (p *GitHubOperationProvider) GetOperation(name string) (*plugin.OperationSchema, bool) {
	op, found := p.operations[name]
	return op, found
}

func (p *GitHubOperationProvider) ListOperations() []*plugin.OperationSchema {
	result := make([]*plugin.OperationSchema, 0, len(p.operations))
	for _, op := range p.operations {
		result = append(result, op)
	}
	return result
}

// Execute runs a GitHub operation by name with the given inputs.
// Dispatches to operation-specific handler methods based on operation name.
//
// Implements ports.OperationProvider.
func (p *GitHubOperationProvider) Execute(ctx context.Context, name string, inputs map[string]any) (*plugin.OperationResult, error) {
	if p.runner == nil {
		return nil, fmt.Errorf("github provider: runner not initialized")
	}

	if p.logger != nil {
		p.logger.Debug("executing github operation", "operation", name)
	}

	switch name {
	case "github.get_issue":
		return p.handleGetIssue(ctx, inputs)
	case "github.get_pr":
		return p.handleGetPR(ctx, inputs)
	case "github.create_pr":
		return p.handleCreatePR(ctx, inputs)
	case "github.create_issue":
		return p.handleCreateIssue(ctx, inputs)
	case "github.add_labels":
		return p.handleAddLabels(ctx, inputs)
	case "github.list_comments":
		return p.handleListComments(ctx, inputs)
	case "github.add_comment":
		return p.handleAddComment(ctx, inputs)
	case "github.batch":
		return p.handleBatch(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown operation: %s", name)
	}
}

// handleGetIssue retrieves issue information via gh CLI.
//
// Inputs:
//   - number: issue number (required)
//   - repo: repository in owner/repo format (optional, auto-detected)
//
// Outputs: JSON fields from gh CLI (number, title, body, state, labels)
func (p *GitHubOperationProvider) handleGetIssue(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	number := formatNumber(inputs["number"])
	if number == "" {
		return nil, fmt.Errorf("github.get_issue: number is required")
	}
	repo, _ := inputs["repo"].(string) // nolint:errcheck // optional input defaults to empty string
	repo = strings.TrimSpace(repo)

	args := []string{"issue", "view", number, "--json", "number,title,body,state,labels"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}

	out, err := p.runner.RunGH(ctx, args)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("github.get_issue failed", "error", err.Error(), "number", number, "repo", repo)
		}
		return &plugin.OperationResult{Success: false, Error: err.Error(), Outputs: make(map[string]any)}, fmt.Errorf("github.get_issue: %w", err)
	}

	outputs := parseJSONOutputs(out)
	return &plugin.OperationResult{Success: true, Outputs: outputs}, nil
}

// handleGetPR retrieves pull request information via gh CLI.
//
// Inputs:
//   - number: PR number (required)
//   - repo: repository in owner/repo format (optional, auto-detected)
//
// Outputs: JSON fields from gh CLI (number, title, body, state, headRefName, baseRefName, mergeable, mergedAt, labels)
func (p *GitHubOperationProvider) handleGetPR(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	number := formatNumber(inputs["number"])
	if number == "" {
		return nil, fmt.Errorf("github.get_pr: number is required")
	}
	repo, _ := inputs["repo"].(string) // nolint:errcheck // optional input defaults to empty string
	repo = strings.TrimSpace(repo)

	args := []string{"pr", "view", number, "--json", "number,title,body,state,headRefName,baseRefName,mergeable,mergedAt,labels"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}

	out, err := p.runner.RunGH(ctx, args)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("github.get_pr failed", "error", err.Error(), "number", number, "repo", repo)
		}
		return &plugin.OperationResult{Success: false, Error: err.Error(), Outputs: make(map[string]any)}, fmt.Errorf("github.get_pr: %w", err)
	}

	outputs := parseJSONOutputs(out)
	return &plugin.OperationResult{Success: true, Outputs: outputs}, nil
}

// handleCreatePR creates a GitHub pull request via gh CLI.
//
// Inputs:
//   - title: PR title (required)
//   - head: head branch name (required)
//   - base: base branch name (required)
//   - body: PR description (optional)
//   - repo: repository in owner/repo format (optional, auto-detected)
//   - draft: create as draft (optional, default false)
//
// Outputs:
//   - number: PR number (string)
//   - url: PR URL
//   - already_exists: "true" if PR already existed
func (p *GitHubOperationProvider) handleCreatePR(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	// nolint:errcheck // optional inputs default to empty string
	title, _ := inputs["title"].(string)
	title = strings.TrimSpace(title)
	head, _ := inputs["head"].(string) // nolint:errcheck // optional input defaults to empty string
	head = strings.TrimSpace(head)
	base, _ := inputs["base"].(string) // nolint:errcheck // optional input defaults to empty string
	base = strings.TrimSpace(base)
	body, _ := inputs["body"].(string) // nolint:errcheck // optional input defaults to empty string
	body = strings.TrimSpace(body)
	repo, _ := inputs["repo"].(string) // nolint:errcheck // optional input defaults to empty string
	repo = strings.TrimSpace(repo)

	if title == "" || head == "" || base == "" {
		return nil, fmt.Errorf("github.create_pr: title, head, and base are required")
	}

	// Build gh pr create args
	args := []string{
		"pr", "create",
		"--title", title,
		"--head", head,
		"--base", base,
	}

	if body != "" {
		args = append(args, "--body", body)
	}

	if repo != "" {
		args = append(args, "--repo", repo)
	}

	// Handle draft flag
	draft := parseDraftFlag(inputs["draft"])
	if draft {
		args = append(args, "--draft")
	}

	// Execute gh pr create — stdout is the PR URL
	out, err := p.runner.RunGH(ctx, args)
	if err != nil {
		errMsg := err.Error()

		// Log error for debugging
		if p.logger != nil {
			p.logger.Warn("github.create_pr failed", "error", errMsg, "title", title, "head", head, "base", base)
		}

		// Check if PR already exists
		if strings.Contains(errMsg, "already exists") {
			if p.logger != nil {
				p.logger.Info("PR already exists, fetching existing PR info", "head", head, "base", base)
			}
			return p.fetchExistingPR(ctx, head, repo)
		}

		return &plugin.OperationResult{
			Success: false,
			Error:   errMsg,
			Outputs: map[string]any{
				"number":         "",
				"url":            "",
				"already_exists": "",
			},
		}, fmt.Errorf("github.create_pr: %w", err)
	}

	prURL := strings.TrimSpace(string(out))
	prNumber := extractPRNumber(prURL)

	if p.logger != nil {
		p.logger.Info("PR created", "url", prURL, "number", prNumber)
	}

	return &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{
			"number":         prNumber,
			"url":            prURL,
			"already_exists": "false",
		},
	}, nil
}

// handleCreateIssue creates a GitHub issue via gh CLI.
//
// Inputs:
//   - title: issue title (required)
//   - body: issue body (optional)
//   - repo: repository in owner/repo format (optional, auto-detected)
//   - labels: array of label names (optional)
//   - assignees: array of assignee usernames (optional)
//
// Outputs:
//   - number: issue number (string)
//   - url: issue URL
func (p *GitHubOperationProvider) handleCreateIssue(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	title, _ := inputs["title"].(string) // nolint:errcheck // required input validated below
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("github.create_issue: title is required")
	}
	body, _ := inputs["body"].(string) // nolint:errcheck // optional input defaults to empty string
	body = strings.TrimSpace(body)
	repo, _ := inputs["repo"].(string) // nolint:errcheck // optional input defaults to empty string
	repo = strings.TrimSpace(repo)

	args := []string{"issue", "create", "--title", title}
	if body != "" {
		args = append(args, "--body", body)
	}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	// Handle labels
	if labels := toStringSlice(inputs["labels"]); len(labels) > 0 {
		for _, l := range labels {
			args = append(args, "--label", l)
		}
	}
	// Handle assignees
	if assignees := toStringSlice(inputs["assignees"]); len(assignees) > 0 {
		for _, a := range assignees {
			args = append(args, "--assignee", a)
		}
	}

	out, err := p.runner.RunGH(ctx, args)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("github.create_issue failed", "error", err.Error(), "title", title, "repo", repo)
		}
		return &plugin.OperationResult{Success: false, Error: err.Error(), Outputs: make(map[string]any)}, fmt.Errorf("github.create_issue: %w", err)
	}

	issueURL := strings.TrimSpace(string(out))
	issueNumber := extractPRNumber(issueURL) // Same URL format

	return &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{"number": issueNumber, "url": issueURL},
	}, nil
}

// handleAddLabels adds labels to an issue or PR via gh CLI.
//
// Inputs:
//   - number: issue/PR number (required)
//   - labels: array of label names to add (required)
//   - repo: repository in owner/repo format (optional, auto-detected)
//
// Outputs:
//   - labels: comma-separated list of added labels
func (p *GitHubOperationProvider) handleAddLabels(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	number := formatNumber(inputs["number"])
	if number == "" {
		return nil, fmt.Errorf("github.add_labels: number is required")
	}
	labels := toStringSlice(inputs["labels"])
	if len(labels) == 0 {
		return nil, fmt.Errorf("github.add_labels: labels is required")
	}
	repo, _ := inputs["repo"].(string) //nolint:errcheck // optional input defaults to empty string
	repo = strings.TrimSpace(repo)

	args := []string{"issue", "edit", number}
	for _, l := range labels {
		args = append(args, "--add-label", l)
	}
	if repo != "" {
		args = append(args, "--repo", repo)
	}

	_, err := p.runner.RunGH(ctx, args)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("github.add_labels failed", "error", err.Error(), "number", number, "repo", repo)
		}
		return &plugin.OperationResult{Success: false, Error: err.Error(), Outputs: make(map[string]any)}, fmt.Errorf("github.add_labels: %w", err)
	}

	return &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{"labels": strings.Join(labels, ",")},
	}, nil
}

// handleListComments retrieves comments for an issue or PR via gh CLI.
//
// Inputs:
//   - number: issue/PR number (required)
//   - repo: repository in owner/repo format (optional, auto-detected)
//
// Outputs: JSON fields from gh CLI (comments array)
func (p *GitHubOperationProvider) handleListComments(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	number := formatNumber(inputs["number"])
	if number == "" {
		return nil, fmt.Errorf("github.list_comments: number is required")
	}
	repo, _ := inputs["repo"].(string) //nolint:errcheck // optional input defaults to empty string
	repo = strings.TrimSpace(repo)

	args := []string{"issue", "view", number, "--json", "comments"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}

	out, err := p.runner.RunGH(ctx, args)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("github.list_comments failed", "error", err.Error(), "number", number, "repo", repo)
		}
		return &plugin.OperationResult{Success: false, Error: err.Error(), Outputs: make(map[string]any)}, fmt.Errorf("github.list_comments: %w", err)
	}

	outputs := parseJSONOutputs(out)
	return &plugin.OperationResult{Success: true, Outputs: outputs}, nil
}

// handleAddComment adds a comment to an issue or PR via gh CLI.
//
// Inputs:
//   - number: issue/PR number (required)
//   - body: comment body (required)
//   - repo: repository in owner/repo format (optional, auto-detected)
//
// Outputs:
//   - comment_id: comment ID (empty string, gh CLI doesn't return it)
//   - url: comment URL
func (p *GitHubOperationProvider) handleAddComment(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	number := formatNumber(inputs["number"])
	if number == "" {
		return nil, fmt.Errorf("github.add_comment: number is required")
	}
	body, _ := inputs["body"].(string) //nolint:errcheck // required input validated below
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("github.add_comment: body is required")
	}
	repo, _ := inputs["repo"].(string) //nolint:errcheck // optional input defaults to empty string
	repo = strings.TrimSpace(repo)

	args := []string{"issue", "comment", number, "--body", body}
	if repo != "" {
		args = append(args, "--repo", repo)
	}

	out, err := p.runner.RunGH(ctx, args)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("github.add_comment failed", "error", err.Error(), "number", number, "repo", repo)
		}
		return &plugin.OperationResult{Success: false, Error: err.Error(), Outputs: make(map[string]any)}, fmt.Errorf("github.add_comment: %w", err)
	}

	commentURL := strings.TrimSpace(string(out))
	return &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{"comment_id": "", "url": commentURL},
	}, nil
}

// handleBatch executes multiple GitHub operations in batch with configurable concurrency.
//
// Inputs:
//   - operations: array of operation specs (required)
//   - strategy: execution strategy (all_succeed, any_succeed, continue_on_error)
//   - concurrency: max concurrent operations (optional, default 3)
//
// Outputs:
//   - total: total operations count
//   - succeeded: count of successful operations
//   - failed: count of failed operations
//   - results: array of operation results
func (p *GitHubOperationProvider) handleBatch(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	opsRaw, _ := inputs["operations"].([]map[string]any) //nolint:errcheck // zero-value is acceptable
	if len(opsRaw) == 0 {
		// Try []any conversion (YAML/JSON deserializes as []any)
		if opsAny, ok := inputs["operations"].([]any); ok {
			opsRaw = make([]map[string]any, 0, len(opsAny))
			for _, o := range opsAny {
				if m, ok := o.(map[string]any); ok {
					opsRaw = append(opsRaw, m)
				}
			}
		}
	}

	strategy, _ := inputs["strategy"].(string) //nolint:errcheck // defaults to empty string
	maxConcurrent := 3
	if mc, ok := inputs["concurrency"].(int); ok && mc > 0 {
		maxConcurrent = mc
	}

	executor := NewBatchExecutor(p, nil)
	batchResult, err := executor.Execute(ctx, opsRaw, BatchConfig{
		Strategy:      strategy,
		MaxConcurrent: maxConcurrent,
	})
	if err != nil {
		return &plugin.OperationResult{
			Success: false,
			Error:   err.Error(),
			Outputs: map[string]any{"total": 0, "succeeded": 0, "failed": 0, "results": []any{}},
		}, fmt.Errorf("github.batch: %w", err)
	}

	return &plugin.OperationResult{
		Success: batchResult.Failed == 0,
		Outputs: map[string]any{
			"total":     batchResult.Total,
			"succeeded": batchResult.Succeeded,
			"failed":    batchResult.Failed,
			"results":   batchResult.Results,
		},
	}, nil
}

// fetchExistingPR retrieves info for an existing PR on the given head branch.
func (p *GitHubOperationProvider) fetchExistingPR(ctx context.Context, head, repo string) (*plugin.OperationResult, error) {
	args := []string{"pr", "view", head, "--json", "number,url"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}

	out, err := p.runner.RunGH(ctx, args)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("fetchExistingPR failed", "error", err.Error(), "head", head, "repo", repo)
		}
		return &plugin.OperationResult{
			Success: false,
			Error:   fmt.Sprintf("fetch existing PR: %s", err),
			Outputs: map[string]any{
				"number":         "",
				"url":            "",
				"already_exists": "",
			},
		}, nil
	}

	// Parse JSON: {"number":123,"url":"..."}
	number, url := parseNumberAndURL(string(out))

	return &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{
			"number":         number,
			"url":            url,
			"already_exists": "true",
		},
	}, nil
}

// extractPRNumber extracts the PR number from a GitHub PR URL.
// e.g., "https://github.com/owner/repo/pull/123" → "123"
func extractPRNumber(prURL string) string {
	parts := strings.Split(strings.TrimRight(prURL, "/\n"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// parseNumberAndURL extracts number and url from gh pr view JSON output.
func parseNumberAndURL(jsonOutput string) (number, url string) {
	// Simple extraction without encoding/json to keep imports minimal.
	// Format: {"number":123,"url":"https://..."}

	if idx := strings.Index(jsonOutput, `"number":`); idx >= 0 {
		rest := jsonOutput[idx+len(`"number":`):]
		rest = strings.TrimLeft(rest, " ")
		end := strings.IndexAny(rest, ",}")
		if end > 0 {
			number = strings.TrimSpace(rest[:end])
		}
	}

	if idx := strings.Index(jsonOutput, `"url":"`); idx >= 0 {
		rest := jsonOutput[idx+len(`"url":"`):]
		end := strings.Index(rest, `"`)
		if end > 0 {
			url = rest[:end]
		}
	}

	return number, url
}

// parseDraftFlag interprets the draft input which can be bool, "true"/"false", or "draft"/"open".
func parseDraftFlag(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "draft"
	default:
		return false
	}
}

// formatNumber converts various numeric input types to string for gh CLI.
func formatNumber(v any) string {
	switch val := v.(type) {
	case int:
		return strconv.Itoa(val)
	case float64:
		return strconv.Itoa(int(val))
	case string:
		return val
	default:
		return ""
	}
}

// toStringSlice converts various input types to []string.
func toStringSlice(v any) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// parseJSONOutputs parses gh CLI JSON output into a map.
func parseJSONOutputs(data []byte) map[string]any {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]any{"raw": string(data)}
	}
	return result
}
