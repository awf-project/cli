package sdk

import (
	"context"
	"encoding/json"
	"fmt"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

// Severity of a validation issue.
// SeverityError (zero value) matches proto SEVERITY_UNSPECIFIED being treated as ERROR by the host.
type Severity int

const (
	SeverityError   Severity = 0
	SeverityWarning Severity = 1
	SeverityInfo    Severity = 2
)

// ValidationIssue describes a single validation problem found by a validator plugin.
type ValidationIssue struct {
	Severity Severity
	Message  string
	Step     string // step name, empty for workflow-level issues
	Field    string // field name within step, empty if not applicable
}

// WorkflowDefinition is the SDK representation of a workflow for validator plugins.
// It is deserialized from the JSON payload sent by the host during validation.
type WorkflowDefinition struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description,omitempty"`
	Version     string                    `json:"version,omitempty"`
	Author      string                    `json:"author,omitempty"`
	Tags        []string                  `json:"tags,omitempty"`
	Initial     string                    `json:"initial"`
	Steps       map[string]StepDefinition `json:"steps"`
}

// StepDefinition is the SDK representation of a workflow step for validator plugins.
type StepDefinition struct {
	Type        string         `json:"type"`
	Description string         `json:"description,omitempty"`
	Command     string         `json:"command,omitempty"`
	Operation   string         `json:"operation,omitempty"`
	Timeout     int            `json:"timeout,omitempty"`
	OnSuccess   string         `json:"on_success,omitempty"`
	OnFailure   string         `json:"on_failure,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
}

// Validator is the plugin-author interface for custom workflow validation.
// Implement this interface to add validation rules that run during `awf validate`.
//
// The host calls ValidateWorkflow once per validation run, then ValidateStep
// for each step in the workflow. Return nil issues for no problems.
type Validator interface {
	ValidateWorkflow(ctx context.Context, workflow WorkflowDefinition) ([]ValidationIssue, error)
	ValidateStep(ctx context.Context, workflow WorkflowDefinition, stepName string) ([]ValidationIssue, error)
}

// validatorServiceServer implements pluginv1.ValidatorServiceServer.
// Delegates to the plugin if it implements Validator; otherwise returns empty results.
type validatorServiceServer struct {
	pluginv1.UnimplementedValidatorServiceServer
	impl Plugin
}

func (s *validatorServiceServer) ValidateWorkflow(ctx context.Context, req *pluginv1.ValidateWorkflowRequest) (*pluginv1.ValidateWorkflowResponse, error) {
	validator, ok := s.impl.(Validator)
	if !ok {
		return &pluginv1.ValidateWorkflowResponse{}, nil
	}

	var def WorkflowDefinition
	if err := json.Unmarshal(req.WorkflowJson, &def); err != nil {
		return nil, fmt.Errorf("unmarshal workflow: %w", err)
	}

	issues, err := validator.ValidateWorkflow(ctx, def)
	if err != nil {
		return nil, fmt.Errorf("validate workflow: %w", err)
	}

	return &pluginv1.ValidateWorkflowResponse{
		Issues: toProtoIssues(issues),
	}, nil
}

func (s *validatorServiceServer) ValidateStep(ctx context.Context, req *pluginv1.ValidateStepRequest) (*pluginv1.ValidateStepResponse, error) {
	validator, ok := s.impl.(Validator)
	if !ok {
		return &pluginv1.ValidateStepResponse{}, nil
	}

	var def WorkflowDefinition
	if err := json.Unmarshal(req.WorkflowJson, &def); err != nil {
		return nil, fmt.Errorf("unmarshal workflow: %w", err)
	}

	issues, err := validator.ValidateStep(ctx, def, req.StepName)
	if err != nil {
		return nil, fmt.Errorf("validate step: %w", err)
	}

	return &pluginv1.ValidateStepResponse{
		Issues: toProtoIssues(issues),
	}, nil
}

func toProtoIssues(issues []ValidationIssue) []*pluginv1.ValidationIssue {
	result := make([]*pluginv1.ValidationIssue, len(issues))
	for i, issue := range issues {
		result[i] = &pluginv1.ValidationIssue{
			Severity: severityToProto(issue.Severity),
			Message:  issue.Message,
			Step:     issue.Step,
			Field:    issue.Field,
		}
	}
	return result
}

func severityToProto(s Severity) pluginv1.Severity {
	switch s {
	case SeverityWarning:
		return pluginv1.Severity_SEVERITY_WARNING
	case SeverityInfo:
		return pluginv1.Severity_SEVERITY_INFO
	default:
		return pluginv1.Severity_SEVERITY_ERROR
	}
}
