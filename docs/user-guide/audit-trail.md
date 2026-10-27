---
title: "Audit Trail"
---

When audit logging is enabled and writes succeed, AWF records two JSONL entries for each started execution attempt — one at start and one at completion — enabling post-hoc tracing of who ran what, when, and what happened.

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

## Completion Status and Exit Codes

Completion records use an unambiguous status/code pair:

- Successful executions are recorded as `status: "success"` with `exit_code: 0`.
- Failed executions are recorded as `status: "failure"` with a non-zero exit code.
- Explicit non-zero codes from a failed command, provider, plugin, custom step, or terminal state are preserved.
- Failures without a more precise code use exit code `3` (`Execution Error`). Cancelled executions have audit status `failure` and also use code `3` when they have no more precise code.

Each initial or resumed execution attempt is finalized once. A cancelled attempt
is recorded as `cancelled` in execution history and `failure` in the audit trail.

AWF normalizes the outcome before writing it, so the execution history and audit
trail contain the same exit code when both writes succeed. This correction applies
to new records; it does not rewrite existing audit entries.

AWF finalizes each started attempt once and writes a matching
`workflow.completed` entry, including executions that fail during a step, time
out, are interrupted by a signal, are cancelled, or recover from a panic. The
completion is written from the final execution result, so an error cannot be
reported as a successful execution.

Before a workflow starts its affected step, AWF also validates referenced inputs
and operation configuration. Provider-backed and conversation steps perform the
available non-interactive readiness checks before invoking the provider. These
checks prevent an invalid or unavailable workflow from producing a partial run.

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

AWF creates the file and parent directories automatically. New audit files use `0600` permissions; newly created parent directories use `0750`.

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
{"event":"workflow.started","inputs":{"api_key":"***"}}
```

## Resilience

Audit trail failures never block workflow execution:

- If the audit file path is not writable, a warning is emitted to stderr and the workflow proceeds normally.
- Audit write errors do not change the workflow exit code.

Each serialized record is bounded to 4 KiB. Large input values are truncated in
deterministic order and marked with `inputs_truncated`. Oversized completion
errors are shortened with an explicit `… [truncated]` marker so the terminal
identity, status, and exit code are still recorded. Records whose identity fields
alone exceed the limit are rejected without appending a partial line. Concurrent executions append
complete JSON objects, so readers can parse each line independently.

For automated tests, redirect the default data directory to a test-owned
temporary location or set `AWF_AUDIT_LOG=off` when audit output is not under test.
This keeps test and fixture executions out of the user's audit trail.

```bash
test_data_dir=$(mktemp -d)
XDG_DATA_HOME="$test_data_dir" awf run example
```

If `AWF_AUDIT_LOG` is set, it overrides the XDG location; set it to a temporary
file path for that run or unset it before using this example.

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
