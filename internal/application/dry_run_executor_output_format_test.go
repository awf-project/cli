package application_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDryRunExecutor_AgentStep_OutputFormatJSON verifies that agent steps
// with output_format: json are correctly captured in the dry run plan.
func TestDryRunExecutor_AgentStep_OutputFormatJSON(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent_json"] = &workflow.Workflow{
		Name:    "agent_json",
		Initial: "generate",
		Steps: map[string]*workflow.Step{
			"generate": {
				Name: "generate",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Generate a JSON response",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "agent_json", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)

	var agentStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "generate" {
			agentStep = &plan.Steps[i]
			break
		}
	}

	require.NotNil(t, agentStep, "should have agent step")
	require.NotNil(t, agentStep.Agent, "agent step should have agent config")
	assert.Equal(t, workflow.OutputFormatJSON, agentStep.Agent.OutputFormat, "output format should be json")
}

// TestDryRunExecutor_AgentStep_OutputFormatText verifies that agent steps
// with output_format: text are correctly captured in the dry run plan.
func TestDryRunExecutor_AgentStep_OutputFormatText(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent_text"] = &workflow.Workflow{
		Name:    "agent_text",
		Initial: "generate",
		Steps: map[string]*workflow.Step{
			"generate": {
				Name: "generate",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "gemini",
					Prompt:       "Generate a text response",
					OutputFormat: workflow.OutputFormatText,
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "agent_text", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)

	var agentStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "generate" {
			agentStep = &plan.Steps[i]
			break
		}
	}

	require.NotNil(t, agentStep, "should have agent step")
	require.NotNil(t, agentStep.Agent, "agent step should have agent config")
	assert.Equal(t, workflow.OutputFormatText, agentStep.Agent.OutputFormat, "output format should be text")
}

// TestDryRunExecutor_AgentStep_NoOutputFormat verifies that agent steps
// without output_format field show empty/none in the dry run plan (backward compatibility).
func TestDryRunExecutor_AgentStep_NoOutputFormat(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["agent_none"] = &workflow.Workflow{
		Name:    "agent_none",
		Initial: "generate",
		Steps: map[string]*workflow.Step{
			"generate": {
				Name: "generate",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "codex",
					Prompt:   "Generate code",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "agent_none", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)

	var agentStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "generate" {
			agentStep = &plan.Steps[i]
			break
		}
	}

	require.NotNil(t, agentStep, "should have agent step")
	require.NotNil(t, agentStep.Agent, "agent step should have agent config")
	assert.Equal(t, workflow.OutputFormatNone, agentStep.Agent.OutputFormat, "output format should be empty/none")
}

// TestDryRunExecutor_AgentStep_MultipleFormats verifies that different agent steps
// can have different output formats within the same workflow.
func TestDryRunExecutor_AgentStep_MultipleFormats(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["mixed"] = &workflow.Workflow{
		Name:    "mixed",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name: "step1",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Generate JSON",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "step2",
			},
			"step2": {
				Name: "step2",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "gemini",
					Prompt:       "Generate text",
					OutputFormat: workflow.OutputFormatText,
				},
				OnSuccess: "step3",
			},
			"step3": {
				Name: "step3",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "codex",
					Prompt:   "No format",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "mixed", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)

	steps := make(map[string]*workflow.DryRunStep)
	for i := range plan.Steps {
		steps[plan.Steps[i].Name] = &plan.Steps[i]
	}

	require.Contains(t, steps, "step1")
	require.NotNil(t, steps["step1"].Agent)
	assert.Equal(t, workflow.OutputFormatJSON, steps["step1"].Agent.OutputFormat, "step1 should have json format")

	require.Contains(t, steps, "step2")
	require.NotNil(t, steps["step2"].Agent)
	assert.Equal(t, workflow.OutputFormatText, steps["step2"].Agent.OutputFormat, "step2 should have text format")

	require.Contains(t, steps, "step3")
	require.NotNil(t, steps["step3"].Agent)
	assert.Equal(t, workflow.OutputFormatNone, steps["step3"].Agent.OutputFormat, "step3 should have no format")
}

// TestDryRunExecutor_AgentStep_WithConversationMode verifies that output_format
// is captured correctly for agent steps in conversation mode.
func TestDryRunExecutor_AgentStep_WithConversationMode(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["conversation"] = &workflow.Workflow{
		Name:    "conversation",
		Initial: "chat",
		Steps: map[string]*workflow.Step{
			"chat": {
				Name: "chat",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Mode:         "conversation",
					OutputFormat: workflow.OutputFormatJSON,
					Conversation: &workflow.ConversationConfig{},
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "conversation", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)

	var chatStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "chat" {
			chatStep = &plan.Steps[i]
			break
		}
	}

	require.NotNil(t, chatStep, "should have chat step")
	require.NotNil(t, chatStep.Agent, "chat step should have agent config")
	assert.Equal(t, workflow.OutputFormatJSON, chatStep.Agent.OutputFormat, "conversation mode should preserve output format")
}

// TestDryRunExecutor_AgentStep_WithOtherConfigs verifies that output_format
// is correctly displayed alongside other agent configurations (timeout, options, etc.).
func TestDryRunExecutor_AgentStep_WithOtherConfigs(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["full_config"] = &workflow.Workflow{
		Name:    "full_config",
		Initial: "complex",
		Steps: map[string]*workflow.Step{
			"complex": {
				Name: "complex",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Complex task",
					Options: map[string]any{
						"model":       "claude-3-opus",
						"temperature": 0.7,
						"max_tokens":  1000,
					},
					Timeout:      120,
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})

	plan, err := executor.Execute(context.Background(), "full_config", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)

	var complexStep *workflow.DryRunStep
	for i := range plan.Steps {
		if plan.Steps[i].Name == "complex" {
			complexStep = &plan.Steps[i]
			break
		}
	}

	require.NotNil(t, complexStep, "should have complex step")
	require.NotNil(t, complexStep.Agent, "complex step should have agent config")

	assert.Equal(t, "claude", complexStep.Agent.Provider, "provider should be preserved")
	assert.Equal(t, 120, complexStep.Agent.Timeout, "timeout should be preserved")
	assert.NotNil(t, complexStep.Agent.Options, "options should be preserved")
	assert.Equal(t, workflow.OutputFormatJSON, complexStep.Agent.OutputFormat, "output format should be json alongside other configs")
}
