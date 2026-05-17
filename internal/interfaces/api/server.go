package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// Option configures a Server on construction.
type Option func(*Server)

// WithShutdownTimeout overrides the default 30s graceful shutdown timeout.
func WithShutdownTimeout(d time.Duration) Option {
	return func(s *Server) {
		s.shutdownTimeout = d
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

	RegisterWorkflowRoutes(s.api, NewWorkflowHandlers(bridge))
	RegisterExecutionRoutes(s.api, NewExecutionHandlers(bridge))
	RegisterSSERoutes(s.api, NewSSEHandler(bridge, &s.sseWG))
	RegisterHistoryRoutes(s.api, NewHistoryHandlers(bridge))

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

// Shutdown gracefully stops the HTTP server within shutdownTimeout and waits for active SSE goroutines.
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, s.shutdownTimeout)
	defer cancel()
	err := s.httpSrv.Shutdown(shutdownCtx)
	s.sseWG.Wait()
	if err != nil {
		return fmt.Errorf("http server shutdown: %w", err)
	}
	return nil
}

// Handler returns the chi mux for use with httptest.NewServer in integration tests.
func (s *Server) Handler() http.Handler {
	return s.mux
}
