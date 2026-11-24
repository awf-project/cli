---
title: "Terminal User Interface (TUI)"
description: "Browse workflows, run them with live step output, and review past executions in an interactive terminal interface."
---

## Overview

The AWF Terminal User Interface (TUI) is an interactive dashboard for working with workflows without leaving the terminal. You can browse and filter workflows, inspect their inputs and steps, run them while watching each step's output as it is produced, answer agent conversation prompts, and review past executions.

```bash
awf tui
```

The TUI needs a real terminal of at least **80x24**. In a smaller terminal it shows a short message asking you to enlarge the window.

---

## Screens

The TUI has two screens, shown under a header with the **AWF** badge, the tab bar (with item counts) and the working directory. A third view, the **run view**, opens full-frame when a run starts.

| Screen | Keys | Shows |
|--------|------|-------|
| **Workflows** | `1` | The workflow list and the selected workflow's details |
| **History** | `2` | Past executions and the selected execution's details |
| **Run** | opens automatically | A run's steps, progress and output |

Move between the two tabs with `tab` / `shift+tab`, or press `1` / `2`. Press `q` to quit.

---

## The Workflows Screen

The list on the left shows each workflow with the icon and age of its last run (`✓` succeeded, `✗` failed, `⊘` cancelled, `○` never run). The panel on the right details the selected workflow:

- **Source** and version
- **Inputs**, with their defaults (defaults of secret inputs are shown as `***`)
- The ordered **steps** with their types
- The **last runs**, with their outcome, age and duration

| Key | Action |
|-----|--------|
| `↑` `↓` / `j` `k` | Move the selection |
| `g` / `G` | Jump to the first / last workflow |
| `/` | Filter by name or description (`enter` keeps the filter, `esc` clears it) |
| `x` or `enter` | Run the selected workflow |
| `v` | Validate the selected workflow (the result appears in the header) |
| `h` | Open the History screen on this workflow's runs |
| `r` | Reload workflows and history |

### Running with inputs

A workflow that declares inputs opens a form before it starts:

- Defaults are prefilled and required inputs are marked with `*`
- `tab` / `shift+tab` (or `↑` `↓`) move between fields; `enter` moves to the next field and starts the run on the last one
- Leaving a required input empty keeps the form open and tells you which one is missing
- Inputs named `SECRET_*`, `API_KEY*` or `PASSWORD*` are masked while you type
- `esc` cancels

---

## The Run View

Starting a run switches to the run view:

```
 AWF  deploy run 62a9eee4                                  ⠋ running · 00:12
 ──────────────────────────────────────────────────────────────────────────
 ██████████████████████░░░░░░░░░░░░░░░░░░░  1/3 steps  33%
 ╭─ Steps ──────────────╮ ╭─ Output · test (live) ──────────────────────────╮
 │ ✓ 1 build       4.2s │ │ ▶ Input:                                        │
 │ ⠋ 2 test        6.1s │ │   $ make test                                   │
 │ ○ 3 done     pending │ │ ◀ Output (live):                                │
 │ Inputs               │ │   running tests...                              │
 │ env prod             │ │                                                 │
 ╰──────────────────────╯ ╰─────────────────────────────────────────────────╯
```

- The **header** shows the workflow, the run identifier and the live status: a spinner and the elapsed time while running, then the outcome (`✓ done`, `✗ failed` or `⊘ cancelled`)
- The **Steps** box lists each step with its status, number and duration, then the inputs the run was started with (secret inputs masked)
- The **Output** box shows the selected step's input, its output and any error. Conversation steps are shown as a chat thread

### Live output

While a step is running, its stdout and stderr appear as they are produced, under *Output (live)* and *Stderr (live)*. When the step finishes, its complete output replaces the live view.

- The view keeps the most recent 256 KiB per stream. If older output was dropped, the panel says so, and the full output still appears when the step ends
- Live output is best effort: if a run produces it faster than the screen can display it, some of it can be skipped. The final output is always complete
- The selection follows the step that is running. Pick another step with `↑` `↓` to stay on it; press `f` to follow the running step again

### Keys

| Key | Action |
|-----|--------|
| `↑` `↓` | Pick a step |
| `f` | Follow the running step and the tail of the output |
| `pgup` / `pgdn`, mouse wheel | Scroll the output |
| `c` | Cancel the run (the header shows *cancelling…* until it stops) |
| `r` | Run the same workflow again with the same inputs (once the run has ended) |
| `y` | Copy the selected step's error, stderr and output to the clipboard |
| `esc` | Go back to the list. **The run keeps going** |
| `o` | (From the tabs) reopen the current run |
| `ctrl+c` | Cancel the running run; quit when no run is in flight |

### Conversations and prompts

When a workflow needs your input (for example an agent conversation step), the TUI switches to the run view, selects the step and shows a *Your message* field under the output. Type your reply and press `enter`. While the field is focused every key is typed text, so `c`, `r` and `y` do nothing until you send your message.

---

## The History Screen

The list shows executions newest first, with the outcome icon, workflow and age. The panel on the right shows the selected execution: its status, run ID, start time, duration, exit code, attempt and resume information, and the error message when it failed.

| Key | Action |
|-----|--------|
| `↑` `↓` / `j` `k` | Move the selection |
| `enter` | Open the step detail of the selected execution |
| `/` | Filter by workflow name |
| `s` | Cycle the status filter: all, success, failed |
| `esc` | Clear the text filter, then the workflow scope |
| `r` | Reload |

When you arrive with `h` from the Workflows screen, the list is limited to that workflow (exact name, so `deploy` does not include `deploy-prod`) and the title reads *History · deploy*. Press `esc` to see every workflow again.

The list footer shows totals (runs, successes, failures).

### Step detail

Press `enter` on an execution to see what each of its steps did, read from the state AWF saved for that run:

```
 ✗ deploy run-bbbb                                          yesterday 11:56 · 3.1s
 ╭─ Steps ──────────────╮ ╭─ Output · test (1–22/48) ───────────────────────────╮
 │ ✓ 1 build       5.0s │ │ ✗ Error:                                            │
 │ ✗ 2 test        3.0s │ │   exit status 2                                     │
 │ ○ 3 done     pending │ │ ⚠ Stderr:                                           │
 │ Inputs               │ │   FAIL: TestX                                       │
 │ env prod             │ │ ◀ Output:                                           │
 ╰──────────────────────╯ ╰─────────────────────────────────────────────────────╯
```

- The **Steps** box lists the steps in the order they started, with their status and duration, then the inputs the run was started with (secret inputs masked)
- The **Output** box shows the selected step's error, stderr and output, or its conversation for an agent step. The first failed step is selected for you, so the reason a run failed is on screen right away
- A step that never ran says so, and so does a step that was still running when the run stopped

| Key | Action |
|-----|--------|
| `↑` `↓` / `j` `k` | Pick a step |
| `pgup` / `pgdn`, mouse wheel | Scroll the output |
| `home` / `end` | Jump to the start / end of the output |
| `y` | Copy the step's error, stderr and output to the clipboard |
| `r` | Run the workflow again with the inputs this execution was started with |
| `R` | Reload the detail |
| `esc` | Back to the list |

`r` reads the inputs back from the saved state; if that state is gone, nothing is started and the header says the inputs are not available.

The detail comes from the state file AWF keeps for each run. If that file was removed, or the run saved no step data, the screen says so instead of showing steps. A run that was resumed keeps a single state, so its detail reflects the latest attempt.

---

## Secret Handling

Input values that look like secrets (names starting with `SECRET_`, `API_KEY` or `PASSWORD`) are masked in the input form, the workflow details and the run's inputs list. The TUI shows **step output exactly as the step produced it**, so a step that prints a secret will display it.

---

## Troubleshooting

### The TUI will not start

The TUI requires an interactive terminal. It fails with a clear message when `TERM` is unset or `dumb`, or when it is not attached to a terminal. For scripts and CI use `awf run`, `awf history` and `awf status` instead.

### "Terminal too small"

Enlarge the window to at least 80x24.

### The workflow list is empty

The TUI lists workflows from `.awf/workflows/`, your global workflows directory and installed packs. Check the working directory shown in the header, and press `r` to reload.

### A workflow shows "could not be read by AWF"

Its definition failed to load. Run `awf validate <name>` to see the error.

### Colors look wrong

The TUI paints its own background and uses true color when available. Use a terminal with 256-color or true-color support.

---

## See Also

- [Commands Reference](commands.md#awf-tui) - `awf tui` command details
- [Workflow Syntax](workflow-syntax.md) - Define workflows for TUI execution
- [Interactive Inputs](interactive-inputs.md) - Prompting for workflow inputs
- [Execution History](./commands.md#awf-history) - CLI-based history access
