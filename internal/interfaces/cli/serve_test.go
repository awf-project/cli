package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackerServeTestBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *trackerServeTestBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *trackerServeTestBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestServePropagatesTrackerDefaultBackend(t *testing.T) {
	t.Setenv("AWF_TEST_MODE", "1")
	tempDir := setupTestDir(t)
	dataHome := filepath.Join(tempDir, "data")
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "config-home"))
	t.Setenv("AWF_PLUGINS_PATH", filepath.Join(tempDir, "plugins"))
	setupProjectConfig(t, tempDir, "tracker:\n  default_backend: jira\n")
	createTestWorkflow(t, tempDir, "tracker-serve.yaml", `name: tracker-serve
version: "1.0.0"
states:
  initial: create
  create:
    type: operation
    operation: tracker.create_issue
    inputs:
      title: Release blocker
    on_success: done
  done:
    type: terminal
    status: success
`)

	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	command := cli.NewRootCommand()
	var output trackerServeTestBuffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"serve", "--host=127.0.0.1", fmt.Sprintf("--port=%d", port)})
	done := make(chan error, 1)
	go func() { done <- command.Execute() }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		request, requestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/api/workflows", http.NoBody)
		if requestErr != nil {
			return false
		}
		response, requestErr := http.DefaultClient.Do(request)
		if requestErr != nil {
			return false
		}
		_ = response.Body.Close()
		return response.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond, output.String())

	request, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, baseURL+"/api/workflows/local/tracker-serve/run",
		strings.NewReader(`{"inputs":{}}`),
	)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	responseBody, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusAccepted, response.StatusCode, string(responseBody))
	var accepted struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	require.NoError(t, json.Unmarshal(responseBody, &accepted))
	require.NotEmpty(t, accepted.Body.ExecutionID, string(responseBody))

	require.Eventually(t, func() bool {
		resolvedBackend := false
		_ = filepath.WalkDir(dataHome, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			contents, readErr := os.ReadFile(path) //nolint:gosec // WalkDir is restricted to the test-owned data root.
			if readErr == nil && (strings.Contains(string(contents), "tracker-jira") ||
				strings.Contains(string(contents), "adapter unavailable")) {
				resolvedBackend = true
			}
			return nil
		})
		return resolvedBackend
	}, 5*time.Second, 20*time.Millisecond, "serve execution never resolved tracker.default_backend; output: %s", output.String())

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	select {
	case serveErr := <-done:
		require.NoError(t, serveErr, output.String())
	case <-time.After(5 * time.Second):
		t.Fatal("serve command did not stop after SIGINT")
	}
}

// TestServeWithoutTrackerDefaultPreservesZeroValue verifies the public HTTP
// execution path preserves an omitted tracker.default_backend rather than
// selecting an implicit backend.
func TestServeWithoutTrackerDefaultPreservesZeroValue(t *testing.T) {
	t.Setenv("AWF_TEST_MODE", "1")
	tempDir := setupTestDir(t)
	dataHome := filepath.Join(tempDir, "data")
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "config-home"))
	t.Setenv("AWF_PLUGINS_PATH", filepath.Join(tempDir, "plugins"))
	setupProjectConfig(t, tempDir, "inputs: {}\n")
	createTestWorkflow(t, tempDir, "tracker-serve-no-default.yaml", `name: tracker-serve-no-default
version: "1.0.0"
states:
  initial: create
  create:
    type: operation
    operation: tracker.create_issue
    inputs:
      title: Release blocker
    on_success: done
  done:
    type: terminal
    status: success
`)

	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	command := cli.NewRootCommand()
	var output trackerServeTestBuffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"serve", "--host=127.0.0.1", fmt.Sprintf("--port=%d", port)})
	done := make(chan error, 1)
	go func() { done <- command.Execute() }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		request, requestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/api/workflows", http.NoBody)
		if requestErr != nil {
			return false
		}
		response, requestErr := http.DefaultClient.Do(request)
		if requestErr != nil {
			return false
		}
		_ = response.Body.Close()
		return response.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond, output.String())

	request, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, baseURL+"/api/workflows/local/tracker-serve-no-default/run",
		strings.NewReader(`{"inputs":{}}`),
	)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusAccepted, response.StatusCode)

	require.Eventually(t, func() bool {
		backendRequired := false
		_ = filepath.WalkDir(dataHome, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			contents, readErr := os.ReadFile(path) //nolint:gosec // WalkDir is restricted to the test-owned data root.
			if readErr == nil && strings.Contains(string(contents), "tracker backend is required") {
				backendRequired = true
			}
			return nil
		})
		return backendRequired
	}, 5*time.Second, 20*time.Millisecond, "serve execution did not preserve the zero-value tracker backend; output: %s", output.String())

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	select {
	case serveErr := <-done:
		require.NoError(t, serveErr, output.String())
	case <-time.After(5 * time.Second):
		t.Fatal("serve command did not stop after SIGINT")
	}
}

func TestServeCommand_FlagDefaults_PortAndHost(t *testing.T) {
	cmd := cli.NewServeCommand()

	portFlag := cmd.Flags().Lookup("port")
	require.NotNil(t, portFlag, "expected --port flag to exist")
	assert.Equal(t, "2511", portFlag.DefValue, "expected default port to be 2511")

	hostFlag := cmd.Flags().Lookup("host")
	require.NotNil(t, hostFlag, "expected --host flag to exist")
	assert.Equal(t, "127.0.0.1", hostFlag.DefValue, "expected default host to be 127.0.0.1")
}

func TestServeCommand_BindsLocalhostByDefault_NFR005(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"serve", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "127.0.0.1:2511", "expected help to show localhost binding")
	assert.Contains(t, output, "prevent inadvertent network exposure", "expected security warning in help")
}

func TestServeCommand_CommandStructure(t *testing.T) {
	cmd := cli.NewServeCommand()

	assert.Equal(t, "serve", cmd.Use, "expected Use field to be 'serve'")
	assert.NotEmpty(t, cmd.Short, "expected Short description to be set")
	assert.NotEmpty(t, cmd.Long, "expected Long description to be set")
	assert.Contains(t, cmd.Short, "API", "expected Short to mention API")
	assert.Contains(t, cmd.Long, "2511", "expected Long to mention default port")
	assert.Contains(t, cmd.Long, "127.0.0.1", "expected Long to mention localhost binding")
}

func TestServeCommand_IsRegisteredInRoot(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "serve" {
			found = true
			break
		}
	}

	assert.True(t, found, "expected 'serve' command to be registered in root")
}

func TestServeCommand_FlagsExist(t *testing.T) {
	cmd := cli.NewServeCommand()

	tests := []struct {
		flagName string
	}{
		{"port"},
		{"host"},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			require.NotNil(t, flag, "expected --%s flag to exist", tt.flagName)
		})
	}
}

func TestServeCommand_FlagTypes(t *testing.T) {
	cmd := cli.NewServeCommand()

	portFlag := cmd.Flags().Lookup("port")
	require.NotNil(t, portFlag)
	assert.Equal(t, "int", portFlag.Value.Type(), "expected --port to be int type")

	hostFlag := cmd.Flags().Lookup("host")
	require.NotNil(t, hostFlag)
	assert.Equal(t, "string", hostFlag.Value.Type(), "expected --host to be string type")
}

func TestServeCommand_NonLoopbackHostAccepted(t *testing.T) {
	cmd := cli.NewServeCommand()

	err := cmd.ParseFlags([]string{"--host=0.0.0.0", "--port=8080"})
	require.NoError(t, err, "non-loopback host must be accepted at flag parse time (warning emitted at runtime)")

	hostFlag := cmd.Flags().Lookup("host")
	require.NotNil(t, hostFlag)
	assert.Equal(t, "0.0.0.0", hostFlag.Value.String(), "parsed non-loopback host must be preserved exactly")

	portFlag := cmd.Flags().Lookup("port")
	require.NotNil(t, portFlag)
	assert.Equal(t, "8080", portFlag.Value.String(), "parsed non-default port must be preserved exactly")
}

func TestServeCommand_HasRunE(t *testing.T) {
	cmd := cli.NewServeCommand()
	assert.NotNil(t, cmd.RunE, "expected RunE to be set")
}

func TestServeCommand_FlagHelpText(t *testing.T) {
	cmd := cli.NewServeCommand()

	portFlag := cmd.Flags().Lookup("port")
	assert.NotEmpty(t, portFlag.Usage, "expected --port to have help text")

	hostFlag := cmd.Flags().Lookup("host")
	assert.NotEmpty(t, hostFlag.Usage, "expected --host to have help text")
	assert.Contains(t, strings.ToLower(hostFlag.Usage), "bind", "expected host flag help to mention 'bind'")
}

func TestServeCommand_DefaultsInHelpOutput(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"serve", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "2511", "expected help output to show port default")
	assert.Contains(t, output, "127.0.0.1", "expected help output to show host default")
}

func TestServeCommand_ExecutesWithDefaults(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Don't actually run - just test that flags are parsed without error
	// The stub implementation returns nil, so execution would succeed if we ran it
	cmd.SetArgs([]string{"serve"})

	// Verify command can be found and executed (stub returns nil)
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "serve" {
			found = true
			assert.NotNil(t, sub.RunE, "expected RunE to exist")
			break
		}
	}
	assert.True(t, found, "expected serve command to be available")
}
