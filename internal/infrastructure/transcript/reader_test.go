package transcript_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/transcript"
	infra "github.com/awf-project/cli/internal/infrastructure/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader_RoundTripsValidJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	// Write 100 events using the JSONL writer
	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)

	ctx := context.Background()
	written := make([]transcript.ExchangeEvent, 100)
	for i := range 100 {
		written[i] = transcript.ExchangeEvent{
			Seq:       uint64(i),
			RunID:     fmt.Sprintf("run-%d", i),
			Type:      transcript.EventTypeMessageUser,
			Path:      "/test/path",
			Iteration: i,
			Timestamp: time.Now(),
			Payload: &transcript.MessagePayload{
				Role: "user",
				Blocks: []transcript.ContentBlock{
					{
						Type:     transcript.BlockTypeText,
						Fidelity: transcript.FidelityRouter,
						Text:     fmt.Sprintf("Event %d", i),
					},
				},
			},
		}
		err := writer.Write(ctx, written[i])
		require.NoError(t, err)
	}
	writer.Close()

	// Read back using the reader
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	reader := infra.NewReader(file)
	require.NotNil(t, reader, "NewReader should return non-nil")

	read := make([]transcript.ExchangeEvent, 0, 100)
	for {
		event, err := reader.Read()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		read = append(read, event)
	}

	require.Equal(t, len(written), len(read), "should read back same number of events")
	for i := range written {
		assert.Equal(t, written[i].Seq, read[i].Seq)
		assert.Equal(t, written[i].RunID, read[i].RunID)
		assert.Equal(t, written[i].Type, read[i].Type)
	}
}

func TestReader_TolerantUnknownEventType(t *testing.T) {
	jsonLine := `{"seq":1,"run_id":"test-run","type":"future.thing","path":"/test","iteration":0,"timestamp":"2026-01-01T00:00:00Z","payload":null}`
	reader := infra.NewReader(strings.NewReader(jsonLine + "\n"))
	require.NotNil(t, reader)

	event, err := reader.Read()
	assert.NoError(t, err, "should not error on unknown event type")
	assert.Equal(t, transcript.EventType("future.thing"), event.Type, "should preserve unknown type verbatim")
	assert.Equal(t, uint64(1), event.Seq)
	assert.Equal(t, "test-run", event.RunID)
}

func TestReader_TolerantUnknownBlockType(t *testing.T) {
	// MessagePayload with a block that has an unknown type
	payload := `{
		"role":"user",
		"blocks":[
			{"type":"text","fidelity":"router","text":"hello"},
			{"type":"future.block.kind","fidelity":"router","data":"custom"}
		]
	}`

	jsonLine := fmt.Sprintf(
		`{"seq":1,"run_id":"test-run","type":"message.user","path":"/test","iteration":0,"timestamp":"2026-01-01T00:00:00Z","payload":%s}`,
		payload,
	)

	reader := infra.NewReader(strings.NewReader(jsonLine + "\n"))
	require.NotNil(t, reader)

	event, err := reader.Read()
	assert.NoError(t, err, "should not error on unknown block type inside payload")
	assert.Equal(t, transcript.EventTypeMessageUser, event.Type)

	msgPayload, ok := event.Payload.(*transcript.MessagePayload)
	require.True(t, ok, "payload should be MessagePayload")
	require.Equal(t, 2, len(msgPayload.Blocks))

	// First block is valid (should be properly decoded)
	assert.Equal(t, transcript.BlockTypeText, msgPayload.Blocks[0].Type)
	assert.Equal(t, "hello", msgPayload.Blocks[0].Text)

	// Second block has unknown type - should be preserved with type as unknown string
	// The reader should tolerate it and preserve the unknown type
	assert.Equal(t, transcript.BlockType("future.block.kind"), msgPayload.Blocks[1].Type)
}

func TestReader_MalformedLineReturnsError(t *testing.T) {
	truncatedJSON := `{"seq":1,"run_id":"test-run","type":"message.user","path":"/test","iteration":0,"timestamp":"2026-01-01T00:00:00Z","pay`

	reader := infra.NewReader(strings.NewReader(truncatedJSON + "\n"))
	require.NotNil(t, reader)

	event, err := reader.Read()
	assert.Error(t, err, "should return error for malformed JSON")
	assert.ErrorIs(t, err, infra.ErrLineMalformed, "should be ErrLineMalformed")
	assert.Equal(t, transcript.ExchangeEvent{}, event, "should return zero event on error")

	// Verify error message contains line number context
	if errMsg, ok := err.(interface{ Error() string }); ok {
		assert.Contains(t, errMsg.Error(), "1", "error should mention line number 1")
	}
}

func TestReader_EmptyLinesSkipped(t *testing.T) {
	jsonLine1 := `{"seq":1,"run_id":"run1","type":"message.user","path":"/test","iteration":0,"timestamp":"2026-01-01T00:00:00Z","payload":null}`
	jsonLine2 := `{"seq":2,"run_id":"run2","type":"message.user","path":"/test","iteration":0,"timestamp":"2026-01-01T00:00:00Z","payload":null}`

	input := jsonLine1 + "\n\n\n" + jsonLine2 + "\n\n"

	reader := infra.NewReader(strings.NewReader(input))
	require.NotNil(t, reader)

	// Read first event
	event1, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), event1.Seq)
	assert.Equal(t, "run1", event1.RunID)

	// Read second event (empty lines should be skipped)
	event2, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), event2.Seq)
	assert.Equal(t, "run2", event2.RunID)

	// EOF
	_, err = reader.Read()
	assert.Equal(t, io.EOF, err)
}

func TestReader_RespectsLargeLines(t *testing.T) {
	// Create a 1MB event
	largeText := strings.Repeat("x", 1<<20) // 1MB of text

	event := transcript.ExchangeEvent{
		Seq:       1,
		RunID:     "large-run",
		Type:      transcript.EventTypeMessageUser,
		Path:      "/test",
		Iteration: 0,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Payload: &transcript.MessagePayload{
			Role: "user",
			Blocks: []transcript.ContentBlock{
				{
					Type:     transcript.BlockTypeText,
					Fidelity: transcript.FidelityRouter,
					Text:     largeText,
				},
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(event)
	require.NoError(t, err)

	reader := infra.NewReader(bytes.NewReader(append(data, '\n')))
	require.NotNil(t, reader)

	readEvent, err := reader.Read()
	assert.NoError(t, err, "should handle large lines without error")
	assert.Equal(t, event.Seq, readEvent.Seq)
	assert.Equal(t, event.RunID, readEvent.RunID)
	assert.Equal(t, len(largeText), len(readEvent.Payload.(*transcript.MessagePayload).Blocks[0].Text))
}

func TestReader_ReadAll(t *testing.T) {
	jsonLine1 := `{"seq":1,"run_id":"run1","type":"message.user","path":"/test","iteration":0,"timestamp":"2026-01-01T00:00:00Z","payload":null}`
	jsonLine2 := `{"seq":2,"run_id":"run2","type":"message.assistant","path":"/test","iteration":0,"timestamp":"2026-01-01T00:00:00Z","payload":null}`
	jsonLine3 := `{"seq":3,"run_id":"run3","type":"tool.call","path":"/test","iteration":0,"timestamp":"2026-01-01T00:00:00Z","payload":null}`

	input := jsonLine1 + "\n" + jsonLine2 + "\n" + jsonLine3 + "\n"

	reader := infra.NewReader(strings.NewReader(input))
	require.NotNil(t, reader)

	events, err := reader.ReadAll()
	require.NoError(t, err)
	require.Equal(t, 3, len(events))

	assert.Equal(t, uint64(1), events[0].Seq)
	assert.Equal(t, uint64(2), events[1].Seq)
	assert.Equal(t, uint64(3), events[2].Seq)
}

func TestReader_ReadAllEmpty(t *testing.T) {
	reader := infra.NewReader(strings.NewReader(""))
	require.NotNil(t, reader)

	events, err := reader.ReadAll()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(events))
}

func TestReader_SequentialReads(t *testing.T) {
	jsonLine1 := `{"seq":1,"run_id":"run1","type":"message.user","path":"/test","iteration":0,"timestamp":"2026-01-01T00:00:00Z","payload":null}`
	jsonLine2 := `{"seq":2,"run_id":"run2","type":"message.assistant","path":"/test","iteration":0,"timestamp":"2026-01-01T00:00:00Z","payload":null}`

	input := jsonLine1 + "\n" + jsonLine2 + "\n"

	reader := infra.NewReader(strings.NewReader(input))
	require.NotNil(t, reader)

	event1, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), event1.Seq)

	event2, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), event2.Seq)

	_, err = reader.Read()
	assert.Equal(t, io.EOF, err)
}
