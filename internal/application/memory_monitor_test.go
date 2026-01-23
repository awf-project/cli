package application_test

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// Feature: C019 - Memory monitoring to detect memory threshold violations

// memoryTestLogger extends mockLogger to capture Info messages needed for memory stats testing.
type memoryTestLogger struct {
	warnings []string
	errors   []string
	infos    []string
}

func (m *memoryTestLogger) Debug(msg string, fields ...any) {}

func (m *memoryTestLogger) Info(msg string, fields ...any) {
	if m.infos == nil {
		m.infos = []string{}
	}
	m.infos = append(m.infos, msg)
}

func (m *memoryTestLogger) Warn(msg string, fields ...any) {
	if m.warnings == nil {
		m.warnings = []string{}
	}
	m.warnings = append(m.warnings, msg)
}

func (m *memoryTestLogger) Error(msg string, fields ...any) {
	if m.errors == nil {
		m.errors = []string{}
	}
	m.errors = append(m.errors, msg)
}

func (m *memoryTestLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

func newMemoryTestLogger() *memoryTestLogger {
	return &memoryTestLogger{
		warnings: []string{},
		errors:   []string{},
		infos:    []string{},
	}
}

// TestNewMemoryMonitor tests creating a memory monitor with various configurations.
func TestNewMemoryMonitor(t *testing.T) {
	tests := []struct {
		name   string
		config workflow.MemoryConfig
		logger ports.Logger
	}{
		{
			name: "monitoring disabled",
			config: workflow.MemoryConfig{
				Enabled:        false,
				ThresholdBytes: 0,
			},
			logger: newMemoryTestLogger(),
		},
		{
			name: "monitoring enabled with 100MB threshold",
			config: workflow.MemoryConfig{
				Enabled:        true,
				ThresholdBytes: 104857600, // 100MB
			},
			logger: newMemoryTestLogger(),
		},
		{
			name: "monitoring enabled with 1GB threshold",
			config: workflow.MemoryConfig{
				Enabled:        true,
				ThresholdBytes: 1073741824, // 1GB
			},
			logger: newMemoryTestLogger(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := application.NewMemoryMonitor(tt.config, tt.logger)
			require.NotNil(t, monitor, "monitor should not be nil")
		})
	}
}

// TestMemoryMonitor_CheckThreshold_DisabledMonitoring tests that disabled monitoring never reports threshold exceeded.
func TestMemoryMonitor_CheckThreshold_DisabledMonitoring(t *testing.T) {
	logger := newMemoryTestLogger()
	config := workflow.MemoryConfig{
		Enabled:        false,
		ThresholdBytes: 1024, // Very low threshold, but monitoring disabled
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx := context.Background()
	heapAlloc, exceeded := monitor.CheckThreshold(ctx)

	// When monitoring is disabled, should not report threshold exceeded
	assert.False(t, exceeded, "disabled monitoring should never report threshold exceeded")
	assert.GreaterOrEqual(t, heapAlloc, uint64(0), "heap allocation should be non-negative")
	assert.Empty(t, logger.warnings, "no warnings should be logged when monitoring is disabled")
}

// TestMemoryMonitor_CheckThreshold_ZeroThreshold tests that zero threshold means no limit.
func TestMemoryMonitor_CheckThreshold_ZeroThreshold(t *testing.T) {
	logger := newMemoryTestLogger()
	config := workflow.MemoryConfig{
		Enabled:        true,
		ThresholdBytes: 0, // Zero means unlimited
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx := context.Background()
	heapAlloc, exceeded := monitor.CheckThreshold(ctx)

	// Zero threshold should never trigger, regardless of actual memory usage
	assert.False(t, exceeded, "zero threshold should never report exceeded")
	assert.GreaterOrEqual(t, heapAlloc, uint64(0), "heap allocation should be non-negative")
	assert.Empty(t, logger.warnings, "no warnings should be logged with zero threshold")
}

// TestMemoryMonitor_CheckThreshold_BelowThreshold tests behavior when memory usage is below threshold.
func TestMemoryMonitor_CheckThreshold_BelowThreshold(t *testing.T) {
	logger := newMemoryTestLogger()

	// Set threshold very high (100GB) - we'll definitely be below it
	config := workflow.MemoryConfig{
		Enabled:        true,
		ThresholdBytes: 107374182400, // 100GB
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx := context.Background()
	heapAlloc, exceeded := monitor.CheckThreshold(ctx)

	assert.False(t, exceeded, "should not exceed threshold when below it")
	assert.Greater(t, heapAlloc, uint64(0), "heap allocation should be positive")
	assert.Less(t, heapAlloc, uint64(107374182400), "heap allocation should be less than threshold")
	assert.Empty(t, logger.warnings, "no warnings should be logged when below threshold")
}

// TestMemoryMonitor_CheckThreshold_AboveThreshold tests behavior when memory usage exceeds threshold.
func TestMemoryMonitor_CheckThreshold_AboveThreshold(t *testing.T) {
	logger := newMemoryTestLogger()

	// Set threshold very low (1KB) - we'll definitely be above it
	config := workflow.MemoryConfig{
		Enabled:        true,
		ThresholdBytes: 1024, // 1KB - unrealistically low
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx := context.Background()
	heapAlloc, exceeded := monitor.CheckThreshold(ctx)

	assert.True(t, exceeded, "should exceed threshold when above it")
	assert.Greater(t, heapAlloc, uint64(1024), "heap allocation should be greater than 1KB threshold")
	assert.NotEmpty(t, logger.warnings, "warning should be logged when threshold exceeded")

	// Verify warning message mentions threshold exceeded
	foundThresholdWarning := false
	for _, warning := range logger.warnings {
		if strings.Contains(warning, "threshold") || strings.Contains(warning, "exceeded") {
			foundThresholdWarning = true
			break
		}
	}
	assert.True(t, foundThresholdWarning, "warning should mention threshold exceeded")
}

// TestMemoryMonitor_CheckThreshold_ExactlyAtThreshold tests boundary condition at threshold.
func TestMemoryMonitor_CheckThreshold_ExactlyAtThreshold(t *testing.T) {
	logger := newMemoryTestLogger()

	// Get current heap allocation
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	currentHeap := m.HeapAlloc

	// Set threshold to current heap (exactly at threshold)
	config := workflow.MemoryConfig{
		Enabled:        true,
		ThresholdBytes: int64(currentHeap), // #nosec G115 - test value, checked bounds
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx := context.Background()
	heapAlloc, exceeded := monitor.CheckThreshold(ctx)

	// At exact threshold, behavior should be consistent (likely not exceeded, or just barely exceeded)
	assert.GreaterOrEqual(t, heapAlloc, uint64(0), "heap allocation should be non-negative")

	// Either no warnings (if below/at threshold) or warnings (if slightly above due to GC timing)
	if exceeded {
		assert.NotEmpty(t, logger.warnings, "warnings should be present if threshold exceeded")
	} else {
		assert.Empty(t, logger.warnings, "no warnings if not exceeded")
	}
}

// TestMemoryMonitor_CheckThreshold_ContextCancellation tests behavior when context is canceled.
func TestMemoryMonitor_CheckThreshold_ContextCancellation(t *testing.T) {
	logger := newMemoryTestLogger()
	config := workflow.MemoryConfig{
		Enabled:        true,
		ThresholdBytes: 1024,
	}
	monitor := application.NewMemoryMonitor(config, logger)

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should handle gracefully even with canceled context
	heapAlloc, exceeded := monitor.CheckThreshold(ctx)

	// Should still return valid results despite canceled context
	assert.GreaterOrEqual(t, heapAlloc, uint64(0), "should return valid heap allocation")
	// exceeded might be true or false depending on actual memory
	_ = exceeded
}

// TestMemoryMonitor_LogMemoryStats tests logging of memory statistics.
func TestMemoryMonitor_LogMemoryStats(t *testing.T) {
	logger := newMemoryTestLogger()
	config := workflow.MemoryConfig{
		Enabled:        true,
		ThresholdBytes: 104857600, // 100MB
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx := context.Background()
	monitor.LogMemoryStats(ctx)

	// Should log info-level statistics
	assert.NotEmpty(t, logger.infos, "should log memory stats at INFO level")

	// Verify that some memory-related info was logged
	foundMemoryInfo := false
	for _, info := range logger.infos {
		if strings.Contains(strings.ToLower(info), "memory") ||
			strings.Contains(strings.ToLower(info), "heap") ||
			strings.Contains(strings.ToLower(info), "alloc") {
			foundMemoryInfo = true
			break
		}
	}
	assert.True(t, foundMemoryInfo, "logged info should mention memory/heap/alloc")
}

// TestMemoryMonitor_LogMemoryStats_DisabledMonitoring tests that stats logging works even when monitoring is disabled.
func TestMemoryMonitor_LogMemoryStats_DisabledMonitoring(t *testing.T) {
	logger := newMemoryTestLogger()
	config := workflow.MemoryConfig{
		Enabled:        false,
		ThresholdBytes: 0,
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx := context.Background()
	monitor.LogMemoryStats(ctx)

	// LogMemoryStats should work even if monitoring is disabled
	// It's a diagnostic tool that can be called explicitly
	assert.NotEmpty(t, logger.infos, "should log stats even when monitoring disabled")
}

// TestMemoryMonitor_StartPeriodicMonitoring_StopsOnContextCancel tests that periodic monitoring stops when context is canceled.
func TestMemoryMonitor_StartPeriodicMonitoring_StopsOnContextCancel(t *testing.T) {
	logger := newMemoryTestLogger()
	config := workflow.MemoryConfig{
		Enabled:        true,
		ThresholdBytes: 104857600, // 100MB
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx, cancel := context.WithCancel(context.Background())

	// Start periodic monitoring
	done := monitor.StartPeriodicMonitoring(ctx)
	require.NotNil(t, done, "done channel should not be nil")

	// Cancel context immediately
	cancel()

	// Wait for monitoring to stop (with timeout to prevent hanging)
	select {
	case <-done:
		// Success: monitoring stopped
	case <-time.After(2 * time.Second):
		t.Fatal("periodic monitoring did not stop within timeout after context cancellation")
	}
}

// TestMemoryMonitor_StartPeriodicMonitoring_DisabledMonitoring tests that periodic monitoring respects disabled config.
func TestMemoryMonitor_StartPeriodicMonitoring_DisabledMonitoring(t *testing.T) {
	logger := newMemoryTestLogger()
	config := workflow.MemoryConfig{
		Enabled:        false,
		ThresholdBytes: 1024,
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start periodic monitoring (should return immediately since disabled)
	done := monitor.StartPeriodicMonitoring(ctx)

	// Should complete immediately or very quickly when monitoring is disabled
	select {
	case <-done:
		// Expected: monitoring not started or stopped immediately
	case <-time.After(500 * time.Millisecond):
		// Also acceptable: may run but with no-op behavior
		cancel()
		<-done
	}

	// Should not log warnings when monitoring is disabled
	assert.Empty(t, logger.warnings, "no warnings should be logged when monitoring disabled")
}

// TestMemoryMonitor_StartPeriodicMonitoring_LogsWarningsWhenThresholdExceeded tests that periodic monitoring logs warnings.
func TestMemoryMonitor_StartPeriodicMonitoring_LogsWarningsWhenThresholdExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping periodic monitoring test in short mode")
	}

	logger := newMemoryTestLogger()
	config := workflow.MemoryConfig{
		Enabled:        true,
		ThresholdBytes: 1024, // Very low threshold - will definitely exceed
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start periodic monitoring
	done := monitor.StartPeriodicMonitoring(ctx)

	// Wait for monitoring to run at least once
	time.Sleep(200 * time.Millisecond)

	// Cancel and wait for completion
	cancel()
	<-done

	// Should have logged warnings about threshold exceeded
	assert.NotEmpty(t, logger.warnings, "should log warnings during periodic monitoring when threshold exceeded")
}

// TestMemoryMonitor_StartPeriodicMonitoring_MultipleMonitors tests running multiple monitors concurrently.
func TestMemoryMonitor_StartPeriodicMonitoring_MultipleMonitors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent monitor test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Create multiple monitors
	monitors := make([]*application.MemoryMonitor, 3)
	doneChannels := make([]<-chan struct{}, 3)

	for i := 0; i < 3; i++ {
		logger := newMemoryTestLogger()
		config := workflow.MemoryConfig{
			Enabled:        true,
			ThresholdBytes: 104857600, // 100MB
		}
		monitors[i] = application.NewMemoryMonitor(config, logger)
		doneChannels[i] = monitors[i].StartPeriodicMonitoring(ctx)
	}

	// Cancel context
	cancel()

	// Wait for all monitors to stop
	for i, done := range doneChannels {
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatalf("monitor %d did not stop within timeout", i)
		}
	}
}

// TestMemoryMonitor_StartPeriodicMonitoring_NoGoroutineLeak tests that periodic monitoring doesn't leak goroutines.
func TestMemoryMonitor_StartPeriodicMonitoring_NoGoroutineLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping goroutine leak test in short mode")
	}

	// Get baseline goroutine count
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	logger := newMemoryTestLogger()
	config := workflow.MemoryConfig{
		Enabled:        true,
		ThresholdBytes: 104857600,
	}
	monitor := application.NewMemoryMonitor(config, logger)

	ctx, cancel := context.WithCancel(context.Background())

	// Start and stop periodic monitoring
	done := monitor.StartPeriodicMonitoring(ctx)
	time.Sleep(100 * time.Millisecond) // Let it run briefly
	cancel()
	<-done

	// Allow time for goroutine cleanup
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Should not leak goroutines (allow small variance for test framework overhead)
	assert.InDelta(t, before, after, 2, "should not leak goroutines after periodic monitoring stops")
}

// TestMemoryMonitor_CheckThreshold_ReturnsActualMemoryUsage tests that CheckThreshold returns real memory values.
func TestMemoryMonitor_CheckThreshold_ReturnsActualMemoryUsage(t *testing.T) {
	logger := newMemoryTestLogger()
	config := workflow.MemoryConfig{
		Enabled:        true,
		ThresholdBytes: 104857600, // 100MB
	}
	monitor := application.NewMemoryMonitor(config, logger)

	// Get actual memory stats directly
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	directHeap := m.HeapAlloc

	ctx := context.Background()
	reportedHeap, _ := monitor.CheckThreshold(ctx)

	// Reported heap should be close to directly measured heap (allow some variance due to timing)
	// We expect them to be within 10MB of each other
	diff := int64(reportedHeap) - int64(directHeap) // #nosec G115 - test value, checked bounds
	if diff < 0 {
		diff = -diff
	}
	assert.Less(t, diff, int64(10*1024*1024), "reported heap should be close to actual heap allocation")
}
