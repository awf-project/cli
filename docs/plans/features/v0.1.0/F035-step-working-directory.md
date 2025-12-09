# F035: Step Working Directory

## Metadata
- **Status**: backlog
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: medium
- **Estimation**: S

## Description

Allow defining a working directory (`dir`) per step in a YAML workflow. Each step executes a new instance of `/bin/sh -c`, so a `cd` in a command does not affect subsequent steps. This feature allows explicitly specifying the execution directory.

## Acceptance Criteria

- [ ] A step can define `dir: "/path/to/dir"` in YAML
- [ ] The step command executes in this directory
- [ ] If `dir` is empty, current behavior is preserved (awf process CWD)
- [ ] The `dir` field supports interpolation (`{{inputs.project_path}}`)
- [ ] Clear error if directory does not exist

## Dependencies

- **Blocked by**: _none_
- **Unblocks**: _none_

## Impacted Files

```
internal/domain/workflow/step.go
internal/application/execution_service.go
```

## Technical Tasks

- [ ] Add `Dir string` to `Step` struct (after `Command`)
- [ ] Pass `Dir: step.Dir` to `ports.Command` in `execution_service.go`
- [ ] Write unit tests
- [ ] Update documentation (CLAUDE.md YAML syntax section)

## Notes

```yaml
# Usage example
steps:
  build:
    command: "make build"
    dir: "/tmp/myproject"
    on_success: test

  test:
    command: "go test ./..."
    dir: "{{inputs.project_path}}"
    on_success: done
```
