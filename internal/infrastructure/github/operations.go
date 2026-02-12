package github

import "github.com/vanoix/awf/internal/domain/plugin"

// AllOperations returns all 9 GitHub operation schemas that can be registered
// with the OperationRegistry. Each operation defines its input/output schema,
// validation rules, and field selection support.
//
// Operations are organized by priority tier (P1, P2, P3) matching the spec:
//   - P1 (Must Have): get_issue, get_pr, create_pr, create_issue
//   - P2 (Should Have): add_labels, set_project_status, list_comments, add_comment, batch
func AllOperations() []plugin.OperationSchema {
	return []plugin.OperationSchema{
		// P1: Get Issue - Retrieve GitHub issue data
		{
			Name:        "github.get_issue",
			Description: "Retrieve GitHub issue data",
			Inputs: map[string]plugin.InputSchema{
				"number": {Type: plugin.InputTypeInteger, Required: true, Description: "Issue number"},
				"repo":   {Type: plugin.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
				"fields": {Type: plugin.InputTypeArray, Required: false, Description: "Fields to include in output (limits data returned)"},
			},
			Outputs: []string{"number", "title", "body", "state", "labels"},
		},

		// P1: Get Pull Request - Retrieve GitHub PR data
		{
			Name:        "github.get_pr",
			Description: "Retrieve GitHub pull request data",
			Inputs: map[string]plugin.InputSchema{
				"number": {Type: plugin.InputTypeInteger, Required: true, Description: "PR number"},
				"repo":   {Type: plugin.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
				"fields": {Type: plugin.InputTypeArray, Required: false, Description: "Fields to include in output (limits data returned)"},
			},
			Outputs: []string{"number", "title", "body", "state", "headRefName", "baseRefName", "mergeable", "mergedAt", "labels"},
		},

		// P1: Create Pull Request - Create a new GitHub PR
		{
			Name:        "github.create_pr",
			Description: "Create a new GitHub pull request",
			Inputs: map[string]plugin.InputSchema{
				"title": {Type: plugin.InputTypeString, Required: true, Description: "PR title"},
				"head":  {Type: plugin.InputTypeString, Required: true, Description: "Head branch name"},
				"base":  {Type: plugin.InputTypeString, Required: true, Description: "Base branch name"},
				"body":  {Type: plugin.InputTypeString, Required: false, Description: "PR body/description"},
				"repo":  {Type: plugin.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
				"draft": {Type: plugin.InputTypeBoolean, Required: false, Description: "Create as draft PR"},
			},
			Outputs: []string{"number", "url", "already_exists"},
		},

		// P1: Create Issue - Create a new GitHub issue
		{
			Name:        "github.create_issue",
			Description: "Create a new GitHub issue",
			Inputs: map[string]plugin.InputSchema{
				"title":     {Type: plugin.InputTypeString, Required: true, Description: "Issue title"},
				"body":      {Type: plugin.InputTypeString, Required: false, Description: "Issue body/description"},
				"repo":      {Type: plugin.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
				"labels":    {Type: plugin.InputTypeArray, Required: false, Description: "Labels to add"},
				"assignees": {Type: plugin.InputTypeArray, Required: false, Description: "Usernames to assign"},
			},
			Outputs: []string{"number", "url"},
		},

		// P2: Add Labels - Add labels to an issue or PR
		{
			Name:        "github.add_labels",
			Description: "Add labels to a GitHub issue or PR",
			Inputs: map[string]plugin.InputSchema{
				"number": {Type: plugin.InputTypeInteger, Required: true, Description: "Issue or PR number"},
				"labels": {Type: plugin.InputTypeArray, Required: true, Description: "Labels to add"},
				"repo":   {Type: plugin.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
			},
			Outputs: []string{"labels"},
		},

		// P2: Set Project Status - Set GitHub Project field value
		{
			Name:        "github.set_project_status",
			Description: "Set GitHub Project field value for an issue or PR",
			Inputs: map[string]plugin.InputSchema{
				"number":  {Type: plugin.InputTypeInteger, Required: true, Description: "Issue or PR number"},
				"project": {Type: plugin.InputTypeString, Required: true, Description: "Project identifier"},
				"field":   {Type: plugin.InputTypeString, Required: true, Description: "Field name to update"},
				"value":   {Type: plugin.InputTypeString, Required: true, Description: "New field value"},
				"repo":    {Type: plugin.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
			},
			Outputs: []string{"project_id", "item_id", "field_name", "value"},
		},

		// P2: List Comments - List comments on an issue or PR
		{
			Name:        "github.list_comments",
			Description: "List comments on a GitHub issue or PR",
			Inputs: map[string]plugin.InputSchema{
				"number": {Type: plugin.InputTypeInteger, Required: true, Description: "Issue or PR number"},
				"repo":   {Type: plugin.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
				"limit":  {Type: plugin.InputTypeInteger, Required: false, Description: "Maximum number of comments to return"},
			},
			Outputs: []string{"comments", "total"},
		},

		// P2: Add Comment - Add a comment to an issue or PR
		{
			Name:        "github.add_comment",
			Description: "Add a comment to a GitHub issue or PR",
			Inputs: map[string]plugin.InputSchema{
				"number": {Type: plugin.InputTypeInteger, Required: true, Description: "Issue or PR number"},
				"body":   {Type: plugin.InputTypeString, Required: true, Description: "Comment body text"},
				"repo":   {Type: plugin.InputTypeString, Required: false, Description: "Repository (owner/repo format, auto-detected if omitted)"},
			},
			Outputs: []string{"comment_id", "url"},
		},

		// P2: Batch Operations - Execute multiple GitHub operations in batch
		{
			Name:        "github.batch",
			Description: "Execute multiple GitHub operations in batch",
			Inputs: map[string]plugin.InputSchema{
				"operations":  {Type: plugin.InputTypeArray, Required: true, Description: "Array of operation definitions to execute"},
				"strategy":    {Type: plugin.InputTypeString, Required: false, Description: "Execution strategy: all_succeed, any_succeed, best_effort (default: best_effort)"},
				"concurrency": {Type: plugin.InputTypeInteger, Required: false, Description: "Maximum concurrent operations (default: 3)"},
			},
			Outputs: []string{"total", "succeeded", "failed", "results"},
		},
	}
}
