# F007: Interpolation de Variables

## Metadata
- **Statut**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: high
- **Estimation**: M

## Description

Implement `{{variable}}` interpolation in workflow commands and strings. Resolve inputs, state outputs, workflow metadata, environment variables, and error context. Use Go template syntax to avoid shell variable conflicts.

## Critères d'Acceptance

- [ ] Interpolate `{{inputs.name}}` from workflow inputs
- [ ] Interpolate `{{states.step.output}}` from previous steps
- [ ] Interpolate `{{workflow.id}}`, `{{workflow.name}}`, etc.
- [ ] Interpolate `{{env.VAR}}` from environment
- [ ] Interpolate `{{error.message}}` in error hooks
- [ ] Support escaping: `\{\{` for literal braces
- [ ] Clear errors for undefined variables

## Dépendances

- **Bloqué par**: F001, F002
- **Débloque**: F003, F008

## Fichiers Impactés

```
pkg/interpolation/resolver.go
pkg/interpolation/resolver_test.go
internal/domain/workflow/context.go
```

## Tâches Techniques

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
