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
| `capabilities` | array | Yes | List: `operations`, `step_types`, `validators`, `events` |
| `events` | object | No | Event subscriptions and emissions (see [Plugin Events](#plugin-events)) |
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
awf plugin install owner/repo[@version]
```

AWF downloads the latest release, verifies the SHA-256 checksum, extracts the archive, validates the manifest, and installs atomically.

Explicit versions use `owner/repo@version` syntax and must be exact SemVer values. Both `1.2.3` and `v1.2.3` are accepted; ranges such as `>=1.0.0` are rejected.

**Flags:**

| Flag | Description |
|------|-------------|
| `--pre-release` | Include alpha/beta/rc versions in resolution |
| `--force` | Overwrite an existing installation |

**Examples:**

```bash
# Install latest stable release
awf plugin install myorg/awf-plugin-jira

# Install an exact version
awf plugin install myorg/awf-plugin-jira@v1.2.3

# Include pre-release versions
awf plugin install myorg/awf-plugin-jira --pre-release

# Reinstall (overwrite existing)
awf plugin install myorg/awf-plugin-jira --force
```

The `owner/repo[@version]` argument must be a GitHub repository path (not a URL). The repository must contain GitHub Releases with `.tar.gz` assets matching the AWF naming convention (see [Release Asset Naming](#release-asset-naming)).

#### Create a Plugin

Use `awf plugin init` to generate a working plugin repository:

```bash
awf plugin init awf-plugin-example --kind operation
cd awf-plugin-example
make test
make build
make install-local
awf plugin enable awf-plugin-example
awf plugin list --operations
awf run examples/demo.yaml
```

The MVP supports `--kind operation`. If `--kind` is omitted, AWF uses the operation scaffold. The generated repository includes source code, tests, a manifest, local install targets, package and checksum targets, a demo workflow, a README, and a GitHub Actions release workflow.

`awf-plugin-example` is the distribution name used for the repository, binary, install directory, and release assets. The generated manifest uses the runtime plugin id `example`, so workflow operation references use `example.echo`.

The scaffold `kind` selects a starter repository shape. Manifest `capabilities` describe what the installed plugin exposes at runtime. The generated operation template advertises `operations`; planned future kinds include `canonical-port`, `adapter`, `direct-integration`, `hybrid`, `validator`, `step-type`, `event-listener`, and `full`.

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

#### Verify Plugin Integrity

Verify that installed plugin binaries have not been modified or corrupted:

```bash
# Verify all plugins
awf plugin verify

# Verify a specific plugin
awf plugin verify jira

# Verify and update stored checksums (useful for manually installed or local plugins)
awf plugin verify jira --update
```

The verify command checks the SHA-256 checksum of each plugin binary against a stored value. For plugins installed via `awf plugin install`, the checksum is recorded automatically at install time. For manually placed or locally-built plugins, use `--update` to compute and store their checksums.

**Output example:**

```
Plugin                Status   Expected Hash                          Actual Hash                            
awf-plugin-jira       ✓ pass   a3f9d4c5e8b2f1g7h8i9j0k1l2m3n4o5p   a3f9d4c5e8b2f1g7h8i9j0k1l2m3n4o5p
awf-plugin-metrics    ✗ fail   b2e8c3d7f6a4h5i9j2k3l4m5n6o7p8q9r   x1y2z3a4b5c6d7e8f9g0h1i2j3k4l5m6n7
awf-plugin-custom     ! miss   (no stored checksum)                   c9h8i7j6k5l4m3n2o1p0q1r2s3t4u5v6w
```

- **✓ pass** - Binary matches the stored checksum (integrity verified)
- **✗ fail** - Binary does not match; the plugin may be corrupted or tampered with
- **! miss** - No checksum stored; plugin will launch without verification (use `--update` to enable verification)

### Plugin Security

AWF implements multiple security layers for plugin execution:

#### Automatic Mutual TLS (AutoMTLS)

All host-plugin communication uses automatic mutual TLS encryption by default. AWF and plugin binaries automatically generate ephemeral certificates at startup — no manual key management is required.

**Benefits:**
- Prevents network sniffing of plugin data on shared infrastructure
- Protects secrets passed through plugin communication
- Transparent to end users and plugin authors

**Backward compatibility:** If a plugin binary is built with an older SDK that doesn't support AutoMTLS, the connection automatically downgrades to plaintext with a warning in the logs. The plugin continues to function.

#### Binary Integrity Verification

AWF verifies the SHA-256 checksum of each plugin binary before launching it. This prevents execution of corrupted or tampered binaries.

**When verification happens:**
- For plugins installed via `awf plugin install`: checksum is verified automatically at runtime using the stored value from install time
- For manually placed plugins: use `awf plugin verify --update` to enable checksum verification

**When verification is skipped:**
- Plugins without a stored checksum launch with a warning recommending checksum verification
- This allows existing plugins installed before this feature to continue functioning

**Example: Detecting a Tampered Plugin**

```bash
# After installing a plugin, its checksum is stored
$ awf plugin install myorg/awf-plugin-jira
✓ Installed awf-plugin-jira v1.2.0

# If the binary is modified later (e.g., by disk corruption or supply chain attack)
$ echo "malware" >> ~/.local/share/awf/plugins/awf-plugin-jira/awf-plugin-jira
$ awf run my-workflow
Error: plugin "awf-plugin-jira" checksum mismatch
  Expected: a3f9d4c5e8b2f1g7h8i9j0k1l2m3n4o5p
  Actual:   x1y2z3a4b5c6d7e8f9g0h1i2j3k4l5m6n7

# The plugin is refused and workflow execution stops
```

#### Plugin Output Forwarding

Plugin logs and subprocess output (stdout/stderr) are forwarded to AWF's log output with structured context. This aids debugging when plugins crash or behave unexpectedly.

**Plugin sources:**
- Plugin-emitted structured logs (via hclog)
- Plugin panic output
- Direct writes to stdout/stderr

**Log level:** Plugin output is forwarded at the INFO level for structured logs and WARN level for panic/output capture. AWF's configured log level (e.g., `--quiet`, `--verbose`) filters what appears in the final output.

**Example:**

```bash
$ awf run workflow --verbose
[INFO] Starting plugin awf-plugin-metrics...
[INFO] plugin=awf-plugin-metrics: Listening on port 50051
[INFO] plugin=awf-plugin-metrics: Registered collectors: cpu, memory, disk
[WARN] plugin=awf-plugin-jira: Deprecated API v2 used — upgrade to v3 recommended
```

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

> **Two ways to invoke an operation.** Beyond the deterministic `operation:` step shown above, plugin operations can also be exposed to AI agents at runtime through the [MCP proxy](mcp-proxy.md). With `mcp_proxy.plugin_tools`, the agent receives the operation as a callable MCP tool named `<plugin>_<operation>` (single underscore, snake_case) and decides when to invoke it. Plugin authors who want their operation to be agent-callable should review the schema constraints in [Exposing Operations as MCP Tools](#exposing-operations-as-mcp-tools).

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

### First Plugin Scaffold

Start new operation plugins with `awf plugin init` instead of copying an example by hand:

```bash
awf plugin init awf-plugin-example --kind operation
cd awf-plugin-example
make test
make install-local
awf plugin enable awf-plugin-example
awf plugin list --operations
awf run examples/demo.yaml
```

The generated `main.go` embeds `sdk.BasePlugin`, calls `sdk.Serve()`, implements `Operations()` and `HandleOperation()`, and exposes operation metadata through `OperationSchemaProvider`. The generated tests cover successful execution, structured operation errors, schema metadata, and manifest validation.

Use the generated repository as the supported authoring baseline. The checked-in examples remain useful for learning additional SDK surfaces: `awf-plugin-echo` mirrors the operation template, `awf-plugin-database` is the future step-type seed, `awf-plugin-security-validator` is the future validator seed, and `awf-plugin-event-logger` is the future event-listener seed.

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
| `sdk.OperationSchemaProvider` | Optional interface for input/output metadata used by docs, validation, and MCP exposure |
| `sdk.EventSubscriber` | Interface for receiving events (`Patterns()` + `HandleEvent()`) |
| `sdk.Event` | SDK event struct with ID, Type, Source, Metadata, Payload |

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

### Plugin Events

Implement `sdk.EventSubscriber` to react to workflow lifecycle events and events emitted by other plugins. This enables real-time notifications, metrics collection, audit logging, and inter-plugin communication.

**Event Subscriber Interface:**

```go
type EventSubscriber interface {
    Patterns() []string                          // Event types to subscribe to (glob patterns)
    HandleEvent(ctx context.Context, event Event) ([]Event, error)  // Handle incoming event
}
```

**Available Events:**

Plugins can subscribe to core workflow lifecycle events emitted by the AWF ExecutionService:

| Event Type | Description | Metadata |
|------------|-------------|----------|
| `workflow.started` | Workflow execution started | `workflow_id`, `workflow_name` |
| `workflow.completed` | Workflow completed successfully | `workflow_id`, `workflow_name`, `duration` |
| `workflow.failed` | Workflow failed | `workflow_id`, `workflow_name`, `error_message` |
| `step.started` | Step execution started | `workflow_id`, `step_name` |
| `step.completed` | Step completed | `workflow_id`, `step_name` |
| `step.failed` | Step failed | `workflow_id`, `step_name`, `error_message` |
| `step.retrying` | Step retrying after failure | `workflow_id`, `step_name`, `attempt` |

Plugins can also emit custom events that other plugins subscribe to (e.g., `deploy.completed`, `notification.sent`).

**Event Subscriber Example:**

```go
package main

import (
    "context"
    "log"

    "github.com/awf-project/cli/pkg/plugin/sdk"
)

type NotificationPlugin struct {
    sdk.BasePlugin
}

// Patterns declares which event types this plugin subscribes to
// Supports glob patterns: workflow.* matches all workflow events
func (p *NotificationPlugin) Patterns() []string {
    return []string{"workflow.completed", "workflow.failed"}
}

// HandleEvent is called when a matching event occurs
func (p *NotificationPlugin) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
    log.Printf("Workflow %s %s\n", event.Metadata["workflow_id"], event.Type)
    
    // Plugins can emit events that other plugins will receive
    if event.Type == "workflow.completed" {
        return []sdk.Event{
            {
                Type:   "notification.sent",
                Source: p.PluginName,
                Metadata: map[string]string{
                    "channel": "slack",
                    "status":  "success",
                },
            },
        }, nil
    }
    
    return nil, nil
}

func main() {
    sdk.Serve(&NotificationPlugin{
        BasePlugin: sdk.BasePlugin{
            PluginName:    "awf-plugin-notify",
            PluginVersion: "1.0.0",
        },
    })
}
```

**Plugin Manifest Declaration:**

Declare event subscriptions and emissions in `plugin.yaml`:

```yaml
name: awf-plugin-notify
version: 1.0.0
description: Sends notifications on workflow events
awf_version: ">=0.7.0"

capabilities:
  - events

events:
  subscribe:
    - "workflow.*"      # Subscribe to all workflow events
    - "step.failed"     # Subscribe to failed steps
  emit:
    - "notification.sent"    # Emit custom events
    - "notification.failed"
```

**Pattern Matching:**

Event patterns use glob matching with `.` as segment separator:

| Pattern | Matches | Does NOT Match |
|---------|---------|----------------|
| `workflow.started` | Exact match | `workflow.completed` |
| `workflow.*` | `workflow.started`, `workflow.completed`, `workflow.failed` | `workflow.step.started` |
| `step.*` | All step events | `workflow.step.completed` |
| `*.*` | All two-segment events | `workflow` (single segment) |
| `*` | Single-segment events only | — |

**Back-Pressure & Isolation:**

- Each plugin receives events on its own buffered channel (256-event capacity)
- Slow plugins don't block event delivery to other plugins
- If a plugin's buffer fills, events are dropped with a warning logged
**Cycle Detection:**

AWF prevents event loops by limiting propagation depth to 3 levels. If Plugin A emits an event that triggers Plugin B, which emits an event triggering Plugin A, propagation stops at depth 3 and a warning is logged.

---

### Exposing Operations as MCP Tools

AWF's [MCP proxy](mcp-proxy.md) (`mcp_proxy.plugin_tools` in a workflow step) re-exposes a plugin's operations as MCP tools, letting an AI agent invoke them directly during execution. Your plugin doesn't have to opt in or implement a new interface — every operation registered via `Operations()` is automatically eligible — **provided its schema satisfies the constraints below.**

#### Schema constraints

The MCP tool schema is derived from your operation's `OperationSchema` via the `MapOperationSchema` translator. Only scalar input types are allowed:

| `OperationSchema.Inputs[].Type` | Eligible? | Notes |
|---------------------------------|-----------|-------|
| `string` | ✅ | Translates to `{"type": "string"}` |
| `integer` | ✅ | Translates to `{"type": "integer"}` |
| `boolean` | ✅ | Translates to `{"type": "boolean"}` |
| `array` | ❌ | Rejected with `USER.MCP_PROXY.UNSUPPORTED_SCHEMA` at step startup |
| `object` | ❌ | Rejected with `USER.MCP_PROXY.UNSUPPORTED_SCHEMA` at step startup |

If an operation needs structured input (a list of items, a nested config), it can still be invoked as a workflow `operation:` step — but it cannot be exposed to agents via the MCP proxy until the schema is refactored to scalar fields or split into multiple smaller operations.

Two `Validation` values are forwarded to the JSON Schema `format` field, which most MCP-aware models honor: `"url"` → `"uri"`, `"email"` → `"email"`. Other `Validation` values are accepted by AWF but not propagated to the MCP tool schema.

#### Tool name

The exposed tool name is `<plugin>_<operation>` (single underscore separator, snake_case) — for example, `awf-plugin-time.time` becomes the MCP tool `awf-plugin-time_time`. Pick operation names that read well in this form: `create_issue`, `kubectl_apply`, `query_db`. Dots in operation names are forbidden because the Claude MCP client rejects them; AWF validates this at workflow load time.

#### Description seen by the agent

The agent sees a description composed from two fields of your `OperationSchema`:

```
<Description>. Returns a JSON object with fields: <Outputs joined by ", ">.
```

Concretely:

| Schema field | Agent-visible result |
|--------------|----------------------|
| `Description: "Returns the current UTC time."` + `Outputs: ["unix", "iso8601", "rfc3339"]` | `Returns the current UTC time. Returns a JSON object with fields: unix, iso8601, rfc3339.` |
| `Description: ""` + `Outputs: ["unix"]` | `Operation 'time' from plugin 'awf-plugin-time'. Returns a JSON object with fields: unix.` |
| `Description: "Fetches an issue."` + `Outputs: []` | `Fetches an issue.` |

**Practical takeaway** for plugin authors who want good agent-tool ergonomics:
- Always populate `Description` with a single sentence stating what the operation does.
- Populate `Outputs` with the field names the agent will read from the result (e.g. `["url", "title", "body"]` for `github.get_issue`). Models perform much better at multi-step reasoning when they know the output shape up front.

#### Minimal MCP-ready operation

```go
package main

import (
    "context"
    "time"

    "github.com/awf-project/cli/pkg/plugin/sdk"
)

type TimePlugin struct {
    sdk.BasePlugin
}

func (p *TimePlugin) Operations() []string {
    return []string{"time"}
}

func (p *TimePlugin) OperationSchema(name string) *sdk.OperationSchema {
    if name != "time" {
        return nil
    }
    return &sdk.OperationSchema{
        Description: "Returns the current UTC time as Unix epoch seconds and ISO-8601.",
        Inputs:      map[string]sdk.InputSpec{}, // no inputs
        Outputs:     []string{"unix", "iso8601"},
    }
}

func (p *TimePlugin) HandleOperation(_ context.Context, _ string, _ map[string]any) (*sdk.OperationResult, error) {
    now := time.Now().UTC()
    return sdk.NewSuccessResult("", map[string]any{
        "unix":    now.Unix(),
        "iso8601": now.Format(time.RFC3339),
    }), nil
}

func main() {
    sdk.Serve(&TimePlugin{
        BasePlugin: sdk.BasePlugin{PluginName: "awf-plugin-time", PluginVersion: "1.0.0"},
    })
}
```

Users then expose it to an agent like so:

```yaml
agent_with_time:
  type: agent
  provider: claude
  prompt: "Use the awf-plugin-time_time tool to read the current UTC time, then ..."
  mcp_proxy:
    enable: true
    intercept_builtins: false
    plugin_tools:
      - plugin: awf-plugin-time
        expose:
          - time
  options:
    dangerously_skip_permissions: true
```

#### Validation at workflow load

When a workflow references `plugin_tools: [{plugin: P, expose: [op]}]`, AWF emits these errors at `awf validate` / `awf run` time, before the agent ever starts:

| Error code | Cause |
|------------|-------|
| `USER.MCP_PROXY.UNKNOWN_PLUGIN` | Plugin `P` is not installed or not enabled |
| `USER.MCP_PROXY.UNKNOWN_OPERATION` | Operation `op` is not in `P.Operations()` |
| `USER.MCP_PROXY.UNSUPPORTED_SCHEMA` | One of `op`'s `Inputs` uses `array` or `object` |
| `USER.MCP_PROXY.NAME_COLLISION` | Two `expose:` entries (across plugins or with a built-in tool) resolve to the same MCP tool name |

Test these paths in your plugin's CI by running a workflow that exposes each operation under `plugin_tools` against a Claude or Gemini provider. The repo includes reference workflows at `.awf/workflows/test-mcp-proxy-{claude,gemini,opencode}-plugin-tools.yaml` that you can adapt for your plugin.

---

### Echo Plugin Example

The `examples/plugins/awf-plugin-echo/` directory contains a complete working plugin that echoes its input text. Use it as a starting point:

```bash
cd examples/plugins/awf-plugin-echo
make install          # Build and install to ~/.local/share/awf/plugins/
awf plugin enable awf-plugin-echo
```

Use it in a workflow by referencing the runtime plugin id:

```yaml
echo_step:
  type: operation
  operation: echo.echo
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

### Generated Release Workflow

Repositories created by `awf plugin init` include package and checksum targets plus `.github/workflows/release.yml`.

```bash
make package
make checksums
```

The generated workflow builds the plugin, packages the binary plus `plugin.yaml`, and publishes `.tar.gz` archives with SHA-256 checksums. Keep the generated asset pattern unless you also update AWF installer compatibility tests.

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

- [Plugin Events](plugin-events.md) - Event subscriptions, inter-plugin communication, and pattern matching
- [MCP Proxy](mcp-proxy.md) - Exposing plugin operations as MCP tools for AI agents
- [Commands](commands.md) - CLI command reference
- [Workflow Syntax](workflow-syntax.md) - Operation usage in workflows
- [Architecture](../development/architecture.md) - Plugin system internals
