// Package tui provides a terminal user interface for AWF workflows.
//
// The TUI layer is an interface adapter in the hexagonal architecture, sitting
// at the same level as the CLI package. It translates Bubbletea v2 events and
// user keystrokes into application service calls and renders responses as
// styled terminal output via Lipgloss v2 and Glamour v2. No domain logic or
// infrastructure details reside here—only presentation, routing, and
// coordination between the domain model and the view.
//
// # Architecture Role
//
// In the hexagonal architecture:
//   - The TUI Model receives user input from Bubbletea key/mouse events
//   - The Bridge adapter translates between service interfaces and tea.Cmd factories
//   - Tab sub-models encapsulate per-tab state and rendering
//   - The Model delegates business operations to application service ports
//     (WorkflowLister, WorkflowRunner, HistoryProvider)
//
// The TUI depends on application service ports defined in bridge.go; it never
// imports infrastructure packages directly. Service implementations are injected
// via NewCommand and buildBridge at CLI startup.
//
// # Bubbletea Model-Update-View Pattern
//
// The package follows the Elm architecture as implemented by Bubbletea:
//
//   - Init() returns an initial tea.Cmd (typically LoadWorkflows via Bridge)
//   - Update(msg) dispatches incoming messages to the appropriate handler,
//     returns a new Model and an optional tea.Cmd for side effects
//   - View() renders the full screen string from current model state
//
// All sub-models (tab types) follow the same convention: value receivers that
// return (TabType, tea.Cmd). The root Model delegates unhandled messages to
// the active tab's Update method.
//
// # Tab Architecture
//
// The Model hosts five tabs, each implemented as a standalone sub-model:
//
// ## Tab 1: Workflows (tab_workflows.go)
//
// Filterable, keyboard-navigable list of available workflow definitions.
// Backed by a bubbles list.Model with a DefaultDelegate for two-line rows
// (workflow name + description and step count). Supports:
//   - Enter: launch selected workflow (emits LaunchWorkflowMsg)
//   - 'v': validate selected workflow via Bridge.ValidateWorkflow
//   - Esc: dismiss validation overlay or clear filter
//   - /: enter filter mode (delegated to list.Model)
//
// ## Tab 2: Monitoring (tab_monitoring.go)
//
// Live execution dashboard with a two-panel layout:
//   - Left panel (40%): execution tree with step status icons (see tree.go)
//   - Right panel (60%): scrollable viewport showing selected step output
//
// A 200 ms tick loop polls ExecutionContext.GetAllStepStates() during active
// runs. Navigation keys (up/k, down/j) move the tree selection; 'f' jumps to
// the bottom and re-enables auto-scroll. The tab auto-selects the first failed
// node to surface errors immediately.
//
// ## Tab 3: History (tab_history.go)
//
// Filterable execution history list with a detail view:
//   - Text filter: substring match on workflow name (case-insensitive)
//   - Tab key: cycles status filter (all → success → failed → all)
//   - Enter: opens detail view for selected record
//   - Esc: returns to list view from detail
//
// Statistics footer shows total, success, failed, cancelled counts and average
// duration in milliseconds.
//
// ## Tab 4: Agent Conversations (tab_agent.go)
//
// Read-only viewport displaying agent conversation turns from the most recent
// execution. Content is populated via agent display events forwarded from the
// execution pipeline. Supports page-up/page-down and mouse wheel scrolling.
//
// ## Tab 5: External Logs (tab_logs.go)
//
// Tailed view of the AWF audit JSONL log file.
// An offset-based Tailer reads one line per poll interval from the AWF
// audit.jsonl file and emits LogLineMsg. Each entry shows the event type,
// workflow name, status, duration, and any error message.
// File rotation is detected automatically and surfaces a notice; the tab
// re-watches the path on each tick until the file reappears.
// Supports up to 1000 log entries (maxLogEntries) with auto-scroll; 'f' jumps
// to the bottom and re-enables auto-scroll after manual upward scroll.
//
// # Bridge Adapter Pattern
//
// Bridge (bridge.go) is the anti-corruption layer between the Bubbletea event
// loop and application service interfaces. It exposes factory methods that
// return tea.Cmd closures:
//
//   - Bridge.LoadWorkflows: fetches all workflow definitions → WorkflowsLoadedMsg
//   - Bridge.LoadHistory: fetches execution records and stats → HistoryLoadedMsg
//   - Bridge.RunWorkflow: starts a workflow execution → ExecutionStartedMsg
//   - Bridge.ValidateWorkflow: validates a workflow by name → ValidationResultMsg or ErrMsg
//
// Bridge fields may be nil independently; a nil dependency returns ErrMsg with
// a descriptive error. This enables partial service wiring without panics.
//
// When CommandConfig is nil or all fields are nil (bridgeless mode), the Model
// initializes without a Bridge. Init() returns a no-op WorkflowsLoadedMsg so
// the update loop starts cleanly. Bridgeless mode is suitable for read-only
// display of local state without live service calls.
//
// # Message Flow
//
// Messages flow through the Update loop in this order:
//
//  1. Global key messages (1–5 for tab switching, q/ctrl+c to quit) are handled
//     by the root Model before delegation.
//  2. tea.WindowSizeMsg is propagated to all five tab sub-models simultaneously
//     so each can reflow its layout.
//  3. Domain messages (WorkflowsLoadedMsg, ExecutionStartedMsg, etc.) are
//     handled by the root Model and update the relevant tab's state fields.
//  4. Unhandled messages are forwarded to the active tab via updateActiveTab.
//  5. Tab sub-models return (NewTab, tea.Cmd); the root Model embeds the updated
//     tab and schedules any returned command for the next event loop iteration.
//
// # Tree Rendering System
//
// tree.go provides BuildTree and RenderTree for visualizing workflow execution:
//
//   - BuildTree(steps, states): constructs a []*TreeNode hierarchy from ordered
//     workflow steps and their current StepState snapshots
//   - RenderTree(roots): produces a UTF-8 tree string with box-drawing characters
//     and status icons (✓ completed, ✗ failed, ⟳ running, ○ pending)
//   - TreeNode carries Name, Status, Depth, Output, and Children for recursive
//     rendering and keyboard navigation in MonitoringTab
//
// # JSONL Tailer System
//
// tailer.go provides offset-based JSONL file tailing:
//
//   - NewTailer(path): creates a Tailer at the given path with offset=0
//   - Tailer.Next(): returns a tea.Cmd that reads exactly one new line per call;
//     returns nil tea.Msg at EOF, logRotationMsg on file-not-found/OS errors
//   - parseLine: maps AWF audit JSONL keys (event, timestamp, workflow_name,
//     execution_id, status, duration_ms, user, error) to LogEntry fields
//
// The 1 MB scanner buffer (1024*1024 bytes) accommodates large single-line JSON
// objects from verbose agent outputs without truncation.
//
// # Key Bindings (keys.go)
//
// All key bindings are declared as package-level key.Binding variables in
// keys.go using bubbles/key. Update methods use key.Matches(msg, keyXxx)
// instead of raw msg.String() switch statements. This enables:
//   - Semantic key binding names (keyFollow, keyValidate) over string literals
//   - Composable multi-key bindings (keyUp matches both "up" and "k")
//   - Integration with bubbles/help for contextual help display
//
// Global (handled by root Model):
//   - 1–5: switch to the corresponding tab (keyTab1–keyTab5)
//   - q: quit (keyQuit), ctrl+c: force quit (keyForceQuit)
//   - ?: toggle help bar (keyHelp)
//
// Workflows tab:
//   - enter: launch selected workflow (keySelect)
//   - v: validate selected workflow (keyValidate)
//   - /: enter filter mode (keyFilter)
//   - esc: dismiss overlay or clear filter (keyBack)
//
// Monitoring tab:
//   - ↑/k, ↓/j: navigate execution tree (keyUp, keyDown)
//   - f: jump to bottom and re-enable auto-scroll (keyFollow)
//   - PageUp/PageDown: scroll log viewport (delegated to viewport)
//
// History tab (using bubbles/table for cursor and scroll management):
//   - ↑/↓: navigate record table (delegated to table.Model)
//   - tab: cycle status filter (keyCycleFilter)
//   - enter: open detail view (keySelect)
//   - esc: close detail view (keyBack)
//   - /: focus text filter (keyFilter)
//
// Agent/Logs tabs:
//   - f: jump to bottom and re-enable auto-scroll (keyFollow)
//   - PageUp/PageDown, ↑/↓: scroll viewport (delegated to viewport)
//
// # Help System (bubbles/help)
//
// The root Model holds a help.Model that renders a contextual help bar at the
// bottom of the screen. Each tab provides its own help.KeyMap implementation
// (workflowsHelpKeys, monitoringHelpKeys, etc.) returned by activeHelpKeys().
// Pressing "?" toggles between short and full help display.
//
// # Spinner Integration (bubbles/spinner)
//
// MonitoringTab and WorkflowsTab embed a spinner.Model for async operation
// feedback. The monitoring spinner is visible while waiting for an execution
// context (showSpinner=true between ExecutionStartedMsg and first
// executionPollMsg). The workflows spinner is visible during validation
// (validating=true between handleValidate and ValidationResultMsg).
//
// # Auto-Scroll Helper (viewportscroll.go)
//
// viewportAutoScroll(vp, &autoScroll, msg) wraps vp.Update(msg) and clears
// the autoScroll flag when the user scrolls away from the bottom. Used by
// MonitoringTab, AgentTab, and LogsTab to eliminate duplicated scroll-tracking
// logic. The follow key (keyFollow) re-enables auto-scroll and jumps to bottom.
//
// # Design Principles
//
//   - Thin adapter layer: all business logic in application/domain layers
//   - Bridgeless mode: TUI is functional without any service wiring
//   - Value receiver convention: all Update/View methods use value receivers
//     following Bubbletea's immutable model pattern
//   - Zero infrastructure imports: Bridge wraps service interfaces; no direct
//     repository, executor, or logger imports in this package
//   - Native Bubbles components: use key.Binding, help.Model, table.Model,
//     spinner.Model, list.Model, viewport.Model, and textinput.Model from
//     the Bubbles ecosystem rather than reimplementing their functionality
//   - Graceful degradation: nil bridge fields, missing files, and malformed
//     JSON all produce informative messages rather than crashes
//   - Context propagation: all Bridge methods accept context.Context for
//     cancellation and deadline support
package tui
