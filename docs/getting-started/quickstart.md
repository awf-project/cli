# Quick Start

Get AWF running in 5 minutes.

## 1. Initialize AWF

```bash
awf init
```

This creates the following structure:

```
.awf.yaml              # Configuration file
.awf/
├── workflows/
│   └── example.yaml   # Sample workflow
└── prompts/           # Prompt templates
```

## 2. Run the Example Workflow

```bash
awf run example
```

Output:
```
Hello from AWF!
Workflow completed successfully
```

## 3. Create Your First Workflow

Create a new file `.awf/workflows/hello.yaml`:

```yaml
name: hello
version: "1.0.0"
description: A simple hello world workflow

states:
  initial: greet
  greet:
    type: step
    command: echo "Hello, {{.inputs.name}}!"
    on_success: done
  done:
    type: terminal
```

Run it with an input:

```bash
awf run hello --input name=World
```

Output:
```
Hello, World!
Workflow completed successfully
```

## 4. Interactive Input Collection

If you run a workflow with missing required inputs, AWF will automatically prompt you for them (when running from a terminal):

```bash
awf run hello
# Output:
# name (string, required):
```

Just enter a value and press Enter. AWF will execute the workflow with your input.

**Note:** Interactive prompting only works in terminal sessions. In scripts or piped contexts, you must provide inputs via `--input` flags.

## 6. Validate Your Workflow

Check your workflow for errors before running:

```bash
awf validate hello
```

## 7. List Available Workflows

See all workflows AWF can find:

```bash
awf list
```

## Workflow Discovery

AWF discovers workflows from multiple locations (priority high to low):

1. `AWF_WORKFLOWS_PATH` environment variable
2. `./.awf/workflows/` (local project)
3. `$XDG_CONFIG_HOME/awf/workflows/` (global, default: `~/.config/awf/workflows/`)

Local workflows override global ones with the same name.

## Prompt Discovery

AWF discovers prompts from multiple locations (priority high to low):

1. `./.awf/prompts/` (local project)
2. `$XDG_CONFIG_HOME/awf/prompts/` (global, default: `~/.config/awf/prompts/`)

Local prompts override global ones with the same name.

```bash
# List all prompts
awf list prompts

# Initialize global prompts directory
awf init --global
```

### Using Prompts

Reference prompts in workflow inputs using the `@prompts/` prefix:

```bash
awf run my-workflow --input prompt=@prompts/system.md
```

The `@prompts/` prefix loads the file content and passes it as the input value.

## Runtime Data Storage

AWF stores runtime data following the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/):

| Data | Default Location |
|------|------------------|
| States | `~/.local/share/awf/states/` |
| History | `~/.local/share/awf/history.db` |
| Plugins | `~/.local/share/awf/plugins/` |

These paths can be customized via `XDG_DATA_HOME` environment variable.

## Common Flags

| Flag | Description |
|------|-------------|
| `--input, -i` | Pass input values (key=value) |
| `--output, -o` | Output mode: silent, streaming, buffered |
| `--verbose, -v` | Enable verbose output |
| `--quiet, -q` | Suppress non-error output |
| `--dry-run` | Show execution plan without running |
| `--interactive` | Step-by-step execution with prompts |

## Example: Code Analysis

```yaml
name: analyze-code
version: "1.0.0"

inputs:
  - name: file
    type: string
    required: true

states:
  initial: read_file
  read_file:
    type: step
    command: cat "{{.inputs.file}}"
    on_success: analyze
    on_failure: error
  analyze:
    type: step
    command: |
      claude -c "Review this code:
      {{.states.read_file.output}}"
    timeout: 120
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
    status: failure
```

```bash
awf run analyze-code --input file=main.go
```

## Next Steps

- [Commands](../user-guide/commands.md) - Full command reference
- [Workflow Syntax](../user-guide/workflow-syntax.md) - Complete YAML syntax
- [Examples](../user-guide/examples.md) - More workflow examples
