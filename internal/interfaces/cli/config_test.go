package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestDefaultConfig(t *testing.T) {
	cfg := cli.DefaultConfig()

	if cfg.Verbose {
		t.Error("expected Verbose to be false by default")
	}
	if cfg.Quiet {
		t.Error("expected Quiet to be false by default")
	}
	if cfg.NoColor {
		t.Error("expected NoColor to be false by default")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel 'info', got '%s'", cfg.LogLevel)
	}
	if !strings.Contains(cfg.StoragePath, "awf") {
		t.Errorf("expected StoragePath to contain 'awf', got '%s'", cfg.StoragePath)
	}
	if cfg.OutputMode != cli.OutputSilent {
		t.Errorf("expected OutputMode to be silent by default, got %v", cfg.OutputMode)
	}
}

func TestOutputMode_String(t *testing.T) {
	tests := []struct {
		mode cli.OutputMode
		want string
	}{
		{cli.OutputSilent, "silent"},
		{cli.OutputStreaming, "streaming"},
		{cli.OutputBuffered, "buffered"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.mode.String())
	}
}

func TestParseOutputMode(t *testing.T) {
	tests := []struct {
		input   string
		want    cli.OutputMode
		wantErr bool
	}{
		{"silent", cli.OutputSilent, false},
		{"streaming", cli.OutputStreaming, false},
		{"buffered", cli.OutputBuffered, false},
		{"invalid", cli.OutputSilent, true},
		{"", cli.OutputSilent, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := cli.ParseOutputMode(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
