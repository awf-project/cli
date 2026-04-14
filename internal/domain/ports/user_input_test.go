package ports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockUserInputReader is a test double that returns pre-configured responses.
type mockUserInputReader struct {
	responses []string
	errors    []error
	callCount int
}

func (m *mockUserInputReader) ReadInput(ctx context.Context) (string, error) {
	defer func() { m.callCount++ }()

	if m.callCount < len(m.errors) && m.errors[m.callCount] != nil {
		return "", m.errors[m.callCount]
	}

	if m.callCount < len(m.responses) {
		return m.responses[m.callCount], nil
	}

	return "", errors.New("no more responses configured")
}

func TestUserInputReader_HappyPath(t *testing.T) {
	reader := &mockUserInputReader{
		responses: []string{"first message", "second message"},
		errors:    []error{nil, nil},
	}

	ctx := context.Background()

	result1, err := reader.ReadInput(ctx)
	require.NoError(t, err)
	assert.Equal(t, "first message", result1)

	result2, err := reader.ReadInput(ctx)
	require.NoError(t, err)
	assert.Equal(t, "second message", result2)

	assert.Equal(t, 2, reader.callCount)
}

func TestUserInputReader_EmptyInputSignalsEnd(t *testing.T) {
	reader := &mockUserInputReader{
		responses: []string{"user message", ""},
		errors:    []error{nil, nil},
	}

	ctx := context.Background()

	result1, err := reader.ReadInput(ctx)
	require.NoError(t, err)
	assert.Equal(t, "user message", result1)

	result2, err := reader.ReadInput(ctx)
	require.NoError(t, err)
	assert.Equal(t, "", result2)
}

func TestUserInputReader_ContextCancellation(t *testing.T) {
	reader := &mockUserInputReader{
		responses: []string{"message"},
		errors:    []error{context.Canceled},
	}

	ctx := context.Background()

	_, err := reader.ReadInput(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestUserInputReader_ContextDeadlineExceeded(t *testing.T) {
	reader := &mockUserInputReader{
		responses: []string{"message"},
		errors:    []error{context.DeadlineExceeded},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond)

	_, err := reader.ReadInput(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestUserInputReader_IOError(t *testing.T) {
	ioErr := errors.New("read error")
	reader := &mockUserInputReader{
		responses: []string{"message"},
		errors:    []error{ioErr},
	}

	ctx := context.Background()

	_, err := reader.ReadInput(ctx)
	require.Error(t, err)
	assert.Equal(t, ioErr, err)
}

func TestUserInputReader_MultipleInputsThenEmpty(t *testing.T) {
	reader := &mockUserInputReader{
		responses: []string{"first", "second", "third", ""},
		errors:    []error{nil, nil, nil, nil},
	}

	ctx := context.Background()

	inputs := []string{}
	for i := 0; i < 4; i++ {
		result, err := reader.ReadInput(ctx)
		require.NoError(t, err)
		inputs = append(inputs, result)
	}

	assert.Equal(t, []string{"first", "second", "third", ""}, inputs)
}

func TestUserInputReader_ErrorAfterSuccess(t *testing.T) {
	reader := &mockUserInputReader{
		responses: []string{"message1", "message2"},
		errors:    []error{nil, errors.New("io error")},
	}

	ctx := context.Background()

	result1, err := reader.ReadInput(ctx)
	require.NoError(t, err)
	assert.Equal(t, "message1", result1)

	_, err = reader.ReadInput(ctx)
	require.Error(t, err)
	assert.Equal(t, "io error", err.Error())
}

func TestUserInputReader_ContextCancelledImmediately(t *testing.T) {
	reader := &mockUserInputReader{
		responses: []string{""},
		errors:    []error{context.Canceled},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := reader.ReadInput(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
