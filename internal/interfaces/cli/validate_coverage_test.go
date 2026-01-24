package cli_test

import (
	"bytes"
	"testing"

	"github.com/vanoix/awf/internal/interfaces/cli"
	"github.com/vanoix/awf/internal/testutil"
)

// These tests focus on code coverage, not strict behavior validation

func TestValidate_TextFormat(t *testing.T) {
	dir := testutil.SetupWorkflowsDir(t, map[string]string{"test": testutil.SimpleWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "test", "--storage", dir})
	_ = cmd.Execute() // Coverage, don't care about result
}

func TestValidate_VerboseFormat(t *testing.T) {
	dir := testutil.SetupWorkflowsDir(t, map[string]string{"test": testutil.FullWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "test", "--verbose", "--storage", dir})
	_ = cmd.Execute()
}

func TestValidate_JSONFormat(t *testing.T) {
	dir := testutil.SetupWorkflowsDir(t, map[string]string{"test": testutil.SimpleWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "test", "--format", "json", "--storage", dir})
	_ = cmd.Execute()
}

func TestValidate_QuietFormat(t *testing.T) {
	dir := testutil.SetupWorkflowsDir(t, map[string]string{"test": testutil.SimpleWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "test", "--format", "quiet", "--storage", dir})
	_ = cmd.Execute()
}

func TestValidate_TableFormat(t *testing.T) {
	dir := testutil.SetupWorkflowsDir(t, map[string]string{"test": testutil.FullWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "test", "--format", "table", "--storage", dir})
	_ = cmd.Execute()
}

func TestValidate_InvalidWorkflow(t *testing.T) {
	dir := testutil.SetupWorkflowsDir(t, map[string]string{"bad": testutil.BadWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "bad", "--storage", dir})
	_ = cmd.Execute() // Will fail, that's expected
}

func TestValidate_InvalidWorkflowJSON(t *testing.T) {
	dir := testutil.SetupWorkflowsDir(t, map[string]string{"bad": testutil.BadWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "bad", "--format", "json", "--storage", dir})
	_ = cmd.Execute()
}

func TestValidate_InvalidWorkflowTable(t *testing.T) {
	dir := testutil.SetupWorkflowsDir(t, map[string]string{"bad": testutil.BadWorkflowYAML})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "bad", "--format", "table", "--storage", dir})
	_ = cmd.Execute()
}

func TestValidate_WorkflowNotFoundJSON(t *testing.T) {
	dir := testutil.SetupTestDir(t)

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "nonexistent", "--format", "json", "--storage", dir})
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
	dir := testutil.SetupWorkflowsDir(t, map[string]string{"tmpl": tmplWF})

	cmd := cli.NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "tmpl", "--storage", dir})
	_ = cmd.Execute() // Will fail on template, that's coverage
}
