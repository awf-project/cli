package tui_test

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/interfaces/tui"
)

func TestTUIInputReader_ReadInput_BlocksUntilResponse(t *testing.T) {
	reader := tui.NewTUIInputReader(nil)

	done := make(chan string, 1)
	go func() {
		input, err := reader.ReadInput(context.Background())
		require.NoError(t, err)
		done <- input
	}()

	select {
	case <-reader.RequestCh():
	case <-time.After(time.Second):
		t.Fatal("reader did not signal input request")
	}

	reader.Respond("hello agent")

	select {
	case got := <-done:
		assert.Equal(t, "hello agent", got)
	case <-time.After(time.Second):
		t.Fatal("ReadInput did not return")
	}
}

func TestTUIInputReader_ReadInput_RespectsContextCancellation(t *testing.T) {
	reader := tui.NewTUIInputReader(nil)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := reader.ReadInput(ctx)
		done <- err
	}()

	select {
	case <-reader.RequestCh():
	case <-time.After(time.Second):
		t.Fatal("reader did not signal")
	}

	cancel()

	select {
	case err := <-done:
		assert.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("ReadInput did not return after cancel")
	}
}

func TestTUIInputReader_ReadInput_EmptyStringEndsConversation(t *testing.T) {
	reader := tui.NewTUIInputReader(nil)

	done := make(chan string, 1)
	go func() {
		input, _ := reader.ReadInput(context.Background())
		done <- input
	}()

	<-reader.RequestCh()
	reader.Respond("")

	got := <-done
	assert.Equal(t, "", got)
}

func TestTUIInputReader_SendsTeaMsgOnRequest(t *testing.T) {
	msgs := make(chan tea.Msg, 1)
	sender := func(msg tea.Msg) { msgs <- msg }
	reader := tui.NewTUIInputReader(sender)

	go func() {
		_, _ = reader.ReadInput(context.Background())
	}()

	select {
	case msg := <-msgs:
		_, ok := msg.(tui.InputRequestedMsg)
		assert.True(t, ok, "expected InputRequestedMsg, got %T", msg)
	case <-time.After(time.Second):
		t.Fatal("no tea.Msg sent")
	}
}
