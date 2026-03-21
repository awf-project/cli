---
title: "Introducing AWF"
description: "AWF is an open-source CLI tool for orchestrating AI agents through declarative YAML workflows — built for developers who value determinism, efficiency, and engineering standards."
date: 2026-03-14
draft: false
categories: ["announcements"]
tags: ["awf", "cli", "ai", "workflow", "go", "open-source"]
contributors: ["Alex"]
---

For the past few months, I've been working on a side project called AWF (AI Workflow Framework). 
The code for the main CLI tool is now [available on GitHub](https://github.com/awf-project/cli) for everyone — including a dedicated [Claude skill](https://github.com/awf-project/awf-marketplace).

## Why I Built This

I wear two hats: tech enthusiast and company owner. Managing multiple Claude sessions with various agents simply doesn't scale with daily responsibilities. 
I wanted a way to test my ideas and improve my workflow without sacrificing my standards — I'm a firm believer in TDD, QA, and high engineering standards.

On the AI side, three things kept frustrating me:

- **Efficiency** — I don't want to waste tokens. If I ask an agent to perform TDD, I want *real* TDD.
- **Determinism** — I want to call my CLI tools because they are predictable. No crossing fingers hoping an agent calls the right MCP tool.
- **Flexibility** — I want to use Claude, Gemini, or any other model without arbitrary limitations.

That's why I built AWF in Go. It's a workflow engine designed to orchestrate CLI tools — including LLMs like Claude, Gemini, and Codex.

## What AWF Is (and Isn't)

AWF isn't another "magic" AI wrapper. It's a professional tool for those who know how to manage their context window and want to build deterministic, industrial-grade workflows using CLI outputs.

With AWF, you design your workflow through discrete steps. Each step can run a CLI program, a shell script, or an AI agent. You can:

- Pass parameters as options
- Capture stdout for the next step
- Abort the workflow on stderr
- Define transitions, pre/post-events, loops, and retries
- Run steps in parallel with configurable strategies (`all_succeed`, `any_succeed`, `best_effort`)

The better you know your CLI basics, the more freedom you have to build complex systems.

## The Agent Step

There is a specialized step type for AI interactions. You provide a prompt — which supports variables via Go templates — and AWF executes it according to your design:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: |
    Review this code for bugs, security issues, and improvements:
    {{.states.read.Output}}
  output_format: json
  options:
    model: sonnet
  timeout: 120
  on_success: report
  on_failure: {message: "Analysis failed: {{.states.analyze.Output}}"}
```

Want to fail fast? No problem. Continue on error? Easy. Call a nested workflow? That's actually how I update the AWF skill on every new PR.

## Features at a Glance

- **State machine execution** — Define workflows as state machines with conditional transitions based on exit codes, command output, or custom expressions
- **Multi-agent orchestration** — Claude, Gemini, Codex, and any OpenAI-compatible API (Ollama, vLLM, Groq) in the same workflow
- **Conversation mode** — Multi-turn conversations with native session resume, context injection, and token tracking across all turns
- **External prompt files** — Load prompts from `.md` files with full template interpolation and local override support
- **External script files** — Shebang-based interpreter dispatch with template interpolation
- **Loop constructs** — For-each and while loops with full context access
- **Retry with backoff** — Exponential, linear, or constant backoff strategies
- **Parallel execution** — Run multiple steps concurrently with configurable strategies
- **Sub-workflows** — Invoke workflows from other workflows with input/output mapping
- **Built-in plugins** — GitHub operations, HTTP calls, desktop notifications
- **Audit trail** — Structured JSONL logging with secret masking
- **Dry-run and interactive modes** — Preview or step through execution before committing
- **XDG support** — Workflows can be defined locally or globally with override support

## Getting Started

Install AWF:

```bash
curl -fsSL https://raw.githubusercontent.com/awf-project/cli/main/scripts/install.sh | sh
```

Or via Go:

```bash
go install github.com/awf-project/cli/cmd/awf@latest
```

Then create your first workflow:

```bash
awf init
awf run example
```

Read the [documentation](/docs/) to learn more, or check out the [AWF organization on GitHub](https://github.com/awf-project) for plenty of examples to help you get started.

## What's Next

AWF is already working great for my own projects, but I want to keep improving it. 
I'd love to get feedback on the code — especially since AWF is now built using its own workflows.

If you're a Go developer, I'd highly appreciate your insights. Check out the [tests](https://github.com/awf-project/cli/tree/main/tests), the [examples](https://github.com/awf-project/cli/tree/main/tests/fixtures/workflows), and don't hesitate to open an issue.
