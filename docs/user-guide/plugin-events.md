---
title: "Plugin Events"
---

Plugin Events enable real-time communication between AWF core and plugins, and between plugins themselves. This guide covers subscribing to workflow lifecycle events, emitting custom events, and building event-driven plugins.

## Overview

AWF emits structured events at each step of workflow execution:

```
awf run workflow
    ├─ workflow.started (core event)
    ├─ step.started     (core event)
    ├─ step.completed   (core event)
    ├─ step.started     (core event)
    ├─ step.completed   (core event)
    ├─ workflow.completed (core event)
    └─ [custom events from plugins]
```

Plugins can:
1. **Subscribe to core events** — react to workflow/step lifecycle (`workflow.started`, `step.failed`, etc.)
2. **Subscribe to custom events** — react to events from other plugins
3. **Emit custom events** — notify other plugins of plugin-specific milestones, either by returning events from `HandleEvent` or by calling `HostClient.Emit()` at any time during execution
4. **Use glob patterns** — subscribe to event families (`workflow.*`, `step.*`)
5. **Receive events via streaming** — opt into persistent gRPC streaming for lower-latency delivery (automatic fallback to unary RPCs)

All communication happens in real-time via gRPC without the plugin polling or managing connections. The host uses GRPCBroker to expose a reverse channel that plugins can use to emit events back at runtime.

## Subscribing to Events

### 1. Implement EventSubscriber

Add the `EventSubscriber` interface to your plugin:

```go
type NotificationPlugin struct {
    sdk.BasePlugin
    config map[string]string
}

// Patterns declares which events this plugin cares about
func (p *NotificationPlugin) Patterns() []string {
    return []string{
        "workflow.completed",  // Exact match
        "step.failed",        // Specific event
        "step.*",             // All step events
    }
}

// HandleEvent is invoked when a matching event occurs
func (p *NotificationPlugin) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
    switch event.Type {
    case "workflow.completed":
        return p.notifySuccess(ctx, event)
    case "workflow.failed":
        return p.notifyFailure(ctx, event)
    case "step.failed":
        return p.logError(ctx, event)
    }
    return nil, nil
}
```

### 2. Declare Events in Manifest

Update `plugin.yaml` to declare subscription patterns:

```yaml
name: awf-plugin-notify
version: 1.0.0
awf_version: ">=0.7.0"

capabilities:
  - events

events:
  subscribe:
    - "workflow.completed"
    - "workflow.failed"
    - "step.failed"
```

### 3. Test with `awf run`

Install your plugin and run a workflow:

```bash
make install
awf plugin enable awf-plugin-notify
awf run example-workflow
```

AWF will invoke `HandleEvent` on your plugin each time a matching event occurs.

## Core Events Reference

### Workflow Events

**`workflow.started`** — Emitted when workflow execution begins

```go
Event{
    ID:        "uuid-1234",
    Type:      "workflow.started",
    Source:    "core",
    Timestamp: time.Now(),
    Metadata: map[string]string{
        "workflow_id":   "wf-abc-123",
        "workflow_name": "deploy-app",
        "version":       "1.0.0",
    },
}
```

**`workflow.completed`** — Emitted when workflow succeeds

```go
Metadata: map[string]string{
    "workflow_id":   "wf-abc-123",
    "workflow_name": "deploy-app",
    "duration":      "45s",
    "status":        "success",
}
```

**`workflow.failed`** — Emitted when workflow fails

```go
Metadata: map[string]string{
    "workflow_id":    "wf-abc-123",
    "workflow_name":  "deploy-app",
    "error_message":  "step 'deploy' failed: exit code 1",
    "failed_step":    "deploy",
}
```

### Step Events

**`step.started`** — Emitted before step execution

```go
Metadata: map[string]string{
    "workflow_id": "wf-abc-123",
    "step_name":   "validate",
    "step_type":   "step",
}
```

**`step.completed`** — Emitted after successful step

```go
Metadata: map[string]string{
    "workflow_id": "wf-abc-123",
    "step_name":   "validate",
    "duration":    "2s",
}
```

**`step.failed`** — Emitted after failed step

```go
Metadata: map[string]string{
    "workflow_id":   "wf-abc-123",
    "step_name":     "validate",
    "error_message": "validation failed: missing version",
    "exit_code":     "1",
}
```

**`step.retrying`** — Emitted before retry attempt

```go
Metadata: map[string]string{
    "workflow_id":   "wf-abc-123",
    "step_name":     "deploy",
    "attempt":       "2",
    "max_attempts":  "3",
}
```

## Emitting Custom Events

Plugins can emit events in two ways:

1. **Via `HandleEvent` return value** — Emit events as a response to received events
2. **Via `HostClient`** — Emit events directly to the host at any time (F092+)

### Method 1: Return Events from HandleEvent

Emit events that other plugins subscribe to by returning them from `HandleEvent`:

### Example: Deploy Plugin → Notification Plugin

**Deploy Plugin** (awf-plugin-deploy):

```go
func (p *DeployPlugin) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
    if event.Type != "step.completed" {
        return nil, nil
    }
    
    if event.Metadata["step_name"] == "production-deployment" {
        // Emit custom event that notification plugin can react to
        return []sdk.Event{
            {
                Type:   "deploy.completed",
                Source: p.PluginName,  // "awf-plugin-deploy"
                Metadata: map[string]string{
                    "environment": "production",
                    "version":     "v2.1.0",
                    "status":      "success",
                },
            },
        }, nil
    }
    return nil, nil
}
```

**Notification Plugin** (awf-plugin-notify) subscribes:

```yaml
# plugin.yaml
events:
  subscribe:
    - "deploy.*"   # Matches "deploy.completed", "deploy.rolled_back", etc.
```

```go
func (p *NotificationPlugin) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
    if event.Type == "deploy.completed" {
        return []sdk.Event{
            {
                Type:   "notification.sent",
                Source: p.PluginName,
                Metadata: map[string]string{
                    "channel": "slack",
                    "recipient": "team-devops",
                },
            },
        }, nil
    }
    return nil, nil
}
```

## Pattern Matching Rules

Patterns use `.` as a segment separator. `*` matches one segment, not multiple:

| Pattern | Matches | Does NOT Match |
|---------|---------|----------------|
| `workflow.started` | `workflow.started` (exact) | `workflow.completed` |
| `workflow.*` | `workflow.started`, `workflow.completed`, `workflow.failed` | `workflow.step.started` |
| `step.*` | `step.started`, `step.completed`, `step.failed`, `step.retrying` | `step.database.query` |
| `*.*` | All two-segment events | One-segment events |
| `*` | One-segment events only | Multi-segment events |
| `deploy.*` | `deploy.completed`, `deploy.rolled_back` | `system.deploy.complete` |

**Subscribing to ALL events:**

```go
func (p *AuditPlugin) Patterns() []string {
    return []string{"*.*"}  // Matches all two-segment events
}
```

## Real-World Examples

### Audit Logger Plugin

Logs all workflow and step events to a file:

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/awf-project/cli/pkg/plugin/sdk"
)

type AuditPlugin struct {
    sdk.BasePlugin
    logFile *os.File
}

func (p *AuditPlugin) Patterns() []string {
    return []string{"*.*"}  // All events
}

func (p *AuditPlugin) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
    // Log to audit file
    entry := fmt.Sprintf("[%s] %s (source: %s) | %v\n",
        time.Now().Format(time.RFC3339),
        event.Type,
        event.Source,
        event.Metadata,
    )
    p.logFile.WriteString(entry)
    return nil, nil
}

func main() {
    f, _ := os.OpenFile("audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    sdk.Serve(&AuditPlugin{
        BasePlugin: sdk.BasePlugin{
            PluginName:    "awf-plugin-audit",
            PluginVersion: "1.0.0",
        },
        logFile: f,
    })
}
```

### Metrics Plugin

Counts events and exports to Prometheus:

```go
package main

import (
    "context"
    "sync"

    "github.com/awf-project/cli/pkg/plugin/sdk"
    "github.com/prometheus/client_golang/prometheus"
)

type MetricsPlugin struct {
    sdk.BasePlugin
    
    workflowsCompleted prometheus.Counter
    stepsFailed        prometheus.Counter
    mu                 sync.Mutex
}

func (p *MetricsPlugin) Patterns() []string {
    return []string{"workflow.*", "step.*"}
}

func (p *MetricsPlugin) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    switch event.Type {
    case "workflow.completed":
        p.workflowsCompleted.Inc()
    case "step.failed":
        p.stepsFailed.Inc()
    }
    
    return nil, nil
}

func main() {
    sdk.Serve(&MetricsPlugin{
        BasePlugin: sdk.BasePlugin{
            PluginName:    "awf-plugin-metrics",
            PluginVersion: "1.0.0",
        },
        workflowsCompleted: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "awf_workflows_completed_total",
        }),
        stepsFailed: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "awf_steps_failed_total",
        }),
    })
}
```

### Slack Notifier Plugin

Sends notifications to Slack channels:

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"

    "github.com/awf-project/cli/pkg/plugin/sdk"
)

type SlackNotifier struct {
    sdk.BasePlugin
    webhookURL string
}

func (p *SlackNotifier) Patterns() []string {
    return []string{"workflow.completed", "workflow.failed"}
}

func (p *SlackNotifier) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
    message := fmt.Sprintf(":wave: Workflow `%s` %s",
        event.Metadata["workflow_name"],
        event.Type,
    )
    
    if event.Type == "workflow.failed" {
        message = fmt.Sprintf(":x: Workflow `%s` failed: %s",
            event.Metadata["workflow_name"],
            event.Metadata["error_message"],
        )
    }
    
    payload := map[string]string{"text": message}
    data, _ := json.Marshal(payload)
    
    http.Post(p.webhookURL, "application/json", bytes.NewReader(data))
    return nil, nil
}

func main() {
    sdk.Serve(&SlackNotifier{
        BasePlugin: sdk.BasePlugin{
            PluginName:    "awf-plugin-slack",
            PluginVersion: "1.0.0",
        },
        webhookURL: os.Getenv("SLACK_WEBHOOK_URL"),
    })
}
```

### Method 2: Emit Directly via HostClient

For long-running operations, async work, or independent event emission, use `HostClient` to emit events directly to the host at any time:

```go
import (
    "context"
    "encoding/json"
    
    "github.com/awf-project/cli/pkg/plugin/sdk"
)

type AnalysisPlugin struct {
    sdk.BasePlugin
    hostClient *sdk.HostClient
}

// Plugin receives broker connection during initialization
func (p *AnalysisPlugin) SetHostClient(client *sdk.HostClient) {
    p.hostClient = client
}

func (p *AnalysisPlugin) Operation(ctx context.Context, req *sdk.OperationRequest) (*sdk.OperationResponse, error) {
    // Do analysis work...
    result := analyzeCode(req.Input)
    
    // Emit event directly via HostClient (doesn't wait for HandleEvent call)
    if p.hostClient != nil {
        payload, _ := json.Marshal(map[string]any{
            "file":     req.Input,
            "severity": result.Severity,
            "issues":   len(result.Issues),
        })
        
        p.hostClient.Emit(ctx, "analysis.complete", payload, map[string]string{
            "status": result.Status,
        })
    }
    
    return &sdk.OperationResponse{Output: result.Summary}, nil
}

func main() {
    sdk.Serve(&AnalysisPlugin{
        BasePlugin: sdk.BasePlugin{
            PluginName:    "awf-plugin-analysis",
            PluginVersion: "1.0.0",
        },
    })
}
```

**Requirements for `HostClient.Emit()`:**

1. **Declare emit patterns in manifest** — The `events.emit` field must list all event types your plugin can emit:

```yaml
name: awf-plugin-analysis
version: 1.0.0
awf_version: ">=0.8.0"

capabilities:
  - operations
  - events

events:
  emit:
    - "analysis.*"              # Pattern: analysis.complete, analysis.failed, etc.
    - "code.scanned"            # Specific event
```

2. **Implement `SetHostClient`** — The framework calls this during plugin initialization to pass the broker connection:

```go
func (p *MyPlugin) SetHostClient(client *sdk.HostClient) {
    p.hostClient = client
}
```

3. **Check for nil** — `HostClient` is only available if the host supports broker communication (AWF v0.8.0+). Always check before using:

```go
if p.hostClient != nil {
    p.hostClient.Emit(ctx, "event.type", payload, metadata)
}
```

**Error Handling:**

Emit calls can fail if:
- Plugin doesn't declare the event type in `events.emit` (authorization denied)
- Event type not correctly declared (misspelled pattern)
- Host's event system is temporarily unavailable (rare)

Handle errors gracefully — emit failures shouldn't break your operation:

```go
if p.hostClient != nil {
    if err := p.hostClient.Emit(ctx, eventType, payload, metadata); err != nil {
        p.logger.Warn("emit failed (continuing anyway)", "event", eventType, "error", err)
    }
}
```

## Handling Errors

If `HandleEvent` returns an error, the event is logged but doesn't block event delivery to other plugins:

```go
func (p *MyPlugin) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
    if event.Type == "step.failed" {
        // Error doesn't break the pipeline
        return nil, fmt.Errorf("notification service temporarily unavailable")
    }
    return nil, nil
}
```

Plugins should handle timeouts gracefully by using context:

```go
func (p *MyPlugin) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
    // Respect context cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case result := <-p.doAsyncWork():
        return []sdk.Event{result}, nil
    }
}
```

## Streaming Event Delivery

By default, AWF delivers events to plugins via individual unary `HandleEvent` RPCs — one RPC per event. For event-heavy workflows, this creates overhead from repeated connection round-trips.

Plugins that implement the `StreamEvents` RPC receive events over a persistent gRPC stream instead. The host (gRPC client) pushes events via `Send()`, and the plugin (gRPC server) receives them in a `Recv()` loop. This is automatic — plugins that support streaming get it; those that don't continue using unary delivery.

### How It Works

```
Host detects plugin supports StreamEvents
    │
    ├─ Opens persistent stream connection
    │
    └─ All subsequent events use stream.Send()
       instead of individual HandleEvent RPCs
```

### Automatic Fallback

If a plugin does not implement `StreamEvents`, the host detects the gRPC `Unimplemented` status and falls back to unary `HandleEvent` transparently. No configuration needed.

If an active stream breaks (plugin crash, network issue), the StreamManager detects the disconnect within 5 seconds and falls back to unary delivery. Three consecutive send timeouts also trigger stream teardown and fallback.

### When to Use Streaming

Streaming is beneficial when:
- Your plugin receives many events in rapid succession (parallel step execution)
- Latency between event emission and handling is critical
- A workflow emits 100+ events to your plugin per run

For plugins that handle a few events per workflow run, unary delivery is equally effective.

## Performance Considerations

**Event Buffer Limits:**
- Each plugin has a 256-event buffer
- If buffer fills, new events are dropped with a warning
- Slow `HandleEvent` implementations can cause buffer overflow

**Solutions:**
1. Keep `HandleEvent` implementations fast (< 100ms ideal)
2. Use goroutines for blocking I/O:
   ```go
   func (p *MyPlugin) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
       go p.sendAsync(event)  // Don't block HandleEvent
       return nil, nil
   }
   ```
**Cycle Prevention:**
- Event propagation limited to depth 3
- Circular event chains automatically halt with a warning
- No manual cycle detection needed

## Debugging Events

### Logging All Events

Create a debug plugin to inspect event flow:

```go
func (p *DebugPlugin) Patterns() []string {
    return []string{"*.*"}
}

func (p *DebugPlugin) HandleEvent(ctx context.Context, event sdk.Event) ([]sdk.Event, error) {
    fmt.Printf("Event: %s (from %s)\n", event.Type, event.Source)
    fmt.Printf("  ID: %s\n", event.ID)
    fmt.Printf("  Metadata: %v\n", event.Metadata)
    return nil, nil
}
```

### Checking Plugin Status

```bash
awf plugin list          # Shows enabled plugins with capabilities
```

## See Also

- [Plugins Guide](plugins.md) - Complete plugin reference
- [Plugin Event Architecture](../development/plugin-event-architecture.md) - EventBus, gRPC adapter, wiring, and design decisions
- [Architecture](../development/architecture.md) - Overall AWF architecture
