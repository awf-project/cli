//go:build integration

package cli_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScriptFileShebang_Python verifies that a script with a Python shebang
// is executed by python3, not by $SHELL.
// AC: script_file with #!/usr/bin/env python3 runs under the Python interpreter.
func TestScriptFileShebang_Python(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found")
	}

	repoRoot, err := findRepoRoot()
	require.NoError(t, err)

	scriptPath := filepath.Join(repoRoot, "tests", "fixtures", "scripts", "shebang_python.py")
	_, statErr := os.Stat(scriptPath)
	require.NoError(t, statErr, "fixture shebang_python.py must exist")

	tmpDir := setupInitTestDir(t)

	workflowContent := fmt.Sprintf(`name: shebang-python
version: "1.0.0"
states:
  initial: run
  run:
    type: step
    script_file: %s
    on_success: done
  done:
    type: terminal
`, scriptPath)
	createTestWorkflow(t, tmpDir, "shebang-python.yaml", workflowContent)

	output, err := runCLI(t, "run", "shebang-python", "--output=buffered")
	require.NoError(t, err)
	assert.Contains(t, output, "python:", "script should be executed by python3 interpreter, not $SHELL")
}

// TestScriptFileShebang_Bash verifies that a script with a bash shebang
// is executed by /bin/bash, producing bash-specific output.
// AC: script_file with #!/bin/bash runs under bash and expands $BASH_VERSION.
func TestScriptFileShebang_Bash(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("/bin/bash not found")
	}

	repoRoot, err := findRepoRoot()
	require.NoError(t, err)

	scriptPath := filepath.Join(repoRoot, "tests", "fixtures", "scripts", "shebang_bash.sh")
	_, statErr := os.Stat(scriptPath)
	require.NoError(t, statErr, "fixture shebang_bash.sh must exist")

	tmpDir := setupInitTestDir(t)

	workflowContent := fmt.Sprintf(`name: shebang-bash
version: "1.0.0"
states:
  initial: run
  run:
    type: step
    script_file: %s
    on_success: done
  done:
    type: terminal
`, scriptPath)
	createTestWorkflow(t, tmpDir, "shebang-bash.yaml", workflowContent)

	output, err := runCLI(t, "run", "shebang-bash", "--output=buffered")
	require.NoError(t, err)
	assert.Contains(t, output, "bash:", "script should be executed by /bin/bash, producing bash-specific BASH_VERSION output")
}

// TestScriptFileShebang_NoShebang_BackwardCompat verifies that a script without
// a shebang line still executes correctly via $SHELL -c (backward compatibility).
// AC: script_file without shebang falls back to $SHELL execution.
func TestScriptFileShebang_NoShebang_BackwardCompat(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err)

	scriptPath := filepath.Join(repoRoot, "tests", "fixtures", "scripts", "no_shebang.sh")
	_, statErr := os.Stat(scriptPath)
	require.NoError(t, statErr, "fixture no_shebang.sh must exist")

	tmpDir := setupInitTestDir(t)

	workflowContent := fmt.Sprintf(`name: no-shebang
version: "1.0.0"
states:
  initial: run
  run:
    type: step
    script_file: %s
    on_success: done
  done:
    type: terminal
`, scriptPath)
	createTestWorkflow(t, tmpDir, "no-shebang.yaml", workflowContent)

	output, err := runCLI(t, "run", "no-shebang", "--output=buffered")
	require.NoError(t, err)
	assert.Contains(t, output, "no-shebang-executed", "script without shebang should fall back to $SHELL -c execution")
}

// TestScriptFileShebang_InlineCommand_Unchanged verifies that the shebang
// detection path does not affect inline command field execution.
// AC: Steps using command field are unaffected by script_file shebang logic.
func TestScriptFileShebang_InlineCommand_Unchanged(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	workflowContent := `name: inline-command
version: "1.0.0"
states:
  initial: run
  run:
    type: step
    command: echo "inline-command-executed"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "inline-command.yaml", workflowContent)

	output, err := runCLI(t, "run", "inline-command", "--output=buffered")
	require.NoError(t, err)
	assert.Contains(t, output, "inline-command-executed", "inline command field should execute normally, unaffected by shebang detection logic")
}
