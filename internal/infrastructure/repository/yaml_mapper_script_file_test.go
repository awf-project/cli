package repository

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapStep_ScriptFile_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.Step
	}{
		{
			name: "relative path to script file",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts/deploy.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/deploy.sh",
			},
		},
		{
			name: "tilde expansion in script file path",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "~/scripts/build.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "~/scripts/build.sh",
			},
		},
		{
			name: "awf template variable in script file path",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "{{.awf.scripts_dir}}/checks/lint.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "{{.awf.scripts_dir}}/checks/lint.sh",
			},
		},
		{
			name: "absolute path to script file",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "/opt/company/scripts/deploy.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "/opt/company/scripts/deploy.sh",
			},
		},
		{
			name: "script file with nested directory structure",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts/ci/integration/test.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/ci/integration/test.sh",
			},
		},
		{
			name: "script file with template interpolation in path",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "{{.awf.config_dir}}/scripts/{{.inputs.environment}}.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "{{.awf.config_dir}}/scripts/{{.inputs.environment}}.sh",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.ScriptFile, got.ScriptFile)
			assert.Equal(t, tt.want.Command, got.Command)
		})
	}
}

// mapStep Tests - ScriptFile Edge Cases

func TestMapStep_ScriptFile_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.Step
	}{
		{
			name: "empty script file field",
			yamlStep: yamlStep{
				Type:       "step",
				Command:    "echo 'Inline command'",
				ScriptFile: "",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				Command:    "echo 'Inline command'",
				ScriptFile: "",
			},
		},
		{
			name: "script file with only filename no extension",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "deploy",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "deploy",
			},
		},
		{
			name: "script file with special characters in path",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts/my-script_v2.0.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/my-script_v2.0.sh",
			},
		},
		{
			name: "script file with spaces in path",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts/my script file.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/my script file.sh",
			},
		},
		{
			name: "script file with unicode characters in path",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts/インストール手順.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/インストール手順.sh",
			},
		},
		{
			name: "script file with multiple path separators",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts//subdir///file.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts//subdir///file.sh",
			},
		},
		{
			name: "script file with dot references",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "../scripts/deploy.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "../scripts/deploy.sh",
			},
		},
		{
			name: "script file with environment variable syntax",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "$HOME/scripts/build.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "$HOME/scripts/build.sh",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.ScriptFile, got.ScriptFile)
			assert.Equal(t, tt.want.Command, got.Command)
		})
	}
}

// mapStep Tests - ScriptFile Mutual Exclusivity with Command

func TestMapStep_ScriptFile_MutualExclusivity(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.Step
	}{
		{
			name: "both command and script file set",
			yamlStep: yamlStep{
				Type:       "step",
				Command:    "echo 'Inline command'",
				ScriptFile: "scripts/deploy.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				Command:    "echo 'Inline command'",
				ScriptFile: "scripts/deploy.sh",
			},
		},
		{
			name: "only command set",
			yamlStep: yamlStep{
				Type:    "step",
				Command: "echo 'Inline command'",
			},
			want: &workflow.Step{
				Type:    workflow.StepTypeCommand,
				Command: "echo 'Inline command'",
			},
		},
		{
			name: "only script file set",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts/deploy.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/deploy.sh",
			},
		},
		{
			name: "neither command nor script file set",
			yamlStep: yamlStep{
				Type: "step",
			},
			want: &workflow.Step{
				Type: workflow.StepTypeCommand,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Command, got.Command)
			assert.Equal(t, tt.want.ScriptFile, got.ScriptFile)
		})
	}
}

// mapStep Tests - ScriptFile with Other Step Attributes

func TestMapStep_ScriptFile_WithOtherAttributes(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.Step
	}{
		{
			name: "script file with timeout and dir",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts/build.sh",
				Dir:        "/workspace",
				Timeout:    "5m",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/build.sh",
				Dir:        "/workspace",
				Timeout:    300,
			},
		},
		{
			name: "script file with transitions",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts/deploy.sh",
				OnSuccess:  "notify",
				OnFailure:  "cleanup",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/deploy.sh",
				OnSuccess:  "notify",
				OnFailure:  "cleanup",
			},
		},
		{
			name: "script file with retry configuration",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts/test.sh",
				Retry: &yamlRetry{
					MaxAttempts:  3,
					InitialDelay: "5s",
				},
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/test.sh",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    3,
					InitialDelayMs: 5000,
				},
			},
		},
		{
			name: "script file with capture",
			yamlStep: yamlStep{
				Type:       "step",
				ScriptFile: "scripts/parse.sh",
				Capture: &yamlCapture{
					Stdout: "result",
				},
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/parse.sh",
				Capture: &workflow.CaptureConfig{
					Stdout: "result",
				},
			},
		},
		{
			name: "script file with continue on error",
			yamlStep: yamlStep{
				Type:            "step",
				ScriptFile:      "scripts/optional.sh",
				ContinueOnError: true,
			},
			want: &workflow.Step{
				Type:            workflow.StepTypeCommand,
				ScriptFile:      "scripts/optional.sh",
				ContinueOnError: true,
			},
		},
		{
			name: "script file with description",
			yamlStep: yamlStep{
				Type:        "step",
				Description: "Deploy to production",
				ScriptFile:  "scripts/deploy.sh",
			},
			want: &workflow.Step{
				Type:        workflow.StepTypeCommand,
				Description: "Deploy to production",
				ScriptFile:  "scripts/deploy.sh",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapStep("test.yaml", "test_step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.ScriptFile, got.ScriptFile)
			assert.Equal(t, tt.want.Description, got.Description)
			assert.Equal(t, tt.want.Dir, got.Dir)
			assert.Equal(t, tt.want.Timeout, got.Timeout)
			assert.Equal(t, tt.want.OnSuccess, got.OnSuccess)
			assert.Equal(t, tt.want.OnFailure, got.OnFailure)
			assert.Equal(t, tt.want.ContinueOnError, got.ContinueOnError)

			if tt.want.Retry != nil {
				require.NotNil(t, got.Retry)
				assert.Equal(t, tt.want.Retry.MaxAttempts, got.Retry.MaxAttempts)
				assert.Equal(t, tt.want.Retry.InitialDelayMs, got.Retry.InitialDelayMs)
			}

			if tt.want.Capture != nil {
				require.NotNil(t, got.Capture)
				assert.Equal(t, tt.want.Capture.Stdout, got.Capture.Stdout)
			}
		})
	}
}

// mapStep Tests - ScriptFile with Different Step Types

func TestMapStep_ScriptFile_WithDifferentStepTypes(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.Step
	}{
		{
			name: "command step with script file",
			yamlStep: yamlStep{
				Type:       "command",
				ScriptFile: "scripts/build.sh",
			},
			want: &workflow.Step{
				Type:       workflow.StepTypeCommand,
				ScriptFile: "scripts/build.sh",
			},
		},
		{
			name: "parallel step does not use script file",
			yamlStep: yamlStep{
				Type:     "parallel",
				Strategy: "all_succeed",
				Parallel: []string{"branch1", "branch2"},
			},
			want: &workflow.Step{
				Type:     workflow.StepTypeParallel,
				Strategy: "all_succeed",
				Branches: []string{"branch1", "branch2"},
			},
		},
		{
			name: "terminal step does not use script file",
			yamlStep: yamlStep{
				Type:   "terminal",
				Status: "success",
			},
			want: &workflow.Step{
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
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
			assert.Equal(t, tt.want.Strategy, got.Strategy)
			assert.Equal(t, tt.want.Branches, got.Branches)
			assert.Equal(t, tt.want.ScriptFile, got.ScriptFile)
		})
	}
}
