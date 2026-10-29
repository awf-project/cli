package httpx

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPDoerInterface(t *testing.T) {
	var _ HTTPDoer = (*http.Client)(nil)
}

func TestClientConstruction(t *testing.T) {
	tests := []struct {
		name        string
		options     []Option
		wantTimeout time.Duration
	}{
		{
			name:        "defaults",
			options:     nil,
			wantTimeout: 0,
		},
		{
			name:        "custom timeout",
			options:     []Option{WithTimeout(5 * time.Second)},
			wantTimeout: 5 * time.Second,
		},
		{
			name:        "non-positive timeout remains disabled",
			options:     []Option{WithTimeout(-time.Second)},
			wantTimeout: 0,
		},
		{
			name:        "zero timeout remains disabled",
			options:     []Option{WithTimeout(0)},
			wantTimeout: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.options...)

			assert.NotNil(t, client)
			assert.Equal(t, tt.wantTimeout, client.timeout)
		})
	}

	t.Run("default transport bounds setup without a total execution limit", func(t *testing.T) {
		client := NewClient()
		httpClient, ok := client.doer.(*http.Client)
		require.True(t, ok)
		assert.Zero(t, httpClient.Timeout)
		transport, ok := httpClient.Transport.(*http.Transport)
		require.True(t, ok, "default construction must expose its execution transport instead of inheriting hidden defaults")
		assert.Zero(t, transport.ResponseHeaderTimeout)
		assert.Equal(t, 10*time.Second, transport.TLSHandshakeTimeout)
		assert.Equal(t, 30*time.Second, newConnectionDialer().Timeout)
		require.NotNil(t, transport.DialContext)
	})

	t.Run("custom dialer receives no implicit total request deadline", func(t *testing.T) {
		dialer := newContextBlockingDialer()
		client := NewClient(WithDoer(&http.Client{Transport: &http.Transport{DialContext: dialer.DialContext}}))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		result := startClientGet(client, ctx)
		observed := receiveContext(t, dialer.observed)
		_, hasDeadline := observed.Deadline()
		assert.False(t, hasDeadline, "the wrapper must preserve caller-owned custom transport policy")

		cancel()
		assert.ErrorIs(t, receiveClientError(t, result), context.Canceled)
	})

	t.Run("user supplied limits remain untouched", func(t *testing.T) {
		userTransport := &http.Transport{
			ResponseHeaderTimeout: 3 * time.Second,
			TLSHandshakeTimeout:   4 * time.Second,
		}
		userClient := &http.Client{Timeout: 5 * time.Second, Transport: userTransport}
		configured := NewClient(WithDoer(userClient))
		assert.Same(t, userClient, configured.doer)
		assert.Equal(t, 5*time.Second, userClient.Timeout)
		assert.Same(t, userTransport, userClient.Transport)
	})
}

type connectionSetupCase struct {
	name           string
	setupTimeout   time.Duration
	parentTimeout  time.Duration
	cancel         bool
	wantContextErr error
}

func connectionSetupCases() []connectionSetupCase {
	return []connectionSetupCase{
		{name: "technical setup bound", setupTimeout: 100 * time.Millisecond},
		{name: "shorter parent deadline", setupTimeout: time.Second, parentTimeout: 100 * time.Millisecond, wantContextErr: context.DeadlineExceeded},
		{name: "caller cancellation", setupTimeout: time.Second, cancel: true, wantContextErr: context.Canceled},
	}
}

func (c connectionSetupCase) context() (context.Context, context.CancelFunc) {
	if c.parentTimeout > 0 {
		return context.WithTimeout(context.Background(), c.parentTimeout)
	}
	return context.WithCancel(context.Background())
}

func (c connectionSetupCase) assertError(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	if c.wantContextErr != nil {
		assert.ErrorIs(t, err, c.wantContextErr)
		return
	}
	var timeoutError net.Error
	require.ErrorAs(t, err, &timeoutError)
	assert.True(t, timeoutError.Timeout(), "connection setup must expire without a caller deadline")
}

func TestClientConnectionSetupBounds(t *testing.T) {
	for _, testCase := range connectionSetupCases() {
		t.Run("TCP/"+testCase.name, func(t *testing.T) { testTCPSetupBound(t, testCase) })
		t.Run("TLS/"+testCase.name, func(t *testing.T) { testTLSSetupBound(t, testCase) })
	}
}

func testTCPSetupBound(t *testing.T, testCase connectionSetupCase) {
	t.Helper()
	dialer := newConnectionDialer()
	dialer.Timeout = testCase.setupTimeout
	observed := make(chan context.Context, 1)
	// Block before connect so expiry is deterministic and does not contact a server.
	dialer.ControlContext = func(ctx context.Context, _, _ string, _ syscall.RawConn) error {
		observed <- ctx
		<-ctx.Done()
		return ctx.Err()
	}
	ctx, cancel := testCase.context()
	defer cancel()
	result := make(chan error, 1)
	go func() {
		conn, err := dialer.DialContext(ctx, "tcp4", "127.0.0.1:1")
		if conn != nil {
			_ = conn.Close()
		}
		result <- err
	}()
	actual := receiveContext(t, observed)
	deadline, bounded := actual.Deadline()
	require.True(t, bounded, "the technical dial budget must bound setup")
	if testCase.parentTimeout > 0 {
		parentDeadline, ok := ctx.Deadline()
		require.True(t, ok)
		assert.Equal(t, parentDeadline, deadline)
	}
	if testCase.cancel {
		cancel()
	}
	testCase.assertError(t, receiveClientError(t, result))
}

func testTLSSetupBound(t *testing.T, testCase connectionSetupCase) {
	t.Helper()
	transport := newExecutionTransport()
	transport.Proxy = nil
	t.Cleanup(transport.CloseIdleConnections)
	transport.TLSHandshakeTimeout = testCase.setupTimeout
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() { _ = clientConn.Close(); _ = serverConn.Close() })
	dialed, drained := make(chan struct{}), make(chan struct{})
	transport.DialContext = func(context.Context, string, string) (net.Conn, error) {
		close(dialed)
		return clientConn, nil
	}
	go func() {
		defer close(drained)
		// Consume ClientHello but never answer it, keeping the handshake incomplete.
		_, _ = io.Copy(io.Discard, serverConn)
	}()
	ctx, cancel := testCase.context()
	defer cancel()
	client := NewClient(WithDoer(&http.Client{Transport: transport}))
	result := startClientGetTo(client, ctx, "https://example.invalid")
	receiveSignal(t, dialed)
	if testCase.cancel {
		cancel()
	}
	testCase.assertError(t, receiveClientError(t, result))
	// net/http may keep a canceled dial for reuse. Close the test-owned pool
	// before requiring its incomplete handshake and peer reader to terminate.
	transport.CloseIdleConnections()
	receiveSignal(t, drained)
}

func TestClientTLSSetupBudgetDoesNotLimitResponse(t *testing.T) {
	for _, waitPoint := range []string{"headers", "body"} {
		t.Run(waitPoint, func(t *testing.T) {
			started, stopped := make(chan struct{}), make(chan struct{})
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if waitPoint == "body" {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				close(started)
				<-r.Context().Done()
				close(stopped)
			}))
			defer server.Close()
			transport := newExecutionTransport()
			serverTransport, ok := server.Client().Transport.(*http.Transport)
			require.True(t, ok)
			transport.TLSClientConfig = serverTransport.TLSClientConfig.Clone()
			transport.TLSHandshakeTimeout = 100 * time.Millisecond
			t.Cleanup(transport.CloseIdleConnections)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := NewClient(WithDoer(&http.Client{Transport: transport}))
			result := startClientGetTo(client, ctx, server.URL)
			receiveSignal(t, started)
			select {
			case err := <-result:
				t.Fatalf("completed TLS setup limited response %s: %v", waitPoint, err)
			case <-time.After(2 * transport.TLSHandshakeTimeout):
			}
			cancel()
			assert.ErrorIs(t, receiveClientError(t, result), context.Canceled)
			receiveSignal(t, stopped)
		})
	}
}

func TestClientHeaders(t *testing.T) {
	tests := []struct {
		name            string
		requestHeaders  map[string]string
		responseHeaders func(http.ResponseWriter)
		checkRequest    func(*testing.T, *http.Request)
		checkResponse   func(*testing.T, *Response)
	}{
		{
			name: "request headers forwarded",
			requestHeaders: map[string]string{
				"Authorization":   "Bearer token123",
				"Content-Type":    "application/json",
				"X-Custom-Header": "custom-value",
			},
			responseHeaders: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusOK)
			},
			checkRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"))
			},
			checkResponse: func(t *testing.T, resp *Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			},
		},
		{
			name:           "response headers captured",
			requestHeaders: nil,
			responseHeaders: func(w http.ResponseWriter) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Request-Id", "abc123")
				w.WriteHeader(http.StatusOK)
			},
			checkRequest: func(t *testing.T, r *http.Request) {},
			checkResponse: func(t *testing.T, resp *Response) {
				assert.Equal(t, "application/json", resp.Headers["Content-Type"])
				assert.Equal(t, "abc123", resp.Headers["X-Request-Id"])
			},
		},
		{
			name:           "multi-value headers joined",
			requestHeaders: nil,
			responseHeaders: func(w http.ResponseWriter) {
				w.Header().Add("Set-Cookie", "session=xyz")
				w.Header().Add("Set-Cookie", "token=abc")
				w.WriteHeader(http.StatusOK)
			},
			checkRequest: func(t *testing.T, r *http.Request) {},
			checkResponse: func(t *testing.T, resp *Response) {
				assert.Equal(t, "session=xyz, token=abc", resp.Headers["Set-Cookie"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.checkRequest(t, r)
				tt.responseHeaders(w)
			}))
			defer server.Close()

			client := NewClient()
			resp, err := client.Get(context.Background(), server.URL, tt.requestHeaders, 1024)

			require.NoError(t, err)
			tt.checkResponse(t, resp)
		})
	}
}

func TestClientContextAndTimeout(t *testing.T) {
	t.Run("omitted timeout leaves request deadline-free until caller cancellation", func(t *testing.T) {
		doer := newContextBlockingDoer()
		client := NewClient(WithDoer(doer))
		ctx, cancel := context.WithCancel(context.Background())
		result := startClientGet(client, ctx)

		observed := <-doer.observed
		_, hasDeadline := observed.Deadline()
		assert.False(t, hasDeadline)
		cancel()
		assert.ErrorIs(t, <-result, context.Canceled)
	})

	t.Run("explicit timeout supplies deadline and preserves sentinel", func(t *testing.T) {
		doer := newContextBlockingDoer()
		client := NewClient(WithTimeout(40*time.Millisecond), WithDoer(doer))
		result := startClientGet(client, context.Background())

		observed := <-doer.observed
		deadline, hasDeadline := observed.Deadline()
		require.True(t, hasDeadline)
		assert.WithinDuration(t, time.Now().Add(40*time.Millisecond), deadline, 20*time.Millisecond)
		assert.ErrorIs(t, <-result, context.DeadlineExceeded)
	})

	t.Run("shorter parent deadline takes precedence", func(t *testing.T) {
		doer := newContextBlockingDoer()
		client := NewClient(WithTimeout(time.Second), WithDoer(doer))
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		defer cancel()
		parentDeadline, ok := ctx.Deadline()
		require.True(t, ok)
		result := startClientGet(client, ctx)

		requestDeadline, ok := (<-doer.observed).Deadline()
		require.True(t, ok)
		assert.Equal(t, parentDeadline, requestDeadline)
		assert.ErrorIs(t, <-result, context.DeadlineExceeded)
	})

	t.Run("explicit timeout takes precedence over a longer parent deadline", func(t *testing.T) {
		doer := newContextBlockingDoer()
		client := NewClient(WithTimeout(40*time.Millisecond), WithDoer(doer))
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		parentDeadline, ok := ctx.Deadline()
		require.True(t, ok)
		result := startClientGet(client, ctx)

		requestDeadline, ok := (<-doer.observed).Deadline()
		require.True(t, ok)
		assert.Less(t, requestDeadline, parentDeadline)
		assert.WithinDuration(t, time.Now().Add(40*time.Millisecond), requestDeadline, 20*time.Millisecond)
		assert.ErrorIs(t, <-result, context.DeadlineExceeded)
	})

	t.Run("default transport waits for headers until caller cancellation", func(t *testing.T) {
		handlerStarted := make(chan struct{})
		requestCanceled := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(handlerStarted)
			<-r.Context().Done()
			close(requestCanceled)
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		result := startClientGetTo(NewClient(), ctx, server.URL)
		receiveSignal(t, handlerStarted)
		assertClientStillWaiting(t, result)

		cancel()
		assert.ErrorIs(t, receiveClientError(t, result), context.Canceled)
		receiveSignal(t, requestCanceled)
	})

	t.Run("caller cancellation closes a delayed response body", func(t *testing.T) {
		bodyStarted := make(chan struct{})
		bodyClosed := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		body := &contextBlockingReadCloser{
			ctx:     ctx,
			started: bodyStarted,
			closed:  bodyClosed,
		}
		client := NewClient(WithDoer(&mockHTTPDoer{response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       body,
		}}))
		result := startClientGet(client, ctx)
		receiveSignal(t, bodyStarted)
		assertClientStillWaiting(t, result)

		cancel()
		assert.ErrorIs(t, receiveClientError(t, result), context.Canceled)
		receiveSignal(t, bodyClosed)
	})

	t.Run("default transport enforces the configured body cap after a delayed body", func(t *testing.T) {
		bodyStarted := make(chan struct{})
		releaseBody := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			close(bodyStarted)
			<-releaseBody
			_, _ = io.WriteString(w, strings.Repeat("x", 16))
		}))
		defer server.Close()

		result := make(chan clientResult, 1)
		go func() {
			response, err := NewClient().Get(context.Background(), server.URL, nil, 8)
			result <- clientResult{response: response, err: err}
		}()
		receiveSignal(t, bodyStarted)
		assertClientResultStillWaiting(t, result)
		close(releaseBody)

		outcome := receiveClientResult(t, result)
		require.NoError(t, outcome.err)
		require.NotNil(t, outcome.response)
		assert.True(t, outcome.response.Truncated)
		assert.Len(t, outcome.response.Body, 8)
	})
}

func TestClientFailureDoesNotDiscloseConfidentialHTTPData(t *testing.T) {
	responseBody := &failingReadCloser{
		data: []byte("response-secret"),
		err:  errors.New("upstream read failed"),
	}
	client := NewClient(WithDoer(&mockHTTPDoer{response: &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     make(http.Header),
		Body:       responseBody,
	}}))

	_, err := client.Do(
		context.Background(),
		http.MethodPost,
		"http://password-secret:token-secret@example.invalid/private",
		map[string]string{"Authorization": "Bearer header-secret"},
		`{"credential":"body-secret"}`,
		1024,
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "external operation failed")
	assertNoConfidentialHTTPData(t, err.Error())
}

func assertNoConfidentialHTTPData(t *testing.T, captured string) {
	t.Helper()
	for _, confidential := range []string{
		"password-secret", "token-secret", "header-secret", "body-secret", "response-secret",
	} {
		if strings.Contains(captured, confidential) {
			t.Error("HTTP diagnostic disclosed confidential request or response data")
		}
	}
}

func TestClientBodyTruncation(t *testing.T) {
	tests := []struct {
		name         string
		responseSize int
		maxBodyBytes int64
		wantTrunc    bool
		wantBodyLen  int
	}{
		{
			name:         "body within limit",
			responseSize: 100,
			maxBodyBytes: 1024,
			wantTrunc:    false,
			wantBodyLen:  100,
		},
		{
			name:         "body exceeds limit",
			responseSize: 2048,
			maxBodyBytes: 1024,
			wantTrunc:    true,
			wantBodyLen:  1024,
		},
		{
			name:         "unlimited (maxBodyBytes=0)",
			responseSize: 10000,
			maxBodyBytes: 0,
			wantTrunc:    false,
			wantBodyLen:  10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseBody := strings.Repeat("A", tt.responseSize)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(responseBody))
			}))
			defer server.Close()

			client := NewClient()
			resp, err := client.Get(context.Background(), server.URL, nil, tt.maxBodyBytes)

			require.NoError(t, err)
			assert.Equal(t, tt.wantTrunc, resp.Truncated)
			assert.Len(t, resp.Body, tt.wantBodyLen)
		})
	}
}

func TestClientDo_AllMethods(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		body     string
		wantBody bool
	}{
		{
			name:     "GET without body",
			method:   http.MethodGet,
			body:     "",
			wantBody: false,
		},
		{
			name:     "POST with body",
			method:   http.MethodPost,
			body:     `{"data":"value"}`,
			wantBody: true,
		},
		{
			name:     "PUT with body",
			method:   http.MethodPut,
			body:     `{"update":"value"}`,
			wantBody: true,
		},
		{
			name:     "DELETE without body",
			method:   http.MethodDelete,
			body:     "",
			wantBody: false,
		},
		{
			name:     "PATCH with body",
			method:   http.MethodPatch,
			body:     `{"patch":"data"}`,
			wantBody: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.method, r.Method)

				if tt.wantBody {
					body, _ := io.ReadAll(r.Body)
					assert.Equal(t, tt.body, string(body))
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}))
			defer server.Close()

			client := NewClient()
			resp, err := client.Do(context.Background(), tt.method, server.URL, nil, tt.body, 1024)

			require.NoError(t, err)
			assert.Equal(t, "OK", resp.Body)
		})
	}
}

func TestClientDo_ErrorCases(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "invalid URL scheme",
			url:  "ftp://invalid.com",
		},
		{
			name: "non-existent host",
			url:  "http://nonexistent-host-12345.invalid",
		},
		{
			name: "malformed URL",
			url:  "ht!tp://invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(WithTimeout(1 * time.Second))

			_, err := client.Get(context.Background(), tt.url, nil, 1024)

			assert.Error(t, err)
		})
	}
}

func TestClientWithMockDoer(t *testing.T) {
	tests := []struct {
		name        string
		mockDoer    *mockHTTPDoer
		wantErr     bool
		checkResult func(*testing.T, *Response, *mockHTTPDoer)
	}{
		{
			name: "success response",
			mockDoer: &mockHTTPDoer{
				response: &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"X-Mock": []string{"true"}},
					Body:       io.NopCloser(strings.NewReader("mock response")),
				},
			},
			wantErr: false,
			checkResult: func(t *testing.T, resp *Response, m *mockHTTPDoer) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, "mock response", resp.Body)
				assert.Equal(t, "true", resp.Headers["X-Mock"])
				assert.True(t, m.called)
			},
		},
		{
			name: "error propagation",
			mockDoer: &mockHTTPDoer{
				err: errors.New("mock error"),
			},
			wantErr: true,
			checkResult: func(t *testing.T, resp *Response, m *mockHTTPDoer) {
				assert.True(t, m.called)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(WithDoer(tt.mockDoer))

			resp, err := client.Get(context.Background(), "http://example.com", nil, 1024)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			tt.checkResult(t, resp, tt.mockDoer)
		})
	}
}

func TestClientConvenienceMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.Method)) //nolint:gosec // G705: test handler writes method name, not user input
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	tests := []struct {
		name           string
		call           func() (*Response, error)
		expectedMethod string
	}{
		{
			name:           "Get",
			call:           func() (*Response, error) { return client.Get(ctx, server.URL, nil, 1024) },
			expectedMethod: http.MethodGet,
		},
		{
			name:           "Post",
			call:           func() (*Response, error) { return client.Post(ctx, server.URL, nil, "body", 1024) },
			expectedMethod: http.MethodPost,
		},
		{
			name:           "Put",
			call:           func() (*Response, error) { return client.Put(ctx, server.URL, nil, "body", 1024) },
			expectedMethod: http.MethodPut,
		},
		{
			name:           "Delete",
			call:           func() (*Response, error) { return client.Delete(ctx, server.URL, nil, 1024) },
			expectedMethod: http.MethodDelete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.call()

			require.NoError(t, err)
			assert.Equal(t, tt.expectedMethod, resp.Body)
		})
	}
}

func TestClientEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			body, _ := io.ReadAll(r.Body)
			assert.Empty(t, body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient()
	resp, err := client.Get(context.Background(), server.URL, nil, 1024)

	require.NoError(t, err)
	assert.Empty(t, resp.Body)
	assert.False(t, resp.Truncated)
}

func TestClientStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"204 No Content", http.StatusNoContent},
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"404 Not Found", http.StatusNotFound},
		{"429 Too Many Requests", http.StatusTooManyRequests},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"502 Bad Gateway", http.StatusBadGateway},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("response body"))
			}))
			defer server.Close()

			client := NewClient()
			resp, err := client.Get(context.Background(), server.URL, nil, 1024)

			require.NoError(t, err)
			assert.Equal(t, tt.statusCode, resp.StatusCode)
			if tt.statusCode == http.StatusNoContent {
				assert.Empty(t, resp.Body)
			} else {
				assert.Equal(t, "response body", resp.Body)
			}
		})
	}
}

type mockHTTPDoer struct {
	response *http.Response
	err      error
	called   bool
}

type contextBlockingDoer struct {
	observed chan context.Context
}

type contextBlockingDialer struct {
	observed chan context.Context
}

func newContextBlockingDialer() *contextBlockingDialer {
	return &contextBlockingDialer{observed: make(chan context.Context, 1)}
}

func (d *contextBlockingDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	d.observed <- ctx
	<-ctx.Done()
	return nil, ctx.Err()
}

func newContextBlockingDoer() *contextBlockingDoer {
	return &contextBlockingDoer{observed: make(chan context.Context, 1)}
}

func (d *contextBlockingDoer) Do(req *http.Request) (*http.Response, error) {
	d.observed <- req.Context()
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func startClientGet(client *Client, ctx context.Context) <-chan error {
	return startClientGetTo(client, ctx, "http://example.invalid")
}

func startClientGetTo(client *Client, ctx context.Context, url string) <-chan error {
	result := make(chan error, 1)
	go func() {
		_, err := client.Get(ctx, url, nil, 1024)
		result <- err
	}()
	return result
}

type clientResult struct {
	response *Response
	err      error
}

type contextBlockingReadCloser struct {
	ctx         context.Context
	started     chan<- struct{}
	closed      chan<- struct{}
	startedOnce sync.Once
	closedOnce  sync.Once
}

type failingReadCloser struct {
	data []byte
	err  error
}

func (r *failingReadCloser) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func (r *failingReadCloser) Close() error { return nil }

func (r *contextBlockingReadCloser) Read(_ []byte) (int, error) {
	r.startedOnce.Do(func() { close(r.started) })
	<-r.ctx.Done()
	return 0, r.ctx.Err()
}

func (r *contextBlockingReadCloser) Close() error {
	r.closedOnce.Do(func() { close(r.closed) })
	return nil
}

func receiveContext(t *testing.T, contexts <-chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-contexts:
		return ctx
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the transport to begin dialing")
		return nil
	}
}

func receiveSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the synchronized HTTP handler")
	}
}

func receiveClientError(t *testing.T, results <-chan error) error {
	t.Helper()
	select {
	case err := <-results:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the canceled HTTP request")
		return nil
	}
}

func receiveClientResult(t *testing.T, results <-chan clientResult) clientResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the released HTTP response body")
		return clientResult{}
	}
}

func assertClientStillWaiting(t *testing.T, results <-chan error) {
	t.Helper()
	select {
	case err := <-results:
		t.Fatalf("HTTP request ended before caller cancellation: %v", err)
	default:
	}
}

func assertClientResultStillWaiting(t *testing.T, results <-chan clientResult) {
	t.Helper()
	select {
	case result := <-results:
		t.Fatalf("HTTP request ended before the delayed body was released: %v", result.err)
	default:
	}
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	m.called = true
	return m.response, m.err
}
