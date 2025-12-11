# F015: Conditions Complexes (if/else)

## Metadata
- **Status**: backlog
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priority**: high
- **Estimation**: L

## Description

Add conditional branching based on expressions. Evaluate conditions using variable values, comparison operators, and logical operators. Enable dynamic workflow paths beyond simple success/failure transitions.

## Acceptance Criteria

- [ ] `when:` clause on transitions
- [ ] Compare variables with operators (==, !=, <, >, <=, >=)
- [ ] Logical operators (and, or, not)
- [ ] Access to all interpolation variables
- [ ] String and numeric comparisons
- [ ] Clear error on invalid expressions
- [ ] Fallback to default transition if no condition matches

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

- [ ] Define Condition struct
  - [ ] Expression string
  - [ ] Target state
- [ ] Define expression syntax
  - [ ] Variable references: `inputs.count`
  - [ ] Comparisons: `==`, `!=`, `<`, `>`, `<=`, `>=`
  - [ ] Logical: `and`, `or`, `not`
  - [ ] Grouping: `(expr)`
- [ ] Implement ExpressionEvaluator
  - [ ] Parse expression
  - [ ] Evaluate against context
  - [ ] Return boolean result
- [ ] Extend state transitions
  - [ ] Support `transitions:` with conditions
  - [ ] Evaluate in order, first match wins
  - [ ] Default transition if none match
- [ ] Handle type coercion
  - [ ] String to number
  - [ ] Empty string = false
- [ ] Write parser tests
- [ ] Write evaluator tests
- [ ] Write integration tests

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
