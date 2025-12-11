# F042: Loop Context Variables

## Metadata
- **Status**: implemented
- **Phase**: 3-Enhanced
- **Version**: v0.3.0
- **Priority**: medium
- **Estimation**: S

## Description

When executing loop steps, context variables like `loop.index`, `loop.item`, `loop.first`, and `loop.last` should be available for use in templates and conditional expressions within the loop body. Currently, loop execution works but these contextual variables are not exposed, limiting the ability to customize behavior based on iteration position or current item.

This feature enables patterns like:
- Conditional logic based on first/last iteration
- Index-based naming or numbering
- Accessing the current item value in templates

## Acceptance Criteria

- [x] `{{loop.index}}` returns the current 0-based iteration index
- [x] `{{loop.index1}}` returns the current 1-based iteration index
- [x] `{{loop.item}}` returns the current item value being iterated
- [x] `{{loop.first}}` returns true on the first iteration
- [x] `{{loop.last}}` returns true on the last iteration
- [x] `{{loop.length}}` returns the total number of items in the loop
- [x] Loop context variables work in step command templates
- [x] Loop context variables work in conditional expressions (`when` field)
- [ ] Loop context variables are scoped to their loop (nested loops have separate contexts) - see F043
- [x] Loop context is cleared after loop completion
- [x] Accessing loop variables outside a loop returns empty/false values (no error)

## Dependencies

- **Blocked by**: F016 (Loop execution support - completed)
- **Unblocks**: _none_

## Impacted Files

```
internal/domain/workflow/context.go
internal/domain/workflow/loop.go
internal/application/execution_service.go
pkg/interpolation/interpolator.go
pkg/interpolation/expression.go
```

## Technical Tasks

- [x] Define loop context structure in domain layer
- [x] Implement loop context injection during iteration
- [x] Update interpolation engine to resolve loop.* variables
- [x] Update expression evaluator to handle loop.* in conditions
- [ ] Handle nested loop context scoping (see F043)
- [x] Write unit tests for loop context resolution
- [ ] Write integration tests for nested loop scenarios (see F043)
- [x] Write integration tests with real workflow files
- [x] Update documentation with loop variable examples

## Notes

- Consider whether `loop.parent` is needed for nested loops (defer to future if complex)
- Loop context should follow same naming as Jinja2/Twig for familiarity
- Must not break existing loop workflows that don't use these variables
