package tui

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

//nolint:gocritic // viewport.Model passed by value per Bubbletea convention; returned copy replaces the caller's field
func viewportAutoScroll(vp viewport.Model, autoScroll *bool, msg tea.Msg) (viewport.Model, tea.Cmd) {
	wasAtBottom := vp.AtBottom()
	vp, cmd := vp.Update(msg)
	if wasAtBottom && !vp.AtBottom() {
		*autoScroll = false
	}
	return vp, cmd
}
