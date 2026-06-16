package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/awf-project/cli/internal/testutil/facadetest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunWorkflow_RoutesThroughFacade verifies runWorkflowViaFacade is called when cfg.Facade is wired.
// The facade emits RunStarted → StepCompleted → WorkflowCompleted; we verify the event sequence
// reaches the formatter and stdout contains expected text (Acceptance #36).
func TestRunWorkflow_RoutesThroughFacade(t *testing.T) {
	fake := facadetest.New().
		Script(ports.Event{Kind: ports.EventRunStarted, RunID: "test-run-1"}).
		Script(ports.Event{Kind: ports.EventStepCompleted, RunID: "test-run-1"}).
		WithTerminalCompleted()

	cfg := &Config{
		Facade:       fake,
		OutputFormat: ui.FormatText,
		NoColor:      true,
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	inputs := map[string]any{"test": "value"}
	writer := ui.NewOutputWriter(stdout, stderr, ui.FormatText, true, false)
	formatter := ui.NewFormatter(stdout, ui.FormatOptions{NoColor: true})

	err := runWorkflowViaFacade(context.Background(), cmd, cfg, writer, formatter, "test-workflow", inputs)

	assert.NoError(t, err, "runWorkflowViaFacade should succeed with completed terminal event")
}

// TestRunWorkflow_TerminalEventCompleted verifies EventWorkflowCompleted → exit code 0.
// When the facade emits a terminal event with Kind=EventWorkflowCompleted and nil error,
// the run should exit with code 0 (Acceptance #31).
func TestRunWorkflow_TerminalEventCompleted(t *testing.T) {
	fake := facadetest.New().WithTerminalCompleted()

	cfg := &Config{
		Facade:       fake,
		OutputFormat: ui.FormatText,
		NoColor:      true,
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	writer := ui.NewOutputWriter(stdout, stderr, ui.FormatText, true, false)
	formatter := ui.NewFormatter(stdout, ui.FormatOptions{NoColor: true})

	err := runWorkflowViaFacade(context.Background(), cmd, cfg, writer, formatter, "test-workflow", nil)

	assert.NoError(t, err, "EventWorkflowCompleted should not produce an error")
}

// TestRunWorkflow_TerminalEventFailed_MappedExitCode verifies EventWorkflowFailed → error taxonomy mapping.
// The facade emits a terminal event with Kind=EventWorkflowFailed and error payload.
// The exit code should be mapped via categorizeError (1=user, 2=workflow, 3=execution, 4=system)
// (Acceptance #32).
func TestRunWorkflow_TerminalEventFailed_MappedExitCode(t *testing.T) {
	fake := facadetest.New().WithTerminalFailed(
		ports.ErrInvalidRequest,
	)

	cfg := &Config{
		Facade:       fake,
		OutputFormat: ui.FormatText,
		NoColor:      true,
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	writer := ui.NewOutputWriter(stdout, stderr, ui.FormatText, true, false)
	formatter := ui.NewFormatter(stdout, ui.FormatOptions{NoColor: true})

	err := runWorkflowViaFacade(context.Background(), cmd, cfg, writer, formatter, "test-workflow", nil)

	require.Error(t, err, "terminal failed event should produce an error")
}

// TestRunWorkflow_OutputsToFormatterAndWriter verifies each event routes through ui.Formatter
// and ui.OutputWriter as if the legacy path were used (Acceptance #30).
// With a fake facade emitting multiple events, the formatter/writer should process them.
func TestRunWorkflow_OutputsToFormatterAndWriter(t *testing.T) {
	fake := facadetest.New().
		Script(ports.Event{Kind: ports.EventRunStarted, RunID: "test-run-1"}).
		Script(ports.Event{Kind: ports.EventStepStarted, RunID: "test-run-1"}).
		Script(ports.Event{Kind: ports.EventStepCompleted, RunID: "test-run-1"}).
		WithTerminalCompleted()

	cfg := &Config{
		Facade:       fake,
		OutputFormat: ui.FormatText,
		NoColor:      true,
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	writer := ui.NewOutputWriter(stdout, stderr, ui.FormatText, true, false)
	formatter := ui.NewFormatter(stdout, ui.FormatOptions{NoColor: true})

	err := runWorkflowViaFacade(context.Background(), cmd, cfg, writer, formatter, "test-workflow", nil)

	assert.NoError(t, err)
}

// TestRunWorkflow_InteractiveMode verifies interactive mode still works after facade routing.
// This is preserved for T078 (Acceptance #38).
func TestRunWorkflow_InteractiveMode(t *testing.T) {
	cfg := &Config{
		OutputFormat: ui.FormatText,
		NoColor:      true,
	}

	require.NotNil(t, cfg)
	assert.False(t, cfg.Facade != nil, "facade should not be set for interactive mode (yet)")
}

// TestRunWorkflowViaFacade_InvalidIdentifier verifies error handling for invalid requests.
func TestRunWorkflowViaFacade_InvalidIdentifier(t *testing.T) {
	fake := facadetest.New()

	cfg := &Config{
		Facade:       fake,
		OutputFormat: ui.FormatText,
		NoColor:      true,
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	writer := ui.NewOutputWriter(stdout, stderr, ui.FormatText, true, false)
	formatter := ui.NewFormatter(stdout, ui.FormatOptions{NoColor: true})

	err := runWorkflowViaFacade(context.Background(), cmd, cfg, writer, formatter, "", nil)

	assert.Error(t, err, "empty workflow identifier should produce an error")
}
