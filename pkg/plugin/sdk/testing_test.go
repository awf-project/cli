package sdk_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vanoix/awf/pkg/plugin/sdk"
)

// MockPlugin tests

func TestNewMockPlugin(t *testing.T) {
	mock := sdk.NewMockPlugin("test-plugin", "1.0.0")

	assert.Equal(t, "test-plugin", mock.Name())
	assert.Equal(t, "1.0.0", mock.Version())
	assert.False(t, mock.InitCalled)
	assert.False(t, mock.ShutdownCalled)
}

func TestMockPlugin_ImplementsPlugin(t *testing.T) {
	var _ sdk.Plugin = (*sdk.MockPlugin)(nil)
}

func TestMockPlugin_Init(t *testing.T) {
	mock := sdk.NewMockPlugin("test", "1.0.0")
	config := map[string]any{"key": "value"}

	err := mock.Init(context.Background(), config)

	assert.NoError(t, err)
	assert.True(t, mock.InitCalled)
	assert.Equal(t, config, mock.LastConfig)
}

func TestMockPlugin_Init_WithError(t *testing.T) {
	mock := sdk.NewMockPlugin("test", "1.0.0")
	expectedErr := errors.New("init failed")
	mock.InitError = expectedErr

	err := mock.Init(context.Background(), nil)

	assert.ErrorIs(t, err, expectedErr)
	assert.True(t, mock.InitCalled)
}

func TestMockPlugin_Shutdown(t *testing.T) {
	mock := sdk.NewMockPlugin("test", "1.0.0")

	err := mock.Shutdown(context.Background())

	assert.NoError(t, err)
	assert.True(t, mock.ShutdownCalled)
}

func TestMockPlugin_Shutdown_WithError(t *testing.T) {
	mock := sdk.NewMockPlugin("test", "1.0.0")
	expectedErr := errors.New("shutdown failed")
	mock.ShutdownError = expectedErr

	err := mock.Shutdown(context.Background())

	assert.ErrorIs(t, err, expectedErr)
	assert.True(t, mock.ShutdownCalled)
}

func TestMockPlugin_WasInitCalled(t *testing.T) {
	mock := sdk.NewMockPlugin("test", "1.0.0")

	assert.False(t, mock.WasInitCalled())

	_ = mock.Init(context.Background(), nil)

	assert.True(t, mock.WasInitCalled())
}

func TestMockPlugin_WasShutdownCalled(t *testing.T) {
	mock := sdk.NewMockPlugin("test", "1.0.0")

	assert.False(t, mock.WasShutdownCalled())

	_ = mock.Shutdown(context.Background())

	assert.True(t, mock.WasShutdownCalled())
}

func TestMockPlugin_Reset(t *testing.T) {
	mock := sdk.NewMockPlugin("test", "1.0.0")
	_ = mock.Init(context.Background(), map[string]any{"key": "value"})
	_ = mock.Shutdown(context.Background())

	mock.Reset()

	assert.False(t, mock.InitCalled)
	assert.False(t, mock.ShutdownCalled)
	assert.Nil(t, mock.LastConfig)
}

func TestMockPlugin_ThreadSafety(t *testing.T) {
	mock := sdk.NewMockPlugin("test", "1.0.0")
	var wg sync.WaitGroup

	// Concurrent Init calls
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mock.Init(context.Background(), nil)
		}()
	}

	// Concurrent WasInitCalled reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mock.WasInitCalled()
		}()
	}

	wg.Wait()
	assert.True(t, mock.WasInitCalled())
}

// MockOperationHandler tests

func TestNewMockOperationHandler(t *testing.T) {
	mock := sdk.NewMockOperationHandler()

	assert.NotNil(t, mock.Outputs)
	assert.Equal(t, 0, mock.GetCallCount())
}

func TestMockOperationHandler_ImplementsOperationHandler(t *testing.T) {
	var _ sdk.OperationHandler = (*sdk.MockOperationHandler)(nil)
}

func TestMockOperationHandler_Handle_ReturnsConfiguredOutputs(t *testing.T) {
	mock := sdk.NewMockOperationHandler()
	mock.Outputs = map[string]any{"result": "success"}

	outputs, err := mock.Handle(context.Background(), map[string]any{"input": "test"})

	assert.NoError(t, err)
	assert.Equal(t, "success", outputs["result"])
	assert.Equal(t, "test", mock.LastInputs["input"])
	assert.Equal(t, 1, mock.GetCallCount())
}

func TestMockOperationHandler_Handle_ReturnsConfiguredError(t *testing.T) {
	mock := sdk.NewMockOperationHandler()
	expectedErr := errors.New("handler error")
	mock.Error = expectedErr

	_, err := mock.Handle(context.Background(), nil)

	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 1, mock.GetCallCount())
}

func TestMockOperationHandler_Handle_WithCustomFunc(t *testing.T) {
	mock := sdk.NewMockOperationHandler()
	mock.HandleFunc = func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		return map[string]any{"custom": "result"}, nil
	}

	outputs, err := mock.Handle(context.Background(), nil)

	assert.NoError(t, err)
	assert.Equal(t, "result", outputs["custom"])
}

func TestMockOperationHandler_Handle_CustomFuncWithError(t *testing.T) {
	mock := sdk.NewMockOperationHandler()
	expectedErr := errors.New("custom error")
	mock.HandleFunc = func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		return nil, expectedErr
	}

	_, err := mock.Handle(context.Background(), nil)

	assert.ErrorIs(t, err, expectedErr)
}

func TestMockOperationHandler_GetCallCount(t *testing.T) {
	mock := sdk.NewMockOperationHandler()

	assert.Equal(t, 0, mock.GetCallCount())

	_, _ = mock.Handle(context.Background(), nil)
	assert.Equal(t, 1, mock.GetCallCount())

	_, _ = mock.Handle(context.Background(), nil)
	assert.Equal(t, 2, mock.GetCallCount())
}

func TestMockOperationHandler_Reset(t *testing.T) {
	mock := sdk.NewMockOperationHandler()
	_, _ = mock.Handle(context.Background(), map[string]any{"key": "value"})
	_, _ = mock.Handle(context.Background(), nil)

	mock.Reset()

	assert.Equal(t, 0, mock.GetCallCount())
	assert.Nil(t, mock.LastInputs)
}

func TestMockOperationHandler_ThreadSafety(t *testing.T) {
	mock := sdk.NewMockOperationHandler()
	var wg sync.WaitGroup

	// Concurrent Handle calls
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = mock.Handle(context.Background(), nil)
		}()
	}

	// Concurrent GetCallCount reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mock.GetCallCount()
		}()
	}

	wg.Wait()
	assert.Equal(t, 100, mock.GetCallCount())
}

// MockOperationProvider tests

func TestNewMockOperationProvider(t *testing.T) {
	mock := sdk.NewMockOperationProvider("op1", "op2")

	assert.Equal(t, []string{"op1", "op2"}, mock.Operations())
	assert.NotNil(t, mock.Results)
	assert.NotNil(t, mock.Errors)
}

func TestNewMockOperationProvider_NoOperations(t *testing.T) {
	mock := sdk.NewMockOperationProvider()

	assert.Empty(t, mock.Operations())
}

func TestMockOperationProvider_ImplementsOperationProvider(t *testing.T) {
	var _ sdk.OperationProvider = (*sdk.MockOperationProvider)(nil)
}

func TestMockOperationProvider_HandleOperation_DefaultSuccess(t *testing.T) {
	mock := sdk.NewMockOperationProvider("test.op")

	result, err := mock.HandleOperation(context.Background(), "test.op", nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "ok", result.Output)
}

func TestMockOperationProvider_HandleOperation_ConfiguredResult(t *testing.T) {
	mock := sdk.NewMockOperationProvider("custom.op")
	expectedResult := sdk.NewSuccessResult("custom output", map[string]any{"data": 123})
	mock.SetResult("custom.op", expectedResult)

	result, err := mock.HandleOperation(context.Background(), "custom.op", map[string]any{"input": "value"})

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	assert.Equal(t, "custom.op", mock.LastOperation)
	assert.Equal(t, "value", mock.LastInputs["input"])
}

func TestMockOperationProvider_HandleOperation_ConfiguredError(t *testing.T) {
	mock := sdk.NewMockOperationProvider("failing.op")
	expectedErr := errors.New("operation error")
	mock.SetError("failing.op", expectedErr)

	result, err := mock.HandleOperation(context.Background(), "failing.op", nil)

	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, result)
}

func TestMockOperationProvider_HandleOperation_WithCustomFunc(t *testing.T) {
	mock := sdk.NewMockOperationProvider("dynamic.op")
	mock.HandleFunc = func(ctx context.Context, name string, inputs map[string]any) (*sdk.OperationResult, error) {
		return sdk.NewSuccessResult("handled: "+name, nil), nil
	}

	result, err := mock.HandleOperation(context.Background(), "dynamic.op", nil)

	assert.NoError(t, err)
	assert.Equal(t, "handled: dynamic.op", result.Output)
}

func TestMockOperationProvider_SetResult(t *testing.T) {
	mock := sdk.NewMockOperationProvider()
	result := sdk.NewSuccessResult("test", nil)

	mock.SetResult("op1", result)

	r, err := mock.HandleOperation(context.Background(), "op1", nil)
	assert.NoError(t, err)
	assert.Equal(t, result, r)
}

func TestMockOperationProvider_SetError(t *testing.T) {
	mock := sdk.NewMockOperationProvider()
	expectedErr := errors.New("configured error")

	mock.SetError("op1", expectedErr)

	_, err := mock.HandleOperation(context.Background(), "op1", nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestMockOperationProvider_ThreadSafety(t *testing.T) {
	mock := sdk.NewMockOperationProvider("op1", "op2", "op3")
	var wg sync.WaitGroup

	// Concurrent HandleOperation calls
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			opName := []string{"op1", "op2", "op3"}[idx%3]
			_, _ = mock.HandleOperation(context.Background(), opName, nil)
		}(i)
	}

	// Concurrent SetResult calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mock.SetResult("op1", sdk.NewSuccessResult("concurrent", nil))
		}()
	}

	// Concurrent Operations reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mock.Operations()
		}()
	}

	wg.Wait()
}

// TestInputs and TestConfig tests

func TestTestInputs(t *testing.T) {
	inputs := sdk.TestInputs("name", "John", "age", 30, "active", true)

	assert.Equal(t, "John", inputs["name"])
	assert.Equal(t, 30, inputs["age"])
	assert.Equal(t, true, inputs["active"])
}

func TestTestInputs_Empty(t *testing.T) {
	inputs := sdk.TestInputs()

	assert.NotNil(t, inputs)
	assert.Empty(t, inputs)
}

func TestTestInputs_SinglePair(t *testing.T) {
	inputs := sdk.TestInputs("key", "value")

	assert.Equal(t, "value", inputs["key"])
}

func TestTestInputs_OddNumberOfArgs(t *testing.T) {
	// Last value is ignored if no key pair
	inputs := sdk.TestInputs("key", "value", "orphan")

	assert.Len(t, inputs, 1)
	assert.Equal(t, "value", inputs["key"])
}

func TestTestInputs_NonStringKey(t *testing.T) {
	// Non-string keys are skipped
	inputs := sdk.TestInputs(123, "value", "key", "value2")

	assert.Len(t, inputs, 1)
	assert.Equal(t, "value2", inputs["key"])
}

func TestTestConfig(t *testing.T) {
	config := sdk.TestConfig("webhook_url", "https://example.com", "retries", 3)

	assert.Equal(t, "https://example.com", config["webhook_url"])
	assert.Equal(t, 3, config["retries"])
}

func TestTestConfig_Empty(t *testing.T) {
	config := sdk.TestConfig()

	assert.NotNil(t, config)
	assert.Empty(t, config)
}
