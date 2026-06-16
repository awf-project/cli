package transcript

type MessagePayload struct {
	Role   string         `json:"role"`
	Blocks []ContentBlock `json:"blocks"`
}

type StepPayload struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Error  string `json:"error,omitempty"`
	Result any    `json:"result,omitempty"`
	// Output and Stderr carry the step's captured stdout/stderr for live, event-only
	// consumers (TUI monitoring, SSE) that have no ExecutionContext to poll. They are
	// json:"-" on purpose: stdout is intentionally NOT persisted in the canonical
	// transcript (F106) — it is streamed out-of-band — so these fields ride only on
	// the in-memory event broadcast to subscribers and are never written to the JSONL.
	Output string `json:"-"`
	Stderr string `json:"-"`
}

type ToolPayload struct {
	Name     string   `json:"name"`
	CallID   string   `json:"call_id"`
	Input    any      `json:"input"`
	Output   any      `json:"output"`
	Error    string   `json:"error,omitempty"`
	Fidelity Fidelity `json:"fidelity"`
}
