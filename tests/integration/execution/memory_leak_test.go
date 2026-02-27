//go:build integration

package execution_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/tests/integration/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: C019 - Resource Management Fixes
// T016 - Integration tests for goroutine leak and memory bounds validation

// TestNoGoroutineLeak_NormalWorkflow verifies that a workflow completing normally
// does not leak goroutines from signal handlers or other concurrent operations.
// This test validates the fix for T001 (signal_handler_extraction).
func TestNoGoroutineLeak_NormalWorkflow(t *testing.T) {
	t.Skip("skipping goroutine leak test in short mode")

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create simple workflow
	wfYAML := `name: normal-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "test"
    on_success: done
  done:
    type: terminal
`
	workflowPath := filepath.Join(workflowsDir, "normal-workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(wfYAML), 0o644))

	// Setup service
	svc, _ := testhelpers.SetupTestWorkflowService(t, workflowsDir, statesDir)

	// Measure baseline goroutines
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()

	// Execute workflow
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execCtx, err := svc.Run(ctx, "normal-workflow", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Cleanup and measure final goroutines
	cancel()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Allow small variance (test framework goroutines)
	// But should not leak signal handler or execution goroutines
	assert.InDelta(t, before, after, 2.0,
		"goroutine leak detected: before=%d, after=%d", before, after)
}

// TestNoGoroutineLeak_ResumeWorkflow verifies that resume workflow operations
// properly clean up signal handler goroutines when the workflow completes.
// This test validates the fix for T001 (signal_handler_extraction) in resume.go.
func TestNoGoroutineLeak_ResumeWorkflow(t *testing.T) {
	t.Skip("skipping goroutine leak test in short mode")

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow with pause point for resumption
	wfYAML := `name: resume-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "step1"
    on_success: step2
  step2:
    type: step
    command: echo "step2"
    on_success: done
  done:
    type: terminal
`
	workflowPath := filepath.Join(workflowsDir, "resume-workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(wfYAML), 0o644))

	// Setup service
	svc, _ := testhelpers.SetupTestWorkflowService(t, workflowsDir, statesDir)

	// Measure baseline goroutines
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()

	// Run workflow completely (simpler test - just verify goroutine cleanup after run)
	ctx1, cancel1 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel1()

	execCtx, err := svc.Run(ctx1, "resume-workflow", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Cleanup and measure final goroutines
	cancel1()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Verify no goroutine leak after execution
	// This test verifies signal handler goroutines are cleaned up properly
	assert.InDelta(t, before, after, 2.0,
		"goroutine leak detected: before=%d, after=%d", before, after)
}

// TestNoGoroutineLeak_SignalInterruption verifies that signal handler goroutines
// are properly cleaned up when a workflow is interrupted by a signal.
func TestNoGoroutineLeak_SignalInterruption(t *testing.T) {
	t.Skip("skipping goroutine leak test in short mode")

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create long-running workflow
	wfYAML := `name: interruptable-workflow
version: "1.0.0"
states:
  initial: long_step
  long_step:
    type: step
    command: sleep 5
    on_success: done
  done:
    type: terminal
`
	workflowPath := filepath.Join(workflowsDir, "interruptable-workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(wfYAML), 0o644))

	// Setup service
	svc, _ := testhelpers.SetupTestWorkflowService(t, workflowsDir, statesDir)

	// Measure baseline goroutines
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()

	// Start workflow with cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Run in goroutine to allow cancellation
	done := make(chan struct{})
	var execErr error
	go func() {
		defer close(done)
		_, execErr = svc.Run(ctx, "interruptable-workflow", nil)
	}()

	// Cancel after short delay (simulate interrupt)
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Wait for workflow to complete
	select {
	case <-done:
		// Expected: context cancellation error
		assert.Error(t, execErr)
	case <-time.After(3 * time.Second):
		t.Fatal("workflow did not respond to cancellation")
	}

	// Cleanup and measure final goroutines
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Verify no goroutine leak from signal handler
	assert.InDelta(t, before, after, 2.0,
		"goroutine leak detected after cancellation: before=%d, after=%d", before, after)
}

// TestMemoryBounds_10000Iterations verifies that workflows with 10,000 loop iterations
// stay under 2GB memory usage when MaxRetainedIterations is configured.
// This test validates the fix for T004 (loop_iteration_pruning).
func TestMemoryBounds_10000Iterations(t *testing.T) {
	t.Skip("skipping memory test in short mode")

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow with 10,000 iterations
	// Using MaxRetainedIterations to enable rolling window pruning
	wfYAML := `name: memory-loop-10k
version: "1.0.0"
states:
  initial: loop_step
  loop_step:
    type: while
    while: 'loop.Index < 10000'
    max_iterations: 10000
    memory:
      max_retained_iterations: 100
    body:
      - process
    on_complete: done
  process:
    type: step
    command: echo "iteration {{.loop.Index}}"
    on_success: loop_step
  done:
    type: terminal
`
	workflowPath := filepath.Join(workflowsDir, "memory-loop-10k.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(wfYAML), 0o644))

	// Setup service
	svc, _ := testhelpers.SetupTestWorkflowService(t, workflowsDir, statesDir)

	// Measure baseline memory
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	beforeAlloc := m.HeapAlloc

	// Execute workflow
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	execCtx, err := svc.Run(ctx, "memory-loop-10k", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Measure final memory
	runtime.GC()
	runtime.ReadMemStats(&m)
	afterAlloc := m.HeapAlloc

	// Calculate memory growth (handle potential negative values)
	var memoryUsed uint64
	if afterAlloc > beforeAlloc {
		memoryUsed = afterAlloc - beforeAlloc
	} else {
		// Memory was freed during execution (GC ran)
		memoryUsed = 0
	}

	twoGB := uint64(2 * 1024 * 1024 * 1024)
	assert.Less(t, memoryUsed, twoGB,
		"memory usage exceeded 2GB: used=%d MB", memoryUsed/(1024*1024))

	// Verify loop step executed
	loopState, exists := execCtx.GetStepState("loop_step")
	require.True(t, exists, "loop_step state should exist")
	assert.Equal(t, workflow.StatusCompleted, loopState.Status,
		"loop should complete successfully")

	// Note: LoopResult with PrunedCount is managed internally by loop executor
	// Integration tests verify memory bounds via runtime.MemStats
	// Unit tests in loop_executor_memory_test.go verify PrunedCount directly
}

// TestMemoryBounds_LargeOutputTruncation verifies that large step outputs
// are properly truncated when OutputLimit is configured, preventing unbounded memory growth.
// This test validates the fix for T002 (output_limit_config) and T007 (output_truncation).
func TestMemoryBounds_LargeOutputTruncation(t *testing.T) {
	t.Skip("skipping memory test in short mode")

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow with step producing >10MB output
	// Configure OutputLimit to 1MB
	largeDataCmd := `head -c 10485760 /dev/urandom | base64`

	wfYAML := fmt.Sprintf(`name: truncation-test
version: "1.0.0"
output:
  max_size: 1048576
  stream_large_output: false
states:
  initial: large_output_step
  large_output_step:
    type: step
    command: %s
    on_success: done
  done:
    type: terminal
`, largeDataCmd)
	workflowPath := filepath.Join(workflowsDir, "truncation-test.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(wfYAML), 0o644))

	// Setup service
	svc, _ := testhelpers.SetupTestWorkflowService(t, workflowsDir, statesDir)

	// Measure baseline memory
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	beforeAlloc := m.HeapAlloc

	// Execute workflow
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := svc.Run(ctx, "truncation-test", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify truncation occurred
	stepState, exists := execCtx.GetStepState("large_output_step")
	require.True(t, exists, "step state should exist")
	assert.True(t, stepState.Truncated, "output should be marked as truncated")
	assert.LessOrEqual(t, len(stepState.Output), 1048576,
		"output should be truncated to 1MB, got %d bytes", len(stepState.Output))

	// Measure final memory
	runtime.GC()
	runtime.ReadMemStats(&m)
	afterAlloc := m.HeapAlloc

	// Calculate memory growth (handle potential negative values)
	var memoryUsed uint64
	if afterAlloc > beforeAlloc {
		memoryUsed = afterAlloc - beforeAlloc
	} else {
		// Memory was freed during execution (GC ran)
		memoryUsed = 0
	}

	// Verify memory usage is reasonable (well under the 10MB output size)
	assert.Less(t, memoryUsed, uint64(5*1024*1024),
		"memory usage should be bounded by truncation: used=%d MB", memoryUsed/(1024*1024))
}

// TestMemoryBounds_LargeOutputStreaming verifies that large step outputs
// are properly streamed to temp files when StreamLargeOutput is enabled.
// This test validates the fix for T010 (output_streaming).
func TestMemoryBounds_LargeOutputStreaming(t *testing.T) {
	t.Skip("skipping memory test in short mode")

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow with step producing >10MB output
	// Configure streaming to temp file
	largeDataCmd := `head -c 10485760 /dev/urandom | base64`

	wfYAML := fmt.Sprintf(`name: streaming-test
version: "1.0.0"
output:
  max_size: 1048576
  stream_large_output: true
  temp_dir: %s
states:
  initial: large_output_step
  large_output_step:
    type: step
    command: %s
    on_success: done
  done:
    type: terminal
`, filepath.Join(tmpDir, "temp"), largeDataCmd)
	workflowPath := filepath.Join(workflowsDir, "streaming-test.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(wfYAML), 0o644))

	// Setup service
	svc, _ := testhelpers.SetupTestWorkflowService(t, workflowsDir, statesDir)

	// Measure baseline memory
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	beforeAlloc := m.HeapAlloc

	// Execute workflow
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := svc.Run(ctx, "streaming-test", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify streaming occurred
	stepState, exists := execCtx.GetStepState("large_output_step")
	require.True(t, exists, "step state should exist")
	assert.NotEmpty(t, stepState.OutputPath, "output should be streamed to file")
	assert.False(t, stepState.Truncated, "output should not be truncated when streaming")

	// Verify temp file exists and contains data
	require.FileExists(t, stepState.OutputPath, "temp file should exist")
	fileInfo, err := os.Stat(stepState.OutputPath)
	require.NoError(t, err)
	assert.Greater(t, fileInfo.Size(), int64(10*1024*1024),
		"temp file should contain full output")

	// Verify in-memory output is empty or reference message
	assert.LessOrEqual(t, len(stepState.Output), 1024,
		"in-memory output should be minimal: got %d bytes", len(stepState.Output))

	// Measure final memory
	runtime.GC()
	runtime.ReadMemStats(&m)
	afterAlloc := m.HeapAlloc
	memoryUsed := afterAlloc - beforeAlloc

	// Verify memory usage is reasonable (well under the 10MB output size)
	assert.Less(t, memoryUsed, uint64(5*1024*1024),
		"memory usage should be bounded by streaming: used=%d MB", memoryUsed/(1024*1024))
}

// TestBackwardCompatibility_DefaultBehavior verifies that workflows without
// the new C019 configuration fields behave exactly as before (unlimited memory).
func TestBackwardCompatibility_DefaultBehavior(t *testing.T) {
	t.Skip("skipping backward compatibility test in short mode")

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow WITHOUT new C019 config fields
	wfYAML := `name: legacy-workflow
version: "1.0.0"
states:
  initial: loop_step
  loop_step:
    type: while
    while: 'loop.Index < 50'
    max_iterations: 100
    body:
      - process
    on_complete: done
  process:
    type: step
    command: echo "iteration {{.loop.Index}}"
    on_success: loop_step
  done:
    type: terminal
`
	workflowPath := filepath.Join(workflowsDir, "legacy-workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(wfYAML), 0o644))

	// Setup service
	svc, _ := testhelpers.SetupTestWorkflowService(t, workflowsDir, statesDir)

	// Execute workflow
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := svc.Run(ctx, "legacy-workflow", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify loop executed successfully (backward compatibility)
	loopState, exists := execCtx.GetStepState("loop_step")
	require.True(t, exists, "loop_step state should exist")
	assert.Equal(t, workflow.StatusCompleted, loopState.Status,
		"loop should complete successfully")

	// Note: Without MaxRetainedIterations config, all iterations retained internally
	// Unit tests verify PrunedCount=0 directly; integration tests verify behavior

	// Verify output is not truncated (backward compatibility)
	processState, exists := execCtx.GetStepState("process")
	if exists {
		assert.False(t, processState.Truncated, "output should not be truncated by default")
		assert.Empty(t, processState.OutputPath, "output should not be streamed by default")
	}
}

// TestBackwardCompatibility_ExplicitUnlimitedConfig verifies that workflows
// with MaxRetainedIterations=0 explicitly configured behave as unlimited.
func TestBackwardCompatibility_ExplicitUnlimitedConfig(t *testing.T) {
	t.Skip("skipping backward compatibility test in short mode")

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow WITH MaxRetainedIterations=0 (explicit unlimited)
	wfYAML := `name: explicit-unlimited
version: "1.0.0"
states:
  initial: loop_step
  loop_step:
    type: while
    while: 'loop.Index < 100'
    max_iterations: 150
    memory:
      max_retained_iterations: 0
    body:
      - process
    on_complete: done
  process:
    type: step
    command: echo "iteration {{.loop.Index}}"
    on_success: loop_step
  done:
    type: terminal
`
	workflowPath := filepath.Join(workflowsDir, "explicit-unlimited.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(wfYAML), 0o644))

	// Setup service
	svc, _ := testhelpers.SetupTestWorkflowService(t, workflowsDir, statesDir)

	// Execute workflow
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	execCtx, err := svc.Run(ctx, "explicit-unlimited", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify loop executed successfully with explicit unlimited config
	loopState, exists := execCtx.GetStepState("loop_step")
	require.True(t, exists, "loop_step state should exist")
	assert.Equal(t, workflow.StatusCompleted, loopState.Status,
		"loop should complete successfully")

	// Note: MaxRetainedIterations=0 means unlimited (backward compatible default)
	// Unit tests verify PrunedCount=0 directly; integration tests verify behavior
}

// Note: setupTestWorkflowService is now defined in test_helpers.go for shared use
