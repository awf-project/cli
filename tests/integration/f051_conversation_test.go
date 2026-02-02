//go:build integration

// Feature: F051
// Component: T012
//
// Integration tests for F051 conversation workflow fixes. These tests validate
// the empty prompt bug fix and executeConversationStep implementation using CLI
// interface with workflow fixtures.
//
// F051 Acceptance Criteria Coverage:
// - Multi-turn conversations complete without "prompt cannot be empty" errors
// - executeConversationStep correctly delegates to ConversationManager
// - Conversation state persisted correctly via ExecutionService
// - All conversation workflow fixtures execute successfully
//
// Test Strategy:
// - CLI invocation via awf binary with real workflow fixtures
// - State verification for multi-turn scenarios
// - Error case validation (stop conditions, max turns)
// - Backward compatibility with single-mode agent steps

package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// =============================================================================
// F051:T012 - Multi-Turn Conversation Without Empty Prompt Error
// =============================================================================

func TestF051_MultiTurnConversation_NoEmptyPromptError(t *testing.T) {
	// Skip in CI: Requires real Claude API provider
	skipInCI(t)

	// Given: Multi-turn conversation workflow (conversation-multiturn.yaml)
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	tmpDir := t.TempDir()

	// When: Execute multi-turn workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-multiturn",
		"--input", "initial_request=test multi-turn flow",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Should complete without "prompt cannot be empty" error
	require.NoError(t, err, "Multi-turn conversation should execute successfully")

	// Verify output does not contain "prompt cannot be empty" error
	output := buf.String()
	assert.NotContains(t, output, "prompt cannot be empty", "Should not encounter empty prompt error")

	// Verify state shows multiple turns completed
	stateFiles, err := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, stateFiles, "Should create state file")

	stateData, err := os.ReadFile(stateFiles[0])
	require.NoError(t, err)

	var state map[string]interface{}
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	// Verify conversation completed multiple turns
	states := state["states"].(map[string]interface{})
	firstTurn := states["first_turn"].(map[string]interface{})
	conversation := firstTurn["conversation"].(map[string]interface{})

	totalTurns := conversation["total_turns"].(float64)
	assert.Greater(t, totalTurns, 1.0, "Should complete more than 1 turn")
}

// =============================================================================
// F051:T012 - ExecuteConversationStep Delegation
// =============================================================================

func TestF051_ExecuteConversationStep_DelegatesToConversationManager(t *testing.T) {
	// Skip in CI: Requires real Claude API provider
	skipInCI(t)

	// Given: Simple conversation workflow
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	tmpDir := t.TempDir()

	// When: Execute conversation via ExecutionService
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-simple",
		"--input", "task=test delegation",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: ExecutionService delegates to ConversationManager
	require.NoError(t, err, "Conversation should execute via delegation")

	// Verify state structure matches ConversationManager output
	stateFiles, err := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, stateFiles)

	stateData, err := os.ReadFile(stateFiles[0])
	require.NoError(t, err)

	var state map[string]interface{}
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	states := state["states"].(map[string]interface{})
	reviewStep := states["review"].(map[string]interface{})

	// Verify conversation field exists (delegated from ConversationManager)
	conversation, ok := reviewStep["conversation"].(map[string]interface{})
	require.True(t, ok, "Should have conversation field from ConversationManager")
	assert.NotNil(t, conversation, "Conversation should be populated")

	// Verify conversation has required fields from ConversationManager
	_, hasTurns := conversation["turns"]
	assert.True(t, hasTurns, "Conversation should have turns from ConversationManager")

	_, hasStoppedBy := conversation["stopped_by"]
	assert.True(t, hasStoppedBy, "Conversation should have stopped_by from ConversationManager")
}

// =============================================================================
// F051:T012 - All Conversation Fixtures Execute Successfully
// =============================================================================

func TestF051_AllConversationFixtures_ExecuteSuccessfully(t *testing.T) {
	// Skip in CI: Requires real Claude API provider
	skipInCI(t)

	fixtures := []struct {
		name         string
		workflow     string
		input        map[string]string
		shouldPass   bool
		expectedStop string // "max_turns", "condition", or ""
		stepName     string // Step name to check for conversation state
	}{
		{
			name:         "simple_conversation",
			workflow:     "conversation-simple",
			input:        map[string]string{"task": "hello"},
			shouldPass:   true,
			expectedStop: "condition",
			stepName:     "review",
		},
		{
			name:         "multiturn_conversation",
			workflow:     "conversation-multiturn",
			input:        map[string]string{"initial_request": "test"},
			shouldPass:   true,
			expectedStop: "max_turns",
			stepName:     "first_turn",
		},
		{
			name:         "context_window_management",
			workflow:     "conversation-window",
			input:        map[string]string{"task": "review"},
			shouldPass:   true,
			expectedStop: "condition",
			stepName:     "review",
		},
		{
			name:         "max_turns_limit",
			workflow:     "conversation-max-turns",
			input:        map[string]string{"task": "iterate"},
			shouldPass:   true,
			expectedStop: "max_turns",
			stepName:     "single_turn",
		},
		{
			name:         "parallel_conversations",
			workflow:     "conversation-parallel",
			input:        map[string]string{"question": "test"},
			shouldPass:   true,
			expectedStop: "",
			stepName:     "parallel_conversations",
		},
		{
			name:         "error_handling",
			workflow:     "conversation-error",
			input:        map[string]string{"task": "test errors"},
			shouldPass:   false, // Expected to fail at handle_failure step
			expectedStop: "",
			stepName:     "conversation_with_retry",
		},
	}

	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Build CLI args
			args := []string{
				"run",
				tc.workflow,
				"--storage", tmpDir,
			}
			for k, v := range tc.input {
				args = append(args, "--input", k+"="+v)
			}

			// Execute workflow
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(args)

			err := cmd.Execute()
			output := buf.String()

			if tc.shouldPass {
				require.NoError(t, err, "Workflow %s should complete successfully", tc.workflow)
				assert.NotContains(t, output, "prompt cannot be empty", "Should not have empty prompt error")

				// Verify state file
				stateFiles, err := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))
				require.NoError(t, err)
				require.NotEmpty(t, stateFiles, "Should create state file for %s", tc.workflow)

				// Verify stopped_by field matches expected
				if tc.expectedStop != "" {
					stateData, err := os.ReadFile(stateFiles[0])
					require.NoError(t, err)

					var state map[string]interface{}
					err = json.Unmarshal(stateData, &state)
					require.NoError(t, err)

					// Check conversation state based on workflow structure
					states := state["states"].(map[string]interface{})
					if step, ok := states[tc.stepName].(map[string]interface{}); ok {
						if conversation, ok := step["conversation"].(map[string]interface{}); ok {
							stoppedBy, _ := conversation["stopped_by"].(string)
							assert.Equal(t, tc.expectedStop, stoppedBy,
								"Workflow %s should stop by %s", tc.workflow, tc.expectedStop)
						}
					}
				}
			} else {
				// Error workflows may fail gracefully
				if err != nil {
					assert.NotEmpty(t, output, "Should provide error details")
				}
			}
		})
	}
}

// =============================================================================
// F051:T012 - Backward Compatibility with Single Mode
// =============================================================================

func TestF051_BackwardCompatibility_SingleModeStillWorks(t *testing.T) {
	// Skip in CI: Requires real Claude API provider
	skipInCI(t)

	// Given: Agent workflow without conversation mode (F039 style)
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	tmpDir := t.TempDir()

	// When: Execute single-mode agent workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"agent-simple",
		"--input", "task=test backward compatibility",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: F039 workflows still work (no conversation field)
	require.NoError(t, err, "Single-mode agent workflows should still work")

	// Verify no conversation field in state
	stateFiles, err := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, stateFiles)

	stateData, err := os.ReadFile(stateFiles[0])
	require.NoError(t, err)

	var state map[string]interface{}
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	states := state["states"].(map[string]interface{})
	analyzeStep := states["analyze"].(map[string]interface{})

	// Single mode should NOT have conversation field
	_, hasConversation := analyzeStep["conversation"]
	assert.False(t, hasConversation, "Single mode should not create conversation field")

	// Verify single mode has output as expected
	output, hasOutput := analyzeStep["output"]
	assert.True(t, hasOutput, "Single mode should have output field")
	assert.NotNil(t, output, "Single mode output should be populated")
}

// =============================================================================
// F051:T012 - Stop Condition Expression Evaluation
// =============================================================================

func TestF051_StopCondition_EvaluatesCorrectly(t *testing.T) {
	// Skip in CI: Requires real Claude API provider
	skipInCI(t)

	// Given: Conversation with stop condition
	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	tmpDir := t.TempDir()

	// When: Execute conversation with stop condition
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-stop-condition",
		"--input", "task=iterate until approved",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Conversation stops when condition met
	require.NoError(t, err, "Should stop when condition is met")

	// Verify stopped_by field
	stateFiles, err := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, stateFiles)

	stateData, err := os.ReadFile(stateFiles[0])
	require.NoError(t, err)

	var state map[string]interface{}
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	states := state["states"].(map[string]interface{})

	// Note: conversation-stop-condition.yaml may use different step name
	// Check for keyword_match or review step
	var conversation map[string]interface{}
	if keywordMatch, ok := states["keyword_match"].(map[string]interface{}); ok {
		conversation = keywordMatch["conversation"].(map[string]interface{})
	} else if reviewStep, ok := states["review"].(map[string]interface{}); ok {
		conversation = reviewStep["conversation"].(map[string]interface{})
	} else {
		t.Fatal("Could not find conversation step in state")
	}

	stoppedBy := conversation["stopped_by"].(string)
	assert.Equal(t, "condition", stoppedBy, "Should stop due to condition, not max_turns")

	// Verify conversation didn't reach max_turns
	totalTurns := conversation["total_turns"].(float64)
	assert.Greater(t, totalTurns, 0.0, "Should have executed at least one turn")
}
