package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// OutputMode defines how command output is displayed.
type OutputMode int

const (
	OutputSilent    OutputMode = iota // default: no streaming
	OutputStreaming                   // real-time prefixed output
	OutputBuffered                    // show after completion
)

func (m OutputMode) String() string {
	switch m {
	case OutputSilent:
		return "silent"
	case OutputStreaming:
		return "streaming"
	case OutputBuffered:
		return "buffered"
	default:
		return "unknown"
	}
}

// ParseOutputMode parses a string to OutputMode.
func ParseOutputMode(s string) (OutputMode, error) {
	switch s {
	case "silent":
		return OutputSilent, nil
	case "streaming":
		return OutputStreaming, nil
	case "buffered":
		return OutputBuffered, nil
	default:
		return OutputSilent, fmt.Errorf("invalid output mode: %s (valid: silent, streaming, buffered)", s)
	}
}

// Config holds CLI configuration.
type Config struct {
	Verbose     bool
	Quiet       bool
	NoColor     bool
	OutputMode  OutputMode
	LogLevel    string
	ConfigPath  string
	StoragePath string
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	defaultStorage := filepath.Join(home, ".awf", "storage")

	return &Config{
		Verbose:     false,
		Quiet:       false,
		NoColor:     false,
		OutputMode:  OutputSilent,
		LogLevel:    "info",
		ConfigPath:  "",
		StoragePath: defaultStorage,
	}
}
