package builders

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func TestWorkflowBuilder_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		builder  *WorkflowBuilder
		validate func(*testing.T, *workflow.Workflow)
	}{
		{
			name:    "default builder returns valid workflow",
			builder: NewWorkflowBuilder(),
			validate: func(t *testing.T, wf *workflow.Workflow) {
				if wf.Name != "test-workflow" {
					t.Errorf("expected name 'test-workflow', got %q", wf.Name)
				}
				if wf.Initial != "start" {
					t.Errorf("expected initial 'start', got %q", wf.Initial)
				}
				if len(wf.Steps) != 1 {
					t.Errorf("expected 1 step, got %d", len(wf.Steps))
				}
				if _, ok := wf.Steps["start"]; !ok {
					t.Error("expected 'start' step to exist")
				}
			},
		},
		{
			name: "builder with custom name",
			builder: NewWorkflowBuilder().
				WithName("my-workflow"),
			validate: func(t *testing.T, wf *workflow.Workflow) {
				if wf.Name != "my-workflow" {
					t.Errorf("expected name 'my-workflow', got %q", wf.Name)
				}
			},
		},
		{
			name: "builder with all metadata",
			builder: NewWorkflowBuilder().
				WithName("full-workflow").
				WithDescription("Test workflow").
				WithVersion("1.0.0").
				WithAuthor("test-author").
				WithTags("test", "example"),
			validate: func(t *testing.T, wf *workflow.Workflow) {
				if wf.Name != "full-workflow" {
					t.Errorf("expected name 'full-workflow', got %q", wf.Name)
				}
				if wf.Description != "Test workflow" {
					t.Errorf("expected description 'Test workflow', got %q", wf.Description)
				}
				if wf.Version != "1.0.0" {
					t.Errorf("expected version '1.0.0', got %q", wf.Version)
				}
				if wf.Author != "test-author" {
					t.Errorf("expected author 'test-author', got %q", wf.Author)
				}
				if len(wf.Tags) != 2 {
					t.Errorf("expected 2 tags, got %d", len(wf.Tags))
				}
			},
		},
		{
			name: "builder with custom steps",
			builder: NewWorkflowBuilder().
				WithInitial("step1").
				WithStep(&workflow.Step{
					Name:    "step1",
					Type:    workflow.StepTypeCommand,
					Command: "echo one",
				}).
				WithStep(&workflow.Step{
					Name:   "step2",
					Type:   workflow.StepTypeTerminal,
					Status: workflow.TerminalSuccess,
				}),
			validate: func(t *testing.T, wf *workflow.Workflow) {
				if wf.Initial != "step1" {
					t.Errorf("expected initial 'step1', got %q", wf.Initial)
				}
				// Default builder has "start" step, plus 2 custom steps = 3 total
				if len(wf.Steps) != 3 {
					t.Errorf("expected 3 steps (including default start), got %d", len(wf.Steps))
				}
				if _, ok := wf.Steps["step1"]; !ok {
					t.Error("expected 'step1' to exist")
				}
				if _, ok := wf.Steps["step2"]; !ok {
					t.Error("expected 'step2' to exist")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := tt.builder.Build()
			tt.validate(t, wf)
		})
	}
}

func TestWorkflowBuilder_FluentAPI(t *testing.T) {
	t.Run("chaining returns same builder", func(t *testing.T) {
		b := NewWorkflowBuilder()
		result := b.WithName("test").WithDescription("desc").WithVersion("1.0")
		if result != b {
			t.Error("expected fluent methods to return same builder instance")
		}
	})

	t.Run("builder can be reused", func(t *testing.T) {
		b := NewWorkflowBuilder().WithName("base")
		wf1 := b.WithDescription("first").Build()
		wf2 := b.WithDescription("second").Build()

		// Both should have the same name (base configuration)
		if wf1.Name != "base" || wf2.Name != "base" {
			t.Error("expected both workflows to share base configuration")
		}
		// Last description wins
		if wf2.Description != "second" {
			t.Errorf("expected description 'second', got %q", wf2.Description)
		}
	})
}

func TestStepBuilder_AllStepTypes(t *testing.T) {
	tests := []struct {
		name     string
		builder  *StepBuilder
		validate func(*testing.T, *workflow.Step)
	}{
		{
			name:    "command step",
			builder: NewCommandStep("cmd", "echo hello"),
			validate: func(t *testing.T, s *workflow.Step) {
				if s.Name != "cmd" {
					t.Errorf("expected name 'cmd', got %q", s.Name)
				}
				if s.Type != workflow.StepTypeCommand {
					t.Errorf("expected command type, got %v", s.Type)
				}
				if s.Command != "echo hello" {
					t.Errorf("expected command 'echo hello', got %q", s.Command)
				}
			},
		},
		{
			name:    "parallel step",
			builder: NewParallelStep("parallel", "branch1", "branch2"),
			validate: func(t *testing.T, s *workflow.Step) {
				if s.Type != workflow.StepTypeParallel {
					t.Errorf("expected parallel type, got %v", s.Type)
				}
				if len(s.Branches) != 2 {
					t.Errorf("expected 2 branches, got %d", len(s.Branches))
				}
				if s.Strategy != "all_succeed" {
					t.Errorf("expected default strategy 'all_succeed', got %q", s.Strategy)
				}
			},
		},
		{
			name:    "terminal step success",
			builder: NewTerminalStep("end", workflow.TerminalSuccess),
			validate: func(t *testing.T, s *workflow.Step) {
				if s.Type != workflow.StepTypeTerminal {
					t.Errorf("expected terminal type, got %v", s.Type)
				}
				if s.Status != workflow.TerminalSuccess {
					t.Errorf("expected success status, got %v", s.Status)
				}
			},
		},
		{
			name:    "terminal step failure",
			builder: NewTerminalStep("fail", workflow.TerminalFailure),
			validate: func(t *testing.T, s *workflow.Step) {
				if s.Status != workflow.TerminalFailure {
					t.Errorf("expected failure status, got %v", s.Status)
				}
			},
		},
		{
			name: "command step with timeout and description",
			builder: NewCommandStep("cmd", "sleep 10").
				WithTimeout(5).
				WithDescription("Command with timeout"),
			validate: func(t *testing.T, s *workflow.Step) {
				if s.Timeout != 5 {
					t.Errorf("expected timeout 5, got %d", s.Timeout)
				}
				if s.Description != "Command with timeout" {
					t.Errorf("expected description 'Command with timeout', got %q", s.Description)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := tt.builder.Build()
			tt.validate(t, step)
		})
	}
}

func TestStepBuilder_WithMethods(t *testing.T) {
	t.Run("WithDir sets working directory", func(t *testing.T) {
		step := NewCommandStep("cmd", "ls").
			WithDir("/tmp").
			Build()
		if step.Dir != "/tmp" {
			t.Errorf("expected dir '/tmp', got %q", step.Dir)
		}
	})

	t.Run("WithStrategy sets parallel strategy", func(t *testing.T) {
		step := NewParallelStep("parallel", "b1", "b2").
			WithStrategy("any_succeed").
			Build()
		if step.Strategy != "any_succeed" {
			t.Errorf("expected strategy 'any_succeed', got %q", step.Strategy)
		}
	})

	t.Run("WithMaxConcurrent sets concurrency limit", func(t *testing.T) {
		step := NewParallelStep("parallel", "b1", "b2", "b3").
			WithMaxConcurrent(2).
			Build()
		if step.MaxConcurrent != 2 {
			t.Errorf("expected max concurrent 2, got %d", step.MaxConcurrent)
		}
	})

	t.Run("WithOnSuccess and WithOnFailure set transitions", func(t *testing.T) {
		step := NewCommandStep("cmd", "test").
			WithOnSuccess("success_state").
			WithOnFailure("failure_state").
			Build()
		if step.OnSuccess != "success_state" {
			t.Errorf("expected on_success 'success_state', got %q", step.OnSuccess)
		}
		if step.OnFailure != "failure_state" {
			t.Errorf("expected on_failure 'failure_state', got %q", step.OnFailure)
		}
	})

	t.Run("WithContinueOnError sets flag", func(t *testing.T) {
		step := NewCommandStep("cmd", "test").
			WithContinueOnError(true).
			Build()
		if !step.ContinueOnError {
			t.Error("expected continue_on_error to be true")
		}
	})

	t.Run("WithOperation sets operation details", func(t *testing.T) {
		inputs := map[string]any{"key": "value"}
		step := NewStepBuilder("op").
			WithType(workflow.StepTypeOperation).
			WithOperation("slack.send", inputs).
			Build()
		if step.Operation != "slack.send" {
			t.Errorf("expected operation 'slack.send', got %q", step.Operation)
		}
		if len(step.OperationInputs) != 1 {
			t.Errorf("expected 1 input, got %d", len(step.OperationInputs))
		}
	})

	t.Run("WithDependsOn sets dependencies", func(t *testing.T) {
		step := NewCommandStep("cmd", "test").
			WithDependsOn("step1", "step2").
			Build()
		if len(step.DependsOn) != 2 {
			t.Errorf("expected 2 dependencies, got %d", len(step.DependsOn))
		}
	})
}

func TestWorkflowBuilder_WithSteps(t *testing.T) {
	step1 := NewCommandStep("step1", "echo one").Build()
	step2 := NewCommandStep("step2", "echo two").Build()
	step3 := NewTerminalStep("end", workflow.TerminalSuccess).Build()

	wf := NewWorkflowBuilder().
		WithName("multi-step").
		WithInitial("step1").
		WithSteps(step1, step2, step3).
		Build()

	// Default builder has "start" step, plus 3 custom steps = 4 total
	if len(wf.Steps) != 4 {
		t.Errorf("expected 4 steps (including default start), got %d", len(wf.Steps))
	}
	if _, ok := wf.Steps["step1"]; !ok {
		t.Error("expected step1 to exist")
	}
	if _, ok := wf.Steps["step2"]; !ok {
		t.Error("expected step2 to exist")
	}
	if _, ok := wf.Steps["end"]; !ok {
		t.Error("expected end step to exist")
	}
}

func TestWorkflowBuilder_WithInput(t *testing.T) {
	input := &workflow.Input{
		Name:        "test_input",
		Type:        "string",
		Description: "Test input parameter",
		Required:    true,
	}

	wf := NewWorkflowBuilder().
		WithInput(input).
		Build()

	if len(wf.Inputs) != 1 {
		t.Errorf("expected 1 input, got %d", len(wf.Inputs))
	}
	if wf.Inputs[0].Name != "test_input" {
		t.Errorf("expected input name 'test_input', got %q", wf.Inputs[0].Name)
	}
}

func TestWorkflowBuilder_WithEnv(t *testing.T) {
	wf := NewWorkflowBuilder().
		WithEnv("VAR1", "VAR2", "VAR3").
		Build()

	if len(wf.Env) != 3 {
		t.Errorf("expected 3 env vars, got %d", len(wf.Env))
	}
}
