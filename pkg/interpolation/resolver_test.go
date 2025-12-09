package interpolation_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/pkg/interpolation"
)

func TestTemplateResolver_Inputs(t *testing.T) {
	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		want     string
		wantErr  bool
	}{
		{
			name:     "simple string input",
			template: "echo {{.inputs.name}}",
			inputs:   map[string]any{"name": "Alice"},
			want:     "echo Alice",
		},
		{
			name:     "integer input",
			template: "count is {{.inputs.count}}",
			inputs:   map[string]any{"count": 42},
			want:     "count is 42",
		},
		{
			name:     "multiple inputs",
			template: "{{.inputs.a}} and {{.inputs.b}}",
			inputs:   map[string]any{"a": "first", "b": "second"},
			want:     "first and second",
		},
		{
			name:     "undefined input",
			template: "{{.inputs.unknown}}",
			inputs:   map[string]any{},
			wantErr:  true,
		},
		{
			name:     "empty string value is valid",
			template: "value=[{{.inputs.empty}}]",
			inputs:   map[string]any{"empty": ""},
			want:     "value=[]",
		},
		{
			name:     "boolean input",
			template: "flag={{.inputs.enabled}}",
			inputs:   map[string]any{"enabled": true},
			want:     "flag=true",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			for k, v := range tt.inputs {
				ctx.Inputs[k] = v
			}

			got, err := resolver.Resolve(tt.template, ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateResolver_States(t *testing.T) {
	tests := []struct {
		name     string
		template string
		states   map[string]interpolation.StepStateData
		want     string
		wantErr  bool
	}{
		{
			name:     "state output",
			template: "result: {{.states.read_file.Output}}",
			states: map[string]interpolation.StepStateData{
				"read_file": {Output: "file content here"},
			},
			want: "result: file content here",
		},
		{
			name:     "state exit code",
			template: "code: {{.states.validate.ExitCode}}",
			states: map[string]interpolation.StepStateData{
				"validate": {ExitCode: 0},
			},
			want: "code: 0",
		},
		{
			name:     "state stderr",
			template: "error: {{.states.cmd.Stderr}}",
			states: map[string]interpolation.StepStateData{
				"cmd": {Stderr: "warning: deprecated"},
			},
			want: "error: warning: deprecated",
		},
		{
			name:     "undefined state",
			template: "{{.states.nonexistent.Output}}",
			states:   map[string]interpolation.StepStateData{},
			wantErr:  true,
		},
		{
			name:     "chained access from prior step",
			template: "previous: {{.states.step1.Output}}",
			states: map[string]interpolation.StepStateData{
				"step1": {Output: "step1 result"},
				"step2": {Output: "step2 result"},
			},
			want: "previous: step1 result",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.States = tt.states

			got, err := resolver.Resolve(tt.template, ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateResolver_Workflow(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		workflow     interpolation.WorkflowData
		want         string
		wantContains string
	}{
		{
			name:     "workflow id",
			template: "id: {{.workflow.ID}}",
			workflow: interpolation.WorkflowData{ID: "abc-123", Name: "test"},
			want:     "id: abc-123",
		},
		{
			name:     "workflow name",
			template: "name: {{.workflow.Name}}",
			workflow: interpolation.WorkflowData{ID: "abc-123", Name: "analyze-code"},
			want:     "name: analyze-code",
		},
		{
			name:     "workflow current state",
			template: "state: {{.workflow.CurrentState}}",
			workflow: interpolation.WorkflowData{CurrentState: "step2"},
			want:     "state: step2",
		},
		{
			name:     "workflow duration contains ms or s",
			template: "duration: {{.workflow.Duration}}",
			workflow: interpolation.WorkflowData{
				StartedAt: time.Now().Add(-100 * time.Millisecond),
			},
			wantContains: "ms",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Workflow = tt.workflow

			got, err := resolver.Resolve(tt.template, ctx)

			require.NoError(t, err)
			if tt.want != "" {
				assert.Equal(t, tt.want, got)
			}
			if tt.wantContains != "" {
				assert.Contains(t, got, tt.wantContains)
			}
		})
	}
}

func TestTemplateResolver_Env(t *testing.T) {
	t.Setenv("TEST_VAR", "test_value")
	t.Setenv("EMPTY_VAR", "")

	tests := []struct {
		name     string
		template string
		env      map[string]string
		want     string
		wantErr  bool
	}{
		{
			name:     "existing env var from context",
			template: "val: {{.env.TEST_VAR}}",
			env:      map[string]string{"TEST_VAR": "test_value"},
			want:     "val: test_value",
		},
		{
			name:     "home dir from context",
			template: "home: {{.env.HOME}}",
			env:      map[string]string{"HOME": "/home/user"},
			want:     "home: /home/user",
		},
		{
			name:     "undefined env var",
			template: "{{.env.NONEXISTENT_VAR_12345}}",
			env:      map[string]string{},
			wantErr:  true,
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Env = tt.env

			got, err := resolver.Resolve(tt.template, ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateResolver_Context(t *testing.T) {
	tests := []struct {
		name     string
		template string
		ctxData  interpolation.ContextData
		want     string
	}{
		{
			name:     "working directory",
			template: "cwd: {{.context.WorkingDir}}",
			ctxData:  interpolation.ContextData{WorkingDir: "/home/user/project"},
			want:     "cwd: /home/user/project",
		},
		{
			name:     "user",
			template: "user: {{.context.User}}",
			ctxData:  interpolation.ContextData{User: "testuser"},
			want:     "user: testuser",
		},
		{
			name:     "hostname",
			template: "host: {{.context.Hostname}}",
			ctxData:  interpolation.ContextData{Hostname: "localhost"},
			want:     "host: localhost",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Context = tt.ctxData

			got, err := resolver.Resolve(tt.template, ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateResolver_Error(t *testing.T) {
	tests := []struct {
		name     string
		template string
		errData  *interpolation.ErrorData
		want     string
		wantErr  bool
	}{
		{
			name:     "error message",
			template: "failed: {{.error.Message}}",
			errData:  &interpolation.ErrorData{Message: "connection timeout"},
			want:     "failed: connection timeout",
		},
		{
			name:     "error type",
			template: "type: {{.error.Type}}",
			errData:  &interpolation.ErrorData{Type: "ExecutionError"},
			want:     "type: ExecutionError",
		},
		{
			name:     "error state",
			template: "at: {{.error.State}}",
			errData:  &interpolation.ErrorData{State: "validate"},
			want:     "at: validate",
		},
		{
			name:     "error exit code",
			template: "code: {{.error.ExitCode}}",
			errData:  &interpolation.ErrorData{ExitCode: 1},
			want:     "code: 1",
		},
		{
			name:     "error without context",
			template: "{{.error.Message}}",
			errData:  nil,
			wantErr:  true,
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Error = tt.errData

			got, err := resolver.Resolve(tt.template, ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateResolver_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		want     string
		wantErr  bool
	}{
		{
			name:     "no variables",
			template: "plain text",
			want:     "plain text",
		},
		{
			name:     "empty template",
			template: "",
			want:     "",
		},
		{
			name:     "unclosed brace",
			template: "{{.inputs.name}",
			wantErr:  true,
		},
		{
			name:     "whitespace preserved",
			template: "  {{.inputs.val}}  ",
			inputs:   map[string]any{"val": "x"},
			want:     "  x  ",
		},
		{
			name:     "newlines in value",
			template: "{{.inputs.multiline}}",
			inputs:   map[string]any{"multiline": "line1\nline2"},
			want:     "line1\nline2",
		},
		{
			name:     "mixed namespaces",
			template: "{{.inputs.a}} - {{.workflow.Name}}",
			inputs:   map[string]any{"a": "value"},
			want:     "mixed namespaces test",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			for k, v := range tt.inputs {
				ctx.Inputs[k] = v
			}
			if tt.name == "mixed namespaces" {
				ctx.Workflow.Name = "test-workflow"
				tt.want = "value - test-workflow"
			}

			got, err := resolver.Resolve(tt.template, ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateResolver_ShellSafety(t *testing.T) {
	// These tests verify the resolver outputs raw values
	// Security is enforced by users calling ShellEscape or using Args[]
	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		want     string
	}{
		{
			name:     "command substitution attempt",
			template: "echo {{.inputs.val}}",
			inputs:   map[string]any{"val": "$(rm -rf /)"},
			want:     "echo $(rm -rf /)",
		},
		{
			name:     "backtick substitution",
			template: "echo {{.inputs.val}}",
			inputs:   map[string]any{"val": "`whoami`"},
			want:     "echo `whoami`",
		},
		{
			name:     "semicolon injection",
			template: "echo {{.inputs.val}}",
			inputs:   map[string]any{"val": "foo; rm -rf /"},
			want:     "echo foo; rm -rf /",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			for k, v := range tt.inputs {
				ctx.Inputs[k] = v
			}

			got, err := resolver.Resolve(tt.template, ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateResolver_WithEscapeFunction(t *testing.T) {
	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		want     string
	}{
		{
			name:     "escape dangerous chars",
			template: "echo {{escape .inputs.val}}",
			inputs:   map[string]any{"val": "$(rm -rf /)"},
			want:     "echo '$(rm -rf /)'",
		},
		{
			name:     "escape single quotes",
			template: "echo {{escape .inputs.val}}",
			inputs:   map[string]any{"val": "it's a test"},
			want:     `echo 'it'\''s a test'`,
		},
		{
			name:     "safe string unchanged",
			template: "echo {{escape .inputs.val}}",
			inputs:   map[string]any{"val": "hello"},
			want:     "echo hello",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			for k, v := range tt.inputs {
				ctx.Inputs[k] = v
			}

			got, err := resolver.Resolve(tt.template, ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorkflowData_Duration(t *testing.T) {
	wd := interpolation.WorkflowData{
		StartedAt: time.Now().Add(-5 * time.Second),
	}

	duration := wd.Duration()
	assert.Contains(t, duration, "s")
}

func TestContextRuntimeValues(t *testing.T) {
	// Verify context can be populated with runtime values
	ctx := interpolation.NewContext()

	// Populate from os
	wd, _ := os.Getwd()
	ctx.Context.WorkingDir = wd
	ctx.Context.User = os.Getenv("USER")
	hostname, _ := os.Hostname()
	ctx.Context.Hostname = hostname

	assert.NotEmpty(t, ctx.Context.WorkingDir)
	// User and hostname may be empty in some environments
}
