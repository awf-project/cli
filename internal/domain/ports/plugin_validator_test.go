package ports_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T001
// Feature: C069

// mockWorkflowValidatorProvider implements ports.WorkflowValidatorProvider for testing.
type mockWorkflowValidatorProvider struct {
	validateWorkflowResults map[string][]ports.ValidationResult
	validateWorkflowErrors  map[string]error
	validateStepResults     map[string][]ports.ValidationResult
	validateStepErrors      map[string]error
	validateWorkflowCalls   int
	validateStepCalls       int
}

func newMockWorkflowValidatorProvider() *mockWorkflowValidatorProvider {
	return &mockWorkflowValidatorProvider{
		validateWorkflowResults: make(map[string][]ports.ValidationResult),
		validateWorkflowErrors:  make(map[string]error),
		validateStepResults:     make(map[string][]ports.ValidationResult),
		validateStepErrors:      make(map[string]error),
	}
}

func (m *mockWorkflowValidatorProvider) ValidateWorkflow(ctx context.Context, workflowJSON []byte) ([]ports.ValidationResult, error) {
	m.validateWorkflowCalls++
	key := string(workflowJSON)
	if err, ok := m.validateWorkflowErrors[key]; ok {
		return nil, err
	}
	return m.validateWorkflowResults[key], nil
}

func (m *mockWorkflowValidatorProvider) ValidateStep(ctx context.Context, workflowJSON []byte, stepName string) ([]ports.ValidationResult, error) {
	m.validateStepCalls++
	key := stepName
	if err, ok := m.validateStepErrors[key]; ok {
		return nil, err
	}
	return m.validateStepResults[key], nil
}

// mockStepTypeProvider implements ports.StepTypeProvider for testing.
type mockStepTypeProvider struct {
	supportedTypes   map[string]bool
	executeResults   map[string]ports.StepExecuteResult
	executeErrors    map[string]error
	hasStepTypeCalls int
	executeStepCalls int
}

func newMockStepTypeProvider(types ...string) *mockStepTypeProvider {
	m := &mockStepTypeProvider{
		supportedTypes: make(map[string]bool),
		executeResults: make(map[string]ports.StepExecuteResult),
		executeErrors:  make(map[string]error),
	}
	for _, t := range types {
		m.supportedTypes[t] = true
	}
	return m
}

func (m *mockStepTypeProvider) HasStepType(typeName string) bool {
	m.hasStepTypeCalls++
	return m.supportedTypes[typeName]
}

func (m *mockStepTypeProvider) ExecuteStep(ctx context.Context, req ports.StepExecuteRequest) (ports.StepExecuteResult, error) {
	m.executeStepCalls++
	key := req.StepType
	if err, ok := m.executeErrors[key]; ok {
		return ports.StepExecuteResult{}, err
	}
	return m.executeResults[key], nil
}

// Tests for Severity type

func TestSeverity_Constants(t *testing.T) {
	tests := []struct {
		name     string
		severity ports.Severity
		value    int
	}{
		{"SeverityError", ports.SeverityError, 0},
		{"SeverityWarning", ports.SeverityWarning, 1},
		{"SeverityInfo", ports.SeverityInfo, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, ports.Severity(tt.value), tt.severity)
		})
	}
}

func TestSeverity_ZeroValueIsError(t *testing.T) {
	var s ports.Severity
	assert.Equal(t, ports.SeverityError, s)
}

// Tests for ValidationResult type

func TestValidationResult_Creation(t *testing.T) {
	tests := []struct {
		name     string
		result   ports.ValidationResult
		wantErr  bool
		wantWarn bool
		wantInfo bool
	}{
		{
			name: "error result",
			result: ports.ValidationResult{
				Severity: ports.SeverityError,
				Message:  "invalid step",
				Step:     "my-step",
				Field:    "timeout",
			},
			wantErr: true,
		},
		{
			name: "warning result",
			result: ports.ValidationResult{
				Severity: ports.SeverityWarning,
				Message:  "deprecated field",
				Step:     "my-step",
				Field:    "retry",
			},
			wantWarn: true,
		},
		{
			name: "info result",
			result: ports.ValidationResult{
				Severity: ports.SeverityInfo,
				Message:  "optimization tip",
				Step:     "",
				Field:    "",
			},
			wantInfo: true,
		},
		{
			name: "workflow-level result (empty step)",
			result: ports.ValidationResult{
				Severity: ports.SeverityError,
				Message:  "workflow has cycles",
				Step:     "",
				Field:    "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.result.Message)
			if tt.wantErr {
				assert.Equal(t, ports.SeverityError, tt.result.Severity)
			}
			if tt.wantWarn {
				assert.Equal(t, ports.SeverityWarning, tt.result.Severity)
			}
			if tt.wantInfo {
				assert.Equal(t, ports.SeverityInfo, tt.result.Severity)
			}
		})
	}
}

func TestValidationResult_AllowsEmptyStepAndField(t *testing.T) {
	result := ports.ValidationResult{
		Severity: ports.SeverityError,
		Message:  "workflow-level error",
		Step:     "",
		Field:    "",
	}

	assert.Empty(t, result.Step)
	assert.Empty(t, result.Field)
	assert.NotEmpty(t, result.Message)
}

// Tests for StepExecuteRequest type

func TestStepExecuteRequest_Creation(t *testing.T) {
	tests := []struct {
		name string
		req  ports.StepExecuteRequest
	}{
		{
			name: "basic request",
			req: ports.StepExecuteRequest{
				StepName: "fetch-data",
				StepType: "database",
				Config: map[string]any{
					"query": "SELECT * FROM users",
				},
				Inputs: map[string]any{
					"id": "123",
				},
			},
		},
		{
			name: "request with empty config",
			req: ports.StepExecuteRequest{
				StepName: "simple-step",
				StepType: "custom",
				Config:   map[string]any{},
				Inputs:   map[string]any{},
			},
		},
		{
			name: "request with nil maps",
			req: ports.StepExecuteRequest{
				StepName: "basic-step",
				StepType: "compute",
				Config:   nil,
				Inputs:   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.req.StepName)
			assert.NotEmpty(t, tt.req.StepType)
		})
	}
}

func TestStepExecuteRequest_ConfigAndInputsAcceptAny(t *testing.T) {
	req := ports.StepExecuteRequest{
		StepName: "test-step",
		StepType: "generic",
		Config: map[string]any{
			"string":  "value",
			"number":  42,
			"boolean": true,
			"nested": map[string]any{
				"key": "value",
			},
		},
		Inputs: map[string]any{
			"input1": 123,
			"input2": []string{"a", "b"},
		},
	}

	assert.Equal(t, "value", req.Config["string"])
	assert.Equal(t, 42, req.Config["number"])
	assert.Equal(t, true, req.Config["boolean"])
	assert.Equal(t, 123, req.Inputs["input1"])
}

// Tests for StepExecuteResult type

func TestStepExecuteResult_Creation(t *testing.T) {
	tests := []struct {
		name       string
		result     ports.StepExecuteResult
		wantOutput bool
		wantData   bool
	}{
		{
			name: "success result",
			result: ports.StepExecuteResult{
				Output: "Command completed successfully",
				Data: map[string]any{
					"rows": 5,
					"id":   "abc-123",
				},
				ExitCode: 0,
			},
			wantOutput: true,
			wantData:   true,
		},
		{
			name: "error result",
			result: ports.StepExecuteResult{
				Output:   "Error executing command",
				Data:     map[string]any{},
				ExitCode: 1,
			},
			wantOutput: true,
		},
		{
			name: "empty data",
			result: ports.StepExecuteResult{
				Output:   "Done",
				Data:     nil,
				ExitCode: 0,
			},
			wantOutput: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantOutput {
				assert.NotEmpty(t, tt.result.Output)
			}
		})
	}
}

func TestStepExecuteResult_ExitCodeVariations(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
	}{
		{"zero exit", 0},
		{"error exit", 1},
		{"not found", 127},
		{"signal", 130},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ports.StepExecuteResult{
				Output:   "test",
				Data:     map[string]any{},
				ExitCode: tt.exitCode,
			}
			assert.Equal(t, tt.exitCode, result.ExitCode)
		})
	}
}

// Tests for WorkflowValidatorProvider interface

func TestWorkflowValidatorProvider_ValidateWorkflowSuccess(t *testing.T) {
	provider := newMockWorkflowValidatorProvider()
	workflowJSON := []byte(`{"name":"test-workflow"}`)

	expectedResults := []ports.ValidationResult{
		{
			Severity: ports.SeverityWarning,
			Message:  "missing timeout",
			Step:     "step-1",
			Field:    "timeout",
		},
	}
	provider.validateWorkflowResults[string(workflowJSON)] = expectedResults

	ctx := context.Background()
	results, err := provider.ValidateWorkflow(ctx, workflowJSON)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, ports.SeverityWarning, results[0].Severity)
	assert.Equal(t, "missing timeout", results[0].Message)
}

func TestWorkflowValidatorProvider_ValidateWorkflowError(t *testing.T) {
	provider := newMockWorkflowValidatorProvider()
	workflowJSON := []byte(`{"name":"bad-workflow"}`)

	provider.validateWorkflowErrors[string(workflowJSON)] = context.DeadlineExceeded

	ctx := context.Background()
	_, err := provider.ValidateWorkflow(ctx, workflowJSON)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestWorkflowValidatorProvider_ValidateStepSuccess(t *testing.T) {
	provider := newMockWorkflowValidatorProvider()
	workflowJSON := []byte(`{"name":"test"}`)
	stepName := "my-step"

	expectedResults := []ports.ValidationResult{
		{
			Severity: ports.SeverityError,
			Message:  "invalid type",
			Step:     stepName,
			Field:    "type",
		},
	}
	provider.validateStepResults[stepName] = expectedResults

	ctx := context.Background()
	results, err := provider.ValidateStep(ctx, workflowJSON, stepName)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, ports.SeverityError, results[0].Severity)
}

func TestWorkflowValidatorProvider_ValidateStepError(t *testing.T) {
	provider := newMockWorkflowValidatorProvider()
	workflowJSON := []byte(`{"name":"test"}`)
	stepName := "unknown-step"

	provider.validateStepErrors[stepName] = context.Canceled

	ctx := context.Background()
	_, err := provider.ValidateStep(ctx, workflowJSON, stepName)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestWorkflowValidatorProvider_MethodsCalled(t *testing.T) {
	provider := newMockWorkflowValidatorProvider()
	ctx := context.Background()

	provider.ValidateWorkflow(ctx, []byte("{}"))
	provider.ValidateWorkflow(ctx, []byte("{}"))
	provider.ValidateStep(ctx, []byte("{}"), "step-1")

	assert.Equal(t, 2, provider.validateWorkflowCalls)
	assert.Equal(t, 1, provider.validateStepCalls)
}

func TestWorkflowValidatorProvider_EmptyResults(t *testing.T) {
	provider := newMockWorkflowValidatorProvider()
	workflowJSON := []byte(`{"name":"valid"}`)

	provider.validateWorkflowResults[string(workflowJSON)] = []ports.ValidationResult{}

	ctx := context.Background()
	results, err := provider.ValidateWorkflow(ctx, workflowJSON)

	assert.NoError(t, err)
	assert.Empty(t, results)
}

// Tests for StepTypeProvider interface

func TestStepTypeProvider_HasStepTypeSupported(t *testing.T) {
	provider := newMockStepTypeProvider("database", "http", "custom")

	assert.True(t, provider.HasStepType("database"))
	assert.True(t, provider.HasStepType("http"))
	assert.True(t, provider.HasStepType("custom"))
}

func TestStepTypeProvider_HasStepTypeNotSupported(t *testing.T) {
	provider := newMockStepTypeProvider("database")

	assert.False(t, provider.HasStepType("unknown"))
	assert.False(t, provider.HasStepType("http"))
}

func TestStepTypeProvider_HasStepTypeEmpty(t *testing.T) {
	provider := newMockStepTypeProvider()

	assert.False(t, provider.HasStepType("anything"))
}

func TestStepTypeProvider_ExecuteStepSuccess(t *testing.T) {
	provider := newMockStepTypeProvider("database")
	req := ports.StepExecuteRequest{
		StepName: "fetch-users",
		StepType: "database",
		Config: map[string]any{
			"query": "SELECT * FROM users",
		},
		Inputs: map[string]any{},
	}

	expectedResult := ports.StepExecuteResult{
		Output: "3 rows returned",
		Data: map[string]any{
			"count":    3,
			"duration": "150ms",
		},
		ExitCode: 0,
	}
	provider.executeResults["database"] = expectedResult

	ctx := context.Background()
	result, err := provider.ExecuteStep(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, expectedResult.Output, result.Output)
	assert.Equal(t, expectedResult.ExitCode, result.ExitCode)
	assert.Equal(t, 3, result.Data["count"])
}

func TestStepTypeProvider_ExecuteStepError(t *testing.T) {
	provider := newMockStepTypeProvider("http")
	req := ports.StepExecuteRequest{
		StepName: "call-api",
		StepType: "http",
		Config:   map[string]any{},
		Inputs:   map[string]any{},
	}

	provider.executeErrors["http"] = context.DeadlineExceeded

	ctx := context.Background()
	_, err := provider.ExecuteStep(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestStepTypeProvider_ExecuteStepNonZeroExit(t *testing.T) {
	provider := newMockStepTypeProvider("shell")
	req := ports.StepExecuteRequest{
		StepName: "run-script",
		StepType: "shell",
		Config:   map[string]any{},
		Inputs:   map[string]any{},
	}

	expectedResult := ports.StepExecuteResult{
		Output:   "Command failed",
		Data:     map[string]any{},
		ExitCode: 1,
	}
	provider.executeResults["shell"] = expectedResult

	ctx := context.Background()
	result, err := provider.ExecuteStep(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, 1, result.ExitCode)
}

func TestStepTypeProvider_MethodsCalled(t *testing.T) {
	provider := newMockStepTypeProvider("type1", "type2")
	ctx := context.Background()

	provider.HasStepType("type1")
	provider.HasStepType("type1")
	provider.HasStepType("unknown")

	req := ports.StepExecuteRequest{
		StepName: "test",
		StepType: "type1",
	}
	provider.ExecuteStep(ctx, req)

	assert.Equal(t, 3, provider.hasStepTypeCalls)
	assert.Equal(t, 1, provider.executeStepCalls)
}

func TestStepTypeProvider_HasStepTypeIsO1(t *testing.T) {
	provider := newMockStepTypeProvider("type1", "type2", "type3")

	provider.HasStepType("type1")
	provider.HasStepType("type2")
	provider.HasStepType("type3")

	assert.Equal(t, 3, provider.hasStepTypeCalls)
}

// Integration-style tests

func TestValidationResult_Serializable(t *testing.T) {
	result := ports.ValidationResult{
		Severity: ports.SeverityWarning,
		Message:  "test message",
		Step:     "test-step",
		Field:    "timeout",
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var unmarshaled ports.ValidationResult
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, result.Severity, unmarshaled.Severity)
	assert.Equal(t, result.Message, unmarshaled.Message)
}

func TestStepExecuteRequest_JSONRoundtrip(t *testing.T) {
	req := ports.StepExecuteRequest{
		StepName: "my-step",
		StepType: "custom",
		Config: map[string]any{
			"key": "value",
		},
		Inputs: map[string]any{
			"input": 42,
		},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var unmarshaled ports.StepExecuteRequest
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, req.StepName, unmarshaled.StepName)
	assert.Equal(t, req.StepType, unmarshaled.StepType)
}

func TestStepExecuteResult_JSONRoundtrip(t *testing.T) {
	result := ports.StepExecuteResult{
		Output: "success",
		Data: map[string]any{
			"rows": 5,
		},
		ExitCode: 0,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var unmarshaled ports.StepExecuteResult
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, result.Output, unmarshaled.Output)
	assert.Equal(t, result.ExitCode, unmarshaled.ExitCode)
}
