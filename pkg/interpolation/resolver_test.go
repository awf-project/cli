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

func TestTemplateResolver_StepStateDataResponse(t *testing.T) {
	tests := []struct {
		name     string
		template string
		states   map[string]interpolation.StepStateData
		want     string
		wantErr  bool
	}{
		{
			name:     "response map with string values",
			template: "result: {{.states.api_call.Response.status}}",
			states: map[string]interpolation.StepStateData{
				"api_call": {
					Output: "success",
					Response: map[string]any{
						"status": "completed",
					},
				},
			},
			want: "result: completed",
		},
		{
			name:     "response map with multiple string fields",
			template: "user: {{.states.get_user.Response.name}}, role: {{.states.get_user.Response.role}}",
			states: map[string]interpolation.StepStateData{
				"get_user": {
					Output: "fetched",
					Response: map[string]any{
						"name": "Alice",
						"role": "admin",
					},
				},
			},
			want: "user: Alice, role: admin",
		},
		{
			name:     "response map with nested objects",
			template: "city: {{.states.get_address.Response.address.city}}",
			states: map[string]interpolation.StepStateData{
				"get_address": {
					Output: "retrieved",
					Response: map[string]any{
						"address": map[string]any{
							"city":    "Seattle",
							"zipcode": "98101",
						},
					},
				},
			},
			want: "city: Seattle",
		},
		{
			name:     "response map with deeply nested objects",
			template: "value: {{.states.nested_data.Response.level1.level2.value}}",
			states: map[string]interpolation.StepStateData{
				"nested_data": {
					Output: "ok",
					Response: map[string]any{
						"level1": map[string]any{
							"level2": map[string]any{
								"value": "deep",
							},
						},
					},
				},
			},
			want: "value: deep",
		},
		{
			name:     "nil response handling",
			template: "{{.states.no_response.Output}}",
			states: map[string]interpolation.StepStateData{
				"no_response": {
					Output:   "plain output",
					Response: nil,
				},
			},
			want: "plain output",
		},
		{
			name:     "empty response map",
			template: "output: {{.states.empty_resp.Output}}",
			states: map[string]interpolation.StepStateData{
				"empty_resp": {
					Output:   "has output",
					Response: map[string]any{},
				},
			},
			want: "output: has output",
		},
		{
			name:     "response with numeric values",
			template: "count: {{.states.counter.Response.count}}, score: {{.states.counter.Response.score}}",
			states: map[string]interpolation.StepStateData{
				"counter": {
					Output: "counted",
					Response: map[string]any{
						"count": 42,
						"score": 98.5,
					},
				},
			},
			want: "count: 42, score: 98.5",
		},
		{
			name:     "response with boolean values",
			template: "success: {{.states.check.Response.success}}, enabled: {{.states.check.Response.enabled}}",
			states: map[string]interpolation.StepStateData{
				"check": {
					Output: "checked",
					Response: map[string]any{
						"success": true,
						"enabled": false,
					},
				},
			},
			want: "success: true, enabled: false",
		},
		{
			name:     "response with array values",
			template: "tags: {{index .states.tags.Response.tags 0}}, {{index .states.tags.Response.tags 1}}",
			states: map[string]interpolation.StepStateData{
				"tags": {
					Output: "listed",
					Response: map[string]any{
						"tags": []string{"urgent", "bug"},
					},
				},
			},
			want: "tags: urgent, bug",
		},
		{
			name:     "response with mixed types",
			template: "id: {{.states.entity.Response.id}}, name: {{.states.entity.Response.name}}, active: {{.states.entity.Response.active}}",
			states: map[string]interpolation.StepStateData{
				"entity": {
					Output: "entity data",
					Response: map[string]any{
						"id":     123,
						"name":   "Service",
						"active": true,
					},
				},
			},
			want: "id: 123, name: Service, active: true",
		},
		{
			name:     "undefined response field",
			template: "{{.states.api_call.Response.missing}}",
			states: map[string]interpolation.StepStateData{
				"api_call": {
					Output:   "success",
					Response: map[string]any{"status": "ok"},
				},
			},
			wantErr: true,
		},
		{
			name:     "accessing response on step without response data",
			template: "{{.states.plain_step.Response.field}}",
			states: map[string]interpolation.StepStateData{
				"plain_step": {
					Output:   "plain",
					Response: nil,
				},
			},
			wantErr: true,
		},
		{
			name:     "combined output and response access",
			template: "output: {{.states.combined.Output}}, status: {{.states.combined.Response.status}}",
			states: map[string]interpolation.StepStateData{
				"combined": {
					Output: "command result",
					Response: map[string]any{
						"status": "success",
					},
				},
			},
			want: "output: command result, status: success",
		},
		{
			name:     "response with null value",
			template: "value: {{.states.nullable.Response.optional}}",
			states: map[string]interpolation.StepStateData{
				"nullable": {
					Output: "ok",
					Response: map[string]any{
						"optional": nil,
					},
				},
			},
			want: "value: <no value>",
		},
		{
			name:     "response with empty string value",
			template: "value=[{{.states.empty_str.Response.field}}]",
			states: map[string]interpolation.StepStateData{
				"empty_str": {
					Output: "ok",
					Response: map[string]any{
						"field": "",
					},
				},
			},
			want: "value=[]",
		},
		{
			name:     "response with zero numeric values",
			template: "count: {{.states.zeros.Response.count}}, score: {{.states.zeros.Response.score}}",
			states: map[string]interpolation.StepStateData{
				"zeros": {
					Output: "ok",
					Response: map[string]any{
						"count": 0,
						"score": 0.0,
					},
				},
			},
			want: "count: 0, score: 0",
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
			expected := tt.want
			if tt.name == "mixed namespaces" {
				ctx.Workflow.Name = "test-workflow"
				expected = "value - test-workflow"
			}

			got, err := resolver.Resolve(tt.template, ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, expected, got)
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

func TestTemplateResolver_Loop(t *testing.T) {
	tests := []struct {
		name     string
		template string
		loop     *interpolation.LoopData
		want     string
		wantErr  bool
	}{
		{
			name:     "loop item string",
			template: "echo {{.loop.Item}}",
			loop:     &interpolation.LoopData{Item: "file.txt"},
			want:     "echo file.txt",
		},
		{
			name:     "loop item integer",
			template: "process item {{.loop.Item}}",
			loop:     &interpolation.LoopData{Item: 42},
			want:     "process item 42",
		},
		{
			name:     "loop index",
			template: "iteration {{.loop.Index}}",
			loop:     &interpolation.LoopData{Index: 5},
			want:     "iteration 5",
		},
		{
			name:     "loop index zero",
			template: "index: {{.loop.Index}}",
			loop:     &interpolation.LoopData{Index: 0},
			want:     "index: 0",
		},
		{
			name:     "loop first flag true",
			template: "first: {{.loop.First}}",
			loop:     &interpolation.LoopData{First: true},
			want:     "first: true",
		},
		{
			name:     "loop first flag false",
			template: "first: {{.loop.First}}",
			loop:     &interpolation.LoopData{First: false},
			want:     "first: false",
		},
		{
			name:     "loop last flag true",
			template: "last: {{.loop.Last}}",
			loop:     &interpolation.LoopData{Last: true},
			want:     "last: true",
		},
		{
			name:     "loop last flag false",
			template: "last: {{.loop.Last}}",
			loop:     &interpolation.LoopData{Last: false},
			want:     "last: false",
		},
		{
			name:     "loop length",
			template: "total: {{.loop.Length}}",
			loop:     &interpolation.LoopData{Length: 10},
			want:     "total: 10",
		},
		{
			name:     "loop length negative for while",
			template: "length: {{.loop.Length}}",
			loop:     &interpolation.LoopData{Length: -1},
			want:     "length: -1",
		},
		{
			name:     "loop without context",
			template: "{{.loop.Item}}",
			loop:     nil,
			wantErr:  true,
		},
		{
			name:     "combined loop variables",
			template: "Processing {{.loop.Item}} ({{.loop.Index}}/{{.loop.Length}})",
			loop: &interpolation.LoopData{
				Item:   "data.csv",
				Index:  2,
				Length: 5,
			},
			want: "Processing data.csv (2/5)",
		},
		{
			name:     "loop with first and last",
			template: "{{if .loop.First}}START{{end}}item={{.loop.Item}}{{if .loop.Last}}END{{end}}",
			loop: &interpolation.LoopData{
				Item:  "middle",
				First: false,
				Last:  false,
			},
			want: "item=middle",
		},
		{
			name:     "loop first item",
			template: "{{if .loop.First}}[FIRST] {{end}}{{.loop.Item}}",
			loop: &interpolation.LoopData{
				Item:  "first_item",
				First: true,
			},
			want: "[FIRST] first_item",
		},
		{
			name:     "loop last item",
			template: "{{.loop.Item}}{{if .loop.Last}} [LAST]{{end}}",
			loop: &interpolation.LoopData{
				Item: "last_item",
				Last: true,
			},
			want: "last_item [LAST]",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = tt.loop

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

func TestLoopData_Fields(t *testing.T) {
	loop := &interpolation.LoopData{
		Item:   "test.txt",
		Index:  3,
		First:  false,
		Last:   true,
		Length: 5,
	}

	assert.Equal(t, "test.txt", loop.Item)
	assert.Equal(t, 3, loop.Index)
	assert.False(t, loop.First)
	assert.True(t, loop.Last)
	assert.Equal(t, 5, loop.Length)
}

func TestLoopData_WhileLoop(t *testing.T) {
	// While loops have Length=-1 since count is unknown
	loop := &interpolation.LoopData{
		Item:   nil, // while loops may not have items
		Index:  7,
		First:  false,
		Last:   false, // unknown for while
		Length: -1,    // unknown for while
	}

	assert.Nil(t, loop.Item)
	assert.Equal(t, 7, loop.Index)
	assert.Equal(t, -1, loop.Length)
}

func TestTemplateResolver_LoopWithOtherNamespaces(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	// Test loop combined with inputs and states
	ctx := interpolation.NewContext()
	ctx.Inputs["prefix"] = "file_"
	ctx.States["check"] = interpolation.StepStateData{
		Output:   "ok",
		ExitCode: 0,
	}
	ctx.Loop = &interpolation.LoopData{
		Item:   "data.csv",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 3,
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "loop with inputs",
			template: "{{.inputs.prefix}}{{.loop.Item}}",
			want:     "file_data.csv",
		},
		{
			name:     "loop with states",
			template: "{{.loop.Item}}: {{.states.check.Output}}",
			want:     "data.csv: ok",
		},
		{
			name:     "all namespaces",
			template: "{{.inputs.prefix}}{{.loop.Index}}_{{.loop.Item}}_{{.states.check.Output}}",
			want:     "file_0_data.csv_ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.Resolve(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateResolver_LoopWithEscape(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	ctx := interpolation.NewContext()
	ctx.Loop = &interpolation.LoopData{
		Item: "file with spaces.txt",
	}

	// Test with escape function
	template := "process {{escape .loop.Item}}"
	got, err := resolver.Resolve(template, ctx)

	require.NoError(t, err)
	assert.Equal(t, "process 'file with spaces.txt'", got)
}

func TestContext_WithLoop(t *testing.T) {
	ctx := interpolation.NewContext()

	// Verify loop is nil by default
	assert.Nil(t, ctx.Loop)

	// Set loop context
	ctx.Loop = &interpolation.LoopData{
		Item:   "item1",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 3,
	}

	assert.NotNil(t, ctx.Loop)
	assert.Equal(t, "item1", ctx.Loop.Item)
	assert.Equal(t, 0, ctx.Loop.Index)
	assert.True(t, ctx.Loop.First)
	assert.False(t, ctx.Loop.Last)
	assert.Equal(t, 3, ctx.Loop.Length)

	// Clear loop context
	ctx.Loop = nil
	assert.Nil(t, ctx.Loop)
}

func TestTemplateResolver_LoopItemTypes(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		item     any
		template string
		want     string
	}{
		{
			name:     "string item",
			item:     "hello",
			template: "{{.loop.Item}}",
			want:     "hello",
		},
		{
			name:     "integer item",
			item:     42,
			template: "{{.loop.Item}}",
			want:     "42",
		},
		{
			name:     "float item",
			item:     3.14,
			template: "{{.loop.Item}}",
			want:     "3.14",
		},
		{
			name:     "boolean item",
			item:     true,
			template: "{{.loop.Item}}",
			want:     "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{Item: tt.item}

			got, err := resolver.Resolve(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateResolver_LoopIndex1(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		template string
		loop     *interpolation.LoopData
		want     string
	}{
		{
			name:     "index1 basic",
			template: "Item {{.loop.Index1}} of {{.loop.Length}}",
			loop: &interpolation.LoopData{
				Item:   "test",
				Index:  2,
				First:  false,
				Last:   false,
				Length: 5,
			},
			want: "Item 3 of 5",
		},
		{
			name:     "index1 at zero index",
			template: "Position: {{.loop.Index1}}",
			loop: &interpolation.LoopData{
				Index: 0,
			},
			want: "Position: 1",
		},
		{
			name:     "index1 at large index",
			template: "Row {{.loop.Index1}}",
			loop: &interpolation.LoopData{
				Index: 99,
			},
			want: "Row 100",
		},
		{
			name:     "index1 combined with index",
			template: "{{.loop.Index}} (0-based) = {{.loop.Index1}} (1-based)",
			loop: &interpolation.LoopData{
				Index: 5,
			},
			want: "5 (0-based) = 6 (1-based)",
		},
		{
			name:     "index1 in realistic template",
			template: "Processing file {{.loop.Index1}}/{{.loop.Length}}: {{.loop.Item}}",
			loop: &interpolation.LoopData{
				Item:   "data.csv",
				Index:  0,
				First:  true,
				Last:   false,
				Length: 3,
			},
			want: "Processing file 1/3: data.csv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = tt.loop

			got, err := resolver.Resolve(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoopData_Index1_Method(t *testing.T) {
	tests := []struct {
		name  string
		index int
		want  int
	}{
		{
			name:  "zero returns one",
			index: 0,
			want:  1,
		},
		{
			name:  "one returns two",
			index: 1,
			want:  2,
		},
		{
			name:  "large index",
			index: 99,
			want:  100,
		},
		{
			name:  "mid-range index",
			index: 42,
			want:  43,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loop := &interpolation.LoopData{Index: tt.index}
			assert.Equal(t, tt.want, loop.Index1())
		})
	}
}

// Item: T004
// Feature: F047
func TestTemplateResolver_WithJSONFunction_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		want     string
	}{
		{
			name:     "string primitive passes through",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": "hello"},
			want:     `"hello"`,
		},
		{
			name:     "integer as JSON",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": 42},
			want:     `42`,
		},
		{
			name:     "float as JSON",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": 3.14},
			want:     `3.14`,
		},
		{
			name:     "boolean true as JSON",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": true},
			want:     `true`,
		},
		{
			name:     "boolean false as JSON",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": false},
			want:     `false`,
		},
		{
			name:     "simple map as JSON object",
			template: `{{json .inputs.val}}`,
			inputs: map[string]any{
				"val": map[string]any{"name": "Alice", "age": 30},
			},
			want: `{"age":30,"name":"Alice"}`,
		},
		{
			name:     "simple slice as JSON array",
			template: `{{json .inputs.val}}`,
			inputs: map[string]any{
				"val": []any{"a", "b", "c"},
			},
			want: `["a","b","c"]`,
		},
		{
			name:     "nested map structure",
			template: `{{json .inputs.val}}`,
			inputs: map[string]any{
				"val": map[string]any{
					"user": map[string]any{
						"name": "Bob",
						"tags": []any{"dev", "admin"},
					},
				},
			},
			want: `{"user":{"name":"Bob","tags":["dev","admin"]}}`,
		},
		{
			name:     "map with mixed types",
			template: `{{json .inputs.val}}`,
			inputs: map[string]any{
				"val": map[string]any{
					"name":   "Service",
					"count":  5,
					"active": true,
					"score":  98.5,
				},
			},
			want: `{"active":true,"count":5,"name":"Service","score":98.5}`,
		},
		{
			name:     "slice of maps",
			template: `{{json .inputs.val}}`,
			inputs: map[string]any{
				"val": []any{
					map[string]any{"id": 1, "name": "first"},
					map[string]any{"id": 2, "name": "second"},
				},
			},
			want: `[{"id":1,"name":"first"},{"id":2,"name":"second"}]`,
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

// Item: T004
// Feature: F047
func TestTemplateResolver_WithJSONFunction_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		want     string
	}{
		{
			name:     "nil value",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": nil},
			want:     `null`,
		},
		{
			name:     "empty map",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": map[string]any{}},
			want:     `{}`,
		},
		{
			name:     "empty slice",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": []any{}},
			want:     `[]`,
		},
		{
			name:     "empty string",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": ""},
			want:     `""`,
		},
		{
			name:     "zero integer",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": 0},
			want:     `0`,
		},
		{
			name:     "zero float",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": 0.0},
			want:     `0`,
		},
		{
			name:     "string with unicode",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": "Hello 世界 🌍"},
			want:     `"Hello 世界 🌍"`,
		},
		{
			name:     "string with special chars",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": `line1\nline2\ttab`},
			want:     `"line1\\nline2\\ttab"`,
		},
		{
			name:     "string with quotes",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": `she said "hello"`},
			want:     `"she said \"hello\""`,
		},
		{
			name:     "deeply nested structure",
			template: `{{json .inputs.val}}`,
			inputs: map[string]any{
				"val": map[string]any{
					"level1": map[string]any{
						"level2": map[string]any{
							"level3": []any{1, 2, 3},
						},
					},
				},
			},
			want: `{"level1":{"level2":{"level3":[1,2,3]}}}`,
		},
		{
			name:     "array with null elements",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": []any{1, nil, 3}},
			want:     `[1,null,3]`,
		},
		{
			name:     "negative numbers",
			template: `{{json .inputs.val}}`,
			inputs:   map[string]any{"val": -42},
			want:     `-42`,
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

// Item: T004
// Feature: F047
func TestTemplateResolver_WithJSONFunction_LoopItem(t *testing.T) {
	tests := []struct {
		name     string
		template string
		item     any
		want     string
	}{
		{
			name:     "loop item map as JSON",
			template: `{{json .loop.Item}}`,
			item:     map[string]any{"name": "S1", "type": "fix"},
			want:     `{"name":"S1","type":"fix"}`,
		},
		{
			name:     "loop item with nested structure",
			template: `{{json .loop.Item}}`,
			item: map[string]any{
				"name":  "S1",
				"files": []any{"a.go", "b.go"},
			},
			want: `{"files":["a.go","b.go"],"name":"S1"}`,
		},
		{
			name:     "loop item string primitive",
			template: `{{json .loop.Item}}`,
			item:     "simple-string",
			want:     `"simple-string"`,
		},
		{
			name:     "loop item integer",
			template: `{{json .loop.Item}}`,
			item:     123,
			want:     `123`,
		},
		{
			name:     "loop item in workflow input context",
			template: `item={{json .loop.Item}}`,
			item: map[string]any{
				"id":     1,
				"status": "pending",
			},
			want: `item={"id":1,"status":"pending"}`,
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{
				Item: tt.item,
			}

			got, err := resolver.Resolve(tt.template, ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Item: T004
// Feature: F047
func TestTemplateResolver_WithJSONFunction_CombinedWithOtherFunctions(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	t.Run("json with escape in same template", func(t *testing.T) {
		ctx := interpolation.NewContext()
		ctx.Inputs["data"] = map[string]any{"key": "value"}
		ctx.Inputs["file"] = "test file.txt"

		template := `echo {{json .inputs.data}} > {{escape .inputs.file}}`
		got, err := resolver.Resolve(template, ctx)

		require.NoError(t, err)
		assert.Equal(t, `echo {"key":"value"} > 'test file.txt'`, got)
	})

	t.Run("multiple json calls in template", func(t *testing.T) {
		ctx := interpolation.NewContext()
		ctx.Inputs["obj1"] = map[string]any{"a": 1}
		ctx.Inputs["obj2"] = map[string]any{"b": 2}

		template := `{{json .inputs.obj1}} {{json .inputs.obj2}}`
		got, err := resolver.Resolve(template, ctx)

		require.NoError(t, err)
		assert.Equal(t, `{"a":1} {"b":2}`, got)
	})
}

// Item: T004
// Feature: F047
func TestTemplateResolver_WithJSONFunction_ErrorHandling(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	t.Run("undefined variable with json function", func(t *testing.T) {
		ctx := interpolation.NewContext()

		template := `{{json .inputs.missing}}`
		_, err := resolver.Resolve(template, ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing")
	})

	t.Run("json function with no arguments", func(t *testing.T) {
		ctx := interpolation.NewContext()

		template := `{{json}}`
		_, err := resolver.Resolve(template, ctx)

		require.Error(t, err)
	})
}

// Item: T006
// Feature: F047
func TestTemplateResolver_WithJSONFunction_UnmarshallableTypes(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	t.Run("channel type cannot be marshaled", func(t *testing.T) {
		ctx := interpolation.NewContext()
		ch := make(chan int)
		ctx.Inputs["val"] = ch

		template := `{{json .inputs.val}}`
		_, err := resolver.Resolve(template, ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "json")
	})

	t.Run("function type cannot be marshaled", func(t *testing.T) {
		ctx := interpolation.NewContext()
		fn := func() {}
		ctx.Inputs["val"] = fn

		template := `{{json .inputs.val}}`
		_, err := resolver.Resolve(template, ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "json")
	})

	t.Run("complex number type cannot be marshaled", func(t *testing.T) {
		ctx := interpolation.NewContext()
		ctx.Inputs["val"] = complex(1, 2)

		template := `{{json .inputs.val}}`
		_, err := resolver.Resolve(template, ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "json")
	})
}

// TestTemplateResolver_LoopItemJSON_Map verifies that map items are serialized to JSON
// Item: T005
// Feature: F047
func TestTemplateResolver_LoopItemJSON_Map(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		item     any
		template string
		want     string
	}{
		{
			name: "simple map",
			item: map[string]any{
				"name": "S1",
				"type": "fix",
			},
			template: "{{.loop.Item}}",
			want:     `{"name":"S1","type":"fix"}`,
		},
		{
			name: "map with nested array",
			item: map[string]any{
				"name":  "S1",
				"type":  "fix",
				"files": []string{"a.go", "b.go"},
			},
			template: "{{.loop.Item}}",
			want:     `{"files":["a.go","b.go"],"name":"S1","type":"fix"}`,
		},
		{
			name: "map with nested object",
			item: map[string]any{
				"task": map[string]any{
					"id":   123,
					"done": true,
				},
			},
			template: "{{.loop.Item}}",
			want:     `{"task":{"done":true,"id":123}}`,
		},
		{
			name: "map with mixed types",
			item: map[string]any{
				"str":   "hello",
				"num":   42,
				"float": 3.14,
				"bool":  true,
				"null":  nil,
			},
			template: "{{.loop.Item}}",
			want:     `{"bool":true,"float":3.14,"null":null,"num":42,"str":"hello"}`,
		},
		{
			name:     "empty map",
			item:     map[string]any{},
			template: "{{.loop.Item}}",
			want:     `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{
				Item:   tt.item,
				Index:  0,
				First:  true,
				Last:   false,
				Length: 1,
			}

			got, err := resolver.Resolve(tt.template, ctx)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, got, "Loop item should be serialized to JSON")
		})
	}
}

// TestTemplateResolver_LoopItemJSON_Slice verifies that slice items are serialized to JSON
// Item: T005
// Feature: F047
func TestTemplateResolver_LoopItemJSON_Slice(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		item     any
		template string
		want     string
	}{
		{
			name:     "string array",
			item:     []string{"a", "b", "c"},
			template: "{{.loop.Item}}",
			want:     `["a","b","c"]`,
		},
		{
			name:     "integer array",
			item:     []int{1, 2, 3},
			template: "{{.loop.Item}}",
			want:     `[1,2,3]`,
		},
		{
			name:     "mixed type array",
			item:     []any{"str", 42, true, nil},
			template: "{{.loop.Item}}",
			want:     `["str",42,true,null]`,
		},
		{
			name: "array of objects",
			item: []map[string]any{
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"},
			},
			template: "{{.loop.Item}}",
			want:     `[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`,
		},
		{
			name:     "empty array",
			item:     []any{},
			template: "{{.loop.Item}}",
			want:     `[]`,
		},
		{
			name:     "nested arrays",
			item:     []any{[]int{1, 2}, []int{3, 4}},
			template: "{{.loop.Item}}",
			want:     `[[1,2],[3,4]]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{
				Item:   tt.item,
				Index:  0,
				First:  true,
				Last:   false,
				Length: 1,
			}

			got, err := resolver.Resolve(tt.template, ctx)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, got, "Loop item should be serialized to JSON")
		})
	}
}

// TestTemplateResolver_LoopItemJSON_BackwardCompatibility verifies primitives still work
// Item: T005
// Feature: F047
func TestTemplateResolver_LoopItemJSON_BackwardCompatibility(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		item     any
		template string
		want     string
	}{
		{
			name:     "string item unchanged",
			item:     "hello",
			template: "{{.loop.Item}}",
			want:     "hello",
		},
		{
			name:     "integer item",
			item:     42,
			template: "{{.loop.Item}}",
			want:     "42",
		},
		{
			name:     "float item",
			item:     3.14,
			template: "{{.loop.Item}}",
			want:     "3.14",
		},
		{
			name:     "boolean true",
			item:     true,
			template: "{{.loop.Item}}",
			want:     "true",
		},
		{
			name:     "boolean false",
			item:     false,
			template: "{{.loop.Item}}",
			want:     "false",
		},
		{
			name:     "nil item",
			item:     nil,
			template: "{{.loop.Item}}",
			want:     "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{
				Item:   tt.item,
				Index:  0,
				First:  true,
				Last:   false,
				Length: 1,
			}

			got, err := resolver.Resolve(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "Primitive types should pass through unchanged")
		})
	}
}

// TestTemplateResolver_LoopItemJSON_AllFields verifies all loop fields are preserved
// Item: T005
// Feature: F047
func TestTemplateResolver_LoopItemJSON_AllFields(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		loop     *interpolation.LoopData
		template string
		want     string
	}{
		{
			name: "map item with all loop fields",
			loop: &interpolation.LoopData{
				Item:   map[string]any{"name": "task1"},
				Index:  2,
				First:  false,
				Last:   false,
				Length: 5,
			},
			template: "{{.loop.Index}},{{.loop.First}},{{.loop.Last}},{{.loop.Length}},{{.loop.Item}}",
			want:     `2,false,false,5,{"name":"task1"}`,
		},
		{
			name: "slice item with index and length",
			loop: &interpolation.LoopData{
				Item:   []string{"a", "b"},
				Index:  1,
				First:  false,
				Last:   true,
				Length: 2,
			},
			template: "Item {{.loop.Index}}: {{.loop.Item}}",
			want:     `Item 1: ["a","b"]`,
		},
		{
			name: "first item serialization",
			loop: &interpolation.LoopData{
				Item:   map[string]string{"status": "first"},
				Index:  0,
				First:  true,
				Last:   false,
				Length: 3,
			},
			template: "{{if .loop.First}}First: {{.loop.Item}}{{end}}",
			want:     `First: {"status":"first"}`,
		},
		{
			name: "last item serialization",
			loop: &interpolation.LoopData{
				Item:   map[string]string{"status": "last"},
				Index:  2,
				First:  false,
				Last:   true,
				Length: 3,
			},
			template: "{{if .loop.Last}}Last: {{.loop.Item}}{{end}}",
			want:     `Last: {"status":"last"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = tt.loop

			got, err := resolver.Resolve(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "All loop fields should be preserved after serialization")
		})
	}
}

// TestTemplateResolver_LoopItemJSON_EdgeCases verifies edge cases in serialization
// Item: T005
// Feature: F047
func TestTemplateResolver_LoopItemJSON_EdgeCases(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		item     any
		template string
		want     string
	}{
		{
			name:     "zero integer",
			item:     0,
			template: "{{.loop.Item}}",
			want:     "0",
		},
		{
			name:     "empty string",
			item:     "",
			template: "{{.loop.Item}}",
			want:     "",
		},
		{
			name:     "zero float",
			item:     0.0,
			template: "{{.loop.Item}}",
			want:     "0",
		},
		{
			name: "map with empty string value",
			item: map[string]any{
				"key": "",
			},
			template: "{{.loop.Item}}",
			want:     `{"key":""}`,
		},
		{
			name: "map with zero values",
			item: map[string]any{
				"zero_int":   0,
				"zero_float": 0.0,
				"false_bool": false,
			},
			template: "{{.loop.Item}}",
			want:     `{"false_bool":false,"zero_float":0,"zero_int":0}`,
		},
		{
			name: "deeply nested structure",
			item: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": []any{"deep", "value"},
					},
				},
			},
			template: "{{.loop.Item}}",
			want:     `{"level1":{"level2":{"level3":["deep","value"]}}}`,
		},
		{
			name: "unicode characters",
			item: map[string]any{
				"emoji":   "🚀",
				"chinese": "你好",
				"arabic":  "مرحبا",
			},
			template: "{{.loop.Item}}",
			want:     `{"arabic":"مرحبا","chinese":"你好","emoji":"🚀"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{
				Item:   tt.item,
				Index:  0,
				First:  true,
				Last:   false,
				Length: 1,
			}

			got, err := resolver.Resolve(tt.template, ctx)
			require.NoError(t, err)

			// For JSON-serialized items, use JSONEq; for primitives use Equal
			if tt.want != "" && (tt.want[0] == '{' || tt.want[0] == '[') {
				assert.JSONEq(t, tt.want, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestTemplateResolver_LoopItemJSON_WithoutLoop verifies behavior when loop is nil
// Item: T005
// Feature: F047
func TestTemplateResolver_LoopItemJSON_WithoutLoop(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	ctx := interpolation.NewContext()
	// ctx.Loop is nil

	template := "{{.inputs.name}}"
	ctx.Inputs["name"] = "test"

	got, err := resolver.Resolve(template, ctx)
	require.NoError(t, err)
	assert.Equal(t, "test", got, "Should work normally when loop is nil")
}

// TestTemplateResolver_LoopItemJSON_MultipleReferences verifies item can be used multiple times
// Item: T005
// Feature: F047
func TestTemplateResolver_LoopItemJSON_MultipleReferences(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	ctx := interpolation.NewContext()
	ctx.Loop = &interpolation.LoopData{
		Item: map[string]any{
			"name": "task",
			"id":   123,
		},
		Index:  0,
		First:  true,
		Last:   false,
		Length: 1,
	}

	// Reference .loop.Item multiple times in same template
	template := "First: {{.loop.Item}} | Second: {{.loop.Item}}"

	got, err := resolver.Resolve(template, ctx)
	require.NoError(t, err)

	expectedJSON := `{"id":123,"name":"task"}`
	expected := "First: " + expectedJSON + " | Second: " + expectedJSON

	assert.Equal(t, expected, got, "Item should be consistently serialized")
}

// TestTemplateResolver_LoopItemJSON_CombinedWithOtherNamespaces verifies serialization with other data
// Item: T005
// Feature: F047
func TestTemplateResolver_LoopItemJSON_CombinedWithOtherNamespaces(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	ctx := interpolation.NewContext()
	ctx.Inputs = map[string]any{"workflow": "test"}
	ctx.States = map[string]interpolation.StepStateData{
		"step1": {Output: "result"},
	}
	ctx.Loop = &interpolation.LoopData{
		Item: map[string]any{
			"file": "data.json",
		},
		Index:  0,
		First:  true,
		Last:   false,
		Length: 1,
	}

	template := "Workflow: {{.inputs.workflow}}, Step: {{.states.step1.Output}}, Item: {{.loop.Item}}"

	got, err := resolver.Resolve(template, ctx)
	require.NoError(t, err)
	assert.Equal(t, `Workflow: test, Step: result, Item: {"file":"data.json"}`, got)
}

// These tests verify that loop items are automatically serialized to JSON
// when used in templates, without requiring explicit | json filter.
// Item: T007
// Feature: F047

// TestTemplateResolver_AutomaticSerialization_ComplexStructures verifies
// automatic JSON serialization for deeply nested and complex data structures.
// Item: T007
// Feature: F047
func TestTemplateResolver_AutomaticSerialization_ComplexStructures(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		item     any
		template string
		want     string
		wantErr  bool
	}{
		{
			name: "three-level nested map",
			item: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": map[string]any{
							"value": "deep",
						},
					},
				},
			},
			template: "{{.loop.Item}}",
			want:     `{"level1":{"level2":{"level3":{"value":"deep"}}}}`,
			wantErr:  false,
		},
		{
			name: "four-level nested array",
			item: []any{
				[]any{
					[]any{
						[]any{"deep", "value"},
					},
				},
			},
			template: "{{.loop.Item}}",
			want:     `[[[["deep","value"]]]]`,
			wantErr:  false,
		},
		{
			name: "mixed nested map and array",
			item: map[string]any{
				"users": []map[string]any{
					{
						"name": "Alice",
						"tags": []string{"admin", "developer"},
						"meta": map[string]any{
							"lastLogin": "2024-01-01",
							"roles":     []string{"owner", "editor"},
						},
					},
					{
						"name": "Bob",
						"tags": []string{"user"},
						"meta": map[string]any{
							"lastLogin": "2024-01-02",
							"roles":     []string{"viewer"},
						},
					},
				},
			},
			template: "{{.loop.Item}}",
			want:     `{"users":[{"meta":{"lastLogin":"2024-01-01","roles":["owner","editor"]},"name":"Alice","tags":["admin","developer"]},{"meta":{"lastLogin":"2024-01-02","roles":["viewer"]},"name":"Bob","tags":["user"]}]}`,
			wantErr:  false,
		},
		{
			name: "large nested structure with multiple types",
			item: map[string]any{
				"config": map[string]any{
					"database": map[string]any{
						"host":     "localhost",
						"port":     5432,
						"ssl":      true,
						"replicas": []string{"replica1", "replica2"},
					},
					"cache": map[string]any{
						"enabled":    true,
						"ttl":        3600,
						"maxEntries": 1000,
						"backends":   []string{"redis", "memcached"},
					},
				},
				"features": []map[string]any{
					{"name": "feature1", "enabled": true, "weight": 0.5},
					{"name": "feature2", "enabled": false, "weight": 0.3},
				},
			},
			template: "Config: {{.loop.Item}}",
			want:     `Config: {"config":{"cache":{"backends":["redis","memcached"],"enabled":true,"maxEntries":1000,"ttl":3600},"database":{"host":"localhost","port":5432,"replicas":["replica1","replica2"],"ssl":true}},"features":[{"enabled":true,"name":"feature1","weight":0.5},{"enabled":false,"name":"feature2","weight":0.3}]}`,
			wantErr:  false,
		},
		{
			name: "array of arrays with mixed types",
			item: []any{
				[]any{"string", 42, true},
				[]any{3.14, false, "another"},
				[]any{nil, map[string]any{"nested": "value"}},
			},
			template: "Items: {{.loop.Item}}",
			want:     `Items: [["string",42,true],[3.14,false,"another"],[null,{"nested":"value"}]]`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{
				Item:   tt.item,
				Index:  0,
				First:  true,
				Last:   false,
				Length: 1,
			}

			got, err := resolver.Resolve(tt.template, ctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			// Use JSONEq for pure JSON outputs, Equal for outputs with text prefixes
			if tt.want != "" && (tt.want[0] == '{' || tt.want[0] == '[') {
				assert.JSONEq(t, tt.want, got, "Complex structure should be serialized to JSON")
			} else {
				assert.Equal(t, tt.want, got, "Complex structure should be serialized to JSON")
			}
		})
	}
}

// TestTemplateResolver_AutomaticSerialization_WithConditionals verifies
// automatic serialization works correctly within template conditionals.
// Item: T007
// Feature: F047
func TestTemplateResolver_AutomaticSerialization_WithConditionals(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		loop     *interpolation.LoopData
		template string
		want     string
		wantErr  bool
	}{
		{
			name: "if with First - serialize map",
			loop: &interpolation.LoopData{
				Item:   map[string]any{"id": 1, "name": "first"},
				Index:  0,
				First:  true,
				Last:   false,
				Length: 3,
			},
			template: `{{if .loop.First}}First item: {{.loop.Item}}{{end}}`,
			want:     `First item: {"id":1,"name":"first"}`,
			wantErr:  false,
		},
		{
			name: "if with Last - serialize array",
			loop: &interpolation.LoopData{
				Item:   []string{"final", "item"},
				Index:  2,
				First:  false,
				Last:   true,
				Length: 3,
			},
			template: `{{if .loop.Last}}Last: {{.loop.Item}}{{end}}`,
			want:     `Last: ["final","item"]`,
			wantErr:  false,
		},
		{
			name: "if-else with complex object",
			loop: &interpolation.LoopData{
				Item: map[string]any{
					"status": "active",
					"count":  42,
				},
				Index:  1,
				First:  false,
				Last:   false,
				Length: 3,
			},
			template: `{{if .loop.First}}First{{else}}Middle: {{.loop.Item}}{{end}}`,
			want:     `Middle: {"count":42,"status":"active"}`,
			wantErr:  false,
		},
		{
			name: "nested if with serialization",
			loop: &interpolation.LoopData{
				Item: map[string]any{
					"type": "critical",
					"data": map[string]any{
						"severity": "high",
						"tags":     []string{"urgent", "prod"},
					},
				},
				Index:  0,
				First:  true,
				Last:   true,
				Length: 1,
			},
			template: `{{if .loop.First}}{{if .loop.Last}}Only item: {{.loop.Item}}{{end}}{{end}}`,
			want:     `Only item: {"data":{"severity":"high","tags":["urgent","prod"]},"type":"critical"}`,
			wantErr:  false,
		},
		{
			name: "conditional with index comparison",
			loop: &interpolation.LoopData{
				Item:   []int{10, 20, 30},
				Index:  2,
				First:  false,
				Last:   true,
				Length: 3,
			},
			template: `{{if gt .loop.Index 1}}Item: {{.loop.Item}}{{end}}`,
			want:     `Item: [10,20,30]`,
			wantErr:  false,
		},
		{
			name: "multiple conditionals with same item",
			loop: &interpolation.LoopData{
				Item: map[string]string{
					"key": "value",
				},
				Index:  0,
				First:  true,
				Last:   false,
				Length: 2,
			},
			template: `{{if .loop.First}}Start: {{.loop.Item}}{{end}} | {{if not .loop.Last}}Next: {{.loop.Item}}{{end}}`,
			want:     `Start: {"key":"value"} | Next: {"key":"value"}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = tt.loop

			got, err := resolver.Resolve(tt.template, ctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "Conditional should properly serialize loop item")
		})
	}
}

// TestTemplateResolver_AutomaticSerialization_WithRangeLoops verifies
// automatic serialization within range loops in templates.
// Item: T007
// Feature: F047
func TestTemplateResolver_AutomaticSerialization_WithRangeLoops(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		inputs   map[string]any
		loop     *interpolation.LoopData
		template string
		want     string
		wantErr  bool
	}{
		{
			name: "range over array with loop item serialization",
			inputs: map[string]any{
				"items": []string{"a", "b", "c"},
			},
			loop: &interpolation.LoopData{
				Item:   map[string]any{"id": 1, "value": "test"},
				Index:  0,
				First:  true,
				Last:   false,
				Length: 1,
			},
			template: `{{range .inputs.items}}[{{.}}] {{end}}Item: {{.loop.Item}}`,
			want:     `[a] [b] [c] Item: {"id":1,"value":"test"}`,
			wantErr:  false,
		},
		{
			name: "loop item within range block",
			inputs: map[string]any{
				"numbers": []int{1, 2, 3},
			},
			loop: &interpolation.LoopData{
				Item:   []int{10, 20, 30},
				Index:  1,
				First:  false,
				Last:   false,
				Length: 5,
			},
			template: `Start{{range $i, $v := .inputs.numbers}} {{$i}}:{{$v}}{{end}} Loop: {{.loop.Item}}`,
			want:     `Start 0:1 1:2 2:3 Loop: [10,20,30]`,
			wantErr:  false,
		},
		{
			name: "nested range with complex loop item",
			inputs: map[string]any{
				"outer": []string{"x", "y"},
			},
			loop: &interpolation.LoopData{
				Item: map[string]any{
					"nested": map[string]any{
						"data": []string{"a", "b"},
					},
				},
				Index:  0,
				First:  true,
				Last:   false,
				Length: 1,
			},
			template: `{{range .inputs.outer}}{{.}}-{{end}}Item: {{.loop.Item}}`,
			want:     `x-y-Item: {"nested":{"data":["a","b"]}}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Inputs = tt.inputs
			ctx.Loop = tt.loop

			got, err := resolver.Resolve(tt.template, ctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "Range loop should work with serialized loop item")
		})
	}
}

// TestTemplateResolver_AutomaticSerialization_StructTypes verifies
// automatic serialization for custom struct types.
// Item: T007
// Feature: F047
func TestTemplateResolver_AutomaticSerialization_StructTypes(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	// Define test structs
	type Address struct {
		Street  string `json:"street"`
		City    string `json:"city"`
		ZipCode string `json:"zipCode"`
	}

	type Person struct {
		Name    string  `json:"name"`
		Age     int     `json:"age"`
		Email   string  `json:"email"`
		Address Address `json:"address"`
	}

	type Task struct {
		ID       int      `json:"id"`
		Title    string   `json:"title"`
		Tags     []string `json:"tags"`
		Done     bool     `json:"done"`
		Priority *int     `json:"priority,omitempty"`
	}

	tests := []struct {
		name     string
		item     any
		template string
		want     string
		wantErr  bool
	}{
		{
			name: "simple struct",
			item: Person{
				Name:  "Alice",
				Age:   30,
				Email: "alice@example.com",
				Address: Address{
					Street:  "123 Main St",
					City:    "Springfield",
					ZipCode: "12345",
				},
			},
			template: "{{.loop.Item}}",
			want:     `{"name":"Alice","age":30,"email":"alice@example.com","address":{"street":"123 Main St","city":"Springfield","zipCode":"12345"}}`,
			wantErr:  false,
		},
		{
			name: "struct with slice",
			item: Task{
				ID:    1,
				Title: "Complete tests",
				Tags:  []string{"urgent", "testing"},
				Done:  false,
			},
			template: "Task: {{.loop.Item}}",
			want:     `Task: {"id":1,"title":"Complete tests","tags":["urgent","testing"],"done":false}`,
			wantErr:  false,
		},
		{
			name: "struct with pointer field (nil)",
			item: Task{
				ID:       2,
				Title:    "Review PR",
				Tags:     []string{"code-review"},
				Done:     true,
				Priority: nil,
			},
			template: "{{.loop.Item}}",
			want:     `{"id":2,"title":"Review PR","tags":["code-review"],"done":true}`,
			wantErr:  false,
		},
		{
			name: "struct with pointer field (non-nil)",
			item: func() Task {
				priority := 5
				return Task{
					ID:       3,
					Title:    "Fix bug",
					Tags:     []string{"bug", "critical"},
					Done:     false,
					Priority: &priority,
				}
			}(),
			template: "{{.loop.Item}}",
			want:     `{"id":3,"title":"Fix bug","tags":["bug","critical"],"done":false,"priority":5}`,
			wantErr:  false,
		},
		{
			name: "array of structs",
			item: []Person{
				{
					Name:  "Bob",
					Age:   25,
					Email: "bob@example.com",
					Address: Address{
						Street:  "456 Oak Ave",
						City:    "Portland",
						ZipCode: "67890",
					},
				},
				{
					Name:  "Carol",
					Age:   35,
					Email: "carol@example.com",
					Address: Address{
						Street:  "789 Pine Rd",
						City:    "Seattle",
						ZipCode: "11111",
					},
				},
			},
			template: "People: {{.loop.Item}}",
			want:     `People: [{"name":"Bob","age":25,"email":"bob@example.com","address":{"street":"456 Oak Ave","city":"Portland","zipCode":"67890"}},{"name":"Carol","age":35,"email":"carol@example.com","address":{"street":"789 Pine Rd","city":"Seattle","zipCode":"11111"}}]`,
			wantErr:  false,
		},
		{
			name:     "empty struct",
			item:     struct{}{},
			template: "{{.loop.Item}}",
			want:     `{}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{
				Item:   tt.item,
				Index:  0,
				First:  true,
				Last:   false,
				Length: 1,
			}

			got, err := resolver.Resolve(tt.template, ctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			// Use JSONEq for pure JSON outputs, Equal for outputs with text prefixes
			if tt.want != "" && (tt.want[0] == '{' || tt.want[0] == '[') {
				assert.JSONEq(t, tt.want, got, "Struct should be serialized to JSON")
			} else {
				assert.Equal(t, tt.want, got, "Struct should be serialized to JSON")
			}
		})
	}
}

// TestTemplateResolver_AutomaticSerialization_InterfaceValues verifies
// automatic serialization for interface{} values containing various types.
// Item: T007
// Feature: F047
func TestTemplateResolver_AutomaticSerialization_InterfaceValues(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		item     any
		template string
		want     string
		wantErr  bool
	}{
		{
			name:     "interface with string value",
			item:     any("hello world"),
			template: "{{.loop.Item}}",
			want:     "hello world",
			wantErr:  false,
		},
		{
			name:     "interface with int value",
			item:     any(42),
			template: "{{.loop.Item}}",
			want:     "42",
			wantErr:  false,
		},
		{
			name:     "interface with bool value",
			item:     any(true),
			template: "{{.loop.Item}}",
			want:     "true",
			wantErr:  false,
		},
		{
			name:     "interface with nil value",
			item:     any(nil),
			template: "{{.loop.Item}}",
			want:     "null",
			wantErr:  false,
		},
		{
			name: "interface with map value",
			item: any(map[string]any{
				"key":   "value",
				"count": 10,
			}),
			template: "{{.loop.Item}}",
			want:     `{"count":10,"key":"value"}`,
			wantErr:  false,
		},
		{
			name:     "interface with slice value",
			item:     any([]any{1, "two", true, nil}),
			template: "{{.loop.Item}}",
			want:     `[1,"two",true,null]`,
			wantErr:  false,
		},
		{
			name: "interface containing interface slice",
			item: any([]any{
				any("string"),
				any(123),
				any(map[string]any{"nested": "value"}),
			}),
			template: "Data: {{.loop.Item}}",
			want:     `Data: ["string",123,{"nested":"value"}]`,
			wantErr:  false,
		},
		{
			name: "interface with deeply nested any types",
			item: any(map[string]any{
				"level1": any(map[string]any{
					"level2": any([]any{
						any("deep"),
						any(42),
					}),
				}),
			}),
			template: "{{.loop.Item}}",
			want:     `{"level1":{"level2":["deep",42]}}`,
			wantErr:  false,
		},
		{
			name: "interface with mixed concrete and interface types",
			item: any(map[string]any{
				"concrete": "value",
				"boxed":    any(123),
				"nested":   any(map[string]string{"key": "val"}),
			}),
			template: "{{.loop.Item}}",
			want:     `{"boxed":123,"concrete":"value","nested":{"key":"val"}}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{
				Item:   tt.item,
				Index:  0,
				First:  true,
				Last:   false,
				Length: 1,
			}

			got, err := resolver.Resolve(tt.template, ctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			// Use JSONEq for complex types, Equal for primitives
			if tt.want != "" && (tt.want[0] == '{' || tt.want[0] == '[') {
				assert.JSONEq(t, tt.want, got, "Interface value should be serialized correctly")
			} else {
				assert.Equal(t, tt.want, got, "Interface value should be serialized correctly")
			}
		})
	}
}

// TestTemplateResolver_AutomaticSerialization_PointerTypes verifies
// automatic serialization for pointer types and nil pointers.
// Item: T007
// Feature: F047
func TestTemplateResolver_AutomaticSerialization_PointerTypes(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		item     any
		template string
		want     string
		wantErr  bool
	}{
		{
			name: "pointer to string",
			item: func() *string {
				s := "hello"
				return &s
			}(),
			template: "{{.loop.Item}}",
			want:     `"hello"`,
			wantErr:  false,
		},
		{
			name: "pointer to int",
			item: func() *int {
				i := 42
				return &i
			}(),
			template: "{{.loop.Item}}",
			want:     `42`,
			wantErr:  false,
		},
		{
			name: "pointer to bool",
			item: func() *bool {
				b := true
				return &b
			}(),
			template: "{{.loop.Item}}",
			want:     `true`,
			wantErr:  false,
		},
		{
			name:     "nil pointer to string",
			item:     (*string)(nil),
			template: "{{.loop.Item}}",
			want:     `null`,
			wantErr:  false,
		},
		{
			name:     "nil pointer to int",
			item:     (*int)(nil),
			template: "{{.loop.Item}}",
			want:     `null`,
			wantErr:  false,
		},
		{
			name: "pointer to map",
			item: func() *map[string]any {
				m := map[string]any{
					"key":   "value",
					"count": 10,
				}
				return &m
			}(),
			template: "{{.loop.Item}}",
			want:     `{"count":10,"key":"value"}`,
			wantErr:  false,
		},
		{
			name: "pointer to slice",
			item: func() *[]string {
				s := []string{"a", "b", "c"}
				return &s
			}(),
			template: "{{.loop.Item}}",
			want:     `["a","b","c"]`,
			wantErr:  false,
		},
		{
			name: "pointer to struct",
			item: func() any {
				type Data struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				}
				d := Data{ID: 1, Name: "test"}
				return &d
			}(),
			template: "{{.loop.Item}}",
			want:     `{"id":1,"name":"test"}`,
			wantErr:  false,
		},
		{
			name: "map containing pointers",
			item: func() map[string]any {
				str := "value"
				num := 42
				return map[string]any{
					"strPtr": &str,
					"numPtr": &num,
					"nilPtr": (*string)(nil),
				}
			}(),
			template: "{{.loop.Item}}",
			want:     `{"nilPtr":null,"numPtr":42,"strPtr":"value"}`,
			wantErr:  false,
		},
		{
			name: "slice containing pointers",
			item: func() []any {
				s1 := "first"
				s2 := "second"
				return []any{&s1, &s2, (*string)(nil)}
			}(),
			template: "{{.loop.Item}}",
			want:     `["first","second",null]`,
			wantErr:  false,
		},
		{
			name: "pointer to pointer (double indirection)",
			item: func() any {
				s := "nested"
				p := &s
				return &p
			}(),
			template: "{{.loop.Item}}",
			want:     `"nested"`,
			wantErr:  false,
		},
		{
			name: "struct with pointer fields",
			item: func() any {
				type Config struct {
					Name    string  `json:"name"`
					Timeout *int    `json:"timeout,omitempty"`
					Enabled *bool   `json:"enabled,omitempty"`
					Host    *string `json:"host,omitempty"`
				}
				timeout := 30
				enabled := true
				return Config{
					Name:    "test-config",
					Timeout: &timeout,
					Enabled: &enabled,
					Host:    nil,
				}
			}(),
			template: "{{.loop.Item}}",
			want:     `{"name":"test-config","timeout":30,"enabled":true}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = &interpolation.LoopData{
				Item:   tt.item,
				Index:  0,
				First:  true,
				Last:   false,
				Length: 1,
			}

			got, err := resolver.Resolve(tt.template, ctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			// Use JSONEq for all cases as pointers to primitives will be JSON-encoded
			if tt.want == "null" {
				assert.Equal(t, tt.want, got, "Nil pointer should serialize to null")
			} else {
				assert.JSONEq(t, tt.want, got, "Pointer should be dereferenced and serialized")
			}
		})
	}
}

func TestTemplateResolver_StepStateDataTokensUsed(t *testing.T) {
	tests := []struct {
		name     string
		template string
		states   map[string]interpolation.StepStateData
		want     string
		wantErr  bool
	}{
		{
			name:     "tokens integer access",
			template: "tokens: {{.states.agent_step.TokensUsed}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {TokensUsed: 1500},
			},
			want: "tokens: 1500",
		},
		{
			name:     "tokens zero value",
			template: "count: {{.states.empty_step.TokensUsed}}",
			states: map[string]interpolation.StepStateData{
				"empty_step": {TokensUsed: 0},
			},
			want: "count: 0",
		},
		{
			name:     "tokens with other fields",
			template: "{{.states.api_call.Output}} used {{.states.api_call.TokensUsed}} tokens",
			states: map[string]interpolation.StepStateData{
				"api_call": {
					Output:     "Success",
					TokensUsed: 2500,
				},
			},
			want: "Success used 2500 tokens",
		},
		{
			name:     "tokens formatting in template",
			template: "API call completed with {{.states.step1.TokensUsed}} tokens (exit {{.states.step1.ExitCode}})",
			states: map[string]interpolation.StepStateData{
				"step1": {
					TokensUsed: 3200,
					ExitCode:   0,
					Status:     "success",
				},
			},
			want: "API call completed with 3200 tokens (exit 0)",
		},
		{
			name:     "multiple steps with tokens",
			template: "Total: {{.states.step1.TokensUsed}} + {{.states.step2.TokensUsed}}",
			states: map[string]interpolation.StepStateData{
				"step1": {TokensUsed: 1000},
				"step2": {TokensUsed: 1500},
			},
			want: "Total: 1000 + 1500",
		},
		{
			name:     "tokens with response data",
			template: "Response tokens: {{.states.agent.TokensUsed}}",
			states: map[string]interpolation.StepStateData{
				"agent": {
					TokensUsed: 4200,
					Response:   map[string]any{"result": "data"},
				},
			},
			want: "Response tokens: 4200",
		},
		{
			name:     "large token count",
			template: "tokens={{.states.large.TokensUsed}}",
			states: map[string]interpolation.StepStateData{
				"large": {TokensUsed: 999999},
			},
			want: "tokens=999999",
		},
		{
			name:     "undefined state with tokens",
			template: "{{.states.nonexistent.TokensUsed}}",
			states:   map[string]interpolation.StepStateData{},
			wantErr:  true,
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

func TestTemplateResolver_LoopDataParent(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		template string
		loop     *interpolation.LoopData
		want     string
		wantErr  bool
	}{
		// Nil parent - single loop, no nesting
		{
			name:     "nil parent - single loop",
			template: "{{.loop.Index}}",
			loop: &interpolation.LoopData{
				Index:  3,
				Item:   "item3",
				Parent: nil,
			},
			want: "3",
		},
		{
			name:     "nil parent with all fields",
			template: "{{.loop.Item}} at {{.loop.Index}}",
			loop: &interpolation.LoopData{
				Item:   "single-loop",
				Index:  0,
				First:  true,
				Last:   true,
				Length: 1,
				Parent: nil,
			},
			want: "single-loop at 0",
		},

		// Single parent - two-level nesting
		{
			name:     "parent.Index access",
			template: "{{.loop.Parent.Index}}",
			loop: &interpolation.LoopData{
				Index: 2,
				Item:  "inner",
				Parent: &interpolation.LoopData{
					Index: 5,
					Item:  "outer",
				},
			},
			want: "5",
		},
		{
			name:     "parent.Item access",
			template: "{{.loop.Parent.Item}}",
			loop: &interpolation.LoopData{
				Index: 1,
				Item:  "child",
				Parent: &interpolation.LoopData{
					Index: 0,
					Item:  "parent",
				},
			},
			want: "parent",
		},
		{
			name:     "parent.Index1 method access",
			template: "{{.loop.Parent.Index1}}",
			loop: &interpolation.LoopData{
				Index: 2,
				Item:  "inner",
				Parent: &interpolation.LoopData{
					Index: 0,
					Item:  "outer",
				},
			},
			want: "1",
		},
		{
			name:     "parent and child indexes",
			template: "outer[{{.loop.Parent.Index}}].inner[{{.loop.Index}}]",
			loop: &interpolation.LoopData{
				Index: 3,
				Item:  "inner-item",
				Parent: &interpolation.LoopData{
					Index: 1,
					Item:  "outer-item",
				},
			},
			want: "outer[1].inner[3]",
		},
		{
			name:     "parent.First flag",
			template: "{{.loop.Parent.First}}",
			loop: &interpolation.LoopData{
				Index: 2,
				First: false,
				Parent: &interpolation.LoopData{
					Index: 0,
					First: true,
				},
			},
			want: "true",
		},
		{
			name:     "parent.Last flag",
			template: "{{.loop.Parent.Last}}",
			loop: &interpolation.LoopData{
				Index: 1,
				Last:  false,
				Parent: &interpolation.LoopData{
					Index: 4,
					Last:  true,
				},
			},
			want: "true",
		},
		{
			name:     "parent.Length",
			template: "{{.loop.Parent.Length}}",
			loop: &interpolation.LoopData{
				Index:  0,
				Length: 5,
				Parent: &interpolation.LoopData{
					Index:  2,
					Length: 10,
				},
			},
			want: "10",
		},

		// Nested parents - three-level nesting (grandparent access)
		{
			name:     "grandparent.Index access",
			template: "{{.loop.Parent.Parent.Index}}",
			loop: &interpolation.LoopData{
				Index: 0,
				Item:  "level3",
				Parent: &interpolation.LoopData{
					Index: 1,
					Item:  "level2",
					Parent: &interpolation.LoopData{
						Index: 2,
						Item:  "level1",
					},
				},
			},
			want: "2",
		},
		{
			name:     "grandparent.Item access",
			template: "{{.loop.Parent.Parent.Item}}",
			loop: &interpolation.LoopData{
				Index: 0,
				Item:  "innermost",
				Parent: &interpolation.LoopData{
					Index: 0,
					Item:  "middle",
					Parent: &interpolation.LoopData{
						Index: 0,
						Item:  "outermost",
					},
				},
			},
			want: "outermost",
		},
		{
			name:     "grandparent.Index1 method",
			template: "{{.loop.Parent.Parent.Index1}}",
			loop: &interpolation.LoopData{
				Index: 5,
				Parent: &interpolation.LoopData{
					Index: 3,
					Parent: &interpolation.LoopData{
						Index: 7,
					},
				},
			},
			want: "8",
		},
		{
			name:     "all three levels - indexes",
			template: "[{{.loop.Parent.Parent.Index}},{{.loop.Parent.Index}},{{.loop.Index}}]",
			loop: &interpolation.LoopData{
				Index: 2,
				Item:  "c",
				Parent: &interpolation.LoopData{
					Index: 1,
					Item:  "b",
					Parent: &interpolation.LoopData{
						Index: 0,
						Item:  "a",
					},
				},
			},
			want: "[0,1,2]",
		},
		{
			name:     "all three levels - items",
			template: "{{.loop.Parent.Parent.Item}}/{{.loop.Parent.Item}}/{{.loop.Item}}",
			loop: &interpolation.LoopData{
				Index: 0,
				Item:  "file.txt",
				Parent: &interpolation.LoopData{
					Index: 0,
					Item:  "subdir",
					Parent: &interpolation.LoopData{
						Index: 0,
						Item:  "rootdir",
					},
				},
			},
			want: "rootdir/subdir/file.txt",
		},
		{
			name:     "all three levels - 1-based indexes",
			template: "{{.loop.Parent.Parent.Index1}}.{{.loop.Parent.Index1}}.{{.loop.Index1}}",
			loop: &interpolation.LoopData{
				Index: 0,
				Parent: &interpolation.LoopData{
					Index: 0,
					Parent: &interpolation.LoopData{
						Index: 0,
					},
				},
			},
			want: "1.1.1",
		},
		{
			name:     "three levels with First/Last flags",
			template: "L1={{.loop.Parent.Parent.First}},L2={{.loop.Parent.Last}},L3={{.loop.First}}",
			loop: &interpolation.LoopData{
				Index: 0,
				First: true,
				Last:  false,
				Parent: &interpolation.LoopData{
					Index: 2,
					First: false,
					Last:  true,
					Parent: &interpolation.LoopData{
						Index: 0,
						First: true,
						Last:  false,
					},
				},
			},
			want: "L1=true,L2=true,L3=true",
		},

		// Complex item types with parent
		// Note: Parent.Item doesn't get serialized automatically, only the current loop.Item does
		{
			name:     "parent with map item (Go format)",
			template: "{{.loop.Parent.Item}}",
			loop: &interpolation.LoopData{
				Index: 0,
				Item:  "child",
				Parent: &interpolation.LoopData{
					Index: 1,
					Item: map[string]any{
						"name": "parent-map",
						"id":   42,
					},
				},
			},
			want: `map[id:42 name:parent-map]`,
		},
		{
			name:     "parent with slice item (Go format)",
			template: "{{.loop.Parent.Item}}",
			loop: &interpolation.LoopData{
				Index: 0,
				Item:  "child",
				Parent: &interpolation.LoopData{
					Index: 0,
					Item:  []string{"a", "b", "c"},
				},
			},
			want: `[a b c]`,
		},
		{
			name:     "parent with string item",
			template: "{{.loop.Parent.Item}}",
			loop: &interpolation.LoopData{
				Index: 1,
				Item:  "child-item",
				Parent: &interpolation.LoopData{
					Index: 0,
					Item:  "parent-item",
				},
			},
			want: "parent-item",
		},

		// Conditional usage with parent
		{
			name:     "conditional on parent.First",
			template: "{{if .loop.Parent.First}}FIRST{{else}}NOT-FIRST{{end}}",
			loop: &interpolation.LoopData{
				Index: 2,
				Parent: &interpolation.LoopData{
					Index: 0,
					First: true,
				},
			},
			want: "FIRST",
		},
		{
			name:     "conditional on parent.Last",
			template: "{{if .loop.Parent.Last}}LAST{{else}}NOT-LAST{{end}}",
			loop: &interpolation.LoopData{
				Index: 1,
				Parent: &interpolation.LoopData{
					Index: 5,
					Last:  true,
				},
			},
			want: "LAST",
		},

		// Realistic nested loop templates
		{
			name:     "nested loop progress indicator",
			template: "Processing {{.loop.Parent.Index1}}/{{.loop.Parent.Length}}: {{.loop.Item}} ({{.loop.Index1}}/{{.loop.Length}})",
			loop: &interpolation.LoopData{
				Item:   "task-3",
				Index:  2,
				Length: 5,
				Parent: &interpolation.LoopData{
					Index:  4,
					Length: 10,
				},
			},
			want: "Processing 5/10: task-3 (3/5)",
		},
		{
			name:     "nested file path construction",
			template: "{{.loop.Parent.Item}}/{{.loop.Item}}",
			loop: &interpolation.LoopData{
				Index: 1,
				Item:  "file.go",
				Parent: &interpolation.LoopData{
					Index: 0,
					Item:  "pkg/interpolation",
				},
			},
			want: "pkg/interpolation/file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = tt.loop

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

// TestLoopData_ParentChain tests the parent chain structure directly
func TestLoopData_ParentChain(t *testing.T) {
	// Three-level nesting
	level1 := &interpolation.LoopData{
		Item:   "level1",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 3,
		Parent: nil,
	}

	level2 := &interpolation.LoopData{
		Item:   "level2",
		Index:  1,
		First:  false,
		Last:   false,
		Length: 5,
		Parent: level1,
	}

	level3 := &interpolation.LoopData{
		Item:   "level3",
		Index:  2,
		First:  false,
		Last:   true,
		Length: 4,
		Parent: level2,
	}

	// Verify level3 can access level2
	assert.NotNil(t, level3.Parent)
	assert.Equal(t, "level2", level3.Parent.Item)
	assert.Equal(t, 1, level3.Parent.Index)
	assert.Equal(t, 2, level3.Parent.Index1())

	// Verify level3 can access level1 through level2
	assert.NotNil(t, level3.Parent.Parent)
	assert.Equal(t, "level1", level3.Parent.Parent.Item)
	assert.Equal(t, 0, level3.Parent.Parent.Index)
	assert.Equal(t, 1, level3.Parent.Parent.Index1())

	// Verify level1 has no parent
	assert.Nil(t, level1.Parent)

	// Verify level2 parent chain
	assert.NotNil(t, level2.Parent)
	assert.Nil(t, level2.Parent.Parent)
}

// TestLoopData_ParentWithComplexItems tests parent with various item types
func TestLoopData_ParentWithComplexItems(t *testing.T) {
	tests := []struct {
		name       string
		parentItem any
		childItem  any
	}{
		{
			name:       "parent map, child string",
			parentItem: map[string]any{"key": "value", "count": 10},
			childItem:  "child-string",
		},
		{
			name:       "parent slice, child map",
			parentItem: []int{1, 2, 3},
			childItem:  map[string]string{"name": "child"},
		},
		{
			name:       "parent struct, child slice",
			parentItem: struct{ Name string }{"parent-struct"},
			childItem:  []string{"a", "b"},
		},
		{
			name:       "both maps",
			parentItem: map[string]int{"x": 1},
			childItem:  map[string]string{"y": "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := &interpolation.LoopData{
				Item:  tt.parentItem,
				Index: 0,
			}

			child := &interpolation.LoopData{
				Item:   tt.childItem,
				Index:  1,
				Parent: parent,
			}

			assert.Equal(t, tt.childItem, child.Item)
			assert.Equal(t, tt.parentItem, child.Parent.Item)
			assert.Equal(t, 0, child.Parent.Index)
			assert.Equal(t, 1, child.Index)
		})
	}
}

// TestTemplateResolver_LoopDataParent_ErrorCases tests error conditions
func TestTemplateResolver_LoopDataParent_ErrorCases(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		template string
		loop     *interpolation.LoopData
		wantErr  bool
	}{
		{
			name:     "access parent when nil",
			template: "{{.loop.Parent.Index}}",
			loop: &interpolation.LoopData{
				Index:  0,
				Parent: nil,
			},
			wantErr: true,
		},
		{
			name:     "access grandparent when parent.parent is nil",
			template: "{{.loop.Parent.Parent.Index}}",
			loop: &interpolation.LoopData{
				Index: 0,
				Parent: &interpolation.LoopData{
					Index:  1,
					Parent: nil,
				},
			},
			wantErr: true,
		},
		{
			name:     "access parent when loop is nil",
			template: "{{.loop.Parent.Index}}",
			loop:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.Loop = tt.loop

			_, err := resolver.Resolve(tt.template, ctx)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestTemplateResolver_ExpressionNamespaces tests lowercase expression namespace access
// for loop, context, and error namespaces
func TestTemplateResolver_ExpressionNamespaces(t *testing.T) {
	resolver := interpolation.NewTemplateResolver()

	tests := []struct {
		name     string
		template string
		setup    func(*interpolation.Context)
		want     string
		wantErr  bool
	}{
		// Loop namespace expressions
		{
			name:     "loop.index expression",
			template: "index: {{loop.index}}",
			setup: func(ctx *interpolation.Context) {
				ctx.Loop = &interpolation.LoopData{Index: 5}
			},
			want: "index: 5",
		},
		{
			name:     "loop.first boolean",
			template: "first: {{loop.first}}",
			setup: func(ctx *interpolation.Context) {
				ctx.Loop = &interpolation.LoopData{First: true}
			},
			want: "first: true",
		},
		{
			name:     "loop.last boolean",
			template: "last: {{loop.last}}",
			setup: func(ctx *interpolation.Context) {
				ctx.Loop = &interpolation.LoopData{Last: false}
			},
			want: "last: false",
		},

		// Context namespace expressions
		{
			name:     "context.working_dir",
			template: "dir: {{context.working_dir}}",
			setup: func(ctx *interpolation.Context) {
				ctx.Context.WorkingDir = "/home/user/project"
			},
			want: "dir: /home/user/project",
		},
		{
			name:     "context.user",
			template: "user: {{context.user}}",
			setup: func(ctx *interpolation.Context) {
				ctx.Context.User = "alice"
			},
			want: "user: alice",
		},
		{
			name:     "context.hostname",
			template: "host: {{context.hostname}}",
			setup: func(ctx *interpolation.Context) {
				ctx.Context.Hostname = "server-01"
			},
			want: "host: server-01",
		},

		// Error namespace expressions
		{
			name:     "error.message",
			template: "error: {{error.message}}",
			setup: func(ctx *interpolation.Context) {
				ctx.Error = &interpolation.ErrorData{
					Message: "command failed",
				}
			},
			want: "error: command failed",
		},
		{
			name:     "error.exit_code",
			template: "code: {{error.exit_code}}",
			setup: func(ctx *interpolation.Context) {
				ctx.Error = &interpolation.ErrorData{
					ExitCode: 127,
				}
			},
			want: "code: 127",
		},
		{
			name:     "error.type",
			template: "type: {{error.type}}",
			setup: func(ctx *interpolation.Context) {
				ctx.Error = &interpolation.ErrorData{
					Type: "execution_error",
				}
			},
			want: "type: execution_error",
		},

		// Missing namespace tests
		{
			name:     "missing loop context",
			template: "{{loop.index}}",
			setup:    func(ctx *interpolation.Context) {},
			wantErr:  true,
		},
		{
			name:     "missing error context",
			template: "{{error.message}}",
			setup:    func(ctx *interpolation.Context) {},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			tt.setup(ctx)

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
