package transcript

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type EventType string

const (
	EventTypeRunStarted                EventType = "run.started"
	EventTypeRunCompleted              EventType = "run.completed"
	EventTypeStepStarted               EventType = "step.started"
	EventTypeStepCompleted             EventType = "step.completed"
	EventTypeStepCallWorkflowStarted   EventType = "step.call_workflow.started"
	EventTypeStepCallWorkflowCompleted EventType = "step.call_workflow.completed"
	EventTypeMessageUser               EventType = "message.user"
	EventTypeMessageAssistant          EventType = "message.assistant"
	EventTypeToolCall                  EventType = "tool.call"
	EventTypeToolResult                EventType = "tool.result"
)

var ErrUnknownEventType = errors.New("unknown event type")

type ExchangeEvent struct {
	Seq         uint64    `json:"seq"`
	RunID       string    `json:"run_id"`
	ParentRunID string    `json:"parent_run_id,omitempty"`
	ChildRunID  string    `json:"child_run_id,omitempty"`
	Type        EventType `json:"type"`
	Path        string    `json:"path"`
	Iteration   int       `json:"iteration"`
	Timestamp   time.Time `json:"timestamp"`
	Payload     any       `json:"payload"`
}

func (e ExchangeEvent) MarshalJSON() ([]byte, error) { //nolint:gocritic // hugeParam: value receiver required so json.Marshal(event) invokes custom marshaler
	type wire struct {
		Seq         uint64    `json:"seq"`
		RunID       string    `json:"run_id"`
		ParentRunID string    `json:"parent_run_id,omitempty"`
		ChildRunID  string    `json:"child_run_id,omitempty"`
		Type        EventType `json:"type"`
		Path        string    `json:"path"`
		Iteration   int       `json:"iteration"`
		Timestamp   time.Time `json:"timestamp"`
		Payload     any       `json:"payload"`
	}
	w := wire(e) //nolint:govet // wire has identical field layout; conversion is safe
	data, err := json.Marshal(w)
	if err != nil {
		return nil, fmt.Errorf("marshaling exchange event: %w", err)
	}
	return data, nil
}

func (e *ExchangeEvent) UnmarshalJSON(data []byte) error {
	type rawEvent struct {
		Seq         uint64          `json:"seq"`
		RunID       string          `json:"run_id"`
		ParentRunID string          `json:"parent_run_id,omitempty"`
		ChildRunID  string          `json:"child_run_id,omitempty"`
		Type        EventType       `json:"type"`
		Path        string          `json:"path"`
		Iteration   int             `json:"iteration"`
		Timestamp   time.Time       `json:"timestamp"`
		Payload     json.RawMessage `json:"payload"`
	}

	var raw rawEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decoding exchange event: %w", err)
	}

	if !validEventType(raw.Type) {
		return fmt.Errorf("%w: %s", ErrUnknownEventType, raw.Type)
	}

	e.Seq = raw.Seq
	e.RunID = raw.RunID
	e.ParentRunID = raw.ParentRunID
	e.ChildRunID = raw.ChildRunID
	e.Type = raw.Type
	e.Path = raw.Path
	e.Iteration = raw.Iteration
	e.Timestamp = raw.Timestamp

	if len(raw.Payload) == 0 || string(raw.Payload) == "null" {
		e.Payload = nil
		return nil
	}

	payload, err := dispatchPayload(raw.Payload)
	if err != nil {
		return fmt.Errorf("decoding payload: %w", err)
	}
	e.Payload = payload
	return nil
}

func validEventType(et EventType) bool {
	switch et {
	case EventTypeRunStarted,
		EventTypeRunCompleted,
		EventTypeStepStarted,
		EventTypeStepCompleted,
		EventTypeStepCallWorkflowStarted,
		EventTypeStepCallWorkflowCompleted,
		EventTypeMessageUser,
		EventTypeMessageAssistant,
		EventTypeToolCall,
		EventTypeToolResult:
		return true
	default:
		return false
	}
}

func dispatchPayload(raw json.RawMessage) (any, error) {
	if len(raw) > 0 && raw[0] == '[' {
		var blocks []ContentBlock
		if err := json.Unmarshal(raw, &blocks); err != nil {
			return nil, fmt.Errorf("decoding content block array: %w", err)
		}
		return blocks, nil
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		var v any
		if err2 := json.Unmarshal(raw, &v); err2 != nil {
			return nil, fmt.Errorf("decoding payload value: %w", err2)
		}
		return v, nil
	}

	if probe["role"] != nil {
		var p MessagePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("decoding message payload: %w", err)
		}
		return &p, nil
	}

	if probe["call_id"] != nil {
		var p ToolPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("decoding tool payload: %w", err)
		}
		return &p, nil
	}

	if probe["kind"] != nil {
		var p StepPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("decoding step payload: %w", err)
		}
		return &p, nil
	}

	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("decoding generic payload: %w", err)
	}
	return v, nil
}
