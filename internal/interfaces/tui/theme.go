package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Page palette: a dark violet page with a pink logo and purple accents.
var (
	cBg     color.Color = lipgloss.Color("#17141f") // page background
	cBorder color.Color = lipgloss.Color("#4a3f6b") // unfocused box border, rules
	cAccent color.Color = lipgloss.Color("#7d56f4") // selected row, badges
	cPurple color.Color = lipgloss.Color("#a58bff") // focused border, section titles
	cPink   color.Color = lipgloss.Color("#ff6eb4") // logo, screen titles
	cOK     color.Color = lipgloss.Color("#5fd7a0") // success
	cBad    color.Color = lipgloss.Color("#ff7a8a") // failure, errors
	cRun    color.Color = lipgloss.Color("#f5d76e") // running, flashes
	cDim    color.Color = lipgloss.Color("#9a92b0") // secondary text
	cText   color.Color = lipgloss.Color("#e6e1f0") // primary text
	cSelBg  color.Color = lipgloss.Color("#2a2440") // selected step, key chips
)

// Named roles used across the package. They map onto the page palette so that older
// call sites (tree rendering, status badges) share the same colors.
var (
	ColorPrimary                  = cAccent
	ColorPrimaryLight             = cPurple
	ColorSuccess                  = cOK
	ColorError                    = cBad
	ColorWarning      color.Color = lipgloss.Color("#ffb86c") // cancelled, caution
	ColorRunning                  = cRun
	ColorMuted                    = cDim
	ColorSurface                  = cSelBg
	ColorText                     = cText
)

// Text styles.
var (
	sText   = lipgloss.NewStyle().Foreground(cText)
	sDim    = lipgloss.NewStyle().Foreground(cDim)
	sOK     = lipgloss.NewStyle().Foreground(cOK)
	sBad    = lipgloss.NewStyle().Foreground(cBad)
	sRun    = lipgloss.NewStyle().Foreground(cRun)
	sPink   = lipgloss.NewStyle().Foreground(cPink)
	sPurple = lipgloss.NewStyle().Foreground(cPurple)
	sLogo   = lipgloss.NewStyle().Background(cPink).Foreground(cBg).Bold(true).Padding(0, 1)
	sTabOn  = lipgloss.NewStyle().Background(lipgloss.Color("#4b36a8")).Foreground(lipgloss.Color("#ffffff")).Bold(true).Padding(0, 1)
	sTabOff = lipgloss.NewStyle().Foreground(cDim).Padding(0, 1)
	sBadge  = lipgloss.NewStyle().Background(cAccent).Foreground(lipgloss.Color("#ffffff")).Bold(true).Padding(0, 1)
	sKey    = lipgloss.NewStyle().Background(cSelBg).Foreground(cText).Padding(0, 1)
)

// Styles used by the run screen's step view.
var (
	StyleEmptyState = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(1, 2)

	StyleBlockInput  = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimaryLight)
	StyleBlockOutput = lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	StyleBlockError  = lipgloss.NewStyle().Bold(true).Foreground(ColorError)
	StyleBlockWarn   = lipgloss.NewStyle().Bold(true).Foreground(ColorWarning)
	StyleBlockDim    = lipgloss.NewStyle().Foreground(ColorMuted)
)
