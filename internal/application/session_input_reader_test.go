package application

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
)

// TestSessionInputReader_EmitsInputRequiredAndUnblocksOnRespond verifies the facade
// conversation parking round-trip (F110 G4): ReadInput appends an EventInputRequired
// event and blocks until RunSession.Respond delivers a value.
func TestSessionInputReader_EmitsInputRequiredAndUnblocksOnRespond(t *testing.T) {
	session := newRunSession(context.Background(), "run-1", 16)
	reader := newSessionInputReader(session)

	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		v, err := reader.ReadInput(context.Background())
		resultCh <- v
		errCh <- err
	}()

	// The first event on the session must be the bridge-synthesized input request.
	select {
	case ev := <-session.Events():
		require.Equal(t, ports.EventInputRequired, ev.Kind)
		req, ok := ev.Payload.(*ports.EnrichedInputRequest)
		require.True(t, ok, "payload must be *EnrichedInputRequest")
		assert.NotEmpty(t, req.Prompt)
	case <-time.After(time.Second):
		t.Fatal("expected EventInputRequired to be emitted")
	}

	require.NoError(t, session.Respond(ports.InputResponse{Value: "next turn"}))

	select {
	case v := <-resultCh:
		assert.Equal(t, "next turn", v)
		assert.NoError(t, <-errCh)
	case <-time.After(time.Second):
		t.Fatal("ReadInput did not unblock after Respond")
	}
}

// TestSessionInputReader_ContextCancelUnblocks verifies that a cancelled caller context
// unblocks the parked read and surfaces the context error (so the conversation loop can
// stop instead of hanging).
func TestSessionInputReader_ContextCancelUnblocks(t *testing.T) {
	session := newRunSession(context.Background(), "run-2", 16)
	reader := newSessionInputReader(session)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := reader.ReadInput(ctx)
		errCh <- err
	}()

	// Drain the emitted input-required event.
	<-session.Events()
	cancel()

	select {
	case err := <-errCh:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("ReadInput did not unblock after context cancel")
	}
}

// TestUserInputReaderContext_RoundTrips verifies the context binding helpers used by the
// Adapter to inject a per-run reader into the conversation execution path.
func TestUserInputReaderContext_RoundTrips(t *testing.T) {
	assert.Nil(t, userInputReaderFrom(context.Background()))

	session := newRunSession(context.Background(), "run-3", 16)
	reader := newSessionInputReader(session)
	ctx := withUserInputReader(context.Background(), reader)

	got := userInputReaderFrom(ctx)
	require.NotNil(t, got)
	assert.Same(t, reader, got)

	// A nil reader leaves the context unchanged.
	assert.Equal(t, context.Background(), withUserInputReader(context.Background(), nil))
}

// TestConversationManager_PrefersContextScopedReader verifies that a per-run reader bound
// onto the context (as the facade Adapter does for the CLI conversation path, F110 G4)
// takes precedence over the static setup-time reader. The static reader would never exit,
// so reaching StopReasonUserExit proves the context reader was used.
func TestConversationManager_PrefersContextScopedReader(t *testing.T) {
	logger := mocks.NewMockLogger()
	registry := mocks.NewMockAgentRegistry()
	provider := mocks.NewMockAgentProvider("claude")
	provider.SetConversationFunc(func(_ context.Context, state *workflow.ConversationState, _ string, _ map[string]any, _, _ io.Writer) (*workflow.ConversationResult, error) {
		return &workflow.ConversationResult{State: state, Provider: "claude"}, nil
	})
	require.NoError(t, registry.Register(provider))

	manager := NewConversationManager(logger, &testResolverWithValues{values: map[string]string{}}, registry)
	// Static reader keeps returning a non-empty message -> would loop forever if used.
	manager.SetUserInputReader(neverExitReader{})

	// Context reader returns empty input -> single turn then graceful user exit.
	ctxReader := mocks.NewMockUserInputReader("")
	ctx := withUserInputReader(context.Background(), ctxReader)

	step := &workflow.Step{
		Name:  "chat",
		Type:  workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{Provider: "claude", Prompt: "Hi"},
	}
	execCtx := workflow.NewExecutionContext("wf", "chat")
	execCtx.States = make(map[string]workflow.StepState)
	buildContext := func(*workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	result, err := manager.ExecuteConversation(
		ctx, step, &workflow.ConversationConfig{}, execCtx, buildContext, "", io.Discard, io.Discard,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, workflow.StopReasonUserExit, result.State.StoppedBy)
}

// neverExitReader is a UserInputReader whose ReadInput never returns empty input, so a
// conversation loop that consults it would never reach StopReasonUserExit.
type neverExitReader struct{}

func (neverExitReader) ReadInput(context.Context) (string, error) { return "keep going", nil }
