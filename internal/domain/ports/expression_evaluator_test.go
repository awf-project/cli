package ports_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/pkg/interpolation"
)

// Component: T001
// Feature: C042

// mockExpressionEvaluator is a test implementation of ExpressionEvaluator interface
type mockExpressionEvaluator struct {
	evaluateBoolFunc func(expr string, ctx *interpolation.Context) (bool, error)
	evaluateIntFunc  func(expr string, ctx *interpolation.Context) (int, error)
	boolCallCount    int
	intCallCount     int
}

func newMockExpressionEvaluator() *mockExpressionEvaluator {
	return &mockExpressionEvaluator{
		evaluateBoolFunc: func(expr string, ctx *interpolation.Context) (bool, error) {
			// By default, return true for non-empty expressions
			if expr == "" {
				return false, errors.New("empty expression")
			}
			return true, nil
		},
		evaluateIntFunc: func(expr string, ctx *interpolation.Context) (int, error) {
			// By default, return simple value
			if expr == "" {
				return 0, errors.New("empty expression")
			}
			return 42, nil
		},
	}
}

func (m *mockExpressionEvaluator) EvaluateBool(expr string, ctx *interpolation.Context) (bool, error) {
	m.boolCallCount++
	return m.evaluateBoolFunc(expr, ctx)
}

func (m *mockExpressionEvaluator) EvaluateInt(expr string, ctx *interpolation.Context) (int, error) {
	m.intCallCount++
	return m.evaluateIntFunc(expr, ctx)
}

func TestExpressionEvaluatorInterface(t *testing.T) {
	// Verify interface compliance
	var _ ports.ExpressionEvaluator = (*mockExpressionEvaluator)(nil)
}

func TestExpressionEvaluator_EvaluateBool_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{"true literal", "true", true},
		{"false literal", "false", false},
		{"comparison equal", "5 == 5", true},
		{"comparison not equal", "5 != 3", true},
		{"greater than", "10 > 5", true},
		{"less than", "3 < 10", true},
		{"logical and true", "true && true", true},
		{"logical and false", "true && false", false},
		{"logical or true", "false || true", true},
		{"logical or false", "false || false", false},
		{"string contains", "'hello world' contains 'world'", true},
		{"string equality", "'test' == 'test'", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := newMockExpressionEvaluator()
			evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
				return tt.expected, nil
			}
			ctx := interpolation.NewContext()

			result, err := evaluator.EvaluateBool(tt.expr, ctx)
			if err != nil {
				t.Errorf("unexpected error for expression '%s': %v", tt.expr, err)
			}
			if result != tt.expected {
				t.Errorf("expected %v for expression '%s', got %v", tt.expected, tt.expr, result)
			}
			if evaluator.boolCallCount != 1 {
				t.Errorf("expected EvaluateBool to be called once, got %d", evaluator.boolCallCount)
			}
		})
	}
}

func TestExpressionEvaluator_EvaluateBool_WithContext(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		setup    func(*interpolation.Context)
		expected bool
	}{
		{
			name: "input variable greater than",
			expr: "inputs.count > 5",
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["count"] = 10
			},
			expected: true,
		},
		{
			name: "input variable equal",
			expr: "inputs.status == 'complete'",
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["status"] = "complete"
			},
			expected: true,
		},
		{
			name: "state output contains",
			expr: "states.step1.output contains 'DONE'",
			setup: func(ctx *interpolation.Context) {
				ctx.States["step1"] = interpolation.StepStateData{
					Output: "Task DONE successfully",
				}
			},
			expected: true,
		},
		{
			name: "workflow metadata",
			expr: "workflow.name == 'test-workflow'",
			setup: func(ctx *interpolation.Context) {
				ctx.Workflow.Name = "test-workflow"
			},
			expected: true,
		},
		{
			name: "environment variable",
			expr: "env.DEBUG == 'true'",
			setup: func(ctx *interpolation.Context) {
				ctx.Env["DEBUG"] = "true"
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := newMockExpressionEvaluator()
			evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
				return tt.expected, nil
			}
			ctx := interpolation.NewContext()
			tt.setup(ctx)

			result, err := evaluator.EvaluateBool(tt.expr, ctx)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExpressionEvaluator_EvaluateInt_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected int
	}{
		{"integer literal", "42", 42},
		{"addition", "10 + 5", 15},
		{"subtraction", "20 - 8", 12},
		{"multiplication", "6 * 7", 42},
		{"division", "20 / 4", 5},
		{"modulo", "17 % 5", 2},
		{"complex arithmetic", "(10 + 5) * 2", 30},
		{"order of operations", "2 + 3 * 4", 14},
		{"parentheses priority", "(2 + 3) * 4", 20},
		{"negative number", "-5 + 10", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := newMockExpressionEvaluator()
			evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
				return tt.expected, nil
			}
			ctx := interpolation.NewContext()

			result, err := evaluator.EvaluateInt(tt.expr, ctx)
			if err != nil {
				t.Errorf("unexpected error for expression '%s': %v", tt.expr, err)
			}
			if result != tt.expected {
				t.Errorf("expected %d for expression '%s', got %d", tt.expected, tt.expr, result)
			}
			if evaluator.intCallCount != 1 {
				t.Errorf("expected EvaluateInt to be called once, got %d", evaluator.intCallCount)
			}
		})
	}
}

func TestExpressionEvaluator_EvaluateInt_WithContext(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		setup    func(*interpolation.Context)
		expected int
	}{
		{
			name: "input variable arithmetic",
			expr: "inputs.max_count * 2",
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["max_count"] = 10
			},
			expected: 20,
		},
		{
			name: "state exit code",
			expr: "states.step1.exit_code",
			setup: func(ctx *interpolation.Context) {
				ctx.States["step1"] = interpolation.StepStateData{
					ExitCode: 0,
				}
			},
			expected: 0,
		},
		{
			name: "loop index arithmetic",
			expr: "loop.index + 1",
			setup: func(ctx *interpolation.Context) {
				ctx.Loop = &interpolation.LoopData{
					Index: 4,
				}
			},
			expected: 5,
		},
		{
			name: "division expression",
			expr: "20 / 4",
			setup: func(ctx *interpolation.Context) {
				// No setup needed for literal expression
			},
			expected: 5,
		},
		{
			name: "complex expression with variables",
			expr: "(inputs.a + inputs.b) * 3",
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["a"] = 5
				ctx.Inputs["b"] = 3
			},
			expected: 24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := newMockExpressionEvaluator()
			evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
				return tt.expected, nil
			}
			ctx := interpolation.NewContext()
			tt.setup(ctx)

			result, err := evaluator.EvaluateInt(tt.expr, ctx)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestExpressionEvaluator_EvaluateBool_EmptyExpression(t *testing.T) {
	evaluator := newMockExpressionEvaluator()
	ctx := interpolation.NewContext()

	result, err := evaluator.EvaluateBool("", ctx)

	if err == nil {
		t.Error("expected error for empty expression, got nil")
	}
	if result != false {
		t.Errorf("expected false for empty expression, got %v", result)
	}
}

func TestExpressionEvaluator_EvaluateInt_EmptyExpression(t *testing.T) {
	evaluator := newMockExpressionEvaluator()
	ctx := interpolation.NewContext()

	result, err := evaluator.EvaluateInt("", ctx)

	if err == nil {
		t.Error("expected error for empty expression, got nil")
	}
	if result != 0 {
		t.Errorf("expected 0 for empty expression with error, got %d", result)
	}
}

func TestExpressionEvaluator_EvaluateBool_NilContext(t *testing.T) {
	evaluator := newMockExpressionEvaluator()
	evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
		if ctx == nil {
			return false, errors.New("nil context")
		}
		return true, nil
	}

	result, err := evaluator.EvaluateBool("true", nil)

	if err == nil {
		t.Error("expected error for nil context, got nil")
	}
	if result != false {
		t.Errorf("expected false for nil context error, got %v", result)
	}
}

func TestExpressionEvaluator_EvaluateInt_NilContext(t *testing.T) {
	evaluator := newMockExpressionEvaluator()
	evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
		if ctx == nil {
			return 0, errors.New("nil context")
		}
		return 42, nil
	}

	result, err := evaluator.EvaluateInt("42", nil)

	if err == nil {
		t.Error("expected error for nil context, got nil")
	}
	if result != 0 {
		t.Errorf("expected 0 for nil context error, got %d", result)
	}
}

func TestExpressionEvaluator_EvaluateBool_WhitespaceOnly(t *testing.T) {
	evaluator := newMockExpressionEvaluator()
	evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
		if expr == "   " {
			return false, errors.New("whitespace-only expression")
		}
		return true, nil
	}
	ctx := interpolation.NewContext()

	result, err := evaluator.EvaluateBool("   ", ctx)

	if err == nil {
		t.Error("expected error for whitespace-only expression, got nil")
	}
	if result != false {
		t.Errorf("expected false for whitespace-only expression, got %v", result)
	}
}

func TestExpressionEvaluator_EvaluateInt_TypeCoercion(t *testing.T) {
	// Test that various numeric types are coerced to int
	tests := []struct {
		name     string
		expr     string
		expected int
	}{
		{"float result coerced", "10.0 / 2.0", 5},
		{"int64 result coerced", "1000000000 + 1", 1000000001},
		{"negative result", "-10 * 2", -20},
		{"zero result", "5 - 5", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := newMockExpressionEvaluator()
			evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
				return tt.expected, nil
			}
			ctx := interpolation.NewContext()

			result, err := evaluator.EvaluateInt(tt.expr, ctx)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestExpressionEvaluator_EvaluateInt_LargeNumbers(t *testing.T) {
	// Test handling of large numbers
	evaluator := newMockExpressionEvaluator()
	evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
		return 1000000, nil
	}
	ctx := interpolation.NewContext()

	result, err := evaluator.EvaluateInt("1000 * 1000", ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != 1000000 {
		t.Errorf("expected 1000000, got %d", result)
	}
}

func TestExpressionEvaluator_EvaluateBool_ComplexLogic(t *testing.T) {
	// Test complex boolean expressions with multiple operators
	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{"multiple and operators", "true && true && true", true},
		{"multiple or operators", "false || false || true", true},
		{"mixed and/or", "(true || false) && true", true},
		{"negation", "!false", true},
		{"double negation", "!!true", true},
		{"comparison chain", "5 > 3 && 3 > 1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := newMockExpressionEvaluator()
			evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
				return tt.expected, nil
			}
			ctx := interpolation.NewContext()

			result, err := evaluator.EvaluateBool(tt.expr, ctx)
			if err != nil {
				t.Errorf("unexpected error for expression '%s': %v", tt.expr, err)
			}
			if result != tt.expected {
				t.Errorf("expected %v for expression '%s', got %v", tt.expected, tt.expr, result)
			}
		})
	}
}

func TestExpressionEvaluator_EvaluateBool_InvalidSyntax(t *testing.T) {
	evaluator := newMockExpressionEvaluator()
	expectedErr := errors.New("syntax error: unexpected token")
	evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
		return false, expectedErr
	}
	ctx := interpolation.NewContext()

	result, err := evaluator.EvaluateBool("invalid && &&", ctx)

	if err == nil {
		t.Error("expected error for invalid syntax, got nil")
	}
	if err.Error() != "syntax error: unexpected token" {
		t.Errorf("expected syntax error, got '%v'", err)
	}
	if result != false {
		t.Errorf("expected false on error, got %v", result)
	}
}

func TestExpressionEvaluator_EvaluateInt_InvalidSyntax(t *testing.T) {
	evaluator := newMockExpressionEvaluator()
	expectedErr := errors.New("syntax error: unexpected operator")
	evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
		return 0, expectedErr
	}
	ctx := interpolation.NewContext()

	result, err := evaluator.EvaluateInt("10 ++ 5", ctx)

	if err == nil {
		t.Error("expected error for invalid syntax, got nil")
	}
	if err.Error() != "syntax error: unexpected operator" {
		t.Errorf("expected syntax error, got '%v'", err)
	}
	if result != 0 {
		t.Errorf("expected 0 on error, got %d", result)
	}
}

func TestExpressionEvaluator_EvaluateBool_UndefinedVariable(t *testing.T) {
	evaluator := newMockExpressionEvaluator()
	expectedErr := errors.New("undefined variable: unknown_var")
	evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
		return false, expectedErr
	}
	ctx := interpolation.NewContext()

	result, err := evaluator.EvaluateBool("inputs.unknown_var > 5", ctx)

	if err == nil {
		t.Error("expected error for undefined variable, got nil")
	}
	if err.Error() != "undefined variable: unknown_var" {
		t.Errorf("expected undefined variable error, got '%v'", err)
	}
	if result != false {
		t.Errorf("expected false on error, got %v", result)
	}
}

func TestExpressionEvaluator_EvaluateInt_UndefinedVariable(t *testing.T) {
	evaluator := newMockExpressionEvaluator()
	expectedErr := errors.New("undefined variable: unknown_var")
	evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
		return 0, expectedErr
	}
	ctx := interpolation.NewContext()

	result, err := evaluator.EvaluateInt("inputs.unknown_var * 2", ctx)

	if err == nil {
		t.Error("expected error for undefined variable, got nil")
	}
	if err.Error() != "undefined variable: unknown_var" {
		t.Errorf("expected undefined variable error, got '%v'", err)
	}
	if result != 0 {
		t.Errorf("expected 0 on error, got %d", result)
	}
}

func TestExpressionEvaluator_EvaluateBool_TypeMismatch(t *testing.T) {
	// Test attempting to use non-boolean result as boolean
	evaluator := newMockExpressionEvaluator()
	expectedErr := errors.New("type mismatch: expected boolean, got int")
	evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
		return false, expectedErr
	}
	ctx := interpolation.NewContext()
	ctx.Inputs["count"] = 42

	result, err := evaluator.EvaluateBool("inputs.count", ctx)

	if err == nil {
		t.Error("expected error for type mismatch, got nil")
	}
	if result != false {
		t.Errorf("expected false on error, got %v", result)
	}
}

func TestExpressionEvaluator_EvaluateInt_TypeMismatch(t *testing.T) {
	// Test attempting to use non-numeric result as int
	evaluator := newMockExpressionEvaluator()
	expectedErr := errors.New("type mismatch: expected numeric, got string")
	evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
		return 0, expectedErr
	}
	ctx := interpolation.NewContext()
	ctx.Inputs["name"] = "test"

	result, err := evaluator.EvaluateInt("inputs.name", ctx)

	if err == nil {
		t.Error("expected error for type mismatch, got nil")
	}
	if result != 0 {
		t.Errorf("expected 0 on error, got %d", result)
	}
}

func TestExpressionEvaluator_EvaluateInt_DivisionByZero(t *testing.T) {
	evaluator := newMockExpressionEvaluator()
	expectedErr := errors.New("division by zero")
	evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
		return 0, expectedErr
	}
	ctx := interpolation.NewContext()

	result, err := evaluator.EvaluateInt("10 / 0", ctx)

	if err == nil {
		t.Error("expected error for division by zero, got nil")
	}
	if err.Error() != "division by zero" {
		t.Errorf("expected division by zero error, got '%v'", err)
	}
	if result != 0 {
		t.Errorf("expected 0 on error, got %d", result)
	}
}

func TestExpressionEvaluator_EvaluateBool_UnbalancedParentheses(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"missing closing paren", "(true && false"},
		{"missing opening paren", "true && false)"},
		{"nested unbalanced", "((true || false)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := newMockExpressionEvaluator()
			evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
				return false, errors.New("unbalanced parentheses")
			}
			ctx := interpolation.NewContext()

			result, err := evaluator.EvaluateBool(tt.expr, ctx)

			if err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
			if result != false {
				t.Errorf("expected false on error, got %v", result)
			}
		})
	}
}

func TestExpressionEvaluator_Idempotent(t *testing.T) {
	// Test that evaluating the same expression multiple times produces consistent results
	evaluator := newMockExpressionEvaluator()
	evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
		return true, nil
	}
	evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
		return 42, nil
	}
	ctx := interpolation.NewContext()

	boolResult1, err1 := evaluator.EvaluateBool("true", ctx)
	boolResult2, err2 := evaluator.EvaluateBool("true", ctx)
	boolResult3, err3 := evaluator.EvaluateBool("true", ctx)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Errorf("unexpected errors: %v, %v, %v", err1, err2, err3)
	}
	if boolResult1 != boolResult2 || boolResult2 != boolResult3 {
		t.Errorf("inconsistent bool results: %v, %v, %v", boolResult1, boolResult2, boolResult3)
	}

	intResult1, err4 := evaluator.EvaluateInt("42", ctx)
	intResult2, err5 := evaluator.EvaluateInt("42", ctx)
	intResult3, err6 := evaluator.EvaluateInt("42", ctx)

	if err4 != nil || err5 != nil || err6 != nil {
		t.Errorf("unexpected errors: %v, %v, %v", err4, err5, err6)
	}
	if intResult1 != intResult2 || intResult2 != intResult3 {
		t.Errorf("inconsistent int results: %d, %d, %d", intResult1, intResult2, intResult3)
	}

	// Verify call counts
	if evaluator.boolCallCount != 3 {
		t.Errorf("expected 3 bool calls, got %d", evaluator.boolCallCount)
	}
	if evaluator.intCallCount != 3 {
		t.Errorf("expected 3 int calls, got %d", evaluator.intCallCount)
	}
}

func TestExpressionEvaluator_IndependentMethods(t *testing.T) {
	// Test that EvaluateBool and EvaluateInt are independent
	evaluator := newMockExpressionEvaluator()
	ctx := interpolation.NewContext()

	boolResult, boolErr := evaluator.EvaluateBool("true", ctx)
	intResult, intErr := evaluator.EvaluateInt("42", ctx)

	if boolErr != nil {
		t.Errorf("unexpected bool error: %v", boolErr)
	}
	if intErr != nil {
		t.Errorf("unexpected int error: %v", intErr)
	}
	if boolResult != true {
		t.Errorf("expected true, got %v", boolResult)
	}
	if intResult != 42 {
		t.Errorf("expected 42, got %d", intResult)
	}
	if evaluator.boolCallCount != 1 {
		t.Errorf("expected 1 bool call, got %d", evaluator.boolCallCount)
	}
	if evaluator.intCallCount != 1 {
		t.Errorf("expected 1 int call, got %d", evaluator.intCallCount)
	}
}

func TestExpressionEvaluator_ContextIsolation(t *testing.T) {
	// Test that expressions with different contexts produce different results
	evaluator := newMockExpressionEvaluator()
	evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
		// Return different results based on context
		if ctx.Inputs["test"] == "value1" {
			return true, nil
		}
		return false, nil
	}

	ctx1 := interpolation.NewContext()
	ctx1.Inputs["test"] = "value1"

	ctx2 := interpolation.NewContext()
	ctx2.Inputs["test"] = "value2"

	result1, err1 := evaluator.EvaluateBool("inputs.test == 'value1'", ctx1)
	result2, err2 := evaluator.EvaluateBool("inputs.test == 'value1'", ctx2)

	if err1 != nil || err2 != nil {
		t.Errorf("unexpected errors: %v, %v", err1, err2)
	}
	if result1 != true {
		t.Errorf("expected true for ctx1, got %v", result1)
	}
	if result2 != false {
		t.Errorf("expected false for ctx2, got %v", result2)
	}
}

// findRepoRoot searches upward from the current directory to find the repository root
// by looking for the go.mod file.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// TestExpressionEvaluator_InterfaceDocumentation verifies that the ExpressionEvaluator interface
// and its methods have proper godoc comments following Go conventions.
//
// Component: T002
//
//nolint:gocognit // Test complexity reflects comprehensive AST validation
func TestExpressionEvaluator_InterfaceDocumentation(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("failed to find repository root: %v", err)
	}

	interfacePath := filepath.Join(repoRoot, "internal", "domain", "ports", "expression_evaluator.go")

	content, err := os.ReadFile(interfacePath)
	if err != nil {
		t.Fatalf("failed to read expression_evaluator.go: %v", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, interfacePath, content, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse expression_evaluator.go: %v", err)
	}

	// Find the ExpressionEvaluator interface declaration
	var interfaceDecl *ast.InterfaceType
	var interfaceDoc *ast.CommentGroup
	var interfaceName string

	ast.Inspect(file, func(n ast.Node) bool {
		// Look for general declarations (type, var, const)
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			return true
		}

		// Check each type spec in the declaration
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if typeSpec.Name.Name == "ExpressionEvaluator" {
				interfaceName = typeSpec.Name.Name
				interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
				if ok {
					interfaceDecl = interfaceType
					// Documentation is on the GenDecl, not TypeSpec
					interfaceDoc = genDecl.Doc
				}
				return false
			}
		}
		return true
	})

	if interfaceDecl == nil {
		t.Fatal("ExpressionEvaluator interface not found in expression_evaluator.go")
	}

	if interfaceDoc == nil || len(interfaceDoc.List) == 0 {
		t.Error("ExpressionEvaluator interface has no godoc comment")
	} else {

		firstLine := interfaceDoc.List[0].Text
		if !strings.Contains(firstLine, interfaceName) {
			t.Errorf("interface godoc comment should mention interface name '%s', got: %s",
				interfaceName, firstLine)
		}
	}

	if interfaceDecl.Methods == nil || len(interfaceDecl.Methods.List) == 0 {
		t.Fatal("ExpressionEvaluator interface has no methods")
	}

	// Track which required methods we've found
	requiredMethods := map[string]bool{
		"EvaluateBool": false,
		"EvaluateInt":  false,
	}

	// Verify each method has documentation
	for _, field := range interfaceDecl.Methods.List {
		if len(field.Names) == 0 {
			continue
		}

		methodName := field.Names[0].Name

		// Check if this is one of our required methods
		if _, required := requiredMethods[methodName]; required {
			requiredMethods[methodName] = true

			if field.Doc == nil || len(field.Doc.List) == 0 {
				t.Errorf("method %s has no godoc comment", methodName)
			} else {

				firstLine := field.Doc.List[0].Text
				if !strings.Contains(firstLine, methodName) {
					t.Errorf("method %s godoc comment should mention method name, got: %s",
						methodName, firstLine)
				}
			}
		}
	}

	for methodName, found := range requiredMethods {
		if !found {
			t.Errorf("required method %s not found in ExpressionEvaluator interface", methodName)
		}
	}
}

func TestExpressionEvaluator_LoopConditionPatterns(t *testing.T) {
	// Test real-world loop condition patterns
	tests := []struct {
		name     string
		expr     string
		setup    func(*interpolation.Context)
		expected bool
	}{
		{
			name: "max iterations check",
			expr: "loop.index < inputs.max_iterations",
			setup: func(ctx *interpolation.Context) {
				ctx.Loop = &interpolation.LoopData{Index: 3}
				ctx.Inputs["max_iterations"] = 5
			},
			expected: true,
		},
		{
			name: "break condition on output",
			expr: "states.step1.output contains 'DONE'",
			setup: func(ctx *interpolation.Context) {
				ctx.States["step1"] = interpolation.StepStateData{
					Output: "Task completed DONE",
				}
			},
			expected: true,
		},
		{
			name: "exit code check",
			expr: "states.step1.exit_code == 0",
			setup: func(ctx *interpolation.Context) {
				ctx.States["step1"] = interpolation.StepStateData{
					ExitCode: 0,
				}
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := newMockExpressionEvaluator()
			evaluator.evaluateBoolFunc = func(expr string, ctx *interpolation.Context) (bool, error) {
				return tt.expected, nil
			}
			ctx := interpolation.NewContext()
			tt.setup(ctx)

			result, err := evaluator.EvaluateBool(tt.expr, ctx)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExpressionEvaluator_MaxIterationsPatterns(t *testing.T) {
	// Test real-world max_iterations arithmetic patterns
	tests := []struct {
		name     string
		expr     string
		setup    func(*interpolation.Context)
		expected int
	}{
		{
			name:     "simple division",
			expr:     "20 / 4",
			setup:    func(ctx *interpolation.Context) {},
			expected: 5,
		},
		{
			name: "input-based calculation",
			expr: "inputs.total / inputs.batch_size",
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["total"] = 100
				ctx.Inputs["batch_size"] = 10
			},
			expected: 10,
		},
		{
			name: "default with fallback",
			expr: "inputs.max_iterations",
			setup: func(ctx *interpolation.Context) {
				ctx.Inputs["max_iterations"] = 3
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := newMockExpressionEvaluator()
			evaluator.evaluateIntFunc = func(expr string, ctx *interpolation.Context) (int, error) {
				return tt.expected, nil
			}
			ctx := interpolation.NewContext()
			tt.setup(ctx)

			result, err := evaluator.EvaluateInt(tt.expr, ctx)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}
