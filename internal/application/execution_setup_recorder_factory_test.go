package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
)

// TestBuild_WiresRecorderFactoryAndTranscriptDir verifies that WithRecorderFactory
// and WithTranscriptDir propagate through ExecutionSetup.Build onto the resulting
// ExecutionService. Without this wiring the executeCallWorkflowStep child-recorder
// block (if s.recorderFactory != nil) is permanently unreachable in production and
// F106 sub-workflow transcript linkage is dead code.
func TestBuild_WiresRecorderFactoryAndTranscriptDir(t *testing.T) {
	repo := testmocks.NewMockWorkflowRepository()
	store := testmocks.NewMockStateStore()
	executor := testmocks.NewMockCommandExecutor()
	logger := testmocks.NewMockLogger()

	var factoryCalledWith string
	factory := func(path string) (ports.Recorder, error) {
		factoryCalledWith = path
		return &fakeRecorder{}, nil
	}

	setup := NewExecutionSetup(repo, store, executor, logger,
		WithRecorderFactory(factory),
		WithTranscriptDir("/tmp/awf-transcripts"),
	)

	result, err := setup.Build(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result.ExecService)

	// transcriptDir must be wired through to the service base dir.
	assert.Equal(t, "/tmp/awf-transcripts", result.ExecService.transcriptBaseDir())

	// recorderFactory must be wired and callable so child recorders can be created.
	require.NotNil(t, result.ExecService.recorderFactory, "recorderFactory must be wired by Build")
	rec, err := result.ExecService.recorderFactory("/tmp/awf-transcripts/child.jsonl")
	require.NoError(t, err)
	assert.NotNil(t, rec)
	assert.Equal(t, "/tmp/awf-transcripts/child.jsonl", factoryCalledWith)
}
