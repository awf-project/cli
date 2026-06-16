package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
)

// sessionLookup adapts *application.SessionRegistry to the SessionLookup interface
// for SSE handler wiring. Risk R5 adapter.
type sessionLookup struct{ reg *application.SessionRegistry }

func (sl sessionLookup) GetSession(id string) (ports.RunSession, bool) {
	return sl.reg.Get(id)
}

// Option configures a Server on construction.
type Option func(*Server)

// WithShutdownTimeout overrides the default 30s graceful shutdown timeout.
func WithShutdownTimeout(d time.Duration) Option {
	return func(s *Server) {
		s.shutdownTimeout = d
	}
}

// WithFacade wires the workflow facade for handlers that need it.
func WithFacade(facade ports.WorkflowFacade) Option {
	return func(s *Server) {
		s.facade = facade
	}
}

// WithWorkflowReader wires the focused read-only port used by the single-workflow GET and
// history-stats endpoints. It is typically the same application.Adapter passed to WithFacade.
// Without it, those two endpoints degrade to 503.
func WithWorkflowReader(reader ports.WorkflowReader) Option {
	return func(s *Server) {
		s.reader = reader
	}
}

// WithSessionRegistry wires a SessionLookup into handlers that need to resolve live
// RunSessions by ID (SSEHandler and RespondHandler). Without this option those
// handlers return 404 for every request — no panic, but no streaming either.
func WithSessionRegistry(sl SessionLookup) Option {
	return func(s *Server) {
		s.sessions = sl
	}
}

// WithRegistryImpl wires an application.SessionRegistry into the server.
// It also creates a sessionLookup adapter and sets it as the SessionLookup for SSE/respond handlers.
func WithRegistryImpl(reg *application.SessionRegistry) Option {
	return func(s *Server) {
		s.reg = reg
		s.sessions = sessionLookup{reg}
	}
}

// Server assembles all handler families into a single HTTP server backed by chi and Huma.
type Server struct {
	bridge          *Bridge
	mux             *chi.Mux
	api             huma.API
	httpSrv         *http.Server
	shutdownTimeout time.Duration
	sseWG           sync.WaitGroup
	facade          ports.WorkflowFacade
	reader          ports.WorkflowReader
	sessions        SessionLookup
	reg             *application.SessionRegistry
}

// NewServer assembles a Server with middleware and all route families on addr.
func NewServer(bridge *Bridge, addr string, opts ...Option) *Server {
	s := &Server{
		bridge:          bridge,
		shutdownTimeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}

	s.mux = chi.NewMux()
	s.mux.Use(chiMiddleware.Logger)
	s.mux.Use(chiMiddleware.Recoverer)
	s.mux.Use(chiMiddleware.RequestID)

	config := huma.DefaultConfig("AWF API", "v1")
	config.Info.Description = "AWF workflow execution and management API"
	s.api = humachi.New(s.mux, config)

	RegisterWorkflowRoutes(s.api, NewWorkflowHandlers(s.facade, s.reader))
	execHandlers := NewExecutionHandlers(bridge)
	if s.facade != nil {
		execHandlers.SetFacade(s.facade)
	}
	if s.reg != nil {
		execHandlers.SetSessionRegistry(s.reg)
	}
	RegisterExecutionRoutes(s.api, execHandlers)
	sseHandler := NewSSEHandler(bridge, &s.sseWG)
	if s.sessions != nil {
		sseHandler.SetSessionLookup(s.sessions)
	}
	RegisterSSERoutes(s.api, sseHandler)
	if s.facade != nil {
		respondHandler := NewRespondHandler(s.facade)
		if s.sessions != nil {
			respondHandler.SetSessionLookup(s.sessions)
		}
		RegisterRespondRoutes(s.api, respondHandler)
	}
	RegisterHistoryRoutes(s.api, NewHistoryHandlers(s.facade, s.reader))

	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second, //nolint:gosec // G112: timeout set to prevent Slowloris
	}

	return s
}

// Start sets the server's BaseContext to ctx and calls ListenAndServe.
// Returns nil when the server shuts down gracefully.
func (s *Server) Start(ctx context.Context) error {
	s.httpSrv.BaseContext = func(_ net.Listener) context.Context { return ctx }
	if err := s.httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

// sseWaitTimeout is the maximum time Shutdown will block for SSE goroutines to drain after
// the HTTP server has stopped accepting new connections. A misbehaving long-running SSE
// consumer must not hold up the process indefinitely; 10 s is generous for real clients
// that honor the server-close signal, and short enough to be tolerable in CI.
const sseWaitTimeout = 10 * time.Second

// Shutdown gracefully stops the HTTP server within shutdownTimeout and waits for active
// SSE goroutines with a bounded deadline.
//
// Ordering:
//  1. httpSrv.Shutdown closes the listener and drains open connections (within shutdownTimeout).
//  2. sseWG.Wait races against sseWaitTimeout. If SSE goroutines do not drain in time, a
//     warning is logged and Shutdown returns — it does not block forever.
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, s.shutdownTimeout)
	defer cancel()
	err := s.httpSrv.Shutdown(shutdownCtx)

	// Drain SSE goroutines with a hard cap so a misbehaving consumer cannot block shutdown.
	done := make(chan struct{})
	go func() {
		s.sseWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		// All SSE goroutines finished cleanly.
	case <-time.After(sseWaitTimeout):
		slog.Warn("SSE goroutines did not drain within deadline; forcing shutdown",
			slog.Duration("timeout", sseWaitTimeout))
	}

	if err != nil {
		return fmt.Errorf("http server shutdown: %w", err)
	}
	return nil
}

// Handler returns the chi mux for use with httptest.NewServer in integration tests.
func (s *Server) Handler() http.Handler {
	return s.mux
}
