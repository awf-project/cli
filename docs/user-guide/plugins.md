---
title: "Plugins"
---

AWF supports plugins to extend functionality with custom operations. AWF ships with **built-in plugins** for HTTP requests, GitHub operations, and notifications, and supports **external RPC plugins** for additional integrations.

## Built-in GitHub Plugin

AWF includes a built-in GitHub operation provider that offers 8 declarative operations for interacting with GitHub issues, pull requests, labels, and comments. Unlike external RPC plugins, the GitHub plugin runs in-process with zero IPC overhead.

**Key features:**
- 8 operations: `get_issue`, `get_pr`, `create_issue`, `create_pr`, `add_labels`, `add_comment`, `list_comments`, `batch`
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

## Built-in HTTP Operation

AWF includes a built-in HTTP operation provider that enables declarative REST API calls without shell commands. The `http.request` operation supports standard HTTP methods and captures structured responses for conditional routing.

**Key features:**
- 4 HTTP methods: GET, POST, PUT, DELETE
- Configurable timeout (default 30 seconds)
- Response capture: status code, body, headers
- Template interpolation in URL, headers, and body
- Retryable status codes for transient failures (429, 502, 503, etc.)
- 1MB response body limit to prevent memory exhaustion

```yaml
fetch_user:
  type: operation
  operation: http.request
  inputs:
    method: GET
    url: "https://api.example.com/users/{{.inputs.user_id}}"
    headers:
      Authorization: "Bearer {{.inputs.api_token}}"
      Accept: "application/json"
    timeout: 10
  on_success: process
  on_failure: error
```

### Operation Inputs

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | HTTP endpoint URL (must start with `http://` or `https://`) |
| `method` | string | Yes | HTTP method: `GET`, `POST`, `PUT`, `DELETE` |
| `headers` | object | No | Custom headers as key-value pairs |
| `body` | string | No | Request body (for POST/PUT) |
| `timeout` | integer | No | Request timeout in seconds (default: 30) |
| `retryable_status_codes` | array | No | Status codes triggering retries (e.g., `[429, 502, 503]`) |

### Operation Outputs

| Output | Type | Description |
|--------|------|-------------|
| `status_code` | integer | HTTP response status (200, 404, 503, etc.) |
| `body` | string | Response body (truncated at 1MB) |
| `headers` | object | Response headers (canonicalized names, multi-value joined with `, `) |
| `body_truncated` | boolean | `true` if the response body exceeded 1MB and was truncated |

### Examples

**GET Request with Response Access:**

```yaml
fetch_data:
  type: operation
  operation: http.request
  inputs:
    method: GET
    url: "https://api.example.com/status"
    headers:
      Accept: "application/json"
  on_success: process
  on_failure: error
```

**POST with Retry:**

```yaml
create_resource:
  type: operation
  operation: http.request
  inputs:
    method: POST
    url: "https://api.example.com/resources"
    headers:
      Content-Type: "application/json"
      Authorization: "Bearer {{.inputs.api_token}}"
    body: '{"name": "{{.inputs.resource_name}}", "owner": "{{.inputs.user_id}}"}'
    timeout: 15
    retryable_status_codes: [429, 502, 503]
  retry:
    max_attempts: 3
    backoff: exponential
    initial_delay: 1s
  on_success: success
  on_failure: error
```

**Multi-Step Workflow with Response Capture:**

```yaml
name: fetch-and-process
version: "1.0.0"

inputs:
  - name: api_url
    type: string
    required: true
  - name: api_key
    type: string
    required: true

states:
  initial: fetch

  fetch:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{.inputs.api_url}}"
      headers:
        Authorization: "Bearer {{.inputs.api_key}}"
    on_success: process_response
    on_failure: handle_error

  process_response:
    type: step
    command: echo "Got status {{.states.fetch.Response.status_code}}: {{.states.fetch.Response.body}}"
    on_success: done

  handle_error:
    type: terminal
    status: failure

  done:
    type: terminal
    status: success
```

See [Workflow Syntax - HTTP Operations](workflow-syntax.md#http-operations) for complete reference.

---

## Built-in Notification Plugin

AWF includes a built-in notification provider that sends alerts when workflows complete. It exposes a single `notify.send` operation that dispatches to two backends: desktop notifications and generic webhooks.

**Key features:**
- 1 operation: `notify.send` with backend dispatch
- 2 backends: `desktop`, `webhook`
- 10-second HTTP timeout for network backends (prevents workflow stalls)
- Platform detection for desktop notifications (`notify-send` on Linux, `osascript` on macOS)
- All inputs support AWF template interpolation (`{{workflow.name}}`, `{{workflow.duration}}`, etc.)

```yaml
notify_team:
  type: operation
  operation: notify.send
  inputs:
    backend: desktop
    title: "Build Complete"
    message: "{{workflow.name}} finished in {{workflow.duration}}"
  on_success: done
  on_failure: error
```

### Notification Backends

| Backend | Transport | Required Config | Required Inputs |
|---------|-----------|-----------------|-----------------|
| `desktop` | OS-native (`notify-send` / `osascript`) | None | `message` |
| `webhook` | HTTP POST to arbitrary URL | None | `message`, `webhook_url` |

### Operation Inputs

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| `backend` | string | Yes | Notification backend: `desktop`, `webhook` |
| `message` | string | Yes | Notification message body |
| `title` | string | No | Notification title (defaults to "AWF Workflow") |
| `priority` | string | No | Priority: `low`, `default`, `high` (defaults to `default`) |
| `webhook_url` | string | No | Webhook URL (required for `webhook` backend) |

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
    default_backend: "desktop"
```

| Config Key | Description |
|------------|-------------|
| `default_backend` | Backend to use when `backend` input is omitted |

When both a config `default_backend` and an explicit `backend` input are set, the explicit input takes precedence.

### Backend Details

**Desktop** - Uses `notify-send` on Linux and `osascript -e 'display notification'` on macOS. Fails gracefully on unsupported platforms (e.g., headless servers).

**Webhook** - Sends a generic JSON POST to any URL. The payload includes `workflow`, `status`, `duration`, `message`, and `outputs` fields. Use this for ntfy, Slack, Discord, Teams, PagerDuty, or any HTTP integration.

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
└── awf-plugin-github/
    ├── plugin.yaml         # Plugin manifest
    └── awf-plugin-github   # Executable binary
```

### Plugin Manifest

Every plugin requires a `plugin.yaml` manifest:

```yaml
name: awf-plugin-github
version: 1.0.0
description: GitHub integration for AWF workflows
awf_version: ">=0.4.0"
capabilities:
  - operations
config:
  token:
    type: string
    required: true
    description: GitHub API token
```

#### Manifest Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Plugin identifier (must match directory name) |
| `version` | string | Yes | Semantic version |
| `description` | string | No | Brief description |
| `awf_version` | string | Yes | AWF version constraint (semver) |
| `capabilities` | array | Yes | List: `operations`, `step_types`, `validators` |
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

Output shows all plugins (built-in and external) with their status and source:

```
NAME                TYPE      VERSION  STATUS   ENABLED  CAPABILITIES  SOURCE
github              builtin   dev      builtin  yes      operations    -
http                builtin   dev      builtin  yes      operations    -
notify              builtin   dev      builtin  yes      operations    -
awf-plugin-jira     external  2.1.0    enabled  yes      operations    myorg/awf-plugin-jira
awf-plugin-metrics  external  1.0.0    disabled no       operations    acme/awf-plugin-metrics
```

The `SOURCE` column shows the GitHub `owner/repo` for plugins installed via `awf plugin install`. Manually installed plugins and built-in plugins show `-`.

#### Install a Plugin

Install an external plugin from GitHub Releases:

```bash
awf plugin install owner/repo
```

AWF downloads the latest release, verifies the SHA-256 checksum, extracts the archive, validates the manifest, and installs atomically.

**Flags:**

| Flag | Description |
|------|-------------|
| `--version` | Version constraint (e.g. `">=1.0.0 <2.0.0"`) |
| `--pre-release` | Include alpha/beta/rc versions in resolution |
| `--force` | Overwrite an existing installation |

**Examples:**

```bash
# Install latest stable release
awf plugin install myorg/awf-plugin-jira

# Install with version constraint
awf plugin install myorg/awf-plugin-jira --version ">=1.0.0 <2.0.0"

# Include pre-release versions
awf plugin install myorg/awf-plugin-jira --pre-release

# Reinstall (overwrite existing)
awf plugin install myorg/awf-plugin-jira --force
```

The `owner/repo` argument must be a GitHub repository path (not a URL). The repository must contain GitHub Releases with `.tar.gz` assets matching the AWF naming convention (see [Release Asset Naming](#release-asset-naming)).

#### Update a Plugin

Update an installed plugin to the latest version:

```bash
awf plugin update <name>
```

AWF fetches the latest release from the plugin's source repository, verifies the checksum, and performs an atomic replacement.

**Flags:**

| Flag | Description |
|------|-------------|
| `--all` | Update all externally installed plugins |

**Examples:**

```bash
# Update a specific plugin
awf plugin update jira

# Update all external plugins
awf plugin update --all
```

Running `awf plugin update` without a plugin name and without `--all` returns a usage error.

#### Remove a Plugin

Remove an installed external plugin:

```bash
awf plugin remove <name>
```

AWF shuts down the plugin process, removes the plugin directory, and clears its state.

**Flags:**

| Flag | Description |
|------|-------------|
| `--keep-data` | Preserve plugin configuration and state |

**Examples:**

```bash
# Remove a plugin
awf plugin remove jira

# Remove but keep configuration
awf plugin remove jira --keep-data
```

Built-in plugins cannot be removed. Attempting to remove a built-in plugin returns an error with a hint to use `awf plugin disable` instead.

#### Search for Plugins

Search for AWF plugins on GitHub:

```bash
awf plugin search [query]
```

Searches GitHub repositories tagged with the `awf-plugin` topic. Results include repository name, description, and latest version.

```bash
# Search all AWF plugins
awf plugin search

# Search with a keyword
awf plugin search jira
```

Use `--output=json` for machine-readable output.

#### Enable/Disable Plugins

```bash
# Disable a plugin
awf plugin disable awf-plugin-github

# Enable a plugin
awf plugin enable awf-plugin-github
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
    operation: notify.send            # Built-in operation
    inputs:
      backend: webhook
      webhook_url: "https://example.com/hooks/deployments"
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
  awf-plugin-github:
    token: "${GITHUB_TOKEN}"
```

Environment variables in config values are expanded at runtime.

### Built-in Plugin Visibility

AWF ships with 3 built-in plugins that always appear in `awf plugin list`:

```
$ awf plugin list
NAME    TYPE     VERSION  STATUS   ENABLED  CAPABILITIES  SOURCE
github  builtin  dev      builtin  yes      operations    -
http    builtin  dev      builtin  yes      operations    -
notify  builtin  dev      builtin  yes      operations    -
```

Built-in plugins can be disabled and re-enabled like external plugins:

```bash
awf plugin disable http    # Disable the HTTP provider
awf plugin enable http     # Re-enable it
```

Use `--operations` to see operations grouped by plugin:

```
$ awf plugin list --operations
NAME                  PLUGIN
github.get_issue      github
github.get_pr         github
github.create_pr      github
github.create_issue   github
github.add_labels     github
github.list_comments  github
github.add_comment    github
github.batch          github
http.request          http
notify.send           notify
```

## Writing External Plugins

External plugins are Go binaries that call `sdk.Serve()` from `main()`. AWF discovers them via their `plugin.yaml` manifest and communicates over gRPC using HashiCorp go-plugin.

### Minimal Plugin

```go
package main

import (
    "context"

    "github.com/awf-project/cli/pkg/plugin/sdk"
)

type MyPlugin struct {
    sdk.BasePlugin
}

func (p *MyPlugin) Operations() []string {
    return []string{"my_op"}
}

func (p *MyPlugin) HandleOperation(_ context.Context, name string, inputs map[string]any) (*sdk.OperationResult, error) {
    text := sdk.GetStringDefault(inputs, "text", "")
    return sdk.NewSuccessResult(text, nil), nil
}

func main() {
    sdk.Serve(&MyPlugin{
        BasePlugin: sdk.BasePlugin{
            PluginName:    "awf-plugin-myplugin",
            PluginVersion: "1.0.0",
        },
    })
}
```

### SDK Helpers

| Helper | Description |
|--------|-------------|
| `sdk.BasePlugin` | Embed to satisfy `sdk.Plugin` interface with no-op defaults |
| `sdk.Serve(p)` | Start the plugin process; blocks until host disconnects |
| `sdk.NewSuccessResult(output, data)` | Build a success result |
| `sdk.NewErrorResult(msg)` | Build an error result |
| `sdk.GetStringDefault(inputs, key, default)` | Extract string input with fallback |
| `sdk.GetIntDefault(inputs, key, default)` | Extract integer input with fallback |
| `sdk.GetBoolDefault(inputs, key, default)` | Extract boolean input with fallback |

### Validator Plugin

Implement `sdk.Validator` to add custom validation rules that run during `awf validate`. AWF calls your validator after built-in validation and displays findings alongside built-in errors.

**Severity levels:**

| Icon | Severity | Constant |
|------|----------|----------|
| `✗` | Error | `sdk.SeverityError` |
| `⚠` | Warning | `sdk.SeverityWarning` |
| `ℹ` | Info | `sdk.SeverityInfo` |

```go
package main

import (
    "context"

    "github.com/awf-project/cli/pkg/plugin/sdk"
)

type SecurityValidator struct {
    sdk.BasePlugin
}

func (v *SecurityValidator) ValidateWorkflow(ctx context.Context, w sdk.WorkflowDefinition) ([]sdk.ValidationIssue, error) {
    var issues []sdk.ValidationIssue
    if w.Version == "" {
        issues = append(issues, sdk.ValidationIssue{
            Severity: sdk.SeverityWarning,
            Message:  "workflow is missing a version field",
        })
    }
    return issues, nil
}

func (v *SecurityValidator) ValidateStep(ctx context.Context, w sdk.WorkflowDefinition, stepName string) ([]sdk.ValidationIssue, error) {
    step, ok := w.Steps[stepName]
    if !ok {
        return nil, nil
    }
    var issues []sdk.ValidationIssue
    if step.Type == "step" && step.Timeout == 0 {
        issues = append(issues, sdk.ValidationIssue{
            Severity: sdk.SeverityInfo,
            Message:  "step has no timeout",
            Step:     stepName,
            Field:    "timeout",
        })
    }
    return issues, nil
}

func main() {
    sdk.Serve(&SecurityValidator{
        BasePlugin: sdk.BasePlugin{
            PluginName:    "awf-plugin-security-validator",
            PluginVersion: "1.0.0",
        },
    })
}
```

Declare the `validators` capability in `plugin.yaml`:

```yaml
capabilities:
  - validators
```

**Flags for `awf validate`:**

| Flag | Default | Description |
|------|---------|-------------|
| `--skip-plugins` | false | Skip all plugin validators |
| `--validator-timeout` | 5s | Per-plugin timeout (e.g., `10s`, `2m`) |

Validator crashes are treated as timeouts — AWF logs a warning and continues with remaining validators. Results are deduplicated by `(message + step + field)`.

---

### Step Type Plugin

Implement `sdk.StepTypeHandler` to register new `type:` values for workflow steps. AWF calls `StepTypes()` once at init to cache registrations, then routes any step with a matching type to your plugin.

**Automatic namespacing:** Plugins declare short step type names (e.g. `query`). The host automatically prefixes with `<manifest-name>.` at registration. Users write the qualified name in YAML (e.g. `type: database.query` where `database` is the `name` in `plugin.yaml`). The plugin receives the short name in `ExecuteStep`. This follows the same pattern as operation namespacing.

```go
package main

import (
    "context"
    "fmt"

    "github.com/awf-project/cli/pkg/plugin/sdk"
)

type DatabasePlugin struct {
    sdk.BasePlugin
}

// StepTypes declares short names — the host auto-prefixes with "awf-plugin-database."
func (p *DatabasePlugin) StepTypes() []sdk.StepTypeInfo {
    return []sdk.StepTypeInfo{
        {Name: "query", Description: "Execute a SQL query"},
        {Name: "migrate", Description: "Run database migrations"},
    }
}

// ExecuteStep receives the short name (prefix stripped by host)
func (p *DatabasePlugin) ExecuteStep(ctx context.Context, req sdk.StepExecuteRequest) (sdk.StepExecuteResult, error) {
    switch req.StepType {
    case "query":
        query, _ := req.Config["query"].(string)
        // ... execute query
        return sdk.StepExecuteResult{
            Output:   fmt.Sprintf("executed: %s", query),
            Data:     map[string]any{"rows": 42},
            ExitCode: 0,
        }, nil
    default:
        return sdk.StepExecuteResult{ExitCode: 1}, fmt.Errorf("unknown step type: %s", req.StepType)
    }
}

func main() {
    sdk.Serve(&DatabasePlugin{
        BasePlugin: sdk.BasePlugin{
            PluginName:    "database",
            PluginVersion: "1.0.0",
        },
    })
}
```

Declare the `step_types` capability in `plugin.yaml`:

```yaml
capabilities:
  - step_types
```

Step type name conflicts are resolved by first-registered-wins on the qualified name. AWF logs a warning if two plugins register the same qualified type name.

See [Workflow Syntax - Custom Step Types](workflow-syntax.md#custom-step-type-state) for how to use custom step types in workflows.

---

### Echo Plugin Example

The `examples/plugins/awf-plugin-echo/` directory contains a complete working plugin that echoes its input text. Use it as a starting point:

```bash
cd examples/plugins/awf-plugin-echo
make install          # Build and install to ~/.local/share/awf/plugins/
awf plugin enable awf-plugin-echo
```

Use it in a workflow:

```yaml
echo_step:
  type: operation
  operation: awf-plugin-echo.echo
  inputs:
    text: "Hello from plugin!"
    prefix: ">>>"
  on_success: done
```

---

## Distributing Plugins via GitHub Releases

AWF installs plugins from GitHub Releases. Plugin authors must publish `.tar.gz` archives with a specific naming convention so AWF can resolve the correct asset for each platform.

### Release Asset Naming

Assets must follow this pattern:

```
<plugin-name>_<version>_<os>_<arch>.tar.gz
```

| Component | Values |
|-----------|--------|
| `plugin-name` | Plugin binary name (e.g. `awf-plugin-jira`) |
| `version` | Semantic version without `v` prefix (e.g. `1.2.0`) |
| `os` | `linux`, `darwin`, `windows` |
| `arch` | `amd64`, `arm64` |

**Example release assets:**

```
awf-plugin-jira_1.2.0_linux_amd64.tar.gz
awf-plugin-jira_1.2.0_linux_arm64.tar.gz
awf-plugin-jira_1.2.0_darwin_amd64.tar.gz
awf-plugin-jira_1.2.0_darwin_arm64.tar.gz
```

Each archive must contain:
- The plugin binary (`awf-plugin-<name>`)
- A `plugin.yaml` manifest

A `checksums.txt` file (SHA-256) must be included as a separate release asset. AWF verifies the checksum before installation.

### GoReleaser Configuration

Use [GoReleaser](https://goreleaser.com/) to automate plugin releases. Add a `.goreleaser.yml` to your plugin repository:

```yaml
project_name: awf-plugin-myplugin

builds:
  - main: .
    binary: awf-plugin-myplugin
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - plugin.yaml

checksum:
  name_template: checksums.txt
  algorithm: sha256

release:
  github:
    owner: your-org
    name: awf-plugin-myplugin
```

All archives must use `.tar.gz` format. This differs from AWF's own releases which use `.zip` on macOS.

### Authentication

AWF uses authentication for GitHub API requests in the following order:

1. `GITHUB_TOKEN` environment variable (if set)
2. `gh auth token` output (if `gh` CLI is installed and authenticated)
3. Unauthenticated requests (subject to lower rate limits)

Set `GITHUB_TOKEN` for CI environments or to avoid rate limiting:

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxx
awf plugin install myorg/awf-plugin-jira
```

When the GitHub API rate limit is exceeded, AWF detects the `X-RateLimit-Remaining: 0` header and returns an actionable error message suggesting authentication.

---

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
