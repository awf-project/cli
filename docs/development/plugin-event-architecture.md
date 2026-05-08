---
title: "Plugin Event System Architecture"
---

## Overview

The Plugin Event System (F090 + F092) enables real-time event-driven communication between AWF core and plugins, and between plugins themselves. It's built on gRPC and implements a fan-out pattern with per-plugin buffered channels for isolation. F092 extends the system with GRPCBroker-based plugin-to-host event emission and persistent gRPC streaming for optimized host-to-plugin delivery.

## Architecture Layers

```
┌──────────────────────────────────────────────────────────┐
│            Plugin SDK Layer (pkg/plugin/sdk)              │
│   EventSubscriber interface │ HostClient (emit via       │
│   + defaults                │ broker) │ StreamEvents     │
│                             │ handler (recv via stream)   │
└──────────────┬──────────────┴──────────┬─────────────────┘
               │ (gRPC HandleEvent /      │ (GRPCBroker
               │  StreamEvents RPC)       │  HostEventService.Emit)
┌──────────────▼──────────────────────────▼────────────────┐
│   StreamManager + gRPC Adapter (pluginmgr/)              │
│   • StreamManager: per-plugin stream tracking            │
│   • Implements EventDeliverer (stream or unary)          │
│   • Automatic fallback on Unimplemented/broken stream    │
│   • grpcEventAdapter: proto ↔ domain conversions         │
└──────────────┬───────────────────────────────────────────┘
               │
┌──────────────▼───────────────────────────────────────────┐
│      EventBus Infrastructure                             │
│   • Pattern matching (glob)                              │
│   • Per-plugin buffered channels (256)                   │
│   • Async delivery goroutines                            │
│   • Cycle detection (depth 3)                            │
│   ┌──────────────────────────────────┐                   │
│   │ HostEventService (broker-served) │                   │
│   │ • Receives plugin Emit() calls   │                   │
│   │ • Validates against manifest     │                   │
│   │ • Publishes to EventBus          │                   │
│   └──────────────────────────────────┘                   │
└──────────────┬───────────────────────────────────────────┘
               │
┌──────────────▼───────────────────────────────────────────┐
│   ExecutionService / Domain                              │
│   Emits lifecycle events (workflow.started)               │
│   Manages EventPublisher port                            │
└──────────────────────────────────────────────────────────┘
```

## Design Decisions

### 1. Unary RPCs vs Streaming

**Decision:** Use unary `HandleEvent` RPC instead of server-side streaming Subscribe.

**Rationale:**
- All existing gRPC services in codebase use unary RPCs on plugin side
- GRPCBroker reverse connections untested in codebase
- Functionally equivalent for fire-and-forget events
- Simpler error handling and timeout management

**Trade-off:**
- No persistent streaming connection (acceptable because go-plugin manages plugin lifecycle)
- Connection breaks cause plugin restart, which re-establishes subscriptions from manifest

### 2. Manifest-Based Subscriptions

**Decision:** Read subscription patterns from plugin manifest at Init time, not via RPC.

**Rationale:**
- Host already parses and validates manifests
- Gating by capability already established pattern
- No additional RPC round-trip
- Consistent with validator/step_type discovery

**Implementation:**
```yaml
# plugin.yaml
capabilities:
  - events
events:
  subscribe:
    - "workflow.*"
    - "step.failed"
```

### 3. Custom Glob Matching

**Decision:** Implement custom dot-segment glob matching, not `filepath.Match`.

**Rationale:**
- Spec requires `.` as segment separator: `workflow.*` must match `workflow.started` but NOT `workflow.step.started`
- `filepath.Match` uses `/` as separator, doesn't apply to dot-separated names
- Custom implementation is simple and clear

**Implementation:**
```
Pattern: "workflow.*"
Event:   "workflow.started"

Split: ["workflow", "*"]  vs  ["workflow", "started"]
       Match!

Event:   "workflow.step.started"
Split:   ["workflow", "step", "started"]  (3 segments vs 2)
         No match!
```

### 4. Per-Plugin Buffered Channels

**Decision:** Each plugin has its own 256-event buffered channel.

**Rationale:**
- Isolation: slow plugin doesn't block others
- Non-blocking delivery: events dropped on full buffer (logged as warning)
- Transport back-pressure: gRPC HTTP/2 flow control beneath application layer
- 256 events sufficient for typical workflow patterns

**If Buffer Overflows:**
```
Event published to slow plugin
│
├─ Non-blocking send to channel (cap 256)
│
└─ If full: Drop event, log warning
   (Other plugins still receive)
```

### 5. Async Delivery Goroutines

**Decision:** One delivery goroutine per plugin subscription.

**Rationale:**
- gRPC HandleEvent RPC runs async from event publication
- Non-blocking: slow RPC doesn't block EventBus or other plugins
- Clean cleanup: done channel signals goroutine exit

**Flow:**
```
Publish(event)
│
├─ For each subscription matching event pattern:
│  │
│  └─ Send to channel (non-blocking)
│
└─ Delivery goroutine (for each plugin):
   │
   ├─ Read from channel
   │
   └─ Call plugin's HandleEvent RPC
      ├─ On success: process emitted events
      ├─ On error: log, continue (don't break)
      └─ On timeout: abort gracefully
```

### 6. Propagation Depth Limiting

**Decision:** Limit event propagation to depth 3.

**Rationale:**
- Prevent infinite loops from circular event chains
- Plugin A emits → triggers Plugin B → emits → triggers Plugin A (caught at depth 3)
- Hard limit prevents stack overflow
- Warnings logged for visibility

**Implementation:**
```go
type DomainEvent struct {
    PropagationDepth int  // Incremented on each plugin emit
}

// In EventBus delivery goroutine:
if event.PropagationDepth >= 3 {
    logger.Warn("propagation depth exceeded for event %s", event.Type)
    return  // Stop propagation
}
```

### 7. DomainEvent in pluginmodel/ Package

**Decision:** Place event entity in `internal/domain/pluginmodel/`, not `internal/domain/workflow/`.

**Rationale:**
- Events are plugin-specific concerns
- Colocated with capability constants and manifest types
- Avoids coupling workflow domain to plugin event system
- Clear separation of concerns

## GRPCBroker and Reverse Channels (F092)

F092 activates the GRPCBroker to enable two capabilities:

1. **Plugin-to-host event emission** — Plugins emit events via `HostEventService` exposed through the broker
2. **Optimized event delivery** — Persistent client-side gRPC streams replace repeated unary RPC calls

### HostEventService

The host exposes a `HostEventService` on the GRPCBroker that plugins can dial to emit events:

```go
// Host side: Expose HostEventService on broker
service HostEventService {
  rpc Emit(EmitRequest) returns (EmitResponse);
}

// Plugin side: Dial and use HostEventService
hostClient := NewHostClient(broker, "plugin-name")
hostClient.Emit(ctx, "event.type", payload, metadata)
```

**Permission Model:**

Emit requests are validated against the plugin's `events.emit` manifest patterns:

```yaml
# plugin.yaml
events:
  emit:
    - "custom.analysis.*"
```

Attempting to emit undeclared event types returns an authorization error:

```
Plugin emits "custom.analysis.complete" → Manifest allows "custom.analysis.*" → ✓ Accepted
Plugin emits "workflow.failed" → Not in manifest → ✗ Denied (BROKER_EMIT_DENIED)
```

**Event Flow:**

```
Plugin→HostClient.Emit(event)
        │
        └─ gRPC Emit() call via broker
           │
           └─ HostEventService receives request
              │
              ├─ Validate: Is emitted event type in plugin's events.emit patterns?
              │
              ├─ Validate: Does plugin have 'events' capability?
              │
              └─ Publish to EventBus (same path as core-emitted events)
                 │
                 └─ EventBus routes to subscribers
                    └─ Each subscriber receives via HandleEvent RPC or streaming
```

### StreamManager and Streaming Delivery

While plugin→host emission goes through the broker's `HostEventService`, the reverse (host→plugin event delivery) is optimized with client-side streaming:

**Streaming RPC Definition:**

```protobuf
service EventService {
  rpc HandleEvent(HandleEventRequest) returns (HandleEventResponse);  // Unary (existing)
  rpc StreamEvents(stream EventStreamMessage) returns (StreamEventsResponse);  // Streaming (F092+)
}
```

Note the asymmetry: `HandleEvent` is **unary** (one request), `StreamEvents` is **client-side streaming** (multiple messages from client). This is because:
- Plugin is gRPC server, host is gRPC client
- Host needs to **push** events to plugin = host (client) sends stream
- Typical gRPC pattern for client→server push

**StreamManager Workflow:**

```
wireEventSubscriptions() called for plugin
    │
    ├─ Check if plugin supports StreamEvents RPC
    │
    ├─ If YES: Register stream via broker
    │  │
    │  └─ StreamManager.RegisterStream(pluginName, stream)
    │     │
    │     └─ Future events routed via stream (low latency, fewer RPC calls)
    │
    └─ If NO: Use unary adapter (fallback)
       │
       └─ Each event triggers a separate HandleEvent RPC call
```

**Benefits:**

- **Lower latency:** One persistent connection vs per-event RPC setup
- **Fewer round-trips:** 100 events = 1 stream + 100 Sends vs 100 HandleEvent calls
- **Transparent fallback:** Plugins without streaming support continue to work unchanged
- **Automatic recovery:** If stream breaks, fallback to unary (no manual intervention)

**Per-Event Sequence Numbers:**

Messages on the stream include a sequence number for ordering and debugging:

```
EventStreamMessage {
  sequence_number: 1  // First event
  id: "evt-123"
  type: "workflow.started"
  ...
}

EventStreamMessage {
  sequence_number: 2  // Second event
  ...
}
```

Sequence numbers are per-plugin (reset on stream reconnect).

## Key Components

### DomainEvent (Domain)

```go
type DomainEvent struct {
    ID               string    // UUID for idempotency
    Type             string    // "workflow.started", "deploy.completed", etc.
    Timestamp        time.Time // When event occurred
    Source           string    // "core" or plugin name
    Metadata         map[string]string
    Payload          []byte    // JSON-serializable data
    PropagationDepth int       // Incremented on inter-plugin routing
}
```

**Properties:**
- UUID enables idempotent processing after reconnection
- Metadata contains context (workflow_id, step_name, etc.)
- Payload for structured event-specific data
- PropagationDepth prevents cycles

### EventPublisher Port (Domain)

```go
type EventPublisher interface {
    Publish(ctx context.Context, event *DomainEvent) error
    Close() error
}
```

**Implementations:**
- `EventBus` in infrastructure
- `MockEventPublisher` in tests

### EventBus (Infrastructure)

```go
type EventBus struct {
    subscriptions map[string]*eventSubscription  // pluginName → subscription
    mu            sync.RWMutex
    logger        ports.Logger
}

type eventSubscription struct {
    patterns      []string
    pluginName    string
    eventCh       chan *DomainEvent  // Buffered
    done          chan struct{}       // Signal goroutine exit
    deliverer     EventDeliverer      // Calls plugin RPC
}
```

**Responsibilities:**
- Pattern matching (glob on event types)
- Per-plugin channel management
- Async delivery goroutines
- Cycle detection
- Back-pressure handling

### gRPC Event Adapter (Infrastructure)

```go
type grpcEventAdapter struct {
    client    pluginv1.EventServiceClient
    pluginName string
    timeout   time.Duration
    logger    ports.Logger
}

func (a *grpcEventAdapter) DeliverEvent(ctx context.Context, event *DomainEvent) ([]*DomainEvent, error) {
    req := domainEventToProto(event)
    resp, err := a.client.HandleEvent(ctx, req)
    if err != nil {
        return nil, err
    }
    return protoToDomainEvents(resp.EmittedEvents), nil
}
```

**Responsibilities:**
- Domain ↔ proto conversions
- Timeout handling
- Emitted event extraction

### EventSubscriber Interface (SDK)

```go
type EventSubscriber interface {
    Patterns() []string
    HandleEvent(ctx context.Context, event Event) ([]Event, error)
}
```

**Plugin Implementation:**
```go
func (p *MyPlugin) Patterns() []string {
    return []string{"workflow.*", "step.failed"}
}

func (p *MyPlugin) HandleEvent(ctx context.Context, event Event) ([]Event, error) {
    // Handle event, optionally emit new events
    return emittedEvents, err
}
```

## F092 Integration: StreamManager and EventDeliverer

F092 introduces minimal coupling by leveraging the existing `EventDeliverer` interface seam:

```go
type EventDeliverer interface {
    DeliverEvent(ctx context.Context, event *DomainEvent) ([]*DomainEvent, error)
}
```

**Before F092:**
- EventBus.Subscribe() takes a `grpcEventAdapter` (implements EventDeliverer)
- grpcEventAdapter calls HandleEvent unary RPC

**After F092:**
- EventBus.Subscribe() takes either:
  - `grpcEventAdapter` (unary, fallback)
  - `streamDeliverer` (streaming, preferred)
- StreamManager decides which deliverer to use per plugin
- No changes to EventBus internals

**SelectionLogic:**

```go
func (sm *StreamManager) GetDeliverer(pluginName string, unaryFallback EventDeliverer) EventDeliverer {
    if sm.HasStream(pluginName) {
        return &streamDeliverer{
            stream:   sm.streams[pluginName],
            fallback: unaryFallback,
        }
    }
    return unaryFallback
}
```

**Fallback Handling:**

If a streaming send fails (stream broken, timeout, or plugin doesn't support streaming), the streamDeliverer automatically falls back to unary:

```go
func (d *streamDeliverer) DeliverEvent(ctx context.Context, event *DomainEvent) ([]*DomainEvent, error) {
    if err := d.stream.Send(eventStreamMessage); err != nil {
        // Detect "Unimplemented" gRPC status (plugin doesn't support StreamEvents)
        if isUnimplemented(err) {
            d.sm.UnregisterStream(d.pluginName)  // Mark stream as unavailable
        }
        // Fall back to unary delivery
        return d.fallback.DeliverEvent(ctx, event)
    }
    return nil, nil
}
```

This design ensures:
- **Backward compatibility:** Existing unary path unchanged
- **Minimal surface area:** No EventBus modifications
- **Graceful degradation:** Streaming failure → unary (always works)
- **Per-plugin granularity:** Each plugin independently uses streaming if available

## Wiring (Interfaces Layer)

**Initialization:**

```
InitSystem()
    │
    ├─ Create EventBus
    │
    └─ Set on RPCPluginManager
        │
        └─ After plugin Init:
           └─ If plugin has "events" capability:
              └─ Create grpcEventAdapter
              └─ Call EventBus.Subscribe()
```

**Execution:**

```
run.go
    │
    ├─ initPluginSystem()
    │   └─ Get EventBus from SystemResult
    │
    └─ Create ExecutionSetup
        │
        └─ WithEventPublisher(eventBus)
            │
            └─ ExecutionService.SetEventPublisher(eventBus)
                │
                └─ On each lifecycle point:
                   └─ Emit domain event via EventPublisher.Publish()
```

## Event Flow (End-to-End)

```
1. Workflow step completes
   └─ ExecutionService.Run() completes step
   
2. ExecutionService emits event
   └─ event := NewDomainEvent("step.completed", "core", metadata)
   └─ eventPublisher.Publish(ctx, event)
   
3. EventBus.Publish() routes event
   └─ For each subscription:
      └─ matchEventPattern("step.*", "step.completed") → true
      └─ Send event to plugin's channel (non-blocking)
   
4. Delivery goroutine processes event
   └─ Read from eventCh
   └─ Call plugin's HandleEvent RPC
   
5. Plugin's HandleEvent runs
   └─ Receives sdk.Event
   └─ Returns []sdk.Event (custom events to emit)
   
6. Emitted events routed
   └─ Increment PropagationDepth
   └─ Send to other subscribed plugins
   └─ If depth ≥ 3: halt propagation (log warning)
```

## Lifecycle & Cleanup

**Plugin Initialization:**
```
plugin process starts
    │
    ├─ Plugin.Init() called
    │
    └─ Plugin has "events" capability:
       └─ EventBus.Subscribe(pluginName, patterns, adapter)
          └─ Start delivery goroutine
```

**Plugin Shutdown:**
```
Plugin process dies / removed via `awf plugin remove`
    │
    └─ EventBus.Unsubscribe(pluginName)
       │
       ├─ Close done channel
       │
       └─ Delivery goroutine exits (receives on done)
          │
          └─ Channel cleanup
          └─ Goroutine count back to baseline (5s max)
```

**Graceful Shutdown:**
```
awf run completes
    │
    └─ ExecutionService.Run() returns
    
    └─ run.go defers cleanup
       │
       └─ pluginResult.Cleanup()
          │
          └─ EventBus.Close()
             │
             └─ Unsubscribe all plugins
             └─ Wait for delivery goroutines
```

## Non-Functional Requirements

### Latency (NFR-001)

Target: Event delivery to plugin HandleEvent < 10ms (p95) for idle streams.

**Achieved via:**
- In-process EventBus (no network hop)
- Buffered channels (no allocation per event)
- Immediate async delivery (no queuing beyond buffer)

### Isolation (NFR-002)

Slow plugin must not block event delivery to other plugins.

**Achieved via:**
- Per-plugin buffered channels (independent)
- Non-blocking send (drops on full)
- Async delivery goroutine per plugin

### Cleanup (NFR-003)

Stream disconnection cleanup within 5 seconds, no goroutine leaks.

**Achieved via:**
- done channel signals goroutine exit
- Delivery goroutine reads from done in select
- Channel cleanup on exit

### Back-Pressure (NFR-004)

Slow/crashed plugins don't cause buffer overflow in EventBus core.

**Achieved via:**
- Non-blocking send to per-plugin channel
- 256-event buffer per plugin
- Drops logged as warnings
- EventBus core never blocks on plugin channels

### Secret Masking (NFR-005)

Event metadata/payload must mask secrets in logs.

**Implementation:**
```go
func emitEvent(ctx context.Context, eventType, source string, metadata map[string]string) {
    // Mask secrets in metadata
    masked := logger.SecretMasker.Mask(metadata)
    event := NewDomainEvent(eventType, source, masked, nil)
    eventPublisher.Publish(ctx, event)
}
```

### Backward Compatibility (NFR-006)

Existing plugins without `events` capability unaffected.

**Achieved via:**
- Optional capability gating
- EventSubscriber on BasePlugin has no-op defaults
- Manifest validation allows empty events field

## Testing Strategy

### Unit Tests

- **Pattern Matching** (11 cases): exact, wildcard single/multi-segment, empty inputs
- **EventBus Publishing**: delivery to matching subscribers, no-match skip
- **Buffer Management**: drop on full, warning logged
- **Cycle Detection**: depth 3 halt, propagation prevented
- **Goroutine Cleanup**: baseline count restored after unsubscribe
- **gRPC Adapter**: proto conversion, RPC calls
- **SDK Event Defaults**: BasePlugin no-op, custom subscriber dispatch

### Integration Tests

- Plugin with events capability receives workflow.completed
- Two plugins: A emits → B receives with source attribution
- Goroutine baseline restored after full cleanup
- All existing plugin tests pass without modification

## Recent Enhancements (F092)

F092 introduced two major enhancements:

1. **Plugin-to-Host Event Emission** — Plugins can now emit events via HostClient, not just return events from HandleEvent
2. **Optimized Event Delivery** — Persistent client-side gRPC streams reduce latency and RPC overhead

These features completed the bidirectional event system (host→plugin via EventBus, plugin→host via broker) and optimized the high-throughput case.

## Future Extensions

### Persistent Event Log

**Status:** Deferred

**Rationale:** Fire-and-forget sufficient for v0.8+; replay adds complexity (durable storage, message ordering guarantees)

### Fine-Grained Filters

**Status:** Deferred

**Rationale:** Glob on event type covers 90% of use cases; field-level filtering adds protocol complexity

### OTel Correlation

**Status:** Deferred to OTel roadmap progress

**Rationale:** Events carry metadata for manual correlation; automatic integration requires OTel lib decisions

### Bidirectional Streaming

**Status:** Not planned (client-side streaming sufficient)

**Rationale:** F092 implements client-side streaming (host→plugin) for optimal push delivery. Bidirectional streaming adds complexity without demonstrated benefit — plugin→host uses broker's separate `Emit` RPC instead

## See Also

- [Plugin Events Guide](../user-guide/plugin-events.md) - User-facing documentation
- [Plugins Guide](../user-guide/plugins.md) - Plugin development reference
- [Architecture](architecture.md) - Overall AWF architecture
