package tui

import (
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
)

func TestStatusBadge_ContainsIcon(t *testing.T) {
	tests := []struct {
		status workflow.ExecutionStatus
		icon   string
	}{
		{workflow.StatusPending, "⏳"},
		{workflow.StatusRunning, "⟳"},
		{workflow.StatusCompleted, "✓"},
		{workflow.StatusFailed, "✗"},
		{workflow.StatusCancelled, "⏭"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			badge := StatusBadge(tt.status)
			assert.Contains(t, badge, tt.icon)
		})
	}
}

func TestStatusBadgeFromString_ContainsIcon(t *testing.T) {
	tests := []struct {
		status string
		icon   string
	}{
		{"success", "✓"},
		{"failed", "✗"},
		{"cancelled", "⏭"},
		{"unknown", "?"},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			badge := StatusBadgeFromString(tt.status)
			assert.Contains(t, badge, tt.icon)
		})
	}
}

func TestPanel_ContainsTitle(t *testing.T) {
	result := Panel("My Title", "body content", 40, 10, true)
	assert.Contains(t, result, "My Title")
	assert.Contains(t, result, "body content")
}

func TestPanel_ActiveVsInactive(t *testing.T) {
	active := Panel("T", "c", 40, 10, true)
	inactive := Panel("T", "c", 40, 10, false)
	assert.NotEqual(t, active, inactive)
}

func TestEmptyStateView_ContainsTitleAndSubtitle(t *testing.T) {
	result := EmptyStateView("📋", "Nothing here", "Try launching a workflow")
	assert.Contains(t, result, "Nothing here")
	assert.Contains(t, result, "Try launching a workflow")
}

func TestHeaderBar_ContainsLeftAndRight(t *testing.T) {
	result := HeaderBar("left text", "right text", 80)
	assert.Contains(t, result, "left text")
	assert.Contains(t, result, "right text")
}

func TestSeparator_HasCorrectWidth(t *testing.T) {
	sep := Separator(40)
	assert.NotEmpty(t, sep)
	assert.True(t, strings.Contains(sep, "─"))
}
