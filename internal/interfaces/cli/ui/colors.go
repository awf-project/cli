package ui

import "github.com/fatih/color"

// Colorizer provides colored output for CLI.
type Colorizer struct {
	enabled bool
	success *color.Color
	err     *color.Color
	warning *color.Color
	info    *color.Color
	bold    *color.Color
	dim     *color.Color
}

// NewColorizer creates a new colorizer.
//
// Color enablement is set per *color.Color instance rather than through the
// fatih/color package global (color.NoColor). Mutating that global here raced with
// concurrent Sprint reads from the facade execution goroutine and the event
// projector (both build/use colorizers on the shared run path). Pinning each
// instance's state via Enable/DisableColor makes Sprint read the per-instance flag
// and never touch the package global.
func NewColorizer(enabled bool) *Colorizer {
	c := &Colorizer{
		enabled: enabled,
		success: color.New(color.FgGreen),
		err:     color.New(color.FgRed),
		warning: color.New(color.FgYellow),
		info:    color.New(color.FgCyan),
		bold:    color.New(color.Bold),
		dim:     color.New(color.Faint),
	}

	for _, instance := range []*color.Color{c.success, c.err, c.warning, c.info, c.bold, c.dim} {
		if enabled {
			instance.EnableColor()
		} else {
			instance.DisableColor()
		}
	}

	return c
}

// Success formats text as success (green).
func (c *Colorizer) Success(s string) string {
	return c.success.Sprint(s)
}

// Error formats text as error (red).
func (c *Colorizer) Error(s string) string {
	return c.err.Sprint(s)
}

// Warning formats text as warning (yellow).
func (c *Colorizer) Warning(s string) string {
	return c.warning.Sprint(s)
}

// Info formats text as info (cyan).
func (c *Colorizer) Info(s string) string {
	return c.info.Sprint(s)
}

// Bold formats text as bold.
func (c *Colorizer) Bold(s string) string {
	return c.bold.Sprint(s)
}

// Dim formats text as dimmed.
func (c *Colorizer) Dim(s string) string {
	return c.dim.Sprint(s)
}

// Status formats text based on execution status.
func (c *Colorizer) Status(status, text string) string {
	switch status {
	case "completed":
		return c.success.Sprint(text)
	case "running":
		return c.info.Sprint(text)
	case "failed":
		return c.err.Sprint(text)
	case "pending":
		return c.dim.Sprint(text)
	case "cancelled":
		return c.warning.Sprint(text)
	default:
		return text
	}
}
