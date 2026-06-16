package ui

import (
	"io"
	"sync"
)

// SyncWriter serializes concurrent writes to an underlying io.Writer.
//
// The facade run path (F107) executes a workflow on a background goroutine while
// the main goroutine drains and renders the session's events. Both sides write to
// the same stdout sink: the background side through the injected logger/Formatter
// (and per-step PrefixedWriters), the main side through the event projector. Those
// writes are otherwise unsynchronized and the race detector flags them as a data
// race on the shared buffer.
//
// Wrapping the shared sink in a single SyncWriter — and routing every writer in the
// run path through that one instance — makes the writes mutually exclusive. Because
// fmt.Fprint/Fprintln emit each message in a single Write call, locking at the Write
// boundary preserves line-level output without interleaving partial lines.
type SyncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewSyncWriter wraps w so that concurrent Write calls are serialized. Passing an
// already-synchronized writer is harmless (the extra lock is uncontended).
func NewSyncWriter(w io.Writer) *SyncWriter {
	return &SyncWriter{w: w}
}

// Write writes p to the underlying writer while holding the mutex, so concurrent
// callers never interleave their bytes.
func (s *SyncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
