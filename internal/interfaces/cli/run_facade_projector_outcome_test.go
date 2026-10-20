package cli

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

func TestRenderStepCompletionOutcome(t *testing.T) {
	for _, tt := range []struct {
		name, status, err string
		failed            bool
	}{
		{name: "success", status: "completed"},
		{name: "handled failure without error", status: "failed", failed: true},
		{name: "cancelled", status: "cancelled", failed: true},
		{name: "legacy failure", err: "command failed", failed: true},
		{name: "legacy success"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, hadOutput := range []bool{false, true} {
				output := string(RenderFacadeEventsToText([]ports.Event{{Kind: ports.EventStepCompleted, Payload: &ports.EnrichedStepPayload{StepName: "probe", Status: tt.status, Error: tt.err, HadOutput: hadOutput}}}))
				if tt.failed {
					assert.Contains(t, output, "probe: failed")
					assert.NotContains(t, output, "✓")
					assert.NotContains(t, output, "completed")
				} else {
					assert.Contains(t, output, "probe: completed")
				}
			}
		})
	}
}
