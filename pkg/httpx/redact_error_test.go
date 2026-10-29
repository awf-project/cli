package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactErrorPreservesCauseWithoutDiagnostics(t *testing.T) {
	require.NoError(t, RedactError(nil))
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded, errors.New("PRIVATE_RESPONSE")} {
		original := &url.Error{Op: "POST", URL: "https://PRIVATE_TOKEN.invalid", Err: fmt.Errorf("PRIVATE_BODY: %w", cause)}
		safe := RedactError(original)
		require.ErrorIs(t, safe, cause)
		var transport *url.Error
		require.ErrorAs(t, safe, &transport)
		require.Same(t, original, transport)
		for _, formatted := range []string{safe.Error(), fmt.Sprintf("%v", safe), fmt.Sprintf("%+v", safe), fmt.Sprintf("%#v", safe)} {
			require.NotContains(t, formatted, "PRIVATE_")
		}
	}
}
