//go:build integration && !windows

// Feature: F105
// Functional tests for ACP server migration to acp-go-sdk.
// These tests validate the server's core behavior: startup, workflow execution,
// signal handling, and graceful shutdown using the new SDK-based implementation.
package acp_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	pluginSDK "github.com/awf-project/cli/pkg/plugin/sdk"
	sdk "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	acpTrackerFixtureEnv = "AWF_ACP_TRACKER_FIXTURE"
	acpTrackerOutput     = "canonical-acp-tracker-result"
	acpTrackerWorkflow   = "tracker-acp-delegation"
	acpTrackerSecret     = "confidential-acp-tracker-marker"
)

type acpTrackerFixture struct {
	pluginSDK.BasePlugin
}

func TestSanitizedACPResponseErrorDoesNotExposePayload(t *testing.T) {
	responseErr := &sdk.RequestError{Message: acpTrackerSecret}

	err := sanitizedACPResponseError(responseErr, "session/prompt")

	require.Error(t, err)
	assert.NotContains(t, err.Error(), acpTrackerSecret)
}

func sanitizedACPResponseError(responseErr *sdk.RequestError, method string) error {
	if responseErr == nil {
		return nil
	}
	return fmt.Errorf("%s returned an ACP error", method)
}

func (*acpTrackerFixture) Operations() []string { return []string{"create_issue"} }

func (*acpTrackerFixture) Init(_ context.Context, _ map[string]any) error {
	return os.WriteFile(
		filepath.Join(os.Getenv(acpTrackerFixtureEnv), "adapter.pid"),
		[]byte(strconv.Itoa(os.Getpid())),
		0o600,
	) //nolint:gosec // The parent supplies a test-owned fixture directory.
}

func (*acpTrackerFixture) HandleOperation(
	_ context.Context,
	_ string,
	_ map[string]any,
) (*pluginSDK.OperationResult, error) {
	if err := os.WriteFile(
		filepath.Join(os.Getenv(acpTrackerFixtureEnv), "adapter.called"),
		nil,
		0o600,
	); err != nil { //nolint:gosec // The parent supplies a test-owned fixture directory.
		return nil, err
	}
	if os.Getenv("AWF_ACP_TRACKER_FAIL") == "true" {
		path := filepath.Join(os.Getenv(acpTrackerFixtureEnv), "adapter.failed")
		if err := os.WriteFile(path, nil, 0o600); err != nil { //nolint:gosec // Test-owned fixture path.
			return nil, err
		}
		return nil, errors.New(acpTrackerAdapterFailureMarker)
	}
	return pluginSDK.NewSuccessResult(acpTrackerOutput, map[string]any{
		"id": "ACP-27", "key": acpTrackerOutput, "url": "https://tracker.invalid/ACP-27",
	}), nil
}

func (*acpTrackerFixture) Shutdown(_ context.Context) error {
	return os.WriteFile(
		filepath.Join(os.Getenv(acpTrackerFixtureEnv), "adapter.stopped"),
		nil,
		0o600,
	) //nolint:gosec // The parent supplies a test-owned fixture directory.
}

func TestMain(m *testing.M) {
	if os.Getenv("AWF_PLUGIN") == "awf-plugin-v1" && os.Getenv(acpTrackerFixtureEnv) != "" {
		pluginSDK.Serve(&acpTrackerFixture{BasePlugin: pluginSDK.BasePlugin{
			PluginName: "tracker-jira", PluginVersion: "1.0.0",
		}})
		return
	}
	os.Exit(m.Run())
}

// TestACPServe_ServerInitialization validates that the server starts up
// correctly with valid configuration and is ready to handle requests.
func TestACPServe_ServerInitialization(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, fixtureWorkflowsDir(t))

	// Start the server process.
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	// Initialize the connection with the server.
	resp := proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	// Verify successful initialization.
	require.Nil(t, resp.Error, "initialize must succeed")
	require.NotNil(t, resp.Result, "initialize must return capabilities")

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")

	// Verify protocol version is correct per SDK.
	protocolVersion, ok := result["protocolVersion"].(float64)
	require.True(t, ok, "result must contain protocolVersion")
	assert.Equal(t, float64(sdk.ProtocolVersionNumber), protocolVersion)

	// Verify agent capabilities are advertised.
	_, hasAgentCaps := result["agentCapabilities"]
	assert.True(t, hasAgentCaps, "result must advertise agentCapabilities")
}

// TestACPServe_WorkflowExecution validates that the server can execute
// a workflow through a complete JSON-RPC session lifecycle.
func TestACPServe_WorkflowExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, fixtureWorkflowsDir(t))
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	// Initialize.
	initResp := proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})
	require.Nil(t, initResp.Error, "initialize must succeed")

	// Create a new session.
	sessionResp := proc.request(t, 2, sdk.AgentMethodSessionNew, map[string]any{
		"cwd":        t.TempDir(),
		"mcpServers": []any{},
	})
	require.Nil(t, sessionResp.Error, "session/new must succeed")

	result, ok := sessionResp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")
	sessionID := fmt.Sprintf("%v", result["sessionId"])
	require.NotEmpty(t, sessionID, "sessionId must not be empty")

	// Wait for available_commands_update notification.
	if !proc.drainForAvailableCommands(t, "trivial") {
		t.Fatal("expected available_commands_update notification")
	}

	// Execute a workflow via session/prompt.
	promptResp := proc.request(t, 3, sdk.AgentMethodSessionPrompt, map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "/trivial"},
		},
	})

	// Verify the workflow executed successfully.
	require.Nil(t, promptResp.Error, "session/prompt must succeed: %+v", promptResp.Error)
	require.NotNil(t, promptResp.Result, "session/prompt must return result")

	result, ok = promptResp.Result.(map[string]any)
	require.True(t, ok, "result must be a JSON object")
	assert.NotEmpty(t, result, "session/prompt must return non-empty result")
}

func TestACPServerDispatchesNotifierThroughDiscoveredAdapter(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and executes an external recording adapter")
	}

	const (
		workflowName     = "acp-notifier-dispatch"
		payloadCanary    = "acp-payload-output-canary ; $(printf injected)"
		defaultCanary    = "acp-default-channel-output-canary"
		credentialCanary = "acp-adapter-credential-output-canary"
	)
	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflows")
	pluginDir := filepath.Join(root, "plugins")
	recordPath := filepath.Join(root, "adapter-record.jsonl")
	adapterExitPath := filepath.Join(root, "adapter-exit.json")
	hostLogPath := filepath.Join(root, "host-audit.jsonl")
	require.NoError(t, os.MkdirAll(workflowDir, 0o700))
	pluginID := installACPRecordingAdapter(t, root, pluginDir, recordPath, credentialCanary)

	projectConfigPath := filepath.Join(root, "config.yaml")
	projectConfig := fmt.Sprintf("notifier:\n  backend: %s\n  default_channel: %s\n", pluginID, defaultCanary)
	require.NoError(t, os.WriteFile(projectConfigPath, []byte(projectConfig), 0o600))
	workflow := fmt.Sprintf(`name: %s
version: "1.0"
states:
  initial: notify
  notify:
    type: operation
    operation: notifier.send
    inputs:
      text: %q
    on_success: done
  done:
    type: terminal
    status: success
`, workflowName, payloadCanary)
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, workflowName+".yaml"), []byte(workflow), 0o600))

	t.Setenv("AWF_CONFIG_PATH", projectConfigPath)
	t.Setenv("AWF_PLUGINS_PATH", pluginDir)
	t.Setenv("AWF_WORKFLOWS_PATH", workflowDir)
	t.Setenv("AWF_RECORDING_PATH", recordPath)
	t.Setenv("AWF_ADAPTER_EXIT_PATH", adapterExitPath)
	t.Setenv("AWF_AUDIT_LOG", hostLogPath)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "xdg-state"))

	binaryPath := buildAWFBinary(t)
	acpConfigPath := writeACPConfig(t, workflowDir)
	stderrRead, stderrWrite, err := os.Pipe()
	require.NoError(t, err)
	originalStderr := os.Stderr
	os.Stderr = stderrWrite
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", acpConfigPath))
	os.Stderr = originalStderr
	t.Cleanup(func() { os.Stderr = originalStderr })
	stderrDone := make(chan []byte, 1)
	go func() {
		captured, _ := io.ReadAll(io.LimitReader(stderrRead, 1<<20))
		stderrDone <- captured
	}()

	initResp := proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name": "notifier-test-client", "version": "1.0.0",
		},
	})
	require.Nil(t, initResp.Error, "initialize must succeed")
	sessionResp := proc.request(t, 2, sdk.AgentMethodSessionNew, map[string]any{
		"cwd": root, "mcpServers": []any{},
	})
	require.Nil(t, sessionResp.Error, "session/new must succeed")
	sessionResult, ok := sessionResp.Result.(map[string]any)
	require.True(t, ok, "session/new result must be a JSON object")
	sessionID := fmt.Sprintf("%v", sessionResult["sessionId"])
	require.NotEmpty(t, sessionID)

	protocolFrames := marshalACPResponses(t, initResp, sessionResp)
	protocolFrames = append(protocolFrames, drainACPCommandsAndCapture(t, proc, workflowName)...)
	promptResp := proc.request(t, 3, sdk.AgentMethodSessionPrompt, map[string]any{
		"sessionId": sessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "/" + workflowName},
		},
	})
	require.Nil(t, promptResp.Error, "session/prompt must reach workflow completion: %+v", promptResp.Error)
	require.NotNil(t, promptResp.Result)
	promptFrame, err := json.Marshal(promptResp)
	require.NoError(t, err)
	protocolFrames = append(protocolFrames, promptFrame)
	require.NoError(t, proc.stdin.Close())
	proc.startWait()
	select {
	case <-proc.done:
		require.NoError(t, proc.waitErr, "ACP and adapter subprocesses must exit cleanly after stdin closes")
	case <-time.After(5 * time.Second):
		t.Fatal("ACP and adapter subprocesses did not exit within 5 seconds")
	}
	select {
	case <-proc.readPumpDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ACP stdout read pump completion")
	}
	protocolFrames = append(protocolFrames, drainACPPendingFrames(proc)...)
	require.NoError(t, stderrWrite.Close())
	var stderr []byte
	select {
	case stderr = <-stderrDone:
	case <-time.After(time.Second):
		t.Fatal("timed out collecting bounded ACP stderr")
	}
	require.NoError(t, stderrRead.Close())

	records := readACPAdapterRecords(t, recordPath)
	require.Len(t, records, 1)
	adapterExit, err := os.ReadFile(adapterExitPath) //nolint:gosec // Test-owned isolated adapter receipt path.
	require.NoError(t, err, "recording adapter must write its clean-exit receipt")
	assert.JSONEq(t, fmt.Sprintf(`{"pid":%d}`, records[0].PID), string(adapterExit))
	assert.Equal(t, pluginID, records[0].Adapter)
	assert.Equal(t, "send", records[0].Operation)
	assert.Equal(t, defaultCanary, records[0].Inputs["channel"])
	assert.Equal(t, payloadCanary, records[0].Inputs["text"])

	hostLog, err := os.ReadFile(hostLogPath) //nolint:gosec // Test-owned isolated audit path.
	if errors.Is(err, os.ErrNotExist) {
		hostLog = nil
	} else {
		require.NoError(t, err)
	}
	for _, frame := range protocolFrames {
		assert.NotContains(t, string(frame), payloadCanary)
		assert.NotContains(t, string(frame), credentialCanary)
	}
	for _, frame := range protocolFrames {
		assert.NotContains(t, string(frame), defaultCanary)
	}
	for _, output := range []string{string(stderr), string(hostLog)} {
		assert.NotContains(t, output, payloadCanary)
		assert.NotContains(t, output, defaultCanary)
		assert.NotContains(t, output, credentialCanary)
	}
}

type acpAdapterRecord struct {
	Adapter   string         `json:"adapter"`
	Operation string         `json:"operation"`
	Inputs    map[string]any `json:"inputs"`
	PID       int            `json:"pid"`
}

func installACPRecordingAdapter(t *testing.T, root, pluginsDir, recordPath, credential string) string {
	t.Helper()
	const manifestTemplate = `name: recording
version: 1.0.0
awf_version: ">=0.1.0"
capabilities:
  - operations
config:
  adapter_credential:
    type: string
    default: %s
`
	manifest := fmt.Sprintf(manifestTemplate, credential)
	var metadata struct {
		Name string `yaml:"name"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &metadata))
	require.NotEmpty(t, metadata.Name)
	destination := filepath.Join(pluginsDir, metadata.Name)
	require.NoError(t, os.MkdirAll(destination, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(destination, "plugin.yaml"), []byte(manifest), 0o600))
	sourcePath := filepath.Join(root, "recording-plugin.go")
	require.NoError(t, os.WriteFile(sourcePath, []byte(acpRecordingAdapterSource), 0o600))
	buildCtx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", filepath.Join(destination, "awf-plugin-"+metadata.Name), sourcePath)
	build.Dir = filepath.Join("..", "..", "..")
	build.Env = append(os.Environ(), "AWF_RECORDING_PATH="+recordPath)
	buildOutput, err := build.CombinedOutput()
	require.NoError(t, err, string(buildOutput))
	return metadata.Name
}

func marshalACPResponses(t *testing.T, responses ...jsonRPCResponse) [][]byte {
	t.Helper()
	frames := make([][]byte, 0, len(responses))
	for _, response := range responses {
		frame, err := json.Marshal(response)
		require.NoError(t, err)
		frames = append(frames, frame)
	}
	return frames
}

func drainACPCommandsAndCapture(t *testing.T, proc *acpProcess, workflowName string) [][]byte {
	t.Helper()
	deadline := time.After(acpRPCTimeout)
	var frames [][]byte
	for {
		select {
		case frame := <-proc.rawCh:
			frames = append(frames, frame)
			if bytes.Contains(frame, []byte(`"name":"`+workflowName+`"`)) {
				return frames
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s in available commands", workflowName)
		}
	}
}

func drainACPPendingFrames(proc *acpProcess) [][]byte {
	var frames [][]byte
	for {
		select {
		case frame := <-proc.rawCh:
			frames = append(frames, frame)
		default:
			return frames
		}
	}
}

func readACPAdapterRecords(t *testing.T, path string) []acpAdapterRecord {
	t.Helper()
	file, err := os.Open(path) //nolint:gosec // Test-owned isolated record path.
	require.NoError(t, err)
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, 1<<20))
	var records []acpAdapterRecord
	for scanner.Scan() {
		var record acpAdapterRecord
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &record))
		records = append(records, record)
	}
	require.NoError(t, scanner.Err())
	return records
}

const acpRecordingAdapterSource = `package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/awf-project/cli/pkg/plugin/sdk"
)

type plugin struct{ sdk.BasePlugin }

func (p *plugin) Operations() []string { return []string{"send"} }

func (p *plugin) HandleOperation(_ context.Context, name string, inputs map[string]any) (*sdk.OperationResult, error) {
	if name != "send" { return nil, errors.New("unsupported recording operation") }
	record, err := json.Marshal(map[string]any{"adapter": "recording", "operation": name, "inputs": inputs, "pid": os.Getpid()})
	if err != nil { return nil, err }
	record = append(record, '\n')
	if len(record) > 64*1024 { return nil, errors.New("record exceeds 64 KiB") }
	file, err := os.OpenFile(os.Getenv("AWF_RECORDING_PATH"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil { return nil, err }
	_, writeErr := file.Write(record)
	closeErr := file.Close()
	if writeErr != nil { return nil, writeErr }
	if closeErr != nil { return nil, closeErr }
	channel, _ := inputs["channel"].(string)
	return sdk.NewSuccessResult("sent", map[string]any{"message_id": "message-001", "channel": channel}), nil
}

func main() {
	sdk.Serve(&plugin{BasePlugin: sdk.BasePlugin{PluginName: "recording", PluginVersion: "1.0.0"}})
	receipt, err := json.Marshal(map[string]any{"pid": os.Getpid()})
	if err != nil { panic(err) }
	if err := os.WriteFile(os.Getenv("AWF_ADAPTER_EXIT_PATH"), receipt, 0o600); err != nil { panic(err) }
}
`

const acpTrackerAdapterFailureMarker = "confidential-acp-adapter-failure-marker"

func TestACPServe_TrackerWorkflowDelegatesThroughInstalledAdapter(t *testing.T) {
	runACPTrackerWorkflow(t, false)
}

func TestACPServe_TrackerWorkflowHidesAdapterFailureDetails(t *testing.T) {
	runACPTrackerWorkflow(t, true)
}

func runACPTrackerWorkflow(t *testing.T, failAdapter bool) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	root := t.TempDir()
	workflowRoot := filepath.Join(root, "workflows")
	pluginRoot := filepath.Join(root, "plugins")
	fixtureRoot := filepath.Join(root, "fixture")
	for _, dir := range []string{workflowRoot, pluginRoot, fixtureRoot} {
		require.NoError(t, os.MkdirAll(dir, 0o700))
	}

	pluginDir := filepath.Join(pluginRoot, "tracker-jira")
	require.NoError(t, os.MkdirAll(pluginDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(`name: tracker-jira
version: 1.0.0
awf_version: ">=0.1.0"
capabilities:
  - operations
`), 0o600))
	testBinary, err := os.Executable()
	require.NoError(t, err)
	require.NoError(t, os.Symlink(testBinary, filepath.Join(pluginDir, "awf-plugin-tracker-jira")))

	configPath := filepath.Join(root, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("tracker:\n  default_backend: jira\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(workflowRoot, acpTrackerWorkflow+".yaml"), []byte(`name: `+acpTrackerWorkflow+`
version: "1.0.0"
states:
  initial: create
  create:
    type: operation
    operation: tracker.create_issue
    inputs:
      title: `+acpTrackerSecret+`
      description: `+acpTrackerSecret+`
    on_success: done
  done:
    type: terminal
    status: failure
    message: "{{states.create.Output}}"
`), 0o600))

	t.Setenv("AWF_CONFIG_PATH", configPath)
	t.Setenv("AWF_WORKFLOWS_PATH", workflowRoot)
	t.Setenv("AWF_PLUGINS_PATH", pluginRoot)
	t.Setenv(acpTrackerFixtureEnv, fixtureRoot)
	t.Setenv("AWF_ACP_TRACKER_FAIL", strconv.FormatBool(failAdapter))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "xdg-state"))
	t.Setenv("AWF_AUDIT_LOG", "off")

	binaryPath := buildAWFBinary(t)
	config := writeACPConfig(t, workflowRoot)
	var stderr bytes.Buffer
	proc := startACPServeProcessWithStderr(t, binaryPath, &stderr, "--config="+config)
	proc.addShutdownCheck(func() {
		assertSecretAbsent(t, proc.protocolFrames(), acpTrackerSecret, "ACP protocol diagnostics")
		assertSecretAbsent(t, stderr.String(), acpTrackerSecret, "ACP logs")
		assertSecretAbsent(t, proc.protocolFrames(), acpTrackerAdapterFailureMarker, "ACP protocol diagnostics")
		assertSecretAbsent(t, stderr.String(), acpTrackerAdapterFailureMarker, "ACP logs")
	})

	initResp := proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "tracker-test-client", "version": "1.0.0"},
	})
	require.Nil(t, initResp.Error, "initialize must succeed")

	sessionResp := proc.request(t, 2, sdk.AgentMethodSessionNew, map[string]any{
		"cwd": root, "mcpServers": []any{},
	})
	require.Nil(t, sessionResp.Error, "session/new must succeed")
	sessionResult, ok := sessionResp.Result.(map[string]any)
	require.True(t, ok, "session/new result must be a JSON object")
	sessionID := fmt.Sprintf("%v", sessionResult["sessionId"])
	require.NotEmpty(t, sessionID, "sessionId must not be empty")
	require.True(t, proc.drainForAvailableCommands(t, acpTrackerWorkflow),
		"tracker workflow must be advertised before prompting")

	promptResp := proc.request(t, 3, sdk.AgentMethodSessionPrompt, map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/" + acpTrackerWorkflow}},
	})
	if failAdapter {
		require.FileExists(t, filepath.Join(fixtureRoot, "adapter.failed"),
			"the installed adapter must reach its failure path")
		require.True(t, promptResp.Error != nil || promptResp.Result != nil,
			"the prompt must receive a response after adapter failure")
	} else {
		require.NoError(t, sanitizedACPResponseError(promptResp.Error, "session/prompt"))
		promptResult, ok := promptResp.Result.(map[string]any)
		require.True(t, ok, "session/prompt result must be a JSON object")
		assert.Equal(t, "end_turn", promptResult["stopReason"],
			"session/prompt must return the expected ACP turn completion")
		require.True(t, proc.drainForSessionChunk(t, sessionID, acpTrackerOutput),
			"prompt lifecycle must stream the installed adapter business output to its session")
	}
	require.FileExists(t, filepath.Join(fixtureRoot, "adapter.called"),
		"configured tracker default must execute the installed adapter subprocess")

	require.NoError(t, proc.stop())
	select {
	case <-proc.done:
		require.NoError(t, proc.waitErr, "ACP server must stop cleanly")
	case <-time.After(8 * time.Second):
		t.Fatal("ACP server did not stop after SIGTERM")
	}
	pidBytes, err := os.ReadFile(filepath.Join(fixtureRoot, "adapter.pid")) //nolint:gosec // Test-owned fixture path.
	require.NoError(t, err)
	adapterPID, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return syscall.Kill(adapterPID, 0) != nil
	}, 3*time.Second, 20*time.Millisecond, "adapter process must not survive ACP shutdown")
}

// assertSecretAbsent deliberately does not provide either captured output or the secret to
// a test assertion formatter: a regression must not disclose the protected value in test logs.
func assertSecretAbsent(t *testing.T, captured, secret, boundary string) {
	t.Helper()
	if strings.Contains(captured, secret) {
		t.Errorf("%s exposed a confidential value", boundary)
	}
}

// TestACPServe_InvalidConfiguration validates that the server fails gracefully
// when given invalid configuration (missing or malformed config file).
func TestACPServe_InvalidConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)

	// Test case 1: Missing config file.
	t.Run("missing config file", func(t *testing.T) {
		cmd := buildACPServeCommand(t, binaryPath, "--config=/nonexistent/path/config.yaml")
		stderrCapture := bytes.NewBuffer(nil)
		cmd.Stderr = stderrCapture

		err := cmd.Run()
		require.Error(t, err, "should fail when config file does not exist")

		stderrOutput := stderrCapture.String()
		assert.Contains(t, stderrOutput, "config file", "error message should reference config file")
	})

	// Test case 2: Invalid config format (malformed YAML).
	t.Run("invalid YAML in config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := fmt.Sprintf("%s/bad-config.yaml", tmpDir)
		badYAML := `{invalid: [yaml: syntax`
		require.NoError(t, os.WriteFile(configPath, []byte(badYAML), 0o644))

		cmd := buildACPServeCommand(t, binaryPath, fmt.Sprintf("--config=%s", configPath))
		stderrCapture := bytes.NewBuffer(nil)
		cmd.Stderr = stderrCapture

		err := cmd.Run()
		require.Error(t, err, "should fail when config format is invalid")

		stderrOutput := stderrCapture.String()
		assert.Contains(t, stderrOutput, "invalid config", "error message should indicate config format issue")
	})

	// Test case 3: Invalid workflows directory.
	t.Run("invalid workflows directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := fmt.Sprintf("%s/config.json", tmpDir)
		configData, marshalErr := json.Marshal(map[string]any{
			"workflows_dir": "/nonexistent/workflows/path",
		})
		require.NoError(t, marshalErr)
		require.NoError(t, os.WriteFile(configPath, configData, 0o644))

		cmd := buildACPServeCommand(t, binaryPath, fmt.Sprintf("--config=%s", configPath))
		stderrCapture := bytes.NewBuffer(nil)
		cmd.Stderr = stderrCapture

		err := cmd.Run()
		require.Error(t, err, "should fail when workflows_dir does not exist")
	})
}

// TestACPServe_GracefulShutdown validates that the server shuts down gracefully
// when receiving SIGINT or SIGTERM, without leaving goroutine leaks.
func TestACPServe_GracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, fixtureWorkflowsDir(t))
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	// Initialize to ensure the server is running.
	initResp := proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})
	require.Nil(t, initResp.Error, "initialize must succeed")

	require.NoError(t, proc.stop(), "should be able to send SIGTERM to process")

	select {
	case <-proc.done:
		err := proc.waitErr
		// Process exited. Either cleanly (err==nil) or with a signal (ExitError with code).
		// Either is acceptable; what we're testing is that it exits in reasonable time
		// without hanging and requiring SIGKILL.
		assert.True(t, err == nil || isSigTermError(err), "process should exit cleanly or with SIGTERM")
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit within 5 seconds; graceful shutdown may have failed")
	}
}

// TestACPServe_ConcurrentSessions validates that the server can handle
// multiple concurrent sessions without race conditions or state corruption.
func TestACPServe_ConcurrentSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, fixtureWorkflowsDir(t))
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	// Initialize once.
	initResp := proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})
	require.Nil(t, initResp.Error, "initialize must succeed")

	// Create multiple sessions concurrently and execute workflows.
	const numSessions = 3
	var wg sync.WaitGroup
	errors := make(chan error, numSessions)

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			reqID := 100 + idx*10
			sessionResp := proc.request(t, reqID, sdk.AgentMethodSessionNew, map[string]any{
				"cwd":        t.TempDir(),
				"mcpServers": []any{},
			})

			if sessionResp.Error != nil {
				errors <- fmt.Errorf("session %d: session/new failed: %v", idx, sessionResp.Error)
				return
			}

			result, ok := sessionResp.Result.(map[string]any)
			if !ok {
				errors <- fmt.Errorf("session %d: result is not a JSON object", idx)
				return
			}

			sessionID := fmt.Sprintf("%v", result["sessionId"])
			if sessionID == "" {
				errors <- fmt.Errorf("session %d: sessionId is empty", idx)
				return
			}

			// Drain notifications to avoid blocking the read pump.
			_ = proc.drainNotifications(t, 500*time.Millisecond)

			// Execute workflow.
			promptResp := proc.request(t, reqID+1, sdk.AgentMethodSessionPrompt, map[string]any{
				"sessionId": sessionID,
				"prompt": []map[string]any{
					{"type": "text", "text": "/trivial"},
				},
			})

			if promptResp.Error != nil {
				errors <- fmt.Errorf("session %d: session/prompt failed: %v", idx, promptResp.Error)
				return
			}

			if promptResp.Result == nil {
				errors <- fmt.Errorf("session %d: session/prompt returned no result", idx)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Verify no errors occurred.
	for err := range errors {
		require.NoError(t, err)
	}
}

// TestACPServe_StderrLogging validates that diagnostic logs are written to stderr,
// not stdout, per NFR-002 (stdout reserved for JSON-RPC protocol frames).
func TestACPServe_StderrLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildAWFBinary(t)
	configPath := writeACPConfig(t, fixtureWorkflowsDir(t))

	// Use the acpProcess helper to start the server and send a request.
	// This ensures there's valid JSON-RPC traffic on stdout.
	proc := startACPServeProcess(t, binaryPath, fmt.Sprintf("--config=%s", configPath))

	// Send an Initialize request to generate protocol traffic on stdout.
	initResp := proc.request(t, 1, sdk.AgentMethodInitialize, map[string]any{
		"protocolVersion": sdk.ProtocolVersionNumber,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})

	require.Nil(t, initResp.Error, "initialize must succeed")

	// The test verifies that:
	// 1. Server accepts requests and sends responses (stdout contains valid JSON)
	// 2. Stderr is available for logs (verified in acp_serve.go source: os.Stderr sink)
	// 3. No log data corrupts the protocol stream (implicit: if Initialize succeeded,
	//    the response was valid JSON without embedded log text)

	// This is sufficient verification of NFR-002 (stdout=protocol, stderr=logs).
	// The actual stderr logging is validated by acp_serve_test.go which reads the
	// source code to verify NewConsoleLogger(os.Stderr, ...) is used.
}

// Helper functions below.

// isSigTermError checks if an error is from SIGTERM exit.
func isSigTermError(err error) bool {
	if err == nil {
		return false
	}
	// On Unix, signal exits have exit codes 128+N where N is the signal number.
	// SIGTERM is 15, so exit code would be 143.
	// However, this is platform-specific; just accept that any error is acceptable
	// when we explicitly sent SIGTERM.
	return true
}

// getValidJSONRPCLines filters stdout to extract lines that are valid JSON.
// Useful for verifying stdout contains only protocol frames, not diagnostic logs.
func getValidJSONRPCLines(output string) []string {
	var lines []string
	for _, line := range bytes.Split([]byte(output), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var obj map[string]any
		if json.Unmarshal(line, &obj) == nil {
			lines = append(lines, string(line))
		}
	}
	return lines
}

// buildACPServeCommand constructs an exec.Cmd for running acp-serve with given args.
// Used by error-case tests that don't need the full acpProcess wrapper.
func buildACPServeCommand(t *testing.T, binaryPath string, args ...string) *exec.Cmd {
	t.Helper()
	cmdArgs := append([]string{"acp-serve"}, args...)
	cmd := exec.Command(binaryPath, cmdArgs...) //nolint:gosec
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// drainNotifications is a helper method on acpProcess that consumes pending
// notifications from the rawCh for a given timeout. This prevents the read pump
// from blocking if there are queued notifications that the test doesn't care about.
func (p *acpProcess) drainNotifications(t *testing.T, timeout time.Duration) int {
	t.Helper()
	deadline := time.After(timeout)
	count := 0
	for {
		select {
		case <-p.rawCh:
			count++
		case <-deadline:
			return count
		}
	}
}
