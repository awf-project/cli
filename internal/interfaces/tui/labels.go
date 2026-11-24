package tui

// User-facing labels for the TUI.
//
// All strings rendered to the user live here so that the interface language stays
// English and can be audited in one place: TestLabels_AreEnglish parses this file and
// checks every string constant, so a new label is covered as soon as it is added.
// Do not inline new user-visible strings in screen files; add a constant here.

// Tabs. The format takes the shortcut digit and the item count.
const (
	labelTabWorkflowsFmt = "%d Workflows (%d)"
	labelTabHistoryFmt   = "%d History (%d)"
)

// Shared key bar descriptions.
const (
	labelKeyApply            = "apply"
	labelKeyClear            = "clear"
	labelKeyTabs             = "tabs"
	labelKeyNavigate         = "navigate"
	labelKeyRun              = "run"
	labelKeyValidate         = "validate"
	labelKeyFilter           = "filter"
	labelKeyRefresh          = "refresh"
	labelKeyQuit             = "quit"
	labelKeyStatusFmt        = "status: %s"
	labelKeyReply            = "reply"
	labelKeyBackRunContinues = "back (run continues)"
	labelKeyPickStep         = "pick a step"
	labelKeyFollow           = "follow"
	labelKeyScroll           = "scroll"
	labelKeyCancelRun        = "cancel run"
	labelKeyRerun            = "run again"
	labelKeyCopy             = "copy"
)

// Page chrome.
const (
	labelHeaderTitle      = "Workflow Manager"
	labelTerminalTooFmt   = "Terminal too small (%dx%d) — minimum %dx%d"
	labelValidationOKFmt  = "✓ %s is valid"
	labelValidationBadFmt = "✗ %s: %s"
	labelNoBackend        = "no backend configured"
	labelStartingFmt      = "starting %s…"
)

// Workflows screen.
const (
	labelWorkflowsTitle      = "Workflows"
	labelWorkflowsFilterHint = "filter…"
	labelWorkflowsNone       = "no workflows"
	labelWorkflowsCountFmt   = "%d workflows"
	labelWorkflowsSelect     = "Select a workflow."
	labelWorkflowsUnreadable = "✗ workflow could not be read by AWF"
)

// History screen.
const (
	labelHistoryTitle      = "History"
	labelHistoryFilterHint = "filter by workflow…"
	labelHistoryNone       = "no executions"
	labelHistorySelect     = "Select an execution."
	labelHistorySummaryFmt = "%d runs · %d ok · %d failed"
	labelHistoryCountFmt   = "%d runs"
)

// Detail panels (shared by Workflows and History).
const (
	labelDetailsTitle      = "Details"
	labelDetailsScope      = "Source"
	labelDetailsInputs     = "Inputs"
	labelDetailsSteps      = "Steps"
	labelDetailsStepsTitle = "Steps"
	labelDetailsLastRuns   = "Last runs"
	labelDetailsNoRuns     = "no runs yet"
	labelDetailsNone       = "none"
	labelDetailsRunID      = "Run ID"
	labelDetailsStarted    = "Started"
	labelDetailsDuration   = "Duration"
	labelDetailsExitCode   = "Exit code"
	labelDetailsAttempt    = "Attempt"
	labelDetailsResumed    = "Resumed from"
	labelDetailsError      = "Error"
)

// History run detail.
const (
	labelDetailNoStore     = "run detail is not available without a state store"
	labelDetailGone        = "the saved state of this run is no longer available"
	labelDetailLoading     = "loading…"
	labelDetailNoSteps     = "no steps"
	labelDetailNoStepsHint = "no step data was saved for this run"
	labelDetailDidNotRun   = "(this step did not run)"
	labelDetailInterrupted = "(the run stopped while this step was still running)"
	labelDetailNoOutput    = "(no output)"
	labelDetailTruncated   = "(output was truncated when it was saved)"
	labelKeyOpen           = "step detail"
	labelKeyBack           = "back"
	labelKeyReload         = "reload"
)

// Input form popup.
const (
	labelFormTitleFmt    = "Run %s"
	labelFormRequiredFmt = "%s is required"
	labelFormNextOrRun   = "next / run"
	labelFormField       = "field"
	labelFormCancel      = "cancel"
)

// Run screen.
const (
	labelRunInputPlaceholder  = "Type your message (empty to end conversation)..."
	labelRunEmptyTitle        = "No active execution"
	labelRunEmptySubtitle     = "Launch a workflow from the Workflows tab."
	labelRunWaitingDimensions = "Waiting for terminal dimensions…"
	labelRunOutputTitle       = "Output"
	labelRunStepsTitle        = "Steps"
	labelRunInputsTitle       = "Inputs"
	labelRunReplyTitle        = "Your message"
	labelRunLiveSuffix        = "(live)"
	labelRunStepPending       = "pending"
	labelRunStepSkipped       = "skipped"
	labelRunFailure           = "✗ Workflow failed:"
	labelRunProgressFmt       = "%d/%d steps"
	labelRunIDFmt             = "run %s"
	labelRunWaitingReplyFmt   = "⏸ waiting for your reply · %s"
	labelRunCancellingFmt     = "⊘ cancelling · %s"
	labelRunRunningFmt        = "%s running · %s"
	labelRunDoneFmt           = "✓ done · %s"
	labelRunCancelledFmt      = "⊘ cancelled · %s"
	labelRunFailedFmt         = "✗ failed · %s"
	labelRunCancelling        = "cancelling…"
	labelRunNothingToCopy     = "nothing to copy"
	labelRunCopiedFmt         = "copied %d bytes"
	labelRunStarting          = "Starting…"
	labelRunWaitingStart      = "Waiting for execution to start…"
	labelRunMoreLinesFmt      = "… (%d more lines)"
	labelRunEarlierLinesFmt   = "… (%d earlier lines)"
	labelRunInput             = "▶ Input:"
	labelRunUser              = "▶ User:"
	labelRunAgent             = "◀ Agent:"
	labelRunAgentThinking     = "Agent is thinking..."
	labelRunError             = "✗ Error:"
	labelRunStderr            = "⚠ Stderr:"
	labelRunStderrLive        = "⚠ Stderr (live):"
	labelRunOutput            = "◀ Output:"
	labelRunOutputLive        = "◀ Output (live):"
	labelRunLiveTruncated     = "(older live output was dropped; the full output appears when the step ends)"
	labelRunWaiting           = "⏳ Waiting…"
	labelRunRunning           = "⟳ Running…"
	labelRunPending           = "⏳ Pending"
	labelRunStepNotStartedFmt = "Step %q has not started yet."
)
