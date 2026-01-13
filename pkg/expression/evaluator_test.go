package expression

import (
	"testing"
	"time"

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
		{
			name: "not equal true",
			expr: `inputs.mode != "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "partial"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not equal false",
			expr: `inputs.mode != "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"mode": "full"},
			},
			want:    false,
			wantErr: false,
		},

		// Comparison operators - greater/less
		{
			name: "greater than true",
			expr: `inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "greater than false",
			expr: `inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 3},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "greater than or equal true (greater)",
			expr: `inputs.count >= 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "greater than or equal true (equal)",
			expr: `inputs.count >= 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 5},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "less than true",
			expr: `inputs.count < 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 3},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "less than false",
			expr: `inputs.count < 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "less than or equal true (less)",
			expr: `inputs.count <= 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 3},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "less than or equal true (equal)",
			expr: `inputs.count <= 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 5},
			},
			want:    true,
			wantErr: false,
		},

		// Logical operators
		{
			name: "logical AND both true",
			expr: `inputs.count > 5 && inputs.mode == "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{
					"count": 10,
					"mode":  "full",
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "logical AND first false",
			expr: `inputs.count > 5 && inputs.mode == "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{
					"count": 3,
					"mode":  "full",
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "logical AND second false",
			expr: `inputs.count > 5 && inputs.mode == "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{
					"count": 10,
					"mode":  "partial",
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "logical OR both true",
			expr: `inputs.count > 5 || inputs.mode == "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{
					"count": 10,
					"mode":  "full",
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "logical OR first true",
			expr: `inputs.count > 5 || inputs.mode == "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{
					"count": 10,
					"mode":  "partial",
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "logical OR second true",
			expr: `inputs.count > 5 || inputs.mode == "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{
					"count": 3,
					"mode":  "full",
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "logical OR both false",
			expr: `inputs.count > 5 || inputs.mode == "full"`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{
					"count": 3,
					"mode":  "partial",
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "logical NOT true",
			expr: `!(inputs.count > 5)`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 3},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "logical NOT false",
			expr: `!(inputs.count > 5)`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    false,
			wantErr: false,
		},

		// String operations
		{
			name: "string contains true",
			expr: `contains(inputs.message, "hello")`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"message": "hello world"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "string contains false",
			expr: `contains(inputs.message, "hello")`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"message": "goodbye world"},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "string has_prefix true",
			expr: `has_prefix(inputs.message, "hello")`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"message": "hello world"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "string has_prefix false",
			expr: `has_prefix(inputs.message, "hello")`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"message": "goodbye world"},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "string has_suffix true",
			expr: `has_suffix(inputs.message, "world")`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"message": "hello world"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "string has_suffix false",
			expr: `has_suffix(inputs.message, "world")`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"message": "hello universe"},
			},
			want:    false,
			wantErr: false,
		},

		// Arithmetic operations
		{
			name: "arithmetic addition",
			expr: `inputs.count + 5 == 15`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "arithmetic subtraction",
			expr: `inputs.count - 5 == 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "arithmetic multiplication",
			expr: `inputs.count * 2 == 20`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "arithmetic division",
			expr: `inputs.count / 2 == 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "arithmetic modulo",
			expr: `inputs.count % 3 == 1`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want:    true,
			wantErr: false,
		},

		// Type coercion
		{
			name: "string to number coercion",
			expr: `inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"count": "10"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "boolean string true",
			expr: `inputs.enabled == true`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"enabled": "true"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "boolean string false",
			expr: `inputs.enabled == false`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{"enabled": "false"},
			},
			want:    true,
			wantErr: false,
		},

		// State access (step results)
		{
			name: "state output equality",
			expr: `states.step1.Output == "success"`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"step1": {Output: "success"},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "state exit code",
			expr: `states.step1.ExitCode == 0`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"step1": {ExitCode: 0},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "state status",
			expr: `states.step1.Status == "completed"`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"step1": {Status: "completed"},
				},
			},
			want:    true,
			wantErr: false,
		},

		// Environment variables
		{
			name: "env variable access",
			expr: `env.HOME == "/home/user"`,
			ctx: &interpolation.Context{
				Env: map[string]string{"HOME": "/home/user"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "env variable contains",
			expr: `contains(env.PATH, "/usr/bin")`,
			ctx: &interpolation.Context{
				Env: map[string]string{"PATH": "/usr/bin:/usr/local/bin"},
			},
			want:    true,
			wantErr: false,
		},

		// Workflow metadata
		{
			name: "workflow ID",
			expr: `workflow.ID == "wf-123"`,
			ctx: &interpolation.Context{
				Workflow: interpolation.WorkflowData{ID: "wf-123"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "workflow name",
			expr: `workflow.Name == "my-workflow"`,
			ctx: &interpolation.Context{
				Workflow: interpolation.WorkflowData{Name: "my-workflow"},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "workflow current state",
			expr: `workflow.CurrentState == "step1"`,
			ctx: &interpolation.Context{
				Workflow: interpolation.WorkflowData{CurrentState: "step1"},
			},
			want:    true,
			wantErr: false,
		},

		// Complex nested conditions
		{
			name: "complex condition with multiple operators",
			expr: `(inputs.count > 5 && inputs.mode == "full") || states.step1.ExitCode != 0`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{
					"count": 10,
					"mode":  "full",
				},
				States: map[string]interpolation.StepStateData{
					"step1": {ExitCode: 0},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "complex condition with parentheses",
			expr: `inputs.count > 5 && (inputs.mode == "full" || inputs.mode == "partial")`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{
					"count": 10,
					"mode":  "partial",
				},
			},
			want:    true,
			wantErr: false,
		},

		// Edge cases and error conditions
		{
			name: "nil context",
			expr: `inputs.count > 5`,
			ctx:  nil,
			// Should not error, inputs should be empty map
			want:    false,
			wantErr: false,
		},
		{
			name: "empty inputs",
			expr: `inputs.count > 5`,
			ctx: &interpolation.Context{
				Inputs: map[string]any{},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "missing state",
			expr: `states.nonexistent.Output == "test"`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{},
			},
			want:    false,
			wantErr: false,
		},

		// Test  PascalCase normalization
		{
			name: "access state.Output field (PascalCase)",
			expr: `states.step1.Output == "success"`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"step1": {Output: "success"},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "access state.ExitCode field (PascalCase)",
			expr: `states.step1.ExitCode == 0`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"step1": {ExitCode: 0},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "access state.Status field (PascalCase)",
			expr: `states.step1.Status == "completed"`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"step1": {Status: "completed"},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "access state.Response field",
			expr: `states.agent.Response.result == "ok"`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"agent": {
						Response: map[string]any{"result": "ok"},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "access state.Tokens field",
			expr: `states.agent.Tokens > 50`,
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"agent": {
						Response: map[string]any{},
						Tokens:   150,
					},
				},
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

func TestCoerceValue(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  any
	}{
		{
			name:  "string number to int",
			value: "42",
			want:  int64(42),
		},
		{
			name:  "string float to float",
			value: "3.14",
			want:  3.14,
		},
		{
			name:  "string true to bool",
			value: "true",
			want:  true,
		},
		{
			name:  "string false to bool",
			value: "false",
			want:  false,
		},
		{
			name:  "string True (capitalized) to bool",
			value: "True",
			want:  true,
		},
		{
			name:  "string FALSE (uppercase) to bool",
			value: "FALSE",
			want:  false,
		},
		{
			name:  "non-numeric string unchanged",
			value: "hello",
			want:  "hello",
		},
		{
			name:  "int unchanged",
			value: 42,
			want:  42,
		},
		{
			name:  "float unchanged",
			value: 3.14,
			want:  3.14,
		},
		{
			name:  "bool unchanged",
			value: true,
			want:  true,
		},
		{
			name:  "nil unchanged",
			value: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coerceValue(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExprEvaluator_LoopContext(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *interpolation.Context
		expr    string
		want    bool
		wantErr bool
	}{
		{
			name: "loop.Index access",
			ctx: &interpolation.Context{
				Loop: &interpolation.LoopData{Index: 5, Length: 10},
			},
			expr:    "loop.Index == 5",
			want:    true,
			wantErr: false,
		},
		{
			name: "loop.Index1 access (1-based index)",
			ctx: &interpolation.Context{
				Loop: &interpolation.LoopData{Index: 5, Length: 10},
			},
			expr:    "loop.Index1 == 6",
			want:    true,
			wantErr: false,
		},
		{
			name: "loop.Item access",
			ctx: &interpolation.Context{
				Loop: &interpolation.LoopData{
					Item:   "test-item",
					Index:  0,
					Length: 5,
				},
			},
			expr:    `loop.Item == "test-item"`,
			want:    true,
			wantErr: false,
		},
		{
			name: "loop.First flag",
			ctx: &interpolation.Context{
				Loop: &interpolation.LoopData{
					Index:  0,
					Length: 5,
					First:  true,
				},
			},
			expr:    "loop.First == true",
			want:    true,
			wantErr: false,
		},
		{
			name: "loop.Last flag",
			ctx: &interpolation.Context{
				Loop: &interpolation.LoopData{
					Index:  4,
					Length: 5,
					Last:   true,
				},
			},
			expr:    "loop.Last == true",
			want:    true,
			wantErr: false,
		},
		{
			name: "loop.Length access",
			ctx: &interpolation.Context{
				Loop: &interpolation.LoopData{
					Index:  2,
					Length: 10,
				},
			},
			expr:    "loop.Length == 10",
			want:    true,
			wantErr: false,
		},
		{
			name: "loop.Parent access (nested loop)",
			ctx: &interpolation.Context{
				Loop: &interpolation.LoopData{
					Index:  1,
					Length: 3,
					Parent: &interpolation.LoopData{
						Index:  5,
						Length: 10,
					},
				},
			},
			expr:    "loop.Parent.Index == 5",
			want:    true,
			wantErr: false,
		},
		{
			name: "nil loop context",
			ctx: &interpolation.Context{
				Loop: nil,
			},
			expr:    "loop.Index == 0",
			want:    false,
			wantErr: true, // Should error because loop namespace doesn't exist
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

func TestExprEvaluator_ErrorContext(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *interpolation.Context
		expr    string
		want    bool
		wantErr bool
	}{
		{
			name: "error.Message access",
			ctx: &interpolation.Context{
				Error: &interpolation.ErrorData{
					Message:  "command failed",
					State:    "step1",
					ExitCode: 1,
					Type:     "execution",
				},
			},
			expr:    `error.Message == "command failed"`,
			want:    true,
			wantErr: false,
		},
		{
			name: "error.State access",
			ctx: &interpolation.Context{
				Error: &interpolation.ErrorData{
					Message:  "command failed",
					State:    "step1",
					ExitCode: 1,
					Type:     "execution",
				},
			},
			expr:    `error.State == "step1"`,
			want:    true,
			wantErr: false,
		},
		{
			name: "error.ExitCode access",
			ctx: &interpolation.Context{
				Error: &interpolation.ErrorData{
					Message:  "command failed",
					State:    "step1",
					ExitCode: 1,
					Type:     "execution",
				},
			},
			expr:    "error.ExitCode == 1",
			want:    true,
			wantErr: false,
		},
		{
			name: "error.Type access",
			ctx: &interpolation.Context{
				Error: &interpolation.ErrorData{
					Message:  "command failed",
					State:    "step1",
					ExitCode: 1,
					Type:     "execution",
				},
			},
			expr:    `error.Type == "execution"`,
			want:    true,
			wantErr: false,
		},
		{
			name: "nil error context",
			ctx: &interpolation.Context{
				Error: nil,
			},
			expr:    `error.Message == ""`,
			want:    false,
			wantErr: true, // Should error because error namespace doesn't exist
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

func TestExprEvaluator_SystemContext(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *interpolation.Context
		expr    string
		want    bool
		wantErr bool
	}{
		{
			name: "context.WorkingDir access",
			ctx: &interpolation.Context{
				Context: interpolation.ContextData{
					WorkingDir: "/home/user/project",
					User:       "testuser",
					Hostname:   "testhost",
				},
			},
			expr:    `context.WorkingDir == "/home/user/project"`,
			want:    true,
			wantErr: false,
		},
		{
			name: "context.User access",
			ctx: &interpolation.Context{
				Context: interpolation.ContextData{
					WorkingDir: "/home/user/project",
					User:       "testuser",
					Hostname:   "testhost",
				},
			},
			expr:    `context.User == "testuser"`,
			want:    true,
			wantErr: false,
		},
		{
			name: "context.Hostname access",
			ctx: &interpolation.Context{
				Context: interpolation.ContextData{
					WorkingDir: "/home/user/project",
					User:       "testuser",
					Hostname:   "testhost",
				},
			},
			expr:    `context.Hostname == "testhost"`,
			want:    true,
			wantErr: false,
		},
		{
			name: "empty system context",
			ctx: &interpolation.Context{
				Context: interpolation.ContextData{},
			},
			expr:    `context.WorkingDir == ""`,
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

func TestExprEvaluator_NewStepFields(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *interpolation.Context
		expr    string
		want    bool
		wantErr bool
	}{
		{
			name: "access state.Response field",
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"agent": {
						Response: map[string]any{
							"result": "ok",
							"data":   "test",
						},
						Tokens: 100,
					},
				},
			},
			expr:    `states.agent.Response.result == "ok"`,
			want:    true,
			wantErr: false,
		},
		{
			name: "access state.Tokens field",
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"agent": {
						Response: map[string]any{},
						Tokens:   150,
					},
				},
			},
			expr:    "states.agent.Tokens == 150",
			want:    true,
			wantErr: false,
		},
		{
			name: "compare state.Tokens with threshold",
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"agent": {
						Tokens: 150,
					},
				},
			},
			expr:    "states.agent.Tokens > 50",
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

func TestBuildExprContext_PascalCaseStateFields(t *testing.T) {
	tests := []struct {
		name      string
		ctx       *interpolation.Context
		checkFunc func(t *testing.T, result map[string]any)
	}{
		{
			name: "state fields use PascalCase keys",
			ctx: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"step1": {
						Output:   "test output",
						Stderr:   "test error",
						ExitCode: 1,
						Status:   "completed",
						Response: map[string]any{"key": "value"},
						Tokens:   100,
					},
				},
			},
			checkFunc: func(t *testing.T, result map[string]any) {
				states, ok := result["states"].(map[string]any)
				require.True(t, ok, "states should be a map")

				step1, ok := states["step1"].(map[string]any)
				require.True(t, ok, "step1 should exist and be a map")

				// Verify PascalCase keys exist
				assert.Contains(t, step1, "Output")
				assert.Contains(t, step1, "Stderr")
				assert.Contains(t, step1, "ExitCode")
				assert.Contains(t, step1, "Status")
				assert.Contains(t, step1, "Response")
				assert.Contains(t, step1, "Tokens")

				// Verify values
				assert.Equal(t, "test output", step1["Output"])
				assert.Equal(t, "test error", step1["Stderr"])
				assert.Equal(t, 1, step1["ExitCode"])
				assert.Equal(t, "completed", step1["Status"])
				assert.Equal(t, 100, step1["Tokens"])
			},
		},
		{
			name: "workflow fields use PascalCase keys",
			ctx: &interpolation.Context{
				Workflow: interpolation.WorkflowData{
					ID:           "wf-123",
					Name:         "test-workflow",
					CurrentState: "step1",
					StartedAt:    time.Now(),
				},
			},
			checkFunc: func(t *testing.T, result map[string]any) {
				workflow, ok := result["workflow"].(map[string]any)
				require.True(t, ok, "workflow should be a map")

				// Verify PascalCase keys exist
				assert.Contains(t, workflow, "ID")
				assert.Contains(t, workflow, "Name")
				assert.Contains(t, workflow, "CurrentState")
				assert.Contains(t, workflow, "Duration")

				// Verify values
				assert.Equal(t, "wf-123", workflow["ID"])
				assert.Equal(t, "test-workflow", workflow["Name"])
				assert.Equal(t, "step1", workflow["CurrentState"])
			},
		},
		{
			name: "loop context uses PascalCase keys",
			ctx: &interpolation.Context{
				Loop: &interpolation.LoopData{
					Index:  2,
					Item:   "test-item",
					Length: 5,
					First:  false,
					Last:   false,
				},
			},
			checkFunc: func(t *testing.T, result map[string]any) {
				loop, ok := result["loop"].(map[string]any)
				require.True(t, ok, "loop should be a map")

				// Verify PascalCase keys exist
				assert.Contains(t, loop, "Index")
				assert.Contains(t, loop, "Index1")
				assert.Contains(t, loop, "Item")
				assert.Contains(t, loop, "Length")
				assert.Contains(t, loop, "First")
				assert.Contains(t, loop, "Last")
				assert.Contains(t, loop, "Parent")

				// Verify values
				assert.Equal(t, 2, loop["Index"])
				assert.Equal(t, 3, loop["Index1"]) // Index1() returns Index + 1
				assert.Equal(t, "test-item", loop["Item"])
				assert.Equal(t, 5, loop["Length"])
				assert.Equal(t, false, loop["First"])
				assert.Equal(t, false, loop["Last"])
				assert.Nil(t, loop["Parent"])
			},
		},
		{
			name: "error context uses PascalCase keys",
			ctx: &interpolation.Context{
				Error: &interpolation.ErrorData{
					Message:  "test error",
					State:    "step1",
					ExitCode: 1,
					Type:     "execution",
				},
			},
			checkFunc: func(t *testing.T, result map[string]any) {
				errorData, ok := result["error"].(map[string]any)
				require.True(t, ok, "error should be a map")

				// Verify PascalCase keys exist
				assert.Contains(t, errorData, "Message")
				assert.Contains(t, errorData, "State")
				assert.Contains(t, errorData, "ExitCode")
				assert.Contains(t, errorData, "Type")

				// Verify values
				assert.Equal(t, "test error", errorData["Message"])
				assert.Equal(t, "step1", errorData["State"])
				assert.Equal(t, 1, errorData["ExitCode"])
				assert.Equal(t, "execution", errorData["Type"])
			},
		},
		{
			name: "system context uses PascalCase keys",
			ctx: &interpolation.Context{
				Context: interpolation.ContextData{
					WorkingDir: "/test/dir",
					User:       "testuser",
					Hostname:   "testhost",
				},
			},
			checkFunc: func(t *testing.T, result map[string]any) {
				contextData, ok := result["context"].(map[string]any)
				require.True(t, ok, "context should be a map")

				// Verify PascalCase keys exist
				assert.Contains(t, contextData, "WorkingDir")
				assert.Contains(t, contextData, "User")
				assert.Contains(t, contextData, "Hostname")

				// Verify values
				assert.Equal(t, "/test/dir", contextData["WorkingDir"])
				assert.Equal(t, "testuser", contextData["User"])
				assert.Equal(t, "testhost", contextData["Hostname"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildExprContext(tt.ctx)
			require.NotNil(t, result)
			tt.checkFunc(t, result)
		})
	}
}

func TestBuildExprContext_NilSafety(t *testing.T) {
	tests := []struct {
		name      string
		ctx       *interpolation.Context
		checkFunc func(t *testing.T, result map[string]any)
	}{
		{
			name: "nil loop context omitted",
			ctx: &interpolation.Context{
				Loop: nil,
			},
			checkFunc: func(t *testing.T, result map[string]any) {
				assert.NotContains(t, result, "loop", "loop should not be present when nil")
			},
		},
		{
			name: "nil error context omitted",
			ctx: &interpolation.Context{
				Error: nil,
			},
			checkFunc: func(t *testing.T, result map[string]any) {
				assert.NotContains(t, result, "error", "error should not be present when nil")
			},
		},
		{
			name: "system context always present",
			ctx: &interpolation.Context{
				Context: interpolation.ContextData{},
			},
			checkFunc: func(t *testing.T, result map[string]any) {
				assert.Contains(t, result, "context", "context should always be present")
				contextData, ok := result["context"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "", contextData["WorkingDir"])
				assert.Equal(t, "", contextData["User"])
				assert.Equal(t, "", contextData["Hostname"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildExprContext(tt.ctx)
			require.NotNil(t, result)
			tt.checkFunc(t, result)
		})
	}
}
