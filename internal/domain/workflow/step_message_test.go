package workflow_test

// F066: Tests for Step.Message field added to domain Step struct.
// ADR-002: yamlStep.Message is now mapped to domain Step.Message for terminal steps.
// ADR-003: Message stores raw template string; interpolation happens at runtime.

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func TestStepMessageField_TerminalWithMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "plain error message",
			message: "Deploy failed",
		},
		{
			name:    "message with interpolation template",
			message: "{{states.build.output}} failed at {{inputs.env}}",
		},
		{
			name:    "message with status interpolation",
			message: "Step {{states.build.output}} exited with code {{error.message}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := workflow.Step{
				Name:    "error_terminal",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: tt.message,
			}

			if step.Message != tt.message {
				t.Errorf("Step.Message = %q, want %q", step.Message, tt.message)
			}
		})
	}
}

func TestStepMessageField_DefaultsToEmpty(t *testing.T) {
	step := workflow.Step{
		Name:   "silent_error",
		Type:   workflow.StepTypeTerminal,
		Status: workflow.TerminalFailure,
	}

	if step.Message != "" {
		t.Errorf("Step.Message should default to empty string, got %q", step.Message)
	}
}

func TestStepMessageField_TemplateStoredVerbatim(t *testing.T) {
	rawTemplate := "{{states.build.output}} failed — check logs at {{env.LOG_URL}}"

	step := workflow.Step{
		Name:    "infra_error",
		Type:    workflow.StepTypeTerminal,
		Status:  workflow.TerminalFailure,
		Message: rawTemplate,
	}

	// ADR-003: template must be stored as-is, not interpolated at parse/struct-creation time
	if step.Message != rawTemplate {
		t.Errorf("Step.Message should preserve raw template %q, got %q", rawTemplate, step.Message)
	}
}

func TestStepMessageField_TerminalWithMessageValidates(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
	}{
		{
			name: "terminal failure with message is valid",
			step: workflow.Step{
				Name:    "deploy_error",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Deploy failed: {{states.deploy.output}}",
			},
			wantErr: false,
		},
		{
			name: "terminal success with message is valid",
			step: workflow.Step{
				Name:    "deploy_done",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalSuccess,
				Message: "Deploy complete",
			},
			wantErr: false,
		},
		{
			name: "terminal with empty status and message is valid",
			step: workflow.Step{
				Name:    "generic_end",
				Type:    workflow.StepTypeTerminal,
				Message: "Workflow ended",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate(nil, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Step.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
