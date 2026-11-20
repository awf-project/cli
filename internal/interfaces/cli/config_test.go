package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/awf-project/cli/pkg/plugin/sdk"
	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errAdapterFailure = errors.New("adapter failure")

type failingNotifierAdapter struct {
	failure    error
	mu         sync.Mutex
	calls      []map[string]any
	lastResult *pluginmodel.OperationResult
	lastErr    error
}

func (*failingNotifierAdapter) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	if name != "adapter.send" {
		return nil, false
	}
	return &pluginmodel.OperationSchema{Name: name, PluginName: "adapter"}, true
}

func (a *failingNotifierAdapter) ListOperations() []*pluginmodel.OperationSchema {
	operation, _ := a.GetOperation("adapter.send")
	return []*pluginmodel.OperationSchema{operation}
}

func (a *failingNotifierAdapter) Execute(
	_ context.Context,
	_ string,
	inputs map[string]any,
) (*pluginmodel.OperationResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, inputs)
	a.lastResult = nil
	a.lastErr = a.failure
	if a.lastErr == nil {
		a.lastErr = errAdapterFailure
	}
	return a.lastResult, a.lastErr
}

func (a *failingNotifierAdapter) recordedOutcome() (*pluginmodel.OperationResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastResult, a.lastErr
}

func (a *failingNotifierAdapter) recordedCalls() []map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]map[string]any(nil), a.calls...)
}

func TestServeWiringPropagatesNotifierDefaultsAndFailures(t *testing.T) {
	const (
		defaultChannel = "serve-default-channel-output-canary"
		payload        = "serve-payload-output-canary"
	)
	root, workflowPath, hostLog := writeNotifierServerFixture(t, defaultChannel, payload)
	adapter := &failingNotifierAdapter{}
	service := application.NewPluginService(nil, nil, nil)
	originalInit := serveInitPluginSystem
	serveInitPluginSystem = func(context.Context, []string, string, string, ports.Logger) (*pluginmgr.SystemResult, error) {
		return &pluginmgr.SystemResult{Service: service, Manager: adapter, Cleanup: func() {}}, nil
	}
	t.Cleanup(func() { serveInitPluginSystem = originalInit })

	port := reserveTCPPort(t)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := NewServeCommand()
	cmd.SetContext(ctx)
	var commandOutput bytes.Buffer
	cmd.SetOut(&commandOutput)
	cmd.SetErr(&commandOutput)
	cmd.SetArgs([]string{"--host", "127.0.0.1", "--port", fmt.Sprintf("%d", port)})
	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTPServer(t, baseURL+"/docs")
	requestBody := strings.NewReader(`{"inputs":{}}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		baseURL+"/api/workflows/local/"+strings.TrimSuffix(filepath.Base(workflowPath), filepath.Ext(workflowPath))+"/run", requestBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	var accepted struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	require.NoError(t, json.Unmarshal(responseBody, &accepted))
	require.NotEmpty(t, accepted.Body.ExecutionID)

	require.Eventually(t, func() bool { return len(adapter.recordedCalls()) == 1 }, 3*time.Second, 10*time.Millisecond)
	calls := adapter.recordedCalls()
	assert.Equal(t, map[string]any{"channel": defaultChannel, "text": payload}, calls[0])
	outcomeResult, executionErr := adapter.recordedOutcome()
	assert.Nil(t, outcomeResult)
	require.Error(t, executionErr)
	assert.ErrorIs(t, executionErr, errAdapterFailure)

	var statusBody []byte
	require.Eventually(t, func() bool {
		statusResp, getErr := http.Get(baseURL + "/api/executions/" + accepted.Body.ExecutionID) //nolint:noctx // bounded test polling against a local server.
		if getErr != nil {
			return false
		}
		defer statusResp.Body.Close()
		statusBody, getErr = io.ReadAll(statusResp.Body)
		return getErr == nil && strings.Contains(strings.ToLower(string(statusBody)), "failed")
	}, 3*time.Second, 10*time.Millisecond)

	combined := commandOutput.String() + string(responseBody) + string(statusBody)
	assert.NotContains(t, strings.ToLower(combined), "workflow completed")
	assertCanariesAbsent(t, combined, defaultChannel, payload)
	assertHostLogCanariesAbsent(t, hostLog, defaultChannel, payload)

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("NewServeCommand did not preserve bounded HTTP shutdown")
	}
	_ = root
}

func TestACPServeWiringPropagatesNotifierDefaultsAndFailures(t *testing.T) {
	const (
		defaultChannel = "acp-default-channel-output-canary"
		payload        = "acp-payload-output-canary"
	)
	_, workflowPath, hostLog := writeNotifierServerFixture(t, defaultChannel, payload)
	const errorCanary = "acp-adapter-error-output-canary"
	adapter := &failingNotifierAdapter{
		failure: errors.Join(errAdapterFailure, errors.New(errorCanary)),
	}
	service := application.NewPluginService(nil, nil, nil)
	originalInit := acpServeInitPluginSystem
	acpServeInitPluginSystem = func(context.Context, *Config, ports.Logger) (*PluginSystemResult, error) {
		return &PluginSystemResult{Service: service, Manager: adapter, Cleanup: func() {}}, nil
	}
	t.Cleanup(func() { acpServeInitPluginSystem = originalInit })

	configPath := filepath.Join(filepath.Dir(workflowPath), "acp-config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("workflows_dir: "+filepath.Dir(workflowPath)+"\n"), 0o600))
	stdinR, stdinW, err := os.Pipe()
	require.NoError(t, err)
	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)
	oldStdin, oldStdout, oldStderr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = stdinR, stdoutW, stderrW
	t.Cleanup(func() { os.Stdin, os.Stdout, os.Stderr = oldStdin, oldStdout, oldStderr })

	frames := make(chan map[string]any, 32)
	go scanJSONRPCFrames(stdoutR, frames)
	var stderr bytes.Buffer
	stderrDone := make(chan struct{})
	go func() { _, _ = io.Copy(&stderr, stderrR); close(stderrDone) }()

	cmd := newACPServeCommand(Deps{})
	cmd.SetArgs([]string{"--config", configPath})
	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()

	writeJSONRPCRequest(t, stdinW, 1, acpsdk.AgentMethodInitialize, map[string]any{"protocolVersion": acpsdk.ProtocolVersionNumber})
	assertJSONRPCResponse(t, frames, 1, false, defaultChannel, payload)
	writeJSONRPCRequest(t, stdinW, 2, acpsdk.AgentMethodSessionNew, map[string]any{"cwd": filepath.Dir(workflowPath), "mcpServers": []any{}})
	sessionResponse := assertJSONRPCResponse(t, frames, 2, false, defaultChannel, payload)
	result, ok := sessionResponse["result"].(map[string]any)
	require.True(t, ok)
	sessionID, ok := result["sessionId"].(string)
	require.True(t, ok)
	writeJSONRPCRequest(t, stdinW, 3, acpsdk.AgentMethodSessionPrompt, map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/notifier-wiring"}},
	})
	assertJSONRPCResponse(t, frames, 3, true, defaultChannel, payload)
	require.Eventually(t, func() bool { return len(adapter.recordedCalls()) == 1 }, 3*time.Second, 10*time.Millisecond)
	calls := adapter.recordedCalls()
	assert.Equal(t, map[string]any{"channel": defaultChannel, "text": payload}, calls[0])
	outcomeResult, executionErr := adapter.recordedOutcome()
	assert.Nil(t, outcomeResult)
	require.Error(t, executionErr)
	assert.ErrorIs(t, executionErr, errAdapterFailure)

	require.NoError(t, stdinW.Close())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("runACPServe did not stop after protocol EOF")
	}
	require.NoError(t, stdoutW.Close())
	require.NoError(t, stderrW.Close())
	<-stderrDone
	var protocolOutput bytes.Buffer
	for frame := range frames {
		encoded, marshalErr := json.Marshal(frame)
		require.NoError(t, marshalErr)
		protocolOutput.Write(encoded)
	}
	assert.NotContains(t, strings.ToLower(protocolOutput.String()), "workflow completed")
	assertCanariesAbsent(t, protocolOutput.String(), defaultChannel, payload, errorCanary)
	assertCanariesAbsent(t, stderr.String(), defaultChannel, payload, errorCanary)
	assert.Contains(t, stderr.String(), "operation failed")
	assertHostLogCanariesAbsent(t, hostLog, defaultChannel, payload, errorCanary)
}

func writeNotifierServerFixture(t *testing.T, defaultChannel, payload string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	workflowPath := filepath.Join(root, "notifier-wiring.yaml")
	hostLog := filepath.Join(root, "host.log")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".awf"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(
		"notifier:\n  backend: adapter\n  default_channel: "+defaultChannel+"\n"), 0o600))
	require.NoError(t, os.WriteFile(workflowPath, []byte(
		"name: notifier-wiring\nversion: \"1.0\"\nstates:\n  initial: send\n  send:\n    type: operation\n    operation: notifier.send\n    inputs:\n      text: "+payload+"\n    on_success: done\n  done:\n    type: terminal\n    status: success\n"), 0o600))
	t.Chdir(root)
	t.Setenv("AWF_WORKFLOWS_PATH", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "xdg-state"))
	t.Setenv("AWF_AUDIT_LOG", hostLog)
	return root, workflowPath, hostLog
}

func reserveTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func waitForHTTPServer(t *testing.T, url string) {
	t.Helper()
	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
		if err != nil {
			return false
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return true
	}, 3*time.Second, 10*time.Millisecond)
}

func writeJSONRPCRequest(t *testing.T, w io.Writer, id int, method string, params map[string]any) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	require.NoError(t, err)
	_, err = fmt.Fprintf(w, "%s\n", payload)
	require.NoError(t, err)
}

func scanJSONRPCFrames(r io.Reader, frames chan<- map[string]any) {
	defer close(frames)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var frame map[string]any
		if json.Unmarshal(scanner.Bytes(), &frame) == nil {
			frames <- frame
		}
	}
}

func assertJSONRPCResponse(
	t *testing.T,
	frames <-chan map[string]any,
	id int,
	wantError bool,
	canaries ...string,
) map[string]any {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case frame, ok := <-frames:
			require.True(t, ok, "JSON-RPC stream closed before response %d", id)
			encoded, err := json.Marshal(frame)
			require.NoError(t, err)
			assertCanariesAbsent(t, string(encoded), canaries...)
			assert.NotContains(t, strings.ToLower(string(encoded)), "workflow completed")
			frameID, hasID := frame["id"].(float64)
			if !hasID || int(frameID) != id {
				continue
			}
			if wantError {
				assert.Contains(t, frame, "error")
				assert.NotContains(t, frame, "result")
			} else {
				assert.NotContains(t, frame, "error")
			}
			return frame
		case <-timer.C:
			t.Fatalf("timed out waiting for JSON-RPC response %d", id)
			return nil
		}
	}
}

func assertCanariesAbsent(t *testing.T, output string, canaries ...string) {
	t.Helper()
	for _, canary := range canaries {
		assert.NotContains(t, output, canary)
	}
}

func assertHostLogCanariesAbsent(t *testing.T, path string, canaries ...string) {
	t.Helper()
	contents, err := os.ReadFile(path) //nolint:gosec // test-owned audit-log path.
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	require.NoError(t, err)
	assertCanariesAbsent(t, string(contents), canaries...)
}

const (
	validationNotifierPluginName        = "awf-plugin-validation-config-test"
	validationNotifierDispatchRecordEnv = "AWF_VALIDATION_NOTIFIER_DISPATCH_RECORD"
)

// init turns this test binary into the discovered notifier adapter when the
// production plugin manager starts the copied executable. It leaves the normal
// test process untouched and uses the same SDK boundary as installed plugins.
func init() {
	if os.Getenv("AWF_PLUGIN") != "awf-plugin-v1" {
		return
	}

	sdk.Serve(&validationNotifierTestPlugin{BasePlugin: sdk.BasePlugin{
		PluginName:    validationNotifierPluginName,
		PluginVersion: "1.0.0",
	}})
	os.Exit(0)
}

type validationNotifierTestPlugin struct {
	sdk.BasePlugin
}

func (*validationNotifierTestPlugin) Operations() []string {
	return []string{"send"}
}

func (*validationNotifierTestPlugin) HandleOperation(
	context.Context,
	string,
	map[string]any,
) (*sdk.OperationResult, error) {
	if recordPath := os.Getenv(validationNotifierDispatchRecordEnv); recordPath != "" {
		//nolint:gosec // test subprocess writes only to the test-owned path supplied by its parent.
		if err := os.WriteFile(recordPath, []byte("dispatched\n"), 0o600); err != nil {
			return nil, err
		}
	}
	return sdk.NewSuccessResult("send", nil), nil
}

func TestValidateCommandAcceptsNotifierOperationWithDiscoveredAdapter(t *testing.T) {
	const (
		configuredChannel = "configured-channel-output-canary"
		explicitChannel   = "explicit-channel-output-canary"
		textCanary        = "workflow-text-output-canary"
		richCanary        = "workflow-rich-output-canary"
	)

	tests := []struct {
		name           string
		defaultChannel string
		channelYAML    string
		wantError      bool
	}{
		{name: "omitted_without_default", wantError: true},
		{name: "omitted_with_default", defaultChannel: configuredChannel},
		{name: "explicit_nonblank_override", defaultChannel: configuredChannel, channelYAML: "      channel: " + explicitChannel + "\n"},
		{name: "explicit_blank", defaultChannel: configuredChannel, channelYAML: "      channel: \"\"\n", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			workflowsDir := filepath.Join(root, "workflows")
			pluginsDir := filepath.Join(root, "plugins")
			pluginDir := filepath.Join(pluginsDir, validationNotifierPluginName)
			storageDir := filepath.Join(root, "storage")
			require.NoError(t, os.MkdirAll(pluginDir, 0o700))
			require.NoError(t, os.MkdirAll(workflowsDir, 0o700))
			require.NoError(t, os.MkdirAll(filepath.Join(root, ".awf"), 0o700))
			installValidationNotifierPlugin(t, filepath.Join(pluginDir, validationNotifierPluginName))
			require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte("name: "+validationNotifierPluginName+"\nversion: 1.0.0\nawf_version: \">=0.1.0\"\ncapabilities:\n  - operations\nconfig:\n  adapter_credential:\n    type: string\n    default: adapter-credential-output-canary\n"), 0o600))
			t.Setenv("AWF_PLUGINS_PATH", pluginsDir)
			t.Setenv("AWF_WORKFLOWS_PATH", workflowsDir)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(root, "xdg-state"))
			dispatchRecord := filepath.Join(root, "adapter-dispatch.log")
			t.Setenv(validationNotifierDispatchRecordEnv, dispatchRecord)
			t.Chdir(root)

			workflowPath := filepath.Join(workflowsDir, "notifier-validation.yaml")
			contents := "name: notifier-validation\nversion: \"1.0\"\nstates:\n  initial: send\n  send:\n    type: operation\n    operation: notifier.send\n    inputs:\n      text: " + textCanary + "\n      rich:\n        marker: " + richCanary + "\n" + tt.channelYAML + "    on_success: done\n  done:\n    type: terminal\n    status: success\n"
			require.NoError(t, os.WriteFile(workflowPath, []byte(contents), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte("notifier:\n  backend: "+validationNotifierPluginName+"\n  default_channel: "+tt.defaultChannel+"\n"), 0o600))

			hostLog, err := os.CreateTemp(root, "validation-host-*.log")
			require.NoError(t, err)
			previousStderr := os.Stderr
			os.Stderr = hostLog
			t.Cleanup(func() {
				os.Stderr = previousStderr
				require.NoError(t, hostLog.Close())
			})

			cfg := DefaultConfig()
			cfg.StoragePath = storageDir
			facade, cleanup, err := buildValidationFacade(cfg)
			require.NoError(t, err)
			cleanupFacade := sync.OnceFunc(cleanup)
			t.Cleanup(cleanupFacade)
			cmd := NewRootCommand(func(cfg *Config) { cfg.Facade = facade })
			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			workflowIdentifier := strings.TrimSuffix(filepath.Base(workflowPath), filepath.Ext(workflowPath))
			cmd.SetArgs([]string{"--no-color", "validate", workflowIdentifier})

			err = cmd.Execute()

			_, dispatchErr := os.Stat(dispatchRecord)
			require.ErrorIs(t, dispatchErr, os.ErrNotExist, "static validation must not execute adapter operations")
			require.NoError(t, hostLog.Sync())
			hostLogContents, readErr := os.ReadFile(hostLog.Name()) //nolint:gosec // test-owned host-log destination
			require.NoError(t, readErr)
			for _, canary := range []string{configuredChannel, explicitChannel, textCanary, richCanary, "adapter-credential-output-canary"} {
				assert.NotContains(t, stdout.String(), canary)
				assert.NotContains(t, stderr.String(), canary)
				assert.NotContains(t, string(hostLogContents), canary)
			}
			validLine := "Workflow '" + workflowIdentifier + "' is valid\n"
			if !tt.wantError {
				require.NoError(t, err)
				assert.Equal(t, validLine, stdout.String())
				assert.Empty(t, stderr.String())
				return
			}

			require.Error(t, err)
			var structured *domainerrors.StructuredError
			require.ErrorAs(t, err, &structured)
			assert.Equal(t, domainerrors.ErrorCodeUserInputValidationFailed, structured.Code)
			assert.Equal(t, 1, structured.ExitCode())
			assert.Contains(t, structured.Error(), "channel")
			assert.NotContains(t, stdout.String(), validLine)
		})
	}
}

func installValidationNotifierPlugin(t *testing.T, destination string) {
	t.Helper()
	source, err := os.Executable()
	require.NoError(t, err)
	in, err := os.Open(source) //nolint:gosec // current test executable is controlled by go test.
	require.NoError(t, err)
	defer in.Close()
	out, err := os.OpenFile(destination+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700) //nolint:gosec // test-owned plugin destination
	require.NoError(t, err)
	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())
	require.NoError(t, os.Rename(destination+".tmp", destination))
}

type routingTestFacade struct{}

type recordedHistoryStore struct {
	events *[]string
}

func (*recordedHistoryStore) Record(context.Context, *workflow.ExecutionRecord) error { return nil }

func (*recordedHistoryStore) List(context.Context, *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	return nil, nil
}

func (*recordedHistoryStore) GetStats(context.Context, *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	return nil, nil
}

func (*recordedHistoryStore) Cleanup(context.Context, time.Duration) (int, error) { return 0, nil }

func (s *recordedHistoryStore) Close() error {
	*s.events = append(*s.events, "history-close")
	return nil
}

func (routingTestFacade) List(context.Context) ([]ports.WorkflowSummary, error) { return nil, nil }

func (routingTestFacade) Validate(context.Context, ports.RunRequest) (ports.ValidationReport, error) {
	return ports.ValidationReport{}, nil
}

func (routingTestFacade) Status(context.Context, string) (ports.RunStatus, error) {
	return ports.RunStatus{RunID: "run-1"}, nil
}

func (routingTestFacade) History(context.Context, ports.HistoryFilter) ([]ports.RunRecord, error) {
	return nil, nil
}

func (routingTestFacade) Run(context.Context, ports.RunRequest) (ports.RunSession, error) {
	return nil, nil
}

func (routingTestFacade) Resume(context.Context, ports.ResumeRequest) (ports.RunSession, error) {
	return nil, nil
}

func TestValidateCommand_IsOnlyPluginAwareFacadeRoute(t *testing.T) {
	cmd := NewRootCommand()
	var marked []string
	var visit func(*cobra.Command)
	visit = func(current *cobra.Command) {
		if current.Annotations[annotationPluginAwareValidation] == "true" {
			marked = append(marked, current.CommandPath())
		}
		for _, child := range current.Commands() {
			visit(child)
		}
	}
	visit(cmd)

	assert.Equal(t, []string{"awf validate"}, marked)
}

func TestAutoFacade_ValidationSetupErrorPreventsRunE(t *testing.T) {
	t.Run("setup error", func(t *testing.T) {
		setupErr := errors.New("validation setup failed")
		cleanupCalls := 0
		runCalls := 0
		cfg := DefaultConfig()
		cfg.StoragePath = t.TempDir()
		cmd, cleanup := newRootCommandWithBuilders(
			cfg,
			true,
			func(*Config) (ports.WorkflowFacade, func()) { return routingTestFacade{}, func() {} },
			func(*Config) (ports.WorkflowFacade, func(), error) {
				return nil, func() { cleanupCalls++ }, setupErr
			},
		)
		validateCmd, _, findErr := cmd.Find([]string{"validate"})
		require.NoError(t, findErr)
		validateCmd.RunE = func(*cobra.Command, []string) error {
			runCalls++
			return errors.New("validate RunE invoked")
		}
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"validate", "workflow"})

		err := cmd.Execute()

		require.ErrorIs(t, err, setupErr)
		assert.ErrorContains(t, err, "initialize validation facade")
		assert.Zero(t, runCalls)
		assert.Equal(t, 1, cleanupCalls)
		cleanup()
		cleanup()
		assert.Equal(t, 1, cleanupCalls)
	})

	t.Run("successful setup cleanup", func(t *testing.T) {
		cleanupCalls := 0
		runCalls := 0
		home := t.TempDir()
		t.Setenv("HOME", home)
		require.NoError(t, os.Mkdir(filepath.Join(home, ".awf"), 0o700))
		migrationNoticeMu.Lock()
		migrationNoticeShown = false
		migrationNoticeMu.Unlock()
		t.Cleanup(func() {
			migrationNoticeMu.Lock()
			migrationNoticeShown = false
			migrationNoticeMu.Unlock()
		})
		cfg := DefaultConfig()
		cfg.StoragePath = t.TempDir()
		cmd, cleanup := newRootCommandWithBuilders(
			cfg,
			true,
			func(*Config) (ports.WorkflowFacade, func()) { return routingTestFacade{}, func() {} },
			func(*Config) (ports.WorkflowFacade, func(), error) {
				return routingTestFacade{}, func() { cleanupCalls++ }, nil
			},
		)
		validateCmd, _, findErr := cmd.Find([]string{"validate"})
		require.NoError(t, findErr)
		validateCmd.RunE = func(*cobra.Command, []string) error {
			runCalls++
			return nil
		}
		output := new(bytes.Buffer)
		cmd.SetOut(output)
		cmd.SetErr(output)
		cmd.SetArgs([]string{"--format", "json", "validate", "workflow"})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, 1, runCalls)
		assert.Equal(t, ui.FormatJSON, cfg.OutputFormat)
		assert.Equal(t, 1, strings.Count(output.String(), "NOTICE: Legacy ~/.awf directory detected"))
		assert.Zero(t, cleanupCalls)
		cleanup()
		cleanup()
		assert.Equal(t, 1, cleanupCalls)
	})
}

func TestValidationFacade_ReleasesPluginResources(t *testing.T) {
	t.Run("plugin initialization error", func(t *testing.T) {
		events := []string{}
		initErr := errors.New("plugin initialization failed")
		facade, cleanup, err := buildValidationFacadeWithDependencies(DefaultConfig(), validationFacadeDependencies{
			initPlugins: func(_ context.Context, _ *Config, logger ports.Logger) (*PluginSystemResult, error) {
				require.NotPanics(t, func() { logger.Warn("synthetic plugin initialization warning") })
				events = append(events, "plugin-init", "plugin-partial-release")
				return nil, initErr
			},
			openHistory: func(string) (ports.HistoryStore, error) {
				events = append(events, "history-open")
				return nil, nil
			},
		})

		require.ErrorIs(t, err, initErr)
		assert.Nil(t, facade)
		cleanup()
		cleanup()
		assert.Equal(t, []string{"plugin-init", "plugin-partial-release"}, events)
	})

	t.Run("history acquisition error", func(t *testing.T) {
		events := []string{}
		historyErr := errors.New("history initialization failed")
		facade, cleanup, err := buildValidationFacadeWithDependencies(DefaultConfig(), validationFacadeDependencies{
			initPlugins: func(context.Context, *Config, ports.Logger) (*PluginSystemResult, error) {
				events = append(events, "plugin-init")
				return &PluginSystemResult{
					Cleanup: func() { events = append(events, "plugin-cleanup") },
				}, nil
			},
			openHistory: func(string) (ports.HistoryStore, error) {
				events = append(events, "history-open")
				return nil, historyErr
			},
		})

		require.ErrorIs(t, err, historyErr)
		assert.Nil(t, facade)
		cleanup()
		cleanup()
		assert.Equal(t, []string{"plugin-init", "history-open", "plugin-cleanup"}, events)
	})

	t.Run("successful cleanup is idempotent", func(t *testing.T) {
		events := []string{}
		facade, cleanup, err := buildValidationFacadeWithDependencies(DefaultConfig(), validationFacadeDependencies{
			initPlugins: func(context.Context, *Config, ports.Logger) (*PluginSystemResult, error) {
				events = append(events, "plugin-init")
				return &PluginSystemResult{
					Service: application.NewPluginService(nil, nil, ports.NopLogger{}),
					Cleanup: func() { events = append(events, "plugin-cleanup") },
				}, nil
			},
			openHistory: func(string) (ports.HistoryStore, error) {
				events = append(events, "history-open")
				return &recordedHistoryStore{events: &events}, nil
			},
		})

		require.NoError(t, err)
		require.NotNil(t, facade)
		cleanup()
		cleanup()
		assert.Equal(t, []string{"plugin-init", "history-open", "plugin-cleanup", "history-close"}, events)
	})
}

func TestAutoFacade_ReadCommandsDoNotInitializePlugins(t *testing.T) {
	pluginsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(pluginsDir, "invalid-plugin.yaml"), []byte("not: [valid"), 0o600))
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	tests := []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"list"}},
		{name: "history", args: []string{"history"}},
		{name: "status", args: []string{"status", "run-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationBuilds := 0
			ordinaryBuilds := 0
			cfg := DefaultConfig()
			cfg.StoragePath = t.TempDir()
			cmd, cleanup := newRootCommandWithBuilders(
				cfg,
				true,
				func(*Config) (ports.WorkflowFacade, func()) {
					ordinaryBuilds++
					return routingTestFacade{}, func() {}
				},
				func(*Config) (ports.WorkflowFacade, func(), error) {
					validationBuilds++
					return nil, func() {}, errors.New("plugin setup must not run")
				},
			)
			defer cleanup()
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			require.NoError(t, err)
			assert.Equal(t, 1, ordinaryBuilds)
			assert.Zero(t, validationBuilds)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Verbose {
		t.Error("expected Verbose to be false by default")
	}
	if cfg.Quiet {
		t.Error("expected Quiet to be false by default")
	}
	if cfg.NoColor {
		t.Error("expected NoColor to be false by default")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel 'info', got '%s'", cfg.LogLevel)
	}
	if !strings.Contains(cfg.StoragePath, "awf") {
		t.Errorf("expected StoragePath to contain 'awf', got '%s'", cfg.StoragePath)
	}
	if cfg.OutputMode != OutputSilent {
		t.Errorf("expected OutputMode to be silent by default, got %v", cfg.OutputMode)
	}
}

func TestOutputMode_String(t *testing.T) {
	tests := []struct {
		mode OutputMode
		want string
	}{
		{OutputSilent, "silent"},
		{OutputStreaming, "streaming"},
		{OutputBuffered, "buffered"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.mode.String())
	}
}

func TestParseOutputMode(t *testing.T) {
	tests := []struct {
		input   string
		want    OutputMode
		wantErr bool
	}{
		{"silent", OutputSilent, false},
		{"streaming", OutputStreaming, false},
		{"buffered", OutputBuffered, false},
		{"invalid", OutputSilent, true},
		{"", OutputSilent, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseOutputMode(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestOutputFormat_String(t *testing.T) {
	tests := []struct {
		format ui.OutputFormat
		want   string
	}{
		{ui.FormatText, "text"},
		{ui.FormatJSON, "json"},
		{ui.FormatTable, "table"},
		{ui.FormatQuiet, "quiet"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.format.String())
	}
}

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    ui.OutputFormat
		wantErr bool
	}{
		{"text", ui.FormatText, false},
		{"json", ui.FormatJSON, false},
		{"table", ui.FormatTable, false},
		{"quiet", ui.FormatQuiet, false},
		{"", ui.FormatText, false}, // default to text
		{"yaml", ui.FormatText, true},
		{"XML", ui.FormatText, true},
		{"invalid", ui.FormatText, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ui.ParseOutputFormat(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid output format")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestBuildPromptPaths_ReturnsCorrectNumberOfPaths(t *testing.T) {
	// BuildPromptPaths should return exactly 2 paths: local and global
	// (no AWF_PROMPTS_PATH env var support per ADR-002)
	paths := BuildPromptPaths()

	require.Len(t, paths, 2, "BuildPromptPaths should return exactly 2 paths (local + global)")
}

func TestBuildPromptPaths_LocalPathFirst(t *testing.T) {
	// FR-001: Local prompts have highest priority
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	assert.Equal(t, repository.SourceLocal, paths[0].Source, "First path should be local source")
	assert.Equal(t, xdg.LocalPromptsDir(), paths[0].Path, "First path should be local prompts directory")
}

func TestBuildPromptPaths_GlobalPathSecond(t *testing.T) {
	// FR-001: Global prompts are searched after local
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	assert.Equal(t, repository.SourceGlobal, paths[1].Source, "Second path should be global source")
	assert.Equal(t, xdg.AWFPromptsDir(), paths[1].Path, "Second path should be XDG prompts directory")
}

func TestBuildPromptPaths_PriorityOrder(t *testing.T) {
	// Comprehensive test: verify priority order matches spec FR-001
	// Order: 1. .awf/prompts/ (local), 2. $XDG_CONFIG_HOME/awf/prompts/ (global)
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)

	// Verify complete ordering
	expectedOrder := []struct {
		source repository.Source
		path   string
	}{
		{repository.SourceLocal, xdg.LocalPromptsDir()},
		{repository.SourceGlobal, xdg.AWFPromptsDir()},
	}

	for i, expected := range expectedOrder {
		assert.Equal(t, expected.source, paths[i].Source,
			"Path %d should have source %v", i, expected.source)
		assert.Equal(t, expected.path, paths[i].Path,
			"Path %d should have path %s", i, expected.path)
	}
}

func TestBuildPromptPaths_LocalPathIsRelative(t *testing.T) {
	// Local prompts dir should be relative (project-specific)
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	localPath := paths[0].Path

	assert.False(t, filepath.IsAbs(localPath),
		"Local prompts path should be relative, got: %s", localPath)
	assert.Equal(t, ".awf/prompts", localPath,
		"Local prompts path should be .awf/prompts")
}

func TestBuildPromptPaths_GlobalPathIsAbsolute(t *testing.T) {
	// Global prompts dir should be absolute (user-level)
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	globalPath := paths[1].Path

	assert.True(t, filepath.IsAbs(globalPath),
		"Global prompts path should be absolute, got: %s", globalPath)
}

func TestBuildPromptPaths_GlobalPathContainsAwfPrompts(t *testing.T) {
	// Verify global path structure contains awf/prompts
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	globalPath := paths[1].Path

	assert.Contains(t, globalPath, "awf",
		"Global path should contain 'awf'")
	assert.True(t, strings.HasSuffix(globalPath, filepath.Join("awf", "prompts")),
		"Global path should end with awf/prompts, got: %s", globalPath)
}

func TestBuildPromptPaths_RespectsXDGConfigHome(t *testing.T) {
	// FR-002: XDG_CONFIG_HOME environment variable is respected
	// Set custom XDG_CONFIG_HOME
	customConfig := "/custom/config/path"
	t.Setenv("XDG_CONFIG_HOME", customConfig)

	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	expectedGlobalPath := filepath.Join(customConfig, "awf", "prompts")
	assert.Equal(t, expectedGlobalPath, paths[1].Path,
		"Global path should respect XDG_CONFIG_HOME")
}

func TestBuildPromptPaths_DefaultsToHomeConfig(t *testing.T) {
	// FR-002: Defaults to ~/.config when XDG_CONFIG_HOME is not set
	// Unset XDG_CONFIG_HOME
	t.Setenv("XDG_CONFIG_HOME", "")

	paths := BuildPromptPaths()

	require.Len(t, paths, 2)
	homeDir, _ := os.UserHomeDir()
	expectedGlobalPath := filepath.Join(homeDir, ".config", "awf", "prompts")
	assert.Equal(t, expectedGlobalPath, paths[1].Path,
		"Global path should default to ~/.config/awf/prompts")
}

func TestBuildPromptPaths_ConsistentResults(t *testing.T) {
	// Multiple calls should return consistent results
	paths1 := BuildPromptPaths()
	paths2 := BuildPromptPaths()

	require.Len(t, paths1, 2)
	require.Len(t, paths2, 2)

	for i := range paths1 {
		assert.Equal(t, paths1[i].Path, paths2[i].Path,
			"Path %d should be consistent across calls", i)
		assert.Equal(t, paths1[i].Source, paths2[i].Source,
			"Source %d should be consistent across calls", i)
	}
}

func TestBuildPromptPaths_SourcedPathStructure(t *testing.T) {
	// Verify returned slice contains properly structured SourcedPath values
	paths := BuildPromptPaths()

	require.Len(t, paths, 2)

	for i, path := range paths {
		// Each path should have non-empty Path field
		assert.NotEmpty(t, path.Path,
			"SourcedPath[%d] should have non-empty Path", i)

		// Source should be a valid Source type
		assert.True(t, path.Source == repository.SourceLocal || path.Source == repository.SourceGlobal,
			"SourcedPath[%d] should have valid Source (Local or Global)", i)
	}
}

func TestBuildPromptPaths_MirrorsWorkflowPathsPattern(t *testing.T) {
	// ADR-003: BuildPromptPaths follows same pattern as BuildWorkflowPaths
	// Both should return paths with same source types in same order (excluding env var)

	promptPaths := BuildPromptPaths()
	workflowPaths := BuildWorkflowPaths()

	// Prompt paths: local, global (2 paths)
	// Workflow paths: env (if set), local, global (2-3 paths)
	require.Len(t, promptPaths, 2)

	// Find matching sources in workflow paths
	workflowLocalIdx, workflowGlobalIdx := -1, -1
	for i, wp := range workflowPaths {
		switch wp.Source {
		case repository.SourceLocal:
			workflowLocalIdx = i
		case repository.SourceGlobal:
			workflowGlobalIdx = i
		case repository.SourceEnv:
			// SourceEnv not used in workflow paths in this test
		}
	}

	// Verify local comes before global in both
	if workflowLocalIdx != -1 && workflowGlobalIdx != -1 {
		assert.Less(t, workflowLocalIdx, workflowGlobalIdx,
			"In BuildWorkflowPaths, local should come before global")
	}

	// Prompt paths: local at 0, global at 1
	assert.Equal(t, repository.SourceLocal, promptPaths[0].Source)
	assert.Equal(t, repository.SourceGlobal, promptPaths[1].Source)
}

func TestBuildPromptPaths_TableDriven(t *testing.T) {
	// Table-driven test for various XDG_CONFIG_HOME scenarios
	tests := []struct {
		name           string
		xdgConfigHome  string
		expectLocalDir string
		expectGlobal   func() string // function to compute expected global path
	}{
		{
			name:           "default_xdg_unset",
			xdgConfigHome:  "",
			expectLocalDir: ".awf/prompts",
			expectGlobal: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, ".config", "awf", "prompts")
			},
		},
		{
			name:           "custom_xdg_config_home",
			xdgConfigHome:  "/tmp/custom-xdg",
			expectLocalDir: ".awf/prompts",
			expectGlobal: func() string {
				return filepath.Join("/tmp/custom-xdg", "awf", "prompts")
			},
		},
		{
			name:           "xdg_with_trailing_slash",
			xdgConfigHome:  "/opt/config/",
			expectLocalDir: ".awf/prompts",
			expectGlobal: func() string {
				return filepath.Join("/opt/config/", "awf", "prompts")
			},
		},
		{
			name:           "xdg_with_spaces_in_path",
			xdgConfigHome:  "/home/user/My Config",
			expectLocalDir: ".awf/prompts",
			expectGlobal: func() string {
				return filepath.Join("/home/user/My Config", "awf", "prompts")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore XDG_CONFIG_HOME
			t.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)

			paths := BuildPromptPaths()

			require.Len(t, paths, 2)
			assert.Equal(t, tt.expectLocalDir, paths[0].Path)
			assert.Equal(t, tt.expectGlobal(), paths[1].Path)
		})
	}
}
