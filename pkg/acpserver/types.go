package acpserver

const (
	MethodInitialize               = "initialize"
	MethodSessionNew               = "session/new"
	MethodSessionPrompt            = "session/prompt"
	MethodSessionCancel            = "session/cancel"
	MethodSessionUpdate            = "session/update"
	MethodSessionRequestPermission = "session/request_permission"

	// ProtocolVersion is the ACP wire protocol version advertised in the
	// "initialize" handshake. It MUST be incremented when a backward-incompatible
	// change is made to the session lifecycle (e.g. a mandatory new field in
	// session/new, a changed error semantics, or removal of a previously
	// guaranteed method). Additive, backward-compatible extensions (new optional
	// methods, new optional response fields) do NOT require a version bump.
	//
	// See docs/ADR/018-acp-transparent-agent-server-protocol.md for the full
	// versioning policy and the rationale for the current version.
	ProtocolVersion int = 1
)
