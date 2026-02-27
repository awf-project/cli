package workflow_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
)

func TestWorkflow_SourceDir_FieldExists(t *testing.T) {
	wf := workflow.Workflow{
		Name:      "test-workflow",
		Initial:   "start",
		SourceDir: "/home/user/.config/awf/workflows",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}

	assert.Equal(t, "/home/user/.config/awf/workflows", wf.SourceDir)
}

func TestWorkflow_SourceDir_EmptyByDefault(t *testing.T) {
	wf := workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}

	assert.Empty(t, wf.SourceDir)
}

func TestWorkflow_SourceDir_CanStoreRelativePath(t *testing.T) {
	wf := workflow.Workflow{
		Name:      "test-workflow",
		Initial:   "start",
		SourceDir: ".awf/workflows",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}

	assert.Equal(t, ".awf/workflows", wf.SourceDir)
}

func TestWorkflow_SourceDir_CanStoreAbsolutePath(t *testing.T) {
	wf := workflow.Workflow{
		Name:      "test-workflow",
		Initial:   "start",
		SourceDir: "/opt/workflows/production",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}

	assert.Equal(t, "/opt/workflows/production", wf.SourceDir)
}

func TestWorkflow_SourceDir_HandlesPathsWithSpaces(t *testing.T) {
	wf := workflow.Workflow{
		Name:      "test-workflow",
		Initial:   "start",
		SourceDir: "/home/user/my workflows/project 123",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}

	assert.Equal(t, "/home/user/my workflows/project 123", wf.SourceDir)
}

func TestWorkflow_SourceDir_PathExtraction(t *testing.T) {
	tests := []struct {
		name              string
		filePath          string
		expectedSourceDir string
	}{
		{
			name:              "absolute path",
			filePath:          "/home/user/.config/awf/workflows/plan.yaml",
			expectedSourceDir: "/home/user/.config/awf/workflows",
		},
		{
			name:              "relative path",
			filePath:          ".awf/workflows/analyze.yaml",
			expectedSourceDir: ".awf/workflows",
		},
		{
			name:              "nested path",
			filePath:          "/opt/company/workflows/prod/deploy.yaml",
			expectedSourceDir: "/opt/company/workflows/prod",
		},
		{
			name:              "path with spaces",
			filePath:          "/home/user/my workflows/test.yaml",
			expectedSourceDir: "/home/user/my workflows",
		},
		{
			name:              "current directory",
			filePath:          "workflow.yaml",
			expectedSourceDir: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualDir := filepath.Dir(tt.filePath)
			assert.Equal(t, tt.expectedSourceDir, actualDir,
				"filepath.Dir() should extract directory from %s", tt.filePath)
		})
	}
}

func TestWorkflow_SourceDir_UsedForRelativePathResolution(t *testing.T) {
	wf := workflow.Workflow{
		Name:      "test-workflow",
		Initial:   "start",
		SourceDir: "/home/user/.config/awf/workflows",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}

	relativePath := "prompts/analyze.md"
	expectedAbsolutePath := filepath.Join(wf.SourceDir, relativePath)

	assert.Equal(t, "/home/user/.config/awf/workflows/prompts/analyze.md", expectedAbsolutePath)
}

func TestWorkflow_SourceDir_PathJoinBehavior(t *testing.T) {
	tests := []struct {
		name         string
		sourceDir    string
		relativePath string
		expectedPath string
	}{
		{
			name:         "trailing slash in SourceDir",
			sourceDir:    "/home/user/workflows/",
			relativePath: "prompts/file.md",
			expectedPath: "/home/user/workflows/prompts/file.md",
		},
		{
			name:         "no trailing slash",
			sourceDir:    "/home/user/workflows",
			relativePath: "prompts/file.md",
			expectedPath: "/home/user/workflows/prompts/file.md",
		},
		{
			name:         "dot-dot navigation",
			sourceDir:    "/home/user/workflows",
			relativePath: "../shared/prompt.md",
			expectedPath: "/home/user/shared/prompt.md",
		},
		{
			name:         "dot current directory",
			sourceDir:    "/home/user/workflows",
			relativePath: "./prompts/file.md",
			expectedPath: "/home/user/workflows/prompts/file.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualPath := filepath.Join(tt.sourceDir, tt.relativePath)
			assert.Equal(t, tt.expectedPath, actualPath)
		})
	}
}

func TestWorkflow_SourceDir_NoYAMLTag(t *testing.T) {
	wfType := reflect.TypeOf(workflow.Workflow{})
	field, found := wfType.FieldByName("SourceDir")

	assert.True(t, found, "SourceDir field should exist on Workflow struct")

	yamlTag := field.Tag.Get("yaml")
	assert.Empty(t, yamlTag, "SourceDir should not have a yaml struct tag (runtime metadata only)")
}
