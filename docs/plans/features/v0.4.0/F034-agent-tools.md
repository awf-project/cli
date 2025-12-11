# F034: Agent Tools

## Metadata
- **Status**: backlog
- **Phase**: 3-AI
- **Version**: v0.4.0
- **Priority**: medium
- **Estimation**: XL

## Description

Enable AI agents to use tools (function calling) during workflow execution. The agent can request tool executions, which the workflow orchestrator fulfills and returns results for. This bridges AI reasoning with concrete actions.

```
┌─────────────────────────────────────────────────────────────┐
│                    AGENT WITH TOOLS                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐      ┌─────────────────────────────────┐  │
│  │   Agent     │      │         TOOL REGISTRY           │  │
│  │   (LLM)     │      │  ┌─────────┐ ┌─────────┐       │  │
│  │             │─────▶│  │ shell   │ │  http   │ ...   │  │
│  │  "I need to │      │  └─────────┘ └─────────┘       │  │
│  │   run X"    │      │  ┌─────────┐ ┌─────────┐       │  │
│  │             │◀─────│  │  read   │ │  write  │       │  │
│  └─────────────┘      │  └─────────┘ └─────────┘       │  │
│        │              └─────────────────────────────────┘  │
│        │                                                    │
│        ▼                                                    │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                  TOOL EXECUTOR                       │   │
│  │  1. Parse tool call from agent response             │   │
│  │  2. Validate against allowed tools                  │   │
│  │  3. Execute tool with sandboxing                    │   │
│  │  4. Return result to agent                          │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

**Design principles:**
1. **Declarative tool definitions** — Tools defined in YAML, not code
2. **Sandboxed execution** — Tools run with restricted permissions
3. **Approval modes** — Auto-approve, ask user, or deny by default
4. **Provider-agnostic** — Abstract over different tool-use protocols

## Acceptance Criteria

- [ ] Tool definitions in workflow YAML
- [ ] Built-in tools: `shell`, `read_file`, `write_file`, `http_request`
- [ ] Custom tool definitions with command templates
- [ ] Tool call parsing from agent responses (JSON, XML formats)
- [ ] Tool result injection back to agent
- [ ] Approval modes: `auto`, `prompt`, `deny`
- [ ] Tool execution sandboxing (restricted paths, commands)
- [ ] Max tool calls limit per step
- [ ] Tool call logging for audit
- [ ] Works with conversation mode (F033)

## Dependencies

- **Blocked by**: F032 (Agent Step Type)
- **Unblocks**: _none_

## Impacted Files

```
internal/domain/workflow/tool.go               # Tool definitions
internal/domain/workflow/tool_call.go          # Tool call/result models
internal/domain/ports/tool_executor.go         # Tool executor interface
internal/infrastructure/tools/                 # Built-in tool implementations
├── shell_tool.go
├── file_tool.go
├── http_tool.go
└── custom_tool.go
internal/application/tool_orchestrator.go      # Tool call loop
internal/infrastructure/sandbox/               # Execution sandboxing
pkg/toolparse/                                 # Parse tool calls from responses
```

## Technical Tasks

- [ ] Define Tool domain model
  - [ ] Tool struct (name, description, parameters, command)
  - [ ] ToolCall struct (name, arguments, id)
  - [ ] ToolResult struct (output, error, duration)
- [ ] Implement built-in tools
  - [ ] `shell` — Execute shell commands
  - [ ] `read_file` — Read file contents
  - [ ] `write_file` — Write/append to files
  - [ ] `http_request` — Make HTTP calls
- [ ] Create tool call parser
  - [ ] JSON format (OpenAI-style)
  - [ ] XML format (Claude-style)
  - [ ] Custom format support
- [ ] Implement tool executor
  - [ ] Route to appropriate tool handler
  - [ ] Apply sandboxing rules
  - [ ] Capture output/errors
- [ ] Add approval flow
  - [ ] Auto-approve for safe tools
  - [ ] Prompt user for dangerous tools
  - [ ] Deny blacklisted operations
- [ ] Implement sandboxing
  - [ ] Path restrictions (allowed directories)
  - [ ] Command restrictions (allowed binaries)
  - [ ] Network restrictions (allowed hosts)
- [ ] Integrate with agent execution loop
  - [ ] Detect tool calls in response
  - [ ] Execute tools
  - [ ] Feed results back to agent
  - [ ] Continue until no more tool calls
- [ ] Write unit tests
- [ ] Write integration tests
- [ ] Security audit

## YAML Syntax

```yaml
name: autonomous-coding
version: "1.0"

tools:
  - name: read_file
    builtin: true
    approval: auto
    restrictions:
      paths:
        - "{{inputs.project_path}}/**"

  - name: write_file
    builtin: true
    approval: prompt  # Ask user before writing
    restrictions:
      paths:
        - "{{inputs.project_path}}/src/**"

  - name: run_tests
    command: "cd {{inputs.project_path}} && npm test"
    description: "Run the project test suite"
    approval: auto

  - name: search_code
    command: "rg '{{args.pattern}}' {{inputs.project_path}}"
    description: "Search for pattern in codebase"
    parameters:
      - name: pattern
        type: string
        required: true
    approval: auto

steps:
  - name: implement_feature
    type: agent
    provider: claude
    prompt: |
      Implement this feature: {{inputs.feature_description}}

      You have access to tools to read/write files and run tests.
      Use them to complete the implementation.
    tools:
      - read_file
      - write_file
      - run_tests
      - search_code
    options:
      model: claude-sonnet-4-20250514
    tool_options:
      max_calls: 50
      timeout_per_call: 30s
```

## Tool Call Flow

```
┌──────────┐     ┌───────────┐     ┌──────────────┐     ┌──────────┐
│  Agent   │────▶│  Workflow │────▶│ Tool Executor│────▶│  Tool    │
│  (LLM)   │     │ Orchestr. │     │  (sandbox)   │     │ Handler  │
└──────────┘     └───────────┘     └──────────────┘     └──────────┘
     │                │                    │                  │
     │  "Use tool X"  │                    │                  │
     │───────────────▶│  Parse & validate  │                  │
     │                │───────────────────▶│  Execute         │
     │                │                    │─────────────────▶│
     │                │                    │◀─────────────────│
     │                │◀───────────────────│  Result          │
     │◀───────────────│  Inject result     │                  │
     │  Continue...   │                    │                  │
```

## State Structure

```yaml
states:
  implement_feature:
    output: "Feature implemented successfully"
    tool_calls:
      - id: "call_001"
        tool: read_file
        arguments:
          path: "src/index.ts"
        result:
          success: true
          output: "file contents..."
          duration_ms: 15
      - id: "call_002"
        tool: write_file
        arguments:
          path: "src/feature.ts"
          content: "new code..."
        result:
          success: true
          output: "File written"
          duration_ms: 23
          approval: "user"  # User approved this
      - id: "call_003"
        tool: run_tests
        arguments: {}
        result:
          success: true
          output: "All tests passed"
          duration_ms: 5420
    tool_stats:
      total_calls: 3
      successful: 3
      failed: 0
      total_duration_ms: 5458
```

## Security Considerations

- **Path traversal** — Validate all paths are within allowed directories
- **Command injection** — Sanitize all arguments before shell execution
- **Resource limits** — Timeout and memory limits for tool execution
- **Audit logging** — Log all tool calls for security review
- **Principle of least privilege** — Tools only get permissions they need

## Notes

- **Provider compatibility** — Not all AI CLIs support native tool use; may need to parse from text
- **Agentic loops** — With tools, agents can run indefinitely; always set max_calls
- **Human in the loop** — Critical operations should require user approval
- **Cost explosion** — Tool use can dramatically increase token usage
- Future: MCP (Model Context Protocol) integration for standardized tool definitions
