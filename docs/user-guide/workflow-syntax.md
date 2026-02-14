# Workflow Syntax Reference

Complete reference for AWF workflow YAML syntax.

## Basic Structure

```yaml
name: my-workflow
version: "1.0.0"
description: Workflow description

inputs:
  - name: file_path
    type: string
    required: true

states:
  initial: step1

  step1:
    type: step
    command: echo "Hello"
    on_success: step2
    on_failure: error

  step2:
    type: step
    command: echo "World"
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
```

## Workflow Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Workflow identifier |
| `version` | string | No | Semantic version |
| `description` | string | No | Human-readable description |
| `inputs` | array | No | Input parameter definitions |
| `states` | object | Yes | State definitions |
| `states.initial` | string | Yes | Name of the starting state |

---

## State Types

| Type | Description |
|------|-------------|
| `step` | Execute a command |
| `agent` | Invoke an AI agent (Claude, Codex, Gemini, etc.) |
| `terminal` | End state with success/failure status |
| `parallel` | Execute multiple steps concurrently |
| `for_each` | Iterate over a list of items |
| `while` | Repeat until condition is false |
| `operation` | Execute a declarative plugin operation (e.g., HTTP, GitHub, notifications) |
| `call_workflow` | Invoke another workflow as a sub-workflow |

---

## Step State

Execute a shell command.

```yaml
my_step:
  type: step
  command: |
    echo "Processing {{.inputs.file}}"
  dir: /tmp/workdir
  timeout: 30
  on_success: next_step
  on_failure: error
  continue_on_error: false
  retry:
    max_attempts: 3
    backoff: exponential
```

### Step Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `command` | string | - | Shell command to execute |
| `dir` | string | cwd | Working directory (supports interpolation) |
| `timeout` | int | 0 | Execution timeout in seconds (0 = no timeout) |
| `on_success` | string | - | Next state on success (exit code 0) |
| `on_failure` | string | - | Next state on failure (exit code ≠ 0) |
| `continue_on_error` | bool | false | Always follow `on_success` regardless of exit code |
| `retry` | object | - | Retry configuration |
| `transitions` | array | - | Conditional transitions |

---

## Agent State

Invoke an AI agent (Claude, Codex, Gemini, OpenCode) with a prompt template.

### Basic Agent Step

```yaml
analyze:
  type: agent
  provider: claude
  prompt: |
    Analyze this code for issues:
    {{.inputs.code}}
  options:
    model: claude-sonnet-4-20250514
    max_tokens: 2048
  timeout: 120
  on_success: review
  on_failure: error
```

### Conversation Mode

Enable multi-turn conversations with automatic context management:

```yaml
refine_code:
  type: agent
  provider: claude
  mode: conversation
  system_prompt: |
    You are a code reviewer. Iterate until code is approved.
    Say "APPROVED" when done.
  initial_prompt: |
    Review this code:
    {{.inputs.code}}
  options:
    model: claude-sonnet-4-20250514
    max_tokens: 4096
  conversation:
    max_turns: 10
    max_context_tokens: 100000
    strategy: sliding_window
    stop_condition: "response contains 'APPROVED'"
  on_success: deploy
  on_failure: error
```

### Agent Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `provider` | string | Yes | Agent provider: `claude`, `codex`, `gemini`, `opencode`, `custom` |
| `mode` | string | No | Set to `conversation` for multi-turn mode |
| `prompt` | string | Yes* | Prompt template (supports `{{.inputs.*}}` and `{{.states.*}}` interpolation) |
| `system_prompt` | string | No | System message (for conversation mode, preserved across turns) |
| `initial_prompt` | string | No* | First user message (for conversation mode) |
| `conversation` | object | No | Conversation configuration (required if mode=conversation) |
| `options` | map | No | Provider-specific options (model, temperature, max_tokens, etc.) |
| `timeout` | int | No | Execution timeout in seconds (0 = no timeout) |
| `on_success` | string | No | Next state on success |
| `on_failure` | string | No | Next state on failure |
| `retry` | object | No | Retry configuration (same as step retry) |

\* Use `prompt` for single-turn mode, `initial_prompt` for conversation mode.

### Conversation Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_turns` | int | 10 | Maximum conversation turns |
| `max_context_tokens` | int | model limit | Token budget for conversation |
| `strategy` | string | `sliding_window` | Context window strategy |
| `stop_condition` | string | - | Expression to exit early |

### Available Providers

| Provider | Binary | Description |
|----------|--------|-------------|
| `claude` | `claude` | Anthropic Claude CLI |
| `codex` | `codex` | OpenAI Codex CLI |
| `gemini` | `gemini` | Google Gemini CLI |
| `opencode` | `opencode` | OpenCode CLI |
| `custom` | user-defined | Custom command template |

### Agent Output

Agent responses are captured in the step state:

| Field | Type | Description |
|-------|------|-------------|
| `{{.states.step_name.Output}}` | string | Raw response text |
| `{{.states.step_name.Response}}` | object | Parsed JSON (if response is valid JSON) |
| `{{.states.step_name.TokensUsed}}` | int | Tokens consumed by this agent step |

### Multi-Turn Conversations

**Recommended**: Use conversation mode for iterative workflows:

```yaml
review:
  type: agent
  provider: claude
  mode: conversation
  system_prompt: "You are a code reviewer."
  initial_prompt: "Review: {{.inputs.code}}"
  conversation:
    max_turns: 10
    stop_condition: "response contains 'APPROVED'"
  on_success: done
```

**Legacy**: Chain multiple agent steps with state passing:

```yaml
states:
  initial: ask_question

  ask_question:
    type: agent
    provider: claude
    prompt: "Initial question here"
    on_success: follow_up

  follow_up:
    type: agent
    provider: claude
    prompt: |
      Based on your previous response:
      {{.states.ask_question.Output}}

      Please elaborate on point 3.
    on_success: done

  done:
    type: terminal
```

**See Also:** [Conversation Mode Guide](conversation-steps.md) for detailed examples and best practices.

### Custom Provider

For AI CLIs not natively supported, use the `custom` provider with a command template:

```yaml
analyze:
  type: agent
  provider: custom
  command: "my-ai-tool --prompt {{prompt}} --json"
  prompt: "Analyze: {{.inputs.data}}"
  timeout: 60
  on_success: next
```

The `{{prompt}}` placeholder is replaced with the resolved prompt. Note that prompt text is automatically shell-escaped to prevent injection.

**See Also:** [Agent Steps Guide](agent-steps.md) for detailed examples and best practices.

---

## Operation State

Execute a declarative plugin operation. Operations provide structured access to external services (e.g., GitHub) without shell scripting. Inputs are validated against the operation schema and outputs are accessible via `{{.states.step_name.Response.field}}`.

### Basic Operation Step

```yaml
get_issue:
  type: operation
  operation: github.get_issue
  inputs:
    number: "{{.inputs.issue_number}}"
  on_success: process
  on_failure: error
```

### Operation Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `operation` | string | Yes | Operation name (e.g., `github.get_issue`) |
| `inputs` | map | Varies | Input parameters (validated against operation schema) |
| `on_success` | string | No | Next state on success |
| `on_failure` | string | No | Next state on failure |
| `retry` | object | No | Retry configuration (same as step retry) |

### Operation Output

Operation results are captured as structured data:

| Field | Type | Description |
|-------|------|-------------|
| `{{.states.step_name.Output}}` | string | Raw JSON response |
| `{{.states.step_name.Response.field}}` | any | Parsed field from structured output |

### Output Interpolation

Chain operations by referencing previous step outputs:

```yaml
states:
  initial: get_issue

  get_issue:
    type: operation
    operation: github.get_issue
    inputs:
      number: "{{.inputs.issue_number}}"
    on_success: show_title
    on_failure: error

  show_title:
    type: step
    command: echo "Issue: {{.states.get_issue.Response.title}}"
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
```

### GitHub Operations

AWF includes a built-in GitHub plugin with 9 declarative operations. Authentication is handled automatically via `gh` CLI or `GITHUB_TOKEN` environment variable. The repository is auto-detected from git remote when the `repo` input is omitted.

#### Issue & PR Operations

| Operation | Description | Required Inputs | Outputs |
|-----------|-------------|-----------------|---------|
| `github.get_issue` | Retrieve issue data | `number` | `number`, `title`, `body`, `state`, `labels` |
| `github.get_pr` | Retrieve pull request data | `number` | `number`, `title`, `body`, `state`, `headRefName`, `baseRefName`, `mergeable`, `mergedAt`, `labels` |
| `github.create_issue` | Create a new issue | `title` | `number`, `url` |
| `github.create_pr` | Create a new pull request | `title`, `head`, `base` | `number`, `url`, `already_exists` |
| `github.add_labels` | Add labels to issue or PR | `number`, `labels` | `labels` |
| `github.add_comment` | Add a comment | `number`, `body` | `comment_id`, `url` |
| `github.list_comments` | List comments | `number` | `comments`, `total` |

#### Project Operations

| Operation | Description | Required Inputs | Outputs |
|-----------|-------------|-----------------|---------|
| `github.set_project_status` | Set project field value | `number`, `project`, `field`, `value` | `project_id`, `item_id`, `field_name`, `value` |

#### Common Optional Inputs

All GitHub operations accept these optional inputs:

| Input | Type | Description |
|-------|------|-------------|
| `repo` | string | Repository in `owner/repo` format (auto-detected from git remote if omitted) |
| `fields` | array | Fields to include in output (limits data returned, supported by get operations) |

#### Examples

**Retrieve an issue:**

```yaml
get_issue:
  type: operation
  operation: github.get_issue
  inputs:
    number: 42
  on_success: next
  on_failure: error
```

**Create a pull request:**

```yaml
create_pr:
  type: operation
  operation: github.create_pr
  inputs:
    title: "feat: add login page"
    head: feature/login
    base: main
    body: "Implements the login UI"
    draft: true
  on_success: next
  on_failure: error
```

**Add labels to an issue:**

```yaml
label_issue:
  type: operation
  operation: github.add_labels
  inputs:
    number: "{{.inputs.issue_number}}"
    labels: ["bug", "priority-high"]
  on_success: done
  on_failure: error
```

### Batch Operations

Execute multiple GitHub operations concurrently using `github.batch`. Batch operations support configurable concurrency and failure strategies.

```yaml
label_multiple:
  type: operation
  operation: github.batch
  inputs:
    strategy: best_effort
    concurrency: 3
    operations:
      - name: github.add_labels
        number: 1
        labels: ["reviewed"]
      - name: github.add_labels
        number: 2
        labels: ["reviewed"]
      - name: github.add_labels
        number: 3
        labels: ["reviewed"]
  on_success: done
  on_failure: error
```

#### Batch Inputs

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `operations` | array | - | Array of operation definitions (each with `name` and operation-specific inputs) |
| `strategy` | string | `best_effort` | Execution strategy |
| `concurrency` | int | 3 | Maximum concurrent operations |

#### Batch Strategies

| Strategy | Description |
|----------|-------------|
| `all_succeed` | All operations must succeed; cancels remaining on first failure |
| `any_succeed` | Succeed if at least one operation succeeds |
| `best_effort` | Complete all operations, collect all results regardless of failures |

#### Batch Outputs

| Output | Type | Description |
|--------|------|-------------|
| `total` | int | Total operations attempted |
| `succeeded` | int | Successfully completed count |
| `failed` | int | Failed operation count |
| `results` | array | Individual operation results |

### Notification Operations

AWF includes a built-in notification provider with a single `notify.send` operation that dispatches to four backends. See [Plugins - Built-in Notification Plugin](plugins.md#built-in-notification-plugin) for configuration details.

#### notify.send

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| `backend` | string | Yes | Backend: `desktop`, `ntfy`, `slack`, `webhook` |
| `message` | string | Yes | Notification message body |
| `title` | string | No | Notification title (defaults to "AWF Workflow") |
| `priority` | string | No | Priority: `low`, `default`, `high` |
| `topic` | string | No | ntfy topic name (required for `ntfy` backend) |
| `webhook_url` | string | No | Webhook URL (required for `webhook` backend) |
| `channel` | string | No | Slack channel override |

**Outputs:** `backend`, `status`, `response`

#### Examples

**Desktop notification after a build:**

```yaml
states:
  initial: build

  build:
    type: step
    command: make build
    on_success: notify
    on_failure: error

  notify:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      title: "Build Complete"
      message: "{{workflow.name}} finished in {{workflow.duration}}"
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
```

**Push notification via ntfy:**

```yaml
notify_phone:
  type: operation
  operation: notify.send
  inputs:
    backend: ntfy
    topic: my-builds
    title: "Deploy Status"
    message: "{{workflow.name}}: {{states.deploy.Output}}"
    priority: high
  on_success: done
  on_failure: error
```

**Slack team notification:**

```yaml
notify_slack:
  type: operation
  operation: notify.send
  inputs:
    backend: slack
    title: "Workflow Complete"
    message: "{{workflow.name}} succeeded in {{workflow.duration}}"
  on_success: done
  on_failure: error
```

**Generic webhook (Discord, Teams, PagerDuty, etc.):**

```yaml
notify_webhook:
  type: operation
  operation: notify.send
  inputs:
    backend: webhook
    webhook_url: "https://example.com/hooks/builds"
    message: "{{workflow.name}} completed"
  on_success: done
  on_failure: error
```

### HTTP Operations

AWF includes a built-in HTTP operation provider for declarative REST API calls. The `http.request` operation supports GET, POST, PUT, and DELETE with configurable timeout and response capture. See [Plugins - Built-in HTTP Operation](plugins.md#built-in-http-operation) for configuration details.

#### http.request

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | HTTP endpoint URL (must start with `http://` or `https://`) |
| `method` | string | Yes | HTTP method: `GET`, `POST`, `PUT`, `DELETE` (case-insensitive) |
| `headers` | object | No | Custom headers as key-value pairs |
| `body` | string | No | Request body (for POST/PUT) |
| `timeout` | integer | No | Per-request timeout in seconds (default: 30) |
| `retryable_status_codes` | array | No | Status codes that signal retryable failures (e.g., `[429, 502, 503]`) |

**Outputs:** `status_code`, `body`, `headers`, `body_truncated`

#### Examples

**Simple GET request:**

```yaml
fetch_data:
  type: operation
  operation: http.request
  inputs:
    method: GET
    url: "https://api.example.com/users/{{.inputs.user_id}}"
    headers:
      Authorization: "Bearer {{.inputs.api_token}}"
      Accept: "application/json"
    timeout: 10
  on_success: process
  on_failure: error
```

**POST with JSON body and retry:**

```yaml
create_resource:
  type: operation
  operation: http.request
  inputs:
    method: POST
    url: "https://api.example.com/resources"
    headers:
      Content-Type: "application/json"
      Authorization: "Bearer {{.inputs.api_token}}"
    body: '{"name": "{{.inputs.resource_name}}"}'
    timeout: 15
    retryable_status_codes: [429, 502, 503]
  retry:
    max_attempts: 3
    backoff: exponential
    initial_delay_ms: 2000
  on_success: done
  on_failure: error
```

**Access response fields in subsequent steps:**

```yaml
states:
  initial: fetch_user

  fetch_user:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "https://api.example.com/users/1"
    on_success: show_result
    on_failure: error

  show_result:
    type: step
    command: |
      echo "Status: {{.states.fetch_user.Response.status_code}}"
      echo "Body: {{.states.fetch_user.Response.body}}"
      echo "Content-Type: {{.states.fetch_user.Response.headers.Content-Type}}"
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
```

**DELETE request (no body):**

```yaml
delete_resource:
  type: operation
  operation: http.request
  inputs:
    method: DELETE
    url: "https://api.example.com/resources/{{.inputs.resource_id}}"
    headers:
      Authorization: "Bearer {{.inputs.api_token}}"
  on_success: done
  on_failure: error
```

---

## Terminal State

End the workflow execution.

```yaml
done:
  type: terminal
  status: success

error:
  type: terminal
  status: failure
```

### Terminal Options

| Option | Type | Values | Description |
|--------|------|--------|-------------|
| `status` | string | `success`, `failure` | Terminal status |

---

## Parallel State

Execute multiple steps concurrently. Branch children are defined as separate states and referenced by name in the `parallel` field.

```yaml
parallel_build:
  type: parallel
  parallel:
    - lint
    - test
    - build
  strategy: all_succeed
  max_concurrent: 3
  on_success: deploy
  on_failure: error

lint:
  type: step
  command: golangci-lint run

test:
  type: step
  command: go test ./...

build:
  type: step
  command: go build ./cmd/...
```

Branch children (`lint`, `test`, `build` above) do not need `on_success`/`on_failure` or `transitions` — the parallel executor controls flow after each branch completes. If provided, transitions are accepted but ignored at runtime.

### Parallel Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `parallel` | array | - | List of step names to execute concurrently |
| `strategy` | string | `all_succeed` | Execution strategy |
| `max_concurrent` | int | unlimited | Maximum concurrent steps |
| `on_success` | string | - | Next state when all branches complete successfully |
| `on_failure` | string | - | Next state on branch failure |

### Parallel Strategies

| Strategy | Description |
|----------|-------------|
| `all_succeed` | All steps must succeed, cancel remaining on first failure |
| `any_succeed` | Succeed if at least one step succeeds |
| `best_effort` | Collect all results, never cancel early |

### Accessing Parallel Results

```yaml
# Branch children are top-level states — access their output directly
command: echo "{{.states.lint.Output}}"
```

---

## For-Each Loop

Iterate over a list of items.

```yaml
process_files:
  type: for_each
  items: '["a.txt", "b.txt", "c.txt"]'
  max_iterations: 100
  break_when: "states.process_single.ExitCode != 0"
  body:
    - process_single
  on_complete: aggregate

process_single:
  type: step
  command: |
    echo "Processing {{.loop.item}} ({{.loop.index1}}/{{.loop.length}})"
  on_success: process_files
```

### For-Each Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `items` | string | - | Template expression or literal JSON array |
| `body` | array | - | List of step names to execute each iteration |
| `max_iterations` | int/string | 100 | Safety limit (max: 10000). Supports interpolation. |
| `break_when` | string | - | Expression to exit loop early |
| `on_complete` | string | - | Next state after loop completes |

### Loop Context Variables

| Variable | Description |
|----------|-------------|
| `{{.loop.item}}` | Current item value |
| `{{.loop.index}}` | 0-based iteration index |
| `{{.loop.index1}}` | 1-based iteration index |
| `{{.loop.first}}` | True on first iteration |
| `{{.loop.last}}` | True on last iteration |
| `{{.loop.length}}` | Total items count |
| `{{.loop.parent}}` | Parent loop context (nested loops) |

### Dynamic Items

Items can come from a template expression:

```yaml
items: "{{.inputs.files}}"
```

### Dynamic Max Iterations

The `max_iterations` field supports interpolation and arithmetic expressions:

```yaml
# From input parameter
max_iterations: "{{.inputs.retry_count}}"

# From environment variable
max_iterations: "{{.env.MAX_RETRIES}}"

# Arithmetic expression
max_iterations: "{{.inputs.pages * .inputs.retries_per_page}}"
```

Supported arithmetic operators: `+`, `-`, `*`, `/`, `%`

Dynamic values are resolved at loop initialization time. Validation ensures:
- Result is a positive integer
- Value does not exceed 10000 (safety limit)

Use `awf validate` to detect undefined variables before runtime.

---

## While Loop

Repeat until condition becomes false.

```yaml
poll_status:
  type: while
  while: "states.check.Output != 'ready'"
  max_iterations: 60
  body:
    - check
    - wait
  on_complete: proceed

check:
  type: step
  command: curl -s https://api.example.com/status
  on_success: poll_status

wait:
  type: step
  command: sleep 5
  on_success: poll_status
```

### While Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `while` | string | - | Condition expression (loop while true). Supports interpolation. |
| `body` | array | - | List of step names to execute each iteration |
| `max_iterations` | int/string | 100 | Safety limit (max: 10000). Supports interpolation. |
| `break_when` | string | - | Expression to exit loop early |
| `on_complete` | string | - | Next state after loop completes |

### While Loop Context

| Variable | Description |
|----------|-------------|
| `{{.loop.index}}` | 0-based iteration index |
| `{{.loop.first}}` | True on first iteration |
| `{{.loop.length}}` | Always -1 (unknown for while loops) |

---

## Nested Loops

Loops can contain other loops. Inner loops access outer loop context via `{{.loop.parent.*}}`:

```yaml
outer_loop:
  type: for_each
  items: '["A", "B"]'
  body:
    - inner_loop
  on_complete: done

inner_loop:
  type: for_each
  items: '["1", "2"]'
  body:
    - process
  on_complete: outer_loop

process:
  type: step
  command: 'echo "outer={{.loop.parent.item}} inner={{.loop.item}}"'
  on_success: inner_loop
```

Parent chains support arbitrary depth: `{{.loop.parent.parent.item}}` for 3-level nesting.

---

## Call Workflow (Sub-Workflow)

Invoke another workflow as a sub-workflow, passing inputs and capturing outputs.

```yaml
analyze_code:
  type: call_workflow
  call_workflow:
    workflow: analyze-single-file
    inputs:
      file_path: "{{.inputs.target_file}}"
      max_tokens: "{{.inputs.max_tokens}}"
    outputs:
      result: analysis_result
    timeout: 300
  on_success: aggregate_results
  on_failure: handle_error
```

### Call Workflow Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `workflow` | string | - | Name of the workflow to invoke |
| `inputs` | map | - | Input mappings (parent var → child input) |
| `outputs` | map | - | Output mappings (child output → parent var) |
| `timeout` | int | 0 | Sub-workflow timeout in seconds (0 = inherit) |

### Child Workflow Definition

The child workflow must define its inputs and outputs:

```yaml
# analyze-single-file.yaml
name: analyze-single-file
version: "1.0.0"

inputs:
  - name: file_path
    type: string
    required: true
  - name: max_tokens
    type: integer
    default: 2000

states:
  initial: read
  read:
    type: step
    command: cat "{{.inputs.file_path}}"
    on_success: analyze
    on_failure: error
  analyze:
    type: step
    command: claude -c "Analyze: {{.states.read.Output}}"
    timeout: 120
    on_success: done
    on_failure: error
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure

outputs:
  - name: analysis_result
    from: states.analyze.Output
```

### Accessing Sub-Workflow Results

Outputs from the sub-workflow are accessible via the standard states interpolation:

```yaml
# In parent workflow, after analyze_code step
aggregate_results:
  type: step
  command: echo "Analysis: {{.states.analyze_code.Output}}"
  on_success: done
```

### Nested Sub-Workflows

Sub-workflows can call other sub-workflows. AWF tracks the call stack to detect circular references:

```yaml
# workflow-a.yaml calls workflow-b
# workflow-b.yaml calls workflow-c
# Supported: A → B → C (3-level nesting)
# Blocked: A → B → A (circular reference)
```

Maximum nesting depth is 10 levels. Circular calls are detected at runtime with clear error messages showing the call stack.

### Error Handling

Sub-workflow errors propagate to the parent:

- If sub-workflow reaches a `terminal` state with `status: failure`, parent follows `on_failure`
- If sub-workflow times out, parent receives timeout error and follows `on_failure`
- If sub-workflow definition is not found, execution fails with `undefined_subworkflow` error

### Using Call Workflow in Loops

Combine `for_each` with `call_workflow` to process multiple items in parallel sub-workflows. Loop items (especially complex objects) are automatically serialized to JSON:

```yaml
# Example: Process multiple files across sub-workflows
prepare_items:
  type: step
  command: |
    echo '[
      {"file":"main.go","language":"Go"},
      {"file":"app.py","language":"Python"},
      {"file":"index.js","language":"JavaScript"}
    ]'
  capture:
    stdout: items_json
  on_success: process_files

process_files:
  type: for_each
  items: "{{.states.prepare_items.Output}}"
  body:
    - analyze_file

analyze_file:
  type: call_workflow
  call_workflow:
    workflow: analyze-source-file
    inputs:
      # {{.loop.item}} is automatically JSON-serialized for complex types
      file_info: "{{.loop.item}}"
    outputs:
      analysis: file_analysis
  on_success: next

next:
  type: terminal
```

Child workflow receives properly formatted JSON input:
```yaml
name: analyze-source-file

inputs:
  - name: file_info
    type: string  # Receives JSON string

states:
  initial: parse
  parse:
    type: step
    command: |
      # Parse JSON input safely
      echo '{{.inputs.file_info}}' | jq -r '.file'
    on_success: done
  done:
    type: terminal

outputs:
  - name: file_analysis
    from: states.parse.Output
```

---

## Retry Configuration

Automatic retry for failed steps.

```yaml
flaky_api_call:
  type: step
  command: curl -f https://api.example.com/data
  retry:
    max_attempts: 5
    initial_delay: 1s
    max_delay: 30s
    backoff: exponential
    multiplier: 2
    jitter: 0.1
    retryable_exit_codes: [1, 22]
  on_success: process_data
  on_failure: error
```

### Retry Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_attempts` | int | 1 | Maximum attempts (1 = no retry) |
| `initial_delay` | duration | 0 | Delay before first retry |
| `max_delay` | duration | - | Maximum delay cap |
| `backoff` | string | `constant` | Strategy: `constant`, `linear`, `exponential` |
| `multiplier` | float | 2 | Multiplier for exponential backoff |
| `jitter` | float | 0 | Random jitter factor 0.0-1.0 |
| `retryable_exit_codes` | array | all | Exit codes to retry (empty = all non-zero) |

### Backoff Strategies

| Strategy | Formula |
|----------|---------|
| `constant` | Always `initial_delay` |
| `linear` | `initial_delay * attempt` |
| `exponential` | `initial_delay * multiplier^(attempt-1)` |

---

## Conditional Transitions

Dynamic branching based on expressions.

```yaml
process:
  type: step
  command: analyze.sh
  transitions:
    - when: "states.process.ExitCode == 0 and inputs.mode == 'full'"
      goto: full_report
    - when: "states.process.ExitCode == 0"
      goto: summary_report
    - goto: error  # default fallback (no when clause)
```

### Transition Options

| Option | Type | Description |
|--------|------|-------------|
| `when` | string | Expression to evaluate (optional for default) |
| `goto` | string | Target state if condition matches |

### Supported Operators

| Type | Operators |
|------|-----------|
| Comparison | `==`, `!=`, `<`, `>`, `<=`, `>=` |
| Logical | `and`, `or`, `not` |
| Grouping | `(expr)` |

### Available Variables

| Variable | Description |
|----------|-------------|
| `inputs.name` | Input values |
| `states.step_name.ExitCode` | Step exit code |
| `states.step_name.Output` | Step output |
| `env.VAR_NAME` | Environment variables |

Transitions are evaluated in order; first match wins. A transition without `when` acts as default fallback.

---

## Input Definitions

Define and validate workflow inputs.

```yaml
inputs:
  - name: file_path
    type: string
    required: true
    validation:
      file_exists: true
      file_extension: [".go", ".py", ".js"]

  - name: max_tokens
    type: integer
    default: 2000
    validation:
      min: 1
      max: 10000

  - name: env
    type: string
    default: staging
    validation:
      enum: [dev, staging, prod]

  - name: debug
    type: boolean
    default: false
```

### Input Options

| Option | Type | Description |
|--------|------|-------------|
| `name` | string | Input identifier |
| `type` | string | `string`, `integer`, `boolean` |
| `required` | bool | If true, must be provided |
| `default` | any | Default value if not provided |
| `validation` | object | Validation rules |

### Validation Rules

| Rule | Type | Description |
|------|------|-------------|
| `pattern` | string | Regex pattern to match |
| `enum` | array | List of allowed values |
| `min` | int | Minimum value (integers only) |
| `max` | int | Maximum value (integers only) |
| `file_exists` | bool | File must exist on filesystem |
| `file_extension` | array | Allowed file extensions |

### Validation Errors

Errors are collected and reported together:

```
input validation failed: 2 errors:
  - inputs.email: does not match pattern
  - inputs.count: value 150 exceeds maximum 100
```

### Interactive Input Collection

When a workflow with required inputs is run from a terminal without providing all inputs via `--input` flags, AWF automatically prompts you for missing required values. This makes it easier to run workflows interactively without remembering all parameters upfront.

**Example:**

```bash
awf run deploy
# Output:
# env (string, required):
# > prod
#
# version (string, required):
# > 1.2.3
#
# Workflow started...
```

If the input has enum constraints, AWF displays numbered options:

```bash
awf run deploy
# Output:
# env (string, required):
# Available options:
#   1) dev
#   2) staging
#   3) prod
# Select option (1-3):
# > 2
```

Optional inputs can be skipped by pressing Enter. Invalid values are rejected with error messages, allowing you to correct and retry.

See [Interactive Input Collection](commands.md#interactive-input-collection) for more details.

---

## Variable Interpolation

AWF uses `{{.var}}` syntax (Go template style with dot prefix).

```yaml
# Inputs
command: echo "{{.inputs.variable_name}}"

# Previous step outputs
command: echo "{{.states.step_name.Output}}"

# Workflow metadata
command: echo "Workflow ID: {{.workflow.id}}"

# Environment variables
command: echo "Home: {{.env.HOME}}"
```

See [Variable Interpolation Reference](../reference/interpolation.md) for complete details.

---

## Hooks

Execute commands before/after steps.

```yaml
my_step:
  type: step
  command: main-command
  pre_hook:
    command: echo "Before step"
  post_hook:
    command: echo "After step"
  on_success: next
```

### Hook Options

| Option | Type | Description |
|--------|------|-------------|
| `command` | string | Hook command to execute |
| `timeout` | int | Hook timeout in seconds |

---

## Working Directory

Steps can specify a working directory:

```yaml
build:
  type: step
  command: make build
  dir: "{{.inputs.project_path}}"
  on_success: test
```

The `dir` field supports variable interpolation.

---

## See Also

- [Commands](commands.md) - CLI command reference
- [Templates](templates.md) - Reusable workflow templates
- [Examples](examples.md) - Workflow examples
- [Variable Interpolation](../reference/interpolation.md) - Template variables
- [Input Validation](../reference/validation.md) - Validation rules
