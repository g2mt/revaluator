package lang

import "context"

// Interpreter is the interface implemented by each embedded language runtime.
// One implementation is registered per build via a build-tagged init().
type Interpreter interface {
	// Start initializes the embedded interpreter. Called once at server startup.
	Start(ctx context.Context) error

	// Eval evaluates the expression located at byte offset within source.
	//
	// source is the full buffer text.
	// offset is the byte position of the start of the current line.
	//
	// The implementation evaluates everything prior to offset to establish
	// state, then evaluates the current expression and returns its result.
	Eval(source string, offset int) (string, error)

	// Close terminates the interpreter and releases resources.
	Close() error
}

// Active is the single interpreter instance for this server process.
// It is set by the build-tagged language file's init() via Register.
var Active Interpreter

// Register sets the active interpreter. Called exactly once during init().
func Register(i Interpreter) {
	Active = i
}
