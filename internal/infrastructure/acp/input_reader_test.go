package acp

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACPInputReader_ReadInput_BlocksUntilResponse(t *testing.T) {
	parked := make(chan struct{}, 1)
	reader := NewACPInputReader(func() {
		parked <- struct{}{}
	})

	var (
		result string
		err    error
		wg     sync.WaitGroup
	)

	wg.Go(func() {
		result, err = reader.ReadInput(context.Background())
	})

	<-parked
	reader.Respond("hello")
	wg.Wait()

	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestACPInputReader_RespectsContextCancellation(t *testing.T) {
	parked := make(chan struct{}, 1)
	reader := NewACPInputReader(func() {
		parked <- struct{}{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		readErr error
		wg      sync.WaitGroup
	)

	wg.Go(func() {
		_, readErr = reader.ReadInput(ctx)
	})

	<-parked
	cancel()
	wg.Wait()

	require.Error(t, readErr)
	assert.True(t, errors.Is(readErr, context.Canceled))
}

func TestACPInputReader_EmptyStringEndsConversation(t *testing.T) {
	parked := make(chan struct{}, 1)
	reader := NewACPInputReader(func() {
		parked <- struct{}{}
	})

	var (
		result string
		err    error
		wg     sync.WaitGroup
	)

	wg.Go(func() {
		result, err = reader.ReadInput(context.Background())
	})

	<-parked
	reader.Respond("")
	wg.Wait()

	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestACPInputReader_FiresEndTurnNotifierOnReadInput(t *testing.T) {
	var count atomic.Int64
	parked := make(chan struct{}, 1)

	reader := NewACPInputReader(func() {
		count.Add(1)
		parked <- struct{}{}
	})

	for turn := 1; turn <= 3; turn++ {
		var wg sync.WaitGroup
		wg.Go(func() {
			_, _ = reader.ReadInput(context.Background())
		})

		<-parked
		assert.Equal(t, int64(turn), count.Load(), "notifier fires exactly once per ReadInput call (turn %d)", turn)
		reader.Respond("x")
		wg.Wait()
	}
}

func TestACPInputReader_ParkHooksFireAroundWait(t *testing.T) {
	var parkCount, unparkCount atomic.Int64
	parked := make(chan struct{}, 1)

	reader := NewACPInputReader(nil)
	reader.SetParkHooks(
		func() {
			parkCount.Add(1)
			parked <- struct{}{}
		},
		func() { unparkCount.Add(1) },
	)

	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = reader.ReadInput(context.Background())
	})

	<-parked
	// OnPark has fired; OnUnpark must not fire until the wait resolves.
	assert.Equal(t, int64(1), parkCount.Load(), "OnPark fires before parking")
	assert.Equal(t, int64(0), unparkCount.Load(), "OnUnpark must not fire while parked")

	reader.Respond("done")
	wg.Wait()

	assert.Equal(t, int64(1), unparkCount.Load(), "OnUnpark fires once the response arrives")
}

func TestACPInputReader_ParkHooksBalanceOnContextCancel(t *testing.T) {
	var parkCount, unparkCount atomic.Int64
	parked := make(chan struct{}, 1)

	reader := NewACPInputReader(nil)
	reader.SetParkHooks(
		func() {
			parkCount.Add(1)
			parked <- struct{}{}
		},
		func() { unparkCount.Add(1) },
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = reader.ReadInput(ctx)
	})

	<-parked
	cancel()
	wg.Wait()

	// Even on cancellation, every OnPark is paired with exactly one OnUnpark.
	assert.Equal(t, int64(1), parkCount.Load())
	assert.Equal(t, int64(1), unparkCount.Load(), "OnUnpark fires via defer even when ctx is cancelled")
}

func TestACPInputReader_NilParkHooksAreNoOp(t *testing.T) {
	parked := make(chan struct{}, 1)
	reader := NewACPInputReader(func() { parked <- struct{}{} })
	// No SetParkHooks call: nil hooks must not panic.

	var (
		result string
		err    error
		wg     sync.WaitGroup
	)
	wg.Go(func() {
		result, err = reader.ReadInput(context.Background())
	})

	<-parked
	reader.Respond("ok")
	wg.Wait()

	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestACPInputReader_SustainsMultipleTurnsSequentially(t *testing.T) {
	parked := make(chan struct{}, 1)
	reader := NewACPInputReader(func() {
		parked <- struct{}{}
	})

	inputs := []string{"turn1", "turn2", "turn3", "turn4", "turn5"}

	for i, input := range inputs {
		var (
			result string
			err    error
			wg     sync.WaitGroup
		)

		wg.Go(func() {
			result, err = reader.ReadInput(context.Background())
		})

		<-parked
		reader.Respond(input)
		wg.Wait()

		require.NoError(t, err, "turn %d", i+1)
		assert.Equal(t, input, result, "turn %d", i+1)
	}
}
