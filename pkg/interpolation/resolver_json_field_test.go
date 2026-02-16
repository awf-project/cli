package interpolation_test

import (
	"testing"

	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTemplateResolver_StepStateDataJSON tests the JSON field on StepStateData
// for F065: Output Format for Agent Steps.
// This field stores explicitly parsed JSON output (separate from auto-parsed Response field).
func TestTemplateResolver_StepStateDataJSON(t *testing.T) {
	tests := []struct {
		name     string
		template string
		states   map[string]interpolation.StepStateData
		want     string
		wantErr  bool
	}{
		{
			name:     "JSON field with simple string value",
			template: "name: {{.states.agent_step.JSON.name}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `{"name":"alice","count":3}`,
					JSON: map[string]any{
						"name": "alice",
					},
				},
			},
			want: "name: alice",
		},
		{
			name:     "JSON field with numeric value",
			template: "count: {{.states.agent_step.JSON.count}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `{"name":"alice","count":3}`,
					JSON: map[string]any{
						"count": 3,
					},
				},
			},
			want: "count: 3",
		},
		{
			name:     "JSON field with multiple fields",
			template: "user: {{.states.agent_step.JSON.name}}, count: {{.states.agent_step.JSON.count}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `{"name":"alice","count":3}`,
					JSON: map[string]any{
						"name":  "alice",
						"count": 3,
					},
				},
			},
			want: "user: alice, count: 3",
		},
		{
			name:     "JSON field with nested objects",
			template: "city: {{.states.agent_step.JSON.address.city}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `{"address":{"city":"Seattle","zipcode":"98101"}}`,
					JSON: map[string]any{
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
			name:     "JSON field with deeply nested objects",
			template: "value: {{.states.agent_step.JSON.level1.level2.value}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `{"level1":{"level2":{"value":"deep"}}}`,
					JSON: map[string]any{
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
			name:     "JSON field is nil when not set",
			template: "{{.states.agent_step.Output}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: "plain output",
					JSON:   nil,
				},
			},
			want: "plain output",
		},
		{
			name:     "JSON field is empty map",
			template: "output: {{.states.agent_step.Output}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: "{}",
					JSON:   map[string]any{},
				},
			},
			want: "output: {}",
		},
		{
			name:     "JSON field with boolean values",
			template: "enabled: {{.states.agent_step.JSON.enabled}}, active: {{.states.agent_step.JSON.active}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `{"enabled":true,"active":false}`,
					JSON: map[string]any{
						"enabled": true,
						"active":  false,
					},
				},
			},
			want: "enabled: true, active: false",
		},
		{
			name:     "JSON field with float values",
			template: "score: {{.states.agent_step.JSON.score}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `{"score":98.5}`,
					JSON: map[string]any{
						"score": 98.5,
					},
				},
			},
			want: "score: 98.5",
		},
		{
			name:     "JSON field separate from Response field",
			template: "json_name: {{.states.agent_step.JSON.name}}, response_role: {{.states.agent_step.Response.role}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `{"name":"alice"}`,
					JSON: map[string]any{
						"name": "alice",
					},
					Response: map[string]any{
						"role": "admin",
					},
				},
			},
			want: "json_name: alice, response_role: admin",
		},
		{
			name:     "accessing non-existent JSON field should error",
			template: "{{.states.agent_step.JSON.nonexistent}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `{"name":"alice"}`,
					JSON: map[string]any{
						"name": "alice",
					},
				},
			},
			wantErr: true,
		},
		{
			name:     "accessing JSON on step without JSON field should error",
			template: "{{.states.command_step.JSON.name}}",
			states: map[string]interpolation.StepStateData{
				"command_step": {
					Output: "plain command output",
					JSON:   nil,
				},
			},
			wantErr: true,
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

// TestTemplateResolver_JSONFieldWithArrays tests JSON field access for array outputs
// (F065 US2: array elements accessible via index notation).
// Note: This test documents the EXPECTED behavior once array handling is implemented.
// Arrays in JSON will be stored as []any and need proper handling in the execution layer.
func TestTemplateResolver_JSONFieldWithArrays(t *testing.T) {
	t.Skip("STUB: Array handling in JSON field not yet implemented - needs execution layer support")

	tests := []struct {
		name     string
		template string
		states   map[string]interpolation.StepStateData
		want     string
		wantErr  bool
	}{
		{
			name:     "JSON field with array stored as slice",
			template: "first: {{index .states.agent_step.JSON.items 0}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `{"items":["alice","bob","charlie"]}`,
					JSON: map[string]any{
						"items": []any{"alice", "bob", "charlie"},
					},
				},
			},
			want: "first: alice",
		},
		{
			name:     "top-level array in JSON field",
			template: "count: {{len .states.agent_step.JSONArray}}",
			states: map[string]interpolation.StepStateData{
				"agent_step": {
					Output: `["alice","bob","charlie"]`,
					// When top-level is array, needs special handling
					// This is a stub for future implementation
				},
			},
			want:    "count: 3",
			wantErr: false,
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

// TestTemplateResolver_JSONFieldBackwardCompatibility tests that existing workflows
// without JSON field continue to work (F065 US4).
func TestTemplateResolver_JSONFieldBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		template string
		states   map[string]interpolation.StepStateData
		want     string
	}{
		{
			name:     "step without JSON field uses Output",
			template: "result: {{.states.step.Output}}",
			states: map[string]interpolation.StepStateData{
				"step": {
					Output: "raw output with markdown fences",
					JSON:   nil, // JSON not populated
				},
			},
			want: "result: raw output with markdown fences",
		},
		{
			name:     "step with Response but no JSON",
			template: "name: {{.states.step.Response.name}}",
			states: map[string]interpolation.StepStateData{
				"step": {
					Output: "success",
					Response: map[string]any{
						"name": "alice",
					},
					JSON: nil, // JSON not populated
				},
			},
			want: "name: alice",
		},
		{
			name:     "multiple steps without JSON field",
			template: "{{.states.step1.Output}} then {{.states.step2.Output}}",
			states: map[string]interpolation.StepStateData{
				"step1": {Output: "first", JSON: nil},
				"step2": {Output: "second", JSON: nil},
			},
			want: "first then second",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := interpolation.NewContext()
			ctx.States = tt.states

			got, err := resolver.Resolve(tt.template, ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTemplateResolver_JSONFieldComplexScenarios tests complex use cases
// combining JSON field with other interpolation features.
func TestTemplateResolver_JSONFieldComplexScenarios(t *testing.T) {
	tests := []struct {
		name     string
		template string
		ctx      func() *interpolation.Context
		want     string
		wantErr  bool
	}{
		{
			name:     "JSON field with workflow metadata",
			template: "workflow {{.workflow.Name}} processed {{.states.agent_step.JSON.count}} items",
			ctx: func() *interpolation.Context {
				c := interpolation.NewContext()
				c.Workflow.Name = "test-workflow"
				c.States = map[string]interpolation.StepStateData{
					"agent_step": {
						JSON: map[string]any{"count": 42},
					},
				}
				return c
			},
			want: "workflow test-workflow processed 42 items",
		},
		{
			name:     "JSON field with inputs",
			template: "user {{.inputs.username}} has role {{.states.agent_step.JSON.role}}",
			ctx: func() *interpolation.Context {
				c := interpolation.NewContext()
				c.Inputs = map[string]any{"username": "alice"}
				c.States = map[string]interpolation.StepStateData{
					"agent_step": {
						JSON: map[string]any{"role": "admin"},
					},
				}
				return c
			},
			want: "user alice has role admin",
		},
		{
			name:     "multiple steps with JSON fields",
			template: "step1: {{.states.step1.JSON.name}}, step2: {{.states.step2.JSON.count}}",
			ctx: func() *interpolation.Context {
				c := interpolation.NewContext()
				c.States = map[string]interpolation.StepStateData{
					"step1": {
						JSON: map[string]any{"name": "alice"},
					},
					"step2": {
						JSON: map[string]any{"count": 3},
					},
				}
				return c
			},
			want: "step1: alice, step2: 3",
		},
		{
			name:     "JSON field in conditional template",
			template: `{{if .states.agent_step.JSON.enabled}}feature enabled{{else}}feature disabled{{end}}`,
			ctx: func() *interpolation.Context {
				c := interpolation.NewContext()
				c.States = map[string]interpolation.StepStateData{
					"agent_step": {
						JSON: map[string]any{"enabled": true},
					},
				}
				return c
			},
			want: "feature enabled",
		},
	}

	resolver := interpolation.NewTemplateResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx()

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
