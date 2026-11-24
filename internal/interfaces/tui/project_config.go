package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// ConfigInfo is what the Config screen shows: the project configuration file and the
// workflow inputs it pre-fills. It carries the same information as `awf config show`.
type ConfigInfo struct {
	// Path is where the configuration is looked for.
	Path string
	// Exists reports whether the file is there; a missing file is a valid (empty) setup.
	Exists bool
	// Inputs are the configured input values; command-line inputs take precedence.
	Inputs map[string]any
}

// ConfigReader is the driven port for reading the project configuration.
type ConfigReader interface {
	ReadProjectConfig(ctx context.Context) (ConfigInfo, error)
}

// ConfigLoadedMsg carries the project configuration, or why it could not be read.
type ConfigLoadedMsg struct {
	Info ConfigInfo
	Err  string
}

// SetConfigReader wires the reader used by LoadConfig. A nil reader is accepted.
func (b *Bridge) SetConfigReader(r ConfigReader) {
	b.config = r
}

// LoadConfig returns a tea.Cmd that reads the project configuration and emits
// ConfigLoadedMsg.
func (b *Bridge) LoadConfig(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if b.config == nil {
			return ConfigLoadedMsg{Err: labelConfigScreenNoReader}
		}
		if err := ctx.Err(); err != nil {
			return ConfigLoadedMsg{Err: err.Error()}
		}
		info, err := b.config.ReadProjectConfig(ctx)
		if err != nil {
			return ConfigLoadedMsg{Info: info, Err: err.Error()}
		}
		return ConfigLoadedMsg{Info: info}
	}
}
