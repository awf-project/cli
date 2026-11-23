package application

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPendingExecutionsTracksRunningGoroutines(t *testing.T) {
	adapter := &Adapter{}
	assert.Empty(t, adapter.PendingExecutions())

	adapter.markExecutionStarted("run-1")
	adapter.markExecutionStarted("run-2")
	pending := adapter.PendingExecutions()
	sort.Strings(pending)
	require.Equal(t, []string{"run-1", "run-2"}, pending)

	adapter.markExecutionFinished("run-1")
	assert.Equal(t, []string{"run-2"}, adapter.PendingExecutions())

	adapter.markExecutionFinished("run-2")
	assert.Empty(t, adapter.PendingExecutions())
}

func TestPendingExecutionsIgnoresUnknownExecution(t *testing.T) {
	adapter := &Adapter{}

	adapter.markExecutionFinished("never-started")

	assert.Empty(t, adapter.PendingExecutions())
}
