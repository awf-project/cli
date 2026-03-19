//go:build integration

// Feature: C062

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
			name:         "continue_from_rejected",
			workflowName: "conversation-invalid-continue-from",
			wantErr:      "continue_from is not yet implemented",
		},
		{
			name:         "inject_context_rejected",
			workflowName: "conversation-invalid-inject-context",
			wantErr:      "inject_context is not yet implemented",
		},
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
			name:         "multiturn_without_continue_from",
			workflowName: "conversation-multiturn",
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
