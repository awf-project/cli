package repository

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mapStep Tests - Message Field Happy Path

func TestMapStep_Message_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.Step
	}{
		{
			name: "terminal step with message",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "Deployment failed",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Deployment failed",
			},
		},
		{
			name: "terminal step with long descriptive message",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "Build process failed: compilation error in src/main.go at line 42",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Build process failed: compilation error in src/main.go at line 42",
			},
		},
		{
			name: "terminal success step with message",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "success",
				Message: "Deployment completed successfully",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalSuccess,
				Message: "Deployment completed successfully",
			},
		},
		{
			name: "terminal step with template in message",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "Step {{states.build.output}} failed with error: {{states.build.error}}",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Step {{states.build.output}} failed with error: {{states.build.error}}",
			},
		},
		{
			name: "terminal step with input interpolation in message",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "Failed to deploy to {{inputs.environment}}",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Failed to deploy to {{inputs.environment}}",
			},
		},
		{
			name: "regular command step with message",
			yamlStep: yamlStep{
				Type:    "step",
				Command: "echo 'test'",
				Message: "This is a command step message",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeCommand,
				Command: "echo 'test'",
				Message: "This is a command step message",
			},
		},
		{
			name: "terminal step with special characters in message",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "Error: '\"special\\nchars\"' in output",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Error: '\"special\\nchars\"' in output",
			},
		},
		{
			name: "terminal step with multiline message content (YAML string)",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "Error occurred:\nLine 1: Database connection failed\nLine 2: Check credentials",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Error occurred:\nLine 1: Database connection failed\nLine 2: Check credentials",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Status, got.Status)
			assert.Equal(t, tt.want.Message, got.Message)
		})
	}
}

// mapStep Tests - Message Field Edge Cases

func TestMapStep_Message_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.Step
	}{
		{
			name: "terminal step with empty message",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "",
			},
		},
		{
			name: "terminal step without message field",
			yamlStep: yamlStep{
				Type:   "terminal",
				Status: "failure",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "",
			},
		},
		{
			name: "terminal step with message containing only spaces",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "   ",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "   ",
			},
		},
		{
			name: "terminal step with very long message",
			yamlStep: yamlStep{
				Type:   "terminal",
				Status: "failure",
				Message: "This is a very long error message that contains a lot of information about what went wrong. " +
					"It includes multiple sentences explaining the failure condition. " +
					"This could be a typical scenario where detailed error information is needed.",
			},
			want: &workflow.Step{
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
				Message: "This is a very long error message that contains a lot of information about what went wrong. " +
					"It includes multiple sentences explaining the failure condition. " +
					"This could be a typical scenario where detailed error information is needed.",
			},
		},
		{
			name: "command step without message",
			yamlStep: yamlStep{
				Type:    "step",
				Command: "echo 'test'",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeCommand,
				Command: "echo 'test'",
				Message: "",
			},
		},
		{
			name: "parallel step without message",
			yamlStep: yamlStep{
				Type:     "parallel",
				Strategy: "all_succeed",
				Parallel: []string{"branch1", "branch2"},
			},
			want: &workflow.Step{
				Type:     workflow.StepTypeParallel,
				Strategy: "all_succeed",
				Branches: []string{"branch1", "branch2"},
				Message:  "",
			},
		},
		{
			name: "terminal step with message containing newlines and tabs",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "Line 1\tTabbed\nLine 2\tTabbed\nLine 3",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Line 1\tTabbed\nLine 2\tTabbed\nLine 3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Status, got.Status)
			assert.Equal(t, tt.want.Message, got.Message)
		})
	}
}

// mapStep Tests - Message Field with Other Step Attributes

func TestMapStep_Message_WithOtherAttributes(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.Step
	}{
		{
			name: "terminal step with message and description",
			yamlStep: yamlStep{
				Type:        "terminal",
				Status:      "failure",
				Description: "Database connection step",
				Message:     "Database connection failed",
			},
			want: &workflow.Step{
				Type:        workflow.StepTypeTerminal,
				Status:      workflow.TerminalFailure,
				Description: "Database connection step",
				Message:     "Database connection failed",
			},
		},
		{
			name: "terminal step with message and transitions",
			yamlStep: yamlStep{
				Type:      "terminal",
				Status:    "failure",
				Message:   "Deployment failed",
				OnSuccess: "notify_success",
				OnFailure: "notify_failure",
			},
			want: &workflow.Step{
				Type:      workflow.StepTypeTerminal,
				Status:    workflow.TerminalFailure,
				Message:   "Deployment failed",
				OnSuccess: "notify_success",
				OnFailure: "notify_failure",
			},
		},
		{
			name: "terminal step with message and depends on",
			yamlStep: yamlStep{
				Type:      "terminal",
				Status:    "failure",
				Message:   "Step failed",
				DependsOn: []string{"setup", "build"},
			},
			want: &workflow.Step{
				Type:      workflow.StepTypeTerminal,
				Status:    workflow.TerminalFailure,
				Message:   "Step failed",
				DependsOn: []string{"setup", "build"},
			},
		},
		{
			name: "command step with message and retry config",
			yamlStep: yamlStep{
				Type:    "step",
				Command: "deploy.sh",
				Message: "Deployment in progress",
				Retry: &yamlRetry{
					MaxAttempts:  3,
					InitialDelay: "10s",
				},
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeCommand,
				Command: "deploy.sh",
				Message: "Deployment in progress",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    3,
					InitialDelayMs: 10000,
				},
			},
		},
		{
			name: "command step with message and timeout",
			yamlStep: yamlStep{
				Type:    "step",
				Command: "long_running_task",
				Message: "Long running task",
				Timeout: "30m",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeCommand,
				Command: "long_running_task",
				Message: "Long running task",
				Timeout: 1800,
			},
		},
		{
			name: "terminal step with message and all attributes",
			yamlStep: yamlStep{
				Type:            "terminal",
				Status:          "failure",
				Description:     "Final terminal state",
				Message:         "Workflow terminated",
				ContinueOnError: true,
				Dir:             "/tmp",
			},
			want: &workflow.Step{
				Type:            workflow.StepTypeTerminal,
				Status:          workflow.TerminalFailure,
				Description:     "Final terminal state",
				Message:         "Workflow terminated",
				ContinueOnError: true,
				Dir:             "/tmp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Status, got.Status)
			assert.Equal(t, tt.want.Message, got.Message)
			assert.Equal(t, tt.want.Description, got.Description)
			assert.Equal(t, tt.want.OnSuccess, got.OnSuccess)
			assert.Equal(t, tt.want.OnFailure, got.OnFailure)
			assert.Equal(t, tt.want.ContinueOnError, got.ContinueOnError)
			assert.Equal(t, tt.want.Dir, got.Dir)
			assert.Equal(t, tt.want.Timeout, got.Timeout)
			assert.Equal(t, tt.want.DependsOn, got.DependsOn)

			if tt.want.Retry != nil {
				require.NotNil(t, got.Retry)
				assert.Equal(t, tt.want.Retry.MaxAttempts, got.Retry.MaxAttempts)
				assert.Equal(t, tt.want.Retry.InitialDelayMs, got.Retry.InitialDelayMs)
			}
		})
	}
}

// mapStep Tests - Message Field with Different Step Types

func TestMapStep_Message_WithDifferentStepTypes(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.Step
	}{
		{
			name: "terminal failure step with message",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "Build failed",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Build failed",
			},
		},
		{
			name: "terminal success step with message",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "success",
				Message: "Deployment successful",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalSuccess,
				Message: "Deployment successful",
			},
		},
		{
			name: "command step with message",
			yamlStep: yamlStep{
				Type:    "step",
				Command: "echo 'Running'",
				Message: "Command step message",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeCommand,
				Command: "echo 'Running'",
				Message: "Command step message",
			},
		},
		{
			name: "agent step with message",
			yamlStep: yamlStep{
				Type:    "agent",
				Message: "Agent step message",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeAgent,
				Message: "Agent step message",
			},
		},
		{
			name: "parallel step with message",
			yamlStep: yamlStep{
				Type:     "parallel",
				Strategy: "all_succeed",
				Parallel: []string{"branch1", "branch2"},
				Message:  "Parallel step message",
			},
			want: &workflow.Step{
				Type:     workflow.StepTypeParallel,
				Strategy: "all_succeed",
				Branches: []string{"branch1", "branch2"},
				Message:  "Parallel step message",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Message, got.Message)
			assert.Equal(t, tt.want.Status, got.Status)
		})
	}
}

// mapStep Tests - Message Field Preservation (No Mutation)

func TestMapStep_Message_Preservation(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     string
	}{
		{
			name: "message is preserved exactly without modification",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "Original message",
			},
			want: "Original message",
		},
		{
			name: "message with template variables preserved exactly",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "{{states.previous_step.output}}",
			},
			want: "{{states.previous_step.output}}",
		},
		{
			name: "message case sensitivity preserved",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "MixedCase MESSAGE",
			},
			want: "MixedCase MESSAGE",
		},
		{
			name: "message with unicode preserved",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "エラーが発生しました: ❌",
			},
			want: "エラーが発生しました: ❌",
		},
		{
			name: "message with escape sequences preserved",
			yamlStep: yamlStep{
				Type:    "terminal",
				Status:  "failure",
				Message: "Line 1\\nLine 2\\tTabbed",
			},
			want: "Line 1\\nLine 2\\tTabbed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.Message)
		})
	}
}
