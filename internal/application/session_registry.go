package application

import (
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
)

// SessionRegistry is an in-process registry of active RunSessions keyed by ID.
// D12: simple map + RWMutex; multi-viewer fan-out is out of scope (A4, spec line 20).
//
// Sessions are stored as the ports.RunSession interface so that any facade
// implementation (the concrete *RunSession produced by the Adapter, or a test
// double such as facadetest.FakeSession) can be registered and resolved by ID.
// The interface storage also lets interface-layer adapters (e.g. the HTTP
// sessionLookup in api/server.go) hand back a ports.RunSession without an extra
// wrapping shim (R5).
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]ports.RunSession
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string]ports.RunSession),
	}
}

// Add registers a session keyed by its own ID(). Returns ports.ErrSessionExists
// if the ID is already registered.
func (r *SessionRegistry) Add(s ports.RunSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := s.ID()
	if _, ok := r.sessions[id]; ok {
		return ports.ErrSessionExists
	}
	r.sessions[id] = s
	return nil
}

func (r *SessionRegistry) Get(id string) (ports.RunSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	return s, ok
}

func (r *SessionRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
}

func (r *SessionRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}
