---
title: "Transcript Schema"
---

Reference for the agent exchange transcript JSONL format produced at `storage/transcripts/<run-id>.jsonl`. Each line is a complete `ExchangeEvent`. See the [Agent Exchange Transcript guide](../user-guide/transcript.md) for usage.

## ExchangeEvent Envelope

All lines share the same envelope. Event-specific data lives in `payload`.

```json
{
  "seq": 1,
  "run_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "run.started",
  "path": "",
  "iteration": 0,
  "timestamp": "2026-06-08T08:14:42.123Z",
  "payload": null
}
```

### Envelope Fields

| Field | Type | Required | Notes |
|---|---|---|---|
| `seq` | uint64 | yes | Strictly monotonic per file, starting at `1`. Allocated atomically by the recorder before write. |
| `run_id` | string | yes | UUID v4 identifying the workflow run. Reused from `ExecutionContext.WorkflowID` — same identifier as the audit trail. |
| `parent_run_id` | string | no | UUID v4 of the enclosing parent run. Present on child lifecycle events when the run was invoked via `call_workflow`. Omitted via `omitempty` when empty. |
| `child_run_id` | string | no | UUID v4 of a spawned sub-workflow run. Present on `step.call_workflow.started` / `step.call_workflow.completed` envelopes in the parent file. Omitted via `omitempty` when empty. |
| `type` | string | yes | One of the closed [event types](#event-types). |
| `path` | string | yes | Dot-separated step path within the workflow definition (e.g., `analyze`, `parallel_block.branch_a`). Empty string at the run-level events. |
| `iteration` | int | yes | Zero-based loop/retry iteration counter. `0` outside loops. |
| `timestamp` | string | yes | RFC 3339 / ISO 8601 with timezone, UTC preferred. Set at emission time. |
| `payload` | any | yes | Concrete shape determined by `type`. May be `null` for run lifecycle events. See [Payload Dispatch](#payload-dispatch). |

---

## Event Types

The vocabulary is **closed**. The complete set of valid `type` values is exactly:

| `type` | Payload | Description |
|---|---|---|
| `run.started` | `*StepPayload` or `null` | Workflow run begins. |
| `run.completed` | `*StepPayload` or `null` | Workflow run ends. |
| `step.started` | `*StepPayload` | A step begins. Covers `agent`, `command`, `operation`, `terminal`, `parallel`, `for_each`, `while`, and generic custom step types. |
| `step.completed` | `*StepPayload` | A step ends (success or failure). |
| `step.call_workflow.started` | `*StepPayload` | Parent emits a sub-workflow invocation; the envelope carries `child_run_id`. |
| `step.call_workflow.completed` | `*StepPayload` | Parent observes sub-workflow completion. |
| `message.user` | `*MessagePayload` | Agent seam — resolved user prompt + composed `system_prompt`. |
| `message.assistant` | `*MessagePayload` | Agent reply, normalized into `ContentBlock`s. |
| `tool.call` | `*ToolPayload` | Tool invocation begins. Captured at the `tools.Router.CallTool` seam (`fidelity:"router"`) or from agent NDJSON (`fidelity:"agent_emitted"`). |
| `tool.result` | `*ToolPayload` | Tool invocation completes. |

Writers never emit values outside this set. Readers must handle unknown `type` values forward-compatibly — see [Forward Compatibility](#forward-compatibility).

---

## Payload Shapes

### `StepPayload`

Carried by `step.*` and `run.*` events.

| Field | Type | Required | Notes |
|---|---|---|---|
| `name` | string | yes | Step name from the workflow YAML, or workflow name for `run.*` events. |
| `kind` | string | yes | Step type discriminator (`agent`, `command`, `operation`, `terminal`, `parallel`, `for_each`, `while`, `call_workflow`, or a custom type name). Reflects the actual `step.Type` field. |
| `error` | string | no | Failure description on `*.completed` events; omitted on success via `omitempty`. |
| `result` | any | no | Step result for `*.completed` events. Shape depends on the step kind (e.g., agent output, command stdout, custom step return value). Omitted via `omitempty` when nil. |

```json
{
  "type": "step.started",
  "payload": {
    "name": "analyze",
    "kind": "agent"
  }
}
```

### `MessagePayload`

Carried by `message.user` and `message.assistant`.

| Field | Type | Required | Notes |
|---|---|---|---|
| `role` | string | yes | `"user"` or `"assistant"`. Discriminates payload dispatch in tolerant decoders. |
| `blocks` | array of `ContentBlock` | yes | Ordered content blocks. For `message.user`, contains the resolved prompt and (when non-empty) the composed `system_prompt`. For `message.assistant`, contains the normalized provider output. |

```json
{
  "type": "message.user",
  "payload": {
    "role": "user",
    "blocks": [
      {"type": "text", "fidelity": "router", "text": "Review main.go for bugs."}
    ]
  }
}
```

### `ToolPayload`

Carried by `tool.call` and `tool.result`.

| Field | Type | Required | Notes |
|---|---|---|---|
| `name` | string | yes | Tool name (`Read`, `Bash`, plugin operation name, …). |
| `call_id` | string | yes | Opaque correlation token linking a `tool.call` to its `tool.result`. |
| `input` | any | yes (on `tool.call`) | Tool arguments as provided. |
| `output` | any | yes (on `tool.result`) | Tool return value. |
| `error` | string | no | Failure description on `tool.result`; omitted on success via `omitempty`. |
| `fidelity` | string | yes | `"router"` (synthesized at the AWF router seam) or `"agent_emitted"` (reported by the agent over stdio NDJSON). |

```json
{
  "type": "tool.call",
  "payload": {
    "name": "Read",
    "call_id": "tool_01H...",
    "input": {"path": "main.go"},
    "output": null,
    "fidelity": "router"
  }
}
```

---

## ContentBlock

Building block inside `MessagePayload.blocks`. The vocabulary is **closed**; field population depends on `type`. Unused fields are omitted via `omitempty`.

| `type` | Carries | Active Fields |
|---|---|---|
| `text` | Plain assistant text | `text` |
| `thinking` | Reasoning output (e.g., Claude extended thinking) | `thinking` |
| `tool_use` | Tool invocation block | `tool_name`, `tool_id`, `tool_input` |
| `tool_result` | Tool output block | `tool_id`, `tool_content` |
| `command` | Shell command executed by a step | `command` |
| `stream` | Provider stream chunk for partial-output replay | `chunk` (in `text` field per the wire format) |

Every block carries:

| Field | Type | Required | Notes |
|---|---|---|---|
| `type` | string | yes | One of the six block types above. |
| `fidelity` | string | yes | `"router"` or `"agent_emitted"`. |

Block-specific fields (only populated for the relevant `type`; others omitted via `omitempty`):

| Field | Type | Used By | Notes |
|---|---|---|---|
| `text` | string | `text`, `stream` | Verbatim assistant text. NUL bytes preserved. |
| `thinking` | string | `thinking` | Verbatim thinking output. |
| `tool_name` | string | `tool_use` | Tool name as advertised by the provider. |
| `tool_id` | string | `tool_use`, `tool_result` | Provider-supplied correlation token; links a `tool_use` block to its `tool_result` block within the same message. |
| `tool_input` | any | `tool_use` | Arguments as provided by the agent. |
| `tool_content` | any | `tool_result` | Output as observed. May be a string, structured object, or `null` for errors. |
| `command` | string | `command` | Resolved shell command string after template interpolation. |
| `chunk` | string | `stream` | Single stream chunk from the provider. |

```json
{"type": "text", "fidelity": "agent_emitted", "text": "Found 2 issues."}
{"type": "thinking", "fidelity": "agent_emitted", "thinking": "First I should..."}
{"type": "tool_use", "fidelity": "agent_emitted", "tool_name": "Read", "tool_id": "toolu_01", "tool_input": {"path": "main.go"}}
{"type": "tool_result", "fidelity": "router", "tool_id": "toolu_01", "tool_content": "package main\n..."}
{"type": "command", "fidelity": "router", "command": "go test ./..."}
```

---

## Fidelity

The `fidelity` field on every `ContentBlock` and `ToolPayload` distinguishes two provenances:

| Value | Meaning |
|---|---|
| `"router"` | Block synthesized by AWF's `tools.Router.CallTool` seam. Authoritative for in-process tool calls (builtin + plugin). |
| `"agent_emitted"` | Block emitted directly by the agent (e.g., stdio proxy NDJSON `tool_use`). Provenance is the agent, not the router. |

Consumers can use this marker to avoid double-counting tool calls reported on multiple channels.

---

## Payload Dispatch

The decoder selects the concrete `payload` type from the envelope `type` combined with a tolerant probe of the JSON shape:

1. If the raw payload is a JSON **array**, decode into `[]ContentBlock`.
2. If the raw payload is an **object** containing `"role"`, decode into `*MessagePayload`.
3. Else if it contains `"call_id"`, decode into `*ToolPayload`.
4. Else if it contains `"kind"`, decode into `*StepPayload`.
5. Otherwise, decode into a generic `any` (forward-compatibility fallback).

A `null` or missing payload yields `payload: nil` — valid for `run.*` lifecycle events.

---

## File Properties

| Property | Value |
|---|---|
| Path | `storage/transcripts/<run-id>.jsonl` |
| Mode | `0o600` |
| Open flags | `O_APPEND \| O_CREATE \| O_WRONLY` |
| Writes | Serialized via `sync.Mutex` (always held — also covers payloads beyond POSIX `PIPE_BUF`) |
| Encoding | UTF-8, one JSON object per line, LF line endings |
| Ordering | Strictly monotonic `seq` per file, starting at `1` |

### Concurrency

- `seq` is allocated from a single `atomic.AddUint64` inside the recorder *before* the write lock is taken.
- All writes are serialized; no torn lines even when a payload exceeds `PIPE_BUF`.
- Multiple goroutines emitting concurrently produce a strictly monotonic `seq` series with no gaps.

### Atomicity

- `O_APPEND` guarantees that each successful `write()` lands at the file end.
- Process kill mid-write cannot tear a line written under `PIPE_BUF`; the mutex covers the larger case.

---

## Sub-Workflow Linkage

A `call_workflow` step produces a **new file** for the child run. Linkage is bidirectional:

- **Parent file** — `step.call_workflow.started` envelope sets `child_run_id` to the new run's UUID.
- **Child file** — every envelope sets `parent_run_id` to the invoking run's UUID.

Each file's `seq` series is independent and starts at `1`. Reconstruction walks `child_run_id` links from the parent to locate every child file and assembles a connected tree.

Nesting deeper than one level produces one file per level with consistent parent linkage at every depth.

---

## Forward Compatibility

The reader is **tolerant**; the writer is **strict**:

- Writers only emit values from the closed `EventType` and `BlockType` vocabularies.
- Readers that encounter an unknown `EventType` surface it through `errors.Is(err, transcript.ErrUnknownEventType)` so callers may treat it as a warning rather than a parse failure. Likewise `transcript.ErrUnknownBlockType` for unknown `BlockType` values.
- Adding new envelope fields is **safe** — older readers ignore unknown fields per the JSON standard.
- Adding new event or block types requires every consumer to handle the unknown-type path explicitly. Removing or renaming values requires a coordinated schema bump.

---

## Coexistence

The transcript is a **pure addition**:

- `audit.jsonl` output is byte-identical whether the transcript recorder is wired or not.
- `DisplayEvent` streams (used by streaming display and TUI) remain unchanged.
- Plugin behavior is unaffected — instrumentation happens at AWF boundaries, not inside plugins.

---

## Live Fan-Out

Subscribers connect via the `Recorder.Subscribe()` port and receive every event written to disk, in order.

| Property | Value |
|---|---|
| Per-subscriber buffer | 256 events (default) |
| Drop policy | drop-newest |
| Ordering | Write-then-broadcast: disk first, subscribers second |
| Back-pressure | Slow subscriber drops events; disk write never blocks |
| Drop visibility | `FanOut.Stats()` exposes a drop counter; rate-limited WARN log (1/s per subscriber) when drops occur |
| `Close()` | Idempotent on both recorder and subscriber |

---

## Limitations

- **No default secret masking.** Prompts, system prompts, and tool inputs are recorded verbatim. The design preserves an opt-in masking hook for future use, but no masking ships in this version.
- **`awf mcp-serve` subprocess capture is not yet instrumented.** The `fidelity:"agent_emitted"` marker documents this gap and enables a future transition without changing the wire format.
- **Token-by-token streaming deltas are not captured.** The `stream` block type exists in the vocabulary but is reserved for future use.

---

## Querying Examples

### Decode every event

```bash
jq '.' storage/transcripts/<run-id>.jsonl
```

### Pair tool calls with their results

```bash
jq -s '
  [.[] | select(.type == "tool.call" or .type == "tool.result")]
  | group_by(.payload.call_id)
  | map({
      call_id: .[0].payload.call_id,
      name: .[0].payload.name,
      input: (map(select(.type == "tool.call"))[0].payload.input),
      output: (map(select(.type == "tool.result"))[0].payload.output)
    })
' storage/transcripts/<run-id>.jsonl
```

### Extract every assistant text response

```bash
jq -r 'select(.type == "message.assistant")
  | .payload.blocks[]
  | select(.type == "text")
  | .text' storage/transcripts/<run-id>.jsonl
```

### Verify monotonic seq

```bash
jq -s '[.[].seq] | . == (sort)' storage/transcripts/<run-id>.jsonl
# true
```

### Reconstruct a step tree

```bash
jq -r 'select(.type == "step.started")
  | "\(.seq)\t\(.path)\t\(.payload.kind)\t\(.payload.name)"' \
  storage/transcripts/<run-id>.jsonl
```

### Walk parent → child runs

```bash
# List every child run spawned by this parent
jq -r 'select(.type == "step.call_workflow.started") | .child_run_id' \
  storage/transcripts/<parent-run-id>.jsonl
```

### Distinguish router-fidelity vs agent-emitted tool calls

```bash
jq -s '
  [.[] | select(.type == "tool.call")]
  | group_by(.payload.fidelity)
  | map({fidelity: .[0].payload.fidelity, count: length})
' storage/transcripts/<run-id>.jsonl
```

---

## See Also

- [Agent Exchange Transcript Guide](../user-guide/transcript.md) — Conceptual overview, file location, fan-out, and security notes
- [Audit Trail Schema](audit-trail-schema.md) — The separate paired-event audit format that coexists with the transcript
- Package documentation: run `go doc github.com/awf-project/cli/internal/domain/transcript` for the in-tree reference
