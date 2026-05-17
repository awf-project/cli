---
title: "Agent Steps Guide"
---

Invoke AI agents (Claude, Codex, Gemini, GitHub Copilot, OpenCode, OpenAI-Compatible) in your workflows with structured prompts and response parsing.

## Overview

Agent steps allow you to integrate AI CLI tools into AWF workflows. Instead of shell commands, you define prompts as templates that get interpolated with workflow context and executed through provider-specific CLIs.

**Benefits:**
- Non-interactive: Suitable for CI/CD automation
- Stateless: Multi-turn conversations via state passing between steps
- Structured output: Automatic JSON parsing and token tracking
- Template interpolation: Access workflow context in prompts

## Basic Usage

```yaml
states:
  initial: analyze

  analyze:
    type: agent
    provider: claude
    prompt: "Analyze this code: {{.inputs.code}}"
    on_success: done

  done:
    type: terminal
```

```bash
awf run workflow --input code="$(cat main.py)"
```

### Dynamic Provider Selection

The `provider` field supports template interpolation, enabling dynamic provider selection based on workflow inputs or state:

```yaml
inputs:
  - name: model_source
    type: string
    default: claude

states:
  initial: process

  process:
    type: agent
    provider: "{{.inputs.model_source}}"
    prompt: "Process: {{.inputs.data}}"
    on_success: done

  done:
    type: terminal
```

```bash
awf run workflow --input model_source=gemini --input data="..."
awf run workflow --input model_source=codex --input data="..."
```

This enables workflows to support multiple AI backends without duplicating step definitions.

## Supported Providers

### Claude (Anthropic)

Requires the `claude` CLI tool installed.

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Code review: {{.inputs.file_content}}"
  options:
    model: claude-sonnet-4-20250514
  timeout: 120
  on_success: next
```

**Provider-Specific Options:**
- `model`: Claude model identifier (alias like `sonnet` or full name like `claude-sonnet-4-20250514`)
- `allowed_tools`: Comma-separated list of tools to allow (e.g., `"bash,read"` → `--allowedTools bash,read`)
- `dangerously_skip_permissions`: Skip permission prompts (boolean, maps to `--dangerously-skip-permissions`). **Security warning**: bypasses all safety prompts — use only in trusted, automated environments. Emits a security audit log.

### Codex (OpenAI)

Requires the `codex` CLI tool installed.

```yaml
generate:
  type: agent
  provider: codex
  prompt: "Generate a function to: {{.inputs.requirement}}"
  options:
    model: gpt-4o
  timeout: 60
  on_success: next
```

**Provider-Specific Options:**
- `model`: Codex model identifier — validated against OpenAI models (see [Model Validation](#model-validation) below)
- `language`: Target programming language
- `quiet`: Suppress progress output (boolean)
- `dangerously_skip_permissions`: Skip permission prompts (boolean, maps to `--yolo`). **Security warning**: bypasses all safety prompts — use only in trusted, automated environments.

### Gemini (Google)

Requires the `gemini` CLI tool installed.

```yaml
summarize:
  type: agent
  provider: gemini
  prompt: "Summarize: {{.inputs.text}}"
  options:
    model: gemini-2.0-flash
  timeout: 60
  on_success: next
```

**Provider-Specific Options:**
- `model`: Gemini model identifier — validated to start with `gemini-` prefix (see [Model Validation](#model-validation) below)
- `dangerously_skip_permissions`: Skip permission prompts (boolean, maps to `--approval-mode=yolo`). **Security warning**: bypasses all safety prompts — use only in trusted, automated environments.

### GitHub Copilot

Requires the `copilot` CLI tool installed and authentication via `copilot login` or environment variables.

```yaml
code_generate:
  type: agent
  provider: github_copilot
  prompt: "Generate a function to: {{.inputs.requirement}}"
  options:
    model: gpt-4o
    mode: interactive
  timeout: 60
  on_success: next
```

**Provider-Specific Options:**
- `model`: GitHub Copilot model identifier (e.g., `gpt-4o`, `gpt-4`, `gpt-3.5-turbo`)
- `mode`: Agent mode — one of `interactive` (default), `plan`, or `autopilot`
- `effort`: Reasoning effort level — one of `low`, `medium`, or `high`
- `allowed_tools`: Comma-separated list of tools to allow (e.g., `"bash,github_api"` → `--allow-tool bash --allow-tool github_api`)
- `denied_tools`: Comma-separated list of tools to deny (maps to `--deny-tool`)
- `allow_all`: Allow all available tools (boolean, maps to `--allow-all`)
- `system_prompt`: Custom system message (passed via prompt prepending)

**Authentication:**
GitHub Copilot CLI supports authentication via:
- `copilot login` (interactive authentication)
- `COPILOT_GITHUB_TOKEN` environment variable
- `GH_TOKEN` environment variable
- `GITHUB_TOKEN` environment variable (classic PATs not supported)

### OpenCode

Requires the `opencode` CLI tool installed.

```yaml
refactor:
  type: agent
  provider: opencode
  prompt: "Refactor this code for readability: {{.inputs.code}}"
  timeout: 120
  on_success: next
```

### OpenAI-Compatible Provider

For any backend that implements the [Chat Completions API](https://platform.openai.com/docs/api-reference/chat) (OpenAI, Ollama, vLLM, Groq, LM Studio, etc.), use the `openai_compatible` provider. Unlike CLI-based providers, this sends HTTP requests directly — no CLI tool installation required.

```yaml
analyze:
  type: agent
  provider: openai_compatible
  prompt: "Analyze: {{.inputs.data}}"
  options:
    base_url: http://localhost:11434/v1
    model: llama3
    api_key: "{{.env.OPENAI_API_KEY}}"
  timeout: 60
  on_success: next
```

**Required Options:**
- `base_url`: Root URL of the API (e.g., `http://localhost:11434/v1`). The provider appends `/chat/completions` automatically.
- `model`: Model identifier (e.g., `llama3`, `gpt-4o`, `mixtral`)

**Optional Options:**
- `api_key`: API key for authentication. Falls back to `OPENAI_API_KEY` environment variable if not set. Omit for local endpoints that don't require auth (e.g., Ollama).
- `temperature`: Creativity level (0-2)
- `max_completion_tokens`: Maximum response tokens (preferred)
- `max_tokens`: Maximum response tokens (deprecated, use `max_completion_tokens`)
- `top_p`: Nucleus sampling threshold
- `system_prompt`: System message prepended to conversation (used in `mode: conversation`)

**Token Tracking:** Unlike CLI-based providers that estimate tokens via the unified `Tokenizer` port, `openai_compatible` reports actual token usage from the API response.

**Display Cadence:** Unlike streaming CLI providers (Claude, Codex, Gemini, OpenCode) that display output incrementally, `openai_compatible` displays all events in a single burst after the HTTP response completes. This means tool-use markers and text output appear together at the end of execution rather than interleaved during streaming. The rendered shape and tool markers are identical across all providers — only the timing differs.

**Example backends:**
- **Ollama**: `base_url: http://localhost:11434/v1`, `model: llama3`
- **OpenAI**: `base_url: https://api.openai.com/v1`, `model: gpt-4o`
- **Groq**: `base_url: https://api.groq.com/openai/v1`, `model: mixtral-8x7b-32768`
- **vLLM**: `base_url: http://localhost:8000/v1`, `model: your-model`

## Model Validation

AWF validates the `model` field for certain providers to catch typos and invalid models at workflow validation time:

### Claude

Claude model validation accepts:
- **Aliases**: `sonnet`, `opus`, `haiku` (resolved to current recommended versions)
- **Full names**: Any model starting with `claude-` (e.g., `claude-3-opus-20250219`)

**Examples:**
```yaml
options:
  model: sonnet                          # Alias (valid)
  model: claude-opus-4-1-20250805        # Full name (valid)
  model: gpt-4                           # Invalid - rejected at validation
```

### Gemini

Gemini model validation requires the `gemini-` prefix. This allows you to use any Gemini model without waiting for AWF CLI updates:

- **Prefix**: Must start with `gemini-`
- **Examples**: `gemini-pro`, `gemini-2.0-flash`, `gemini-1.5-pro-latest`

**Examples:**
```yaml
options:
  model: gemini-2.0-flash                # Valid - new models automatically supported
  model: gemini-pro                      # Valid - legacy models still work
  model: gpt-4                           # Invalid - rejects non-Gemini models
  model: gemini-                         # Valid at validation (provider CLI rejects at runtime)
```

**Error message example:**
```
step validation error: model must start with "gemini-"
```

### Codex

Codex model validation accepts OpenAI model prefixes:

- **`gpt-` prefix**: Models like `gpt-4o`, `gpt-3.5-turbo`
- **`codex-` prefix**: Forward compatibility for future Codex-branded models
- **`o-` series**: OpenAI's reasoning models (e.g., `o1`, `o3-mini`) — must have a digit after the `o`

**Examples:**
```yaml
options:
  model: gpt-4o                          # Valid - current OpenAI model
  model: gpt-3.5-turbo                   # Valid - legacy OpenAI model
  model: o1                              # Valid - o-series reasoning model
  model: o3-mini                         # Valid - o-series with suffix
  model: codex-mini                      # Valid - forward compatibility
  model: code-davinci                    # Invalid - rejects old Codex models
  model: toto                            # Invalid - no recognized prefix
```

**Error message example:**
```
step validation error: model must start with "gpt-", "codex-", or match o-series pattern (e.g., o1, o3-mini)
```

### GitHub Copilot, OpenCode & OpenAI-Compatible

No model validation for `github_copilot`, `opencode`, or `openai_compatible` providers — these support arbitrary backend models.

### When Validation Occurs

Model validation runs during:
- `awf validate <workflow>` — Catch errors before execution
- `awf run <workflow>` — Catches errors at startup before execution begins
- `--dry-run` — Validates without execution

**Error handling:**
- Validation errors are reported with clear guidance on expected format
- Workflow stops immediately on validation failure (exit code 2)
- No need for downstream CLI error handling — wrong models are caught early

## Agent Skills

Agent steps can declare `skills:` to inject deterministic domain knowledge into the agent's prompt context. Skills are loaded from SKILL.md files following the [agentskills.io](https://agentskills.io) specification. Unlike agent-native skill activation (model-driven), AWF injects skills explicitly — the workflow YAML is the single source of truth.

### Basic Usage

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review this code: {{.inputs.code}}"
  skills:
    - go-conventions
    - code-review
  on_success: done
```

Each declared skill's markdown body is injected into the prompt context wrapped in a `<skill_content>` block, placed before the user prompt.

### Skill References

Skills can be referenced by **name** (discovered from filesystem) or by **explicit path**:

```yaml
# Name-based: discovered from search paths
skills:
  - go-conventions

# Path-based: relative to workflow file
skills:
  - path: ./custom-skills/audit

# Mixed: both in the same step
skills:
  - go-conventions
  - path: ./custom-skills/audit
```

### Discovery Directories

Name-based skills are discovered across 7 priority-ordered directories (first match wins):

| Priority | Directory | Scope |
|----------|-----------|-------|
| 1 | `AWF_SKILLS_PATH` env var | Override (exclusive — disables all other paths) |
| 2 | `<project>/.awf/skills/` | Local project (AWF-specific) |
| 3 | `<project>/.agents/skills/` | Local project (cross-client interop) |
| 4 | `<project>/.claude/skills/` | Local project (Claude Code compat) |
| 5 | `$XDG_CONFIG_HOME/awf/skills/` | Global user (AWF-specific) |
| 6 | `~/.agents/skills/` | Global user (cross-client interop) |
| 7 | `~/.claude/skills/` | Global user (Claude Code compat) |

### SKILL.md Format

Each skill is a directory containing a `SKILL.md` file. YAML frontmatter is stripped; only the markdown body is injected:

```
.awf/skills/go-conventions/
├── SKILL.md
├── scripts/check.sh          # Optional bundled resources
└── references/style-guide.md  # Optional bundled resources
```

**SKILL.md:**
```markdown
---
name: go-conventions
description: Go coding conventions
---

Follow these Go conventions:
- Use `gofmt` for formatting
- Error messages start lowercase
- Return early, avoid deep nesting
```

The frontmatter (`---` delimited block) is stripped. The skill name comes from the directory name, not frontmatter fields.

### Injection Format

Skills are injected as structured XML blocks before the user prompt:

```xml
<skill_content name="go-conventions">
Follow these Go conventions:
- Use `gofmt` for formatting
- Error messages start lowercase
- Return early, avoid deep nesting

Skill directory: /project/.awf/skills/go-conventions
Relative paths in this skill are relative to the skill directory.

<skill_resources>
  <file>scripts/check.sh</file>
  <file>references/style-guide.md</file>
</skill_resources>
</skill_content>
```

Bundled resources are listed but not eagerly read — agents can access them using their native file tools via the skill directory path.

### Validation

`awf validate` checks all skill references before runtime:

```bash
awf validate my-workflow
# Error: skill 'nonexistent' not found in search paths
# Warning: skill 'empty-skill' has empty SKILL.md (no content to inject)
```

| Check | Result |
|-------|--------|
| Skill directory not found | Error (`skill_not_found`) |
| Directory exists but no SKILL.md | Error (`skill_missing_skillmd`) |
| SKILL.md is empty (0 bytes) | Warning (`skill_empty_content`) |
| SKILL.md > 500KB | Warning (context window impact) |

### Limitations

- Skills are injected only for single-turn agent steps (`mode: single`). Conversation mode (`mode: conversation`) does not support skill injection in v1.
- No template interpolation is applied to skill content — it is injected as-is.
- Skills do not reference other skills; only workflow steps reference skills.

## Prompt Templates

Prompts support full variable interpolation with access to workflow context:

```yaml
review:
  type: agent
  provider: claude
  prompt: |
    Review this code file:
    Path: {{.inputs.file_path}}
    Language: {{.inputs.language}}

    File content:
    {{.inputs.file_content}}

    Focus on:
    - Performance issues
    - Security vulnerabilities
    - Code style violations
  on_success: generate_report
```

**Available Variables:**
- `{{.inputs.*}}` - Workflow input values
- `{{.states.step_name.Output}}` - Previous step raw output
- `{{.states.step_name.Response}}` - Previous step parsed JSON (heuristic)
- `{{.states.step_name.JSON}}` - Parsed JSON from `output_format: json` (explicit)
- `{{.env.VAR_NAME}}` - Environment variables
- `{{.workflow.id}}` - Workflow execution ID
- `{{.workflow.name}}` - Workflow name

See [Variable Interpolation Reference](../reference/interpolation.md) for complete details.

## Agent Skills

Inject reusable domain knowledge (skills) into agent steps to provide specialized context, instructions, or guidelines. Skills are Markdown files bundled in standard discovery directories that AWF loads and prepends to agent prompts automatically.

**Why use skills?**
- **Reusable knowledge** — Define best practices, coding standards, or domain guidelines once, use across workflows
- **Deterministic injection** — Explicit YAML declarations ensure exactly the right skills reach each step
- **Bundled resources** — Include scripts, reference files, or examples alongside skill content

### Basic Usage

Create a skill directory with a `SKILL.md` file:

**Directory structure:**
```
.awf/skills/
├── go-conventions/
│   ├── SKILL.md                 # Markdown content (frontmatter optional)
│   └── references/
│       └── style-guide.md       # Bundled resources
└── code-review/
    └── SKILL.md
```

**File:** `.awf/skills/go-conventions/SKILL.md`
```markdown
# Go Conventions Skill

Follow these Go best practices:

- Use CamelCase for public identifiers
- Keep functions under 30 lines
- Handle errors explicitly
- Use interfaces for abstraction
```

Reference the skill by name in your workflow:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review this Go code: {{.inputs.code}}"
  skills: [go-conventions]
  on_success: done
```

When executed, the skill content is automatically injected into the agent's prompt context before the user prompt:

```
<skill_content name="go-conventions">
Follow these Go best practices:
- Use CamelCase for public identifiers
- Keep functions under 30 lines
- Handle errors explicitly
- Use interfaces for abstraction

Skill directory: /project/.awf/skills/go-conventions
Relative paths in this skill are relative to the skill directory.
</skill_content>

Review this Go code: <code>
...
</code>
```

### Multiple Skills

Declare multiple skills in a single step:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review this code: {{.inputs.code}}"
  skills:
    - go-conventions
    - security-review
    - performance-optimization
  on_success: done
```

Skills are injected in declaration order, each wrapped in separate `<skill_content>` blocks.

### Skill Discovery

AWF searches for skills in the following directories, in priority order:

1. **`AWF_SKILLS_PATH` environment variable** (if set, exclusive — overrides all others)
2. **`.awf/skills/`** (project-level, AWF-specific)
3. **`.agents/skills/`** (project-level, cross-client compatibility per agentskills.io spec)
4. **`.claude/skills/`** (project-level, Claude Code compatibility)
5. **`$XDG_CONFIG_HOME/awf/skills/`** (global user, AWF-specific — defaults to `~/.config/awf/skills/`)
6. **`~/.agents/skills/`** (global user, cross-client)
7. **`~/.claude/skills/`** (global user, Claude Code compatibility)

This enables shared global skills while allowing project-specific overrides.

**Example with environment variable:**

```bash
# Use only skills from custom directory
AWF_SKILLS_PATH=/shared/skills:~/my-skills awf run workflow
```

### Explicit Path References

Reference skills by explicit path instead of discovery:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review: {{.inputs.code}}"
  skills:
    - path: ./custom-skills/audit
    - path: /shared/skills/compliance
  on_success: done
```

Paths can be:
- **Relative** to the workflow directory: `path: ./custom-skills/audit`
- **Absolute**: `path: /home/user/skills/audit`
- **Home-relative**: `path: ~/shared-skills/audit`

### Skill Content Format

**Frontmatter is optional** — AWF strips YAML frontmatter (content between `---` delimiters) if present, preserving only the Markdown body:

```markdown
---
name: go-conventions
description: Go coding standards
license: MIT
---

# Go Conventions

Follow these best practices...
```

Only the Markdown body is injected. The skill name comes from the directory name (e.g., `go-conventions`), not from frontmatter.

### Bundled Resources

Include additional files (scripts, guides, examples) alongside `SKILL.md`:

```
.awf/skills/audit/
├── SKILL.md                    # Main skill content
├── scripts/
│   ├── check-security.sh
│   └── validate-config.py
└── references/
    ├── checklist.md
    └── examples/
        └── sample-config.yaml
```

AWF automatically enumerates bundled resources in the skill context:

```
<skill_content name="audit">
[skill markdown body]

Skill directory: /project/.awf/skills/audit
Relative paths in this skill are relative to the skill directory.

<skill_resources>
  <file>references/checklist.md</file>
  <file>references/examples/sample-config.yaml</file>
  <file>scripts/check-security.sh</file>
  <file>scripts/validate-config.py</file>
</skill_resources>
</skill_content>
```

Agents can use bundled resources via their native file tools (Read, Bash) by referencing the skill directory and relative paths.

### Validation

Use `awf validate` to check that all skill references exist and are readable:

```bash
awf validate workflow.yaml
```

Validation reports:
- ✓ Skills found with valid `SKILL.md` files
- ✗ Skills referenced but not found
- ✗ Skill directories found but missing `SKILL.md`
- ⚠ Empty `SKILL.md` files (warning — content will be empty)

### Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| `SkillNotFoundError` | Referenced skill not found in discovery paths | Check skill directory name matches YAML declaration; verify `SKILL.md` exists |
| Empty skill content | `SKILL.md` is 0 bytes or contains only frontmatter | Add Markdown body to `SKILL.md` |
| Skill not in expected location | Wrong discovery directory | Move skill to one of the 7 standard directories or use `path:` reference |
| Large skill file warning | `SKILL.md` exceeds 500KB | Consider splitting into multiple smaller skills |

## Agent Roles

Agent roles define reusable personas that are injected into the agent's system prompt. Unlike skills (which inject knowledge into the user prompt), roles establish the agent's identity, behavior, and perspective via the system prompt. Roles are stored in AGENTS.md files following the [agents.md](https://github.com/agentsmd/agents.md) specification.

### Basic Usage

Create a role directory with an `AGENTS.md` file:

**Directory structure:**
```
.awf/agents/
├── go-senior/
│   └── AGENTS.md          # Agent persona (frontmatter optional)
├── security-reviewer/
│   └── AGENTS.md
└── docs-writer/
    └── AGENTS.md
```

**File:** `.awf/agents/go-senior/AGENTS.md`
```markdown
---
name: go-senior
description: Senior Go engineer persona
---

You are a senior Go engineer with 10+ years of experience. 

When reviewing code:
- Prioritize readability and maintainability
- Suggest idiomatic Go patterns
- Consider performance implications
- Flag potential race conditions
- Reference Go proverbs and best practices
```

Reference the role by name in your workflow:

```yaml
review:
  type: agent
  provider: claude
  role: go-senior
  prompt: "Review this Go code: {{.inputs.code}}"
  on_success: done
```

When executed, the AGENTS.md content is stripped of frontmatter and injected as the system prompt to the agent, establishing the persona before the user prompt is sent.

### Multiple Roles

A single agent step uses one role only. To combine multiple personas, either:
1. Create a composite role that includes all relevant perspectives
2. Chain multiple agent steps with different roles

```yaml
# ❌ Invalid: only one role per step
step:
  type: agent
  role: go-senior
  role: security-reviewer  # Error

# ✅ Valid: chain steps for multiple perspectives
initial_review:
  type: agent
  provider: claude
  role: go-senior
  prompt: "Review this code: {{.inputs.code}}"
  on_success: security_review

security_review:
  type: agent
  provider: claude
  role: security-reviewer
  prompt: |
    After this Go review:
    {{.states.initial_review.Output}}
    
    Now analyze for security issues: {{.inputs.code}}
  on_success: done
```

### Role Discovery

AWF searches for roles in the following directories, in priority order:

1. **`AWF_AGENTS_PATH` environment variable** (if set, exclusive — overrides all others)
2. **`.awf/agents/`** (project-level, AWF-specific)
3. **`.agents/`** (project-level, cross-client compatibility)
4. **`$XDG_CONFIG_HOME/awf/agents/`** (global user, AWF-specific — defaults to `~/.config/awf/agents/`)
5. **`~/.agents/`** (global user, cross-client)

This enables shared global personas while allowing project-specific overrides.

**Example with environment variable:**

```bash
# Use only roles from custom directory
AWF_AGENTS_PATH=/shared/agents awf run workflow
```

### Role Content Format

**Frontmatter is optional** — AWF strips YAML frontmatter (content between `---` delimiters) if present, preserving only the Markdown body:

```markdown
---
name: go-senior
description: Senior Go engineer persona
license: MIT
tags: [go, senior, code-review]
---

You are a senior Go engineer...
```

Only the Markdown body is injected as the system prompt. The role name comes from the directory name (e.g., `go-senior`), not from frontmatter.

### Combining Role with Inline System Prompt

A step can combine a role with an inline `system_prompt` field. The role content is injected first, followed by the inline prompt, separated by a blank line:

```yaml
review:
  type: agent
  provider: claude
  role: go-senior
  system_prompt: "Focus on performance optimizations and memory leaks."
  prompt: "Review this code: {{.inputs.code}}"
  on_success: done
```

The effective system prompt sent to the agent is:

```
<go-senior AGENTS.md content>

Focus on performance optimizations and memory leaks.
```

This allows you to reuse a base persona while adding step-specific context or overrides.

### Explicit Path References

Reference roles by explicit path instead of discovery:

```yaml
review:
  type: agent
  provider: claude
  role: ./custom-agents/senior-go
  prompt: "Review: {{.inputs.code}}"
  on_success: done
```

Paths can be:
- **Relative** to the workflow directory: `role: ./custom-agents/senior-go`
- **Absolute**: `role: /home/user/agents/senior-go`
- **Home-relative**: `role: ~/shared-agents/senior-go`

### Dynamic Role Selection

The `role` field supports template interpolation, enabling dynamic role selection based on workflow inputs or state:

```yaml
inputs:
  - name: persona
    type: string
    default: go-senior

review:
  type: agent
  provider: claude
  role: "{{.inputs.persona}}"
  prompt: "Review: {{.inputs.code}}"
  on_success: done
```

```bash
awf run workflow --input persona=security-reviewer --input code="..."
```

### Validation

Use `awf validate` to check that all role references exist and are readable:

```bash
awf validate workflow.yaml
```

Validation reports:
- ✓ Roles found with valid `AGENTS.md` files
- ✗ Roles referenced but not found in any discovery path
- ✗ Role directories found but missing `AGENTS.md`
- ⚠ Empty `AGENTS.md` files (warning — content will be empty)
- ⚠ `AGENTS.md` exceeds 500KB (warning — context window impact)
- ⚠ Combined `role + system_prompt` exceeds 10KB (warning — context window impact)

### Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| `AgentRoleNotFoundError` | Referenced role not found in discovery paths | Check role directory name matches YAML declaration; verify `AGENTS.md` exists |
| Empty role content | `AGENTS.md` is 0 bytes or contains only frontmatter | Add Markdown body to `AGENTS.md` |
| Role not in expected location | Wrong discovery directory | Move role to one of the 5 standard directories or use explicit path reference |
| Large role file warning | `AGENTS.md` exceeds 500KB | Consider splitting into multiple smaller roles or removing verbose content |
| Combined prompt too large | `role + system_prompt` over 10KB | Reduce role/prompt size or split into separate steps |

### Conversation Mode with Roles

Roles work seamlessly in conversation mode (`mode: conversation`). The role is resolved and injected once at the start of the conversation, establishing the agent's persona for all subsequent user turns:

```yaml
chat:
  type: agent
  provider: claude
  mode: conversation
  role: go-senior
  system_prompt: "Be helpful and concise."
  prompt: "{{.inputs.topic}}"
  timeout: 600
  on_success: done
```

### Limitations

- A single agent step uses one role only — no composition or inheritance
- No template interpolation is applied to role content — it is injected as-is
- Roles do not reference other roles; only workflow steps reference roles
- Roles establish system-level persona via the standard `system_prompt` mechanism available to all providers (Claude native `--system-prompt`, CLI-based providers via first-turn concat, HTTP providers via API field)

## External Prompt Files

Instead of inlining prompts in YAML, you can load prompts from external Markdown files using the `prompt_file` field:

```yaml
analyze:
  type: agent
  provider: claude
  prompt_file: prompts/code_review.md
  timeout: 120
  on_success: done
```

**File:** `prompts/code_review.md`
```markdown
# Code Review Instructions

Analyze the following file for:
- Performance issues
- Security vulnerabilities
- Code style violations

## File Path
{{.inputs.file_path}}

## File Content
{{.inputs.file_content}}

## Language
{{.inputs.language}}
```

### Features

- **Full Template Interpolation** — Same variable access as inline prompts
- **Helper Functions** — String manipulation directly in templates
- **Path Resolution** — Relative paths resolve to workflow directory
- **XDG Directory Support** — Access system directories via `{{.awf.*}}`

### Mutual Exclusivity

You cannot specify both `prompt` and `prompt_file` on the same agent step:

```yaml
# ❌ Invalid: both prompt and prompt_file
step:
  type: agent
  provider: claude
  prompt: "Do this"
  prompt_file: "prompts/template.md"  # ERROR: only one allowed

# ✅ Valid: prompt only
step:
  type: agent
  provider: claude
  prompt: "Do this"

# ✅ Valid: prompt_file only
step:
  type: agent
  provider: claude
  prompt_file: "prompts/template.md"
```

### Path Resolution

Paths can be:

1. **Relative to workflow directory:**
   ```yaml
   prompt_file: prompts/analyze.md           # Resolves to <workflow_dir>/prompts/analyze.md
   ```

2. **Absolute paths:**
   ```yaml
   prompt_file: /home/user/my-prompts/template.md
   ```

3. **Home directory expansion:**
   ```yaml
   prompt_file: ~/my-prompts/template.md      # Expands to user's home directory
   ```

4. **XDG prompts directory with local override** — via template interpolation with local-before-global resolution:
   ```yaml
   prompt_file: "{{.awf.prompts_dir}}/analyze.md"
   # Checks in order:
   # 1. <workflow_dir>/prompts/analyze.md (local override)
   # 2. ~/.config/awf/prompts/analyze.md (global fallback)
   ```

#### Local-Before-Global Resolution

When using `{{.awf.prompts_dir}}` in `prompt_file`, AWF prioritizes local project files over global ones:

- **If local file exists** at `<workflow_dir>/prompts/<suffix>` → use it
- **If local file missing** → fall back to global `~/.config/awf/prompts/<suffix>`

This enables shared prompts at the global level while allowing projects to override them locally:

```yaml
# Workflow at: ~/myproject/.awf/workflows/review.yaml
analyze:
  type: agent
  provider: claude
  prompt_file: "{{.awf.prompts_dir}}/code_review.md"
  on_success: done
```

Resolution order:
1. Check `~/myproject/.awf/workflows/prompts/code_review.md` (local override)
2. Check `~/.config/awf/prompts/code_review.md` (global shared)

### Template Helper Functions

When interpolating prompt templates, four helper functions are available:

#### `split`

Split a string into an array:

```markdown
## Selected Agents

{{range split .states.select_agents.Output ","}}
- {{trimSpace .}}
{{end}}
```

#### `join`

Join an array into a string:

```markdown
Skills to use: {{join .states.available_skills.Output ", "}}
```

#### `readFile`

Inline file contents (with 1MB size limit):

```markdown
## Specification

{{readFile .states.get_spec.Output}}
```

#### `trimSpace`

Remove leading/trailing whitespace:

```markdown
Result: {{trimSpace .states.process.Output}}
```

### Example: Multi-File Workflow

**Workflow:** `code-review.yaml`
```yaml
name: code-review
version: "1.0.0"

inputs:
  - name: file_path
    type: string
    required: true
    validation:
      file_exists: true
  - name: focus_areas
    type: string

states:
  initial: read_file

  read_file:
    type: step
    command: cat "{{.inputs.file_path}}"
    on_success: analyze

  analyze:
    type: agent
    provider: claude
    prompt_file: prompts/code_review.md
    timeout: 120
    on_success: done

  done:
    type: terminal
```

**Template:** `prompts/code_review.md`
```markdown
# Code Review

File: `{{.inputs.file_path}}`

Focus on:
{{.inputs.focus_areas}}

## Code to Review

{{.states.read_file.Output}}

Provide:
1. Issues found
2. Suggested fixes
3. Overall assessment
```

Run:
```bash
awf run code-review --input file_path=main.py --input focus_areas="Performance and security"
```

## Capturing Responses

Agent responses are automatically captured in the execution state:

| Field | Type | Description |
|-------|------|-------------|
| `{{.states.step_name.Output}}` | string | Raw response text (or cleaned text if `output_format` is set) |
| `{{.states.step_name.Response}}` | object | Parsed JSON response (automatic heuristic) |
| `{{.states.step_name.JSON}}` | object | Parsed JSON from `output_format: json` (explicit, see [Output Formatting](#output-formatting)) |
| `{{.states.step_name.TokensUsed}}` | int | Total tokens consumed by this step |
| `{{.states.step_name.TokensInput}}` | int | Input tokens (prompt + context). `0` in single-turn mode. |
| `{{.states.step_name.TokensOutput}}` | int | Output tokens (assistant response) |
| `{{.states.step_name.TokensEstimated}}` | bool | `false` when tokens come from the provider, `true` when estimated |
| `{{.states.step_name.ExitCode}}` | int | 0 for success, non-zero for failure |

### Accessing Raw Output

```yaml
report_results:
  type: step
  command: echo "Agent said: {{.states.analyze.Output}}"
  on_success: done
```

### Parsing JSON Responses

If an agent returns valid JSON, it's automatically parsed:

```yaml
# Agent returns: {"issues": ["bug1", "bug2"], "severity": "high"}

process_response:
  type: step
  command: echo "Found {{.states.analyze.Response.issues}} issues"
  on_success: done
```

## Output Formatting

The `output_format` field serves two purposes:

1. **Post-processing**: Strips markdown code fences and optionally validates JSON (F065)
2. **Display filtering**: Controls how agent responses appear on terminal during streaming and buffered execution, with optional verbose tool-use markers (F082, F085)

When an agent wraps its output in markdown code fences (common with many LLMs), use `output_format` to automatically strip the fences and optionally validate the content:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Return JSON analysis"
  output_format: json
  on_success: process
```

### Available Formats

#### `json` Format

Strips markdown code fences and validates the output as valid JSON. Parsed JSON is accessible via `{{.states.step_name.JSON}}`:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: |
    Analyze the code and return results as JSON:
    {
      "issues": [<list of issues>],
      "severity": "high|medium|low"
    }
  output_format: json
  on_success: process_results

process_results:
  type: step
  command: echo "Severity: {{.states.analyze.JSON.severity}}"
  on_success: done
```

**Behavior:**
- Strips outermost markdown code fences (e.g., ````json ... ``` ``)
- Validates stripped content as valid JSON
- Stores parsed JSON in `{{.states.step_name.JSON}}`
- If validation fails, step fails with a descriptive error
- Works with both objects and arrays

**Example agent output:**
```
The analysis shows the following:
```json
{"issues": ["buffer overflow", "memory leak"], "severity": "high"}
```
```

**After processing:**
- `{{.states.analyze.Output}}` = `{"issues": ["buffer overflow", "memory leak"], "severity": "high"}`
- `{{.states.analyze.JSON.issues}}` = `["buffer overflow", "memory leak"]`
- `{{.states.analyze.JSON.severity}}` = `"high"`

#### `text` Format

Strips markdown code fences without JSON validation. Useful for code or plain text output:

```yaml
generate_code:
  type: agent
  provider: claude
  prompt: "Generate a Python function to..."
  output_format: text
  on_success: save_code

save_code:
  type: step
  command: echo "{{.states.generate_code.Output}}" > generated.py
  on_success: done
```

**Behavior:**
- Strips outermost markdown code fences (e.g., ````python ... ``` ``)
- Returns clean text in `{{.states.step_name.Output}}`
- Does not populate `{{.states.step_name.JSON}}`

**Example agent output:**
```
Here's the function:
```python
def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)
```
```

**After processing:**
- `{{.states.generate_code.Output}}` = `def fibonacci(n):\n    if n <= 1:\n        return n\n    return fibonacci(n-1) + fibonacci(n-2)`

#### No Format (Default)

Omit `output_format` for backward compatibility. Raw agent output is stored unchanged:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Analyze this code"
  on_success: next
```

### Streaming Output Display & Tool Markers

The `output_format` field controls how agent responses appear on the terminal (F082). Additionally, the `--verbose` flag displays tool-use activity markers (F085) — showing which tools the agent invoked — alongside agent output when running with `awf run --output streaming` or `--output buffered`:

| `output_format` | Streaming Display | Buffered Display | Raw Storage |
|---|---|---|---|
| `text` (or omitted) | Human-readable filtered text | Filtered text in summary | Raw NDJSON |
| `json` | Raw NDJSON (unfiltered) | Raw NDJSON (unfiltered) | Raw NDJSON |

#### Streaming Mode (`--output streaming`)

When running with streaming output, agent responses display incrementally as they're generated:

```bash
# Raw NDJSON appears on terminal (hard to read)
awf run code-review --output streaming
# Output: {"type":"content_block_delta",...}{"type":"content_block_delta",...}

# Human-readable text with default output_format
awf run code-review --output streaming  # output_format: text (or omitted)
# Output: The code has several issues...
```

**Filtering behavior:**
- `output_format: text` or omitted — Extracted text content displayed (filtered NDJSON)
- `output_format: json` — Raw NDJSON passed through unchanged

**Tool-Use Markers (Verbose Mode):**

When running with `--verbose` flag, agent tool invocations are displayed as markers alongside agent text:

```bash
# Without verbose mode (default — text only)
awf run code-review --output streaming
# Output: The code has several issues...

# With verbose mode (text + tool markers)
awf run code-review --output streaming --verbose
# Output: [tool: Read(main.py)]The code has several issues...[tool: Bash(grep -n "TODO" main.py)]
```

Tool markers show:
- **Tool name** (e.g., `Read`, `Write`, `Edit`, `Bash`, `Grep`, `Glob`, `Task`)
- **Truncated argument** (≤ 40 characters) in parentheses for context
- **Interleaved order** — markers appear in the same source order as agent output
- **Graceful degradation** — unknown tool names display as-is with no crash or error

This works consistently across all 6 supported providers (Claude, Codex, Gemini, GitHub Copilot, OpenCode, OpenAI-Compatible). Verbose mode has no effect on `output_format: json` — raw NDJSON is always passed through unchanged.

#### Buffered Mode (`--output buffered`)

When running with buffered output, the post-execution summary displays filtered text:

```bash
awf run code-review --output buffered

# With output_format: text (or omitted):
# Output of "analyze" step:
# The code has several issues...

# With output_format: json:
# Output of "analyze" step:
# {"type":"content_block_delta",...}
```

#### Silent Mode (`--output silent`)

Silent mode suppresses all display regardless of `output_format`:

```bash
awf run code-review --output silent
# No output displayed (silent mode is absolute)
# state.Output still contains raw NDJSON for template interpolation
```

**Note:** `state.Output` always contains the raw NDJSON regardless of display filtering. Filtering only affects terminal display, not data storage.

#### Provider Event Cadence

CLI-based providers (Claude, Codex, Gemini, OpenCode) emit display events incrementally as stream-json lines arrive. The `openai_compatible` provider emits all display events in a single post-response burst after the HTTP response completes — the rendered shape is identical to streaming providers, but "live feedback" timing differs because events are not streamed.

#### Line Buffer Cap

The stream filter processes NDJSON events line by line with a **10 MB per-line cap**. Individual events up to 10 MB are parsed and displayed normally. If a single NDJSON event exceeds 10 MB (e.g., a very large `content_block_delta` or `tool_use.input` payload), the line is written to `state.Output` as raw text and a structured warning is logged with the line size. Subsequent events continue processing normally — the stream is not aborted.

In parallel execution, each step has its own stream filter, so the worst-case memory for N concurrent agent steps is N × 10 MB for the line buffers.

### Error Handling

When `output_format: json` is specified but the output is invalid JSON:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Return valid JSON"
  output_format: json
  timeout: 60
  on_failure: handle_json_error

handle_json_error:
  type: step
  command: echo "JSON parsing failed"
  on_success: done
```

**Error message includes:**
- Clear indication of JSON validation failure
- First 200 characters of the malformed output (for debugging)
- Suggestions on how to fix the issue

## Multi-Turn Conversations

AWF offers three approaches for multi-turn interactions, from stateless to fully session-tracked.

### 1. State Passing (Chained Steps, Stateless)

Chain agent steps via template interpolation. Each agent call is stateless — the provider has no memory of prior steps — but the next prompt carries the prior step's output as text. Cheapest and simplest.

```yaml
initial_review:
  type: agent
  provider: claude
  prompt: |
    Review this code for issues:
    {{.inputs.code}}
  on_success: follow_up

follow_up:
  type: agent
  provider: claude
  prompt: |
    Based on your previous analysis:
    {{.states.initial_review.Output}}

    Can you elaborate on performance concerns?
  on_success: done
```

Use this when the prior output is small and the agent doesn't need implicit memory of prior conversation turns.

### 2. Cross-Step Session Tracking

Add a `conversation:` sub-struct to an agent step (still `mode: single`, the default) to have AWF call `provider.ExecuteConversation` — one turn only, but the provider's session ID is captured. A later step with `conversation: {continue_from: prior_step}` clones that session state and resumes the actual provider-side conversation.

```yaml
seed:
  type: agent
  provider: claude
  system_prompt: "You are a memory test assistant."
  prompt: |
    Remember this secret: BANANA42.
    Reply "stored".
  conversation: {}            # opt into session tracking
  on_success: recall

recall:
  type: agent
  provider: claude
  prompt: "What was the secret?"
  conversation:
    continue_from: seed       # resume seed's session
  on_success: done
```

No interactive loop, no stdin. Each step runs exactly one agent turn. The provider retains the conversation between steps via its native session store (`claude -r`, `gemini --resume`, `codex resume`, `opencode -s`).

Use this when the agent needs implicit memory of earlier turns or when prior context is large and you want to avoid re-sending it in each prompt.

### 3. Interactive Conversation Mode

`mode: conversation` spawns a live user-driven chat loop: the agent replies, AWF prompts for your next message via stdin, and the loop continues until you submit an empty line, `exit`, or `quit`.

```yaml
chat:
  type: agent
  provider: claude
  mode: conversation
  system_prompt: "You are a concise technical assistant."
  prompt: "{{.inputs.topic}}"
  timeout: 600
  on_success: done
```

Requires a TTY. Use this for human-in-the-loop clarification sessions or iterative prompting driven by a user.

See [Conversation Mode & Session Tracking](conversation-steps.md) for the complete reference, including `continue_from` rules, cross-provider limitations, and observability fields.

## Error Handling

Agent steps follow standard error handling:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review: {{.inputs.code}}"
  timeout: 120
  on_success: success_path
  on_failure: error_path
  retry:
    max_attempts: 3
    backoff: exponential
    initial_delay: 2s

success_path:
  type: terminal

error_path:
  type: terminal
  status: failure
```

You can also use **inline error shorthand** to avoid defining separate terminal states:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review: {{.inputs.code}}"
  timeout: 120
  on_success: done
  on_failure: {message: "Agent analysis failed", status: 3}

done:
  type: terminal
```

See [Workflow Syntax — Inline Error Shorthand](workflow-syntax.md#inline-error-shorthand) for full details.

### Common Error Scenarios

| Error | Cause | Solution |
|-------|-------|----------|
| Provider not found | CLI tool not installed | Install required CLI (e.g., `claude install`) |
| Skill not found | Skill name doesn't match any directory in discovery paths | Check skill name and discovery directories (see [Agent Skills](#agent-skills)) |
| Timeout | Agent response took too long | Increase timeout or reduce prompt complexity |
| Invalid provider | Unsupported provider | Use `claude`, `codex`, `gemini`, `opencode`, or `openai_compatible` |
| Command failed | Provider CLI returned error | Check provider configuration and logs |

### Debugging

Use `--dry-run` to preview resolved prompts without execution:

```bash
awf run workflow --dry-run
# Shows: [DRY RUN] Agent: claude
# Prompt: <resolved prompt text>
```

## Parallel Agent Execution

Run multiple agents concurrently:

```yaml
parallel_analysis:
  type: parallel
  parallel:
    - claude_review
    - codex_suggest
  strategy: all_succeed
  on_success: aggregate

claude_review:
  type: agent
  provider: claude
  prompt: "Analyze for security: {{.inputs.code}}"

codex_suggest:
  type: agent
  provider: codex
  prompt: "Optimize performance: {{.inputs.code}}"

aggregate:
  type: step
  command: echo "Claude: {{.states.claude_review.Output}}\nCodex: {{.states.codex_suggest.Output}}"
  on_success: done
```

## Token Tracking

All agent providers report token usage in the `TokensUsed` field:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review: {{.inputs.code}}"
  options:
    model: claude-sonnet-4-20250514
  on_success: log_tokens

log_tokens:
  type: step
  command: echo "Tokens used: {{.states.analyze.TokensUsed}}"
  on_success: done
```

**How it works:**

All 6 providers extract **real token counts** from their CLI/API JSON output when available. `TokensEstimated` is `false` in this case. If the provider output does not contain token data, AWF falls back to an approximation (`len(output)/4`) and sets `TokensEstimated` to `true`.

| Provider | Source of real tokens | Fields available |
|----------|----------------------|-----------------|
| Claude | `result` event `usage` field | input, output, cost |
| Gemini | `result` event `stats` field | input, output, total |
| Codex | `turn.completed` event `usage` field | input, output |
| Copilot | `assistant.message` event | output only |
| OpenCode | `step_finish` event `part.tokens` field | input, output, total, cost |
| OpenAI-Compatible | API response `usage` field | input, output, total |

Use `TokensInput` and `TokensOutput` for detailed tracking:

```yaml
log_details:
  type: step
  command: |
    echo "Input: {{.states.analyze.TokensInput}}, Output: {{.states.analyze.TokensOutput}}"
    echo "Estimated: {{.states.analyze.TokensEstimated}}"
  on_success: done
```

In conversation mode (`continue_from`), `TokensInput` includes all prior turns. In single-turn mode, `TokensInput` is `0` and `TokensOutput` equals `TokensUsed`.

## Best Practices

### 1. Keep Prompts Focused

Long, complex prompts may hit token limits or timeout. Break into multiple steps:

```yaml
# ❌ Too much
ask_everything:
  type: agent
  provider: claude
  prompt: |
    Review code for security, performance, style, and suggest
    improvements, then estimate refactoring effort...

# ✅ Better
security_review:
  type: agent
  provider: claude
  prompt: "Security review: {{.inputs.code}}"
  on_success: performance_review

performance_review:
  type: agent
  provider: claude
  prompt: |
    After this security review:
    {{.states.security_review.Output}}

    Now analyze performance: {{.inputs.code}}
  on_success: done
```

### 2. Use Consistent Formatting

Request structured output when relevant:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: |
    Analyze code and respond in JSON format:
    {
      "issues": [...],
      "severity": "high|medium|low",
      "estimate_hours": number
    }

    Code: {{.inputs.code}}
  on_success: process_response
```

### 3. Add Timeouts

Always set reasonable timeouts:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review: {{.inputs.code}}"
  timeout: 120  # 2 minutes
  on_success: next
```

### 4. Test with Dry-Run

Preview prompts before running:

```bash
awf run my-workflow --dry-run --input file=/path/to/file
```

### 5. Handle Missing Providers

Test that required providers are installed:

```yaml
states:
  initial: check_claude

  check_claude:
    type: step
    command: which claude
    on_success: analyze
    on_failure: install_claude

  analyze:
    type: agent
    provider: claude
    prompt: "Review: {{.inputs.code}}"
    on_success: done

  install_claude:
    type: terminal
    status: failure

  done:
    type: terminal
```

## See Also

- [Workflow Syntax Reference](workflow-syntax.md#agent-state) - Complete agent step options
- [Template Variables](../reference/interpolation.md) - Available interpolation variables
- [Examples](examples.md) - More workflow examples
