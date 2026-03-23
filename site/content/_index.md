---
title: "AWF — AI Workflow Framework CLI"
description: "Orchestrate AI agents through declarative YAML workflows with state machine execution."
lead: "Orchestrate AI agents through declarative YAML workflows."
date: 2026-03-14
draft: false
---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/awf-project/cli/main/scripts/install.sh | sh
```

Or via Go:

```bash
go install github.com/awf-project/cli/cmd/awf@latest
```

## Quick Start

```yaml
name: hello-world
inputs:
  topic:
    type: string
    description: "Topic to write about"

states:
  generate:
    type: step
    agent:
      provider: claude
      prompt: "Write a short paragraph about {{inputs.topic}}"
    on_success: done

  done:
    type: terminal
    status: success
```

```bash
awf run hello-world --input topic="AI workflows"
```
