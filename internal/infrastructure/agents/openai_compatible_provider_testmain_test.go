package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// mockPorts lists local ports where mock Chat Completions servers are started.
// Matches the base_url values used in unit tests that run against local endpoints.
var mockPorts = []string{"11434", "8000"}

// externalToLocalTransport redirects requests for known test hostnames to the
// local mock server. Required because unit tests use realistic (non-localhost)
// base_url values to verify URL normalization, and those hosts are unreachable
// in CI/offline environments.
type externalToLocalTransport struct {
	hostMap map[string]string // hostname → local host:port
	base    http.RoundTripper
}

func (t *externalToLocalTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if local, ok := t.hostMap[req.URL.Hostname()]; ok {
		r := req.Clone(req.Context())
		r.URL.Scheme = "http"
		r.URL.Host = local
		resp, err := t.base.RoundTrip(r)
		if err != nil {
			return nil, fmt.Errorf("redirected request failed: %w", err)
		}
		return resp, nil
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}

// TestMain starts mock Chat Completions servers on local ports used by unit tests
// so that tests can make real HTTP calls without requiring Ollama or other services.
// If a port is already in use (e.g., real Ollama is running), the tests use the live server.
func TestMain(m *testing.M) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", mockChatCompletionsHandler)

	lc := &net.ListenConfig{}
	var servers []*http.Server
	for _, port := range mockPorts {
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:"+port)
		if err == nil {
			server := &http.Server{
				Handler:           mux,
				ReadHeaderTimeout: 5 * time.Second,
			}
			servers = append(servers, server)
			go server.Serve(listener) //nolint:errcheck // background test server; errors are acceptable in test setup
		}
	}

	// Redirect external test hostnames to the local mock server so tests work
	// offline and in CI without network access.
	original := http.DefaultTransport
	http.DefaultTransport = &externalToLocalTransport{
		hostMap: map[string]string{
			"api.openai.com":     "127.0.0.1:11434",
			"ollama.example.com": "127.0.0.1:11434",
		},
		base: original,
	}

	// Point env-var-only callers (tests that pass nil options) to the mock server.
	os.Setenv("OPENAI_BASE_URL", "http://localhost:11434/v1") //nolint:errcheck // test env setup; failure is non-critical
	os.Setenv("OPENAI_MODEL", "test-model")                   //nolint:errcheck // test env setup; failure is non-critical

	exitCode := m.Run()

	for _, s := range servers {
		s.Close()
	}

	os.Exit(exitCode)
}

func mockChatCompletionsHandler(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"id":     "chatcmpl-test",
		"object": "chat.completion",
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "This is a mock response from the test server.",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 15,
			"total_tokens":      25,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck // test helper; response encoding error is non-critical
}
