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
		{workflow.StepTypeForEach, "for_each"},
		{workflow.StepTypeWhile, "while"},
		{workflow.StepTypeOperation, "operation"},
		{workflow.StepTypeCallWorkflow, "call_workflow"},
		{workflow.StepTypeAgent, "agent"},
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
	return len(s) >= len(substr) && (s == substr || s != "" && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// Loop Step Tests (F016)
// =============================================================================

func TestForEachStepValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid for_each step",
			step: workflow.Step{
				Name: "process_files",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         `["a.txt", "b.txt"]`,
					Body:          []string{"process"},
					MaxIterations: 100,
					OnComplete:    "done",
				},
			},
			wantErr: false,
		},
		{
			name: "for_each step without loop config",
			step: workflow.Step{
				Name: "bad_foreach",
				Type: workflow.StepTypeForEach,
			},
			wantErr: true,
			errMsg:  "loop config is required",
		},
		{
			name: "for_each step without items",
			step: workflow.Step{
				Name: "no_items",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type: workflow.LoopTypeForEach,
					Body: []string{"process"},
				},
			},
			wantErr: true,
			errMsg:  "items is required",
		},
		{
			name: "for_each step without body",
			step: workflow.Step{
				Name: "no_body",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:  workflow.LoopTypeForEach,
					Items: `["a"]`,
				},
			},
			wantErr: true,
			errMsg:  "body is required",
		},
		{
			name: "for_each with template items",
			step: workflow.Step{
				Name: "template_items",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         "{{inputs.files}}",
					Body:          []string{"process"},
					MaxIterations: 50,
				},
			},
			wantErr: false,
		},
		{
			name: "for_each with break condition",
			step: workflow.Step{
				Name: "with_break",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeForEach,
					Items:          `["a", "b", "c"]`,
					Body:           []string{"check"},
					MaxIterations:  100,
					BreakCondition: "states.check.output == 'stop'",
				},
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
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestWhileStepValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid while step",
			step: workflow.Step{
				Name: "poll_status",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeWhile,
					Condition:     "states.check.output != 'ready'",
					Body:          []string{"check", "wait"},
					MaxIterations: 60,
					OnComplete:    "proceed",
				},
			},
			wantErr: false,
		},
		{
			name: "while step without loop config",
			step: workflow.Step{
				Name: "bad_while",
				Type: workflow.StepTypeWhile,
			},
			wantErr: true,
			errMsg:  "loop config is required",
		},
		{
			name: "while step without condition",
			step: workflow.Step{
				Name: "no_condition",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type: workflow.LoopTypeWhile,
					Body: []string{"check"},
				},
			},
			wantErr: true,
			errMsg:  "condition is required",
		},
		{
			name: "while step without body",
			step: workflow.Step{
				Name: "no_body",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:      workflow.LoopTypeWhile,
					Condition: "true",
				},
			},
			wantErr: true,
			errMsg:  "body is required",
		},
		{
			name: "while with break condition",
			step: workflow.Step{
				Name: "with_break",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeWhile,
					Condition:      "true",
					Body:           []string{"poll"},
					MaxIterations:  100,
					BreakCondition: "states.poll.exit_code != 0",
				},
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
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestLoopStepCreation(t *testing.T) {
	// Test for_each step creation
	forEachStep := workflow.Step{
		Name:        "process_files",
		Type:        workflow.StepTypeForEach,
		Description: "Process each file in the input list",
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         "{{inputs.files}}",
			Body:          []string{"process_single"},
			MaxIterations: 100,
			OnComplete:    "aggregate",
		},
	}

	if forEachStep.Name != "process_files" {
		t.Errorf("expected name 'process_files', got '%s'", forEachStep.Name)
	}
	if forEachStep.Type != workflow.StepTypeForEach {
		t.Errorf("expected type StepTypeForEach, got '%v'", forEachStep.Type)
	}
	if forEachStep.Loop == nil {
		t.Fatal("expected Loop to be set")
	}
	if forEachStep.Loop.Type != workflow.LoopTypeForEach {
		t.Errorf("expected loop type LoopTypeForEach, got '%v'", forEachStep.Loop.Type)
	}
	if forEachStep.Loop.OnComplete != "aggregate" {
		t.Errorf("expected OnComplete 'aggregate', got '%s'", forEachStep.Loop.OnComplete)
	}

	// Test while step creation
	whileStep := workflow.Step{
		Name:        "poll_api",
		Type:        workflow.StepTypeWhile,
		Description: "Poll API until ready",
		Timeout:     300,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "states.check.output != 'ready'",
			Body:          []string{"check", "sleep"},
			MaxIterations: 60,
			OnComplete:    "proceed",
		},
	}

	if whileStep.Name != "poll_api" {
		t.Errorf("expected name 'poll_api', got '%s'", whileStep.Name)
	}
	if whileStep.Type != workflow.StepTypeWhile {
		t.Errorf("expected type StepTypeWhile, got '%v'", whileStep.Type)
	}
	if whileStep.Loop == nil {
		t.Fatal("expected Loop to be set")
	}
	if whileStep.Loop.Type != workflow.LoopTypeWhile {
		t.Errorf("expected loop type LoopTypeWhile, got '%v'", whileStep.Loop.Type)
	}
	if whileStep.Loop.Condition != "states.check.output != 'ready'" {
		t.Errorf("expected condition 'states.check.output != 'ready'', got '%s'", whileStep.Loop.Condition)
	}
}

func TestLoopStepWithHooks(t *testing.T) {
	step := workflow.Step{
		Name: "process_with_hooks",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["item1", "item2"]`,
			Body:          []string{"process"},
			MaxIterations: 10,
		},
		Hooks: workflow.StepHooks{
			Pre:  workflow.Hook{{Log: "Starting loop"}},
			Post: workflow.Hook{{Log: "Loop complete"}},
		},
	}

	if len(step.Hooks.Pre) != 1 {
		t.Errorf("expected 1 pre hook, got %d", len(step.Hooks.Pre))
	}
	if len(step.Hooks.Post) != 1 {
		t.Errorf("expected 1 post hook, got %d", len(step.Hooks.Post))
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("loop step with hooks should be valid: %v", err)
	}
}

func TestLoopStepWithTimeout(t *testing.T) {
	step := workflow.Step{
		Name:    "timed_loop",
		Type:    workflow.StepTypeWhile,
		Timeout: 60,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"work"},
			MaxIterations: 1000,
		},
	}

	if step.Timeout != 60 {
		t.Errorf("expected Timeout 60, got %d", step.Timeout)
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("loop step with timeout should be valid: %v", err)
	}
}

// =============================================================================
// CallWorkflow Step Tests (F023)
// =============================================================================

func TestCallWorkflowStepValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid call_workflow step",
			step: workflow.Step{
				Name: "call_analyzer",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "analyze-file",
					Inputs: map[string]string{
						"file_path": "{{inputs.source_file}}",
					},
					Outputs: map[string]string{
						"analysis_result": "result",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid call_workflow step with timeout",
			step: workflow.Step{
				Name: "call_with_timeout",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "long-running-workflow",
					Timeout:  600,
				},
			},
			wantErr: false,
		},
		{
			name: "valid call_workflow step minimal config",
			step: workflow.Step{
				Name: "call_simple",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "simple-workflow",
				},
			},
			wantErr: false,
		},
		{
			name: "call_workflow step without config",
			step: workflow.Step{
				Name: "bad_call",
				Type: workflow.StepTypeCallWorkflow,
			},
			wantErr: true,
			errMsg:  "call_workflow config is required",
		},
		{
			name: "call_workflow step with nil config",
			step: workflow.Step{
				Name:         "nil_config",
				Type:         workflow.StepTypeCallWorkflow,
				CallWorkflow: nil,
			},
			wantErr: true,
			errMsg:  "call_workflow config is required",
		},
		{
			name: "call_workflow step with empty workflow name",
			step: workflow.Step{
				Name: "empty_workflow",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "",
				},
			},
			wantErr: true,
			errMsg:  "workflow name is required",
		},
		{
			name: "call_workflow step with negative timeout",
			step: workflow.Step{
				Name: "negative_timeout",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "some-workflow",
					Timeout:  -1,
				},
			},
			wantErr: true,
			errMsg:  "timeout must be non-negative",
		},
		{
			name: "call_workflow step with zero timeout (uses default)",
			step: workflow.Step{
				Name: "zero_timeout",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "some-workflow",
					Timeout:  0,
				},
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
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestCallWorkflowStepCreation(t *testing.T) {
	step := workflow.Step{
		Name:        "invoke_analyzer",
		Type:        workflow.StepTypeCallWorkflow,
		Description: "Call the file analyzer sub-workflow",
		Timeout:     120,
		OnSuccess:   "aggregate",
		OnFailure:   "handle_error",
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "analyze-single-file",
			Inputs: map[string]string{
				"file_path":  "{{loop.item}}",
				"max_tokens": "{{inputs.max_tokens}}",
			},
			Outputs: map[string]string{
				"result": "analysis_result",
			},
			Timeout: 300,
		},
	}

	if step.Name != "invoke_analyzer" {
		t.Errorf("expected name 'invoke_analyzer', got '%s'", step.Name)
	}
	if step.Type != workflow.StepTypeCallWorkflow {
		t.Errorf("expected type StepTypeCallWorkflow, got '%v'", step.Type)
	}
	if step.CallWorkflow == nil {
		t.Fatal("expected CallWorkflow to be set")
	}
	if step.CallWorkflow.Workflow != "analyze-single-file" {
		t.Errorf("expected workflow 'analyze-single-file', got '%s'", step.CallWorkflow.Workflow)
	}
	if len(step.CallWorkflow.Inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(step.CallWorkflow.Inputs))
	}
	if step.CallWorkflow.Inputs["file_path"] != "{{loop.item}}" {
		t.Errorf("expected input file_path '{{loop.item}}', got '%s'", step.CallWorkflow.Inputs["file_path"])
	}
	if len(step.CallWorkflow.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(step.CallWorkflow.Outputs))
	}
	if step.CallWorkflow.Outputs["result"] != "analysis_result" {
		t.Errorf("expected output 'result' -> 'analysis_result', got '%s'", step.CallWorkflow.Outputs["result"])
	}
	if step.CallWorkflow.Timeout != 300 {
		t.Errorf("expected timeout 300, got %d", step.CallWorkflow.Timeout)
	}
	if step.OnSuccess != "aggregate" {
		t.Errorf("expected OnSuccess 'aggregate', got '%s'", step.OnSuccess)
	}
	if step.OnFailure != "handle_error" {
		t.Errorf("expected OnFailure 'handle_error', got '%s'", step.OnFailure)
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("valid call_workflow step should not return error: %v", err)
	}
}

func TestCallWorkflowStepWithHooks(t *testing.T) {
	step := workflow.Step{
		Name: "call_with_hooks",
		Type: workflow.StepTypeCallWorkflow,
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "sub-workflow",
		},
		Hooks: workflow.StepHooks{
			Pre:  workflow.Hook{{Log: "Calling sub-workflow"}},
			Post: workflow.Hook{{Log: "Sub-workflow completed"}},
		},
	}

	if len(step.Hooks.Pre) != 1 {
		t.Errorf("expected 1 pre hook, got %d", len(step.Hooks.Pre))
	}
	if len(step.Hooks.Post) != 1 {
		t.Errorf("expected 1 post hook, got %d", len(step.Hooks.Post))
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("call_workflow step with hooks should be valid: %v", err)
	}
}

func TestCallWorkflowStepWithRetry(t *testing.T) {
	step := workflow.Step{
		Name: "call_with_retry",
		Type: workflow.StepTypeCallWorkflow,
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "flaky-workflow",
		},
		Retry: &workflow.RetryConfig{
			MaxAttempts:    3,
			InitialDelayMs: 1000,
			Backoff:        "exponential",
			Multiplier:     2.0,
		},
	}

	if step.Retry == nil {
		t.Fatal("expected Retry to be set")
	}
	if step.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", step.Retry.MaxAttempts)
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("call_workflow step with retry should be valid: %v", err)
	}
}

func TestCallWorkflowStepWithContinueOnError(t *testing.T) {
	step := workflow.Step{
		Name: "optional_call",
		Type: workflow.StepTypeCallWorkflow,
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "optional-workflow",
		},
		ContinueOnError: true,
	}

	if !step.ContinueOnError {
		t.Error("expected ContinueOnError to be true")
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("call_workflow step with ContinueOnError should be valid: %v", err)
	}
}

func TestCallWorkflowTypeConstant(t *testing.T) {
	// Verify the constant is defined correctly
	if workflow.StepTypeCallWorkflow != "call_workflow" {
		t.Errorf("StepTypeCallWorkflow should be 'call_workflow', got '%s'", workflow.StepTypeCallWorkflow)
	}

	// Verify it stringifies correctly
	if workflow.StepTypeCallWorkflow.String() != "call_workflow" {
		t.Errorf("StepTypeCallWorkflow.String() should be 'call_workflow', got '%s'", workflow.StepTypeCallWorkflow.String())
	}
}

func TestCallWorkflowStepWithEmptyInputsOutputs(t *testing.T) {
	// Steps can have empty inputs/outputs - workflow may not need any
	step := workflow.Step{
		Name: "call_no_io",
		Type: workflow.StepTypeCallWorkflow,
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "standalone-workflow",
			Inputs:   map[string]string{},
			Outputs:  map[string]string{},
		},
	}

	if step.CallWorkflow.Inputs == nil {
		t.Error("expected Inputs to be initialized")
	}
	if step.CallWorkflow.Outputs == nil {
		t.Error("expected Outputs to be initialized")
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("call_workflow step with empty inputs/outputs should be valid: %v", err)
	}
}

func TestCallWorkflowStepWithTemplateInterpolation(t *testing.T) {
	// Test that template expressions in inputs are accepted
	step := workflow.Step{
		Name: "call_with_templates",
		Type: workflow.StepTypeCallWorkflow,
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "process-file",
			Inputs: map[string]string{
				"file":      "{{inputs.source_file}}",
				"output":    "{{states.prepare.output}}",
				"env_value": "{{env.API_KEY}}",
				"combined":  "prefix-{{inputs.name}}-suffix",
			},
		},
	}

	if len(step.CallWorkflow.Inputs) != 4 {
		t.Errorf("expected 4 inputs, got %d", len(step.CallWorkflow.Inputs))
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("call_workflow step with template inputs should be valid: %v", err)
	}
}

// Component: step_type_extension
// Feature: 39 - AI Agent Step Type

func TestStepTypeAgent_String(t *testing.T) {
	if got := workflow.StepTypeAgent.String(); got != "agent" {
		t.Errorf("StepTypeAgent.String() = %s, want %s", got, "agent")
	}
}

func TestStep_Validate_AgentType_HappyPath(t *testing.T) {
	tests := []struct {
		name string
		step workflow.Step
	}{
		{
			name: "valid agent step with all fields",
			step: workflow.Step{
				Name: "ask_claude",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Analyze {{inputs.data}}",
					Options: map[string]any{
						"model":       "claude-3-5-sonnet-20241022",
						"temperature": 0.7,
						"max_tokens":  1000,
					},
					Timeout: 60,
				},
			},
		},
		{
			name: "valid agent step with minimal fields",
			step: workflow.Step{
				Name: "simple_agent",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "codex",
					Prompt:   "Generate code for {{inputs.task}}",
				},
			},
		},
		{
			name: "valid agent step with custom provider",
			step: workflow.Step{
				Name: "custom_agent",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "custom",
					Prompt:   "Process {{inputs.text}}",
					Command:  "python3 custom_agent.py --prompt={{prompt}}",
					Timeout:  120,
				},
			},
		},
		{
			name: "valid agent step with gemini provider",
			step: workflow.Step{
				Name: "gemini_agent",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "gemini",
					Prompt:   "Summarize {{inputs.article}}",
					Options: map[string]any{
						"model": "gemini-pro",
					},
				},
			},
		},
		{
			name: "valid agent step with zero timeout (uses default)",
			step: workflow.Step{
				Name: "default_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "opencode",
					Prompt:   "Review {{inputs.code}}",
					Timeout:  0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if err != nil {
				t.Errorf("Step.Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestStep_Validate_AgentType_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "agent step with empty options map",
			step: workflow.Step{
				Name: "agent_no_options",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Test prompt",
					Options:  map[string]any{},
				},
			},
			wantErr: false,
		},
		{
			name: "agent step with nil options",
			step: workflow.Step{
				Name: "agent_nil_options",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Test prompt",
					Options:  nil,
				},
			},
			wantErr: false,
		},
		{
			name: "agent step with very long prompt",
			step: workflow.Step{
				Name: "long_prompt",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   string(make([]byte, 10000)) + "{{inputs.data}}",
				},
			},
			wantErr: false,
		},
		{
			name: "agent step with complex nested options",
			step: workflow.Step{
				Name: "complex_options",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Process {{inputs.data}}",
					Options: map[string]any{
						"model":       "claude-3-5-sonnet-20241022",
						"temperature": 0.7,
						"max_tokens":  1000,
						"metadata": map[string]any{
							"user_id": "123",
							"tags":    []string{"test", "ai"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "agent step with max timeout value",
			step: workflow.Step{
				Name: "max_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Long task {{inputs.data}}",
					Timeout:  3600,
				},
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
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("Step.Validate() error message = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestStep_Validate_AgentType_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "agent step without agent config",
			step: workflow.Step{
				Name:  "no_config",
				Type:  workflow.StepTypeAgent,
				Agent: nil,
			},
			wantErr: true,
			errMsg:  "agent config is required for agent-type steps",
		},
		{
			name: "agent step with missing provider",
			step: workflow.Step{
				Name: "no_provider",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "",
					Prompt:   "Test prompt",
				},
			},
			wantErr: true,
		},
		{
			name: "agent step with missing prompt",
			step: workflow.Step{
				Name: "no_prompt",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "",
				},
			},
			wantErr: true,
		},
		{
			name: "agent step with negative timeout",
			step: workflow.Step{
				Name: "negative_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Test prompt",
					Timeout:  -60,
				},
			},
			wantErr: true,
		},
		{
			name: "agent step with empty name",
			step: workflow.Step{
				Name: "",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Test prompt",
				},
			},
			wantErr: true,
			errMsg:  "step name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Step.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("Step.Validate() error message = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestStep_Validate_AgentType_WithTransitions(t *testing.T) {
	step := workflow.Step{
		Name: "agent_with_transitions",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Analyze {{inputs.data}}",
		},
		Transitions: workflow.Transitions{
			{
				When: "{{states.agent_with_transitions.output}} == 'success'",
				Goto: "next_step",
			},
			{
				When: "{{states.agent_with_transitions.output}} == 'failure'",
				Goto: "error_handler",
			},
		},
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with transitions should be valid: %v", err)
	}

	if len(step.Transitions) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(step.Transitions))
	}
}

func TestStep_Validate_AgentType_WithRetry(t *testing.T) {
	step := workflow.Step{
		Name: "agent_with_retry",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Analyze {{inputs.data}}",
			Timeout:  30,
		},
		Retry: &workflow.RetryConfig{
			MaxAttempts:    3,
			InitialDelayMs: 1000,
			MaxDelayMs:     10000,
			Backoff:        "exponential",
			Multiplier:     2.0,
		},
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with retry config should be valid: %v", err)
	}

	if step.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", step.Retry.MaxAttempts)
	}
}

func TestStep_Validate_AgentType_WithCapture(t *testing.T) {
	step := workflow.Step{
		Name: "agent_with_capture",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Generate report for {{inputs.data}}",
		},
		Capture: &workflow.CaptureConfig{
			Stdout:   "agent_output",
			Stderr:   "agent_errors",
			MaxSize:  "5MB",
			Encoding: "utf-8",
		},
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with capture config should be valid: %v", err)
	}

	if step.Capture.Stdout != "agent_output" {
		t.Errorf("expected Stdout 'agent_output', got %s", step.Capture.Stdout)
	}
}

func TestStep_Validate_AgentType_WithHooks(t *testing.T) {
	step := workflow.Step{
		Name: "agent_with_hooks",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Process {{inputs.data}}",
		},
		Hooks: workflow.StepHooks{
			Pre:  workflow.Hook{{Command: "echo 'Starting agent execution'"}},
			Post: workflow.Hook{{Command: "echo 'Agent execution completed'"}},
		},
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with hooks should be valid: %v", err)
	}

	if len(step.Hooks.Pre) != 1 {
		t.Errorf("expected 1 pre hook, got %d", len(step.Hooks.Pre))
	}
}

func TestStep_Validate_AgentType_WithTimeout(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
	}{
		{
			name: "agent step with valid timeout",
			step: workflow.Step{
				Name: "agent_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Quick task",
					Timeout:  30,
				},
				Timeout: 60, // step-level timeout
			},
			wantErr: false,
		},
		{
			name: "agent step with only agent timeout",
			step: workflow.Step{
				Name: "agent_only_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Quick task",
					Timeout:  45,
				},
			},
			wantErr: false,
		},
		{
			name: "agent step with only step timeout",
			step: workflow.Step{
				Name: "step_only_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Quick task",
				},
				Timeout: 90,
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

func TestStep_Validate_AgentType_WithContinueOnError(t *testing.T) {
	step := workflow.Step{
		Name: "agent_continue_on_error",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Try to process {{inputs.data}}",
		},
		ContinueOnError: true,
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with continue_on_error should be valid: %v", err)
	}

	if !step.ContinueOnError {
		t.Errorf("expected ContinueOnError true, got false")
	}
}

func TestStep_Validate_AgentType_WithDependsOn(t *testing.T) {
	step := workflow.Step{
		Name: "dependent_agent",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Analyze results from {{states.step1.output}} and {{states.step2.output}}",
		},
		DependsOn: []string{"step1", "step2"},
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with dependencies should be valid: %v", err)
	}

	if len(step.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(step.DependsOn))
	}
}

func TestStep_Validate_AgentType_CompleteWorkflow(t *testing.T) {
	// Test a complete agent step with all possible configurations
	step := workflow.Step{
		Name:        "comprehensive_agent",
		Type:        workflow.StepTypeAgent,
		Description: "Comprehensive AI agent with all features",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Analyze and process {{inputs.data}} considering {{inputs.context}}",
			Options: map[string]any{
				"model":            "claude-3-5-sonnet-20241022",
				"temperature":      0.7,
				"max_tokens":       2000,
				"top_p":            0.9,
				"stop_sequences":   []string{"\n\nHuman:"},
				"presence_penalty": 0.0,
			},
			Timeout: 120,
		},
		Timeout: 180,
		Retry: &workflow.RetryConfig{
			MaxAttempts:    3,
			InitialDelayMs: 2000,
			MaxDelayMs:     30000,
			Backoff:        "exponential",
			Multiplier:     2.0,
			Jitter:         0.1,
		},
		Capture: &workflow.CaptureConfig{
			Stdout:   "agent_response",
			Stderr:   "agent_errors",
			MaxSize:  "10MB",
			Encoding: "utf-8",
		},
		Hooks: workflow.StepHooks{
			Pre:  workflow.Hook{{Command: "echo 'Preparing agent execution'"}},
			Post: workflow.Hook{{Command: "echo 'Agent execution finished'"}},
		},
		Transitions: workflow.Transitions{
			{
				When: "{{states.comprehensive_agent.output}} contains 'approved'",
				Goto: "approval_step",
			},
			{
				When: "{{states.comprehensive_agent.output}} contains 'rejected'",
				Goto: "rejection_step",
			},
		},
		DependsOn:       []string{"prepare_data", "load_context"},
		ContinueOnError: false,
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("comprehensive agent step should be valid: %v", err)
	}

	// Verify all configurations are set
	if step.Agent.Provider != "claude" {
		t.Errorf("expected provider 'claude', got %s", step.Agent.Provider)
	}
	if step.Agent.Timeout != 120 {
		t.Errorf("expected agent timeout 120, got %d", step.Agent.Timeout)
	}
	if step.Timeout != 180 {
		t.Errorf("expected step timeout 180, got %d", step.Timeout)
	}
	if step.Retry.MaxAttempts != 3 {
		t.Errorf("expected 3 retry attempts, got %d", step.Retry.MaxAttempts)
	}
	if step.Capture.Stdout != "agent_response" {
		t.Errorf("expected stdout capture 'agent_response', got %s", step.Capture.Stdout)
	}
	if len(step.Hooks.Pre) != 1 {
		t.Errorf("expected 1 pre hook, got %d", len(step.Hooks.Pre))
	}
	if len(step.Transitions) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(step.Transitions))
	}
	if len(step.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(step.DependsOn))
	}
}
