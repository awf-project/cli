package pluginmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

const defaultStepTypeTimeout = 5 * time.Second

// grpcStepTypeAdapter implements ports.StepTypeProvider for a single plugin connection.
// Step type names are cached at Init() via ListStepTypes(); HasStepType() is O(1).
// First-registered-wins on name conflict; subsequent registrations log a warning.
type grpcStepTypeAdapter struct {
	client     pluginv1.StepTypeServiceClient
	pluginName string
	timeout    time.Duration
	logger     ports.Logger
	cache      map[string]bool
}

var _ ports.StepTypeProvider = (*grpcStepTypeAdapter)(nil)

func newGRPCStepTypeAdapter(client pluginv1.StepTypeServiceClient, pluginName string, timeout time.Duration, logger ports.Logger) *grpcStepTypeAdapter {
	if timeout <= 0 {
		timeout = defaultStepTypeTimeout
	}
	return &grpcStepTypeAdapter{
		client:     client,
		pluginName: pluginName,
		timeout:    timeout,
		logger:     logger,
		cache:      make(map[string]bool),
	}
}

// qualifiedName returns the host-side qualified name for a step type: "<pluginName>.<rawName>".
// Plugins declare short names (e.g. "query"); the host auto-prefixes to avoid collisions
// (e.g. "awf-plugin-database.query"). Same pattern as operation namespacing.
func (a *grpcStepTypeAdapter) qualifiedName(rawName string) string {
	return a.pluginName + "." + rawName
}

// rawName strips the plugin prefix from a qualified step type name.
// Returns the original name if not prefixed by this adapter's plugin.
func (a *grpcStepTypeAdapter) rawName(qualified string) string {
	prefix := a.pluginName + "."
	return strings.TrimPrefix(qualified, prefix)
}

// listAndCache fetches registered step types from the plugin and populates the cache.
// Called once after plugin Init() succeeds. Step type names are automatically prefixed
// with "<pluginName>." to prevent cross-plugin collisions. Conflicts are resolved
// first-registered-wins with a warning logged per duplicate.
func (a *grpcStepTypeAdapter) listAndCache(ctx context.Context, existing map[string]bool) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	resp, err := a.client.ListStepTypes(ctx, &pluginv1.ListStepTypesRequest{})
	if err != nil {
		return fmt.Errorf("list step types: %w", err)
	}

	for _, st := range resp.StepTypes {
		qName := a.qualifiedName(st.Name)
		if a.cache[qName] || existing[qName] {
			if a.logger != nil {
				a.logger.Warn(fmt.Sprintf("step type %q already registered, skipping from plugin %s", qName, a.pluginName))
			}
			continue
		}
		a.cache[qName] = true
	}

	return nil
}

func (a *grpcStepTypeAdapter) HasStepType(typeName string) bool {
	return a.cache[typeName]
}

// compositeStepTypeProvider aggregates multiple per-plugin step type adapters into a single
// StepTypeProvider. HasStepType checks all adapters; ExecuteStep dispatches to the owning adapter.
type compositeStepTypeProvider struct {
	adapters []*grpcStepTypeAdapter
}

var _ ports.StepTypeProvider = (*compositeStepTypeProvider)(nil)

func (c *compositeStepTypeProvider) HasStepType(typeName string) bool {
	for _, a := range c.adapters {
		if a.HasStepType(typeName) {
			return true
		}
	}
	return false
}

func (c *compositeStepTypeProvider) ExecuteStep(ctx context.Context, req ports.StepExecuteRequest) (ports.StepExecuteResult, error) {
	for _, a := range c.adapters {
		if a.HasStepType(req.StepType) {
			return a.ExecuteStep(ctx, req)
		}
	}
	return ports.StepExecuteResult{}, fmt.Errorf("no plugin registered for step type %q", req.StepType)
}

func (a *grpcStepTypeAdapter) ExecuteStep(ctx context.Context, req ports.StepExecuteRequest) (ports.StepExecuteResult, error) {
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return ports.StepExecuteResult{}, fmt.Errorf("marshal config: %w", err)
	}

	inputsJSON, err := json.Marshal(req.Inputs)
	if err != nil {
		return ports.StepExecuteResult{}, fmt.Errorf("marshal inputs: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// Strip plugin prefix before sending to gRPC — plugin expects its raw short name
	resp, err := a.client.ExecuteStep(ctx, &pluginv1.ExecuteStepRequest{
		StepName: req.StepName,
		StepType: a.rawName(req.StepType),
		Config:   configJSON,
		Inputs:   inputsJSON,
	})
	if err != nil {
		return ports.StepExecuteResult{}, fmt.Errorf("execute step: %w", err)
	}

	var data map[string]any
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return ports.StepExecuteResult{}, fmt.Errorf("unmarshal data: %w", err)
		}
	}

	return ports.StepExecuteResult{
		Output:   resp.Output,
		Data:     data,
		ExitCode: int(resp.ExitCode),
	}, nil
}
