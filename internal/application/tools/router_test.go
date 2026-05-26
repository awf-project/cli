package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
)

// Test helpers: mock implementations

type mockToolProvider struct {
	tools         []ports.ToolDefinition
	listToolsErr  error
	callToolErr   error
	closeErr      error
	callToolCalls atomic.Int32 // atomic to avoid data races in concurrent tests
	closeCalls    atomic.Int32 // atomic to avoid data races in concurrent tests
}

func (m *mockToolProvider) ListTools(ctx context.Context) ([]ports.ToolDefinition, error) {
	if m.listToolsErr != nil {
		return nil, m.listToolsErr
	}
	return m.tools, nil
}

func (m *mockToolProvider) CallTool(ctx context.Context, name string, args map[string]any) (*ports.ToolResult, error) {
	m.callToolCalls.Add(1)
	if m.callToolErr != nil {
		return nil, m.callToolErr
	}
	return &ports.ToolResult{
		Content: []ports.ToolContent{{Type: "text", Text: "result"}},
		IsError: false,
	}, nil
}

func (m *mockToolProvider) Close(ctx context.Context) error {
	m.closeCalls.Add(1)
	return m.closeErr
}

type mockSpan struct {
	name   string
	attrs  map[string]any
	errors []error
	events []string
}

func (s *mockSpan) End()                             {}
func (s *mockSpan) SetAttribute(key string, val any) { s.attrs[key] = val }
func (s *mockSpan) RecordError(err error)            { s.errors = append(s.errors, err) }
func (s *mockSpan) AddEvent(name string)             { s.events = append(s.events, name) }

type mockTracer struct {
	mu    sync.Mutex
	spans []*mockSpan
}

func (t *mockTracer) Start(ctx context.Context, spanName string) (context.Context, ports.Span) {
	span := &mockSpan{name: spanName, attrs: make(map[string]any)}
	t.mu.Lock()
	t.spans = append(t.spans, span)
	t.mu.Unlock()
	return ctx, span
}

type mockLogger struct {
	mu        sync.Mutex
	infoLogs  []string
	errorLogs []string
	fields    []map[string]any
}

func (l *mockLogger) Debug(msg string, fields ...any) {}
func (l *mockLogger) Info(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infoLogs = append(l.infoLogs, msg)
	if len(fields) > 0 {
		l.fields = append(l.fields, parseFields(fields))
	}
}
func (l *mockLogger) Warn(msg string, fields ...any) {}
func (l *mockLogger) Error(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errorLogs = append(l.errorLogs, msg)
}
func (l *mockLogger) WithContext(ctx map[string]any) ports.Logger { return l }

func parseFields(fields []any) map[string]any {
	result := make(map[string]any)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			result[key] = fields[i+1]
		}
	}
	return result
}

// Tests

func TestNewRouter_EmptyRegistry(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}

	router := NewRouter(tracer, logger)

	require.NotNil(t, router)
	tools, err := router.ListTools(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, tools)
}

func TestRouter_Register_SingleProvider(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Description: "Test tool", Source: "test"},
		},
	}

	err := router.Register(context.Background(), provider)
	require.NoError(t, err)

	tools, err := router.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "tool1", tools[0].Name)
}

func TestRouter_Register_MultipleProviders(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider1 := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Description: "First tool", Source: "p1"},
		},
	}
	provider2 := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool2", Description: "Second tool", Source: "p2"},
		},
	}

	err := router.Register(context.Background(), provider1)
	require.NoError(t, err)
	err = router.Register(context.Background(), provider2)
	require.NoError(t, err)

	tools, err := router.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 2)
}

func TestRouter_Register_NameCollision_ReturnsError(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider1 := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Description: "First", Source: "p1"},
		},
	}
	provider2 := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Description: "Duplicate", Source: "p2"},
		},
	}

	err := router.Register(context.Background(), provider1)
	require.NoError(t, err)

	err = router.Register(context.Background(), provider2)
	require.Error(t, err)

	var structErr *domerrors.StructuredError
	assert.True(t, errors.As(err, &structErr))
	assert.Equal(t, domerrors.ErrorCodeUserMCPProxyNameCollision, structErr.Code)
}

func TestRouter_Register_ListToolsError_WrapsError(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	testErr := errors.New("provider failed")
	provider := &mockToolProvider{
		listToolsErr: testErr,
	}

	err := router.Register(context.Background(), provider)
	require.Error(t, err)
	assert.True(t, errors.Is(err, testErr))
}

func TestRouter_CallTool_RoutesToProvider(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "test"},
		},
	}
	router.Register(context.Background(), provider)

	result, err := router.CallTool(context.Background(), "tool1", map[string]any{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int32(1), provider.callToolCalls.Load())
}

func TestRouter_CallTool_UnknownName_ReturnsError(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "test"},
		},
	}
	router.Register(context.Background(), provider)

	result, err := router.CallTool(context.Background(), "unknown", map[string]any{})
	assert.Error(t, err)
	assert.Nil(t, result)

	var structErr *domerrors.StructuredError
	assert.True(t, errors.As(err, &structErr))
}

// TestRouter_CallTool_SpanNameIncludesToolName verifies AC 2.1:
// span name must be "tool.call.<name>".
func TestRouter_CallTool_SpanNameIncludesToolName(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "mytool", Source: "src"},
		},
	}
	router.Register(context.Background(), provider)

	router.CallTool(context.Background(), "mytool", map[string]any{})

	require.Len(t, tracer.spans, 1)
	assert.Equal(t, "tool.call.mytool", tracer.spans[0].name)
}

// TestRouter_CallTool_SpanAttributesPresent verifies AC 2.2:
// span must have tool.name, tool.source, and tool.duration_ms attributes.
func TestRouter_CallTool_SpanAttributesPresent(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "my-source"},
		},
	}
	router.Register(context.Background(), provider)

	router.CallTool(context.Background(), "tool1", map[string]any{})

	require.Len(t, tracer.spans, 1)
	span := tracer.spans[0]

	assert.Equal(t, "tool1", span.attrs["tool.name"], "tool.name attribute must be set")
	assert.Equal(t, "my-source", span.attrs["tool.source"], "tool.source attribute must be set")
	_, hasDuration := span.attrs["tool.duration_ms"]
	assert.True(t, hasDuration, "tool.duration_ms attribute must be set")
}

// TestRouter_CallTool_SingleInfoLog verifies AC 2.3:
// exactly one Info log emitted per CallTool with fields tool, source, duration.
// The "error" field is only present when CallTool returns an error (no nil noise).
func TestRouter_CallTool_SingleInfoLog(t *testing.T) {
	tests := []struct {
		name        string
		callToolErr error
		wantError   bool
	}{
		{name: "success", callToolErr: nil, wantError: false},
		{name: "failure", callToolErr: errors.New("boom"), wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := &mockTracer{}
			logger := &mockLogger{}
			router := NewRouter(tracer, logger)

			provider := &mockToolProvider{
				tools: []ports.ToolDefinition{
					{Name: "tool1", Source: "src"},
				},
				callToolErr: tt.callToolErr,
			}
			router.Register(context.Background(), provider)

			router.CallTool(context.Background(), "tool1", map[string]any{})

			logger.mu.Lock()
			infoCount := len(logger.infoLogs)
			fields := logger.fields
			logger.mu.Unlock()

			assert.Equal(t, 1, infoCount, "exactly one Info log must be emitted per CallTool")
			require.Len(t, fields, 1)

			f := fields[0]
			assert.Contains(t, f, "tool", "log must contain 'tool' field")
			assert.Contains(t, f, "source", "log must contain 'source' field")
			assert.Contains(t, f, "duration", "log must contain 'duration' field")
			if tt.wantError {
				assert.Contains(t, f, "error", "log must contain 'error' field on failure")
			} else {
				assert.NotContains(t, f, "error", "log must NOT contain 'error' field on success (no nil noise)")
			}
		})
	}
}

func TestRouter_CallTool_RecordsErrorOnSpan(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	testErr := errors.New("tool failed")
	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "test"},
		},
		callToolErr: testErr,
	}
	router.Register(context.Background(), provider)

	router.CallTool(context.Background(), "tool1", map[string]any{})

	require.Len(t, tracer.spans, 1)
	span := tracer.spans[0]
	assert.NotEmpty(t, span.errors)
}

func TestRouter_Close_CallsAllProviders(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider1 := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "p1"},
		},
	}
	provider2 := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool2", Source: "p2"},
		},
	}

	router.Register(context.Background(), provider1)
	router.Register(context.Background(), provider2)

	err := router.Close(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int32(1), provider1.closeCalls.Load())
	assert.Equal(t, int32(1), provider2.closeCalls.Load())
}

func TestRouter_Close_AggregatesErrors(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	err1 := errors.New("close failed 1")
	err2 := errors.New("close failed 2")

	provider1 := &mockToolProvider{
		tools:    []ports.ToolDefinition{{Name: "tool1", Source: "p1"}},
		closeErr: err1,
	}
	provider2 := &mockToolProvider{
		tools:    []ports.ToolDefinition{{Name: "tool2", Source: "p2"}},
		closeErr: err2,
	}

	router.Register(context.Background(), provider1)
	router.Register(context.Background(), provider2)

	err := router.Close(context.Background())
	assert.Error(t, err)
}

func TestRouter_ConcurrentRegisterAndCallTool(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	var wg sync.WaitGroup
	var registerCount, callCount int32

	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			provider := &mockToolProvider{
				tools: []ports.ToolDefinition{
					{Name: fmt.Sprintf("tool%d", idx), Source: "test"},
				},
			}
			if err := router.Register(context.Background(), provider); err == nil {
				atomic.AddInt32(&registerCount, 1)
			}
		}(i)
	}

	wg.Wait()

	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := router.CallTool(context.Background(), fmt.Sprintf("tool%d", idx), map[string]any{})
			if err == nil {
				atomic.AddInt32(&callCount, 1)
			}
		}(i)
	}

	wg.Wait()

	assert.True(t, registerCount > 0)
	assert.True(t, callCount > 0)
}

func TestRouter_ListTools_AggregatesFromAllProviders(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider1 := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "p1"},
			{Name: "tool2", Source: "p1"},
		},
	}
	provider2 := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool3", Source: "p2"},
		},
	}

	router.Register(context.Background(), provider1)
	router.Register(context.Background(), provider2)

	tools, err := router.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 3)
}
