package application

import "errors"

// ErrUnsupportedContentBlock is the sentinel error returned by flattenContentBlocks when
// the prompt contains a block type the agent does not handle (image, audio, resource).
// Callers test with errors.Is to distinguish unsupported-block failures from other errors.
var ErrUnsupportedContentBlock = errors.New("unsupported content block")

// ACPErrorKind classifies an ACP request-handler failure independently of any
// transport. The interfaces/cli layer maps each kind onto its JSON-RPC error code
// (see adaptACPHandler in the cli package), so the application layer never imports
// pkg/acpserver and the transport stays an interface-layer concern.
type ACPErrorKind int

const (
	// ACPErrInvalidParams reports malformed params or a request the caller got
	// wrong (unknown session, prompt already in flight). Maps to JSON-RPC -32602.
	ACPErrInvalidParams ACPErrorKind = iota
	// ACPErrInternal reports a server-side failure (missing dependency, factory
	// error, corrupted session state). Maps to JSON-RPC -32603.
	ACPErrInternal
	// ACPErrMethodNotFound maps to JSON-RPC -32601. Reserved for handlers that
	// dispatch on a sub-method; unused today.
	ACPErrMethodNotFound
)

// ACPHandlerError is the transport-neutral error returned by ACPSessionService
// handlers. Kind selects the JSON-RPC code at the interface boundary; Message is the
// human-readable detail surfaced to the editor. Data carries machine-readable
// supplementary information (e.g. an error code) that the interface layer maps to the
// JSON-RPC error object's "data" field — keeping codes out of Message (C-3 fix).
type ACPHandlerError struct {
	Kind    ACPErrorKind
	Message string
	// Data is optional machine-readable context (e.g. an ErrorCode string).
	// It is mapped to the JSON-RPC error object's "data" field at the interface boundary.
	Data any
}

func (e *ACPHandlerError) Error() string { return e.Message }

// acpInvalidParamsWithData builds an ACPErrInvalidParams handler error carrying
// supplementary machine-readable data (e.g. an error code) in the Data field.
func acpInvalidParamsWithData(msg string, data any) *ACPHandlerError {
	return &ACPHandlerError{Kind: ACPErrInvalidParams, Message: msg, Data: data}
}

// acpInvalidParams builds an ACPErrInvalidParams handler error.
func acpInvalidParams(msg string) *ACPHandlerError {
	return &ACPHandlerError{Kind: ACPErrInvalidParams, Message: msg}
}

// acpInternal builds an ACPErrInternal handler error.
func acpInternal(msg string) *ACPHandlerError {
	return &ACPHandlerError{Kind: ACPErrInternal, Message: msg}
}

// acpMethodNotFound builds an ACPErrMethodNotFound handler error. Used by handlers
// that dispatch on a sub-method when the requested sub-method is unknown. Maps to
// JSON-RPC -32601 at the interface boundary.
func acpMethodNotFound(msg string) *ACPHandlerError {
	return &ACPHandlerError{Kind: ACPErrMethodNotFound, Message: msg}
}
