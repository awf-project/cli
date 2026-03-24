package pluginmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
)

const pluginsFileName = "plugins.json"

// JSONPluginStateStore persists plugin states to a JSON file.
// Implements ports.PluginStateStore interface.
type JSONPluginStateStore struct {
	mu       sync.RWMutex
	basePath string                              // Directory containing plugins.json
	states   map[string]*pluginmodel.PluginState // plugin name -> state
}

// NewJSONPluginStateStore creates a new JSONPluginStateStore.
func NewJSONPluginStateStore(basePath string) *JSONPluginStateStore {
	return &JSONPluginStateStore{
		basePath: basePath,
		states:   make(map[string]*pluginmodel.PluginState),
	}
}

// Save persists all plugin states to storage with atomic write.
func (s *JSONPluginStateStore) Save(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	s.mu.RLock()
	statesToSave := make(map[string]*pluginmodel.PluginState, len(s.states))
	for k, v := range s.states {
		statesToSave[k] = v
	}
	s.mu.RUnlock()

	if err := os.MkdirAll(s.basePath, 0o750); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	finalPath := s.filePath()
	tmpPath := fmt.Sprintf("%s.%d.%d.tmp", finalPath, os.Getpid(), time.Now().UnixNano())

	data, err := json.MarshalIndent(statesToSave, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil { //nolint:gosec // G115: file descriptors are within int range on all supported platforms
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("lock file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec // G115: file descriptors are within int range on all supported platforms
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec // G115: file descriptors are within int range on all supported platforms
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync temp file: %w", err)
	}

	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec // G115: file descriptors are within int range on all supported platforms
	_ = f.Close()

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename to final: %w", err)
	}
	return nil
}

// Load reads plugin states from storage.
func (s *JSONPluginStateStore) Load(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("load: %w", err)
	}

	filePath := s.filePath()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No file = no persisted state, start fresh
			s.mu.Lock()
			s.states = make(map[string]*pluginmodel.PluginState)
			s.mu.Unlock()
			return nil
		}
		return fmt.Errorf("read state file: %w", err)
	}

	// Handle empty file
	if len(data) == 0 {
		s.mu.Lock()
		s.states = make(map[string]*pluginmodel.PluginState)
		s.mu.Unlock()
		return nil
	}

	var loadedStates map[string]*pluginmodel.PluginState
	if err := json.Unmarshal(data, &loadedStates); err != nil {
		return fmt.Errorf("unmarshal state: %w", err)
	}

	s.mu.Lock()
	s.states = loadedStates
	if s.states == nil {
		s.states = make(map[string]*pluginmodel.PluginState)
	}
	s.mu.Unlock()

	return nil
}

// SetEnabled enables or disables a plugin by name.
func (s *JSONPluginStateStore) SetEnabled(ctx context.Context, name string, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("set enabled: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.states[name]
	if !exists {
		state = pluginmodel.NewPluginState()
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
		return fmt.Errorf("set config: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.states[name]
	if !exists {
		state = pluginmodel.NewPluginState()
		s.states[name] = state
	}

	state.Config = config

	return nil
}

// GetState returns the full state for a plugin, or nil if not found.
func (s *JSONPluginStateStore) GetState(name string) *pluginmodel.PluginState {
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
