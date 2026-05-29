---
title: "HTTP API"
description: "AWF HTTP server for remote workflow execution and monitoring"
lead: "Run and monitor AWF workflows over HTTP with REST endpoints and Server-Sent Events streaming"
---

## Overview

The AWF HTTP API server lets you trigger, monitor, and manage workflow executions remotely without shelling out to the CLI. Perfect for integrating AWF into CI/CD pipelines, web dashboards, IDE extensions, or any external system that needs to orchestrate workflows over HTTP.

**Key features:**
- **REST endpoints** for workflow discovery, validation, and execution
- **Server-Sent Events (SSE)** for real-time step-by-step execution monitoring
- **Auto-generated OpenAPI 3.1 specification** at `/openapi.json` with Swagger UI at `/docs`
- **Async execution** — start a workflow and check progress via polling or SSE stream
- **Execution history and statistics** for audit and analytics

## Starting the Server

```bash
awf serve
```

**Flags:**
- `--port <int>` — Port to bind on (default: `2511`)
- `--host <string>` — Host to bind on (default: `127.0.0.1`)

**Example:**
```bash
# Start on default localhost:2511
awf serve

# Start on a specific port
awf serve --port 8080

# Bind to all interfaces (use at your own risk in production)
awf serve --host 0.0.0.0 --port 8080
```

Once running:
- **Swagger UI**: `http://localhost:2511/docs`
- **OpenAPI spec**: `http://localhost:2511/openapi.json`
- **API endpoints**: `http://localhost:2511/api/workflows`, `/api/executions`, `/api/history`

## Endpoints

Workflows are identified by a `(scope, name)` tuple in the URL path:
- **Local workflows**: `scope = "local"`, e.g., `GET /api/workflows/local/deploy-prod`
- **Pack workflows**: `scope = "<pack-name>"`, e.g., `GET /api/workflows/speckit/specify`

This two-segment grammar replaces the prior single-segment `{name}` placeholder, enabling support for pack workflows which previously resolved with URL mismatches. The tokens `local` and `global` are **reserved scope sentinels** — pack manifests cannot use them as `name`. See [Workflow Packs — Reserved Pack Names](workflow-packs.md#reserved-pack-names).

### Workflow Discovery & Validation

#### List workflows

```http
GET /api/workflows
```

**Response (200 OK):**
```json
{
  "body": {
    "workflows": [
      {
        "name": "code-review",
        "scope": "local",
        "workflow": "code-review",
        "version": "1.0.0",
        "description": "Review code for bugs and security issues"
      },
      {
        "name": "speckit/specify",
        "scope": "speckit",
        "workflow": "specify",
        "version": "1.0.0",
        "description": "Specification-driven workflow"
      }
    ]
  }
}
```

**Fields:**
- `name` — Canonical workflow identifier (`scope/workflow` for packs, plain name for local)
- `scope` — Scope token (`local` for non-pack, pack name for pack workflows)
- `workflow` — Local part of the workflow name (without scope prefix)
- `version` — Semantic version
- `description` — Brief description

Clients build operation URLs from the `scope` and `workflow` fields: `GET /api/workflows/{scope}/{workflow}`.

#### Get workflow details (local)

```http
GET /api/workflows/local/{name}
```

**Example:**
```bash
curl http://localhost:2511/api/workflows/local/deploy-prod
```

**Response (200 OK):**
```json
{
  "body": {
    "name": "deploy-prod",
    "version": "1.0.0",
    "description": "Deploy application to production",
    "states": {
      "initial": "build",
      "build": {
        "type": "step",
        "command": "go build ./cmd/app"
      }
    }
  }
}
```

#### Get workflow details (pack)

```http
GET /api/workflows/{pack}/{name}
```

**Example:**
```bash
curl http://localhost:2511/api/workflows/speckit/specify
```

**Response (200 OK):**
```json
{
  "body": {
    "name": "specify",
    "version": "1.0.0",
    "description": "Specification-driven workflow",
    "states": {
      "initial": "read",
      "read": {
        "type": "step",
        "command": "cat {{inputs.file}}"
      }
    }
  }
}
```

**Error (404 Not Found):**
```json
{
  "status": 404,
  "title": "Not Found",
  "detail": "workflow not found"
}
```

Returned uniformly for: unknown scopes, unknown workflows within a known scope, and unknown packs.

#### Validate workflow (local)

```http
POST /api/workflows/local/{name}/validate
```

**Example:**
```bash
curl -X POST http://localhost:2511/api/workflows/local/deploy-prod/validate
```

**Response (200 OK — valid workflow):**
```json
{
  "body": {
    "errors": []
  }
}
```

#### Validate workflow (pack)

```http
POST /api/workflows/{pack}/{name}/validate
```

**Example:**
```bash
curl -X POST http://localhost:2511/api/workflows/speckit/specify/validate
```

**Response (200 OK — invalid workflow):**
```json
{
  "body": {
    "errors": [
      "state 'invalid_ref' references undefined state"
    ]
  }
}
```

### Workflow Execution

#### Run workflow (local)

```http
POST /api/workflows/local/{name}/run
Content-Type: application/json

{
  "inputs": {
    "file": "main.go",
    "model": "claude-opus-4-20250805"
  }
}
```

**Example:**
```bash
curl -X POST http://localhost:2511/api/workflows/local/code-review/run \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"file": "main.go"}}'
```

**Response (202 Accepted):**
```json
{
  "body": {
    "execution_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "accepted"
  }
}
```

The workflow begins execution asynchronously. Use the `execution_id` to monitor progress via the events endpoint or polling.

#### Run workflow (pack)

```http
POST /api/workflows/{pack}/{name}/run
Content-Type: application/json

{
  "inputs": {
    "feature": "F101"
  }
}
```

**Example:**
```bash
curl -X POST http://localhost:2511/api/workflows/speckit/specify/run \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"feature": "F101"}}'
```

**Response (202 Accepted):**
```json
{
  "body": {
    "execution_id": "661f9600-e39c-52e5-c827-557766552000",
    "status": "accepted"
  }
}
```

**Error (404 Not Found):**
```json
{
  "status": 404,
  "title": "Not Found",
  "detail": "workflow not found"
}
```

Returned when:
- The scope (pack name or `local`) does not exist
- The workflow name does not exist in that scope
- The pack is not installed

**Error (422 Unprocessable Entity):**
```json
{
  "status": 422,
  "title": "Unprocessable Entity",
  "detail": "failed to start execution: <reason>"
}
```

#### List executions

```http
GET /api/executions
```

**Response (200 OK):**
```json
{
  "body": {
    "executions": [
      {
        "execution_id": "550e8400-e29b-41d4-a716-446655440000",
        "workflow_name": "code-review",
        "status": "running",
        "current_step": "analyze",
        "started_at": "2026-05-17T20:39:00Z",
        "updated_at": "2026-05-17T20:39:05Z"
      }
    ]
  }
}
```

#### Get execution details

```http
GET /api/executions/{id}
```

**Response (200 OK):**
```json
{
  "body": {
    "execution_id": "550e8400-e29b-41d4-a716-446655440000",
    "workflow_name": "code-review",
    "status": "running",
    "current_step": "analyze",
    "started_at": "2026-05-17T20:39:00Z",
    "updated_at": "2026-05-17T20:39:05Z"
  }
}
```

**Error (404 Not Found):**
```json
{
  "status": 404,
  "title": "Not Found",
  "detail": "execution not found: <id>"
}
```

#### Cancel execution

```http
DELETE /api/executions/{id}
```

**Response (204 No Content)**

Cancels the running workflow. The execution transitions to `cancelled` state and all running steps are terminated gracefully. Idempotent — returns 204 even if the execution does not exist.

#### Resume failed execution

```http
POST /api/executions/{id}/resume
Content-Type: application/json

{
  "input_overrides": {"model": "claude-sonnet-4-20250514"},
  "from_step": "analyze"
}
```

**Response (200 OK):**
```json
{
  "body": {
    "execution_id": "661f9500-f30c-52e5-c827-557766551111",
    "status": "accepted"
  }
}
```

A new execution ID is assigned to the resumed run. Monitor progress via the new ID.

**Error (404 Not Found):**
```json
{
  "status": 404,
  "title": "Not Found",
  "detail": "execution not found or cannot be resumed: <id>"
}
```

### Real-Time Event Streaming

#### Stream execution events (Server-Sent Events)

```http
GET /api/executions/{id}/events
Accept: text/event-stream
```

This endpoint returns a Server-Sent Events stream. Keep the connection open to receive real-time step updates as the workflow executes. The stream closes automatically when the workflow reaches a terminal state (completed, failed, or cancelled).

**Event stream example:**
```
event: step.started
data: {"step_name":"read","status":"running","started_at":"2026-05-17T20:39:00Z"}

event: step.completed
data: {"step_name":"read","status":"completed","output":"package main...","completed_at":"2026-05-17T20:39:02Z"}

event: step.started
data: {"step_name":"analyze","status":"running","started_at":"2026-05-17T20:39:02Z"}

event: step.failed
data: {"step_name":"analyze","status":"failed","error":"timeout exceeded","completed_at":"2026-05-17T20:40:15Z"}

event: workflow.failed
data: {"workflow_name":"code-review","status":"failed","completed_at":"2026-05-17T20:40:15Z"}
```

**Event types:**

| Event | Description | Fields |
|-------|-------------|--------|
| `step.started` | Step execution began | `step_name`, `status`, `started_at` |
| `step.completed` | Step finished successfully | `step_name`, `status`, `output`, `completed_at` |
| `step.failed` | Step execution failed | `step_name`, `status`, `error`, `completed_at` |
| `workflow.completed` | Workflow completed | `workflow_name`, `status`, `completed_at` |
| `workflow.failed` | Workflow failed or cancelled | `workflow_name`, `status`, `error`, `completed_at` |
| `output` | Incremental step output | `step_name`, `output` |

**Polling interval:** Events are emitted every ~200ms as state transitions occur.

**Error (404 Not Found):**
Stream returns 404 before opening if the execution does not exist.

### Execution History & Statistics

#### List historical executions

```http
GET /api/history?workflow=code-review&status=failed&limit=50
```

**Query parameters:**
- `workflow` — Filter by workflow name (optional)
- `status` — Filter by status: `success`, `failed`, `cancelled` (optional)
- `since` — Start date, RFC 3339 (optional)
- `until` — End date, RFC 3339 (optional)
- `limit` — Max results (optional)

**Response (200 OK):**
```json
{
  "body": {
    "entries": [
      {
        "id": "rec-abc123",
        "workflow_name": "code-review",
        "status": "failed",
        "started_at": "2026-05-16T15:30:00Z",
        "completed_at": "2026-05-16T15:31:00Z",
        "duration_ms": 60000
      },
      {
        "id": "rec-def456",
        "workflow_name": "code-review",
        "status": "success",
        "started_at": "2026-05-16T14:20:00Z",
        "completed_at": "2026-05-16T14:21:00Z",
        "duration_ms": 60000
      }
    ]
  }
}
```

#### Get execution statistics

```http
GET /api/history/stats?workflow=code-review
```

**Query parameters:**
- `workflow` — Filter by workflow name (optional)
- `status` — Filter by status (optional)
- `since` — Start date, RFC 3339 (optional)
- `until` — End date, RFC 3339 (optional)

**Response (200 OK):**
```json
{
  "body": {
    "TotalExecutions": 142,
    "SuccessCount": 128,
    "FailedCount": 12,
    "CancelledCount": 2,
    "AvgDurationMs": 45000
  }
}
```

## OpenAPI Specification

The API serves an auto-generated OpenAPI 3.1 specification:

```bash
# Download OpenAPI specification
curl http://localhost:2511/openapi.json

# Or YAML format
curl http://localhost:2511/openapi.yaml

# View Swagger UI in browser
open http://localhost:2511/docs
```

## Client Libraries & Integration

### cURL

**Run a local workflow:**
```bash
# Run a local workflow
RESULT=$(curl -s -X POST http://localhost:2511/api/workflows/local/code-review/run \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"file": "main.go"}}')

EXEC_ID=$(echo $RESULT | jq -r '.body.execution_id')

# Stream events
curl -N http://localhost:2511/api/executions/$EXEC_ID/events

# Get status
curl http://localhost:2511/api/executions/$EXEC_ID
```

**Run a pack workflow:**
```bash
# Run a pack workflow
RESULT=$(curl -s -X POST http://localhost:2511/api/workflows/speckit/specify/run \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"feature": "F101"}}')

EXEC_ID=$(echo $RESULT | jq -r '.body.execution_id')

# Stream events
curl -N http://localhost:2511/api/executions/$EXEC_ID/events
```

### JavaScript/TypeScript

**Run a local workflow:**
```typescript
// Start execution
const response = await fetch('http://localhost:2511/api/workflows/local/code-review/run', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ inputs: { file: 'main.go' } })
});

const { body } = await response.json();
const executionId = body.execution_id;

// Stream events
const eventSource = new EventSource(
  `http://localhost:2511/api/executions/${executionId}/events`
);

eventSource.addEventListener('step.started', (event) => {
  console.log('Step started:', JSON.parse(event.data));
});

eventSource.addEventListener('workflow.completed', (event) => {
  console.log('Workflow done:', JSON.parse(event.data));
  eventSource.close();
});
```

**Run a pack workflow:**
```typescript
// Start pack workflow execution
const response = await fetch('http://localhost:2511/api/workflows/speckit/specify/run', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ inputs: { feature: 'F101' } })
});

const { body } = await response.json();
const executionId = body.execution_id;
```

### Python

**Run a local workflow:**
```python
import requests
import json

# Start execution
response = requests.post(
    'http://localhost:2511/api/workflows/local/code-review/run',
    json={'inputs': {'file': 'main.go'}}
)

execution_id = response.json()['body']['execution_id']

# Stream events
response = requests.get(
    f'http://localhost:2511/api/executions/{execution_id}/events',
    stream=True
)

for line in response.iter_lines():
    if line.startswith(b'event: '):
        event_type = line.decode().split(': ', 1)[1]
        print(f'Event: {event_type}')
```

**Run a pack workflow:**
```python
import requests

# Start pack workflow execution
response = requests.post(
    'http://localhost:2511/api/workflows/speckit/specify/run',
    json={'inputs': {'feature': 'F101'}}
)

execution_id = response.json()['body']['execution_id']
```

## Error Handling

All error responses follow RFC 7807 Problem Details format (provided by Huma):

```json
{
  "status": 422,
  "title": "Unprocessable Entity",
  "detail": "Human-readable error description"
}
```

**Common HTTP status codes:**
- `400` — Bad request (missing required field, invalid JSON)
- `404` — Resource not found (unknown workflow or execution ID)
- `422` — Unprocessable entity (valid JSON but semantic error)
- `500` — Internal server error

## Security Considerations

**Default behavior:**
- Server binds to `127.0.0.1:2511` by default — localhost only
- No authentication in v1 (assumes isolated network or reverse proxy)
- No HTTPS/TLS in the server (use a reverse proxy like nginx)

**For production deployments:**
1. Run behind a reverse proxy (nginx, HAProxy, etc.) with:
   - HTTPS/TLS termination
   - Authentication (OAuth, API key, mutual TLS)
   - Rate limiting
   - Request logging
2. Use `--host 127.0.0.1` or `--host [::1]` to prevent accidental network exposure
3. Consider running in a container with restricted network access
4. Monitor `/api/executions` for long-running or stuck workflows
5. Configure appropriate timeouts for long-duration workflows

## Graceful Shutdown

The server listens for SIGINT and SIGTERM signals. On shutdown:

1. New requests return `503 Service Unavailable`
2. Active SSE streams are drained (30-second timeout)
3. Running workflows continue execution (separate from the HTTP server)
4. Server exits cleanly

To stop the server:
```bash
kill -TERM $(pgrep -f "awf serve")  # or Ctrl+C in foreground
```

## Examples

### Full workflow execution flow

**Local workflow example:**
```bash
# 1. Start server
awf serve --port 8080 &

# 2. List available workflows
curl http://localhost:8080/api/workflows

# 3. Validate a workflow before running
curl -X POST http://localhost:8080/api/workflows/local/code-review/validate

# 4. Start a workflow execution
RESPONSE=$(curl -s -X POST http://localhost:8080/api/workflows/local/code-review/run \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"file": "src/main.go"}}')

EXEC_ID=$(echo $RESPONSE | jq -r '.body.execution_id')
echo "Execution ID: $EXEC_ID"

# 5. Monitor execution in real-time via SSE
curl -N http://localhost:8080/api/executions/$EXEC_ID/events

# Or poll for status
curl http://localhost:8080/api/executions/$EXEC_ID | jq '.body | {status, current_step}'

# 6. Check execution history
curl "http://localhost:8080/api/history?workflow=code-review&limit=10"

# 7. Get statistics
curl http://localhost:8080/api/history/stats?workflow=code-review
```

**Pack workflow example:**
```bash
# 1. Start server with pack installed
awf workflow install myorg/awf-workflow-speckit
awf serve --port 8080 &

# 2. Run a pack workflow
RESPONSE=$(curl -s -X POST http://localhost:8080/api/workflows/speckit/specify/run \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"feature": "F101"}}')

EXEC_ID=$(echo $RESPONSE | jq -r '.body.execution_id')

# 3. Monitor the execution
curl -N http://localhost:8080/api/executions/$EXEC_ID/events
```

### Integrate with CI/CD (GitHub Actions)

```yaml
name: Code Review Workflow
on: [pull_request]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run AWF code review via API
        run: |
          # Run a local workflow via the API
          RESPONSE=$(curl -s -X POST http://awf-server:2511/api/workflows/local/code-review/run \
            -H "Content-Type: application/json" \
            -d '{"inputs": {"file": "src/main.go"}}')

          EXEC_ID=$(echo $RESPONSE | jq -r '.body.execution_id')

          # Poll until complete
          while true; do
            STATUS=$(curl -s http://awf-server:2511/api/executions/$EXEC_ID | jq -r '.body.status')
            [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ] && break
            sleep 2
          done

          # Get final status
          curl -s http://awf-server:2511/api/executions/$EXEC_ID | jq '.body'
```

## Performance & Limits

- **Default port**: `2511`
- **Event polling cadence**: 200ms (real-time within ~200ms)
- **Graceful shutdown timeout**: 30 seconds
- **Max concurrent SSE subscribers per execution**: 50+ (tested)
- **Response time**: Most endpoints respond within 100ms
- **Workflow execution**: Runs asynchronously; HTTP response is immediate (≤100ms)

## Troubleshooting

**Server won't start on port 2511**
```bash
# Check if port is in use
lsof -i :2511

# Use a different port
awf serve --port 8080
```

**SSE stream closes unexpectedly**
- Check network connectivity (firewall, proxy timeouts)
- Verify execution exists: `curl http://localhost:2511/api/executions/{id}`
- Check server logs for errors

**API returns 404 for a workflow**
- Verify workflow exists: `awf list` or `curl http://localhost:2511/api/workflows`
- Check workflow YAML syntax: `awf validate <workflow>`

**404 on `/` or `/api`**
- There is no root handler. Use specific endpoints: `/api/workflows`, `/api/executions`, `/api/history`
- Browse `/docs` for interactive Swagger UI

**Slow response times**
- Check server load: `curl http://localhost:2511/api/executions`
- Look for long-running workflows blocking execution
- Monitor reverse proxy (if using one) for bottlenecks
