package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

const formWidth = 64

// inputForm collects a workflow's inputs before it runs. It is drawn as a popup over the
// workflows screen. Secret-looking inputs (SECRET_*, API_KEY*, PASSWORD*) are masked while
// typing, matching the masking applied everywhere else values are displayed.
type inputForm struct {
	wf       *workflow.Workflow
	names    []string
	types    []string
	hints    []string
	required []bool
	fields   []textinput.Model
	focus    int
	// problem is the validation message shown under the fields ("" when none).
	problem string
}

func newInputForm(wf *workflow.Workflow) *inputForm {
	n := len(wf.Inputs)
	f := &inputForm{
		wf:       wf,
		names:    make([]string, n),
		types:    make([]string, n),
		hints:    make([]string, n),
		required: make([]bool, n),
		fields:   make([]textinput.Model, n),
	}
	for i := range wf.Inputs {
		in := &wf.Inputs[i]
		ti := textinput.New()
		ti.Placeholder = in.Description
		ti.CharLimit = 512
		ti.SetWidth(formWidth - 12)
		if isSecret(in.Name) {
			ti.EchoMode = textinput.EchoPassword
		}
		if in.Default != nil {
			ti.SetValue(fmt.Sprint(in.Default))
		}
		f.names[i], f.types[i], f.required[i], f.hints[i] = in.Name, in.Type, in.Required, in.Description
		f.fields[i] = ti
	}
	if n > 0 {
		_ = f.fields[0].Focus()
	}
	return f
}

// formResult tells the caller what a key press did to the form.
type formResult int

const (
	formContinue formResult = iota
	formSubmitted
	formCancelled
)

func (f *inputForm) moveFocus(delta int) {
	next := f.focus + delta
	if next < 0 || next >= len(f.fields) {
		return
	}
	f.fields[f.focus].Blur()
	f.focus = next
	_ = f.fields[f.focus].Focus()
}

// update handles a key press and reports whether the form was submitted or cancelled.
func (f *inputForm) update(msg tea.KeyPressMsg) (formResult, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return formCancelled, nil
	case "tab", "down":
		f.moveFocus(1)
		return formContinue, nil
	case "shift+tab", "up":
		f.moveFocus(-1)
		return formContinue, nil
	case "enter":
		if f.focus < len(f.fields)-1 {
			f.moveFocus(1)
			return formContinue, nil
		}
		if missing := f.firstMissing(); missing >= 0 {
			f.problem = fmt.Sprintf(labelFormRequiredFmt, f.names[missing])
			f.fields[f.focus].Blur()
			f.focus = missing
			_ = f.fields[f.focus].Focus()
			return formContinue, nil
		}
		return formSubmitted, nil
	}
	f.problem = ""
	var cmd tea.Cmd
	f.fields[f.focus], cmd = f.fields[f.focus].Update(msg)
	return formContinue, cmd
}

// firstMissing returns the index of the first required field left empty, or -1.
func (f *inputForm) firstMissing() int {
	for i := range f.fields {
		if f.required[i] && strings.TrimSpace(f.fields[i].Value()) == "" {
			return i
		}
	}
	return -1
}

// values returns the non-empty inputs keyed by name.
func (f *inputForm) values() map[string]any {
	out := make(map[string]any, len(f.fields))
	for i := range f.fields {
		if v := strings.TrimSpace(f.fields[i].Value()); v != "" {
			out[f.names[i]] = v
		}
	}
	return out
}

// view draws the popup.
func (f *inputForm) view() string {
	lines := []string{""}
	for i := range f.fields {
		label := sText.Render(f.names[i])
		if f.types[i] != "" {
			label += " " + sDim.Render("("+f.types[i]+")")
		}
		if f.required[i] {
			label += " " + sPink.Render("*")
		}
		marker := "  "
		if i == f.focus {
			marker = sPink.Render("▸ ")
		}
		lines = append(lines, marker+label)
		if f.hints[i] != "" {
			lines = append(lines, "  "+sDim.Render(f.hints[i]))
		}
		lines = append(lines, "  "+f.fields[i].View(), "")
	}
	if f.problem != "" {
		lines = append(lines, sBad.Render("✗ "+f.problem), "")
	}
	lines = append(lines, sKey.Render("enter")+" "+sDim.Render(labelFormNextOrRun)+"  "+
		sKey.Render("tab")+" "+sDim.Render(labelFormField)+"  "+
		sKey.Render("esc")+" "+sDim.Render(labelFormCancel))
	return box(fmt.Sprintf(labelFormTitleFmt, f.wf.Name), lines, formWidth, len(lines)+2, true)
}
