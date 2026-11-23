package transcript

type MessagePayload struct {
	Role   string         `json:"role"`
	Blocks []ContentBlock `json:"blocks"`
}

type StepPayload struct {
	// Status is the execution outcome; step.completed marks termination, not success.
	Status string `json:"status,omitempty"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Error  string `json:"error,omitempty"`
	Result any    `json:"result,omitempty"`
	// CostUSD is what the provider charged for an agent step, in USD. Zero when the
	// provider reports no price (shell steps, HTTP providers without billing data).
	CostUSD float64 `json:"cost_usd,omitempty"`
	// TokensIn, TokensOut and TokensTotal are the agent step's token counts. Zero
	// when the provider reports none (shell steps) or reports only a total.
	TokensIn    int `json:"tokens_in,omitempty"`
	TokensOut   int `json:"tokens_out,omitempty"`
	TokensTotal int `json:"tokens_total,omitempty"`
	// TokensEstimated marks the counts as an approximation rather than a provider
	// measurement.
	TokensEstimated bool `json:"tokens_estimated,omitempty"`
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
