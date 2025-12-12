---
name: awf
description: |
  AWF (AI Workflow CLI) - Go CLI for orchestrating AI agents via YAML workflows.
  Use when: (1) Creating workflows, (2) Understanding AWF syntax,
  (3) Debugging workflow issues, (4) Using AWF CLI commands,
  (5) Developing features for AWF project.
---

# AWF - AI Workflow CLI

## Workflow Decision Tree

**Creating a workflow?**
1. `awf init` to initialize project
2. Create YAML file in `.awf/workflows/`
3. See [Workflow Syntax](references/workflow-syntax.md)

**Running a workflow?**
1. `awf run <name> --input key=value`
2. Use `--dry-run` to preview
3. Use `--interactive` for step-by-step
4. See [CLI Commands](references/cli-commands.md)

**Debugging issues?**
1. `awf validate <name>` to check syntax
2. Run with `--verbose` for details
3. Check `storage/logs/` for logs

**Developing AWF?**
1. See [Architecture](references/architecture.md)
2. Follow hexagonal architecture
3. Domain layer has no dependencies

## Quick Start

```yaml
# .awf/workflows/hello.yaml
name: hello
version: "1.0.0"

inputs:
  - name: name
    type: string
    default: World

states:
  initial: greet

  greet:
    type: step
    command: echo "Hello, {{.inputs.name}}!"
    on_success: done

  done:
    type: terminal
    status: success
```

```bash
awf run hello --input name=Claude
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `awf init` | Initialize AWF in directory |
| `awf run <workflow>` | Execute workflow |
| `awf validate <workflow>` | Check syntax |
| `awf list` | List workflows |
| `awf resume` | Resume interrupted |
| `awf history` | Show history |

**Details**: [CLI Commands Reference](references/cli-commands.md)

## State Types

| Type | Use |
|------|-----|
| `step` | Execute command |
| `parallel` | Run concurrent steps |
| `terminal` | End workflow |
| `for_each` | Iterate over list |
| `while` | Repeat until false |

**Details**: [Workflow Syntax Reference](references/workflow-syntax.md)

## Variable Interpolation

```yaml
# Inputs
command: echo "{{.inputs.file}}"

# Previous outputs
command: echo "{{.states.prev.output}}"

# Environment
command: echo "{{.env.HOME}}"

# Loop context
command: echo "{{.loop.index1}}/{{.loop.length}}"
```

## Common Patterns

### Retry with Backoff

```yaml
api_call:
  type: step
  command: curl -f https://api.example.com
  retry:
    max_attempts: 3
    backoff: exponential
    initial_delay: 1s
  on_success: process
  on_failure: error
```

### Parallel Execution

```yaml
build_all:
  type: parallel
  strategy: all_succeed
  max_concurrent: 3
  steps:
    - name: lint
      command: make lint
    - name: test
      command: make test
  on_success: deploy
```

### Conditional Branching

```yaml
check:
  type: step
  command: ./check.sh
  transitions:
    - when: "states.check.exit_code == 0 and inputs.env == 'prod'"
      goto: deploy_prod
    - when: "states.check.exit_code == 0"
      goto: deploy_staging
    - goto: error
```

## Resources

- **[references/workflow-syntax.md](references/workflow-syntax.md)** - Complete YAML syntax
- **[references/cli-commands.md](references/cli-commands.md)** - All CLI commands and flags
- **[references/architecture.md](references/architecture.md)** - Hexagonal architecture details
