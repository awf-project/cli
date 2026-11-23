package notifier

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type providerCall struct {
	ctx    context.Context
	name   string
	inputs map[string]any
}

type contextKey struct{}

type recordingProvider struct {
	mu         sync.Mutex
	operations map[string]*pluginmodel.OperationSchema
	execute    func(context.Context, string, map[string]any) (*pluginmodel.OperationResult, error)
	lookups    int
	lists      int
	calls      []providerCall
}

func (p *recordingProvider) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lookups++
	operation, found := p.operations[name]
	return operation, found
}

func (p *recordingProvider) ListOperations() []*pluginmodel.OperationSchema {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lists++
	operations := make([]*pluginmodel.OperationSchema, 0, len(p.operations))
	for _, operation := range p.operations {
		operations = append(operations, operation)
	}
	return operations
}

func (p *recordingProvider) Execute(
	ctx context.Context,
	name string,
	inputs map[string]any,
) (*pluginmodel.OperationResult, error) {
	p.mu.Lock()
	p.calls = append(p.calls, providerCall{ctx: ctx, name: name, inputs: inputs})
	execute := p.execute
	p.mu.Unlock()
	if execute != nil {
		return execute(ctx, name, inputs)
	}
	return &pluginmodel.OperationResult{Success: true}, nil
}

func (p *recordingProvider) counts() (int, int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lookups, p.lists, len(p.calls)
}

func (p *recordingProvider) recordedCalls() []providerCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	calls := make([]providerCall, len(p.calls))
	copy(calls, p.calls)
	return calls
}

func operation(name string) *pluginmodel.OperationSchema {
	return &pluginmodel.OperationSchema{Name: name, PluginName: "recording"}
}

func TestNotifierDelegatesOperationsToQualifiedAdapterOperations(t *testing.T) {
	tests := []struct {
		canonical string
		adapter   string
		inputs    map[string]any
	}{
		{OperationSend, "recording.send", map[string]any{"channel": "alerts", "text": "canary-text", "rich": map[string]any{"secret": "canary-rich"}, "thread_id": "thread-1"}},
		{OperationUpdate, "recording.update", map[string]any{"channel": "alerts", "message_id": "message-1", "text": "updated", "rich": map[string]any{"kind": "card"}}},
		{OperationReact, "recording.react", map[string]any{"channel": "alerts", "message_id": "message-1", "emoji": ":white_check_mark:"}},
		{OperationUpload, "recording.upload", map[string]any{"channel": "alerts", "filename": "report.txt", "content": "canary-content", "title": "Report"}},
	}

	for _, test := range tests {
		t.Run(test.canonical, func(t *testing.T) {
			expected := &pluginmodel.OperationResult{Success: false, Outputs: map[string]any{"adapter": test.adapter}, Error: "adapter status"}
			adapter := &recordingProvider{
				operations: map[string]*pluginmodel.OperationSchema{test.adapter: operation(test.adapter)},
				execute: func(context.Context, string, map[string]any) (*pluginmodel.OperationResult, error) {
					return expected, nil
				},
			}
			provider := NewProvider(adapter, "recording", "")

			result, err := provider.Execute(context.Background(), test.canonical, test.inputs)

			require.NoError(t, err)
			assert.Same(t, expected, result)
			calls := adapter.recordedCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, test.adapter, calls[0].name)
			assert.Equal(t, test.inputs, calls[0].inputs)
		})
	}
}

func TestNotifierAppliesDefaultsAndExplicitOverrides(t *testing.T) {
	adapter := &recordingProvider{operations: map[string]*pluginmodel.OperationSchema{
		"recording.send": operation("recording.send"),
		"slack.send":     operation("slack.send"),
	}}
	provider := NewProvider(adapter, "recording", "default-channel")

	_, err := provider.Execute(context.Background(), OperationSend, map[string]any{"text": "defaulted"})
	require.NoError(t, err)
	_, err = provider.Execute(context.Background(), OperationSend, map[string]any{
		"backend": "slack",
		"channel": "override-channel",
		"text":    "overridden",
	})
	require.NoError(t, err)

	calls := adapter.recordedCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, "recording.send", calls[0].name)
	assert.Equal(t, "default-channel", calls[0].inputs["channel"])
	assert.Equal(t, "slack.send", calls[1].name)
	assert.Equal(t, "override-channel", calls[1].inputs["channel"])
}

func TestNotifierRejectsInvalidInputsBeforeAdapterExecution(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		defaults  settings
		inputs    map[string]any
		want      string
	}{
		{"unknown_operation", "notifier.unknown", settings{"recording", "alerts"}, map[string]any{"text": "hello"}, `notifier: invalid input: operation: unknown operation "notifier.unknown"`},
		{"missing_backend", OperationSend, settings{"", "alerts"}, map[string]any{"text": "hello"}, "notifier: invalid input: backend: is required"},
		{"explicit_blank_backend", OperationSend, settings{"recording", "alerts"}, map[string]any{"backend": "", "text": "hello"}, "notifier: invalid input: backend: must not be blank"},
		{"dotted_backend", OperationSend, settings{"recording", "alerts"}, map[string]any{"backend": "bad.backend", "text": "hello"}, "notifier: invalid input: backend: must not contain dots"},
		{"reserved_backend", OperationSend, settings{"recording", "alerts"}, map[string]any{"backend": "notifier", "text": "hello"}, "notifier: invalid input: backend: is reserved"},
		{"missing_channel", OperationSend, settings{"recording", ""}, map[string]any{"text": "hello"}, "notifier: invalid input: channel: is required"},
		{"explicit_blank_channel", OperationSend, settings{"recording", "alerts"}, map[string]any{"channel": "", "text": "hello"}, "notifier: invalid input: channel: must not be blank"},
		{"missing_required_input", OperationUpdate, settings{"recording", "alerts"}, map[string]any{"message_id": "message-1"}, "notifier: invalid input: text: is required"},
		{"wrong_string_type", OperationReact, settings{"recording", "alerts"}, map[string]any{"message_id": "message-1", "emoji": 7}, "notifier: invalid input: emoji: must be a string"},
		{"non_object_rich", OperationSend, settings{"recording", "alerts"}, map[string]any{"text": "hello", "rich": "{}"}, "notifier: invalid input: rich: must be an object"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter := &recordingProvider{operations: map[string]*pluginmodel.OperationSchema{}}
			provider := NewProvider(adapter, test.defaults.backend, test.defaults.channel)

			result, err := provider.Execute(context.Background(), test.operation, test.inputs)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, ErrInvalidInput)
			assert.EqualError(t, err, test.want)
			lookups, lists, executions := adapter.counts()
			assert.Zero(t, lookups)
			assert.Zero(t, lists)
			assert.Zero(t, executions)
		})
	}
}

func TestNotifierReportsUnavailableAdaptersDeterministically(t *testing.T) {
	tests := []struct {
		name    string
		adapter *recordingProvider
		want    string
	}{
		{"nil_provider", nil, `notifier: adapter operation unavailable: "recording.send" (discovered backends: [])`},
		{"typed_nil_provider", (*recordingProvider)(nil), `notifier: adapter operation unavailable: "recording.send" (discovered backends: [])`},
		{
			"partial_adapter_with_sorted_backends",
			&recordingProvider{operations: map[string]*pluginmodel.OperationSchema{
				"zeta.update": operation("zeta.update"),
				"alpha.send":  operation("alpha.send"),
				"zeta.send":   operation("zeta.send"),
			}},
			`notifier: adapter operation unavailable: "recording.send" (discovered backends: [alpha zeta])`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := NewProvider(test.adapter, "recording", "alerts")

			result, err := provider.Execute(context.Background(), OperationSend, map[string]any{"text": "hello"})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, ErrAdapterUnavailable)
			assert.EqualError(t, err, test.want)
			if test.adapter != nil {
				_, _, executions := test.adapter.counts()
				assert.Zero(t, executions)
			}
		})
	}
}

func TestNotifierPreservesCancellationAndAdapterFailures(t *testing.T) {
	t.Run("caller_context_is_unchanged", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), contextKey{}, "context-canary")
		adapter := &recordingProvider{operations: map[string]*pluginmodel.OperationSchema{"recording.send": operation("recording.send")}}
		provider := NewProvider(adapter, "recording", "alerts")

		_, err := provider.Execute(ctx, OperationSend, map[string]any{"text": "hello"})

		require.NoError(t, err)
		calls := adapter.recordedCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, ctx, calls[0].ctx)
	})

	t.Run("cancellation_identity", func(t *testing.T) {
		adapter := &recordingProvider{
			operations: map[string]*pluginmodel.OperationSchema{"recording.send": operation("recording.send")},
			execute: func(context.Context, string, map[string]any) (*pluginmodel.OperationResult, error) {
				return nil, context.Canceled
			},
		}
		result, err := NewProvider(adapter, "recording", "alerts").Execute(
			context.Background(), OperationSend, map[string]any{"text": "hello"},
		)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, context.Canceled)
		assert.EqualError(t, err, "notifier notifier.send via recording.send: context canceled")
	})

	t.Run("adapter_failure", func(t *testing.T) {
		adapterErr := errors.New("canary-adapter-error")
		adapter := &recordingProvider{
			operations: map[string]*pluginmodel.OperationSchema{"recording.send": operation("recording.send")},
			execute: func(context.Context, string, map[string]any) (*pluginmodel.OperationResult, error) {
				return &pluginmodel.OperationResult{Success: true}, adapterErr
			},
		}
		result, err := NewProvider(adapter, "recording", "alerts").Execute(
			context.Background(), OperationSend, map[string]any{"text": "hello"},
		)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, adapterErr)
		assert.EqualError(t, err, "notifier notifier.send via recording.send: canary-adapter-error")
	})

	t.Run("nil_adapter_result", func(t *testing.T) {
		adapter := &recordingProvider{
			operations: map[string]*pluginmodel.OperationSchema{"recording.send": operation("recording.send")},
			execute: func(context.Context, string, map[string]any) (*pluginmodel.OperationResult, error) {
				var result *pluginmodel.OperationResult
				var err error
				return result, err
			},
		}
		result, err := NewProvider(adapter, "recording", "alerts").Execute(
			context.Background(), OperationSend, map[string]any{"text": "hello"},
		)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ErrAdapterUnavailable)
		assert.EqualError(t, err, `notifier: adapter operation unavailable: "recording.send" (adapter returned no result)`)
		_, _, executions := adapter.counts()
		assert.Equal(t, 1, executions)
	})
}
