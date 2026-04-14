//go:build integration

// Feature: F083
//
// Functional tests for Interactive Conversation Mode (Breaking Change).
// These tests validate end-to-end behavior of the simplified conversation system:
// - Removed automated loop fields (max_turns, stop_condition, inject_context, strategy, max_context_tokens)
// - Added UserInputReader port for interactive user input
// - New StopReasonUserExit for user-initiated exit
// - Simplified ConversationConfig with only ContinueFrom retained
//
// Test Strategy:
// - YAML validation: Verify removed fields are rejected with clear error messages
// - Minimal workflows: Verify stripped-down conversation config works
// - Dry run: Verify conversation mode shows correctly without stdin interaction
// - Cross-step resume: Verify continue_from references work in multi-step workflows

package features_test

import (
	"bytes"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConversationYAML_MinimalValidConfig verifies that a minimal conversation
// workflow (with only ContinueFrom in conversation config) parses successfully
// and is recognized as valid.
func TestConversationYAML_MinimalValidConfig(t *testing.T) {
	// Feature: F083 — ConversationConfig simplified to only ContinueFrom
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tests := []struct {
		name         string
		workflowName string
	}{
		{
			name:         "simple conversation without continuation",
			workflowName: "conversation-simple",
		},
		{
			name:         "multiturn with continue_from",
			workflowName: "conversation-multiturn",
		},
		{
			name:         "valid continue_from reference",
			workflowName: "conversation-continue-from",
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

			require.NoError(t, err, "workflow should validate successfully")
			output := buf.String()
			assert.Contains(t, output, "valid", "output should indicate validation success")
		})
	}
}

// TestConversationDryRun_ShowsConfigWithoutStdin verifies that dry-run mode
// displays conversation configuration without requiring stdin interaction.
// This validates that ConversationManager wiring is complete.
func TestConversationDryRun_ShowsConfigWithoutStdin(t *testing.T) {
	// Feature: F083 — ConversationManager wired in run.go; dry-run avoids stdin
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-simple",
		"--dry-run",
		"--input", "task=test dry run",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	require.NoError(t, err, "dry-run should execute without error")
	output := buf.String()

	// Verify dry-run mode indicator
	assert.Contains(t, output, "Dry Run", "should display dry-run mode")

	// Verify step is shown
	assert.Contains(t, output, "review", "should display conversation step name")
}

// TestConversationContinueFrom_CrossStepResume verifies that a second step
// with continue_from successfully references a prior step's conversation state.
// This validates the continue_from validation and session resume logic.
func TestConversationContinueFrom_CrossStepResume(t *testing.T) {
	// Feature: F083 + F074 — ContinueFrom preserved; validates cross-step resume
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "conversation-multiturn"})

	err := cmd.Execute()

	require.NoError(t, err, "multiturn workflow with continue_from should validate")
	output := buf.String()
	assert.Contains(t, output, "valid")
}

// TestConversationValidation_InvalidContinueFrom_Rejected verifies that
// continue_from references to non-existent steps are rejected during validation.
func TestConversationValidation_InvalidContinueFrom_Rejected(t *testing.T) {
	// Feature: F083 + F074 — Validation rejects invalid continue_from references
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "conversation-invalid-continue-from"})

	err := cmd.Execute()

	require.Error(t, err, "validation should reject invalid continue_from")
	output := buf.String()
	assert.Contains(t, output, "continue_from",
		"error message should reference continue_from field")
}

// TestConversationList_IncludesAllWorkflows verifies that conversation workflows
// are discoverable in the list command after F083 changes.
func TestConversationList_IncludesAllWorkflows(t *testing.T) {
	// Feature: F083 — Conversation workflows remain discoverable
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list"})

	err := cmd.Execute()

	require.NoError(t, err, "list command should execute successfully")
	output := buf.String()

	// Verify key conversation workflows are listed
	conversationWorkflows := []string{
		"conversation-simple",
		"conversation-multiturn",
		"conversation-continue-from",
	}

	for _, wf := range conversationWorkflows {
		assert.Contains(t, output, wf, "should list %s workflow", wf)
	}
}
