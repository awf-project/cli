# F007: Interpolation de Variables

## Metadata
- **Status**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: high
- **Estimation**: M

## Description

Implement `{{variable}}` interpolation in workflow commands and strings. Resolve inputs, state outputs, workflow metadata, environment variables, and error context. Use Go template syntax to avoid shell variable conflicts.

## Acceptance Criteria

- [x] Interpolate `{{inputs.name}}` from workflow inputs
- [x] Interpolate `{{states.step.output}}` from previous steps
- [x] Interpolate `{{workflow.id}}`, `{{workflow.name}}`, etc.
- [x] Interpolate `{{env.VAR}}` from environment
- [x] Interpolate `{{error.message}}` in error hooks
- [x] Support escaping: `\{\{` for literal braces
- [x] Clear errors for undefined variables

## Dependencies

- **Blocked by**: F001, F002
- **Unblocks**: F003, F008

## Impacted Files

```
pkg/interpolation/resolver.go
pkg/interpolation/resolver_test.go
internal/domain/workflow/context.go
```

## Technical Tasks

- [ ] Define Resolver interface
  - [ ] Resolve(template string, context Context) (string, error)
- [ ] Implement TemplateResolver
  - [ ] Parse `{{...}}` patterns
  - [ ] Resolve variable paths (inputs.x, states.y.output)
  - [ ] Handle nested paths
  - [ ] Handle escaping
- [ ] Define variable namespaces
  - [ ] inputs: workflow input values
  - [ ] states: previous state outputs and metadata
  - [ ] workflow: id, name, version, duration, current_state, started_at
  - [ ] context: working_dir, user, hostname
  - [ ] env: environment variables
  - [ ] error: message, state, exit_code, type (in error hooks)
  - [ ] metadata: timestamp, file_size, etc.
- [ ] Integrate with ExecutionContext
- [ ] Write comprehensive unit tests
  - [ ] Valid variable resolution
  - [ ] Undefined variables
  - [ ] Nested paths
  - [ ] Escaping

## Notes

Use `{{}}` syntax (Go template) instead of `${}` to avoid shell conflicts:

```yaml
command: |
  echo "Input: {{inputs.file_path}}"
  echo "Shell var: ${HOME}"
  echo "Literal: \{\{ not interpolated \}\}"
```

Consider using `text/template` from stdlib for implementation.
