# F041: Validate Template Interpolation References

## Metadata
- **Status**: planned
- **Phase**: 2-Enhanced
- **Version**: v0.2.0
- **Priority**: medium
- **Estimation**: M

## Description

Currently, workflow validation does not check whether template interpolation references (e.g., `{{inputs.name}}`, `{{states.step_name.output}}`) point to valid targets. A workflow can pass validation but fail at runtime when a referenced input doesn't exist or a step output is accessed before the step runs.

This feature adds static validation of template references during `awf validate` to catch these errors early, before workflow execution begins.

## Acceptance Criteria

- [ ] Validation detects references to undefined inputs (`{{inputs.undefined_var}}`)
- [ ] Validation detects references to non-existent steps (`{{states.missing_step.output}}`)
- [ ] Validation detects forward references (step A references step B's output, but B runs after A)
- [ ] Validation reports all template errors in a single pass (not fail-fast)
- [ ] Error messages include the step name and exact template reference that failed
- [ ] Valid template references pass validation without false positives
- [ ] Environment variable references (`{{env.VAR}}`) are allowed without validation (runtime-resolved)
- [ ] Workflow-level variables (`{{workflow.id}}`, etc.) are validated against known set

## Dependencies

- **Blocked by**: F010 (parallel execution - needs graph structure for dependency analysis)
- **Unblocks**: _none_

## Impacted Files

```
pkg/interpolation/...
internal/domain/workflow/...
internal/application/...
```

## Technical Tasks

- [ ] Extract template references from strings
  - [ ] Parse `{{...}}` patterns
  - [ ] Categorize by type (inputs, states, env, workflow, error)
- [ ] Build reference validator
  - [ ] Validate input references against workflow inputs
  - [ ] Validate state references against defined steps
  - [ ] Validate workflow references against known properties
- [ ] Integrate with execution order
  - [ ] Use workflow graph to detect forward references
  - [ ] Handle parallel step references correctly
- [ ] Aggregate validation errors
  - [ ] Collect all errors before returning
  - [ ] Format errors with context (step, field, reference)
- [ ] Write unit tests
- [ ] Write integration tests
- [ ] Update documentation

## Notes

- Template syntax uses Go template style `{{var}}` per CLAUDE.md
- Error hooks have access to `{{error.type}}` and `{{error.message}}` - these should only be validated in error hook contexts
- Consider whether to warn or error on unused inputs
- Parallel steps can reference each other only if strategy allows partial success (`best_effort`, `any_succeed`)
