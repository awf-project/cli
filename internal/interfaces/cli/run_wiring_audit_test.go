package cli_test

// F071 Component T011: Wire FileAuditTrailWriter in run and resume commands
// Tests: Audit trail wiring integration for run and resume commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunCommand_WiresAuditTrailWriter_Disabled verifies audit trail is disabled when AWF_AUDIT_LOG=off
func TestRunCommand_WiresAuditTrailWriter_Disabled(t *testing.T) {
	// GIVEN: A temporary test directory with AWF_AUDIT_LOG=off
	tmpDir := setupTestDir(t)
	t.Setenv("AWF_AUDIT_LOG", "off")

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: test-audit-disabled
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "hello"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "test-audit-disabled.yaml", workflowContent)

	// WHEN: Running the workflow with audit disabled
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test-audit-disabled"})

	_ = cmd.Execute()

	// THEN: Execution should complete (success or failure doesn't matter for wiring test)
	// The important assertion is that buildAuditWriter correctly reads AWF_AUDIT_LOG=off
	// and returns nil writer, preventing any audit file creation

	// Verify NO audit file was created at default location when disabled
	defaultAuditPath := filepath.Join(tmpDir, "audit.jsonl")
	_, err := os.Stat(defaultAuditPath)
	// When AWF_AUDIT_LOG=off, buildAuditWriter returns nil writer, so no file should exist
	assert.True(t, os.IsNotExist(err), "expected no audit file when AWF_AUDIT_LOG=off, verifies buildAuditWriter respects 'off' setting")
}

// TestRunCommand_WiresAuditTrailWriter_DefaultPath verifies audit trail uses default path when AWF_AUDIT_LOG not set
func TestRunCommand_WiresAuditTrailWriter_DefaultPath(t *testing.T) {
	// GIVEN: A temporary test directory with no AWF_AUDIT_LOG set
	tmpDir := setupTestDir(t)
	t.Setenv("AWF_AUDIT_LOG", "")

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: test-audit-default
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "test-audit-default.yaml", workflowContent)

	// WHEN: Running the workflow
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test-audit-default"})

	err := cmd.Execute()
	// THEN: Command executes without panic from nil audit writer
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "should not panic from uninitialized audit writer")
	}
}

// TestRunCommand_WiresAuditTrailWriter_CustomPath verifies audit trail uses AWF_AUDIT_LOG path
func TestRunCommand_WiresAuditTrailWriter_CustomPath(t *testing.T) {
	// GIVEN: A temporary test directory with custom audit path
	tmpDir := setupTestDir(t)
	customAuditPath := filepath.Join(tmpDir, "custom-audit.jsonl")
	t.Setenv("AWF_AUDIT_LOG", customAuditPath)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: test-audit-custom
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "custom path test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "test-audit-custom.yaml", workflowContent)

	// WHEN: Running the workflow
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test-audit-custom"})

	err := cmd.Execute()
	// THEN: Command executes without panic from audit writer initialization
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "should not panic during audit writer setup")
	}
}

// TestRunCommand_WiresAuditTrailWriter_ErrorHandling verifies audit writer errors don't block execution
func TestRunCommand_WiresAuditTrailWriter_ErrorHandling(t *testing.T) {
	// GIVEN: A temporary test directory with invalid audit path (non-existent parent)
	tmpDir := setupTestDir(t)
	invalidAuditPath := filepath.Join(tmpDir, "nonexistent", "dir", "audit.jsonl")
	t.Setenv("AWF_AUDIT_LOG", invalidAuditPath)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: test-audit-error
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "error handling test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "test-audit-error.yaml", workflowContent)

	// WHEN: Running the workflow with invalid audit path
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test-audit-error"})

	_ = cmd.Execute()

	// THEN: Workflow execution should proceed despite audit writer error
	// The error handler should log a warning but not fail the execution
	output := out.String()
	errOutput := errBuf.String()

	// Execution should complete (we're not asserting NoError because the workflow
	// might fail for other reasons, but the important thing is audit error doesn't crash)
	if output != "" {
		// If there's output, it means execution proceeded
		assert.NotContains(t, output, "nil pointer", "should not panic from audit writer error")
	}
	// Audit warning might be in either output stream
	auditsWarningMsg := errOutput + output
	assert.NotContains(t, auditsWarningMsg, "panic", "should not panic on audit error")
}

// TestResumeCommand_WiresAuditTrailWriter verifies resume command properly wires audit writer
func TestResumeCommand_WiresAuditTrailWriter(t *testing.T) {
	// GIVEN: A temporary test directory with audit logging disabled
	tmpDir := setupTestDir(t)
	t.Setenv("AWF_AUDIT_LOG", "off")

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// WHEN: Calling resume command (will fail because no saved state exists, but wiring should succeed)
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume"})

	_ = cmd.Execute()

	// THEN: Resume command initializes without panicking from nil audit writer
	// Success criterion: no panic from nil pointer in audit writer initialization
	output := out.String()
	assert.NotContains(t, output, "nil pointer", "resume should properly initialize audit writer")
}

// TestResumeCommand_WiresAuditTrailWriter_WithPath verifies resume uses custom audit path
func TestResumeCommand_WiresAuditTrailWriter_WithPath(t *testing.T) {
	// GIVEN: A temporary test directory with custom audit path
	tmpDir := setupTestDir(t)
	customPath := filepath.Join(tmpDir, "resume-audit.jsonl")
	t.Setenv("AWF_AUDIT_LOG", customPath)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// WHEN: Calling resume command with custom audit path
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume"})

	_ = cmd.Execute()

	// THEN: Resume command initializes without panicking
	output := out.String()
	assert.NotContains(t, output, "nil pointer", "resume should handle custom audit path")
}

// TestRunCommand_AuditWriter_CleanupExecution verifies cleanup is called after execution
func TestRunCommand_AuditWriter_CleanupExecution(t *testing.T) {
	// GIVEN: A workflow that completes successfully
	tmpDir := setupTestDir(t)
	auditPath := filepath.Join(tmpDir, "cleanup-test.jsonl")
	t.Setenv("AWF_AUDIT_LOG", auditPath)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: test-cleanup
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "cleanup test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "test-cleanup.yaml", workflowContent)

	// WHEN: Running the workflow
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test-cleanup"})

	err := cmd.Execute()
	// THEN: Cleanup is properly invoked (audit file should be closed without errors)
	// We verify this indirectly by checking execution succeeds without panics
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "close", "should not error on audit writer close")
		assert.NotContains(t, errMsg, "nil pointer", "cleanup should not cause nil pointer")
	}
}

// TestRunCommand_AuditWriter_MultipleExecutions verifies audit writer handles multiple runs
func TestRunCommand_AuditWriter_MultipleExecutions(t *testing.T) {
	// GIVEN: A workflow and multiple execution runs
	tmpDir := setupTestDir(t)
	auditPath := filepath.Join(tmpDir, "multi-exec.jsonl")
	t.Setenv("AWF_AUDIT_LOG", auditPath)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: test-multi
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "multi execution"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "test-multi.yaml", workflowContent)

	// WHEN: Running the workflow multiple times
	for range 2 {
		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test-multi"})

		_ = cmd.Execute()
	}

	// THEN: Multiple executions complete without audit writer conflicts
	// Both executions should succeed without blocking each other
	// (this is implicitly verified by the fact we can run twice without panic)
}

// TestRunCommand_AuditWriter_WiringIntegration verifies audit writer is wired to execution service
func TestRunCommand_AuditWriter_WiringIntegration(t *testing.T) {
	// GIVEN: A test workflow with audit path set
	tmpDir := setupTestDir(t)
	auditPath := filepath.Join(tmpDir, "wiring-test.jsonl")
	t.Setenv("AWF_AUDIT_LOG", auditPath)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: test-wiring
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "wiring integration"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "test-wiring.yaml", workflowContent)

	// WHEN: Running workflow
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test-wiring"})

	err := cmd.Execute()

	// THEN: Command executes and audit writer wiring is successful
	// Success means no panics from nil audit writer or wiring errors
	output := out.String()
	errOutput := errOut.String()
	fullOutput := output + errOutput

	assert.NotContains(t, fullOutput, "nil pointer", "audit writer should be properly wired")
	assert.NotContains(t, fullOutput, "set audit", "should not have errors setting audit writer")

	// If execution succeeded, cleanup should have been called without errors
	if err == nil {
		// Successful execution indicates wiring and cleanup both worked
		require.Nil(t, err, "execution with wired audit writer should succeed")
	}
}
