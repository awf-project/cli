//go:build integration

// Feature: C062, F074

package features_test

import (
	"bytes"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationValidation_RejectsUnimplementedFeatures(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tests := []struct {
		name         string
		workflowName string
		wantErr      string
	}{
		{
			name:         "summarize_strategy_rejected",
			workflowName: "conversation-invalid-summarize",
			wantErr:      "not yet implemented",
		},
		{
			name:         "truncate_middle_strategy_rejected",
			workflowName: "conversation-invalid-truncate-middle",
			wantErr:      "not yet implemented",
		},
		{
			name:         "continue_from_invalid_step_reference",
			workflowName: "conversation-invalid-continue-from",
			wantErr:      "continue_from references unknown step",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"validate", tt.workflowName})

			err := cmd.Execute()

			require.Error(t, err)
			output := buf.String()
			assert.Contains(t, output, tt.wantErr,
				"validation output should contain rejection message")
		})
	}
}

func TestConversationValidation_AcceptsValidConfigs(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tests := []struct {
		name         string
		workflowName string
	}{
		{
			name:         "sliding_window_strategy",
			workflowName: "conversation-window",
		},
		{
			name:         "no_strategy",
			workflowName: "conversation-simple",
		},
		{
			name:         "multiturn_with_continue_from",
			workflowName: "conversation-multiturn",
		},
		{
			name:         "continue_from_valid_step_reference",
			workflowName: "conversation-continue-from",
		},
		{
			name:         "inject_context_valid",
			workflowName: "conversation-inject-context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"validate", tt.workflowName})

			err := cmd.Execute()

			require.NoError(t, err)
			assert.Contains(t, buf.String(), "valid")
		})
	}
}
