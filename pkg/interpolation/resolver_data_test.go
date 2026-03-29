package interpolation_test

import (
	"testing"

	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateResolver_StepStateDataData(t *testing.T) {
	tests := []struct {
		name     string
		template string
		states   map[string]interpolation.StepStateData
		want     string
		wantErr  bool
	}{
		{
			name:     "data map with string values",
			template: "result: {{.states.custom_step.Data.result}}",
			states: map[string]interpolation.StepStateData{
				"custom_step": {
					Output: "step completed",
					Data: map[string]any{
						"result": "success",
					},
				},
			},
			want: "result: success",
		},
		{
			name:     "data map with numeric values",
			template: "count: {{.states.counter.Data.count}}, score: {{.states.counter.Data.score}}",
			states: map[string]interpolation.StepStateData{
				"counter": {
					Output: "computed",
					Data: map[string]any{
						"count": 42,
						"score": 98.5,
					},
				},
			},
			want: "count: 42, score: 98.5",
		},
		{
			name:     "data map with nested objects",
			template: "city: {{.states.location.Data.address.city}}",
			states: map[string]interpolation.StepStateData{
				"location": {
					Output: "resolved",
					Data: map[string]any{
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
			name:     "data map with deeply nested objects",
			template: "value: {{.states.nested.Data.level1.level2.level3.value}}",
			states: map[string]interpolation.StepStateData{
				"nested": {
					Output: "ok",
					Data: map[string]any{
						"level1": map[string]any{
							"level2": map[string]any{
								"level3": map[string]any{
									"value": "deep_value",
								},
							},
						},
					},
				},
			},
			want: "value: deep_value",
		},
		{
			name:     "nil data field handling",
			template: "{{.states.no_data.Output}}",
			states: map[string]interpolation.StepStateData{
				"no_data": {
					Output: "plain output",
					Data:   nil,
				},
			},
			want: "plain output",
		},
		{
			name:     "empty data map",
			template: "output: {{.states.empty_data.Output}}",
			states: map[string]interpolation.StepStateData{
				"empty_data": {
					Output: "has output",
					Data:   map[string]any{},
				},
			},
			want: "output: has output",
		},
		{
			name:     "data map with boolean values",
			template: "enabled: {{.states.config.Data.enabled}}, disabled: {{.states.config.Data.disabled}}",
			states: map[string]interpolation.StepStateData{
				"config": {
					Output: "configured",
					Data: map[string]any{
						"enabled":  true,
						"disabled": false,
					},
				},
			},
			want: "enabled: true, disabled: false",
		},
		{
			name:     "data map with array values",
			template: "items: {{.states.list_data.Data.items}}",
			states: map[string]interpolation.StepStateData{
				"list_data": {
					Output: "retrieved",
					Data: map[string]any{
						"items": []any{"a", "b", "c"},
					},
				},
			},
			want: "items: [a b c]",
		},
		{
			name:     "undefined data key",
			template: "{{.states.custom_step.Data.nonexistent}}",
			states: map[string]interpolation.StepStateData{
				"custom_step": {
					Output: "step completed",
					Data: map[string]any{
						"result": "success",
					},
				},
			},
			wantErr: true,
		},
		{
			name:     "data field with same step having other fields",
			template: "data: {{.states.multi.Data.value}}, output: {{.states.multi.Output}}, code: {{.states.multi.ExitCode}}",
			states: map[string]interpolation.StepStateData{
				"multi": {
					Output:   "done",
					ExitCode: 0,
					Data: map[string]any{
						"value": "data_value",
					},
				},
			},
			want: "data: data_value, output: done, code: 0",
		},
		{
			name:     "multiple steps with different data",
			template: "step1: {{.states.step1.Data.key}}, step2: {{.states.step2.Data.key}}",
			states: map[string]interpolation.StepStateData{
				"step1": {
					Output: "first",
					Data: map[string]any{
						"key": "value1",
					},
				},
				"step2": {
					Output: "second",
					Data: map[string]any{
						"key": "value2",
					},
				},
			},
			want: "step1: value1, step2: value2",
		},
		{
			name:     "data field with string that looks like template",
			template: "data: {{.states.step.Data.template_string}}",
			states: map[string]interpolation.StepStateData{
				"step": {
					Output: "processed",
					Data: map[string]any{
						"template_string": "{{not_interpolated}}",
					},
				},
			},
			want: "data: {{not_interpolated}}",
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

func TestStepStateData_DataFieldType(t *testing.T) {
	state := interpolation.StepStateData{
		Output: "test",
		Data: map[string]any{
			"string_val": "text",
			"int_val":    123,
			"float_val":  45.67,
			"bool_val":   true,
			"nil_val":    nil,
		},
	}

	assert.NotNil(t, state.Data)
	assert.Len(t, state.Data, 5)
	assert.Equal(t, "text", state.Data["string_val"])
	assert.Equal(t, 123, state.Data["int_val"])
	assert.Equal(t, 45.67, state.Data["float_val"])
	assert.Equal(t, true, state.Data["bool_val"])
	assert.Nil(t, state.Data["nil_val"])
}

func TestStepStateData_DataFieldInitialization(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want bool
	}{
		{
			name: "non-nil initialized data",
			data: map[string]any{"key": "value"},
			want: true,
		},
		{
			name: "nil data",
			data: nil,
			want: false,
		},
		{
			name: "empty map",
			data: map[string]any{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := interpolation.StepStateData{
				Output: "test",
				Data:   tt.data,
			}

			if tt.want {
				require.NotNil(t, state.Data)
			} else {
				assert.Nil(t, state.Data)
			}
		})
	}
}
