package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/tracker"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type publicTrackerAdapter struct {
	calls     int
	lookups   int
	onExecute func(context.Context) (*pluginmodel.OperationResult, error)
}

func (a *publicTrackerAdapter) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	a.lookups++
	if name != "tracker-jira.create_issue" {
		return nil, false
	}
	return &pluginmodel.OperationSchema{Name: name, PluginName: "tracker-jira"}, true
}

func (a *publicTrackerAdapter) ListOperations() []*pluginmodel.OperationSchema {
	schema, _ := a.GetOperation("tracker-jira.create_issue")
	return []*pluginmodel.OperationSchema{schema}
}

func (a *publicTrackerAdapter) Execute(
	ctx context.Context,
	_ string,
	_ map[string]any,
) (*pluginmodel.OperationResult, error) {
	a.calls++
	if a.onExecute != nil {
		return a.onExecute(ctx)
	}
	return &pluginmodel.OperationResult{Success: true, Outputs: map[string]any{
		"id": "provider-id", "key": "TEAM-8", "url": "https://tracker.invalid/TEAM-8",
	}}, nil
}

var _ ports.OperationProvider = (*publicTrackerAdapter)(nil)

func TestValidateCommandAcceptsCanonicalTrackerOperation(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflows")
	require.NoError(t, os.MkdirAll(workflowDir, 0o755))
	t.Setenv("AWF_WORKFLOWS_PATH", workflowDir)
	t.Setenv("AWF_PLUGINS_PATH", filepath.Join(root, "plugins"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))

	writeTrackerValidationWorkflow := func(t *testing.T, name, operation string) {
		t.Helper()
		content := `name: ` + name + `
version: "1.0.0"
states:
  initial: create
  create:
    type: operation
    operation: ` + operation + `
    inputs:
      backend: jira
      title: Release blocker
    on_success: done
  done:
    type: terminal
    status: success
`
		require.NoError(t, os.WriteFile(filepath.Join(workflowDir, name+".yaml"), []byte(content), 0o600))
	}

	executeValidate := func(t *testing.T, workflowName string) (string, error) {
		t.Helper()
		command, cleanup := cli.NewRootCommandAutoFacade()
		defer cleanup()
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&output)
		command.SetArgs([]string{"--storage=" + filepath.Join(root, workflowName+"-storage"), "validate", workflowName})
		err := command.Execute()
		return output.String(), err
	}

	t.Run("canonical operation", func(t *testing.T) {
		writeTrackerValidationWorkflow(t, "tracker-valid", "tracker.create_issue")
		output, err := executeValidate(t, "tracker-valid")

		require.NoError(t, err, output)
		assert.Contains(t, output, "is valid")
	})

	t.Run("unknown operation", func(t *testing.T) {
		writeTrackerValidationWorkflow(t, "tracker-unknown", "tracker.unknown")
		output, err := executeValidate(t, "tracker-unknown")

		require.Error(t, err)
		diagnostic := strings.ToLower(output + err.Error())
		assert.Contains(t, diagnostic, "tracker.unknown")
		assert.Contains(t, diagnostic, "not found")
	})
}

func TestRunLoadsTrackerDefaultBackend(t *testing.T) {
	t.Setenv("AWF_TEST_MODE", "1")
	tempDir := setupTestDir(t)
	setupProjectConfig(t, tempDir, "tracker:\n  default_backend: jira\n")
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".awf", "states"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "history"), 0o755))
	createTestWorkflow(t, tempDir, "tracker-public.yaml", `name: tracker-public
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
    message: "{{states.create.output}}"
`)

	adapter := &publicTrackerAdapter{}
	command := cli.NewRootCommand(cli.WithExecutionSetupOptions(
		application.WithPluginProviders(application.PluginProviders{Operations: adapter}),
	))
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"--storage=" + tempDir, "run", "tracker-public", "--output=buffered"})

	require.NoError(t, command.Execute())
	var persisted []byte
	require.NoError(t, filepath.WalkDir(tempDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".json" {
			return walkErr
		}
		//nolint:gosec // WalkDir is restricted to the test-owned temporary root.
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		persisted = bytes.Join([][]byte{persisted, content}, nil)
		return nil
	}))
	assert.Contains(t, string(persisted), "TEAM-8")
	assert.Contains(t, string(persisted), "https://tracker.invalid/TEAM-8")
	assert.Equal(t, 1, adapter.calls)
}

func TestSingleStepLoadsTrackerDefaultBackend(t *testing.T) {
	t.Setenv("AWF_TEST_MODE", "1")
	tempDir := setupTestDir(t)
	setupProjectConfig(t, tempDir, "tracker:\n  default_backend: jira\n")
	createTestWorkflow(t, tempDir, "tracker-single-step.yaml", `name: tracker-single-step
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

	adapter := &publicTrackerAdapter{}
	command := cli.NewRootCommand(cli.WithExecutionSetupOptions(
		application.WithPluginProviders(application.PluginProviders{Operations: adapter}),
	))
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{
		"--storage=" + tempDir,
		"run", "tracker-single-step", "--step=create", "--output=buffered",
	})

	require.NoError(t, command.Execute(), output.String())
	assert.Contains(t, output.String(), "TEAM-8")
	assert.Contains(t, output.String(), "https://tracker.invalid/TEAM-8")
	assert.Equal(t, 1, adapter.calls)
}

func TestTrackerSingleStepHidesAdapterFailureDetails(t *testing.T) {
	const marker = "confidential-adapter-failure-marker"
	for _, tt := range []struct {
		name string
		run  func(context.Context) (*pluginmodel.OperationResult, error)
	}{
		{name: "Go error", run: func(context.Context) (*pluginmodel.OperationResult, error) {
			return nil, errors.New(marker)
		}},
		{name: "failure result", run: func(context.Context) (*pluginmodel.OperationResult, error) {
			return &pluginmodel.OperationResult{
				Error:   marker,
				Outputs: map[string]any{"secret": marker},
			}, nil
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AWF_TEST_MODE", "1")
			tempDir := setupTestDir(t)
			setupProjectConfig(t, tempDir, "tracker:\n  default_backend: jira\n")
			createTestWorkflow(t, tempDir, "tracker-failure.yaml", `name: tracker-failure
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

			adapter := &publicTrackerAdapter{onExecute: tt.run}
			cmd := cli.NewRootCommand(cli.WithExecutionSetupOptions(
				application.WithPluginProviders(application.PluginProviders{Operations: adapter}),
			))
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"--storage=" + tempDir, "run", "tracker-failure", "--step=create", "--output=buffered"})

			err := cmd.Execute()
			require.Error(t, err)
			assert.Equal(t, 1, adapter.calls)
			if strings.Contains(output.String(), marker) || strings.Contains(err.Error(), marker) {
				t.Fatal("single-step failure exposed confidential adapter details")
			}
			assert.NotContains(t, output.String(), "completed successfully")
		})
	}
}

func TestTrackerSingleStepPropagatesCallerCancellation(t *testing.T) {
	t.Setenv("AWF_TEST_MODE", "1")
	tempDir := setupTestDir(t)
	setupProjectConfig(t, tempDir, "tracker:\n  default_backend: jira\n")
	createTestWorkflow(t, tempDir, "tracker-cancel.yaml", `name: tracker-cancel
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var adapterContextErr error
	adapter := &publicTrackerAdapter{
		onExecute: func(adapterCtx context.Context) (*pluginmodel.OperationResult, error) {
			cancel()
			select {
			case <-adapterCtx.Done():
				adapterContextErr = adapterCtx.Err()
				return nil, adapterContextErr
			case <-time.After(2 * time.Second):
				return nil, context.DeadlineExceeded
			}
		},
	}
	command := cli.NewRootCommand(cli.WithExecutionSetupOptions(
		application.WithPluginProviders(application.PluginProviders{Operations: adapter}),
	))
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{
		"--storage=" + tempDir,
		"run", "tracker-cancel", "--step=create", "--output=buffered",
	})

	require.Error(t, command.ExecuteContext(ctx), output.String())
	assert.Equal(t, 1, adapter.calls)
	assert.ErrorIs(t, adapterContextErr, context.Canceled)
	assert.NotContains(t, output.String(), "completed successfully")
}

// TestRunAndSingleStepWithoutTrackerDefaultPreserveZeroValue verifies that public
// run entry points do not silently choose a tracker backend when project
// configuration omits tracker.default_backend. The injected adapter makes a
// delegation observable: backend selection must fail before the adapter
// execution boundary is reached.
func TestRunAndSingleStepWithoutTrackerDefaultPreserveZeroValue(t *testing.T) {
	workflow := `name: tracker-no-default
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
`

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "ordinary run",
			args: []string{"run", "tracker-no-default", "--output=buffered"},
		},
		{
			name: "single step",
			args: []string{"run", "tracker-no-default", "--step=create", "--output=buffered"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("AWF_TEST_MODE", "1")
			tempDir := setupTestDir(t)
			setupProjectConfig(t, tempDir, "inputs: {}\n")
			createTestWorkflow(t, tempDir, "tracker-no-default.yaml", workflow)

			adapter := &publicTrackerAdapter{}
			command := cli.NewRootCommand(cli.WithExecutionSetupOptions(
				application.WithPluginProviders(application.PluginProviders{Operations: adapter}),
			))
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs(append([]string{"--storage=" + tempDir}, test.args...))

			err := command.Execute()

			require.ErrorIs(t, err, tracker.ErrBackendRequired)
			assert.ErrorContains(t, err, "tracker backend is required")
			assert.Contains(t, output.String(), "tracker backend is required")
			assert.Zero(t, adapter.calls, "an omitted backend must not execute an external tracker adapter")
		})
	}
}

// TestTrackerPreviewsDoNotInitializeOrExecuteAdapter verifies that preview modes
// retain their infrastructure-free boundary even for a workflow containing a
// tracker operation. Interactive mode aborts at the public prompt before the
// operation can execute; dry-run renders the plan without constructing runtime
// tracker wiring.
func TestTrackerPreviewsDoNotInitializeOrExecuteAdapter(t *testing.T) {
	workflow := `name: tracker-preview
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
`

	tests := []struct {
		name  string
		args  []string
		stdin string
	}{
		{
			name: "dry run",
			args: []string{"run", "tracker-preview", "--dry-run"},
		},
		{
			name:  "interactive preview",
			args:  []string{"run", "tracker-preview", "--interactive"},
			stdin: "a\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("AWF_TEST_MODE", "1")
			t.Setenv("AWF_CONFIG_PATH", "")
			tempDir := setupTestDir(t)
			createTestWorkflow(t, tempDir, "tracker-preview.yaml", workflow)

			adapter := &publicTrackerAdapter{}
			command := cli.NewRootCommand(cli.WithExecutionSetupOptions(
				application.WithPluginProviders(application.PluginProviders{Operations: adapter}),
			))
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetIn(strings.NewReader(test.stdin))
			command.SetArgs(append([]string{"--storage=" + tempDir}, test.args...))

			require.NoError(t, command.Execute(), output.String())
			assert.Zero(t, adapter.lookups, "preview must not initialize tracker adapter resolution")
			assert.Zero(t, adapter.calls, "preview must not execute tracker adapter")
		})
	}
}
