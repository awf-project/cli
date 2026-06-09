package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
)

type mockRunSession struct {
	events chan ports.Event
	err    error
}

func (m *mockRunSession) ID() string {
	return "mock-session"
}

func (m *mockRunSession) Events() <-chan ports.Event {
	return m.events
}

func (m *mockRunSession) Respond(ports.InputResponse) error {
	return nil
}

func (m *mockRunSession) Err() error {
	return m.err
}

func (m *mockRunSession) Close() error {
	return nil
}

func TestDrain_ReturnsNilOnCleanCompletion(t *testing.T) {
	events := make(chan ports.Event, 5)

	for i := 0; i < 3; i++ {
		events <- ports.Event{
			Seq:  uint64(i),
			Kind: ports.EventRunStarted,
		}
	}
	close(events)

	session := &mockRunSession{
		events: events,
		err:    nil,
	}

	err := Drain(session)

	assert.NoError(t, err)
}

func TestDrain_ReturnsTerminalCause(t *testing.T) {
	events := make(chan ports.Event, 5)

	for i := 0; i < 3; i++ {
		events <- ports.Event{
			Seq:  uint64(i),
			Kind: ports.EventRunStarted,
		}
	}
	close(events)

	expectedErr := errors.New("execution failed")
	session := &mockRunSession{
		events: events,
		err:    expectedErr,
	}

	err := Drain(session)

	assert.Equal(t, expectedErr, err)
}

func TestDrain_ConsumesAllEvents(t *testing.T) {
	events := make(chan ports.Event, 10)

	for i := 0; i < 5; i++ {
		events <- ports.Event{
			Seq:  uint64(i),
			Kind: ports.EventRunStarted,
		}
	}
	close(events)

	session := &mockRunSession{
		events: events,
		err:    nil,
	}

	err := Drain(session)
	require.NoError(t, err)

	select {
	case _, ok := <-session.Events():
		if ok {
			t.Error("expected channel to be empty")
		}
	default:
	}
}

func TestDrain_WithEmptyEventStream(t *testing.T) {
	events := make(chan ports.Event)
	close(events)

	session := &mockRunSession{
		events: events,
		err:    nil,
	}

	err := Drain(session)

	assert.NoError(t, err)
}

func TestDrain_WithContextCancelledError(t *testing.T) {
	events := make(chan ports.Event, 2)
	events <- ports.Event{Seq: 1, Kind: ports.EventRunStarted}
	close(events)

	session := &mockRunSession{
		events: events,
		err:    context.Canceled,
	}

	err := Drain(session)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestDrain_WithMultipleEvents(t *testing.T) {
	events := make(chan ports.Event, 100)

	for i := 0; i < 50; i++ {
		events <- ports.Event{
			Seq:  uint64(i),
			Kind: ports.EventMessageUser,
		}
	}
	close(events)

	session := &mockRunSession{
		events: events,
		err:    nil,
	}

	err := Drain(session)

	assert.NoError(t, err)
}

func TestDrain_ImplementsInterfaceContract(t *testing.T) {
	events := make(chan ports.Event)
	close(events)

	session := &mockRunSession{
		events: events,
		err:    nil,
	}

	var _ ports.RunSession = session

	err := Drain(session)
	assert.NoError(t, err)
}
