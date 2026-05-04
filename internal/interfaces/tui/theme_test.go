package tui

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThemeColors_AreLipglossColors(t *testing.T) {
	colors := []color.Color{
		ColorPrimary, ColorPrimaryLight, ColorSuccess,
		ColorError, ColorWarning, ColorRunning,
		ColorMuted, ColorSurface, ColorText,
	}
	for _, c := range colors {
		r, g, b, a := c.RGBA()
		// A non-empty color has at least one non-zero channel or full alpha.
		assert.NotZero(t, r+g+b+a, "theme color must not be zero")
	}
}

func TestThemeStyles_AreNonZero(t *testing.T) {
	assert.NotEmpty(t, StyleStatusBar.String())
	assert.NotEmpty(t, StyleTabActive.String())
	assert.NotEmpty(t, StyleTabInactive.String())
	assert.NotEmpty(t, StyleSelectedRow.String())
	assert.NotEmpty(t, StyleEmptyState.String())
	assert.NotEmpty(t, StyleHeader.String())
	assert.NotEmpty(t, StyleInputLabel.String())
	assert.NotEmpty(t, StyleInputHint.String())
	assert.NotEmpty(t, StyleErrorBanner.String())
}
