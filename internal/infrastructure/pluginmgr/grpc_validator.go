package pluginmgr

import (
	"context"
	"fmt"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

const defaultValidatorTimeout = 5 * time.Second

// grpcValidatorAdapter implements ports.WorkflowValidatorProvider for a single plugin connection.
// Per-plugin timeout (default 5s), crash treated as timeout, results deduplicated by (message+step+field).
type grpcValidatorAdapter struct {
	client     pluginv1.ValidatorServiceClient
	pluginName string
	timeout    time.Duration
}

var _ ports.WorkflowValidatorProvider = (*grpcValidatorAdapter)(nil)

func newGRPCValidatorAdapter(client pluginv1.ValidatorServiceClient, pluginName string, timeout time.Duration) *grpcValidatorAdapter {
	if timeout <= 0 {
		timeout = defaultValidatorTimeout
	}
	return &grpcValidatorAdapter{
		client:     client,
		pluginName: pluginName,
		timeout:    timeout,
	}
}

func (a *grpcValidatorAdapter) ValidateWorkflow(ctx context.Context, workflowJSON []byte) ([]ports.ValidationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	resp, err := a.client.ValidateWorkflow(ctx, &pluginv1.ValidateWorkflowRequest{
		WorkflowJson: workflowJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("validate workflow: %w", err)
	}

	return deduplicateResults(convertValidationIssues(resp.Issues)), nil
}

func (a *grpcValidatorAdapter) ValidateStep(ctx context.Context, workflowJSON []byte, stepName string) ([]ports.ValidationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	resp, err := a.client.ValidateStep(ctx, &pluginv1.ValidateStepRequest{
		WorkflowJson: workflowJSON,
		StepName:     stepName,
	})
	if err != nil {
		return nil, fmt.Errorf("validate step: %w", err)
	}

	return deduplicateResults(convertValidationIssues(resp.Issues)), nil
}

// mapProtoSeverity converts proto Severity to domain Severity.
// SEVERITY_UNSPECIFIED (proto3 zero) and SEVERITY_ERROR both map to SeverityError.
// Ordinals do NOT align between proto and domain — explicit mapping required.
func mapProtoSeverity(s pluginv1.Severity) ports.Severity {
	switch s {
	case pluginv1.Severity_SEVERITY_WARNING:
		return ports.SeverityWarning
	case pluginv1.Severity_SEVERITY_INFO:
		return ports.SeverityInfo
	default:
		// SEVERITY_UNSPECIFIED (0) and SEVERITY_ERROR (2) both map to SeverityError
		return ports.SeverityError
	}
}

// deduplicationKey is the tuple used to identify duplicate validation results.
type deduplicationKey struct {
	message string
	step    string
	field   string
}

// convertValidationIssues converts proto ValidationIssue messages to domain ValidationResult.
func convertValidationIssues(issues []*pluginv1.ValidationIssue) []ports.ValidationResult {
	results := make([]ports.ValidationResult, 0, len(issues))
	for _, issue := range issues {
		results = append(results, ports.ValidationResult{
			Severity: mapProtoSeverity(issue.Severity),
			Message:  issue.Message,
			Step:     issue.Step,
			Field:    issue.Field,
		})
	}
	return results
}

// compositeValidatorProvider aggregates multiple per-plugin validator adapters into a single
// WorkflowValidatorProvider. Each plugin is called with its own timeout; results are merged
// and deduplicated across all plugins.
type compositeValidatorProvider struct {
	adapters []*grpcValidatorAdapter
}

var _ ports.WorkflowValidatorProvider = (*compositeValidatorProvider)(nil)

func (c *compositeValidatorProvider) ValidateWorkflow(ctx context.Context, workflowJSON []byte) ([]ports.ValidationResult, error) {
	var allResults []ports.ValidationResult
	for _, a := range c.adapters {
		results, err := a.ValidateWorkflow(ctx, workflowJSON)
		if err != nil {
			// Plugin errors are non-fatal — skip with warning, continue others
			continue
		}
		allResults = append(allResults, results...)
	}
	return deduplicateResults(allResults), nil
}

func (c *compositeValidatorProvider) ValidateStep(ctx context.Context, workflowJSON []byte, stepName string) ([]ports.ValidationResult, error) {
	var allResults []ports.ValidationResult
	for _, a := range c.adapters {
		results, err := a.ValidateStep(ctx, workflowJSON, stepName)
		if err != nil {
			continue
		}
		allResults = append(allResults, results...)
	}
	return deduplicateResults(allResults), nil
}

// deduplicateResults removes duplicate validation results based on (message, step, field) tuple.
// Returns results in original order, keeping first occurrence of each unique tuple.
func deduplicateResults(results []ports.ValidationResult) []ports.ValidationResult {
	seen := make(map[deduplicationKey]bool)
	deduped := make([]ports.ValidationResult, 0, len(results))

	for _, r := range results {
		key := deduplicationKey{
			message: r.Message,
			step:    r.Step,
			field:   r.Field,
		}
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, r)
		}
	}

	return deduped
}
