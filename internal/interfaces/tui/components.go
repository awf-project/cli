package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// StatusBadge renders a colored icon + label for an ExecutionStatus.
func StatusBadge(status workflow.ExecutionStatus) string {
	switch status {
	case workflow.StatusPending:
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("⏳ pending")
	case workflow.StatusRunning:
		return lipgloss.NewStyle().Foreground(ColorRunning).Render("⟳ running")
	case workflow.StatusCompleted:
		return lipgloss.NewStyle().Foreground(ColorSuccess).Render("✓ success")
	case workflow.StatusFailed:
		return lipgloss.NewStyle().Foreground(ColorError).Render("✗ failed")
	case workflow.StatusCancelled:
		return lipgloss.NewStyle().Foreground(ColorWarning).Render("⏭ cancelled")
	default:
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("⏳ pending")
	}
}

// StatusBadgeFromString renders a colored icon + label from a status string.
func StatusBadgeFromString(status string) string {
	switch status {
	case "success", "completed":
		return lipgloss.NewStyle().Foreground(ColorSuccess).Render("✓ success")
	case "failed":
		return lipgloss.NewStyle().Foreground(ColorError).Render("✗ failed")
	case "cancelled":
		return lipgloss.NewStyle().Foreground(ColorWarning).Render("⏭ cancelled")
	default:
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("? " + status)
	}
}

// Panel renders content inside a titled bordered box.
func Panel(title, content string, width, height int, active bool) string {
	borderColor := ColorMuted
	if active {
		borderColor = ColorPrimaryLight
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorText)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(width).
		Height(height)

	header := titleStyle.Render(title)
	body := header + "\n" + content

	return style.Render(body)
}

// EmptyStateView renders a centered empty state with icon, title, and subtitle.
func EmptyStateView(icon, title, subtitle string) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorText)
	subStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	var b strings.Builder
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "    %s\n\n", icon)
	fmt.Fprintf(&b, "    %s\n", titleStyle.Render(title))
	fmt.Fprintf(&b, "    %s\n", subStyle.Render(subtitle))
	return b.String()
}

// HeaderBar renders a full-width bar with left and right content.
func HeaderBar(left, right string, width int) string {
	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	gap := width - leftLen - rightLen
	if gap < 1 {
		gap = 1
	}

	// Render each segment independently with the bar style to apply colors,
	// then join raw to prevent lipgloss word-wrapping "right text" across lines.
	barStyle := StyleStatusBar.UnsetWidth()
	renderedLeft := barStyle.Render(left)
	renderedGap := barStyle.Render(strings.Repeat(" ", gap))
	renderedRight := barStyle.Render(right)
	return renderedLeft + renderedGap + renderedRight
}

// Separator renders a styled horizontal line.
func Separator(width int) string {
	if width < 1 {
		width = 1
	}
	return lipgloss.NewStyle().Foreground(ColorPrimaryLight).Render(strings.Repeat("─", width))
}
