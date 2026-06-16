package application

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
)

func TestSessionRegistry_DuplicateAddReturnsErrSessionExists(t *testing.T) {
	reg := NewSessionRegistry()
	s1 := newRunSession(context.Background(), "session-1", 256)

	err1 := reg.Add(s1)
	require.NoError(t, err1)

	s2 := newRunSession(context.Background(), "session-1", 256)
	err2 := reg.Add(s2)

	assert.ErrorIs(t, err2, ports.ErrSessionExists)
}

func TestSessionRegistry_GetMissing(t *testing.T) {
	reg := NewSessionRegistry()

	s, ok := reg.Get("nonexistent")

	assert.Nil(t, s)
	assert.False(t, ok)
}

func TestSessionRegistry_ConcurrentAddRemove(t *testing.T) {
	reg := NewSessionRegistry()
	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			id := "session-" + string(rune(idx%10))
			s := newRunSession(context.Background(), id, 256)

			err := reg.Add(s)
			if err != nil && err != ports.ErrSessionExists {
				errs <- err
			}

			retrieved, ok := reg.Get(id)
			if ok && retrieved.ID() != id {
				errs <- ports.ErrInvalidRequest
			}

			if idx%2 == 0 {
				reg.Remove(id)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent operation failed: %v", err)
	}
}

func TestSessionRegistry_Add(t *testing.T) {
	reg := NewSessionRegistry()
	s := newRunSession(context.Background(), "test-session", 256)

	err := reg.Add(s)

	assert.NoError(t, err)
	retrieved, ok := reg.Get("test-session")
	require.True(t, ok)
	assert.Equal(t, s.ID(), retrieved.ID())
}

func TestSessionRegistry_Remove(t *testing.T) {
	reg := NewSessionRegistry()
	s := newRunSession(context.Background(), "test-session", 256)

	err := reg.Add(s)
	require.NoError(t, err)

	reg.Remove("test-session")

	_, ok := reg.Get("test-session")
	assert.False(t, ok)
}

func TestSessionRegistry_RemoveNonexistent(t *testing.T) {
	reg := NewSessionRegistry()

	reg.Remove("nonexistent")

	assert.Equal(t, 0, reg.Len())
}

func TestSessionRegistry_Len(t *testing.T) {
	reg := NewSessionRegistry()

	assert.Equal(t, 0, reg.Len())

	s1 := newRunSession(context.Background(), "session-1", 256)
	reg.Add(s1)
	assert.Equal(t, 1, reg.Len())

	s2 := newRunSession(context.Background(), "session-2", 256)
	reg.Add(s2)
	assert.Equal(t, 2, reg.Len())

	reg.Remove("session-1")
	assert.Equal(t, 1, reg.Len())
}

func TestSessionRegistry_AddMultipleSessions(t *testing.T) {
	reg := NewSessionRegistry()

	sessions := make([]*RunSession, 5)
	for i := 0; i < 5; i++ {
		s := newRunSession(context.Background(), "session-"+string(rune(i)), 256)
		sessions[i] = s
		err := reg.Add(s)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, reg.Len())

	for i := 0; i < 5; i++ {
		retrieved, ok := reg.Get("session-" + string(rune(i)))
		require.True(t, ok)
		assert.Equal(t, sessions[i].ID(), retrieved.ID())
	}
}

func TestSessionRegistry_GetAfterRemove(t *testing.T) {
	reg := NewSessionRegistry()
	s := newRunSession(context.Background(), "test-session", 256)

	reg.Add(s)
	reg.Remove("test-session")

	_, ok := reg.Get("test-session")
	assert.False(t, ok)
}

func TestSessionRegistry_ConcurrentGet(t *testing.T) {
	reg := NewSessionRegistry()
	s := newRunSession(context.Background(), "test-session", 256)
	reg.Add(s)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			retrieved, ok := reg.Get("test-session")
			if !ok {
				t.Error("expected to find session")
			}
			if retrieved.ID() != "test-session" {
				t.Error("session ID mismatch")
			}
		}()
	}

	wg.Wait()
}

func TestSessionRegistry_NewSessionRegistry(t *testing.T) {
	reg := NewSessionRegistry()
	assert.NotNil(t, reg)
	assert.Equal(t, 0, reg.Len())
}
