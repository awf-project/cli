package repository

import (
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// normalizeOnFailure Tests - String Form (Backward Compatibility - US2)

func TestNormalizeOnFailure_StringForm(t *testing.T) {
	tests := []struct {
		name      string
		onFailure any
		want      string
	}{
		{
			name:      "named terminal reference passes through",
			onFailure: "error_terminal",
			want:      "error_terminal",
		},
		{
			name:      "nil on_failure returns empty string",
			onFailure: nil,
			want:      "",
		},
		{
			name:      "empty string passes through",
			onFailure: "",
			want:      "",
		},
		{
			name:      "step name with underscore passes through",
			onFailure: "deploy_error",
			want:      "deploy_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeOnFailure("test.yaml", "build_step", tt.onFailure)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// normalizeOnFailure Tests - Inline Error Object (US1, FR-001, FR-002)

func TestNormalizeOnFailure_InlineMessageOnly(t *testing.T) {
	inlineError := map[string]any{
		"message": "Deploy failed",
	}

	got, err := normalizeOnFailure("test.yaml", "deploy_step", inlineError)

	require.NoError(t, err)
	assert.Equal(t, "__inline_error_deploy_step", got)
}

func TestNormalizeOnFailure_InlineMessageAndStatus(t *testing.T) {
	inlineError := map[string]any{
		"message": "Build failed",
		"status":  3,
	}

	got, err := normalizeOnFailure("test.yaml", "build_step", inlineError)

	require.NoError(t, err)
	assert.Equal(t, "__inline_error_build_step", got)
}

func TestNormalizeOnFailure_InlineWithInterpolationTemplate(t *testing.T) {
	// ADR-003: template must be stored raw, not interpolated at parse time
	inlineError := map[string]any{
		"message": "{{states.build.Output}} failed",
	}

	got, err := normalizeOnFailure("test.yaml", "build_step", inlineError)

	require.NoError(t, err)
	assert.Equal(t, "__inline_error_build_step", got)
}

// normalizeOnFailure Tests - Validation Errors (US3, FR-006)

func TestNormalizeOnFailure_InlineMissingMessage(t *testing.T) {
	inlineError := map[string]any{
		"status": 3,
	}

	_, err := normalizeOnFailure("workflow.yaml", "deploy_step", inlineError)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deploy_step")
	assert.Contains(t, err.Error(), "message")
}

func TestNormalizeOnFailure_InlineEmptyMessage(t *testing.T) {
	inlineError := map[string]any{
		"message": "",
		"status":  3,
	}

	_, err := normalizeOnFailure("workflow.yaml", "build_step", inlineError)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "build_step")
	assert.Contains(t, err.Error(), "message")
}

func TestNormalizeOnFailure_InlineEmptyObject(t *testing.T) {
	inlineError := map[string]any{}

	_, err := normalizeOnFailure("workflow.yaml", "checkout_step", inlineError)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checkout_step")
	assert.Contains(t, err.Error(), "message")
}

// validateInlineErrorObject Tests - Error Messages (NFR-003)

func TestValidateInlineErrorObject_IncludesFieldPath(t *testing.T) {
	tests := []struct {
		name        string
		obj         map[string]any
		wantErrText string
	}{
		{
			name:        "missing message field includes on_failure.message path",
			obj:         map[string]any{"status": 1},
			wantErrText: "on_failure.message",
		},
		{
			name:        "empty message includes on_failure.message path",
			obj:         map[string]any{"message": ""},
			wantErrText: "on_failure.message",
		},
		{
			name:        "empty object includes on_failure.message path",
			obj:         map[string]any{},
			wantErrText: "on_failure.message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInlineErrorObject("workflow.yaml", "my_step", tt.obj)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrText)
		})
	}
}

func TestValidateInlineErrorObject_ValidObject(t *testing.T) {
	tests := []struct {
		name string
		obj  map[string]any
	}{
		{
			name: "message only is valid",
			obj:  map[string]any{"message": "Deploy failed"},
		},
		{
			name: "message and status is valid",
			obj:  map[string]any{"message": "Build failed", "status": 3},
		},
		{
			name: "message with template is valid",
			obj:  map[string]any{"message": "{{states.build.output}} failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInlineErrorObject("workflow.yaml", "my_step", tt.obj)

			require.NoError(t, err)
		})
	}
}

// synthesizeInlineErrorTerminal Tests

func TestSynthesizeInlineErrorTerminal_MessageOnly(t *testing.T) {
	obj := map[string]any{
		"message": "Deploy failed",
	}

	got, err := synthesizeInlineErrorTerminal("deploy_step", obj)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "terminal", got.Type)
	assert.Equal(t, "failure", got.Status)
	assert.Equal(t, "Deploy failed", got.Message)
	// FR-004: no status → default exit code 1
	assert.Equal(t, 1, got.ExitCode)
}

func TestSynthesizeInlineErrorTerminal_MessageAndStatus(t *testing.T) {
	obj := map[string]any{
		"message": "Build failed",
		"status":  3,
	}

	got, err := synthesizeInlineErrorTerminal("build_step", obj)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "terminal", got.Type)
	assert.Equal(t, "failure", got.Status)
	assert.Equal(t, "Build failed", got.Message)
	// FR-004: status 3 → ExitCode 3
	assert.Equal(t, 3, got.ExitCode)
}

func TestSynthesizeInlineErrorTerminal_DefaultStatusIsFailure(t *testing.T) {
	// FR-004: When status omitted, default to failure
	obj := map[string]any{
		"message": "Something went wrong",
	}

	got, err := synthesizeInlineErrorTerminal("step1", obj)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "failure", got.Status)
}

// TestSynthesizeInlineErrorTerminal_StatusMapsToExitCode verifies FR-004: status field → ExitCode.
func TestSynthesizeInlineErrorTerminal_StatusMapsToExitCode(t *testing.T) {
	tests := []struct {
		name         string
		obj          map[string]any
		wantExitCode int
	}{
		{
			name:         "status 1 maps to ExitCode 1",
			obj:          map[string]any{"message": "Build failed", "status": 1},
			wantExitCode: 1,
		},
		{
			name:         "status 3 maps to ExitCode 3",
			obj:          map[string]any{"message": "Execution failed", "status": 3},
			wantExitCode: 3,
		},
		{
			name:         "status 4 maps to ExitCode 4",
			obj:          map[string]any{"message": "System error", "status": 4},
			wantExitCode: 4,
		},
		{
			name:         "float64 status (JSON round-trip) maps to ExitCode",
			obj:          map[string]any{"message": "Float status", "status": float64(2)},
			wantExitCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := synthesizeInlineErrorTerminal("my_step", tt.obj)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantExitCode, got.ExitCode)
		})
	}
}

// TestSynthesizeInlineErrorTerminal_DefaultExitCode1 verifies FR-004: missing status → ExitCode 1.
func TestSynthesizeInlineErrorTerminal_DefaultExitCode1(t *testing.T) {
	obj := map[string]any{
		"message": "Something failed",
	}

	got, err := synthesizeInlineErrorTerminal("step1", obj)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, got.ExitCode, "omitted status must default to ExitCode 1 per FR-004")
}

func TestSynthesizeInlineErrorTerminal_TemplateMessagePreservedRaw(t *testing.T) {
	// ADR-003: message templates stored raw, not interpolated at parse time
	obj := map[string]any{
		"message": "{{states.build.Output}} failed",
	}

	got, err := synthesizeInlineErrorTerminal("build_step", obj)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "{{states.build.Output}} failed", got.Message)
}

// mapToDomain Integration — synthesized terminals injected into wf.Steps (FR-002)

func TestMapToDomain_InlineOnFailure_SynthesizesTerminal(t *testing.T) {
	y := &yamlWorkflow{
		Name: "test-workflow",
		States: yamlStates{
			Initial: "build",
			Steps: map[string]yamlStep{
				"build": {
					Type:    "step",
					Command: "make build",
					OnFailure: map[string]any{
						"message": "Build failed",
					},
				},
			},
		},
	}

	wf, err := mapToDomain("test.yaml", y)

	require.NoError(t, err)
	require.NotNil(t, wf)

	// The build step's OnFailure must be the synthesized name
	buildStep, ok := wf.Steps["build"]
	require.True(t, ok, "build step must exist")
	assert.Equal(t, "__inline_error_build", buildStep.OnFailure)

	// The synthesized terminal must be injected into wf.Steps
	synth, ok := wf.Steps["__inline_error_build"]
	require.True(t, ok, "synthesized terminal must exist in wf.Steps")
	assert.Equal(t, workflow.StepTypeTerminal, synth.Type)
	assert.Equal(t, workflow.TerminalFailure, synth.Status)
	assert.Equal(t, "Build failed", synth.Message)
}

func TestMapToDomain_InlineOnFailure_MessageAndStatus(t *testing.T) {
	// Inline object with explicit status — synthesized terminal uses failure status and propagates ExitCode
	y := &yamlWorkflow{
		Name: "test-workflow",
		States: yamlStates{
			Initial: "deploy",
			Steps: map[string]yamlStep{
				"deploy": {
					Type:    "step",
					Command: "deploy.sh",
					OnFailure: map[string]any{
						"message": "Deploy failed",
						"status":  3,
					},
				},
			},
		},
	}

	wf, err := mapToDomain("test.yaml", y)

	require.NoError(t, err)
	require.NotNil(t, wf)

	deployStep, ok := wf.Steps["deploy"]
	require.True(t, ok)
	assert.Equal(t, "__inline_error_deploy", deployStep.OnFailure)

	synth, ok := wf.Steps["__inline_error_deploy"]
	require.True(t, ok, "synthesized terminal must exist")
	assert.Equal(t, workflow.TerminalFailure, synth.Status)
	assert.Equal(t, "Deploy failed", synth.Message)
	// FR-004: status 3 must propagate as ExitCode 3
	assert.Equal(t, 3, synth.ExitCode)
}

func TestMapToDomain_StringOnFailure_PreservesExistingBehavior(t *testing.T) {
	// US2: string-form on_failure must not be affected by F066 changes
	y := &yamlWorkflow{
		Name: "test-workflow",
		States: yamlStates{
			Initial: "build",
			Steps: map[string]yamlStep{
				"build": {
					Type:      "step",
					Command:   "make build",
					OnFailure: "error_terminal",
				},
				"error_terminal": {
					Type:   "terminal",
					Status: "failure",
				},
			},
		},
	}

	wf, err := mapToDomain("test.yaml", y)

	require.NoError(t, err)
	require.NotNil(t, wf)

	buildStep, ok := wf.Steps["build"]
	require.True(t, ok)
	assert.Equal(t, "error_terminal", buildStep.OnFailure)

	// No extra synthesized steps injected for string-form
	for name := range wf.Steps {
		assert.False(t, len(name) > len("__inline_error_") && name[:len("__inline_error_")] == "__inline_error_",
			"no synthesized terminal should exist for string-form on_failure, got: %s", name)
	}
}

func TestMapToDomain_InlineOnFailure_ValidationError(t *testing.T) {
	// Missing message → parse error returned from mapToDomain
	y := &yamlWorkflow{
		Name: "test-workflow",
		States: yamlStates{
			Initial: "build",
			Steps: map[string]yamlStep{
				"build": {
					Type:    "step",
					Command: "make build",
					OnFailure: map[string]any{
						"status": 3,
					},
				},
			},
		},
	}

	_, err := mapToDomain("test.yaml", y)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "build")
	assert.Contains(t, err.Error(), "message")
}

func TestMapToDomain_InlineOnFailure_EmptyMessage_ValidationError(t *testing.T) {
	// Empty message → parse error
	y := &yamlWorkflow{
		Name: "test-workflow",
		States: yamlStates{
			Initial: "deploy",
			Steps: map[string]yamlStep{
				"deploy": {
					Type:      "step",
					Command:   "deploy.sh",
					OnFailure: map[string]any{"message": ""},
				},
			},
		},
	}

	_, err := mapToDomain("test.yaml", y)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deploy")
}

func TestMapToDomain_InlineOnFailure_MultipleSteps(t *testing.T) {
	// Multiple steps each with inline on_failure → distinct synthesized terminals
	y := &yamlWorkflow{
		Name: "test-workflow",
		States: yamlStates{
			Initial: "build",
			Steps: map[string]yamlStep{
				"build": {
					Type:      "step",
					Command:   "make build",
					OnSuccess: "deploy",
					OnFailure: map[string]any{"message": "Build failed"},
				},
				"deploy": {
					Type:      "step",
					Command:   "deploy.sh",
					OnFailure: map[string]any{"message": "Deploy failed"},
				},
			},
		},
	}

	wf, err := mapToDomain("test.yaml", y)

	require.NoError(t, err)
	require.NotNil(t, wf)

	buildStep, ok := wf.Steps["build"]
	require.True(t, ok)
	assert.Equal(t, "__inline_error_build", buildStep.OnFailure)

	deployStep, ok := wf.Steps["deploy"]
	require.True(t, ok)
	assert.Equal(t, "__inline_error_deploy", deployStep.OnFailure)

	// Both synthesized terminals exist as distinct entries
	_, hasBuildSynth := wf.Steps["__inline_error_build"]
	_, hasDeploySynth := wf.Steps["__inline_error_deploy"]
	assert.True(t, hasBuildSynth, "synthesized terminal for build must exist")
	assert.True(t, hasDeploySynth, "synthesized terminal for deploy must exist")
}

// mapStep Tests - OnFailure Field Backward Compatibility

func TestMapStep_OnFailure_StringForm(t *testing.T) {
	// US2: string on_failure passes through unchanged
	y := &yamlStep{
		Type:      "step",
		Command:   "echo hello",
		OnFailure: "named_error_terminal",
	}

	got, err := mapStep("test.yaml", "my_step", y)

	require.NoError(t, err)
	assert.Equal(t, "named_error_terminal", got.OnFailure)
}

func TestMapStep_OnFailure_NilValue(t *testing.T) {
	// nil on_failure → empty OnFailure on domain step
	y := &yamlStep{
		Type:      "step",
		Command:   "echo hello",
		OnFailure: nil,
	}

	got, err := mapStep("test.yaml", "my_step", y)

	require.NoError(t, err)
	assert.Equal(t, "", got.OnFailure)
}

func TestNormalizeOnFailure_InvalidType(t *testing.T) {
	// on_failure with an unsupported type (e.g., integer) → parse error
	_, err := normalizeOnFailure("workflow.yaml", "my_step", 42)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a string or object")
}
