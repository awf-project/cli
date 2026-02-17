//go:build integration

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunCommand_ScriptFile_Basic tests loading external script file
// AC: Step with script_file loads and executes external shell script
func TestRunCommand_ScriptFile_Basic(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err)

	tmpDir := setupInitTestDir(t)

	scriptPath := filepath.Join(repoRoot, "tests", "fixtures", "scripts", "basic.sh")

	workflowContent := `name: script-file-basic
version: "1.0.0"
states:
  initial: execute
  execute:
    type: step
    script_file: ` + scriptPath + `
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "script-file-basic.yaml", workflowContent)

	output, err := runCLI(t, "run", "script-file-basic", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, output, "basic script executed", "script should be loaded and executed")
}

// TestRunCommand_ScriptFile_RelativePath tests relative path resolution
// AC: Relative paths in script_file resolve against workflow directory
func TestRunCommand_ScriptFile_RelativePath(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	scriptsDir := filepath.Join(tmpDir, ".awf", "workflows", "scripts")
	require.NoError(t, os.MkdirAll(scriptsDir, 0o755))

	scriptContent := `#!/bin/sh
echo "relative script executed"
`
	scriptPath := filepath.Join(scriptsDir, "test.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	workflowContent := `name: relative-path
version: "1.0.0"
states:
  initial: execute
  execute:
    type: step
    script_file: scripts/test.sh
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "relative-path.yaml", workflowContent)

	output, err := runCLI(t, "run", "relative-path", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, output, "relative script executed", "should load script from relative path")
}

// TestRunCommand_ScriptFile_AWFScriptsDir tests AWF scripts directory template variable
// AC: script_file supports {{.awf.scripts_dir}} for XDG-compliant directory
func TestRunCommand_ScriptFile_AWFScriptsDir(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	awfScriptsDir := filepath.Join(tmpDir, "awf", "scripts")
	require.NoError(t, os.MkdirAll(awfScriptsDir, 0o755))

	scriptContent := `#!/bin/sh
echo "awf scripts dir executed"
`
	scriptPath := filepath.Join(awfScriptsDir, "build.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	workflowContent := `name: awf-scripts-dir
version: "1.0.0"
states:
  initial: build
  build:
    type: step
    script_file: "{{.awf.scripts_dir}}/build.sh"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "awf-scripts-dir.yaml", workflowContent)

	output, err := runCLI(t, "run", "awf-scripts-dir", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, output, "awf scripts dir executed", "should resolve AWF scripts directory template variable")
}

// TestRunCommand_ScriptFile_AbsolutePath tests absolute path resolution
// AC: Absolute paths in script_file load directly without modification
func TestRunCommand_ScriptFile_AbsolutePath(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	scriptDir := filepath.Join(tmpDir, "absolute-scripts")
	require.NoError(t, os.MkdirAll(scriptDir, 0o755))

	scriptContent := `#!/bin/sh
echo "absolute path executed"
`
	scriptPath := filepath.Join(scriptDir, "deploy.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	workflowContent := `name: absolute-path
version: "1.0.0"
states:
  initial: deploy
  deploy:
    type: step
    script_file: ` + scriptPath + `
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "absolute-path.yaml", workflowContent)

	output, err := runCLI(t, "run", "absolute-path", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, output, "absolute path executed", "should load script from absolute path")
}

// TestRunCommand_ScriptFile_TildePath tests tilde expansion
// AC: Paths starting with ~ expand to user home directory
func TestRunCommand_ScriptFile_TildePath(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	scriptsDir := filepath.Join(homeDir, ".awf-test-scripts")
	require.NoError(t, os.MkdirAll(scriptsDir, 0o755))
	t.Cleanup(func() {
		os.RemoveAll(scriptsDir)
	})

	scriptContent := `#!/bin/sh
echo "tilde path executed"
`
	scriptPath := filepath.Join(scriptsDir, "custom.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	workflowContent := `name: tilde-expansion
version: "1.0.0"
states:
  initial: custom
  custom:
    type: step
    script_file: ~/.awf-test-scripts/custom.sh
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "tilde-expansion.yaml", workflowContent)

	output, err := runCLI(t, "run", "tilde-expansion", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, output, "tilde path executed", "should expand tilde to home directory")
}

// TestRunCommand_ScriptFile_ContentInterpolation tests template interpolation in script content
// AC: Loaded script file contents support Go template interpolation with workflow variables
func TestRunCommand_ScriptFile_ContentInterpolation(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err)

	tmpDir := setupInitTestDir(t)

	scriptPath := filepath.Join(repoRoot, "tests", "fixtures", "scripts", "interpolate.sh")

	workflowContent := `name: content-interpolation
version: "1.0.0"
inputs:
  - name: value
    type: string
    required: true
states:
  initial: interpolate
  interpolate:
    type: step
    script_file: ` + scriptPath + `
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "content-interpolation.yaml", workflowContent)

	output, err := runCLI(t, "run", "content-interpolation", "--dry-run", "--input=value=TestData")
	require.NoError(t, err)
	assert.Contains(t, output, "Value: TestData", "script content should be interpolated with input value")
}

// TestRunCommand_ScriptFile_DryRun tests dry run mode output
// AC: Dry run shows resolved script file path without executing
func TestRunCommand_ScriptFile_DryRun(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	scriptsDir := filepath.Join(tmpDir, ".awf", "workflows", "scripts")
	require.NoError(t, os.MkdirAll(scriptsDir, 0o755))

	scriptContent := `#!/bin/sh
echo "dry run test"
`
	scriptPath := filepath.Join(scriptsDir, "dryrun.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	workflowContent := `name: dry-run-test
version: "1.0.0"
states:
  initial: test
  test:
    type: step
    script_file: scripts/dryrun.sh
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "dry-run-test.yaml", workflowContent)

	output, err := runCLI(t, "run", "dry-run-test", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, output, "echo \"dry run test\"", "dry run should show resolved script content")
}

// TestRunCommand_ScriptFile_MissingFile tests error handling for non-existent file
// AC: Missing script file produces clear error with resolved path
func TestRunCommand_ScriptFile_MissingFile(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	workflowContent := `name: missing-file
version: "1.0.0"
states:
  initial: execute
  execute:
    type: step
    script_file: nonexistent/missing.sh
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "missing-file.yaml", workflowContent)

	output, err := runCLI(t, "run", "missing-file", "--dry-run")
	require.Error(t, err, "should fail with missing file error")
	assert.Contains(t, output, "file not found", "error should mention missing script file")
	assert.Contains(t, output, "missing.sh", "error should include the file path")
}

// TestRunCommand_ScriptFile_MutualExclusivity tests command/script_file XOR validation
// AC: Workflow validation fails when both command and script_file are set
func TestRunCommand_ScriptFile_MutualExclusivity(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	workflowContent := `name: mutual-exclusivity
version: "1.0.0"
states:
  initial: execute
  execute:
    type: step
    command: echo "inline command"
    script_file: scripts/test.sh
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "mutual-exclusivity.yaml", workflowContent)

	output, err := runCLI(t, "validate", "mutual-exclusivity")
	require.Error(t, err, "validation should fail when both command and script_file are set")
	assert.Contains(t, output, "mutually exclusive", "error should mention mutual exclusivity")
}

// TestRunCommand_ScriptFile_NestedPath tests nested directory script resolution
// AC: Script files in nested directories resolve correctly relative to workflow
func TestRunCommand_ScriptFile_NestedPath(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err)

	tmpDir := setupInitTestDir(t)

	scriptPath := filepath.Join(repoRoot, "tests", "fixtures", "scripts", "local", "local-script.sh")

	workflowContent := `name: nested-path
version: "1.0.0"
states:
  initial: execute
  execute:
    type: step
    script_file: ` + scriptPath + `
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "nested-path.yaml", workflowContent)

	output, err := runCLI(t, "run", "nested-path", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, output, "local script", "nested script should be loaded and executed")
}

// TestRunCommand_ScriptFile_MultipleSteps tests multiple steps with different script files
// AC: Multiple command steps can each reference different script files
func TestRunCommand_ScriptFile_MultipleSteps(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	scriptsDir := filepath.Join(tmpDir, ".awf", "workflows", "scripts")
	require.NoError(t, os.MkdirAll(scriptsDir, 0o755))

	script1Content := `#!/bin/sh
echo "step one executed"
`
	script1Path := filepath.Join(scriptsDir, "step1.sh")
	require.NoError(t, os.WriteFile(script1Path, []byte(script1Content), 0o755))

	script2Content := `#!/bin/sh
echo "step two executed"
`
	script2Path := filepath.Join(scriptsDir, "step2.sh")
	require.NoError(t, os.WriteFile(script2Path, []byte(script2Content), 0o755))

	workflowContent := `name: multiple-steps
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    script_file: scripts/step1.sh
    on_success: step2
  step2:
    type: step
    script_file: scripts/step2.sh
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "multiple-steps.yaml", workflowContent)

	output, err := runCLI(t, "run", "multiple-steps", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, output, "step one executed", "first script should be loaded")
	assert.Contains(t, output, "step two executed", "second script should be loaded")
}

// TestRunCommand_ScriptFile_LocalOverridesGlobalAWFScriptsDir verifies local scripts take precedence over global XDG scripts
// AC: When both local and global scripts exist, {{.awf.scripts_dir}} resolves to the local workflow-relative script
// Regression guard for B005
func TestRunCommand_ScriptFile_LocalOverridesGlobalAWFScriptsDir(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	globalScriptsDir := filepath.Join(tmpDir, "awf", "scripts")
	require.NoError(t, os.MkdirAll(globalScriptsDir, 0o755))

	globalScriptContent := `#!/bin/sh
echo "GLOBAL SCRIPT"
`
	globalScriptPath := filepath.Join(globalScriptsDir, "deploy.sh")
	require.NoError(t, os.WriteFile(globalScriptPath, []byte(globalScriptContent), 0o755))

	localScriptsDir := filepath.Join(tmpDir, ".awf", "workflows", "scripts")
	require.NoError(t, os.MkdirAll(localScriptsDir, 0o755))

	localScriptContent := `#!/bin/sh
echo "LOCAL SCRIPT"
`
	localScriptPath := filepath.Join(localScriptsDir, "deploy.sh")
	require.NoError(t, os.WriteFile(localScriptPath, []byte(localScriptContent), 0o755))

	workflowContent := `name: local-override-test
version: "1.0.0"
states:
  initial: deploy
  deploy:
    type: step
    script_file: "{{.awf.scripts_dir}}/deploy.sh"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "local-override-test.yaml", workflowContent)

	output, err := runCLI(t, "run", "local-override-test", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, output, "LOCAL SCRIPT", "local script should take precedence over global XDG script")
	assert.NotContains(t, output, "GLOBAL SCRIPT", "global script should not be used when local exists")
}
