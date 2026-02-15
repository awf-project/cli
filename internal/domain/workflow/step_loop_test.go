package workflow_test

// C013: Domain test file splitting
// Source: internal/domain/workflow/step_test.go
// Test count: 5 tests
// Tests: Loop step functionality including for_each and while validation,
//        creation, hooks integration, and timeout handling

import (
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
)

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
			err := tt.step.Validate(nil)
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
			err := tt.step.Validate(nil)
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

	err := step.Validate(nil)
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

	err := step.Validate(nil)
	if err != nil {
		t.Errorf("loop step with timeout should be valid: %v", err)
	}
}
