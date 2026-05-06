package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/awf-project/cli/pkg/plugin/sdk"
)

// EventLoggerPlugin implements sdk.Plugin and sdk.EventSubscriber.
// It subscribes to workflow and step lifecycle events, logs them to .awf/logs/event-logger.log,
// and emits a "logger.workflow_failed" summary event when a workflow fails.
type EventLoggerPlugin struct {
	sdk.BasePlugin
	mu      sync.Mutex
	logFile *os.File
}

func (p *EventLoggerPlugin) Init(_ context.Context, _ map[string]any) error {
	logDir := filepath.Join(".awf", "logs")
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	f, err := os.OpenFile(filepath.Join(logDir, "event-logger.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	p.logFile = f

	return nil
}

func (p *EventLoggerPlugin) Shutdown(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.logFile != nil {
		err := p.logFile.Close()
		p.logFile = nil
		return err
	}
	return nil
}

func (p *EventLoggerPlugin) log(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.logFile != nil {
		fmt.Fprintln(p.logFile, line)
	}
}

// Patterns declares which event types this plugin subscribes to.
func (p *EventLoggerPlugin) Patterns() []string {
	return []string{"workflow.*", "step.*"}
}

// HandleEvent processes incoming events and optionally emits derived events.
func (p *EventLoggerPlugin) HandleEvent(_ context.Context, event sdk.Event) ([]sdk.Event, error) { //nolint:gocritic // hugeParam: interface constraint, Event cannot be a pointer here
	ts := event.Timestamp.Format(time.RFC3339)
	meta, _ := json.Marshal(event.Metadata)
	p.log(fmt.Sprintf("%s | %-20s | source=%-10s | %s", ts, event.Type, event.Source, meta))

	if event.Type == "workflow.failed" {
		return []sdk.Event{
			{
				Type:   "logger.workflow_failed",
				Source: "event-logger",
				Metadata: map[string]string{
					"original_workflow": event.Metadata["workflow_id"],
					"failure_reason":    event.Metadata["error"],
					"logged_at":         ts,
				},
			},
		}, nil
	}

	return nil, nil
}

func main() {
	sdk.Serve(&EventLoggerPlugin{
		BasePlugin: sdk.BasePlugin{
			PluginName:    "event-logger",
			PluginVersion: "1.0.0",
		},
	})
}
