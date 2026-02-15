package plugin_test

import (
	"errors"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/plugin"
	"github.com/stretchr/testify/assert"
)

func TestPluginStatus_Values(t *testing.T) {
	tests := []struct {
		status plugin.PluginStatus
		want   string
	}{
		{plugin.StatusDiscovered, "discovered"},
		{plugin.StatusLoaded, "loaded"},
		{plugin.StatusInitialized, "initialized"},
		{plugin.StatusRunning, "running"},
		{plugin.StatusStopped, "stopped"},
		{plugin.StatusFailed, "failed"},
		{plugin.StatusDisabled, "disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.status))
		})
	}
}

func TestPluginInfo_Creation(t *testing.T) {
	manifest := &plugin.Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
	}
	now := time.Now().Unix()

	info := plugin.PluginInfo{
		Manifest:      manifest,
		Status:        plugin.StatusLoaded,
		Path:          "/plugins/test-plugin",
		LoadedAt:      now,
		InitializedAt: 0,
	}

	assert.Equal(t, manifest, info.Manifest)
	assert.Equal(t, plugin.StatusLoaded, info.Status)
	assert.Equal(t, "/plugins/test-plugin", info.Path)
	assert.Equal(t, now, info.LoadedAt)
	assert.Zero(t, info.InitializedAt)
	assert.Nil(t, info.Error)
}

func TestPluginInfo_WithError(t *testing.T) {
	testErr := errors.New("initialization failed")
	info := plugin.PluginInfo{
		Status: plugin.StatusFailed,
		Error:  testErr,
	}

	assert.Equal(t, plugin.StatusFailed, info.Status)
	assert.Equal(t, testErr, info.Error)
}

func TestPluginInfo_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status plugin.PluginStatus
		want   bool
	}{
		{"discovered is not active", plugin.StatusDiscovered, false},
		{"loaded is not active", plugin.StatusLoaded, false},
		{"initialized is not active", plugin.StatusInitialized, false},
		{"running is active", plugin.StatusRunning, true},
		{"stopped is not active", plugin.StatusStopped, false},
		{"failed is not active", plugin.StatusFailed, false},
		{"disabled is not active", plugin.StatusDisabled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := plugin.PluginInfo{Status: tt.status}
			assert.Equal(t, tt.want, info.IsActive())
		})
	}
}

func TestPluginInfo_CanLoad(t *testing.T) {
	tests := []struct {
		name   string
		status plugin.PluginStatus
		want   bool
	}{
		{"discovered can load", plugin.StatusDiscovered, true},
		{"loaded cannot load", plugin.StatusLoaded, false},
		{"initialized cannot load", plugin.StatusInitialized, false},
		{"running cannot load", plugin.StatusRunning, false},
		{"stopped can load", plugin.StatusStopped, true},
		{"failed can load", plugin.StatusFailed, true},
		{"disabled cannot load", plugin.StatusDisabled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := plugin.PluginInfo{Status: tt.status}
			assert.Equal(t, tt.want, info.CanLoad())
		})
	}
}

func TestPluginInfo_Timestamps(t *testing.T) {
	now := time.Now().Unix()
	laterTime := now + 5

	info := plugin.PluginInfo{
		LoadedAt:      now,
		InitializedAt: laterTime,
	}

	assert.Less(t, info.LoadedAt, info.InitializedAt)
	assert.Equal(t, int64(5), info.InitializedAt-info.LoadedAt)
}

func TestPluginInfo_ZeroValue(t *testing.T) {
	var info plugin.PluginInfo

	assert.Nil(t, info.Manifest)
	assert.Empty(t, info.Status)
	assert.Empty(t, info.Path)
	assert.Nil(t, info.Error)
	assert.Zero(t, info.LoadedAt)
	assert.Zero(t, info.InitializedAt)
	assert.False(t, info.IsActive())
	assert.False(t, info.CanLoad()) // empty status is not loadable
}

func TestPluginInfo_NilManifest(t *testing.T) {
	info := plugin.PluginInfo{
		Status: plugin.StatusDiscovered,
		Path:   "/plugins/orphan",
	}

	assert.Nil(t, info.Manifest)
	assert.True(t, info.CanLoad())
}

func TestPluginInfo_StatusTransitions(t *testing.T) {
	// Test typical lifecycle: Discovered -> Loaded -> Initialized -> Running -> Stopped
	info := plugin.PluginInfo{
		Manifest: &plugin.Manifest{Name: "test"},
		Status:   plugin.StatusDiscovered,
		Path:     "/plugins/test",
	}

	assert.True(t, info.CanLoad())
	assert.False(t, info.IsActive())

	// Simulate load
	info.Status = plugin.StatusLoaded
	info.LoadedAt = time.Now().Unix()
	assert.False(t, info.CanLoad())
	assert.False(t, info.IsActive())

	// Simulate init
	info.Status = plugin.StatusInitialized
	info.InitializedAt = time.Now().Unix()
	assert.False(t, info.CanLoad())
	assert.False(t, info.IsActive())

	// Simulate run
	info.Status = plugin.StatusRunning
	assert.False(t, info.CanLoad())
	assert.True(t, info.IsActive())

	// Simulate stop
	info.Status = plugin.StatusStopped
	assert.True(t, info.CanLoad()) // can reload after stop
	assert.False(t, info.IsActive())
}

func TestPluginInfo_ErrorRecovery(t *testing.T) {
	info := plugin.PluginInfo{
		Manifest: &plugin.Manifest{Name: "test"},
		Status:   plugin.StatusFailed,
		Error:    errors.New("connection refused"),
	}

	assert.True(t, info.CanLoad()) // can retry after failure
	assert.False(t, info.IsActive())

	// Simulate recovery
	info.Status = plugin.StatusRunning
	info.Error = nil
	assert.True(t, info.IsActive())
	assert.Nil(t, info.Error)
}

func TestPluginStatus_UnknownValue(t *testing.T) {
	// Test behavior with an unrecognized status value
	unknownStatus := plugin.PluginStatus("unknown")
	info := plugin.PluginInfo{Status: unknownStatus}

	assert.False(t, info.IsActive())
	assert.False(t, info.CanLoad())
	assert.Equal(t, "unknown", string(info.Status))
}
