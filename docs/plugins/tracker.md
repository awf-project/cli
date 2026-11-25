# Tracker operations

Use `tracker.*` to create, update, transition, comment on, retrieve, and search
issues from AWF workflows. The built-in tracker facade delegates each operation
to a separately installed and enabled adapter.

## Configure a backend

Set the default backend in your project's `.awf/config.yaml`:

```yaml
tracker:
  default_backend: jira
```

Use the short backend name. For example, `jira` selects the plugin
`tracker-jira`, which must expose the requested operation, such as
`tracker-jira.create_issue`. A step can override the project default with its
`backend` input.

At execution time, AWF reports an error if no backend is selected or if the
selected adapter is unavailable or lacks the requested operation. Workflow
validation can succeed without an installed adapter.

## Operations

Each operation accepts an optional `backend` when a project default is
configured. Operations that target an existing issue require at least one
non-empty identifier. `id` is the provider's identifier and `key` is the
human-readable issue key. You can supply both.

| Operation | Required inputs | Other optional inputs | Outputs |
| --- | --- | --- | --- |
| `tracker.create_issue` | `title` | `project`, `team`, `type`, `description`, `priority`, `assignee`, `labels` | `id`, `key`, `url` |
| `tracker.update_issue` | `id` or `key` | `title`, `description`, `priority`, `assignee`, `state`, `labels` | `id`, `key` |
| `tracker.transition` | `id` or `key`, and `transition` | — | `id`, `key`, `status` |
| `tracker.add_comment` | `id` or `key`, and `body` | — | `comment_id` |
| `tracker.get_issue` | `id` or `key` | — | `id`, `key`, `title`, `status`, `description`, `assignee`, `url` |
| `tracker.search` | `query` | `project`, `team`, `state`, `max_results` | `issues`, `total` |

For `tracker.get_issue` and each element of `tracker.search`'s `issues` array, adapters must return `id`, `key`, `title`, `status`, `description`, `assignee`, and `url` as strings. Use an empty string for a missing description or assignee. Omitted fields and `null` values are rejected.

`labels` is an array and `max_results` is an integer. The remaining inputs
listed above are strings. Search returns an array of issues and a total count.
Use query and transition values supported by the selected adapter.

## Example workflow

This example uses the project default and requires an enabled adapter that
supports `create_issue`.

```yaml
name: create-tracker-issue
version: "1.0.0"
states:
  initial: create
  create:
    type: operation
    operation: tracker.create_issue
    inputs:
      project: PLATFORM
      title: "Investigate missing export"
      description: "The export completed without creating a file."
    on_success: done
  done:
    type: terminal
    status: success
```

To override the project default, add `backend` with the short adapter name under
the step's `inputs`.
