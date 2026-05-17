package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// HistoryHandlers exposes execution history query operations via HTTP.
type HistoryHandlers struct {
	b *Bridge
}

// NewHistoryHandlers creates a HistoryHandlers bound to the given Bridge.
func NewHistoryHandlers(b *Bridge) *HistoryHandlers {
	return &HistoryHandlers{b: b}
}

func (h *HistoryHandlers) List(ctx context.Context, in *HistoryListInput) (*HistoryListOutput, error) {
	filter := buildHistoryFilter(in)
	records, err := h.b.history.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	entries := make([]HistoryEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, HistoryEntry{
			ID:           r.ID,
			WorkflowName: r.WorkflowName,
			Status:       r.Status,
			StartedAt:    r.StartedAt,
			CompletedAt:  r.CompletedAt,
			DurationMs:   r.DurationMs,
		})
	}
	out := &HistoryListOutput{}
	out.Body.Body = historyListBody{Entries: entries}
	return out, nil
}

func (h *HistoryHandlers) Stats(ctx context.Context, in *HistoryListInput) (*HistoryStatsOutput, error) {
	filter := buildHistoryFilter(in)
	stats, err := h.b.history.GetStats(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := &HistoryStatsOutput{}
	out.Body.Body = stats
	return out, nil
}

// RegisterHistoryRoutes mounts the history list and stats routes on the given Huma API.
func RegisterHistoryRoutes(api huma.API, h *HistoryHandlers) {
	huma.Register(api, huma.Operation{
		Method:      "GET",
		Path:        "/api/history",
		OperationID: "list-history",
		Tags:        []string{"History"},
	}, h.List)

	huma.Register(api, huma.Operation{
		Method:      "GET",
		Path:        "/api/history/stats",
		OperationID: "history-stats",
		Tags:        []string{"History"},
	}, h.Stats)
}

func buildHistoryFilter(in *HistoryListInput) *workflow.HistoryFilter {
	return &workflow.HistoryFilter{
		WorkflowName: in.Workflow,
		Status:       in.Status,
		Since:        in.Since,
		Until:        in.Until,
		Limit:        in.Limit,
	}
}
