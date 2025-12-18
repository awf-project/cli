package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/vanoix/awf/internal/domain/plugin"
)

// ErrStateStoreNotImplemented indicates a stub method that needs implementation.
// Kept for backwards compatibility with tests checking for stub behavior.
var ErrStateStoreNotImplemented = errors.New("state_store: not implemented")

const pluginsFileName = "plugins.json"

// JSONPluginStateStore persists plugin states to a JSON file.
// Implements ports.PluginStateStore interface.
type JSONPluginStateStore struct {
	mu       sync.RWMutex
	basePath string                         // Directory containing plugins.json
	states   map[string]*plugin.PluginState // plugin name -> state
}

// NewJSONPluginStateStore creates a new JSONPluginStateStore.
func NewJSONPluginStateStore(basePath string) *JSONPluginStateStore {
	return &JSONPluginStateStore{
		basePath: basePath,
		states:   make(map[string]*plugin.PluginState),
	}
}

// Save persists all plugin states to storage with atomic write.
func (s *JSONPluginStateStore) Save(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.RLock()
	statesToSave := make(map[string]*plugin.PluginState, len(s.states))
	for k, v := range s.states {
		statesToSave[k] = v
	}
	s.mu.RUnlock()

	if err := os.MkdirAll(s.basePath, 0750); err != nil {
		return err
	}

	finalPath := s.filePath()
	tmpPath := fmt.Sprintf("%s.%d.%d.tmp", finalPath, os.Getpid(), time.Now().UnixNano())

	data, err := json.MarshalIndent(statesToSave, "", "  ")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	if _, err := f.Write(data); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	if err := f.Sync(); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()

	return os.Rename(tmpPath, finalPath)
}

// Load reads plugin states from storage.
func (s *JSONPluginStateStore) Load(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	filePath := s.filePath()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No file = no persisted state, start fresh
			s.mu.Lock()
			s.states = make(map[string]*plugin.PluginState)
			s.mu.Unlock()
			return nil
		}
		return err
	}

	// Handle empty file
	if len(data) == 0 {
		s.mu.Lock()
		s.states = make(map[string]*plugin.PluginState)
		s.mu.Unlock()
		return nil
	}

	var loadedStates map[string]*plugin.PluginState
	if err := json.Unmarshal(data, &loadedStates); err != nil {
		return err
	}

	s.mu.Lock()
	s.states = loadedStates
	if s.states == nil {
		s.states = make(map[string]*plugin.PluginState)
	}
	s.mu.Unlock()

	return nil
}

// SetEnabled enables or disables a plugin by name.
func (s *JSONPluginStateStore) SetEnabled(ctx context.Context, name string, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.states[name]
	if !exists {
		state = plugin.NewPluginState()
		s.states[name] = state
	}

	state.Enabled = enabled

	if enabled {
		state.DisabledAt = 0
	} else {
		state.DisabledAt = time.Now().Unix()
	}

	return nil
}

// IsEnabled returns whether a plugin is enabled.
// Returns true for unknown plugins (enabled by default).
func (s *JSONPluginStateStore) IsEnabled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, exists := s.states[name]
	if !exists {
		return true // Default: plugins are enabled
	}
	return state.Enabled
}

// GetConfig returns the stored configuration for a plugin.
// Returns nil if plugin has no stored configuration.
func (s *JSONPluginStateStore) GetConfig(name string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, exists := s.states[name]
	if !exists {
		return nil
	}
	return state.Config
}

// SetConfig stores configuration for a plugin.
func (s *JSONPluginStateStore) SetConfig(ctx context.Context, name string, config map[string]any) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.states[name]
	if !exists {
		state = plugin.NewPluginState()
		s.states[name] = state
	}

	state.Config = config

	return nil
}

// GetState returns the full state for a plugin, or nil if not found.
func (s *JSONPluginStateStore) GetState(name string) *plugin.PluginState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.states[name]
}

// ListDisabled returns names of all explicitly disabled plugins.
func (s *JSONPluginStateStore) ListDisabled() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var disabled []string
	for name, state := range s.states {
		if !state.Enabled {
			disabled = append(disabled, name)
		}
	}
	return disabled
}

// BasePath returns the storage directory path.
func (s *JSONPluginStateStore) BasePath() string {
	return s.basePath
}

// filePath returns the full path to the plugins.json file.
func (s *JSONPluginStateStore) filePath() string {
	return filepath.Join(s.basePath, pluginsFileName)
}
