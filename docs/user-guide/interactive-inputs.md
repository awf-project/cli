# Interactive Input Collection

AWF automatically prompts for missing required workflow inputs when running in a terminal environment, eliminating the need to remember all parameters upfront.

## How It Works

### Automatic Detection

When you run a workflow, AWF:

1. Checks which inputs are required by the workflow
2. Compares with inputs provided via `--input` flags
3. If required inputs are missing:
   - **Terminal environment** (stdin is a TTY): Prompts interactively for each missing input
   - **Non-terminal environment** (pipes, scripts, CI/CD): Returns error with clear message

### Terminal Detection

AWF only prompts when stdin is connected to a terminal:

```bash
# Interactive prompt (terminal)
awf run deploy
# ✓ Prompts for missing inputs

# Non-interactive error (piped)
echo "" | awf run deploy
# ✗ Error: missing required inputs and stdin is not a terminal

# Non-interactive error (script without TTY)
./deploy-script.sh  # calls awf run deploy
# ✗ Error: provide inputs via --input flags
```

## Required Inputs

### Basic Required Input

When a required input is missing, AWF displays the input name, type, and description:

**Workflow definition:**
```yaml
name: deploy
inputs:
  - name: environment
    type: string
    required: true
    description: Target environment for deployment
```

**Execution:**
```bash
$ awf run deploy

Enter value for 'environment' (string, required)
Description: Target environment for deployment
> _
```

### Type Validation

AWF validates input types and provides immediate feedback:

**Integer input:**
```bash
Enter value for 'count' (integer, required)
Description: Number of instances to deploy
> abc
Error: invalid integer value "abc"

Enter value for 'count' (integer, required)
> 42
✓ Accepted
```

**Boolean input:**
```bash
Enter value for 'dry_run' (boolean, required)
Description: Run in dry-run mode
> yes
Error: invalid boolean value "yes" (use: true, false, 1, 0, t, f)

Enter value for 'dry_run' (boolean, required)
> true
✓ Accepted
```

## Enum Inputs

### Numbered Selection

For inputs with enum constraints (≤9 options), AWF displays a numbered list:

**Workflow definition:**
```yaml
inputs:
  - name: environment
    type: string
    required: true
    description: Deployment environment
    validation:
      enum: [dev, staging, prod]
```

**Execution:**
```bash
$ awf run deploy

Enter value for 'environment' (string, required)
Description: Deployment environment
Options:
  1. dev
  2. staging
  3. prod
Select option (1-3): 2
✓ Selected: staging
```

### Invalid Selection Retry

Invalid selections trigger validation errors with retry:

```bash
Select option (1-3): 5
Error: invalid selection "5" (valid: 1-3)

Select option (1-3): 0
Error: invalid selection "0" (valid: 1-3)

Select option (1-3): 2
✓ Selected: staging
```

### Large Enum Lists

For enums with >9 options, AWF falls back to freetext with validation:

```yaml
validation:
  enum: [opt1, opt2, opt3, opt4, opt5, opt6, opt7, opt8, opt9, opt10]
```

```bash
Enter value for 'option' (string, required)
Valid values: opt1, opt2, opt3, opt4, opt5, opt6, opt7, opt8, opt9, opt10
> opt11
Error: value "opt11" not in allowed values

Enter value for 'option' (string, required)
> opt5
✓ Accepted
```

## Optional Inputs

### Skipping Optional Inputs

Press Enter without providing a value to skip optional inputs:

**Workflow definition:**
```yaml
inputs:
  - name: timeout
    type: integer
    required: false
    description: Timeout in seconds
```

**Execution:**
```bash
Enter value for 'timeout' (integer, optional)
Description: Timeout in seconds
Press Enter to skip
>
✓ Skipped (no value provided)
```

### Default Values

Optional inputs with default values show the default in the prompt:

**Workflow definition:**
```yaml
inputs:
  - name: timeout
    type: integer
    required: false
    default: 300
    description: Timeout in seconds
```

**Execution:**
```bash
Enter value for 'timeout' (integer, optional, default: 300)
Description: Timeout in seconds
Press Enter to use default
>
✓ Using default: 300
```

**Or provide a custom value:**
```bash
> 600
✓ Accepted: 600
```

## Pattern Validation

### Regex Patterns

Inputs with pattern validation are checked against the regex:

**Workflow definition:**
```yaml
inputs:
  - name: version
    type: string
    required: true
    description: Semantic version tag
    validation:
      pattern: '^v\d+\.\d+\.\d+$'
```

**Execution:**
```bash
Enter value for 'version' (string, required)
Description: Semantic version tag
Pattern: ^v\d+\.\d+\.\d+$
> 1.2.3
Error: value "1.2.3" does not match pattern "^v\d+\.\d+\.\d+$"

Enter value for 'version' (string, required)
> v1.2.3
✓ Accepted
```

## Cancellation

### Ctrl+C

Press `Ctrl+C` at any time to cancel input collection:

```bash
Enter value for 'environment' (string, required)
> ^C
Error: input collection cancelled by user
exit code 1
```

### Ctrl+D (EOF)

Sending EOF (Ctrl+D on Unix, Ctrl+Z on Windows) cancels input:

```bash
Enter value for 'environment' (string, required)
> ^D
Error: input cancelled
exit code 1
```

## Non-Interactive Execution

### Providing All Inputs Upfront

To skip interactive prompts, provide all required inputs via flags:

```bash
awf run deploy --input environment=prod --input version=v1.2.3
```

### Scripts and Automation

In non-terminal environments, AWF returns a clear error:

**Shell script:**
```bash
#!/bin/bash
# This will fail if inputs are missing
awf run deploy
```

**Error:**
```
Error: missing required inputs and stdin is not a terminal; provide inputs via --input flags
exit code 1
```

**Fixed script:**
```bash
#!/bin/bash
awf run deploy --input environment=prod --input version=v1.2.3
```

### CI/CD Pipelines

Always provide inputs explicitly in CI/CD:

```yaml
# GitHub Actions
- name: Deploy
  run: awf run deploy --input environment=prod --input version=${{ github.ref_name }}
```

## Configuration File Integration

Interactive input collection automatically merges values from `.awf/config.yaml`, reducing re-prompting for pre-configured inputs.

### How Config Values are Used

When running a workflow with interactive input collection:

1. **Config values are loaded** from `.awf/config.yaml` under the `inputs:` key
2. **Config values are pre-filled** for required inputs that are defined there
3. **Only missing inputs are prompted** — inputs already in config or CLI flags are skipped
4. **CLI flags take priority** over config values (if provided, CLI wins)

### Example with Config

**Workflow definition:**
```yaml
name: deploy
inputs:
  - name: api_key
    type: string
    required: true
    description: API key for authentication
  - name: environment
    type: string
    required: true
    description: Target environment
```

**Config file (`.awf/config.yaml`):**
```yaml
inputs:
  api_key: "sk-test-123"  # Pre-configured
```

**Interactive execution:**
```bash
$ awf run deploy

# api_key is NOT prompted (already in config)
# Only environment is prompted
Enter value for 'environment' (string, required)
Description: Target environment
> prod
✓ Accepted

Deploying...
✓ Workflow completed successfully
```

**CLI override:**
```bash
$ awf run deploy --input api_key=sk-cli-456

# api_key from CLI flag (overrides config)
# Only environment is prompted
Enter value for 'environment' (string, required)
> prod
✓ Accepted
```

## Mixed Inputs

### Combining Flags and Prompts

You can provide some inputs via flags and be prompted for the rest:

**Workflow definition:**
```yaml
inputs:
  - name: environment
    type: string
    required: true
  - name: version
    type: string
    required: true
  - name: dry_run
    type: boolean
    required: false
    default: false
```

**Execution:**
```bash
$ awf run deploy --input environment=prod

# Only prompts for missing 'version' (dry_run is optional with default)
Enter value for 'version' (string, required)
> v1.2.3
✓ Accepted

# Executes with: environment=prod, version=v1.2.3, dry_run=false
```

## Examples

### Deploy Workflow

**Workflow:**
```yaml
name: deploy
description: Deploy application to specified environment

inputs:
  - name: environment
    type: string
    required: true
    description: Target environment
    validation:
      enum: [dev, staging, prod]

  - name: version
    type: string
    required: true
    description: Version tag to deploy
    validation:
      pattern: '^v\d+\.\d+\.\d+$'

  - name: dry_run
    type: boolean
    required: false
    default: false
    description: Perform dry-run without applying changes

states:
  initial: deploy
  deploy:
    type: step
    command: ./deploy.sh {{.inputs.environment}} {{.inputs.version}} {{.inputs.dry_run}}
    on_success: done
  done:
    type: terminal
```

**Interactive execution:**
```bash
$ awf run deploy

Enter value for 'environment' (string, required)
Description: Target environment
Options:
  1. dev
  2. staging
  3. prod
Select option (1-3): 3
✓ Selected: prod

Enter value for 'version' (string, required)
Description: Version tag to deploy
Pattern: ^v\d+\.\d+\.\d+$
> v2.1.0
✓ Accepted

Enter value for 'dry_run' (boolean, optional, default: false)
Description: Perform dry-run without applying changes
Press Enter to use default
>
✓ Using default: false

Deploying...
✓ Workflow completed successfully
```

### Configuration Workflow

**Workflow:**
```yaml
name: configure
description: Configure application settings

inputs:
  - name: log_level
    type: string
    required: true
    description: Logging level
    validation:
      enum: [debug, info, warn, error]

  - name: max_connections
    type: integer
    required: false
    default: 100
    description: Maximum concurrent connections

states:
  initial: config
  config:
    type: step
    command: ./configure.sh --log-level={{.inputs.log_level}} --max-conn={{.inputs.max_connections}}
    on_success: done
  done:
    type: terminal
```

**Partial input execution:**
```bash
$ awf run configure --input max_connections=500

# Only prompts for log_level (max_connections provided via flag)
Enter value for 'log_level' (string, required)
Description: Logging level
Options:
  1. debug
  2. info
  3. warn
  4. error
Select option (1-4): 2
✓ Selected: info

Configuring...
✓ Workflow completed successfully
```

## Best Practices

### 1. Always Provide Descriptions

Help users understand what each input is for:

```yaml
inputs:
  - name: api_key
    type: string
    required: true
    description: API key for authentication (get from dashboard)
```

### 2. Use Enums for Constrained Values

Make it easy to select from known options:

```yaml
inputs:
  - name: region
    type: string
    required: true
    validation:
      enum: [us-east-1, us-west-2, eu-west-1]
```

### 3. Provide Sensible Defaults

Reduce friction for common use cases:

```yaml
inputs:
  - name: timeout
    type: integer
    required: false
    default: 300
    description: Request timeout in seconds
```

### 4. Use Patterns for Format Validation

Catch errors early with regex patterns:

```yaml
inputs:
  - name: email
    type: string
    required: true
    validation:
      pattern: '^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
```

### 5. Document Non-Interactive Usage

Help users understand how to run workflows in scripts:

```yaml
name: deploy
description: |
  Deploy application to environment.

  Non-interactive usage:
    awf run deploy --input env=prod --input version=v1.0.0
```

## Troubleshooting

### "stdin is not a terminal" Error

**Problem:** Running AWF in a non-terminal context without providing all required inputs.

**Solutions:**
1. Provide all inputs via `--input` flags
2. Run from a terminal (not piped or scripted without TTY)
3. Use `awf run <workflow> --help` to see required inputs

### Validation Keeps Failing

**Problem:** Input doesn't match validation rules.

**Solutions:**
1. Read the error message carefully
2. Check the pattern/enum constraint in the prompt
3. Use `awf validate <workflow>` to inspect input definitions
4. Check workflow YAML for validation rules

### Can't Skip Optional Input

**Problem:** Prompt keeps asking for value even when pressing Enter.

**Check:**
- Is the input truly optional (`required: false`)?
- Does the workflow definition have a default value?
- Try providing a value that matches the type and validation

### Prompt Not Appearing

**Problem:** AWF doesn't prompt for missing inputs.

**Check:**
1. Are you running in a terminal? (not piped, not in CI)
2. Did you provide all required inputs via `--input` flags?
3. Is stdin redirected? (`awf run deploy < /dev/null` won't prompt)

## See Also

- [Commands Reference](commands.md) - All AWF CLI commands
- [Project Configuration](configuration.md) - Config file setup and input pre-population
- [Workflow Syntax](workflow-syntax.md) - Input definitions and validation
- [Input Validation Reference](../reference/validation.md) - Validation rules reference
