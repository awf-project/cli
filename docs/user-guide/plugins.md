# Plugins

AWF supports plugins to extend functionality with custom operations. AWF ships with **built-in plugins** for GitHub operations and notifications, and supports **external RPC plugins** for additional integrations.

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

## Built-in Notification Plugin

AWF includes a built-in notification provider that sends alerts when workflows complete. It exposes a single `notify.send` operation that dispatches to four backends: desktop notifications, [ntfy](https://ntfy.sh), Slack, and generic webhooks.

**Key features:**
- 1 operation: `notify.send` with backend dispatch
- 4 backends: `desktop`, `ntfy`, `slack`, `webhook`
- 10-second HTTP timeout for network backends (prevents workflow stalls)
- Platform detection for desktop notifications (`notify-send` on Linux, `osascript` on macOS)
- All inputs support AWF template interpolation (`{{workflow.name}}`, `{{workflow.duration}}`, etc.)

```yaml
notify_team:
  type: operation
  operation: notify.send
  inputs:
    backend: slack
    title: "Build Complete"
    message: "{{workflow.name}} finished in {{workflow.duration}}"
  on_success: done
  on_failure: error
```

### Notification Backends

| Backend | Transport | Required Config | Required Inputs |
|---------|-----------|-----------------|-----------------|
| `desktop` | OS-native (`notify-send` / `osascript`) | None | `message` |
| `ntfy` | HTTP POST to ntfy server | `ntfy_url` in config | `message`, `topic` |
| `slack` | HTTP POST to Slack webhook | `slack_webhook_url` in config | `message` |
| `webhook` | HTTP POST to arbitrary URL | None | `message`, `webhook_url` |

### Operation Inputs

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| `backend` | string | Yes | Notification backend: `desktop`, `ntfy`, `slack`, `webhook` |
| `message` | string | Yes | Notification message body |
| `title` | string | No | Notification title (defaults to "AWF Workflow") |
| `priority` | string | No | Priority: `low`, `default`, `high` (defaults to `default`) |
| `topic` | string | No | ntfy topic name (required for `ntfy` backend) |
| `webhook_url` | string | No | Webhook URL (required for `webhook` backend) |
| `channel` | string | No | Slack channel override |

### Operation Outputs

| Output | Type | Description |
|--------|------|-------------|
| `backend` | string | Which backend handled the notification |
| `status` | string | HTTP status code (network backends) or confirmation |
| `response` | string | Response body or confirmation message |

### Configuration

Configure notification backends in `.awf/config.yaml`:

```yaml
plugins:
  notify:
    ntfy_url: "https://ntfy.sh"
    slack_webhook_url: "https://hooks.slack.com/services/..."
    default_backend: "desktop"
```

| Config Key | Description |
|------------|-------------|
| `ntfy_url` | Base URL for ntfy server (required for `ntfy` backend) |
| `slack_webhook_url` | Slack incoming webhook URL (required for `slack` backend) |
| `default_backend` | Backend to use when `backend` input is omitted |

When both a config `default_backend` and an explicit `backend` input are set, the explicit input takes precedence.

### Backend Details

**Desktop** - Uses `notify-send` on Linux and `osascript -e 'display notification'` on macOS. Fails gracefully on unsupported platforms (e.g., headless servers).

**ntfy** - Posts to `<ntfy_url>/<topic>` with the notification payload. Supports priority mapping. Ideal for self-hosted push notifications to mobile devices.

**Slack** - Posts a formatted message block to the configured Slack incoming webhook URL. Includes workflow name, status, and duration in the message.

**Webhook** - Sends a generic JSON POST to any URL. The payload includes `workflow`, `status`, `duration`, `message`, and `outputs` fields. Use this for Discord, Teams, PagerDuty, or any HTTP integration.

See [Workflow Syntax - Notification Operations](workflow-syntax.md#notification-operations) for complete examples.

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
