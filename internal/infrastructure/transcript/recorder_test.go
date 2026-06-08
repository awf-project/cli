package transcript

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
)

func TestRecorder_NewRecorder_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	rec, err := NewRecorder(path)
	require.NoError(t, err)
	require.NotNil(t, rec)

	err = rec.Close()
	assert.NoError(t, err)
}

func TestRecorder_InvalidEventRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	rec, err := NewRecorder(path)
	require.NoError(t, err)

	ctx := context.Background()
	zeroEvent := transcript.ExchangeEvent{}
	err = rec.Record(ctx, zeroEvent)
	assert.ErrorIs(t, err, ports.ErrInvalidEvent)

	err = rec.Close()
	assert.NoError(t, err)

	// Verify file was not written
	data, err := os.ReadFile(path)
	assert.True(t, os.IsNotExist(err), "file should not exist after invalid event rejected")
	assert.Empty(t, data)
}

func TestRecorder_RecordWritesThenBroadcasts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	rec, err := NewRecorder(path)
	require.NoError(t, err)

	// Subscribe before recording
	ch, cancel := rec.Subscribe()
	defer cancel()

	ctx := context.Background()
	event := transcript.ExchangeEvent{
		RunID:     "test-run-123",
		Type:      transcript.EventTypeRunStarted,
		Timestamp: time.Now(),
	}

	err = rec.Record(ctx, event)
	assert.NoError(t, err)

	// Verify broadcast to subscriber
	select {
	case received := <-ch:
		assert.NotZero(t, received.Seq)
		assert.Equal(t, event.RunID, received.RunID)
		assert.Equal(t, event.Type, received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("event not received by subscriber within timeout")
	}

	// Verify file was written
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	assert.True(t, scanner.Scan(), "expected at least one line in file")

	var readEvent transcript.ExchangeEvent
	err = json.Unmarshal(scanner.Bytes(), &readEvent)
	require.NoError(t, err)
	assert.NotZero(t, readEvent.Seq)
	assert.Equal(t, event.RunID, readEvent.RunID)

	err = rec.Close()
	assert.NoError(t, err)
}

func TestRecorder_RecordWriterFailureDoesNotBroadcast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "subdir", "test.jsonl")

	rec, err := NewRecorder(path)
	require.NoError(t, err)

	// Subscribe before recording
	ch, cancel := rec.Subscribe()
	defer cancel()

	ctx := context.Background()
	event := transcript.ExchangeEvent{
		RunID:     "test-run-456",
		Type:      transcript.EventTypeStepStarted,
		Timestamp: time.Now(),
	}

	// Recording should fail because we can't write to nonexistent path
	err = rec.Record(ctx, event)
	assert.Error(t, err)

	// Subscriber should not receive anything
	select {
	case <-ch:
		t.Fatal("subscriber should not receive event when writer fails")
	case <-time.After(50 * time.Millisecond):
		// Expected: no event received
	}

	rec.Close()
}

func TestRecorder_SeqMonotonicConcurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.jsonl")

	rec, err := NewRecorder(path)
	require.NoError(t, err)

	numGoroutines := 64
	callsPerGoroutine := 1000
	var wg sync.WaitGroup

	for g := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			ctx := context.Background()
			for i := range callsPerGoroutine {
				event := transcript.ExchangeEvent{
					RunID:     "concurrent-test",
					Type:      transcript.EventTypeMessageUser,
					Iteration: goroutineID*1000 + i,
					Timestamp: time.Now(),
				}
				err := rec.Record(ctx, event)
				assert.NoError(t, err, "goroutine %d iteration %d", goroutineID, i)
			}
		}(g)
	}

	wg.Wait()
	err = rec.Close()
	require.NoError(t, err)

	// Read file and verify Seq is monotonic 1..64000 with no duplicates and no gaps
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	seqMap := make(map[uint64]bool)
	maxSeq := uint64(0)
	lineCount := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineCount++
		var event transcript.ExchangeEvent
		err := json.Unmarshal(scanner.Bytes(), &event)
		require.NoError(t, err)

		assert.NotZero(t, event.Seq, "Seq must be allocated")
		seqMap[event.Seq] = true

		if event.Seq > maxSeq {
			maxSeq = event.Seq
		}
	}
	require.NoError(t, scanner.Err())

	expectedLineCount := numGoroutines * callsPerGoroutine
	assert.Equal(t, expectedLineCount, lineCount)

	// Verify no duplicates and no gaps
	for i := uint64(1); i <= maxSeq; i++ {
		assert.True(t, seqMap[i], "Seq %d missing (duplicate or gap detected)", i)
	}
}

func TestRecorder_IdempotentClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	rec, err := NewRecorder(path)
	require.NoError(t, err)

	// First close
	err = rec.Close()
	assert.NoError(t, err)

	// Second close should also return nil
	err = rec.Close()
	assert.NoError(t, err)

	// Third close for good measure
	err = rec.Close()
	assert.NoError(t, err)
}

func TestRecorder_MaskerHookApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "masked.jsonl")

	// Masker that uppercases the RunID
	uppercaseMasker := func(event transcript.ExchangeEvent) transcript.ExchangeEvent {
		event.RunID = strings.ToUpper(event.RunID)
		return event
	}

	rec, err := NewRecorder(path, WithMasker(uppercaseMasker))
	require.NoError(t, err)

	// Subscribe to capture broadcast
	ch, cancel := rec.Subscribe()
	defer cancel()

	ctx := context.Background()
	event := transcript.ExchangeEvent{
		RunID:     "lowercase-test",
		Type:      transcript.EventTypeMessageAssistant,
		Timestamp: time.Now(),
	}

	err = rec.Record(ctx, event)
	assert.NoError(t, err)

	// Verify subscriber receives masked event
	select {
	case received := <-ch:
		assert.Equal(t, "LOWERCASE-TEST", received.RunID, "subscriber should see masked event")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("event not received by subscriber within timeout")
	}

	// Verify file contains masked event
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	assert.True(t, scanner.Scan())

	var fileEvent transcript.ExchangeEvent
	err = json.Unmarshal(scanner.Bytes(), &fileEvent)
	require.NoError(t, err)
	assert.Equal(t, "LOWERCASE-TEST", fileEvent.RunID, "file should contain masked event")

	err = rec.Close()
	assert.NoError(t, err)
}

func TestRecorder_WithFanOutBufferSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	rec, err := NewRecorder(path, WithFanOutBufferSize(512))
	require.NoError(t, err)
	require.NotNil(t, rec)

	err = rec.Close()
	assert.NoError(t, err)
}

func TestRecorder_WithRecorderLogger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	logger := ports.NopLogger{}
	rec, err := NewRecorder(path, WithRecorderLogger(logger))
	require.NoError(t, err)
	require.NotNil(t, rec)

	err = rec.Close()
	assert.NoError(t, err)
}

func TestRecorder_SequenceAllocation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	rec, err := NewRecorder(path)
	require.NoError(t, err)

	ctx := context.Background()

	// Record three events
	event1 := transcript.ExchangeEvent{
		RunID: "seq-test",
		Type:  transcript.EventTypeRunStarted,
	}
	err = rec.Record(ctx, event1)
	assert.NoError(t, err)

	event2 := transcript.ExchangeEvent{
		RunID: "seq-test",
		Type:  transcript.EventTypeStepStarted,
	}
	err = rec.Record(ctx, event2)
	assert.NoError(t, err)

	event3 := transcript.ExchangeEvent{
		RunID: "seq-test",
		Type:  transcript.EventTypeStepCompleted,
	}
	err = rec.Record(ctx, event3)
	assert.NoError(t, err)

	err = rec.Close()
	require.NoError(t, err)

	// Read file and verify Seq allocation (1, 2, 3)
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	var seqs []uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event transcript.ExchangeEvent
		err := json.Unmarshal(scanner.Bytes(), &event)
		require.NoError(t, err)
		seqs = append(seqs, event.Seq)
	}
	require.NoError(t, scanner.Err())

	require.Len(t, seqs, 3)
	assert.Equal(t, uint64(1), seqs[0])
	assert.Equal(t, uint64(2), seqs[1])
	assert.Equal(t, uint64(3), seqs[2])
}

func TestRecorder_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	rec, err := NewRecorder(path)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := transcript.ExchangeEvent{
		RunID: "cancel-test",
		Type:  transcript.EventTypeRunStarted,
	}

	err = rec.Record(ctx, event)
	assert.Error(t, err)

	err = rec.Close()
	assert.NoError(t, err)
}

func TestRecorder_Subscribe_MultipleSubscribers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	rec, err := NewRecorder(path)
	require.NoError(t, err)

	// Create three subscribers
	ch1, cancel1 := rec.Subscribe()
	defer cancel1()

	ch2, cancel2 := rec.Subscribe()
	defer cancel2()

	ch3, cancel3 := rec.Subscribe()
	defer cancel3()

	ctx := context.Background()
	event := transcript.ExchangeEvent{
		RunID: "multi-sub",
		Type:  transcript.EventTypeRunStarted,
	}

	err = rec.Record(ctx, event)
	assert.NoError(t, err)

	// All subscribers should receive the event
	timeout := 100 * time.Millisecond
	for i, ch := range []<-chan transcript.ExchangeEvent{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			assert.Equal(t, event.RunID, received.RunID, "subscriber %d", i)
		case <-time.After(timeout):
			t.Fatalf("subscriber %d did not receive event", i)
		}
	}

	err = rec.Close()
	assert.NoError(t, err)
}
