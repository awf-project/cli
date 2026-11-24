package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// press builds a key press from a name as Bubble Tea prints it ("tab", "shift+tab",
// "ctrl+c", "esc", "enter", "up", "down") or a single typed character.
func press(name string) tea.KeyPressMsg {
	switch name {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(name)
	return tea.KeyPressMsg{Code: r[0], Text: name}
}

// typeText sends each rune of s as a key press to the model.
func typeText(m Model, s string) Model {
	for _, r := range s {
		next, _ := m.Update(press(string(r)))
		m = next.(Model)
	}
	return m
}

func send(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

// sampleEntries/sampleWorkflow give the screens something realistic to show.
func sampleEntries() []workflow.WorkflowEntry {
	return []workflow.WorkflowEntry{
		{Name: "deploy", Source: "local", Version: "1.2.0", Description: "Build and deploy the app"},
		{Name: "lint", Source: "local"},
		{Name: "my-pack/release", Source: "pack", Version: "0.3.1"},
	}
}

func sampleWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:        "deploy",
		Description: "Build and deploy the app",
		Initial:     "build",
		Inputs: []workflow.Input{
			{Name: "env", Type: "string", Required: true, Default: "staging"},
			{Name: "API_KEY", Type: "string"},
		},
		Steps: map[string]*workflow.Step{
			"build": {Name: "build", Type: workflow.StepTypeCommand, Command: "make build", OnSuccess: "test"},
			"test":  {Name: "test", Type: workflow.StepTypeCommand, Command: "make test", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
}

func sampleRecords(now time.Time) []*workflow.ExecutionRecord {
	return []*workflow.ExecutionRecord{
		{ID: "run-aaaaaaaa11", WorkflowID: "wf-state-aaa", WorkflowName: "deploy", Status: "success", StartedAt: now.Add(-3 * time.Hour), DurationMs: 12400},
		{ID: "run-bbbbbbbb22", WorkflowName: "deploy", Status: "failed", StartedAt: now.Add(-26 * time.Hour), DurationMs: 3100, ExitCode: 2, ErrorMessage: "step test failed: exit status 2 while running make test"},
		{ID: "run-cccccccc33", WorkflowName: "lint", Status: "cancelled", StartedAt: now.Add(-5 * time.Minute), DurationMs: 900},
	}
}

// loadedModel returns a sized model with workflows and history loaded.
func loadedModel(t *testing.T, w, h int) Model {
	t.Helper()
	m := New()
	m.dir = "~/project"
	m, _ = send(m, tea.WindowSizeMsg{Width: w, Height: h})
	m, _ = send(m, WorkflowsLoadedMsg{Entries: sampleEntries(), Workflows: []*workflow.Workflow{sampleWorkflow()}})
	m, _ = send(m, HistoryLoadedMsg{
		Records: sampleRecords(time.Now()),
		Stats:   &workflow.HistoryStats{TotalExecutions: 3, SuccessCount: 1, FailedCount: 1},
	})
	return m
}

// plain returns the visible text of the model's view.
func plain(m Model) string { return ansi.Strip(m.View().Content) }

func startedRun(t *testing.T, m Model, sess ports.RunSession) Model {
	t.Helper()
	// The event loop goroutine stops with the context, so tests do not leak it.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	m.ctx = ctx
	m, _ = send(m, ExecutionStartedMsg{ExecutionID: "deploy", Workflow: sampleWorkflow(), Session: sess})
	return m
}

func lines(s string) []string { return strings.Split(s, "\n") }

// fakeStates is a StateReader returning a canned execution (or error).
type fakeStates struct {
	ec  *workflow.ExecutionContext
	err error
	// loadedIDs records the identifiers requested.
	loadedIDs []string
}

func (f *fakeStates) Load(_ context.Context, id string) (*workflow.ExecutionContext, error) {
	f.loadedIDs = append(f.loadedIDs, id)
	return f.ec, f.err
}

// sampleExecution is a finished run: build succeeded, test failed, done never ran.
func sampleExecution(now time.Time) *workflow.ExecutionContext {
	ec := workflow.NewExecutionContext("wf-1", "deploy")
	ec.Inputs = map[string]any{"env": "prod", "API_KEY": "s3cr3t"}
	ec.SetStepState("done", workflow.StepState{Name: "done", Status: workflow.StatusPending})
	ec.SetStepState("test", workflow.StepState{
		Name: "test", Status: workflow.StatusFailed, Error: "exit status 2", Stderr: "FAIL: TestX",
		Output:    strings.Repeat("test output line\n", 40),
		StartedAt: now.Add(-15 * time.Second), CompletedAt: now.Add(-12 * time.Second),
	})
	ec.SetStepState("build", workflow.StepState{
		Name: "build", Status: workflow.StatusCompleted, Output: "compiled ok",
		StartedAt: now.Add(-20 * time.Second), CompletedAt: now.Add(-15 * time.Second),
	})
	return ec
}

// historyModel returns a loaded model on the History tab, wired to the given reader, with
// the success record of "deploy" selected.
func historyModel(t *testing.T, states StateReader) Model {
	t.Helper()
	m := loadedModel(t, 110, 30)
	b := NewBridge(nil, nil)
	b.SetStateReader(states)
	m.bridge = b
	m, _ = send(m, press("2"))
	m, _ = send(m, press("down")) // newest first: lint, deploy (success), deploy (failed)
	return m
}

// settle runs a command and feeds each resulting message back into the model until no
// command is left, the way the Bubble Tea runtime would. Batches are not expanded.
func settle(m Model, cmd tea.Cmd) Model {
	for cmd != nil {
		msg := cmd()
		if _, ok := msg.(tea.BatchMsg); ok || msg == nil {
			return m
		}
		m, cmd = send(m, msg)
	}
	return m
}

// openDetail presses enter on the selected record and settles the load.
func openDetail(t *testing.T, m Model) Model {
	t.Helper()
	m, cmd := send(m, press("enter"))
	return settle(m, cmd)
}

// fakeSources is a SourceReader returning canned YAML (or an error).
type fakeSources struct {
	data []byte
	path string
	err  error
	// requested records the names asked for.
	requested []string
}

func (f *fakeSources) WorkflowSource(_ context.Context, name string) ([]byte, string, error) {
	f.requested = append(f.requested, name)
	return f.data, f.path, f.err
}

// sampleYAML is a small workflow file with a comment, inputs and three states.
const sampleYAML = `# deploys the app
name: deploy
version: "1.2.0"
inputs:
  env:
    type: string
    required: true

states:
  initial: build

  build:
    type: step
    command: make build   # compile
    on_success: test

  test:
    type: step
    command: make test
    on_success: done

  done:
    type: terminal
    status: success
`

// configModel returns a model on the Workflows tab wired to the given source reader, with
// "deploy" selected.
func configModel(t *testing.T, src SourceReader) Model {
	t.Helper()
	m := loadedModel(t, 110, 30)
	b := NewBridge(nil, nil)
	b.SetSourceReader(src)
	m.bridge = b
	return m
}

// openConfig presses enter on the selected workflow and settles the load.
func openConfig(t *testing.T, m Model) Model {
	t.Helper()
	m, cmd := send(m, press("enter"))
	return settle(m, cmd)
}

// stripANSI removes terminal escape sequences.
func stripANSI(s string) string { return ansi.Strip(s) }

// flatText collapses a rendered screen into its words: box borders removed and every run of
// whitespace (including line breaks from wrapping) turned into one space.
func flatText(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(ansi.Strip(s), "│", " ")), " ")
}
