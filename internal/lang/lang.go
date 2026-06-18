package lang

import "context"

// Interpreter is the interface implemented by each embedded language runtime.
// One implementation is registered per build via a build-tagged init().
type Interpreter interface {
	// Start initializes the embedded interpreter. Called once at server startup.
	Start(ctx context.Context) error

	// Eval evaluates the expression located at the given 0-based line within
	// source.
	//
	// source is the full buffer text.
	// line is the 0-based cursor line number.
	//
	// The implementation evaluates everything prior to that line to
	// establish state, then evaluates the current expression and returns
	// its result.
	Eval(source string, line int) (string, error)

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
