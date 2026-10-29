package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

func newTestServer(t *testing.T) (*Server, *Bridge) {
	t.Helper()
	bridge := NewBridge()
	srv := NewServer(bridge, ":0")
	return srv, bridge
}

func TestServer_RegistersAllRoutes(t *testing.T) {
	srv, bridge := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Pre-register an active execution so GET/DELETE /api/executions/{id} reach the
	// handler body, and the SSE route is verified as registered (returns non-404).
	knownID, _ := seedExecution(bridge, "wf-1")

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
	const (
		executionID         = "active-sse-execution"
		workflowInput       = "workflow-input-must-not-appear"
		commandOutput       = "command-output-must-not-appear"
		prompt              = "prompt-must-not-appear"
		environment         = "environment-must-not-appear"
		requestBody         = "request-body-must-not-appear"
		secretHeader        = "authorization-must-not-appear"
		pluginConfiguration = "plugin-config-must-not-appear"
		token               = "token-must-not-appear"
		confidentialMarker  = "credential-must-not-appear"
	)

	session := newMockRunSession(executionID)
	session.events <- ports.Event{Kind: ports.EventRunStarted, RunID: executionID}
	lookup := newMockSessionLookup()
	lookup.add(session)

	shutdownTimeout := 100 * time.Millisecond
	var logOutput bytes.Buffer
	previousRequestLogger := chiMiddleware.DefaultLogger
	chiMiddleware.DefaultLogger = chiMiddleware.RequestLogger(&chiMiddleware.DefaultLogFormatter{
		Logger:  log.New(&logOutput, "", 0),
		NoColor: true,
	})
	t.Cleanup(func() { chiMiddleware.DefaultLogger = previousRequestLogger })
	srv := NewServer(NewBridge(), ":0", WithSessionRegistry(lookup), WithShutdownTimeout(shutdownTimeout))
	ts := httptest.NewUnstartedServer(srv.Handler())
	srv.httpSrv = ts.Config
	ts.Start()

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet,
		ts.URL+"/api/executions/"+executionID+"/events", strings.NewReader(requestBody))
	require.NoError(t, err)
	req.Header.Set("Authorization", secretHeader)
	req.Header.Set("X-Workflow-Input", workflowInput)
	req.Header.Set("X-Command-Output", commandOutput)
	req.Header.Set("X-Prompt", prompt)
	req.Header.Set("X-Environment", environment)
	req.Header.Set("X-Plugin-Configuration", pluginConfiguration)
	req.Header.Set("X-Token", token)
	req.Header.Set("X-Credential", confidentialMarker)

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() {
		resp.Body.Close()
		ts.Close()
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	streamDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(io.Discard, resp.Body)
		streamDone <- copyErr
	}()

	done := make(chan error, 1)
	go func() {
		done <- srv.Shutdown(context.Background())
	}()

	shutdownExceededBound := false
	select {
	case err := <-done:
		require.NoError(t, err, "shutdown must cancel the active stream and complete gracefully")
	case <-time.After(5 * shutdownTimeout):
		cancelRequest()
		select {
		case err := <-done:
			require.ErrorIs(t, err, context.DeadlineExceeded,
				"shutdown timeout must preserve context deadline sentinel identity")
			shutdownExceededBound = true
		case <-time.After(shutdownTimeout):
			t.Fatal("shutdown remained blocked after the active SSE request was cancelled")
		}
	}

	select {
	case err := <-streamDone:
		if shutdownExceededBound {
			assert.ErrorIs(t, err, context.Canceled,
				"the bounded failure path must cancel the active SSE request")
		} else {
			assert.NoError(t, err, "shutdown must close the active SSE response cleanly")
		}
	case <-time.After(shutdownTimeout):
		t.Fatal("active SSE response remained open after shutdown completed")
	}
	diagnostics := logOutput.String()
	require.NotEmpty(t, diagnostics, "the active SSE shutdown path must emit observable diagnostics")
	assert.Contains(t, diagnostics, executionID,
		"observable diagnostics must identify the SSE execution being shut down")
	for _, secret := range []string{
		workflowInput, commandOutput, prompt, environment, requestBody,
		secretHeader, pluginConfiguration, token, confidentialMarker,
	} {
		assert.NotContains(t, diagnostics, secret,
			"shutdown diagnostics must not disclose confidential request data")
	}
	if shutdownExceededBound {
		t.Fatal("shutdown exceeded its configured bound with an active SSE stream")
	}
}

func TestWithShutdownTimeout_SetsOption(t *testing.T) {
	bridge := NewBridge()

	want := 5 * time.Second
	srv := NewServer(bridge, ":0", WithShutdownTimeout(want))

	assert.Equal(t, want, srv.shutdownTimeout)
}

func TestServer_Handler_ReturnsHTTPHandler(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.Handler()
	assert.NotNil(t, h, "Handler() must return a non-nil http.Handler")
}
