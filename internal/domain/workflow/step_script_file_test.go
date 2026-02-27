package workflow_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func TestStepScriptFileValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid command step with script_file only",
			step: workflow.Step{
				Name:       "run_script",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/deploy.sh",
			},
			wantErr: false,
		},
		{
			name: "valid command step with command only",
			step: workflow.Step{
				Name:    "run_command",
				Type:    workflow.StepTypeCommand,
				Command: "echo hello",
			},
			wantErr: false,
		},
		{
			name: "invalid - both command and script_file specified",
			step: workflow.Step{
				Name:       "invalid_both",
				Type:       workflow.StepTypeCommand,
				Command:    "echo hello",
				ScriptFile: "scripts/test.sh",
			},
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name: "invalid - neither command nor script_file specified",
			step: workflow.Step{
				Name: "invalid_neither",
				Type: workflow.StepTypeCommand,
			},
			wantErr: true,
			errMsg:  "either command or script_file is required",
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

func TestStepScriptFileEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "script_file with absolute path",
			step: workflow.Step{
				Name:       "absolute_path",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "/usr/local/bin/deploy.sh",
			},
			wantErr: false,
		},
		{
			name: "script_file with relative path",
			step: workflow.Step{
				Name:       "relative_path",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "../scripts/build.sh",
			},
			wantErr: false,
		},
		{
			name: "script_file with interpolated path",
			step: workflow.Step{
				Name:       "interpolated_path",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "{{.awf.scripts_dir}}/deploy.sh",
			},
			wantErr: false,
		},
		{
			name: "script_file with tilde path",
			step: workflow.Step{
				Name:       "tilde_path",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "~/scripts/cleanup.sh",
			},
			wantErr: false,
		},
		{
			name: "script_file with empty string (same as not specified)",
			step: workflow.Step{
				Name:       "empty_script_file",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "",
			},
			wantErr: true,
			errMsg:  "either command or script_file is required",
		},
		{
			name: "command with empty string (same as not specified)",
			step: workflow.Step{
				Name:    "empty_command",
				Type:    workflow.StepTypeCommand,
				Command: "",
			},
			wantErr: true,
			errMsg:  "either command or script_file is required",
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

func TestStepScriptFileWithOtherFields(t *testing.T) {
	tests := []struct {
		name string
		step workflow.Step
		want string
	}{
		{
			name: "script_file with dir field",
			step: workflow.Step{
				Name:       "script_with_dir",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/deploy.sh",
				Dir:        "/tmp/workspace",
			},
			want: "/tmp/workspace",
		},
		{
			name: "script_file with timeout",
			step: workflow.Step{
				Name:       "script_with_timeout",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/slow.sh",
				Timeout:    300,
			},
			want: "",
		},
		{
			name: "script_file with retry config",
			step: workflow.Step{
				Name:       "script_with_retry",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/flaky.sh",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    3,
					InitialDelayMs: 1000,
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate(nil)
			if err != nil {
				t.Errorf("Step.Validate() unexpected error = %v", err)
			}
			if tt.want != "" && tt.step.Dir != tt.want {
				t.Errorf("Step.Dir = %q, want %q", tt.step.Dir, tt.want)
			}
			if tt.step.ScriptFile == "" {
				t.Error("ScriptFile should not be empty")
			}
		})
	}
}

func TestStepScriptFileOnlyForCommandType(t *testing.T) {
	nonCommandTypes := []workflow.StepType{
		workflow.StepTypeParallel,
		workflow.StepTypeTerminal,
		workflow.StepTypeForEach,
		workflow.StepTypeWhile,
		workflow.StepTypeOperation,
		workflow.StepTypeCallWorkflow,
		workflow.StepTypeAgent,
	}

	for _, stepType := range nonCommandTypes {
		t.Run("script_file ignored on "+stepType.String(), func(t *testing.T) {
			step := workflow.Step{
				Name:       "test_step",
				Type:       stepType,
				ScriptFile: "scripts/ignored.sh",
			}

			switch stepType {
			case workflow.StepTypeParallel:
				step.Branches = []string{"step1"}
			case workflow.StepTypeForEach:
				step.Loop = &workflow.LoopConfig{
					Type:  workflow.LoopTypeForEach,
					Items: "{{inputs.items}}",
					Body:  []string{"item_step"},
				}
			case workflow.StepTypeWhile:
				step.Loop = &workflow.LoopConfig{
					Type:      workflow.LoopTypeWhile,
					Condition: "{{loop.iteration}} < 5",
					Body:      []string{"loop_step"},
				}
			case workflow.StepTypeOperation:
				step.Operation = "test.operation"
			case workflow.StepTypeCallWorkflow:
				step.CallWorkflow = &workflow.CallWorkflowConfig{
					Workflow: "sub_workflow",
				}
			case workflow.StepTypeAgent:
				step.Agent = &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "test prompt",
				}
			case workflow.StepTypeCommand, workflow.StepTypeTerminal:
				// No additional fields needed for these types in this test
			}

			err := step.Validate(nil)
			if stepType == workflow.StepTypeTerminal {
				if err != nil {
					t.Errorf("Terminal step should validate even with ScriptFile: %v", err)
				}
			} else if err != nil {
				t.Errorf("Step with type %s should validate (ScriptFile should be ignored): %v", stepType, err)
			}
		})
	}
}
