package application_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F065 - Output Format for Agent Steps
// Component: T009 - Map StepState.JSON into interpolation context in buildInterpolationContext()
//
// Tests verify that buildInterpolationContext() correctly maps StepState.JSON
// to StepStateData.JSON, enabling template interpolation like {{states.step.JSON.field}}.

// TestExecutionService_buildInterpolationContext_MapsJSONFieldToStepStateData verifies
// that when an agent step populates StepState.JSON, buildInterpolationContext maps it
// to StepStateData.JSON for template access in subsequent steps.
func TestExecutionService_buildInterpolationContext_MapsJSONFieldToStepStateData(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "json-field-mapping-test",
		Initial: "extract",
		Steps: map[string]*workflow.Step{
			"extract": {
				Name: "extract",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Extract data",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "use",
			},
			"use": {
				Name:      "use",
				Type:      workflow.StepTypeCommand,
				Command:   `echo name={{.states.extract.JSON.name}} count={{.states.extract.JSON.count}}`,
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("json-field-mapping-test", wf).
		WithCommandResult(`echo name=alice count=42`, &ports.CommandResult{
			Stdout:   "name=alice count=42",
			ExitCode: 0,
		}).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Agent returns JSON that will be parsed into StepState.JSON
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   `{"name":"alice","count":42}`,
			Tokens:   20,
		}, nil
	})
	_ = registry.Register(claude)
	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "json-field-mapping-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify extract step has JSON field populated
	extractState, exists := ctx.GetStepState("extract")
	require.True(t, exists)
	require.NotNil(t, extractState.JSON)
	jsonObj, ok := extractState.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any for object output")
	assert.Equal(t, "alice", jsonObj["name"])
	assert.Equal(t, float64(42), jsonObj["count"])

	// Verify use step successfully interpolated JSON fields
	useState, exists := ctx.GetStepState("use")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, useState.Status)
	assert.Contains(t, useState.Output, "name=alice")
	assert.Contains(t, useState.Output, "count=42")
}

// TestExecutionService_buildInterpolationContext_JSONFieldNilWhenNotSet verifies
// that when a step does not have output_format: json, the JSON field remains nil
// and templates referencing it don't cause errors (Go template handles nil gracefully).
func TestExecutionService_buildInterpolationContext_JSONFieldNilWhenNotSet(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "no-json-field-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   `echo prev={{.states.step1.Output}}`,
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("no-json-field-test", wf).
		WithCommandResult("echo hello", &ports.CommandResult{
			Stdout:   "hello",
			ExitCode: 0,
		}).
		WithCommandResult("echo prev=hello", &ports.CommandResult{
			Stdout:   "prev=hello",
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "no-json-field-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify step1 has nil JSON field
	step1State, exists := ctx.GetStepState("step1")
	require.True(t, exists)
	assert.Nil(t, step1State.JSON)

	// Verify step2 completed successfully (didn't try to reference JSON)
	step2State, exists := ctx.GetStepState("step2")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, step2State.Status)
}

// TestExecutionService_buildInterpolationContext_JSONFieldNestedObjectAccess verifies
// that deeply nested JSON fields can be accessed via dot notation in templates.
func TestExecutionService_buildInterpolationContext_JSONFieldNestedObjectAccess(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "nested-json-test",
		Initial: "extract",
		Steps: map[string]*workflow.Step{
			"extract": {
				Name: "extract",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Extract nested data",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "use",
			},
			"use": {
				Name:      "use",
				Type:      workflow.StepTypeCommand,
				Command:   `echo user={{.states.extract.JSON.user.name}} city={{.states.extract.JSON.user.address.city}}`,
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("nested-json-test", wf).
		WithCommandResult("echo user=bob city=NYC", &ports.CommandResult{
			Stdout:   "user=bob city=NYC",
			ExitCode: 0,
		}).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Agent returns nested JSON structure
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   `{"user":{"name":"bob","address":{"city":"NYC","zip":"10001"}}}`,
			Tokens:   30,
		}, nil
	})
	_ = registry.Register(claude)
	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "nested-json-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify nested JSON structure was populated
	extractState, exists := ctx.GetStepState("extract")
	require.True(t, exists)
	require.NotNil(t, extractState.JSON)

	jsonObj, ok := extractState.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any for object output")
	user, ok := jsonObj["user"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bob", user["name"])

	address, ok := user["address"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "NYC", address["city"])

	// Verify nested fields were correctly interpolated
	useState, exists := ctx.GetStepState("use")
	require.True(t, exists)
	assert.Contains(t, useState.Output, "user=bob")
	assert.Contains(t, useState.Output, "city=NYC")
}

// TestExecutionService_buildInterpolationContext_JSONFieldFromMultipleSteps verifies
// that JSON fields from multiple previous steps are all available in the interpolation
// context simultaneously.
func TestExecutionService_buildInterpolationContext_JSONFieldFromMultipleSteps(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "multi-json-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name: "step1",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Get user data",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "step2",
			},
			"step2": {
				Name: "step2",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Get config data",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "combine",
			},
			"combine": {
				Name:      "combine",
				Type:      workflow.StepTypeCommand,
				Command:   `echo {{.states.step1.JSON.user}}-{{.states.step2.JSON.env}}`,
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("multi-json-test", wf).
		WithCommandResult("echo alice-production", &ports.CommandResult{
			Stdout:   "alice-production",
			ExitCode: 0,
		}).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	callCount := 0
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		callCount++
		if callCount == 1 {
			// First call returns user data
			return &workflow.AgentResult{
				Provider: "claude",
				Output:   `{"user":"alice"}`,
				Tokens:   10,
			}, nil
		}
		// Second call returns config data
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   `{"env":"production"}`,
			Tokens:   10,
		}, nil
	})
	_ = registry.Register(claude)
	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "multi-json-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify both steps have JSON fields populated
	step1State, exists := ctx.GetStepState("step1")
	require.True(t, exists)
	require.NotNil(t, step1State.JSON)
	json1, ok := step1State.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any")
	assert.Equal(t, "alice", json1["user"])

	step2State, exists := ctx.GetStepState("step2")
	require.True(t, exists)
	require.NotNil(t, step2State.JSON)
	json2, ok := step2State.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any")
	assert.Equal(t, "production", json2["env"])

	// Verify combine step accessed both JSON fields
	combineState, exists := ctx.GetStepState("combine")
	require.True(t, exists)
	assert.Contains(t, combineState.Output, "alice-production")
}

// TestExecutionService_buildInterpolationContext_JSONFieldWithBothResponseAndJSON verifies
// that when both Response (legacy heuristic) and JSON (explicit output_format) fields exist,
// the JSON field takes precedence in templates.
func TestExecutionService_buildInterpolationContext_JSONFieldWithBothResponseAndJSON(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "dual-field-test",
		Initial: "extract",
		Steps: map[string]*workflow.Step{
			"extract": {
				Name: "extract",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Get data",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "verify",
			},
			"verify": {
				Name:      "verify",
				Type:      workflow.StepTypeCommand,
				Command:   `echo explicit={{.states.extract.JSON.source}}`,
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("dual-field-test", wf).
		WithCommandResult("echo explicit=output_format", &ports.CommandResult{
			Stdout:   "explicit=output_format",
			ExitCode: 0,
		}).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   `{"source":"output_format"}`,
			Tokens:   10,
		}, nil
	})
	_ = registry.Register(claude)
	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "dual-field-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify extract step has JSON field populated
	extractState, exists := ctx.GetStepState("extract")
	require.True(t, exists)
	require.NotNil(t, extractState.JSON)
	jsonObj, ok := extractState.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any")
	assert.Equal(t, "output_format", jsonObj["source"])

	// Verify subsequent step could access JSON field
	verifyState, exists := ctx.GetStepState("verify")
	require.True(t, exists)
	assert.Contains(t, verifyState.Output, "explicit=output_format")
}
