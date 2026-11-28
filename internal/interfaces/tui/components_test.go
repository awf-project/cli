package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmptyStateView_ContainsTitleAndSubtitle(t *testing.T) {
	result := EmptyStateView("📋", "Nothing here", "Try launching a workflow")
	assert.Contains(t, result, "Nothing here")
	assert.Contains(t, result, "Try launching a workflow")
}
