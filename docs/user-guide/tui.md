---
title: "Terminal User Interface (TUI)"
description: "Interactive terminal interface for browsing, executing, and monitoring AWF workflows"
---

## Overview

The AWF Terminal User Interface (TUI) is an interactive dashboard for managing your workflows without touching the command line. It provides real-time visibility into workflow execution, browsing of available workflows, exploration of execution history, and monitoring of agent conversations.

Launch the TUI at any time with:

```bash
awf tui
```

The TUI is designed for terminal environments and requires a minimum of 80x24 character dimensions.

---

## Getting Started with the TUI

### Launching the TUI

```bash
# Start the TUI
awf tui

# The TUI will initialize with:
# - Auto-detected project configuration from .awf/config.yaml
# - Loaded workflows from .awf/workflows/, global, and installed packs
# - Recent execution history from history.db
```

### Tab Navigation

The TUI is organized into five tabs. Switch between them using keyboard shortcuts:

| Tab | Keyboard | Purpose |
|-----|----------|---------|
| **Workflows** | `1` or `Ctrl+1` | Browse, filter, and launch workflows |
| **Monitoring** | `2` or `Ctrl+2` | Watch real-time execution of active workflows |
| **History** | `3` or `Ctrl+3` | Explore past execution records |
| **Agent Conversations** | `4` or `Ctrl+4` | View agent interactions with formatting |
| **External Logs** | `5` or `Ctrl+5` | Live tail Claude Code session files |

> **Tip:** Use `Tab` to cycle forward and `Shift+Tab` to cycle backward through tabs.

---

## The Workflows Tab

The **Workflows** tab is your entry point for discovering and launching workflows.

### Finding Workflows

1. **Browse the list**: All available workflows appear in chronological order (most recently modified first) or by name (alphabetical)
2. **Filter the list**: Press `f` and type to search by workflow name or description
3. **View details**: Press `?` on a selected workflow to see its description, inputs, and step count

### Launching a Workflow

1. **Select a workflow** from the list using arrow keys
2. **Press Enter** to launch
3. **Provide inputs** in the prompted dialog (if required by the workflow)
4. **Confirm** and the TUI will switch to the Monitoring tab

#### Interactive Input Collection

If a workflow requires inputs, the TUI prompts you in a form:

```
Workflow: deploy-app

Input Parameters:
  environment (string, required)
  > prod

  version (string, required)
  > 1.2.3

  timeout (integer, optional, default: 300)
  > (press Enter for default)
```

Enum inputs present numbered options:

```
environment (string, required):
Available options:
  1) dev
  2) staging
  3) prod
Select (1-3):
> 2
```

### Validating Workflows

Validate a workflow without executing it:

1. **Select a workflow** from the list
2. **Press `v`** to validate
3. **View results** in a popup overlay (errors or success confirmation)

---

## The Monitoring Tab

The **Monitoring** tab provides real-time visibility into workflow execution with an execution tree, status indicators, and live log streaming.

### Understanding the Execution Tree

The tree represents the workflow's state structure:

```
▶ main (duration: 0.5s)
  ├─ ✓ read_input (0.1s)
  ├─ ▶ process (0.3s)
  │  ├─ ▶ validate (0.1s)
  │  └─ ⏳ transform
  └─ ⏳ output
```

| Icon | Meaning |
|------|---------|
| ⏳ | Pending (waiting to run) |
| ▶ | Running (currently executing) |
| ✓ | Success (completed successfully) |
| ✗ | Failed (encountered an error) |
| ⊘ | Skipped (not executed) |

### Viewing Step Logs

1. **Select a step** in the execution tree using arrow keys
2. **View live logs** in the right panel updating in real-time
3. **Manual scroll**: Press `Page Up`/`Page Down` or `j`/`k` to scroll
4. **Toggle auto-scroll**: Press `a` to lock/unlock automatic scrolling

> **Tip:** When a step fails, the TUI automatically selects it and scrolls the log viewport to show the error message.

### Parallel Execution Tracking

When a workflow runs parallel steps, they appear as siblings in the tree:

```
▶ process-batch
  ├─ ▶ task_1 (0.2s)
  ├─ ✓ task_2 (0.4s)
  ├─ ▶ task_3 (0.1s)
  └─ ⏳ task_4
```

Each parallel task has independent status tracking and log output.

### Stopping a Running Workflow

Press `Ctrl+C` while monitoring a workflow to request cancellation. The workflow will attempt graceful shutdown within the configured timeout.

---

## The History Tab

The **History** tab lets you explore past workflow executions without re-running them.

### Browsing Execution History

The history list shows recent executions in reverse chronological order:

```
Workflow          ID                 Status    Start Time              Duration
deploy            abc123-def456      success   2025-05-01 14:30:22     2m 45s
build             ghi789-jkl012      failed    2025-05-01 13:15:10     45s
test              mno345-pqr678      success   2025-05-01 12:00:00     1m 30s
```

### Filtering History

Press `f` to open the filter dialog:

```
Filter History

Workflow (optional):
> deploy

Status (optional):
  1) All
  2) success
  3) failed
  4) cancelled
Select: > 1

Date Range:
From (YYYY-MM-DD, optional):
> 2025-05-01

To (YYYY-MM-DD, optional):
> (press Enter for today)
```

### Inspecting Execution Details

1. **Select an execution** from the filtered list
2. **Press Enter** to view details
3. **Review the execution tree** with final step statuses
4. **Examine step outputs** by selecting steps
5. **Press Escape** to return to the history list

---

## The Agent Conversations Tab

The **Agent Conversations** tab renders multi-turn agent interactions with proper Markdown formatting.

### Viewing Agent Messages

When a workflow with agent steps is running, the Conversations tab shows:

```
╔════════════════════════════════════════════════════════════════════╗
║                      Agent Conversation                             ║
╚════════════════════════════════════════════════════════════════════╝

USER: Analyze this code for performance issues.

function fibonacci(n) {
  if (n <= 1) return n;
  return fibonacci(n-1) + fibonacci(n-2);
}

AGENT (Claude): This recursive implementation has exponential time
complexity O(2^n) due to overlapping subproblems.

**Recommendations:**
1. Use memoization to cache results
2. Consider iterative approach
3. Dynamic programming solution optimal for n < 93

[Would you like me to show optimization examples?]
```

### Message Types

- **User messages**: System prompts and user-provided inputs
- **Agent responses**: LLM outputs with Markdown formatting (headings, lists, code blocks)
- **Tool use**: If enabled, shows tool invocations and results
- **Approval prompts**: Interactive yes/no decisions (if applicable)

### Scrolling Through Conversations

- Use arrow keys or `j`/`k` to scroll line-by-line
- `Page Up`/`Page Down` for page-by-page navigation
- Press `Home`/`End` to jump to conversation start/end

---

## The External Logs Tab

The **External Logs** tab live-tails Claude Code JSONL session files for monitoring development sessions alongside workflow execution.

### Automatic Session Detection

The TUI automatically discovers and tails the latest Claude Code session:

1. **Monitors** `~/.claude/projects/*/session.jsonl`
2. **Auto-switches** to the most recently modified file
3. **Displays new entries** within 2 seconds of being written

### Session Output Format

```
[14:30:22] User: Implement OAuth2 authentication
[14:30:45] Assistant: I'll implement OAuth2 with these components...
[14:31:12] Tool: claude-code exec npm install
[14:31:20] Output: ✓ Installed 5 packages
[14:31:35] Assistant: Now implementing the provider...
```

### When No Session Is Active

```
╔════════════════════════════════════════════════════════════════════╗
║                    External Logs (Claude Code)                      ║
╚════════════════════════════════════════════════════════════════════╝

No Claude Code session file detected.

Expected location:
~/.claude/projects/<project-id>/session.jsonl

To start monitoring:
1. Open Claude Code: https://claude.ai/code
2. Create a new session
3. The TUI will auto-detect and begin tailing

```

### Pausing and Resuming

- Press `Space` to pause tailing (useful for reading a specific section)
- Press `Space` again to resume auto-scrolling

---

## Common Workflows

### Workflow: Launch and Monitor

The most common TUI workflow:

1. Launch TUI: `awf tui`
2. Navigate to **Workflows** tab (press `1`)
3. Filter by name (press `f`, type workflow name)
4. Select workflow (arrow keys, press Enter)
5. Provide inputs when prompted
6. TUI auto-switches to **Monitoring** tab
7. Watch execution in real-time
8. Inspect logs by selecting steps
9. When complete, view summary status

### Workflow: Investigate a Failed Execution

Debugging a recent failure:

1. Launch TUI: `awf tui`
2. Navigate to **History** tab (press `3`)
3. Filter by status (press `f`, select "failed")
4. Select the failed execution
5. Review the execution tree (red ✗ marks)
6. Select the failed step to view error logs
7. Review related steps' output for context
8. Navigate to **Workflows** tab to re-run the workflow with corrections

### Workflow: Compare Recent Runs

Analyzing trends across recent executions:

1. Navigate to **History** tab
2. Select first execution (press Enter to view)
3. Note timing and outputs
4. Press Escape to return to list
5. Select another execution for comparison
6. Compare step durations and status patterns

### Workflow: Monitor Agent Development

Watching a development agent alongside workflow execution:

1. Have a Claude Code session open in another window
2. Launch TUI: `awf tui`
3. Start a workflow in **Workflows** tab
4. Switch between tabs to monitor:
   - **Monitoring**: Workflow execution status
   - **Agent Conversations**: Multi-turn interactions
   - **External Logs**: Claude Code session activity
5. All three views update in real-time

---

## Tips & Tricks

### Keyboard Efficiency

- **Quick tab navigation**: Use `Ctrl+1` through `Ctrl+5` for direct tab jumping
- **Search filtering**: Always available in list views with `f` key
- **Quick quit**: `q` exits cleanly (or `Ctrl+C`)
- **Refresh view**: `r` manually refreshes (auto-refresh happens every 200ms)

### Terminal Optimization

- **Maximize terminal window** for better tree visualization and log readability
- **Use a terminal with true color support** for accurate status icon rendering
- **Increase scroll buffer** in your terminal settings if you need to review long logs

### Performance Tips

- **Close other applications** if terminal rendering is slow (large log output or slow machine)
- **Disable verbose mode** in workflows (`--verbose` flag not needed in TUI)
- **Limit history entries** in config if database is very large

### Secret Handling

The TUI automatically masks sensitive values in all views:

- Environment variables starting with `SECRET_`, `API_KEY`, or `PASSWORD` are masked as `***`
- This applies to logs, execution tree details, and agent conversation output
- Masking happens at render time (original data is never exposed in TUI)

---

## Troubleshooting

### TUI won't start

**Symptom:** `awf tui` command not found

**Solution:**
```bash
# Rebuild the project
make build

# Or reinstall
make install
```

### Execution tree is empty

**Symptom:** Monitoring tab shows no tree nodes

**Causes & Solutions:**
- Workflow hasn't started yet — check the status bar at bottom
- Workflow finished very quickly — switch back to Workflows tab and re-run
- Rendering issue — try pressing `r` to refresh or resizing terminal

### Logs not updating

**Symptom:** Log viewport stuck at old output

**Solutions:**
- Toggle auto-scroll off/on (press `a`)
- Refresh the view (press `r`)
- Check if workflow is actually running in status bar
- Terminal might be too small — resize to minimum 80x24

### History tab shows no entries

**Symptom:** History tab is empty despite running workflows

**Causes & Solutions:**
- No workflows have been executed yet — run a workflow first
- `history.db` file is corrupted — delete `.awf/storage/history.db` and re-run a workflow
- Permission issue on history.db — check file permissions

### Claude Code logs not appearing

**Symptom:** External Logs tab empty despite active Claude Code session

**Causes & Solutions:**
- Session file location differs from expected path — check `~/.claude/projects/`
- File format changed — the TUI expects JSONL format
- Tailer hasn't detected the file yet — wait a few seconds and refresh (press `r`)

### Colors look wrong

**Symptom:** Text is hard to read or colors are incorrect

**Solutions:**
- Check terminal `TERM` variable: `echo $TERM`
- Set correct term type: `export TERM=xterm-256color`
- Test with basic ANSI mode: the TUI auto-detects and degrades gracefully
- Verify terminal supports true color (most modern terminals do)

---

## Best Practices

### For Daily Operations

- **Keep TUI open** during development sessions to monitor background workflows
- **Use History tab** to track execution patterns and identify slowdowns
- **Combine with dry-run**: Use `awf run --dry-run` to preview before launching from TUI

### For Troubleshooting

- **Capture logs**: Take screenshots or pipe logs to file for analysis
- **Check Agent tab**: Before assuming a step failed, view the agent conversation
- **Cross-reference with CLI**: Use `awf status <id>` from terminal for raw JSON output

### For Performance Monitoring

- **Monitor step durations**: Watch for steps taking longer than expected
- **Check parallel performance**: Verify parallel steps complete as expected
- **Review history trends**: Spot regressions in execution time

---

## See Also

- [Commands Reference](commands.md#awf-tui) - `awf tui` command details
- [Workflow Syntax](workflow-syntax.md) - Define workflows for TUI execution
- [Interactive Inputs](interactive-inputs.md) - Prompting for workflow inputs
- [Execution History](./commands.md#awf-history) - CLI-based history access
