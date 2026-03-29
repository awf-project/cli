# AWF Database Step Type Plugin

An example step type plugin for AWF that provides custom database operation step types.

## Features

The Database Step Type Plugin provides the following capabilities:

### Custom Step Types
- Extend AWF with custom `type:` values for database operations
- Define step-specific configuration via `config:` field
- Access results via state interpolation

## Installation

### From Plugin Directory

```bash
cd examples/plugins/awf-plugin-database
make build install
```

The plugin will be installed to `~/.local/share/awf/plugins/awf-plugin-database/`

### Manual Installation

```bash
cd examples/plugins/awf-plugin-database
go build -o awf-plugin-database .
mkdir -p ~/.local/share/awf/plugins/awf-plugin-database
cp awf-plugin-database ~/.local/share/awf/plugins/awf-plugin-database/
cp plugin.yaml ~/.local/share/awf/plugins/awf-plugin-database/
```

## Usage

Once installed, you can use custom step types in workflow YAML:

```yaml
workflows:
  my-workflow:
    initial: fetch-users
    steps:
      fetch-users:
        type: database.query
        config:
          connection: postgres://localhost/mydb
          query: "SELECT * FROM users"
        on_success: show-result
      show-result:
        type: command
        command: "echo {{states.fetch-users.Output}}"
```

> **Note:** The host automatically prefixes step type names with `<plugin-name>.` at registration.
> The plugin declares `query`; the user writes `type: awf-plugin-database.query` in YAML.

Results are accessible via state interpolation:
- `{{states.fetch-users.Output}}` — text output
- `{{states.fetch-users.Data.rows_affected}}` — structured data fields

## Testing

Run the test suite:

```bash
make test
```

Tests include:
- Unit tests for plugin interface compliance
- Integration tests using self-hosting pattern (plugin as subprocess)
- Manifest and configuration validation

## Extending the Plugin

The plugin implements the `sdk.StepTypeHandler` interface. To add new step types:

1. Implement `StepTypes()` to register available types
2. Implement `ExecuteStep()` to handle execution requests
3. Return results with `Output`, `Data`, and `ExitCode` fields
