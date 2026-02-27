package builders

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
)

func TestStepBuilder_WithMessage_HappyPath(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		expectedMessage string
	}{
		{
			name:            "sets plain error message",
			message:         "Deploy failed",
			expectedMessage: "Deploy failed",
		},
		{
			name:            "sets message with template interpolation",
			message:         "{{states.build.output}} failed",
			expectedMessage: "{{states.build.output}} failed",
		},
		{
			name:            "sets message with inputs interpolation",
			message:         "Step {{inputs.env}} failed",
			expectedMessage: "Step {{inputs.env}} failed",
		},
		{
			name:            "sets message with env interpolation",
			message:         "Error in {{env.STAGE}} environment",
			expectedMessage: "Error in {{env.STAGE}} environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewTerminalStep("error-terminal", workflow.TerminalFailure).WithMessage(tt.message)
			step := builder.Build()

			assert.Equal(t, tt.expectedMessage, step.Message, "Message should match input")
		})
	}
}

func TestStepBuilder_WithMessage_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		expectedMessage string
	}{
		{
			name:            "handles empty string",
			message:         "",
			expectedMessage: "",
		},
		{
			name:            "handles message with special characters",
			message:         "Deploy failed: exit code 1 (non-zero)",
			expectedMessage: "Deploy failed: exit code 1 (non-zero)",
		},
		{
			name:            "handles multiword message",
			message:         "Critical failure in production pipeline stage",
			expectedMessage: "Critical failure in production pipeline stage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewTerminalStep("error-terminal", workflow.TerminalFailure).WithMessage(tt.message)
			step := builder.Build()

			assert.Equal(t, tt.expectedMessage, step.Message)
		})
	}
}

func TestStepBuilder_WithMessage_FluentInterface(t *testing.T) {
	builder := NewTerminalStep("error-terminal", workflow.TerminalFailure).
		WithMessage("Deploy failed").
		WithDescription("Terminal state for deployment errors")

	step := builder.Build()

	assert.Equal(t, "error-terminal", step.Name)
	assert.Equal(t, "Deploy failed", step.Message)
	assert.Equal(t, "Terminal state for deployment errors", step.Description)
}

func TestStepBuilder_WithMessage_Overwrite(t *testing.T) {
	builder := NewTerminalStep("error-terminal", workflow.TerminalFailure).
		WithMessage("first message").
		WithMessage("second message")

	step := builder.Build()

	assert.Equal(t, "second message", step.Message, "should use last set value")
}

func TestStepBuilder_WithMessage_StatusNotAffected(t *testing.T) {
	builder := NewTerminalStep("error-terminal", workflow.TerminalFailure).WithMessage("Build failed")
	step := builder.Build()

	assert.Equal(t, "Build failed", step.Message)
	assert.Equal(t, workflow.TerminalFailure, step.Status, "Status should remain unchanged when Message is set")
}
