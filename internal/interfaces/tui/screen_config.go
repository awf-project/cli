package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// ConfigScreen shows the project configuration, read-only: the file, whether it exists and
// the workflow inputs it pre-fills. It mirrors `awf config show`.
type ConfigScreen struct {
	info   ConfigInfo
	loaded bool
	// err is why the configuration could not be read ("" when it was).
	err    string
	offset int
	// w/h are the size of the screen body, set by the Model; they bound the scrolling.
	w, h int
}

func newConfigScreen() ConfigScreen { return ConfigScreen{} }

// SetSize tells the screen the size of its body.
func (s *ConfigScreen) SetSize(w, h int) {
	s.w, s.h = w, h
	s.clampOffset()
}

func (s *ConfigScreen) rows() int { return max(1, s.h-2) }

func (s *ConfigScreen) clampOffset() {
	s.offset = min(max(s.offset, 0), max(0, len(s.lines())-s.rows()))
}

// lines renders the configuration as the lines of the box.
func (s *ConfigScreen) lines() []string {
	kv := func(k, v string) string { return sDim.Render(fit(k, 10)) + v }
	switch {
	case !s.loaded && s.err == "":
		return []string{sDim.Render(labelConfigLoading)}
	case s.err != "":
		return styleAll(wrapText(s.err, max(s.w-4, 1)), sBad.Render)
	}

	status := sOK.Render(labelConfigScreenFound)
	if !s.info.Exists {
		status = sDim.Render(labelConfigScreenMissing)
	}
	out := []string{kv(labelConfigScreenFile, s.info.Path), kv(labelConfigScreenStatus, status), ""}

	out = append(out, sPurple.Bold(true).Render(labelDetailsInputs))
	if len(s.info.Inputs) == 0 {
		out = append(out, sDim.Render(labelConfigScreenNoInputs))
	}
	for _, k := range sortedKeys(s.info.Inputs) {
		out = append(out, sPurple.Render(k)+sDim.Render(" = ")+maskValue(k, fmt.Sprint(s.info.Inputs[k])))
	}
	return append(out, "", sDim.Render(labelConfigScreenHint))
}

// Update handles the configuration reply, keys and the mouse wheel.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s ConfigScreen) Update(msg tea.Msg) (ConfigScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case ConfigLoadedMsg:
		s.info, s.err, s.loaded = msg.Info, msg.Err, msg.Err == ""
		s.clampOffset()
		return s, nil
	case tea.MouseWheelMsg:
		switch tea.Mouse(msg).Button {
		case tea.MouseWheelUp:
			s.offset -= wheelStep
		case tea.MouseWheelDown:
			s.offset += wheelStep
		}
		s.clampOffset()
		return s, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			s.offset--
		case "down", "j":
			s.offset++
		case "pgup":
			s.offset -= max(1, s.rows()-1)
		case "pgdown", " ":
			s.offset += max(1, s.rows()-1)
		case "home", "g":
			s.offset = 0
		case "end", "G":
			s.offset = len(s.lines())
		case "r":
			return s, func() tea.Msg { return refreshMsg{} }
		}
		s.clampOffset()
	}
	return s, nil
}

func (s *ConfigScreen) keys(w int) string {
	return keyBar(w,
		keyHint{"tab", labelKeyTabs}, keyHint{"↑↓", labelKeyScroll},
		keyHint{"r", labelKeyRefresh}, keyHint{"q", labelKeyQuit})
}

func (s *ConfigScreen) view(w, h int) string {
	all := s.lines()
	rows := max(1, h-2)
	visible, start, end := scrollWindow(all, s.offset, rows)
	return box(scrollTitle(labelDetailsTitleConfig, start, end, len(all), rows), visible, w, h, true)
}
