//go:build integration

// Feature: F088
package tui_test

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
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/tui"
)

func TestTUICommand_DispatchesNotifierThroughDiscoveredAdapter(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and executes the public TUI with an external recording adapter")
	}

	const (
		workflowName     = "tui-notifier-dispatch"
		payloadCanary    = "tui-payload-canary"
		defaultCanary    = "tui-channel-canary"
		credentialCanary = "tui-credential-canary"
	)
	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflows")
	pluginDir := filepath.Join(root, ".awf", "plugins")
	recordPath := filepath.Join(root, "adapter-record.jsonl")
	exitPath := filepath.Join(root, "adapter-exit.json")
	hostLogPath := filepath.Join(root, "host-audit.jsonl")
	require.NoError(t, os.MkdirAll(workflowDir, 0o700))
	pluginID := installTUIRecordingAdapter(t, root, pluginDir, credentialCanary)

	configPath := filepath.Join(root, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(fmt.Sprintf(
		"notifier:\n  backend: %s\n  default_channel: %s\n", pluginID, defaultCanary)), 0o600))
	workflowYAML := fmt.Sprintf(`name: %s
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
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, workflowName+".yaml"), []byte(workflowYAML), 0o600))

	binaryPath := buildTUIAWFBinary(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	commandLine := shellQuote(binaryPath) + " tui"
	cmd := exec.CommandContext(ctx, "script", "-qefc", commandLine, "/dev/null")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color", "AWF_CONFIG_PATH="+configPath,
		"AWF_PLUGINS_PATH="+pluginDir, "AWF_WORKFLOWS_PATH="+workflowDir,
		"AWF_RECORDING_PATH="+recordPath, "AWF_ADAPTER_EXIT_PATH="+exitPath,
		"AWF_AUDIT_LOG="+hostLogPath, "XDG_CONFIG_HOME="+filepath.Join(root, "xdg-config"),
		"XDG_DATA_HOME="+filepath.Join(root, "xdg-data"), "XDG_STATE_HOME="+filepath.Join(root, "xdg-state"),
	)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	output := &boundedSynchronizedBuffer{limit: 1 << 20}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("bounded TUI transcript:\n%s", output.String())
		}
	})
	cmd.Stdout = output
	cmd.Stderr = output
	require.NoError(t, cmd.Start())

	waitForTUI(t, 10*time.Second, func() bool { return strings.Contains(output.String(), workflowName) }, "workflow readiness")
	_, err = io.WriteString(stdin, "\r")
	require.NoError(t, err)
	waitForTUI(t, 10*time.Second, func() bool {
		records := readTUIAdapterRecords(t, recordPath, false)
		return len(records) == 1
	}, "notifier completion")
	waitForTUI(t, 5*time.Second, func() bool { return strings.Contains(output.String(), "output=sent") }, "rendered completion")
	// The run view shows the outcome in its header once the run has ended. Only then does
	// ctrl+c quit: while a run is in flight it cancels the run instead.
	waitForTUI(t, 5*time.Second, func() bool { return strings.Contains(output.String(), "✓ done ·") }, "run outcome in the header")
	_, err = io.WriteString(stdin, "\x03")
	require.NoError(t, err)
	require.NoError(t, stdin.Close())
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	select {
	case waitErr := <-waitDone:
		require.NoError(t, waitErr)
	case <-time.After(5 * time.Second):
		t.Fatal("public TUI command did not exit within 5 seconds")
	}

	records := readTUIAdapterRecords(t, recordPath, true)
	require.Len(t, records, 1)
	assert.Equal(t, pluginID, records[0].Adapter)
	assert.Equal(t, "send", records[0].Operation)
	assert.Equal(t, defaultCanary, records[0].Inputs["channel"])
	assert.Equal(t, payloadCanary, records[0].Inputs["text"])
	exitReceipt, err := os.ReadFile(exitPath) //nolint:gosec // Test-owned isolated adapter receipt.
	require.NoError(t, err, "recording adapter must exit during TUI cleanup")
	assert.JSONEq(t, fmt.Sprintf(`{"pid":%d}`, records[0].PID), string(exitReceipt))

	hostLog, err := os.ReadFile(hostLogPath) //nolint:gosec // Test-owned isolated audit path.
	if errors.Is(err, os.ErrNotExist) {
		hostLog = nil
	} else {
		require.NoError(t, err)
	}
	for _, rendered := range []string{output.String(), string(hostLog)} {
		assert.NotContains(t, rendered, payloadCanary)
		assert.NotContains(t, rendered, defaultCanary)
		assert.NotContains(t, rendered, credentialCanary)
	}
}

type tuiAdapterRecord struct {
	Adapter   string         `json:"adapter"`
	Operation string         `json:"operation"`
	Inputs    map[string]any `json:"inputs"`
	PID       int            `json:"pid"`
}

type boundedSynchronizedBuffer struct {
	mu    sync.Mutex
	data  bytes.Buffer
	limit int
}

func (b *boundedSynchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - b.data.Len()
	if remaining > 0 {
		_, _ = b.data.Write(p[:min(len(p), remaining)])
	}
	return len(p), nil
}

func (b *boundedSynchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.String()
}

func waitForTUI(t *testing.T, timeout time.Duration, ready func() bool, stage string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for TUI %s", stage)
}

func readTUIAdapterRecords(t *testing.T, path string, required bool) []tuiAdapterRecord {
	t.Helper()
	file, err := os.Open(path) //nolint:gosec // Test-owned isolated record path.
	if errors.Is(err, os.ErrNotExist) && !required {
		return nil
	}
	require.NoError(t, err)
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, 1<<20))
	var records []tuiAdapterRecord
	for scanner.Scan() {
		var record tuiAdapterRecord
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &record))
		records = append(records, record)
	}
	require.NoError(t, scanner.Err())
	return records
}

func buildTUIAWFBinary(t *testing.T) string {
	t.Helper()
	repoRoot := findTUIRepoRoot(t)
	binaryPath := filepath.Join(t.TempDir(), "awf")
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, "./cmd/awf")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	return binaryPath
}

func installTUIRecordingAdapter(t *testing.T, root, pluginsDir, credential string) string {
	t.Helper()
	manifest := fmt.Sprintf("name: recording\nversion: 1.0.0\nawf_version: \">=0.1.0\"\ncapabilities:\n  - operations\nconfig:\n  adapter_credential:\n    type: string\n    default: %s\n", credential)
	var metadata struct {
		Name string `yaml:"name"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &metadata))
	require.NotEmpty(t, metadata.Name)
	destination := filepath.Join(pluginsDir, metadata.Name)
	require.NoError(t, os.MkdirAll(destination, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(destination, "plugin.yaml"), []byte(manifest), 0o600))
	sourcePath := filepath.Join(root, "recording-plugin.go")
	require.NoError(t, os.WriteFile(sourcePath, []byte(tuiRecordingAdapterSource), 0o600))
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(destination, "awf-plugin-"+metadata.Name), sourcePath)
	cmd.Dir = findTUIRepoRoot(t)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	return metadata.Name
}

func findTUIRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "repository root not found")
		dir = parent
	}
}

func shellQuote(value string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(value, "'", "'\\''"))
}

const tuiRecordingAdapterSource = `package main

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
	return sdk.NewSuccessResult("sent", map[string]any{"message_id": "message-001"}), nil
}

func main() {
	sdk.Serve(&plugin{BasePlugin: sdk.BasePlugin{PluginName: "recording", PluginVersion: "1.0.0"}})
	receipt, err := json.Marshal(map[string]any{"pid": os.Getpid()})
	if err != nil { _, _ = os.Stderr.WriteString(err.Error()); os.Exit(1) }
	if err := os.WriteFile(os.Getenv("AWF_ADAPTER_EXIT_PATH"), receipt, 0o600); err != nil {
		_, _ = os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}
`

func TestTUIModel_WorkflowExecutionLifecycle(t *testing.T) {
	wf := &workflow.Workflow{Name: "deploy-app", Description: "Deploy to production"}

	// The TUI's sole execution entry point is the WorkflowFacade. Wire a fake facade
	// (returning a live RunSession) into the bridge exactly as production does via
	// command.go/SetFacade — no legacy ExecutionContext or Done channel is involved.
	facade := &fakeFacade{session: newFakeRunSession("exec-abc123")}
	bridge := tui.NewBridge(&fakeLister{wf: wf}, nil)
	bridge.SetFacade(facade)
	m := tui.NewWithBridge(bridge, context.Background())

	resizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updated, _ := m.Update(resizeMsg)
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.WorkflowsLoadedMsg{Workflows: []*workflow.Workflow{wf}})
	m = updated.(tui.Model)

	output := m.View().Content
	assert.Contains(t, output, "1 Workflows", "tab bar should show the workflows tab")
	assert.Contains(t, output, "2 History", "tab bar should show the history tab")

	// Launch through the facade — this is exactly what LaunchWorkflowMsg triggers in
	// production. The resulting ExecutionStartedMsg carries the live RunSession.
	startedMsg := bridge.RunWorkflowViaFacade(context.Background(), wf.Name, nil)()
	started, ok := startedMsg.(tui.ExecutionStartedMsg)
	require.True(t, ok, "facade launch must yield ExecutionStartedMsg, got %T", startedMsg)
	require.NotNil(t, started.Session, "facade-driven start must carry a live RunSession")

	updated, _ = m.Update(started)
	m = updated.(tui.Model)

	// A started run opens the full-frame run view for that workflow.
	output = m.View().Content
	assert.Contains(t, output, "deploy-app", "the run view is titled with the workflow name")
	assert.Contains(t, output, "Steps", "the run view lists the steps")

	// In production the session's terminal event drives ExecutionFinishedMsg via
	// StartEventLoop; deliver it directly here to finalize the run view.
	updated, _ = m.Update(tui.ExecutionFinishedMsg{Err: nil})
	m = updated.(tui.Model)

	output = m.View().Content
	assert.NotEmpty(t, output, "view should render after facade-driven execution lifecycle")
}

func TestTUIModel_ErrorsAreHandledGracefully(t *testing.T) {
	m := tui.New()

	errMsg := tui.ErrMsg{Err: assert.AnError}
	updated, _ := m.Update(errMsg)
	m = updated.(tui.Model)

	// ErrMsg does not crash and the model remains functional.
	output := m.View().Content
	assert.NotEmpty(t, output, "view should still render after error message")
}

func TestTUIModel_WindowResize(t *testing.T) {
	m := tui.New()

	resizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updated, _ := m.Update(resizeMsg)
	m = updated.(tui.Model)

	output := m.View().Content
	assert.NotEmpty(t, output, "view should render after resize")
}

func TestTUIModel_HistoryLoadedMsg_PopulatesHistoryTab(t *testing.T) {
	m := tui.New()

	records := []*workflow.ExecutionRecord{
		{ID: "exec-1", WorkflowName: "my-workflow", Status: "success"},
	}
	stats := &workflow.HistoryStats{TotalExecutions: 1, SuccessCount: 1}

	updated, _ := m.Update(tui.HistoryLoadedMsg{Records: records, Stats: stats})
	m = updated.(tui.Model)

	// Switch to the history tab to verify content.
	updated, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(tui.Model)

	output := m.View().Content
	assert.Contains(t, output, "my-workflow", "the loaded execution is listed")
}

// --- Facade test doubles: the TUI's only execution path is the WorkflowFacade ---

// fakeRunSession is a minimal ports.RunSession with no live events; the execution
// lifecycle is asserted at the model/message boundary, not by streaming events.
type fakeRunSession struct {
	id     string
	events chan ports.Event
}

func newFakeRunSession(id string) *fakeRunSession {
	ch := make(chan ports.Event)
	close(ch) // StartEventLoop ranges over a closed channel and exits cleanly
	return &fakeRunSession{id: id, events: ch}
}

func (s *fakeRunSession) ID() string                        { return s.id }
func (s *fakeRunSession) Events() <-chan ports.Event        { return s.events }
func (s *fakeRunSession) Respond(ports.InputResponse) error { return nil }
func (s *fakeRunSession) Err() error                        { return nil }
func (s *fakeRunSession) Close() error                      { return nil }

// fakeFacade is a minimal ports.WorkflowFacade whose Run returns the wired session.
type fakeFacade struct {
	session ports.RunSession
}

func (f *fakeFacade) List(context.Context) ([]ports.WorkflowSummary, error) { return nil, nil }

func (f *fakeFacade) Validate(context.Context, ports.RunRequest) (ports.ValidationReport, error) { //nolint:gocritic // interface contract: RunRequest passed by value
	return ports.ValidationReport{}, nil
}

func (f *fakeFacade) Status(context.Context, string) (ports.RunStatus, error) {
	return ports.RunStatus{}, nil
}

func (f *fakeFacade) History(context.Context, ports.HistoryFilter) ([]ports.RunRecord, error) {
	return nil, nil
}

func (f *fakeFacade) Run(_ context.Context, _ ports.RunRequest) (ports.RunSession, error) { //nolint:gocritic // interface contract: RunRequest passed by value
	return f.session, nil
}

func (f *fakeFacade) Resume(_ context.Context, _ ports.ResumeRequest) (ports.RunSession, error) { //nolint:gocritic // interface contract: ResumeRequest passed by value
	return f.session, nil
}

// fakeLister is a minimal tui.WorkflowLister so RunWorkflowViaFacade can load the
// workflow definition for the monitoring tab's step tree.
type fakeLister struct {
	wf *workflow.Workflow
}

func (l *fakeLister) ListAllWorkflows(context.Context) ([]workflow.WorkflowEntry, error) {
	return []workflow.WorkflowEntry{{Name: l.wf.Name}}, nil
}

func (l *fakeLister) GetWorkflow(context.Context, string) (*workflow.Workflow, error) {
	return l.wf, nil
}

func (l *fakeLister) ValidateWorkflow(context.Context, string) error { return nil }
