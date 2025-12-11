# F015: Conditions Complexes (if/else)

## Metadata
- **Status**: implemented
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priority**: high
- **Estimation**: L

## Description

Add conditional branching based on expressions. Evaluate conditions using variable values, comparison operators, and logical operators. Enable dynamic workflow paths beyond simple success/failure transitions.

## Acceptance Criteria

- [x] `when:` clause on transitions
- [x] Compare variables with operators (==, !=, <, >, <=, >=)
- [x] Logical operators (and, or, not)
- [x] Access to all interpolation variables
- [x] String and numeric comparisons
- [x] Clear error on invalid expressions
- [x] Fallback to default transition if no condition matches

## Dependencies

- **Blocked by**: F007, F009
- **Unblocks**: F016

## Impacted Files

```
pkg/expression/evaluator.go
pkg/expression/evaluator_test.go
internal/domain/workflow/condition.go
internal/domain/workflow/state.go
internal/application/executor.go
```

## Technical Tasks

- [x] Define Condition struct
  - [x] Expression string
  - [x] Target state
- [x] Define expression syntax
  - [x] Variable references: `inputs.count`
  - [x] Comparisons: `==`, `!=`, `<`, `>`, `<=`, `>=`
  - [x] Logical: `and`, `or`, `not`
  - [x] Grouping: `(expr)`
- [x] Implement ExpressionEvaluator
  - [x] Parse expression
  - [x] Evaluate against context
  - [x] Return boolean result
- [x] Extend state transitions
  - [x] Support `transitions:` with conditions
  - [x] Evaluate in order, first match wins
  - [x] Default transition if none match
- [x] Handle type coercion
  - [x] String to number
  - [x] Empty string = false
- [x] Write parser tests
- [x] Write evaluator tests
- [x] Write integration tests

## Notes

Conditional transition syntax:
```yaml
process:
  type: step
  command: analyze.sh
  capture:
    stdout: result
  transitions:
    - when: "states.process.exit_code == 0 and inputs.mode == 'full'"
      goto: full_report
    - when: "states.process.exit_code == 0"
      goto: summary_report
    - goto: error  # default fallback
```

Consider using govaluate or expr library for expression parsing.
