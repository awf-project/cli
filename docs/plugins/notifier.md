---
title: "Notifier Plugin Reference"
---

# Notifier Plugin Reference

The provider-neutral `notifier` facade lets workflows send conversational notifications without coupling them to Slack, Discord, or another channel-specific implementation. Concrete adapters are discovered as external plugins and are addressed by their backend name.

This facade is separate from the legacy `notify` provider. Existing `notify.send` workflows continue to work; use `notifier.*` when an adapter-backed, provider-neutral contract is required.

## Operations

| Operation | Purpose | Required inputs | Outputs |
|-----------|---------|-----------------|---------|
| `notifier.send` | Send a message to a channel | `channel` (unless a default is configured), `text` | `message_id`, `channel` |
| `notifier.update` | Update an existing message | `channel` (unless a default is configured), `message_id`, `text` | `message_id` |
| `notifier.react` | Add an emoji reaction | `channel` (unless a default is configured), `message_id`, `emoji` | `ok` |
| `notifier.upload` | Upload a file when supported by the adapter | `channel` (unless a default is configured), `filename`, `content` | `file_id`, optional `permalink` |

All operations also accept the following optional inputs where applicable:

| Input | Type | Description |
|-------|------|-------------|
| `backend` | string | Adapter runtime ID. If omitted, `notifier.backend` is used. |
| `rich` | object | Provider-neutral rich payload passed unchanged to the adapter. |
| `thread_id` | string | Existing conversation thread for `send`. |
| `title` | string | Optional upload title. |

The facade validates inputs before delegation. `backend` must be a non-empty, undotted adapter ID and cannot be `notifier`. A channel is required unless a non-blank `notifier.default_channel` is configured; an explicitly blank `channel` is rejected.

## Configuration

Configure the canonical facade at the top level of `.awf/config.yaml`:

```yaml
notifier:
  backend: slack
  default_channel: "#builds"
```

An operation-level `backend` or `channel` overrides the configured default. The selected adapter must be installed and expose the matching short operation (`send`, `update`, `react`, or `upload`). AWF delegates, for example, `notifier.send` with `backend: slack` to `slack.send`.

The legacy `notify.send` provider has separate settings. See [Plugin Configuration](../user-guide/configuration.md#plugin-configuration).

## Workflow example

```yaml
name: notify-review
version: "1.0.0"

inputs:
  - name: pr_url
    type: string
    required: true

states:
  initial: analyze

  analyze:
    type: agent
    provider: claude
    prompt: "Summarize the pull request: {{.inputs.pr_url}}"
    on_success: notify

  notify:
    type: operation
    operation: notifier.send
    inputs:
      text: "Review complete for {{.inputs.pr_url}}"
      rich:
        color: "#2da44e"
        fields:
          - name: status
            value: complete
    on_success: done

  done:
    type: terminal
    status: success
```

With the configuration above, the workflow sends through `slack.send` to `#builds`. To target another installed adapter for one operation, add `backend: discord` to that operation's inputs.

## Adapter requirements

An adapter plugin uses its runtime ID as the backend name and exposes short operation names. For example, a `slack` adapter should provide `slack.send`, `slack.update`, and `slack.react`; `slack.upload` is optional. Adapter-specific payload translation belongs inside the adapter, while `rich` remains unchanged at the notifier boundary.

If an adapter or operation is unavailable, AWF reports the requested qualified operation and the sorted list of discovered backend IDs. Adapter errors and caller cancellation are propagated without replacing the caller's context.

## Related documentation

- [Workflow Syntax - Notification Operations](../user-guide/workflow-syntax.md#notification-operations)
- [Plugins](../user-guide/plugins.md)
- [Configuration](../user-guide/configuration.md#plugin-configuration)
