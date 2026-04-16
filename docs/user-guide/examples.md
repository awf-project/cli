---
title: "Workflow Examples"
---

Real-world workflow examples demonstrating AWF capabilities.

## Hello World

The simplest possible workflow:

```yaml
name: hello
version: "1.0.0"
description: A simple hello world workflow

states:
  initial: greet
  greet:
    type: step
    command: echo "Hello, World!"
    on_success: done
  done:
    type: terminal
```

```bash
awf run hello
```

## With Inputs

Accept user input and use it in commands:

```yaml
name: greet-user
version: "1.0.0"

inputs:
  - name: name
    type: string
    required: true
  - name: greeting
    type: string
    default: "Hello"

states:
  initial: greet
  greet:
    type: step
    command: echo "{{.inputs.greeting}}, {{.inputs.name}}!"
    on_success: done
  done:
    type: terminal
```

```bash
awf run greet-user --input name=Alice --input greeting=Hi
```

## Code Analysis with Agent Step

Analyze code using AI with the agent step type:

```yaml
name: analyze-code
version: "1.0.0"

inputs:
  - name: file
    type: string
    required: true
    validation:
      file_exists: true

states:
  initial: read_file

  read_file:
    type: step
    command: cat "{{.inputs.file}}"
    on_success: analyze
    on_failure: error

  analyze:
    type: agent
    provider: claude
    prompt: |
      Review this code and suggest improvements:

      {{.states.read_file.Output}}
    output_format: json
    options:
      model: claude-sonnet-4-20250514
    timeout: 120
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

```bash
awf run analyze-code --input file=main.go
```

## Parallel Execution

Run multiple steps concurrently:

```yaml
name: parallel-build
version: "1.0.0"

states:
  initial: build_all

  build_all:
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

  deploy:
    type: step
    command: echo "All checks passed, deploying..."
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

## Loop Over Files

Process multiple files using for_each:

```yaml
name: process-files
version: "1.0.0"

states:
  initial: process_loop

  process_loop:
    type: for_each
    items: '["file1.txt", "file2.txt", "file3.txt"]'
    max_iterations: 100
    body:
      - process_single
    on_complete: aggregate

  process_single:
    type: step
    command: |
      echo "Processing {{.loop.Item}} ({{.loop.Index1}}/{{.loop.Length}})"
      wc -l "{{.loop.Item}}"
    on_success: process_loop
    on_failure: error

  aggregate:
    type: step
    command: echo "Processed all files"
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

## Conditional Branching

Branch based on conditions:

```yaml
name: conditional-deploy
version: "1.0.0"

inputs:
  - name: env
    type: string
    required: true
    validation:
      enum: [dev, staging, prod]

states:
  initial: check_env

  check_env:
    type: step
    command: echo "Deploying to {{.inputs.env}}"
    transitions:
      - when: "inputs.env == 'prod'"
        goto: prod_approval
      - when: "inputs.env == 'staging'"
        goto: staging_deploy
      - goto: dev_deploy

  prod_approval:
    type: step
    command: echo "Requesting production approval..."
    on_success: prod_deploy
    on_failure: error

  prod_deploy:
    type: step
    command: ./deploy.sh prod
    timeout: 300
    on_success: done
    on_failure: error

  staging_deploy:
    type: step
    command: ./deploy.sh staging
    timeout: 180
    on_success: done
    on_failure: error

  dev_deploy:
    type: step
    command: ./deploy.sh dev
    timeout: 60
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

```bash
awf run conditional-deploy --input env=prod
```

## Retry with Backoff

Retry flaky operations:

```yaml
name: api-call
version: "1.0.0"

states:
  initial: fetch_data

  fetch_data:
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
    on_success: process
    on_failure: error

  process:
    type: step
    command: echo "Processing data..."
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

## HTTP API Integration

Fetch data from a REST API and process the response:

```yaml
name: api-integration
version: "1.0.0"

inputs:
  - name: api_token
    type: string
    required: true

states:
  initial: fetch_users

  fetch_users:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "https://api.example.com/users"
      headers:
        Authorization: "Bearer {{.inputs.api_token}}"
        Accept: "application/json"
      timeout: 10
    on_success: create_report
    on_failure: error

  create_report:
    type: operation
    operation: http.request
    inputs:
      method: POST
      url: "https://api.example.com/reports"
      headers:
        Content-Type: "application/json"
        Authorization: "Bearer {{.inputs.api_token}}"
      body: '{"source": "users", "data": {{.states.fetch_users.Response.body}}}'
      timeout: 15
      retryable_status_codes: [429, 502, 503]
    retry:
      max_attempts: 3
      backoff: exponential
      initial_delay: 1s
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

```bash
awf run api-integration --input api_token=$API_TOKEN
```

## Nested Loops

Loops within loops with parent context:

```yaml
name: nested-loops
version: "1.0.0"

states:
  initial: outer_loop

  outer_loop:
    type: for_each
    items: '["A", "B", "C"]'
    body:
      - inner_loop
    on_complete: done

  inner_loop:
    type: for_each
    items: '["1", "2", "3"]'
    body:
      - process
    on_complete: outer_loop

  process:
    type: step
    command: |
      echo "outer={{.loop.Parent.Item}} inner={{.loop.Item}}"
    on_success: inner_loop

  done:
    type: terminal
```

Output:
```
outer=A inner=1
outer=A inner=2
outer=A inner=3
outer=B inner=1
...
```

## AI Agent Integration

Invoke AI agents (Claude, Cursor, Codex, Gemini) directly in workflows:

```yaml
name: code-review-with-agent
version: "1.0.0"

inputs:
  - name: file
    type: string
    required: true
    validation:
      file_exists: true

states:
  initial: read_file

  read_file:
    type: step
    command: cat "{{.inputs.file}}"
    on_success: analyze
    on_failure: error

  analyze:
    type: agent
    provider: claude
    prompt: |
      Review this code for issues and improvements:
      {{.states.read_file.Output}}
    options:
      model: claude-sonnet-4-20250514
    timeout: 120
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

```bash
awf run code-review-with-agent --input file=main.go
```

## Multi-Turn Agent Conversation

Chain multiple agent steps for conversational workflows:

```yaml
name: code-conversation
version: "1.0.0"

inputs:
  - name: code
    type: string
    required: true

states:
  initial: initial_review

  initial_review:
    type: agent
    provider: claude
    prompt: |
      Review this code:
      {{.inputs.code}}
    on_success: ask_performance

  ask_performance:
    type: agent
    provider: claude
    prompt: |
      Based on your previous review:
      {{.states.initial_review.Output}}

      Now focus specifically on performance bottlenecks.
    on_success: suggest_fixes

  suggest_fixes:
    type: agent
    provider: claude
    prompt: |
      Using your analysis, suggest specific code improvements for:
      {{.inputs.code}}
    on_success: done

  done:
    type: terminal
```

```bash
awf run code-conversation --input code="$(cat main.py)"
```

## Parallel AI Analysis

Run multiple agents concurrently for comprehensive analysis:

```yaml
name: parallel-code-analysis
version: "1.0.0"

inputs:
  - name: code
    type: string
    required: true

states:
  initial: parallel_analysis

  parallel_analysis:
    type: parallel
    parallel:
      - security_review
      - performance_review
      - style_review
    strategy: all_succeed
    on_success: aggregate

  security_review:
    type: agent
    provider: claude
    prompt: |
      Security review:
      {{.inputs.code}}

  performance_review:
    type: agent
    provider: codex
    prompt: |
      Performance analysis:
      {{.inputs.code}}

  style_review:
    type: agent
    provider: gemini
    prompt: |
      Code style review:
      {{.inputs.code}}

  aggregate:
    type: step
    command: |
      echo "Security: {{.states.security_review.Output}}"
      echo "Performance: {{.states.performance_review.Output}}"
      echo "Style: {{.states.style_review.Output}}"
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

## Built-in Workflows

AWF includes production-ready workflows in `.awf/workflows/`:

| Workflow | Description | Lines |
|----------|-------------|-------|
| `audit.yaml` | Code quality audit with AI analysis | 541 |
| `commit.yaml` | Git commit workflow with message generation | 353 |
| `feature.yaml` | Feature creation workflow | 263 |
| `implement.yaml` | TDD implementation workflow | 917 |

Explore these workflows for advanced patterns and best practices.

## See Also

- [Workflow Syntax](workflow-syntax.md) - Complete YAML reference
- [Templates](templates.md) - Reusable workflow templates
- [Variable Interpolation](../reference/interpolation.md) - Template variables
