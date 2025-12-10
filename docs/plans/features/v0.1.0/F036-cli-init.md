# F036: CLI Init Command

## Metadata
- **Status**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: medium
- **Estimation**: S

## Description

Implement the `awf init` command to initialize AWF configuration in the current directory. This command sets up the required directory structure and creates a default configuration file, making it easy to start using AWF in any project.

## Acceptance Criteria

- [x] `awf init` creates `.awf.yaml` configuration file
- [x] `awf init` creates `.awf/workflows/` directory for YAML definitions
- [x] `awf init` creates `.awf/storage/` directory for states and logs
- [x] Command is idempotent (safe to run multiple times)
- [x] Existing files are NOT overwritten (skip if exists)
- [x] `awf init --force` overwrites existing configuration
- [x] Creates a sample workflow file in `.awf/workflows/example.yaml`
- [x] Displays clear success message with next steps

## Dependencies

- **Blocked by**: F005
- **Unblocks**: _none_

## Impacted Files

```
internal/interfaces/cli/init.go
internal/interfaces/cli/root.go
configs/templates/awf.yaml.tmpl
configs/templates/hello.yaml.tmpl
tests/integration/init_test.go
```

## Technical Tasks

- [x] Create `init` command in Cobra
  - [x] Register in root command
  - [x] Add `--force` flag for overwriting
- [x] Implement directory creation
  - [x] Create `.awf/workflows/` directory
  - [x] Create `.awf/storage/` directory with subdirs (states/, logs/)
  - [x] Handle existing directories gracefully
- [x] Implement configuration file generation
  - [x] Create `.awf.yaml` with sensible defaults
  - [x] Include comments explaining each option
- [x] Create sample workflow
  - [x] Generate `.awf/workflows/example.yaml` with simple echo example
- [x] Write unit tests

## Notes

Default `.awf.yaml` structure:

```yaml
# AWF Configuration
version: "1"

# Directory for workflow definitions
workflows_dir: workflows

# Directory for runtime data (states, logs)
storage_dir: storage

# Default log level: debug, info, warn, error
log_level: info

# Output format: text, json
output_format: text
```

Sample `hello.yaml` workflow:

```yaml
name: hello
description: A simple hello world workflow
version: "1.0.0"

inputs:
  name:
    type: string
    default: World

steps:
  - id: greet
    type: step
    operation: shell
    command: echo "Hello, {{inputs.name}}!"
```
