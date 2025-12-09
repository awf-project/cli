# F036: CLI Init Command

## Metadata
- **Status**: backlog
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: medium
- **Estimation**: S

## Description

Implement the `awf init` command to initialize AWF configuration in the current directory. This command sets up the required directory structure and creates a default configuration file, making it easy to start using AWF in any project.

## Acceptance Criteria

- [ ] `awf init` creates `.awf.yaml` configuration file
- [ ] `awf init` creates `workflows/` directory for YAML definitions
- [ ] `awf init` creates `storage/` directory for states and logs
- [ ] Command is idempotent (safe to run multiple times)
- [ ] Existing files are NOT overwritten (prompt or skip)
- [ ] `awf init --force` overwrites existing configuration
- [ ] Creates a sample workflow file in `workflows/hello.yaml`
- [ ] Displays clear success message with next steps

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

- [ ] Create `init` command in Cobra
  - [ ] Register in root command
  - [ ] Add `--force` flag for overwriting
  - [ ] Add `--workflows-dir` flag (default: `workflows/`)
  - [ ] Add `--storage-dir` flag (default: `storage/`)
- [ ] Implement directory creation
  - [ ] Create workflows directory
  - [ ] Create storage directory with subdirs (states/, logs/)
  - [ ] Handle existing directories gracefully
- [ ] Implement configuration file generation
  - [ ] Create `.awf.yaml` with sensible defaults
  - [ ] Include comments explaining each option
  - [ ] Respect custom directory flags
- [ ] Create sample workflow
  - [ ] Generate `workflows/hello.yaml` with simple echo example
  - [ ] Include comments explaining workflow syntax
- [ ] Write unit tests
- [ ] Write integration tests

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
