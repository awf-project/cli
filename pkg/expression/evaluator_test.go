package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/pkg/interpolation"
)

func TestExprEvaluator_Evaluate(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		ctx     *interpolation.Context
		want    bool
		wantErr bool
	}{
		// Comparison operators - equality
		{
			name: "string equality true",
			expr: `inputs.mode == "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "full"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "string equality false",
			expr: `inputs.mode == "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "partial"},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "integer equality true",
			expr: `inputs.count == 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "integer equality false",
			expr: `inputs.count == 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 5},
			},
			want:    false,
			wantErr: false,
		},

		// Comparison operators - inequality
		{
			name: "string inequality true",
			expr: `inputs.mode != "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "partial"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "integer inequality true",
			expr: `inputs.count != 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 5},
			},
			want:    true,
			wantErr: false,
		},

		// Comparison operators - less than
		{
			name: "less than true",
			expr: `inputs.count < 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 5},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "less than false",
			expr: `inputs.count < 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 15},
			},
			want:    false,
			wantErr: false,
		},

		// Comparison operators - greater than
		{
			name: "greater than true",
			expr: `inputs.count > 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 15},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "greater than false",
			expr: `inputs.count > 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 5},
			},
			want:    false,
			wantErr: false,
		},

		// Comparison operators - less than or equal
		{
			name: "less than or equal true (less)",
			expr: `inputs.count <= 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 5},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "less than or equal true (equal)",
			expr: `inputs.count <= 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "less than or equal false",
			expr: `inputs.count <= 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 15},
			},
			want:    false,
			wantErr: false,
		},

		// Comparison operators - greater than or equal
		{
			name: "greater than or equal true (greater)",
			expr: `inputs.count >= 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 15},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "greater than or equal true (equal)",
			expr: `inputs.count >= 10`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},

		// Logical operators - and
		{
			name: "logical and true",
			expr: `inputs.mode == "full" and inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "full", "count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "logical and false (first false)",
			expr: `inputs.mode == "full" and inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "partial", "count": 10},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "logical and false (second false)",
			expr: `inputs.mode == "full" and inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "full", "count": 3},
			},
			want:    false,
			wantErr: false,
		},

		// Logical operators - or
		{
			name: "logical or true (first true)",
			expr: `inputs.mode == "full" or inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "full", "count": 3},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "logical or true (second true)",
			expr: `inputs.mode == "full" or inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "partial", "count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "logical or false",
			expr: `inputs.mode == "full" or inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "partial", "count": 3},
			},
			want:    false,
			wantErr: false,
		},

		// Logical operators - not
		{
			name: "logical not true",
			expr: `not (inputs.mode == "full")`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "partial"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "logical not false",
			expr: `not (inputs.mode == "full")`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "full"},
			},
			want:    false,
			wantErr: false,
		},

		// Parentheses grouping
		{
			name: "parentheses precedence",
			expr: `(inputs.a == 1 or inputs.b == 2) and inputs.c == 3`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"a": 1, "b": 0, "c": 3},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "parentheses precedence (without parens different result)",
			expr: `inputs.a == 1 or (inputs.b == 2 and inputs.c == 3)`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"a": 0, "b": 2, "c": 3},
			},
			want:    true,
			wantErr: false,
		},

		// Access to states (step results)
		{
			name: "access states exit_code",
			expr: `states.process.exit_code == 0`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"process": {ExitCode: 0, Output: "success"},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "access states output",
			expr: `states.process.output == "success"`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"process": {ExitCode: 0, Output: "success"},
				},
			},
			want:    true,
			wantErr: false,
		},

		// Complex condition from spec
		{
			name: "complex condition from spec",
			expr: `states.process.exit_code == 0 and inputs.mode == "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "full"},
				States: map[string]interpolation.StepStateData{
					"process": {ExitCode: 0},
				},
			},
			want:    true,
			wantErr: false,
		},

		// Access to env variables
		{
			name: "access env variable",
			expr: `env.DEBUG == "true"`,
			ctx: &interpolation.Context{
				Env: map[string]string{"DEBUG": "true"},
			},
			want:    true,
			wantErr: false,
		},

		// Access to workflow metadata
		{
			name: "access workflow name",
			expr: `workflow.name == "my-workflow"`,
			ctx: &interpolation.Context{
				Workflow: interpolation.WorkflowData{Name: "my-workflow"},
			},
			want:    true,
			wantErr: false,
		},

		// Error cases
		{
			name:    "invalid expression syntax",
			expr:    `inputs.mode ==`,
			ctx:     &interpolation.Context{Inputs: map[string]any{}},
			want:    false,
			wantErr: true,
		},
		{
			name: "undefined variable returns false",
			expr: `inputs.undefined_var == "value"`,
			ctx:  &interpolation.Context{Inputs: map[string]any{}},
			// expr library treats undefined as nil, comparison returns false
			want:    false,
			wantErr: false,
		},
		{
			name:    "empty expression",
			expr:    ``,
			ctx:     &interpolation.Context{},
			want:    false,
			wantErr: true,
		},

		// Type coercion
		{
			name: "string number comparison",
			expr: `inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": "10"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "float comparison",
			expr: `inputs.rate >= 0.5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"rate": 0.75},
			},
			want:    true,
			wantErr: false,
		},

		// Boolean values
		{
			name: "boolean true",
			expr: `inputs.enabled == true`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"enabled": true},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "boolean false",
			expr: `inputs.enabled == false`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"enabled": false},
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExprEvaluator()
			got, err := e.Evaluate(tt.expr, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildExprContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  *interpolation.Context
		want map[string]any
	}{
		{
			name: "full context",
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "full", "count": 10},
				States: map[string]interpolation.StepStateData{
					"step1": {Output: "out", Stderr: "err", ExitCode: 0, Status: "completed"},
				},
				Env: map[string]string{"HOME": "/home/user"},
				Workflow: interpolation.WorkflowData{
					ID:   "wf-123",
					Name: "my-workflow",
				},
			},
			want: map[string]any{
				"inputs": map[string]any{"mode": "full", "count": 10},
				"states": map[string]any{
					"step1": map[string]any{
						"output":    "out",
						"stderr":    "err",
						"exit_code": 0,
						"status":    "completed",
					},
				},
				"env": map[string]any{"HOME": "/home/user"},
				"workflow": map[string]any{
					"id":   "wf-123",
					"name": "my-workflow",
				},
			},
		},
		{
			name: "empty context",
			ctx:  &interpolation.Context{},
			want: map[string]any{
				"inputs":   map[string]any{},
				"states":   map[string]any{},
				"env":      map[string]any{},
				"workflow": map[string]any{},
			},
		},
		{
			name: "nil context",
			ctx:  nil,
			want: map[string]any{
				"inputs":   map[string]any{},
				"states":   map[string]any{},
				"env":      map[string]any{},
				"workflow": map[string]any{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildExprContext(tt.ctx)
			assert.NotNil(t, got)
			// Basic structure checks - detailed validation in implementation
			assert.Contains(t, got, "inputs")
			assert.Contains(t, got, "states")
			assert.Contains(t, got, "env")
			assert.Contains(t, got, "workflow")
		})
	}
}

// Additional edge case tests for expression evaluation

func TestExprEvaluator_StringComparisons(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		ctx     *interpolation.Context
		want    bool
		wantErr bool
	}{
		{
			name: "string contains check",
			expr: `inputs.message contains "error"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"message": "an error occurred"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "string does not contain",
			expr: `inputs.message contains "error"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"message": "all good"},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "string startsWith",
			expr: `inputs.path startsWith "/home"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"path": "/home/user"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "string endsWith",
			expr: `inputs.file endsWith ".go"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"file": "main.go"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "empty string comparison",
			expr: `inputs.value == ""`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"value": ""},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "string with special characters",
			expr: `inputs.pattern == "test.*\\d+"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"pattern": "test.*\\d+"},
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExprEvaluator()
			got, err := e.Evaluate(tt.expr, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExprEvaluator_NumericOperations(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		ctx     *interpolation.Context
		want    bool
		wantErr bool
	}{
		{
			name: "integer division comparison",
			expr: `inputs.total / 2 >= 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"total": 12},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "modulo operation",
			expr: `inputs.count % 2 == 0`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "negative number comparison",
			expr: `inputs.offset < 0`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"offset": -5},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "float precision comparison",
			expr: `inputs.ratio > 0.99 and inputs.ratio < 1.01`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"ratio": 1.0},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "mixed int and float comparison",
			expr: `inputs.value >= 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"value": 5.5},
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExprEvaluator()
			got, err := e.Evaluate(tt.expr, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExprEvaluator_ComplexLogicalExpressions(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		ctx     *interpolation.Context
		want    bool
		wantErr bool
	}{
		{
			name: "triple and",
			expr: `inputs.a == 1 and inputs.b == 2 and inputs.c == 3`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"a": 1, "b": 2, "c": 3},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "triple or",
			expr: `inputs.a == 1 or inputs.b == 2 or inputs.c == 3`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"a": 0, "b": 0, "c": 3},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "mixed and/or without parentheses",
			expr: `inputs.a == 1 and inputs.b == 2 or inputs.c == 3`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"a": 0, "b": 2, "c": 3},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "double negation",
			expr: `not (not (inputs.enabled == true))`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"enabled": true},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "nested parentheses",
			expr: `((inputs.a > 0) and (inputs.b > 0)) or (inputs.c > 0)`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"a": 1, "b": 1, "c": 0},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "de morgan law equivalent",
			expr: `not (inputs.a == true or inputs.b == true)`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"a": false, "b": false},
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExprEvaluator()
			got, err := e.Evaluate(tt.expr, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExprEvaluator_StateAccessPatterns(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		ctx     *interpolation.Context
		want    bool
		wantErr bool
	}{
		{
			name: "check step status completed",
			expr: `states.build.status == "completed"`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"build": {Status: "completed", ExitCode: 0},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "check step status failed",
			expr: `states.deploy.status == "failed"`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"deploy": {Status: "failed", ExitCode: 1},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "check exit code non-zero",
			expr: `states.test.exit_code != 0`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"test": {ExitCode: 127},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "check stderr is empty",
			expr: `states.compile.stderr == ""`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"compile": {Stderr: ""},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "combined state checks",
			expr: `states.build.exit_code == 0 and states.test.exit_code == 0`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"build": {ExitCode: 0},
					"test":  {ExitCode: 0},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "access undefined state errors - deep access",
			expr: `states.nonexistent.exit_code == 0`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{},
			},
			// expr library errors when trying to access property of nil
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExprEvaluator()
			got, err := e.Evaluate(tt.expr, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExprEvaluator_TypeCoercion(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		ctx     *interpolation.Context
		want    bool
		wantErr bool
	}{
		{
			name: "string to int coercion in comparison",
			expr: `inputs.port > 1000`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"port": "8080"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "float string to number",
			expr: `inputs.rate < 1.0`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"rate": "0.5"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "boolean string true",
			expr: `inputs.debug == true`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"debug": "true"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "boolean string false",
			expr: `inputs.debug == false`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"debug": "false"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "int64 comparison",
			expr: `inputs.timestamp > 1700000000`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"timestamp": int64(1700000001)},
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExprEvaluator()
			got, err := e.Evaluate(tt.expr, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExprEvaluator_ErrorMessages(t *testing.T) {
	tests := []struct {
		name       string
		expr       string
		ctx        *interpolation.Context
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "syntax error - missing operand",
			expr:       `inputs.a == `,
			ctx:        &interpolation.Context{Inputs: map[string]any{"a": 1}},
			wantErr:    true,
			wantErrMsg: "compile expression",
		},
		{
			name:    "undefined variable in context - returns false not error",
			expr:    `inputs.undefined == 1`,
			ctx:     &interpolation.Context{Inputs: map[string]any{}},
			wantErr: false, // expr library treats undefined as nil, comparison returns false
		},
		{
			name:       "invalid operator",
			expr:       `inputs.a === 1`,
			ctx:        &interpolation.Context{Inputs: map[string]any{"a": 1}},
			wantErr:    true,
			wantErrMsg: "", // Should error but message varies
		},
		{
			name:       "unclosed parenthesis",
			expr:       `(inputs.a == 1`,
			ctx:        &interpolation.Context{Inputs: map[string]any{"a": 1}},
			wantErr:    true,
			wantErrMsg: "", // Should error - syntax
		},
		{
			name:       "unclosed string",
			expr:       `inputs.a == "test`,
			ctx:        &interpolation.Context{Inputs: map[string]any{"a": "test"}},
			wantErr:    true,
			wantErrMsg: "", // Should error - syntax
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExprEvaluator()
			_, err := e.Evaluate(tt.expr, tt.ctx)

			if tt.wantErr {
				require.Error(t, err, "expected error for invalid expression")
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestExprEvaluator_WorkflowMetadata(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		ctx     *interpolation.Context
		want    bool
		wantErr bool
	}{
		{
			name: "check workflow id",
			expr: `workflow.id != ""`,
			ctx: &interpolation.Context{
				Workflow: interpolation.WorkflowData{ID: "wf-abc-123"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "check workflow name pattern",
			expr: `workflow.name == "deploy-prod"`,
			ctx: &interpolation.Context{
				Workflow: interpolation.WorkflowData{Name: "deploy-prod"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "check current state",
			expr: `workflow.current_state == "build"`,
			ctx: &interpolation.Context{
				Workflow: interpolation.WorkflowData{CurrentState: "build"},
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExprEvaluator()
			got, err := e.Evaluate(tt.expr, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExprEvaluator_InOperator(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		ctx     *interpolation.Context
		want    bool
		wantErr bool
	}{
		{
			name: "string in array",
			expr: `inputs.env in ["dev", "staging", "prod"]`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"env": "staging"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "string not in array",
			expr: `inputs.env in ["dev", "staging", "prod"]`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"env": "local"},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "int in array",
			expr: `inputs.code in [0, 1, 2]`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"code": 1},
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExprEvaluator()
			got, err := e.Evaluate(tt.expr, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
