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
		{
			name: "parallel step with valid strategy all_succeed",
			step: workflow.Step{
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"step1", "step2"},
				Strategy: "all_succeed",
			},
			wantErr: false,
		},
		{
			name: "parallel step with valid strategy any_succeed",
			step: workflow.Step{
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"step1", "step2"},
				Strategy: "any_succeed",
			},
			wantErr: false,
		},
		{
			name: "parallel step with valid strategy best_effort",
			step: workflow.Step{
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"step1", "step2"},
				Strategy: "best_effort",
			},
			wantErr: false,
		},
		{
			name: "parallel step with invalid strategy",
			step: workflow.Step{
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"step1", "step2"},
				Strategy: "invalid_strategy",
			},
			wantErr: true,
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

func TestStepWithDir(t *testing.T) {
	tests := []struct {
		name string
		step workflow.Step
		want string
	}{
		{
			name: "step with absolute dir",
			step: workflow.Step{
				Name:    "build",
				Type:    workflow.StepTypeCommand,
				Command: "make build",
				Dir:     "/tmp/project",
			},
			want: "/tmp/project",
		},
		{
			name: "step with interpolated dir",
			step: workflow.Step{
				Name:    "test",
				Type:    workflow.StepTypeCommand,
				Command: "go test ./...",
				Dir:     "{{inputs.project_path}}",
			},
			want: "{{inputs.project_path}}",
		},
		{
			name: "step without dir",
			step: workflow.Step{
				Name:    "echo",
				Type:    workflow.StepTypeCommand,
				Command: "echo hello",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.step.Dir != tt.want {
				t.Errorf("Step.Dir = %q, want %q", tt.step.Dir, tt.want)
			}
		})
	}
}

func TestStepWithNewFields(t *testing.T) {
	step := workflow.Step{
		Name:    "extract",
		Type:    workflow.StepTypeCommand,
		Command: "cat {{inputs.file_path}}",
		Dir:     "/tmp/workdir",
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
	if step.Dir != "/tmp/workdir" {
		t.Errorf("expected Dir /tmp/workdir, got %s", step.Dir)
	}
}

// =============================================================================
// TerminalStatus Tests (F009)
// =============================================================================

func TestTerminalStatusValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "terminal with success status",
			step: workflow.Step{
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			wantErr: false,
		},
		{
			name: "terminal with failure status",
			step: workflow.Step{
				Name:   "error",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
			wantErr: false,
		},
		{
			name: "terminal with empty status (allowed)",
			step: workflow.Step{
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
			wantErr: false,
		},
		{
			name: "terminal with invalid status",
			step: workflow.Step{
				Name:   "bad",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalStatus("invalid"),
			},
			wantErr: true,
			errMsg:  "invalid terminal status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Step.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestTerminalStatusConstants(t *testing.T) {
	// Verify constants are defined correctly
	if workflow.TerminalSuccess != "success" {
		t.Errorf("TerminalSuccess should be 'success', got '%s'", workflow.TerminalSuccess)
	}
	if workflow.TerminalFailure != "failure" {
		t.Errorf("TerminalFailure should be 'failure', got '%s'", workflow.TerminalFailure)
	}
}

func TestTerminalStepWithStatus(t *testing.T) {
	successStep := workflow.Step{
		Name:   "success_end",
		Type:   workflow.StepTypeTerminal,
		Status: workflow.TerminalSuccess,
	}

	failureStep := workflow.Step{
		Name:   "failure_end",
		Type:   workflow.StepTypeTerminal,
		Status: workflow.TerminalFailure,
	}

	if successStep.Status != workflow.TerminalSuccess {
		t.Errorf("expected TerminalSuccess, got '%s'", successStep.Status)
	}
	if failureStep.Status != workflow.TerminalFailure {
		t.Errorf("expected TerminalFailure, got '%s'", failureStep.Status)
	}

	// Both should validate successfully
	if err := successStep.Validate(); err != nil {
		t.Errorf("success terminal step should be valid: %v", err)
	}
	if err := failureStep.Validate(); err != nil {
		t.Errorf("failure terminal step should be valid: %v", err)
	}
}

// Helper function for string containment check
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
