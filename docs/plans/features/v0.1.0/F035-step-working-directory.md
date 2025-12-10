# F035: Step Working Directory

## Metadata
- **Status**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: medium
- **Estimation**: S

## Description

Allow defining a working directory (`dir`) per step in a YAML workflow. Each step executes a new instance of `/bin/sh -c`, so a `cd` in a command does not affect subsequent steps. This feature allows explicitly specifying the execution directory.

## Acceptance Criteria

- [x] A step can define `dir: "/path/to/dir"` in YAML
- [x] The step command executes in this directory
- [x] If `dir` is empty, current behavior is preserved (awf process CWD)
- [x] The `dir` field supports interpolation (`{{inputs.project_path}}`)
- [x] Clear error if directory does not exist (handled by OS/shell)

## Dependencies

- **Blocked by**: _none_
- **Unblocks**: _none_

## Impacted Files

```
internal/domain/workflow/step.go
internal/application/execution_service.go
```

## Technical Tasks

- [x] Add `Dir string` to `Step` struct (after `Command`)
- [x] Pass `Dir: step.Dir` to `ports.Command` in `execution_service.go`
- [x] Write unit tests
- [x] Update documentation (README.md YAML syntax section)

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
