# Plugins

AWF supports plugins to extend functionality with custom operations. AWF ships with a **built-in GitHub plugin** for common GitHub operations, and supports **external RPC plugins** for additional integrations.

## Built-in GitHub Plugin

AWF includes a built-in GitHub operation provider that offers 9 declarative operations for interacting with GitHub issues, pull requests, labels, comments, and projects. Unlike external RPC plugins, the GitHub plugin runs in-process with zero IPC overhead.

**Key features:**
- 9 operations: `get_issue`, `get_pr`, `create_issue`, `create_pr`, `add_labels`, `add_comment`, `list_comments`, `set_project_status`, `batch`
- Automatic authentication via `gh` CLI or `GITHUB_TOKEN` environment variable
- Repository auto-detection from git remote
- Batch execution with configurable concurrency and failure strategies

```yaml
get_issue:
  type: operation
  operation: github.get_issue
  inputs:
    number: 42
  on_success: process
  on_failure: error
```

See [Workflow Syntax - Operation State](workflow-syntax.md#operation-state) for complete reference and examples.

---

## External RPC Plugins

### Overview

External plugins are standalone executables that communicate with AWF via RPC (HashiCorp go-plugin). This architecture provides:

- **Process isolation** - Plugins run in separate processes
- **Cross-platform support** - No CGO or platform-specific binaries
- **Safe updates** - Replace plugins without recompiling AWF
- **Graceful failures** - Plugin crashes don't affect AWF core

### Plugin Directory

AWF discovers plugins from:

```
$XDG_DATA_HOME/awf/plugins/     # Default: ~/.local/share/awf/plugins/
```

Each plugin must have:
- A `plugin.yaml` manifest file
- An executable binary (named `awf-plugin-<name>`)

```
plugins/
└── awf-plugin-slack/
    ├── plugin.yaml         # Plugin manifest
    └── awf-plugin-slack    # Executable binary
```

### Plugin Manifest

Every plugin requires a `plugin.yaml` manifest:

```yaml
name: awf-plugin-slack
version: 1.0.0
description: Slack notifications for AWF workflows
awf_version: ">=0.4.0"
capabilities:
  - operations
config:
  webhook_url:
    type: string
    required: true
    description: Slack webhook URL
```

#### Manifest Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Plugin identifier (must match directory name) |
| `version` | string | Yes | Semantic version |
| `description` | string | No | Brief description |
| `awf_version` | string | Yes | AWF version constraint (semver) |
| `capabilities` | array | Yes | List: `operations`, `commands`, `validators` |
| `config` | object | No | Configuration schema |

#### Config Field Schema

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `string`, `integer`, `boolean` |
| `required` | bool | If true, must be configured |
| `default` | any | Default value |
| `description` | string | Help text |

### Managing Plugins

#### List Plugins

```bash
awf plugin list
```

Output shows all discovered plugins with their status:

```
NAME                VERSION  STATUS   CAPABILITIES  DESCRIPTION
awf-plugin-slack    1.0.0    enabled  operations    Slack notifications
awf-plugin-github   2.1.0    disabled operations    GitHub API integration
```

#### Enable/Disable Plugins

```bash
# Disable a plugin
awf plugin disable awf-plugin-slack

# Enable a plugin
awf plugin enable awf-plugin-slack
```

Plugin state persists across AWF restarts.

### Using Plugin Operations

Plugins register custom operations that can be used in workflow steps:

```yaml
name: deploy-with-notification
version: "1.0.0"

states:
  initial: deploy

  deploy:
    type: step
    command: ./deploy.sh
    on_success: notify
    on_failure: error

  notify:
    type: step
    operation: slack.send_message    # Plugin operation
    inputs:
      channel: "#deployments"
      message: "Deploy completed: {{.states.deploy.Output}}"
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

#### Operation Syntax

```yaml
step_name:
  type: step
  operation: <plugin>.<operation>
  inputs:
    key: value
```

- `operation` - Plugin operation in format `plugin_name.operation_name`
- `inputs` - Operation-specific parameters (supports variable interpolation)

### Plugin Configuration

Configure plugins via environment variables or config file:

```yaml
# .awf.yaml
plugins:
  awf-plugin-slack:
    webhook_url: "https://hooks.slack.com/services/..."
  awf-plugin-github:
    token: "${GITHUB_TOKEN}"
```

Environment variables in config values are expanded at runtime.

### Plugin Development

Use the `pkg/plugin/sdk` package to create your own plugins.

#### Quick Start

```go
package main

import (
    "github.com/vanoix/awf/pkg/plugin/sdk"
)

type MyPlugin struct{}

func (p *MyPlugin) Name() string    { return "awf-plugin-example" }
func (p *MyPlugin) Version() string { return "1.0.0" }

func (p *MyPlugin) Init(config map[string]interface{}) error {
    return nil
}

func (p *MyPlugin) Shutdown() error {
    return nil
}

func (p *MyPlugin) Operations() []sdk.Operation {
    return []sdk.Operation{
        {
            Name:        "greet",
            Description: "Say hello",
            Execute: func(ctx sdk.Context, inputs map[string]interface{}) (sdk.Result, error) {
                name := inputs["name"].(string)
                return sdk.Result{Output: "Hello, " + name}, nil
            },
        },
    }
}

func main() {
    sdk.Serve(&MyPlugin{})
}
```

## Troubleshooting

### Plugin Not Found

```
Error: plugin "awf-plugin-foo" not found
```

Check:
1. Plugin directory exists in `~/.local/share/awf/plugins/`
2. Directory name matches plugin name
3. `plugin.yaml` manifest is present and valid

### Plugin Load Failed

```
Error: failed to load plugin: exec format error
```

The plugin binary is not compatible with your system. Rebuild for your platform.

### Version Mismatch

```
Error: plugin requires awf >=0.5.0, current version is 0.4.0
```

Update AWF or use a compatible plugin version.

## See Also

- [Commands](commands.md) - CLI command reference
- [Workflow Syntax](workflow-syntax.md) - Operation usage in workflows
- [Architecture](../development/architecture.md) - Plugin system internals
