package workflow_test

import (
	"testing"

	"github.com/vanoix/awf/internal/domain/workflow"
)

func TestStepTypeString(t *testing.T) {
	tests := []struct {
		stepType workflow.StepType
		want     string
	}{
		{workflow.StepTypeCommand, "command"},
		{workflow.StepTypeParallel, "parallel"},
		{workflow.StepTypeTerminal, "terminal"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.stepType.String(); got != tt.want {
				t.Errorf("StepType.String() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestStepCreation(t *testing.T) {
	step := workflow.Step{
		Name:        "validate",
		Type:        workflow.StepTypeCommand,
		Description: "Validate input file",
		Command:     "test -f {{inputs.file_path}}",
		Timeout:     5,
		OnSuccess:   "extract",
		OnFailure:   "error",
	}

	if step.Name != "validate" {
		t.Errorf("expected name 'validate', got '%s'", step.Name)
	}
	if step.Type != workflow.StepTypeCommand {
		t.Errorf("expected type StepTypeCommand, got '%v'", step.Type)
	}
	if step.OnSuccess != "extract" {
		t.Errorf("expected OnSuccess 'extract', got '%s'", step.OnSuccess)
	}
}

func TestStepValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
	}{
		{
			name: "valid command step",
			step: workflow.Step{
				Name:    "test",
				Type:    workflow.StepTypeCommand,
				Command: "echo hello",
			},
			wantErr: false,
		},
		{
			name: "valid terminal step",
			step: workflow.Step{
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			wantErr: false,
		},
		{
			name:    "missing name",
			step:    workflow.Step{Type: workflow.StepTypeCommand, Command: "echo hello"},
			wantErr: true,
		},
		{
			name:    "command step without command",
			step:    workflow.Step{Name: "test", Type: workflow.StepTypeCommand},
			wantErr: true,
		},
		{
			name: "parallel step without branches",
			step: workflow.Step{
				Name: "parallel",
				Type: workflow.StepTypeParallel,
			},
			wantErr: true,
		},
		{
			name: "valid parallel step",
			step: workflow.Step{
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"step1", "step2"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Step.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRetryConfig(t *testing.T) {
	t.Run("empty retry config", func(t *testing.T) {
		retry := workflow.RetryConfig{}
		if retry.MaxAttempts != 0 {
			t.Errorf("expected MaxAttempts 0, got %d", retry.MaxAttempts)
		}
	})

	t.Run("full retry config", func(t *testing.T) {
		retry := workflow.RetryConfig{
			MaxAttempts:        3,
			InitialDelayMs:     1000,
			MaxDelayMs:         30000,
			Backoff:            "exponential",
			Multiplier:         2.0,
			Jitter:             0.1,
			RetryableExitCodes: []int{1, 2, 3},
		}
		if retry.MaxAttempts != 3 {
			t.Errorf("expected MaxAttempts 3, got %d", retry.MaxAttempts)
		}
		if retry.Backoff != "exponential" {
			t.Errorf("expected Backoff exponential, got %s", retry.Backoff)
		}
		if len(retry.RetryableExitCodes) != 3 {
			t.Errorf("expected 3 retryable codes, got %d", len(retry.RetryableExitCodes))
		}
	})
}

func TestCaptureConfig(t *testing.T) {
	t.Run("empty capture config", func(t *testing.T) {
		capture := workflow.CaptureConfig{}
		if capture.Stdout != "" {
			t.Errorf("expected empty Stdout, got %s", capture.Stdout)
		}
	})

	t.Run("full capture config", func(t *testing.T) {
		capture := workflow.CaptureConfig{
			Stdout:   "output",
			Stderr:   "errors",
			MaxSize:  "10MB",
			Encoding: "utf-8",
		}
		if capture.Stdout != "output" {
			t.Errorf("expected Stdout output, got %s", capture.Stdout)
		}
		if capture.MaxSize != "10MB" {
			t.Errorf("expected MaxSize 10MB, got %s", capture.MaxSize)
		}
	})
}

func TestStepWithNewFields(t *testing.T) {
	step := workflow.Step{
		Name:    "extract",
		Type:    workflow.StepTypeCommand,
		Command: "cat {{inputs.file_path}}",
		Retry: &workflow.RetryConfig{
			MaxAttempts:    3,
			InitialDelayMs: 1000,
			Backoff:        "exponential",
		},
		Capture: &workflow.CaptureConfig{
			Stdout:  "file_content",
			MaxSize: "10MB",
		},
		Hooks: workflow.StepHooks{
			Pre:  workflow.Hook{{Log: "Starting extraction"}},
			Post: workflow.Hook{{Log: "Extraction complete"}},
		},
		ContinueOnError: true,
	}

	if step.Retry == nil {
		t.Fatal("expected Retry to be set")
	}
	if step.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", step.Retry.MaxAttempts)
	}
	if step.Capture == nil {
		t.Fatal("expected Capture to be set")
	}
	if step.Capture.Stdout != "file_content" {
		t.Errorf("expected Stdout file_content, got %s", step.Capture.Stdout)
	}
	if len(step.Hooks.Pre) != 1 {
		t.Errorf("expected 1 pre hook, got %d", len(step.Hooks.Pre))
	}
	if !step.ContinueOnError {
		t.Error("expected ContinueOnError to be true")
	}
}
