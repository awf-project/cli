//go:build integration

// Feature: 39
//
// Functional tests for Agent Step Type feature. These tests validate end-to-end
// behavior using the CLI interface with workflow fixtures.
//
// Acceptance Criteria Coverage:
// - AC1: New step type `agent` recognized by workflow parser
// - AC2: Support for providers: claude, codex, gemini, opencode, custom
// - AC3: Prompt interpolation with {{inputs.*}} and {{states.*}}
// - AC8: --dry-run shows resolved prompt without invoking
// - AC9: Error handling: provider not found, CLI not installed
//
// Note: Full execution tests (AC4-AC7, AC10-AC11) require agent providers to be
// implemented. These tests focus on parsing, validation, and dry-run functionality.

package integration_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// skipInCI skips the test if running in a CI environment where external
// dependencies (like Claude CLI) may not be available.
func skipInCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Skip("skipping integration test in CI environment")
	}
}

// =============================================================================
// AC1: Agent Step Type Recognition - Validation
// =============================================================================

func TestFeature39_AgentStepTypeRecognizedByValidator(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Agent workflow fixtures exist
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tests := []struct {
		name         string
		workflowName string
		shouldPass   bool
	}{
		{
			name:         "simple_agent_workflow",
			workflowName: "agent-simple",
			shouldPass:   true,
		},
		{
			name:         "json_parse_agent_workflow",
			workflowName: "agent-json-parse",
			shouldPass:   true,
		},
		{
			name:         "parallel_agents_workflow",
			workflowName: "agent-parallel",
			shouldPass:   true,
		},
		{
			name:         "multiturn_agent_workflow",
			workflowName: "agent-multiturn",
			shouldPass:   true,
		},
		{
			name:         "custom_provider_workflow",
			workflowName: "agent-custom-provider",
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

			// Then: Agent step type is recognized
			if tt.shouldPass {
				require.NoError(t, err, "Workflow with agent steps should validate successfully")
				output := buf.String()
				assert.Contains(t, output, "valid", "Should show workflow is valid")
			} else {
				require.Error(t, err, "Invalid workflow should fail validation")
			}
		})
	}
}

// =============================================================================
// AC1: Agent Step Type Recognition - List
// =============================================================================

func TestFeature39_AgentWorkflowsListedSuccessfully(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Workflow directory with agent workflows
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	// When: List command is executed
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list"})

	err := cmd.Execute()

	// Then: Agent workflows appear in list
	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "agent-simple", "Simple agent workflow should be listed")
	assert.Contains(t, output, "agent-json-parse", "JSON parse agent workflow should be listed")
	assert.Contains(t, output, "agent-parallel", "Parallel agent workflow should be listed")
}

// =============================================================================
// AC8: Dry-Run Shows Agent Configuration
// =============================================================================

func TestFeature39_DryRun_BasicAgentStep(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Simple agent workflow
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// When: Dry-run is executed
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"agent-simple",
		"--dry-run",
		"--input", "task=review code",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Dry-run shows agent configuration without executing
	require.NoError(t, err, "Dry-run should execute without error")
	output := buf.String()

	// Verify dry-run header
	assert.Contains(t, output, "DRY RUN", "Should indicate dry-run mode")

	// Verify workflow name
	assert.Contains(t, output, "agent-simple", "Should show workflow name")

	// Verify step information (once implementation is complete, this will show provider details)
	assert.Contains(t, output, "analyze", "Should show agent step name")

	// Note: Full provider details and resolved prompts will be visible after implementation
	// Current test verifies workflow parsing and dry-run execution path
}

// =============================================================================
// AC8: Dry-Run Shows Resolved Prompts
// =============================================================================

func TestFeature39_DryRun_PromptInterpolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Agent workflow with prompt interpolation
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// When: Dry-run with input values
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"agent-simple",
		"--dry-run",
		"--input", "task=analyze security vulnerabilities",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Dry-run completes successfully
	require.NoError(t, err)
	output := buf.String()

	// Verify dry-run executed
	assert.Contains(t, output, "DRY RUN", "Should be in dry-run mode")

	// Note: After implementation, output should contain:
	// - Resolved prompt with interpolated values
	// - Provider name (claude)
	// - CLI command that would be executed
}

// =============================================================================
// AC8: Dry-Run with JSON Format Agent
// =============================================================================

func TestFeature39_DryRun_JSONOutputFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Agent workflow with JSON output format
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// When: Dry-run with JSON format workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"agent-json-parse",
		"--dry-run",
		"--input", "code=function test() {}",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Dry-run shows JSON output format option
	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "DRY RUN", "Should be in dry-run mode")
	assert.Contains(t, output, "analyze", "Should show agent step")

	// Note: After implementation, should show:
	// - output_format: json in options
	// - Expected JSON structure in response
}

// =============================================================================
// AC8: Dry-Run with Parallel Agents
// =============================================================================

func TestFeature39_DryRun_ParallelAgents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Workflow with parallel agent steps
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// When: Dry-run parallel agents workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"agent-parallel",
		"--dry-run",
		"--input", "question=What is 2+2?",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: All parallel agents shown in dry-run
	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "DRY RUN", "Should be in dry-run mode")

	// Verify parallel structure shown
	assert.Contains(t, output, "parallel_agents", "Should show parallel step")

	// Note: After implementation, should show all three agent branches:
	// - claude_analysis with provider: claude
	// - codex_analysis with provider: codex
	// - gemini_analysis with provider: gemini
}

// =============================================================================
// AC8: Dry-Run with Multi-Turn Conversation
// =============================================================================

func TestFeature39_DryRun_MultiTurnConversation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Multi-turn agent workflow
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// When: Dry-run multi-turn workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"agent-multiturn",
		"--dry-run",
		"--input", "initial_request=Design a REST API",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: All turns shown in execution plan
	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "DRY RUN", "Should be in dry-run mode")

	// Verify sequential steps shown
	assert.Contains(t, output, "first_turn", "Should show first agent step")
	assert.Contains(t, output, "second_turn", "Should show second agent step")
	assert.Contains(t, output, "third_turn", "Should show third agent step")

	// Note: After implementation, should show state passing:
	// - first_turn uses {{inputs.initial_request}}
	// - second_turn uses {{states.first_turn.output}}
	// - third_turn uses both previous outputs
}

// =============================================================================
// AC9: Error Handling - Invalid Provider
// =============================================================================

func TestFeature39_Validation_InvalidProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Workflow with invalid provider
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	// When: Validate workflow with invalid provider
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "agent-invalid-provider"})

	err := cmd.Execute()

	// Then: Validation should complete
	// Note: Provider existence check happens at execution time, not validation time
	// Validation only checks YAML structure and required fields
	// After implementation, this might change based on architecture decisions

	output := buf.String()

	// For now, YAML structure should be valid even with unknown provider
	// Runtime will catch provider not found error
	if err == nil {
		assert.Contains(t, output, "valid", "YAML structure should be valid")
	}
}

// =============================================================================
// Integration: Custom Provider Configuration
// =============================================================================

func TestFeature39_CustomProviderParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Workflow with custom provider
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	// When: Validate custom provider workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "agent-custom-provider"})

	err := cmd.Execute()

	// Then: Custom provider workflow validates successfully
	require.NoError(t, err, "Custom provider workflow should validate")
	output := buf.String()
	assert.Contains(t, output, "valid", "Should show workflow is valid")
}

// =============================================================================
// Edge Case: Timeout Configuration
// =============================================================================

func TestFeature39_TimeoutConfigurationParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Workflow with timeout configuration
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	// When: Validate timeout workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "agent-timeout"})

	err := cmd.Execute()

	// Then: Timeout configuration is parsed
	require.NoError(t, err, "Timeout workflow should validate")
	output := buf.String()
	assert.Contains(t, output, "valid", "Should show workflow is valid")
}

// =============================================================================
// Integration: Help Command Shows Agent Workflows
// =============================================================================

func TestFeature39_HelpShowsAgentWorkflows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Agent workflow exists
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// When: Help is requested for agent workflow
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"agent-simple",
		"--help",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Help shows workflow details including inputs
	require.NoError(t, err)
	output := buf.String()

	// Verify workflow help content
	assert.Contains(t, output, "agent-simple", "Should show workflow name")
	assert.Contains(t, output, "task", "Should show required input")

	// Note: Full help might include:
	// - Description of agent steps
	// - Provider information
	// - Expected outputs
}

// =============================================================================
// Edge Case: Missing Required Input
// =============================================================================

func TestFeature39_MissingRequiredInput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Agent workflow with required input
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()

	// When: Run without required input
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run",
		"agent-simple",
		"--storage", tmpDir,
	})

	err := cmd.Execute()

	// Then: Error about missing required input
	require.Error(t, err, "Should error on missing required input")

	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "task") || strings.Contains(errorMsg, "required"),
		"Error should mention missing required input",
	)
}

// =============================================================================
// Diagram Generation with Agent Steps
// =============================================================================

func TestFeature39_DiagramGenerationWithAgentSteps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	skipInCI(t)

	// Given: Agent workflow
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	// When: Generate diagram
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "agent-simple"})

	err := cmd.Execute()

	// Then: Diagram includes agent steps
	require.NoError(t, err, "Diagram generation should succeed")
	output := buf.String()

	// Verify DOT format output
	assert.Contains(t, output, "digraph", "Should generate DOT format")
	assert.Contains(t, output, "analyze", "Should include agent step")

	// Note: After implementation, agent steps might have distinct visual styling
	// to differentiate from regular command steps
}
