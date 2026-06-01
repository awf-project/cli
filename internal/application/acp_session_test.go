package application

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInputPairs(t *testing.T) {
	tests := []struct {
		name    string
		pairs   []string
		want    map[string]any
		wantErr bool
	}{
		{name: "empty list", pairs: nil, want: map[string]any{}},
		{name: "single pair", pairs: []string{"name=value"}, want: map[string]any{"name": "value"}},
		{name: "multiple pairs", pairs: []string{"a=1", "b=2"}, want: map[string]any{"a": "1", "b": "2"}},
		{name: "value contains equals", pairs: []string{"url=http://x?a=1&b=2"}, want: map[string]any{"url": "http://x?a=1&b=2"}},
		{name: "whitespace trimmed", pairs: []string{"  key  =  val  "}, want: map[string]any{"key": "val"}},
		{name: "empty value allowed", pairs: []string{"key="}, want: map[string]any{"key": ""}},
		{name: "missing separator", pairs: []string{"novalue"}, wantErr: true},
		{name: "empty key", pairs: []string{"=value"}, wantErr: true},
		{name: "whitespace-only key", pairs: []string{"   =value"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInputPairs(tt.pairs)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTokenizePrompt(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "empty", text: "", want: nil},
		{name: "whitespace only", text: "   \t ", want: nil},
		{name: "simple words", text: "/echo name=World", want: []string{"/echo", "name=World"}},
		{name: "collapses runs of spaces", text: "/echo   a=1\tb=2", want: []string{"/echo", "a=1", "b=2"}},
		{name: "double quotes stripped", text: `/echo name="salut"`, want: []string{"/echo", "name=salut"}},
		{name: "single quotes stripped", text: `/echo name='salut'`, want: []string{"/echo", "name=salut"}},
		{name: "quoted value with spaces", text: `/echo msg="hello world"`, want: []string{"/echo", "msg=hello world"}},
		{name: "empty quoted value", text: `/echo name=""`, want: []string{"/echo", "name="}},
		{name: "unterminated quote tolerated", text: `/echo name="salut`, want: []string{"/echo", "name=salut"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tokenizePrompt(tt.text))
		})
	}
}

func TestExtractInputPairs(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		want   []string
	}{
		{name: "bare key=value", tokens: []string{"name=World"}, want: []string{"name=World"}},
		{name: "input equals form", tokens: []string{"--input=name=World"}, want: []string{"name=World"}},
		{name: "input space form", tokens: []string{"--input", "name=World"}, want: []string{"name=World"}},
		{name: "mixed forms", tokens: []string{"name=World", "--input=lang=fr", "--input", "n=3"}, want: []string{"name=World", "lang=fr", "n=3"}},
		{name: "dangling --input ignored", tokens: []string{"name=World", "--input"}, want: []string{"name=World"}},
		{name: "unknown flag ignored", tokens: []string{"--verbose", "name=World"}, want: []string{"name=World"}},
		{name: "non-pair token ignored", tokens: []string{"hello", "name=World"}, want: []string{"name=World"}},
		{name: "none", tokens: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractInputPairs(tt.tokens))
		})
	}
}

func TestParseSlashCommand_AcceptedForms(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantName  string
		wantInput map[string]any
		wantErr   bool
	}{
		{name: "no inputs", text: "/echo", wantName: "echo", wantInput: map[string]any{}},
		{name: "bare pair", text: "/echo name=World", wantName: "echo", wantInput: map[string]any{"name": "World"}},
		{name: "input equals", text: "/echo --input=name=World", wantName: "echo", wantInput: map[string]any{"name": "World"}},
		{name: "input space", text: "/echo --input name=World", wantName: "echo", wantInput: map[string]any{"name": "World"}},
		{name: "quoted value", text: `/echo name="salut"`, wantName: "echo", wantInput: map[string]any{"name": "salut"}},
		{name: "quoted value with spaces", text: `/echo msg="hello world"`, wantName: "echo", wantInput: map[string]any{"msg": "hello world"}},
		{name: "multiple mixed", text: `/build target=linux --input=mode=release`, wantName: "build", wantInput: map[string]any{"target": "linux", "mode": "release"}},
		{name: "missing slash", text: "echo name=World", wantErr: true},
		{name: "empty command", text: "/ name=World", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotInputs, err := parseSlashCommand(tt.text)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, gotName)
			assert.Equal(t, tt.wantInput, gotInputs)
		})
	}
}

// TestACPSession_ConcurrentInFlight exercises the atomic InFlight/ParkedTurnCount guards
// under -race: exactly one of N concurrent CompareAndSwap(false,true) wins.
func TestACPSession_ConcurrentInFlight(t *testing.T) {
	session := &ACPSession{ID: "s1"}

	const n = 50
	var wg sync.WaitGroup
	var wins atomic.Int32
	for range n {
		wg.Go(func() {
			if session.InFlight.CompareAndSwap(false, true) {
				wins.Add(1)
			}
			session.ParkedTurnCount.Add(1)
		})
	}
	wg.Wait()

	assert.Equal(t, int32(1), wins.Load(), "exactly one goroutine should win the InFlight swap")
	assert.Equal(t, int32(n), session.ParkedTurnCount.Load(), "every goroutine should have incremented ParkedTurnCount")
}

// TestACPSession_InputReaderHolder_StoreLoadRoundtrip verifies the C-2 fix: storing an
// ACPInputResponder via inputReaderHolder in atomic.Pointer[inputReaderHolder] and loading
// it back yields the same concrete value without indirection through a pointer-to-interface.
// Run with -race to confirm the Store/Load is race-free.
func TestACPSession_InputReaderHolder_StoreLoadRoundtrip(t *testing.T) {
	session := &ACPSession{ID: "s-holder"}
	require.Equal(t, "s-holder", session.ID, "session ID must match the initialized value")

	// Initially nil — no reader wired yet.
	require.Nil(t, session.inputReader.Load(), "inputReader must be nil before any Store")

	reader := &fakeInputResponder{}
	session.inputReader.Store(&inputReaderHolder{r: reader})

	h := session.inputReader.Load()
	require.NotNil(t, h, "Load must return a non-nil holder after Store")
	require.Equal(t, reader, h.r, "holder must expose the original ACPInputResponder")

	// Drive Respond through the loaded holder to confirm the concrete value is intact.
	h.r.Respond("hello")
	assert.Equal(t, []string{"hello"}, reader.recorded(),
		"calling h.r.Respond must reach the concrete fakeInputResponder")
}

// TestACPSession_InputReaderHolder_ConcurrentStoreLoad exercises the atomic.Pointer
// Store/Load under concurrent access (-race) to confirm no data race on inputReader.
func TestACPSession_InputReaderHolder_ConcurrentStoreLoad(t *testing.T) {
	session := &ACPSession{ID: "s-race"}
	reader := &fakeInputResponder{}

	const n = 100
	var wg sync.WaitGroup
	// Half the goroutines store, half load; neither must race.
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				session.inputReader.Store(&inputReaderHolder{r: reader})
			} else {
				_ = session.inputReader.Load()
			}
		}(i)
	}
	wg.Wait()
	// After the loop the holder must be set (all even goroutines stored it).
	h := session.inputReader.Load()
	require.NotNil(t, h, "inputReader must be non-nil after concurrent stores")
	assert.Equal(t, reader, h.r)
}

// TestNewACPSessionService_NilDepsDoNotPanic verifies the defensive wiring: a nil logger is
// replaced with a no-op (no panic on the first handler call), and a nil workflowRepo yields
// a structured ErrInternal instead of a nil-pointer dereference.
func TestNewACPSessionService_NilDepsDoNotPanic(t *testing.T) {
	svc := NewACPSessionService(nil, nil, nil, nil)
	require.NotNil(t, svc)

	_, acpErr := svc.HandleSessionNew(context.Background(), json.RawMessage(`{"session_id":"s1"}`))
	require.NotNil(t, acpErr, "nil workflowRepo should surface a structured error, not panic")
	assert.Equal(t, ACPErrInternal, acpErr.Kind)
}
