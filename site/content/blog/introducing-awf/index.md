---
title: "Introducing AWF"
description: "AWF is an open-source CLI tool for orchestrating AI agents through declarative YAML workflows."
date: 2026-03-14
draft: false
categories: []
tags: []
contributors: []
---

AWF (AI Workflow Framework CLI) is an open-source tool for orchestrating AI agents — Claude, Gemini, Codex, and OpenAI-compatible providers — through declarative YAML workflows.

## Why AWF?

Modern AI development often requires coordinating multiple agents, chaining outputs, and handling failures gracefully. AWF provides a declarative approach: define your workflow as a state machine in YAML, and let AWF handle execution, retries, and parallel coordination.

## Key Features

- **Multi-agent orchestration** — Run different AI providers in the same workflow
- **State machine execution** — Typed states, transitions, and error handling
- **Parallel strategies** — `all_succeed`, `any_succeed`, `best_effort`
- **Built-in operations** — GitHub, HTTP, notifications, and plugins
- **Audit trail** — Structured JSONL logging of every execution

## Getting Started

Install AWF and create your first workflow:

```bash
go install github.com/awf-project/cli/cmd/awf@latest
awf init
awf run hello-world
```

Read the [documentation](/docs/) to learn more.
