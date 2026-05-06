# awf-plugin-event-logger

Example AWF plugin demonstrating the `sdk.EventSubscriber` pattern. Subscribes to workflow and step lifecycle events, writes them to `.awf/logs/event-logger.log` in the project directory, and emits a `logger.workflow_failed` summary event when a workflow fails.

## Build and Install

```bash
make install
```

This builds the plugin binary and installs it alongside `plugin.yaml` into `~/.local/share/awf/plugins/awf-plugin-event-logger/`.

## Event Subscriptions

| Pattern | Matches |
|---------|---------|
| `workflow.*` | `workflow.started`, `workflow.completed`, `workflow.failed` |
| `step.*` | `step.started`, `step.completed`, `step.failed`, `step.retrying` |

## Emitted Events

| Type | When | Metadata |
|------|------|----------|
| `logger.workflow_failed` | After receiving `workflow.failed` | `original_workflow`, `failure_reason`, `logged_at` |

Other plugins can subscribe to `logger.*` to react to failure summaries.

## Log Output

Events are written to `.awf/logs/event-logger.log` in the working directory where `awf run` is executed. The directory is created automatically.

```
2026-05-06T14:30:00Z | workflow.started       | source=core       | {"workflow_id":"wf-abc","workflow_name":"deploy"}
2026-05-06T14:30:01Z | step.started           | source=core       | {"workflow_id":"wf-abc","step_name":"build"}
2026-05-06T14:30:05Z | step.completed         | source=core       | {"workflow_id":"wf-abc","step_name":"build"}
2026-05-06T14:30:08Z | workflow.completed     | source=core       | {"workflow_id":"wf-abc","workflow_name":"deploy"}
```

> **Note:** Plugin processes run under go-plugin's gRPC bridge — their stdout is captured by the host runtime and not visible in the terminal. This is why the plugin writes to a file instead.

## Usage

```yaml
name: deploy
version: "1.0.0"

states:
  initial: build

  build:
    type: step
    command: make build
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
```

Enable the plugin, then run:

```bash
awf plugin enable awf-plugin-event-logger
awf run deploy
cat .awf/logs/event-logger.log
```

## Plugin Structure

```go
type EventLoggerPlugin struct {
    sdk.BasePlugin
    logFile *os.File
}

func (p *EventLoggerPlugin) Init(_ context.Context, _ map[string]any) error {
    p.logFile, _ = os.OpenFile(".awf/logs/event-logger.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
    return nil
}

func (p *EventLoggerPlugin) Patterns() []string {
    return []string{"workflow.*", "step.*"}
}

func (p *EventLoggerPlugin) HandleEvent(_ context.Context, event sdk.Event) ([]sdk.Event, error) {
    meta, _ := json.Marshal(event.Metadata)
    fmt.Fprintf(p.logFile, "%s | %-20s | source=%-10s | %s\n", event.Timestamp.Format(time.RFC3339), event.Type, event.Source, meta)

    if event.Type == "workflow.failed" {
        return []sdk.Event{{
            Type:   "logger.workflow_failed",
            Source: "event-logger",
            Metadata: map[string]string{
                "original_workflow": event.Metadata["workflow_id"],
                "failure_reason":   event.Metadata["error"],
            },
        }}, nil
    }
    return nil, nil
}
```

## Features Demonstrated

- **`sdk.EventSubscriber`** — Patterns() + HandleEvent() for receiving lifecycle events
- **File-based logging** — writes to `.awf/logs/` since plugin stdout is captured by go-plugin
- **Event emission** — returning events from HandleEvent() for inter-plugin routing
- **Glob patterns** — `workflow.*` matches `workflow.started` but not `workflow.step.started`
- **BasePlugin embedding** — override only Init/Shutdown/Patterns/HandleEvent
- **Plugin manifest** — `events` capability with `subscribe` and `emit` declarations
