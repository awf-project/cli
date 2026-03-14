---
title: "Audit Trail Schema"
---

Reference documentation for audit trail JSONL entries. Each line in the audit trail is a complete JSON object.

## Entry Structure

All entries follow a common envelope with event-specific fields added:

```json
{
  "event": "workflow.started | workflow.completed",
  "execution_id": "unique-uuid-v4",
  "timestamp": "2026-02-20T23:15:42.123+01:00",
  "user": "operating-system-username",
  "workflow_name": "workflow-name",
  "schema_version": 1,
  "...": "event-specific fields"
}
```

---

## Field Specifications

### Common to All Entries

| Field | Type | Required | Example | Notes |
|-------|------|----------|---------|-------|
| `event` | string | yes | `"workflow.started"` | Event type: one of `workflow.started`, `workflow.completed` |
| `execution_id` | string | yes | `"550e8400-e29b-41d4-a716-446655440000"` | UUID v4, shared between paired start/completed entries for correlation |
| `timestamp` | string | yes | `"2026-02-20T23:15:42.123+01:00"` | ISO 8601 format with millisecond precision and timezone offset |
| `user` | string | yes | `"deploy-bot"` | OS username of effective user (resolves via `user.Current().Username`, fallback to `USER` env var, then `"unknown"`) |
| `workflow_name` | string | yes | `"deploy-app"` | Workflow name from YAML definition |
| `schema_version` | integer | yes | `1` | Schema version for forward compatibility; currently always `1` |

### `workflow.started` Event

Emitted when a workflow execution begins, capturing **intent** (before execution starts).

**Additional Fields:**

| Field | Type | Required | Example | Notes |
|-------|------|----------|---------|-------|
| `inputs` | object | yes | `{"env":"staging","api_key":"***"}` | Key-value map of workflow inputs from `--input` flags. Secret-pattern keys masked as `"***"`. If entry would exceed 4KB, input values are truncated progressively (longest first) and `inputs_truncated` flag set to `true`. |
| `inputs_truncated` | boolean | no | `true` | Present only if input values were truncated to stay under 4KB limit |

**Minimal Example:**

```json
{
  "event": "workflow.started",
  "execution_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-02-20T23:15:42.123+01:00",
  "user": "deploy-bot",
  "workflow_name": "deploy-app",
  "inputs": {},
  "schema_version": 1
}
```

**With Inputs:**

```json
{
  "event": "workflow.started",
  "execution_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-02-20T23:15:42.123+01:00",
  "user": "deploy-bot",
  "workflow_name": "deploy-app",
  "inputs": {
    "env": "staging",
    "region": "us-east-1",
    "api_key": "***",
    "database_password": "***"
  },
  "schema_version": 1
}
```

### `workflow.completed` Event

Emitted when a workflow execution ends (success or failure), capturing **outcome**.

**Additional Fields:**

| Field | Type | Required | Example | Notes |
|-------|------|----------|---------|-------|
| `status` | string | yes | `"success"` or `"failure"` | Final execution status |
| `exit_code` | integer | yes | `0` (success) or non-zero (failure) | Process exit code |
| `duration_ms` | integer | yes | `30333` | Elapsed milliseconds from execution start to completion |
| `error` | string | no | `"step 'deploy' failed: connection timeout"` | Error description if status is `"failure"`. Omitted if status is `"success"` (not empty string). |

**Success Example:**

```json
{
  "event": "workflow.completed",
  "execution_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-02-20T23:16:12.456+01:00",
  "user": "deploy-bot",
  "workflow_name": "deploy-app",
  "status": "success",
  "exit_code": 0,
  "duration_ms": 30333,
  "schema_version": 1
}
```

**Failure Example:**

```json
{
  "event": "workflow.completed",
  "execution_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-02-20T23:16:12.456+01:00",
  "user": "deploy-bot",
  "workflow_name": "deploy-app",
  "status": "failure",
  "exit_code": 3,
  "duration_ms": 30333,
  "error": "step 'deploy' failed: connection timeout",
  "schema_version": 1
}
```

---

## Field Constraints

### execution_id

- **Format**: UUID v4 (RFC 4122)
- **Uniqueness**: One per workflow execution
- **Correlation**: Both `workflow.started` and `workflow.completed` for the same execution share the same `execution_id`
- **Usage**: `jq 'select(.execution_id == "550e8400-e29b-41d4-a716-446655440000")' audit.jsonl` to find all events for one execution

### timestamp

- **Format**: ISO 8601 with timezone: `YYYY-MM-DDTHH:MM:SS.sssZ±HH:MM`
- **Precision**: Millisecond (3 decimal places for fractional seconds)
- **Timezone**: Always includes offset (e.g., `+01:00`, `-05:00`, `Z` for UTC)
- **Examples**: `2026-02-20T23:15:42.123+01:00`, `2026-02-20T04:15:42.000Z`

### user

- **Derivation**:
  1. Try `user.Current().Username` (POSIX)
  2. Fallback to `USER` environment variable
  3. Fallback to `"unknown"` if both fail
- **Behavior with sudo**: Records the *effective* user (user after `sudo`), not the original user who invoked `sudo`
- **Example**: Running `sudo awf run workflow` records `user: "root"` (the effective user)

### inputs (masking rules)

Keys matching these patterns have values masked as `"***"`:

- `SECRET_*` (case-insensitive)
- `API_KEY*` (case-insensitive)
- `PASSWORD*` (case-insensitive)
- `TOKEN*` (case-insensitive)

**Examples:**

| Original Input | Masked Value |
|---|---|
| `api_key: "sk-1234567890abcdef"` | `"api_key": "***"` |
| `APIKey: "sk-1234567890abcdef"` | `"APIKey": "***"` |
| `secret_token: "ghp_abcdef123456"` | `"secret_token": "***"` |
| `normal_input: "value"` | `"normal_input": "value"` |

### Entry Size (4KB Limit)

- **Constraint**: Each serialized JSON entry MUST NOT exceed 4KB (4096 bytes)
- **Rationale**: Guarantees atomic writes under POSIX `PIPE_BUF` without file locking
- **Truncation**: If an entry would exceed 4KB:
  1. Input values are truncated progressively (longest values first)
  2. `inputs_truncated: true` flag is added to the entry
  3. The entry is still written (truncated state recorded for audit trail)

**Truncation Example:**

```json
{
  "event": "workflow.started",
  "execution_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-02-20T23:15:42.123+01:00",
  "user": "deploy-bot",
  "workflow_name": "deploy-app",
  "inputs": {
    "large_config": "[truncated: 8192 bytes]",
    "api_key": "***"
  },
  "inputs_truncated": true,
  "schema_version": 1
}
```

---

## Format Guarantees

### JSONL (JSON Lines)

- One JSON object per line (delimited by `\n`)
- Each line is a complete, valid JSON object
- Lines are ordered chronologically (earliest first)
- No JSON array wrapper; enables append-only writes and `tail -f`

### Encoding

- **Character encoding**: UTF-8
- **Line endings**: LF (`\n`) only, not CRLF
- **Special characters**: All JSON control characters properly escaped (e.g., newlines as `\n`, quotes as `\"`)

### Atomicity & Concurrency

- Each entry is written as a single atomic operation (single `write()` syscall)
- Multiple `awf` processes can append concurrently without file locking (POSIX `O_APPEND` semantics)
- Entries are never partially written (guaranteed under 4KB on POSIX systems)

---

## Backward Compatibility

### Schema Versioning

The `schema_version` field enables forward compatibility:

- **Current**: `1`
- **Future**: When schema changes (e.g., new fields added), version increments
- **Consumers**: Should branch on `schema_version` to handle multiple formats
- **Migration**: No migration tools needed; old and new entries coexist in the same file

**Example consumer logic:**

```go
switch entry.SchemaVersion {
case 1:
    // Handle schema v1
case 2:
    // Handle schema v2 (hypothetical)
}
```

### Field Evolution

- **Adding new fields**: Safe — old consumers ignore unknown fields (JSON standard)
- **Removing fields**: Unsafe — old format incompatible with new schema
- **Renaming fields**: Requires schema version bump
- **Changing field types**: Requires schema version bump

---

## Querying Examples

### Parse and Pretty-Print

```bash
jq '.' ~/.local/share/awf/audit.jsonl
```

### Filter by Status

```bash
# Failed executions only
jq 'select(.status == "failure")' ~/.local/share/awf/audit.jsonl

# Successful executions only
jq 'select(.status == "success")' ~/.local/share/awf/audit.jsonl
```

### Group by Execution ID

```bash
# Verify paired entries (should be 2 per execution)
jq -s 'group_by(.execution_id) | map(length)' ~/.local/share/awf/audit.jsonl

# Extract pairs with durations
jq -s '
  group_by(.execution_id)
  | map(select(length == 2) | {
      execution_id: .[0].execution_id,
      workflow_name: .[0].workflow_name,
      duration_ms: .[1].duration_ms,
      status: .[1].status,
      user: .[0].user
    })
' ~/.local/share/awf/audit.jsonl
```

### Detect Incomplete Executions

```bash
# Orphaned start entries (no matching completed)
jq -s '
  group_by(.execution_id)
  | map(select(length == 1) | .[0])
  | map({execution_id, workflow_name, timestamp, user})
' ~/.local/share/awf/audit.jsonl
```

---

## See Also

- [Audit Trail Guide](../user-guide/audit-trail.md) - Configuration and usage guide
- [ADR-010: Paired JSONL Audit Trail with Atomic Append](../ADR/010-paired-jsonl-audit-trail-with-atomic-append.md)
- [ADR-011: Application-Layer Secret Masking for Audit Events](../ADR/011-application-layer-secret-masking-for-audit-events.md)
