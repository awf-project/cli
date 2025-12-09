package cli_test

import (
	"strings"
	"testing"

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
	if !strings.Contains(cfg.StoragePath, ".awf") {
		t.Errorf("expected StoragePath to contain '.awf', got '%s'", cfg.StoragePath)
	}
}
