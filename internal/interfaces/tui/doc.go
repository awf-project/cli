// Package tui implements the interactive terminal user interface for AWF, launched with
// `awf tui`.
//
// The TUI is a full-screen Bubble Tea (v2) application. It is a thin interface adapter: it
// renders state and turns key presses into requests, and leaves every decision to the
// application layer. Workflows run in-process through ports.WorkflowFacade, so the TUI
// receives the complete event stream (step lifecycle, live output, conversation turns,
// input requests) without going through the HTTP API.
//
// # Look and feel
//
// The page is a dark violet canvas painted over the whole terminal with a one-cell margin
// (see paintPage). Content is drawn with a small set of primitives in chrome.go, shared by
// every screen:
//
//   - box(title, lines, w, h, focused): a rounded frame with the title on its top border
//   - row(left, right, w): left and right text on one line of exact width
//   - selRow(line, w, bg, bold): a solid full-width highlight for the selected row
//   - keyBar(w, hints...): the rule and key chips that close every screen
//   - overlay(base, popup, w, h): a centered popup drawn over the page
//
// All of them measure and truncate in terminal cells (ANSI aware, wide runes counted as
// two), which is what keeps every row exactly as wide as the terminal. The palette lives
// in theme.go. The minimum supported size is 80x24; below it a short message is shown.
//
// # Screens
//
// The Model (model.go) owns the page chrome and switches between two modes:
//
// Tabs mode shows a header (the AWF badge, the tab bar with item counts and the working
// directory) over one of two screens, reached with tab, shift+tab, 1 and 2:
//
//   - Workflows (screen_workflows.go): the workflow list on the left, with the status icon
//     and age of each workflow's last run, and a detail panel on the right showing source,
//     inputs (secret defaults masked), the ordered steps and the last runs. Keys: up/down,
//     / to filter, x or enter to run, v to validate, r to refresh. A workflow that declares
//     inputs opens a popup form first (form_inputs.go); required inputs are enforced and
//     SECRET_*, API_KEY* and PASSWORD* inputs are masked while typing.
//   - History (screen_history.go): past executions, newest first, with a detail panel for
//     the selected one. Keys: / to filter by workflow name, s to cycle the status filter
//     (all, success, failed), r to refresh, enter to open the step detail.
//
// The step detail (screen_history_detail.go) shows what each step of a past execution did.
// Pressing enter makes the Model read the state AWF persisted for the run through the
// Bridge (run_detail.go): StateReader is satisfied by ports.StateStore and is called with
// ExecutionRecord.WorkflowID, the key the state was saved under, not the history record ID.
// The reply, RunDetailLoadedMsg, echoes the record ID so a late reply for an execution the
// user already left is dropped. Steps are ordered by start time, steps that never started
// come last, and the first failed step is preselected. The output panel scrolls with
// pgup/pgdn, home/end and the mouse wheel; offsets are clamped to the content, which is why
// the screen is told its body size through SetSize. A step with no captured output explains
// why (it did not run, the run stopped while it was running, or it printed nothing) instead
// of showing the live run view's waiting placeholders. Step rendering is shared with the
// run view through pure functions (writeStepResult, writeConversation, stepCopyText).
// A missing state (a nil result from the store) is reported, not treated as an error.
//
// Run mode is a full-frame view of a run, opened as soon as a run starts and left with esc
// (the run keeps going; `o` reopens it). It is documented below.
//
// # The run view
//
// RunScreen is split by responsibility: screen_run.go (state, Update, View, layout),
// screen_run_events.go (event consumption and live output) and screen_run_render.go (pure
// rendering of steps and conversations).
//
// Layout: a progress bar, a Steps box (status icon, number, name and duration per step,
// with the run's inputs listed below, secrets masked) and an Output box for the selected
// step. While the run waits for a reply, a message field appears under the Output box. The
// header shows the workflow, the run identifier and a live status: spinner and elapsed
// clock, or the outcome (done, failed, cancelled).
//
// State is event-driven, never polled. StartEventLoop spawns a goroutine that ranges over
// ports.RunSession.Events() and forwards each event to the update loop as a facadeEventMsg;
// terminal events also emit an ExecutionFinishedMsg. The only timer is clockMsg, which
// animates the spinner and the clock and stops by itself when no run is in flight.
//
// Live step output: the TUI facade enables Adapter.SetStepOutputEvents, so a running step
// streams its stdout and stderr as ports.EventStepOutput chunks. RunScreen keeps them per
// step, capped at maxLiveOutputBytes with the newest bytes winning, and shows them while
// the step runs. Chunks are raw and may end mid-line. When the step completes, the
// authoritative output carried by EventStepCompleted replaces the streamed copy. Chunks are
// best effort: the session drops them under back-pressure, which only costs live fidelity.
//
// The tree cursor follows the step that starts, unless the user moved away from the tail.
// Selecting a step shows its input, its conversation or its output. The first failed step
// is selected automatically so errors surface immediately.
//
// Keys in the run view: up/down pick a step; f follows the tail; pgup/pgdn scroll; c
// cancels the run; r repeats the last launch once the run has ended; y copies the selected
// step's error, stderr and output to the clipboard; esc goes back. ctrl+c cancels a run in
// flight and quits once there is none. While the reply field is focused every key is typed
// text, so none of these actions fire.
//
// Cancellation mirrors the HTTP bridge: a session that implements CancelWithEvent(reason)
// is cancelled that way and publishes its own terminal event; any other session is closed
// directly and the screen finishes immediately.
//
// # Conversations
//
// An agent step in conversation mode emits EventMessageUser and EventMessageAssistant
// events, which RunScreen attributes to the running agent step and renders as a chat
// thread. When the workflow needs the user, the facade publishes EventInputRequired; the
// Model switches to the run view, the reply field takes focus and enter answers through
// RunSession.Respond. TUIInputReader (bridge.go) is the blocking ports.UserInputReader
// handed to the execution services; it signals the program with InputRequestedMsg.
//
// # Architecture
//
//   - command.go is the composition root: cobra command, terminal validation, and
//     buildBridge, which builds configuration, repositories, state store, executor, plugins
//     and notifier wiring, then the facade. It is the only file here that imports
//     infrastructure.
//   - backend.go declares Backend, the single contract between the screens and the
//     application layer. *Bridge (bridge.go) implements it over the facade, and every
//     method returns a tea.Cmd so screens never block.
//   - run_detail.go holds the History detail view model and the StateReader port.
//   - messages.go lists the messages exchanged between the Bridge and the Model.
//   - model.go routes messages, handles global keys and draws the chrome.
//   - labels.go holds every user-visible string. The interface is English-only, and
//     TestLabels_AreEnglish parses the file to enforce it.
//
// Bridgeless mode: with a nil Bridge the Model still starts and each screen renders its
// empty state. Preview modes must stay free of optional infrastructure.
//
// # Testing
//
// Screens are plain values with Update and view methods, so tests drive them with key
// presses and assert on ANSI-stripped output. TestModel_RenderFillsTheTerminalExactly
// checks that every screen fills the terminal to the exact cell at several sizes. The
// integration tests in tests/integration/tui run the real binary in a pseudo-terminal.
//
// The facade conformance golden (tests/fixtures/facade/tui-tea-msg.golden) is produced by
// RenderEventsToTUIMsgs and pins the projection of facade events to TUI messages.
package tui
