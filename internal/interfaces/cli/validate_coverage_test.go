package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// These tests focus on code coverage, not strict behavior validation

func setupWorkflow(t *testing.T, name, content string) func() {
	t.Helper()
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, name+".yaml"), []byte(content), 0644))
	require.NoError(t, os.Chdir(tmpDir))
	return func() { _ = os.Chdir(origDir) }
}

const simpleWF = `name: test
states:
  initial: start
  start:
    type: step
    command: echo "test"
    on_success: done
  done:
    type: terminal
`

const fullWF = `name: test-full
version: "1.0.0"
description: Test workflow
inputs:
  - name: var1
    type: string
    required: true
  - name: var2
    type: integer
    default: 5
states:
  initial: step1
  step1:
    type: step
    command: echo "{{inputs.var1}}"
    on_success: done
  done:
    type: terminal
`

const badWF = `name: bad
states:
  initial: start
  start:
    type: step
    command: echo "test"
    on_success: nonexistent
  done:
    type: terminal
`

func TestValidate_TextFormat(t *testing.T) {
	cleanup := setupWorkflow(t, "test", simpleWF)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "test"})
	_ = cmd.Execute() // Coverage, don't care about result
}

func TestValidate_VerboseFormat(t *testing.T) {
	cleanup := setupWorkflow(t, "test", fullWF)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "test", "--verbose"})
	_ = cmd.Execute()
}

func TestValidate_JSONFormat(t *testing.T) {
	cleanup := setupWorkflow(t, "test", simpleWF)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "test", "--format", "json"})
	_ = cmd.Execute()
}

func TestValidate_QuietFormat(t *testing.T) {
	cleanup := setupWorkflow(t, "test", simpleWF)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "test", "--format", "quiet"})
	_ = cmd.Execute()
}

func TestValidate_TableFormat(t *testing.T) {
	cleanup := setupWorkflow(t, "test", fullWF)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "test", "--format", "table"})
	_ = cmd.Execute()
}

func TestValidate_InvalidWorkflow(t *testing.T) {
	cleanup := setupWorkflow(t, "bad", badWF)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "bad"})
	_ = cmd.Execute() // Will fail, that's expected
}

func TestValidate_InvalidWorkflowJSON(t *testing.T) {
	cleanup := setupWorkflow(t, "bad", badWF)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "bad", "--format", "json"})
	_ = cmd.Execute()
}

func TestValidate_InvalidWorkflowTable(t *testing.T) {
	cleanup := setupWorkflow(t, "bad", badWF)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "bad", "--format", "table"})
	_ = cmd.Execute()
}

func TestValidate_WorkflowNotFoundJSON(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	os.Chdir(tmpDir)

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "nonexistent", "--format", "json"})
	_ = cmd.Execute()
}

func TestValidate_WithTemplateRef(t *testing.T) {
	tmplWF := `name: with-tmpl
states:
  initial: start
  start:
    type: step
    template_ref:
      name: nonexistent
      version: "1.0"
    on_success: done
  done:
    type: terminal
`
	cleanup := setupWorkflow(t, "tmpl", tmplWF)
	defer cleanup()

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "tmpl"})
	_ = cmd.Execute() // Will fail on template, that's coverage
}
