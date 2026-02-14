//go:build integration

// Feature: 33
//
// Integration tests for Agent Conversation feature (F033). These tests validate
// end-to-end conversation workflow behavior using CLI interface with workflow fixtures.
//
// Acceptance Criteria Coverage:
// - AC1: New mode: conversation option for agent steps
// - AC2: Conversation history maintained in step state
// - AC3: Automatic context window management with configurable strategy
// - AC4: Token counting for supported providers
// - AC5: System prompt preserved across truncations
// - AC6: max_turns limit to prevent infinite loops
// - AC7: stop_condition expression to exit conversation early
// - AC8: Conversation state accessible via {{states.step.conversation}}
// - AC9: Support for injecting context mid-conversation
// - AC10: Works with streaming output (F029)
//
// Test Strategy:
// - CLI invocation via awf binary
// - State structure verification (conversation field populated)
// - Token accounting validation (input + output tokens tracked)
// - Parallel conversation execution
// - Context window truncation verification
// - Stop condition triggering
// - Max turns limit enforcement

package features_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
	"github.com/vanoix/awf/tests/integration/testhelpers"
)

// Note: skipInCI helper is defined in agent_test.go to avoid duplication

func TestConversationModeRecognizedByValidator(t *testing.T) {
	// CI-enabled: Only validates YAML syntax, no external API calls required

	// Given: Conversation workflow fixtures exist
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tests := []struct {
		name         string
		workflowName string
		shouldPass   bool
	}{
		{
			name:         "simple_conversation_workflow",
			workflowName: "conversation-simple",
			shouldPass:   true,
		},
		{
			name:         "multiturn_conversation_workflow",
			workflowName: "conversation-multiturn",
			shouldPass:   true,
		},
		{
			name:         "context_window_conversation_workflow",
			workflowName: "conversation-window",
			shouldPass:   true,
		},
		{
			name:         "stop_condition_conversation_workflow",
			workflowName: "conversation-stop-condition",
			shouldPass:   true,
		},
		{
			name:         "max_turns_conversation_workflow",
			workflowName: "conversation-max-turns",
			shouldPass:   true,
		},
		{
			name:         "parallel_conversation_workflow",
			workflowName: "conversation-parallel",
			shouldPass:   true,
		},
		{
			name:         "error_handling_conversation_workflow",
			workflowName: "conversation-error",
			shouldPass:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: Workflow is validated
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"validate", tt.workflowName})

			err := cmd.Execute()

			// Then: Conversation mode is recognized
			if tt.shouldPass {
				require.NoError(t, err)
				output := buf.String()
				assert.Contains(t, output, "valid")
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestConversationWorkflowsListedSuccessfully(t *testing.T) {
	// CI-enabled: Only lists workflow files, no external API calls required

	// Given: Workflow directory with conversation workflows
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	// When: List command is executed
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list"})

	err := cmd.Execute()

	// Then: Conversation workflows appear in list
	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "conversation-simple")
	assert.Contains(t, output, "conversation-multiturn")
	assert.Contains(t, output, "conversation-window")
}

func TestBasicConversation_SimpleWorkflow(t *testing.T) {
	// Skip in CI: Requires real Claude API provider with ANTHROPIC_API_KEY and billable API calls
	testhelpers.SkipInCI(t)

	// Given: Simple conversation workflow with stop condition
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute conversation workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-simple",
		"--input", "task=review this code",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Conversation executes successfully
	require.NoError(t, err)

	// Verify state file created
	stateFiles, err := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, stateFiles, "Should create state file")

	// Read state and verify conversation structure
	stateData, err := os.ReadFile(stateFiles[0])
	require.NoError(t, err)

	var state map[string]interface{}
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	// Verify conversation state exists
	states, ok := state["states"].(map[string]interface{})
	require.True(t, ok, "Should have states field")

	reviewStep, ok := states["review"].(map[string]interface{})
	require.True(t, ok, "Should have review step state")

	// AC2: Conversation history maintained in step state
	conversation, ok := reviewStep["conversation"].(map[string]interface{})
	require.True(t, ok, "Should have conversation field in step state")

	turns, ok := conversation["turns"].([]interface{})
	require.True(t, ok, "Should have turns array")
	assert.NotEmpty(t, turns, "Should have at least one turn")
}

func TestDryRun_ConversationConfiguration(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Conversation workflow with full configuration
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Dry-run conversation workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-simple",
		"--dry-run",
		"--input", "task=analyze code",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Dry-run shows conversation configuration
	require.NoError(t, err, "Dry-run should execute without error")
	output := buf.String()

	// Verify dry-run mode
	assert.Contains(t, output, "DRY RUN", "Should indicate dry-run mode")

	// Verify step shown
	assert.Contains(t, output, "review", "Should show conversation step name")

	// Note: After implementation, should show:
	// - mode: conversation
	// - max_turns configuration
	// - stop_condition expression
	// - context_window strategy
}

func TestMaxTurns_MultiTurnWorkflow(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Multi-turn conversation workflow with max_turns limit
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute multi-turn conversation
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-multiturn",
		"--input", "initial_request=design api",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Conversation respects max_turns limit
	require.NoError(t, err, "Multi-turn conversation should execute")

	// Read state and verify turn count
	stateFiles, err := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, stateFiles)

	stateData, err := os.ReadFile(stateFiles[0])
	require.NoError(t, err)

	var state map[string]interface{}
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	states := state["states"].(map[string]interface{})
	firstTurn := states["first_turn"].(map[string]interface{})
	conversation := firstTurn["conversation"].(map[string]interface{})

	// Verify total turns tracked
	totalTurns, ok := conversation["total_turns"].(float64)
	require.True(t, ok, "Should track total_turns")
	assert.Greater(t, totalTurns, 0.0, "Should have executed turns")

	// Verify stopped_by field
	stoppedBy, ok := conversation["stopped_by"].(string)
	require.True(t, ok, "Should have stopped_by field")
	assert.NotEmpty(t, stoppedBy, "Should indicate why conversation stopped")
}

func TestContextWindow_TruncationPreservesSystemPrompt(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Conversation workflow with context window limit
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute conversation that exceeds context window
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-window",
		"--input", "task=review large codebase",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Context window truncation applied
	require.NoError(t, err, "Conversation with window management should execute")

	// Read state and verify context window state
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
	conversation := reviewStep["conversation"].(map[string]interface{})

	// Verify turns exist
	turns, ok := conversation["turns"].([]interface{})
	require.True(t, ok, "Should have turns")
	require.NotEmpty(t, turns, "Should have at least one turn")

	// AC5: System prompt should be first turn and preserved
	firstTurn := turns[0].(map[string]interface{})
	role, ok := firstTurn["role"].(string)
	require.True(t, ok, "First turn should have role")
	assert.Equal(t, "system", role, "First turn should be system prompt")

	// Verify context window state tracked
	contextWindowState, ok := reviewStep["context_window_state"].(map[string]interface{})
	if ok {
		// Should track truncation events
		truncated, _ := contextWindowState["truncated"].(bool)
		strategy, _ := contextWindowState["strategy"].(string)

		if truncated {
			assert.NotEmpty(t, strategy, "Should indicate truncation strategy used")
		}
	}
}

func TestTokenCounting_InputOutputTracking(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Conversation workflow
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute conversation
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-simple",
		"--input", "task=count tokens",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Token usage tracked
	require.NoError(t, err)

	// Read state and verify token tracking
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

	// AC4: Verify tokens tracked
	tokens, ok := reviewStep["tokens"].(map[string]interface{})
	require.True(t, ok, "Should have tokens field")

	inputTokens, ok := tokens["input"].(float64)
	require.True(t, ok, "Should track input tokens")
	assert.Greater(t, inputTokens, 0.0, "Should have input tokens")

	outputTokens, ok := tokens["output"].(float64)
	require.True(t, ok, "Should track output tokens")
	assert.Greater(t, outputTokens, 0.0, "Should have output tokens")

	totalTokens, ok := tokens["total"].(float64)
	require.True(t, ok, "Should track total tokens")
	assert.Equal(t, inputTokens+outputTokens, totalTokens, "Total should equal input + output")

	// Verify per-turn token tracking
	conversation := reviewStep["conversation"].(map[string]interface{})
	turns := conversation["turns"].([]interface{})

	for i, turn := range turns {
		turnMap := turn.(map[string]interface{})
		turnTokens, ok := turnMap["tokens"].(float64)
		require.True(t, ok, "Turn %d should have token count", i)
		assert.Greater(t, turnTokens, 0.0, "Turn %d should have positive tokens", i)
	}
}

func TestStopCondition_ExpressionEvaluation(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Conversation workflow with stop condition expression
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

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

	// Then: Stop condition triggers correctly
	require.NoError(t, err, "Conversation should stop when condition met")

	// Read state and verify stopped_by
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
	conversation := reviewStep["conversation"].(map[string]interface{})

	// AC7: Verify stopped by condition
	stoppedBy, ok := conversation["stopped_by"].(string)
	require.True(t, ok, "Should have stopped_by field")
	assert.Equal(t, "condition", stoppedBy, "Should stop due to condition")

	// Verify output exists
	_, hasOutput := reviewStep["output"]
	require.True(t, hasOutput, "Should have output")

	// Note: After implementation, should verify stop condition expression matched
	// For now, just verify conversation stopped early (not max_turns)
	totalTurns := conversation["total_turns"].(float64)
	assert.Greater(t, totalTurns, 0.0, "Should have executed some turns")
}

func TestMaxTurns_BoundaryEnforcement(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Conversation workflow with max_turns=3
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute conversation that would exceed max_turns
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-max-turns",
		"--input", "task=iterate many times",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Conversation stops at max_turns
	require.NoError(t, err, "Should stop gracefully at max_turns")

	// Read state and verify turn limit
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
	conversation := reviewStep["conversation"].(map[string]interface{})

	// AC6: Verify stopped by max_turns
	stoppedBy, ok := conversation["stopped_by"].(string)
	require.True(t, ok, "Should have stopped_by field")
	assert.Equal(t, "max_turns", stoppedBy, "Should stop due to max_turns")

	// Verify turn count at or below max_turns
	totalTurns := conversation["total_turns"].(float64)
	assert.LessOrEqual(t, totalTurns, 3.0, "Should not exceed max_turns=3")
}

func TestInjectContext_ContinueConversation(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Multi-turn workflow with continue_from
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute workflow with conversation continuation
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-multiturn",
		"--input", "initial_request=design api",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Second step continues first conversation
	require.NoError(t, err, "Conversation continuation should work")

	// Read state and verify continuation
	stateFiles, err := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, stateFiles)

	stateData, err := os.ReadFile(stateFiles[0])
	require.NoError(t, err)

	var state map[string]interface{}
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	states := state["states"].(map[string]interface{})

	// Verify first_turn has conversation
	firstTurn := states["first_turn"].(map[string]interface{})
	firstConv := firstTurn["conversation"].(map[string]interface{})
	firstTurns := firstConv["turns"].([]interface{})
	firstTurnCount := len(firstTurns)

	// Verify second_turn continues conversation (if it exists)
	if secondTurn, ok := states["second_turn"].(map[string]interface{}); ok {
		secondConv := secondTurn["conversation"].(map[string]interface{})
		secondTurns := secondConv["turns"].([]interface{})

		// AC9: Second conversation should include previous turns
		assert.GreaterOrEqual(t, len(secondTurns), firstTurnCount,
			"Continued conversation should include previous turns")
	}
}

func TestStateInterpolation_ConversationAccess(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Multi-step workflow accessing conversation state
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute workflow with state interpolation
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-multiturn",
		"--input", "initial_request=test",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Conversation state accessible via {{states.step.conversation}}
	require.NoError(t, err)

	// Verify state structure allows interpolation
	stateFiles, err := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, stateFiles)

	stateData, err := os.ReadFile(stateFiles[0])
	require.NoError(t, err)

	var state map[string]interface{}
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	states := state["states"].(map[string]interface{})
	firstTurn := states["first_turn"].(map[string]interface{})

	// AC8: Verify conversation field accessible for interpolation
	_, hasConversation := firstTurn["conversation"]
	assert.True(t, hasConversation, "Conversation state should be accessible")

	_, hasOutput := firstTurn["output"]
	assert.True(t, hasOutput, "Output should be accessible")

	_, hasTokens := firstTurn["tokens"]
	assert.True(t, hasTokens, "Tokens should be accessible")
}

func TestParallelConversations_ConcurrentExecution(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Workflow with parallel conversation steps
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute parallel conversations
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-parallel",
		"--input", "question=test parallel execution",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: All parallel conversations execute
	require.NoError(t, err, "Parallel conversations should execute")

	// Read state and verify all branches
	stateFiles, err := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, stateFiles)

	stateData, err := os.ReadFile(stateFiles[0])
	require.NoError(t, err)

	var state map[string]interface{}
	err = json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	states := state["states"].(map[string]interface{})

	// Verify all three parallel conversations executed
	claudeAnalysis, hasClaudeAnalysis := states["claude_analysis"]
	require.True(t, hasClaudeAnalysis, "Should have claude_analysis branch")
	claudeConv := claudeAnalysis.(map[string]interface{})["conversation"]
	assert.NotNil(t, claudeConv, "Claude branch should have conversation")

	codexAnalysis, hasCodexAnalysis := states["codex_analysis"]
	require.True(t, hasCodexAnalysis, "Should have codex_analysis branch")
	codexConv := codexAnalysis.(map[string]interface{})["conversation"]
	assert.NotNil(t, codexConv, "Codex branch should have conversation")

	geminiAnalysis, hasGeminiAnalysis := states["gemini_analysis"]
	require.True(t, hasGeminiAnalysis, "Should have gemini_analysis branch")
	geminiConv := geminiAnalysis.(map[string]interface{})["conversation"]
	assert.NotNil(t, geminiConv, "Gemini branch should have conversation")
}

func TestErrorHandling_ConversationErrors(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Conversation workflow with error handling
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute conversation that may encounter errors
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-error",
		"--input", "task=test error handling",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Error handling works correctly
	// Note: May succeed or fail depending on error scenario
	// The important part is that the workflow handles errors gracefully

	// Read state (if exists) and verify error handling
	stateFiles, _ := filepath.Glob(filepath.Join(tmpDir, "states", "*.json"))

	if len(stateFiles) > 0 {
		stateData, err := os.ReadFile(stateFiles[0])
		require.NoError(t, err)

		var state map[string]interface{}
		err = json.Unmarshal(stateData, &state)
		require.NoError(t, err)

		// Verify workflow status indicates error handling occurred
		status := state["status"].(string)
		assert.Contains(t, []string{"completed", "failed"}, status,
			"Workflow should complete or fail gracefully")

		// Verify error output provides useful information
		output := buf.String()
		if err != nil {
			assert.NotEmpty(t, output, "Should provide error information")
		}
	} else if err != nil {
		// If no state file, verify error output
		output := buf.String()
		assert.NotEmpty(t, output, "Should provide error information")
	}
}

func TestEdgeCase_EmptyConversationConfig(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: Agent step with mode: conversation but minimal config
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute with default conversation settings
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-simple",
		"--input", "task=test defaults",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Should use reasonable defaults
	require.NoError(t, err, "Should work with default conversation config")

	// Verify defaults applied
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
	conversation := reviewStep["conversation"].(map[string]interface{})

	// Should have conversation even with minimal config
	assert.NotNil(t, conversation, "Should create conversation with defaults")
}

func TestDiagramGeneration_ConversationSteps(t *testing.T) {
	// CI-enabled: Only generates DOT diagram from YAML, no external API calls required

	// Given: Conversation workflow
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	// When: Generate diagram
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "conversation-simple"})

	err := cmd.Execute()

	// Then: Diagram includes conversation steps
	require.NoError(t, err, "Diagram generation should succeed")
	output := buf.String()

	// Verify DOT format output
	assert.Contains(t, output, "digraph", "Should generate DOT format")
	assert.Contains(t, output, "review", "Should include conversation step")

	// Note: After implementation, conversation steps might have distinct visual styling
}

func TestHelpCommand_ConversationWorkflows(t *testing.T) {
	// CI-enabled: Only displays workflow help from YAML, no external API calls required

	// Given: Conversation workflow exists
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Help requested for conversation workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"conversation-simple",
		"--help",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Help shows workflow details
	require.NoError(t, err)
	output := buf.String()

	// Verify help shows workflow input parameters
	assert.Contains(t, output, "Input Parameters", "Should show input parameters section")
	assert.Contains(t, output, "task", "Should show required input")

	// Note: After implementation, help might include:
	// - Conversation mode indication
	// - Max turns configuration
	// - Stop condition description
}

func TestBackwardsCompatibility_StatelessMode(t *testing.T) {
	// Skip in CI: Requires real AI provider (claude/gemini/codex) with billable API calls
	testhelpers.SkipInCI(t)

	// Given: F039 agent workflow (stateless mode, no conversation config)
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

	tmpDir := t.TempDir()

	// When: Execute F039 stateless agent workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"agent-simple",
		"--input", "task=test backwards compatibility",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: F039 workflows still work (no conversation field required)
	require.NoError(t, err, "Stateless agent workflows should still work")

	// Read state and verify no conversation field
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

	// Stateless mode should NOT have conversation field
	_, hasConversation := analyzeStep["conversation"]
	assert.False(t, hasConversation, "Stateless mode should not create conversation field")
}

func TestMultiTurnConversation_NoEmptyPromptError(t *testing.T) {
	// Skip in CI: Requires real Claude API provider
	testhelpers.SkipInCI(t)

	// Given: Multi-turn conversation workflow (conversation-multiturn.yaml)
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")
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

func TestExecuteConversationStep_DelegatesToConversationManager(t *testing.T) {
	// Skip in CI: Requires real Claude API provider
	testhelpers.SkipInCI(t)

	// Given: Simple conversation workflow
	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")
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

func TestAllConversationFixtures_ExecuteSuccessfully(t *testing.T) {
	// Skip in CI: Requires real Claude API provider
	testhelpers.SkipInCI(t)

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

	t.Setenv("AWF_WORKFLOWS_PATH", "../../fixtures/workflows")

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
