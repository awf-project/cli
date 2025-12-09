package ui_test

import (
	"testing"

	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

func TestColorizer_WithColorDisabled(t *testing.T) {
	c := ui.NewColorizer(false)

	tests := []struct {
		name   string
		fn     func(string) string
		input  string
		expect string
	}{
		{"success", c.Success, "ok", "ok"},
		{"error", c.Error, "fail", "fail"},
		{"warning", c.Warning, "warn", "warn"},
		{"info", c.Info, "info", "info"},
		{"bold", c.Bold, "bold", "bold"},
		{"dim", c.Dim, "dim", "dim"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.input)
			if got != tt.expect {
				t.Errorf("expected %q, got %q", tt.expect, got)
			}
		})
	}
}

func TestColorizer_WithColorEnabled(t *testing.T) {
	c := ui.NewColorizer(true)

	// Note: fatih/color auto-detects TTY and may disable colors in tests
	// We just verify the functions return non-empty strings
	got := c.Success("ok")
	if got == "" {
		t.Error("expected non-empty output")
	}

	got = c.Error("fail")
	if got == "" {
		t.Error("expected non-empty output")
	}
}

func TestColorizer_StatusColors(t *testing.T) {
	c := ui.NewColorizer(true)

	tests := []struct {
		status string
		input  string
	}{
		{"completed", "done"},
		{"running", "in progress"},
		{"failed", "error"},
		{"pending", "waiting"},
		{"cancelled", "stopped"},
		{"unknown", "???"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := c.Status(tt.status, tt.input)
			if got == "" {
				t.Error("expected non-empty output")
			}
		})
	}
}
