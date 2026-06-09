package application

import (
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
)

// SessionRegistry is an in-process registry of active RunSessions keyed by ID.
// D12: simple map + RWMutex; multi-viewer fan-out is out of scope (A4, spec line 20).
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*RunSession
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string]*RunSession),
	}
}

// Add registers a session. Returns ports.ErrSessionExists if the ID is already registered.
func (r *SessionRegistry) Add(s *RunSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[s.id]; ok {
		return ports.ErrSessionExists
	}
	r.sessions[s.id] = s
	return nil
}

func (r *SessionRegistry) Get(id string) (*RunSession, bool) {
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
