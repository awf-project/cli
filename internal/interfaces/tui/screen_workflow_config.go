package tui

import (
	"fmt"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// workflowConfig is the open configuration view of one workflow: its steps on the left and
// the YAML of the selected one on the right. It is opened with enter on a workflow.
type workflowConfig struct {
	entry   workflow.WorkflowEntry
	loading bool
	// err is why the source could not be read ("" when it was).
	err   string
	path  string
	items []ConfigItem
	sel   int
	// offset is the first visible line of the YAML panel.
	offset int
}

func newWorkflowConfig(e workflow.WorkflowEntry) *workflowConfig {
	return &workflowConfig{entry: e, loading: true}
}

// loaded applies a load result.
func (c *workflowConfig) loaded(msg WorkflowSourceLoadedMsg) {
	c.loading = false
	c.err = msg.Err
	c.path = msg.Path
	c.items = msg.Items
	c.sel, c.offset = 0, 0
}

func (c *workflowConfig) selected() (*ConfigItem, bool) {
	if c.sel < 0 || c.sel >= len(c.items) {
		return nil, false
	}
	return &c.items[c.sel], true
}

func (c *workflowConfig) moveSelection(delta int) {
	next := c.sel + delta
	if next < 0 || next >= len(c.items) {
		return
	}
	c.sel, c.offset = next, 0
}

// styledLines returns the selected block as display lines.
func (c *workflowConfig) styledLines() []string {
	item, ok := c.selected()
	if !ok {
		return nil
	}
	out := make([]string, len(item.Lines))
	for i, l := range item.Lines {
		out[i] = styleYAMLLine(l)
	}
	return out
}

func (c *workflowConfig) clampOffset(rows int) {
	c.offset = min(max(c.offset, 0), max(0, len(c.styledLines())-max(rows, 1)))
}

// copyText is the selected block as plain YAML.
func (c *workflowConfig) copyText() string {
	item, ok := c.selected()
	if !ok {
		return ""
	}
	return strings.Join(item.Lines, "\n")
}

func (c *workflowConfig) loadRequest() tea.Cmd {
	name := c.entry.Name
	return func() tea.Msg { return loadWorkflowSourceMsg{Name: name} }
}

// yamlKeyRe matches the indentation, optional list dash and key of a YAML line.
var yamlKeyRe = regexp.MustCompile(`^(\s*(?:- )?)([^\s#:][^:#]*?)(:)(\s.*|)$`)

// styleYAMLLine colors keys and dims comments. It only decorates: the text is unchanged.
func styleYAMLLine(l string) string {
	trimmed := strings.TrimSpace(l)
	if strings.HasPrefix(trimmed, "#") {
		return sDim.Render(l)
	}
	if m := yamlKeyRe.FindStringSubmatch(l); m != nil {
		return m[1] + sPurple.Render(m[2]) + sDim.Render(m[3]) + m[4]
	}
	return l
}

// updateConfig handles keys while the config view is open.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s WorkflowsScreen) updateConfig(msg tea.KeyPressMsg) (WorkflowsScreen, tea.Cmd) {
	c := s.cfg
	rows := outputRows(s.h)
	switch msg.String() {
	case "esc":
		s.cfg = nil
		return s, nil
	case "x":
		return s.launchEntry(c.entry)
	case "up", "k":
		c.moveSelection(-1)
	case "down", "j":
		c.moveSelection(1)
	case "pgup":
		c.offset -= max(1, rows-1)
	case "pgdown", " ":
		c.offset += max(1, rows-1)
	case "home", "g":
		c.offset = 0
	case "end", "G":
		c.offset = len(c.styledLines())
	case "R":
		c.loading, c.err = true, ""
		return s, c.loadRequest()
	case "y":
		return s, copyCmd(c.copyText())
	}
	c.clampOffset(rows)
	return s, nil
}

// scrollConfig scrolls the YAML panel by delta lines (mouse wheel).
func (s *WorkflowsScreen) scrollConfig(delta int) {
	if s.cfg == nil {
		return
	}
	s.cfg.offset += delta
	s.cfg.clampOffset(outputRows(s.h))
}

func (s *WorkflowsScreen) configKeys(w int) string {
	return keyBar(w,
		keyHint{"↑↓", labelKeyPickStep}, keyHint{"pgup/dn", labelKeyScroll}, keyHint{"x", labelKeyRun},
		keyHint{"y", labelKeyCopy}, keyHint{"R", labelKeyReload}, keyHint{"esc", labelKeyBack})
}

// configView draws the open config: a summary line over the Steps and YAML boxes.
func (s *WorkflowsScreen) configView(w, h int) string {
	c := s.cfg
	left := sPink.Bold(true).Render(c.entry.Name)
	if c.path != "" {
		left += " " + sDim.Render(tildePath(c.path))
	}
	right := c.entry.Source
	if c.entry.Version != "" {
		right += " · v" + c.entry.Version
	}

	lw := max(min(38, w/3), 4)
	bh := max(h-1, 3)
	return row(left, sDim.Render(right), w) + "\n" + joinH(s.configStepsBox(lw, bh), s.configYAMLBox(w-lw-1, bh), 1)
}

func (s *WorkflowsScreen) configStepsBox(w, h int) string {
	c := s.cfg
	inner := w - 4
	var lines []string
	switch {
	case c.loading:
		lines = append(lines, sDim.Render(labelConfigLoading))
	case c.err != "":
		lines = append(lines, sBad.Render("✗"))
	}
	n := 0
	for i := range c.items {
		it := &c.items[i]
		label := it.Name
		if it.Name != labelConfigGeneral || i != 0 {
			n++
			label = fmt.Sprintf("%d %s", n, it.Name)
		}
		line := row(label, sDim.Render(it.Kind), inner)
		if i == c.sel {
			line = selRow(line, inner, cSelBg, true)
		}
		lines = append(lines, line)
	}
	return box(labelConfigStepsTitle, lines, w, h, false)
}

func (s *WorkflowsScreen) configYAMLBox(w, h int) string {
	c := s.cfg
	title := labelDetailsTitleConfig
	switch {
	case c.loading:
		return box(title, []string{sDim.Render(labelConfigLoading)}, w, h, true)
	case c.err != "":
		return box(title, styleAll(wrapText(c.err, w-4), sBad.Render), w, h, true)
	}
	if item, ok := c.selected(); ok {
		title = item.Name
		if item.Kind != "" {
			title += " · " + item.Kind
		}
	}
	all := c.styledLines()
	rows := max(1, h-2)
	visible, start, end := scrollWindow(all, c.offset, rows)
	return box(scrollTitle(title, start, end, len(all), rows), visible, w, h, true)
}
