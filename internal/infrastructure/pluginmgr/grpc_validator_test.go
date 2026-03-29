package pluginmgr

import (
	"context"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc" // grpc.CallOption for mock interface
)

// TestNewGRPCValidatorAdapter_HappyPath tests successful construction with valid timeout.
func TestNewGRPCValidatorAdapter_HappyPath(t *testing.T) {
	mockClient := new(MockValidatorServiceClient)
	timeout := 3 * time.Second

	adapter := newGRPCValidatorAdapter(mockClient, "test-plugin", timeout)

	require.NotNil(t, adapter)
	assert.Equal(t, "test-plugin", adapter.pluginName)
	assert.Equal(t, 3*time.Second, adapter.timeout)
	assert.Equal(t, mockClient, adapter.client)
}

// TestNewGRPCValidatorAdapter_DefaultTimeout tests that zero timeout is replaced with default.
func TestNewGRPCValidatorAdapter_DefaultTimeout(t *testing.T) {
	mockClient := new(MockValidatorServiceClient)

	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"zero timeout", 0},
		{"negative timeout", -1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := newGRPCValidatorAdapter(mockClient, "plugin", tt.timeout)

			require.NotNil(t, adapter)
			assert.Equal(t, defaultValidatorTimeout, adapter.timeout)
		})
	}
}

// TestValidateWorkflow_HappyPath tests successful workflow validation with results.
func TestValidateWorkflow_HappyPath(t *testing.T) {
	mockClient := new(MockValidatorServiceClient)
	workflowJSON := []byte(`{"name":"test"}`)

	protoIssues := []*pluginv1.ValidationIssue{
		{
			Severity: pluginv1.Severity_SEVERITY_WARNING,
			Message:  "step timeout not set",
			Step:     "step1",
			Field:    "timeout",
		},
		{
			Severity: pluginv1.Severity_SEVERITY_ERROR,
			Message:  "invalid workflow",
			Step:     "",
			Field:    "",
		},
	}

	mockClient.On("ValidateWorkflow", mock.Anything, &pluginv1.ValidateWorkflowRequest{
		WorkflowJson: workflowJSON,
	}).Return(&pluginv1.ValidateWorkflowResponse{
		Issues: protoIssues,
	}, nil)

	adapter := newGRPCValidatorAdapter(mockClient, "test-plugin", 5*time.Second)
	results, err := adapter.ValidateWorkflow(context.Background(), workflowJSON)

	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, ports.SeverityWarning, results[0].Severity)
	assert.Equal(t, "step timeout not set", results[0].Message)
	assert.Equal(t, "step1", results[0].Step)
	assert.Equal(t, "timeout", results[0].Field)

	assert.Equal(t, ports.SeverityError, results[1].Severity)
	assert.Equal(t, "invalid workflow", results[1].Message)
}

// TestValidateWorkflow_EmptyResults tests workflow validation with no issues.
func TestValidateWorkflow_EmptyResults(t *testing.T) {
	mockClient := new(MockValidatorServiceClient)
	workflowJSON := []byte(`{"name":"valid-workflow"}`)

	mockClient.On("ValidateWorkflow", mock.Anything, &pluginv1.ValidateWorkflowRequest{
		WorkflowJson: workflowJSON,
	}).Return(&pluginv1.ValidateWorkflowResponse{
		Issues: []*pluginv1.ValidationIssue{},
	}, nil)

	adapter := newGRPCValidatorAdapter(mockClient, "test-plugin", 5*time.Second)
	results, err := adapter.ValidateWorkflow(context.Background(), workflowJSON)

	require.NoError(t, err)
	require.Len(t, results, 0)
}

// TestValidateWorkflow_ClientError tests handling of gRPC client errors.
func TestValidateWorkflow_ClientError(t *testing.T) {
	mockClient := new(MockValidatorServiceClient)
	workflowJSON := []byte(`{}`)

	mockClient.On("ValidateWorkflow", mock.Anything, mock.Anything).
		Return(nil, context.DeadlineExceeded)

	adapter := newGRPCValidatorAdapter(mockClient, "test-plugin", 5*time.Second)
	results, err := adapter.ValidateWorkflow(context.Background(), workflowJSON)

	assert.Error(t, err)
	assert.Nil(t, results)
}

// TestValidateStep_HappyPath tests successful step validation with results.
func TestValidateStep_HappyPath(t *testing.T) {
	mockClient := new(MockValidatorServiceClient)
	workflowJSON := []byte(`{"name":"test","steps":[{"name":"step1"}]}`)
	stepName := "step1"

	protoIssues := []*pluginv1.ValidationIssue{
		{
			Severity: pluginv1.Severity_SEVERITY_WARNING,
			Message:  "missing description",
			Step:     "step1",
			Field:    "description",
		},
	}

	mockClient.On("ValidateStep", mock.Anything, &pluginv1.ValidateStepRequest{
		WorkflowJson: workflowJSON,
		StepName:     stepName,
	}).Return(&pluginv1.ValidateStepResponse{
		Issues: protoIssues,
	}, nil)

	adapter := newGRPCValidatorAdapter(mockClient, "test-plugin", 5*time.Second)
	results, err := adapter.ValidateStep(context.Background(), workflowJSON, stepName)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "missing description", results[0].Message)
}

// TestValidateStep_ClientError tests handling of validation errors.
func TestValidateStep_ClientError(t *testing.T) {
	mockClient := new(MockValidatorServiceClient)
	workflowJSON := []byte(`{}`)

	mockClient.On("ValidateStep", mock.Anything, mock.Anything).
		Return(nil, context.Canceled)

	adapter := newGRPCValidatorAdapter(mockClient, "test-plugin", 5*time.Second)
	results, err := adapter.ValidateStep(context.Background(), workflowJSON, "unknown-step")

	assert.Error(t, err)
	assert.Nil(t, results)
}

// TestMapProtoSeverity_AllValues tests severity enum conversion for all proto values.
func TestMapProtoSeverity_AllValues(t *testing.T) {
	tests := []struct {
		name     string
		protoSev pluginv1.Severity
		wantDom  ports.Severity
	}{
		{"UNSPECIFIED maps to ERROR", pluginv1.Severity_SEVERITY_UNSPECIFIED, ports.SeverityError},
		{"WARNING maps to WARNING", pluginv1.Severity_SEVERITY_WARNING, ports.SeverityWarning},
		{"ERROR maps to ERROR", pluginv1.Severity_SEVERITY_ERROR, ports.SeverityError},
		{"INFO maps to INFO", pluginv1.Severity_SEVERITY_INFO, ports.SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapProtoSeverity(tt.protoSev)
			assert.Equal(t, tt.wantDom, result)
		})
	}
}

// TestMapProtoSeverity_UnknownValue tests handling of unknown severity values.
func TestMapProtoSeverity_UnknownValue(t *testing.T) {
	// Default case in switch should handle unknown values safely
	// Directly test with an enum value that might not be defined
	result := mapProtoSeverity(pluginv1.Severity(999))

	assert.Equal(t, ports.SeverityError, result)
}

// TestDeduplicationKey_Struct verifies deduplicationKey type definition.
func TestDeduplicationKey_Struct(t *testing.T) {
	key1 := deduplicationKey{
		message: "timeout exceeded",
		step:    "step1",
		field:   "timeout",
	}

	key2 := deduplicationKey{
		message: "timeout exceeded",
		step:    "step1",
		field:   "timeout",
	}

	// Identical tuples should be equal
	assert.Equal(t, key1, key2)
}

// TestDeduplicationKey_DifferentValues tests deduplication key differentiation.
func TestDeduplicationKey_DifferentValues(t *testing.T) {
	key1 := deduplicationKey{
		message: "timeout exceeded",
		step:    "step1",
		field:   "timeout",
	}

	tests := []struct {
		name string
		key  deduplicationKey
	}{
		{
			"different message",
			deduplicationKey{message: "different", step: "step1", field: "timeout"},
		},
		{
			"different step",
			deduplicationKey{message: "timeout exceeded", step: "step2", field: "timeout"},
		},
		{
			"different field",
			deduplicationKey{message: "timeout exceeded", step: "step1", field: "other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEqual(t, key1, tt.key)
		})
	}
}

// MockValidatorServiceClient is a testify mock for ValidatorServiceClient.
type MockValidatorServiceClient struct {
	mock.Mock
}

func (m *MockValidatorServiceClient) ValidateWorkflow(ctx context.Context, in *pluginv1.ValidateWorkflowRequest, opts ...grpc.CallOption) (*pluginv1.ValidateWorkflowResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pluginv1.ValidateWorkflowResponse), args.Error(1)
}

func (m *MockValidatorServiceClient) ValidateStep(ctx context.Context, in *pluginv1.ValidateStepRequest, opts ...grpc.CallOption) (*pluginv1.ValidateStepResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pluginv1.ValidateStepResponse), args.Error(1)
}
