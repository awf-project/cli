//go:build integration

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunCommand_PromptFile_Basic tests loading external prompt template
// AC: Agent step with prompt_file loads and interpolates external .md template
func TestRunCommand_PromptFile_Basic(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	promptContent := `Analyze the following name: {{.inputs.name}}`
	promptPath := filepath.Join(tmpDir, ".awf", "workflows", "prompts", "analyze.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptPath), 0o755))
	require.NoError(t, os.WriteFile(promptPath, []byte(promptContent), 0o644))

	workflowContent := `name: prompt-file-basic
version: "1.0.0"
inputs:
  - name: name
    type: string
    required: true
states:
  initial: analyze
  analyze:
    type: agent
    provider: claude
    prompt_file: prompts/analyze.md
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "prompt-file-basic.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "prompt-file-basic", "--dry-run", "--input=name=TestValue"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Analyze the following name: TestValue", "prompt should be interpolated with input value")
}

// TestRunCommand_PromptFile_RelativePath tests relative path resolution
// AC: Relative paths in prompt_file resolve against workflow directory
func TestRunCommand_PromptFile_RelativePath(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	promptsDir := filepath.Join(tmpDir, ".awf", "workflows", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	promptContent := `Execute the task with context.`
	promptPath := filepath.Join(promptsDir, "task.md")
	require.NoError(t, os.WriteFile(promptPath, []byte(promptContent), 0o644))

	workflowContent := `name: relative-path
version: "1.0.0"
states:
  initial: task
  task:
    type: agent
    provider: claude
    prompt_file: prompts/task.md
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "relative-path.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "relative-path", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Execute the task with context", "should load prompt from relative path")
}

// TestRunCommand_PromptFile_AWFDirectory tests AWF directory template variable
// AC: prompt_file supports {{.awf.prompts_dir}} for XDG-compliant directory
func TestRunCommand_PromptFile_AWFDirectory(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Override XDG_CONFIG_HOME to use tmpDir for testing
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	awfPromptsDir := filepath.Join(tmpDir, "awf", "prompts")
	require.NoError(t, os.MkdirAll(awfPromptsDir, 0o755))

	promptContent := `Research the topic thoroughly.`
	promptPath := filepath.Join(awfPromptsDir, "research.md")
	require.NoError(t, os.WriteFile(promptPath, []byte(promptContent), 0o644))

	workflowContent := `name: awf-dir
version: "1.0.0"
states:
  initial: research
  research:
    type: agent
    provider: claude
    prompt_file: "{{.awf.prompts_dir}}/research.md"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "awf-dir.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "awf-dir", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Research the topic thoroughly", "should resolve AWF directory template variable")
}

// TestRunCommand_PromptFile_TildeExpansion tests home directory expansion
// AC: prompt_file supports ~ expansion to user's home directory
func TestRunCommand_PromptFile_TildeExpansion(t *testing.T) {
	tmpDir := setupInitTestDir(t)
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	promptsDir := filepath.Join(homeDir, ".awf-test-prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))
	t.Cleanup(func() {
		os.RemoveAll(promptsDir)
	})

	promptContent := `Custom prompt from home directory.`
	promptPath := filepath.Join(promptsDir, "custom.md")
	require.NoError(t, os.WriteFile(promptPath, []byte(promptContent), 0o644))

	workflowContent := `name: tilde-expansion
version: "1.0.0"
states:
  initial: custom
  custom:
    type: agent
    provider: claude
    prompt_file: ~/.awf-test-prompts/custom.md
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "tilde-expansion.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "tilde-expansion", "--dry-run"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Custom prompt from home directory", "should expand tilde to home directory")
}

// TestRunCommand_PromptFile_TemplateHelpers tests template helper functions
// AC: Template helpers (split, join, readFile, trimSpace) work in prompt files
func TestRunCommand_PromptFile_TemplateHelpers(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	dataFile := filepath.Join(tmpDir, ".awf", "data", "spec.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(dataFile), 0o755))
	require.NoError(t, os.WriteFile(dataFile, []byte("  Specification Content  \n"), 0o644))

	promptContent := `Process the agents: {{join (split .inputs.agents ",") " and "}}
Spec: {{trimSpace (readFile .inputs.spec_path)}}`

	promptPath := filepath.Join(tmpDir, ".awf", "workflows", "prompts", "helpers.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptPath), 0o755))
	require.NoError(t, os.WriteFile(promptPath, []byte(promptContent), 0o644))

	workflowContent := `name: template-helpers
version: "1.0.0"
inputs:
  - name: agents
    type: string
    required: true
  - name: spec_path
    type: string
    required: true
states:
  initial: process
  process:
    type: agent
    provider: claude
    prompt_file: prompts/helpers.md
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "template-helpers.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	specPath := filepath.Join(tmpDir, ".awf", "data", "spec.txt")
	cmd.SetArgs([]string{
		"run", "template-helpers",
		"--dry-run",
		"--input=agents=claude,gemini,codex",
		"--input=spec_path=" + specPath,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "claude and gemini and codex", "split and join should work")
	assert.Contains(t, output, "Specification Content", "readFile and trimSpace should work")
}

// TestValidateCommand_PromptFile_Missing tests validation for missing files
// AC: awf validate returns USER.INPUT.INVALID for nonexistent prompt_file
func TestValidateCommand_PromptFile_Missing(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	workflowContent := `name: missing-file
version: "1.0.0"
states:
  initial: analyze
  analyze:
    type: agent
    provider: claude
    prompt_file: nonexistent.md
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "missing-file.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "missing-file"})

	err := cmd.Execute()
	require.Error(t, err, "validation should fail for missing prompt_file")
	assert.Contains(t, err.Error(), "prompt_file not found", "error should mention prompt_file not found")
}

// TestValidateCommand_PromptFile_Valid tests validation passing for existing file
// AC: awf validate passes when prompt_file points to valid file
func TestValidateCommand_PromptFile_Valid(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	promptPath := filepath.Join(tmpDir, ".awf", "workflows", "prompts", "valid.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptPath), 0o755))
	require.NoError(t, os.WriteFile(promptPath, []byte("Valid prompt"), 0o644))

	workflowContent := `name: valid-file
version: "1.0.0"
states:
  initial: analyze
  analyze:
    type: agent
    provider: claude
    prompt_file: prompts/valid.md
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "valid-file.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "valid-file"})

	err := cmd.Execute()
	require.NoError(t, err, "validation should pass for existing prompt_file")
}

// TestValidateCommand_PromptFile_MutualExclusivity tests prompt/prompt_file mutual exclusivity
// AC: Both prompt and prompt_file set returns validation error
func TestValidateCommand_PromptFile_MutualExclusivity(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	promptPath := filepath.Join(tmpDir, ".awf", "prompts", "test.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptPath), 0o755))
	require.NoError(t, os.WriteFile(promptPath, []byte("Template"), 0o644))

	workflowContent := `name: mutual-exclusive
version: "1.0.0"
states:
  initial: analyze
  analyze:
    type: agent
    provider: claude
    prompt: "Inline prompt"
    prompt_file: prompts/test.md
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "mutual-exclusive.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "mutual-exclusive"})

	err := cmd.Execute()
	require.Error(t, err, "validation should fail when both prompt and prompt_file are set")
	assert.Contains(t, err.Error(), "mutually exclusive", "error should mention mutual exclusivity")
}

// TestValidateCommand_PromptFile_RequireOnePromptSource tests validation requiring at least one prompt source
// AC: Neither prompt nor prompt_file returns validation error
func TestValidateCommand_PromptFile_RequireOnePromptSource(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	workflowContent := `name: no-prompt
version: "1.0.0"
states:
  initial: analyze
  analyze:
    type: agent
    provider: claude
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "no-prompt.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "no-prompt"})

	err := cmd.Execute()
	require.Error(t, err, "validation should fail when neither prompt nor prompt_file is set")
	assert.Contains(t, err.Error(), "required", "error should mention field is required")
}

// TestValidateCommand_PromptFile_Directory tests validation rejecting directories
// AC: prompt_file pointing to directory returns validation error
func TestValidateCommand_PromptFile_Directory(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	dirPath := filepath.Join(tmpDir, ".awf", "workflows", "prompts", "not-a-file")
	require.NoError(t, os.MkdirAll(dirPath, 0o755))

	workflowContent := `name: directory-instead-of-file
version: "1.0.0"
states:
  initial: analyze
  analyze:
    type: agent
    provider: claude
    prompt_file: prompts/not-a-file
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "directory-instead-of-file.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "directory-instead-of-file"})

	err := cmd.Execute()
	require.Error(t, err, "validation should fail when prompt_file points to directory")
	assert.Contains(t, err.Error(), "directory", "error should mention directory issue")
}

// TestRunCommand_PromptFile_FileSizeLimit tests 1MB file size limit
// NFR-001: File reading must fail for files exceeding 1MB
func TestRunCommand_PromptFile_FileSizeLimit(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	largeContent := make([]byte, 2*1024*1024) // 2MB
	for i := range largeContent {
		largeContent[i] = 'A'
	}

	promptPath := filepath.Join(tmpDir, ".awf", "workflows", "prompts", "large.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptPath), 0o755))
	require.NoError(t, os.WriteFile(promptPath, largeContent, 0o644))

	workflowContent := `name: large-file
version: "1.0.0"
states:
  initial: analyze
  analyze:
    type: agent
    provider: claude
    prompt_file: prompts/large.md
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "large-file.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "large-file", "--dry-run"})

	err := cmd.Execute()
	require.Error(t, err, "execution should fail for files exceeding 1MB")
	assert.Contains(t, err.Error(), "exceeds", "error should mention size limit")
}

// TestRunCommand_PromptFile_StateInterpolation tests interpolation with state outputs
// AC: prompt_file content supports {{.states.step.output}} interpolation
func TestRunCommand_PromptFile_StateInterpolation(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	promptContent := `Based on previous step: {{.states.prepare.output}}`
	promptPath := filepath.Join(tmpDir, ".awf", "workflows", "prompts", "with-state.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptPath), 0o755))
	require.NoError(t, os.WriteFile(promptPath, []byte(promptContent), 0o644))

	workflowContent := `name: state-interpolation
version: "1.0.0"
states:
  initial: prepare
  prepare:
    type: step
    command: echo "prepared data"
    on_success: analyze
  analyze:
    type: agent
    provider: claude
    prompt_file: prompts/with-state.md
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "state-interpolation.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "state-interpolation", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Based on previous step:", "prompt should reference state output")
}
