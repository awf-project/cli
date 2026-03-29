package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validatorPlugin implements Validator for testing.
type validatorPlugin struct {
	BasePlugin
	workflowIssues []ValidationIssue
	stepIssues     []ValidationIssue
	workflowErr    error
	stepErr        error
}

func (p *validatorPlugin) Init(_ context.Context, _ map[string]any) error {
	return nil
}

func (p *validatorPlugin) Shutdown(_ context.Context) error {
	return nil
}

//nolint:gocritic // interface requirement - test implementation
func (p *validatorPlugin) ValidateWorkflow(_ context.Context, _ WorkflowDefinition) ([]ValidationIssue, error) {
	return p.workflowIssues, p.workflowErr
}

//nolint:gocritic // interface requirement - test implementation
func (p *validatorPlugin) ValidateStep(_ context.Context, _ WorkflowDefinition, _ string) ([]ValidationIssue, error) {
	return p.stepIssues, p.stepErr
}

// TestSeverityConstants verifies severity constants have expected values.
func TestSeverityConstants(t *testing.T) {
	assert.Equal(t, Severity(0), SeverityError)
	assert.Equal(t, Severity(1), SeverityWarning)
	assert.Equal(t, Severity(2), SeverityInfo)
}

// TestValidationIssue_Fields verifies ValidationIssue struct can hold all fields.
func TestValidationIssue_Fields(t *testing.T) {
	issue := ValidationIssue{
		Severity: SeverityWarning,
		Message:  "test message",
		Step:     "test-step",
		Field:    "test-field",
	}

	assert.Equal(t, SeverityWarning, issue.Severity)
	assert.Equal(t, "test message", issue.Message)
	assert.Equal(t, "test-step", issue.Step)
	assert.Equal(t, "test-field", issue.Field)
}

// TestWorkflowDefinition_JSONMarshaling verifies WorkflowDefinition can be marshaled/unmarshaled.
func TestWorkflowDefinition_JSONMarshaling(t *testing.T) {
	original := WorkflowDefinition{
		Name:        "test-workflow",
		Description: "A test workflow",
		Version:     "1.0.0",
		Author:      "test-author",
		Tags:        []string{"test", "demo"},
		Initial:     "step1",
		Steps: map[string]StepDefinition{
			"step1": {
				Type:        "command",
				Command:     "echo hello",
				Description: "A simple echo step",
			},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var unmarshaled WorkflowDefinition
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, original.Name, unmarshaled.Name)
	assert.Equal(t, original.Version, unmarshaled.Version)
	assert.Equal(t, original.Initial, unmarshaled.Initial)
	assert.Len(t, unmarshaled.Steps, 1)
}

// TestStepDefinition_JSONMarshaling verifies StepDefinition can be marshaled/unmarshaled.
func TestStepDefinition_JSONMarshaling(t *testing.T) {
	original := StepDefinition{
		Type:        "agent",
		Description: "An agent step",
		Operation:   "test.operation",
		Timeout:     30,
		OnSuccess:   "step2",
		OnFailure:   "error-handler",
		Config: map[string]any{
			"model": "gpt-4",
			"temp":  0.7,
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var unmarshaled StepDefinition
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, original.Type, unmarshaled.Type)
	assert.Equal(t, original.Operation, unmarshaled.Operation)
	assert.Equal(t, original.Timeout, unmarshaled.Timeout)
	assert.Equal(t, original.Config["model"], unmarshaled.Config["model"])
}

// TestValidatorServiceServer_ValidateWorkflow_PluginImplementsValidator verifies ValidateWorkflow calls validator.
func TestValidatorServiceServer_ValidateWorkflow_PluginImplementsValidator(t *testing.T) {
	plugin := &validatorPlugin{
		BasePlugin: BasePlugin{
			PluginName:    "validator-plugin",
			PluginVersion: "1.0.0",
		},
		workflowIssues: []ValidationIssue{
			{Severity: SeverityError, Message: "Invalid workflow"},
		},
	}

	server := &validatorServiceServer{impl: plugin}

	workflow := WorkflowDefinition{
		Name:    "test",
		Initial: "step1",
		Steps:   map[string]StepDefinition{},
	}
	workflowJSON, _ := json.Marshal(workflow)

	resp, err := server.ValidateWorkflow(context.Background(), &pluginv1.ValidateWorkflowRequest{
		WorkflowJson: workflowJSON,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Issues, 1)
	assert.Equal(t, "Invalid workflow", resp.Issues[0].Message)
}

// TestValidatorServiceServer_ValidateWorkflow_PluginDoesNotImplementValidator verifies empty response when not implemented.
func TestValidatorServiceServer_ValidateWorkflow_PluginDoesNotImplementValidator(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "non-validator-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &validatorServiceServer{impl: plugin}

	workflow := WorkflowDefinition{
		Name:    "test",
		Initial: "step1",
		Steps:   map[string]StepDefinition{},
	}
	workflowJSON, _ := json.Marshal(workflow)

	resp, err := server.ValidateWorkflow(context.Background(), &pluginv1.ValidateWorkflowRequest{
		WorkflowJson: workflowJSON,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Issues)
}

// TestValidatorServiceServer_ValidateWorkflow_InvalidJSON verifies error on unmarshal failure.
func TestValidatorServiceServer_ValidateWorkflow_InvalidJSON(t *testing.T) {
	plugin := &validatorPlugin{
		BasePlugin: BasePlugin{
			PluginName:    "validator-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &validatorServiceServer{impl: plugin}

	resp, err := server.ValidateWorkflow(context.Background(), &pluginv1.ValidateWorkflowRequest{
		WorkflowJson: []byte(`{invalid json}`),
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "unmarshal workflow")
}

// TestValidatorServiceServer_ValidateWorkflow_ValidatorReturnsError verifies error propagation.
func TestValidatorServiceServer_ValidateWorkflow_ValidatorReturnsError(t *testing.T) {
	testErr := errors.New("validation check failed")
	plugin := &validatorPlugin{
		BasePlugin: BasePlugin{
			PluginName:    "validator-plugin",
			PluginVersion: "1.0.0",
		},
		workflowErr: testErr,
	}

	server := &validatorServiceServer{impl: plugin}

	workflow := WorkflowDefinition{
		Name:    "test",
		Initial: "step1",
		Steps:   map[string]StepDefinition{},
	}
	workflowJSON, _ := json.Marshal(workflow)

	resp, err := server.ValidateWorkflow(context.Background(), &pluginv1.ValidateWorkflowRequest{
		WorkflowJson: workflowJSON,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, testErr)
}

// TestValidatorServiceServer_ValidateStep_PluginImplementsValidator verifies ValidateStep calls validator.
func TestValidatorServiceServer_ValidateStep_PluginImplementsValidator(t *testing.T) {
	plugin := &validatorPlugin{
		BasePlugin: BasePlugin{
			PluginName:    "validator-plugin",
			PluginVersion: "1.0.0",
		},
		stepIssues: []ValidationIssue{
			{Severity: SeverityWarning, Message: "Step timeout too long", Step: "step1"},
		},
	}

	server := &validatorServiceServer{impl: plugin}

	workflow := WorkflowDefinition{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]StepDefinition{
			"step1": {Type: "command", Command: "echo"},
		},
	}
	workflowJSON, _ := json.Marshal(workflow)

	resp, err := server.ValidateStep(context.Background(), &pluginv1.ValidateStepRequest{
		WorkflowJson: workflowJSON,
		StepName:     "step1",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Issues, 1)
	assert.Equal(t, "Step timeout too long", resp.Issues[0].Message)
	assert.Equal(t, "step1", resp.Issues[0].Step)
}

// TestValidatorServiceServer_ValidateStep_PluginDoesNotImplementValidator verifies empty response when not implemented.
func TestValidatorServiceServer_ValidateStep_PluginDoesNotImplementValidator(t *testing.T) {
	plugin := &testPlugin{
		BasePlugin{
			PluginName:    "non-validator-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &validatorServiceServer{impl: plugin}

	workflow := WorkflowDefinition{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]StepDefinition{
			"step1": {Type: "command"},
		},
	}
	workflowJSON, _ := json.Marshal(workflow)

	resp, err := server.ValidateStep(context.Background(), &pluginv1.ValidateStepRequest{
		WorkflowJson: workflowJSON,
		StepName:     "step1",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Issues)
}

// TestValidatorServiceServer_ValidateStep_InvalidJSON verifies error on unmarshal failure.
func TestValidatorServiceServer_ValidateStep_InvalidJSON(t *testing.T) {
	plugin := &validatorPlugin{
		BasePlugin: BasePlugin{
			PluginName:    "validator-plugin",
			PluginVersion: "1.0.0",
		},
	}

	server := &validatorServiceServer{impl: plugin}

	resp, err := server.ValidateStep(context.Background(), &pluginv1.ValidateStepRequest{
		WorkflowJson: []byte(`bad json`),
		StepName:     "step1",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "unmarshal workflow")
}

// TestValidatorServiceServer_ValidateStep_ValidatorReturnsError verifies error propagation.
func TestValidatorServiceServer_ValidateStep_ValidatorReturnsError(t *testing.T) {
	testErr := errors.New("step validation failed")
	plugin := &validatorPlugin{
		BasePlugin: BasePlugin{
			PluginName:    "validator-plugin",
			PluginVersion: "1.0.0",
		},
		stepErr: testErr,
	}

	server := &validatorServiceServer{impl: plugin}

	workflow := WorkflowDefinition{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]StepDefinition{
			"step1": {Type: "command"},
		},
	}
	workflowJSON, _ := json.Marshal(workflow)

	resp, err := server.ValidateStep(context.Background(), &pluginv1.ValidateStepRequest{
		WorkflowJson: workflowJSON,
		StepName:     "step1",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, testErr)
}

// TestToProtoIssues_Empty verifies conversion of empty issues list.
func TestToProtoIssues_Empty(t *testing.T) {
	result := toProtoIssues([]ValidationIssue{})

	assert.Empty(t, result)
}

// TestToProtoIssues_MultipleIssues verifies conversion of multiple issues.
func TestToProtoIssues_MultipleIssues(t *testing.T) {
	issues := []ValidationIssue{
		{Severity: SeverityError, Message: "Error 1", Step: "step1", Field: "type"},
		{Severity: SeverityWarning, Message: "Warning 1", Step: "step2", Field: "timeout"},
		{Severity: SeverityInfo, Message: "Info 1", Step: "", Field: ""},
	}

	result := toProtoIssues(issues)

	require.Len(t, result, 3)
	assert.Equal(t, pluginv1.Severity_SEVERITY_ERROR, result[0].Severity)
	assert.Equal(t, "Error 1", result[0].Message)
	assert.Equal(t, "step1", result[0].Step)
	assert.Equal(t, "type", result[0].Field)

	assert.Equal(t, pluginv1.Severity_SEVERITY_WARNING, result[1].Severity)
	assert.Equal(t, "step2", result[1].Step)

	assert.Equal(t, pluginv1.Severity_SEVERITY_INFO, result[2].Severity)
}

// TestToProtoIssues_PreservesAllFields verifies all fields are preserved in conversion.
func TestToProtoIssues_PreservesAllFields(t *testing.T) {
	issues := []ValidationIssue{
		{
			Severity: SeverityWarning,
			Message:  "Complex message with special chars: @#$%",
			Step:     "step-with-dash",
			Field:    "field_with_underscore",
		},
	}

	result := toProtoIssues(issues)

	require.Len(t, result, 1)
	assert.Equal(t, "Complex message with special chars: @#$%", result[0].Message)
	assert.Equal(t, "step-with-dash", result[0].Step)
	assert.Equal(t, "field_with_underscore", result[0].Field)
}

// TestSeverityToProto_Error verifies SeverityError maps correctly.
func TestSeverityToProto_Error(t *testing.T) {
	result := severityToProto(SeverityError)
	assert.Equal(t, pluginv1.Severity_SEVERITY_ERROR, result)
}

// TestSeverityToProto_Warning verifies SeverityWarning maps correctly.
func TestSeverityToProto_Warning(t *testing.T) {
	result := severityToProto(SeverityWarning)
	assert.Equal(t, pluginv1.Severity_SEVERITY_WARNING, result)
}

// TestSeverityToProto_Info verifies SeverityInfo maps correctly.
func TestSeverityToProto_Info(t *testing.T) {
	result := severityToProto(SeverityInfo)
	assert.Equal(t, pluginv1.Severity_SEVERITY_INFO, result)
}

// TestSeverityToProto_Unknown verifies unknown severity maps to ERROR.
func TestSeverityToProto_Unknown(t *testing.T) {
	result := severityToProto(Severity(99))
	assert.Equal(t, pluginv1.Severity_SEVERITY_ERROR, result)
}

// TestSeverityToProto_ZeroValue verifies zero value maps to ERROR.
func TestSeverityToProto_ZeroValue(t *testing.T) {
	var severity Severity
	result := severityToProto(severity)
	assert.Equal(t, pluginv1.Severity_SEVERITY_ERROR, result)
}

// TestValidatorServiceServer_ValidateWorkflow_WithMultipleSeverities verifies issue severity preservation.
func TestValidatorServiceServer_ValidateWorkflow_WithMultipleSeverities(t *testing.T) {
	plugin := &validatorPlugin{
		BasePlugin: BasePlugin{
			PluginName:    "validator-plugin",
			PluginVersion: "1.0.0",
		},
		workflowIssues: []ValidationIssue{
			{Severity: SeverityError, Message: "Critical issue"},
			{Severity: SeverityWarning, Message: "Non-critical issue"},
			{Severity: SeverityInfo, Message: "FYI message"},
		},
	}

	server := &validatorServiceServer{impl: plugin}

	workflow := WorkflowDefinition{
		Name:    "test",
		Initial: "step1",
		Steps:   map[string]StepDefinition{},
	}
	workflowJSON, _ := json.Marshal(workflow)

	resp, err := server.ValidateWorkflow(context.Background(), &pluginv1.ValidateWorkflowRequest{
		WorkflowJson: workflowJSON,
	})

	require.NoError(t, err)
	require.Len(t, resp.Issues, 3)
	assert.Equal(t, pluginv1.Severity_SEVERITY_ERROR, resp.Issues[0].Severity)
	assert.Equal(t, pluginv1.Severity_SEVERITY_WARNING, resp.Issues[1].Severity)
	assert.Equal(t, pluginv1.Severity_SEVERITY_INFO, resp.Issues[2].Severity)
}
