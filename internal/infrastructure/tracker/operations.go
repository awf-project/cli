package tracker

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
)

const (
	createIssueOperation = "tracker.create_issue"
	updateIssueOperation = "tracker.update_issue"
	transitionOperation  = "tracker.transition"
	addCommentOperation  = "tracker.add_comment"
	getIssueOperation    = "tracker.get_issue"
	searchOperation      = "tracker.search"
)

// ErrInvalidTrackerInput identifies invalid canonical tracker input.
var ErrInvalidTrackerInput = errors.New("invalid tracker input")

// OperationSchemas returns the provider-independent tracker operation contract.
func OperationSchemas() []*pluginmodel.OperationSchema {
	return []*pluginmodel.OperationSchema{
		operationSchema(
			createIssueOperation,
			"Create an issue in the configured tracker backend",
			map[string]pluginmodel.InputSchema{
				"backend":     stringInput(false, "Tracker backend namespace"),
				"project":     stringInput(false, "Project identifier"),
				"team":        stringInput(false, "Team identifier"),
				"type":        stringInput(false, "Issue type"),
				"title":       stringInput(true, "Issue title"),
				"description": stringInput(false, "Issue description"),
				"priority":    stringInput(false, "Issue priority"),
				"assignee":    stringInput(false, "Issue assignee"),
				"labels":      arrayInput("Issue labels"),
			},
			[]string{"id", "key", "url"},
		),
		operationSchema(
			updateIssueOperation,
			"Update an issue in the configured tracker backend",
			map[string]pluginmodel.InputSchema{
				"backend":     stringInput(false, "Tracker backend namespace"),
				"id":          stringInput(false, "Provider issue identifier"),
				"key":         stringInput(false, "Human-readable issue key"),
				"title":       stringInput(false, "Issue title"),
				"description": stringInput(false, "Issue description"),
				"priority":    stringInput(false, "Issue priority"),
				"assignee":    stringInput(false, "Issue assignee"),
				"state":       stringInput(false, "Issue state"),
				"labels":      arrayInput("Issue labels"),
			},
			[]string{"id", "key"},
		),
		operationSchema(
			transitionOperation,
			"Transition an issue in the configured tracker backend",
			map[string]pluginmodel.InputSchema{
				"backend":    stringInput(false, "Tracker backend namespace"),
				"id":         stringInput(false, "Provider issue identifier"),
				"key":        stringInput(false, "Human-readable issue key"),
				"transition": stringInput(true, "Transition to apply"),
			},
			[]string{"id", "key", "status"},
		),
		operationSchema(
			addCommentOperation,
			"Add a comment to an issue in the configured tracker backend",
			map[string]pluginmodel.InputSchema{
				"backend": stringInput(false, "Tracker backend namespace"),
				"id":      stringInput(false, "Provider issue identifier"),
				"key":     stringInput(false, "Human-readable issue key"),
				"body":    stringInput(true, "Comment body"),
			},
			[]string{"comment_id"},
		),
		operationSchema(
			getIssueOperation,
			"Get an issue from the configured tracker backend",
			map[string]pluginmodel.InputSchema{
				"backend": stringInput(false, "Tracker backend namespace"),
				"id":      stringInput(false, "Provider issue identifier"),
				"key":     stringInput(false, "Human-readable issue key"),
			},
			[]string{"id", "key", "title", "status", "description", "assignee", "url"},
		),
		operationSchema(
			searchOperation,
			"Search issues in the configured tracker backend",
			map[string]pluginmodel.InputSchema{
				"backend":     stringInput(false, "Tracker backend namespace"),
				"project":     stringInput(false, "Project identifier"),
				"team":        stringInput(false, "Team identifier"),
				"state":       stringInput(false, "Issue state"),
				"query":       stringInput(true, "Search query"),
				"max_results": integerInput("Maximum number of results"),
			},
			[]string{"issues", "total"},
		),
	}
}

// AdapterOperationName derives the operation name exposed by a tracker adapter.
func AdapterOperationName(backend, operation string) (string, error) {
	if backend == "" || strings.ContainsRune(backend, '.') || strings.IndexFunc(backend, unicode.IsSpace) >= 0 {
		return "", fmt.Errorf("backend: %w", ErrInvalidTrackerInput)
	}

	localOperation, ok := localOperationName(operation)
	if !ok {
		return "", fmt.Errorf("operation: %w", ErrInvalidTrackerInput)
	}

	return "tracker-" + backend + "." + localOperation, nil
}

func operationSchema(
	name string,
	description string,
	inputs map[string]pluginmodel.InputSchema,
	outputs []string,
) *pluginmodel.OperationSchema {
	return &pluginmodel.OperationSchema{
		Name:        name,
		Description: description,
		Inputs:      inputs,
		Outputs:     outputs,
		PluginName:  "tracker",
	}
}

func stringInput(required bool, description string) pluginmodel.InputSchema {
	return pluginmodel.InputSchema{
		Type:        pluginmodel.InputTypeString,
		Required:    required,
		Description: description,
	}
}

func arrayInput(description string) pluginmodel.InputSchema {
	return pluginmodel.InputSchema{Type: pluginmodel.InputTypeArray, Description: description}
}

func integerInput(description string) pluginmodel.InputSchema {
	return pluginmodel.InputSchema{Type: pluginmodel.InputTypeInteger, Description: description}
}

func localOperationName(operation string) (string, bool) {
	switch operation {
	case createIssueOperation, updateIssueOperation, transitionOperation,
		addCommentOperation, getIssueOperation, searchOperation:
		return strings.TrimPrefix(operation, "tracker."), true
	default:
		return "", false
	}
}
