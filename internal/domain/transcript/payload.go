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
}

type ToolPayload struct {
	Name     string   `json:"name"`
	CallID   string   `json:"call_id"`
	Input    any      `json:"input"`
	Output   any      `json:"output"`
	Error    string   `json:"error,omitempty"`
	Fidelity Fidelity `json:"fidelity"`
}
