//go:build integration

// Feature: B018
package loops_test

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configuredLoopFixture = "../../fixtures/workflows"

func TestConfiguredLoopHandlesFailureTransitions(t *testing.T) {
	routes := []struct {
		code        int
		destination string
	}{
		{code: 0, destination: "mark_task_complete"},
		{code: 1, destination: "mark_task_blocked"},
		{code: 70, destination: "error_feature_memory"},
		{code: 2, destination: "error_assessment"},
	}

	for _, loopKind := range []string{"for_each", "while"} {
		for _, route := range routes {
			t.Run(fmt.Sprintf("%s/code_%d", loopKind, route.code), func(t *testing.T) {
				execCtx, err := runConfiguredLoop(t, map[string]any{
					"loop_kind": loopKind, "exit_code": route.code, "failure_mode": "routed",
				})
				require.NoError(t, err)
				require.Equal(t, workflow.StatusCompleted, execCtx.GetStatus())

				destination, exists := execCtx.GetStepState(route.destination)
				require.True(t, exists)
				assert.Equal(t, workflow.StatusCompleted, destination.Status)
				assert.Contains(t, destination.Output, "DESTINATION:"+route.destination)

				source, exists := execCtx.GetStepState("routed_probe")
				require.True(t, exists)
				assert.Equal(t, route.code, source.ExitCode)
				if route.code == 1 {
					assert.Equal(t, workflow.StatusFailed, source.Status)
					assert.Contains(t, source.Output, "ZPM-CRITICAL:1")
				}
			})
		}

		t.Run(loopKind+"/code_1_cli_output", func(t *testing.T) {
			stdout, stderr, err := runConfiguredLoopCLI(t, loopKind, 1, "routed")
			require.NoError(t, err)
			output := stdout + stderr
			assert.Contains(t, output, "ZPM-CRITICAL:1")
			assert.Contains(t, output, "routed_probe: failed")
			assert.NotContains(t, output, "✓ routed_probe: completed")
			assert.Contains(t, output, "Workflow completed.")
		})
	}
}

func TestConfiguredLoopReportsActualFailure(t *testing.T) {
	for _, loopKind := range []string{"for_each", "while"} {
		for _, failure := range []struct {
			name string
			code int
			mode string
			err  string
		}{
			{
				name: "unmatched_non_zero", code: 1, mode: "unmatched",
				err: "step unmatched_probe output: ZPM-CRITICAL:1: step unmatched_probe: exit code 1",
			},
			{
				name: "failed_external_escape", code: 2, mode: "external",
				err: "step external_probe failed: ZPM-CRITICAL:1",
			},
		} {
			t.Run(loopKind+"/"+failure.name, func(t *testing.T) {
				stdout, stderr, err := runConfiguredLoopCLI(t, loopKind, failure.code, failure.mode)
				require.ErrorContains(t, err, failure.err)
				output := stdout + stderr
				assert.Contains(t, output, "ZPM-CRITICAL:1")
				assert.Contains(t, output, "PRECEDING-OK")
				assert.Contains(t, output, "✓ preceding_step: completed")
				assert.NotContains(t, output, "Workflow completed.")
				assert.NotContains(t, output, "LOOP-WORKFLOW-SUCCESS")
				assert.NotContains(t, output, "✓ routed_probe: completed")
				assert.NotContains(t, output, "✓ unmatched_probe: completed")
				assert.NotContains(t, output, "✓ external_probe: completed")
				assert.NotContains(t, output, "✓ "+loopKind+"_loop: completed")
				assert.NotContains(t, output, "EXTERNAL-TARGET-EXECUTED")
			})
		}
	}
}

func runConfiguredLoop(t *testing.T, inputs map[string]any) (*workflow.ExecutionContext, error) {
	t.Helper()
	repo := repository.NewYAMLRepository(configuredLoopFixture)
	wf, err := repo.Load(t.Context(), "loop-handled-failure-transitions")
	require.NoError(t, err)

	stateStore := store.NewJSONStore(filepath.Join(t.TempDir(), "states"))
	shellExecutor := executor.NewShellExecutor(executor.WithShell("/bin/sh"))
	setup, err := application.NewExecutionSetup(repo, stateStore, shellExecutor, ports.NopLogger{}).Build(t.Context())
	require.NoError(t, err)
	t.Cleanup(setup.Cleanup)

	return setup.ExecService.RunWithWorkflowAndRunID(context.Background(), wf, inputs, "configured-loop-test")
}

func runConfiguredLoopCLI(t *testing.T, loopKind string, exitCode int, failureMode string) (string, string, error) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("AWF_WORKFLOWS_PATH", configuredLoopFixture)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("AWF_PLUGINS_DIR", filepath.Join(root, "plugins"))

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{
		"run", "loop-handled-failure-transitions",
		"--input", "loop_kind=" + loopKind,
		"--input", fmt.Sprintf("exit_code=%d", exitCode),
		"--input", "failure_mode=" + failureMode,
		"--storage", filepath.Join(root, "storage"),
		"--no-color", "--output", "buffered", "--skip-plugins",
	})

	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}
