---
title: "Audit Trail"
---

AWF records a structured audit log for every workflow execution. Each execution produces two JSONL entries — one at start, one at completion — enabling post-hoc tracing of who ran what, when, and what happened.

## How It Works

When you run a workflow, AWF automatically appends two entries to the audit trail file:

1. **`workflow.started`** — written immediately when execution begins (captures intent)
2. **`workflow.completed`** — written when execution ends, whether success or failure (captures outcome)

Both entries share the same `execution_id`, enabling correlation.

```bash
awf run deploy-app --input env=staging
```

```
# audit.jsonl (two lines appended)
{"event":"workflow.started","execution_id":"550e8400-...","timestamp":"2026-02-20T23:15:42.123+01:00","user":"deploy-bot","workflow_name":"deploy-app","inputs":{"env":"staging"},"schema_version":1}
{"event":"workflow.completed","execution_id":"550e8400-...","timestamp":"2026-02-20T23:16:12.456+01:00","user":"deploy-bot","workflow_name":"deploy-app","status":"success","exit_code":0,"duration_ms":30333,"schema_version":1}
```

## Default Location

The audit trail is written to:

```
$XDG_DATA_HOME/awf/audit.jsonl
```

This defaults to `~/.local/share/awf/audit.jsonl` on most systems.

## Configure the Path

Set `AWF_AUDIT_LOG` to write to a custom location:

```bash
export AWF_AUDIT_LOG=/var/log/awf/audit.jsonl
awf run deploy-app --input env=production
```

AWF creates the file and parent directories automatically with `0600` permissions.

## Disable the Audit Trail

Set `AWF_AUDIT_LOG=off` to disable audit recording entirely:

```bash
AWF_AUDIT_LOG=off awf run my-workflow
```

No file is created or appended to.

## Query the Audit Trail

The JSONL format works with standard tools:

```bash
# View all entries for a specific execution
jq 'select(.execution_id == "550e8400-...")' audit.jsonl

# Count execution pairs
jq -s 'group_by(.execution_id) | length' audit.jsonl

# Find failed executions
jq 'select(.status == "failure")' audit.jsonl

# Detect abnormal terminations (orphaned start entries)
jq -s 'group_by(.execution_id) | map(select(length == 1)) | .[] | .[0]' audit.jsonl

# Stream in real-time
tail -f audit.jsonl | jq
```

## Secret Masking

Input values whose keys match secret patterns (`SECRET_*`, `API_KEY*`, `PASSWORD*`, `TOKEN*`) are automatically masked:

```bash
awf run deploy --input api_key=sk-secret123
```

```json
{"event":"workflow.started","inputs":{"api_key":"***"},...}
```

## Resilience

Audit trail failures never block workflow execution:

- If the audit file path is not writable, a warning is emitted to stderr and the workflow proceeds normally.
- Audit write errors do not change the workflow exit code.

## Canonical Transcript vs. Audit Trail

In addition to the audit trail, AWF automatically creates a **canonical transcript** file for every workflow run. While similar in format (both JSONL), they serve different purposes:

| Aspect | Audit Trail | Canonical Transcript |
|--------|-----------|----------------------|
| **Scope** | Workflow-level summary | Full execution details |
| **Events per run** | 2 (start + completion) | Hundreds (every step, message, tool call) |
| **File location** | `$XDG_DATA_HOME/awf/audit.jsonl` | `storage/transcripts/<run-id>.jsonl` |
| **Purpose** | Compliance, accounting | Replay, debugging, detailed audit |
| **Step details** | Minimal | Complete (prompts, tool inputs/outputs, loop iterations) |
| **Agent exchange** | Not recorded | Full lifecycle (every message) |

**Use the audit trail** for:
- Compliance logging (who ran what, when)
- Execution accounting (success/failure counts)
- Historical queries across all runs

**Use the canonical transcript** for:
- Debugging failed workflows (see exact agent response)
- Replaying executions offline
- Analyzing agent behavior
- Sub-workflow tree reconstruction

## See Also

- [Transcript Schema](../reference/transcript-schema.md) — Full transcript field reference and content blocks
- [Audit Trail Schema](../reference/audit-trail-schema.md) — Full field reference and constraints
- [ADR-0010](../ADR/010-paired-jsonl-audit-trail-with-atomic-append.md) — Design decision: paired JSONL with atomic append
- [ADR-0011](../ADR/011-application-layer-secret-masking-for-audit-events.md) — Design decision: application-layer secret masking
