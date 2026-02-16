package builders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStepBuilder_WithScriptFile_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		scriptFile     string
		expectedScript string
	}{
		{
			name:           "sets simple relative path",
			scriptFile:     "scripts/deploy.sh",
			expectedScript: "scripts/deploy.sh",
		},
		{
			name:           "sets path with template interpolation",
			scriptFile:     "{{.awf.scripts_dir}}/build.sh",
			expectedScript: "{{.awf.scripts_dir}}/build.sh",
		},
		{
			name:           "sets absolute path",
			scriptFile:     "/opt/company/scripts/test.sh",
			expectedScript: "/opt/company/scripts/test.sh",
		},
		{
			name:           "sets tilde-prefixed path",
			scriptFile:     "~/scripts/lint.sh",
			expectedScript: "~/scripts/lint.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewStepBuilder("test-step").WithScriptFile(tt.scriptFile)
			step := builder.Build()

			assert.Equal(t, tt.expectedScript, step.ScriptFile, "ScriptFile should match input")
		})
	}
}

func TestStepBuilder_WithScriptFile_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		scriptFile     string
		expectedScript string
	}{
		{
			name:           "handles empty string",
			scriptFile:     "",
			expectedScript: "",
		},
		{
			name:           "handles path with spaces",
			scriptFile:     "my scripts/build script.sh",
			expectedScript: "my scripts/build script.sh",
		},
		{
			name:           "handles path with special chars",
			scriptFile:     "scripts/deploy-v2.0_final.sh",
			expectedScript: "scripts/deploy-v2.0_final.sh",
		},
		{
			name:           "handles deep nested path",
			scriptFile:     "scripts/ci/checks/static/lint.sh",
			expectedScript: "scripts/ci/checks/static/lint.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewStepBuilder("test-step").WithScriptFile(tt.scriptFile)
			step := builder.Build()

			assert.Equal(t, tt.expectedScript, step.ScriptFile)
		})
	}
}

func TestStepBuilder_WithScriptFile_FluentInterface(t *testing.T) {
	builder := NewStepBuilder("deploy").
		WithScriptFile("scripts/deploy.sh").
		WithDir("/tmp/workspace").
		WithTimeout(300)

	step := builder.Build()

	assert.Equal(t, "deploy", step.Name)
	assert.Equal(t, "scripts/deploy.sh", step.ScriptFile)
	assert.Equal(t, "/tmp/workspace", step.Dir)
	assert.Equal(t, 300, step.Timeout)
}

func TestStepBuilder_WithScriptFile_Overwrite(t *testing.T) {
	builder := NewStepBuilder("test-step").
		WithScriptFile("old-script.sh").
		WithScriptFile("new-script.sh")

	step := builder.Build()

	assert.Equal(t, "new-script.sh", step.ScriptFile, "should use last set value")
}

func TestStepBuilder_WithScriptFile_CommandNotSet(t *testing.T) {
	builder := NewStepBuilder("test-step").WithScriptFile("deploy.sh")
	step := builder.Build()

	assert.Equal(t, "deploy.sh", step.ScriptFile)
	assert.Empty(t, step.Command, "Command should remain empty when ScriptFile is set")
}
