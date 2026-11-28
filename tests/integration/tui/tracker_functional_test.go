//go:build integration && !windows

package tui_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pluginSDK "github.com/awf-project/cli/pkg/plugin/sdk"
)

const (
	tuiTrackerFixtureEnv = "AWF_TUI_TRACKER_FIXTURE"
	tuiTrackerSecretEnv  = "AWF_TUI_TRACKER_SECRET"
	tuiTrackerOutput     = "canonical-tui-tracker-result"
	tuiTrackerWorkflow   = "tracker-tui-delegation"
	tuiTrackerSecret     = "confidential-tui-tracker-marker"
)

type tuiTrackerFixture struct {
	pluginSDK.BasePlugin
}

func (*tuiTrackerFixture) Operations() []string { return []string{"create_issue"} }

func (*tuiTrackerFixture) Init(_ context.Context, _ map[string]any) error {
	return os.WriteFile(filepath.Join(os.Getenv(tuiTrackerFixtureEnv), "adapter.pid"), []byte(strconv.Itoa(os.Getpid())), 0o600) //nolint:gosec // Parent supplies a test-owned directory.
}

func (*tuiTrackerFixture) HandleOperation(_ context.Context, _ string, inputs map[string]any) (*pluginSDK.OperationResult, error) {
	root := os.Getenv(tuiTrackerFixtureEnv)
	if inputs["title"] != os.Getenv(tuiTrackerSecretEnv) {
		return nil, errors.New("tracker fixture did not receive the confidential title")
	}
	if err := os.WriteFile(filepath.Join(root, "adapter.called"), nil, 0o600); err != nil { //nolint:gosec // Parent supplies a test-owned directory.
		return nil, err
	}
	return pluginSDK.NewSuccessResult(tuiTrackerOutput, map[string]any{
		"id": "TUI-27", "key": tuiTrackerOutput, "url": "https://tracker.invalid/TUI-27", "secret": tuiTrackerSecret,
	}), nil
}

func (*tuiTrackerFixture) Shutdown(_ context.Context) error {
	return os.WriteFile(filepath.Join(os.Getenv(tuiTrackerFixtureEnv), "adapter.stopped"), nil, 0o600) //nolint:gosec // Parent supplies a test-owned directory.
}

func TestMain(m *testing.M) {
	if os.Getenv("AWF_PLUGIN") == "awf-plugin-v1" && os.Getenv(tuiTrackerFixtureEnv) != "" {
		pluginSDK.Serve(&tuiTrackerFixture{BasePlugin: pluginSDK.BasePlugin{PluginName: "tracker-jira", PluginVersion: "1.0.0"}})
		return
	}
	os.Exit(m.Run())
}

func TestTUICommand_TrackerWorkflowDelegatesThroughInstalledAdapter(t *testing.T) {
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

	repositoryRoot := tuiRepositoryRoot(t)
	buildRoot, err := os.MkdirTemp(repositoryRoot, ".tui-functional-*")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(buildRoot)) })
	awfBinary := filepath.Join(buildRoot, "awf")
	build := exec.CommandContext(context.Background(), "go", "build", "-buildvcs=false", "-o", awfBinary, "./cmd/awf") //nolint:gosec // Test-owned output path.
	build.Dir = repositoryRoot
	require.NoError(t, build.Run(), "AWF test binary must build")

	testBinary, err := os.Executable()
	require.NoError(t, err)
	pluginDir := filepath.Join(pluginRoot, "tracker-jira")
	require.NoError(t, os.MkdirAll(pluginDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte("name: tracker-jira\nversion: 1.0.0\nawf_version: \">=0.1.0\"\ncapabilities:\n  - operations\n"), 0o600))
	require.NoError(t, os.Symlink(testBinary, filepath.Join(pluginDir, "awf-plugin-tracker-jira")))

	configPath := filepath.Join(root, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("tracker:\n  default_backend: jira\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(workflowRoot, tuiTrackerWorkflow+".yaml"), []byte(`name: `+tuiTrackerWorkflow+`
version: "1.0.0"
states:
  initial: create
  create:
    type: operation
    operation: tracker.create_issue
    inputs:
      title: "{{env.`+tuiTrackerSecretEnv+`}}"
    on_success: done
  done:
    type: terminal
    status: success
`), 0o600))

	process := startTUIUnderPTY(t, awfBinary, []string{"--storage=" + filepath.Join(root, "storage"), "tui"}, []string{
		"TERM=xterm-256color",
		"AWF_CONFIG_PATH=" + configPath,
		"AWF_WORKFLOWS_PATH=" + workflowRoot,
		"AWF_PLUGINS_PATH=" + pluginRoot,
		tuiTrackerFixtureEnv + "=" + fixtureRoot,
		tuiTrackerSecretEnv + "=" + tuiTrackerSecret,
		"XDG_CONFIG_HOME=" + filepath.Join(root, "xdg-config"),
		"XDG_DATA_HOME=" + filepath.Join(root, "xdg-data"),
		"XDG_STATE_HOME=" + filepath.Join(root, "xdg-state"),
		"AWF_AUDIT_LOG=off",
	})
	require.True(t, process.waitFor(10*time.Second, tuiTrackerWorkflow), "tracker workflow must render before input is sent")
	require.NoError(t, process.send("/"+tuiTrackerWorkflow+"\r\r"))
	if !process.waitFor(10*time.Second, tuiTrackerOutput) {
		t.Fatalf("monitoring output did not render canonical result; sanitized terminal: %s", strings.ReplaceAll(process.output.text(), tuiTrackerSecret, "[REDACTED]"))
	}
	require.True(t, process.waitFor(3*time.Second, "success"), "monitoring view must render terminal success")
	require.FileExists(t, filepath.Join(fixtureRoot, "adapter.called"))

	require.NoError(t, process.send("q"))
	require.NoError(t, process.wait(8*time.Second), "TUI host must stop after quit")
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(filepath.Join(fixtureRoot, "adapter.stopped"))
		return statErr == nil
	}, 3*time.Second, 20*time.Millisecond, "adapter shutdown must complete")
	pidBytes, err := os.ReadFile(filepath.Join(fixtureRoot, "adapter.pid")) //nolint:gosec // Test-owned fixture path.
	require.NoError(t, err)
	adapterPID, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return syscall.Kill(adapterPID, 0) != nil }, 3*time.Second, 20*time.Millisecond, "adapter process must not survive TUI shutdown")
	if strings.Contains(process.output.text(), tuiTrackerSecret) {
		t.Fatal("TUI diagnostics exposed a confidential tracker value")
	}
}

func tuiRepositoryRoot(t *testing.T) string {
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
