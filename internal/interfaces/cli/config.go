package cli

import (
	"os"
	"path/filepath"
)

// Config holds CLI configuration.
type Config struct {
	Verbose     bool
	Quiet       bool
	NoColor     bool
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
		LogLevel:    "info",
		ConfigPath:  "",
		StoragePath: defaultStorage,
	}
}
