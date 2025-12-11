package workflow

import "time"

// ExecutionRecord represents a completed workflow execution for history tracking.
// It captures essential metadata for reporting and analysis.
type ExecutionRecord struct {
	ID           string
	WorkflowID   string
	WorkflowName string
	Status       string // success, failed, cancelled
	ExitCode     int
	StartedAt    time.Time
	CompletedAt  time.Time
	DurationMs   int64
	ErrorMessage string
}

// HistoryFilter defines criteria for querying execution history.
type HistoryFilter struct {
	WorkflowName string
	Status       string
	Since        time.Time
	Until        time.Time
	Limit        int
}

// HistoryStats contains aggregated execution statistics.
type HistoryStats struct {
	TotalExecutions int
	SuccessCount    int
	FailedCount     int
	CancelledCount  int
	AvgDurationMs   int64
}
