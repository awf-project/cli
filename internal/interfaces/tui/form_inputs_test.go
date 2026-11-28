package tui

import (
	"testing"

	"charm.land/bubbles/v2/textinput"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func formWorkflow() *workflow.Workflow {
	return &workflow.Workflow{Name: "deploy", Inputs: []workflow.Input{
		{Name: "env", Type: "string", Required: true, Description: "target environment"},
		{Name: "count", Type: "integer", Default: 3},
		{Name: "API_KEY", Type: "string"},
	}}
}

func typeInto(f *inputForm, s string) {
	for _, r := range s {
		f.update(press(string(r)))
	}
}

func TestInputForm_DefaultsAndFocus(t *testing.T) {
	f := newInputForm(formWorkflow())

	assert.Equal(t, 0, f.focus)
	assert.True(t, f.fields[0].Focused(), "the first field starts focused")
	assert.Equal(t, "3", f.fields[1].Value(), "defaults are prefilled")
	assert.Empty(t, f.fields[0].Value())
}

func TestInputForm_SecretFieldsAreMaskedWhileTyping(t *testing.T) {
	f := newInputForm(formWorkflow())

	assert.Equal(t, textinput.EchoNormal, f.fields[0].EchoMode)
	assert.Equal(t, textinput.EchoPassword, f.fields[2].EchoMode, "API_KEY must not echo")

	f.focus = 2
	_ = f.fields[2].Focus()
	typeInto(f, "hunter2")

	assert.NotContains(t, ansi.Strip(f.view()), "hunter2", "a secret must never be drawn")
	assert.Equal(t, "hunter2", f.values()["API_KEY"], "but it is still submitted")
}

func TestInputForm_Navigation(t *testing.T) {
	f := newInputForm(formWorkflow())

	f.update(press("tab"))
	assert.Equal(t, 1, f.focus)
	f.update(press("down"))
	assert.Equal(t, 2, f.focus)
	f.update(press("down"))
	assert.Equal(t, 2, f.focus, "focus stops at the last field")
	f.update(press("shift+tab"))
	assert.Equal(t, 1, f.focus)
	f.update(press("up"))
	f.update(press("up"))
	assert.Equal(t, 0, f.focus, "focus stops at the first field")
	assert.True(t, f.fields[0].Focused())
	assert.False(t, f.fields[1].Focused())
}

func TestInputForm_EnterAdvancesThenSubmits(t *testing.T) {
	f := newInputForm(formWorkflow())
	typeInto(f, "prod")

	res, _ := f.update(press("enter"))
	assert.Equal(t, formContinue, res)
	assert.Equal(t, 1, f.focus)
	res, _ = f.update(press("enter"))
	assert.Equal(t, formContinue, res)
	res, _ = f.update(press("enter"))
	assert.Equal(t, formSubmitted, res, "enter on the last field submits")
}

func TestInputForm_RequiredFieldBlocksSubmitAndRefocuses(t *testing.T) {
	f := newInputForm(formWorkflow())
	f.focus = 2 // jump to the last field with the required one left empty
	f.fields[0].Blur()
	_ = f.fields[2].Focus()

	res, _ := f.update(press("enter"))

	assert.Equal(t, formContinue, res, "a missing required input must not submit")
	assert.Equal(t, 0, f.focus, "focus returns to the missing field")
	assert.Equal(t, "env is required", f.problem)
	assert.Contains(t, ansi.Strip(f.view()), "env is required")

	typeInto(f, "x")
	assert.Empty(t, f.problem, "typing clears the message")
}

func TestInputForm_WhitespaceOnlyCountsAsMissing(t *testing.T) {
	f := newInputForm(formWorkflow())
	typeInto(f, "   ")
	assert.Equal(t, 0, f.firstMissing())
}

func TestInputForm_ValuesAreTrimmedAndEmptyOnesOmitted(t *testing.T) {
	f := newInputForm(formWorkflow())
	typeInto(f, "  prod  ")

	assert.Equal(t, map[string]any{"env": "prod", "count": "3"}, f.values())
}

func TestInputForm_EscCancels(t *testing.T) {
	f := newInputForm(formWorkflow())
	res, _ := f.update(press("esc"))
	assert.Equal(t, formCancelled, res)
}

func TestInputForm_ViewShowsLabelsTypesAndRequiredMarker(t *testing.T) {
	out := ansi.Strip(newInputForm(formWorkflow()).view())

	assert.Contains(t, out, "Run deploy")
	assert.Contains(t, out, "env (string) *")
	assert.Contains(t, out, "target environment")
	assert.Contains(t, out, "count (integer)")
	assert.NotContains(t, out, "count (integer) *", "optional inputs carry no marker")
	require.Equal(t, formWidth, len([]rune(lines(out)[0])))
}

func TestInputForm_WorkflowWithoutInputsDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		f := newInputForm(&workflow.Workflow{Name: "empty"})
		_ = f.view()
		f.update(press("enter"))
	})
}
