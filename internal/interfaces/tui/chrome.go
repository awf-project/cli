package tui

import (
	"fmt"
	"image/color"
	"regexp"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// This file holds the drawing primitives shared by every screen: fixed-width text,
// rounded boxes with a title on the border, the key bar, the page background and
// small formatting helpers. They are pure functions of their inputs.

const (
	marginX = 2
	marginY = 1

	minTermWidth  = 80
	minTermHeight = 24
)

// fit truncates or pads s to exactly w cells (ANSI aware).
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	if d := w - lipgloss.Width(s); d > 0 {
		s += strings.Repeat(" ", d)
	}
	return s
}

// row lays out left and right text on one line of width w.
func row(left, right string, w int) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return fit(left+" "+right, w)
	}
	return left + strings.Repeat(" ", gap) + right
}

// selRow paints a full-width row background (the selected line). Styling inside s is
// dropped so the row reads as one solid bar.
func selRow(s string, w int, bg color.Color, bold bool) string {
	st := lipgloss.NewStyle().Background(bg).Foreground(cText)
	if bg == cAccent {
		st = st.Foreground(lipgloss.Color("#ffffff"))
	}
	if bold {
		st = st.Bold(true)
	}
	return st.Render(fit(ansi.Strip(s), w))
}

// box draws a rounded frame with its title on the top border. w and h are the outer
// size; lines are clipped to the inner area.
func box(title string, lines []string, w, h int, focused bool) string {
	if w < 4 || h < 3 {
		return ""
	}
	bc := lipgloss.NewStyle().Foreground(cBorder)
	if focused {
		bc = lipgloss.NewStyle().Foreground(cPurple)
	}
	inner := w - 2
	t := ""
	if title != "" {
		t = " " + sPurple.Bold(true).Render(ansi.Truncate(title, inner-4, "…")) + " "
	}
	top := bc.Render("╭─") + t + bc.Render(strings.Repeat("─", max(0, inner-1-lipgloss.Width(t)))+"╮")
	var b strings.Builder
	b.WriteString(top + "\n")
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		b.WriteString(bc.Render("│") + fit(" "+l, inner-1) + " " + bc.Render("│") + "\n")
	}
	b.WriteString(bc.Render("╰" + strings.Repeat("─", inner) + "╯"))
	return b.String()
}

// keyHint is one entry of the key bar: the key chip and what it does.
type keyHint struct{ key, desc string }

// keyBar renders the rule and the key chips that close every screen.
func keyBar(w int, items ...keyHint) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, sKey.Render(it.key)+" "+sDim.Render(it.desc))
	}
	return ruleLine(w) + "\n" + fit(strings.Join(parts, "  "), w)
}

func ruleLine(w int) string {
	return lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("─", max(w, 0)))
}

// joinH places two multi-line blocks side by side separated by gap columns.
func joinH(a, b string, gap int) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, a, strings.Repeat(" ", gap), b)
}

// resetRe matches every SGR sequence that starts with a full reset: "\x1b[m",
// "\x1b[0m" and combined forms such as "\x1b[0;4m".
var resetRe = regexp.MustCompile(`\x1b\[0?(?:;[0-9;]*)?m`)

// bgSeq is the escape sequence that selects the page background in the current
// color profile (empty when colors are disabled).
func bgSeq() string {
	const probe = "\x00"
	r := lipgloss.NewStyle().Background(cBg).Render(probe)
	if i := strings.Index(r, probe); i > 0 {
		return r[:i]
	}
	return ""
}

// paintPage paints the page background over the whole sw x sh terminal and surrounds the
// w x h content with a margin. Every style reset inside the content is followed by the
// background again, otherwise it would leave holes behind colored text.
func paintPage(content string, sw, sh, w, h int) string {
	bg := bgSeq()
	const reset = "\x1b[0m"
	lines := strings.Split(content, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	blank := bg + strings.Repeat(" ", sw) + reset
	side := bg + strings.Repeat(" ", marginX)
	out := make([]string, 0, sh)
	for i := 0; i < marginY; i++ {
		out = append(out, blank)
	}
	for i := 0; i < h; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		l = fit(l, w)
		l = resetRe.ReplaceAllStringFunc(l, func(m string) string { return m + bg })
		out = append(out, side+l+bg+strings.Repeat(" ", marginX)+reset)
	}
	for len(out) < sh {
		out = append(out, blank)
	}
	return strings.Join(out[:sh], "\n")
}

// overlay draws a centered popup over the rendered screen.
func overlay(base, pop string, w, h int) string {
	baseLines := strings.Split(base, "\n")
	popLines := strings.Split(pop, "\n")
	pw := lipgloss.Width(pop)
	x := max(0, (w-pw)/2)
	y := max(0, (h-len(popLines))/2)
	for i, pl := range popLines {
		if y+i >= len(baseLines) {
			break
		}
		l := baseLines[y+i]
		baseLines[y+i] = ansi.Cut(l, 0, x) + pl + ansi.Cut(l, x+pw, max(lipgloss.Width(l), x+pw))
	}
	return strings.Join(baseLines, "\n")
}

// statusIcon renders the colored icon of a status. It accepts both the execution
// statuses ("completed", "failed"...) and the history statuses ("success").
func statusIcon(status string) string {
	switch status {
	case "completed", "success":
		return sOK.Render("✓")
	case "failed":
		return sBad.Render("✗")
	case "running":
		return sRun.Render("●")
	case "cancelled":
		return sDim.Render("⊘")
	case "skipped":
		return sDim.Render("–")
	}
	return sDim.Render("○")
}

// progressBar renders a bar of width w filled to frac (0..1).
func progressBar(w int, frac float64) string {
	frac = min(max(frac, 0), 1)
	filled := int(float64(w) * frac)
	return lipgloss.NewStyle().Foreground(cAccent).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("░", w-filled))
}

// fmtDur formats a duration compactly: "0s", "3.2s", "4m05s", "2h07m".
func fmtDur(d time.Duration) string {
	switch {
	case d <= 0:
		return "0s"
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// fmtClock formats an elapsed time as mm:ss.
func fmtClock(d time.Duration) string {
	s := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

// relTime formats t relative to now ("just now", "5 min ago", "yesterday 14:03").
func relTime(t time.Time) string { return relTimeAt(t, time.Now()) }

func relTimeAt(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := now.Sub(t)
	switch {
	case d < 20*time.Second:
		return "just now"
	case d < time.Minute:
		return "<1 min ago"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday " + t.Format("15:04")
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d d ago", int(d.Hours()/24))
	}
	return t.Format("02/01 15:04")
}

// isSecret mirrors AWF's masking convention: such values are never rendered.
func isSecret(name string) bool {
	u := strings.ToUpper(name)
	return strings.HasPrefix(u, "SECRET_") || strings.HasPrefix(u, "API_KEY") || strings.HasPrefix(u, "PASSWORD")
}

// maskValue hides the value of secret-looking inputs.
func maskValue(name, v string) string {
	if isSecret(name) {
		return "***"
	}
	return v
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// shortID shortens a run identifier for display.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// scrollWindow returns the lines of a panel of rows lines scrolled to offset. The offset is
// clamped to the content, so a stale offset (after a resize or a shorter step) still shows
// a full panel. start and end are the zero-based bounds of the visible range.
func scrollWindow(lines []string, offset, rows int) (win []string, start, end int) {
	rows = max(rows, 1)
	start = min(max(offset, 0), max(0, len(lines)-rows))
	end = min(start+rows, len(lines))
	return append([]string(nil), lines[start:end]...), start, end
}

// scrollTitle appends the visible range to a box title when the content overflows.
func scrollTitle(title string, start, end, total, rows int) string {
	if total <= rows {
		return title
	}
	return title + " " + sDim.Render(fmt.Sprintf("(%d–%d/%d)", start+1, end, total))
}

// wrapText breaks text into lines of at most width cells, so a long message stays readable
// inside a box instead of being cut off by an ellipsis.
func wrapText(text string, width int) []string {
	wrapped := lipgloss.NewStyle().Width(max(width, 1)).Render(text)
	out := strings.Split(wrapped, "\n")
	for i := range out {
		out[i] = strings.TrimRight(out[i], " ")
	}
	return out
}

// styleAll applies a style to every line.
func styleAll(lines []string, style func(...string) string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = style(l)
	}
	return out
}
