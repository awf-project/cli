package pluginmodel_test

import (
	"errors"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
)

func TestPluginStatus_Values(t *testing.T) {
	tests := []struct {
		status pluginmodel.PluginStatus
		want   string
	}{
		{pluginmodel.StatusDiscovered, "discovered"},
		{pluginmodel.StatusLoaded, "loaded"},
		{pluginmodel.StatusInitialized, "initialized"},
		{pluginmodel.StatusRunning, "running"},
		{pluginmodel.StatusStopped, "stopped"},
		{pluginmodel.StatusFailed, "failed"},
		{pluginmodel.StatusDisabled, "disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.status))
		})
	}
}

func TestPluginInfo_Creation(t *testing.T) {
	manifest := &pluginmodel.Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
	}
	now := time.Now().Unix()

	info := pluginmodel.PluginInfo{
		Manifest:      manifest,
		Status:        pluginmodel.StatusLoaded,
		Path:          "/plugins/test-plugin",
		LoadedAt:      now,
		InitializedAt: 0,
	}

	assert.Equal(t, manifest, info.Manifest)
	assert.Equal(t, pluginmodel.StatusLoaded, info.Status)
	assert.Equal(t, "/plugins/test-plugin", info.Path)
	assert.Equal(t, now, info.LoadedAt)
	assert.Zero(t, info.InitializedAt)
	assert.Nil(t, info.Error)
}

func TestPluginInfo_WithError(t *testing.T) {
	testErr := errors.New("initialization failed")
	info := pluginmodel.PluginInfo{
		Status: pluginmodel.StatusFailed,
		Error:  testErr,
	}

	assert.Equal(t, pluginmodel.StatusFailed, info.Status)
	assert.Equal(t, testErr, info.Error)
}

func TestPluginInfo_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status pluginmodel.PluginStatus
		want   bool
	}{
		{"discovered is not active", pluginmodel.StatusDiscovered, false},
		{"loaded is not active", pluginmodel.StatusLoaded, false},
		{"initialized is not active", pluginmodel.StatusInitialized, false},
		{"running is active", pluginmodel.StatusRunning, true},
		{"stopped is not active", pluginmodel.StatusStopped, false},
		{"failed is not active", pluginmodel.StatusFailed, false},
		{"disabled is not active", pluginmodel.StatusDisabled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := pluginmodel.PluginInfo{Status: tt.status}
			assert.Equal(t, tt.want, info.IsActive())
		})
	}
}

func TestPluginInfo_CanLoad(t *testing.T) {
	tests := []struct {
		name   string
		status pluginmodel.PluginStatus
		want   bool
	}{
		{"discovered can load", pluginmodel.StatusDiscovered, true},
		{"loaded cannot load", pluginmodel.StatusLoaded, false},
		{"initialized cannot load", pluginmodel.StatusInitialized, false},
		{"running cannot load", pluginmodel.StatusRunning, false},
		{"stopped can load", pluginmodel.StatusStopped, true},
		{"failed can load", pluginmodel.StatusFailed, true},
		{"disabled cannot load", pluginmodel.StatusDisabled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := pluginmodel.PluginInfo{Status: tt.status}
			assert.Equal(t, tt.want, info.CanLoad())
		})
	}
}

func TestPluginInfo_Timestamps(t *testing.T) {
	now := time.Now().Unix()
	laterTime := now + 5

	info := pluginmodel.PluginInfo{
		LoadedAt:      now,
		InitializedAt: laterTime,
	}

	assert.Less(t, info.LoadedAt, info.InitializedAt)
	assert.Equal(t, int64(5), info.InitializedAt-info.LoadedAt)
}

func TestPluginInfo_ZeroValue(t *testing.T) {
	var info pluginmodel.PluginInfo

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
	info := pluginmodel.PluginInfo{
		Status: pluginmodel.StatusDiscovered,
		Path:   "/plugins/orphan",
	}

	assert.Nil(t, info.Manifest)
	assert.True(t, info.CanLoad())
}

func TestPluginInfo_StatusTransitions(t *testing.T) {
	// Test typical lifecycle: Discovered -> Loaded -> Initialized -> Running -> Stopped
	info := pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{Name: "test"},
		Status:   pluginmodel.StatusDiscovered,
		Path:     "/plugins/test",
	}

	assert.True(t, info.CanLoad())
	assert.False(t, info.IsActive())

	// Simulate load
	info.Status = pluginmodel.StatusLoaded
	info.LoadedAt = time.Now().Unix()
	assert.False(t, info.CanLoad())
	assert.False(t, info.IsActive())

	// Simulate init
	info.Status = pluginmodel.StatusInitialized
	info.InitializedAt = time.Now().Unix()
	assert.False(t, info.CanLoad())
	assert.False(t, info.IsActive())

	// Simulate run
	info.Status = pluginmodel.StatusRunning
	assert.False(t, info.CanLoad())
	assert.True(t, info.IsActive())

	// Simulate stop
	info.Status = pluginmodel.StatusStopped
	assert.True(t, info.CanLoad()) // can reload after stop
	assert.False(t, info.IsActive())
}

func TestPluginInfo_ErrorRecovery(t *testing.T) {
	info := pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{Name: "test"},
		Status:   pluginmodel.StatusFailed,
		Error:    errors.New("connection refused"),
	}

	assert.True(t, info.CanLoad()) // can retry after failure
	assert.False(t, info.IsActive())

	// Simulate recovery
	info.Status = pluginmodel.StatusRunning
	info.Error = nil
	assert.True(t, info.IsActive())
	assert.Nil(t, info.Error)
}

func TestPluginStatus_UnknownValue(t *testing.T) {
	// Test behavior with an unrecognized status value
	unknownStatus := pluginmodel.PluginStatus("unknown")
	info := pluginmodel.PluginInfo{Status: unknownStatus}

	assert.False(t, info.IsActive())
	assert.False(t, info.CanLoad())
	assert.Equal(t, "unknown", string(info.Status))
}
