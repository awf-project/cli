package github

import "github.com/awf-project/cli/internal/domain/pluginmodel"

// AllOperations returns all 8 GitHub operation schemas that can be registered
// with the OperationRegistry. Each operation defines its input/output schema,
// validation rules, and field selection support.
//
// Operations are organized by priority tier (P1, P2, P3) matching the spec:
//   - P1 (Must Have): get_issue, get_pr, create_pr, create_issue
//   - P2 (Should Have): add_labels, list_comments, add_comment, batch
func AllOperations() []pluginmodel.OperationSchema {
	return []pluginmodel.OperationSchema{
		// P1: Get Issue - Retrieve GitHub issue data
		{
			Name:        "github.get_issue",
			Description: "Retrieve GitHub issue data",
			Inputs: map[string]pluginmodel.InputSchema{
				"number": {Type: pluginmodel.InputTypeInteger, Required: true, Description: "Issue number"},
				"repo":   {Type: pluginmodel.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
				"fields": {Type: pluginmodel.InputTypeArray, Required: false, Description: "Fields to include in output (limits data returned)"},
			},
			Outputs: []string{"number", "title", "body", "state", "labels"},
		},

		// P1: Get Pull Request - Retrieve GitHub PR data
		{
			Name:        "github.get_pr",
			Description: "Retrieve GitHub pull request data",
			Inputs: map[string]pluginmodel.InputSchema{
				"number": {Type: pluginmodel.InputTypeInteger, Required: true, Description: "PR number"},
				"repo":   {Type: pluginmodel.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
				"fields": {Type: pluginmodel.InputTypeArray, Required: false, Description: "Fields to include in output (limits data returned)"},
			},
			Outputs: []string{"number", "title", "body", "state", "headRefName", "baseRefName", "mergeable", "mergedAt", "labels"},
		},

		// P1: Create Pull Request - Create a new GitHub PR
		{
			Name:        "github.create_pr",
			Description: "Create a new GitHub pull request",
			Inputs: map[string]pluginmodel.InputSchema{
				"title": {Type: pluginmodel.InputTypeString, Required: true, Description: "PR title"},
				"head":  {Type: pluginmodel.InputTypeString, Required: true, Description: "Head branch name"},
				"base":  {Type: pluginmodel.InputTypeString, Required: true, Description: "Base branch name"},
				"body":  {Type: pluginmodel.InputTypeString, Required: false, Description: "PR body/description"},
				"repo":  {Type: pluginmodel.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
				"draft": {Type: pluginmodel.InputTypeBoolean, Required: false, Description: "Create as draft PR"},
			},
			Outputs: []string{"number", "url", "already_exists"},
		},

		// P1: Create Issue - Create a new GitHub issue
		{
			Name:        "github.create_issue",
			Description: "Create a new GitHub issue",
			Inputs: map[string]pluginmodel.InputSchema{
				"title":     {Type: pluginmodel.InputTypeString, Required: true, Description: "Issue title"},
				"body":      {Type: pluginmodel.InputTypeString, Required: false, Description: "Issue body/description"},
				"repo":      {Type: pluginmodel.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
				"labels":    {Type: pluginmodel.InputTypeArray, Required: false, Description: "Labels to add"},
				"assignees": {Type: pluginmodel.InputTypeArray, Required: false, Description: "Usernames to assign"},
			},
			Outputs: []string{"number", "url"},
		},

		// P2: Add Labels - Add labels to an issue or PR
		{
			Name:        "github.add_labels",
			Description: "Add labels to a GitHub issue or PR",
			Inputs: map[string]pluginmodel.InputSchema{
				"number": {Type: pluginmodel.InputTypeInteger, Required: true, Description: "Issue or PR number"},
				"labels": {Type: pluginmodel.InputTypeArray, Required: true, Description: "Labels to add"},
				"repo":   {Type: pluginmodel.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
			},
			Outputs: []string{"labels"},
		},

		// P2: List Comments - List comments on an issue or PR
		{
			Name:        "github.list_comments",
			Description: "List comments on a GitHub issue or PR",
			Inputs: map[string]pluginmodel.InputSchema{
				"number": {Type: pluginmodel.InputTypeInteger, Required: true, Description: "Issue or PR number"},
				"repo":   {Type: pluginmodel.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
				"limit":  {Type: pluginmodel.InputTypeInteger, Required: false, Description: "Maximum number of comments to return"},
			},
			Outputs: []string{"comments", "total"},
		},

		// P2: Add Comment - Add a comment to an issue or PR
		{
			Name:        "github.add_comment",
			Description: "Add a comment to a GitHub issue or PR",
			Inputs: map[string]pluginmodel.InputSchema{
				"number": {Type: pluginmodel.InputTypeInteger, Required: true, Description: "Issue or PR number"},
				"body":   {Type: pluginmodel.InputTypeString, Required: true, Description: "Comment body text"},
				"repo":   {Type: pluginmodel.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
			},
			Outputs: []string{"comment_id", "url"},
		},

		// P2: Batch Operations - Execute multiple GitHub operations in batch
		{
			Name:        "github.batch",
			Description: "Execute multiple GitHub operations in batch",
			Inputs: map[string]pluginmodel.InputSchema{
				"operations":  {Type: pluginmodel.InputTypeArray, Required: true, Description: "Array of operation definitions to execute"},
				"strategy":    {Type: pluginmodel.InputTypeString, Required: false, Description: "Execution strategy: all_succeed, any_succeed, best_effort (default: best_effort)"},
				"concurrency": {Type: pluginmodel.InputTypeInteger, Required: false, Description: "Maximum concurrent operations (default: 3)"},
			},
			Outputs: []string{"total", "succeeded", "failed", "results"},
		},
	}
}
