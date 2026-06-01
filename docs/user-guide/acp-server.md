---
title: "ACP Editor Integration"
description: "Connect AWF to your ACP-compatible editor as a transparent AI agent"
---

## Overview

AWF integrates with [ACP](https://agentclientprotocol.com) (Agent Client Protocol)-compatible editors such as [Zed](https://zed.dev) and [acp.nvim](https://github.com/huynhsontung/acp.nvim). This allows you to invoke AWF workflows directly from your editor's agent panel.

## Supported Editors

- **Zed** - via External Agent mechanism
- **acp.nvim** - Neovim plugin for ACP protocol
- Future: VS Code, JetBrains IDEs (via ACP plugins)

## Setup

### Configuration

First, create a workflow configuration if you haven't already:

```bash
awf init
```

Define your workflows in `.awf/workflows/` directory as usual.

### Editor Configuration

#### Zed

Configure Zed to use AWF as an external agent:

```json
{
  "assistant": {
    "default_model": {
      "provider": "custom",
      "name": "awf"
    },
    "custom_model": {
      "awf": {
        "type": "command",
        "command": "awf",
        "arguments": ["acp-serve", "--config", "$PROJECT_CONFIG_PATH"]
      }
    }
  }
}
```

Where `$PROJECT_CONFIG_PATH` is the path to your AWF config file (default: `.awf/config.yaml`).

#### acp.nvim

Configure acp.nvim to spawn AWF as an external agent:

```lua
require("acp").setup({
  agent = {
    type = "command",
    command = "awf",
    arguments = { "acp-serve", "--config", vim.loop.cwd() .. "/.awf/config.yaml" }
  }
})
```

## Usage

### Invoking Workflows

Once configured, your workflows appear as slash commands in the editor's agent panel.

1. Open the agent panel (Zed: `Cmd+Shift+A`)
2. Type `/` followed by your workflow name
3. Add any inputs as `key=value` pairs
4. Press Enter to execute

Example (recommended bare form):
```
/my-workflow file=main.go review-type=security
```

#### Input syntax

Inputs are `key=value` pairs. Three forms are accepted (all equivalent):

| Form | Example |
|------|---------|
| Bare pair (recommended) | `/my-workflow file=main.go` |
| `--input=` (CLI `=` form) | `/my-workflow --input=file=main.go` |
| `--input ` (CLI space form) | `/my-workflow --input file=main.go` |

Details:
- The prompt is tokenized shell-style, so **quotes group values and are stripped**: `msg="hello world"` and `msg='hello world'` both set `msg` to `hello world`. Use quotes whenever a value contains spaces.
- Values may contain `=` (only the first `=` splits key from value): `url=https://x?a=1&b=2` works.
- Tokens that are not `key=value` pairs (or an unrecognized `--flag`) are ignored.
- Unlike the CLI, the `@prompts/` file-reference prefix is **not** resolved — editors send literal values.

### Multi-turn Conversations

Workflows with conversational steps (using `ConversationManager`) maintain state across multiple editor prompts:

1. Send `/my-workflow` with initial prompt
2. Workflow pauses at first conversation turn
3. Editor shows the agent's response
4. Send follow-up prompt
5. Workflow resumes with your input (turn-boundary resume semantics)

### Approval Gates

Workflows with approval gates display a permission dialog in the editor instead of interrupting the workflow. Approve or deny directly from the editor UI.

## Limitations

**AWF v1 (current):**
- Stdio transport only (no HTTP/WebSocket yet)
- Read-only execution (workflows control tool calls; editor `fs/` and `terminal/` methods are not supported)
- Single-turn workflow execution (multi-turn via conversation steps only)
- No authentication methods advertised

**Future versions:**
- HTTP/WebSocket remote transport
- Editor filesystem and terminal access
- Session resume across editor restarts
- Custom ACP session modes

## Debugging

### Enable Verbose Output

Run AWF with increased logging:

```bash
awf acp-serve --config=.awf/config.yaml --log-level=debug
```

Check the logs for connection state and workflow execution details.

### Dry-Run Preview

Test a workflow without editor integration:

```bash
awf run my-workflow --input=arg=value --dry-run
```

### Protocol Validation

Verify your workflows work with ACP by testing basic execution:

```bash
awf validate my-workflow
```

## Example Workflow

A code review workflow compatible with ACP editors:

```yaml
name: code-review
version: "1.0.0"

inputs:
  - name: file
    type: string
    required: true
    validation:
      file_exists: true
  - name: review_type
    type: string
    default: general
    enum: [security, performance, style, general]

states:
  initial: read

  read:
    type: step
    command: cat "{{.inputs.file}}"
    on_success: review

  review:
    type: agent
    provider: claude
    prompt: |
      Review this {{.inputs.review_type}} code for issues:
      {{.states.read.Output}}
      
      Focus on:
      - Code correctness
      - Security vulnerabilities
      - {{.inputs.review_type}} concerns
    options:
      model: claude-sonnet-4-20250514
    timeout: 120
    on_success: done

  done:
    type: terminal
    status: success
```

Use from Zed:
```
/code-review --input=file=src/main.rs --input=review_type=security
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Editor cannot connect to AWF | Verify `awf acp-serve --config=...` runs without errors locally; check log output with `--log-level=debug` |
| Workflow not listed in agent panel | Run `awf list` to verify workflow exists; reload editor or restart ACP session |
| Multi-turn conversation stuck | Send empty input to exit conversation; or stop and restart the ACP session |
| Approval gate not appearing | Ensure workflow step has `approval_gate` configuration; check editor ACP permissions |

## See Also

- [ACP Specification](https://agentclientprotocol.com)
- [Workflow Syntax](workflow-syntax.md)
- [Agent Steps](agent-steps.md)
- [Conversation Mode](conversation-steps.md)
