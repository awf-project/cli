package cli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLITestMainOverridesInheritedAuditPath(t *testing.T) {
	if os.Getenv("AWF_TEST_MAIN_CHILD") == "1" {
		require.Empty(t, os.Getenv("AWF_AUDIT_LOG"))
		writer, cleanup, err := audit.NewWriterFromEnv()
		require.NoError(t, err)
		t.Cleanup(cleanup)
		require.NotNil(t, writer)
		event := workflow.NewStartedEvent(workflow.NewExecutionContext("isolated", "test-main"), nil, "test")
		require.NoError(t, writer.Write(context.Background(), &event))
		return
	}
	path := filepath.Join(t.TempDir(), "user-audit.jsonl")
	const original = "user audit must remain unchanged\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))
	executable, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestCLITestMainOverridesInheritedAuditPath$")
	cmd.Env = append(os.Environ(), "AWF_TEST_MAIN_CHILD=1", "AWF_AUDIT_LOG="+path)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, string(contents))
}
