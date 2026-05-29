package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowEntry_LocalSource(t *testing.T) {
	tests := []struct {
		name         string
		entry        WorkflowEntry
		wantName     string
		wantSource   string
		wantScope    string
		wantWorkflow string
	}{
		{
			name: "local workflow",
			entry: WorkflowEntry{
				Name:     "deploy",
				Source:   "local",
				Scope:    "local",
				Workflow: "deploy",
			},
			wantName:     "deploy",
			wantSource:   "local",
			wantScope:    "local",
			wantWorkflow: "deploy",
		},
		{
			name: "global workflow scope mirrors source",
			entry: WorkflowEntry{
				Name:     "deploy",
				Source:   "global",
				Scope:    "global",
				Workflow: "deploy",
			},
			wantName:     "deploy",
			wantSource:   "global",
			wantScope:    "global",
			wantWorkflow: "deploy",
		},
		{
			name: "env workflow scope mirrors source",
			entry: WorkflowEntry{
				Name:     "audit",
				Source:   "env",
				Scope:    "env",
				Workflow: "audit",
			},
			wantName:     "audit",
			wantSource:   "env",
			wantScope:    "env",
			wantWorkflow: "audit",
		},
		{
			name: "local workflow with optional fields empty",
			entry: WorkflowEntry{
				Name:     "build",
				Source:   "local",
				Scope:    "local",
				Workflow: "build",
			},
			wantName:     "build",
			wantSource:   "local",
			wantScope:    "local",
			wantWorkflow: "build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantName, tt.entry.Name)
			assert.Equal(t, tt.wantSource, tt.entry.Source)
			assert.Equal(t, tt.wantScope, tt.entry.Scope)
			assert.Equal(t, tt.wantWorkflow, tt.entry.Workflow)
		})
	}
}

func TestWorkflowEntry_PackSource(t *testing.T) {
	tests := []struct {
		name         string
		entry        WorkflowEntry
		wantName     string
		wantSource   string
		wantScope    string
		wantWorkflow string
	}{
		{
			name: "pack workflow carries vendor scope",
			entry: WorkflowEntry{
				Name:     "speckit/specify",
				Source:   "pack",
				Scope:    "speckit",
				Workflow: "specify",
			},
			wantName:     "speckit/specify",
			wantSource:   "pack",
			wantScope:    "speckit",
			wantWorkflow: "specify",
		},
		{
			name: "pack workflow name differs from workflow bare name",
			entry: WorkflowEntry{
				Name:     "acme/deploy",
				Source:   "pack",
				Scope:    "acme",
				Workflow: "deploy",
			},
			wantName:     "acme/deploy",
			wantSource:   "pack",
			wantScope:    "acme",
			wantWorkflow: "deploy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantName, tt.entry.Name)
			assert.Equal(t, tt.wantSource, tt.entry.Source)
			assert.Equal(t, tt.wantScope, tt.entry.Scope)
			assert.Equal(t, tt.wantWorkflow, tt.entry.Workflow)
		})
	}
}

func TestWorkflowEntry_OptionalFields(t *testing.T) {
	// Version and Description are optional and may be empty.
	entry := WorkflowEntry{
		Name:     "ci",
		Source:   "local",
		Scope:    "local",
		Workflow: "ci",
	}

	assert.Empty(t, entry.Version)
	assert.Empty(t, entry.Description)
}

func TestWorkflowEntry_WithVersionAndDescription(t *testing.T) {
	entry := WorkflowEntry{
		Name:        "speckit/specify",
		Source:      "pack",
		Scope:       "speckit",
		Workflow:    "specify",
		Version:     "1.2.0",
		Description: "Spec-driven development workflow",
	}

	assert.Equal(t, "1.2.0", entry.Version)
	assert.Equal(t, "Spec-driven development workflow", entry.Description)
}
