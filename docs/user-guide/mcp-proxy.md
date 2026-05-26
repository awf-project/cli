---
title: \"MCP Proxy — Tool Interception and Plugin Tool Exposure\"
---

## Overview

MCP Proxy intercepts tool calls from AI agents and routes them through an AWF-controlled local MCP server. This enables:

- **Tool call observability**: Every tool invocation by an agent produces an OpenTelemetry span and structured log line for auditing and monitoring
- **Built-in tool re-exposure**: AWF's 6 core tools (`Read`, `Write`, `Edit`, `Bash`, `Glob`, `Grep`) are re-exposed through the MCP protocol, giving you introspection into all file and shell operations
- **Plugin tool exposure**: Extend the agent's available tools with custom operations from installed AWF plugins without modifying the plugin interface

The proxy is configured per-step via the `mcp_proxy:` block in your workflow YAML.

## Why Use MCP Proxy?

**Observability**: Without the proxy, agent tool calls are opaque — you see only the final output. With the proxy, you get:
- OTel spans per tool call (child of the step span)
- Structured `zap` logs with tool name, source, duration, and errors
- Integration with your telemetry backend (Jaeger, Grafana Tempo, Honeycomb)

**Extensibility**: Expose gRPC plugin operations as MCP tools alongside the built-ins:
```yaml
mcp_proxy:
  enable: true
  plugin_tools:
    - plugin: kubernetes
      expose: [kubectl_apply, kubectl_get]
```

**Security & Control**: Choose between full interception (only MCP tools available) or additive mode (native built-ins + MCP tools):
```yaml
mcp_proxy:
  enable: true
  intercept_builtins: true   # Full control: agent uses ONLY MCP tools
  # vs.
  intercept_builtins: false  # Additive: native built-ins + MCP tools
```

## Configuration

### Basic Syntax

Add the `mcp_proxy:` block to any agent step:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Analyze this file: {{.inputs.file}}"
  mcp_proxy:
    enable: true
    intercept_builtins: true
  options:
    model: claude-sonnet-4-20250514
  on_success: done
```

### Schema

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enable` | boolean | `false` | Activate the proxy on this step |
| `intercept_builtins` | boolean | `true` | If `true`, agent sees ONLY MCP tools (built-ins + plugins). If `false`, agent sees native built-ins + MCP tools. |
| `plugin_tools` | array | `[]` | List of plugins and operations to expose as MCP tools |
| `plugin_tools[].plugin` | string | — | Plugin name (must exist in `.awf/plugins.yaml`) |
| `plugin_tools[].expose` | array | — | Operations from that plugin to expose as MCP tools |

### Examples

#### Example 1: Full Interception (Built-ins Only)

Pure observability — every `Read`, `Write`, `Edit`, `Bash`, `Glob`, `Grep` call flows through the proxy.

```yaml
refactor:
  type: agent
  provider: claude
  prompt: "Refactor src/main.go for clarity"
  mcp_proxy:
    enable: true
    # intercept_builtins: true is the default
  options:
    model: claude-sonnet-4-20250514
  on_success: done
```

Agent sees: `Read`, `Write`, `Edit`, `Bash`, `Glob`, `Grep`.
All calls are logged and traced.

#### Example 2: Full Interception + Plugin Tools

Agent sees built-ins plus custom plugin operations.

```yaml
deploy:
  type: agent
  provider: claude
  prompt: "Apply the Kubernetes manifest to staging"
  mcp_proxy:
    enable: true
    plugin_tools:
      - plugin: kubernetes
        expose: [kubectl_apply, kubectl_get, kubectl_describe]
  options:
    model: claude-sonnet-4-20250514
  timeout: 300
  on_success: verify
```

Agent sees: `Read`, `Write`, `Edit`, `Bash`, `Glob`, `Grep`, `kubernetes_kubectl_apply`, `kubernetes_kubectl_get`, `kubernetes_kubectl_describe`.

Plugin tool names are prefixed with `<plugin>_` to avoid collisions.

> **Naming convention.** Built-in tools intentionally use **PascalCase** (`Read`, `Write`,
> `Edit`, `Bash`, `Glob`, `Grep`) to align with the names Anthropic-class agents
> (Claude Code, OpenCode) emit in their `tool_use` events. Plugin tools use
> **snake_case** with a `<plugin>_<operation>` prefix. This is the only deliberate
> exception to the snake_case convention documented in ADR 017, and it keeps the proxy
> a drop-in replacement for the agent's native tools.

#### Example 3: Additive Mode (Native Built-ins + Plugin Tools)

Keep the agent's native tools and add plugin operations alongside.

```yaml
notify:
  type: agent
  provider: claude
  prompt: "Send a deployment notification"
  mcp_proxy:
    enable: true
    intercept_builtins: false
    plugin_tools:
      - plugin: notify
        expose: [send_slack, send_webhook]
  options:
    model: claude-sonnet-4-20250514
  on_success: done
```

Agent sees: all native built-ins (without MCP tracing) + `notify_send_slack`, `notify_send_webhook` (with tracing).

## Built-in Tools

When `mcp_proxy.enable: true`, the following tools are available to the agent (unless `intercept_builtins: false`):

### Read

Read a file and return its contents.

```yaml
# Agent call
tool: Read
args:
  path: "/etc/config.yaml"

# Returns
content: "..."
```

### Write

Write contents to a file. Creates parent directories if needed.

```yaml
tool: Write
args:
  path: "/tmp/output.txt"
  content: "File content here"

# Returns
path: "/tmp/output.txt"
success: true
```

### Edit

Edit a file using regex-based find-and-replace.

```yaml
tool: Edit
args:
  path: "/src/main.go"
  old_string: "// TODO: fix this"
  new_string: "// FIXED in v2.0"

# Returns
path: "/src/main.go"
success: true
```

### Bash

Execute a shell command.

```yaml
tool: Bash
args:
  command: "ls -la /home"
  working_dir: "/tmp"

# Returns
stdout: "..."
stderr: ""
exit_code: 0
```

### Glob

Find files matching a pattern.

```yaml
tool: Glob
args:
  pattern: "**/*.go"
  directory: "."

# Returns
matches: ["main.go", "cmd/cli.go", "internal/pkg.go"]
```

### Grep

Search for text in files.

```yaml
tool: Grep
args:
  pattern: "TODO"
  path: "./src"
  context_lines: 2

# Returns
matches:
  - file: "src/main.go"
    line: 42
    text: "// TODO: implement this"
```

## Plugin Tools

When you expose plugin operations via `plugin_tools:`, they become available as MCP tools with the naming pattern `<plugin>_<operation>`.

### Exposure

List the operations you want to expose:

```yaml
plugin_tools:
  - plugin: github
    expose:
      - create_issue
      - add_comment_to_pr
      - list_pull_requests
```

### Tool Schema

Plugin tools are automatically converted from the plugin's `OperationSchema` to MCP's `InputSchema` (JSON Schema). For example:

**Plugin operation schema:**
```go
OperationSchema{
  Name: "create_issue",
  Inputs: InputSchema{
    Type: "object",
    Required: []string{"title", "body"},
    Properties: map[string]any{
      "title": {Type: "string"},
      "body": {Type: "string"},
      "labels": {Type: "array"},
    },
  },
}
```

**Becomes MCP tool schema:**
```json
{
  "name": "github_create_issue",
  "description": "Create a GitHub issue",
  "inputSchema": {
    "type": "object",
    "required": ["title", "body"],
    "properties": {
      "title": {"type": "string"},
      "body": {"type": "string"},
      "labels": {"type": "array"}
    }
  }
}
```

## Supported Providers

MCP Proxy works with all six AWF agent providers:

| Provider | Mechanism | Interception Mode | Notes |
|----------|-----------|-------------------|-------|
| **claude** | `--mcp-config` flag | Full control (intercept_builtins:true enforced) | MCP-only isolation guaranteed |
| **gemini** | `--mcp-server` flag | Full control | MCP-only isolation guaranteed |
| **codex** | `-c 'mcp_servers.awf-proxy'` | Coexistence (⚠️ see below) | Native tools remain accessible; startup warning emitted |
| **opencode** | `opencode mcp add` | Coexistence (⚠️ see below) | Native tools remain accessible; startup warning emitted |
| **github_copilot** | `--additional-mcp-config @<file>` (+ `--disable-builtin-mcps` in intercept mode) | Coexistence (⚠️ see below) | Native tools remain accessible; startup warning emitted |
| **openai_compatible** | HTTP `tools[]` field | Full control | MCP tools injected in Chat Completions request |

### Codex, OpenCode & Copilot Coexistence Warning

Codex, OpenCode and GitHub Copilot CLIs cannot fully disable their native built-in tools — they lack a `--tools ""` equivalent. When you use `mcp_proxy.enable: true` on these providers, AWF:

1. Injects the proxy MCP server
2. Emits a startup **`WARN`** log message:
   ```
   WARN: mcp_proxy on provider=codex runs in coexistence mode.
         Built-in tools cannot be disabled and may bypass the proxy.
         Use 'claude' or 'openai-compatible' for guaranteed MCP-only isolation.
   ```
3. Adds system prompt mitigation ("Use only MCP tools, never built-in tools")

**If you need strict MCP-only isolation**, use `claude` or `openai_compatible` instead.

## Validation

`awf validate` checks the `mcp_proxy:` block for configuration errors:

| Error Code | Condition | Example |
|------------|-----------|---------|
| `USER.MCP_PROXY.UNKNOWN_KEY` | Unknown key in the block (typo, future schema) | `intercept_builtins_future: true` |
| `USER.MCP_PROXY.UNKNOWN_PLUGIN` | Plugin does not exist in `.awf/plugins.yaml` | `plugin: nonexistent_plugin` |
| `USER.MCP_PROXY.UNKNOWN_OPERATION` | Operation not found in the plugin | `expose: [invalid_op]` |
| `USER.MCP_PROXY.NAME_COLLISION` | Two tools resolve to the same name | Two plugins with `create_issue` operation |
| `USER.MCP_PROXY.EMPTY_PROXY` | `enable: true` + `intercept_builtins: false` + no plugins | Dead config with no effect |

Example validation output:

```bash
$ awf validate my-workflow.yaml

Error: USER.MCP_PROXY.UNKNOWN_PLUGIN
Step: deploy
Details: plugin 'k8s' not found in .awf/plugins.yaml
```

## Observability

### OpenTelemetry Spans

Each tool call produces a child span of the step span:

```
workflow.execution
  └─ step: deploy
      └─ tool.call: kubernetes_kubectl_apply
          ├─ Duration: 2.341s
          ├─ tool.name: kubernetes_kubectl_apply
          ├─ tool.source: plugin:kubernetes
          └─ [Error]: command timeout
```

Attributes available for export to your telemetry backend:
- `tool.name` — Name of the tool
- `tool.source` — `builtin` or `plugin:<name>`
- `tool.duration_ms` — Duration in milliseconds
- Error information (if the call failed)

### Structured Logging

Each tool call produces a zap log entry:

```json
{
  "level": "info",
  "message": "tool call",
  "tool": "Read",
  "source": "builtin",
  "duration_ms": 12,
  "timestamp": "2026-05-23T10:30:45Z"
}
```

If the call fails:

```json
{
  "level": "error",
  "message": "tool call",
  "tool": "Bash",
  "source": "builtin",
  "duration_ms": 456,
  "error": "command exited with code 127: command not found",
  "timestamp": "2026-05-23T10:30:46Z"
}
```

## Performance Considerations

- **Per-step overhead**: Each step with `mcp_proxy.enable: true` spawns a subprocess (~10 MB memory) for stdio providers (Claude, Gemini, Codex, OpenCode). The subprocess is cleaned up when the step completes.
- **Tool call latency**: MCP tool calls add ~50ms overhead (process communication) compared to direct agent calls. This is negligible for most workflows.
- **Tracing cost**: OTel spans and structured logging have zero cost when no telemetry exporter is configured (default behavior).

## Troubleshooting

### "mcp_proxy on provider=codex runs in coexistence mode" warning

This is expected behavior for Codex and OpenCode. They cannot disable native tools via CLI flags. Options:

1. **Accept the warning** and understand that native tools may be called (though the system prompt discourages this)
2. **Switch to Claude or OpenAI Compatible** for guaranteed MCP-only isolation
3. **Use `intercept_builtins: false`** to intentionally add plugin tools alongside native ones

### "NAME_COLLISION detected at step startup"

Two tools (built-ins or plugins) resolved to the same name. Examples:

- Two plugins both have a `create_issue` operation (both become `<plugin>_create_issue`)
- A plugin has a `read` operation (collides with built-in `Read`)

**Fix**: Rename one of the operations in the plugin, or remove one from `expose:`.

### "UNKNOWN_OPERATION in plugin"

The plugin does not expose the operation you're trying to expose. Verify:

1. Run `awf plugin list <name>` to see available operations
2. Check the plugin's documentation
3. Correct the operation name in `expose:`

### Tool call takes longer than expected

Check your telemetry backend or logs for the tool call duration. Possible causes:

- The underlying command is slow (not proxy-related)
- Network latency (for HTTP providers)
- High system load

### Proxy subprocess crashes or hangs

AWF automatically detects subprocess failure and reports it as a structured error. If it hangs:

1. Press `Ctrl+C` to interrupt (the proxy subprocess will be forcefully terminated after a 5-second grace period)
2. Check logs for error details
3. Report the issue with the full AWF log output

## Examples

### Example: Code Review with Full Observability

```yaml
name: code-review-with-tracing
version: "1.0.0"

inputs:
  - name: file
    type: string
    required: true

states:
  initial: analyze

  analyze:
    type: agent
    provider: claude
    prompt: |
      Review this code for bugs, security issues, and style:
      
      {{.inputs.file}}
    mcp_proxy:
      enable: true
      # All file reads and shell commands flow through MCP,
      # producing OTel spans and structured logs
    options:
      model: claude-sonnet-4-20250514
    on_success: report

  report:
    type: step
    command: echo "Code review complete. Check logs for observability data."
    on_success: done

  done:
    type: terminal
    status: success
```

Run with tracing enabled:

```bash
awf run code-review-with-tracing \
  --input file=src/main.go \
  --otel-exporter otlp \
  --otel-service-name my-workflow
```

### Example: K8s Deployment with Plugin Tools

```yaml
name: deploy-to-k8s
version: "1.0.0"

inputs:
  - name: manifest
    type: string
    required: true
  - name: namespace
    type: string
    default: default

states:
  initial: validate

  validate:
    type: step
    command: kubectl --version
    on_success: deploy

  deploy:
    type: agent
    provider: claude
    prompt: |
      Apply this Kubernetes manifest to {{.inputs.namespace}}:
      
      {{.inputs.manifest}}
      
      First validate with kubectl apply --dry-run, then apply for real.
    mcp_proxy:
      enable: true
      plugin_tools:
        - plugin: kubernetes
          expose: [kubectl_apply, kubectl_get, kubectl_describe]
    options:
      model: claude-sonnet-4-20250514
    timeout: 300
    on_success: verify

  verify:
    type: step
    command: kubectl get all -n {{.inputs.namespace}}
    on_success: done

  done:
    type: terminal
    status: success
```

### Example: Additive Mode (Native + Plugin)

```yaml
notify_deployment:
  type: agent
  provider: claude
  prompt: "Send a deployment summary to Slack"
  mcp_proxy:
    enable: true
    intercept_builtins: false
    # Agent uses native Read/Write/Bash (not traced)
    # + notify_send_slack (traced)
    plugin_tools:
      - plugin: notify
        expose: [send_slack, send_email]
  options:
    model: claude-haiku-4-5
  on_success: done
```

## See Also

- [Agent Steps](agent-steps.md) — Full agent step reference
- [OpenTelemetry Tracing](tracing.md) — Configure telemetry export
- [Plugins](plugins.md) — Install and manage plugins
- [Error Codes](../reference/error-codes.md) — USER.MCP_PROXY.* codes
