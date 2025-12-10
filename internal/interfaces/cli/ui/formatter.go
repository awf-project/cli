package ui

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// FormatOptions configures formatter behavior.
type FormatOptions struct {
	Verbose bool
	Quiet   bool
	NoColor bool
}

// Formatter handles CLI output formatting.
type Formatter struct {
	out   io.Writer
	opts  FormatOptions
	color *Colorizer
}

// NewFormatter creates a new formatter.
func NewFormatter(out io.Writer, opts FormatOptions) *Formatter {
	return &Formatter{
		out:   out,
		opts:  opts,
		color: NewColorizer(!opts.NoColor),
	}
}

// Printf writes formatted output.
func (f *Formatter) Printf(format string, args ...any) {
	fmt.Fprintf(f.out, format, args...)
}

// Println writes a line.
func (f *Formatter) Println(args ...any) {
	fmt.Fprintln(f.out, args...)
}

// Info writes an info message (suppressed in quiet mode).
func (f *Formatter) Info(msg string) {
	if f.opts.Quiet {
		return
	}
	fmt.Fprintln(f.out, msg)
}

// Debug writes a debug message (only in verbose mode).
func (f *Formatter) Debug(msg string) {
	if !f.opts.Verbose {
		return
	}
	fmt.Fprintln(f.out, f.color.Dim(msg))
}

// Success writes a success message.
func (f *Formatter) Success(msg string) {
	fmt.Fprintln(f.out, f.color.Success(msg))
}

// Error writes an error message (always shown).
func (f *Formatter) Error(msg string) {
	fmt.Fprintln(f.out, f.color.Error(msg))
}

// Warning writes a warning message.
func (f *Formatter) Warning(msg string) {
	if f.opts.Quiet {
		return
	}
	fmt.Fprintln(f.out, f.color.Warning(msg))
}

// StepSuccess displays success feedback for steps with no output.
// Hidden in quiet mode.
func (f *Formatter) StepSuccess(stepID string) {
	if f.opts.Quiet {
		return
	}
	msg := fmt.Sprintf("  ✓ %s: completed successfully", stepID)
	fmt.Fprintln(f.out, f.color.Success(msg))
}

// Table renders data as a formatted table.
func (f *Formatter) Table(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(f.out, 0, 0, 2, ' ', 0)

	// Headers
	headerLine := strings.Join(headers, "\t")
	fmt.Fprintln(w, f.color.Bold(headerLine))

	// Separator
	sep := make([]string, len(headers))
	for i, h := range headers {
		sep[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintln(w, strings.Join(sep, "\t"))

	// Rows
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	w.Flush()
}

// StatusLine writes a status line with colored status.
func (f *Formatter) StatusLine(label, status, detail string) {
	statusText := f.color.Status(status, status)
	fmt.Fprintf(f.out, "%-12s %s %s\n", label+":", statusText, detail)
}

// Colorizer returns the underlying colorizer.
func (f *Formatter) Colorizer() *Colorizer {
	return f.color
}
