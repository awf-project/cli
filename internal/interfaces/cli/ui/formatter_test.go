package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

func TestFormatter_Verbose(t *testing.T) {
	buf := new(bytes.Buffer)
	f := ui.NewFormatter(buf, ui.FormatOptions{
		Verbose: true,
		Quiet:   false,
		NoColor: true,
	})

	f.Info("test message")
	f.Debug("debug message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected info message, got: %s", output)
	}
	if !strings.Contains(output, "debug message") {
		t.Errorf("expected debug message in verbose mode, got: %s", output)
	}
}

func TestFormatter_QuietMode(t *testing.T) {
	buf := new(bytes.Buffer)
	f := ui.NewFormatter(buf, ui.FormatOptions{
		Verbose: false,
		Quiet:   true,
		NoColor: true,
	})

	f.Info("info message")
	f.Debug("debug message")
	f.Error("error message")

	output := buf.String()
	if strings.Contains(output, "info message") {
		t.Errorf("quiet mode should suppress info, got: %s", output)
	}
	if !strings.Contains(output, "error message") {
		t.Errorf("quiet mode should show errors, got: %s", output)
	}
}

func TestFormatter_NormalMode(t *testing.T) {
	buf := new(bytes.Buffer)
	f := ui.NewFormatter(buf, ui.FormatOptions{
		Verbose: false,
		Quiet:   false,
		NoColor: true,
	})

	f.Info("info message")
	f.Debug("debug message")

	output := buf.String()
	if !strings.Contains(output, "info message") {
		t.Errorf("expected info message, got: %s", output)
	}
	if strings.Contains(output, "debug message") {
		t.Errorf("normal mode should not show debug, got: %s", output)
	}
}

func TestFormatter_Table(t *testing.T) {
	buf := new(bytes.Buffer)
	f := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	headers := []string{"NAME", "VERSION", "DESCRIPTION"}
	rows := [][]string{
		{"workflow1", "1.0.0", "First workflow"},
		{"workflow2", "2.0.0", "Second workflow"},
	}

	f.Table(headers, rows)

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Errorf("expected headers, got: %s", output)
	}
	if !strings.Contains(output, "workflow1") {
		t.Errorf("expected row data, got: %s", output)
	}
}

func TestFormatter_Success(t *testing.T) {
	buf := new(bytes.Buffer)
	f := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	f.Success("operation completed")

	output := buf.String()
	if !strings.Contains(output, "operation completed") {
		t.Errorf("expected success message, got: %s", output)
	}
}

func TestFormatter_Printf(t *testing.T) {
	buf := new(bytes.Buffer)
	f := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	f.Printf("value: %d\n", 42)

	output := buf.String()
	if !strings.Contains(output, "value: 42") {
		t.Errorf("expected formatted output, got: %s", output)
	}
}
