# F002: Parsing YAML des Workflows

## Metadata
- **Status**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: critical
- **Estimation**: M

## Description

Implement YAML workflow file parsing. Load workflow definitions from `configs/workflows/` directory, validate structure, and convert to domain entities. This is the primary input mechanism for AWF.

## Acceptance Criteria

- [x] Parse valid YAML workflow files without error
- [x] Return clear errors for invalid YAML syntax
- [x] Return clear errors for missing required fields
- [x] Map YAML structure to domain Workflow entity
- [x] Support all workflow fields defined in projectBrief.md
- [x] Implements WorkflowRepository port interface

## Dependencies

- **Blocked by**: F001
- **Unblocks**: F003, F005, F007

## Impacted Files

```
internal/infrastructure/repository/yaml_repository.go
internal/domain/workflow/workflow.go
internal/domain/workflow/step.go
internal/domain/workflow/state.go
internal/domain/workflow/hooks.go
internal/domain/workflow/input.go
internal/domain/ports/repository.go
configs/workflows/examples/
```

## Technical Tasks

- [ ] Define complete Workflow struct with YAML tags
  - [ ] Metadata (name, description, version, author, tags)
  - [ ] Inputs definition
  - [ ] States map
  - [ ] Hooks (workflow_start, workflow_end, workflow_error, workflow_cancel)
  - [ ] Persistence config
  - [ ] Timeouts config
  - [ ] Logging config
- [ ] Define Step struct with YAML tags
  - [ ] Type (step, parallel, terminal)
  - [ ] Operation, command
  - [ ] Capture config
  - [ ] Timeout, retry config
  - [ ] on_success, on_failure transitions
  - [ ] Hooks (pre, post)
- [ ] Implement YAMLRepository
  - [ ] Load(name) method
  - [ ] List() method
  - [ ] FindByPath(path) method
- [ ] Add validation for required fields
- [ ] Create example workflow files
- [ ] Write unit tests

## Notes

Use `gopkg.in/yaml.v3` for parsing. Define custom UnmarshalYAML for complex types if needed.

Example workflow structure:
```yaml
name: example
states:
  initial: start
  start:
    type: step
    command: echo "hello"
    on_success: end
  end:
    type: terminal
    status: success
```
