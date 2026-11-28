//go:build integration && !windows

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/plugin/sdk"
	"github.com/stretchr/testify/require"
)

const (
	serveFixtureEnv = "AWF_SERVE_TRACKER_FIXTURE"
	adapterResult   = "canonical-tracker-result"
	workflowName    = "tracker-http-delegation"
	listeningSignal = "AWF API server listening on http://"

	fixtureTitle               = "fixture-title-marker"
	fixtureDescription         = "fixture-description-marker"
	fixtureIssueIdentity       = "fixture-issue-identity-marker"
	fixtureAdapterOutputSecret = "fixture-adapter-output-secret-marker"
)

var fixtureSensitiveMarkers = []string{
	fixtureTitle,
	fixtureDescription,
	fixtureIssueIdentity,
	fixtureAdapterOutputSecret,
}

// synchronizedBuffer permits readiness polling to inspect the serve process's
// output while os/exec continues to write it.
type synchronizedBuffer struct {
	mu sync.RWMutex
	b  bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *synchronizedBuffer) Contains(value string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return bytes.Contains(b.b.Bytes(), []byte(value))
}

type serveTrackerFixture struct {
	sdk.BasePlugin
}

func (*serveTrackerFixture) Operations() []string { return []string{"create_issue"} }

func (*serveTrackerFixture) Init(_ context.Context, _ map[string]any) error {
	root := os.Getenv(serveFixtureEnv)
	return os.WriteFile(filepath.Join(root, "adapter.pid"), []byte(strconv.Itoa(os.Getpid())), 0o600) //nolint:gosec // The parent supplies a test-owned fixture directory.
}

func (*serveTrackerFixture) HandleOperation(
	ctx context.Context,
	_ string,
	_ map[string]any,
) (*sdk.OperationResult, error) {
	root := os.Getenv(serveFixtureEnv)
	if err := os.WriteFile(filepath.Join(root, "adapter.called"), nil, 0o600); err != nil { //nolint:gosec // The parent supplies a test-owned fixture directory.
		return nil, err
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if release, err := os.ReadFile(filepath.Join(root, "release")); err == nil {
			if string(release) == "fail" {
				return nil, errors.New(fixtureAdapterOutputSecret)
			} //nolint:gosec // The parent supplies a test-owned fixture directory.
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
	return sdk.NewSuccessResult(adapterResult, map[string]any{
		"id": fixtureIssueIdentity, "key": adapterResult, "url": "https://tracker.invalid/result",
		"secret": fixtureAdapterOutputSecret,
	}), nil
}

func (*serveTrackerFixture) Shutdown(_ context.Context) error {
	root := os.Getenv(serveFixtureEnv)
	return os.WriteFile(filepath.Join(root, "adapter.stopped"), nil, 0o600) //nolint:gosec // The parent supplies a test-owned fixture directory.
}

func TestMain(m *testing.M) {
	if os.Getenv("AWF_PLUGIN") == "awf-plugin-v1" && os.Getenv(serveFixtureEnv) != "" {
		sdk.Serve(&serveTrackerFixture{BasePlugin: sdk.BasePlugin{
			PluginName:    "tracker-jira",
			PluginVersion: "1.0.0",
		}})
		return
	}
	os.Exit(m.Run())
}

func TestServeWorkflowDelegatesTrackerOperationThroughInstalledAdapter(t *testing.T) {
	runServeTrackerWorkflow(t, "ready", "completed")
}

func TestServeTrackerWorkflowHidesAdapterFailureDetails(t *testing.T) {
	runServeTrackerWorkflow(t, "fail", "failed")
}

func runServeTrackerWorkflow(t *testing.T, releaseMode, expectedStatus string) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	root := t.TempDir()
	workflowRoot := filepath.Join(root, "workflows")
	xdgDataRoot := filepath.Join(root, "xdg-data")
	pluginRoot := filepath.Join(xdgDataRoot, "awf", "plugins")
	fixtureRoot := filepath.Join(root, "fixture")
	for _, dir := range []string{workflowRoot, pluginRoot, fixtureRoot} {
		require.NoError(t, os.MkdirAll(dir, 0o700))
	}

	repoRoot := findRepositoryRoot(t)
	awfBinary := filepath.Join(root, "awf")
	build := exec.CommandContext(testContext(t), "go", "build", "-buildvcs=false", "-o", awfBinary, "./cmd/awf")
	build.Dir = repoRoot
	require.NoError(t, build.Run(), "AWF test binary must build")

	testBinary, err := os.Executable()
	require.NoError(t, err)
	pluginDir := filepath.Join(pluginRoot, "tracker-jira")
	require.NoError(t, os.MkdirAll(pluginDir, 0o700))
	writePrivateFile(t, filepath.Join(pluginDir, "plugin.yaml"), `name: tracker-jira
version: 1.0.0
awf_version: ">=0.1.0"
capabilities:
  - operations
`)
	require.NoError(t, os.Symlink(testBinary, filepath.Join(pluginDir, "awf-plugin-tracker-jira")))

	configPath := filepath.Join(root, "config.yaml")
	writePrivateFile(t, configPath, "tracker:\n  default_backend: jira\n")
	writePrivateFile(t, filepath.Join(workflowRoot, workflowName+".yaml"), `name: `+workflowName+`
version: "1.0.0"
states:
  initial: create
  create:
    type: operation
    operation: tracker.create_issue
    inputs: {title: `+fixtureTitle+`, description: `+fixtureDescription+`}
    transitions:
      - when: "states.create.Output contains 'key=`+adapterResult+`'"
        goto: done
      - goto: invalid_result
  done:
    type: terminal
    status: success
  invalid_result:
    type: terminal
    status: failure
`)

	address := reserveLoopbackAddress(t)
	var logs synchronizedBuffer
	command := exec.Command(awfBinary, "--storage="+filepath.Join(root, "storage"), "serve", "--host="+address.IP, "--port="+address.Port) //nolint:gosec // Arguments and binary are test-owned.
	command.Dir = root
	command.Stdout = &logs
	command.Stderr = &logs
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Env = append(os.Environ(),
		"AWF_CONFIG_PATH="+configPath,
		"AWF_WORKFLOWS_PATH="+workflowRoot,
		"AWF_PLUGINS_PATH="+pluginRoot,
		serveFixtureEnv+"="+fixtureRoot,
		"XDG_CONFIG_HOME="+filepath.Join(root, "xdg-config"),
		"XDG_DATA_HOME="+xdgDataRoot,
		"XDG_STATE_HOME="+filepath.Join(root, "xdg-state"),
		"AWF_AUDIT_LOG=off",
	)
	require.NoError(t, command.Start())
	processDone := make(chan error, 1)
	go func() { processDone <- command.Wait() }()
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-processDone
		}
	})

	client := &http.Client{Timeout: 2 * time.Second}
	baseURL := "http://" + address.String()
	waitForWorkflowReadiness(t, client, baseURL, &logs)

	runBody := doJSONRequest(t, client, http.MethodPost, baseURL+"/api/workflows/local/"+workflowName+"/run", `{"inputs":{}}`, http.StatusAccepted)
	var runResponse struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	require.NoError(t, json.Unmarshal(runBody, &runResponse), "run response must be valid JSON")
	require.NotEmpty(t, runResponse.Body.ExecutionID, "run response must include an execution ID")

	events := make(chan []byte, 1)
	eventErrors := make(chan error, 1)
	eventReady := make(chan struct{})
	go collectEvents(client, baseURL+"/api/executions/"+runResponse.Body.ExecutionID+"/events", eventReady, events, eventErrors)
	select {
	case <-eventReady:
	case eventErr := <-eventErrors:
		require.NoError(t, eventErr, "event stream must connect")
	case <-time.After(3 * time.Second):
		t.Fatal("event stream did not connect")
	}
	writePrivateFile(t, filepath.Join(fixtureRoot, "release"), releaseMode)
	waitForTrackerTerminalStatus(t, client, baseURL, runResponse.Body.ExecutionID, fixtureRoot, expectedStatus)

	var eventBody []byte
	select {
	case eventBody = <-events:
	case eventErr := <-eventErrors:
		require.NoError(t, eventErr, "event stream must complete")
	case <-time.After(5 * time.Second):
		t.Fatal("event stream did not complete after terminal status")
	}
	expectedEvent := `"kind":"workflow.` + expectedStatus + `"`
	require.True(t, bytes.Contains(eventBody, []byte(expectedEvent)), "event stream must report the expected terminal status")
	for _, marker := range fixtureSensitiveMarkers {
		if bytes.Contains(eventBody, []byte(marker)) {
			t.Fatal("event stream exposed a protected fixture marker")
		}
	}
	require.FileExists(t, filepath.Join(fixtureRoot, "adapter.called"), "installed adapter must execute")

	require.NoError(t, syscall.Kill(command.Process.Pid, syscall.SIGTERM))
	select {
	case waitErr := <-processDone:
		require.NoError(t, waitErr, "serve process must stop cleanly after SIGTERM")
		stopped = true
	case <-time.After(8 * time.Second):
		t.Fatal("serve process did not stop after SIGTERM")
	}
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(filepath.Join(fixtureRoot, "adapter.stopped"))
		return statErr == nil
	}, 3*time.Second, 20*time.Millisecond, "adapter must shut down with the host")
	pidBytes, err := os.ReadFile(filepath.Join(fixtureRoot, "adapter.pid")) //nolint:gosec // The path is inside the test-owned fixture directory.
	require.NoError(t, err)
	adapterPID, err := strconv.Atoi(string(pidBytes))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return !processExists(adapterPID) }, 3*time.Second, 20*time.Millisecond,
		"adapter process must not survive host shutdown")

	requireNoSensitiveMarkers(t, &logs)
}

type loopbackAddress struct {
	IP   string
	Port string
}

func (a loopbackAddress) String() string { return net.JoinHostPort(a.IP, a.Port) }

func reserveLoopbackAddress(t *testing.T) loopbackAddress {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(testContext(t), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().(*net.TCPAddr)
	require.NoError(t, listener.Close())
	return loopbackAddress{IP: address.IP.String(), Port: strconv.Itoa(address.Port)}
}

func waitForWorkflowReadiness(t *testing.T, client *http.Client, baseURL string, logs *synchronizedBuffer) {
	t.Helper()
	require.Eventually(t, func() bool {
		if !logs.Contains(listeningSignal) {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/workflows", http.NoBody)
		if err != nil {
			return false
		}
		response, err := client.Do(request)
		if err != nil {
			return false
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return err == nil && response.StatusCode == http.StatusOK && bytes.Contains(body, []byte(workflowName))
	}, 8*time.Second, 25*time.Millisecond, "serve did not emit its listening signal and expose the tracker workflow")
}

func requireNoSensitiveMarkers(t *testing.T, logs *synchronizedBuffer) {
	t.Helper()
	for _, marker := range fixtureSensitiveMarkers {
		if logs.Contains(marker) {
			t.Fatal("captured host logs exposed a protected fixture marker")
		}
	}
}

func waitForTrackerTerminalStatus(t *testing.T, client *http.Client, baseURL, executionID, fixtureRoot, expectedStatus string) {
	t.Helper()
	observedStatus := "unavailable"
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		body, status, err := requestBody(client, http.MethodGet, baseURL+"/api/executions/"+executionID, "")
		if err != nil {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		if status != http.StatusOK {
			observedStatus = fmt.Sprintf("http-%d", status)
			time.Sleep(25 * time.Millisecond)
			continue
		}
		var response struct {
			Body struct {
				Status string `json:"status"`
			} `json:"body"`
		}
		if json.Unmarshal(body, &response) != nil {
			observedStatus = "invalid-json"
			time.Sleep(25 * time.Millisecond)
			continue
		}
		observedStatus = response.Body.Status
		if observedStatus == expectedStatus {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	_, callErr := os.Stat(filepath.Join(fixtureRoot, "adapter.called"))
	t.Fatalf("execution did not reach terminal status %s; last status: %s; adapter called: %t", expectedStatus, observedStatus, callErr == nil)
}

func collectEvents(client *http.Client, endpoint string, ready chan<- struct{}, bodies chan<- []byte, errs chan<- error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		errs <- err
		return
	}
	response, err := client.Do(request)
	if err != nil {
		errs <- err
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		errs <- fmt.Errorf("event stream returned HTTP %d", response.StatusCode)
		return
	}
	close(ready)
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		errs <- err
		return
	}
	bodies <- body
}

func doJSONRequest(t *testing.T, client *http.Client, method, endpoint, payload string, expectedStatus int) []byte {
	t.Helper()
	body, status, err := requestBody(client, method, endpoint, payload)
	require.NoError(t, err, "HTTP request must complete")
	require.Equal(t, expectedStatus, status, "HTTP request returned an unexpected status")
	return body
}

func requestBody(client *http.Client, method, endpoint, payload string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	if payload != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	return body, response.StatusCode, err
}

func writePrivateFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func findRepositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	deadline, ok := t.Deadline()
	if !ok {
		return context.Background()
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	t.Cleanup(cancel)
	return ctx
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || !errors.Is(err, syscall.ESRCH)
}
