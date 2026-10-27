//go:build integration

// Feature: B017
package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSignalPersistsOneTerminalAuditEvent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support os.Interrupt delivery to child processes")
	}

	repoRoot, err := findRepoRoot()
	require.NoError(t, err)
	root := t.TempDir()
	storagePath := filepath.Join(root, "storage")
	auditPath := filepath.Join(root, "audit.jsonl")
	xdgDataHome := filepath.Join(root, "xdg-data")
	defaultAuditPath := filepath.Join(xdgDataHome, "awf", "audit.jsonl")
	workflowDir := filepath.Join(root, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowDir, 0o755))
	requestStarted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		requestStarted <- struct{}{}
		<-request.Context().Done()
	}))
	t.Cleanup(server.Close)
	workflowSource := fmt.Sprintf(`name: signal
version: "1.0.0"
states:
  initial: wait
  wait:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: %q
    on_success: done
  done:
    type: terminal
`, server.URL)
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "signal.yaml"), []byte(workflowSource), 0o644))

	binaryPath := filepath.Join(root, "awf")
	build := exec.Command("go", "build", "-o", binaryPath+".tmp", "./cmd/awf")
	build.Dir = repoRoot
	require.NoError(t, build.Run())
	require.NoError(t, os.Rename(binaryPath+".tmp", binaryPath))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, "--storage="+storagePath, "run", "signal", "--skip-plugins")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"AWF_AUDIT_LOG="+auditPath,
		"XDG_DATA_HOME="+xdgDataHome,
		"XDG_CONFIG_HOME="+filepath.Join(root, "xdg-config"),
	)
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})

	var started workflow.AuditEvent
	require.Eventually(t, func() bool {
		events, readErr := readSignalAuditEvents(auditPath)
		if readErr != nil {
			return false
		}
		for i := range events {
			if events[i].Event == workflow.EventWorkflowStarted {
				started = events[i]
				return started.ExecutionID != ""
			}
		}
		return false
	}, 10*time.Second, 25*time.Millisecond, "workflow start was not durably persisted; output: %s", output.String())
	select {
	case <-requestStarted:
	case <-time.After(10 * time.Second):
		t.Fatalf("workflow operation did not start before signal deadline; output: %s", output.String())
	}

	require.NoError(t, cmd.Process.Signal(os.Interrupt))
	waitErr := cmd.Wait()
	var exitErr *exec.ExitError
	require.ErrorAs(t, waitErr, &exitErr, "output: %s", output.String())
	assert.Equal(t, 3, exitErr.ExitCode(), "output: %s", output.String())

	events, err := readSignalAuditEvents(auditPath)
	require.NoError(t, err)
	var completions []workflow.AuditEvent
	for i := range events {
		if events[i].ExecutionID == started.ExecutionID && events[i].Event == workflow.EventWorkflowCompleted {
			completions = append(completions, events[i])
		}
	}
	require.Len(t, completions, 1)
	require.NotNil(t, completions[0].ExitCode)
	assert.Equal(t, "failure", completions[0].Status)
	assert.Equal(t, 3, *completions[0].ExitCode)
	assert.Contains(t, completions[0].Error, "request cancelled")

	history, err := store.NewSQLiteHistoryStore(filepath.Join(storagePath, "history.db"))
	require.NoError(t, err)
	records, err := history.List(context.Background(), &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	require.NoError(t, history.Close())
	require.Len(t, records, 1)
	assert.Equal(t, started.ExecutionID, records[0].ID)
	assert.Equal(t, "cancelled", records[0].Status)
	assert.Equal(t, 3, records[0].ExitCode)
	assert.Contains(t, records[0].ErrorMessage, "request cancelled")
	_, err = os.Stat(defaultAuditPath)
	assert.ErrorIs(t, err, os.ErrNotExist, "custom audit output must not touch the default user audit path")
}

func readSignalAuditEvents(path string) ([]workflow.AuditEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var events []workflow.AuditEvent
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event workflow.AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}
