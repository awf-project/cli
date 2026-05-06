package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/plugin/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_ sdk.Plugin          = (*EventLoggerPlugin)(nil)
	_ sdk.EventSubscriber = (*EventLoggerPlugin)(nil)
)

func newTestPlugin(t *testing.T) *EventLoggerPlugin {
	t.Helper()
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck // test cleanup

	p := &EventLoggerPlugin{
		BasePlugin: sdk.BasePlugin{
			PluginName:    "event-logger",
			PluginVersion: "1.0.0",
		},
	}
	require.NoError(t, p.Init(context.Background(), nil))
	t.Cleanup(func() { p.Shutdown(context.Background()) }) //nolint:errcheck // test cleanup
	return p
}

func readLog(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(".awf", "logs", "event-logger.log"))
	require.NoError(t, err)
	return string(data)
}

func TestEventLoggerPlugin_Name(t *testing.T) {
	p := &EventLoggerPlugin{BasePlugin: sdk.BasePlugin{PluginName: "event-logger", PluginVersion: "1.0.0"}}
	assert.Equal(t, "event-logger", p.Name())
}

func TestEventLoggerPlugin_Version(t *testing.T) {
	p := &EventLoggerPlugin{BasePlugin: sdk.BasePlugin{PluginName: "event-logger", PluginVersion: "1.0.0"}}
	assert.Equal(t, "1.0.0", p.Version())
}

func TestEventLoggerPlugin_Init_CreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck // test cleanup

	p := &EventLoggerPlugin{BasePlugin: sdk.BasePlugin{PluginName: "event-logger", PluginVersion: "1.0.0"}}
	require.NoError(t, p.Init(context.Background(), nil))
	defer p.Shutdown(context.Background()) //nolint:errcheck // test cleanup

	_, statErr := os.Stat(filepath.Join(".awf", "logs", "event-logger.log"))
	assert.NoError(t, statErr)
}

func TestEventLoggerPlugin_Patterns(t *testing.T) {
	p := &EventLoggerPlugin{}
	assert.Equal(t, []string{"workflow.*", "step.*"}, p.Patterns())
}

func TestEventLoggerPlugin_HandleEvent_WritesToLogFile(t *testing.T) {
	p := newTestPlugin(t)
	event := sdk.Event{
		ID:        "evt-1",
		Type:      "workflow.started",
		Timestamp: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		Source:    "core",
		Metadata:  map[string]string{"workflow_id": "wf-123"},
	}

	_, err := p.HandleEvent(context.Background(), event)
	require.NoError(t, err)

	log := readLog(t)
	assert.Contains(t, log, "workflow.started")
	assert.Contains(t, log, "2026-05-06T12:00:00Z")
	assert.Contains(t, log, "source=core")
	assert.Contains(t, log, "wf-123")
}

func TestEventLoggerPlugin_HandleEvent_WorkflowStarted_NoEmission(t *testing.T) {
	p := newTestPlugin(t)
	event := sdk.Event{
		ID:        "evt-2",
		Type:      "workflow.started",
		Timestamp: time.Now(),
		Source:    "core",
		Metadata:  map[string]string{"workflow_id": "wf-123"},
	}

	emitted, err := p.HandleEvent(context.Background(), event)
	require.NoError(t, err)
	assert.Nil(t, emitted)
}

func TestEventLoggerPlugin_HandleEvent_WorkflowFailed_EmitsLoggerEvent(t *testing.T) {
	p := newTestPlugin(t)
	event := sdk.Event{
		ID:        "evt-3",
		Type:      "workflow.failed",
		Timestamp: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		Source:    "core",
		Metadata: map[string]string{
			"workflow_id":   "wf-456",
			"workflow_name": "deploy",
			"error":         "step build failed: exit code 1",
		},
	}

	emitted, err := p.HandleEvent(context.Background(), event)
	require.NoError(t, err)
	require.Len(t, emitted, 1)

	e := emitted[0]
	assert.Equal(t, "logger.workflow_failed", e.Type)
	assert.Equal(t, "event-logger", e.Source)
	assert.Equal(t, "wf-456", e.Metadata["original_workflow"])
	assert.Equal(t, "step build failed: exit code 1", e.Metadata["failure_reason"])
	assert.NotEmpty(t, e.Metadata["logged_at"])
}

func TestEventLoggerPlugin_HandleEvent_WorkflowFailed_MissingMetadata(t *testing.T) {
	p := newTestPlugin(t)
	event := sdk.Event{
		ID:        "evt-4",
		Type:      "workflow.failed",
		Timestamp: time.Now(),
		Source:    "core",
		Metadata:  map[string]string{},
	}

	emitted, err := p.HandleEvent(context.Background(), event)
	require.NoError(t, err)
	require.Len(t, emitted, 1)
	assert.Equal(t, "logger.workflow_failed", emitted[0].Type)
	assert.Empty(t, emitted[0].Metadata["original_workflow"])
}

func TestEventLoggerPlugin_Shutdown_ClosesLogFile(t *testing.T) {
	p := newTestPlugin(t)
	require.NotNil(t, p.logFile)
	require.NoError(t, p.Shutdown(context.Background()))
	assert.Nil(t, p.logFile)
}

func TestPluginYAMLManifest_ExistsAndIsValid(t *testing.T) {
	content, err := os.ReadFile("plugin.yaml")
	require.NoError(t, err, "plugin.yaml must exist in plugin directory")

	manifestStr := string(content)
	assert.Contains(t, manifestStr, "name: event-logger")
	assert.Contains(t, manifestStr, "version: 1.0.0")
	assert.Contains(t, manifestStr, "awf_version:")
	assert.Contains(t, manifestStr, "events")
	assert.Contains(t, manifestStr, "subscribe:")
	assert.Contains(t, manifestStr, "emit:")
	assert.Contains(t, manifestStr, "logger.workflow_failed")
}

func TestPluginMakefile_ExistsAndBuildable(t *testing.T) {
	content, err := os.ReadFile("Makefile")
	require.NoError(t, err, "Makefile must exist in plugin directory")
	assert.Contains(t, string(content), "build:")
}

func TestPluginREADME_ExistsAndDocumented(t *testing.T) {
	content, err := os.ReadFile("README.md")
	require.NoError(t, err, "README.md must exist in plugin directory")

	readmeStr := string(content)
	assert.Contains(t, readmeStr, "awf-plugin-event-logger")
	assert.Contains(t, readmeStr, "EventSubscriber")
	assert.Contains(t, readmeStr, "workflow.failed")
}

func BenchmarkEventLoggerPlugin_HandleEvent(b *testing.B) {
	dir := b.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer os.Chdir(origDir) //nolint:errcheck // benchmark cleanup

	p := &EventLoggerPlugin{BasePlugin: sdk.BasePlugin{PluginName: "event-logger", PluginVersion: "1.0.0"}}
	_ = p.Init(context.Background(), nil)
	defer p.Shutdown(context.Background()) //nolint:errcheck // benchmark cleanup

	ctx := context.Background()
	event := sdk.Event{
		ID:        "bench-1",
		Type:      "workflow.started",
		Timestamp: time.Now(),
		Source:    "core",
		Metadata:  map[string]string{"workflow_id": "wf-bench"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.HandleEvent(ctx, event)
	}
}
