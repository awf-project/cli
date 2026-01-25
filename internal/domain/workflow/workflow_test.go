package workflow_test

import (
	"testing"

	"github.com/vanoix/awf/internal/domain/workflow"
)

func TestWorkflowCreation(t *testing.T) {
	wf := workflow.Workflow{
		Name:        "analyze-code",
		Description: "Analyze code with AI",
		Version:     "1.0.0",
		Author:      "test",
		Tags:        []string{"ai", "analysis"},
		Initial:     "validate",
		Steps: map[string]*workflow.Step{
			"validate": {
				Name:      "validate",
				Type:      workflow.StepTypeCommand,
				Command:   "test -f {{inputs.file_path}}",
				OnSuccess: "extract",
				OnFailure: "error",
			},
			"extract": {
				Name:      "extract",
				Type:      workflow.StepTypeCommand,
				Command:   "cat {{inputs.file_path}}",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	if wf.Name != "analyze-code" {
		t.Errorf("expected name 'analyze-code', got '%s'", wf.Name)
	}
	if len(wf.Steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(wf.Steps))
	}
	if len(wf.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(wf.Tags))
	}
}

func TestWorkflowGetStep(t *testing.T) {
	wf := workflow.Workflow{
		Steps: map[string]*workflow.Step{
			"validate": {Name: "validate", Type: workflow.StepTypeCommand, Command: "test"},
		},
	}

	step, ok := wf.GetStep("validate")
	if !ok {
		t.Error("expected step 'validate' to exist")
	}
	if step.Name != "validate" {
		t.Errorf("expected name 'validate', got '%s'", step.Name)
	}

	_, ok = wf.GetStep("nonexistent")
	if ok {
		t.Error("expected step 'nonexistent' to not exist")
	}
}

func TestWorkflowValidation(t *testing.T) {
	tests := []struct {
		name    string
		wf      workflow.Workflow
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid workflow",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "end"},
					"end":   {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: false,
		},
		{
			name:    "missing name",
			wf:      workflow.Workflow{Initial: "start"},
			wantErr: true,
			errMsg:  "workflow name is required",
		},
		{
			name:    "missing initial state",
			wf:      workflow.Workflow{Name: "test"},
			wantErr: true,
			errMsg:  "initial state is required",
		},
		{
			name: "initial state does not exist",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "nonexistent",
				Steps:   map[string]*workflow.Step{},
			},
			wantErr: true,
			errMsg:  "initial state 'nonexistent' not found in steps",
		},
		{
			name: "no terminal state",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "start"},
				},
			},
			wantErr: true,
			errMsg:  "at least one terminal state is required",
		},
		{
			name: "invalid step",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {Name: "", Type: workflow.StepTypeCommand}, // invalid: missing name
					"end":   {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
		},
		{
			name: "unreachable terminal from non-terminal",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start":  {Name: "start", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "middle"},
					"middle": {Name: "middle", Type: workflow.StepTypeCommand, Command: "echo"},
					"end":    {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'middle': command step must have OnSuccess/OnFailure or Transitions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wf.Validate(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Workflow.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("expected error '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestInputStruct(t *testing.T) {
	input := workflow.Input{
		Name:        "file_path",
		Type:        "string",
		Description: "Path to input file",
		Required:    true,
		Default:     nil,
	}

	if input.Name != "file_path" {
		t.Errorf("expected Name 'file_path', got '%s'", input.Name)
	}
	if !input.Required {
		t.Error("expected Required to be true")
	}
}

func TestWorkflowWithInputs(t *testing.T) {
	wf := workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Inputs: []workflow.Input{
			{Name: "file", Type: "string", Required: true},
			{Name: "count", Type: "integer", Default: 10},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}

	if len(wf.Inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(wf.Inputs))
	}
	if wf.Inputs[1].Default != 10 {
		t.Errorf("expected default 10, got %v", wf.Inputs[1].Default)
	}
}

func TestWorkflowWithHooks(t *testing.T) {
	wf := workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
		Hooks: workflow.WorkflowHooks{
			WorkflowStart: workflow.Hook{{Log: "Starting workflow"}},
			WorkflowEnd:   workflow.Hook{{Log: "Completed"}},
			WorkflowError: workflow.Hook{{Log: "Error: {{error.message}}"}},
		},
	}

	if len(wf.Hooks.WorkflowStart) != 1 {
		t.Errorf("expected 1 workflow_start hook, got %d", len(wf.Hooks.WorkflowStart))
	}
	if wf.Hooks.WorkflowStart[0].Log != "Starting workflow" {
		t.Errorf("expected log message 'Starting workflow', got '%s'", wf.Hooks.WorkflowStart[0].Log)
	}
}

func TestWorkflowWithEnv(t *testing.T) {
	wf := workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
		Env: []string{"API_KEY", "DATABASE_URL"},
	}

	if len(wf.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(wf.Env))
	}
	if wf.Env[0] != "API_KEY" {
		t.Errorf("expected first env 'API_KEY', got '%s'", wf.Env[0])
	}
}

func TestInputWithValidation(t *testing.T) {
	minVal := 1
	maxVal := 100
	input := workflow.Input{
		Name:     "count",
		Type:     "integer",
		Required: true,
		Validation: &workflow.InputValidation{
			Min: &minVal,
			Max: &maxVal,
		},
	}

	if input.Validation == nil {
		t.Fatal("expected Validation to be set")
	}
	if *input.Validation.Min != 1 {
		t.Errorf("expected Min 1, got %d", *input.Validation.Min)
	}
	if *input.Validation.Max != 100 {
		t.Errorf("expected Max 100, got %d", *input.Validation.Max)
	}
}

func TestInputWithPatternValidation(t *testing.T) {
	input := workflow.Input{
		Name:     "email",
		Type:     "string",
		Required: true,
		Validation: &workflow.InputValidation{
			Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
		},
	}

	if input.Validation.Pattern == "" {
		t.Error("expected Pattern to be set")
	}
}

func TestInputWithEnumValidation(t *testing.T) {
	input := workflow.Input{
		Name:     "environment",
		Type:     "string",
		Required: true,
		Validation: &workflow.InputValidation{
			Enum: []string{"dev", "staging", "prod"},
		},
	}

	if len(input.Validation.Enum) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(input.Validation.Enum))
	}
}
