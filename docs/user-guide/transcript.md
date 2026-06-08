---
title: "Agent Exchange Transcript"
---

AWF records a canonical agent exchange transcript for every workflow run. It is an append-only JSONL stream that captures the full workflow lifecycle — step start/end events, user prompts, assistant replies, tool calls, tool results, and sub-workflow linkage — in a single normalized format suitable for offline replay, auditing, and reconstruction of the execution tree.

The transcript coexists with the [audit trail](audit-trail.md) and the streaming `DisplayEvent` output; none of those channels change because the transcript exists.

## How It Works

When a workflow runs, AWF appends one JSON object per line to:

```
storage/transcripts/<run-id>.jsonl
```

Each line is a complete [`ExchangeEvent`](../reference/transcript-schema.md#exchangeevent) with a strictly monotonic `seq`, the workflow `run_id`, the dot-separated `path` of the current step, a UTC `timestamp`, and a typed `payload` whose shape is determined by the event `type`.

Events are written **to disk first**, then broadcast to live subscribers. The disk file is the durable contract: a slow consumer never blocks a write.

## Event Vocabulary

The `type` field uses a closed vocabulary. Every line in the file is one of:

| Event Type | Emitted When |
|---|---|
| `run.started` | Workflow run begins |
| `run.completed` | Workflow run ends (success or failure) |
| `step.started` | Any step begins (`agent`, `command`, `operation`, `terminal`, `parallel`, `for_each`, `while`, custom) |
| `step.completed` | Any step ends |
| `step.call_workflow.started` | Parent emits a sub-workflow invocation; carries `child_run_id` |
| `step.call_workflow.completed` | Parent observes sub-workflow completion |
| `message.user` | Agent seam — resolved user prompt + composed `system_prompt` |
| `message.assistant` | Agent reply, normalized into `ContentBlock`s |
| `tool.call` | A tool invocation begins (router seam or agent-emitted) |
| `tool.result` | A tool invocation completes |

Readers must treat unknown event types as forward-compatible: surface them with the type intact, do not panic. See [Schema → Forward Compatibility](../reference/transcript-schema.md#forward-compatibility).

## Content Blocks

Assistant messages and tool payloads carry typed `ContentBlock` entries. The closed vocabulary is:

| Block Type | Carries |
|---|---|
| `text` | Plain assistant text |
| `thinking` | Reasoning / thinking output (Claude extended thinking, etc.) |
| `tool_use` | Tool invocation block with `tool_name`, `tool_id`, `tool_input` |
| `tool_result` | Tool output block with `tool_content` |
| `command` | Shell command executed by a step |
| `stream` | Provider stream chunk (for replays that preserve partials) |

Every block also carries a `fidelity` marker:

- `"router"` — block synthesized by AWF's `tools.Router.CallTool` seam. Authoritative for in-process tool calls.
- `"agent_emitted"` — block emitted directly by the agent (e.g., stdio proxy NDJSON `tool_use`). Provenance is the agent, not the router.

This marker lets consumers distinguish ground truth from agent-reported events without double-counting.

## Per-Provider Normalization

Outputs from Claude, Codex, Gemini, Copilot, and OpenAI HTTP are normalized into the same `ContentBlock` stream by a single mapping layer in `internal/infrastructure/agents/`. Provider quirks are absorbed there:

- **Codex** — embedded NUL bytes in JSONL are handled without corrupting the transcript line.
- **Dangling `tool_use`** — a `tool_use` without a matching `tool_result` (timeout, crash) is recorded as-is; the parser does not panic or drop the message.
- **Mixed blocks** — Claude `thinking` + `text` + `tool_use` in one response yields three blocks in order.

The existing per-provider `DisplayEvent` output is **not** changed; the transcript adds a sibling mapping rather than replacing the display layer.

## Sub-Workflow Linkage

When a parent run invokes `call_workflow`, AWF writes the child run to its own file. Linkage is bidirectional:

- The **parent** emits `step.call_workflow.started` carrying `child_run_id`.
- The **child** writes every lifecycle event with `parent_run_id` populated.

Each file is self-contained — one `seq` series per file, reconstructable in isolation. Reading both files yields a single connected execution tree.

Nesting deeper than one level produces one file per level with consistent parent linkage at every depth.

## Live Fan-Out

Consumers subscribe to the recorder to receive events as they are written. Fan-out is bounded:

- Per-subscriber channel buffer: **256 events** (default).
- Back-pressure policy: **drop-newest** — once a subscriber's buffer fills, new events are dropped for that subscriber until it drains.
- A drop counter is exposed via `FanOut.Stats()`; the recorder logs a rate-limited WARN (1/s) per subscriber when drops occur.
- The disk write is **never** blocked by a slow subscriber; the file remains complete and monotonic regardless.

`Close()` on the recorder or a subscriber is idempotent — calling it twice is safe.

## File Properties

| Property | Value |
|---|---|
| Path | `storage/transcripts/<run-id>.jsonl` |
| Mode | `0o600` (owner read/write only) |
| Write mode | `O_APPEND` |
| Atomicity | Single `write()` per line; mutex-serialized beyond `PIPE_BUF` |
| Encoding | UTF-8, one JSON object per line, LF line endings |
| Ordering | Strictly monotonic `seq` starting at `1` |

`<run-id>` is the same UUID v4 used by the audit trail and the state machine — there is no separate ID infrastructure.

## Querying the Transcript

The format is JSONL, so standard tooling works:

```bash
# Pretty-print an entire run
jq '.' storage/transcripts/<run-id>.jsonl

# Extract every tool call with its result
jq 'select(.type == "tool.call" or .type == "tool.result")' storage/transcripts/<run-id>.jsonl

# List all assistant text content blocks
jq -r 'select(.type == "message.assistant") | .payload.blocks[] | select(.type == "text") | .text' \
  storage/transcripts/<run-id>.jsonl

# Show only router-fidelity tool events
jq 'select(.type == "tool.call" and .payload.fidelity == "router")' \
  storage/transcripts/<run-id>.jsonl

# Reconstruct the step path tree
jq -r 'select(.type == "step.started") | "\(.seq)\t\(.path)\t\(.payload.kind)"' \
  storage/transcripts/<run-id>.jsonl

# Follow nested sub-workflows: find every child run referenced by a parent
jq -r 'select(.type == "step.call_workflow.started") | .child_run_id' \
  storage/transcripts/<run-id>.jsonl
```

## Coexistence Guarantees

- `audit.jsonl` output is **byte-identical** whether the transcript recorder is wired or not.
- `DisplayEvent` streams remain unchanged; the transcript adds a parallel mapping, not a replacement.
- Plugins are unaffected — capture happens at AWF boundaries (router seam, agent seam), never inside plugin internals.

## Security & Privacy

- Files are written with mode `0o600` — only the owning user can read them.
- **No secrets are masked by default.** A `message.user` event includes the resolved prompt and composed system prompt verbatim; `tool.call` payloads include arguments verbatim. Treat transcript files like raw command logs.
- The design preserves an opt-in masking hook for future use, but no masking ships in this version.

## See Also

- [Transcript Schema](../reference/transcript-schema.md) — Full field reference, payload shapes, and constraints
- [Audit Trail](audit-trail.md) — The separate paired-event audit channel that coexists with the transcript
- [Distributed Tracing](tracing.md) — OpenTelemetry spans for cross-system correlation
