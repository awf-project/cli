package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		w    int
		want string
	}{
		{"pads short text", "ab", 5, "ab   "},
		{"exact width unchanged", "abcde", 5, "abcde"},
		{"truncates with ellipsis", "abcdefgh", 5, "abcd…"},
		{"zero width is empty", "abc", 0, ""},
		{"negative width is empty", "abc", -3, ""},
		{"counts wide runes by cells", "日本語", 5, "日本…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fit(tt.in, tt.w)
			assert.Equal(t, tt.want, got)
			if tt.w > 0 {
				assert.Equal(t, tt.w, lipgloss.Width(got))
			}
		})
	}
}

func TestFit_IsANSIAware(t *testing.T) {
	styled := sOK.Render("✓ done")
	got := fit(styled, 10)
	assert.Equal(t, 10, lipgloss.Width(got))
	assert.Contains(t, ansi.Strip(got), "✓ done")
}

func TestRow(t *testing.T) {
	assert.Equal(t, "left     right", row("left", "right", 14))
	assert.Equal(t, 14, lipgloss.Width(row("left", "right", 14)))
	// No room for a gap: both survive, joined by one space, clipped to the width.
	got := row("left-side", "right-side", 12)
	assert.Equal(t, 12, lipgloss.Width(got))
}

func TestBox_HasExactOuterSizeAndTitle(t *testing.T) {
	out := box("Steps", []string{"one", "two"}, 30, 6, true)
	lines := strings.Split(ansi.Strip(out), "\n")

	require.Len(t, lines, 6)
	for i, l := range lines {
		assert.Equal(t, 30, lipgloss.Width(l), "line %d must span the outer width", i)
	}
	assert.True(t, strings.HasPrefix(lines[0], "╭─ Steps "))
	assert.True(t, strings.HasSuffix(lines[0], "╮"))
	assert.True(t, strings.HasPrefix(lines[5], "╰"))
	assert.Contains(t, lines[1], "one")
	assert.Contains(t, lines[2], "two")
}

func TestBox_ClipsOverflowingContentAndLongTitles(t *testing.T) {
	lines := []string{"1", "2", "3", "4", "5", "6"}
	out := ansi.Strip(box(strings.Repeat("T", 80), lines, 20, 5, false))

	got := strings.Split(out, "\n")
	require.Len(t, got, 5, "content beyond the inner height must be clipped")
	for _, l := range got {
		assert.Equal(t, 20, lipgloss.Width(l))
	}
	assert.NotContains(t, out, "4", "rows past the inner height are not drawn")
}

func TestBox_TooSmallRendersNothing(t *testing.T) {
	assert.Empty(t, box("x", nil, 3, 5, false))
	assert.Empty(t, box("x", nil, 10, 2, false))
}

func TestSelRow_PaintsFullWidthAndDropsInnerStyling(t *testing.T) {
	out := selRow(sOK.Render("✓")+" step", 12, cAccent, true)
	assert.Equal(t, 12, lipgloss.Width(out))
	assert.Equal(t, "✓ step      ", ansi.Strip(out))
}

func TestKeyBar_RendersRuleAndChips(t *testing.T) {
	out := ansi.Strip(keyBar(40, keyHint{"x", "run"}, keyHint{"q", "quit"}))
	lines := strings.Split(out, "\n")

	require.Len(t, lines, 2)
	assert.Equal(t, strings.Repeat("─", 40), lines[0])
	assert.Contains(t, lines[1], " x  run")
	assert.Contains(t, lines[1], " q  quit")
	assert.Equal(t, 40, lipgloss.Width(lines[1]))
}

func TestPaintPage_FillsTheWholeTerminal(t *testing.T) {
	const sw, sh, w, h = 40, 12, 36, 10
	out := paintPage("hello\nworld", sw, sh, w, h)
	lines := strings.Split(out, "\n")

	require.Len(t, lines, sh)
	for i, l := range lines {
		assert.Equal(t, sw, lipgloss.Width(l), "line %d must span the terminal width", i)
	}
	assert.Contains(t, ansi.Strip(lines[marginY]), "hello")
	assert.Contains(t, ansi.Strip(lines[marginY+1]), "world")
}

func TestPaintPage_ClipsContentTallerThanTheArea(t *testing.T) {
	content := strings.Repeat("row\n", 50)
	out := paintPage(content, 30, 8, 26, 6)
	assert.Len(t, strings.Split(out, "\n"), 8)
}

func TestOverlay_CentersPopupWithoutChangingWidths(t *testing.T) {
	base := strings.Repeat(strings.Repeat(".", 30)+"\n", 9) + strings.Repeat(".", 30)
	pop := "┌──┐\n│ab│\n└──┘"

	out := overlay(base, pop, 30, 10)
	lines := strings.Split(out, "\n")

	require.Len(t, lines, 10)
	for i, l := range lines {
		assert.Equal(t, 30, lipgloss.Width(l), "line %d width must be preserved", i)
	}
	assert.Contains(t, out, "│ab│")
	assert.Equal(t, ".", string(lines[0][0]), "rows outside the popup are untouched")
}

func TestRelTimeAt(t *testing.T) {
	now := time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, "never"},
		{"seconds", now.Add(-5 * time.Second), "just now"},
		{"under a minute", now.Add(-40 * time.Second), "<1 min ago"},
		{"minutes", now.Add(-7 * time.Minute), "7 min ago"},
		{"hours", now.Add(-3 * time.Hour), "3 h ago"},
		{"yesterday", now.Add(-30 * time.Hour), "yesterday 09:00"},
		{"days", now.Add(-4 * 24 * time.Hour), "4 d ago"},
		{"old dates", time.Date(2026, 10, 1, 8, 5, 0, 0, time.UTC), "01/10 08:05"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, relTimeAt(tt.t, now))
		})
	}
}

func TestFmtDurAndClock(t *testing.T) {
	assert.Equal(t, "0s", fmtDur(0))
	assert.Equal(t, "0s", fmtDur(-time.Second))
	assert.Equal(t, "3.2s", fmtDur(3200*time.Millisecond))
	assert.Equal(t, "4m05s", fmtDur(4*time.Minute+5*time.Second))
	assert.Equal(t, "2h07m", fmtDur(2*time.Hour+7*time.Minute))
	assert.Equal(t, "00:00", fmtClock(0))
	assert.Equal(t, "02:05", fmtClock(2*time.Minute+5*time.Second))
}

func TestSecretMasking(t *testing.T) {
	tests := []struct {
		name   string
		secret bool
	}{
		{"SECRET_TOKEN", true},
		{"secret_lower", true},
		{"API_KEY", true},
		{"API_KEY_PROD", true},
		{"PASSWORD", true},
		{"password_db", true},
		{"username", false},
		{"MY_SECRET", false}, // only the documented prefixes are secret
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.secret, isSecret(tt.name))
			want := "value"
			if tt.secret {
				want = "***"
			}
			assert.Equal(t, want, maskValue(tt.name, "value"))
		})
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		frac       float64
		wantFilled int
	}{{0, 0}, {0.5, 5}, {1, 10}, {-1, 0}, {7, 10}}
	for _, tt := range tests {
		plain := ansi.Strip(progressBar(10, tt.frac))
		assert.Equal(t, 10, lipgloss.Width(plain))
		assert.Equal(t, tt.wantFilled, strings.Count(plain, "█"), "frac %v", tt.frac)
	}
}

func TestStatusIcon_CoversExecutionAndHistoryStatuses(t *testing.T) {
	for status, want := range map[string]string{
		"completed": "✓", "success": "✓", "failed": "✗", "running": "●",
		"cancelled": "⊘", "skipped": "–", "pending": "○", "": "○", "weird": "○",
	} {
		assert.Equal(t, want, ansi.Strip(statusIcon(status)), "status %q", status)
	}
}

func TestShortIDAndSortedKeys(t *testing.T) {
	assert.Equal(t, "abcdefgh", shortID("abcdefghijkl"))
	assert.Equal(t, "abc", shortID("abc"))
	assert.Equal(t, []string{"a", "b", "c"}, sortedKeys(map[string]int{"c": 1, "a": 2, "b": 3}))
	assert.Empty(t, sortedKeys(map[string]int(nil)))
}
