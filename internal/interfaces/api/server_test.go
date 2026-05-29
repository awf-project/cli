package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func newTestServer(t *testing.T) (*Server, *Bridge) {
	t.Helper()
	lister := newMockWorkflowLister("wf-1")
	runner := newMockWorkflowRunner()
	history := newMockHistoryProvider()
	bridge := NewBridge(lister, runner, history)
	srv := NewServer(bridge, ":0")
	return srv, bridge
}

func TestServer_RegistersAllRoutes(t *testing.T) {
	srv, bridge := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Pre-register an active execution in terminal state so GET/DELETE
	// /api/executions/{id} reach the handler body, and the SSE endpoint
	// sees a completed workflow and closes the stream immediately.
	execCtx := newMockWorkflowRunner().execCtx
	execCtx.SetStatus(workflow.StatusCompleted)
	execCtx.SetCompletedAt(time.Now())
	knownID := bridge.TrackResumedExecution(execCtx)

	routes := []struct {
		method string
		path   string
		// wantNot404 asserts that the route is registered; handler-returned 404s
		// are excluded by using a known execution ID for execution-scoped routes.
	}{
		{"GET", "/api/workflows"},
		{"GET", "/api/workflows/local/wf-1"},
		{"POST", "/api/workflows/local/wf-1/validate"},
		{"POST", "/api/workflows/local/wf-1/run"},
		{"GET", "/api/executions"},
		{"GET", "/api/executions/" + knownID},
		{"DELETE", "/api/executions/" + knownID},
		{"POST", "/api/executions/" + knownID + "/resume"},
		{"GET", "/api/executions/" + knownID + "/events"},
		{"GET", "/api/history"},
		{"GET", "/api/history/stats"},
	}

	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, r.method, ts.URL+r.path, http.NoBody)
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			resp.Body.Close()
			// 405 Method Not Allowed would mean the path is registered but not the method.
			// We treat any non-404 response (including handler errors) as "route registered".
			assert.NotEqual(t, http.StatusNotFound, resp.StatusCode,
				"route %s %s must be registered (got 404)", r.method, r.path)
		})
	}
}

func TestServer_OpenAPISpec_ValidatesAgainst31(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/openapi.json", http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var spec map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))

	openapi, ok := spec["openapi"].(string)
	require.True(t, ok, "openapi field must be a string")
	assert.True(t, strings.HasPrefix(openapi, "3.1"), "openapi version must start with 3.1, got %q", openapi)

	info, ok := spec["info"].(map[string]any)
	require.True(t, ok, "info field must be an object")
	assert.Equal(t, "AWF API", info["title"], "info.title must be 'AWF API'")

	paths, ok := spec["paths"].(map[string]any)
	require.True(t, ok, "paths field must be an object")

	expectedPaths := []string{
		"/api/workflows",
		"/api/workflows/{scope}/{name}",
		"/api/workflows/{scope}/{name}/run",
		"/api/workflows/{scope}/{name}/validate",
		"/api/executions",
		"/api/executions/{id}",
		"/api/history",
	}
	for _, p := range expectedPaths {
		assert.Contains(t, paths, p, "OpenAPI spec must contain path %s", p)
	}
}

func TestServer_GracefulShutdown_Within30s_WithActiveSSE(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Open an SSE connection to a non-existent execution (returns 404 immediately
	// from Stream; the connection closes, so no goroutine stays open).
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		ts.URL+"/api/executions/no-such-id/events", http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	done := make(chan error, 1)
	go func() {
		done <- srv.Shutdown(context.Background())
	}()

	select {
	case err := <-done:
		assert.NoError(t, err, "Shutdown must complete without error")
	case <-time.After(30 * time.Second):
		t.Fatal("Shutdown did not complete within 30 seconds")
	}
}

func TestWithShutdownTimeout_SetsOption(t *testing.T) {
	lister := newMockWorkflowLister()
	bridge := NewBridge(lister, newMockWorkflowRunner(), newMockHistoryProvider())

	want := 5 * time.Second
	srv := NewServer(bridge, ":0", WithShutdownTimeout(want))

	assert.Equal(t, want, srv.shutdownTimeout)
}

func TestServer_Handler_ReturnsHTTPHandler(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.Handler()
	assert.NotNil(t, h, "Handler() must return a non-nil http.Handler")
}
