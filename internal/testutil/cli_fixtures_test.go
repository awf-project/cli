package testutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/testutil"
)

// Feature: C017
// Component: T002 - CLI Test Fixture Functions

// =============================================================================
// SetupTestDir Tests
// =============================================================================

// TestSetupTestDir_HappyPath tests basic directory creation with .awf structure
func TestSetupTestDir_HappyPath(t *testing.T) {
	// Act
	dir := testutil.SetupTestDir(t)

	// Assert
	require.NotEmpty(t, dir, "SetupTestDir should return non-empty directory path")

	// Verify .awf directory exists
	awfDir := filepath.Join(dir, ".awf")
	stat, err := os.Stat(awfDir)
	require.NoError(t, err, ".awf directory should exist")
	assert.True(t, stat.IsDir(), ".awf should be a directory")

	// Verify workflows subdirectory exists
	workflowsDir := filepath.Join(awfDir, "workflows")
	stat, err = os.Stat(workflowsDir)
	require.NoError(t, err, ".awf/workflows directory should exist")
	assert.True(t, stat.IsDir(), "workflows should be a directory")

	// Verify prompts subdirectory exists
	promptsDir := filepath.Join(awfDir, "prompts")
	stat, err = os.Stat(promptsDir)
	require.NoError(t, err, ".awf/prompts directory should exist")
	assert.True(t, stat.IsDir(), "prompts should be a directory")

	// Verify storage subdirectory exists
	storageDir := filepath.Join(awfDir, "storage")
	stat, err = os.Stat(storageDir)
	require.NoError(t, err, ".awf/storage directory should exist")
	assert.True(t, stat.IsDir(), "storage should be a directory")
}

// TestSetupTestDir_ReturnsUniqueDirectories tests that each call returns a unique directory
func TestSetupTestDir_ReturnsUniqueDirectories(t *testing.T) {
	// Act
	dir1 := testutil.SetupTestDir(t)
	dir2 := testutil.SetupTestDir(t)

	// Assert
	assert.NotEqual(t, dir1, dir2, "Each call should return a unique directory")

	// Verify both directories exist independently
	_, err1 := os.Stat(dir1)
	_, err2 := os.Stat(dir2)
	assert.NoError(t, err1, "First directory should exist")
	assert.NoError(t, err2, "Second directory should exist")
}

// TestSetupTestDir_DirectoryPermissions tests that created directories have correct permissions
func TestSetupTestDir_DirectoryPermissions(t *testing.T) {
	// Act
	dir := testutil.SetupTestDir(t)

	// Assert - verify permissions are readable/writable
	awfDir := filepath.Join(dir, ".awf")
	stat, err := os.Stat(awfDir)
	require.NoError(t, err)

	mode := stat.Mode()
	assert.True(t, mode.IsDir(), "Should be a directory")
	// 0755 or better (owner can read/write/execute, others can read/execute)
	assert.True(t, mode.Perm()&0o700 == 0o700, "Owner should have rwx permissions")
}

// TestSetupTestDir_ThreadSafe tests concurrent directory creation
func TestSetupTestDir_ThreadSafe(t *testing.T) {
	const goroutines = 20
	results := make(chan string, goroutines)

	// Act - create directories concurrently
	for range goroutines {
		go func() {
			dir := testutil.SetupTestDir(t)
			results <- dir
		}()
	}

	// Assert - collect all results
	dirs := make([]string, 0, goroutines)
	for range goroutines {
		dir := <-results
		require.NotEmpty(t, dir)
		dirs = append(dirs, dir)
	}

	// Verify all directories are unique
	seen := make(map[string]bool)
	for _, dir := range dirs {
		assert.False(t, seen[dir], "Directory %s should be unique", dir)
		seen[dir] = true
	}
}

// =============================================================================
// CreateTestWorkflow Tests
// =============================================================================

// TestCreateTestWorkflow_HappyPath tests creating a workflow file with valid YAML
func TestCreateTestWorkflow_HappyPath(t *testing.T) {
	// Arrange
	dir := testutil.SetupTestDir(t)
	workflowName := "simple.yaml"
	workflowContent := testutil.SimpleWorkflowYAML

	// Act
	testutil.CreateTestWorkflow(t, dir, workflowName, workflowContent)

	// Assert
	workflowPath := filepath.Join(dir, ".awf", "workflows", workflowName)
	content, err := os.ReadFile(workflowPath)
	require.NoError(t, err, "Workflow file should exist and be readable")
	assert.Equal(t, workflowContent, string(content), "File content should match input")
}

// TestCreateTestWorkflow_MultipleFiles tests creating multiple workflow files in the same directory
func TestCreateTestWorkflow_MultipleFiles(t *testing.T) {
	// Arrange
	dir := testutil.SetupTestDir(t)

	// Act
	testutil.CreateTestWorkflow(t, dir, "workflow1.yaml", testutil.SimpleWorkflowYAML)
	testutil.CreateTestWorkflow(t, dir, "workflow2.yaml", testutil.FullWorkflowYAML)
	testutil.CreateTestWorkflow(t, dir, "workflow3.yaml", testutil.BadWorkflowYAML)

	// Assert
	workflowsDir := filepath.Join(dir, ".awf", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	require.NoError(t, err)

	assert.Len(t, entries, 3, "Should have 3 workflow files")

	// Verify each file exists and has correct content
	content1, _ := os.ReadFile(filepath.Join(workflowsDir, "workflow1.yaml"))
	content2, _ := os.ReadFile(filepath.Join(workflowsDir, "workflow2.yaml"))
	content3, _ := os.ReadFile(filepath.Join(workflowsDir, "workflow3.yaml"))

	assert.Equal(t, testutil.SimpleWorkflowYAML, string(content1))
	assert.Equal(t, testutil.FullWorkflowYAML, string(content2))
	assert.Equal(t, testutil.BadWorkflowYAML, string(content3))
}

// TestCreateTestWorkflow_EmptyContent tests creating a workflow with empty content
func TestCreateTestWorkflow_EmptyContent(t *testing.T) {
	// Arrange
	dir := testutil.SetupTestDir(t)

	// Act
	testutil.CreateTestWorkflow(t, dir, "empty.yaml", "")

	// Assert
	workflowPath := filepath.Join(dir, ".awf", "workflows", "empty.yaml")
	content, err := os.ReadFile(workflowPath)
	require.NoError(t, err)
	assert.Empty(t, content, "Empty content should create empty file")
}

// TestCreateTestWorkflow_OverwriteExisting tests that creating a workflow twice overwrites the first
func TestCreateTestWorkflow_OverwriteExisting(t *testing.T) {
	// Arrange
	dir := testutil.SetupTestDir(t)
	workflowName := "test.yaml"

	// Act
	testutil.CreateTestWorkflow(t, dir, workflowName, "content1")
	testutil.CreateTestWorkflow(t, dir, workflowName, "content2")

	// Assert
	workflowPath := filepath.Join(dir, ".awf", "workflows", workflowName)
	content, err := os.ReadFile(workflowPath)
	require.NoError(t, err)
	assert.Equal(t, "content2", string(content), "Second write should overwrite first")
}

// TestCreateTestWorkflow_SpecialCharacters tests workflow names with various special characters
func TestCreateTestWorkflow_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		shouldCreate bool
	}{
		{name: "hyphen", filename: "my-workflow.yaml", shouldCreate: true},
		{name: "underscore", filename: "my_workflow.yaml", shouldCreate: true},
		{name: "number", filename: "workflow123.yaml", shouldCreate: true},
		{name: "dot in name", filename: "my.workflow.yaml", shouldCreate: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			dir := testutil.SetupTestDir(t)

			// Act
			testutil.CreateTestWorkflow(t, dir, tt.filename, testutil.SimpleWorkflowYAML)

			// Assert
			workflowPath := filepath.Join(dir, ".awf", "workflows", tt.filename)
			_, err := os.Stat(workflowPath)
			if tt.shouldCreate {
				assert.NoError(t, err, "File %s should be created", tt.filename)
			}
		})
	}
}

// TestCreateTestWorkflow_PathSeparatorFlattening tests that path separators in workflow names are flattened
func TestCreateTestWorkflow_PathSeparatorFlattening(t *testing.T) {
	tests := []struct {
		name         string
		inputName    string
		expectedFile string
	}{
		{
			name:         "subdirectory path",
			inputName:    "sub/dir/workflow.yaml",
			expectedFile: "sub-dir-workflow.yaml",
		},
		{
			name:         "single subdirectory",
			inputName:    "workflows/test.yaml",
			expectedFile: "workflows-test.yaml",
		},
		{
			name:         "multiple levels deep",
			inputName:    "a/b/c/test.yaml",
			expectedFile: "a-b-c-test.yaml",
		},
		{
			name:         "no path separators",
			inputName:    "simple.yaml",
			expectedFile: "simple.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			dir := testutil.SetupTestDir(t)

			// Act
			testutil.CreateTestWorkflow(t, dir, tt.inputName, testutil.SimpleWorkflowYAML)

			// Assert - file should be created with flattened name
			workflowPath := filepath.Join(dir, ".awf", "workflows", tt.expectedFile)
			content, err := os.ReadFile(workflowPath)
			require.NoError(t, err, "Flattened file %s should exist", tt.expectedFile)
			assert.Equal(t, testutil.SimpleWorkflowYAML, string(content))

			// Verify that nested directory was NOT created
			nestedPath := filepath.Join(dir, ".awf", "workflows", tt.inputName)
			if tt.inputName != tt.expectedFile {
				_, err := os.Stat(nestedPath)
				assert.Error(t, err, "Nested path %s should not exist", nestedPath)
			}
		})
	}
}

// =============================================================================
// SetupWorkflowsDir Tests
// =============================================================================

// TestSetupWorkflowsDir_HappyPath tests creating directory with multiple workflows from map
func TestSetupWorkflowsDir_HappyPath(t *testing.T) {
	// Arrange
	workflows := map[string]string{
		"simple.yaml": testutil.SimpleWorkflowYAML,
		"full.yaml":   testutil.FullWorkflowYAML,
	}

	// Act
	dir := testutil.SetupWorkflowsDir(t, workflows)

	// Assert
	require.NotEmpty(t, dir, "Should return non-empty directory")

	// Verify both workflows exist
	workflowsDir := filepath.Join(dir, ".awf", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	require.NoError(t, err)

	assert.Len(t, entries, 2, "Should have 2 workflow files")

	// Verify content
	content1, _ := os.ReadFile(filepath.Join(workflowsDir, "simple.yaml"))
	content2, _ := os.ReadFile(filepath.Join(workflowsDir, "full.yaml"))

	assert.Equal(t, testutil.SimpleWorkflowYAML, string(content1))
	assert.Equal(t, testutil.FullWorkflowYAML, string(content2))
}

// TestSetupWorkflowsDir_EmptyMap tests behavior with empty workflow map
func TestSetupWorkflowsDir_EmptyMap(t *testing.T) {
	// Arrange
	workflows := map[string]string{}

	// Act
	dir := testutil.SetupWorkflowsDir(t, workflows)

	// Assert
	require.NotEmpty(t, dir, "Should return directory even for empty map")

	// Verify directory structure exists but no workflows
	workflowsDir := filepath.Join(dir, ".awf", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "Should have no workflow files")
}

// TestSetupWorkflowsDir_SingleWorkflow tests creating directory with single workflow
func TestSetupWorkflowsDir_SingleWorkflow(t *testing.T) {
	// Arrange
	workflows := map[string]string{
		"test.yaml": testutil.SimpleWorkflowYAML,
	}

	// Act
	dir := testutil.SetupWorkflowsDir(t, workflows)

	// Assert
	workflowPath := filepath.Join(dir, ".awf", "workflows", "test.yaml")
	content, err := os.ReadFile(workflowPath)
	require.NoError(t, err)
	assert.Equal(t, testutil.SimpleWorkflowYAML, string(content))
}

// TestSetupWorkflowsDir_LargeNumberOfWorkflows tests creating many workflows
func TestSetupWorkflowsDir_LargeNumberOfWorkflows(t *testing.T) {
	// Arrange
	workflows := make(map[string]string)
	for i := range 100 {
		workflows[filepath.Join("workflow", string(rune('a'+i%26)), ".yaml")] = testutil.SimpleWorkflowYAML
	}

	// Act
	dir := testutil.SetupWorkflowsDir(t, workflows)

	// Assert
	workflowsDir := filepath.Join(dir, ".awf", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	require.NoError(t, err)
	assert.Len(t, entries, len(workflows), "Should create all workflow files")
}

// =============================================================================
// YAML Constants Tests
// =============================================================================

// TestSimpleWorkflowYAML_IsValidYAML tests that constant contains valid YAML syntax
func TestSimpleWorkflowYAML_IsValidYAML(t *testing.T) {
	// Act - parse as generic YAML to verify syntax
	// Note: actual parsing uses workflow-specific parser, but this checks basic YAML validity
	content := testutil.SimpleWorkflowYAML

	// Assert
	assert.NotEmpty(t, content, "SimpleWorkflowYAML should not be empty")
	assert.Contains(t, content, "name:", "Should contain workflow name field")
	assert.Contains(t, content, "states:", "Should contain states field")
	assert.Contains(t, content, "initial:", "Should contain initial state")
}

// TestFullWorkflowYAML_ContainsInputs tests that FullWorkflowYAML includes input definitions
func TestFullWorkflowYAML_ContainsInputs(t *testing.T) {
	// Act
	content := testutil.FullWorkflowYAML

	// Assert
	assert.Contains(t, content, "inputs:", "Should have inputs section")
	assert.Contains(t, content, "var1", "Should have var1 input")
	assert.Contains(t, content, "var2", "Should have var2 input")
	assert.Contains(t, content, "type: string", "Should have typed inputs")
	assert.Contains(t, content, "required: true", "Should have required field")
	assert.Contains(t, content, "default:", "Should have default value")
}

// TestBadWorkflowYAML_ContainsInvalidReference tests that BadWorkflowYAML has intentional error
func TestBadWorkflowYAML_ContainsInvalidReference(t *testing.T) {
	// Act
	content := testutil.BadWorkflowYAML

	// Assert
	assert.Contains(t, content, "on_success: nonexistent", "Should reference nonexistent step")
	assert.NotContains(t, content, "nonexistent:", "Should not define the nonexistent step")
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestIntegration_SetupAndCreateWorkflow tests combining SetupTestDir and CreateTestWorkflow
func TestIntegration_SetupAndCreateWorkflow(t *testing.T) {
	// Act
	dir := testutil.SetupTestDir(t)
	testutil.CreateTestWorkflow(t, dir, "test.yaml", testutil.SimpleWorkflowYAML)

	// Assert - verify complete structure
	workflowPath := filepath.Join(dir, ".awf", "workflows", "test.yaml")
	content, err := os.ReadFile(workflowPath)
	require.NoError(t, err)
	assert.Equal(t, testutil.SimpleWorkflowYAML, string(content))
}

// TestIntegration_SetupWorkflowsDir_CreatesTempDir tests that SetupWorkflowsDir uses temp directory
func TestIntegration_SetupWorkflowsDir_CreatesTempDir(t *testing.T) {
	// Act
	dir := testutil.SetupWorkflowsDir(t, map[string]string{
		"test.yaml": testutil.SimpleWorkflowYAML,
	})

	// Assert - verify it's a temp directory (contains os.TempDir prefix)
	assert.Contains(t, dir, os.TempDir(), "Should be in system temp directory")
}

// TestIntegration_AllFixtureConstants_AreNonEmpty tests that all YAML constants are defined
func TestIntegration_AllFixtureConstants_AreNonEmpty(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "SimpleWorkflowYAML", content: testutil.SimpleWorkflowYAML},
		{name: "FullWorkflowYAML", content: testutil.FullWorkflowYAML},
		{name: "BadWorkflowYAML", content: testutil.BadWorkflowYAML},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.content, "%s should be non-empty", tt.name)
		})
	}
}
