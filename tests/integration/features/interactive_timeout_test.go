//go:build integration

package features_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInteractiveRun_TemplateTimeoutCancelsActiveCommand_Integration exercises
// the public interactive command with its real YAML repository, template service,
// JSON state store, and shell executor. The timeout originates in the referenced
// template, rather than the workflow step, so Run must consume the expanded value.
func TestInteractiveRun_TemplateTimeoutCancelsActiveCommand_Integration(t *testing.T) {
	workflowDir := t.TempDir()
	storageDir := t.TempDir()
	templateDir := filepath.Join(storageDir, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o700))
	t.Setenv("AWF_WORKFLOWS_PATH", workflowDir)

	const templateYAML = `name: interactive-timeout
states:
  initial: wait
  wait:
    type: command
    command: sleep 5
    timeout: 1s
`
	const workflowYAML = `name: interactive-template-timeout
states:
  initial: start
  start:
    type: command
    command: echo placeholder
    use_template: interactive-timeout
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "interactive-timeout.yaml"), []byte(templateYAML), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "interactive-template-timeout.yaml"), []byte(workflowYAML), 0o600))

	cmd := cli.NewRootCommand()
	output := new(bytes.Buffer)
	cmd.SetOut(output)
	cmd.SetErr(output)
	cmd.SetIn(strings.NewReader("c\n"))
	cmd.SetArgs([]string{"--storage=" + storageDir, "run", "interactive-template-timeout", "--interactive", "--no-color"})

	startedAt := time.Now()
	err := cmd.ExecuteContext(context.Background())
	elapsed := time.Since(startedAt)

	require.Error(t, err, "the template-provided timeout must cancel active interactive work")
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "timeout must preserve the deadline sentinel: %v", err)
	assert.Less(t, elapsed, 3*time.Second, "the 1s template timeout must interrupt sleep 5")
	assert.Contains(t, output.String(), "Timeout: 1s", "the interactive view must receive the resolved template timeout")
}
