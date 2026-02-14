package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// mapLoopConfig Tests (F016)

func TestMapLoopConfig_ForEach(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.LoopConfig
	}{
		{
			name: "for_each with string items",
			yamlStep: yamlStep{
				Type:          "for_each",
				Items:         "{{inputs.files}}",
				Body:          []string{"process"},
				MaxIterations: 50,
				OnComplete:    "done",
			},
			want: &workflow.LoopConfig{
				Type:          workflow.LoopTypeForEach,
				Items:         "{{inputs.files}}",
				Body:          []string{"process"},
				MaxIterations: 50,
				OnComplete:    "done",
			},
		},
		{
			name: "for_each with array items",
			yamlStep: yamlStep{
				Type:          "for_each",
				Items:         []any{"a.txt", "b.txt", "c.txt"},
				Body:          []string{"process_file"},
				MaxIterations: 100,
			},
			want: &workflow.LoopConfig{
				Type:          workflow.LoopTypeForEach,
				Items:         `["a.txt","b.txt","c.txt"]`,
				Body:          []string{"process_file"},
				MaxIterations: 100,
			},
		},
		{
			name: "for_each with default max_iterations",
			yamlStep: yamlStep{
				Type:  "for_each",
				Items: "{{inputs.items}}",
				Body:  []string{"step1"},
			},
			want: &workflow.LoopConfig{
				Type:          workflow.LoopTypeForEach,
				Items:         "{{inputs.items}}",
				Body:          []string{"step1"},
				MaxIterations: workflow.DefaultMaxIterations,
			},
		},
		{
			name: "for_each with break condition",
			yamlStep: yamlStep{
				Type:          "for_each",
				Items:         `["a", "b", "c"]`,
				Body:          []string{"check"},
				MaxIterations: 10,
				BreakWhen:     "states.check.output == 'stop'",
				OnComplete:    "next",
			},
			want: &workflow.LoopConfig{
				Type:           workflow.LoopTypeForEach,
				Items:          `["a", "b", "c"]`,
				Body:           []string{"check"},
				MaxIterations:  10,
				BreakCondition: "states.check.output == 'stop'",
				OnComplete:     "next",
			},
		},
		{
			name: "for_each with multiple body steps",
			yamlStep: yamlStep{
				Type:          "for_each",
				Items:         "{{inputs.urls}}",
				Body:          []string{"fetch", "parse", "store"},
				MaxIterations: 200,
			},
			want: &workflow.LoopConfig{
				Type:          workflow.LoopTypeForEach,
				Items:         "{{inputs.urls}}",
				Body:          []string{"fetch", "parse", "store"},
				MaxIterations: 200,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapLoopConfig(&tt.yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Items, got.Items)
			assert.Equal(t, tt.want.Body, got.Body)
			assert.Equal(t, tt.want.MaxIterations, got.MaxIterations)
			assert.Equal(t, tt.want.BreakCondition, got.BreakCondition)
			assert.Equal(t, tt.want.OnComplete, got.OnComplete)
		})
	}
}

func TestMapLoopConfig_While(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
		want     *workflow.LoopConfig
	}{
		{
			name: "while with condition",
			yamlStep: yamlStep{
				Type:          "while",
				While:         "states.check.output != 'ready'",
				Body:          []string{"check", "wait"},
				MaxIterations: 60,
				OnComplete:    "proceed",
			},
			want: &workflow.LoopConfig{
				Type:          workflow.LoopTypeWhile,
				Condition:     "states.check.output != 'ready'",
				Body:          []string{"check", "wait"},
				MaxIterations: 60,
				OnComplete:    "proceed",
			},
		},
		{
			name: "while with default max_iterations",
			yamlStep: yamlStep{
				Type:  "while",
				While: "true",
				Body:  []string{"poll"},
			},
			want: &workflow.LoopConfig{
				Type:          workflow.LoopTypeWhile,
				Condition:     "true",
				Body:          []string{"poll"},
				MaxIterations: workflow.DefaultMaxIterations,
			},
		},
		{
			name: "while with break condition",
			yamlStep: yamlStep{
				Type:          "while",
				While:         "true",
				Body:          []string{"work"},
				MaxIterations: 1000,
				BreakWhen:     "states.work.exit_code != 0",
			},
			want: &workflow.LoopConfig{
				Type:           workflow.LoopTypeWhile,
				Condition:      "true",
				Body:           []string{"work"},
				MaxIterations:  1000,
				BreakCondition: "states.work.exit_code != 0",
			},
		},
		{
			name: "while with expression condition",
			yamlStep: yamlStep{
				Type:          "while",
				While:         "states.counter.output < 10",
				Body:          []string{"increment"},
				MaxIterations: 15,
			},
			want: &workflow.LoopConfig{
				Type:          workflow.LoopTypeWhile,
				Condition:     "states.counter.output < 10",
				Body:          []string{"increment"},
				MaxIterations: 15,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapLoopConfig(&tt.yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Condition, got.Condition)
			assert.Equal(t, tt.want.Body, got.Body)
			assert.Equal(t, tt.want.MaxIterations, got.MaxIterations)
			assert.Equal(t, tt.want.BreakCondition, got.BreakCondition)
			assert.Equal(t, tt.want.OnComplete, got.OnComplete)
		})
	}
}

func TestMapLoopConfig_NoLoop(t *testing.T) {
	tests := []struct {
		name     string
		yamlStep yamlStep
	}{
		{
			name: "command step",
			yamlStep: yamlStep{
				Type:    "step",
				Command: "echo hello",
			},
		},
		{
			name: "parallel step",
			yamlStep: yamlStep{
				Type:     "parallel",
				Parallel: []string{"branch1", "branch2"},
			},
		},
		{
			name: "terminal step",
			yamlStep: yamlStep{
				Type: "terminal",
			},
		},
		{
			name:     "empty step",
			yamlStep: yamlStep{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapLoopConfig(&tt.yamlStep)
			assert.Nil(t, got)
		})
	}
}

// parseStepType Tests for Loop Types (F016)

func TestParseStepType_LoopTypes(t *testing.T) {
	tests := []struct {
		input   string
		want    workflow.StepType
		wantErr bool
	}{
		{"for_each", workflow.StepTypeForEach, false},
		{"FOR_EACH", workflow.StepTypeForEach, false},
		{"For_Each", workflow.StepTypeForEach, false},
		{"while", workflow.StepTypeWhile, false},
		{"WHILE", workflow.StepTypeWhile, false},
		{"While", workflow.StepTypeWhile, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseStepType(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// mapStep Tests for Loop Steps (F016)

func TestMapStep_ForEachStep(t *testing.T) {
	yamlStep := yamlStep{
		Type:          "for_each",
		Description:   "Process each file",
		Items:         "{{inputs.files}}",
		Body:          []string{"process_file", "validate_output"},
		MaxIterations: 100,
		BreakWhen:     "states.validate_output.exit_code != 0",
		OnComplete:    "aggregate",
		Timeout:       "5m",
	}

	step, err := mapStep("test.yaml", "process_files", &yamlStep)

	require.NoError(t, err)
	require.NotNil(t, step)
	assert.Equal(t, "process_files", step.Name)
	assert.Equal(t, workflow.StepTypeForEach, step.Type)
	assert.Equal(t, "Process each file", step.Description)
	assert.Equal(t, 300, step.Timeout) // 5m = 300s

	require.NotNil(t, step.Loop)
	assert.Equal(t, workflow.LoopTypeForEach, step.Loop.Type)
	assert.Equal(t, "{{inputs.files}}", step.Loop.Items)
	assert.Equal(t, []string{"process_file", "validate_output"}, step.Loop.Body)
	assert.Equal(t, 100, step.Loop.MaxIterations)
	assert.Equal(t, "states.validate_output.exit_code != 0", step.Loop.BreakCondition)
	assert.Equal(t, "aggregate", step.Loop.OnComplete)
}

func TestMapStep_WhileStep(t *testing.T) {
	yamlStep := yamlStep{
		Type:          "while",
		Description:   "Poll until ready",
		While:         "states.check.output != 'ready'",
		Body:          []string{"check", "sleep"},
		MaxIterations: 60,
		OnComplete:    "proceed",
		Timeout:       "10m",
	}

	step, err := mapStep("test.yaml", "poll_status", &yamlStep)

	require.NoError(t, err)
	require.NotNil(t, step)
	assert.Equal(t, "poll_status", step.Name)
	assert.Equal(t, workflow.StepTypeWhile, step.Type)
	assert.Equal(t, "Poll until ready", step.Description)
	assert.Equal(t, 600, step.Timeout) // 10m = 600s

	require.NotNil(t, step.Loop)
	assert.Equal(t, workflow.LoopTypeWhile, step.Loop.Type)
	assert.Equal(t, "states.check.output != 'ready'", step.Loop.Condition)
	assert.Equal(t, []string{"check", "sleep"}, step.Loop.Body)
	assert.Equal(t, 60, step.Loop.MaxIterations)
	assert.Equal(t, "proceed", step.Loop.OnComplete)
}

func TestMapStep_LoopWithHooks(t *testing.T) {
	yamlStep := yamlStep{
		Type:          "for_each",
		Items:         `["item1", "item2"]`,
		Body:          []string{"process"},
		MaxIterations: 10,
		Hooks: &yamlStepHooks{
			Pre:  []yamlHookAction{{Log: "Starting loop"}},
			Post: []yamlHookAction{{Log: "Loop complete"}},
		},
	}

	step, err := mapStep("test.yaml", "loop_with_hooks", &yamlStep)

	require.NoError(t, err)
	require.NotNil(t, step)
	require.NotNil(t, step.Loop)
	assert.Len(t, step.Hooks.Pre, 1)
	assert.Len(t, step.Hooks.Post, 1)
	assert.Equal(t, "Starting loop", step.Hooks.Pre[0].Log)
	assert.Equal(t, "Loop complete", step.Hooks.Post[0].Log)
}

func TestMapLoopConfig_ItemsTypes(t *testing.T) {
	// Test different item types that might come from YAML parsing
	tests := []struct {
		name     string
		items    any
		wantJSON string
	}{
		{
			name:     "string items",
			items:    "{{inputs.list}}",
			wantJSON: "{{inputs.list}}",
		},
		{
			name:     "array of strings",
			items:    []any{"a", "b", "c"},
			wantJSON: `["a","b","c"]`,
		},
		{
			name:     "array of integers",
			items:    []any{1, 2, 3},
			wantJSON: `[1,2,3]`,
		},
		{
			name:     "mixed array",
			items:    []any{"file.txt", 42, true},
			wantJSON: `["file.txt",42,true]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlStep := yamlStep{
				Items: tt.items,
				Body:  []string{"process"},
			}

			got := mapLoopConfig(&yamlStep)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantJSON, got.Items)
		})
	}
}

func TestMapStep_LoopPreservesOtherFields(t *testing.T) {
	// Ensure loop steps can have other standard fields
	yamlStep := yamlStep{
		Type:            "for_each",
		Description:     "Process files with retry",
		Items:           "{{inputs.files}}",
		Body:            []string{"process"},
		MaxIterations:   100,
		OnComplete:      "done",
		ContinueOnError: true,
	}

	step, err := mapStep("test.yaml", "resilient_loop", &yamlStep)

	require.NoError(t, err)
	require.NotNil(t, step)
	assert.Equal(t, workflow.StepTypeForEach, step.Type)
	assert.True(t, step.ContinueOnError)
	require.NotNil(t, step.Loop)
}

func TestMapLoopConfig_DynamicMaxIterations(t *testing.T) {
	tests := []struct {
		name            string
		maxIterations   any
		wantMaxIter     int
		wantMaxIterExpr string
		wantDynamic     bool
	}{
		{
			name:            "integer max_iterations",
			maxIterations:   50,
			wantMaxIter:     50,
			wantMaxIterExpr: "",
			wantDynamic:     false,
		},
		{
			name:            "string expression max_iterations",
			maxIterations:   "{{inputs.limit}}",
			wantMaxIter:     0,
			wantMaxIterExpr: "{{inputs.limit}}",
			wantDynamic:     true,
		},
		{
			name:            "env variable expression",
			maxIterations:   "{{env.MAX_RETRIES}}",
			wantMaxIter:     0,
			wantMaxIterExpr: "{{env.MAX_RETRIES}}",
			wantDynamic:     true,
		},
		{
			name:            "arithmetic expression",
			maxIterations:   "{{inputs.pages * inputs.retries_per_page}}",
			wantMaxIter:     0,
			wantMaxIterExpr: "{{inputs.pages * inputs.retries_per_page}}",
			wantDynamic:     true,
		},
		{
			name:            "simple arithmetic",
			maxIterations:   "{{inputs.a + inputs.b}}",
			wantMaxIter:     0,
			wantMaxIterExpr: "{{inputs.a + inputs.b}}",
			wantDynamic:     true,
		},
		{
			name:            "nil uses default",
			maxIterations:   nil,
			wantMaxIter:     workflow.DefaultMaxIterations,
			wantMaxIterExpr: "",
			wantDynamic:     false,
		},
		{
			name:            "zero integer uses default",
			maxIterations:   0,
			wantMaxIter:     workflow.DefaultMaxIterations,
			wantMaxIterExpr: "",
			wantDynamic:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlStep := yamlStep{
				Type:          "for_each",
				Items:         "{{inputs.items}}",
				Body:          []string{"process"},
				MaxIterations: tt.maxIterations,
			}

			got := mapLoopConfig(&yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.wantMaxIter, got.MaxIterations, "MaxIterations mismatch")
			assert.Equal(t, tt.wantMaxIterExpr, got.MaxIterationsExpr, "MaxIterationsExpr mismatch")
			assert.Equal(t, tt.wantDynamic, got.IsMaxIterationsDynamic(), "IsMaxIterationsDynamic mismatch")
		})
	}
}

func TestMapLoopConfig_DynamicMaxIterations_While(t *testing.T) {
	tests := []struct {
		name            string
		maxIterations   any
		wantMaxIter     int
		wantMaxIterExpr string
	}{
		{
			name:            "while with dynamic max_iterations",
			maxIterations:   "{{inputs.max_retries}}",
			wantMaxIter:     0,
			wantMaxIterExpr: "{{inputs.max_retries}}",
		},
		{
			name:            "while with integer max_iterations",
			maxIterations:   30,
			wantMaxIter:     30,
			wantMaxIterExpr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlStep := yamlStep{
				Type:          "while",
				While:         "states.check.output != 'done'",
				Body:          []string{"check"},
				MaxIterations: tt.maxIterations,
			}

			got := mapLoopConfig(&yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, workflow.LoopTypeWhile, got.Type)
			assert.Equal(t, tt.wantMaxIter, got.MaxIterations)
			assert.Equal(t, tt.wantMaxIterExpr, got.MaxIterationsExpr)
		})
	}
}

func TestMapLoopConfig_MaxIterationsEdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		maxIterations   any
		wantMaxIter     int
		wantMaxIterExpr string
	}{
		{
			name:            "empty string expression treated as dynamic",
			maxIterations:   "",
			wantMaxIter:     workflow.DefaultMaxIterations, // empty string should use default
			wantMaxIterExpr: "",
		},
		{
			name:            "large integer",
			maxIterations:   9999,
			wantMaxIter:     9999,
			wantMaxIterExpr: "",
		},
		{
			name:            "expression with state reference",
			maxIterations:   "{{states.setup.output}}",
			wantMaxIter:     0,
			wantMaxIterExpr: "{{states.setup.output}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlStep := yamlStep{
				Type:          "for_each",
				Items:         "{{inputs.items}}",
				Body:          []string{"process"},
				MaxIterations: tt.maxIterations,
			}

			got := mapLoopConfig(&yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.wantMaxIter, got.MaxIterations, "MaxIterations")
			assert.Equal(t, tt.wantMaxIterExpr, got.MaxIterationsExpr, "MaxIterationsExpr")
		})
	}
}

func TestMapLoopConfig_MaxIterationsInvalidTypes(t *testing.T) {
	// These are edge cases where YAML might parse unexpected types
	// The mapper should handle them gracefully (use defaults)
	tests := []struct {
		name          string
		maxIterations any
		wantMaxIter   int
	}{
		{
			name:          "float falls back to default",
			maxIterations: 3.14,
			wantMaxIter:   workflow.DefaultMaxIterations,
		},
		{
			name:          "bool falls back to default",
			maxIterations: true,
			wantMaxIter:   workflow.DefaultMaxIterations,
		},
		{
			name:          "slice falls back to default",
			maxIterations: []string{"a", "b"},
			wantMaxIter:   workflow.DefaultMaxIterations,
		},
		{
			name:          "map falls back to default",
			maxIterations: map[string]int{"x": 1},
			wantMaxIter:   workflow.DefaultMaxIterations,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlStep := yamlStep{
				Type:          "for_each",
				Items:         "{{inputs.items}}",
				Body:          []string{"process"},
				MaxIterations: tt.maxIterations,
			}

			got := mapLoopConfig(&yamlStep)

			require.NotNil(t, got)
			assert.Equal(t, tt.wantMaxIter, got.MaxIterations)
			assert.Empty(t, got.MaxIterationsExpr, "invalid types should not set MaxIterationsExpr")
		})
	}
}

func TestMapStep_DynamicMaxIterations(t *testing.T) {
	yamlStep := yamlStep{
		Type:          "for_each",
		Description:   "Dynamic iteration limit",
		Items:         "{{inputs.files}}",
		Body:          []string{"process"},
		MaxIterations: "{{inputs.limit}}",
		OnComplete:    "done",
	}

	step, err := mapStep("test.yaml", "dynamic_loop", &yamlStep)

	require.NoError(t, err)
	require.NotNil(t, step)
	require.NotNil(t, step.Loop)

	assert.Equal(t, 0, step.Loop.MaxIterations, "MaxIterations should be 0 when expression is used")
	assert.Equal(t, "{{inputs.limit}}", step.Loop.MaxIterationsExpr)
	assert.True(t, step.Loop.IsMaxIterationsDynamic())
}

func TestMapStep_DynamicMaxIterations_WithArithmetic(t *testing.T) {
	yamlStep := yamlStep{
		Type:          "for_each",
		Description:   "Calculate iteration limit",
		Items:         "{{inputs.pages}}",
		Body:          []string{"fetch_page"},
		MaxIterations: "{{inputs.pages * inputs.retries_per_page}}",
		OnComplete:    "aggregate",
	}

	step, err := mapStep("test.yaml", "paginated_loop", &yamlStep)

	require.NoError(t, err)
	require.NotNil(t, step)
	require.NotNil(t, step.Loop)

	assert.Equal(t, 0, step.Loop.MaxIterations)
	assert.Equal(t, "{{inputs.pages * inputs.retries_per_page}}", step.Loop.MaxIterationsExpr)
	assert.True(t, step.Loop.IsMaxIterationsDynamic())
}
