---
title: "Distributed Tracing"
---

AWF integrates with OpenTelemetry to provide visibility into workflow execution. Enable distributed tracing to export spans to any OTLP-compatible backend (Jaeger, Grafana Tempo, Honeycomb, Datadog, or any other observability platform) and visualize workflow execution flow, identify slow steps, and diagnose failures.

## How It Works

When tracing is enabled, AWF emits spans for:

- **Workflow execution** — Root span capturing the entire workflow run
- **Individual steps** — Child spans for each step with duration and status
- **Agent calls** — LLM invocations with provider, model, and token usage
- **Parallel/loop blocks** — Nested spans showing concurrent or iterative execution
- **Shell commands** — Low-level system execution with exit codes
- **Plugin operations** — gRPC calls to external plugins

All spans are automatically propagated through context and exported to your configured backend without blocking workflow execution.

## Quick Start

### 1. Start Jaeger

The project includes a `compose.yaml` with a pre-configured Jaeger instance:

```bash
docker compose up -d
```

This exposes:
- **Jaeger UI:** http://localhost:16686 (view traces)
- **OTLP gRPC Endpoint:** localhost:4317 (receive spans)

### 2. Enable Tracing

**Option A: Project configuration (recommended)**

Add to `.awf/config.yaml`:

```yaml
telemetry:
  exporter: "localhost:4317"
  service_name: "my-app"
```

Then run workflows as usual — tracing is automatic:

```bash
awf run my-workflow
```

**Option B: CLI flags**

```bash
awf run my-workflow --otel-exporter=localhost:4317 --otel-service-name=my-app
```

### 3. View Traces

Open http://localhost:16686, select your service (`my-app`), and inspect the trace waterfall.

## Configuration

### Project Configuration (recommended)

Add a `telemetry` section to `.awf/config.yaml` to enable tracing for all workflows in the project:

```yaml
telemetry:
  exporter: "localhost:4317"
  service_name: "my-app"
```

| Key | Default | Description |
|-----|---------|-------------|
| `exporter` | *(empty)* | OTLP gRPC endpoint. Empty or omitted disables tracing (zero overhead). |
| `service_name` | `awf` | Service name for resource attributes in your observability backend. |

This is the recommended approach for development — the configuration is committed with the project and applies to every `awf run` without additional flags.

### CLI Flags

CLI flags override the project configuration:

```bash
awf run <workflow> \
  --otel-exporter=localhost:4317 \
  --otel-service-name=my-service
```

**`--otel-exporter`** — OTLP gRPC endpoint (default: empty, tracing disabled)
- Omitted or empty — Uses project config value, or disables tracing if not configured
- `localhost:4317` — Local Jaeger or OTLP collector
- `collector.example.com:4317` — Remote collector
- Prefix with `https://` for TLS; `http://` or bare host defaults to insecure

**`--otel-service-name`** — Service name for resource attributes (default: `awf`)
- Used to identify your service in observability backends
- Example: `staging-workflows`, `prod-executor`, `ml-pipeline`

### Priority Order

Tracing configuration is resolved in this order (later sources override earlier ones):

```
Project Config (.awf/config.yaml) < CLI Flags (--otel-*)
```

To temporarily disable tracing when it is enabled in the project config, pass an empty exporter:

```bash
awf run my-workflow --otel-exporter=""
```

### Environment Variables

Standard OpenTelemetry environment variables are respected by the underlying SDK:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://collector.example.com:4317
export OTEL_SERVICE_NAME=my-app

awf run my-workflow --otel-exporter=collector.example.com:4317
```

If `--otel-service-name` is provided, it takes precedence over `OTEL_SERVICE_NAME`.

## Span Structure

### Root Span: `workflow.run`

The root span represents the entire workflow execution.

**Attributes:**
- `workflow.name` — Workflow name
- `workflow.version` — Workflow version (if defined in YAML)
- `execution_id` — Unique execution ID
- `user` — User who initiated the workflow

**Example:**
```json
{
  "traceID": "4bf92f3577b34da6...",
  "spanID": "3aa7e3a476d566f6",
  "name": "workflow.run",
  "attributes": {
    "workflow.name": "data-pipeline",
    "workflow.version": "1.2.0",
    "execution_id": "550e8400-e29b-41d4-a716-446655440000",
    "user": "deploy-bot"
  },
  "startTime": "2026-02-20T15:30:00Z",
  "endTime": "2026-02-20T15:30:45Z",
  "status": "OK"
}
```

### Step Spans: `step.<name>`

Each step produces a child span.

**Attributes:**
- `step.name` — Step name from workflow YAML
- `step.type` — Step type (`step`, `parallel`, `loop`, `agent`, `operation`, etc.)

**Example:**
```json
{
  "name": "step.validate_input",
  "attributes": {
    "step.name": "validate_input",
    "step.type": "step"
  },
  "parentSpanID": "3aa7e3a476d566f6",
  "status": "OK"
}
```

### Agent Call Spans: `agent.call`

LLM invocations produce a child span under the step.

**Attributes:**
- `agent.provider` — Provider name (`claude`, `gemini`, `codex`, `openai`, etc.)
- `agent.model` — Model identifier
- `agent.tokens_used` — Total tokens used by the call

**Example:**
```json
{
  "name": "agent.call",
  "attributes": {
    "agent.provider": "claude",
    "agent.model": "claude-opus-4-1",
    "agent.tokens_used": 1250
  },
  "parentSpanID": "...",
  "status": "OK"
}
```

### Parallel Block Spans: `parallel`

Concurrent steps produce a parent span with overlapping child spans.

**Attributes:**
- `parallel.strategy` — Execution strategy (`all_succeed`, `any_succeed`, `best_effort`)
- `parallel.branches` — Number of concurrent branches

**Example:**
```json
{
  "name": "parallel",
  "attributes": {
    "step.name": "process_shards",
    "parallel.strategy": "all_succeed",
    "parallel.branches": 4
  },
  "startTime": "2026-02-20T15:30:10Z",
  "endTime": "2026-02-20T15:30:25Z",
  "children": [
    { "name": "step.process_shard_0" },
    { "name": "step.process_shard_1" },
    { "name": "step.process_shard_2" },
    { "name": "step.process_shard_3" }
  ]
}
```

### Loop Spans: `loop.for_each` / `loop.while`

Loops produce a parent span with child spans for each iteration.

**Attributes:**
- `loop.type` — Loop type (`for_each`, `while`)
- `loop.iterations` — Total number of iterations (for completed loops)

**Example:**
```json
{
  "name": "loop.for_each",
  "attributes": {
    "step.name": "process_items",
    "loop.type": "for_each",
    "loop.iterations": 10
  },
  "children": [
    { "name": "loop.for_each.iteration", "attributes": { "iteration.index": 0 } },
    { "name": "loop.for_each.iteration", "attributes": { "iteration.index": 1 } },
    ...
  ]
}
```

### Shell Command Spans: `shell.execute`

Shell command execution produces a span with sanitized command details.

**Attributes:**
- `shell.command` — Command line (secrets masked)
- `shell.exit_code` — Exit code from the command

**Security Note:** Variable values matching secret patterns (`SECRET_*`, `API_KEY*`, `PASSWORD*`) are automatically masked as `***` in span attributes.

**Example:**
```json
{
  "name": "shell.execute",
  "attributes": {
    "shell.command": "curl -H 'Authorization: Bearer ***'",
    "shell.exit_code": 0
  }
}
```

### Plugin RPC Spans: `plugin.rpc`

Plugin operations produce a span capturing the gRPC call.

**Attributes:**
- `plugin.name` — Plugin name
- `rpc.method` — gRPC method name

**Example:**
```json
{
  "name": "plugin.rpc",
  "attributes": {
    "plugin.name": "github",
    "step.name": "create_issue"
  }
}
```

## Backends

### Jaeger (Local Development)

**Setup with compose.yaml (included in this project):**

```bash
docker compose up -d
```

Or standalone:

```bash
docker run -d \
  --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest
```

Then enable tracing in `.awf/config.yaml`:

```yaml
telemetry:
  exporter: "localhost:4317"
```

**View traces:** http://localhost:16686

### Grafana Tempo

**Setup:**

```bash
awf run my-workflow \
  --otel-exporter=https://tempo.example.com:4317 \
  --otel-service-name=my-app
```

Traces appear in Grafana under the specified service name.

### Honeycomb

**Setup:**

```bash
export OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=<your-api-key>

awf run my-workflow \
  --otel-exporter=https://api.honeycomb.io:443 \
  --otel-service-name=my-app
```

### Datadog

**Setup:**

```bash
export OTEL_EXPORTER_OTLP_HEADERS=dd-api-key=<your-api-key>
export DD_ENV=production
export DD_VERSION=1.0.0

awf run my-workflow \
  --otel-exporter=https://opentelemetry-collector-http.datadoghq.com:443 \
  --otel-service-name=my-app
```

## Real-World Example

**Workflow YAML:**

```yaml
name: data-pipeline
version: "1.0.0"

inputs:
  - name: data_source
    type: string
    default: production

states:
  initial: validate
  validate:
    type: step
    command: echo "Validating {{.inputs.data_source}}"
    on_success: fetch_data
  fetch_data:
    type: step
    command: curl https://data.example.com/{{.inputs.data_source}}
    on_success: process
  process:
    type: parallel
    strategy: all_succeed
    steps:
      - name: parse
        command: jq . > parsed.json
      - name: compress
        command: gzip parsed.json
    on_success: done
  done:
    type: terminal
    status: success
```

**Enable tracing in `.awf/config.yaml`:**

```yaml
telemetry:
  exporter: "localhost:4317"
  service_name: "etl-pipeline"
```

**Run:**

```bash
awf run data-pipeline --input data_source=sales
```

**Expected trace structure in Jaeger:**

```
workflow.run [data-pipeline]
├── step.validate [0.5s]
├── step.fetch_data [2.3s]
├── parallel [1.8s]
│   ├── step.parse [1.2s]
│   └── step.compress [1.8s]
```

## Troubleshooting

### Traces not appearing

1. **Verify exporter is running:**
   ```bash
   curl -X POST http://localhost:4317/v1/traces -d '{}' -v
   ```
   Should not return connection refused.

2. **Check endpoint configuration:**
   ```bash
   export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
   awf run my-workflow --otel-exporter=otlp -v
   ```

3. **Verify service name:**
   - In Jaeger UI, the service dropdown lists all services that sent spans
   - If your service doesn't appear, the endpoint may not be receiving spans

### Network issues

If the OTLP endpoint is unreachable:
- Tracing failures are logged as warnings
- Workflow execution continues normally (NFR-003 graceful degradation)
- Spans are dropped silently to avoid blocking the workflow

No errors are raised — tracing is designed to be non-disruptive.

### Too many spans

Large workflows (100+ steps) can generate thousands of spans. Most backends handle this efficiently, but if you experience performance issues:

1. Filter steps using `--dry-run` to verify the execution plan
2. Export to a local Jaeger instance first for development
3. Use backend sampling rules to reduce ingestion volume in production

## Performance

- **Overhead:** < 5% for workflows under 100 steps when tracing is enabled
- **Disabled (default):** Zero measurable overhead when `--otel-exporter=none` or flag omitted
- **Shutdown:** Pending spans are flushed within 5 seconds on process exit

## See Also

- [Audit Trail](audit-trail.md) — Structured JSONL execution logs (complementary to tracing)
- [Commands Reference](commands.md) — Full CLI flag documentation
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/) — Advanced configuration and custom instrumentation
