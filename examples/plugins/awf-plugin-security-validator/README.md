# AWF Security Validator Plugin

An example security validation plugin for AWF that checks workflows for common security issues.

## Features

The Security Validator Plugin performs the following security checks:

### Hardcoded Secrets Detection
- Detects patterns that look like hardcoded API keys, passwords, tokens, and secrets
- Reports findings as errors to prevent accidental credential exposure
- Uses regex patterns to identify common secret naming conventions

### Dangerous Commands Detection
- Warns when workflows contain potentially dangerous shell commands
- Monitored commands include: `rm`, `dd`, `mkfs`, `format`, `fdisk`, `kill`, `killall`, `pkill`
- Helps prevent accidental destructive operations

### Command Injection Warnings
- Detects unquoted variable references in commands
- Warns about potential shell injection vulnerabilities
- Encourages proper quoting and escaping of user-provided values

### Timeout Enforcement
- Warns when command steps lack timeout configuration
- Prevents runaway processes from consuming resources indefinitely
- Recommends setting explicit timeouts for all command steps

## Installation

### From Plugin Directory

```bash
cd examples/plugins/awf-plugin-security-validator
make build install
```

The plugin will be installed to `~/.local/share/awf/plugins/awf-plugin-security-validator/`

### Manual Installation

```bash
cd examples/plugins/awf-plugin-security-validator
go build -o awf-plugin-security-validator .
mkdir -p ~/.local/share/awf/plugins/awf-plugin-security-validator
cp awf-plugin-security-validator ~/.local/share/awf/plugins/awf-plugin-security-validator/
cp plugin.yaml ~/.local/share/awf/plugins/awf-plugin-security-validator/
```

## Usage

Once installed, the security validator automatically runs during `awf validate`:

```bash
awf validate my-workflow.yaml
```

The output will include security validation issues alongside built-in AWF validation results.

### Example Output

```
Validating workflow 'my-workflow.yaml'...

Built-in Validation:
✓ All steps are reachable
✓ No circular references detected

Security Validation:
✗ Step 'deploy' - Potential hardcoded secret detected in command
⚠ Step 'update' - Command step has no timeout
⚠ Step 'cleanup' - Command uses potentially dangerous operation: rm
```

## Severity Levels

The plugin reports issues at three severity levels:

- **Error (✗)**: Security issue that should be fixed (e.g., hardcoded secrets)
- **Warning (⚠)**: Security concern that should be reviewed (e.g., dangerous commands)
- **Info (ℹ)**: Informational guidance (e.g., missing timeouts)

## Testing

Run the test suite:

```bash
make test
```

Tests include:
- Unit tests for each validation check
- Integration tests using self-hosting pattern (plugin as subprocess)
- Manifest and configuration validation

## Security Patterns Detected

### API Keys
```yaml
steps:
  fetch:
    type: command
    command: "curl -H 'api_key=sk_prod_abc123' https://api.example.com"  # ✗ Error
```

### Passwords
```yaml
steps:
  database:
    type: command
    command: "psql -U admin -p password123 mydb"  # ✗ Error
```

### Dangerous Commands
```yaml
steps:
  cleanup:
    type: command
    command: "rm -rf /tmp/*"  # ⚠ Warning
```

### Missing Timeouts
```yaml
steps:
  process:
    type: command
    command: "long-running-script"  # ⚠ Warning: no timeout set
```

## Example Workflow

See `../../tests/fixtures/workflows/` for example workflows that demonstrate the validator's capabilities.

## Manifest

The plugin declares itself as a validator in `plugin.yaml`:

```yaml
name: security-validator
version: 1.0.0
description: Example security validator plugin
awf_version: ">=0.5.0"
capabilities:
  - validators
```

## Implementation Details

The plugin implements the `sdk.Validator` interface with two methods:

### ValidateWorkflow
Called once per validation run with the entire workflow definition. Can perform workflow-level security checks.

### ValidateStep
Called for each step in the workflow. This is where per-step security checks are performed (secrets detection, command validation, etc.).

## Extending the Validator

To add additional security checks:

1. Add a new regex pattern to `secretPatterns` in `Init()`
2. Add a new dangerous word to `dangerousWords` list
3. Implement additional check methods following the existing pattern
4. Add corresponding tests

## Version Information

- **Plugin Version**: 1.0.0
- **Requires AWF**: >= 0.5.0
- **Go Version**: 1.21+

## License

MIT - See LICENSE file in the AWF project root
