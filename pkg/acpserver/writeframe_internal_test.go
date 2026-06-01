package acpserver

// writeframe_internal_test.go — white-box tests for the writeFrame nil-encoder
// defensive branch (P17).
//
// writeFrame returns "server not serving" when s.enc is nil, i.e. when Serve has
// not yet been called (or has already returned and the encoder was never set).
// This branch is not reachable through CallClient or Notify in the normal
// lifecycle because both block on <-s.ready, which is closed only after s.enc is
// assigned in Serve. It is a defensive guard against misuse or future refactoring.
//
// The test exercises it by calling writeFrame directly on a Server constructed
// with New but never passed to Serve, which leaves s.enc nil. The package-level
// test (package acpserver, not acpserver_test) is necessary because writeFrame is
// unexported.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteFrame_NilEncoder_ReturnsError covers the s.enc == nil defensive branch
// in writeFrame. A Server that has never been served has a nil encoder; writeFrame
// must return an error rather than panic.
func TestWriteFrame_NilEncoder_ReturnsError(t *testing.T) {
	srv := New(nil) // nil logger falls back to slog.Default()

	// s.enc is nil because Serve was never called.
	err := srv.writeFrame(map[string]string{"test": "value"})

	require.Error(t, err, "writeFrame must return an error when the encoder is nil")
	assert.Contains(t, err.Error(), "not serving",
		"error message should indicate the server is not serving")
}

// TestWriteFrame_NilEncoder_ErrorIsNotWrapped verifies the exact sentinel text so
// writeOrLog can inspect it — and to ensure the error is not accidentally wrapped
// with additional context that would change the message contract.
func TestWriteFrame_NilEncoder_SentinelText(t *testing.T) {
	srv := New(nil)

	err := srv.writeFrame(nil)

	require.Error(t, err)
	// The sentinel is "acpserver: server not serving" — verify it is stable.
	assert.True(t, errors.Is(err, err), "error must satisfy errors.Is with itself")
	assert.Equal(t, "acpserver: server not serving", err.Error())
}
