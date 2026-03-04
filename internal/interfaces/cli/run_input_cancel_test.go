package cli

// T005: Thread ctx through collectMissingInputsIfNeeded and callers in run.go
//
// These tests verify that context.Context is properly threaded through
// collectMissingInputsIfNeeded so Ctrl+C (SIGINT → ctx cancel) propagates
// to the blocking I/O layer without hanging the process.
//
// Test strategy:
//   - collectMissingInputsIfNeeded is unexported → package cli (internal) tests
//   - Blocking I/O path (stdin is terminal) cannot be triggered in unit tests
//     because strings.NewReader is not a TTY; tests cover all other paths
//   - hasMissingRequiredInputs is tested directly as the guard condition
//
// ASCII wireframe:
//
//	collectMissingInputsIfNeeded(ctx, cmd, wf, inputs, cfg, logger)
//	     │
//	     ├─ hasMissingRequiredInputs → false → return inputs, nil
//	     ├─ isTerminal(stdin) → false → return UserError (no blocking I/O)
//	     └─ isTerminal(stdin) → true → CollectMissingInputs(ctx, ...) → PromptForInput(ctx, ...)

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildCmdWithStdin creates a cobra.Command whose stdin is reader.
// This avoids a real TTY so isTerminal returns false, exercising the
// non-blocking error path without hanging.
func buildCmdWithStdin(stdin *strings.Reader) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetIn(stdin)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

// buildTestWorkflowWithInputs creates a minimal workflow with the given inputs.
func buildTestWorkflowWithInputs(inputs []workflow.Input) *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "test-wf",
		Initial: "start",
		Inputs:  inputs,
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}
}

// buildMinimalConfig returns a Config with NoColor set to avoid terminal-color
// side effects in test output.
func buildMinimalConfig() *Config {
	return &Config{NoColor: true}
}

// buildSilentLogger returns a cliLogger that discards all output.
func buildSilentLogger() *cliLogger {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})
	return &cliLogger{formatter: formatter, silent: true}
}

// TestCollectMissingInputsIfNeeded_NoMissingInputs verifies that when all required
// inputs are already provided, collectMissingInputsIfNeeded returns immediately
// without touching the collector — regardless of ctx state.
//
// This covers the happy path: ctx is valid, no prompts are triggered.
func TestCollectMissingInputsIfNeeded_NoMissingInputs(t *testing.T) {
	wf := buildTestWorkflowWithInputs([]workflow.Input{
		{Name: "env", Required: true},
	})

	providedInputs := map[string]any{"env": "prod"}
	cmd := buildCmdWithStdin(strings.NewReader(""))
	cfg := buildMinimalConfig()
	logger := buildSilentLogger()

	result, err := collectMissingInputsIfNeeded(context.Background(), cmd, wf, providedInputs, cfg, logger)

	require.NoError(t, err, "no missing inputs must return without error")
	assert.Equal(t, providedInputs, result, "result must equal the originally provided inputs")
}

// TestCollectMissingInputsIfNeeded_PreCancelledCtx_NoMissingInputs verifies that
// a pre-cancelled context with all inputs provided returns without error.
//
// The guard short-circuits before any context check on the collection path.
func TestCollectMissingInputsIfNeeded_PreCancelledCtx_NoMissingInputs(t *testing.T) {
	wf := buildTestWorkflowWithInputs([]workflow.Input{
		{Name: "env", Required: true},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	result, err := collectMissingInputsIfNeeded(
		ctx,
		buildCmdWithStdin(strings.NewReader("")),
		wf,
		map[string]any{"env": "prod"},
		buildMinimalConfig(),
		buildSilentLogger(),
	)

	require.NoError(t, err, "short-circuit before collection means no error on pre-cancelled ctx")
	assert.Equal(t, map[string]any{"env": "prod"}, result)
}

// TestCollectMissingInputsIfNeeded_NonTerminalStdin_MissingInputs verifies that when
// stdin is not a terminal and required inputs are missing, a UserError is returned
// (not a hang, not context.Canceled).
//
// This confirms the non-blocking code path works correctly: the context is accepted
// by the function signature and the function completes without blocking I/O.
func TestCollectMissingInputsIfNeeded_NonTerminalStdin_MissingInputs(t *testing.T) {
	wf := buildTestWorkflowWithInputs([]workflow.Input{
		{Name: "api_key", Required: true},
	})

	cmd := buildCmdWithStdin(strings.NewReader("")) // strings.Reader is not a TTY

	result, err := collectMissingInputsIfNeeded(
		context.Background(),
		cmd,
		wf,
		map[string]any{}, // api_key is missing
		buildMinimalConfig(),
		buildSilentLogger(),
	)

	require.Error(t, err, "missing required input with non-terminal stdin must return an error")
	assert.Nil(t, result, "result must be nil on error")
	assert.Contains(t, err.Error(), "missing required inputs",
		"error must describe the missing input condition")
}

// TestCollectMissingInputsIfNeeded_CtxAcceptedAsFirstParam is a compile-time
// and runtime verification that collectMissingInputsIfNeeded accepts
// context.Context as its first parameter (T005 primary wiring test).
//
// RED phase: if ctx is not threaded (old signature without ctx), this call
// will not compile → failing the build. With the correct signature it compiles
// and the non-terminal path returns a UserError without blocking.
func TestCollectMissingInputsIfNeeded_CtxAcceptedAsFirstParam(t *testing.T) {
	wf := buildTestWorkflowWithInputs([]workflow.Input{
		{Name: "region", Required: true},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Pass the cancellable ctx — verifies the signature accepts context.Context.
	_, err := collectMissingInputsIfNeeded(
		ctx,
		buildCmdWithStdin(strings.NewReader("")),
		wf,
		map[string]any{},
		buildMinimalConfig(),
		buildSilentLogger(),
	)

	// With non-terminal stdin and missing required input, we expect a UserError.
	// The key assertion is that the call compiled and completed without hanging.
	require.Error(t, err)
	assert.False(t, errors.Is(err, context.Canceled),
		"non-terminal path must not return context.Canceled; it short-circuits before blocking I/O")
}

// TestCollectMissingInputsIfNeeded_NilWorkflow verifies graceful handling when
// the workflow is nil (defensive guard in hasMissingRequiredInputs).
func TestCollectMissingInputsIfNeeded_NilWorkflow(t *testing.T) {
	result, err := collectMissingInputsIfNeeded(
		context.Background(),
		buildCmdWithStdin(strings.NewReader("")),
		nil,
		map[string]any{},
		buildMinimalConfig(),
		buildSilentLogger(),
	)

	require.NoError(t, err, "nil workflow must return without error")
	assert.NotNil(t, result, "result must not be nil for nil workflow")
}
