package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		name         string
		options      []Option
		wantTimeout  time.Duration
		checkDoer    bool
		doerIsCustom bool
	}{
		{
			name:        "defaults",
			options:     nil,
			wantTimeout: 30 * time.Second,
			checkDoer:   false,
		},
		{
			name:        "custom timeout",
			options:     []Option{WithTimeout(5 * time.Second)},
			wantTimeout: 5 * time.Second,
			checkDoer:   false,
		},
		{
			name:         "custom doer",
			options:      []Option{WithDoer(&mockHTTPDoer{})},
			wantTimeout:  30 * time.Second,
			checkDoer:    true,
			doerIsCustom: true,
		},
		{
			name: "multiple options",
			options: []Option{
				WithTimeout(10 * time.Second),
				WithDoer(&mockHTTPDoer{}),
			},
			wantTimeout:  10 * time.Second,
			checkDoer:    true,
			doerIsCustom: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.options...)

			assert.NotNil(t, client)
			assert.NotNil(t, client.doer)
			assert.Equal(t, tt.wantTimeout, client.timeout)

			if tt.checkDoer {
				if tt.doerIsCustom {
					_, ok := client.doer.(*mockHTTPDoer)
					assert.True(t, ok)
				} else {
					_, ok := client.doer.(*http.Client)
					assert.True(t, ok)
				}
			}
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
	tests := []struct {
		name          string
		clientTimeout time.Duration
		contextSetup  func() (context.Context, context.CancelFunc)
		serverDelay   time.Duration
		wantErr       bool
		checkErr      func(*testing.T, error)
		maxElapsed    time.Duration
	}{
		{
			name:          "context cancellation",
			clientTimeout: 30 * time.Second,
			contextSetup: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			serverDelay: 100 * time.Millisecond,
			wantErr:     true,
			checkErr: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, context.Canceled))
			},
			maxElapsed: 50 * time.Millisecond,
		},
		{
			name:          "client timeout applied",
			clientTimeout: 50 * time.Millisecond,
			contextSetup: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			serverDelay: 200 * time.Millisecond,
			wantErr:     true,
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
			maxElapsed: 150 * time.Millisecond,
		},
		{
			name:          "context deadline takes precedence",
			clientTimeout: 5 * time.Second,
			contextSetup: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 50*time.Millisecond) //nolint:gosec // cancel is returned to caller and deferred at call site
			},
			serverDelay: 100 * time.Millisecond,
			wantErr:     true,
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
			maxElapsed: 150 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.serverDelay)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := NewClient(WithTimeout(tt.clientTimeout))
			ctx, cancel := tt.contextSetup()
			defer cancel()

			start := time.Now()
			_, err := client.Get(ctx, server.URL, nil, 1024)
			elapsed := time.Since(start)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
				if tt.maxElapsed > 0 {
					assert.Less(t, elapsed, tt.maxElapsed)
				}
			} else {
				assert.NoError(t, err)
			}
		})
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

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	m.called = true
	return m.response, m.err
}
