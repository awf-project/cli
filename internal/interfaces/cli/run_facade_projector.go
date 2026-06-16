package cli

import (
	"bytes"
	"fmt"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
)

// facadeRenderOptions controls quiet/verbose-sensitive rendering of facade events.
// The zero value renders in the default (non-quiet) mode used by the conformance golden.
type facadeRenderOptions struct {
	Quiet bool
}

// RenderFacadeEventsToText converts facade events to CLI text output in the default
// (non-quiet) mode. This is the renderer the facade conformance golden exercises; the live
// run driver uses RenderFacadeEventsToTextWithOptions to honor --quiet.
func RenderFacadeEventsToText(events []ports.Event) []byte {
	return RenderFacadeEventsToTextWithOptions(events, facadeRenderOptions{})
}

// RenderFacadeEventsToTextWithOptions converts facade events to CLI text output, honoring
// the supplied options (e.g. --quiet). It restores the legacy phrasing so downstream tooling
// and users that grep "Workflow ID:" / "completed successfully" keep working:
//   - EventRunStarted prints both a human "Workflow started:" line and the legacy
//     "Workflow ID: <run-id>" line (suppressed in quiet mode via Info).
//   - EventStepCompleted prints the legacy "<step>: completed successfully" feedback line for
//     a silent step (no stdout/stderr), suppressed in quiet mode; a step that already printed
//     output gets a plain completion marker instead (its output is the feedback).
func RenderFacadeEventsToTextWithOptions(events []ports.Event, opts facadeRenderOptions) []byte { //nolint:gocritic // hugeParam: opts is a tiny value struct; pointer indirection adds no benefit
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf, ui.FormatOptions{NoColor: true, Quiet: opts.Quiet})
	for _, ev := range events {
		switch ev.Kind {
		case ports.EventRunStarted:
			// Info is suppressed in quiet mode (legacy parity).
			f.Info(fmt.Sprintf("Workflow started: %s", ev.RunID))
			f.Info(fmt.Sprintf("Workflow ID: %s", ev.RunID))
		case ports.EventStepStarted:
			if step, ok := ev.Payload.(*ports.EnrichedStepPayload); ok && step != nil {
				f.Info(fmt.Sprintf("  → %s", step.StepName))
			}
		case ports.EventStepCompleted:
			if step, ok := ev.Payload.(*ports.EnrichedStepPayload); ok && step != nil {
				if step.HadOutput {
					// The step already printed its output out-of-band; just mark completion.
					f.Success(fmt.Sprintf("  ✓ %s: completed", step.StepName))
				} else {
					// Silent step: restore the legacy F037 success feedback (hidden in quiet).
					f.StepSuccess(step.StepName)
				}
			} else {
				f.Success("Step completed.")
			}
		case ports.EventMessageAssistant:
			if msg, ok := ev.Payload.(*ports.EnrichedMessagePayload); ok && msg != nil {
				f.Println(msg.Content)
			}
		case ports.EventWorkflowCompleted:
			f.Success("Workflow completed.")
		case ports.EventWorkflowFailed:
			if term, ok := ev.Payload.(*ports.EnrichedTerminal); ok && term != nil {
				f.Error(fmt.Sprintf("Workflow failed: %s", term.Error))
			} else {
				f.Error("Workflow failed.")
			}
		case ports.EventKindUnknown,
			ports.EventRunCompleted,
			ports.EventStepCallWorkflowStarted,
			ports.EventStepCallWorkflowCompleted,
			ports.EventMessageUser,
			ports.EventToolCall,
			ports.EventToolResult,
			ports.EventInputRequired:
			// no visible output for these kinds in default (non-verbose) mode
		}
	}
	return buf.Bytes()
}
