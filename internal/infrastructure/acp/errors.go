package acp

import (
	"github.com/awf-project/cli/internal/application"
	sdk "github.com/coder/acp-go-sdk"
)

// toACPError converts an application-layer ACPHandlerError to the SDK RequestError
// variant understood by the ACP transport. Returns nil when e is nil so callers can
// pass handler results directly without an extra nil-check.
//
// NewRequestError preserves the application's message and encodes its structured
// data for the wire. The SDK's specialized constructors use generic messages.
func toACPError(e *application.ACPHandlerError) error {
	if e == nil {
		return nil
	}
	var code int
	switch e.Kind {
	case application.ACPErrMethodNotFound:
		// NewMethodNotFound carries the method name in Data{"method": ...}; the reserved
		// MethodNotFound path has no human-readable detail to preserve beyond it.
		return sdk.NewMethodNotFound(e.Message) //nolint:wrapcheck // ACP transport requires the concrete *sdk.RequestError to preserve the JSON-RPC code and data.
	case application.ACPErrInvalidParams:
		code = -32602 // JSON-RPC invalid params
	default: // ACPErrInternal and unknown application errors
		code = -32603
	}
	return sdk.NewRequestError(code, e.Message, e.Data) //nolint:wrapcheck // The ACP transport expects the concrete SDK error.
}

func invalidParamsErr(msg string) error {
	return toACPError(&application.ACPHandlerError{Kind: application.ACPErrInvalidParams, Message: msg})
}

func internalErr(msg string) error {
	return toACPError(&application.ACPHandlerError{Kind: application.ACPErrInternal, Message: msg})
}

func methodNotFoundErr(method string) error {
	return toACPError(&application.ACPHandlerError{Kind: application.ACPErrMethodNotFound, Message: method})
}
