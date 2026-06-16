package api

import (
	"context"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"github.com/awf-project/cli/internal/domain/ports"
)

// HistoryHandlers exposes execution history query operations via HTTP.
//
// Listing routes through ports.WorkflowFacade.History and statistics through the focused
// ports.WorkflowReader.HistoryStats — both satisfied by the same application.Adapter — so the
// HTTP history endpoints share the common facade rather than a legacy Bridge port (F108).
type HistoryHandlers struct {
	facade ports.WorkflowFacade
	reader ports.WorkflowReader
}

// NewHistoryHandlers creates a HistoryHandlers bound to the facade and reader ports.
// Either may be nil; the handlers degrade to 503 rather than panicking.
func NewHistoryHandlers(facade ports.WorkflowFacade, reader ports.WorkflowReader) *HistoryHandlers {
	return &HistoryHandlers{facade: facade, reader: reader}
}

func (h *HistoryHandlers) List(ctx context.Context, in *HistoryListInput) (*HistoryListOutput, error) {
	if h.facade == nil {
		return nil, huma.Error503ServiceUnavailable("history is temporarily unavailable")
	}
	records, err := h.facade.History(ctx, buildHistoryFilter(in))
	if err != nil {
		slog.Error("list history: internal error", slog.Any("error", err))
		return nil, huma.Error500InternalServerError("failed to list history")
	}
	entries := make([]HistoryEntry, 0, len(records))
	for i := range records {
		r := &records[i]
		entries = append(entries, HistoryEntry{
			ID:           r.RunID,
			WorkflowName: r.WorkflowName,
			Status:       string(r.Status),
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
	if h.reader == nil {
		return nil, huma.Error503ServiceUnavailable("history is temporarily unavailable")
	}
	stats, err := h.reader.HistoryStats(ctx, buildHistoryFilter(in))
	if err != nil {
		slog.Error("history stats: internal error", slog.Any("error", err))
		return nil, huma.Error500InternalServerError("failed to compute history stats")
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

// buildHistoryFilter maps the HTTP query input to the facade history filter DTO.
func buildHistoryFilter(in *HistoryListInput) ports.HistoryFilter {
	return ports.HistoryFilter{
		WorkflowName: in.Workflow,
		Status:       in.Status,
		Since:        in.Since,
		Until:        in.Until,
		Limit:        in.Limit,
	}
}
