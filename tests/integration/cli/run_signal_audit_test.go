//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
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
	buildRoot := t.TempDir()
	binaryPath := filepath.Join(buildRoot, "awf")
	build := exec.Command("go", "build", "-o", binaryPath+".tmp", "./cmd/awf")
	build.Dir = repoRoot
	require.NoError(t, build.Run())
	require.NoError(t, os.Rename(binaryPath+".tmp", binaryPath))

	tests := []struct {
		name   string
		signal os.Signal
	}{
		{name: "SIGINT", signal: os.Interrupt},
		{name: "SIGTERM", signal: syscall.SIGTERM},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			storagePath := filepath.Join(root, "storage")
			auditPath := filepath.Join(root, "audit.jsonl")
			xdgDataHome := filepath.Join(root, "xdg-data")
			defaultAuditPath := filepath.Join(xdgDataHome, "awf", "audit.jsonl")
			workflowDir := filepath.Join(root, ".awf", "workflows")
			require.NoError(t, os.MkdirAll(workflowDir, 0o755))

			const (
				environmentSecret     = "environment-secret-b021"
				operationOutputSecret = "operation-output-secret-b021"
			)
			requestStarted := make(chan struct{}, 1)
			requestBody := make(chan string, 1)
			requestStopped := make(chan struct{})
			releaseRequest := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				body, _ := io.ReadAll(request.Body)
				requestBody <- string(body)
				requestStarted <- struct{}{}
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-request.Context().Done():
						close(requestStopped)
						return
					case <-releaseRequest:
						close(requestStopped)
						return
					case <-ticker.C:
						_, _ = response.Write([]byte(operationOutputSecret))
						if flusher, ok := response.(http.Flusher); ok {
							flusher.Flush()
						}
					}
				}
			}))
			t.Cleanup(func() {
				close(releaseRequest)
				server.CloseClientConnections()
				server.Close()
			})

			const requestSecret = "request-body-secret-b021"
			const headerSecret = "header-secret-b021"
			workflowSource := fmt.Sprintf(`name: signal
version: "1.0.0"
states:
  initial: wait
  wait:
    type: operation
    operation: http.request
    inputs:
      method: POST
      url: %q
      headers:
        Authorization: %q
      body: "{{env.B021_SECRET_ENV}}"
    on_success: done
  done:
    type: terminal
`, server.URL+"?request="+requestSecret, "Bearer "+headerSecret)
			require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "signal.yaml"), []byte(workflowSource), 0o600))

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binaryPath, "--storage="+storagePath, "run", "signal", "--skip-plugins", "--input=prompt=prompt-secret-b021", "--input=plugin_config=plugin-secret-b021", "--input=token=token-secret-b021")
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"AWF_AUDIT_LOG="+auditPath,
				"XDG_DATA_HOME="+xdgDataHome,
				"XDG_CONFIG_HOME="+filepath.Join(root, "xdg-config"),
				"B021_SECRET_ENV="+environmentSecret,
			)
			var output synchronizedBuffer
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
			startedPersisted := assert.Eventually(t, func() bool {
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
			}, 10*time.Second, 25*time.Millisecond, "workflow start was not durably persisted")
			require.True(t, startedPersisted, "workflow output: %s", output.String())
			select {
			case <-requestStarted:
			case <-time.After(10 * time.Second):
				t.Fatalf("workflow operation did not start before signal deadline; output: %s", output.String())
			}
			select {
			case body := <-requestBody:
				require.Equal(t, environmentSecret, body, "workflow must interpolate the confidential environment value into the exercised operation")
			case <-time.After(time.Second):
				t.Fatal("workflow operation did not submit its interpolated request body")
			}

			require.NoError(t, cmd.Process.Signal(tt.signal))
			waitErr := cmd.Wait()
			var exitErr *exec.ExitError
			require.ErrorAs(t, waitErr, &exitErr, "output: %s", output.String())
			assert.Equal(t, 3, exitErr.ExitCode(), "output: %s", output.String())
			select {
			case <-requestStopped:
			case <-time.After(time.Second):
				t.Fatal("active HTTP operation did not stop after root context cancellation")
			}

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
			assert.Contains(t, completions[0].Error, context.Canceled.Error())

			stateStore := store.NewJSONStore(filepath.Join(storagePath, "states"))
			ids, err := stateStore.List(context.Background())
			require.NoError(t, err)
			require.Len(t, ids, 1)
			state, err := stateStore.Load(context.Background(), ids[0])
			require.NoError(t, err)
			require.NotNil(t, state)
			assert.Equal(t, started.ExecutionID, state.HistoryRecordID)
			assert.Equal(t, workflow.StatusCancelled, state.Status)
			assert.Equal(t, 3, state.ExitCode)

			history, err := store.NewSQLiteHistoryStore(filepath.Join(storagePath, "history.db"))
			require.NoError(t, err)
			records, err := history.List(context.Background(), &workflow.HistoryFilter{Limit: 100})
			require.NoError(t, err)
			require.NoError(t, history.Close())
			require.Len(t, records, 1)
			assert.Equal(t, state.WorkflowID, records[0].WorkflowID)
			assert.Equal(t, state.Status.String(), records[0].Status)
			assert.Equal(t, state.ExitCode, records[0].ExitCode)
			assert.Contains(t, records[0].ErrorMessage, context.Canceled.Error())

			artifactDiagnostics, err := readSignalDiagnosticArtifacts(auditPath, xdgDataHome, storagePath)
			require.NoError(t, err)
			capturedDiagnostics := output.String() + completions[0].Error + records[0].ErrorMessage + artifactDiagnostics
			for _, secret := range []string{requestSecret, headerSecret, "prompt-secret-b021", environmentSecret, operationOutputSecret, "plugin-secret-b021", "token-secret-b021"} {
				assert.NotContains(t, capturedDiagnostics, secret)
			}
			_, err = os.Stat(defaultAuditPath)
			assert.ErrorIs(t, err, os.ErrNotExist, "custom audit output must not touch the default user audit path")
		})
	}
}

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer.String()
}

func readSignalDiagnosticArtifacts(paths ...string) (string, error) {
	var diagnostics strings.Builder
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if !info.IsDir() {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", readErr
			}
			diagnostics.Write(data)
			continue
		}

		err = filepath.WalkDir(path, func(artifactPath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !isSignalDiagnosticArtifact(artifactPath) {
				return nil
			}
			data, readErr := os.ReadFile(artifactPath)
			if readErr != nil {
				return readErr
			}
			diagnostics.Write(data)
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return diagnostics.String(), nil
}

func isSignalDiagnosticArtifact(path string) bool {
	extension := filepath.Ext(path)
	if extension == ".log" || extension == ".jsonl" {
		return true
	}
	for _, component := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		if component == "logs" {
			return true
		}
	}
	return false
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
