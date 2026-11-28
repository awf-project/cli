package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

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
