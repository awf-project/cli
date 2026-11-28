package tracker

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type adapterStub struct {
	schemas  map[string]*pluginmodel.OperationSchema
	results  map[string]*pluginmodel.OperationResult
	lastCtx  context.Context
	lastName string
	lastIn   map[string]any
	calls    int
	execErr  error
}

type contextMarkerKey struct{}

func (a *adapterStub) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	schema, ok := a.schemas[name]
	return schema, ok
}

func (a *adapterStub) ListOperations() []*pluginmodel.OperationSchema {
	result := make([]*pluginmodel.OperationSchema, 0, len(a.schemas))
	for _, schema := range a.schemas {
		result = append(result, schema)
	}
	return result
}

func (a *adapterStub) Execute(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
	a.calls++
	a.lastCtx, a.lastName, a.lastIn = ctx, name, inputs
	return a.results[name], a.execErr
}

type capturingLogger struct{ records []string }

func (l *capturingLogger) Debug(msg string, fields ...any) { l.record(msg, fields...) }
func (l *capturingLogger) Info(msg string, fields ...any)  { l.record(msg, fields...) }
func (l *capturingLogger) Warn(msg string, fields ...any)  { l.record(msg, fields...) }
func (l *capturingLogger) Error(msg string, fields ...any) { l.record(msg, fields...) }
func (l *capturingLogger) WithContext(fields map[string]any) ports.Logger {
	values := make([]any, 0, len(fields)*2)
	for key, value := range fields {
		values = append(values, key, value)
	}
	l.record("context", values...)
	return l
}

func (l *capturingLogger) record(msg string, fields ...any) {
	l.records = append(l.records, fmt.Sprint(append([]any{msg}, fields...)...))
}

func TestProviderDelegatesEveryCanonicalOperationToSelectedAdapter(t *testing.T) {
	const secret = "secret-marker"
	tests := []struct {
		name           string
		inputs         map[string]any
		adapterOutputs map[string]any
		wantOutputs    map[string]any
	}{
		{name: createIssueOperation, inputs: map[string]any{"title": secret, "backend": "jira"}, adapterOutputs: map[string]any{"id": secret, "key": "A-1", "url": "https://example.invalid/1"}, wantOutputs: map[string]any{"id": secret, "key": "A-1", "url": "https://example.invalid/1"}},
		{name: updateIssueOperation, inputs: map[string]any{"id": "1", "key": "A-1"}, adapterOutputs: map[string]any{"id": "1", "key": "A-1"}, wantOutputs: map[string]any{"id": "1", "key": "A-1"}},
		{name: transitionOperation, inputs: map[string]any{"key": "A-1", "transition": "done"}, adapterOutputs: map[string]any{"id": "1", "key": "A-1", "status": "done"}, wantOutputs: map[string]any{"id": "1", "key": "A-1", "status": "done"}},
		{name: addCommentOperation, inputs: map[string]any{"id": "1", "body": "confidential"}, adapterOutputs: map[string]any{"comment_id": "comment-1", "id": "wrong", "key": "wrong"}, wantOutputs: map[string]any{"comment_id": "comment-1"}},
		{name: getIssueOperation, inputs: map[string]any{"key": "A-1"}, adapterOutputs: map[string]any{"id": "1", "key": "A-1", "title": "title", "status": "open", "description": "description", "assignee": "user", "url": "https://example.invalid/1"}, wantOutputs: map[string]any{"id": "1", "key": "A-1", "title": "title", "status": "open", "description": "description", "assignee": "user", "url": "https://example.invalid/1"}},
		{name: searchOperation, inputs: map[string]any{"query": "confidential", "max_results": 2}, adapterOutputs: map[string]any{"issues": []any{}, "total": 0}, wantOutputs: map[string]any{"issues": []any{}, "total": 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := AdapterOperationName("jira", tt.name)
			require.NoError(t, err)
			adapter := &adapterStub{
				schemas: map[string]*pluginmodel.OperationSchema{target: {Name: target, PluginName: "tracker-jira"}},
				results: map[string]*pluginmodel.OperationResult{target: {Success: true, Outputs: tt.adapterOutputs}},
			}
			logger := &capturingLogger{}
			provider := NewProvider(adapter, "jira", logger)
			ctx := context.WithValue(context.Background(), contextMarkerKey{}, "marker")

			result, execErr := provider.Execute(ctx, tt.name, tt.inputs)

			require.NoError(t, execErr)
			assert.Equal(t, tt.wantOutputs, result.Outputs)
			assert.True(t, result.Success)
			assert.Equal(t, target, adapter.lastName)
			assert.Same(t, ctx, adapter.lastCtx)
			wantPayload := make(map[string]any, len(tt.inputs))
			for key, value := range tt.inputs {
				if key != "backend" {
					wantPayload[key] = value
				}
			}
			assert.Equal(t, wantPayload, adapter.lastIn)
			assert.Equal(t, 1, adapter.calls)
			logs := strings.Join(logger.records, "\n")
			assert.NotContains(t, logs, secret)
		})
	}
}

func TestProviderRejectsInvalidCanonicalInputsWithoutDisclosure(t *testing.T) {
	secret := "do-not-disclose"
	tests := []struct {
		name      string
		operation string
		inputs    map[string]any
		field     string
	}{
		{name: "unknown operation", operation: "tracker." + secret, inputs: map[string]any{}, field: "operation"},
		{name: "missing title", operation: createIssueOperation, inputs: map[string]any{"description": secret}, field: "title"},
		{name: "blank title", operation: createIssueOperation, inputs: map[string]any{"title": " ", "description": secret}, field: "title"},
		{name: "blank transition", operation: transitionOperation, inputs: map[string]any{"id": secret, "transition": " "}, field: "transition"},
		{name: "wrong identity type", operation: transitionOperation, inputs: map[string]any{"id": []string{secret}, "transition": "done"}, field: "id"},
		{name: "wrong query type", operation: searchOperation, inputs: map[string]any{"query": []string{secret}}, field: "query"},
		{name: "wrong maximum type", operation: searchOperation, inputs: map[string]any{"query": secret, "max_results": "2"}, field: "max_results"},
		{name: "missing update identity", operation: updateIssueOperation, inputs: map[string]any{"title": secret}, field: "id or key"},
		{name: "blank comment identity", operation: addCommentOperation, inputs: map[string]any{"id": " ", "key": "", "body": secret}, field: "id or key"},
		{name: "missing get identity", operation: getIssueOperation, inputs: map[string]any{}, field: "id or key"},
	}

	logger := &capturingLogger{}
	provider := NewProvider(nil, "jira", logger)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Execute(context.Background(), tt.operation, tt.inputs)
			assert.Nil(t, result)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidTrackerInput)
			assert.Contains(t, err.Error(), tt.field)
			assert.NotContains(t, err.Error(), secret)
			assert.NotContains(t, strings.Join(logger.records, "\n"), secret)
		})
	}
}

func TestProviderExposesOnlyCanonicalOperations(t *testing.T) {
	downstream := &adapterStub{schemas: map[string]*pluginmodel.OperationSchema{
		"tracker-jira.search": {Name: "tracker-jira.search"},
	}}
	provider := NewProvider(downstream, "jira", nil)
	wantNames := []string{createIssueOperation, updateIssueOperation, transitionOperation, addCommentOperation, getIssueOperation, searchOperation}

	schemas := provider.ListOperations()
	require.Len(t, schemas, len(wantNames))
	for index, name := range wantNames {
		assert.Equal(t, name, schemas[index].Name)
		got, ok := provider.GetOperation(name)
		require.True(t, ok)
		assert.Equal(t, schemas[index], got)
	}
	for _, name := range []string{"tracker-jira.search", "tracker.search.extra", " search", ""} {
		got, ok := provider.GetOperation(name)
		assert.False(t, ok)
		assert.Nil(t, got)
	}
}

func TestProviderUsesConfiguredBackendUnlessInputOverridesIt(t *testing.T) {
	for _, backend := range []string{"jira", "linear"} {
		target, err := AdapterOperationName(backend, searchOperation)
		require.NoError(t, err)
		adapter := &adapterStub{
			schemas: map[string]*pluginmodel.OperationSchema{target: {Name: target, PluginName: "tracker-" + backend}},
			results: map[string]*pluginmodel.OperationResult{target: {Success: true, Outputs: map[string]any{"issues": []any{}, "total": 0}}},
		}
		inputs := map[string]any{"query": "q"}
		if backend == "linear" {
			inputs["backend"] = backend
		}
		_, err = NewProvider(adapter, "jira", nil).Execute(context.Background(), searchOperation, inputs)
		require.NoError(t, err)
		assert.Equal(t, target, adapter.lastName)
	}

	blankTarget, err := AdapterOperationName("jira", searchOperation)
	require.NoError(t, err)
	blankAdapter := &adapterStub{
		schemas: map[string]*pluginmodel.OperationSchema{blankTarget: {Name: blankTarget, PluginName: "tracker-jira"}},
		results: map[string]*pluginmodel.OperationResult{blankTarget: {Success: true, Outputs: map[string]any{"issues": []any{}, "total": 0}}},
	}
	_, err = NewProvider(blankAdapter, "jira", nil).Execute(
		context.Background(),
		searchOperation,
		map[string]any{"backend": "", "query": "q"},
	)
	require.NoError(t, err)
	assert.Equal(t, blankTarget, blankAdapter.lastName)

	result, err := NewProvider(nil, "", nil).Execute(context.Background(), searchOperation, map[string]any{"query": "q"})
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrBackendRequired)
}

func TestProviderReportsUnavailableAdaptersDeterministically(t *testing.T) {
	const secret = "secret-marker"
	adapter := &adapterStub{schemas: map[string]*pluginmodel.OperationSchema{
		"tracker-zendesk.search": {Name: "tracker-zendesk.search", PluginName: "tracker-zendesk"},
		"tracker-linear.search":  {Name: "tracker-linear.search", PluginName: "tracker-linear"},
	}}
	logger := &capturingLogger{}
	provider := NewProvider(adapter, "jira", logger)
	result, err := provider.Execute(context.Background(), searchOperation, map[string]any{"query": secret})
	assert.Nil(t, result)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAdapterUnavailable)
	assert.Contains(t, err.Error(), "linear, zendesk")
	assert.NotContains(t, err.Error(), secret)
	assert.NotContains(t, strings.Join(logger.records, "\n"), secret)
	assert.Zero(t, adapter.calls)
	assert.Contains(t, err.Error(), "backend jira")
	assert.Contains(t, err.Error(), "operation search")

	for _, backend := range []string{"bad.backend", "bad backend", " bad"} {
		result, invalidErr := provider.Execute(context.Background(), searchOperation, map[string]any{"backend": backend, "query": secret})
		assert.Nil(t, result)
		assert.ErrorIs(t, invalidErr, ErrInvalidTrackerInput)
		assert.NotContains(t, invalidErr.Error(), secret)
		assert.NotContains(t, strings.Join(logger.records, "\n"), secret)
		assert.Zero(t, adapter.calls)
	}
}

func TestProviderRejectsMalformedAdapterResult(t *testing.T) {
	target, err := AdapterOperationName("jira", addCommentOperation)
	require.NoError(t, err)
	tests := []struct {
		name   string
		result *pluginmodel.OperationResult
	}{
		{name: "nil result"},
		{name: "unsuccessful without error", result: &pluginmodel.OperationResult{Success: false}},
		{name: "unsuccessful with blank error", result: &pluginmodel.OperationResult{Success: false, Error: " \t"}},
		{name: "nil outputs", result: &pluginmodel.OperationResult{Success: true}},
		{name: "missing required output", result: &pluginmodel.OperationResult{Success: true, Outputs: map[string]any{}}},
		{name: "wrong output type", result: &pluginmodel.OperationResult{Success: true, Outputs: map[string]any{"comment_id": 42}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &adapterStub{
				schemas: map[string]*pluginmodel.OperationSchema{target: {Name: target, PluginName: "tracker-jira"}},
				results: map[string]*pluginmodel.OperationResult{target: tt.result},
			}
			result, execErr := NewProvider(adapter, "jira", nil).Execute(context.Background(), addCommentOperation, map[string]any{"id": "1", "body": "body"})
			assert.Nil(t, result)
			assert.ErrorIs(t, execErr, ErrMalformedAdapterResult)
			assert.False(t, errors.Is(execErr, ErrInvalidTrackerInput))
		})
	}

	t.Run("malformed search issue", func(t *testing.T) {
		searchTarget, targetErr := AdapterOperationName("jira", searchOperation)
		require.NoError(t, targetErr)
		adapter := &adapterStub{
			schemas: map[string]*pluginmodel.OperationSchema{searchTarget: {Name: searchTarget, PluginName: "tracker-jira"}},
			results: map[string]*pluginmodel.OperationResult{searchTarget: {
				Success: true,
				Outputs: map[string]any{"issues": []any{42}, "total": 1},
			}},
		}

		result, execErr := NewProvider(adapter, "jira", nil).Execute(
			context.Background(),
			searchOperation,
			map[string]any{"query": "query"},
		)

		assert.Nil(t, result)
		assert.ErrorIs(t, execErr, ErrMalformedAdapterResult)
	})
}

func TestProviderPreservesDownstreamErrorsAndKeepsLogsMetadataOnly(t *testing.T) {
	const secret = "secret-marker"
	target, err := AdapterOperationName("jira", searchOperation)
	require.NoError(t, err)
	downstreamErr := &fs.PathError{Op: "adapter", Path: secret, Err: context.Canceled}
	adapter := &adapterStub{
		schemas: map[string]*pluginmodel.OperationSchema{target: {Name: target, PluginName: "tracker-jira"}},
		results: map[string]*pluginmodel.OperationResult{},
		execErr: downstreamErr,
	}
	logger := &capturingLogger{}

	result, execErr := NewProvider(adapter, "jira", logger).Execute(context.Background(), searchOperation, map[string]any{"query": secret})

	assert.Nil(t, result)
	require.Error(t, execErr)
	assert.Equal(t, "tracker adapter execution failed", execErr.Error())
	assert.ErrorIs(t, execErr, downstreamErr)
	assert.ErrorIs(t, execErr, context.Canceled)
	var pathErr *fs.PathError
	require.ErrorAs(t, execErr, &pathErr)
	assert.Same(t, downstreamErr, pathErr)
	logs := strings.Join(logger.records, "\n")
	assert.Contains(t, logs, searchOperation)
	assert.Contains(t, logs, "jira")
	assert.NotContains(t, logs, secret)

	t.Run("business failure result", func(t *testing.T) {
		failure := &pluginmodel.OperationResult{
			Success: false,
			Error:   "adapter failure " + secret,
			Outputs: map[string]any{"secret": secret},
		}
		failureAdapter := &adapterStub{
			schemas: map[string]*pluginmodel.OperationSchema{target: {Name: target, PluginName: "tracker-jira"}},
			results: map[string]*pluginmodel.OperationResult{target: failure},
		}
		failureLogger := &capturingLogger{}

		failureResult, failureErr := NewProvider(failureAdapter, "jira", failureLogger).Execute(
			context.Background(),
			searchOperation,
			map[string]any{"query": secret},
		)

		require.NoError(t, failureErr)
		require.NotNil(t, failureResult)
		assert.NotSame(t, failure, failureResult)
		assert.False(t, failureResult.Success)
		assert.Equal(t, "tracker adapter reported failure", failureResult.Error)
		assert.Empty(t, failureResult.Outputs)
		assert.NotContains(t, strings.Join(failureLogger.records, "\n"), secret)
	})
}
