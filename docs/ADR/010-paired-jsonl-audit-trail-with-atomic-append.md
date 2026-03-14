---
title: "010. Paired JSONL Audit Trail with Atomic Append"
---

Date: 2026-02-21
Status: Accepted
Issue: F071

## Context

F071 requires a structured audit trail for workflow executions. The trail must be append-only, concurrent-safe (multiple `awf` processes writing simultaneously), streamable with `tail -f`, and parseable by `jq`/`grep`. A separate format from the SQLite history store is needed since audit logs target sysadmins (log aggregation, compliance) while history targets developers (replay, debug).

## Decision

Emit paired `workflow.started` + `workflow.completed` JSONL entries via `O_APPEND|O_CREATE|O_WRONLY` writes. Each entry must stay under 4KB (POSIX `PIPE_BUF` guarantee for atomic writes without file locking). Default path is `$XDG_DATA_HOME/awf/audit.jsonl`, overridable via `AWF_AUDIT_LOG` env var; `AWF_AUDIT_LOG=off` disables the trail entirely.

Alternatives rejected:
- **JSON array** — requires read-modify-write; not append-friendly; corrupts on concurrent write.
- **SQLite** — heavier than needed; audit trail serves a different audience than the existing history store.
- **Single end-only entry** — loses evidence of executions killed before completion.

## Consequences

### Positive
- No file locking needed: `O_APPEND` + entries under 4KB = atomic on POSIX.
- Streamable with `tail -f`; parseable with `jq` without buffering full file.
- Paired entries enable detection of crashed/killed workflows via unmatched `workflow.started`.

### Negative
- 4KB entry limit requires input truncation logic for workflows with many large inputs.
- Paired model means analysis tools must join on `execution_id` to get full execution data.
