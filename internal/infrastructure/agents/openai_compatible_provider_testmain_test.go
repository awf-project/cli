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

// testToMockTransport redirects all HTTP requests from test base_url values
// to the single mock server. Matches on req.URL.Host (hostname:port) so that
// localhost:<port> entries are intercepted even when a real service (e.g. Ollama)
// occupies that port.
type testToMockTransport struct {
	hostMap map[string]string // host or host:port → mock host:port
	base    http.RoundTripper
}

func (t *testToMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if local, ok := t.hostMap[req.URL.Host]; ok {
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

// TestMain starts a single mock Chat Completions server on a random available
// port, then redirects every test base_url (including localhost variants) to it.
// This avoids conflicts with real services (e.g. Ollama) running on well-known ports.
func TestMain(m *testing.M) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", mockChatCompletionsHandler)

	lc := &net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start mock server: %v\n", err)
		os.Exit(1)
	}
	mockAddr := listener.Addr().String()

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go server.Serve(listener) //nolint:errcheck // background test server

	original := http.DefaultTransport
	http.DefaultTransport = &testToMockTransport{
		hostMap: map[string]string{
			"localhost:11434":    mockAddr,
			"localhost:8000":     mockAddr,
			"api.openai.com":     mockAddr,
			"ollama.example.com": mockAddr,
		},
		base: original,
	}

	os.Setenv("OPENAI_BASE_URL", "http://"+mockAddr+"/v1") //nolint:errcheck // test env setup
	os.Setenv("OPENAI_MODEL", "test-model")                //nolint:errcheck // test env setup

	exitCode := m.Run()
	server.Close()
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
