# Implementation Plan: F015 - Conditional Steps

## Summary

Implement conditional branching using `when:` clauses on step transitions. Expressions evaluate against the existing interpolation context (inputs, states, env) using the [expr-lang/expr](https://github.com/expr-lang/expr) library. This allows dynamic workflow paths beyond simple success/failure, with first-match-wins semantics and a fallback default transition.

```
┌───────────────────────────────────────────────────────────────────┐
│  YAML WORKFLOW                                                    │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │ process:                                                    │  │
│  │   type: step                                                │  │
│  │   command: analyze.sh                                       │  │
│  │   transitions:                                              │  │
│  │     - when: "states.process.exit_code == 0 and             │  │
│  │              inputs.mode == 'full'"                         │  │
│  │       goto: full_report                                     │  │
│  │     - when: "states.process.exit_code == 0"                │  │
│  │       goto: summary_report                                  │  │
│  │     - goto: error  # default fallback                      │  │
│  └─────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────────┐
│  EXPRESSION EVALUATOR (pkg/expression)                            │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │ type Evaluator interface {                                  │  │
│  │   Evaluate(expr string, ctx *Context) (bool, error)         │  │
│  │ }                                                           │  │
│  │                                                             │  │
│  │ Context = map[string]any {                                  │  │
│  │   "inputs":   {"mode": "full", "count": 10}                │  │
│  │   "states":   {"process": {"exit_code": 0, "output": ""}}  │  │
│  │   "env":      {"HOME": "/home/user"}                       │  │
│  │   "workflow": {"id": "...", "name": "..."}                 │  │
│  │ }                                                           │  │
│  └─────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────────┐
│  EXECUTION SERVICE                                                │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │ for _, transition := range step.Transitions {               │  │
│  │   if transition.When == "" {  // default fallback           │  │
│  │     return transition.Goto, nil                             │  │
│  │   }                                                         │  │
│  │   result, err := evaluator.Evaluate(transition.When, ctx)   │  │
│  │   if err != nil { return "", err }                          │  │
│  │   if result {                                               │  │
│  │     return transition.Goto, nil                             │  │
│  │   }                                                         │  │
│  │ }                                                           │  │
│  │ // fallback to on_success/on_failure for backward compat    │  │
│  └─────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### 1. Add Expression Evaluator Package
   - **File:** `pkg/expression/evaluator.go`
   - **Action:** CREATE
   - **Changes:**
     - Define `Evaluator` interface with `Evaluate(expr string, ctx map[string]any) (bool, error)`
     - Implement `ExprEvaluator` using `expr-lang/expr`
     - Build context map from `interpolation.Context` with flattened structure for dot access
     - Handle type coercion (string→number, empty→false)
     - Return clear errors for invalid expressions

### 2. Add Expression Evaluator Tests
   - **File:** `pkg/expression/evaluator_test.go`
   - **Action:** CREATE
   - **Changes:**
     - Table-driven tests for comparison operators (`==`, `!=`, `<`, `>`, `<=`, `>=`)
     - Tests for logical operators (`and`, `or`, `not`)
     - Tests for parentheses grouping
     - Tests for variable access (`inputs.x`, `states.step.exit_code`)
     - Tests for type coercion edge cases
     - Error cases for invalid expressions

### 3. Define Transition Domain Entity
   - **File:** `internal/domain/workflow/condition.go`
   - **Action:** CREATE
   - **Changes:**
     - Define `Transition` struct: `When string`, `Goto string`
     - Define `Transitions` type as `[]Transition`
     - Add `EvaluateTransitions()` method that takes evaluator and context

### 4. Extend Step Entity
   - **File:** `internal/domain/workflow/step.go`
   - **Action:** MODIFY
   - **Changes:**
     - Add `Transitions []Transition` field to `Step` struct
     - Update `Validate()` to validate transition `Goto` targets are not empty

### 5. Extend YAML Types
   - **File:** `internal/infrastructure/repository/yaml_types.go`
   - **Action:** MODIFY
   - **Changes:**
     - Add `yamlTransition` struct with `When string`, `Goto string`
     - Add `Transitions []yamlTransition` field to `yamlStep`

### 6. Extend YAML Mapper
   - **File:** `internal/infrastructure/repository/yaml_mapper.go`
   - **Action:** MODIFY
   - **Changes:**
     - Add `mapTransitions()` function to convert `[]yamlTransition` to `[]workflow.Transition`
     - Update `mapStep()` to map transitions field

### 7. Extend Workflow Validation
   - **File:** `internal/domain/workflow/validation.go`
   - **Action:** MODIFY
   - **Changes:**
     - Validate transition `Goto` references exist in workflow steps
     - Validate at least one transition has no `When` (fallback) OR `OnSuccess`/`OnFailure` exists

### 8. Update Execution Service
   - **File:** `internal/application/execution_service.go`
   - **Action:** MODIFY
   - **Changes:**
     - Add `ExprEvaluator` dependency to `ExecutionService`
     - Update `NewExecutionService()` to accept evaluator
     - Create `resolveNextStep()` method:
       1. If `step.Transitions` exists, evaluate in order, return first match
       2. If no condition matches and no fallback, fall through to `OnSuccess`/`OnFailure`
       3. Return error if no transition matches
     - Update `executeStep()` to call `resolveNextStep()` instead of hardcoded `step.OnSuccess`/`step.OnFailure`

### 9. Add Expression Context Builder
   - **File:** `internal/application/execution_service.go`
   - **Action:** MODIFY
   - **Changes:**
     - Add `buildExpressionContext()` method to convert `interpolation.Context` to `map[string]any`
     - Flatten nested structures for expr access: `states.process.exit_code`

### 10. Update CLI Dependency Injection
   - **File:** `internal/interfaces/cli/commands/run.go`
   - **Action:** MODIFY
   - **Changes:**
     - Create and inject `ExprEvaluator` when building `ExecutionService`

### 11. Integration Tests
   - **File:** `tests/integration/conditions_test.go`
   - **Action:** CREATE
   - **Changes:**
     - Test workflow with conditional transitions
     - Test fallback to default transition
     - Test backward compatibility (workflows with only `on_success`/`on_failure`)

### 12. Add Dependency
   - **File:** `go.mod`
   - **Action:** MODIFY
   - **Changes:**
     - Add `github.com/expr-lang/expr` dependency

## Test Plan

### Unit Tests
- `pkg/expression/evaluator_test.go`:
  - Comparison operators with strings, integers, floats
  - Logical operators (and, or, not)
  - Parentheses precedence
  - Variable access paths
  - Type coercion
  - Error cases (undefined variables, invalid syntax)
- `internal/domain/workflow/condition_test.go`:
  - Transition evaluation order (first match wins)
  - Default fallback handling
  - Validation errors

### Integration Tests
- `tests/integration/conditions_test.go`:
  - Full workflow execution with conditional branching
  - Multiple conditions evaluated in order
  - Fallback to default
  - Mixed mode (transitions + on_success/on_failure)
  - Error handling for invalid expressions

## Files to Modify

| File | Action | Complexity |
|------|--------|------------|
| `pkg/expression/evaluator.go` | CREATE | M |
| `pkg/expression/evaluator_test.go` | CREATE | M |
| `internal/domain/workflow/condition.go` | CREATE | S |
| `internal/domain/workflow/step.go` | MODIFY | S |
| `internal/domain/workflow/validation.go` | MODIFY | S |
| `internal/infrastructure/repository/yaml_types.go` | MODIFY | S |
| `internal/infrastructure/repository/yaml_mapper.go` | MODIFY | S |
| `internal/application/execution_service.go` | MODIFY | M |
| `internal/interfaces/cli/commands/run.go` | MODIFY | S |
| `tests/integration/conditions_test.go` | CREATE | M |
| `go.mod` | MODIFY | S |

## Risks

| Risk | Mitigation |
|------|------------|
| **Backward compatibility**: Existing workflows use `on_success`/`on_failure` | Treat `transitions` as optional extension; if empty, fall back to `on_success`/`on_failure` |
| **Expression security**: Arbitrary code execution | Use `expr-lang/expr` which is memory-safe and side-effect-free by design |
| **Performance**: Expression parsing on every step | Consider caching compiled expressions if profiling shows bottleneck |
| **Type coercion edge cases**: `"10" == 10` behavior | Document expected behavior, default to strict comparison, use explicit coercion functions if needed |
| **Complex debugging**: Users don't know why condition failed | Log evaluated values at debug level when condition evaluates to false |

---

Sources:
- [expr-lang/expr](https://github.com/expr-lang/expr) - Expression evaluation library
- [Knetic/govaluate](https://github.com/Knetic/govaluate) - Alternative expression library
- [PaesslerAG/gval](https://github.com/PaesslerAG/gval) - Go-like expression evaluation

