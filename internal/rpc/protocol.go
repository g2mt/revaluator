package rpc

// Request is an incoming JSON-RPC message from the Neovim plugin.
type Request struct {
	// ID is an opaque string chosen by the client to correlate responses.
	ID string `json:"id"`

	// Method is the operation: "eval" or "shutdown".
	Method string `json:"method"`

	// Params carries method-specific arguments.
	// For "eval": {"source": "<full buffer>", "offset": <byte offset>}
	Params map[string]any `json:"params"`
}

// Response is a JSON-RPC reply sent back to the Neovim plugin.
type Response struct {
	// ID matches the Request.ID for correlation.
	ID string `json:"id"`

	// Value is the repr of the evaluated expression, empty if no value.
	Value string `json:"value"`

	// Error is a human-readable error message, empty on success.
	Error string `json:"error"`
}
