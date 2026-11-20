//go:build integration

// Feature: 25
package cli_test

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

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/notifier"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	recordingOperationsEnv = "AWF_RECORDING_OPERATIONS"
	recordingFailureEnv    = "AWF_RECORDING_FAILURE"
	recordingPathEnv       = "AWF_RECORDING_PATH"
	recordingCredential    = "adapter-credential-output-canary"
)

type notifierAdapterRecord struct {
	Adapter   string         `json:"adapter"`
	Operation string         `json:"operation"`
	Inputs    map[string]any `json:"inputs"`
}

type notifierRunFixture struct {
	root         string
	repoRoot     string
	workflowDir  string
	storageDir   string
	recordPath   string
	hostLogPath  string
	pluginID     string
	workflowName string
}

func TestRunCommandDispatchesNotifierOperationsToDiscoveredAdapter(t *testing.T) {
	fixture := newNotifierRunFixture(t, []string{"send", "update", "react", "upload"})
	fixture.writeConfig(t, "recording", "default-channel-output-canary")
	fixture.writeWorkflow(t, `
  initial: send
  send:
    type: operation
    operation: notifier.send
    inputs:
      backend: recording
      channel: dispatch-channel
      text: "payload ; $(printf injected) ' quoted"
    on_success: update
  update:
    type: operation
    operation: notifier.update
    inputs:
      backend: recording
      channel: dispatch-channel
      message_id: message-existing
      text: "updated | & payload"
    on_success: react
  react:
    type: operation
    operation: notifier.react
    inputs:
      backend: recording
      channel: dispatch-channel
      message_id: message-existing
      emoji: ":white_check_mark:"
    on_success: upload
  upload:
    type: operation
    operation: notifier.upload
    inputs:
      backend: recording
      channel: dispatch-channel
      filename: report.txt
      content: "content > /tmp/not-created"
    on_success: done
  done:
    type: terminal
    status: success
`)

	output, err := fixture.execute(t)
	require.NoError(t, err, output)
	records := fixture.readRecords(t)
	require.Len(t, records, 4)
	assert.Equal(t, []string{"send", "update", "react", "upload"}, recordOperations(records))
	assert.Equal(t, "payload ; $(printf injected) ' quoted", records[0].Inputs["text"])
	assert.Equal(t, "content > /tmp/not-created", records[3].Inputs["content"])

	execution := fixture.loadExecution(t)
	assert.Equal(t, "message-001", execution.States["send"].Response["message_id"])
	assert.Equal(t, "dispatch-channel", execution.States["send"].Response["channel"])
	assert.Equal(t, "message-updated-001", execution.States["update"].Response["message_id"])
	assert.Equal(t, true, execution.States["react"].Response["ok"])
	assert.Equal(t, "file-001", execution.States["upload"].Response["file_id"])
	assert.Equal(t, "https://recording.invalid/file-001", execution.States["upload"].Response["permalink"])
	fixture.assertConfidential(t, output, []string{
		"payload ; $(printf injected) ' quoted", "updated | & payload",
		"content > /tmp/not-created", recordingCredential,
	})
}

func TestRunCommandAppliesNotifierDefaultsAndPreservesRichPayload(t *testing.T) {
	fixture := newNotifierRunFixture(t, []string{"send"})
	overridePluginID := installRecordingNotifierAdapter(t, fixture.root, fixture.repoRoot, "recording-override", []string{"send"})
	fixture.writeConfig(t, fixture.pluginID, "default-channel-output-canary")
	fixture.writeWorkflow(t, `
  initial: defaulted
  defaulted:
    type: operation
    operation: notifier.send
    inputs:
      text: "default-payload-output-canary ; $HOME"
      rich:
        card:
          title: "rich-title-output-canary"
          actions:
            - label: ""
              command: "$(touch should-not-exist)"
    on_success: overridden
  overridden:
    type: operation
    operation: notifier.send
    inputs:
      backend: recording-override
      channel: explicit-channel
      text: override-payload
      rich:
        nested:
          enabled: true
          count: 2
    on_success: done
  done:
    type: terminal
    status: success
`)

	output, err := fixture.execute(t)
	require.NoError(t, err, output)
	records := fixture.readRecords(t)
	require.Len(t, records, 2)
	assert.Equal(t, []string{fixture.pluginID, overridePluginID}, recordAdapters(records))
	assert.Equal(t, "default-channel-output-canary", records[0].Inputs["channel"])
	assert.Equal(t, "explicit-channel", records[1].Inputs["channel"])
	assert.Equal(t, map[string]any{
		"card": map[string]any{
			"title": "rich-title-output-canary",
			"actions": []any{map[string]any{
				"label":   "",
				"command": "$(touch should-not-exist)",
			}},
		},
	}, records[0].Inputs["rich"])
	assert.Equal(t, map[string]any{
		"nested": map[string]any{"enabled": true, "count": float64(2)},
	}, records[1].Inputs["rich"])

	execution := fixture.loadExecution(t)
	assert.Equal(t, "default-channel-output-canary", execution.States["defaulted"].Response["channel"])
	assert.Equal(t, "explicit-channel", execution.States["overridden"].Response["channel"])
	fixture.assertConfidential(t, output, []string{
		"default-payload-output-canary", "rich-title-output-canary",
		"$(touch should-not-exist)", recordingCredential,
	})
}

func TestRunCommandReportsMissingOrUnsupportedNotifierAdapter(t *testing.T) {
	t.Run("missing adapter", func(t *testing.T) {
		fixture := newNotifierRunFixture(t, nil)
		fixture.removePlugin(t)
		fixture.writeConfig(t, "recording", "alerts")
		fixture.writeSingleOperationWorkflow(t, "notifier.send", "      text: payload\n")

		output, err := fixture.execute(t)
		require.Error(t, err)
		assert.ErrorIs(t, err, notifier.ErrAdapterUnavailable)
		assert.Contains(t, output+err.Error(), `recording.send`)
		assert.Equal(t, cli.ExitExecution, commandExitCode(t, err))
		assert.Empty(t, fixture.readRecords(t))
	})

	t.Run("disabled adapter", func(t *testing.T) {
		fixture := newNotifierRunFixture(t, []string{"send"})
		fixture.writeConfig(t, fixture.pluginID, "alerts")
		fixture.writeSingleOperationWorkflow(t, "notifier.send", "      text: payload\n")

		output, err := fixture.execute(t)
		require.NoError(t, err, output)
		before := fixture.readRecords(t)
		require.Len(t, before, 1)
		assert.Equal(t, "send", before[0].Operation)

		cmd := cli.NewRootCommand()
		cmd.SetArgs([]string{"plugin", "disable", fixture.pluginID, "--storage", fixture.storageDir})
		require.NoError(t, cmd.Execute())

		output, err = fixture.execute(t)
		require.Error(t, err)
		assert.ErrorIs(t, err, notifier.ErrAdapterUnavailable)
		assert.Contains(t, output+err.Error(), fixture.pluginID+".send")
		assert.Equal(t, cli.ExitExecution, commandExitCode(t, err))
		assert.Equal(t, before, fixture.readRecords(t), "disabled adapter must not receive another call")
	})

	t.Run("unsupported adapter operation", func(t *testing.T) {
		fixture := newNotifierRunFixture(t, []string{"send"})
		fixture.writeConfig(t, fixture.pluginID, "alerts")
		fixture.writeSingleOperationWorkflow(t, "notifier.update", "      message_id: message-001\n      text: updated\n")

		output, err := fixture.execute(t)
		require.Error(t, err)
		assert.ErrorIs(t, err, notifier.ErrAdapterUnavailable)
		assert.Contains(t, output+err.Error(), `recording.update`)
		assert.Equal(t, cli.ExitExecution, commandExitCode(t, err))
		assert.Empty(t, fixture.readRecords(t))
	})

	t.Run("empty required value", func(t *testing.T) {
		fixture := newNotifierRunFixture(t, []string{"send"})
		fixture.writeConfig(t, fixture.pluginID, "alerts")
		fixture.writeSingleOperationWorkflow(t, "notifier.send", "      text: \"\"\n")

		_, err := fixture.execute(t)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "text")
		assert.Empty(t, fixture.readRecords(t))
	})
}

func TestRunCommandReportsNotifierAdapterFailure(t *testing.T) {
	fixture := newNotifierRunFixture(t, []string{"send"})
	fixture.writeConfig(t, fixture.pluginID, "failure-channel")
	fixture.writeSingleOperationWorkflow(t, "notifier.send", "      text: failure-payload-output-canary\n")
	t.Setenv(recordingFailureEnv, "recording-adapter-failure-identity")

	output, err := fixture.execute(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recording-adapter-failure-identity")
	assert.Equal(t, cli.ExitExecution, commandExitCode(t, err))
	records := fixture.readRecords(t)
	require.Len(t, records, 1)
	assert.Equal(t, "send", records[0].Operation)
	fixture.assertConfidential(t, output, []string{"failure-payload-output-canary", recordingCredential})
}

func newNotifierRunFixture(t *testing.T, operations []string) notifierRunFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("builds and executes an external recording adapter")
	}
	root := t.TempDir()
	repoRoot, err := findRepoRoot()
	require.NoError(t, err)
	fixture := notifierRunFixture{
		root:         root,
		repoRoot:     repoRoot,
		workflowDir:  filepath.Join(root, "workflows"),
		storageDir:   filepath.Join(root, "storage"),
		recordPath:   filepath.Join(root, "adapter-records.jsonl"),
		hostLogPath:  filepath.Join(root, "host-audit.jsonl"),
		workflowName: "notifier-public-dispatch",
	}
	require.NoError(t, os.MkdirAll(fixture.workflowDir, 0o700))
	fixture.pluginID = installRecordingNotifierAdapter(t, root, fixture.repoRoot, "recording", operations)
	t.Setenv("AWF_PLUGINS_PATH", filepath.Join(root, "plugins"))
	t.Setenv("AWF_WORKFLOWS_PATH", fixture.workflowDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "xdg-state"))
	t.Setenv("AWF_AUDIT_LOG", fixture.hostLogPath)
	t.Setenv(recordingPathEnv, fixture.recordPath)
	t.Setenv(recordingOperationsEnv, strings.Join(operations, ","))
	t.Chdir(root)
	return fixture
}

func installRecordingNotifierAdapter(t *testing.T, root, repoRoot, pluginName string, operations []string) string {
	t.Helper()
	const manifestTemplate = `name: %s
version: 1.0.0
awf_version: ">=0.1.0"
capabilities:
  - operations
config:
  adapter_credential:
    type: string
    default: adapter-credential-output-canary
`
	manifest := fmt.Sprintf(manifestTemplate, pluginName)
	var metadata struct {
		Name string `yaml:"name"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &metadata))
	require.NotEmpty(t, metadata.Name)
	pluginDir := filepath.Join(root, "plugins", metadata.Name)
	require.NoError(t, os.MkdirAll(pluginDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(manifest), 0o600))
	if operations == nil {
		return metadata.Name
	}
	sourcePath := filepath.Join(root, "recording-plugin.go")
	source := strings.ReplaceAll(recordingNotifierAdapterSource, "__PLUGIN_NAME__", metadata.Name)
	require.NoError(t, os.WriteFile(sourcePath, []byte(source), 0o600))
	buildCtx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", filepath.Join(pluginDir, "awf-plugin-"+metadata.Name), sourcePath)
	build.Dir = repoRoot
	buildOutput, err := build.CombinedOutput()
	require.NoError(t, err, string(buildOutput))
	return metadata.Name
}

func (f notifierRunFixture) removePlugin(t *testing.T) {
	t.Helper()
	require.NoError(t, os.RemoveAll(filepath.Join(f.root, "plugins", f.pluginID)))
}

func (f notifierRunFixture) writeConfig(t *testing.T, backend, channel string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(f.root, ".awf"), 0o700))
	contents := fmt.Sprintf("notifier:\n  backend: %s\n  default_channel: %s\n", backend, channel)
	require.NoError(t, os.WriteFile(filepath.Join(f.root, ".awf", "config.yaml"), []byte(contents), 0o600))
}

func (f notifierRunFixture) writeWorkflow(t *testing.T, states string) {
	t.Helper()
	contents := "name: " + f.workflowName + "\nversion: \"1.0\"\nstates:\n" + states
	require.NoError(t, os.WriteFile(filepath.Join(f.workflowDir, f.workflowName+".yaml"), []byte(contents), 0o600))
}

func (f notifierRunFixture) writeSingleOperationWorkflow(t *testing.T, operation, inputs string) {
	t.Helper()
	f.writeWorkflow(t, "  initial: invoke\n  invoke:\n    type: operation\n    operation: "+operation+"\n    inputs:\n"+inputs+"    on_success: done\n  done:\n    type: terminal\n    status: success\n")
}

func (f notifierRunFixture) execute(t *testing.T) (string, error) {
	t.Helper()
	cmd, cleanup := cli.NewRootCommandAutoFacade()
	cleanupOnce := sync.OnceFunc(cleanup)
	t.Cleanup(cleanupOnce)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--storage", f.storageDir, "--no-color", "run", f.workflowName})
	execCtx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	err := cmd.ExecuteContext(execCtx)
	cleanupOnce()
	return output.String(), err
}

func (f notifierRunFixture) loadExecution(t *testing.T) *workflow.ExecutionContext {
	t.Helper()
	stateStore := store.NewJSONStore(filepath.Join(f.storageDir, "states"))
	ids, err := stateStore.List(context.Background())
	require.NoError(t, err)
	require.Len(t, ids, 1)
	execution, err := stateStore.Load(context.Background(), ids[0])
	require.NoError(t, err)
	return execution
}

func (f notifierRunFixture) readRecords(t *testing.T) []notifierAdapterRecord {
	t.Helper()
	file, err := os.Open(f.recordPath) //nolint:gosec // test-owned path.
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	require.NoError(t, err)
	defer file.Close()
	limited := io.LimitReader(file, 1<<20)
	scanner := bufio.NewScanner(limited)
	var records []notifierAdapterRecord
	for scanner.Scan() {
		var record notifierAdapterRecord
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &record))
		records = append(records, record)
	}
	require.NoError(t, scanner.Err())
	return records
}

func (f notifierRunFixture) assertConfidential(t *testing.T, commandOutput string, canaries []string) {
	t.Helper()
	hostLog, err := os.ReadFile(f.hostLogPath) //nolint:gosec // test-owned path.
	if errors.Is(err, os.ErrNotExist) {
		hostLog = nil
	} else {
		require.NoError(t, err)
	}
	for _, canary := range canaries {
		assert.NotContains(t, commandOutput, canary)
		assert.NotContains(t, string(hostLog), canary)
	}
}

func recordOperations(records []notifierAdapterRecord) []string {
	operations := make([]string, len(records))
	for i := range records {
		operations[i] = records[i].Operation
	}
	return operations
}

func recordAdapters(records []notifierAdapterRecord) []string {
	adapters := make([]string, len(records))
	for i := range records {
		adapters[i] = records[i].Adapter
	}
	return adapters
}

func commandExitCode(t *testing.T, err error) int {
	t.Helper()
	var exitCoder interface{ ExitCode() int }
	require.ErrorAs(t, err, &exitCoder)
	return exitCoder.ExitCode()
}

const recordingNotifierAdapterSource = `package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/awf-project/cli/pkg/plugin/sdk"
)

type plugin struct{ sdk.BasePlugin }

func (p *plugin) Operations() []string {
	configured := strings.TrimSpace(os.Getenv("AWF_RECORDING_OPERATIONS"))
	if configured == "" { return nil }
	return strings.Split(configured, ",")
}

func (p *plugin) HandleOperation(_ context.Context, name string, inputs map[string]any) (*sdk.OperationResult, error) {
	record, err := json.Marshal(map[string]any{"adapter": "__PLUGIN_NAME__", "operation": name, "inputs": inputs})
	if err != nil { return nil, err }
	record = append(record, '\n')
	if len(record) > 64*1024 { return nil, errors.New("record exceeds 64 KiB") }
	file, err := os.OpenFile(os.Getenv("AWF_RECORDING_PATH"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil { return nil, err }
	_, writeErr := file.Write(record)
	closeErr := file.Close()
	if writeErr != nil { return nil, writeErr }
	if closeErr != nil { return nil, closeErr }
	if failure := os.Getenv("AWF_RECORDING_FAILURE"); failure != "" { return nil, errors.New(failure) }
	channel, _ := inputs["channel"].(string)
	switch name {
	case "send":
		return sdk.NewSuccessResult("sent", map[string]any{"message_id": "message-001", "channel": channel, "permalink": "https://recording.invalid/message-001"}), nil
	case "update":
		return sdk.NewSuccessResult("updated", map[string]any{"message_id": "message-updated-001"}), nil
	case "react":
		return sdk.NewSuccessResult("ok", map[string]any{"ok": true}), nil
	case "upload":
		return sdk.NewSuccessResult("uploaded", map[string]any{"file_id": "file-001", "permalink": "https://recording.invalid/file-001"}), nil
	default:
		return nil, errors.New("unsupported recording operation")
	}
}

func main() {
	sdk.Serve(&plugin{BasePlugin: sdk.BasePlugin{PluginName: "__PLUGIN_NAME__", PluginVersion: "1.0.0"}})
}
`
