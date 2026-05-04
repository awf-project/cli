package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Color palette — indigo/violet accent, balanced density.
var (
	ColorPrimary      color.Color = lipgloss.Color("99")  // Indigo — active tab, selection, titles
	ColorPrimaryLight color.Color = lipgloss.Color("141") // Lavender — active borders, secondary accents
	ColorSuccess      color.Color = lipgloss.Color("78")  // Green — completed, success
	ColorError        color.Color = lipgloss.Color("204") // Rose red — failed, errors
	ColorWarning      color.Color = lipgloss.Color("214") // Orange — cancelled, warnings
	ColorRunning      color.Color = lipgloss.Color("81")  // Cyan — running, spinners
	ColorMuted        color.Color = lipgloss.Color("242") // Gray — secondary text, inactive borders
	ColorSurface      color.Color = lipgloss.Color("236") // Dark gray — panel backgrounds
	ColorText         color.Color = lipgloss.Color("252") // Off-white — primary text
)

var (
	StyleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(ColorPrimary).
			Padding(0, 2)

	StyleTabInactive = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 2)

	StyleSelectedRow = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")).
				Background(ColorPrimary)

	StyleStatusBar = lipgloss.NewStyle().
			Background(ColorSurface).
			Foreground(ColorText).
			Padding(0, 1)

	StyleEmptyState = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(1, 2)

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimaryLight)

	StyleInputLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	StyleInputHint = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleErrorBanner = lipgloss.NewStyle().
				Foreground(ColorError).
				Bold(true)
)
