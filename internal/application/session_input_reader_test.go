package application

import (
	"context"
	"errors"
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

func TestSessionInputReaderWaitsForEventOwner(t *testing.T) {
	session := newRunSession(context.Background(), "run-1", 16)
	reader := newSessionInputReader(session)
	t.Cleanup(func() { require.NoError(t, session.Close()) })

	testCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	type readResult struct {
		value string
		err   error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		value, err := reader.ReadInput(testCtx)
		resultCh <- readResult{value: value, err: err}
	}()

	var publication inputPublicationRequest
	select {
	case publication = <-session.inputPublicationCh:
	case <-testCtx.Done():
		t.Fatal("ReadInput did not hand publication to the event owner")
	}
	assert.NotNil(t, publication.acknowledge)

	select {
	case event := <-session.Events():
		t.Fatalf("input reader published an event before the event owner acknowledged it: %#v", event)
	default:
	}
	select {
	case result := <-resultCh:
		t.Fatalf("ReadInput returned before publication acknowledgement: value %q, error %v", result.value, result.err)
	default:
	}

	publication.acknowledge()
	const submitted = "next turn"
	require.NoError(t, session.Respond(ports.InputResponse{Value: submitted}))

	result := receiveSessionInputResult(t, testCtx, resultCh)
	require.NoError(t, result.err)
	assert.Equal(t, submitted, result.value)
}

func TestSessionInputReaderStopsWhenInputPublicationIsCancelled(t *testing.T) {
	tests := []struct {
		name           string
		deliverRequest bool
		closeSession   bool
	}{
		{name: "caller cancellation while delivery is pending"},
		{name: "caller cancellation while acknowledgement is pending", deliverRequest: true},
		{name: "session close while delivery is pending", closeSession: true},
		{name: "session close while acknowledgement is pending", deliverRequest: true, closeSession: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := newRunSession(context.Background(), "run-2", 16)
			reader := newSessionInputReader(session)
			t.Cleanup(func() { require.NoError(t, session.Close()) })

			testCtx, stopTest := context.WithTimeout(context.Background(), time.Second)
			defer stopTest()
			readCtx, cancelRead := context.WithCancel(testCtx)
			defer cancelRead()

			type readResult struct {
				value string
				err   error
			}
			resultCh := make(chan readResult, 1)
			go func() {
				value, err := reader.ReadInput(readCtx)
				resultCh <- readResult{value: value, err: err}
			}()

			if tt.deliverRequest {
				select {
				case <-session.inputPublicationCh:
				case <-testCtx.Done():
					t.Fatal("ReadInput did not deliver the publication request")
				}
			}

			if tt.closeSession {
				require.NoError(t, session.Close())
			} else {
				cancelRead()
			}

			result := receiveSessionInputResult(t, testCtx, resultCh)
			assert.Empty(t, result.value)
			expectedErr := readCtx.Err()
			if tt.closeSession {
				expectedErr = session.ctx.Err()
			}
			assert.Equal(t, expectedErr, result.err)
			assert.NotErrorIs(t, result.err, ports.ErrSessionClosed)
		})
	}
}

func TestSessionInputReaderPublishesThroughFacadeOwner(t *testing.T) {
	adapter := NewAdapter(nil, nil, nil, nil, nil, NewSessionRegistry())
	adapter.SetUserInputReader(neverExitReader{})

	testCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan string, 1)
	session, err := adapter.newSession(testCtx, "run-facade", &mockRecorder{}, func(execCtx context.Context) error {
		reader := userInputReaderFrom(execCtx)
		if reader == nil {
			return errors.New("session input reader was not bound")
		}
		value, readErr := reader.ReadInput(execCtx)
		if readErr == nil {
			resultCh <- value
		}
		return readErr
	})
	require.NoError(t, err)

	select {
	case event := <-session.Events():
		require.Equal(t, ports.EventInputRequired, event.Kind)
		request, ok := event.Payload.(*ports.EnrichedInputRequest)
		require.True(t, ok)
		assert.Equal(t, "> ", request.Prompt)
	case <-testCtx.Done():
		t.Fatalf("facade event owner did not publish the input event: %v", testCtx.Err())
	}

	const submitted = "facade response"
	require.NoError(t, session.Respond(ports.InputResponse{Value: submitted}))
	assert.Equal(t, submitted, receiveSessionInputResult(t, testCtx, resultCh))
}

func receiveSessionInputResult[T any](t *testing.T, ctx context.Context, resultCh <-chan T) T {
	t.Helper()
	select {
	case result := <-resultCh:
		return result
	case <-ctx.Done():
		t.Fatalf("ReadInput did not return before the test deadline: %v", ctx.Err())
		var zero T
		return zero
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
