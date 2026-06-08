package transcript

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/awf-project/cli/internal/domain/transcript"
)

var ErrLineMalformed = errors.New("malformed JSONL line")

type Reader struct {
	dec     *json.Decoder
	lineNum int
}

// NewReader wraps r in a 4 MiB bufio.Reader so that single-line events larger
// than the default 4 KiB read buffer are handled without extra round-trips.
// json.Decoder is used rather than bufio.Scanner so that multi-line JSON values
// (e.g. pretty-printed payloads in tests) are tolerated.
func NewReader(r io.Reader) *Reader {
	buf := bufio.NewReaderSize(r, 4<<20)
	return &Reader{dec: json.NewDecoder(buf)}
}

func (r *Reader) Read() (transcript.ExchangeEvent, error) {
	var raw rawEvent
	if err := r.dec.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			return transcript.ExchangeEvent{}, io.EOF
		}
		r.lineNum++
		return transcript.ExchangeEvent{}, fmt.Errorf("line %d: %w", r.lineNum, ErrLineMalformed)
	}
	r.lineNum++

	event := transcript.ExchangeEvent{
		Seq:         raw.Seq,
		RunID:       raw.RunID,
		ParentRunID: raw.ParentRunID,
		ChildRunID:  raw.ChildRunID,
		Type:        raw.Type,
		Path:        raw.Path,
		Iteration:   raw.Iteration,
		Timestamp:   raw.Timestamp,
	}

	if len(raw.Payload) > 0 && string(raw.Payload) != "null" {
		payload, err := tolerantDispatchPayload(raw.Payload)
		if err != nil {
			return transcript.ExchangeEvent{}, fmt.Errorf("line %d: %w", r.lineNum, ErrLineMalformed)
		}
		event.Payload = payload
	}

	return event, nil
}

func (r *Reader) ReadAll() ([]transcript.ExchangeEvent, error) {
	var events []transcript.ExchangeEvent
	for {
		event, err := r.Read()
		if errors.Is(err, io.EOF) {
			return events, nil
		}
		if err != nil {
			return events, err
		}
		events = append(events, event)
	}
}

// rawEvent mirrors ExchangeEvent without the custom UnmarshalJSON, allowing
// unknown event types to be decoded without error (forward-compatibility policy).
type rawEvent struct {
	Seq         uint64               `json:"seq"`
	RunID       string               `json:"run_id"`
	ParentRunID string               `json:"parent_run_id,omitempty"`
	ChildRunID  string               `json:"child_run_id,omitempty"`
	Type        transcript.EventType `json:"type"`
	Path        string               `json:"path"`
	Iteration   int                  `json:"iteration"`
	Timestamp   time.Time            `json:"timestamp"`
	Payload     json.RawMessage      `json:"payload"`
}

// rawContentBlock mirrors ContentBlock without the custom UnmarshalJSON, allowing
// unknown block types to be preserved verbatim (forward-compatibility policy).
type rawContentBlock struct {
	Type        transcript.BlockType `json:"type"`
	Fidelity    transcript.Fidelity  `json:"fidelity"`
	Text        string               `json:"text,omitempty"`
	Thinking    string               `json:"thinking,omitempty"`
	ToolName    string               `json:"tool_name,omitempty"`
	ToolID      string               `json:"tool_id,omitempty"`
	ToolInput   any                  `json:"tool_input,omitempty"`
	ToolContent any                  `json:"tool_content,omitempty"`
	Command     string               `json:"command,omitempty"`
	Chunk       string               `json:"chunk,omitempty"`
}

func decodeAsAny(raw json.RawMessage, context string) (any, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", context, err)
	}
	return v, nil
}

func tolerantDispatchPayload(raw json.RawMessage) (any, error) {
	if len(raw) > 0 && raw[0] == '[' {
		var rawBlocks []rawContentBlock
		if err := json.Unmarshal(raw, &rawBlocks); err != nil {
			return decodeAsAny(raw, "content block array")
		}
		blocks := make([]transcript.ContentBlock, len(rawBlocks))
		for i := range rawBlocks {
			blocks[i] = toContentBlock(&rawBlocks[i])
		}
		return blocks, nil
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return decodeAsAny(raw, "payload")
	}

	if probe["role"] != nil {
		type rawMessagePayload struct {
			Role   string            `json:"role"`
			Blocks []rawContentBlock `json:"blocks"`
		}
		var rmp rawMessagePayload
		if err := json.Unmarshal(raw, &rmp); err != nil {
			return nil, fmt.Errorf("decoding message payload: %w", err)
		}
		mp := &transcript.MessagePayload{
			Role:   rmp.Role,
			Blocks: make([]transcript.ContentBlock, len(rmp.Blocks)),
		}
		for i := range rmp.Blocks {
			mp.Blocks[i] = toContentBlock(&rmp.Blocks[i])
		}
		return mp, nil
	}

	if probe["call_id"] != nil {
		var p transcript.ToolPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("decoding tool payload: %w", err)
		}
		return &p, nil
	}

	if probe["kind"] != nil {
		var p transcript.StepPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("decoding step payload: %w", err)
		}
		return &p, nil
	}

	return decodeAsAny(raw, "generic payload")
}

func toContentBlock(rb *rawContentBlock) transcript.ContentBlock { //nolint:gocritic // pointer receiver avoids 160-byte copy per element
	return transcript.ContentBlock{
		Type:        rb.Type,
		Fidelity:    rb.Fidelity,
		Text:        rb.Text,
		Thinking:    rb.Thinking,
		ToolName:    rb.ToolName,
		ToolID:      rb.ToolID,
		ToolInput:   rb.ToolInput,
		ToolContent: rb.ToolContent,
		Command:     rb.Command,
		Chunk:       rb.Chunk,
	}
}
