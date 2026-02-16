package workflow_test

import (
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
)

func TestDryRunStep_ScriptFileField(t *testing.T) {
	tests := []struct {
		name       string
		step       workflow.DryRunStep
		wantScript string
	}{
		{
			name: "script_file set with relative path",
			step: workflow.DryRunStep{
				Name:       "deploy",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/deploy.sh",
			},
			wantScript: "scripts/deploy.sh",
		},
		{
			name: "script_file set with absolute path",
			step: workflow.DryRunStep{
				Name:       "build",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "/usr/local/bin/build.sh",
			},
			wantScript: "/usr/local/bin/build.sh",
		},
		{
			name: "script_file with template variable",
			step: workflow.DryRunStep{
				Name:       "lint",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "{{.awf.scripts_dir}}/lint.sh",
			},
			wantScript: "{{.awf.scripts_dir}}/lint.sh",
		},
		{
			name: "script_file with tilde path",
			step: workflow.DryRunStep{
				Name:       "cleanup",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "~/scripts/cleanup.sh",
			},
			wantScript: "~/scripts/cleanup.sh",
		},
		{
			name: "empty script_file",
			step: workflow.DryRunStep{
				Name:       "cmd",
				Type:       workflow.StepTypeCommand,
				Command:    "echo hello",
				ScriptFile: "",
			},
			wantScript: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.step.ScriptFile != tt.wantScript {
				t.Errorf("DryRunStep.ScriptFile = %q, want %q", tt.step.ScriptFile, tt.wantScript)
			}
		})
	}
}

func TestDryRunStep_ScriptFileWithOtherFields(t *testing.T) {
	tests := []struct {
		name string
		step workflow.DryRunStep
	}{
		{
			name: "script_file with command creates no conflict in struct",
			step: workflow.DryRunStep{
				Name:       "test",
				Type:       workflow.StepTypeCommand,
				Command:    "echo test",
				ScriptFile: "scripts/test.sh",
			},
		},
		{
			name: "script_file with dir field",
			step: workflow.DryRunStep{
				Name:       "deploy",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/deploy.sh",
				Dir:        "/tmp/workspace",
			},
		},
		{
			name: "script_file with timeout",
			step: workflow.DryRunStep{
				Name:       "slow_script",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/slow.sh",
				Timeout:    300,
			},
		},
		{
			name: "script_file with retry config",
			step: workflow.DryRunStep{
				Name:       "flaky_script",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/flaky.sh",
				Retry: &workflow.DryRunRetry{
					MaxAttempts:    3,
					InitialDelayMs: 1000,
					MaxDelayMs:     5000,
					Backoff:        "exponential",
					Multiplier:     2.0,
				},
			},
		},
		{
			name: "script_file with capture config",
			step: workflow.DryRunStep{
				Name:       "monitored_script",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/monitor.sh",
				Capture: &workflow.DryRunCapture{
					Stdout:  "captured",
					Stderr:  "captured",
					MaxSize: "1024",
				},
			},
		},
		{
			name: "script_file with hooks",
			step: workflow.DryRunStep{
				Name:       "hooked_script",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/main.sh",
				Hooks: workflow.DryRunHooks{
					Pre: []workflow.DryRunHook{
						{Type: "shell", Content: "echo pre"},
					},
					Post: []workflow.DryRunHook{
						{Type: "shell", Content: "echo post"},
					},
				},
			},
		},
		{
			name: "script_file with continue_on_error",
			step: workflow.DryRunStep{
				Name:            "optional_script",
				Type:            workflow.StepTypeCommand,
				ScriptFile:      "scripts/optional.sh",
				ContinueOnError: true,
			},
		},
		{
			name: "script_file with all fields",
			step: workflow.DryRunStep{
				Name:        "complex_script",
				Type:        workflow.StepTypeCommand,
				Description: "Complex script with all options",
				ScriptFile:  "scripts/complex.sh",
				Dir:         "/app",
				Timeout:     600,
				Retry: &workflow.DryRunRetry{
					MaxAttempts:    5,
					InitialDelayMs: 500,
				},
				Capture: &workflow.DryRunCapture{
					Stdout: "captured",
					Stderr: "captured",
				},
				ContinueOnError: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.step.ScriptFile == "" {
				t.Error("ScriptFile should not be empty in this test case")
			}
			if tt.step.Name == "" {
				t.Error("Name should not be empty")
			}
		})
	}
}

func TestDryRunStep_ScriptFileWithDifferentStepTypes(t *testing.T) {
	tests := []struct {
		name     string
		stepType workflow.StepType
		step     workflow.DryRunStep
	}{
		{
			name:     "script_file on command type step",
			stepType: workflow.StepTypeCommand,
			step: workflow.DryRunStep{
				Name:       "cmd_script",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/cmd.sh",
			},
		},
		{
			name:     "script_file on parallel step",
			stepType: workflow.StepTypeParallel,
			step: workflow.DryRunStep{
				Name:       "parallel_script",
				Type:       workflow.StepTypeParallel,
				ScriptFile: "scripts/ignored.sh",
				Branches:   []string{"branch1", "branch2"},
			},
		},
		{
			name:     "script_file on agent step",
			stepType: workflow.StepTypeAgent,
			step: workflow.DryRunStep{
				Name:       "agent_script",
				Type:       workflow.StepTypeAgent,
				ScriptFile: "scripts/ignored.sh",
				Agent: &workflow.DryRunAgent{
					Provider:       "claude",
					ResolvedPrompt: "test prompt",
				},
			},
		},
		{
			name:     "script_file on terminal step",
			stepType: workflow.StepTypeTerminal,
			step: workflow.DryRunStep{
				Name:       "done",
				Type:       workflow.StepTypeTerminal,
				ScriptFile: "scripts/ignored.sh",
				Status:     workflow.TerminalSuccess,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.step.Type != tt.stepType {
				t.Errorf("DryRunStep.Type = %v, want %v", tt.step.Type, tt.stepType)
			}
			if tt.stepType == workflow.StepTypeCommand && tt.step.ScriptFile == "" {
				t.Error("ScriptFile should be set for command type step")
			}
		})
	}
}

func TestDryRunStep_ScriptFileZeroValue(t *testing.T) {
	step := workflow.DryRunStep{}

	if step.ScriptFile != "" {
		t.Errorf("Zero-value DryRunStep.ScriptFile should be empty string, got %q", step.ScriptFile)
	}
}

func TestDryRunStep_ScriptFileAssignment(t *testing.T) {
	step := workflow.DryRunStep{
		Name: "test",
		Type: workflow.StepTypeCommand,
	}

	step.ScriptFile = "scripts/new.sh"

	if step.ScriptFile != "scripts/new.sh" {
		t.Errorf("ScriptFile assignment failed: got %q, want %q", step.ScriptFile, "scripts/new.sh")
	}

	step.ScriptFile = "scripts/updated.sh"

	if step.ScriptFile != "scripts/updated.sh" {
		t.Errorf("ScriptFile reassignment failed: got %q, want %q", step.ScriptFile, "scripts/updated.sh")
	}
}
